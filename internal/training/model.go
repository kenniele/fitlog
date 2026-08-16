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
	ErrNotEditable     = errors.New("exercise cannot be edited yet")
	ErrInvalidPage     = errors.New("invalid page")
)

// InputMode describes which free-form Telegram message the workout card is
// currently waiting for.
type InputMode string

const (
	InputNone                   InputMode = ""
	InputSet                    InputMode = "set"
	InputNote                   InputMode = "note"
	InputImportFile             InputMode = "import_file"
	InputImportOK               InputMode = "import_preview"
	InputRename                 InputMode = "exercise_rename"
	InputProgramExerciseChoice  InputMode = "program_exercise_choice"
	InputProgramExerciseNew     InputMode = "program_exercise_new"
	InputProgramExerciseConfirm InputMode = "program_exercise_confirm"
)

// ProgramInput is the normalized representation produced by a TXT or CSV
// import before database IDs are assigned.
type ProgramInput struct {
	Name      string   `json:"name"`
	Exercises []string `json:"exercises"`
}

type ImportPreview struct {
	Filename       string         `json:"filename"`
	Programs       []ProgramInput `json:"programs"`
	ReviewStarted  bool           `json:"review_started,omitempty"`
	ReviewProgram  int            `json:"review_program,omitempty"`
	ReviewExercise int            `json:"review_exercise,omitempty"`
}

type ImportExerciseReview struct {
	ProgramIndex  int
	ExerciseIndex int
	Current       int
	Total         int
	ProgramName   string
	ProposedName  string
	Similar       []Exercise
}

type Program struct {
	ID            int64
	OwnerID       int64
	Name          string
	Exercises     []string
	ExerciseItems []ProgramExercise
}

type ProgramExercise struct {
	ID         int64
	ExerciseID *int64
	Position   int
	Name       string
}

type Exercise struct {
	ID       int64
	OwnerID  int64
	Name     string
	Programs []string
}

type ExercisePage struct {
	Items      []Exercise
	Page       int
	PageSize   int
	TotalItems int
	TotalPages int
}

type SessionPage struct {
	Items      []Session
	Page       int
	PageSize   int
	TotalItems int
	TotalPages int
}

type RenameResult struct {
	Exercise          Exercise
	Merged            bool
	PublishedSessions []Session
}

type ProgramExerciseReplacement struct {
	Program Program
	Current ProgramExercise
	Target  Exercise
}

type ProgramExerciseReplaceResult struct {
	Program           Program
	PublishedSessions []Session
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
	ID         int64
	ExerciseID *int64
	Position   int
	Name       string
	Note       string
	Complete   bool
	Sets       []WorkoutSet
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
	OwnerID                  int64
	ChatID                   int64
	MessageID                int
	Mode                     InputMode
	PendingImport            *ImportPreview
	PendingExerciseID        *int64
	PendingExerciseName      string
	PendingProgramExerciseID *int64
	PendingTargetExerciseID  *int64
}

// Repository persists all durable workout and card state. The PostgreSQL
// implementation lives in internal/storage.
type Repository interface {
	GetUIState(ctx context.Context, ownerID int64) (UIState, error)
	SaveUIState(ctx context.Context, state UIState) error
	ListPrograms(ctx context.Context, ownerID int64) ([]Program, error)
	Program(ctx context.Context, ownerID, programID int64) (Program, error)
	ReplacePrograms(ctx context.Context, ownerID int64, programs []ProgramInput) error
	ListExercises(ctx context.Context, ownerID int64, limit, offset int) ([]Exercise, int, error)
	SimilarExercises(ctx context.Context, ownerID int64, name string, limit int) ([]Exercise, error)
	Exercise(ctx context.Context, ownerID, exerciseID int64) (Exercise, error)
	RenameExercise(ctx context.Context, ownerID, exerciseID int64, name string) (RenameResult, error)
	ProgramExercise(ctx context.Context, ownerID, programExerciseID int64) (ProgramExerciseReplacement, error)
	ReplaceProgramExercise(ctx context.Context, ownerID, programExerciseID int64, targetExerciseID *int64, targetName string, replaceHistory bool) (ProgramExerciseReplaceResult, error)
	StartSession(ctx context.Context, ownerID, programID int64, now time.Time) (Session, error)
	ActiveSession(ctx context.Context, ownerID int64) (Session, error)
	Session(ctx context.Context, ownerID, sessionID int64) (Session, error)
	AddSet(ctx context.Context, ownerID int64, input SetInput) (Session, error)
	SetCurrentExerciseNote(ctx context.Context, ownerID int64, note string) (Session, error)
	FinishCurrentExercise(ctx context.Context, ownerID int64, now time.Time) (Session, error)
	ReopenExercise(ctx context.Context, ownerID, sessionID, exerciseID int64) (Session, error)
	PreviousExercise(ctx context.Context, ownerID, sessionID int64, exerciseName string) (*PreviousExercise, error)
	RecentSessions(ctx context.Context, ownerID int64, limit, offset int) ([]Session, int, error)
	MarkPublished(ctx context.Context, ownerID, sessionID, chatID int64, messageID int) error
	DeleteSession(ctx context.Context, ownerID, sessionID int64) error
}
