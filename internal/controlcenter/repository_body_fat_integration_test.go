//go:build integration

package controlcenter

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

func TestPostgresRepositoryBodyFatInputs(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	const owner int64 = 9101023990
	repo := NewRepository(pool)
	cleanup := func() {
		require.NoError(t, repo.DeleteAll(ctx, owner))
		require.NoError(t, repo.DeleteAll(ctx, owner+1))
	}
	cleanup()
	t.Cleanup(cleanup)
	loc, err := time.LoadLocation("Europe/Moscow")
	require.NoError(t, err)
	now := time.Date(2026, 9, 5, 12, 0, 0, 0, time.UTC)
	_, err = pool.Exec(ctx, `INSERT INTO body_measurements (owner_id,measured_at,weight_kg,body_fat_percent,source) VALUES
		($1,'2026-09-01T22:00:00Z',100,20,'inbody'),
		($1,'2026-09-01T21:00:00Z',101,21,'inbody'),
		($1,'2026-09-04T10:00:00Z',102,22,'manual'),
		($1,'2026-09-06T10:00:00Z',103,23,'inbody'),
		($2,'2026-09-04T10:00:00Z',104,24,'inbody')`, owner, owner+1)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `INSERT INTO nutrition_days (owner_id,entry_date,calories_kcal,source,updated_at) VALUES
		($1,'2026-09-02',100,'manual','2026-09-05T10:00:00Z'),
		($1,'2026-09-03',2300,'fatsecret','2026-09-05T10:00:00Z'),
		($1,'2026-09-03',NULL,'manual','2026-09-05T11:00:00Z'),
		($1,'2026-09-04',2100,'manual','2026-09-05T10:00:00Z'),
		($1,'2026-09-04',2200,'fatsecret','2026-09-05T10:00:00Z'),
		($1,'2026-09-05',100,'manual','2026-09-05T10:00:00Z'),
		($2,'2026-09-04',9900,'manual','2026-09-05T10:00:00Z')`, owner, owner+1)
	require.NoError(t, err)
	inputs, err := repo.loadBodyFatInputs(ctx, owner, now, loc)
	require.NoError(t, err)
	require.Equal(t, "2026-09-01T22:00:00Z", inputs.Baseline.MeasuredAt.UTC().Format(time.RFC3339))
	require.Len(t, inputs.Days, 2)
	require.Equal(t, "2026-09-03", inputs.Days[0].Date)
	require.Nil(t, inputs.Days[0].CaloriesKcal, "latest unknown diary must not resurrect an older known value")
	require.Equal(t, 2200.0, *inputs.Days[1].CaloriesKcal, "latest id wins ties and sources must not be summed")
	require.Equal(t, "insufficient_nutrition", estimateBodyFat(inputs, 2700, now, loc).Status)
	list, err := repo.List(ctx, owner, "body-measurements", Pagination{Page: 1, PageSize: 2, To: &now, Filters: map[string]string{"source": "inbody"}}, loc)
	require.NoError(t, err)
	var listed struct {
		MeasuredAt time.Time `json:"measured_at"`
	}
	require.NoError(t, json.Unmarshal(list.Items[0], &listed))
	require.True(t, listed.MeasuredAt.Equal(inputs.Baseline.MeasuredAt), "same-day backfills must not override the latest scan")

	_, err = pool.Exec(ctx, `UPDATE nutrition_days SET calories_kcal=2300 WHERE owner_id=$1 AND entry_date='2026-09-03' AND source='manual'`, owner)
	require.NoError(t, err)
	service := NewService(repo, owner, loc, WithEstimatedTDEE(2700))
	service.now = func() time.Time { return now }
	estimate, err := service.bodyFatEstimate(ctx)
	require.NoError(t, err)
	require.Equal(t, "estimated", estimate.Status)
	require.Equal(t, 2, estimate.TotalDays)
	require.Equal(t, 900.0, *estimate.EstimatedDeficitKcal)

	_, err = pool.Exec(ctx, `INSERT INTO body_measurements (owner_id,measured_at,weight_kg,source) VALUES ($1,'2026-09-04T10:00:00Z',100,'inbody')`, owner)
	require.NoError(t, err)
	estimate, err = service.bodyFatEstimate(ctx)
	require.NoError(t, err)
	require.Equal(t, "incomplete_inbody", estimate.Status, "do not fall back to an older usable InBody")
}
