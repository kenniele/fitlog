package whoop

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	"fitlog/internal/domain"
)

const MaxBackfillDays = 366

// HealthBackfillSink atomically persists a fully fetched WHOOP batch.
type HealthBackfillSink interface {
	UpsertWhoopHealth(context.Context, int64, []domain.WhoopRecoverySnapshot, []domain.WhoopSleepSnapshot) error
}

type BackfillOptions struct {
	From     time.Time
	To       time.Time
	Location *time.Location
	DryRun   bool
}

type BackfillResult struct {
	From              string
	To                string
	RequestedDays     int
	FetchedCycles     int
	FetchedRecoveries int
	FetchedSleeps     int
	RecoveryRows      int
	SleepRows         int
	UnmatchedRecovery int
	DryRun            bool
}

// BackfillHealth fetches every provider collection before writing anything.
// Recovery is dated by wake-up day and enriched with the related cycle strain
// and sleep respiratory rate. A provider/pagination failure cannot therefore
// leave a partially refreshed history in PostgreSQL.
func BackfillHealth(
	ctx context.Context,
	source API,
	sink HealthBackfillSink,
	ownerID int64,
	options BackfillOptions,
) (BackfillResult, error) {
	if source == nil {
		return BackfillResult{}, errors.New("WHOOP backfill source is required")
	}
	if !options.DryRun && sink == nil {
		return BackfillResult{}, errors.New("WHOOP backfill sink is required")
	}
	loc := options.Location
	if loc == nil {
		loc = time.UTC
	}
	from := whoopCalendarDay(options.From, loc)
	to := whoopCalendarDay(options.To, loc)
	if from.After(to) {
		return BackfillResult{}, errors.New("invalid WHOOP range: from is after to")
	}
	days := whoopInclusiveDayCount(from, to)
	if days < 1 || days > MaxBackfillDays {
		return BackfillResult{}, fmt.Errorf("invalid WHOOP range: use 1-%d days", MaxBackfillDays)
	}

	result := BackfillResult{
		From: from.Format("2006-01-02"), To: to.Format("2006-01-02"),
		RequestedDays: days, DryRun: options.DryRun,
	}
	// Include the prior local day because a sleep ending on `from` normally
	// starts the previous evening. Snapshots are filtered by wake-up date below.
	rng := domain.TimeRange{From: from.AddDate(0, 0, -1), To: to.AddDate(0, 0, 1)}
	cycles, err := source.Cycles(ctx, rng, 25)
	if err != nil {
		return BackfillResult{}, fmt.Errorf("fetch WHOOP cycles: %w", err)
	}
	recoveries, err := source.Recoveries(ctx, rng, 25)
	if err != nil {
		return BackfillResult{}, fmt.Errorf("fetch WHOOP recoveries: %w", err)
	}
	sleeps, err := source.Sleeps(ctx, rng, 25)
	if err != nil {
		return BackfillResult{}, fmt.Errorf("fetch WHOOP sleeps: %w", err)
	}
	result.FetchedCycles = len(cycles)
	result.FetchedRecoveries = len(recoveries)
	result.FetchedSleeps = len(sleeps)

	cycleByID := make(map[int64]domain.Cycle, len(cycles))
	for _, cycle := range cycles {
		cycleByID[cycle.ID] = cycle
	}
	sleepByID := make(map[string]domain.Sleep, len(sleeps))
	for _, sleep := range sleeps {
		sleepByID[sleep.ExternalID] = sleep
	}

	sleepSnapshots := make([]domain.WhoopSleepSnapshot, 0, len(sleeps))
	for _, sleep := range sleeps {
		day := whoopCalendarDay(sleep.End, loc)
		if sleep.ExternalID == "" || sleep.Start.IsZero() || sleep.End.IsZero() || day.Before(from) || day.After(to) {
			continue
		}
		snapshot := domain.WhoopSleepSnapshot{
			SleepDate: day, ExternalID: sleep.ExternalID, Start: sleep.Start,
			End: sleep.End, IsNap: sleep.IsNap,
		}
		if sleep.ScoreState == "SCORED" {
			snapshot.TimeInBedSeconds = whoopInt64Pointer(whoopSeconds(sleep.Stages.InBedMs))
			snapshot.ActualSleepSeconds = whoopInt64Pointer(whoopSeconds(
				sleep.Stages.LightMs + sleep.Stages.SWSMs + sleep.Stages.REMMs,
			))
			snapshot.AwakeSeconds = whoopInt64Pointer(whoopSeconds(sleep.Stages.AwakeMs))
			snapshot.REMSeconds = whoopInt64Pointer(whoopSeconds(sleep.Stages.REMMs))
			snapshot.DeepSeconds = whoopInt64Pointer(whoopSeconds(sleep.Stages.SWSMs))
			snapshot.LightSeconds = whoopInt64Pointer(whoopSeconds(sleep.Stages.LightMs))
			snapshot.SleepPerformancePct = whoopFloat64Pointer(sleep.SleepPerformancePct)
			snapshot.EfficiencyPct = whoopFloat64Pointer(sleep.SleepEfficiencyPct)
			snapshot.ConsistencyPct = whoopFloat64Pointer(sleep.SleepConsistencyPct)
			snapshot.SleepDebtSeconds = whoopInt64Pointer(whoopSeconds(sleep.SleepNeed.FromDebtMs))
			snapshot.Disturbances = whoopIntPointer(max(sleep.DisturbanceCount, 0))
		}
		sleepSnapshots = append(sleepSnapshots, snapshot)
	}

	recoverySnapshots := make([]domain.WhoopRecoverySnapshot, 0, len(recoveries))
	for _, recovery := range recoveries {
		if recovery.ScoreState != "SCORED" || recovery.CycleID == 0 {
			continue
		}
		sleep, hasSleep := sleepByID[recovery.SleepID]
		cycle, hasCycle := cycleByID[recovery.CycleID]
		var instant time.Time
		switch {
		case hasSleep:
			instant = sleep.End
		case hasCycle:
			instant = cycle.Start
		default:
			result.UnmatchedRecovery++
			continue
		}
		day := whoopCalendarDay(instant, loc)
		if day.Before(from) || day.After(to) {
			continue
		}
		snapshot := domain.WhoopRecoverySnapshot{
			EntryDate: day, CycleID: recovery.CycleID,
			RecoveryScore:    whoopFloat64Pointer(recovery.Score),
			HRVMs:            whoopFloat64Pointer(recovery.HRVMilli),
			SpO2Percent:      recovery.SpO2Pct,
			SkinTemperatureC: recovery.SkinTempC,
		}
		if recovery.RestingHR > 0 {
			snapshot.RestingHeartRateBPM = whoopFloat64Pointer(recovery.RestingHR)
		}
		if hasSleep && sleep.ScoreState == "SCORED" && sleep.RespiratoryRate > 0 {
			snapshot.RespiratoryRate = whoopFloat64Pointer(sleep.RespiratoryRate)
		}
		if hasCycle && cycle.ScoreState == "SCORED" {
			snapshot.DailyStrain = whoopFloat64Pointer(cycle.Strain)
		}
		recoverySnapshots = append(recoverySnapshots, snapshot)
	}

	sort.Slice(sleepSnapshots, func(i, j int) bool {
		if sleepSnapshots[i].SleepDate.Equal(sleepSnapshots[j].SleepDate) {
			return sleepSnapshots[i].Start.Before(sleepSnapshots[j].Start)
		}
		return sleepSnapshots[i].SleepDate.Before(sleepSnapshots[j].SleepDate)
	})
	sort.Slice(recoverySnapshots, func(i, j int) bool {
		return recoverySnapshots[i].EntryDate.Before(recoverySnapshots[j].EntryDate)
	})
	result.RecoveryRows = len(recoverySnapshots)
	result.SleepRows = len(sleepSnapshots)
	if options.DryRun {
		return result, nil
	}
	if err := sink.UpsertWhoopHealth(ctx, ownerID, recoverySnapshots, sleepSnapshots); err != nil {
		return BackfillResult{}, fmt.Errorf("persist WHOOP health: %w", err)
	}
	return result, nil
}

func whoopCalendarDay(value time.Time, loc *time.Location) time.Time {
	value = value.In(loc)
	return time.Date(value.Year(), value.Month(), value.Day(), 0, 0, 0, 0, loc)
}

func whoopInclusiveDayCount(from, to time.Time) int {
	count := 0
	for day := from; !day.After(to); day = day.AddDate(0, 0, 1) {
		count++
	}
	return count
}

func whoopSeconds(milliseconds int64) int64 {
	if milliseconds <= 0 {
		return 0
	}
	return milliseconds / 1000
}

func whoopFloat64Pointer(value float64) *float64 { return &value }
func whoopInt64Pointer(value int64) *int64       { return &value }
func whoopIntPointer(value int) *int             { return &value }
