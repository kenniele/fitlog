//go:build integration

package storage

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"fitlog/internal/training"
)

func TestTrainingRepo_Integration(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping integration test")
	}
	ctx := context.Background()
	pool, err := NewPool(ctx, dsn)
	require.NoError(t, err)
	defer pool.Close()

	const ownerID int64 = 987654321
	cleanup := func() {
		_, _ = pool.Exec(ctx, `DELETE FROM training_ui_states WHERE owner_id = $1`, ownerID)
		_, _ = pool.Exec(ctx, `DELETE FROM training_sessions WHERE owner_id = $1`, ownerID)
		_, _ = pool.Exec(ctx, `DELETE FROM training_programs WHERE owner_id = $1`, ownerID)
		_, _ = pool.Exec(ctx, `DELETE FROM training_exercises WHERE owner_id = $1`, ownerID)
	}
	cleanup()
	t.Cleanup(cleanup)

	repo := NewTrainingRepo(pool)
	_, err = repo.GetUIState(ctx, ownerID)
	require.ErrorIs(t, err, training.ErrNotFound)
	preview := &training.ImportPreview{
		Filename: "plan.txt",
		Programs: []training.ProgramInput{{Name: "Понедельник", Exercises: []string{"Тяга", "Отжимания"}}},
	}
	require.NoError(t, repo.SaveUIState(ctx, training.UIState{
		OwnerID: ownerID, ChatID: ownerID, MessageID: 123, Mode: training.InputImportOK, PendingImport: preview,
	}))
	state, err := repo.GetUIState(ctx, ownerID)
	require.NoError(t, err)
	require.Equal(t, preview, state.PendingImport)

	require.NoError(t, repo.ReplacePrograms(ctx, ownerID, preview.Programs))
	programs, err := repo.ListPrograms(ctx, ownerID)
	require.NoError(t, err)
	require.Len(t, programs, 1)
	editedProgramID := programs[0].ID
	require.Equal(t, []string{"Тяга", "Отжимания"}, programs[0].Exercises)
	catalog, totalExercises, err := repo.ListExercises(ctx, ownerID, 5, 0)
	require.NoError(t, err)
	require.Equal(t, 2, totalExercises)
	require.Len(t, catalog, 2)
	require.Equal(t, []string{"Понедельник"}, catalog[0].Programs)
	similar, err := repo.SimilarExercises(ctx, ownerID, "Тяга гантели", 5)
	require.NoError(t, err)
	require.Len(t, similar, 1)
	require.Equal(t, "Тяга", similar[0].Name)
	pendingProgramExerciseID := programs[0].ExerciseItems[0].ID
	pendingTargetExerciseID := catalog[1].ID
	require.NoError(t, repo.SaveUIState(ctx, training.UIState{
		OwnerID: ownerID, ChatID: ownerID, MessageID: 123, Mode: training.InputProgramExerciseConfirm,
		PendingProgramExerciseID: &pendingProgramExerciseID,
		PendingTargetExerciseID:  &pendingTargetExerciseID,
	}))
	state, err = repo.GetUIState(ctx, ownerID)
	require.NoError(t, err)
	require.Equal(t, training.InputProgramExerciseConfirm, state.Mode)
	require.Equal(t, pendingProgramExerciseID, *state.PendingProgramExerciseID)
	require.Equal(t, pendingTargetExerciseID, *state.PendingTargetExerciseID)

	startedAt := time.Now().UTC().Truncate(time.Microsecond)
	session, err := repo.StartSession(ctx, ownerID, programs[0].ID, startedAt)
	require.NoError(t, err)
	require.True(t, session.Active())
	require.Equal(t, 1, session.CurrentPosition)
	require.Len(t, session.Exercises, 2)
	firstExerciseID := session.Exercises[0].ID
	secondExerciseID := session.Exercises[1].ID

	session, err = repo.PrioritizeExercise(ctx, ownerID, secondExerciseID)
	require.NoError(t, err)
	require.Equal(t, secondExerciseID, session.CurrentExercise().ID)
	require.Equal(t, []int{1, 2}, []int{session.Exercises[0].Position, session.Exercises[1].Position})
	session, err = repo.PrioritizeExercise(ctx, ownerID, firstExerciseID)
	require.NoError(t, err)
	require.Equal(t, firstExerciseID, session.CurrentExercise().ID)
	require.Equal(t, []int{1, 2}, []int{session.Exercises[0].Position, session.Exercises[1].Position})

	_, err = repo.StartSession(ctx, ownerID, programs[0].ID, startedAt)
	require.ErrorIs(t, err, training.ErrActiveSession)

	weight := 40.0
	session, err = repo.AddSet(ctx, ownerID, training.SetInput{Reps: 12, WeightKG: &weight})
	require.NoError(t, err)
	require.Len(t, session.CurrentExercise().Sets, 1)
	session, err = repo.SetCurrentExerciseNote(ctx, ownerID, "рабочий подход")
	require.NoError(t, err)
	require.Equal(t, "рабочий подход", session.CurrentExercise().Note)

	session, err = repo.FinishCurrentExercise(ctx, ownerID, startedAt.Add(time.Minute))
	require.NoError(t, err)
	require.True(t, session.Active())
	require.Equal(t, 2, session.CurrentPosition)
	session, err = repo.AddSet(ctx, ownerID, training.SetInput{Reps: 15})
	require.NoError(t, err)
	require.Nil(t, session.CurrentExercise().Sets[0].WeightKG)

	session, err = repo.FinishCurrentExercise(ctx, ownerID, startedAt.Add(2*time.Minute))
	require.NoError(t, err)
	require.False(t, session.Active())
	require.NotNil(t, session.FinishedAt)
	require.WithinDuration(t, startedAt.Add(2*time.Minute), *session.FinishedAt, 0)
	firstSession := session
	_, err = repo.ActiveSession(ctx, ownerID)
	require.ErrorIs(t, err, training.ErrNoActiveSession)

	reopened, err := repo.ReopenExercise(ctx, ownerID, firstSession.ID, firstSession.Exercises[0].ID)
	require.NoError(t, err)
	require.True(t, reopened.Active())
	require.Equal(t, 1, reopened.CurrentPosition)
	require.Len(t, reopened.CurrentExercise().Sets, 1)
	require.Equal(t, "рабочий подход", reopened.CurrentExercise().Note)
	reopened, err = repo.FinishCurrentExercise(ctx, ownerID, startedAt.Add(3*time.Minute))
	require.NoError(t, err)
	require.False(t, reopened.Active())
	require.WithinDuration(t, startedAt.Add(2*time.Minute), *reopened.FinishedAt, 0, "editing preserves the original workout end")

	secondStartedAt := startedAt.Add(24 * time.Hour)
	second, err := repo.StartSession(ctx, ownerID, programs[0].ID, secondStartedAt)
	require.NoError(t, err)
	previous, err := repo.PreviousExercise(ctx, ownerID, second.ID, second.CurrentExercise().Name)
	require.NoError(t, err)
	require.NotNil(t, previous)
	require.Equal(t, firstSession.StartedAt, previous.StartedAt)
	require.Len(t, previous.Sets, 1)
	require.Equal(t, 12, previous.Sets[0].Reps)
	require.Equal(t, 40.0, *previous.Sets[0].WeightKG)

	_, err = repo.ReopenExercise(ctx, ownerID, second.ID, second.Exercises[1].ID)
	require.ErrorIs(t, err, training.ErrNotEditable)
	_, err = repo.ReopenExercise(ctx, ownerID, firstSession.ID, firstSession.Exercises[0].ID)
	require.ErrorIs(t, err, training.ErrActiveSession)

	secondWeight := 45.0
	second, err = repo.AddSet(ctx, ownerID, training.SetInput{Reps: 10, WeightKG: &secondWeight})
	require.NoError(t, err)
	second, err = repo.FinishCurrentExercise(ctx, ownerID, secondStartedAt.Add(time.Minute))
	require.NoError(t, err)
	require.Equal(t, 2, second.CurrentPosition)
	second, err = repo.ReopenExercise(ctx, ownerID, second.ID, second.Exercises[0].ID)
	require.NoError(t, err)
	require.Equal(t, 1, second.CurrentPosition)
	require.Len(t, second.CurrentExercise().Sets, 1, "editing preserves existing sets")
	second, err = repo.FinishCurrentExercise(ctx, ownerID, secondStartedAt.Add(2*time.Minute))
	require.NoError(t, err)
	require.Equal(t, 2, second.CurrentPosition, "finishing an edited exercise returns to the remaining one")
	second, err = repo.AddSet(ctx, ownerID, training.SetInput{Reps: 15})
	require.NoError(t, err)
	second, err = repo.FinishCurrentExercise(ctx, ownerID, secondStartedAt.Add(3*time.Minute))
	require.NoError(t, err)
	require.False(t, second.Active())

	require.NoError(t, repo.MarkPublished(ctx, ownerID, second.ID, -100123, 456))
	loaded, err := repo.Session(ctx, ownerID, second.ID)
	require.NoError(t, err)
	require.Equal(t, int64(-100123), *loaded.PublishedChatID)
	require.Equal(t, 456, *loaded.PublishedMessageID)
	second, err = repo.ReopenExercise(ctx, ownerID, second.ID, second.Exercises[0].ID)
	require.NoError(t, err)
	require.True(t, second.Active())
	require.Equal(t, int64(-100123), *second.PublishedChatID)
	require.Equal(t, 456, *second.PublishedMessageID)
	publishedEditWeight := 47.5
	second, err = repo.AddSet(ctx, ownerID, training.SetInput{Reps: 8, WeightKG: &publishedEditWeight})
	require.NoError(t, err)
	require.Len(t, second.CurrentExercise().Sets, 2)
	second, err = repo.FinishCurrentExercise(ctx, ownerID, secondStartedAt.Add(4*time.Minute))
	require.NoError(t, err)
	require.False(t, second.Active())
	require.Equal(t, 456, *second.PublishedMessageID)
	require.WithinDuration(t, secondStartedAt.Add(3*time.Minute), *second.FinishedAt, 0, "published edit preserves duration")

	history, totalHistory, err := repo.RecentSessions(ctx, ownerID, 1, 0)
	require.NoError(t, err)
	require.Equal(t, 2, totalHistory)
	require.Len(t, history, 1)
	require.Equal(t, second.ID, history[0].ID)
	history, totalHistory, err = repo.RecentSessions(ctx, ownerID, 1, 1)
	require.NoError(t, err)
	require.Equal(t, 2, totalHistory)
	require.Len(t, history, 1)
	require.Equal(t, firstSession.ID, history[0].ID)

	require.NoError(t, repo.ReplacePrograms(ctx, ownerID, []training.ProgramInput{{
		Name: "Другая программа", Exercises: []string{"Тяга", "Отжимания"},
	}}))
	programs, err = repo.ListPrograms(ctx, ownerID)
	require.NoError(t, err)
	require.Len(t, programs, 2)
	editedProgram, err := repo.Program(ctx, ownerID, editedProgramID)
	require.NoError(t, err)
	require.Len(t, editedProgram.ExerciseItems, 2)

	withHistory, err := repo.ReplaceProgramExercise(
		ctx, ownerID, editedProgram.ExerciseItems[1].ID, nil, "Отжимания на брусьях", true,
	)
	require.NoError(t, err)
	require.Len(t, withHistory.PublishedSessions, 1)
	require.Equal(t, []string{"Тяга", "Отжимания на брусьях"}, withHistory.Program.Exercises)
	loaded, err = repo.Session(ctx, ownerID, firstSession.ID)
	require.NoError(t, err)
	require.Equal(t, "Отжимания на брусьях", loaded.Exercises[1].Name)
	loaded, err = repo.Session(ctx, ownerID, second.ID)
	require.NoError(t, err)
	require.Equal(t, "Отжимания на брусьях", loaded.Exercises[1].Name)

	targetID := *withHistory.Program.ExerciseItems[1].ExerciseID
	programOnly, err := repo.ReplaceProgramExercise(
		ctx, ownerID, withHistory.Program.ExerciseItems[0].ID, &targetID, "", false,
	)
	require.NoError(t, err)
	require.Empty(t, programOnly.PublishedSessions)
	require.Equal(t, []string{"Отжимания на брусьях", "Отжимания на брусьях"}, programOnly.Program.Exercises)
	var otherProgramID int64
	for _, program := range programs {
		if program.ID != editedProgramID {
			otherProgramID = program.ID
		}
	}
	otherProgram, err := repo.Program(ctx, ownerID, otherProgramID)
	require.NoError(t, err)
	require.Equal(t, []string{"Тяга", "Отжимания"}, otherProgram.Exercises)
	loaded, err = repo.Session(ctx, ownerID, firstSession.ID)
	require.NoError(t, err)
	require.Equal(t, "Тяга", loaded.Exercises[0].Name)
	loaded, err = repo.Session(ctx, ownerID, second.ID)
	require.NoError(t, err)
	require.Equal(t, "Тяга", loaded.Exercises[0].Name)

	catalog, totalExercises, err = repo.ListExercises(ctx, ownerID, 5, 0)
	require.NoError(t, err)
	require.Equal(t, 3, totalExercises)
	var pull training.Exercise
	for _, exercise := range catalog {
		if exercise.Name == "Тяга" {
			pull = exercise
		}
	}
	require.NotZero(t, pull.ID)
	rename, err := repo.RenameExercise(ctx, ownerID, pull.ID, "Тяга верхнего блока")
	require.NoError(t, err)
	require.False(t, rename.Merged)
	require.Equal(t, "Тяга верхнего блока", rename.Exercise.Name)
	require.Len(t, rename.PublishedSessions, 1)
	loaded, err = repo.Session(ctx, ownerID, firstSession.ID)
	require.NoError(t, err)
	require.Equal(t, "Тяга верхнего блока", loaded.Exercises[0].Name)
	programs, err = repo.ListPrograms(ctx, ownerID)
	require.NoError(t, err)

	require.ErrorIs(t, repo.DeleteSession(ctx, ownerID+1, second.ID), training.ErrNotFound)
	require.NoError(t, repo.DeleteSession(ctx, ownerID, second.ID))
	_, err = repo.Session(ctx, ownerID, second.ID)
	require.ErrorIs(t, err, training.ErrNotFound)
	history, totalHistory, err = repo.RecentSessions(ctx, ownerID, 5, 0)
	require.NoError(t, err)
	require.Equal(t, 1, totalHistory)
	require.Len(t, history, 1)
	require.Equal(t, firstSession.ID, history[0].ID)

	activeToDelete, err := repo.StartSession(ctx, ownerID, editedProgramID, secondStartedAt.Add(24*time.Hour))
	require.NoError(t, err)
	activeExerciseID := activeToDelete.CurrentExercise().ID
	activeToDelete, err = repo.AddSet(ctx, ownerID, training.SetInput{Reps: 8, WeightKG: &secondWeight})
	require.NoError(t, err)
	activeSetID := activeToDelete.CurrentExercise().Sets[0].ID
	require.NoError(t, repo.DeleteSession(ctx, ownerID, activeToDelete.ID))
	_, err = repo.ActiveSession(ctx, ownerID)
	require.ErrorIs(t, err, training.ErrNoActiveSession)
	var exercises, sets int
	require.NoError(t, pool.QueryRow(ctx, `
		SELECT
			(SELECT count(*) FROM training_session_exercises WHERE id = $1),
			(SELECT count(*) FROM training_sets WHERE id = $2)`, activeExerciseID, activeSetID,
	).Scan(&exercises, &sets))
	require.Zero(t, exercises, "session exercises are deleted through cascading foreign keys")
	require.Zero(t, sets, "training sets are deleted through cascading foreign keys")
}

