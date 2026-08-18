package fatsecret

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
	AnalysisReport
)

const caloriesPerKilogram = 7700.0

type ReportRequest struct {
	Mode ReportMode
	From time.Time
	To   time.Time
}

func Day(day time.Time, loc *time.Location) ReportRequest {
	day = day.In(loc)
	from := time.Date(day.Year(), day.Month(), day.Day(), 0, 0, 0, 0, loc)
	return ReportRequest{Mode: DailyReport, From: from, To: from.AddDate(0, 0, 1)}
}

func Today(now time.Time, loc *time.Location) ReportRequest { return Day(now, loc) }

func Yesterday(now time.Time, loc *time.Location) ReportRequest {
	return Day(now.In(loc).AddDate(0, 0, -1), loc)
}

func LastCompletedDays(now time.Time, loc *time.Location, days int) ReportRequest {
	now = now.In(loc)
	to := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)
	return ReportRequest{Mode: SummaryReport, From: to.AddDate(0, 0, -days), To: to}
}

func NutritionAnalysis(now time.Time, loc *time.Location) ReportRequest {
	req := LastCompletedDays(now, loc, 14)
	req.Mode = AnalysisReport
	return req
}

type FetchedReport struct {
	Request ReportRequest
	Entries []domain.MealEntry
	Days    []domain.DailyNutrition
}

type MealGroup struct {
	Kind     domain.MealKind
	Entries  []domain.MealEntry
	Calories float64
	Protein  float64
	Fat      float64
	Carbs    float64
}

type Report struct {
	Request ReportRequest
	Groups  []MealGroup

	LoggedDays int
	Calories   float64
	Protein    float64
	Fat        float64
	Carbs      float64
	Analysis   *domain.NutritionAnalysis
}

type Source interface {
	FoodEntriesForDay(context.Context, time.Time) ([]domain.MealEntry, error)
	FoodEntriesMonth(context.Context, time.Time) ([]domain.DailyNutrition, error)
}

// ReportUseCase mirrors the Whoop module's Fetch → Transform → Format
// contract, while keeping provider-specific models inside this package.
type ReportUseCase interface {
	Fetch(context.Context, ReportRequest) (FetchedReport, error)
	Transform(FetchedReport) Report
	Format(Report) string
	Execute(context.Context, ReportRequest) (string, error)
}

type UseCase struct {
	source        Source
	loc           *time.Location
	estimatedTDEE float64
}

type ReportOptions struct {
	EstimatedTDEE float64
}

func NewUseCase(source Source, loc *time.Location, options ...ReportOptions) *UseCase {
	u := &UseCase{source: source, loc: loc}
	if len(options) > 0 {
		u.estimatedTDEE = options[0].EstimatedTDEE
	}
	return u
}

func (u *UseCase) Fetch(ctx context.Context, req ReportRequest) (FetchedReport, error) {
	out := FetchedReport{Request: req}
	if req.Mode == DailyReport {
		entries, err := u.source.FoodEntriesForDay(ctx, req.From)
		if err != nil {
			return FetchedReport{}, fmt.Errorf("fetch food entries: %w", err)
		}
		out.Entries = entries
		return out, nil
	}

	seen := make(map[int]domain.DailyNutrition)
	for _, month := range reportMonths(req.From, req.To.AddDate(0, 0, -1), u.loc) {
		days, err := u.source.FoodEntriesMonth(ctx, month)
		if err != nil {
			return FetchedReport{}, fmt.Errorf("fetch nutrition month %s: %w", month.Format("2006-01"), err)
		}
		for _, day := range days {
			seen[day.DateInt] = day
		}
	}
	for _, day := range seen {
		out.Days = append(out.Days, day)
	}
	sort.Slice(out.Days, func(i, j int) bool { return out.Days[i].DateInt < out.Days[j].DateInt })
	return out, nil
}

