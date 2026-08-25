package bot

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"html"
	"io"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	tele "gopkg.in/telebot.v3"

	"fitlog/internal/training"
)

const (
	trainingCallbackHome                    = "training_home"
	trainingCallbackStart                   = "training_start"
	trainingCallbackContinue                = "training_continue"
	trainingCallbackProgram                 = "training_program"
	trainingCallbackAddSet                  = "training_add_set"
	trainingCallbackAddWarmup               = "training_add_warmup"
	trainingCallbackWarmupDone              = "training_warmup_done"
	trainingCallbackWorkingReps             = "training_work_reps"
	trainingCallbackRIR                     = "training_rir"
	trainingCallbackOverride                = "training_override"
	trainingCallbackReorder                 = "training_reorder"
	trainingCallbackPrioritizeExercise      = "training_prioritize_ex"
	trainingCallbackFinishExercise          = "training_finish_exercise"
	trainingCallbackNote                    = "training_note"
	trainingCallbackPrograms                = "training_programs"
	trainingCallbackExercises               = "training_exercises"
	trainingCallbackExercise                = "training_exercise"
	trainingCallbackRenameExercise          = "training_ex_rename"
	trainingCallbackProgramView             = "training_program_view"
	trainingCallbackProgramExercise         = "training_program_ex"
	trainingCallbackProgramExercisePage     = "training_program_ex_page"
	trainingCallbackProgramExerciseExisting = "tr_program_ex_existing"
	trainingCallbackProgramExerciseNew      = "tr_program_ex_new"
	trainingCallbackProgramExerciseOnly     = "tr_program_ex_only"
	trainingCallbackProgramExerciseHistory  = "tr_program_ex_history"
	trainingCallbackImport                  = "training_import"
	trainingCallbackConfirmImport           = "training_confirm_import"
	trainingCallbackImportExisting          = "training_import_existing"
	trainingCallbackImportNew               = "training_import_new"
	trainingCallbackHistory                 = "training_history"
	trainingCallbackHistorySession          = "training_history_session"
	trainingCallbackPublish                 = "training_publish"
	trainingCallbackPublishChannel          = "tr_publish_channel"
	trainingCallbackEdit                    = "training_edit"
	trainingCallbackReopen                  = "training_reopen"
	trainingCallbackEditBack                = "training_edit_back"
	trainingCallbackDelete                  = "training_delete"
	trainingCallbackConfirmDelete           = "training_delete_confirm"
	trainingCallbackDeleteLocal             = "training_delete_local"
	trainingCallbackSaveOnly                = "training_save_only"

	maxTrainingImportBytes = 1 << 20
	trainingPageSize       = 5
)

func (b *Bot) registerTrainingHandlers() {
	currentCard := b.trainingCardMiddleware()
	b.b.Handle(&tele.Btn{Unique: trainingCallbackHome}, b.handleTrainingHome, currentCard)
	b.b.Handle(&tele.Btn{Unique: trainingCallbackStart}, b.handleTrainingStart, currentCard)
	b.b.Handle(&tele.Btn{Unique: trainingCallbackContinue}, b.handleTrainingContinue, currentCard)
	b.b.Handle(&tele.Btn{Unique: trainingCallbackProgram}, b.handleTrainingProgram, currentCard)
	b.b.Handle(&tele.Btn{Unique: trainingCallbackAddSet}, b.handleTrainingAddSet, currentCard)
	b.b.Handle(&tele.Btn{Unique: trainingCallbackAddWarmup}, b.handleTrainingAddWarmup, currentCard)
	b.b.Handle(&tele.Btn{Unique: trainingCallbackWarmupDone}, b.handleTrainingWarmupDone, currentCard)
	b.b.Handle(&tele.Btn{Unique: trainingCallbackWorkingReps}, b.handleTrainingWorkingReps, currentCard)
	b.b.Handle(&tele.Btn{Unique: trainingCallbackRIR}, b.handleTrainingRIR, currentCard)
	b.b.Handle(&tele.Btn{Unique: trainingCallbackOverride}, b.handleTrainingOverride, currentCard)
	b.b.Handle(&tele.Btn{Unique: trainingCallbackReorder}, b.handleTrainingReorder, currentCard)
	b.b.Handle(&tele.Btn{Unique: trainingCallbackPrioritizeExercise}, b.handleTrainingPrioritizeExercise, currentCard)
	b.b.Handle(&tele.Btn{Unique: trainingCallbackFinishExercise}, b.handleTrainingFinishExercise, currentCard)
	b.b.Handle(&tele.Btn{Unique: trainingCallbackNote}, b.handleTrainingNote, currentCard)
	b.b.Handle(&tele.Btn{Unique: trainingCallbackPrograms}, b.handleTrainingPrograms, currentCard)
	b.b.Handle(&tele.Btn{Unique: trainingCallbackExercises}, b.handleTrainingExercises, currentCard)
	b.b.Handle(&tele.Btn{Unique: trainingCallbackExercise}, b.handleTrainingExercise, currentCard)
	b.b.Handle(&tele.Btn{Unique: trainingCallbackRenameExercise}, b.handleTrainingRenameExercise, currentCard)
	b.b.Handle(&tele.Btn{Unique: trainingCallbackProgramView}, b.handleTrainingProgramView, currentCard)
	b.b.Handle(&tele.Btn{Unique: trainingCallbackProgramExercise}, b.handleTrainingProgramExercise, currentCard)
	b.b.Handle(&tele.Btn{Unique: trainingCallbackProgramExercisePage}, b.handleTrainingProgramExercisePage, currentCard)
	b.b.Handle(&tele.Btn{Unique: trainingCallbackProgramExerciseExisting}, b.handleTrainingProgramExerciseExisting, currentCard)
	b.b.Handle(&tele.Btn{Unique: trainingCallbackProgramExerciseNew}, b.handleTrainingProgramExerciseNew, currentCard)
	b.b.Handle(&tele.Btn{Unique: trainingCallbackProgramExerciseOnly}, b.handleTrainingProgramExerciseOnly, currentCard)
	b.b.Handle(&tele.Btn{Unique: trainingCallbackProgramExerciseHistory}, b.handleTrainingProgramExerciseHistory, currentCard)
	b.b.Handle(&tele.Btn{Unique: trainingCallbackImport}, b.handleTrainingImport, currentCard)
	b.b.Handle(&tele.Btn{Unique: trainingCallbackConfirmImport}, b.handleTrainingConfirmImport, currentCard)
	b.b.Handle(&tele.Btn{Unique: trainingCallbackImportExisting}, b.handleTrainingImportExisting, currentCard)
	b.b.Handle(&tele.Btn{Unique: trainingCallbackImportNew}, b.handleTrainingImportNew, currentCard)
	b.b.Handle(&tele.Btn{Unique: trainingCallbackHistory}, b.handleTrainingHistory, currentCard)
	b.b.Handle(&tele.Btn{Unique: trainingCallbackPublish}, b.handleTrainingPublish, currentCard)
	b.b.Handle(&tele.Btn{Unique: trainingCallbackHistorySession}, b.handleTrainingHistorySession, currentCard)
	b.b.Handle(&tele.Btn{Unique: trainingCallbackPublishChannel}, b.handleTrainingPublishChannel, currentCard)
	b.b.Handle(&tele.Btn{Unique: trainingCallbackEdit}, b.handleTrainingEdit, currentCard)
	b.b.Handle(&tele.Btn{Unique: trainingCallbackReopen}, b.handleTrainingReopen, currentCard)
	b.b.Handle(&tele.Btn{Unique: trainingCallbackEditBack}, b.handleTrainingEditBack, currentCard)
	b.b.Handle(&tele.Btn{Unique: trainingCallbackDelete}, b.handleTrainingDelete, currentCard)
	b.b.Handle(&tele.Btn{Unique: trainingCallbackConfirmDelete}, b.handleTrainingConfirmDelete, currentCard)
	b.b.Handle(&tele.Btn{Unique: trainingCallbackDeleteLocal}, b.handleTrainingDeleteLocal, currentCard)
	b.b.Handle(&tele.Btn{Unique: trainingCallbackSaveOnly}, b.handleTrainingSaveOnly, currentCard)
}

