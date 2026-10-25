package fatsecret

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"fitlog/internal/domain"
)

type fakeBackfillSource struct {
	months   map[string][]domain.DailyNutrition
	errAt    string
	calls    []string
	entries  map[string][]domain.MealEntry
	dayCalls []string
	errDay   string
}

func (f *fakeBackfillSource) FoodEntriesForDay(_ context.Context, day time.Time) ([]domain.MealEntry, error) {
	key := day.Format("2006-01-02")
	f.dayCalls = append(f.dayCalls, key)
	if key == f.errDay {
		return nil, errors.New("daily provider unavailable")
	}
	return f.entries[key], nil
}

func (f *fakeBackfillSource) FoodEntriesMonth(_ context.Context, month time.Time) ([]domain.DailyNutrition, error) {
	key := month.Format("2006-01")
	f.calls = append(f.calls, key)
	if key == f.errAt {
		return nil, errors.New("provider unavailable")
	}
	return f.months[key], nil
}

type fakeBackfillSink struct {
	calls   int
	ownerID int64
	days    []domain.NutritionDaySnapshot
}

func (f *fakeBackfillSink) UpsertFatSecretNutritionDays(_ context.Context, ownerID int64, days []domain.NutritionDaySnapshot) error {
	f.calls++
	f.ownerID = ownerID
	f.days = append([]domain.NutritionDaySnapshot(nil), days...)
	return nil
}

func TestBackfillNutritionDaysFiltersDeduplicatesAndPersistsOnce(t *testing.T) {
	loc, err := time.LoadLocation("Europe/Moscow")
	require.NoError(t, err)
	from := time.Date(2026, 7, 30, 19, 0, 0, 0, loc)
	to := time.Date(2026, 8, 2, 8, 0, 0, 0, loc)
	jul29 := ToDateInt(time.Date(2026, 7, 29, 0, 0, 0, 0, loc))
	jul30 := ToDateInt(time.Date(2026, 7, 30, 0, 0, 0, 0, loc))
	aug1 := ToDateInt(time.Date(2026, 8, 1, 0, 0, 0, 0, loc))
	aug3 := ToDateInt(time.Date(2026, 8, 3, 0, 0, 0, 0, loc))
	source := &fakeBackfillSource{months: map[string][]domain.DailyNutrition{
		"2026-07": {
			{DateInt: jul29, Calories: 999},
			{DateInt: jul30, Calories: 2100, Protein: 150, Fat: 70, Carbs: 220},
		},
		"2026-08": {
			{DateInt: aug1, Calories: 1900, Protein: 140, Fat: 60, Carbs: 200},
			{DateInt: aug1, Calories: 1950, Protein: 145, Fat: 62, Carbs: 205},
			{DateInt: aug3, Calories: 999},
		},
	}}
	sink := &fakeBackfillSink{}

	result, err := BackfillNutritionDays(context.Background(), source, sink, 42, BackfillOptions{
		From: from, To: to, Location: loc, StorageAuthorized: true,
	})
	require.NoError(t, err)
	require.Equal(t, []string{"2026-07", "2026-08"}, source.calls)
	require.Equal(t, 4, result.RequestedDays)
	require.Equal(t, 2, result.RequestedMonths)
	require.Equal(t, 2, result.LoggedDays)
	require.Equal(t, 2, result.UpsertedDays)
	require.Equal(t, "2026-08-01", result.LatestAvailableDate)
	require.Equal(t, 1, sink.calls)
	require.EqualValues(t, 42, sink.ownerID)
	require.Len(t, sink.days, 2)
	require.Equal(t, jul30, sink.days[0].DateInt)
	require.Equal(t, 1950.0, *sink.days[1].CaloriesKcal)
	require.Nil(t, sink.days[1].FiberG)
}

func TestBackfillNutritionDaysDryRunDoesNotWrite(t *testing.T) {
	day := time.Date(2026, 8, 20, 0, 0, 0, 0, time.UTC)
	source := &fakeBackfillSource{months: map[string][]domain.DailyNutrition{
		"2026-08": {{DateInt: ToDateInt(day), Calories: 2000}},
	}}
	sink := &fakeBackfillSink{}

	result, err := BackfillNutritionDays(context.Background(), source, sink, 42, BackfillOptions{
		From: day, To: day, Location: time.UTC, DryRun: true,
	})
	require.NoError(t, err)
	require.True(t, result.DryRun)
	require.Equal(t, 1, result.LoggedDays)
	require.Zero(t, result.UpsertedDays)
	require.Zero(t, sink.calls)
}

