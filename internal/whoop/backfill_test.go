package whoop

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"fitlog/internal/domain"
)

type backfillAPI struct {
	cycles     []domain.Cycle
	recoveries []domain.Recovery
	sleeps     []domain.Sleep
	errAt      string
	ranges     []domain.TimeRange
	limits     []int
}

func (a *backfillAPI) Cycles(_ context.Context, rng domain.TimeRange, limit int) ([]domain.Cycle, error) {
	a.ranges = append(a.ranges, rng)
	a.limits = append(a.limits, limit)
	if a.errAt == "cycles" {
		return nil, errors.New("cycles unavailable")
	}
	return a.cycles, nil
}

func (a *backfillAPI) Recoveries(_ context.Context, rng domain.TimeRange, limit int) ([]domain.Recovery, error) {
	a.ranges = append(a.ranges, rng)
	a.limits = append(a.limits, limit)
	if a.errAt == "recoveries" {
		return nil, errors.New("recoveries unavailable")
	}
	return a.recoveries, nil
}

func (a *backfillAPI) Sleeps(_ context.Context, rng domain.TimeRange, limit int) ([]domain.Sleep, error) {
	a.ranges = append(a.ranges, rng)
	a.limits = append(a.limits, limit)
	if a.errAt == "sleeps" {
		return nil, errors.New("sleeps unavailable")
	}
	return a.sleeps, nil
}

func (a *backfillAPI) Workouts(context.Context, domain.TimeRange, int) ([]domain.Workout, error) {
	return nil, nil
}

type backfillSink struct {
	calls      int
	ownerID    int64
	recoveries []domain.WhoopRecoverySnapshot
	sleeps     []domain.WhoopSleepSnapshot
}

func (s *backfillSink) UpsertWhoopHealth(
	_ context.Context,
	ownerID int64,
	recoveries []domain.WhoopRecoverySnapshot,
	sleeps []domain.WhoopSleepSnapshot,
) error {
	s.calls++
	s.ownerID = ownerID
	s.recoveries = append([]domain.WhoopRecoverySnapshot(nil), recoveries...)
	s.sleeps = append([]domain.WhoopSleepSnapshot(nil), sleeps...)
	return nil
}

func TestBackfillHealthJoinsFiltersAndPersistsOnce(t *testing.T) {
	loc, err := time.LoadLocation("Europe/Moscow")
	require.NoError(t, err)
	from := time.Date(2026, 8, 19, 0, 0, 0, 0, loc)
	to := time.Date(2026, 8, 21, 0, 0, 0, 0, loc)
	wake := time.Date(2026, 8, 20, 7, 30, 0, 0, loc)
	start := time.Date(2026, 8, 19, 23, 0, 0, 0, loc)
	spo2 := 97.2
	skin := 33.4
	api := &backfillAPI{
		cycles: []domain.Cycle{{ID: 101, Start: wake, ScoreState: "SCORED", Strain: 12.4}},
		recoveries: []domain.Recovery{{
			CycleID: 101, SleepID: "sleep-101", ScoreState: "SCORED", Score: 76,
			HRVMilli: 58.3, RestingHR: 51, SpO2Pct: &spo2, SkinTempC: &skin,
		}},
		sleeps: []domain.Sleep{{
			ExternalID: "sleep-101", Start: start, End: wake, ScoreState: "SCORED",
			RespiratoryRate: 15.2, SleepPerformancePct: 88, SleepEfficiencyPct: 91,
			SleepConsistencyPct: 84, DisturbanceCount: 7,
			Stages:    domain.SleepStages{InBedMs: 30_000_000, AwakeMs: 1_000_000, LightMs: 14_000_000, SWSMs: 7_000_000, REMMs: 6_000_000},
			SleepNeed: domain.SleepNeed{FromDebtMs: 1_500_000},
		}},
	}
	sink := &backfillSink{}

	result, err := BackfillHealth(context.Background(), api, sink, 42, BackfillOptions{
		From: from, To: to, Location: loc,
	})
	require.NoError(t, err)
	require.Equal(t, 3, result.RequestedDays)
	require.Equal(t, 1, result.RecoveryRows)
	require.Equal(t, 1, result.SleepRows)
	require.Equal(t, 1, sink.calls)
	require.EqualValues(t, 42, sink.ownerID)
	require.Len(t, api.ranges, 3)
	require.Equal(t, from.AddDate(0, 0, -1), api.ranges[0].From)
	require.Equal(t, to.AddDate(0, 0, 1), api.ranges[0].To)
	require.Equal(t, []int{25, 25, 25}, api.limits)
	require.Equal(t, "2026-08-20", sink.recoveries[0].EntryDate.Format("2006-01-02"))
	require.Equal(t, 12.4, *sink.recoveries[0].DailyStrain)
	require.Equal(t, 15.2, *sink.recoveries[0].RespiratoryRate)
	require.EqualValues(t, 27_000, *sink.sleeps[0].ActualSleepSeconds)
	require.EqualValues(t, 1_500, *sink.sleeps[0].SleepDebtSeconds)
}

func TestBackfillHealthDryRunDoesNotWrite(t *testing.T) {
	day := time.Date(2026, 8, 21, 0, 0, 0, 0, time.UTC)
	api := &backfillAPI{}
	sink := &backfillSink{}
	result, err := BackfillHealth(context.Background(), api, sink, 42, BackfillOptions{
		From: day, To: day, Location: time.UTC, DryRun: true,
	})
	require.NoError(t, err)
	require.True(t, result.DryRun)
	require.Zero(t, sink.calls)
}

func TestBackfillHealthProviderFailureLeavesSinkUntouched(t *testing.T) {
	day := time.Date(2026, 8, 21, 0, 0, 0, 0, time.UTC)
	api := &backfillAPI{errAt: "recoveries"}
	sink := &backfillSink{}
	_, err := BackfillHealth(context.Background(), api, sink, 42, BackfillOptions{
		From: day, To: day, Location: time.UTC,
	})
	require.ErrorContains(t, err, "recoveries")
	require.Zero(t, sink.calls)
}

func TestBackfillHealthRejectsOversizedRange(t *testing.T) {
	from := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	to := from.AddDate(0, 0, MaxBackfillDays)
	_, err := BackfillHealth(context.Background(), &backfillAPI{}, nil, 42, BackfillOptions{
		From: from, To: to, Location: time.UTC, DryRun: true,
	})
	require.ErrorContains(t, err, "1-366")
}
