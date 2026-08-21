package controlcenter

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"time"
)

type Service struct {
	store   Store
	ownerID int64
	loc     *time.Location
	now     func() time.Time
}

type requestLocationKey struct{}

func recoveryRangeBounds(raw json.RawMessage) (float64, float64, error) {
	low, high := 34.0, 67.0
	if len(raw) == 0 {
		return low, high, nil
	}
	var ranges struct {
		Low  *float64 `json:"low"`
		High *float64 `json:"high"`
	}
	if err := json.Unmarshal(raw, &ranges); err != nil {
		return 0, 0, fmt.Errorf("must be an object with numeric low/high thresholds")
	}
	if ranges.Low != nil {
		low = *ranges.Low
	}
	if ranges.High != nil {
		high = *ranges.High
	}
	if math.IsNaN(low) || math.IsInf(low, 0) || math.IsNaN(high) || math.IsInf(high, 0) || low <= 0 || high > 100 || low >= high {
		return 0, 0, fmt.Errorf("low and high must satisfy 0 < low < high <= 100")
	}
	return low, high, nil
}

func contextWithLocation(ctx context.Context, loc *time.Location) context.Context {
	return context.WithValue(ctx, requestLocationKey{}, loc)
}

func (s *Service) location(ctx context.Context) *time.Location {
	if loc, ok := ctx.Value(requestLocationKey{}).(*time.Location); ok && loc != nil {
		return loc
	}
	return s.loc
}

func NewService(store Store, ownerID int64, loc *time.Location) *Service {
	if loc == nil {
		loc = time.UTC
	}
	return &Service{store: store, ownerID: ownerID, loc: loc, now: time.Now}
}

func (s *Service) Overview(ctx context.Context, dateRange DateRange) (Overview, error) {
	return s.store.Overview(ctx, s.ownerID, dateRange, s.location(ctx))
}

type focusedAnalyticsStore interface {
	TrainingAnalytics(context.Context, int64, DateRange, *time.Location, AnalyticsFilters) (map[string]any, error)
	FilteredTrainingDaily(context.Context, int64, DateRange, *time.Location, AnalyticsFilters) ([]DailyPoint, error)
}

