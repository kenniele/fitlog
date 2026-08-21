package controlcenter

import (
	"context"
	"encoding/json"
	"errors"
	"time"
)

const (
	MaxPageSize   = 100
	MaxRangeDays  = 366
	MaxImportSize = 5 << 20
	// JSON can escape one input byte into six bytes (for example control
	// characters), so the bounded transport envelope must be larger than the
	// decoded 5 MiB file limit enforced by parseImport.
	MaxImportEnvelopeSize = MaxImportSize*6 + (1 << 20)
	MaxImportErrorSamples = 100
)

var (
	ErrNotFound   = errors.New("control center data not found")
	ErrConflict   = errors.New("control center data conflict")
	ErrDisabled   = errors.New("control center disabled")
	ErrValidation = errors.New("control center validation failed")
)

// FieldError is returned to clients for input that can be corrected in place.
type FieldError struct {
	Field   string
	Message string
}

type ValidationError struct {
	Message string
	Fields  map[string]string
}

func (e *ValidationError) Error() string {
	if e.Message != "" {
		return e.Message
	}
	return ErrValidation.Error()
}

func (e *ValidationError) Unwrap() error { return ErrValidation }

type DateRange struct {
	From    time.Time
	To      time.Time
	Compare bool
}

func (r DateRange) Days() int {
	days := 0
	for day := r.From; !day.After(r.To); day = day.AddDate(0, 0, 1) {
		days++
	}
	return days
}

func (r DateRange) Previous() DateRange {
	days := r.Days()
	to := r.From.AddDate(0, 0, -1)
	return DateRange{From: to.AddDate(0, 0, -(days - 1)), To: to}
}

type RangeView struct {
	From       string `json:"from"`
	To         string `json:"to"`
	Days       int    `json:"days"`
	Timezone   string `json:"timezone"`
	Comparison bool   `json:"comparison"`
}

type Pagination struct {
	Page     int
	PageSize int
	Search   string
	From     *time.Time
	To       *time.Time
	Filters  map[string]string
}

type ListResult struct {
	Items    []json.RawMessage `json:"items"`
	Total    int               `json:"total"`
	Page     int               `json:"page"`
	PageSize int               `json:"page_size"`
}

// BodySegmentSnapshot keeps the values printed by InBody for one of its five
// standard body regions. The percentage values are relative-to-reference
// scores in common InBody exports and can legitimately exceed 100.
type BodySegmentSnapshot struct {
	Segment     string   `json:"segment"`
	LeanMassKG  *float64 `json:"lean_mass_kg"`
	LeanPercent *float64 `json:"lean_percent"`
	FatMassKG   *float64 `json:"fat_mass_kg"`
	FatPercent  *float64 `json:"fat_percent"`
}

