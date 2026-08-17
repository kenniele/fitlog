package progression

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func config() ExerciseConfig {
	return ExerciseConfig{
		WorkingSets: 3, MinReps: 8, MaxReps: 12, TargetRIR: 2,
		WeightStepKG: 2.5,
		RestSeconds:  180, AfterSeconds: 180, Progression: "double",
	}
}

func set(weight float64, reps int, rir *float64) Set {
	return Set{Type: SetTypeWorking, WeightKG: &weight, Reps: reps, RIR: rir}
}

func recommend(t *testing.T, cfg ExerciseConfig, history ...PreviousSession) Recommendation {
	t.Helper()
	got, err := New().Recommend(context.Background(), Input{Exercise: cfg, History: history})
	require.NoError(t, err)
	return got
}

func TestIncreaseWhenAllSetsReachMax(t *testing.T) {
	rir := 2.0
	got := recommend(t, config(), PreviousSession{Sets: []Set{set(60, 12, &rir), set(60, 12, &rir), set(60, 12, &rir)}})
	require.Equal(t, ActionIncrease, got.Action)
	require.Equal(t, ReasonReachedTopRange, got.ReasonCode)
	require.InDelta(t, 62.5, *got.WeightKG, 0.001)
}

func TestKeepWhenOneSetBelowMax(t *testing.T) {
	got := recommend(t, config(), PreviousSession{Sets: []Set{set(60, 12, nil), set(60, 11, nil), set(60, 9, nil)}})
	require.Equal(t, ActionKeep, got.Action)
	require.Equal(t, ReasonBelowTopRange, got.ReasonCode)
}

func TestKeepWhenRIRBelowTarget(t *testing.T) {
	rir2, rir1 := 2.0, 1.0
	got := recommend(t, config(), PreviousSession{Sets: []Set{set(60, 12, &rir2), set(60, 12, &rir1), set(60, 12, nil)}})
	require.Equal(t, ActionKeep, got.Action)
	require.Equal(t, ReasonRIRTooLow, got.ReasonCode)
}

func TestIncreaseWhenRIRMissing(t *testing.T) {
	got := recommend(t, config(), PreviousSession{Sets: []Set{set(60, 12, nil), set(60, 12, nil), set(60, 12, nil)}})
	require.Equal(t, ActionIncrease, got.Action)
}

func TestUseStartingWeightWithoutHistory(t *testing.T) {
	start := 60.0
	cfg := config()
	cfg.StartingWeight = &start
	got := recommend(t, cfg)
	require.Equal(t, ReasonNoHistory, got.ReasonCode)
	require.InDelta(t, 60, *got.WeightKG, 0.001)
}

func TestHandlePartialPreviousWorkout(t *testing.T) {
	got := recommend(t, config(), PreviousSession{Sets: []Set{set(60, 12, nil), set(60, 12, nil)}})
	require.Equal(t, ReasonPartialWorkout, got.ReasonCode)
	require.Equal(t, ActionKeep, got.Action)
}

func TestIgnoreWarmupSets(t *testing.T) {
	rir, warmupWeight := 2.0, 20.0
	got := recommend(t, config(), PreviousSession{Sets: []Set{
		{Type: SetTypeWarmup, WeightKG: &warmupWeight, Reps: 10},
		set(60, 12, &rir), set(60, 12, &rir), set(60, 12, &rir),
	}})
	require.Equal(t, ActionIncrease, got.Action)
}

func TestRespectWeightStep(t *testing.T) {
	cfg := config()
	cfg.WeightStepKG = 4.5
	got := recommend(t, cfg, PreviousSession{Sets: []Set{set(68, 12, nil), set(68, 12, nil), set(68, 12, nil)}})
	require.InDelta(t, 72.5, *got.WeightKG, 0.001)
}

func TestRespectFixedRepRange(t *testing.T) {
	cfg := config()
	cfg.MinReps, cfg.MaxReps = 12, 12
	got := recommend(t, cfg, PreviousSession{Sets: []Set{set(60, 12, nil), set(60, 12, nil), set(60, 12, nil)}})
	require.Equal(t, ActionIncrease, got.Action)
}
