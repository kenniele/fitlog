package whoop

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"fitlog/internal/domain"
	"fitlog/internal/reportfmt"
)

type ReportMode uint8

const (
	DailyReport ReportMode = iota + 1
	SummaryReport
)

// ReportRequest is a half-open calendar window. Daily reports use exactly one
// day; summaries use completed days only.
type ReportRequest struct {
	Mode ReportMode
	From time.Time
	To   time.Time
}

func Today(now time.Time, loc *time.Location) ReportRequest {
	now = now.In(loc)
	from := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)
	return ReportRequest{Mode: DailyReport, From: from, To: from.AddDate(0, 0, 1)}
}

func LastCompletedDays(now time.Time, loc *time.Location, days int) ReportRequest {
	now = now.In(loc)
	to := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)
	return ReportRequest{Mode: SummaryReport, From: to.AddDate(0, 0, -days), To: to}
}

type FetchedReport struct {
	Request    ReportRequest
	Cycles     []domain.Cycle
	Recoveries []domain.Recovery
	Sleeps     []domain.Sleep
	Workouts   []domain.Workout
}

type Baseline struct {
	Days        int
	RecoveryAvg float64
	HRVAvg      float64
	RHRAvg      float64
}

type Report struct {
	Request ReportRequest

	Cycle    *domain.Cycle
	Recovery *domain.Recovery
	Sleep    *domain.Sleep
	Workouts []domain.Workout
	Baseline *Baseline

	RecoveryAvg *float64
	HRVAvg      *float64
	StrainTotal *float64
	StrainAvg   *float64
	SleepAvgMs  *int64
	SleepPerf   *float64
	TopWorkouts []domain.Workout
}

// ReportUseCase makes the module's pipeline explicit and independently
// testable: transport data is fetched, transformed into a report model, then
// formatted for the delivery channel.
type ReportUseCase interface {
	Fetch(context.Context, ReportRequest) (FetchedReport, error)
	Transform(FetchedReport) Report
	Format(Report) string
	Execute(context.Context, ReportRequest) (string, error)
}

type UseCase struct {
	provider Provider
	loc      *time.Location
}

func NewUseCase(provider Provider, loc *time.Location) *UseCase {
	return &UseCase{provider: provider, loc: loc}
}

func (u *UseCase) Fetch(ctx context.Context, req ReportRequest) (FetchedReport, error) {
	client, err := u.provider.Client(ctx)
	if err != nil {
		return FetchedReport{}, err
	}

	fetched := FetchedReport{Request: req}
	// Cycles and sleeps commonly start on the previous evening. Query a small
	// lead-in and let Transform attribute records by their day anchor.
	rng := domain.TimeRange{From: req.From.Add(-18 * time.Hour), To: req.To}
	if req.Mode == DailyReport {
		// Sleep commonly starts on the previous day. Recoveries need extra
		// history for the rolling seven-day baseline.
		rng = domain.TimeRange{From: req.From.Add(-18 * time.Hour), To: req.To.Add(6 * time.Hour)}
	}

	fetched.Sleeps, err = client.Sleeps(ctx, rng, 25)
	if err != nil {
		return FetchedReport{}, fmt.Errorf("fetch sleeps: %w", err)
	}
	recoveryRange := rng
	if req.Mode == DailyReport {
		recoveryRange.From = req.To.AddDate(0, 0, -14)
	}
	fetched.Recoveries, err = client.Recoveries(ctx, recoveryRange, 25)
	if err != nil {
		return FetchedReport{}, fmt.Errorf("fetch recoveries: %w", err)
	}
	fetched.Cycles, err = client.Cycles(ctx, rng, 25)
	if err != nil {
		return FetchedReport{}, fmt.Errorf("fetch cycles: %w", err)
	}
	fetched.Workouts, err = client.Workouts(ctx, rng, 25)
	if err != nil {
		return FetchedReport{}, fmt.Errorf("fetch workouts: %w", err)
	}
	return fetched, nil
}

