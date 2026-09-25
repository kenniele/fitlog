package bot

import (
	"context"
	"errors"
	"fmt"
	"html"
	"strings"
	"time"

	tele "gopkg.in/telebot.v3"

	"fitlog/internal/training"
)

func (b *Bot) handleTrainingReplaceExercisePage(c tele.Context) error {
	b.respond(c)
	exerciseID, page, err := parseTrainingPair(c.Data())
	if err != nil || page < 1 {
		return fmt.Errorf("invalid session exercise replacement page %q", c.Data())
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	ownerID := c.Sender().ID
	session, err := b.deps.Training.Active(ctx, ownerID)
	if err != nil {
		return b.showTrainingFailure(ctx, c, ownerID, err)
	}
	current := session.CurrentExercise()
	if current == nil || current.ID != exerciseID {
		return b.showActiveTraining(ctx, c, session, "Карточка уже обновилась. Выбери упражнение ещё раз.")
	}
	if !current.CanReplace() {
		return b.showActiveTraining(ctx, c, session, "Заменить упражнение можно до первого записанного подхода.")
	}
	exercises, err := b.deps.Training.Exercises(ctx, ownerID, int(page), trainingPageSize)
	if err != nil {
		return b.showTrainingFailure(ctx, c, ownerID, err)
	}
	if err := b.deps.Training.ClearInput(ctx, ownerID); err != nil {
		return b.showTrainingFailure(ctx, c, ownerID, err)
	}
	text, markup := trainingReplacementCard(*current, exercises)
	return b.editTrainingCard(ctx, c, ownerID, text, markup)
}

func (b *Bot) handleTrainingReplaceExercise(c tele.Context) error {
	b.respond(c)
	exerciseID, targetID, err := parseTrainingPair(c.Data())
	if err != nil || targetID < 1 {
		return fmt.Errorf("invalid session exercise replacement %q", c.Data())
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	ownerID := c.Sender().ID
	session, err := b.deps.Training.ReplaceCurrentExercise(ctx, ownerID, exerciseID, targetID)
	if err != nil {
		if errors.Is(err, training.ErrNotEditable) || errors.Is(err, training.ErrExerciseHasSets) {
			if active, activeErr := b.deps.Training.Active(ctx, ownerID); activeErr == nil {
				notice := "Карточка уже обновилась. Выбери упражнение ещё раз."
				if errors.Is(err, training.ErrExerciseHasSets) {
					notice = "У упражнения уже есть записанные подходы. Замена доступна до первого подхода."
				}
				return b.showActiveTraining(ctx, c, active, notice)
			}
		}
		return b.showTrainingFailure(ctx, c, ownerID, err)
	}
	return b.showActiveTraining(ctx, c, session,
		"Упражнение заменено только в этой тренировке. Вводи вес и повторы для нового упражнения.")
}

func trainingReplacementCard(current training.SessionExercise, page training.ExercisePage) (string, *tele.ReplyMarkup) {
	var text strings.Builder
	text.WriteString("<b>🔁 Заменить упражнение</b>\n\n")
	text.WriteString("Сейчас: <b>" + html.EscapeString(current.Name) + "</b>\n\n")
	text.WriteString("Выбери упражнение для этой тренировки. Программа и прошлые тренировки сохранятся. " +
		"Плановые веса и разминка прежнего упражнения будут сброшены; подходы вводятся вручную.")

	markup := &tele.ReplyMarkup{}
	rows := make([]tele.Row, 0, len(page.Items)+2)
	for _, exercise := range page.Items {
		if current.ExerciseID != nil && exercise.ID == *current.ExerciseID {
			continue
		}
		rows = append(rows, markup.Row(markup.Data(
			truncateTrainingButton(exercise.Name), trainingCallbackReplaceExercise,
			trainingPair(current.ID, exercise.ID),
		)))
	}
	if len(rows) == 0 && page.TotalPages == 1 {
		text.WriteString("\n\nВ справочнике пока нет других упражнений.")
	}
	if page.TotalPages > 1 {
		var navigation tele.Row
		if page.Page > 1 {
			navigation = append(navigation, markup.Data("‹", trainingCallbackReplaceExercisePage, trainingPair(current.ID, int64(page.Page-1))))
		}
		navigation = append(navigation, markup.Data(
			fmt.Sprintf("%d/%d", page.Page, page.TotalPages), trainingCallbackReplaceExercisePage,
			trainingPair(current.ID, int64(page.Page)),
		))
		if page.Page < page.TotalPages {
			navigation = append(navigation, markup.Data("›", trainingCallbackReplaceExercisePage, trainingPair(current.ID, int64(page.Page+1))))
		}
		rows = append(rows, navigation)
	}
	rows = append(rows, markup.Row(markup.Data("‹ Отмена", trainingCallbackContinue)))
	markup.Inline(rows...)
	return text.String(), markup
}
