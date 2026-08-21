package controlcenter

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"fitlog/internal/domain"
)

func TestPostgresRepository_UpsertFatSecretNutritionDays(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	repository := NewRepository(pool)
	const ownerID int64 = 9_101_023_960
	const otherOwnerID int64 = ownerID + 1
	for _, id := range []int64{ownerID, otherOwnerID} {
		if _, err := pool.Exec(ctx, `DELETE FROM nutrition_days WHERE owner_id=$1`, id); err != nil {
			t.Fatal(err)
		}
	}
	t.Cleanup(func() {
		for _, id := range []int64{ownerID, otherOwnerID} {
			_, _ = pool.Exec(ctx, `DELETE FROM nutrition_days WHERE owner_id=$1`, id)
		}
	})

	value := func(number float64) *float64 { return &number }
	first := domain.NutritionDaySnapshot{
		DateInt: 20686, CaloriesKcal: value(2100), ProteinG: value(150),
		FatG: value(70), CarbohydratesG: value(220),
	}
	if err := repository.UpsertFatSecretNutritionDays(ctx, ownerID, []domain.NutritionDaySnapshot{first}); err != nil {
		t.Fatalf("first sync: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO nutrition_days (owner_id,entry_date,calories_kcal,source)
		VALUES ($1,'2026-08-21',1800,'manual')`, ownerID); err != nil {
		t.Fatalf("insert manual row: %v", err)
	}
	if err := repository.UpsertFatSecretNutritionDays(ctx, otherOwnerID, []domain.NutritionDaySnapshot{first}); err != nil {
		t.Fatalf("other owner sync: %v", err)
	}

	updated := first
	updated.CaloriesKcal = value(2250)
	updated.FiberG = value(28)
	if err := repository.UpsertFatSecretNutritionDays(ctx, ownerID, []domain.NutritionDaySnapshot{updated}); err != nil {
		t.Fatalf("second sync: %v", err)
	}

	var count int
	var calories, fiber *float64
	if err := pool.QueryRow(ctx, `SELECT count(*)::int,max(calories_kcal)::double precision,
		max(fiber_g)::double precision FROM nutrition_days
		WHERE owner_id=$1 AND source='fatsecret' AND external_id='sync:day:20686'`, ownerID).
		Scan(&count, &calories, &fiber); err != nil {
		t.Fatal(err)
	}
	if count != 1 || calories == nil || *calories != 2250 || fiber == nil || *fiber != 28 {
		t.Fatalf("owner sync row: count=%d calories=%v fiber=%v", count, calories, fiber)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*)::int FROM nutrition_days WHERE owner_id=$1 AND source='manual'`, ownerID).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("manual rows changed: %d", count)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*)::int FROM nutrition_days
		WHERE owner_id=$1 AND source='fatsecret' AND external_id='sync:day:20686'`, otherOwnerID).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("other owner row changed: %d", count)
	}
	statuses, err := repository.Sources(ctx, ownerID)
	if err != nil {
		t.Fatalf("list sources after sync: %v", err)
	}
	for _, status := range statuses {
		if status.Source == "fatsecret" && (status.Connected || status.Status != "manual_sync") {
			t.Fatalf("FatSecret sync status = %+v", status)
		}
	}
}
