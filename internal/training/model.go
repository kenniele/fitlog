// Package training implements manually logged strength workouts. It is kept
// separate from domain.Workout, which represents activities imported from
// Whoop rather than sets entered by the user.
package training

import (
	"context"
	"errors"
	"time"
)

var (
	ErrNotFound        = errors.New("training data not found")
	ErrNoPrograms      = errors.New("no training programs")
	ErrActiveSession   = errors.New("an active training session already exists")
	ErrNoActiveSession = errors.New("no active training session")
	ErrNoPendingImport = errors.New("no pending program import")
	ErrPublished       = errors.New("published training cannot be edited")
	ErrNotEditable     = errors.New("exercise cannot be edited yet")
)

// InputMode describes which free-form Telegram message the workout card is
// currently waiting for.
type InputMode string

const (
	InputNone       InputMode = ""
	InputSet        InputMode = "set"
	InputNote       InputMode = "note"
	InputImportFile InputMode = "import_file"
	InputImportOK   InputMode = "import_preview"
)

// ProgramInput is the normalized representation produced by a TXT or CSV
// import before database IDs are assigned.
type ProgramInput struct {
	Name      string   `json:"name"`
	Exercises []string `json:"exercises"`
}

type ImportPreview struct {
	Filename string         `json:"filename"`
	Programs []ProgramInput `json:"programs"`
}

type Program struct {
	ID        int64
	OwnerID   int64
	Name      string
	Exercises []string
}

// SetInput is the structured meaning of either "12Р 40КГ" or "12Р -".
// A nil WeightKG means bodyweight.
type SetInput struct {
	Reps     int
	WeightKG *float64
}

type WorkoutSet struct {
	ID       int64
	Position int
	Reps     int
	WeightKG *float64
}

type PreviousExercise struct {
	StartedAt   time.Time
	ProgramName string
	Sets        []WorkoutSet
}

type SessionExercise struct {
	ID       int64
	Position int
	Name     string
	Note     string
	Complete bool
	Sets     []WorkoutSet
}

type Session struct {
	ID                 int64
	OwnerID            int64
	ProgramID          *int64
	ProgramName        string
	Status             string
	CurrentPosition    int
	StartedAt          time.Time
	FinishedAt         *time.Time
	PublishedChatID    *int64
	PublishedMessageID *int
	Exercises          []SessionExercise
}

func (s Session) Active() bool { return s.Status == "active" }

func (s Session) CurrentExercise() *SessionExercise {
	for i := range s.Exercises {
		if s.Exercises[i].Position == s.CurrentPosition {
			return &s.Exercises[i]
		}
	}
	return nil
}

type UIState struct {
	OwnerID       int64
	ChatID        int64
	MessageID     int
	Mode          InputMode
	PendingImport *ImportPreview
}

// Repository persists all durable workout and card state. The PostgreSQL
// implementation lives in internal/storage.
type Repository interface {
	GetUIState(ctx context.Context, ownerID int64) (UIState, error)
	SaveUIState(ctx context.Context, state UIState) error
	ListPrograms(ctx context.Context, ownerID int64) ([]Program, error)
	ReplacePrograms(ctx context.Context, ownerID int64, programs []ProgramInput) error
	StartSession(ctx context.Context, ownerID, programID int64, now time.Time) (Session, error)
	ActiveSession(ctx context.Context, ownerID int64) (Session, error)
	Session(ctx context.Context, ownerID, sessionID int64) (Session, error)
	AddSet(ctx context.Context, ownerID int64, input SetInput) (Session, error)
	SetCurrentExerciseNote(ctx context.Context, ownerID int64, note string) (Session, error)
	FinishCurrentExercise(ctx context.Context, ownerID int64, now time.Time) (Session, error)
	ReopenExercise(ctx context.Context, ownerID, sessionID, exerciseID int64) (Session, error)
	PreviousExercise(ctx context.Context, ownerID, sessionID int64, exerciseName string) (*PreviousExercise, error)
	RecentSessions(ctx context.Context, ownerID int64, limit int) ([]Session, error)
	MarkPublished(ctx context.Context, ownerID, sessionID, chatID int64, messageID int) error
}
