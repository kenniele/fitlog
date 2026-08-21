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
	months map[string][]domain.DailyNutrition
	errAt  string
	calls  []string
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
