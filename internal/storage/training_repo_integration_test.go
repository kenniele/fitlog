//go:build integration

package storage

import (
	"context"
	"os"
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
	pendingExerciseID := similar[0].ID
	require.NoError(t, repo.SaveUIState(ctx, training.UIState{
		OwnerID: ownerID, ChatID: ownerID, MessageID: 123, Mode: training.InputRename,
		PendingExerciseID: &pendingExerciseID,
	}))
	state, err = repo.GetUIState(ctx, ownerID)
	require.NoError(t, err)
	require.Equal(t, training.InputRename, state.Mode)
	require.Equal(t, pendingExerciseID, *state.PendingExerciseID)

	startedAt := time.Now().UTC().Truncate(time.Microsecond)
	session, err := repo.StartSession(ctx, ownerID, programs[0].ID, startedAt)
	require.NoError(t, err)
	require.True(t, session.Active())
	require.Equal(t, 1, session.CurrentPosition)
	require.Len(t, session.Exercises, 2)

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
	programs, err = repo.ListPrograms(ctx, ownerID)
	require.NoError(t, err)
	require.Equal(t, "Тяга верхнего блока", programs[0].Exercises[0])
	loaded, err = repo.Session(ctx, ownerID, firstSession.ID)
	require.NoError(t, err)
	require.Equal(t, "Тяга верхнего блока", loaded.Exercises[0].Name)
	merged, err := repo.RenameExercise(ctx, ownerID, rename.Exercise.ID, "отжимания")
	require.NoError(t, err)
	require.True(t, merged.Merged)
	require.Equal(t, "Отжимания", merged.Exercise.Name)
	programs, err = repo.ListPrograms(ctx, ownerID)
	require.NoError(t, err)
	require.Equal(t, []string{"Отжимания", "Отжимания"}, programs[0].Exercises)
	catalog, totalExercises, err = repo.ListExercises(ctx, ownerID, 5, 0)
	require.NoError(t, err)
	require.Equal(t, 1, totalExercises)
	require.Len(t, catalog, 1)

	require.ErrorIs(t, repo.DeleteSession(ctx, ownerID+1, second.ID), training.ErrNotFound)
	require.NoError(t, repo.DeleteSession(ctx, ownerID, second.ID))
	_, err = repo.Session(ctx, ownerID, second.ID)
	require.ErrorIs(t, err, training.ErrNotFound)
	history, totalHistory, err = repo.RecentSessions(ctx, ownerID, 5, 0)
	require.NoError(t, err)
	require.Equal(t, 1, totalHistory)
	require.Len(t, history, 1)
	require.Equal(t, firstSession.ID, history[0].ID)

	activeToDelete, err := repo.StartSession(ctx, ownerID, programs[0].ID, secondStartedAt.Add(24*time.Hour))
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
