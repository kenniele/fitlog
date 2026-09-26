package controlcenter

import (
	"context"
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func bodyFatFixture() (bodyFatInputs, time.Time) {
	now := time.Date(2026, 9, 12, 12, 0, 0, 0, time.UTC)
	inputs := bodyFatInputs{Baseline: &bodyFatBaseline{
		MeasuredAt: time.Date(2026, 9, 1, 8, 0, 0, 0, time.UTC), WeightKG: pointer(100), FatMassKG: pointer(20),
	}}
	for day := 2; day <= 11; day++ {
		inputs.Days = append(inputs.Days, DailyPoint{
			Date: time.Date(2026, 9, day, 0, 0, 0, 0, time.UTC).Format("2006-01-02"), CaloriesKcal: pointer(1930),
		})
	}
	return inputs, now
}

func TestEstimateBodyFatDeficitAndSurplus(t *testing.T) {
	for _, tc := range []struct {
		name                           string
		calories, deficit, wantPercent float64
	}{
		{name: "deficit", calories: 1930, deficit: 7700, wantPercent: 19.0 / 99 * 100},
		{name: "maintenance", calories: 2700, wantPercent: 20},
		{name: "surplus", calories: 3470, deficit: -7700, wantPercent: 21.0 / 101 * 100},
	} {
		t.Run(tc.name, func(t *testing.T) {
			inputs, now := bodyFatFixture()
			for index := range inputs.Days {
				inputs.Days[index].CaloriesKcal = pointer(tc.calories)
			}
			// Boundary days must not enter the energy sum, including today's partial diary.
			inputs.Days = append(inputs.Days, DailyPoint{Date: "2026-09-01", CaloriesKcal: pointer(0)}, DailyPoint{Date: "2026-09-12", CaloriesKcal: pointer(0)})
			got := estimateBodyFat(inputs, 2700, now, time.UTC)
			require.Equal(t, "estimated", got.Status)
			require.Equal(t, 10, got.ObservedDays)
			require.Equal(t, 100.0, got.CoveragePercent)
			require.InDelta(t, tc.deficit, *got.EstimatedDeficitKcal, 0.001)
			require.InDelta(t, tc.wantPercent, *got.Percent, 0.0001)
			require.Less(t, *got.LowerPercent, *got.Percent)
			require.Greater(t, *got.UpperPercent, *got.Percent)
		})
	}
}

func TestEstimateBodyFatMissingDays(t *testing.T) {
	inputs, now := bodyFatFixture()
	full := estimateBodyFat(inputs, 2700, now, time.UTC)
	inputs.Days = inputs.Days[:8]
	got := estimateBodyFat(inputs, 2700, now, time.UTC)
	require.Equal(t, "estimated", got.Status)
	require.Equal(t, 2, got.MissingDays)
	require.Equal(t, 80.0, got.CoveragePercent)
	require.InDelta(t, 6160, *got.ObservedDeficitKcal, 0.001)
	require.InDelta(t, 7700, *got.EstimatedDeficitKcal, 0.001)
	require.Less(t, *got.LowerPercent, *full.LowerPercent)
	require.Greater(t, *got.UpperPercent, *full.UpperPercent)
	inputs.Days[0].CaloriesKcal = nil
	got = estimateBodyFat(inputs, 2700, now, time.UTC)
	require.Equal(t, "insufficient_nutrition", got.Status)
	require.Nil(t, got.Percent)
	require.Equal(t, 7, got.ObservedDays)
}

func TestEstimateBodyFatUnavailable(t *testing.T) {
	for _, tc := range []struct {
		name, status string
		change       func(*bodyFatInputs, *time.Time, *float64)
	}{
		{"missing baseline", "no_inbody", func(in *bodyFatInputs, _ *time.Time, _ *float64) { in.Baseline = nil }},
		{"incomplete baseline", "incomplete_inbody", func(in *bodyFatInputs, _ *time.Time, _ *float64) { in.Baseline.WeightKG = nil }},
		{"no TDEE", "no_tdee", func(_ *bodyFatInputs, _ *time.Time, tdee *float64) { *tdee = 0 }},
		{"nonfinite TDEE", "no_tdee", func(_ *bodyFatInputs, _ *time.Time, tdee *float64) { *tdee = math.Inf(1) }},
		{"no full days", "no_complete_days", func(in *bodyFatInputs, now *time.Time, _ *float64) { in.Baseline.MeasuredAt = now.Add(-time.Hour) }},
		{"old baseline", "stale_inbody", func(in *bodyFatInputs, _ *time.Time, _ *float64) {
			in.Baseline.MeasuredAt = in.Baseline.MeasuredAt.AddDate(-2, 0, 0)
		}},
		{"no nutrition", "insufficient_nutrition", func(in *bodyFatInputs, _ *time.Time, _ *float64) { in.Days = nil }},
		{"invalid result", "outside_model", func(_ *bodyFatInputs, _ *time.Time, tdee *float64) { *tdee = 100000 }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			inputs, now := bodyFatFixture()
			tdee := 2700.0
			tc.change(&inputs, &now, &tdee)
			got := estimateBodyFat(inputs, tdee, now, time.UTC)
			require.Equal(t, tc.status, got.Status)
			require.Nil(t, got.Percent)
			require.Nil(t, got.LowerPercent)
			require.Nil(t, got.UpperPercent)
		})
	}
}