func (u *UseCase) Transform(in FetchedReport) Report {
	out := Report{Request: in.Request}
	if in.Request.Mode == DailyReport {
		groups := make(map[domain.MealKind]*MealGroup)
		for _, entry := range in.Entries {
			group := groups[entry.Meal]
			if group == nil {
				group = &MealGroup{Kind: entry.Meal}
				groups[entry.Meal] = group
			}
			group.Entries = append(group.Entries, entry)
			group.Calories += pointerValue(entry.Calories)
			group.Protein += pointerValue(entry.Protein)
			group.Fat += pointerValue(entry.Fat)
			group.Carbs += pointerValue(entry.Carbs)
		}
		for _, kind := range []domain.MealKind{domain.MealBreakfast, domain.MealLunch, domain.MealDinner, domain.MealOther} {
			if group := groups[kind]; group != nil {
				sort.SliceStable(group.Entries, func(i, j int) bool { return group.Entries[i].FoodName < group.Entries[j].FoodName })
				out.Groups = append(out.Groups, *group)
				out.Calories += group.Calories
				out.Protein += group.Protein
				out.Fat += group.Fat
				out.Carbs += group.Carbs
			}
		}
		return out
	}

	from, to := ToDateInt(in.Request.From), ToDateInt(in.Request.To)
	for _, day := range in.Days {
		if day.DateInt < from || day.DateInt >= to {
			continue
		}
		out.Calories += day.Calories
		out.Protein += day.Protein
		out.Fat += day.Fat
		out.Carbs += day.Carbs
		out.LoggedDays++
	}
	if out.LoggedDays > 0 {
		count := float64(out.LoggedDays)
		out.Calories /= count
		out.Protein /= count
		out.Fat /= count
		out.Carbs /= count
		if in.Request.Mode == AnalysisReport && u.estimatedTDEE > 0 {
			out.Analysis = &domain.NutritionAnalysis{
				Calories: out.Calories, Protein: out.Protein, Fat: out.Fat, Carbs: out.Carbs,
				EstimatedTDEE: u.estimatedTDEE,
				Deficit:       u.estimatedTDEE - out.Calories,
			}
		}
	}
	return out
}

func (u *UseCase) Format(report Report) string {
	if report.Request.Mode == AnalysisReport {
		return u.formatAnalysis(report)
	}
	if report.Request.Mode == SummaryReport {
		return u.formatSummary(report)
	}
	return u.formatDaily(report)
}

func (u *UseCase) formatAnalysis(r Report) string {
	var b strings.Builder
	b.WriteString("📉 *Анализ дефицита за 14 дней*\n")
	if r.LoggedDays == 0 {
		b.WriteString("Записей о питании за период нет\\.")
		return b.String()
	}
	if r.Analysis == nil {
		fmt.Fprintf(&b, "Среднее питание: %s ккал · Б %s · Ж %s · У %s\n\n",
			reportfmt.Number(r.Calories, 0), reportfmt.Number(r.Protein, 0),
			reportfmt.Number(r.Fat, 0), reportfmt.Number(r.Carbs, 0))
		b.WriteString("Для расчёта дефицита задай `NUTRITION_ESTIMATED_TDEE`\\.")
		return b.String()
	}
	a := r.Analysis
	weeklyChange := a.Deficit * 7 / caloriesPerKilogram
	fmt.Fprintf(&b, "За последние 14 дней средний дефицит — *%s ккал*\\.\n", reportfmt.Number(a.Deficit, 0))
	fmt.Fprintf(&b, "Расчётная потеря веса — *%s кг/неделю*\\.\n", reportfmt.Number(weeklyChange, 2))
	fmt.Fprintf(&b, "Белок держался в среднем на *%s г*\\.\n\n", reportfmt.Number(a.Protein, 0))
	fmt.Fprintf(&b, "Среднее: %s ккал · Б %s · Ж %s · У %s\n", reportfmt.Number(a.Calories, 0),
		reportfmt.Number(a.Protein, 0), reportfmt.Number(a.Fat, 0), reportfmt.Number(a.Carbs, 0))
	fmt.Fprintf(&b, "Оценочный TDEE: %s ккал\n", reportfmt.Number(a.EstimatedTDEE, 0))
	fmt.Fprintf(&b, "Учтено %d из 14 %s", r.LoggedDays, reportfmt.PluralDays(r.LoggedDays))
	return b.String()
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
	fmt.Fprintf(&b, "🥑 *Питание · %s*\n\n", reportfmt.Escape(reportfmt.DateLong(r.Request.From, u.loc)))
	if len(r.Groups) == 0 {
		b.WriteString("Записей за выбранный день нет\\.")
		return b.String()
	}
	fmt.Fprintf(&b, "*Итого:* %s kcal · Б %s · Ж %s · У %s\n\n",
		reportfmt.Number(r.Calories, 0), reportfmt.Number(r.Protein, 1),
		reportfmt.Number(r.Fat, 1), reportfmt.Number(r.Carbs, 1))
	for _, group := range r.Groups {
		fmt.Fprintf(&b, "%s *%s* — %s kcal · Б %s · Ж %s · У %s\n",
			mealIcon(group.Kind), reportfmt.Escape(mealName(group.Kind)), reportfmt.Number(group.Calories, 0),
			reportfmt.Number(group.Protein, 1), reportfmt.Number(group.Fat, 1), reportfmt.Number(group.Carbs, 1))
		for _, entry := range group.Entries {
			fmt.Fprintf(&b, "  • %s", reportfmt.Escape(entry.FoodName))
			if entry.Calories != nil {
				fmt.Fprintf(&b, " · %s kcal", reportfmt.Number(*entry.Calories, 0))
			}
			b.WriteByte('\n')
		}
		b.WriteByte('\n')
	}
	return strings.TrimSpace(b.String())
}

