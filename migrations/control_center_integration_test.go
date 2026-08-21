package migrations

import (
	"context"
	"database/sql"
	"os"
	"testing"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
)

// This test is opt-in because it performs a real down/up round trip. Point it
// only at an isolated database; production migration code remains up-only.
func TestControlCenterMigrationDownUp(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	ctx := context.Background()
	if err := Up(ctx, databaseURL); err != nil {
		t.Fatalf("apply migrations before round trip: %v", err)
	}
	db, err := sql.Open("pgx", databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	goose.SetBaseFS(migrationFiles)
	if err := goose.SetDialect("postgres"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := goose.UpContext(context.Background(), db, "."); err != nil {
			t.Errorf("restore migration after test: %v", err)
		}
	})
	if err := goose.DownContext(ctx, db, "."); err != nil {
		t.Fatalf("roll back InBody migration: %v", err)
	}
	var segmentTableExists, bodyTableExists bool
	if err := db.QueryRowContext(ctx, `SELECT
		to_regclass('public.body_segment_measurements') IS NOT NULL,
		to_regclass('public.body_measurements') IS NOT NULL`).Scan(&segmentTableExists, &bodyTableExists); err != nil {
		t.Fatal(err)
	}
	if segmentTableExists || !bodyTableExists {
		t.Fatalf("unexpected schema after InBody down: segments=%v body=%v", segmentTableExists, bodyTableExists)
	}
	if err := goose.DownContext(ctx, db, "."); err != nil {
		t.Fatalf("roll back control center migration: %v", err)
	}
	var recoveryTableExists bool
	if err := db.QueryRowContext(ctx, `SELECT to_regclass('public.recovery_entries') IS NOT NULL`).Scan(&recoveryTableExists); err != nil {
		t.Fatal(err)
	}
	if recoveryTableExists {
		t.Fatal("recovery_entries still exists after migration down")
	}
	if err := goose.UpContext(ctx, db, "."); err != nil {
		t.Fatalf("re-apply control center migration: %v", err)
	}
}
