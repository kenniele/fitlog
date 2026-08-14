package training

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"
)

type UseCase struct {
	repo Repository
}

func NewUseCase(repo Repository) *UseCase {
	return &UseCase{repo: repo}
}

func (u *UseCase) State(ctx context.Context, ownerID int64) (UIState, error) {
	state, err := u.repo.GetUIState(ctx, ownerID)
	if err == nil {
		return state, nil
	}
	if !errors.Is(err, ErrNotFound) {
		return UIState{}, err
	}
	return UIState{OwnerID: ownerID}, nil
}

func (u *UseCase) SaveControlMessage(ctx context.Context, ownerID, chatID int64, messageID int) error {
	state, err := u.State(ctx, ownerID)
	if err != nil {
		return err
	}
	state.ChatID = chatID
	state.MessageID = messageID
	return u.repo.SaveUIState(ctx, state)
}

// OpenControlMessage starts a fresh workout card in the current chat. The old
// message ID is forgotten so opening the section from the persistent reply
// keyboard cannot silently edit a card that is far above in the conversation.
func (u *UseCase) OpenControlMessage(ctx context.Context, ownerID, chatID int64) error {
	state, err := u.State(ctx, ownerID)
	if err != nil {
		return err
	}
	state.ChatID = chatID
	state.MessageID = 0
	state.Mode = InputNone
	state.PendingImport = nil
	state.PendingExerciseID = nil
	return u.repo.SaveUIState(ctx, state)
}

func (u *UseCase) Expect(ctx context.Context, ownerID int64, mode InputMode) error {
	state, err := u.State(ctx, ownerID)
	if err != nil {
		return err
	}
	state.Mode = mode
	if mode != InputImportOK {
		state.PendingImport = nil
	}
	if mode != InputRename {
		state.PendingExerciseID = nil
	}
	return u.repo.SaveUIState(ctx, state)
}

func (u *UseCase) ClearInput(ctx context.Context, ownerID int64) error {
	state, err := u.State(ctx, ownerID)
	if err != nil {
		return err
	}
	state.Mode = InputNone
	state.PendingImport = nil
	state.PendingExerciseID = nil
	return u.repo.SaveUIState(ctx, state)
}

func (u *UseCase) PreviewImport(ctx context.Context, ownerID int64, filename string, r io.Reader) (ImportPreview, error) {
	programs, err := ParsePrograms(filename, r)
	if err != nil {
		return ImportPreview{}, err
	}
	preview := ImportPreview{Filename: filename, Programs: programs}
	state, err := u.State(ctx, ownerID)
	if err != nil {
		return ImportPreview{}, err
	}
	state.Mode = InputImportOK
	state.PendingImport = &preview
	if err := u.repo.SaveUIState(ctx, state); err != nil {
		return ImportPreview{}, err
	}
	return preview, nil
}

func (u *UseCase) ConfirmImport(ctx context.Context, ownerID int64) (ImportPreview, error) {
	state, err := u.State(ctx, ownerID)
	if err != nil {
		return ImportPreview{}, err
	}
	if state.PendingImport == nil || len(state.PendingImport.Programs) == 0 {
		return ImportPreview{}, ErrNoPendingImport
	}
	preview := *state.PendingImport
	if err := u.repo.ReplacePrograms(ctx, ownerID, preview.Programs); err != nil {
		return ImportPreview{}, err
	}
	state.Mode = InputNone
	state.PendingImport = nil
	if err := u.repo.SaveUIState(ctx, state); err != nil {
		return ImportPreview{}, err
	}
	return preview, nil
}

func (u *UseCase) BeginImportReview(ctx context.Context, ownerID int64) (ImportExerciseReview, error) {
	state, err := u.State(ctx, ownerID)
	if err != nil {
		return ImportExerciseReview{}, err
	}
	if state.PendingImport == nil || len(state.PendingImport.Programs) == 0 {
		return ImportExerciseReview{}, ErrNoPendingImport
	}
	state.Mode = InputImportOK
	state.PendingImport.ReviewStarted = true
	state.PendingImport.ReviewProgram = 0
	state.PendingImport.ReviewExercise = 0
	if err := u.repo.SaveUIState(ctx, state); err != nil {
		return ImportExerciseReview{}, err
	}
	return u.importReview(ctx, ownerID, state)
}

func (u *UseCase) ImportReview(ctx context.Context, ownerID int64) (ImportExerciseReview, error) {
	state, err := u.State(ctx, ownerID)
	if err != nil {
		return ImportExerciseReview{}, err
	}
	return u.importReview(ctx, ownerID, state)
}

