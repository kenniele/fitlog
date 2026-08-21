package controlcenter

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"
)

func TestSaveSettingsValidatesThemeTargetsAndRanges(t *testing.T) {
	positive := 100.0
	minimum, maximum := 8*60*60, 7*60*60
	tests := []struct {
		name     string
		settings Settings
		field    string
	}{
		{
			name:     "theme",
			settings: Settings{Timezone: "UTC", Units: "metric", Theme: "neon", FirstDayOfWeek: 1},
			field:    "theme",
		},
		{
			name:     "target",
			settings: Settings{Timezone: "UTC", Units: "metric", Theme: "dark", FirstDayOfWeek: 1, CalorieTargetKcal: pointer(-positive)},
			field:    "calorie_target_kcal",
		},
		{
			name:     "sleep range",
			settings: Settings{Timezone: "UTC", Units: "metric", Theme: "dark", FirstDayOfWeek: 1, SleepTargetMinSeconds: &minimum, SleepTargetMaxSeconds: &maximum},
			field:    "sleep_target_max_seconds",
		},
		{
			name:     "recovery ranges",
			settings: Settings{Timezone: "UTC", Units: "metric", Theme: "dark", FirstDayOfWeek: 1, RecoveryRanges: json.RawMessage(`[]`)},
			field:    "recovery_ranges",
		},
	}
	service := NewService(&handlerStore{}, 42, time.UTC)
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := service.SaveSettings(context.Background(), test.settings)
			var validation *ValidationError
			if !errors.As(err, &validation) || validation.Fields[test.field] == "" {
				t.Fatalf("SaveSettings error = %#v, want field %q", err, test.field)
			}
		})
	}
}

func TestSaveSettingsAcceptsLightTheme(t *testing.T) {
	service := NewService(&handlerStore{}, 42, time.UTC)
	settings, err := service.SaveSettings(context.Background(), Settings{
		Timezone: "UTC", Units: "metric", Theme: "light", FirstDayOfWeek: 1,
		RecoveryRanges: json.RawMessage(`{"low":35}`),
	})
	if err != nil {
		t.Fatal(err)
	}
	if settings.Theme != "light" {
		t.Fatalf("theme = %q", settings.Theme)
	}
}

func TestValidatePlanExerciseRequiresCompleteProgression(t *testing.T) {
	workingSets, minReps, maxReps := 3, 6, 10
	exercise := planExerciseInput{WorkingSets: &workingSets, MinReps: &minReps, MaxReps: &maxReps}
	err := validatePlanExercise(exercise)
	var validation *ValidationError
	if !errors.As(err, &validation) || validation.Fields["templates.exercises.weight_step_kg"] == "" {
		t.Fatalf("validation error = %#v", err)
	}

	step := 2.5
	exercise.WeightStepKG = &step
	exercise.WarmupSets = []planWarmupSetInput{{WeightMode: "bar", Reps: 10}, {WeightMode: "kg", WeightKG: pointer(30), Reps: 6}}
	if err := validatePlanExercise(exercise); err != nil {
		t.Fatalf("complete prescription rejected: %v", err)
	}
}

func TestValidateSessionExerciseRejectsNegativeRestAfterExercise(t *testing.T) {
	rest := -1
	err := validateSessionExercise(sessionExerciseInput{RestAfterExerciseSeconds: &rest})
	var validation *ValidationError
	if !errors.As(err, &validation) || validation.Fields["exercises.rest_after_exercise_seconds"] == "" {
		t.Fatalf("validation error = %#v", err)
	}

	rest = 0
	if err := validateSessionExercise(sessionExerciseInput{RestAfterExerciseSeconds: &rest}); err != nil {
		t.Fatalf("zero rest rejected: %v", err)
	}
}

