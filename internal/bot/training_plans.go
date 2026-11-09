package bot

import (
	"context"
	"errors"
	"fmt"
	"html"
	"strconv"
	"time"

	"fitlog/internal/training"
	tele "gopkg.in/telebot.v3"
)

type trainingPlanGroup struct {
	ID       int64
	Name     string
	Workouts []training.Program
}

func groupTrainingPlans(workouts []training.Program) []trainingPlanGroup {
	groups := make([]trainingPlanGroup, 0)
	indexes := make(map[int64]int)
	for _, workout := range workouts {
		index, exists := indexes[workout.PlanID]
		if !exists {
			index = len(groups)
			indexes[workout.PlanID] = index
			groups = append(groups, trainingPlanGroup{ID: workout.PlanID, Name: workout.PlanName, Workouts: []training.Program{}})
		}
		groups[index].Workouts = append(groups[index].Workouts, workout)
	}
	return groups
}

func trainingPlansCard(workouts []training.Program, editing bool) (string, *tele.ReplyMarkup) {
	text := "<b>🏋️ Начать тренировку</b>\n\nВыбери программу тренировок:"
	callback := trainingCallbackStartPlan
	if editing {
		text = "<b>📚 Программы тренировок</b>\n\nВыбери программу, затем тренировку для редактирования."
		callback = trainingCallbackEditPlan
	}
	markup := &tele.ReplyMarkup{}
	groups := groupTrainingPlans(workouts)
	rows := make([]tele.Row, 0, len(groups)+2)
	for _, group := range groups {
		rows = append(rows, markup.Row(markup.Data(
			truncateTrainingButton(group.Name), callback, strconv.FormatInt(group.ID, 10),
		)))
	}
	if len(groups) == 0 {
		text += "\n\nПока нет сохранённых программ. Создай программу через импорт."
	}
	if editing {
		rows = append(rows, markup.Row(markup.Data("➕ Создать / обновить программу", trainingCallbackImport)))
	}
	rows = append(rows, markup.Row(markup.Data("‹ Назад", trainingCallbackHome)))
	markup.Inline(rows...)
	return text, markup
}

func trainingPlanWorkoutsCard(workouts []training.Program, planID int64, editing bool) (string, *tele.ReplyMarkup, error) {
	for _, group := range groupTrainingPlans(workouts) {
		if group.ID != planID {
			continue
		}
		callback, back := trainingCallbackProgram, trainingCallbackStart
		prompt := "Выбери тренировку:"
		if editing {
			callback, back = trainingCallbackProgramView, trainingCallbackPrograms
			prompt = "Выбери тренировку для редактирования:"
		}
		markup := &tele.ReplyMarkup{}
		rows := make([]tele.Row, 0, len(group.Workouts)+1)
		for _, workout := range group.Workouts {
			rows = append(rows, markup.Row(markup.Data(
				truncateTrainingButton(workout.Name), callback, strconv.FormatInt(workout.ID, 10),
			)))
		}
		rows = append(rows, markup.Row(markup.Data("‹ К программам", back)))
		markup.Inline(rows...)
		return "<b>" + html.EscapeString(group.Name) + "</b>\n\n" + prompt, markup, nil
	}
	return "", nil, training.ErrNotFound
}

func (b *Bot) handleTrainingPlanWorkouts(c tele.Context) error {
	b.respond(c)
	planID, err := parseTrainingID(c.Data())
	if err != nil {
		return fmt.Errorf("invalid training plan callback: %w", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	ownerID := c.Sender().ID
	editing := c.Callback().Unique == trainingCallbackEditPlan
	if !editing {
		session, activeErr := b.deps.Training.Active(ctx, ownerID)
		if activeErr == nil {
			return b.showActiveTraining(ctx, c, session, "")
		}
		if !errors.Is(activeErr, training.ErrNoActiveSession) {
			return b.showTrainingFailure(ctx, c, ownerID, activeErr)
		}
	}
	if err := b.deps.Training.ClearInput(ctx, ownerID); err != nil {
		return b.showTrainingFailure(ctx, c, ownerID, err)
	}
	workouts, err := b.deps.Training.Programs(ctx, ownerID)
	if err != nil {
		return b.showTrainingFailure(ctx, c, ownerID, err)
	}
	text, markup, err := trainingPlanWorkoutsCard(workouts, planID, editing)
	if errors.Is(err, training.ErrNotFound) {
		text, markup = trainingPlansCard(workouts, editing)
		text = "Программа больше недоступна.\n\n" + text
	} else if err != nil {
		return b.showTrainingFailure(ctx, c, ownerID, err)
	}
	return b.editTrainingCard(ctx, c, ownerID, text, markup)
}