func TestTrainingRepo_PrioritizeExerciseIntegration(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping integration test")
	}
	ctx := context.Background()
	pool, err := NewPool(ctx, dsn)
	require.NoError(t, err)
	defer pool.Close()

	const ownerID int64 = 987654322
	cleanup := func() {
		_, _ = pool.Exec(ctx, `DELETE FROM training_sessions WHERE owner_id = $1`, ownerID)
		_, _ = pool.Exec(ctx, `DELETE FROM training_programs WHERE owner_id = $1`, ownerID)
		_, _ = pool.Exec(ctx, `DELETE FROM training_exercises WHERE owner_id = $1`, ownerID)
	}
	cleanup()
	t.Cleanup(cleanup)

	repo := NewTrainingRepo(pool)
	require.NoError(t, repo.ReplacePrograms(ctx, ownerID, []training.ProgramInput{{
		Name: "Порядок", Exercises: []string{"Первое", "Второе", "Третье"},
	}}))
	programs, err := repo.ListPrograms(ctx, ownerID)
	require.NoError(t, err)
	require.Len(t, programs, 1)
	session, err := repo.StartSession(ctx, ownerID, programs[0].ID, time.Now())
	require.NoError(t, err)
	require.Len(t, session.Exercises, 3)

	thirdID := session.Exercises[2].ID
	session, err = repo.PrioritizeExercise(ctx, ownerID, thirdID)
	require.NoError(t, err)
	require.Equal(t, []string{"Третье", "Первое", "Второе"}, []string{
		session.Exercises[0].Name, session.Exercises[1].Name, session.Exercises[2].Name,
	})
	require.Equal(t, []int{1, 2, 3}, []int{
		session.Exercises[0].Position, session.Exercises[1].Position, session.Exercises[2].Position,
	})
	require.Equal(t, thirdID, session.CurrentExercise().ID)

	session, err = repo.FinishCurrentExercise(ctx, ownerID, time.Now())
	require.NoError(t, err)
	_, err = repo.PrioritizeExercise(ctx, ownerID, thirdID)
	require.ErrorIs(t, err, training.ErrNotEditable, "a completed exercise cannot be moved back into the active queue")
}