func TestEstimateBodyFatUsesLocalDatesAndPercentageFallback(t *testing.T) {
	loc, err := time.LoadLocation("Europe/Moscow")
	require.NoError(t, err)
	now := time.Date(2026, 9, 4, 22, 0, 0, 0, time.UTC) // September 5 locally.
	inputs := bodyFatInputs{Baseline: &bodyFatBaseline{
		MeasuredAt: time.Date(2026, 9, 1, 22, 0, 0, 0, time.UTC), WeightKG: pointer(100), FatPercent: pointer(20),
	}, Days: []DailyPoint{{Date: "2026-09-03", CaloriesKcal: pointer(2700)}, {Date: "2026-09-04", CaloriesKcal: pointer(2700)}}}
	got := estimateBodyFat(inputs, 2700, now, loc)
	require.Equal(t, "estimated", got.Status)
	require.Equal(t, "2026-09-03", got.From)
	require.Equal(t, "2026-09-04", got.Through)
	require.Equal(t, 2, got.TotalDays)
	require.Equal(t, 20.0, *got.Percent)
}

type bodyFatStoreStub struct {
	handlerStore
	inputs bodyFatInputs
	owner  int64
	loc    *time.Location
}

func (s *bodyFatStoreStub) loadBodyFatInputs(_ context.Context, owner int64, _ time.Time, loc *time.Location) (bodyFatInputs, error) {
	s.owner, s.loc = owner, loc
	return s.inputs, nil
}

func TestBodyAnalyticsEstimateIgnoresChartRangeAndUsesRequestTimezone(t *testing.T) {
	inputs, now := bodyFatFixture()
	store := &bodyFatStoreStub{inputs: inputs}
	service := NewService(store, 42, time.UTC, WithEstimatedTDEE(2700))
	service.now = func() time.Time { return now }
	loc, err := time.LoadLocation("Europe/Moscow")
	require.NoError(t, err)
	ctx := contextWithLocation(context.Background(), loc)
	result, err := service.Analytics(ctx, "body", DateRange{From: now, To: now})
	require.NoError(t, err)
	estimate := result.(map[string]any)["body_fat_estimate"].(BodyFatEstimate)
	require.Equal(t, "2026-09-02", estimate.From)
	require.Equal(t, "2026-09-11", estimate.Through)
	require.Equal(t, int64(42), store.owner)
	require.Equal(t, loc, store.loc)
	require.Equal(t, 2700.0, estimate.TDEEKcal)
}