type DailyPoint struct {
	Date                    string                `json:"date"`
	RecoveryScore           *float64              `json:"recovery_score"`
	HRVMs                   *float64              `json:"hrv_ms"`
	RestingHeartRateBPM     *float64              `json:"resting_heart_rate_bpm"`
	RespiratoryRate         *float64              `json:"respiratory_rate"`
	SpO2Percent             *float64              `json:"spo2_percent"`
	SkinTemperatureC        *float64              `json:"skin_temperature_c"`
	SkinTemperatureCelsius  *float64              `json:"skin_temperature_celsius"`
	DailyStrain             *float64              `json:"daily_strain"`
	SleepSeconds            *int64                `json:"sleep_seconds"`
	TimeInBedSeconds        *int64                `json:"time_in_bed_seconds"`
	SleepPerformancePercent *float64              `json:"sleep_performance_percent"`
	SleepEfficiencyPercent  *float64              `json:"sleep_efficiency_percent"`
	SleepConsistencyPercent *float64              `json:"consistency_percent"`
	SleepDebtSeconds        *int64                `json:"sleep_debt_seconds"`
	Disturbances            *int                  `json:"disturbances"`
	REMSeconds              *int64                `json:"rem_seconds"`
	DeepSeconds             *int64                `json:"deep_seconds"`
	LightSeconds            *int64                `json:"light_seconds"`
	AwakeSeconds            *int64                `json:"awake_seconds"`
	CaloriesKcal            *float64              `json:"calories_kcal"`
	ProteinG                *float64              `json:"protein_g"`
	FatG                    *float64              `json:"fat_g"`
	CarbohydratesG          *float64              `json:"carbohydrates_g"`
	FiberG                  *float64              `json:"fiber_g"`
	SugarG                  *float64              `json:"sugar_g"`
	SaturatedFatG           *float64              `json:"saturated_fat_g"`
	SodiumMG                *float64              `json:"sodium_mg"`
	PotassiumMG             *float64              `json:"potassium_mg"`
	WaterML                 *float64              `json:"water_ml"`
	WeightKG                *float64              `json:"weight_kg"`
	BodyFatPercent          *float64              `json:"body_fat_percent"`
	FatMassKG               *float64              `json:"fat_mass_kg"`
	LeanMassKG              *float64              `json:"lean_mass_kg"`
	SkeletalMuscleMassKG    *float64              `json:"skeletal_muscle_mass_kg"`
	TotalBodyWaterL         *float64              `json:"total_body_water_l"`
	IntracellularWaterL     *float64              `json:"intracellular_water_l"`
	ExtracellularWaterL     *float64              `json:"extracellular_water_l"`
	ECWTBWRatio             *float64              `json:"ecw_tbw_ratio"`
	ProteinMassKG           *float64              `json:"protein_mass_kg"`
	MineralMassKG           *float64              `json:"mineral_mass_kg"`
	BMI                     *float64              `json:"bmi"`
	VisceralFatLevel        *float64              `json:"visceral_fat_level"`
	VisceralFatAreaCM2      *float64              `json:"visceral_fat_area_cm2"`
	BasalMetabolicRateKcal  *float64              `json:"basal_metabolic_rate_kcal"`
	InBodyScore             *float64              `json:"inbody_score"`
	PhaseAngleDegrees       *float64              `json:"phase_angle_degrees"`
	BodySegments            []BodySegmentSnapshot `json:"segments,omitempty"`
	WaistCM                 *float64              `json:"waist_cm"`
	ChestCM                 *float64              `json:"chest_cm"`
	BicepsCM                *float64              `json:"biceps_cm"`
	ThighCM                 *float64              `json:"thigh_cm"`
	Weight7DAverage         *float64              `json:"weight_7d_average"`
	Estimated1RM            *float64              `json:"estimated_1rm"`
	HRV7DAverage            *float64              `json:"hrv_7d_average"`
	HRV28DAverage           *float64              `json:"hrv_28d_average"`
	RHR7DAverage            *float64              `json:"rhr_7d_average"`
	RHR28DAverage           *float64              `json:"rhr_28d_average"`
	Sleep7DAverage          *float64              `json:"sleep_7d_average"`
	Sleep28DAverage         *float64              `json:"sleep_28d_average"`
	WorkoutCount            int                   `json:"workout_count"`
	ScheduledSessions       int                   `json:"scheduled_sessions"`
	CompletedScheduled      int                   `json:"completed_scheduled_sessions"`
	PlanAdherence           *float64              `json:"plan_adherence_percent"`
	WorkingSets             int                   `json:"working_sets"`
	Repetitions             int                   `json:"repetitions"`
	TrainingVolumeKG        float64               `json:"training_volume_kg"`
	AverageRIR              *float64              `json:"average_rir"`
	// RIRSum and RIRSamples are carried alongside the daily average so period
	// summaries can weight days by the number of completed sets with an RIR.
	// They are internal aggregation details and must not leak into the API.
	RIRSum     float64 `json:"-"`
	RIRSamples int     `json:"-"`
}

type MetricSummary struct {
	Current *float64 `json:"current"`
	Average *float64 `json:"average"`
	Minimum *float64 `json:"minimum"`
	Maximum *float64 `json:"maximum"`
	Change  *float64 `json:"change"`
	Samples int      `json:"samples"`
}

type TrainingSummary struct {
	Sessions        int      `json:"sessions"`
	WorkingSets     int      `json:"working_sets"`
	Repetitions     int      `json:"repetitions"`
	VolumeKG        float64  `json:"volume_kg"`
	AverageRIR      *float64 `json:"average_rir"`
	AverageMinutes  *float64 `json:"average_duration_minutes"`
	Adherence       *float64 `json:"adherence_percent"`
	PersonalRecords int      `json:"personal_records"`
}