func TestBackfillNutritionDaysFiber(t *testing.T) {
	day := time.Date(2026, 10, 24, 0, 0, 0, 0, time.FixedZone("UTC+3", 3*60*60))
	dateInt := ToDateInt(day)
	for _, tt := range []struct {
		name       string
		fibers     []*float64
		want       *float64
		incomplete bool
	}{
		{name: "sum", fibers: []*float64{floatPointer(4.5), floatPointer(3)}, want: floatPointer(7.5)},
		{name: "zero", fibers: []*float64{floatPointer(0)}, want: floatPointer(0)},
		{name: "empty diary", fibers: []*float64{}},
		{name: "all unknown", fibers: []*float64{nil, nil}, incomplete: true},
		{name: "partial sum is marked", fibers: []*float64{floatPointer(4.5), nil}, want: floatPointer(4.5), incomplete: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			entries := make([]domain.MealEntry, 0, len(tt.fibers))
			for _, fiber := range tt.fibers {
				entries = append(entries, domain.MealEntry{DateInt: dateInt, Fiber: fiber})
			}
			source := &fakeBackfillSource{
				months:  map[string][]domain.DailyNutrition{"2026-10": {{DateInt: dateInt, Calories: 2000}}},
				entries: map[string][]domain.MealEntry{"2026-10-24": entries},
			}
			sink := &fakeBackfillSink{}
			result, err := BackfillNutritionDays(context.Background(), source, sink, 42, BackfillOptions{
				From: day, To: day, Location: day.Location(), StorageAuthorized: true,
			})
			require.NoError(t, err)
			require.Equal(t, []string{"2026-10-24"}, source.dayCalls)
			require.Len(t, sink.days, 1)
			require.Equal(t, tt.want, sink.days[0].FiberG)
			require.Equal(t, tt.incomplete, sink.days[0].FiberIncomplete)
			require.Equal(t, 2000.0, *sink.days[0].CaloriesKcal)
			if tt.want == nil {
				require.Zero(t, result.FiberDays)
			} else {
				require.Equal(t, 1, result.FiberDays)
			}
		})
	}
}

func TestBackfillNutritionDaysDailyFailureLeavesSinkUntouched(t *testing.T) {
	day := time.Date(2026, 10, 24, 0, 0, 0, 0, time.UTC)
	for _, wrongDate := range []bool{false, true} {
		name := "provider error"
		if wrongDate {
			name = "wrong entry date"
		}
		t.Run(name, func(t *testing.T) {
			source := &fakeBackfillSource{
				months: map[string][]domain.DailyNutrition{"2026-10": {
					{DateInt: ToDateInt(day.AddDate(0, 0, -1)), Calories: 1900},
					{DateInt: ToDateInt(day), Calories: 2000},
				}},
				errDay: "2026-10-24",
			}
			if wrongDate {
				source.errDay = ""
				source.entries = map[string][]domain.MealEntry{"2026-10-24": {{DateInt: ToDateInt(day) - 1}}}
			}
			sink := &fakeBackfillSink{}
			_, err := BackfillNutritionDays(context.Background(), source, sink, 42, BackfillOptions{
				From: day.AddDate(0, 0, -1), To: day, Location: time.UTC, StorageAuthorized: true,
			})
			require.ErrorContains(t, err, "2026-10-24")
			require.Zero(t, sink.calls)
		})
	}
}

func TestBackfillNutritionDaysRequiresStoragePermissionBeforeFetching(t *testing.T) {
	day := time.Date(2026, 8, 20, 0, 0, 0, 0, time.UTC)
	source := &fakeBackfillSource{}

	_, err := BackfillNutritionDays(context.Background(), source, &fakeBackfillSink{}, 42, BackfillOptions{
		From: day, To: day, Location: time.UTC,
	})
	require.ErrorIs(t, err, ErrStoragePermissionRequired)
	require.Empty(t, source.calls)
}

func TestBackfillNutritionDaysProviderFailureLeavesSinkUntouched(t *testing.T) {
	from := time.Date(2026, 7, 30, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 8, 2, 0, 0, 0, 0, time.UTC)
	source := &fakeBackfillSource{months: map[string][]domain.DailyNutrition{
		"2026-07": {{DateInt: ToDateInt(from), Calories: 2000}},
	}, errAt: "2026-08"}
	sink := &fakeBackfillSink{}

	_, err := BackfillNutritionDays(context.Background(), source, sink, 42, BackfillOptions{
		From: from, To: to, Location: time.UTC, StorageAuthorized: true,
	})
	require.ErrorContains(t, err, "2026-08")
	require.Zero(t, sink.calls)
}

func TestBackfillNutritionDaysRejectsOversizedRange(t *testing.T) {
	from := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	to := from.AddDate(0, 0, MaxBackfillDays)
	_, err := BackfillNutritionDays(context.Background(), &fakeBackfillSource{}, nil, 42, BackfillOptions{
		From: from, To: to, Location: time.UTC, DryRun: true,
	})
	require.ErrorContains(t, err, "1-366")
}
