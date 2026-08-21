package controlcenter

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"fitlog/internal/domain"
)

// UpsertWhoopHealth persists recovery and sleep in one owner-scoped
// transaction. Only source=whoop rows in the sync namespace are owned by this
// operation; manual/file records and other owners are never modified.
func (r *PostgresRepository) UpsertWhoopHealth(
	ctx context.Context,
	ownerID int64,
	recoveries []domain.WhoopRecoverySnapshot,
	sleeps []domain.WhoopSleepSnapshot,
) error {
	if ownerID <= 0 {
		return fmt.Errorf("invalid WHOOP owner_id %d", ownerID)
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin WHOOP health sync: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	lockKey := "whoop-health:" + strconv.FormatInt(ownerID, 10)
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`, lockKey); err != nil {
		return fmt.Errorf("lock WHOOP health sync: %w", err)
	}
	for _, recovery := range recoveries {
		if recovery.EntryDate.IsZero() || recovery.CycleID <= 0 {
			return fmt.Errorf("invalid WHOOP recovery cycle %d", recovery.CycleID)
		}
		externalID := "sync:recovery:" + strconv.FormatInt(recovery.CycleID, 10)
		if _, err := tx.Exec(ctx, `INSERT INTO recovery_entries (
			owner_id,entry_date,recovery_score,hrv_ms,resting_heart_rate_bpm,
			respiratory_rate,spo2_percent,skin_temperature_c,daily_strain,
			source,external_id,notes)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,'whoop',$10,'')
			ON CONFLICT (owner_id,source,external_id) WHERE external_id IS NOT NULL
			DO UPDATE SET entry_date=EXCLUDED.entry_date,recovery_score=EXCLUDED.recovery_score,
				hrv_ms=EXCLUDED.hrv_ms,resting_heart_rate_bpm=EXCLUDED.resting_heart_rate_bpm,
				respiratory_rate=EXCLUDED.respiratory_rate,spo2_percent=EXCLUDED.spo2_percent,
				skin_temperature_c=EXCLUDED.skin_temperature_c,daily_strain=EXCLUDED.daily_strain,
				updated_at=now()`,
			ownerID, recovery.EntryDate.Format("2006-01-02"), recovery.RecoveryScore,
			recovery.HRVMs, recovery.RestingHeartRateBPM, recovery.RespiratoryRate,
			recovery.SpO2Percent, recovery.SkinTemperatureC, recovery.DailyStrain, externalID,
		); err != nil {
			return fmt.Errorf("upsert WHOOP recovery %d: %w", recovery.CycleID, err)
		}
	}
	for _, sleep := range sleeps {
		sleepID := strings.TrimSpace(sleep.ExternalID)
		if sleep.SleepDate.IsZero() || sleepID == "" || sleep.Start.IsZero() || sleep.End.IsZero() || sleep.End.Before(sleep.Start) {
			return fmt.Errorf("invalid WHOOP sleep %q", sleep.ExternalID)
		}
		externalID := "sync:sleep:" + sleepID
		if _, err := tx.Exec(ctx, `INSERT INTO sleep_entries (
			owner_id,sleep_date,sleep_start,sleep_end,is_nap,time_in_bed_seconds,
			actual_sleep_seconds,awake_seconds,rem_seconds,deep_seconds,light_seconds,
			sleep_performance_percent,efficiency_percent,consistency_percent,
			sleep_debt_seconds,disturbances,source,external_id,notes)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,'whoop',$17,'')
			ON CONFLICT (owner_id,source,external_id) WHERE external_id IS NOT NULL
			DO UPDATE SET sleep_date=EXCLUDED.sleep_date,sleep_start=EXCLUDED.sleep_start,
				sleep_end=EXCLUDED.sleep_end,is_nap=EXCLUDED.is_nap,
				time_in_bed_seconds=EXCLUDED.time_in_bed_seconds,
				actual_sleep_seconds=EXCLUDED.actual_sleep_seconds,awake_seconds=EXCLUDED.awake_seconds,
				rem_seconds=EXCLUDED.rem_seconds,deep_seconds=EXCLUDED.deep_seconds,
				light_seconds=EXCLUDED.light_seconds,
				sleep_performance_percent=EXCLUDED.sleep_performance_percent,
				efficiency_percent=EXCLUDED.efficiency_percent,
				consistency_percent=EXCLUDED.consistency_percent,
				sleep_debt_seconds=EXCLUDED.sleep_debt_seconds,
				disturbances=EXCLUDED.disturbances,updated_at=now()`,
			ownerID, sleep.SleepDate.Format("2006-01-02"), sleep.Start, sleep.End, sleep.IsNap,
			sleep.TimeInBedSeconds, sleep.ActualSleepSeconds, sleep.AwakeSeconds,
			sleep.REMSeconds, sleep.DeepSeconds, sleep.LightSeconds,
			sleep.SleepPerformancePct, sleep.EfficiencyPct, sleep.ConsistencyPct,
			sleep.SleepDebtSeconds, sleep.Disturbances, externalID,
		); err != nil {
			return fmt.Errorf("upsert WHOOP sleep %s: %w", sleepID, err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit WHOOP health sync: %w", err)
	}
	return nil
}
