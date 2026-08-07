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
	trainingCallbackHome           = "training_home"
	trainingCallbackStart          = "training_start"
	trainingCallbackContinue       = "training_continue"
	trainingCallbackProgram        = "training_program"
	trainingCallbackAddSet         = "training_add_set"
	trainingCallbackFinishExercise = "training_finish_exercise"
	trainingCallbackNote           = "training_note"
	trainingCallbackPrograms       = "training_programs"
	trainingCallbackImport         = "training_import"
	trainingCallbackConfirmImport  = "training_confirm_import"
	trainingCallbackHistory        = "training_history"
	trainingCallbackPublish        = "training_publish"
	trainingCallbackSaveOnly       = "training_save_only"

	maxTrainingImportBytes = 1 << 20
)

func (b *Bot) registerTrainingHandlers() {
	currentCard := b.trainingCardMiddleware()
	b.b.Handle(&tele.Btn{Unique: trainingCallbackHome}, b.handleTrainingHome, currentCard)
	b.b.Handle(&tele.Btn{Unique: trainingCallbackStart}, b.handleTrainingStart, currentCard)
	b.b.Handle(&tele.Btn{Unique: trainingCallbackContinue}, b.handleTrainingContinue, currentCard)
	b.b.Handle(&tele.Btn{Unique: trainingCallbackProgram}, b.handleTrainingProgram, currentCard)
	b.b.Handle(&tele.Btn{Unique: trainingCallbackAddSet}, b.handleTrainingAddSet, currentCard)
	b.b.Handle(&tele.Btn{Unique: trainingCallbackFinishExercise}, b.handleTrainingFinishExercise, currentCard)
	b.b.Handle(&tele.Btn{Unique: trainingCallbackNote}, b.handleTrainingNote, currentCard)
	b.b.Handle(&tele.Btn{Unique: trainingCallbackPrograms}, b.handleTrainingPrograms, currentCard)
	b.b.Handle(&tele.Btn{Unique: trainingCallbackImport}, b.handleTrainingImport, currentCard)
	b.b.Handle(&tele.Btn{Unique: trainingCallbackConfirmImport}, b.handleTrainingConfirmImport, currentCard)
	b.b.Handle(&tele.Btn{Unique: trainingCallbackHistory}, b.handleTrainingHistory, currentCard)
	b.b.Handle(&tele.Btn{Unique: trainingCallbackPublish}, b.handleTrainingPublish, currentCard)
	b.b.Handle(&tele.Btn{Unique: trainingCallbackSaveOnly}, b.handleTrainingSaveOnly, currentCard)
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
	return b.showActiveTraining(ctx, c, session, "Отправь: 12Р 40КГ или 12Р -")
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
	case training.InputSet:
		raw := c.Text()
		b.deleteIncoming(c)
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
		b.deleteIncoming(c)
		return b.showImportPrompt(ctx, c, ownerID, "Нужен именно файл .txt или .csv.")
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
	if ext != ".txt" && ext != ".csv" {
		b.deleteIncoming(c)
		return b.showImportPrompt(ctx, c, ownerID, "Поддерживаются только файлы .txt и .csv.")
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
	preview, err := b.deps.Training.ConfirmImport(ctx, ownerID)
	if errors.Is(err, training.ErrNoPendingImport) {
		return b.showTrainingHome(ctx, c, ownerID, "Этот импорт уже обработан.")
	}
	if err != nil {
		return b.showTrainingFailure(ctx, c, ownerID, err)
	}
	exercises := 0
	for _, program := range preview.Programs {
		exercises += len(program.Exercises)
	}
	notice := fmt.Sprintf("Сохранено программ: %d, упражнений: %d.", len(preview.Programs), exercises)
	return b.showTrainingHome(ctx, c, ownerID, notice)
}

func (b *Bot) handleTrainingPrograms(c tele.Context) error {
	b.respond(c)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	ownerID := c.Sender().ID
	programs, err := b.deps.Training.Programs(ctx, ownerID)
	if err != nil {
		return b.showTrainingFailure(ctx, c, ownerID, err)
	}
	var text strings.Builder
	text.WriteString("<b>📚 Программы</b>\n")
	if len(programs) == 0 {
		text.WriteString("\nПока ничего не сохранено.")
	} else {
		for _, program := range programs {
			fmt.Fprintf(&text, "\n• %s — %d упр.", html.EscapeString(program.Name), len(program.Exercises))
		}
	}
	markup := &tele.ReplyMarkup{}
	markup.Inline(
		markup.Row(markup.Data("📎 Импорт", trainingCallbackImport)),
		markup.Row(markup.Data("‹ Назад", trainingCallbackHome)),
	)
	return b.editTrainingCard(ctx, c, ownerID, text.String(), markup)
}

func (b *Bot) handleTrainingHistory(c tele.Context) error {
	b.respond(c)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	ownerID := c.Sender().ID
	sessions, err := b.deps.Training.Recent(ctx, ownerID, 10)
	if err != nil {
		return b.showTrainingFailure(ctx, c, ownerID, err)
	}
	var text strings.Builder
	text.WriteString("<b>🕘 Последние тренировки</b>\n")
	if len(sessions) == 0 {
		text.WriteString("\nЗавершённых тренировок пока нет.")
	} else {
		for _, session := range sessions {
			sets := 0
			for _, exercise := range session.Exercises {
				sets += len(exercise.Sets)
			}
			fmt.Fprintf(&text, "\n• %s · %s · %d подх.",
				session.StartedAt.In(b.deps.Location).Format("02.01.2006"),
				html.EscapeString(session.ProgramName), sets,
			)
		}
	}
	markup := &tele.ReplyMarkup{}
	markup.Inline(markup.Row(markup.Data("‹ Назад", trainingCallbackHome)))
	return b.editTrainingCard(ctx, c, ownerID, text.String(), markup)
}

func (b *Bot) handleTrainingPublish(c tele.Context) error {
	b.respond(c)
	sessionID, err := strconv.ParseInt(strings.TrimSpace(c.Data()), 10, 64)
	if err != nil || sessionID <= 0 {
		return fmt.Errorf("invalid training publish callback %q", c.Data())
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	ownerID := c.Sender().ID
	session, err := b.deps.Training.Session(ctx, ownerID, sessionID)
	if err != nil {
		return b.showTrainingFailure(ctx, c, ownerID, err)
	}
	if b.deps.WorkoutChannelID == 0 {
		return b.showFinishedTraining(ctx, c, session, "Канал для публикации не настроен.")
	}
	if session.PublishedMessageID != nil {
		return b.showFinishedTraining(ctx, c, session, "Эта тренировка уже опубликована.")
	}
	message, err := b.b.Send(
		&tele.Chat{ID: b.deps.WorkoutChannelID},
		training.FormatFinished(session, b.deps.Location),
		&tele.SendOptions{ParseMode: tele.ModeHTML},
	)
	if err != nil {
		return b.showFinishedTraining(ctx, c, session, "Не удалось опубликовать: "+err.Error())
	}
	if err := b.deps.Training.MarkPublished(ctx, ownerID, session.ID, b.deps.WorkoutChannelID, message.ID); err != nil {
		b.deps.Logger.Error("mark training published", "err", err, "session_id", session.ID, "message_id", message.ID)
		return b.showFinishedTraining(ctx, c, session, "Сообщение отправлено, но отметка о публикации не сохранилась.")
	}
	session.PublishedChatID = &b.deps.WorkoutChannelID
	session.PublishedMessageID = &message.ID
	return b.showFinishedTraining(ctx, c, session, "Опубликовано в канале.")
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
			markup.Data("📎 Импорт", trainingCallbackImport),
		),
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
	exerciseID := strconv.FormatInt(exercise.ID, 10)
	markup := &tele.ReplyMarkup{}
	markup.Inline(
		markup.Row(
			markup.Data("➕ Подход", trainingCallbackAddSet, exerciseID),
			markup.Data("✅ Конец упражнения", trainingCallbackFinishExercise, exerciseID),
		),
		markup.Row(markup.Data("📝 Заметка", trainingCallbackNote, exerciseID)),
	)
	return b.editTrainingCard(ctx, c, session.OwnerID, training.FormatActiveCard(session, prompt), markup)
}

func (b *Bot) showFinishedTraining(ctx context.Context, c tele.Context, session training.Session, notice string) error {
	text := training.FormatFinished(session, b.deps.Location)
	if notice != "" {
		text += "\n\n<b>" + html.EscapeString(notice) + "</b>"
	}
	markup := &tele.ReplyMarkup{}
	rows := make([]tele.Row, 0, 2)
	if b.deps.WorkoutChannelID != 0 && session.PublishedMessageID == nil {
		rows = append(rows, markup.Row(markup.Data("📣 Опубликовать", trainingCallbackPublish, strconv.FormatInt(session.ID, 10))))
	}
	rows = append(rows, markup.Row(markup.Data("💾 Готово", trainingCallbackSaveOnly)))
	markup.Inline(rows...)
	return b.editTrainingCard(ctx, c, session.OwnerID, text, markup)
}

func (b *Bot) showImportPrompt(ctx context.Context, c tele.Context, ownerID int64, notice string) error {
	if err := b.deps.Training.Expect(ctx, ownerID, training.InputImportFile); err != nil {
		return b.showTrainingFailure(ctx, c, ownerID, err)
	}
	text := "<b>📎 Импорт программ</b>\n\nОтправь файл .txt или .csv размером до 1 МБ.\n\n" +
		"TXT: первая строка блока — название программы, остальные — упражнения. Блоки разделяются пустой строкой."
	if notice != "" {
		text += "\n\n<b>" + html.EscapeString(notice) + "</b>"
	}
	markup := &tele.ReplyMarkup{}
	markup.Inline(markup.Row(markup.Data("‹ Отмена", trainingCallbackHome)))
	return b.editTrainingCard(ctx, c, ownerID, text, markup)
}

func (b *Bot) showImportPreview(ctx context.Context, c tele.Context, ownerID int64, preview training.ImportPreview) error {
	var text strings.Builder
	fmt.Fprintf(&text, "<b>📎 %s</b>\n", html.EscapeString(preview.Filename))
	exercises := 0
	for _, program := range preview.Programs {
		exercises += len(program.Exercises)
		fmt.Fprintf(&text, "\n• %s — %d упр.", html.EscapeString(program.Name), len(program.Exercises))
	}
	fmt.Fprintf(&text, "\n\nНайдено программ: %d\nУпражнений: %d", len(preview.Programs), exercises)
	text.WriteString("\n\nПрограммы с совпадающими названиями будут заменены. Остальные сохранятся.")
	markup := &tele.ReplyMarkup{}
	markup.Inline(
		markup.Row(markup.Data("✅ Сохранить", trainingCallbackConfirmImport)),
		markup.Row(markup.Data("‹ Отмена", trainingCallbackHome)),
	)
	return b.editTrainingCard(ctx, c, ownerID, text.String(), markup)
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