func (s *Service) Analytics(ctx context.Context, kind string, dateRange DateRange, requested ...AnalyticsFilters) (any, error) {
	filters := AnalyticsFilters{}
	if len(requested) > 0 {
		filters = requested[0]
	}
	if kind == "training" {
		if focused, ok := s.store.(focusedAnalyticsStore); ok {
			result, err := focused.TrainingAnalytics(ctx, s.ownerID, dateRange, s.location(ctx), filters)
			if err != nil {
				return nil, err
			}
			if dateRange.Compare {
				previous, previousErr := focused.TrainingAnalytics(ctx, s.ownerID, dateRange.Previous(), s.location(ctx), filters)
				if previousErr != nil {
					return nil, previousErr
				}
				result["comparison"] = previous
			}
			return result, nil
		}
	}
	queryRange := dateRange
	queryRange.Compare = false
	overview, err := s.store.Overview(ctx, s.ownerID, queryRange, s.location(ctx))
	if err != nil {
		return nil, err
	}
	allDaily := overview.Daily
	correlationCalendar := allDaily
	if kind == "correlations" {
		calendarRange := queryRange
		calendarRange.To = calendarRange.To.AddDate(0, 0, 1)
		calendarOverview, calendarErr := s.store.Overview(ctx, s.ownerID, calendarRange, s.location(ctx))
		if calendarErr != nil {
			return nil, calendarErr
		}
		correlationCalendar = calendarOverview.Daily
	}
	daily := allDaily
	hasTrainingFilter := filters.ExerciseID != nil || filters.PlanID != nil || filters.TemplateID != nil || filters.Status != ""
	if focused, ok := s.store.(focusedAnalyticsStore); ok && hasTrainingFilter {
		trainingDaily, focusedErr := focused.FilteredTrainingDaily(ctx, s.ownerID, dateRange, s.location(ctx), filters)
		if focusedErr != nil {
			return nil, focusedErr
		}
		daily = mergeTrainingDaily(daily, trainingDaily)
		// Exercise/template/status filters describe sessions, so correlation
		// samples are limited to local dates that contain a matching session.
		daily = filterDailyByType(daily, "training")
	}
	daily = filterDailyByType(daily, filters.DayType)
	base := map[string]any{"range": rangeView(dateRange, s.location(ctx)), "daily": daily}
	switch kind {
	case "training":
		base["summary"] = overview.Summary.Training
	case "recovery":
		base["summary"] = overview.Summary.Recovery
	case "nutrition":
		base["summary"] = overview.Summary.Nutrition
	case "body":
		base["summary"] = overview.Summary.Body
	case "correlations":
		base["correlations"] = correlationsFromDaily(daily, correlationCalendar)
	default:
		return nil, ErrNotFound
	}
	if dateRange.Compare {
		previousRange := dateRange.Previous()
		previousOverview, previousErr := s.store.Overview(ctx, s.ownerID, previousRange, s.location(ctx))
		if previousErr != nil {
			return nil, previousErr
		}
		previousAllDaily := previousOverview.Daily
		previousCorrelationCalendar := previousAllDaily
		if kind == "correlations" {
			calendarRange := previousRange
			calendarRange.To = calendarRange.To.AddDate(0, 0, 1)
			calendarOverview, calendarErr := s.store.Overview(ctx, s.ownerID, calendarRange, s.location(ctx))
			if calendarErr != nil {
				return nil, calendarErr
			}
			previousCorrelationCalendar = calendarOverview.Daily
		}
		previousDaily := previousAllDaily
		if focused, ok := s.store.(focusedAnalyticsStore); ok && hasTrainingFilter {
			trainingDaily, focusedErr := focused.FilteredTrainingDaily(ctx, s.ownerID, previousRange, s.location(ctx), filters)
			if focusedErr != nil {
				return nil, focusedErr
			}
			previousDaily = filterDailyByType(mergeTrainingDaily(previousDaily, trainingDaily), "training")
		}
		previousDaily = filterDailyByType(previousDaily, filters.DayType)
		comparison := map[string]any{"range": previousOverview.Range, "daily": previousDaily}
		switch kind {
		case "recovery":
			comparison["summary"] = previousOverview.Summary.Recovery
		case "nutrition":
			comparison["summary"] = previousOverview.Summary.Nutrition
		case "body":
			comparison["summary"] = previousOverview.Summary.Body
		case "correlations":
			comparison["correlations"] = correlationsFromDaily(previousDaily, previousCorrelationCalendar)
		}
		base["comparison"] = comparison
	}
	return base, nil
}

func mergeTrainingDaily(base, focused []DailyPoint) []DailyPoint {
	byDate := make(map[string]DailyPoint, len(focused))
	for _, point := range focused {
		byDate[point.Date] = point
	}
	merged := append([]DailyPoint(nil), base...)
	for index := range merged {
		point := byDate[merged[index].Date]
		merged[index].WorkoutCount = point.WorkoutCount
		merged[index].WorkingSets = point.WorkingSets
		merged[index].Repetitions = point.Repetitions
		merged[index].TrainingVolumeKG = point.TrainingVolumeKG
		merged[index].AverageRIR = point.AverageRIR
		merged[index].Estimated1RM = point.Estimated1RM
	}
	return merged
}

func filterDailyByType(points []DailyPoint, dayType string) []DailyPoint {
	if dayType == "" {
		return points
	}
	filtered := make([]DailyPoint, 0, len(points))
	for _, point := range points {
		if (dayType == "training" && point.WorkoutCount > 0) || (dayType == "rest" && point.WorkoutCount == 0) {
			filtered = append(filtered, point)
		}
	}
	return filtered
}