type RecoverySummary struct {
	Recovery MetricSummary `json:"recovery"`
	HRV      MetricSummary `json:"hrv"`
	RHR      MetricSummary `json:"rhr"`
	Strain   MetricSummary `json:"strain"`
	Sleep    MetricSummary `json:"sleep_seconds"`
}

type NutritionSummary struct {
	Calories      MetricSummary `json:"calories"`
	Protein       MetricSummary `json:"protein"`
	Fat           MetricSummary `json:"fat"`
	Carbohydrates MetricSummary `json:"carbohydrates"`
	DaysLogged    int           `json:"days_logged"`
	DaysInTarget  int           `json:"days_in_target"`
}

type BodySummary struct {
	Weight             MetricSummary        `json:"weight"`
	BodyFat            MetricSummary        `json:"body_fat"`
	FatMass            MetricSummary        `json:"fat_mass"`
	LeanMass           MetricSummary        `json:"lean_mass"`
	SkeletalMuscleMass MetricSummary        `json:"skeletal_muscle_mass"`
	TotalBodyWater     MetricSummary        `json:"total_body_water"`
	IntracellularWater MetricSummary        `json:"intracellular_water"`
	ExtracellularWater MetricSummary        `json:"extracellular_water"`
	ECWTBWRatio        MetricSummary        `json:"ecw_tbw_ratio"`
	ProteinMass        MetricSummary        `json:"protein_mass"`
	MineralMass        MetricSummary        `json:"mineral_mass"`
	BMI                MetricSummary        `json:"bmi"`
	VisceralFatLevel   MetricSummary        `json:"visceral_fat_level"`
	VisceralFatArea    MetricSummary        `json:"visceral_fat_area"`
	BasalMetabolicRate MetricSummary        `json:"basal_metabolic_rate"`
	InBodyScore        MetricSummary        `json:"inbody_score"`
	PhaseAngle         MetricSummary        `json:"phase_angle"`
	Segments           []BodySegmentSummary `json:"segments"`
	WeightMovingAvg    []*float64           `json:"weight_moving_average_7d"`
}

type BodySegmentSummary struct {
	Segment     string        `json:"segment"`
	LeanMass    MetricSummary `json:"lean_mass"`
	LeanPercent MetricSummary `json:"lean_percent"`
	FatMass     MetricSummary `json:"fat_mass"`
	FatPercent  MetricSummary `json:"fat_percent"`
}

type DashboardSummary struct {
	Training  TrainingSummary  `json:"training"`
	Recovery  RecoverySummary  `json:"recovery"`
	Nutrition NutritionSummary `json:"nutrition"`
	Body      BodySummary      `json:"body"`
}