func (u *UseCase) Transform(in FetchedReport) Report {
	out := Report{Request: in.Request}
	if in.Request.Mode == DailyReport {
		out.Sleep = pickReportSleep(in.Sleeps, in.Request.From, in.Request.To)
		out.Recovery = pickReportRecovery(in.Recoveries, out.Sleep)
		var cycleID int64
		if out.Recovery != nil {
			cycleID = out.Recovery.CycleID
		}
		out.Cycle = pickReportCycle(in.Cycles, in.Request.From, in.Request.To, cycleID)
		out.Baseline = recoveryBaseline(in.Recoveries, out.Recovery)
		for _, workout := range in.Workouts {
			if !workout.Start.Before(in.Request.From) && workout.Start.Before(in.Request.To) {
				out.Workouts = append(out.Workouts, workout)
			}
		}
		return out
	}

	validCycles := make(map[int64]struct{})
	var strainTotal float64
	strainCount := 0
	for _, cycle := range in.Cycles {
		anchor := cycleReportAnchor(cycle)
		if anchor.Before(in.Request.From) || !anchor.Before(in.Request.To) {
			continue
		}
		validCycles[cycle.ID] = struct{}{}
		if cycle.ScoreState != "SCORED" {
			continue
		}
		strainTotal += cycle.Strain
		strainCount++
	}
	if strainCount > 0 {
		strainAvg := strainTotal / float64(strainCount)
		out.StrainTotal, out.StrainAvg = &strainTotal, &strainAvg
	}

	var recoveryTotal, hrvTotal float64
	recoveryCount := 0
	for _, recovery := range in.Recoveries {
		if _, ok := validCycles[recovery.CycleID]; !ok {
			continue
		}
		if recovery.Score == 0 && recovery.HRVMilli == 0 {
			continue
		}
		recoveryTotal += recovery.Score
		hrvTotal += recovery.HRVMilli
		recoveryCount++
	}
	if recoveryCount > 0 {
		recoveryAvg := recoveryTotal / float64(recoveryCount)
		hrvAvg := hrvTotal / float64(recoveryCount)
		out.RecoveryAvg, out.HRVAvg = &recoveryAvg, &hrvAvg
	}

	var sleepTotal int64
	var performanceTotal float64
	sleepCount := 0
	for _, sleep := range in.Sleeps {
		if sleep.IsNap || sleep.End.Before(in.Request.From) || !sleep.End.Before(in.Request.To) {
			continue
		}
		sleepTotal += sleep.Stages.LightMs + sleep.Stages.SWSMs + sleep.Stages.REMMs
		performanceTotal += sleep.SleepPerformancePct
		sleepCount++
	}
	if sleepCount > 0 {
		sleepAvg := sleepTotal / int64(sleepCount)
		performanceAvg := performanceTotal / float64(sleepCount)
		out.SleepAvgMs, out.SleepPerf = &sleepAvg, &performanceAvg
	}

	for _, workout := range in.Workouts {
		if !workout.Start.Before(in.Request.From) && workout.Start.Before(in.Request.To) {
			out.TopWorkouts = append(out.TopWorkouts, workout)
		}
	}
	sort.SliceStable(out.TopWorkouts, func(i, j int) bool {
		return out.TopWorkouts[i].Strain > out.TopWorkouts[j].Strain
	})
	if len(out.TopWorkouts) > 5 {
		out.TopWorkouts = out.TopWorkouts[:5]
	}
	return out
}

func (u *UseCase) Format(report Report) string {
	if report.Request.Mode == SummaryReport {
		return u.formatSummary(report)
	}
	return u.formatDaily(report)
}

func (u *UseCase) Execute(ctx context.Context, req ReportRequest) (string, error) {
	fetched, err := u.Fetch(ctx, req)
	if err != nil {
		return "", err
	}
	return u.Format(u.Transform(fetched)), nil
}