func correlationsFromDaily(points []DailyPoint, calendar ...[]DailyPoint) []map[string]any {
	type candidate struct {
		id, title, definition, x, y, xLabel, yLabel string
		pairs                                       []PairedValue
	}
	candidates := []candidate{
		{id: "sleep_recovery", title: "Сон и восстановление", definition: "Сопоставляет длительность сна с оценкой восстановления того же дня.", x: "sleep_seconds", y: "recovery_score", xLabel: "Сон, сек", yLabel: "Recovery, %"},
		{id: "strain_next_recovery", title: "Нагрузка и восстановление на следующий день", definition: "Сопоставляет дневную нагрузку с recovery следующей локальной даты.", x: "daily_strain", y: "next_day_recovery_score", xLabel: "Strain", yLabel: "Recovery следующего дня, %"},
		{id: "calories_weight", title: "Калории и вес", definition: "Сопоставляет дневную калорийность с измерением веса той же даты.", x: "calories_kcal", y: "weight_kg", xLabel: "Ккал", yLabel: "Вес, кг"},
		{id: "protein_lean_mass", title: "Белок и безжировая масса", definition: "Сопоставляет потребление белка с измерением lean mass той же даты.", x: "protein_g", y: "lean_mass_kg", xLabel: "Белок, г", yLabel: "Lean mass, кг"},
		{id: "hrv_average_rir", title: "HRV и интенсивность тренировки", definition: "Сопоставляет HRV с фактическим средним RIR завершённых подходов.", x: "hrv_ms", y: "average_rir", xLabel: "HRV, мс", yLabel: "Средний RIR"},
	}
	completeCalendar := points
	if len(calendar) > 0 {
		completeCalendar = calendar[0]
	}
	recoveryByDate := make(map[string]*float64, len(completeCalendar))
	for _, point := range completeCalendar {
		recoveryByDate[point.Date] = point.RecoveryScore
	}
	for _, point := range points {
		candidates[0].pairs = append(candidates[0].pairs, PairedValue{X: int64AsFloat(point.SleepSeconds), Y: point.RecoveryScore})
		var nextRecovery *float64
		if day, err := time.Parse("2006-01-02", point.Date); err == nil {
			nextRecovery = recoveryByDate[day.AddDate(0, 0, 1).Format("2006-01-02")]
		}
		candidates[1].pairs = append(candidates[1].pairs, PairedValue{X: point.DailyStrain, Y: nextRecovery})
		candidates[2].pairs = append(candidates[2].pairs, PairedValue{X: point.CaloriesKcal, Y: point.WeightKG})
		candidates[3].pairs = append(candidates[3].pairs, PairedValue{X: point.ProteinG, Y: point.LeanMassKG})
		candidates[4].pairs = append(candidates[4].pairs, PairedValue{X: point.HRVMs, Y: point.AverageRIR})
	}
	result := make([]map[string]any, 0, len(candidates))
	from, to, period := "", "", ""
	if len(points) > 0 {
		from, to = points[0].Date, points[len(points)-1].Date
		period = from + " — " + to
	}
	for _, item := range candidates {
		correlation := PearsonCorrelation(item.pairs)
		result = append(result, map[string]any{
			"id": item.id, "title": item.title, "definition": item.definition,
			"period": period, "from": from, "to": to,
			"x_metric": item.x, "y_metric": item.y, "x_label": item.xLabel, "y_label": item.yLabel,
			"coefficient": correlation.Coefficient, "sample_size": correlation.SampleSize,
			"insufficient_sample": correlation.InsufficientSample,
		})
	}
	return result
}

func int64AsFloat(value *int64) *float64 {
	if value == nil {
		return nil
	}
	converted := float64(*value)
	return &converted
}

func (s *Service) List(ctx context.Context, resource string, options Pagination) (ListResult, error) {
	if !knownResource(resource) {
		return ListResult{}, ErrNotFound
	}
	return s.store.List(ctx, s.ownerID, resource, options, s.location(ctx))
}