type Highlight struct {
	ID          string `json:"id"`
	Type        string `json:"type"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Date        string `json:"date,omitempty"`
	Rule        string `json:"rule,omitempty"`
}

type SourceStatus struct {
	Source       string     `json:"source"`
	Label        string     `json:"label"`
	Status       string     `json:"status"`
	Connected    bool       `json:"connected"`
	LastSyncedAt *time.Time `json:"last_synced_at"`
	Detail       string     `json:"detail,omitempty"`
}

type Comparison struct {
	Range   RangeView        `json:"range"`
	Summary DashboardSummary `json:"summary"`
}

type Overview struct {
	Range          RangeView         `json:"range"`
	Today          DailyPoint        `json:"today"`
	PreviousDay    DailyPoint        `json:"previous_day"`
	Daily          []DailyPoint      `json:"daily"`
	Summary        DashboardSummary  `json:"summary"`
	Comparison     *Comparison       `json:"comparison"`
	Sessions       []json.RawMessage `json:"sessions"`
	TodaySessions  []json.RawMessage `json:"today_sessions"`
	WeeklySessions []json.RawMessage `json:"weekly_sessions"`
	WeeklyRange    RangeView         `json:"weekly_range"`
	Highlights     []Highlight       `json:"highlights"`
	Sources        []SourceStatus    `json:"sources"`
}

type Settings struct {
	Timezone              string          `json:"timezone"`
	Units                 string          `json:"units"`
	Theme                 string          `json:"theme"`
	FirstDayOfWeek        int             `json:"first_day_of_week"`
	CalorieTargetKcal     *float64        `json:"calorie_target_kcal"`
	ProteinTargetG        *float64        `json:"protein_target_g"`
	FatTargetG            *float64        `json:"fat_target_g"`
	CarbohydratesTargetG  *float64        `json:"carbohydrates_target_g"`
	SleepTargetMinSeconds *int            `json:"sleep_target_min_seconds"`
	SleepTargetMaxSeconds *int            `json:"sleep_target_max_seconds"`
	RecoveryRanges        json.RawMessage `json:"recovery_ranges"`
	UpdatedAt             *time.Time      `json:"updated_at"`
}

type ImportRequest struct {
	DataType string            `json:"data_type"`
	Filename string            `json:"filename"`
	Format   string            `json:"format"`
	Content  string            `json:"content"`
	Mapping  map[string]string `json:"mapping"`
	Source   string            `json:"source"`
}

type ImportPreview struct {
	DataType         string              `json:"data_type"`
	Format           string              `json:"format"`
	Columns          []string            `json:"columns"`
	TargetFields     []string            `json:"target_fields"`
	RequiredFields   []string            `json:"required_fields"`
	Rows             []map[string]string `json:"rows"`
	SuggestedMapping map[string]string   `json:"suggested_mapping"`
	Errors           []ImportRowError    `json:"errors"`
	DuplicateRows    int                 `json:"duplicate_rows"`
	TotalRows        int                 `json:"total_rows"`
	ValidRows        int                 `json:"valid_rows"`
	InvalidRows      int                 `json:"invalid_rows"`
}

type ImportRowError struct {
	Row     int               `json:"row"`
	Message string            `json:"message"`
	Fields  map[string]string `json:"fields,omitempty"`
}

type ImportRow struct {
	Row        int
	DataType   string
	Source     string
	ExternalID string
	Values     map[string]string
}

type ImportBatch struct {
	DataType   string
	Filename   string
	Format     string
	Source     string
	TotalRows  int
	FailedRows int
	Rows       []ImportRow
	Errors     []ImportRowError
}

type ImportResult struct {
	ID       int64            `json:"id"`
	Status   string           `json:"status"`
	Total    int              `json:"total_rows"`
	Imported int              `json:"imported_rows"`
	Skipped  int              `json:"skipped_rows"`
	Failed   int              `json:"failed_rows"`
	Errors   []ImportRowError `json:"errors"`
}

type DemoSeedResult struct {
	Days             int `json:"days"`
	WorkoutSessions  int `json:"workout_sessions"`
	RecoveryEntries  int `json:"recovery_entries"`
	SleepEntries     int `json:"sleep_entries"`
	NutritionEntries int `json:"nutrition_entries"`
	BodyMeasurements int `json:"body_measurements"`
}

type AnalyticsFilters struct {
	ExerciseID *int64
	PlanID     *int64
	TemplateID *int64
	Status     string
	DayType    string
}

type Store interface {
	Overview(context.Context, int64, DateRange, *time.Location) (Overview, error)
	List(context.Context, int64, string, Pagination, *time.Location) (ListResult, error)
	Get(context.Context, int64, string, int64, *time.Location) (json.RawMessage, error)
	Create(context.Context, int64, string, json.RawMessage, *time.Location) (json.RawMessage, error)
	Update(context.Context, int64, string, int64, json.RawMessage, *time.Location) (json.RawMessage, error)
	Delete(context.Context, int64, string, int64) error
	ExportSessionsCSV(context.Context, int64, DateRange, Pagination, *time.Location) ([]byte, error)
	Settings(context.Context, int64, string) (Settings, error)
	SaveSettings(context.Context, int64, Settings) (Settings, error)
	Sources(context.Context, int64) ([]SourceStatus, error)
	ExistingExternalIDs(context.Context, int64, string, string, []string) (map[string]struct{}, error)
	ExecuteImport(context.Context, int64, ImportBatch, *time.Location) (ImportResult, error)
	ExportAll(context.Context, int64, *time.Location) (json.RawMessage, error)
	DeleteAll(context.Context, int64) error
	SeedDemo(context.Context, int64, time.Time, *time.Location) (DemoSeedResult, error)
}
