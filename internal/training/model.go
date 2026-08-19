// Package training implements manually logged strength workouts. It is kept
// separate from domain.Workout, which represents activities imported from
// Whoop rather than sets entered by the user.
package training

import (
	"context"
	"errors"
	"time"

	"fitlog/internal/training/progression"
)

var (
	ErrNotFound        = errors.New("training data not found")
	ErrNoPrograms      = errors.New("no training programs")
	ErrActiveSession   = errors.New("an active training session already exists")
	ErrNoActiveSession = errors.New("no active training session")
	ErrNoPendingImport = errors.New("no pending program import")
	ErrNotEditable     = errors.New("exercise cannot be edited yet")
	ErrInvalidPage     = errors.New("invalid page")
	ErrNoPendingSet    = errors.New("no pending training set")
)

// InputMode describes which free-form Telegram message the workout card is
// currently waiting for.
type InputMode string

const (
	InputNone                   InputMode = ""
	InputSet                    InputMode = "set"
	InputWarmup                 InputMode = "warmup"
	InputNote                   InputMode = "note"
	InputImportFile             InputMode = "import_file"
	InputImportOK               InputMode = "import_preview"
	InputRename                 InputMode = "exercise_rename"
	InputProgramExerciseChoice  InputMode = "program_exercise_choice"
	InputProgramExerciseNew     InputMode = "program_exercise_new"
	InputProgramExerciseConfirm InputMode = "program_exercise_confirm"
	InputRIR                    InputMode = "rir"
	InputOverride               InputMode = "override"
)

type RepRange struct {
	Min int `json:"min"`
	Max int `json:"max"`
}

type SetType string

const (
	SetTypeWarmup  SetType = "warmup"
	SetTypeWorking SetType = "working"
)

type ProgressionAction = progression.Action
type ReasonCode = progression.ReasonCode

const (
	ProgressionKeep     = progression.ActionKeep
	ProgressionIncrease = progression.ActionIncrease
	ProgressionDecrease = progression.ActionDecrease
)

const ProgressionDouble = "double"

type WarmupSet struct {
	WeightKG *float64 `json:"weight_kg,omitempty"`
	Bar      bool     `json:"bar,omitempty"`
	Reps     int      `json:"reps"`
}

type ExercisePrescription struct {
	WorkingSets    int         `json:"working_sets"`
	Reps           RepRange    `json:"reps"`
	TargetRIR      float64     `json:"target_rir"`
	WeightStepKG   float64     `json:"weight_step_kg"`
	StartingWeight *float64    `json:"starting_weight_kg,omitempty"`
	RestSeconds    int         `json:"rest_seconds"`
	AfterSeconds   int         `json:"after_seconds"`
	Progression    string      `json:"progression"`
	Warmup         []WarmupSet `json:"warmup,omitempty"`
}

func (p ExercisePrescription) Structured() bool {
	return p.WorkingSets > 0 && p.Reps.Min > 0 && p.Reps.Max >= p.Reps.Min
}

type Recommendation = progression.Recommendation

type ExerciseOverride struct {
	WeightKG    *float64
	Reps        RepRange
	WorkingSets int
	TargetRIR   float64
	RestSeconds int
}

type PendingSet struct {
	Type     SetType  `json:"type"`
	WeightKG *float64 `json:"weight_kg,omitempty"`
	Reps     int      `json:"reps"`
}

// ProgramInput is one normalized workout template produced by YAML, TXT, or
// CSV import before database IDs are assigned.
type ProgramInput struct {
	Name            string                 `json:"name"`
	Exercises       []string               `json:"exercises"`
	Prescriptions   []ExercisePrescription `json:"prescriptions,omitempty"`
	PlanName        string                 `json:"plan_name,omitempty"`
	PlanDescription string                 `json:"plan_description,omitempty"`
	DaysPerWeek     int                    `json:"days_per_week,omitempty"`
	Version         int                    `json:"version,omitempty"`
	WorkoutKey      string                 `json:"workout_key,omitempty"`
	RawSource       string                 `json:"raw_source,omitempty"`
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
	PlanID        int64
	RevisionID    int64
	Revision      int
	Name          string
	PlanName      string
	WorkoutKey    string
	Exercises     []string
	ExerciseItems []ProgramExercise
}

