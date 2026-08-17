package progression

import (
	"context"
	"fmt"
	"strconv"
)

type Action string

const (
	ActionKeep     Action = "keep"
	ActionIncrease Action = "increase"
	ActionDecrease Action = "decrease"
)

type ReasonCode string

const (
	ReasonReachedTopRange ReasonCode = "reached_top_range"
	ReasonBelowTopRange   ReasonCode = "below_top_range"
	ReasonRIRTooLow       ReasonCode = "rir_too_low"
	ReasonNoHistory       ReasonCode = "no_history"
	ReasonPartialWorkout  ReasonCode = "partial_previous_workout"
	ReasonMixedWeight     ReasonCode = "mixed_working_weight"
	ReasonBodyweight      ReasonCode = "bodyweight"
)

type SetType string

const (
	SetTypeWarmup  SetType = "warmup"
	SetTypeWorking SetType = "working"
)

type ExerciseConfig struct {
	WorkingSets    int
	MinReps        int
	MaxReps        int
	TargetRIR      float64
	WeightStepKG   float64
	StartingWeight *float64
	RestSeconds    int
	AfterSeconds   int
	Progression    string
}

type Set struct {
	Type     SetType
	WeightKG *float64
	Reps     int
	RIR      *float64
}

type PreviousSession struct {
	Sets []Set
}

type Input struct {
	Exercise ExerciseConfig
	History  []PreviousSession
}

type Recommendation struct {
	WeightKG     *float64   `json:"weight_kg,omitempty"`
	MinReps      int        `json:"min_reps"`
	MaxReps      int        `json:"max_reps"`
	WorkingSets  int        `json:"working_sets"`
	TargetRIR    float64    `json:"target_rir"`
	RestSeconds  int        `json:"rest_seconds"`
	AfterSeconds int        `json:"after_seconds"`
	Action       Action     `json:"action"`
	ReasonCode   ReasonCode `json:"reason_code"`
	Reason       string     `json:"reason"`
}

type Engine interface {
	Recommend(context.Context, Input) (Recommendation, error)
}

type DoubleProgression struct{}

func New() Engine { return DoubleProgression{} }

func (DoubleProgression) Recommend(_ context.Context, input Input) (Recommendation, error) {
	config := input.Exercise
	result := Recommendation{
		MinReps: config.MinReps, MaxReps: config.MaxReps, WorkingSets: config.WorkingSets,
		TargetRIR: config.TargetRIR, RestSeconds: config.RestSeconds, AfterSeconds: config.AfterSeconds,
		Action: ActionKeep,
	}
	if err := validateConfig(config); err != nil {
		return Recommendation{}, err
	}
	if len(input.History) == 0 {
		result.WeightKG = cloneFloat(config.StartingWeight)
		result.ReasonCode = ReasonNoHistory
		if result.WeightKG == nil {
			result.Reason = "Истории нет. Укажи рабочий вес перед первым подходом."
		} else {
			result.Reason = fmt.Sprintf("Истории нет. Использован стартовый вес %s кг.", number(*result.WeightKG))
		}
		return result, nil
	}

	previous := input.History[0]
	working := make([]Set, 0, len(previous.Sets))
	for _, set := range previous.Sets {
		if set.Type == "" || set.Type == SetTypeWorking {
			working = append(working, set)
		}
	}
	if len(working) == 0 {
		result.WeightKG = cloneFloat(config.StartingWeight)
		result.ReasonCode = ReasonPartialWorkout
		result.Reason = "В прошлой тренировке нет рабочих подходов. Вес оставлен без автоматической прогрессии."
		return result, nil
	}

	base := working[len(working)-1].WeightKG
	result.WeightKG = cloneFloat(base)
	if base == nil {
		result.ReasonCode = ReasonBodyweight
		result.Reason = "Последний рабочий подход выполнен с собственным весом. Автопрогрессия нагрузки не применена."
		return result, nil
	}
	if len(working) != config.WorkingSets {
		result.ReasonCode = ReasonPartialWorkout
		result.Reason = fmt.Sprintf("В прошлый раз выполнено %d из %d рабочих подходов. Вес сохранён.", len(working), config.WorkingSets)
		return result, nil
	}

	minKnownRIR := 0.0
	hasKnownRIR := false
	for _, set := range working {
		if set.WeightKG == nil || *set.WeightKG != *base {
			result.ReasonCode = ReasonMixedWeight
			result.Reason = "Рабочие подходы выполнены с разным весом. Использован вес последнего подхода без повышения."
			return result, nil
		}
		if set.Reps < config.MaxReps {
			result.ReasonCode = ReasonBelowTopRange
			result.Reason = fmt.Sprintf("Не все %d рабочих подхода достигли %d повторений. Вес сохранён.", config.WorkingSets, config.MaxReps)
			return result, nil
		}
		if set.RIR != nil {
			if !hasKnownRIR || *set.RIR < minKnownRIR {
				minKnownRIR = *set.RIR
			}
			hasKnownRIR = true
		}
	}
	if hasKnownRIR && minKnownRIR < config.TargetRIR {
		result.ReasonCode = ReasonRIRTooLow
		result.Reason = fmt.Sprintf("Верхняя граница повторений достигнута, но минимальный RIR %s ниже цели %s. Вес сохранён.", number(minKnownRIR), number(config.TargetRIR))
		return result, nil
	}

	next := *base + config.WeightStepKG
	result.WeightKG = &next
	result.Action = ActionIncrease
	result.ReasonCode = ReasonReachedTopRange
	if hasKnownRIR {
		result.Reason = fmt.Sprintf(
			"Все %d рабочих подхода достигли верхней границы %d повторений. Минимальный известный RIR %s не ниже цели %s. Вес увеличен с %s до %s кг.",
			config.WorkingSets, config.MaxReps, number(minKnownRIR), number(config.TargetRIR), number(*base), number(next),
		)
	} else {
		result.Reason = fmt.Sprintf(
			"Все %d рабочих подхода достигли верхней границы %d повторений, известных RIR ниже цели нет. Вес увеличен с %s до %s кг.",
			config.WorkingSets, config.MaxReps, number(*base), number(next),
		)
	}
	return result, nil
}

func validateConfig(config ExerciseConfig) error {
	if config.Progression != "double" {
		return fmt.Errorf("unsupported progression %q", config.Progression)
	}
	if config.WorkingSets <= 0 || config.MinReps <= 0 || config.MaxReps < config.MinReps {
		return fmt.Errorf("invalid exercise prescription")
	}
	if config.TargetRIR < 0 || config.WeightStepKG <= 0 || config.RestSeconds < 0 || config.AfterSeconds < 0 {
		return fmt.Errorf("invalid exercise progression values")
	}
	return nil
}

func cloneFloat(value *float64) *float64 {
	if value == nil {
		return nil
	}
	copy := *value
	return &copy
}

func number(value float64) string {
	return strconv.FormatFloat(value, 'f', -1, 64)
}
