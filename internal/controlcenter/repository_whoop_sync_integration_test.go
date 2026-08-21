package controlcenter

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"fitlog/internal/domain"
)

func TestPostgresRepository_UpsertWhoopHealth(t *testing.T) {
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
	const ownerID int64 = 9_101_023_970
	const otherOwnerID int64 = ownerID + 1
	for _, id := range []int64{ownerID, otherOwnerID} {
		for _, table := range []string{"sleep_entries", "recovery_entries"} {
			if _, err := pool.Exec(ctx, `DELETE FROM `+table+` WHERE owner_id=$1`, id); err != nil {
				t.Fatal(err)
			}
		}
	}
	t.Cleanup(func() {
		for _, id := range []int64{ownerID, otherOwnerID} {
			_, _ = pool.Exec(ctx, `DELETE FROM sleep_entries WHERE owner_id=$1`, id)
			_, _ = pool.Exec(ctx, `DELETE FROM recovery_entries WHERE owner_id=$1`, id)
		}
	})

	float := func(value float64) *float64 { return &value }
	integer := func(value int64) *int64 { return &value }
	count := func(value int) *int { return &value }
	day := time.Date(2026, 8, 20, 0, 0, 0, 0, time.UTC)
	start := time.Date(2026, 8, 19, 21, 0, 0, 0, time.UTC)
	end := time.Date(2026, 8, 20, 5, 0, 0, 0, time.UTC)
	recovery := domain.WhoopRecoverySnapshot{
		EntryDate: day, CycleID: 101, RecoveryScore: float(72), HRVMs: float(54.2),
		RestingHeartRateBPM: float(52), RespiratoryRate: float(15.1),
		SpO2Percent: float(97), SkinTemperatureC: float(33.5), DailyStrain: float(11.4),
	}
	sleep := domain.WhoopSleepSnapshot{
		SleepDate: day, ExternalID: "sleep-101", Start: start, End: end,
		TimeInBedSeconds: integer(28_800), ActualSleepSeconds: integer(26_000),
		AwakeSeconds: integer(2_800), REMSeconds: integer(5_000), DeepSeconds: integer(7_000),
		LightSeconds: integer(14_000), SleepPerformancePct: float(86), EfficiencyPct: float(90),
		ConsistencyPct: float(82), SleepDebtSeconds: integer(1_200), Disturbances: count(8),
	}
	if err := repository.UpsertWhoopHealth(ctx, ownerID, []domain.WhoopRecoverySnapshot{recovery}, []domain.WhoopSleepSnapshot{sleep}); err != nil {
		t.Fatalf("first sync: %v", err)
	}
	if _, err := pool.Exec(ctx, `UPDATE recovery_entries SET notes='keep recovery note'
		WHERE owner_id=$1 AND external_id='sync:recovery:101'`, ownerID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `UPDATE sleep_entries SET notes='keep sleep note'
		WHERE owner_id=$1 AND external_id='sync:sleep:sleep-101'`, ownerID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO recovery_entries (owner_id,entry_date,recovery_score,source)
		VALUES ($1,'2026-08-20',50,'manual')`, ownerID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO sleep_entries (owner_id,sleep_date,is_nap,source)
		VALUES ($1,'2026-08-20',false,'manual')`, ownerID); err != nil {
		t.Fatal(err)
	}
	if err := repository.UpsertWhoopHealth(ctx, otherOwnerID, []domain.WhoopRecoverySnapshot{recovery}, []domain.WhoopSleepSnapshot{sleep}); err != nil {
		t.Fatalf("other owner sync: %v", err)
	}

	recovery.RecoveryScore = float(78)
	recovery.DailyStrain = float(13.2)
	sleep.SleepPerformancePct = float(91)
	sleep.ActualSleepSeconds = integer(27_000)
	if err := repository.UpsertWhoopHealth(ctx, ownerID, []domain.WhoopRecoverySnapshot{recovery}, []domain.WhoopSleepSnapshot{sleep}); err != nil {
		t.Fatalf("second sync: %v", err)
	}

	var rows int
	var recoveryScore, strain float64
	var notes string
	if err := pool.QueryRow(ctx, `SELECT count(*)::int,max(recovery_score)::double precision,
		max(daily_strain)::double precision,max(notes) FROM recovery_entries
		WHERE owner_id=$1 AND source='whoop' AND external_id='sync:recovery:101'`, ownerID).
		Scan(&rows, &recoveryScore, &strain, &notes); err != nil {
		t.Fatal(err)
	}
	if rows != 1 || recoveryScore != 78 || strain != 13.2 || notes != "keep recovery note" {
		t.Fatalf("recovery row count=%d score=%v strain=%v notes=%q", rows, recoveryScore, strain, notes)
	}
	var performance float64
	var actualSleep int64
	if err := pool.QueryRow(ctx, `SELECT count(*)::int,max(sleep_performance_percent)::double precision,
		max(actual_sleep_seconds),max(notes) FROM sleep_entries
		WHERE owner_id=$1 AND source='whoop' AND external_id='sync:sleep:sleep-101'`, ownerID).
		Scan(&rows, &performance, &actualSleep, &notes); err != nil {
		t.Fatal(err)
	}
	if rows != 1 || performance != 91 || actualSleep != 27_000 || notes != "keep sleep note" {
		t.Fatalf("sleep row count=%d performance=%v seconds=%d notes=%q", rows, performance, actualSleep, notes)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*)::int FROM recovery_entries WHERE owner_id=$1 AND source='manual'`, ownerID).Scan(&rows); err != nil {
		t.Fatal(err)
	}
	if rows != 1 {
		t.Fatalf("manual recovery rows changed: %d", rows)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*)::int FROM recovery_entries
		WHERE owner_id=$1 AND external_id='sync:recovery:101'`, otherOwnerID).Scan(&rows); err != nil {
		t.Fatal(err)
	}
	if rows != 1 {
		t.Fatalf("other owner recovery rows changed: %d", rows)
	}
	statuses, err := repository.Sources(ctx, ownerID)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, status := range statuses {
		if status.Source == "whoop" {
			found = true
			if status.Connected || status.Status != "manual_sync" || status.LastSyncedAt == nil {
				t.Fatalf("WHOOP sync status = %+v", status)
			}
		}
	}
	if !found {
		t.Fatal("WHOOP source status not found")
	}
}

