package mcpserver

import (
	"context"
	"math"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"fitlog/internal/controlcenter"
)

func (a *adapter) addWriteTools(s *mcp.Server) {
	addTool(a, s, "log_nutrition", "Add one manual daily nutrition total after the user agrees to its values. Not a meal entry and not a FatSecret update. First inspect list_nutrition for that day; do not add a second total for the same day. A repeated request_id produces a conflict without adding a duplicate.", true,
		func(ctx context.Context, service *controlcenter.Service, loc *time.Location, in nutritionInput) (any, error) {
			found := false
			for _, v := range []*float64{in.CaloriesKcal, in.ProteinG, in.FatG, in.CarbohydratesG, in.WaterML} {
				if v != nil {
					found = true
					if !finiteNonNegative(*v) {
						return nil, invalid("nutrients", "values must be finite and nonnegative")
					}
				}
			}
			if !found {
				return nil, invalid("nutrients", "provide at least one measured total")
			}
			return create(ctx, service, "nutrition", in, in.RequestID)
		})
	addTool(a, s, "log_body_measurement", "Add a body measurement after the user agrees. Specify measured_at in RFC3339 or YYYY-MM-DD in the owner's timezone. Missing measurements must be omitted, not guessed. Reuse request_id on retries.", true,
		func(ctx context.Context, service *controlcenter.Service, _ *time.Location, in bodyInput) (any, error) {
			return create(ctx, service, "body-measurements", in, in.RequestID)
		})
	addTool(a, s, "log_workout", "Add an already completed workout to history after the user agrees. Does not start/advance an active Telegram workout or publish to channels. Each supplied set is completed. weight_kg=null means bodyweight. Use existing exercise IDs where possible and reuse request_id on retries.", true,
		func(ctx context.Context, service *controlcenter.Service, _ *time.Location, in workoutInput) (any, error) {
			if len(in.Exercises) == 0 || len(in.Exercises) > 100 {
				return nil, invalid("exercises", "provide 1 to 100 exercises")
			}
			for i := range in.Exercises {
				e := &in.Exercises[i]
				if len(e.Sets) == 0 || len(e.Sets) > 100 {
					return nil, invalid("sets", "provide 1 to 100 sets per exercise")
				}
				if e.ExerciseID != nil && *e.ExerciseID <= 0 {
					return nil, invalid("exercise_id", "must be positive")
				}
				if e.ExerciseID == nil && strings.TrimSpace(e.Name) == "" {
					return nil, invalid("name", "supply an existing exercise_id or a name")
				}
				for _, set := range e.Sets {
					if set.RestSeconds != nil && *set.RestSeconds < 0 {
						return nil, invalid("rest_seconds", "must be nonnegative")
					}
					if set.Reps <= 0 || (set.WeightKG != nil && (!finiteNonNegative(*set.WeightKG) || *set.WeightKG == 0)) || (set.RIR != nil && (!finiteNonNegative(*set.RIR) || *set.RIR > 10)) {
						return nil, invalid("sets", "positive reps and weight (or null for bodyweight), and RIR between 0 and 10 are required")
					}
					switch set.Type {
					case "working", "warmup", "drop":
					default:
						return nil, invalid("type", "use working, warmup, or drop")
					}
				}
			}
			exercises := make([]completedExercise, len(in.Exercises))
			for i, e := range in.Exercises {
				sets := make([]completedSet, len(e.Sets))
				for j, set := range e.Sets {
					sets[j] = completedSet{set, true}
				}
				exercises[i] = completedExercise{e, true, sets}
			}
			// The transport does not expose the interactive Telegram state machine.
			return create(ctx, service, "workout-sessions", struct {
				workoutInput
				Status    string              `json:"status"`
				Exercises []completedExercise `json:"exercises"`
			}{in, "finished", exercises}, in.RequestID)
		})
	addTool(a, s, "create_workout_plan", "Create a new plan with workout templates and exercise prescriptions after the user reviews and agrees to the full plan. Existing plans are preserved. Find exercise IDs with search_exercises first; names create exercises when needed. Reuse request_id on retries.", true,
		func(ctx context.Context, service *controlcenter.Service, _ *time.Location, in planInput) (any, error) {
			if len(in.Templates) == 0 || len(in.Templates) > 14 {
				return nil, invalid("templates", "provide 1 to 14 templates")
			}
			for _, template := range in.Templates {
				if len(template.Exercises) == 0 || len(template.Exercises) > 100 {
					return nil, invalid("exercises", "provide 1 to 100 exercises per template")
				}
			}
			return create(ctx, service, "workout-plans", in, in.RequestID)
		})
}