func TestAnalyticsComparisonLoadsPreviousEquivalentRange(t *testing.T) {
	var requested []DateRange
	store := &handlerStore{overviewFn: func(_ context.Context, _ int64, dateRange DateRange, _ *time.Location) (Overview, error) {
		requested = append(requested, dateRange)
		value := float64(dateRange.To.Day())
		return Overview{
			Range:   rangeView(dateRange, time.UTC),
			Daily:   []DailyPoint{{Date: dateRange.To.Format("2006-01-02"), RecoveryScore: &value}},
			Summary: DashboardSummary{Recovery: RecoverySummary{Recovery: MetricSummary{Current: &value, Samples: 1}}},
		}, nil
	}}
	service := NewService(store, 42, time.UTC)
	dateRange := DateRange{
		From: time.Date(2026, 8, 15, 0, 0, 0, 0, time.UTC),
		To:   time.Date(2026, 8, 21, 0, 0, 0, 0, time.UTC), Compare: true,
	}
	result, err := service.Analytics(context.Background(), "recovery", dateRange)
	if err != nil {
		t.Fatal(err)
	}
	payload, ok := result.(map[string]any)
	if !ok || payload["comparison"] == nil {
		t.Fatalf("analytics comparison missing: %#v", result)
	}
	if len(requested) != 2 || !requested[1].From.Equal(time.Date(2026, 8, 8, 0, 0, 0, 0, time.UTC)) ||
		!requested[1].To.Equal(time.Date(2026, 8, 14, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("requested ranges = %#v", requested)
	}
}

func TestAnalyticsCorrelationsLoadsNextDayCalendarsWithoutExpandingPayload(t *testing.T) {
	var requested []DateRange
	store := &handlerStore{overviewFn: func(_ context.Context, _ int64, dateRange DateRange, _ *time.Location) (Overview, error) {
		requested = append(requested, dateRange)
		daily := make([]DailyPoint, 0, dateRange.Days())
		for day := dateRange.From; !day.After(dateRange.To); day = day.AddDate(0, 0, 1) {
			value := float64(day.Day())
			sleep := int64(7 * time.Hour / time.Second)
			daily = append(daily, DailyPoint{
				Date:          day.Format("2006-01-02"),
				RecoveryScore: &value,
				DailyStrain:   &value,
				SleepSeconds:  &sleep,
				CaloriesKcal:  &value,
				WeightKG:      &value,
				ProteinG:      &value,
				LeanMassKG:    &value,
				HRVMs:         &value,
				AverageRIR:    &value,
			})
		}
		return Overview{Range: rangeView(dateRange, time.UTC), Daily: daily}, nil
	}}
	service := NewService(store, 42, time.UTC)
	dateRange := DateRange{
		From:    time.Date(2026, 8, 15, 0, 0, 0, 0, time.UTC),
		To:      time.Date(2026, 8, 21, 0, 0, 0, 0, time.UTC),
		Compare: true,
	}

	result, err := service.Analytics(context.Background(), "correlations", dateRange)
	if err != nil {
		t.Fatal(err)
	}
	payload, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("analytics payload = %#v", result)
	}
	assertCorrelationPeriod := func(label string, section map[string]any, wantFrom, wantTo string) {
		t.Helper()
		daily, ok := section["daily"].([]DailyPoint)
		if !ok || len(daily) != 7 || daily[0].Date != wantFrom || daily[len(daily)-1].Date != wantTo {
			t.Fatalf("%s daily = %#v, want seven points from %s through %s", label, section["daily"], wantFrom, wantTo)
		}
		correlations, ok := section["correlations"].([]map[string]any)
		if !ok || len(correlations) != 5 {
			t.Fatalf("%s correlations = %#v", label, section["correlations"])
		}
		for _, correlation := range correlations {
			if correlation["from"] != wantFrom || correlation["to"] != wantTo || correlation["sample_size"] != 7 {
				t.Fatalf("%s correlation = %#v, want original period and seven samples", label, correlation)
			}
		}
	}
	assertCorrelationPeriod("current", payload, "2026-08-15", "2026-08-21")
	comparison, ok := payload["comparison"].(map[string]any)
	if !ok {
		t.Fatalf("comparison = %#v", payload["comparison"])
	}
	assertCorrelationPeriod("comparison", comparison, "2026-08-08", "2026-08-14")

	if len(requested) != 4 {
		t.Fatalf("requested ranges = %#v, want current/calendar and previous/calendar", requested)
	}
	want := []struct{ from, to string }{
		{"2026-08-15", "2026-08-21"},
		{"2026-08-15", "2026-08-22"},
		{"2026-08-08", "2026-08-14"},
		{"2026-08-08", "2026-08-15"},
	}
	for index, dateRange := range requested {
		if gotFrom, gotTo := dateRange.From.Format("2006-01-02"), dateRange.To.Format("2006-01-02"); gotFrom != want[index].from || gotTo != want[index].to {
			t.Fatalf("requested[%d] = %s through %s, want %s through %s", index, gotFrom, gotTo, want[index].from, want[index].to)
		}
		if dateRange.Compare {
			t.Fatalf("requested[%d] unexpectedly retained comparison flag", index)
		}
	}
}

func TestStartOfWeekUsesConfiguredFirstDay(t *testing.T) {
	friday := time.Date(2026, 8, 21, 12, 0, 0, 0, time.UTC)
	if got := startOfWeek(friday, 1); got.Format("2006-01-02") != "2026-08-17" {
		t.Fatalf("monday week start = %s", got)
	}
	if got := startOfWeek(friday, 7); got.Format("2006-01-02") != "2026-08-16" {
		t.Fatalf("sunday week start = %s", got)
	}
}

func pointer(value float64) *float64 { return &value }