func (s *Service) Get(ctx context.Context, resource string, id int64) (json.RawMessage, error) {
	if !knownResource(resource) {
		return nil, ErrNotFound
	}
	return s.store.Get(ctx, s.ownerID, resource, id, s.location(ctx))
}

func (s *Service) Create(ctx context.Context, resource string, raw json.RawMessage) (json.RawMessage, error) {
	if !mutableResource(resource) {
		return nil, ErrNotFound
	}
	return s.store.Create(ctx, s.ownerID, resource, raw, s.location(ctx))
}

func (s *Service) Update(ctx context.Context, resource string, id int64, raw json.RawMessage) (json.RawMessage, error) {
	if !mutableResource(resource) {
		return nil, ErrNotFound
	}
	return s.store.Update(ctx, s.ownerID, resource, id, raw, s.location(ctx))
}

func (s *Service) Delete(ctx context.Context, resource string, id int64) error {
	if !mutableResource(resource) {
		return ErrNotFound
	}
	return s.store.Delete(ctx, s.ownerID, resource, id)
}

func knownResource(resource string) bool {
	switch resource {
	case "workout-sessions", "workout-plans", "exercises", "recovery", "sleep", "nutrition", "body-measurements", "imports":
		return true
	default:
		return false
	}
}

func mutableResource(resource string) bool {
	return resource != "imports" && knownResource(resource)
}

func (s *Service) Settings(ctx context.Context) (Settings, error) {
	return s.store.Settings(ctx, s.ownerID, s.location(ctx).String())
}

func (s *Service) SaveSettings(ctx context.Context, settings Settings) (Settings, error) {
	settings.Timezone = strings.TrimSpace(settings.Timezone)
	if settings.Timezone == "" {
		settings.Timezone = s.location(ctx).String()
	}
	if _, err := time.LoadLocation(settings.Timezone); err != nil {
		return Settings{}, &ValidationError{Message: "invalid settings", Fields: map[string]string{"timezone": "must be an IANA timezone"}}
	}
	if settings.Units == "" {
		settings.Units = "metric"
	}
	if settings.Units != "metric" {
		return Settings{}, &ValidationError{Message: "invalid settings", Fields: map[string]string{"units": "only metric units are supported"}}
	}
	if settings.Theme == "" {
		settings.Theme = "dark"
	}
	if settings.Theme != "dark" && settings.Theme != "light" && settings.Theme != "system" {
		return Settings{}, &ValidationError{Message: "invalid settings", Fields: map[string]string{"theme": "use dark, light, or system"}}
	}
	if settings.FirstDayOfWeek == 0 {
		settings.FirstDayOfWeek = 1
	}
	if settings.FirstDayOfWeek < 1 || settings.FirstDayOfWeek > 7 {
		return Settings{}, &ValidationError{Message: "invalid settings", Fields: map[string]string{"first_day_of_week": "must be between 1 and 7"}}
	}
	positiveTargets := map[string]*float64{
		"calorie_target_kcal":    settings.CalorieTargetKcal,
		"protein_target_g":       settings.ProteinTargetG,
		"fat_target_g":           settings.FatTargetG,
		"carbohydrates_target_g": settings.CarbohydratesTargetG,
	}
	for field, value := range positiveTargets {
		if value != nil && (*value <= 0 || math.IsNaN(*value) || math.IsInf(*value, 0)) {
			return Settings{}, &ValidationError{Message: "invalid settings", Fields: map[string]string{field: "must be a positive finite number"}}
		}
	}
	if settings.SleepTargetMinSeconds != nil && *settings.SleepTargetMinSeconds <= 0 {
		return Settings{}, &ValidationError{Message: "invalid settings", Fields: map[string]string{"sleep_target_min_seconds": "must be positive"}}
	}
	if settings.SleepTargetMaxSeconds != nil && *settings.SleepTargetMaxSeconds <= 0 {
		return Settings{}, &ValidationError{Message: "invalid settings", Fields: map[string]string{"sleep_target_max_seconds": "must be positive"}}
	}
	if settings.SleepTargetMinSeconds != nil && settings.SleepTargetMaxSeconds != nil && *settings.SleepTargetMinSeconds > *settings.SleepTargetMaxSeconds {
		return Settings{}, &ValidationError{Message: "invalid settings", Fields: map[string]string{"sleep_target_max_seconds": "must not be less than sleep_target_min_seconds"}}
	}
	if len(settings.RecoveryRanges) > 0 {
		if _, _, err := recoveryRangeBounds(settings.RecoveryRanges); err != nil {
			return Settings{}, &ValidationError{Message: "invalid settings", Fields: map[string]string{"recovery_ranges": err.Error()}}
		}
	}
	return s.store.SaveSettings(ctx, s.ownerID, settings)
}