func TestTrainingRepo_ProgramsV1Integration(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping integration test")
	}
	ctx := context.Background()
	pool, err := NewPool(ctx, dsn)
	require.NoError(t, err)
	defer pool.Close()

	const ownerID int64 = 987654322
	cleanup := func() {
		_, _ = pool.Exec(ctx, `DELETE FROM training_ui_states WHERE owner_id = $1`, ownerID)
		_, _ = pool.Exec(ctx, `DELETE FROM training_sessions WHERE owner_id = $1`, ownerID)
		_, _ = pool.Exec(ctx, `DELETE FROM training_programs WHERE owner_id = $1`, ownerID)
		_, _ = pool.Exec(ctx, `DELETE FROM training_exercises WHERE owner_id = $1`, ownerID)
	}
	cleanup()
	t.Cleanup(cleanup)

	raw := `version: 1
program:
  name: Test Program
workouts:
  - id: bench_day
    name: Bench Day
    exercises:
      - exercise: Жим штанги лёжа
        sets: 3
        reps: 8-12
        target_rir: 2
        starting_weight: 60kg
        weight_step: 2.5kg
        rest: 180s
        after: 180s
        progression: double
      - exercise: Тяга блока
        sets: 1
        reps: 8
        target_rir: 2
        starting_weight: 40kg
        weight_step: 5kg
        rest: 120s
        after: 0s
        progression: double
`
	programs, err := training.ParsePrograms("program.yaml", strings.NewReader(raw))
	require.NoError(t, err)
	require.InDelta(t, 2.5, programs[0].Prescriptions[0].WeightStepKG, 0.001)
	repo := NewTrainingRepo(pool)
	require.NoError(t, repo.ReplacePrograms(ctx, ownerID, programs))

	templates, err := repo.ListPrograms(ctx, ownerID)
	require.NoError(t, err)
	require.Len(t, templates, 1)
	require.Equal(t, 1, templates[0].Revision)
	require.Equal(t, "bench_day", templates[0].WorkoutKey)
	startedAt := time.Date(2026, 8, 17, 8, 0, 0, 0, time.UTC)
	first, err := repo.StartSession(ctx, ownerID, templates[0].ID, startedAt)
	require.NoError(t, err)
	require.NotNil(t, first.RevisionID)
	require.InDelta(t, 60, *first.CurrentExercise().Recommendation.WeightKG, 0.001)
	require.InDelta(t, 60, *first.CurrentExercise().Plan.WeightKG, 0.001)

	rir := 2.0
	weight := 60.0
	for index := 0; index < 3; index++ {
		first, err = repo.AddSet(ctx, ownerID, training.SetInput{
			Type: training.SetTypeWorking, WeightKG: &weight, Reps: 12, RIR: &rir,
			CompletedAt: startedAt.Add(time.Duration(index+1) * time.Minute),
		})
		require.NoError(t, err)
	}
	first, err = repo.FinishCurrentExercise(ctx, ownerID, startedAt.Add(4*time.Minute))
	require.NoError(t, err)
	require.True(t, first.Active())
	require.Equal(t, 2, first.CurrentPosition)
	require.InDelta(t, 40, *first.CurrentExercise().Plan.WeightKG, 0.001)
	require.NotNil(t, first.RestUntil)
	require.WithinDuration(t, startedAt.Add(7*time.Minute), *first.RestUntil, 0)
	pullWeight := 40.0
	first, err = repo.AddSet(ctx, ownerID, training.SetInput{
		Type: training.SetTypeWorking, WeightKG: &pullWeight, Reps: 8, RIR: &rir,
		CompletedAt: startedAt.Add(7 * time.Minute),
	})
	require.NoError(t, err)
	first, err = repo.FinishCurrentExercise(ctx, ownerID, startedAt.Add(8*time.Minute))
	require.NoError(t, err)
	require.False(t, first.Active())

	// Re-import creates a new immutable revision and activates its templates.
	require.NoError(t, repo.ReplacePrograms(ctx, ownerID, programs))
	templates, err = repo.ListPrograms(ctx, ownerID)
	require.NoError(t, err)
	require.Len(t, templates, 1)
	require.Equal(t, 2, templates[0].Revision)
	require.NotEqual(t, *first.RevisionID, templates[0].RevisionID)

	second, err := repo.StartSession(ctx, ownerID, templates[0].ID, startedAt.Add(7*24*time.Hour))
	require.NoError(t, err)
	require.InDelta(t, 62.5, *second.CurrentExercise().Recommendation.WeightKG, 0.001)
	require.Equal(t, training.ProgressionIncrease, second.CurrentExercise().Recommendation.Action)
	require.NotEmpty(t, second.CurrentExercise().Recommendation.ReasonCode)

	overrideWeight := 60.0
	second, err = repo.OverrideCurrentExercise(ctx, ownerID, training.ExerciseOverride{
		WeightKG: &overrideWeight, Reps: training.RepRange{Min: 8, Max: 12},
		WorkingSets: 3, TargetRIR: 2, RestSeconds: 180,
	})
	require.NoError(t, err)
	require.InDelta(t, 62.5, *second.CurrentExercise().Recommendation.WeightKG, 0.001)
	require.InDelta(t, 60, *second.CurrentExercise().Plan.WeightKG, 0.001)

	actualWeight := 55.0
	second, err = repo.AddSet(ctx, ownerID, training.SetInput{
		Type: training.SetTypeWorking, WeightKG: &actualWeight, Reps: 11,
		CompletedAt: startedAt.Add(7*24*time.Hour + time.Minute),
	})
	require.NoError(t, err)
	set := second.CurrentExercise().Sets[0]
	require.InDelta(t, 60, *set.PlannedWeightKG, 0.001)
	require.InDelta(t, 55, *set.ActualWeightKG, 0.001)
	require.Equal(t, 11, *set.ActualReps)

	second, err = repo.AddSet(ctx, ownerID, training.SetInput{
		Type: training.SetTypeWorking, WeightKG: &actualWeight, Reps: 10,
		CompletedAt: startedAt.Add(7*24*time.Hour + 2*time.Minute),
	})
	require.NoError(t, err)
	set = second.CurrentExercise().Sets[1]
	require.InDelta(t, 55, *set.PlannedWeightKG, 0.001)
	require.InDelta(t, 55, *set.ActualWeightKG, 0.001)
}
