package training

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestSessionExerciseCanReplace(t *testing.T) {
	reps, weight, now := 10, 60.0, time.Now()
	for _, tc := range []struct {
		name     string
		exercise SessionExercise
		want     bool
	}{
		{name: "unstarted", want: true},
		{name: "untouched plan", exercise: SessionExercise{Sets: []WorkoutSet{{PlannedWeightKG: &weight, PlannedMinReps: &reps}}}, want: true},
		{name: "finished", exercise: SessionExercise{Complete: true}},
		{name: "warmup", exercise: SessionExercise{Sets: []WorkoutSet{{Type: SetTypeWarmup, ActualReps: &reps}}}},
		{name: "working", exercise: SessionExercise{Sets: []WorkoutSet{{Type: SetTypeWorking, ActualReps: &reps}}}},
		{name: "drop", exercise: SessionExercise{Sets: []WorkoutSet{{Type: SetTypeDrop, ActualReps: &reps}}}},
		{name: "partial weight", exercise: SessionExercise{Sets: []WorkoutSet{{ActualWeightKG: &weight}}}},
		{name: "started", exercise: SessionExercise{Sets: []WorkoutSet{{StartedAt: &now}}}},
		{name: "legacy result", exercise: SessionExercise{Sets: []WorkoutSet{{Reps: reps}}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, tc.exercise.CanReplace())
		})
	}
}