func finiteNonNegative(v float64) bool { return !math.IsNaN(v) && !math.IsInf(v, 0) && v >= 0 }

type writeInput struct {
	RequestID string `json:"request_id" jsonschema:"Unique request ID (8-128 characters). Reuse the same ID when retrying this exact operation to prevent duplicates."`
}
type nutritionInput struct {
	writeInput
	Date           string   `json:"date" jsonschema:"Local journal date YYYY-MM-DD."`
	CaloriesKcal   *float64 `json:"calories_kcal,omitempty" jsonschema:"Daily energy total in kcal."`
	ProteinG       *float64 `json:"protein_g,omitempty" jsonschema:"Daily protein total in grams."`
	FatG           *float64 `json:"fat_g,omitempty" jsonschema:"Daily fat total in grams."`
	CarbohydratesG *float64 `json:"carbohydrates_g,omitempty" jsonschema:"Daily carbohydrate total in grams."`
	WaterML        *float64 `json:"water_ml,omitempty" jsonschema:"Daily water total in milliliters."`
	Notes          string   `json:"notes,omitempty"`
}
type bodyInput struct {
	writeInput
	MeasuredAt           string   `json:"measured_at" jsonschema:"RFC3339 timestamp with offset or local YYYY-MM-DD."`
	WeightKG             *float64 `json:"weight_kg,omitempty"`
	BodyFatPercent       *float64 `json:"body_fat_percent,omitempty"`
	FatMassKG            *float64 `json:"fat_mass_kg,omitempty"`
	LeanMassKG           *float64 `json:"lean_mass_kg,omitempty"`
	SkeletalMuscleMassKG *float64 `json:"skeletal_muscle_mass_kg,omitempty"`
	WaistCM              *float64 `json:"waist_cm,omitempty"`
	Notes                string   `json:"notes,omitempty"`
}
type workoutInput struct {
	writeInput
	StartedAt   string            `json:"started_at" jsonschema:"Actual start time, RFC3339 with timezone offset."`
	FinishedAt  *string           `json:"finished_at,omitempty" jsonschema:"Actual finish time, RFC3339 with timezone offset; omit if unknown."`
	ProgramName string            `json:"program_name" jsonschema:"Workout name."`
	Notes       string            `json:"notes,omitempty"`
	Exercises   []workoutExercise `json:"exercises" jsonschema:"Exercises in performed order."`
}
type workoutExercise struct {
	ExerciseID *int64       `json:"exercise_id,omitempty"`
	Name       string       `json:"name,omitempty" jsonschema:"Exercise name if no existing exercise_id is known."`
	Notes      string       `json:"notes,omitempty"`
	Sets       []workoutSet `json:"sets" jsonschema:"Completed sets in performed order."`
}
type workoutSet struct {
	Type        string   `json:"type" jsonschema:"working, warmup, or drop."`
	WeightKG    *float64 `json:"weight_kg" jsonschema:"Kilograms; null for bodyweight, use a positive number for external load."`
	Reps        int      `json:"reps"`
	RIR         *float64 `json:"rir,omitempty" jsonschema:"Repetitions in reserve, 0 to 10; omit if unknown."`
	RestSeconds *int     `json:"rest_seconds,omitempty"`
	Notes       string   `json:"notes,omitempty"`
}
type planInput struct {
	writeInput
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	DaysPerWeek *int           `json:"days_per_week,omitempty" jsonschema:"Planned training days per week, 1 to 7."`
	Templates   []planTemplate `json:"templates"`
}
type planTemplate struct {
	Name      string         `json:"name"`
	Exercises []planExercise `json:"exercises" jsonschema:"Exercises in execution order."`
}
type planExercise struct {
	ExerciseID       *int64   `json:"exercise_id,omitempty"`
	Name             string   `json:"name,omitempty"`
	WorkingSets      int      `json:"working_sets"`
	MinReps          int      `json:"min_reps"`
	MaxReps          int      `json:"max_reps"`
	TargetRIR        *float64 `json:"target_rir,omitempty"`
	StartingWeightKG *float64 `json:"starting_weight_kg,omitempty"`
	WeightStepKG     float64  `json:"weight_step_kg" jsonschema:"Positive weight increment for double progression."`
	ProgressionType  *string  `json:"progression_type,omitempty" jsonschema:"double; omit to use double progression."`
	RestSeconds      *int     `json:"rest_seconds,omitempty"`
	Notes            string   `json:"notes,omitempty"`
}

// Completed flags are transport-generated and never inferred from user input.
type completedSet struct {
	workoutSet
	Completed bool `json:"completed"`
}
type completedExercise struct {
	workoutExercise
	Completed bool           `json:"completed"`
	Sets      []completedSet `json:"sets"`
}
