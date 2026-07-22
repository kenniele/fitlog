package fatsecret

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"fitlog/internal/domain"
)

type reportSource struct {
	entries []domain.MealEntry
	days    []domain.DailyNutrition
}

func (s reportSource) FoodEntriesForDay(context.Context, time.Time) ([]domain.MealEntry, error) {
	return s.entries, nil
}
func (s reportSource) FoodEntriesMonth(context.Context, time.Time) ([]domain.DailyNutrition, error) {
	return s.days, nil
}

func TestUseCaseDailyPipeline(t *testing.T) {
	calories, protein, fat, carbs := 500.0, 30.0, 20.0, 40.0
	source := reportSource{entries: []domain.MealEntry{{
		Meal: domain.MealBreakfast, FoodName: "Омлет", Calories: &calories,
		Protein: &protein, Fat: &fat, Carbs: &carbs,
	}}}
	u := NewUseCase(source, time.UTC)
	day := time.Date(2026, 7, 22, 12, 0, 0, 0, time.UTC)

	output, err := u.Execute(context.Background(), Today(day, time.UTC))
	require.NoError(t, err)
	require.Contains(t, output, "*Питание · 22 июля 2026*")
	require.Contains(t, output, "*Итого:* 500 kcal")
	require.Contains(t, output, "Омлет")
}

func TestUseCaseSummaryUsesOnlyRequestedDays(t *testing.T) {
	from := time.Date(2026, 6, 22, 0, 0, 0, 0, time.UTC)
	source := reportSource{days: []domain.DailyNutrition{
		{DateInt: ToDateInt(from.AddDate(0, 0, -1)), Calories: 9999},
		{DateInt: ToDateInt(from), Calories: 1800, Protein: 120, Fat: 60, Carbs: 180},
		{DateInt: ToDateInt(from.AddDate(0, 0, 1)), Calories: 2000, Protein: 140, Fat: 70, Carbs: 200},
	}}
	u := NewUseCase(source, time.UTC)

	fetched, err := u.Fetch(context.Background(), ReportRequest{Mode: SummaryReport, From: from, To: from.AddDate(0, 0, 30)})
	require.NoError(t, err)
	report := u.Transform(fetched)
	require.Equal(t, 2, report.LoggedDays)
	require.InDelta(t, 1900, report.Calories, 0.01)
}