func (b *Bot) handleImportProgramCommand(c tele.Context) error {
	if b.deps.Training == nil || c.Sender() == nil || c.Chat() == nil {
		return b.handleMenu(c)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	ownerID := c.Sender().ID
	if err := b.deps.Training.OpenControlMessage(ctx, ownerID, c.Chat().ID); err != nil {
		return b.showTrainingFailure(ctx, c, ownerID, err)
	}
	b.deleteIncoming(c)
	return b.showImportPrompt(ctx, c, ownerID, "")
}

func (b *Bot) handleTrainingButton(c tele.Context) error {
	if b.deps.Training == nil {
		return c.Send("Тренировки пока не настроены.", b.menu)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	ownerID := c.Sender().ID
	chat := c.Chat()
	if chat == nil {
		return fmt.Errorf("training card has no chat")
	}
	if err := b.deps.Training.OpenControlMessage(ctx, ownerID, chat.ID); err != nil {
		return b.showTrainingFailure(ctx, c, ownerID, err)
	}
	b.deleteIncoming(c)
	if session, err := b.deps.Training.Active(ctx, ownerID); err == nil {
		return b.showActiveTraining(ctx, c, session, "")
	} else if !errors.Is(err, training.ErrNoActiveSession) {
		return b.showTrainingFailure(ctx, c, ownerID, err)
	}
	return b.showTrainingHome(ctx, c, ownerID, "")
}

func (b *Bot) handleTrainingHome(c tele.Context) error {
	b.respond(c)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	ownerID := c.Sender().ID
	if err := b.deps.Training.ClearInput(ctx, ownerID); err != nil {
		return b.showTrainingFailure(ctx, c, ownerID, err)
	}
	return b.showTrainingHome(ctx, c, ownerID, "")
}

func (b *Bot) handleTrainingStart(c tele.Context) error {
	b.respond(c)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	ownerID := c.Sender().ID
	if session, err := b.deps.Training.Active(ctx, ownerID); err == nil {
		return b.showActiveTraining(ctx, c, session, "")
	} else if !errors.Is(err, training.ErrNoActiveSession) {
		return b.showTrainingFailure(ctx, c, ownerID, err)
	}
	programs, err := b.deps.Training.Programs(ctx, ownerID)
	if err != nil {
		return b.showTrainingFailure(ctx, c, ownerID, err)
	}
	if len(programs) == 0 {
		return b.showImportPrompt(ctx, c, ownerID, "Сначала импортируй хотя бы одну программу.")
	}
	markup := &tele.ReplyMarkup{}
	rows := make([]tele.Row, 0, len(programs)+1)
	for _, program := range programs {
		rows = append(rows, markup.Row(markup.Data(program.Name, trainingCallbackProgram, strconv.FormatInt(program.ID, 10))))
	}
	rows = append(rows, markup.Row(markup.Data("‹ Назад", trainingCallbackHome)))
	markup.Inline(rows...)
	return b.editTrainingCard(ctx, c, ownerID, "<b>🏋️ Начать тренировку</b>\n\nВыбери программу:", markup)
}

func (b *Bot) handleTrainingProgram(c tele.Context) error {
	b.respond(c)
	programID, err := strconv.ParseInt(strings.TrimSpace(c.Data()), 10, 64)
	if err != nil || programID <= 0 {
		return fmt.Errorf("invalid training program callback %q", c.Data())
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	ownerID := c.Sender().ID
	session, err := b.deps.Training.Start(ctx, ownerID, programID, time.Now())
	if errors.Is(err, training.ErrActiveSession) {
		session, err = b.deps.Training.Active(ctx, ownerID)
	}
	if err != nil {
		return b.showTrainingFailure(ctx, c, ownerID, err)
	}
	return b.showActiveTraining(ctx, c, session, "")
}

func (b *Bot) handleTrainingContinue(c tele.Context) error {
	b.respond(c)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	ownerID := c.Sender().ID
	session, err := b.deps.Training.Active(ctx, ownerID)
	if errors.Is(err, training.ErrNoActiveSession) {
		return b.showTrainingHome(ctx, c, ownerID, "Активной тренировки уже нет.")
	}
	if err != nil {
		return b.showTrainingFailure(ctx, c, ownerID, err)
	}
	return b.showActiveTraining(ctx, c, session, b.trainingPrompt(ctx, ownerID))
}

func (b *Bot) handleTrainingAddSet(c tele.Context) error {
	b.respond(c)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	ownerID := c.Sender().ID
	session, current, err := b.trainingCallbackExercise(ctx, c, ownerID)
	if err != nil {
		return b.showTrainingFailure(ctx, c, ownerID, err)
	}
	if !current {
		return b.showActiveTraining(ctx, c, session, "Карточка уже обновилась")
	}
	if err := b.deps.Training.Expect(ctx, ownerID, training.InputSet); err != nil {
		return b.showTrainingFailure(ctx, c, ownerID, err)
	}
	if exercise := session.CurrentExercise(); exercise != nil && exercise.Structured() {
		return b.showActiveTraining(ctx, c, session, "Отправь повторы числом или фактический подход: 10 либо 10Р 60КГ")
	}
	return b.showActiveTraining(ctx, c, session, "Отправь: 12Р 40КГ или 12Р -")
}

func (b *Bot) handleTrainingAddWarmup(c tele.Context) error {
	b.respond(c)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	ownerID := c.Sender().ID
	session, current, err := b.trainingCallbackExercise(ctx, c, ownerID)
	if err != nil {
		return b.showTrainingFailure(ctx, c, ownerID, err)
	}
	if !current {
		return b.showActiveTraining(ctx, c, session, "Карточка уже обновилась")
	}
	session, err = b.deps.Training.BeginWarmup(ctx, ownerID)
	if err != nil {
		return b.showTrainingFailure(ctx, c, ownerID, err)
	}
	return b.showActiveTraining(ctx, c, session, "Отправь разминочный подход: 10Р 20КГ или 10Р -")
}

func (b *Bot) handleTrainingWarmupDone(c tele.Context) error {
	b.respond(c)
	exerciseID, warmupPosition, err := parseTrainingPair(c.Data())
	if err != nil || warmupPosition <= 0 {
		return fmt.Errorf("invalid warmup callback %q", c.Data())
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	ownerID := c.Sender().ID
	session, activeErr := b.deps.Training.Active(ctx, ownerID)
	if activeErr != nil {
		return b.showTrainingFailure(ctx, c, ownerID, activeErr)
	}
	current := session.CurrentExercise()
	if current == nil || current.ID != exerciseID || int64(len(current.WarmupSets())+1) != warmupPosition {
		return b.showActiveTraining(ctx, c, session, "Карточка уже обновилась")
	}
	session, err = b.deps.Training.CompleteWarmup(ctx, ownerID, time.Now())
	if err != nil {
		return b.showTrainingFailure(ctx, c, ownerID, err)
	}
	return b.showActiveTraining(ctx, c, session, "")
}

func (b *Bot) handleTrainingWorkingReps(c tele.Context) error {
	b.respond(c)
	exerciseID, reps, err := parseTrainingPair(c.Data())
	if err != nil || reps <= 0 || reps > 1000 {
		return fmt.Errorf("invalid working set callback %q", c.Data())
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
		return b.showActiveTraining(ctx, c, session, "Карточка уже обновилась")
	}
	session, err = b.deps.Training.PrepareWorkingSet(ctx, ownerID, int(reps))
	if err != nil {
		return b.showTrainingFailure(ctx, c, ownerID, err)
	}
	return b.showActiveTraining(ctx, c, session, "Какой был запас повторений?")
}

func (b *Bot) handleTrainingRIR(c tele.Context) error {
	b.respond(c)
	raw := strings.TrimSpace(c.Data())
	var rir *float64
	if raw != "skip" {
		value, err := strconv.ParseFloat(raw, 64)
		if err != nil || value < 0 || value > 10 {
			return fmt.Errorf("invalid RIR callback %q", raw)
		}
		rir = &value
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	ownerID := c.Sender().ID
	session, err := b.deps.Training.CompletePendingSet(ctx, ownerID, rir, time.Now())
	if err != nil {
		return b.showTrainingFailure(ctx, c, ownerID, err)
	}
	return b.showTrainingAfterProgress(ctx, c, session)
}

func (b *Bot) handleTrainingOverride(c tele.Context) error {
	b.respond(c)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	ownerID := c.Sender().ID
	if _, current, err := b.trainingCallbackExercise(ctx, c, ownerID); err != nil {
		return b.showTrainingFailure(ctx, c, ownerID, err)
	} else if !current {
		session, activeErr := b.deps.Training.Active(ctx, ownerID)
		if activeErr != nil {
			return b.showTrainingFailure(ctx, c, ownerID, activeErr)
		}
		return b.showActiveTraining(ctx, c, session, "Карточка уже обновилась")
	}
	session, err := b.deps.Training.BeginOverride(ctx, ownerID)
	if err != nil {
		return b.showTrainingFailure(ctx, c, ownerID, err)
	}
	return b.showActiveTraining(ctx, c, session, "Измени: вес;подходы;повторы;RIR;отдых. Например: 60;3;8-12;2;180s")
}

func (b *Bot) showTrainingAfterProgress(ctx context.Context, c tele.Context, session training.Session) error {
	if session.Active() {
		return b.showActiveTraining(ctx, c, session, "")
	}
	updated, updateErr := b.updatePublishedTraining(session)
	if updateErr != nil {
		b.deps.Logger.Error("update published training", "err", updateErr, "session_id", session.ID)
		return b.showFinishedTraining(ctx, c, session, "Тренировка сохранена, но публикацию обновить не удалось: "+updateErr.Error())
	}
	if updated {
		return b.showFinishedTraining(ctx, c, session, "Публикация в канале обновлена.")
	}
	return b.showFinishedTraining(ctx, c, session, "")
}

func (b *Bot) handleTrainingNote(c tele.Context) error {
	b.respond(c)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	ownerID := c.Sender().ID
	session, current, err := b.trainingCallbackExercise(ctx, c, ownerID)
	if err != nil {
		return b.showTrainingFailure(ctx, c, ownerID, err)
	}
	if !current {
		return b.showActiveTraining(ctx, c, session, "Карточка уже обновилась")
	}
	if err := b.deps.Training.Expect(ctx, ownerID, training.InputNote); err != nil {
		return b.showTrainingFailure(ctx, c, ownerID, err)
	}
	return b.showActiveTraining(ctx, c, session, "Отправь заметку для этого упражнения")
}

func (b *Bot) handleTrainingReorder(c tele.Context) error {
	b.respond(c)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	ownerID := c.Sender().ID
	if err := b.deps.Training.ClearInput(ctx, ownerID); err != nil {
		return b.showTrainingFailure(ctx, c, ownerID, err)
	}
	session, err := b.deps.Training.Active(ctx, ownerID)
	if err != nil {
		return b.showTrainingFailure(ctx, c, ownerID, err)
	}
	return b.showTrainingReorder(ctx, c, session, "")
}

func (b *Bot) handleTrainingPrioritizeExercise(c tele.Context) error {
	b.respond(c)
	exerciseID, err := parseTrainingID(c.Data())
	if err != nil {
		return fmt.Errorf("invalid training prioritize callback %q: %w", c.Data(), err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	ownerID := c.Sender().ID
	session, err := b.deps.Training.PrioritizeExercise(ctx, ownerID, exerciseID)
	if errors.Is(err, training.ErrNotEditable) {
		if active, activeErr := b.deps.Training.Active(ctx, ownerID); activeErr == nil {
			return b.showActiveTraining(ctx, c, active, "Порядок уже изменился. Выбери упражнение ещё раз.")
		}
	}
	if err != nil {
		return b.showTrainingFailure(ctx, c, ownerID, err)
	}
	current := session.CurrentExercise()
	notice := "Порядок упражнений изменён."
	if current != nil {
		notice += " Сейчас: " + current.Name + "."
	}
	return b.showActiveTraining(ctx, c, session, notice)
}

func (b *Bot) handleTrainingFinishExercise(c tele.Context) error {
	b.respond(c)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	ownerID := c.Sender().ID
	session, current, err := b.trainingCallbackExercise(ctx, c, ownerID)
	if err != nil {
		return b.showTrainingFailure(ctx, c, ownerID, err)
	}
	if !current {
		return b.showActiveTraining(ctx, c, session, "Карточка уже обновилась")
	}
	session, err = b.deps.Training.FinishExercise(ctx, ownerID, time.Now())
	if err != nil {
		return b.showTrainingFailure(ctx, c, ownerID, err)
	}
	if session.Active() {
		return b.showActiveTraining(ctx, c, session, "")
	}
	updated, updateErr := b.updatePublishedTraining(session)
	if updateErr != nil {
		b.deps.Logger.Error("update published training", "err", updateErr, "session_id", session.ID)
		return b.showFinishedTraining(ctx, c, session, "Тренировка сохранена, но публикацию в канале обновить не удалось: "+updateErr.Error())
	}
	if updated {
		return b.showFinishedTraining(ctx, c, session, "Публикация в канале обновлена.")
	}
	return b.showFinishedTraining(ctx, c, session, "")
}

func (b *Bot) handleText(c tele.Context) error {
	if b.deps.Training == nil || c.Sender() == nil {
		return b.handleMenu(c)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	ownerID := c.Sender().ID
	state, err := b.deps.Training.State(ctx, ownerID)
	if err != nil {
		return b.handleMenu(c)
	}
	switch state.Mode {
	case training.InputWarmup:
		raw := c.Text()
		b.deleteIncoming(c)
		session, err := b.deps.Training.AddWarmup(ctx, ownerID, raw, time.Now())
		if err != nil {
			if active, activeErr := b.deps.Training.Active(ctx, ownerID); activeErr == nil {
				return b.showActiveTraining(ctx, c, active, "Не понял разминку: "+err.Error()+". Отправь, например: 10Р 20КГ")
			}
			return b.showTrainingFailure(ctx, c, ownerID, err)
		}
		return b.showActiveTraining(ctx, c, session, "")
	case training.InputSet:
		raw := c.Text()
		b.deleteIncoming(c)
		if active, activeErr := b.deps.Training.Active(ctx, ownerID); activeErr == nil {
			if exercise := active.CurrentExercise(); exercise != nil && exercise.Structured() {
				reps, parseErr := strconv.Atoi(strings.TrimSpace(raw))
				var session training.Session
				var prepareErr error
				if parseErr == nil && reps > 0 {
					session, prepareErr = b.deps.Training.PrepareWorkingSet(ctx, ownerID, reps)
				} else if actual, setErr := training.ParseSet(raw); setErr == nil {
					session, prepareErr = b.deps.Training.PrepareWorkingSetWithWeight(ctx, ownerID, actual.Reps, actual.WeightKG)
				} else {
					return b.showActiveTraining(ctx, c, active, "Отправь число повторений или фактический подход: 10 либо 10Р 60КГ")
				}
				if prepareErr != nil {
					return b.showTrainingFailure(ctx, c, ownerID, prepareErr)
				}
				return b.showActiveTraining(ctx, c, session, "Какой был запас повторений?")
			}
		}
		session, err := b.deps.Training.AddSet(ctx, ownerID, raw)
		if err != nil {
			if active, activeErr := b.deps.Training.Active(ctx, ownerID); activeErr == nil {
				return b.showActiveTraining(ctx, c, active, "Не понял: "+err.Error()+". Попробуй ещё раз")
			}
			return b.showTrainingFailure(ctx, c, ownerID, err)
		}
		return b.showActiveTraining(ctx, c, session, "")
	case training.InputNote:
		raw := c.Text()
		b.deleteIncoming(c)
		session, err := b.deps.Training.AddNote(ctx, ownerID, raw)
		if err != nil {
			if active, activeErr := b.deps.Training.Active(ctx, ownerID); activeErr == nil {
				return b.showActiveTraining(ctx, c, active, "Не удалось сохранить заметку: "+err.Error())
			}
			return b.showTrainingFailure(ctx, c, ownerID, err)
		}
		return b.showActiveTraining(ctx, c, session, "")
	case training.InputImportFile:
		raw := stripYAMLCodeBlock(c.Text())
		b.deleteIncoming(c)
		preview, err := b.deps.Training.PreviewImport(ctx, ownerID, "telegram-program.yaml", strings.NewReader(raw))
		if err != nil {
			return b.showImportPrompt(ctx, c, ownerID, "Не удалось разобрать YAML: "+err.Error())
		}
		return b.showImportPreview(ctx, c, ownerID, preview)
	case training.InputRIR:
		b.deleteIncoming(c)
		if active, activeErr := b.deps.Training.Active(ctx, ownerID); activeErr == nil {
			return b.showActiveTraining(ctx, c, active, "Выбери RIR кнопкой или нажми «Пропустить»")
		}
		return b.showTrainingFailure(ctx, c, ownerID, training.ErrNoActiveSession)
	case training.InputOverride:
		raw := c.Text()
		b.deleteIncoming(c)
		session, err := b.deps.Training.OverrideCurrentExercise(ctx, ownerID, raw)
		if err != nil {
			if active, activeErr := b.deps.Training.Active(ctx, ownerID); activeErr == nil {
				return b.showActiveTraining(ctx, c, active, "Не удалось изменить: "+err.Error())
			}
			return b.showTrainingFailure(ctx, c, ownerID, err)
		}
		return b.showActiveTraining(ctx, c, session, "Рекомендация изменена только для этой тренировки")
	case training.InputRename:
		raw := c.Text()
		b.deleteIncoming(c)
		result, err := b.deps.Training.RenameExercise(ctx, ownerID, raw)
		if err != nil {
			if state.PendingExerciseID != nil {
				if exercise, loadErr := b.deps.Training.Exercise(ctx, ownerID, *state.PendingExerciseID); loadErr == nil {
					return b.showTrainingExercise(ctx, c, exercise, 1, "Не удалось переименовать: "+err.Error())
				}
			}
			return b.showTrainingFailure(ctx, c, ownerID, err)
		}
		updated, failed := b.updatePublishedTrainings(result.PublishedSessions)
		notice := "Упражнение переименовано во всех программах и тренировках."
		if result.Merged {
			notice = "Упражнение заменено существующим во всех программах и тренировках."
		}
		if failed > 0 {
			notice += fmt.Sprintf(" Публикаций обновлено: %d; не удалось: %d.", updated, failed)
		} else if updated > 0 {
			notice += fmt.Sprintf(" Публикаций обновлено: %d.", updated)
		}
		return b.showTrainingExercises(ctx, c, ownerID, 1, notice)
	case training.InputProgramExerciseChoice:
		b.deleteIncoming(c)
		replacement, err := b.deps.Training.PendingProgramExerciseReplacement(ctx, ownerID)
		if err != nil {
			return b.showTrainingFailure(ctx, c, ownerID, err)
		}
		return b.showTrainingProgramExerciseChoices(ctx, c, ownerID, replacement, 1, "Выбери упражнение кнопкой ниже.")
	case training.InputProgramExerciseNew:
		raw := c.Text()
		b.deleteIncoming(c)
		replacement, err := b.deps.Training.PrepareNewProgramExercise(ctx, ownerID, raw)
		if err != nil {
			if pending, loadErr := b.deps.Training.PendingProgramExerciseReplacement(ctx, ownerID); loadErr == nil {
				return b.showTrainingProgramExerciseNew(ctx, c, ownerID, pending, "Не удалось сохранить название: "+err.Error())
			}
			return b.showTrainingFailure(ctx, c, ownerID, err)
		}
		return b.showTrainingProgramExerciseScope(ctx, c, ownerID, replacement)
	case training.InputProgramExerciseConfirm:
		b.deleteIncoming(c)
		replacement, err := b.deps.Training.PendingProgramExerciseReplacement(ctx, ownerID)
		if err != nil {
			return b.showTrainingFailure(ctx, c, ownerID, err)
		}
		return b.showTrainingProgramExerciseScope(ctx, c, ownerID, replacement)
	default:
		return b.handleMenu(c)
	}
}

func (b *Bot) handleTrainingImport(c tele.Context) error {
	b.respond(c)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	ownerID := c.Sender().ID
	if err := b.deps.Training.Expect(ctx, ownerID, training.InputImportFile); err != nil {
		return b.showTrainingFailure(ctx, c, ownerID, err)
	}
	return b.showImportPrompt(ctx, c, ownerID, "")
}

func (b *Bot) handleTrainingDocument(c tele.Context) error {
	if b.deps.Training == nil || c.Sender() == nil || c.Message() == nil || c.Message().Document == nil {
		return b.handleMenu(c)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	ownerID := c.Sender().ID
	state, err := b.deps.Training.State(ctx, ownerID)
	if err != nil || state.Mode != training.InputImportFile {
		return c.Send("Чтобы загрузить программу, сначала нажми «Тренировка 🏋️» → «Импорт».", b.menu)
	}
	document := c.Message().Document
	filename := document.FileName
	ext := strings.ToLower(filepath.Ext(filename))
	if ext != ".txt" && ext != ".csv" && ext != ".yaml" && ext != ".yml" {
		b.deleteIncoming(c)
		return b.showImportPrompt(ctx, c, ownerID, "Поддерживаются файлы .yaml, .yml, .txt и .csv.")
	}
	if document.FileSize > maxTrainingImportBytes {
		b.deleteIncoming(c)
		return b.showImportPrompt(ctx, c, ownerID, "Файл больше 1 МБ.")
	}
	reader, err := b.b.File(&document.File)
	if err != nil {
		b.deleteIncoming(c)
		return b.showImportPrompt(ctx, c, ownerID, "Не удалось скачать файл: "+err.Error())
	}
	data, readErr := io.ReadAll(io.LimitReader(reader, maxTrainingImportBytes+1))
	closeErr := reader.Close()
	b.deleteIncoming(c)
	if readErr != nil {
		return b.showImportPrompt(ctx, c, ownerID, "Не удалось прочитать файл: "+readErr.Error())
	}
	if closeErr != nil {
		b.deps.Logger.Warn("close training import", "err", closeErr)
	}
	if len(data) > maxTrainingImportBytes {
		return b.showImportPrompt(ctx, c, ownerID, "Файл больше 1 МБ.")
	}
	preview, err := b.deps.Training.PreviewImport(ctx, ownerID, filename, bytes.NewReader(data))
	if err != nil {
		return b.showImportPrompt(ctx, c, ownerID, "Не удалось разобрать файл: "+err.Error())
	}
	return b.showImportPreview(ctx, c, ownerID, preview)
}

func (b *Bot) handleTrainingConfirmImport(c tele.Context) error {
	b.respond(c)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	ownerID := c.Sender().ID
	review, err := b.deps.Training.BeginImportReview(ctx, ownerID)
	if errors.Is(err, training.ErrNoPendingImport) {
		return b.showTrainingHome(ctx, c, ownerID, "Этот импорт уже обработан.")
	}
	if err != nil {
		return b.showTrainingFailure(ctx, c, ownerID, err)
	}
	return b.showImportExerciseReview(ctx, c, ownerID, review)
}

func (b *Bot) handleTrainingImportExisting(c tele.Context) error {
	b.respond(c)
	exerciseID, err := parseTrainingID(c.Data())
	if err != nil {
		return fmt.Errorf("invalid existing import exercise %q: %w", c.Data(), err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	ownerID := c.Sender().ID
	review, done, err := b.deps.Training.UseExistingImportExercise(ctx, ownerID, exerciseID)
	if err != nil {
		return b.showTrainingFailure(ctx, c, ownerID, err)
	}
	if done {
		return b.finishTrainingImport(ctx, c, ownerID)
	}
	return b.showImportExerciseReview(ctx, c, ownerID, review)
}

func (b *Bot) handleTrainingImportNew(c tele.Context) error {
	b.respond(c)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	ownerID := c.Sender().ID
	review, done, err := b.deps.Training.KeepNewImportExercise(ctx, ownerID)
	if err != nil {
		return b.showTrainingFailure(ctx, c, ownerID, err)
	}
	if done {
		return b.finishTrainingImport(ctx, c, ownerID)
	}
	return b.showImportExerciseReview(ctx, c, ownerID, review)
}

func (b *Bot) finishTrainingImport(ctx context.Context, c tele.Context, ownerID int64) error {
	preview, err := b.deps.Training.ConfirmImport(ctx, ownerID)
	if err != nil {
		return b.showTrainingFailure(ctx, c, ownerID, err)
	}
	exercises := 0
	for _, program := range preview.Programs {
		exercises += len(program.Exercises)
	}
	return b.showTrainingHome(ctx, c, ownerID,
		fmt.Sprintf("Сохранено программ: %d, упражнений: %d.", len(preview.Programs), exercises),
	)
}

func (b *Bot) handleTrainingPrograms(c tele.Context) error {
	b.respond(c)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	ownerID := c.Sender().ID
	if err := b.deps.Training.ClearInput(ctx, ownerID); err != nil {
		return b.showTrainingFailure(ctx, c, ownerID, err)
	}
	programs, err := b.deps.Training.Programs(ctx, ownerID)
	if err != nil {
		return b.showTrainingFailure(ctx, c, ownerID, err)
	}
	var text strings.Builder
	fmt.Fprintf(&text, "<b>📚 %s</b>\n", html.EscapeString(trainingProgramsTitle(programs)))
	if len(programs) == 0 {
		text.WriteString("\nПока ничего не сохранено.")
	} else {
		text.WriteString("\nВыбери тренировку для редактирования.")
	}
	markup := &tele.ReplyMarkup{}
	rows := make([]tele.Row, 0, len(programs)+2)
	for _, program := range programs {
		rows = append(rows, markup.Row(markup.Data(
			truncateTrainingButton("✏️ "+program.Name),
			trainingCallbackProgramView,
			strconv.FormatInt(program.ID, 10),
		)))
	}
	rows = append(rows,
		markup.Row(markup.Data("📎 Импорт", trainingCallbackImport)),
		markup.Row(markup.Data("‹ Назад", trainingCallbackHome)),
	)
	markup.Inline(rows...)
	return b.editTrainingCard(ctx, c, ownerID, text.String(), markup)
}

func trainingProgramsTitle(programs []training.Program) string {
	if len(programs) == 0 || strings.TrimSpace(programs[0].PlanName) == "" {
		return "Программы"
	}
	planID := programs[0].PlanID
	planName := programs[0].PlanName
	for _, program := range programs[1:] {
		if program.PlanID != planID {
			return "Программы"
		}
	}
	return planName
}

func (b *Bot) handleTrainingProgramView(c tele.Context) error {
	b.respond(c)
	programID, err := parseTrainingID(c.Data())
	if err != nil {
		return fmt.Errorf("invalid training program callback %q: %w", c.Data(), err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	ownerID := c.Sender().ID
	if err := b.deps.Training.ClearInput(ctx, ownerID); err != nil {
		return b.showTrainingFailure(ctx, c, ownerID, err)
	}
	program, err := b.deps.Training.Program(ctx, ownerID, programID)
	if err != nil {
		return b.showTrainingFailure(ctx, c, ownerID, err)
	}
	return b.showTrainingProgram(ctx, c, program, "")
}

func (b *Bot) handleTrainingProgramExercise(c tele.Context) error {
	b.respond(c)
	programExerciseID, err := parseTrainingID(c.Data())
	if err != nil {
		return fmt.Errorf("invalid program exercise callback %q: %w", c.Data(), err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	ownerID := c.Sender().ID
	replacement, err := b.deps.Training.BeginProgramExerciseReplacement(ctx, ownerID, programExerciseID)
	if err != nil {
		return b.showTrainingFailure(ctx, c, ownerID, err)
	}
	return b.showTrainingProgramExerciseChoices(ctx, c, ownerID, replacement, 1, "")
}

func (b *Bot) handleTrainingProgramExercisePage(c tele.Context) error {
	b.respond(c)
	page, err := parseTrainingPage(c.Data())
	if err != nil {
		return fmt.Errorf("invalid program exercise page %q: %w", c.Data(), err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	ownerID := c.Sender().ID
	replacement, err := b.deps.Training.PendingProgramExerciseReplacement(ctx, ownerID)
	if err != nil {
		return b.showTrainingFailure(ctx, c, ownerID, err)
	}
	return b.showTrainingProgramExerciseChoices(ctx, c, ownerID, replacement, page, "")
}

func (b *Bot) handleTrainingProgramExerciseExisting(c tele.Context) error {
	b.respond(c)
	targetExerciseID, err := parseTrainingID(c.Data())
	if err != nil {
		return fmt.Errorf("invalid replacement exercise callback %q: %w", c.Data(), err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	ownerID := c.Sender().ID
	replacement, err := b.deps.Training.ChooseExistingProgramExercise(ctx, ownerID, targetExerciseID)
	if err != nil {
		return b.showTrainingFailure(ctx, c, ownerID, err)
	}
	return b.showTrainingProgramExerciseScope(ctx, c, ownerID, replacement)
}

func (b *Bot) handleTrainingProgramExerciseNew(c tele.Context) error {
	b.respond(c)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	ownerID := c.Sender().ID
	replacement, err := b.deps.Training.ExpectNewProgramExercise(ctx, ownerID)
	if err != nil {
		return b.showTrainingFailure(ctx, c, ownerID, err)
	}
	return b.showTrainingProgramExerciseNew(ctx, c, ownerID, replacement, "")
}

func (b *Bot) handleTrainingProgramExerciseOnly(c tele.Context) error {
	return b.handleTrainingProgramExerciseConfirmation(c, false)
}

func (b *Bot) handleTrainingProgramExerciseHistory(c tele.Context) error {
	return b.handleTrainingProgramExerciseConfirmation(c, true)
}

func (b *Bot) handleTrainingProgramExerciseConfirmation(c tele.Context, replaceHistory bool) error {
	b.respond(c)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	ownerID := c.Sender().ID
	result, err := b.deps.Training.ConfirmProgramExerciseReplacement(ctx, ownerID, replaceHistory)
	if err != nil {
		return b.showTrainingFailure(ctx, c, ownerID, err)
	}

	if !replaceHistory {
		return b.showTrainingProgram(ctx, c, result.Program,
			"Упражнение заменено только в программе. Прошлые тренировки и публикации не изменены.")
	}
	updated, failed := b.updatePublishedTrainings(result.PublishedSessions)
	notice := "Упражнение заменено в программе и прошлых тренировках."
	if failed > 0 {
		notice += fmt.Sprintf(" Публикаций обновлено: %d; не удалось: %d.", updated, failed)
	} else if updated > 0 {
		notice += fmt.Sprintf(" Публикаций обновлено: %d.", updated)
	}
	return b.showTrainingProgram(ctx, c, result.Program, notice)
}

func (b *Bot) handleTrainingExercises(c tele.Context) error {
	b.respond(c)
	page, err := parseTrainingPage(c.Data())
	if err != nil {
		return fmt.Errorf("invalid exercise page %q: %w", c.Data(), err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	ownerID := c.Sender().ID
	if err := b.deps.Training.ClearInput(ctx, ownerID); err != nil {
		return b.showTrainingFailure(ctx, c, ownerID, err)
	}
	return b.showTrainingExercises(ctx, c, ownerID, page, "")
}

func (b *Bot) handleTrainingExercise(c tele.Context) error {
	b.respond(c)
	exerciseID, pageValue, err := parseTrainingPair(c.Data())
	if err != nil {
		return fmt.Errorf("invalid exercise callback %q: %w", c.Data(), err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	ownerID := c.Sender().ID
	exercise, err := b.deps.Training.Exercise(ctx, ownerID, exerciseID)
	if err != nil {
		return b.showTrainingFailure(ctx, c, ownerID, err)
	}
	return b.showTrainingExercise(ctx, c, exercise, int(pageValue), "")
}

func (b *Bot) handleTrainingRenameExercise(c tele.Context) error {
	b.respond(c)
	exerciseID, pageValue, err := parseTrainingPair(c.Data())
	if err != nil {
		return fmt.Errorf("invalid rename exercise callback %q: %w", c.Data(), err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	ownerID := c.Sender().ID
	exercise, err := b.deps.Training.ExpectExerciseRename(ctx, ownerID, exerciseID)
	if err != nil {
		return b.showTrainingFailure(ctx, c, ownerID, err)
	}
	return b.showTrainingExercise(ctx, c, exercise, int(pageValue), "Отправь новое название одним сообщением.")
}

func (b *Bot) handleTrainingHistory(c tele.Context) error {
	b.respond(c)
	pageNumber, err := parseTrainingPage(c.Data())
	if err != nil {
		return fmt.Errorf("invalid training history page %q: %w", c.Data(), err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	ownerID := c.Sender().ID
	page, err := b.deps.Training.History(ctx, ownerID, pageNumber, trainingPageSize)
	if err != nil {
		return b.showTrainingFailure(ctx, c, ownerID, err)
	}
	var text strings.Builder
	text.WriteString("<b>🕘 Последние тренировки</b>\n")
	markup := &tele.ReplyMarkup{}
	rows := make([]tele.Row, 0, len(page.Items)+2)
	if len(page.Items) == 0 {
		text.WriteString("\nЗавершённых тренировок пока нет.")
	} else {
		for _, session := range page.Items {
			working, warmup := session.SetCounts()
			setsLabel := fmt.Sprintf("%d подх.", working)
			if warmup > 0 {
				setsLabel = fmt.Sprintf("%d раб. + %d разм.", working, warmup)
			}
			fmt.Fprintf(&text, "\n• %s · %s · %s · %s",
				session.StartedAt.In(b.deps.Location).Format("02.01.2006"),
				html.EscapeString(session.ProgramName), setsLabel,
				html.EscapeString(training.FormatSessionDuration(session)),
			)
			label := session.StartedAt.In(b.deps.Location).Format("02.01") + " · " + session.ProgramName
			rows = append(rows, markup.Row(markup.Data(
				truncateTrainingButton(label),
				trainingCallbackHistorySession,
				strconv.FormatInt(session.ID, 10),
			)))
		}
	}
	if page.TotalPages > 1 {
		var navigation tele.Row
		if page.Page > 1 {
			navigation = append(navigation, markup.Data("‹", trainingCallbackHistory, strconv.Itoa(page.Page-1)))
		}
		navigation = append(navigation, markup.Data(fmt.Sprintf("%d/%d", page.Page, page.TotalPages), trainingCallbackHistory, strconv.Itoa(page.Page)))
		if page.Page < page.TotalPages {
			navigation = append(navigation, markup.Data("›", trainingCallbackHistory, strconv.Itoa(page.Page+1)))
		}
		rows = append(rows, navigation)
	}
	rows = append(rows, markup.Row(markup.Data("‹ Назад", trainingCallbackHome)))
	markup.Inline(rows...)
	return b.editTrainingCard(ctx, c, ownerID, text.String(), markup)
}

func (b *Bot) handleTrainingHistorySession(c tele.Context) error {
	b.respond(c)
	sessionID, err := parseTrainingID(c.Data())
	if err != nil {
		return fmt.Errorf("invalid training history callback %q: %w", c.Data(), err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	ownerID := c.Sender().ID
	session, err := b.deps.Training.Session(ctx, ownerID, sessionID)
	if err != nil {
		return b.showTrainingFailure(ctx, c, ownerID, err)
	}
	if session.Active() {
		return b.showActiveTraining(ctx, c, session, "Тренировка ещё не завершена.")
	}
	return b.showFinishedTraining(ctx, c, session, "")
}

func (b *Bot) handleTrainingPublish(c tele.Context) error {
	b.respond(c)
	sessionID, err := parseTrainingID(c.Data())
	if err != nil {
		return fmt.Errorf("invalid training publish callback %q: %w", c.Data(), err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	ownerID := c.Sender().ID
	session, err := b.deps.Training.Session(ctx, ownerID, sessionID)
	if err != nil {
		return b.showTrainingFailure(ctx, c, ownerID, err)
	}
	if session.Active() {
		return b.showActiveTraining(ctx, c, session, "Сначала закончи тренировку.")
	}
	if len(b.deps.WorkoutChannelIDs) == 0 {
		return b.showFinishedTraining(ctx, c, session, "Канал для публикации не настроен.")
	}
	if session.PublishedMessageID != nil {
		return b.showFinishedTraining(ctx, c, session, "Эта тренировка уже опубликована.")
	}
	return b.showTrainingChannels(ctx, c, session)
}

func (b *Bot) handleTrainingPublishChannel(c tele.Context) error {
	b.respond(c)
	sessionID, channelID, err := parseTrainingPair(c.Data())
	if err != nil {
		return fmt.Errorf("invalid training channel callback %q: %w", c.Data(), err)
	}
	if !b.workoutChannelAllowed(channelID) {
		return fmt.Errorf("workout channel %d is not configured", channelID)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	ownerID := c.Sender().ID
	session, err := b.deps.Training.Session(ctx, ownerID, sessionID)
	if err != nil {
		return b.showTrainingFailure(ctx, c, ownerID, err)
	}
	if session.Active() {
		return b.showActiveTraining(ctx, c, session, "Сначала закончи тренировку.")
	}
	if session.PublishedMessageID != nil {
		return b.showFinishedTraining(ctx, c, session, "Эта тренировка уже опубликована.")
	}
	message, err := b.b.Send(
		&tele.Chat{ID: channelID},
		training.FormatFinished(session, b.deps.Location),
		&tele.SendOptions{ParseMode: tele.ModeHTML},
	)
	if err != nil {
		return b.showFinishedTraining(ctx, c, session, "Не удалось опубликовать: "+err.Error())
	}
	if err := b.deps.Training.MarkPublished(ctx, ownerID, session.ID, channelID, message.ID); err != nil {
		b.deps.Logger.Error("mark training published", "err", err, "session_id", session.ID, "message_id", message.ID)
		return b.showFinishedTraining(ctx, c, session, "Сообщение отправлено, но отметка о публикации не сохранилась.")
	}
	session.PublishedChatID = &channelID
	session.PublishedMessageID = &message.ID
	return b.showFinishedTraining(ctx, c, session, "Опубликовано в «"+b.workoutChannelTitle(channelID)+"».")
}

func (b *Bot) updatePublishedTraining(session training.Session) (bool, error) {
	if session.PublishedChatID == nil && session.PublishedMessageID == nil {
		return false, nil
	}
	if session.PublishedChatID == nil || session.PublishedMessageID == nil {
		return false, fmt.Errorf("неполная отметка о публикации")
	}
	stored := tele.StoredMessage{
		ChatID:    *session.PublishedChatID,
		MessageID: strconv.Itoa(*session.PublishedMessageID),
	}
	_, err := b.b.Edit(
		stored,
		training.FormatFinished(session, b.deps.Location),
		&tele.SendOptions{ParseMode: tele.ModeHTML, DisableWebPagePreview: true},
	)
	if err != nil && !isMessageNotModified(err) {
		return false, err
	}
	return true, nil
}

func (b *Bot) updatePublishedTrainings(sessions []training.Session) (updated, failed int) {
	for _, session := range sessions {
		ok, err := b.updatePublishedTraining(session)
		if err != nil {
			failed++
			b.deps.Logger.Error("update renamed exercise in published training", "err", err, "session_id", session.ID)
			continue
		}
		if ok {
			updated++
		}
	}
	return updated, failed
}

func (b *Bot) handleTrainingEdit(c tele.Context) error {
	b.respond(c)
	sessionID, err := parseTrainingID(c.Data())
	if err != nil {
		return fmt.Errorf("invalid training edit callback %q: %w", c.Data(), err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	ownerID := c.Sender().ID
	session, err := b.deps.Training.Session(ctx, ownerID, sessionID)
	if err != nil {
		return b.showTrainingFailure(ctx, c, ownerID, err)
	}
	return b.showTrainingExerciseEditor(ctx, c, session, "")
}

func (b *Bot) handleTrainingEditBack(c tele.Context) error {
	b.respond(c)
	sessionID, err := parseTrainingID(c.Data())
	if err != nil {
		return fmt.Errorf("invalid training edit back callback %q: %w", c.Data(), err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	ownerID := c.Sender().ID
	session, err := b.deps.Training.Session(ctx, ownerID, sessionID)
	if err != nil {
		return b.showTrainingFailure(ctx, c, ownerID, err)
	}
	if session.Active() {
		return b.showActiveTraining(ctx, c, session, b.trainingPrompt(ctx, ownerID))
	}
	return b.showFinishedTraining(ctx, c, session, "")
}

func (b *Bot) handleTrainingReopen(c tele.Context) error {
	b.respond(c)
	sessionID, exerciseID, err := parseTrainingPair(c.Data())
	if err != nil {
		return fmt.Errorf("invalid training reopen callback %q: %w", c.Data(), err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	ownerID := c.Sender().ID
	session, err := b.deps.Training.ReopenExercise(ctx, ownerID, sessionID, exerciseID)
	if err == nil {
		return b.showActiveTraining(ctx, c, session, "Упражнение открыто. Подходы и заметка сохранены.")
	}
	original, loadErr := b.deps.Training.Session(ctx, ownerID, sessionID)
	if loadErr == nil {
		switch {
		case errors.Is(err, training.ErrActiveSession):
			return b.showFinishedTraining(ctx, c, original, "Сначала закончи текущую активную тренировку.")
		case errors.Is(err, training.ErrNotEditable):
			return b.showTrainingExerciseEditor(ctx, c, original, "Это упражнение ещё не было завершено.")
		}
	}
	return b.showTrainingFailure(ctx, c, ownerID, err)
}

func (b *Bot) handleTrainingDelete(c tele.Context) error {
	b.respond(c)
	sessionID, err := parseTrainingID(c.Data())
	if err != nil {
		return fmt.Errorf("invalid training delete callback %q: %w", c.Data(), err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	ownerID := c.Sender().ID
	session, err := b.deps.Training.Session(ctx, ownerID, sessionID)
	if err != nil {
		return b.showTrainingFailure(ctx, c, ownerID, err)
	}
	return b.showTrainingDeleteConfirmation(ctx, c, session, "")
}

func (b *Bot) handleTrainingConfirmDelete(c tele.Context) error {
	b.respond(c)
	sessionID, err := parseTrainingID(c.Data())
	if err != nil {
		return fmt.Errorf("invalid training delete confirmation %q: %w", c.Data(), err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	ownerID := c.Sender().ID
	session, err := b.deps.Training.Session(ctx, ownerID, sessionID)
	if errors.Is(err, training.ErrNotFound) {
		return b.showTrainingHome(ctx, c, ownerID, "Тренировка уже удалена.")
	}
	if err != nil {
		return b.showTrainingFailure(ctx, c, ownerID, err)
	}
	if session.PublishedChatID != nil || session.PublishedMessageID != nil {
		if session.PublishedChatID == nil || session.PublishedMessageID == nil {
			return b.showTrainingDeleteFallback(ctx, c, session, "У тренировки повреждена отметка о публикации.")
		}
		stored := tele.StoredMessage{
			ChatID:    *session.PublishedChatID,
			MessageID: strconv.Itoa(*session.PublishedMessageID),
		}
		if err := b.b.Delete(stored); err != nil {
			if !isMessageAlreadyDeleted(err) {
				return b.showTrainingDeleteFallback(ctx, c, session, "Не удалось удалить публикацию из канала: "+err.Error())
			}
			b.deps.Logger.Warn("published training message already absent",
				"err", err, "session_id", session.ID, "chat_id", *session.PublishedChatID,
			)
		}
	}
	if err := b.deps.Training.DeleteSession(ctx, ownerID, session.ID); err != nil {
		return b.showTrainingFailure(ctx, c, ownerID, err)
	}
	return b.showTrainingHome(ctx, c, ownerID, "Тренировка удалена.")
}

func (b *Bot) handleTrainingDeleteLocal(c tele.Context) error {
	b.respond(c)
	sessionID, err := parseTrainingID(c.Data())
	if err != nil {
		return fmt.Errorf("invalid local training delete confirmation %q: %w", c.Data(), err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	ownerID := c.Sender().ID
	if err := b.deps.Training.DeleteSession(ctx, ownerID, sessionID); err != nil {
		if errors.Is(err, training.ErrNotFound) {
			return b.showTrainingHome(ctx, c, ownerID, "Тренировка уже удалена.")
		}
		return b.showTrainingFailure(ctx, c, ownerID, err)
	}
	return b.showTrainingHome(ctx, c, ownerID, "Тренировка удалена из Fitlog. Публикация в канале осталась.")
}

func (b *Bot) handleTrainingSaveOnly(c tele.Context) error {
	b.respond(c)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	ownerID := c.Sender().ID
	if err := b.deps.Training.ClearInput(ctx, ownerID); err != nil {
		return b.showTrainingFailure(ctx, c, ownerID, err)
	}
	return b.showTrainingHome(ctx, c, ownerID, "Тренировка сохранена.")
}

func (b *Bot) showTrainingHome(ctx context.Context, c tele.Context, ownerID int64, notice string) error {
	programs, err := b.deps.Training.Programs(ctx, ownerID)
	if err != nil {
		return b.showTrainingFailure(ctx, c, ownerID, err)
	}
	active, activeErr := b.deps.Training.Active(ctx, ownerID)
	if activeErr != nil && !errors.Is(activeErr, training.ErrNoActiveSession) {
		return b.showTrainingFailure(ctx, c, ownerID, activeErr)
	}
	var text strings.Builder
	text.WriteString("<b>🏋️ Тренировки</b>\n")
	if notice != "" {
		text.WriteString("\n✅ " + html.EscapeString(notice) + "\n")
	}
	if activeErr == nil {
		text.WriteString("\nАктивна: <b>" + html.EscapeString(active.ProgramName) + "</b>")
	} else {
		text.WriteString("\nАктивной тренировки нет.")
	}
	fmt.Fprintf(&text, "\nПрограмм сохранено: %d", len(programs))

	markup := &tele.ReplyMarkup{}
	rows := make([]tele.Row, 0, 4)
	if activeErr == nil {
		rows = append(rows, markup.Row(markup.Data("▶️ Продолжить", trainingCallbackContinue)))
	} else {
		rows = append(rows, markup.Row(markup.Data("▶️ Начать тренировку", trainingCallbackStart)))
	}
	rows = append(rows,
		markup.Row(
			markup.Data("📚 Программы", trainingCallbackPrograms),
			markup.Data("🏷 Упражнения", trainingCallbackExercises),
		),
		markup.Row(markup.Data("📎 Импорт", trainingCallbackImport)),
		markup.Row(markup.Data("🕘 История", trainingCallbackHistory)),
	)
	markup.Inline(rows...)
	return b.editTrainingCard(ctx, c, ownerID, text.String(), markup)
}

func (b *Bot) showActiveTraining(ctx context.Context, c tele.Context, session training.Session, prompt string) error {
	exercise := session.CurrentExercise()
	if exercise == nil {
		return b.showTrainingFailure(ctx, c, session.OwnerID, training.ErrNotFound)
	}
	previous, err := b.deps.Training.PreviousExercise(ctx, session.OwnerID, session.ID, exercise.Name)
	if err != nil {
		b.deps.Logger.Warn("load previous exercise", "err", err, "session_id", session.ID, "exercise", exercise.Name)
		previous = nil
	}
	exerciseID := strconv.FormatInt(exercise.ID, 10)
	sessionID := strconv.FormatInt(session.ID, 10)
	markup := &tele.ReplyMarkup{}
	if exercise.Structured() {
		state, stateErr := b.deps.Training.State(ctx, session.OwnerID)
		if stateErr != nil {
			return b.showTrainingFailure(ctx, c, session.OwnerID, stateErr)
		}
		rows := make([]tele.Row, 0, 6)
		if state.Mode == training.InputRIR && state.PendingSet != nil {
			rows = append(rows,
				markup.Row(
					markup.Data("RIR 0", trainingCallbackRIR, "0"),
					markup.Data("RIR 1", trainingCallbackRIR, "1"),
					markup.Data("RIR 2", trainingCallbackRIR, "2"),
				),
				markup.Row(
					markup.Data("RIR 3", trainingCallbackRIR, "3"),
					markup.Data("RIR 4+", trainingCallbackRIR, "4"),
					markup.Data("Пропустить", trainingCallbackRIR, "skip"),
				),
			)
		} else if len(exercise.WarmupSets()) < len(exercise.Warmup) {
			nextIndex := len(exercise.WarmupSets())
			next := exercise.Warmup[nextIndex]
			label := fmt.Sprintf("✅ Разминка: %d повторов", next.Reps)
			rows = append(rows, markup.Row(markup.Data(
				label, trainingCallbackWarmupDone, trainingPair(exercise.ID, int64(nextIndex+1)),
			)))
		} else if len(exercise.WorkingSets()) < exercise.Plan.WorkingSets {
			if len(exercise.WorkingSets()) == 0 {
				rows = append(rows, markup.Row(markup.Data("➕ Разминочный подход", trainingCallbackAddWarmup, exerciseID)))
			}
			if exercise.NextWorkingWeightKG() == nil {
				rows = append(rows, markup.Row(markup.Data("✏️ Указать рабочий вес", trainingCallbackOverride, exerciseID)))
			} else {
				var repsRow tele.Row
				for reps := exercise.Plan.MinReps; reps <= exercise.Plan.MaxReps && reps-exercise.Plan.MinReps < 5; reps++ {
					repsRow = append(repsRow, markup.Data(
						strconv.Itoa(reps), trainingCallbackWorkingReps, trainingPair(exercise.ID, int64(reps)),
					))
				}
				if len(repsRow) > 0 {
					rows = append(rows, repsRow)
				}
				rows = append(rows, markup.Row(markup.Data("Другой вес / повторы", trainingCallbackAddSet, exerciseID)))
			}
		} else {
			rows = append(rows, markup.Row(markup.Data("➕ Дополнительный подход", trainingCallbackAddSet, exerciseID)))
		}
		rows = append(rows,
			markup.Row(
				markup.Data("✏️ Изменить рекомендацию", trainingCallbackOverride, exerciseID),
				markup.Data("📝 Заметка", trainingCallbackNote, exerciseID),
			),
		)
		if hasAnotherUnfinishedExercise(session, exercise.ID) {
			rows = append(rows, markup.Row(markup.Data("🔀 Изменить порядок", trainingCallbackReorder)))
		}
		rows = append(rows,
			markup.Row(markup.Data("✅ Завершить упражнение", trainingCallbackFinishExercise, exerciseID)),
			markup.Row(markup.Data("🗑 Удалить тренировку", trainingCallbackDelete, sessionID)),
		)
		markup.Inline(rows...)
		return b.editTrainingCard(ctx, c, session.OwnerID,
			training.FormatActiveCard(session, previous, b.deps.Location, prompt), markup,
		)
	}
	rows := []tele.Row{
		markup.Row(
			markup.Data("➕ Подход", trainingCallbackAddSet, exerciseID),
			markup.Data("✅ Конец упражнения", trainingCallbackFinishExercise, exerciseID),
		),
		markup.Row(
			markup.Data("📝 Заметка", trainingCallbackNote, exerciseID),
			markup.Data("✏️ Исправить", trainingCallbackEdit, sessionID),
		),
	}
	if hasAnotherUnfinishedExercise(session, exercise.ID) {
		rows = append(rows, markup.Row(markup.Data("🔀 Изменить порядок", trainingCallbackReorder)))
	}
	rows = append(rows, markup.Row(markup.Data("🗑 Удалить тренировку", trainingCallbackDelete, sessionID)))
	markup.Inline(rows...)
	return b.editTrainingCard(ctx, c, session.OwnerID,
		training.FormatActiveCard(session, previous, b.deps.Location, prompt), markup,
	)
}

func (b *Bot) showTrainingReorder(ctx context.Context, c tele.Context, session training.Session, notice string) error {
	current := session.CurrentExercise()
	if current == nil {
		return b.showTrainingFailure(ctx, c, session.OwnerID, training.ErrNotFound)
	}
	var text strings.Builder
	text.WriteString("<b>🔀 Изменить порядок</b>\n\n")
	text.WriteString("Сейчас: <b>" + html.EscapeString(current.Name) + "</b>\n\n")
	text.WriteString("Выбери упражнение, которое выполнить сейчас. Оно встанет перед текущим, а текущее останется в очереди.")
	if notice != "" {
		text.WriteString("\n\n<b>" + html.EscapeString(notice) + "</b>")
	}

	markup := &tele.ReplyMarkup{}
	rows := make([]tele.Row, 0, len(session.Exercises))
	for _, exercise := range session.Exercises {
		if exercise.Complete || exercise.ID == current.ID {
			continue
		}
		rows = append(rows, markup.Row(markup.Data(
			truncateTrainingButton(fmt.Sprintf("%d. %s", exercise.Position, exercise.Name)),
			trainingCallbackPrioritizeExercise,
			strconv.FormatInt(exercise.ID, 10),
		)))
	}
	rows = append(rows, markup.Row(markup.Data("‹ Назад", trainingCallbackContinue)))
	markup.Inline(rows...)
	return b.editTrainingCard(ctx, c, session.OwnerID, text.String(), markup)
}

func hasAnotherUnfinishedExercise(session training.Session, currentExerciseID int64) bool {
	for _, exercise := range session.Exercises {
		if !exercise.Complete && exercise.ID != currentExerciseID {
			return true
		}
	}
	return false
}

func (b *Bot) showFinishedTraining(ctx context.Context, c tele.Context, session training.Session, notice string) error {
	text := training.FormatFinished(session, b.deps.Location)
	if notice != "" {
		text += "\n\n<b>" + html.EscapeString(notice) + "</b>"
	}
	markup := &tele.ReplyMarkup{}
	rows := make([]tele.Row, 0, 4)
	if len(b.deps.WorkoutChannelIDs) > 0 && session.PublishedMessageID == nil {
		rows = append(rows, markup.Row(markup.Data("📣 Опубликовать", trainingCallbackPublish, strconv.FormatInt(session.ID, 10))))
	}
	rows = append(rows, markup.Row(markup.Data("✏️ Исправить упражнение", trainingCallbackEdit, strconv.FormatInt(session.ID, 10))))
	rows = append(rows,
		markup.Row(markup.Data("🗑 Удалить тренировку", trainingCallbackDelete, strconv.FormatInt(session.ID, 10))),
		markup.Row(markup.Data("💾 Готово", trainingCallbackSaveOnly)),
	)
	markup.Inline(rows...)
	return b.editTrainingCard(ctx, c, session.OwnerID, text, markup)
}

func (b *Bot) showTrainingDeleteConfirmation(
	ctx context.Context,
	c tele.Context,
	session training.Session,
	notice string,
) error {
	var text strings.Builder
	text.WriteString("<b>🗑 Удалить тренировку?</b>\n\n")
	fmt.Fprintf(&text, "%s · %s\n", session.StartedAt.In(b.deps.Location).Format("02.01.2006"), html.EscapeString(session.ProgramName))
	if session.Active() {
		text.WriteString("Активная тренировка и все введённые подходы будут удалены.")
	} else {
		text.WriteString("Тренировка и все её подходы будут удалены из истории.")
	}
	if session.PublishedMessageID != nil {
		text.WriteString(" Бот также удалит публикацию из канала.")
	}
	text.WriteString("\n\n<b>Это действие нельзя отменить.</b>")
	if notice != "" {
		text.WriteString("\n\n" + html.EscapeString(notice))
	}

	sessionID := strconv.FormatInt(session.ID, 10)
	markup := &tele.ReplyMarkup{}
	markup.Inline(
		markup.Row(markup.Data("🗑 Да, удалить", trainingCallbackConfirmDelete, sessionID)),
		markup.Row(markup.Data("‹ Не удалять", trainingCallbackEditBack, sessionID)),
	)
	return b.editTrainingCard(ctx, c, session.OwnerID, text.String(), markup)
}

func (b *Bot) showTrainingDeleteFallback(
	ctx context.Context,
	c tele.Context,
	session training.Session,
	reason string,
) error {
	var text strings.Builder
	text.WriteString("<b>Не удалось удалить публикацию из канала</b>\n\n")
	text.WriteString(html.EscapeString(reason))
	text.WriteString("\n\nМожно удалить тренировку только из Fitlog. Сообщение в канале останется.")

	sessionID := strconv.FormatInt(session.ID, 10)
	markup := &tele.ReplyMarkup{}
	markup.Inline(
		markup.Row(markup.Data("🗑 Удалить только из Fitlog", trainingCallbackDeleteLocal, sessionID)),
		markup.Row(markup.Data("‹ Не удалять", trainingCallbackEditBack, sessionID)),
	)
	return b.editTrainingCard(ctx, c, session.OwnerID, text.String(), markup)
}

func (b *Bot) showTrainingChannels(ctx context.Context, c tele.Context, session training.Session) error {
	var text strings.Builder
	text.WriteString("<b>📣 Куда опубликовать?</b>\n\n")
	fmt.Fprintf(&text, "%s · %s\n", session.StartedAt.In(b.deps.Location).Format("02.01.2006"), html.EscapeString(session.ProgramName))
	text.WriteString("Выбери канал:")

	markup := &tele.ReplyMarkup{}
	rows := make([]tele.Row, 0, len(b.deps.WorkoutChannelIDs)+1)
	for _, channelID := range b.deps.WorkoutChannelIDs {
		payload := trainingPair(session.ID, channelID)
		rows = append(rows, markup.Row(markup.Data(
			"📣 "+truncateTrainingButton(b.workoutChannelTitle(channelID)),
			trainingCallbackPublishChannel,
			payload,
		)))
	}
	rows = append(rows, markup.Row(markup.Data(
		"‹ Назад", trainingCallbackEditBack, strconv.FormatInt(session.ID, 10),
	)))
	markup.Inline(rows...)
	return b.editTrainingCard(ctx, c, session.OwnerID, text.String(), markup)
}

func (b *Bot) showTrainingExerciseEditor(
	ctx context.Context,
	c tele.Context,
	session training.Session,
	notice string,
) error {
	var text strings.Builder
	text.WriteString("<b>✏️ Исправить упражнение</b>\n\n")
	text.WriteString("Выбери упражнение. Оно снова станет текущим; сохранённые подходы и заметка останутся на месте.")
	if notice != "" {
		text.WriteString("\n\n<b>" + html.EscapeString(notice) + "</b>")
	}

	markup := &tele.ReplyMarkup{}
	rows := make([]tele.Row, 0, len(session.Exercises)+1)
	for _, exercise := range session.Exercises {
		if session.Active() && !exercise.Complete && exercise.Position != session.CurrentPosition {
			continue
		}
		label := fmt.Sprintf("%d. %s", exercise.Position, exercise.Name)
		if exercise.Position == session.CurrentPosition && session.Active() {
			label += " · сейчас"
		}
		rows = append(rows, markup.Row(markup.Data(
			truncateTrainingButton(label),
			trainingCallbackReopen,
			trainingPair(session.ID, exercise.ID),
		)))
	}
	rows = append(rows, markup.Row(markup.Data(
		"‹ Назад", trainingCallbackEditBack, strconv.FormatInt(session.ID, 10),
	)))
	markup.Inline(rows...)
	return b.editTrainingCard(ctx, c, session.OwnerID, text.String(), markup)
}

func (b *Bot) showTrainingExercises(
	ctx context.Context,
	c tele.Context,
	ownerID int64,
	pageNumber int,
	notice string,
) error {
	page, err := b.deps.Training.Exercises(ctx, ownerID, pageNumber, trainingPageSize)
	if err != nil {
		return b.showTrainingFailure(ctx, c, ownerID, err)
	}
	var text strings.Builder
	text.WriteString("<b>🏷 Упражнения</b>\n")
	if notice != "" {
		text.WriteString("\n<b>" + html.EscapeString(notice) + "</b>\n")
	}
	if len(page.Items) == 0 {
		text.WriteString("\nКаталог пока пуст. Упражнения появятся после импорта программы.")
	} else {
		for _, exercise := range page.Items {
			fmt.Fprintf(&text, "\n• <b>%s</b>\n  Программы: %s",
				html.EscapeString(exercise.Name), html.EscapeString(formatExercisePrograms(exercise.Programs)),
			)
		}
		fmt.Fprintf(&text, "\n\nВсего: %d", page.TotalItems)
	}

	markup := &tele.ReplyMarkup{}
	rows := make([]tele.Row, 0, len(page.Items)+2)
	for _, exercise := range page.Items {
		rows = append(rows, markup.Row(markup.Data(
			truncateTrainingButton(exercise.Name), trainingCallbackExercise,
			trainingPair(exercise.ID, int64(page.Page)),
		)))
	}
	if page.TotalPages > 1 {
		var navigation tele.Row
		if page.Page > 1 {
			navigation = append(navigation, markup.Data("‹", trainingCallbackExercises, strconv.Itoa(page.Page-1)))
		}
		navigation = append(navigation, markup.Data(fmt.Sprintf("%d/%d", page.Page, page.TotalPages), trainingCallbackExercises, strconv.Itoa(page.Page)))
		if page.Page < page.TotalPages {
			navigation = append(navigation, markup.Data("›", trainingCallbackExercises, strconv.Itoa(page.Page+1)))
		}
		rows = append(rows, navigation)
	}
	rows = append(rows, markup.Row(markup.Data("‹ Назад", trainingCallbackHome)))
	markup.Inline(rows...)
	return b.editTrainingCard(ctx, c, ownerID, text.String(), markup)
}

func (b *Bot) showTrainingExercise(
	ctx context.Context,
	c tele.Context,
	exercise training.Exercise,
	page int,
	notice string,
) error {
	var text strings.Builder
	text.WriteString("<b>🏷 " + html.EscapeString(exercise.Name) + "</b>\n")
	text.WriteString("\nПрограммы: " + html.EscapeString(formatExercisePrograms(exercise.Programs)))
	if notice != "" {
		text.WriteString("\n\n<b>" + html.EscapeString(notice) + "</b>")
	}
	markup := &tele.ReplyMarkup{}
	markup.Inline(
		markup.Row(markup.Data(
			"✏️ Поменять название", trainingCallbackRenameExercise,
			trainingPair(exercise.ID, int64(page)),
		)),
		markup.Row(markup.Data("‹ К упражнениям", trainingCallbackExercises, strconv.Itoa(page))),
	)
	return b.editTrainingCard(ctx, c, exercise.OwnerID, text.String(), markup)
}

func (b *Bot) showTrainingProgram(
	ctx context.Context,
	c tele.Context,
	program training.Program,
	notice string,
) error {
	var text strings.Builder
	text.WriteString("<b>📚 " + html.EscapeString(program.Name) + "</b>\n")
	text.WriteString("\nНажми упражнение, которое хочешь заменить:")
	for _, exercise := range program.ExerciseItems {
		fmt.Fprintf(&text, "\n%d. %s", exercise.Position, html.EscapeString(exercise.Name))
	}
	if notice != "" {
		text.WriteString("\n\n✅ " + html.EscapeString(notice))
	}
	markup := &tele.ReplyMarkup{}
	rows := make([]tele.Row, 0, len(program.ExerciseItems)+1)
	for _, exercise := range program.ExerciseItems {
		rows = append(rows, markup.Row(markup.Data(
			truncateTrainingButton(fmt.Sprintf("%d. %s", exercise.Position, exercise.Name)),
			trainingCallbackProgramExercise,
			strconv.FormatInt(exercise.ID, 10),
		)))
	}
	rows = append(rows, markup.Row(markup.Data("‹ К программам", trainingCallbackPrograms)))
	markup.Inline(rows...)
	return b.editTrainingCard(ctx, c, program.OwnerID, text.String(), markup)
}

func (b *Bot) showTrainingProgramExerciseChoices(
	ctx context.Context,
	c tele.Context,
	ownerID int64,
	replacement training.ProgramExerciseReplacement,
	pageNumber int,
	notice string,
) error {
	page, err := b.deps.Training.Exercises(ctx, ownerID, pageNumber, trainingPageSize)
	if err != nil {
		return b.showTrainingFailure(ctx, c, ownerID, err)
	}
	var text strings.Builder
	text.WriteString("<b>🔁 Замена в программе</b>\n")
	text.WriteString("\n" + html.EscapeString(replacement.Program.Name))
	fmt.Fprintf(&text, "\n%d. <b>%s</b>", replacement.Current.Position, html.EscapeString(replacement.Current.Name))
	text.WriteString("\n\nВыбери существующее упражнение или добавь новое.")
	if notice != "" {
		text.WriteString("\n\n<b>" + html.EscapeString(notice) + "</b>")
	}

	markup := &tele.ReplyMarkup{}
	rows := make([]tele.Row, 0, len(page.Items)+3)
	for _, exercise := range page.Items {
		if replacement.Current.ExerciseID != nil && exercise.ID == *replacement.Current.ExerciseID {
			continue
		}
		rows = append(rows, markup.Row(markup.Data(
			truncateTrainingButton("♻️ "+exercise.Name),
			trainingCallbackProgramExerciseExisting,
			strconv.FormatInt(exercise.ID, 10),
		)))
	}
	if page.TotalPages > 1 {
		var navigation tele.Row
		if page.Page > 1 {
			navigation = append(navigation, markup.Data("‹", trainingCallbackProgramExercisePage, strconv.Itoa(page.Page-1)))
		}
		navigation = append(navigation, markup.Data(
			fmt.Sprintf("%d/%d", page.Page, page.TotalPages),
			trainingCallbackProgramExercisePage,
			strconv.Itoa(page.Page),
		))
		if page.Page < page.TotalPages {
			navigation = append(navigation, markup.Data("›", trainingCallbackProgramExercisePage, strconv.Itoa(page.Page+1)))
		}
		rows = append(rows, navigation)
	}
	rows = append(rows,
		markup.Row(markup.Data("➕ Новое упражнение", trainingCallbackProgramExerciseNew)),
		markup.Row(markup.Data(
			"‹ Отмена", trainingCallbackProgramView, strconv.FormatInt(replacement.Program.ID, 10),
		)),
	)
	markup.Inline(rows...)
	return b.editTrainingCard(ctx, c, ownerID, text.String(), markup)
}

func (b *Bot) showTrainingProgramExerciseNew(
	ctx context.Context,
	c tele.Context,
	ownerID int64,
	replacement training.ProgramExerciseReplacement,
	notice string,
) error {
	text := "<b>➕ Новое упражнение</b>\n\n" +
		html.EscapeString(replacement.Program.Name) + "\n" +
		fmt.Sprintf("%d. %s", replacement.Current.Position, html.EscapeString(replacement.Current.Name)) +
		"\n\nОтправь название нового упражнения одним сообщением."
	if notice != "" {
		text += "\n\n<b>" + html.EscapeString(notice) + "</b>"
	}
	markup := &tele.ReplyMarkup{}
	markup.Inline(markup.Row(markup.Data(
		"‹ Отмена", trainingCallbackProgramView, strconv.FormatInt(replacement.Program.ID, 10),
	)))
	return b.editTrainingCard(ctx, c, ownerID, text, markup)
}

func (b *Bot) showTrainingProgramExerciseScope(
	ctx context.Context,
	c tele.Context,
	ownerID int64,
	replacement training.ProgramExerciseReplacement,
) error {
	text := "<b>🔁 Замена в программе</b>\n\n" +
		html.EscapeString(replacement.Program.Name) + "\n" +
		fmt.Sprintf("%d. ", replacement.Current.Position) + html.EscapeString(replacement.Current.Name) +
		" → <b>" + html.EscapeString(replacement.Target.Name) + "</b>\n\n" +
		"Заменить упражнение в прошлых завершённых тренировках этой программы и их публикациях в каналах?"
	markup := &tele.ReplyMarkup{}
	markup.Inline(
		markup.Row(markup.Data("Нет, только программу", trainingCallbackProgramExerciseOnly)),
		markup.Row(markup.Data("Да, заменить и в каналах", trainingCallbackProgramExerciseHistory)),
		markup.Row(markup.Data(
			"‹ Отмена", trainingCallbackProgramView, strconv.FormatInt(replacement.Program.ID, 10),
		)),
	)
	return b.editTrainingCard(ctx, c, ownerID, text, markup)
}

func (b *Bot) showImportExerciseReview(
	ctx context.Context,
	c tele.Context,
	ownerID int64,
	review training.ImportExerciseReview,
) error {
	var text strings.Builder
	text.WriteString("<b>🔎 Проверка упражнений</b>\n")
	fmt.Fprintf(&text, "\n%d из %d · %s", review.Current, review.Total, html.EscapeString(review.ProgramName))
	fmt.Fprintf(&text, "\n\nДобавляется: <b>%s</b>", html.EscapeString(review.ProposedName))
	if len(review.Similar) == 0 {
		text.WriteString("\n\nПохожих упражнений в каталоге не найдено.")
	} else {
		text.WriteString("\n\nВыбери существующее упражнение для замены или добавь новое:")
	}
	markup := &tele.ReplyMarkup{}
	rows := make([]tele.Row, 0, len(review.Similar)+2)
	for _, exercise := range review.Similar {
		label := "♻️ " + exercise.Name
		rows = append(rows, markup.Row(markup.Data(
			truncateTrainingButton(label), trainingCallbackImportExisting, strconv.FormatInt(exercise.ID, 10),
		)))
	}
	rows = append(rows,
		markup.Row(markup.Data("➕ Добавить новым", trainingCallbackImportNew)),
		markup.Row(markup.Data("‹ Отмена", trainingCallbackHome)),
	)
	markup.Inline(rows...)
	return b.editTrainingCard(ctx, c, ownerID, text.String(), markup)
}

func (b *Bot) showImportPrompt(ctx context.Context, c tele.Context, ownerID int64, notice string) error {
	if err := b.deps.Training.Expect(ctx, ownerID, training.InputImportFile); err != nil {
		return b.showTrainingFailure(ctx, c, ownerID, err)
	}
	text := "<b>📥 Импорт программы</b>\n\nОтправь YAML файлом, обычным сообщением или код-блоком. Также поддерживаются старые .txt и .csv до 1 МБ.\n\n" +
		"YAML version 1 может содержать несколько тренировочных дней, назначения, разминку, отдых и double progression."
	if notice != "" {
		text += "\n\n<b>" + html.EscapeString(notice) + "</b>"
	}
	markup := &tele.ReplyMarkup{}
	markup.Inline(markup.Row(markup.Data("‹ Отмена", trainingCallbackHome)))
	return b.editTrainingCard(ctx, c, ownerID, text, markup)
}

func (b *Bot) showImportPreview(ctx context.Context, c tele.Context, ownerID int64, preview training.ImportPreview) error {
	var text strings.Builder
	fmt.Fprintf(&text, "<b>📥 Найдена программа</b>\n\n%s\n", html.EscapeString(preview.Filename))
	exercises := 0
	for _, program := range preview.Programs {
		exercises += len(program.Exercises)
		label := program.Name
		if program.WorkoutKey != "" {
			label = program.WorkoutKey + " — " + label
		}
		fmt.Fprintf(&text, "\n• %s — %d упр.", html.EscapeString(label), len(program.Exercises))
	}
	if len(preview.Programs) > 0 && preview.Programs[0].PlanName != "" {
		fmt.Fprintf(&text, "\n\n<b>%s</b>", html.EscapeString(preview.Programs[0].PlanName))
	}
	fmt.Fprintf(&text, "\nТренировок: %d\nУпражнений: %d", len(preview.Programs), exercises)
	text.WriteString("\n\nПосле подтверждения будет создана новая revision. Старые тренировки не изменятся.")
	markup := &tele.ReplyMarkup{}
	markup.Inline(
		markup.Row(markup.Data("✅ Проверить и импортировать", trainingCallbackConfirmImport)),
		markup.Row(markup.Data("‹ Отмена", trainingCallbackHome)),
	)
	return b.editTrainingCard(ctx, c, ownerID, text.String(), markup)
}

func formatExercisePrograms(programs []string) string {
	if len(programs) == 0 {
		return "ни в одной"
	}
	return strings.Join(programs, ", ")
}

func (b *Bot) showTrainingFailure(ctx context.Context, c tele.Context, ownerID int64, err error) error {
	b.deps.Logger.Error("training flow", "err", err, "owner_id", ownerID)
	markup := &tele.ReplyMarkup{}
	markup.Inline(markup.Row(markup.Data("‹ В тренировки", trainingCallbackHome)))
	return b.editTrainingCard(ctx, c, ownerID,
		"<b>Не удалось выполнить действие</b>\n\n"+html.EscapeString(err.Error()), markup,
	)
}

func (b *Bot) editTrainingCard(ctx context.Context, c tele.Context, ownerID int64, text string, markup *tele.ReplyMarkup) error {
	chatID := int64(0)
	messageID := 0
	if callback := c.Callback(); callback != nil && callback.Message != nil && callback.Message.Chat != nil {
		chatID = callback.Message.Chat.ID
		messageID = callback.Message.ID
	} else if chat := c.Chat(); chat != nil {
		chatID = chat.ID
	}
	state, err := b.deps.Training.State(ctx, ownerID)
	if err != nil {
		return err
	}
	if messageID == 0 {
		messageID = state.MessageID
	}
	if chatID == 0 {
		chatID = state.ChatID
	}
	options := &tele.SendOptions{ParseMode: tele.ModeHTML, DisableWebPagePreview: true}
	if chatID != 0 && messageID != 0 {
		stored := tele.StoredMessage{ChatID: chatID, MessageID: strconv.Itoa(messageID)}
		if _, editErr := b.b.Edit(stored, text, options, markup); editErr == nil || isMessageNotModified(editErr) {
			return b.deps.Training.SaveControlMessage(ctx, ownerID, chatID, messageID)
		} else {
			b.deps.Logger.Warn("edit training card; sending replacement", "err", editErr, "chat_id", chatID, "message_id", messageID)
		}
	}
	if chatID == 0 {
		return fmt.Errorf("training card has no chat")
	}
	message, err := b.b.Send(&tele.Chat{ID: chatID}, text, options, markup)
	if err != nil {
		return err
	}
	return b.deps.Training.SaveControlMessage(ctx, ownerID, chatID, message.ID)
}

func (b *Bot) trainingPrompt(ctx context.Context, ownerID int64) string {
	state, err := b.deps.Training.State(ctx, ownerID)
	if err != nil {
		return ""
	}
	switch state.Mode {
	case training.InputSet:
		return "Отправь: 12Р 40КГ или 12Р -"
	case training.InputRIR:
		return "Выбери RIR кнопкой или пропусти"
	case training.InputOverride:
		return "Измени: вес;подходы;повторы;RIR;отдых"
	case training.InputNote:
		return "Отправь заметку для этого упражнения"
	default:
		return ""
	}
}

func (b *Bot) trainingCallbackExercise(ctx context.Context, c tele.Context, ownerID int64) (training.Session, bool, error) {
	expectedID, err := strconv.ParseInt(strings.TrimSpace(c.Data()), 10, 64)
	if err != nil || expectedID <= 0 {
		return training.Session{}, false, fmt.Errorf("invalid exercise callback %q", c.Data())
	}
	session, err := b.deps.Training.Active(ctx, ownerID)
	if err != nil {
		return training.Session{}, false, err
	}
	exercise := session.CurrentExercise()
	return session, exercise != nil && exercise.ID == expectedID, nil
}

func (b *Bot) deleteIncoming(c tele.Context) {
	if c.Message() == nil {
		return
	}
	if err := c.Delete(); err != nil {
		b.deps.Logger.Debug("delete temporary training input", "err", err, "message_id", c.Message().ID)
	}
}

func (b *Bot) respond(c tele.Context) {
	if c.Callback() == nil {
		return
	}
	if err := c.Respond(); err != nil {
		b.deps.Logger.Debug("answer training callback", "err", err)
	}
}

func (b *Bot) trainingCardMiddleware() tele.MiddlewareFunc {
	return func(next tele.HandlerFunc) tele.HandlerFunc {
		return func(c tele.Context) error {
			if b.deps.Training == nil || c.Sender() == nil || c.Callback() == nil || c.Callback().Message == nil {
				return next(c)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			state, err := b.deps.Training.State(ctx, c.Sender().ID)
			if err != nil {
				return err
			}
			if state.MessageID != 0 && state.MessageID != c.Callback().Message.ID {
				return c.Respond(&tele.CallbackResponse{Text: "Эта карточка устарела. Открой «Тренировка 🏋️» ещё раз.", ShowAlert: true})
			}
			return next(c)
		}
	}
}

func isMessageNotModified(err error) bool {
	return strings.Contains(strings.ToLower(err.Error()), "message is not modified")
}

func isMessageAlreadyDeleted(err error) bool {
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "message to delete not found") ||
		strings.Contains(message, "message not found")
}

func parseTrainingID(raw string) (int64, error) {
	id, err := strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
	if err != nil || id <= 0 {
		return 0, fmt.Errorf("invalid ID")
	}
	return id, nil
}

func parseTrainingPage(raw string) (int, error) {
	if strings.TrimSpace(raw) == "" {
		return 1, nil
	}
	page, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || page < 1 {
		return 0, fmt.Errorf("page must be positive")
	}
	return page, nil
}

func stripYAMLCodeBlock(raw string) string {
	value := strings.TrimSpace(raw)
	if !strings.HasPrefix(value, "```") {
		return value
	}
	lines := strings.Split(value, "\n")
	if len(lines) < 2 {
		return value
	}
	lines = lines[1:]
	if len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) == "```" {
		lines = lines[:len(lines)-1]
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

func trainingPair(first, second int64) string {
	return strconv.FormatInt(first, 10) + ":" + strconv.FormatInt(second, 10)
}

func parseTrainingPair(raw string) (int64, int64, error) {
	parts := strings.Split(strings.TrimSpace(raw), ":")
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("expected two IDs")
	}
	first, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil || first <= 0 {
		return 0, 0, fmt.Errorf("invalid first ID")
	}
	second, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil || second == 0 {
		return 0, 0, fmt.Errorf("invalid second ID")
	}
	return first, second, nil
}

func (b *Bot) workoutChannelAllowed(channelID int64) bool {
	for _, configured := range b.deps.WorkoutChannelIDs {
		if configured == channelID {
			return true
		}
	}
	return false
}

func (b *Bot) workoutChannelTitle(channelID int64) string {
	chat, err := b.b.ChatByID(channelID)
	if err != nil {
		b.deps.Logger.Warn("resolve workout channel", "err", err, "channel_id", channelID)
		return strconv.FormatInt(channelID, 10)
	}
	if title := strings.TrimSpace(chat.Title); title != "" {
		return title
	}
	if username := strings.TrimSpace(chat.Username); username != "" {
		return "@" + username
	}
	return strconv.FormatInt(channelID, 10)
}

func truncateTrainingButton(value string) string {
	const maxRunes = 48
	runes := []rune(strings.TrimSpace(value))
	if len(runes) <= maxRunes {
		return string(runes)
	}
	return string(runes[:maxRunes-1]) + "…"
}