func (u *UseCase) formatDaily(r Report) string {
	var b strings.Builder
	fmt.Fprintf(&b, "🫀 *Здоровье · %s*\n\n", reportfmt.Escape(reportfmt.DateLong(r.Request.From, u.loc)))
	if r.Sleep == nil {
		b.WriteString("🌙 *Сон* — данных нет\n\n")
	} else {
		total := r.Sleep.Stages.LightMs + r.Sleep.Stages.SWSMs + r.Sleep.Stages.REMMs
		fmt.Fprintf(&b, "🌙 *Сон* %s → %s · %s\n",
			reportfmt.Escape(reportfmt.Clock(r.Sleep.Start, u.loc)),
			reportfmt.Escape(reportfmt.Clock(r.Sleep.End, u.loc)),
			reportfmt.Escape(reportfmt.Duration(total)))
		fmt.Fprintf(&b, "  Performance %s%% · efficiency %s%% · REM %s · deep %s\n\n",
			reportfmt.Number(r.Sleep.SleepPerformancePct, 0), reportfmt.Number(r.Sleep.SleepEfficiencyPct, 0),
			reportfmt.Escape(reportfmt.Duration(r.Sleep.Stages.REMMs)),
			reportfmt.Escape(reportfmt.Duration(r.Sleep.Stages.SWSMs)))
	}
	if r.Recovery == nil {
		b.WriteString("💪 *Recovery* — данных нет\n\n")
	} else {
		fmt.Fprintf(&b, "💪 *Recovery* %s%% %s · HRV %s ms · RHR %s bpm",
			reportfmt.Number(r.Recovery.Score, 0), recoveryColor(r.Recovery.Score),
			reportfmt.Number(r.Recovery.HRVMilli, 0), reportfmt.Number(r.Recovery.RestingHR, 0))
		if r.Baseline != nil {
			fmt.Fprintf(&b, "\n  Среднее за %d %s: recovery %s%% · HRV %s · RHR %s",
				r.Baseline.Days, reportfmt.PluralDays(r.Baseline.Days),
				reportfmt.Number(r.Baseline.RecoveryAvg, 0), reportfmt.Number(r.Baseline.HRVAvg, 0),
				reportfmt.Number(r.Baseline.RHRAvg, 0))
		}
		b.WriteString("\n\n")
	}
	if r.Cycle == nil {
		b.WriteString("⚡ *Нагрузка* — данных нет\n\n")
	} else {
		fmt.Fprintf(&b, "⚡ *Нагрузка* %s / 21 · HR %s/%s · \\~%s kcal\n\n",
			reportfmt.Number(r.Cycle.Strain, 1), reportfmt.Number(r.Cycle.AvgHR, 0),
			reportfmt.Number(r.Cycle.MaxHR, 0), reportfmt.Number(r.Cycle.Kilojoule/4.184, 0))
	}
	if len(r.Workouts) == 0 {
		b.WriteString("🏋 *Тренировки* — нет")
	} else {
		fmt.Fprintf(&b, "🏋 *Тренировки* \\(%d\\)\n", len(r.Workouts))
		for _, workout := range r.Workouts {
			fmt.Fprintf(&b, "  • %s · %s · strain %s\n",
				reportfmt.Escape(workout.SportName),
				reportfmt.Escape(reportfmt.Duration(workout.End.Sub(workout.Start).Milliseconds())),
				reportfmt.Number(workout.Strain, 1))
		}
	}
	return strings.TrimSpace(b.String())
}

