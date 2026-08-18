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

func TestYesterdayRequestUsesPreviousCalendarDay(t *testing.T) {
	loc := time.FixedZone("UTC+3", 3*60*60)
	now := time.Date(2026, 7, 22, 0, 30, 0, 0, loc)
	request := Yesterday(now, loc)
	require.Equal(t, "2026-07-21", request.From.Format("2006-01-02"))
	require.Equal(t, "2026-07-22", request.To.Format("2006-01-02"))
}

func TestUseCaseDailyFallsBackToMonthlyTotals(t *testing.T) {
	day := time.Date(2026, 8, 17, 12, 0, 0, 0, time.UTC)
	source := reportSource{days: []domain.DailyNutrition{
		{DateInt: ToDateInt(day), Calories: 2345, Protein: 210, Fat: 68, Carbs: 220},
	}}
	u := NewUseCase(source, time.UTC)

	output, err := u.Execute(context.Background(), Day(day, time.UTC))
	require.NoError(t, err)
	require.Contains(t, output, "*Итого:* 2345 kcal · Б 210 · Ж 68 · У 220")
	require.Contains(t, output, "без детализации приёмов пищи")
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

func TestNutritionAnalysisCalculatesDeficitAndWeeklyWeightChange(t *testing.T) {
	now := time.Date(2026, 7, 22, 12, 0, 0, 0, time.UTC)
	request := NutritionAnalysis(now, time.UTC)
	source := reportSource{days: []domain.DailyNutrition{
		{DateInt: ToDateInt(request.From), Calories: 1800, Protein: 140, Fat: 60, Carbs: 180},
		{DateInt: ToDateInt(request.From.AddDate(0, 0, 1)), Calories: 2000, Protein: 150, Fat: 70, Carbs: 200},
	}}
	u := NewUseCase(source, time.UTC, ReportOptions{EstimatedTDEE: 2620})

	report := u.Transform(FetchedReport{Request: request, Days: source.days})
	require.NotNil(t, report.Analysis)
	require.InDelta(t, 720, report.Analysis.Deficit, 0.01)
	require.InDelta(t, 145, report.Analysis.Protein, 0.01)

	output := u.Format(report)
	require.Contains(t, output, "средний дефицит — *720 ккал*")
	require.Contains(t, output, "*0\\.65 кг/неделю*")
	require.Contains(t, output, "*145 г*")
}