func TestPostgresRepository_UpsertWhoopHealthRollsBackInvalidBatch(t *testing.T) {
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
	const ownerID int64 = 9_101_023_972
	_, _ = pool.Exec(ctx, `DELETE FROM sleep_entries WHERE owner_id=$1`, ownerID)
	_, _ = pool.Exec(ctx, `DELETE FROM recovery_entries WHERE owner_id=$1`, ownerID)
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM sleep_entries WHERE owner_id=$1`, ownerID)
		_, _ = pool.Exec(ctx, `DELETE FROM recovery_entries WHERE owner_id=$1`, ownerID)
	})
	day := time.Date(2026, 8, 20, 0, 0, 0, 0, time.UTC)
	value := 70.0
	err = NewRepository(pool).UpsertWhoopHealth(ctx, ownerID,
		[]domain.WhoopRecoverySnapshot{{EntryDate: day, CycleID: 202, RecoveryScore: &value}},
		[]domain.WhoopSleepSnapshot{{SleepDate: day, ExternalID: "broken"}},
	)
	if err == nil {
		t.Fatal("expected invalid sleep error")
	}
	var rows int
	if err := pool.QueryRow(ctx, `SELECT count(*)::int FROM recovery_entries WHERE owner_id=$1`, ownerID).Scan(&rows); err != nil {
		t.Fatal(err)
	}
	if rows != 0 {
		t.Fatalf("partial recovery batch committed: %d", rows)
	}
}