func (u *UseCase) UseExistingImportExercise(ctx context.Context, ownerID, exerciseID int64) (ImportExerciseReview, bool, error) {
	state, err := u.State(ctx, ownerID)
	if err != nil {
		return ImportExerciseReview{}, false, err
	}
	review, err := u.importReview(ctx, ownerID, state)
	if err != nil {
		return ImportExerciseReview{}, false, err
	}
	exercise, err := u.repo.Exercise(ctx, ownerID, exerciseID)
	if err != nil {
		return ImportExerciseReview{}, false, err
	}
	state.PendingImport.Programs[review.ProgramIndex].Exercises[review.ExerciseIndex] = exercise.Name
	u.advanceImportReview(state.PendingImport)
	if err := u.repo.SaveUIState(ctx, state); err != nil {
		return ImportExerciseReview{}, false, err
	}
	if state.PendingImport.ReviewProgram >= len(state.PendingImport.Programs) {
		return ImportExerciseReview{}, true, nil
	}
	next, err := u.importReview(ctx, ownerID, state)
	return next, false, err
}

func (u *UseCase) KeepNewImportExercise(ctx context.Context, ownerID int64) (ImportExerciseReview, bool, error) {
	state, err := u.State(ctx, ownerID)
	if err != nil {
		return ImportExerciseReview{}, false, err
	}
	if _, err := u.importReview(ctx, ownerID, state); err != nil {
		return ImportExerciseReview{}, false, err
	}
	u.advanceImportReview(state.PendingImport)
	if err := u.repo.SaveUIState(ctx, state); err != nil {
		return ImportExerciseReview{}, false, err
	}
	if state.PendingImport.ReviewProgram >= len(state.PendingImport.Programs) {
		return ImportExerciseReview{}, true, nil
	}
	next, err := u.importReview(ctx, ownerID, state)
	return next, false, err
}

func (u *UseCase) importReview(ctx context.Context, ownerID int64, state UIState) (ImportExerciseReview, error) {
	preview := state.PendingImport
	if preview == nil || !preview.ReviewStarted || preview.ReviewProgram >= len(preview.Programs) {
		return ImportExerciseReview{}, ErrNoPendingImport
	}
	program := preview.Programs[preview.ReviewProgram]
	if preview.ReviewExercise >= len(program.Exercises) {
		return ImportExerciseReview{}, ErrNoPendingImport
	}
	proposed := program.Exercises[preview.ReviewExercise]
	similar, err := u.repo.SimilarExercises(ctx, ownerID, proposed, 5)
	if err != nil {
		return ImportExerciseReview{}, err
	}
	current := 0
	total := 0
	for i, item := range preview.Programs {
		total += len(item.Exercises)
		if i < preview.ReviewProgram {
			current += len(item.Exercises)
		}
	}
	current += preview.ReviewExercise + 1
	return ImportExerciseReview{
		ProgramIndex: preview.ReviewProgram, ExerciseIndex: preview.ReviewExercise,
		Current: current, Total: total, ProgramName: program.Name,
		ProposedName: proposed, Similar: similar,
	}, nil
}

func (u *UseCase) advanceImportReview(preview *ImportPreview) {
	preview.ReviewExercise++
	for preview.ReviewProgram < len(preview.Programs) && preview.ReviewExercise >= len(preview.Programs[preview.ReviewProgram].Exercises) {
		preview.ReviewProgram++
		preview.ReviewExercise = 0
	}
}

func (u *UseCase) Programs(ctx context.Context, ownerID int64) ([]Program, error) {
	return u.repo.ListPrograms(ctx, ownerID)
}

func (u *UseCase) Exercises(ctx context.Context, ownerID int64, page, pageSize int) (ExercisePage, error) {
	if page < 1 || pageSize < 1 || pageSize > 20 {
		return ExercisePage{}, ErrInvalidPage
	}
	items, total, err := u.repo.ListExercises(ctx, ownerID, pageSize, (page-1)*pageSize)
	if err != nil {
		return ExercisePage{}, err
	}
	totalPages := (total + pageSize - 1) / pageSize
	if totalPages == 0 {
		totalPages = 1
	}
	if page > totalPages {
		return ExercisePage{}, ErrInvalidPage
	}
	return ExercisePage{Items: items, Page: page, PageSize: pageSize, TotalItems: total, TotalPages: totalPages}, nil
}

func (u *UseCase) Exercise(ctx context.Context, ownerID, exerciseID int64) (Exercise, error) {
	return u.repo.Exercise(ctx, ownerID, exerciseID)
}

func (u *UseCase) ExpectExerciseRename(ctx context.Context, ownerID, exerciseID int64) (Exercise, error) {
	exercise, err := u.repo.Exercise(ctx, ownerID, exerciseID)
	if err != nil {
		return Exercise{}, err
	}
	state, err := u.State(ctx, ownerID)
	if err != nil {
		return Exercise{}, err
	}
	state.Mode = InputRename
	state.PendingImport = nil
	state.PendingExerciseID = &exerciseID
	if err := u.repo.SaveUIState(ctx, state); err != nil {
		return Exercise{}, err
	}
	return exercise, nil
}

