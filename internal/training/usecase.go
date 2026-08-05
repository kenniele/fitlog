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

func (u *UseCase) Expect(ctx context.Context, ownerID int64, mode InputMode) error {
	state, err := u.State(ctx, ownerID)
	if err != nil {
		return err
	}
	state.Mode = mode
	if mode != InputImportOK {
		state.PendingImport = nil
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

func (u *UseCase) Programs(ctx context.Context, ownerID int64) ([]Program, error) {
	return u.repo.ListPrograms(ctx, ownerID)
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

func (u *UseCase) Session(ctx context.Context, ownerID, sessionID int64) (Session, error) {
	return u.repo.Session(ctx, ownerID, sessionID)
}

func (u *UseCase) Recent(ctx context.Context, ownerID int64, limit int) ([]Session, error) {
	return u.repo.RecentSessions(ctx, ownerID, limit)
}

func (u *UseCase) MarkPublished(ctx context.Context, ownerID, sessionID, chatID int64, messageID int) error {
	return u.repo.MarkPublished(ctx, ownerID, sessionID, chatID, messageID)
}