func (s *Service) Sources(ctx context.Context) ([]SourceStatus, error) {
	return s.store.Sources(ctx, s.ownerID)
}

func (s *Service) ExportSessionsCSV(ctx context.Context, dateRange DateRange, filters Pagination) ([]byte, error) {
	return s.store.ExportSessionsCSV(ctx, s.ownerID, dateRange, filters, s.location(ctx))
}

func (s *Service) ExportAll(ctx context.Context) (json.RawMessage, error) {
	return s.store.ExportAll(ctx, s.ownerID, s.location(ctx))
}

func (s *Service) DeleteAll(ctx context.Context, confirmation string) error {
	if confirmation != "DELETE MY DATA" {
		return &ValidationError{Message: "confirmation does not match", Fields: map[string]string{"confirmation": "enter DELETE MY DATA exactly"}}
	}
	return s.store.DeleteAll(ctx, s.ownerID)
}

func (s *Service) PreviewImport(ctx context.Context, request ImportRequest) (ImportPreview, error) {
	batch, preview, err := parseImport(request)
	if err != nil {
		return ImportPreview{}, err
	}
	ids := make([]string, 0, len(batch.Rows))
	for _, row := range batch.Rows {
		if row.ExternalID != "" {
			ids = append(ids, importDuplicateKey(row))
		}
	}
	existing, err := s.store.ExistingExternalIDs(ctx, s.ownerID, batch.DataType, batch.Source, ids)
	if err != nil {
		return ImportPreview{}, err
	}
	seen := make(map[string]struct{}, len(batch.Rows))
	for _, row := range batch.Rows {
		if row.ExternalID == "" {
			continue
		}
		key := importDuplicateKey(row)
		_, repeatedInFile := seen[key]
		_, alreadyStored := existing[key]
		if repeatedInFile || alreadyStored {
			preview.DuplicateRows++
		}
		seen[key] = struct{}{}
	}
	return preview, nil
}

func importDuplicateKey(row ImportRow) string {
	if row.DataType == "sets" {
		return row.Values["session_external_id"] + compositeExternalSeparator + row.ExternalID
	}
	return row.ExternalID
}

func (s *Service) ExecuteImport(ctx context.Context, request ImportRequest) (ImportResult, error) {
	batch, _, err := parseImport(request)
	if err != nil {
		return ImportResult{}, err
	}
	if len(batch.Rows) == 0 {
		return ImportResult{}, &ValidationError{Message: "import has no valid rows", Fields: map[string]string{"content": "fix validation errors before execution"}}
	}
	return s.store.ExecuteImport(ctx, s.ownerID, batch, s.location(ctx))
}

func (s *Service) SeedDemo(ctx context.Context, now time.Time) (DemoSeedResult, error) {
	if now.IsZero() {
		now = s.now()
	}
	return s.store.SeedDemo(ctx, s.ownerID, now, s.location(ctx))
}

// SeedDemo is an explicit, opt-in entry point; no startup path calls it.
func SeedDemo(ctx context.Context, store Store, ownerID int64, now time.Time, loc *time.Location) (DemoSeedResult, error) {
	if store == nil {
		return DemoSeedResult{}, fmt.Errorf("seed demo: nil store")
	}
	return NewService(store, ownerID, loc).SeedDemo(ctx, now)
}