func (u *UseCase) formatSummary(r Report) string {
	days := reportDayCount(r.Request)
	var b strings.Builder
	fmt.Fprintf(&b, "🫀 *Whoop за %d %s* \\(%s — %s\\)\n",
		days, reportfmt.PluralDays(days), reportfmt.Escape(reportfmt.Date(r.Request.From, u.loc)),
		reportfmt.Escape(reportfmt.Date(r.Request.To.AddDate(0, 0, -1), u.loc)))
	if r.RecoveryAvg != nil {
		fmt.Fprintf(&b, "  Recovery avg %s%% %s · HRV avg %s ms\n",
			reportfmt.Number(*r.RecoveryAvg, 0), recoveryColor(*r.RecoveryAvg), reportfmt.Number(*r.HRVAvg, 0))
	}
	if r.StrainTotal != nil {
		fmt.Fprintf(&b, "  Strain total %s · avg %s\n",
			reportfmt.Number(*r.StrainTotal, 1), reportfmt.Number(*r.StrainAvg, 1))
	}
	if r.SleepAvgMs != nil {
		fmt.Fprintf(&b, "  Сон avg %s · performance %s%%\n",
			reportfmt.Escape(reportfmt.Duration(*r.SleepAvgMs)), reportfmt.Number(*r.SleepPerf, 0))
	}
	if r.RecoveryAvg == nil && r.StrainTotal == nil && r.SleepAvgMs == nil {
		b.WriteString("  Данных Whoop за период нет\\.\n")
	}
	if len(r.TopWorkouts) > 0 {
		b.WriteString("\n  *Топ тренировок*\n")
		for _, workout := range r.TopWorkouts {
			fmt.Fprintf(&b, "  • %s · %s · strain %s\n",
				reportfmt.Escape(reportfmt.Date(workout.Start, u.loc)), reportfmt.Escape(workout.SportName),
				reportfmt.Number(workout.Strain, 1))
		}
	}
	return strings.TrimSpace(b.String())
}

func recoveryColor(score float64) string {
	switch {
	case score >= 67:
		return "🟢"
	case score >= 34:
		return "🟡"
	default:
		return "🔴"
	}
}

func pickReportSleep(sleeps []domain.Sleep, from, to time.Time) *domain.Sleep {
	var main, nap *domain.Sleep
	for i := range sleeps {
		sleep := &sleeps[i]
		if sleep.End.Before(from) || !sleep.End.Before(to) {
			continue
		}
		if sleep.IsNap {
			if nap == nil || sleep.End.After(nap.End) {
				nap = sleep
			}
		} else if main == nil || sleep.End.After(main.End) {
			main = sleep
		}
	}
	if main != nil {
		return main
	}
	return nap
}

func pickReportRecovery(recoveries []domain.Recovery, sleep *domain.Sleep) *domain.Recovery {
	if sleep == nil {
		return nil
	}
	for i := range recoveries {
		if recoveries[i].SleepID == sleep.ExternalID {
			return &recoveries[i]
		}
	}
	return nil
}

func pickReportCycle(cycles []domain.Cycle, from, to time.Time, hintID int64) *domain.Cycle {
	if hintID != 0 {
		for i := range cycles {
			if cycles[i].ID == hintID {
				return &cycles[i]
			}
		}
	}
	var picked *domain.Cycle
	for i := range cycles {
		cycle := &cycles[i]
		if !cycle.Start.Before(to) || cycle.End != nil && !cycle.End.After(from) {
			continue
		}
		if picked == nil || cycle.Start.After(picked.Start) {
			picked = cycle
		}
	}
	return picked
}

func recoveryBaseline(recoveries []domain.Recovery, target *domain.Recovery) *Baseline {
	var recoveryTotal, hrvTotal, rhrTotal float64
	count := 0
	for _, recovery := range recoveries {
		if recovery.Score == 0 && recovery.HRVMilli == 0 || target != nil && recovery.CycleID == target.CycleID {
			continue
		}
		recoveryTotal += recovery.Score
		hrvTotal += recovery.HRVMilli
		rhrTotal += recovery.RestingHR
		count++
		if count == 7 {
			break
		}
	}
	if count == 0 {
		return nil
	}
	return &Baseline{Days: count, RecoveryAvg: recoveryTotal / float64(count), HRVAvg: hrvTotal / float64(count), RHRAvg: rhrTotal / float64(count)}
}

func cycleReportAnchor(cycle domain.Cycle) time.Time {
	if cycle.End != nil {
		return cycle.End.Add(-time.Hour)
	}
	return cycle.Start.Add(12 * time.Hour)
}

func reportDayCount(request ReportRequest) int {
	days := 0
	for day := request.From; day.Before(request.To); day = day.AddDate(0, 0, 1) {
		days++
	}
	return days
}