func (u *UseCase) formatSummary(r Report) string {
	days := reportDayCount(r.Request)
	var b strings.Builder
	fmt.Fprintf(&b, "🥑 *FatSecret за %d %s* \\(%s — %s\\)\n",
		days, reportfmt.PluralDays(days), reportfmt.Escape(reportfmt.Date(r.Request.From, u.loc)),
		reportfmt.Escape(reportfmt.Date(r.Request.To.AddDate(0, 0, -1), u.loc)))
	if r.LoggedDays == 0 {
		b.WriteString("  Записей о питании за период нет\\.")
		return b.String()
	}
	fmt.Fprintf(&b, "  Среднее: %s kcal · Б %s · Ж %s · У %s\n",
		reportfmt.Number(r.Calories, 0), reportfmt.Number(r.Protein, 1),
		reportfmt.Number(r.Fat, 1), reportfmt.Number(r.Carbs, 1))
	fmt.Fprintf(&b, "  Учтено %d %s с записями", r.LoggedDays, reportfmt.PluralDays(r.LoggedDays))
	return b.String()
}

func pointerValue(value *float64) float64 {
	if value == nil {
		return 0
	}
	return *value
}

func mealIcon(kind domain.MealKind) string {
	switch kind {
	case domain.MealBreakfast:
		return "🍳"
	case domain.MealLunch:
		return "🥗"
	case domain.MealDinner:
		return "🍽"
	default:
		return "🥨"
	}
}

func mealName(kind domain.MealKind) string {
	switch kind {
	case domain.MealBreakfast:
		return "Завтрак"
	case domain.MealLunch:
		return "Обед"
	case domain.MealDinner:
		return "Ужин"
	default:
		return "Другое"
	}
}

func reportMonths(from, to time.Time, loc *time.Location) []time.Time {
	var months []time.Time
	month := time.Date(from.Year(), from.Month(), 1, 0, 0, 0, 0, loc)
	last := time.Date(to.Year(), to.Month(), 1, 0, 0, 0, 0, loc)
	for !month.After(last) {
		months = append(months, month)
		month = month.AddDate(0, 1, 0)
	}
	return months
}

func reportDayCount(request ReportRequest) int {
	days := 0
	for day := request.From; day.Before(request.To); day = day.AddDate(0, 0, 1) {
		days++
	}
	return days
}
