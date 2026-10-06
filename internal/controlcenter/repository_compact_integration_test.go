//go:build integration

package controlcenter

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestNextCompactWorkoutUsesNearestScheduledSessionOfOwner(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	const ownerID int64 = 9_101_026_006
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, "DELETE FROM training_sessions WHERE owner_id IN ($1,$2)", ownerID, ownerID+1)
	})
	from := time.Date(2026, 10, 6, 0, 0, 0, 0, time.UTC)
	for _, row := range []struct {
		owner        int64
		name, status string
		when         time.Time
	}{
		{ownerID, "Yesterday", "scheduled", from.Add(-time.Hour)},
		{ownerID, "Nearest", "scheduled", from.Add(time.Hour)},
		{ownerID, "Later", "scheduled", from.Add(24 * time.Hour)},
		{ownerID, "Cancelled", "cancelled", from.Add(time.Minute)},
		{ownerID + 1, "Other owner", "scheduled", from.Add(time.Second)},
	} {
		if _, err := pool.Exec(ctx, `INSERT INTO training_sessions(owner_id,program_name,status,scheduled_at) VALUES ($1,$2,$3,$4)`, row.owner, row.name, row.status, row.when); err != nil {
			t.Fatal(err)
		}
	}
	repository := NewRepository(pool)
	workout, err := repository.NextCompactWorkout(ctx, ownerID, from)
	if err != nil {
		t.Fatal(err)
	}
	if workout == nil || workout.Title != "Nearest" || !workout.ScheduledAt.Equal(from.Add(time.Hour)) {
		t.Fatalf("wrong next workout: %#v", workout)
	}
	empty, err := repository.NextCompactWorkout(ctx, ownerID, from.Add(48*time.Hour))
	if err != nil || empty != nil {
		t.Fatalf("empty: %#v %v", empty, err)
	}
}