func (u *UseCase) RenameExercise(ctx context.Context, ownerID int64, raw string) (RenameResult, error) {
	name := strings.TrimSpace(raw)
	if name == "" {
		return RenameResult{}, fmt.Errorf("название пустое")
	}
	if len([]rune(name)) > 200 {
		return RenameResult{}, fmt.Errorf("название длиннее 200 символов")
	}
	state, err := u.State(ctx, ownerID)
	if err != nil {
		return RenameResult{}, err
	}
	if state.Mode != InputRename || state.PendingExerciseID == nil {
		return RenameResult{}, ErrNotEditable
	}
	result, err := u.repo.RenameExercise(ctx, ownerID, *state.PendingExerciseID, name)
	if err != nil {
		return RenameResult{}, err
	}
	if err := u.ClearInput(ctx, ownerID); err != nil {
		return RenameResult{}, err
	}
	return result, nil
}

func (u *UseCase) Start(ctx context.Context, ownerID, programID int64, now time.Time) (Session, error) {
	if _, err := u.repo.ActiveSession(ctx, ownerID); err == nil {
		return Session{}, ErrActiveSession
	} else if !errors.Is(err, ErrNoActiveSession) {
		return Session{}, err
	}
	if err := u.ClearInput(ctx, ownerID); err != nil {
		return Session{}, err
	}
	return u.repo.StartSession(ctx, ownerID, programID, now)
}

func (u *UseCase) Active(ctx context.Context, ownerID int64) (Session, error) {
	return u.repo.ActiveSession(ctx, ownerID)
}

func (u *UseCase) AddSet(ctx context.Context, ownerID int64, raw string) (Session, error) {
	input, err := ParseSet(raw)
	if err != nil {
		return Session{}, err
	}
	session, err := u.repo.AddSet(ctx, ownerID, input)
	if err != nil {
		return Session{}, err
	}
	if err := u.ClearInput(ctx, ownerID); err != nil {
		return Session{}, err
	}
	return session, nil
}

func (u *UseCase) AddNote(ctx context.Context, ownerID int64, raw string) (Session, error) {
	note := strings.TrimSpace(raw)
	if note == "" {
		return Session{}, fmt.Errorf("заметка пустая")
	}
	if len([]rune(note)) > 1000 {
		return Session{}, fmt.Errorf("заметка длиннее 1000 символов")
	}
	session, err := u.repo.SetCurrentExerciseNote(ctx, ownerID, note)
	if err != nil {
		return Session{}, err
	}
	if err := u.ClearInput(ctx, ownerID); err != nil {
		return Session{}, err
	}
	return session, nil
}

func (u *UseCase) FinishExercise(ctx context.Context, ownerID int64, now time.Time) (Session, error) {
	if err := u.ClearInput(ctx, ownerID); err != nil {
		return Session{}, err
	}
	return u.repo.FinishCurrentExercise(ctx, ownerID, now)
}

func (u *UseCase) ReopenExercise(ctx context.Context, ownerID, sessionID, exerciseID int64) (Session, error) {
	if err := u.ClearInput(ctx, ownerID); err != nil {
		return Session{}, err
	}
	return u.repo.ReopenExercise(ctx, ownerID, sessionID, exerciseID)
}

func (u *UseCase) PreviousExercise(
	ctx context.Context,
	ownerID, sessionID int64,
	exerciseName string,
) (*PreviousExercise, error) {
	return u.repo.PreviousExercise(ctx, ownerID, sessionID, exerciseName)
}

func (u *UseCase) Session(ctx context.Context, ownerID, sessionID int64) (Session, error) {
	return u.repo.Session(ctx, ownerID, sessionID)
}

func (u *UseCase) History(ctx context.Context, ownerID int64, page, pageSize int) (SessionPage, error) {
	if page < 1 || pageSize < 1 || pageSize > 20 {
		return SessionPage{}, ErrInvalidPage
	}
	items, total, err := u.repo.RecentSessions(ctx, ownerID, pageSize, (page-1)*pageSize)
	if err != nil {
		return SessionPage{}, err
	}
	totalPages := (total + pageSize - 1) / pageSize
	if totalPages == 0 {
		totalPages = 1
	}
	if page > totalPages {
		return SessionPage{}, ErrInvalidPage
	}
	return SessionPage{Items: items, Page: page, PageSize: pageSize, TotalItems: total, TotalPages: totalPages}, nil
}

func (u *UseCase) MarkPublished(ctx context.Context, ownerID, sessionID, chatID int64, messageID int) error {
	return u.repo.MarkPublished(ctx, ownerID, sessionID, chatID, messageID)
}

func (u *UseCase) DeleteSession(ctx context.Context, ownerID, sessionID int64) error {
	if err := u.ClearInput(ctx, ownerID); err != nil {
		return err
	}
	return u.repo.DeleteSession(ctx, ownerID, sessionID)
}
