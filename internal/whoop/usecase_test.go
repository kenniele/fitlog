package whoop

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"fitlog/internal/domain"
)

type reportProvider struct{ api API }

func (p reportProvider) Client(context.Context) (API, error) { return p.api, nil }

type reportAPI struct {
	cycles     []domain.Cycle
	recoveries []domain.Recovery
	sleeps     []domain.Sleep
	workouts   []domain.Workout
}

func (a reportAPI) Cycles(context.Context, domain.TimeRange, int) ([]domain.Cycle, error) {
	return a.cycles, nil
}
func (a reportAPI) Recoveries(context.Context, domain.TimeRange, int) ([]domain.Recovery, error) {
	return a.recoveries, nil
}
func (a reportAPI) Sleeps(context.Context, domain.TimeRange, int) ([]domain.Sleep, error) {
	return a.sleeps, nil
}
func (a reportAPI) Workouts(context.Context, domain.TimeRange, int) ([]domain.Workout, error) {
	return a.workouts, nil
}

func TestUseCaseDailyPipeline(t *testing.T) {
	day := time.Date(2026, 7, 22, 0, 0, 0, 0, time.UTC)
	sleep := domain.Sleep{
		ExternalID: "sleep-1", Start: day.Add(-time.Hour), End: day.Add(7 * time.Hour),
		SleepPerformancePct: 90, SleepEfficiencyPct: 92,
		Stages: domain.SleepStages{LightMs: int64(4 * time.Hour / time.Millisecond), SWSMs: int64(2 * time.Hour / time.Millisecond), REMMs: int64(90 * time.Minute / time.Millisecond)},
	}
	end := day.Add(20 * time.Hour)
	api := reportAPI{
		sleeps:     []domain.Sleep{sleep},
		recoveries: []domain.Recovery{{CycleID: 7, SleepID: "sleep-1", Score: 75, HRVMilli: 68, RestingHR: 51}, {CycleID: 6, Score: 65, HRVMilli: 60, RestingHR: 53}},
		cycles:     []domain.Cycle{{ID: 7, Start: day.Add(-time.Hour), End: &end, ScoreState: "SCORED", Strain: 12, AvgHR: 70, MaxHR: 150, Kilojoule: 2000}},
		workouts:   []domain.Workout{{SportName: "Running", Start: day.Add(10 * time.Hour), End: day.Add(11 * time.Hour), Strain: 10}},
	}
	u := NewUseCase(reportProvider{api: api}, time.UTC)

	output, err := u.Execute(context.Background(), Today(day.Add(time.Hour), time.UTC))
	require.NoError(t, err)
	require.Contains(t, output, "*Здоровье · 22 июля 2026*")
	require.Contains(t, output, "*Recovery* 75% 🟢")
	require.Contains(t, output, "Среднее за 1 день")
	require.Contains(t, output, "Running")
}

func TestUseCaseSummaryTransform(t *testing.T) {
	from := time.Date(2026, 6, 22, 0, 0, 0, 0, time.UTC)
	u := NewUseCase(reportProvider{}, time.UTC)
	report := u.Transform(FetchedReport{
		Request:    ReportRequest{Mode: SummaryReport, From: from, To: from.AddDate(0, 0, 30)},
		Recoveries: []domain.Recovery{{CycleID: 1, Score: 80, HRVMilli: 70}, {CycleID: 2, Score: 60, HRVMilli: 50}},
		Cycles: []domain.Cycle{
			{ID: 1, Start: from, ScoreState: "SCORED", Strain: 10},
			{ID: 2, Start: from.AddDate(0, 0, 1), ScoreState: "SCORED", Strain: 14},
		},
		Sleeps: []domain.Sleep{{End: from.Add(8 * time.Hour), SleepPerformancePct: 90, Stages: domain.SleepStages{
			LightMs: int64(4 * time.Hour / time.Millisecond), SWSMs: int64(2 * time.Hour / time.Millisecond), REMMs: int64(time.Hour / time.Millisecond),
		}}},
	})
	require.InDelta(t, 70, *report.RecoveryAvg, 0.01)
	require.InDelta(t, 12, *report.StrainAvg, 0.01)
	require.Equal(t, int64(7*time.Hour/time.Millisecond), *report.SleepAvgMs)
}
