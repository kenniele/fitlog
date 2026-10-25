package controlcenter

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

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
	updated.FiberIncomplete = true
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
	day := time.Unix(int64(first.DateInt)*86400, 0).UTC()
	overview, err := repository.Overview(ctx, ownerID, DateRange{From: day, To: day}, time.UTC)
	if err != nil {
		t.Fatal(err)
	}
	metric := overview.Summary.Nutrition.Fiber
	if len(overview.Daily) != 1 || overview.Daily[0].FiberG == nil || *overview.Daily[0].FiberG != 28 ||
		!overview.Daily[0].FiberIncomplete || metric.IncompleteSamples != 1 ||
		metric.Current == nil || *metric.Current != 28 || metric.Average == nil || *metric.Average != 28 || metric.Samples != 1 {
		t.Fatalf("fiber missing from overview: daily=%+v metric=%+v", overview.Daily, metric)
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
		if status.Source == "fatsecret" && (status.Connected || status.Status != "provider_sync") {
			t.Fatalf("FatSecret sync status = %+v", status)
		}
	}
	if err := repository.UpsertFatSecretNutritionDays(ctx, ownerID, []domain.NutritionDaySnapshot{
		{DateInt: first.DateInt + 1, CaloriesKcal: value(2000), FiberG: value(0)},
		{DateInt: first.DateInt + 2, CaloriesKcal: value(2000)},
	}); err != nil {
		t.Fatal(err)
	}
	overview, err = repository.Overview(ctx, ownerID, DateRange{From: day, To: day.AddDate(0, 0, 2)}, time.UTC)
	if err != nil {
		t.Fatal(err)
	}
	metric = overview.Summary.Nutrition.Fiber
	if metric.Samples != 2 || metric.IncompleteSamples != 1 || metric.Average == nil || *metric.Average != 14 {
		t.Fatalf("fiber summary lost zero, null or incomplete semantics: %+v", metric)
	}
	listed, err := repository.List(ctx, ownerID, "nutrition", Pagination{Page: 1, PageSize: 10}, time.UTC)
	if err != nil {
		t.Fatal(err)
	}
	for _, raw := range listed.Items {
		var record struct {
			ExternalID      string   `json:"external_id"`
			FiberG          *float64 `json:"fiber_g"`
			FiberIncomplete bool     `json:"fiber_incomplete"`
		}
		if err := json.Unmarshal(raw, &record); err != nil {
			t.Fatal(err)
		}
		if record.ExternalID == "sync:day:20686" {
			if record.FiberG == nil || *record.FiberG != 28 || !record.FiberIncomplete {
				t.Fatalf("list lost fiber metadata: %s", raw)
			}
			return
		}
	}
	t.Fatal("synced fiber record absent from list")
}