type ProgramExercise struct {
	ID           int64
	ExerciseID   *int64
	Position     int
	Name         string
	Prescription ExercisePrescription
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
	Reps        int
	WeightKG    *float64
	Type        SetType
	RIR         *float64
	CompletedAt time.Time
}

type WorkoutSet struct {
	ID                int64
	SessionExerciseID int64
	Position          int
	Type              SetType
	PlannedWeightKG   *float64
	PlannedMinReps    *int
	PlannedMaxReps    *int
	PlannedRIR        *float64
	ActualWeightKG    *float64
	ActualReps        *int
	ActualRIR         *float64
	StartedAt         *time.Time
	CompletedAt       *time.Time
	RestUntil         *time.Time

	// Compatibility fields for legacy TXT/CSV sessions and public formatting.
	Reps     int
	WeightKG *float64
	RIR      *float64
}

type PreviousExercise struct {
	StartedAt   time.Time
	ProgramName string
	Sets        []WorkoutSet
}

type SessionExercise struct {
	ID             int64
	ExerciseID     *int64
	Position       int
	Name           string
	Note           string
	Complete       bool
	Sets           []WorkoutSet
	Warmup         []WarmupSet
	Recommendation Recommendation
	Plan           Recommendation
	Overridden     bool
}

func (e SessionExercise) Structured() bool { return e.Plan.WorkingSets > 0 }

func (e SessionExercise) WarmupSets() []WorkoutSet {
	sets := make([]WorkoutSet, 0, len(e.Sets))
	for _, set := range e.Sets {
		if set.Type == SetTypeWarmup {
			sets = append(sets, set)
		}
	}
	return sets
}

func (e SessionExercise) WorkingSets() []WorkoutSet {
	sets := make([]WorkoutSet, 0, len(e.Sets))
	for _, set := range e.Sets {
		if set.Type == "" || set.Type == SetTypeWorking {
			sets = append(sets, set)
		}
	}
	return sets
}

// NextWorkingWeightKG returns the weight to prefill for the next working set.
// Once the athlete has completed a working set in the current workout, its
// actual weight takes precedence over the recommendation snapshotted from
// previous workouts.
func (e SessionExercise) NextWorkingWeightKG() *float64 {
	working := e.WorkingSets()
	if len(working) == 0 {
		return e.Plan.WeightKG
	}
	latest := working[len(working)-1]
	if latest.ActualWeightKG != nil {
		return latest.ActualWeightKG
	}
	return latest.WeightKG
}

func (e SessionExercise) PlanComplete() bool {
	return e.Structured() && len(e.WarmupSets()) >= len(e.Warmup) && len(e.WorkingSets()) >= e.Plan.WorkingSets
}

type Session struct {
	ID      int64
	OwnerID int64
	// ProgramID is the workout template ID retained under its legacy Go name.
	ProgramID          *int64
	RevisionID         *int64
	ProgramName        string
	Status             string
	CurrentPosition    int
	StartedAt          time.Time
	FinishedAt         *time.Time
	PublishedChatID    *int64
	PublishedMessageID *int
	RestUntil          *time.Time
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

func (s Session) SetCounts() (working, warmup int) {
	for _, exercise := range s.Exercises {
		for _, set := range exercise.Sets {
			if set.Type == SetTypeWarmup {
				warmup++
			} else {
				working++
			}
		}
	}
	return working, warmup
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
	PendingSet               *PendingSet
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
	OverrideCurrentExercise(ctx context.Context, ownerID int64, override ExerciseOverride) (Session, error)
	SetCurrentExerciseNote(ctx context.Context, ownerID int64, note string) (Session, error)
	FinishCurrentExercise(ctx context.Context, ownerID int64, now time.Time) (Session, error)
	ReopenExercise(ctx context.Context, ownerID, sessionID, exerciseID int64) (Session, error)
	PreviousExercise(ctx context.Context, ownerID, sessionID int64, exerciseName string) (*PreviousExercise, error)
	RecentSessions(ctx context.Context, ownerID int64, limit, offset int) ([]Session, int, error)
	MarkPublished(ctx context.Context, ownerID, sessionID, chatID int64, messageID int) error
	DeleteSession(ctx context.Context, ownerID, sessionID int64) error
}
