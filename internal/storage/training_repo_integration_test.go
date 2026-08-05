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
	_, err = repo.ActiveSession(ctx, ownerID)
	require.ErrorIs(t, err, training.ErrNoActiveSession)

	require.NoError(t, repo.MarkPublished(ctx, ownerID, session.ID, -100123, 456))
	loaded, err := repo.Session(ctx, ownerID, session.ID)
	require.NoError(t, err)
	require.Equal(t, int64(-100123), *loaded.PublishedChatID)
	require.Equal(t, 456, *loaded.PublishedMessageID)

	history, err := repo.RecentSessions(ctx, ownerID, 10)
	require.NoError(t, err)
	require.Len(t, history, 1)
	require.Equal(t, session.ID, history[0].ID)
}
