package controlcenter

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

const recoveryJSON = `jsonb_build_object(
	'id', id, 'date', entry_date, 'recovery_score', recovery_score::double precision,
	'hrv_ms', hrv_ms::double precision, 'resting_heart_rate_bpm', resting_heart_rate_bpm::double precision,
	'respiratory_rate', respiratory_rate::double precision, 'spo2_percent', spo2_percent::double precision,
	'skin_temperature_celsius', skin_temperature_c::double precision, 'daily_strain', daily_strain::double precision,
	'source', source, 'external_id', external_id, 'notes', notes, 'created_at', created_at, 'updated_at', updated_at)`

const sleepJSON = `jsonb_build_object(
	'id', id, 'date', sleep_date, 'sleep_start', sleep_start, 'sleep_end', sleep_end, 'is_nap', is_nap,
	'time_in_bed_seconds', time_in_bed_seconds, 'actual_sleep_seconds', actual_sleep_seconds,
	'awake_seconds', awake_seconds, 'rem_seconds', rem_seconds, 'deep_seconds', deep_seconds,
	'light_seconds', light_seconds, 'sleep_performance_percent', sleep_performance_percent::double precision,
	'efficiency_percent', efficiency_percent::double precision, 'consistency_percent', consistency_percent::double precision,
	'sleep_debt_seconds', sleep_debt_seconds, 'disturbances', disturbances,
	'source', source, 'external_id', external_id, 'notes', notes, 'created_at', created_at, 'updated_at', updated_at)`

const nutritionJSON = `jsonb_build_object(
	'id', id, 'date', entry_date, 'calories_kcal', calories_kcal::double precision,
	'protein_g', protein_g::double precision, 'fat_g', fat_g::double precision,
	'carbohydrates_g', carbohydrates_g::double precision, 'fiber_g', fiber_g::double precision,
	'sugar_g', sugar_g::double precision, 'saturated_fat_g', saturated_fat_g::double precision,
	'sodium_mg', sodium_mg::double precision, 'potassium_mg', potassium_mg::double precision,
	'water_ml', water_ml::double precision, 'source', source, 'external_id', external_id,
	'notes', notes, 'created_at', created_at, 'updated_at', updated_at)`

const bodyJSON = `jsonb_build_object(
	'id', body_measurements.id, 'measured_at', measured_at, 'weight_kg', weight_kg::double precision,
	'body_fat_percent', body_fat_percent::double precision, 'fat_mass_kg', fat_mass_kg::double precision,
	'lean_mass_kg', lean_mass_kg::double precision, 'skeletal_muscle_mass_kg', skeletal_muscle_mass_kg::double precision,
	'total_body_water_l', total_body_water_l::double precision,
	'intracellular_water_l', intracellular_water_l::double precision,
	'extracellular_water_l', extracellular_water_l::double precision,
	'ecw_tbw_ratio', ecw_tbw_ratio::double precision, 'protein_mass_kg', protein_mass_kg::double precision,
	'mineral_mass_kg', mineral_mass_kg::double precision, 'bmi', bmi::double precision,
	'visceral_fat_level', visceral_fat_level::double precision,
	'visceral_fat_area_cm2', visceral_fat_area_cm2::double precision,
	'basal_metabolic_rate_kcal', basal_metabolic_rate_kcal::double precision,
	'inbody_score', inbody_score::double precision, 'phase_angle_degrees', phase_angle_degrees::double precision,
	'waist_cm', waist_cm::double precision, 'chest_cm', chest_cm::double precision,
	'biceps_cm', biceps_cm::double precision, 'thigh_cm', thigh_cm::double precision,
	'segments', COALESCE((SELECT jsonb_agg(jsonb_build_object(
		'segment', segment.segment, 'lean_mass_kg', segment.lean_mass_kg::double precision,
		'lean_percent', segment.lean_percent::double precision,
		'fat_mass_kg', segment.fat_mass_kg::double precision,
		'fat_percent', segment.fat_percent::double precision)
		ORDER BY CASE segment.segment WHEN 'left_arm' THEN 1 WHEN 'right_arm' THEN 2
			WHEN 'trunk' THEN 3 WHEN 'left_leg' THEN 4 ELSE 5 END)
		FROM body_segment_measurements segment
		WHERE segment.body_measurement_id=body_measurements.id
			AND segment.owner_id=body_measurements.owner_id), '[]'::jsonb),
	'source', source, 'external_id', external_id, 'notes', notes, 'created_at', created_at, 'updated_at', updated_at)`

type recoveryInput struct {
	Date                   string   `json:"date"`
	RecoveryScore          *float64 `json:"recovery_score"`
	HRVMs                  *float64 `json:"hrv_ms"`
	RestingHeartRateBPM    *float64 `json:"resting_heart_rate_bpm"`
	RespiratoryRate        *float64 `json:"respiratory_rate"`
	SpO2Percent            *float64 `json:"spo2_percent"`
	SkinTemperatureCelsius *float64 `json:"skin_temperature_celsius"`
	SkinTemperatureC       *float64 `json:"skin_temperature_c"`
	DailyStrain            *float64 `json:"daily_strain"`
	Source                 string   `json:"source"`
	ExternalID             *string  `json:"external_id"`
	Notes                  string   `json:"notes"`
}

type sleepInput struct {
	Date                    string   `json:"date"`
	SleepStart              *string  `json:"sleep_start"`
	SleepEnd                *string  `json:"sleep_end"`
	IsNap                   bool     `json:"is_nap"`
	TimeInBedSeconds        *int64   `json:"time_in_bed_seconds"`
	ActualSleepSeconds      *int64   `json:"actual_sleep_seconds"`
	AwakeSeconds            *int64   `json:"awake_seconds"`
	REMSeconds              *int64   `json:"rem_seconds"`
	DeepSeconds             *int64   `json:"deep_seconds"`
	LightSeconds            *int64   `json:"light_seconds"`
	SleepPerformancePercent *float64 `json:"sleep_performance_percent"`
	EfficiencyPercent       *float64 `json:"efficiency_percent"`
	ConsistencyPercent      *float64 `json:"consistency_percent"`
	SleepDebtSeconds        *int64   `json:"sleep_debt_seconds"`
	Disturbances            *int     `json:"disturbances"`
	Source                  string   `json:"source"`
	ExternalID              *string  `json:"external_id"`
	Notes                   string   `json:"notes"`
}

type nutritionInput struct {
	Date           string   `json:"date"`
	CaloriesKcal   *float64 `json:"calories_kcal"`
	ProteinG       *float64 `json:"protein_g"`
	FatG           *float64 `json:"fat_g"`
	CarbohydratesG *float64 `json:"carbohydrates_g"`
	FiberG         *float64 `json:"fiber_g"`
	SugarG         *float64 `json:"sugar_g"`
	SaturatedFatG  *float64 `json:"saturated_fat_g"`
	SodiumMg       *float64 `json:"sodium_mg"`
	PotassiumMg    *float64 `json:"potassium_mg"`
	WaterML        *float64 `json:"water_ml"`
	Source         string   `json:"source"`
	ExternalID     *string  `json:"external_id"`
	Notes          string   `json:"notes"`
}

type bodyInput struct {
	MeasuredAt             string                 `json:"measured_at"`
	Date                   string                 `json:"date"`
	WeightKG               *float64               `json:"weight_kg"`
	BodyFatPercent         *float64               `json:"body_fat_percent"`
	FatMassKG              *float64               `json:"fat_mass_kg"`
	LeanMassKG             *float64               `json:"lean_mass_kg"`
	SkeletalMuscleMassKG   *float64               `json:"skeletal_muscle_mass_kg"`
	TotalBodyWaterL        *float64               `json:"total_body_water_l"`
	IntracellularWaterL    *float64               `json:"intracellular_water_l"`
	ExtracellularWaterL    *float64               `json:"extracellular_water_l"`
	ECWTBWRatio            *float64               `json:"ecw_tbw_ratio"`
	ProteinMassKG          *float64               `json:"protein_mass_kg"`
	MineralMassKG          *float64               `json:"mineral_mass_kg"`
	BMI                    *float64               `json:"bmi"`
	VisceralFatLevel       *int                   `json:"visceral_fat_level"`
	VisceralFatAreaCM2     *float64               `json:"visceral_fat_area_cm2"`
	BasalMetabolicRateKcal *float64               `json:"basal_metabolic_rate_kcal"`
	InBodyScore            *float64               `json:"inbody_score"`
	PhaseAngleDegrees      *float64               `json:"phase_angle_degrees"`
	WaistCM                *float64               `json:"waist_cm"`
	ChestCM                *float64               `json:"chest_cm"`
	BicepsCM               *float64               `json:"biceps_cm"`
	ThighCM                *float64               `json:"thigh_cm"`
	Segments               *[]BodySegmentSnapshot `json:"segments"`
	Source                 string                 `json:"source"`
	ExternalID             *string                `json:"external_id"`
	Notes                  string                 `json:"notes"`
}

func (r *PostgresRepository) listRecords(
	ctx context.Context,
	ownerID int64,
	resource string,
	options Pagination,
	loc *time.Location,
) ([]json.RawMessage, int, error) {
	offset := (options.Page - 1) * options.PageSize
	from, to := dateFilterArgs(options)
	var expression, table, dateExpression string
	switch resource {
	case "recovery":
		expression, table, dateExpression = recoveryJSON, "recovery_entries", "entry_date"
	case "sleep":
		expression, table, dateExpression = sleepJSON, "sleep_entries", "sleep_date"
	case "nutrition":
		expression, table, dateExpression = nutritionJSON, "nutrition_days", "entry_date"
	case "body-measurements":
		expression, table, dateExpression = bodyJSON, "body_measurements", "(measured_at AT TIME ZONE $5)::date"
	default:
		return nil, 0, ErrNotFound
	}
	// Table/expression values are selected exclusively from the closed switch.
	query := fmt.Sprintf(`SELECT %s FROM %s WHERE owner_id=$1
		AND ($2::date IS NULL OR %s >= $2::date)
		AND ($3::date IS NULL OR %s <= $3::date)
		AND ($4::text = '' OR source = $4)
		AND $5::text <> ''
		ORDER BY %s DESC, id DESC LIMIT $6 OFFSET $7`, expression, table, dateExpression, dateExpression, dateExpression)
	rows, err := r.pool.Query(ctx, query, ownerID, from, to, options.Filters["source"], loc.String(), options.PageSize, offset)
	if err != nil {
		return nil, 0, err
	}
	items, err := collectJSONRows(rows)
	if err != nil {
		return nil, 0, err
	}
	countQuery := fmt.Sprintf(`SELECT count(*) FROM %s WHERE owner_id=$1
		AND ($2::date IS NULL OR %s >= $2::date)
		AND ($3::date IS NULL OR %s <= $3::date)
		AND ($4::text = '' OR source = $4)
		AND $5::text <> ''`, table, dateExpression, dateExpression)
	var total int
	err = r.pool.QueryRow(ctx, countQuery, ownerID, from, to, options.Filters["source"], loc.String()).Scan(&total)
	return items, total, err
}

func dateFilterArgs(options Pagination) (any, any) {
	var from, to any
	if options.From != nil {
		from = options.From.Format("2006-01-02")
	}
	if options.To != nil {
		to = options.To.Format("2006-01-02")
	}
	return from, to
}

func (r *PostgresRepository) getRecord(ctx context.Context, ownerID int64, resource string, id int64, _ *time.Location) (json.RawMessage, error) {
	var expression, table string
	switch resource {
	case "recovery":
		expression, table = recoveryJSON, "recovery_entries"
	case "sleep":
		expression, table = sleepJSON, "sleep_entries"
	case "nutrition":
		expression, table = nutritionJSON, "nutrition_days"
	case "body-measurements":
		expression, table = bodyJSON, "body_measurements"
	default:
		return nil, ErrNotFound
	}
	var raw json.RawMessage
	err := r.pool.QueryRow(ctx, fmt.Sprintf(`SELECT %s FROM %s WHERE owner_id=$1 AND id=$2`, expression, table), ownerID, id).Scan(&raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return raw, err
}

func (r *PostgresRepository) createRecord(ctx context.Context, ownerID int64, resource string, raw json.RawMessage, loc *time.Location) (json.RawMessage, error) {
	return r.saveRecord(ctx, ownerID, resource, 0, raw, loc)
}

func (r *PostgresRepository) updateRecord(ctx context.Context, ownerID int64, resource string, id int64, raw json.RawMessage, loc *time.Location) (json.RawMessage, error) {
	return r.saveRecord(ctx, ownerID, resource, id, raw, loc)
}

func (r *PostgresRepository) saveRecord(ctx context.Context, ownerID int64, resource string, id int64, raw json.RawMessage, loc *time.Location) (json.RawMessage, error) {
	var resultID int64
	var err error
	switch resource {
	case "recovery":
		var input recoveryInput
		if err := decodeStrict(raw, &input); err != nil {
			return nil, err
		}
		day, validation := parseDate(input.Date, loc, "date")
		if validation != nil {
			return nil, validation
		}
		if input.SkinTemperatureCelsius == nil {
			input.SkinTemperatureCelsius = input.SkinTemperatureC
		}
		input.Source = defaultSource(input.Source)
		resultID, err = upsertRecordID(ctx, r.pool, id, ownerID, "recovery_entries", `
			entry_date=$3,recovery_score=$4,hrv_ms=$5,resting_heart_rate_bpm=$6,
			respiratory_rate=$7,spo2_percent=$8,skin_temperature_c=$9,daily_strain=$10,
			source=$11,external_id=$12,notes=$13,updated_at=now()`, []any{day, input.RecoveryScore, input.HRVMs, input.RestingHeartRateBPM, input.RespiratoryRate, input.SpO2Percent, input.SkinTemperatureCelsius, input.DailyStrain, input.Source, cleanExternalID(input.ExternalID), input.Notes})
	case "sleep":
		var input sleepInput
		if err := decodeStrict(raw, &input); err != nil {
			return nil, err
		}
		day, validation := parseDate(input.Date, loc, "date")
		if validation != nil {
			return nil, validation
		}
		start, startErr := parseOptionalTime(input.SleepStart, loc, "sleep_start")
		if startErr != nil {
			return nil, startErr
		}
		end, endErr := parseOptionalTime(input.SleepEnd, loc, "sleep_end")
		if endErr != nil {
			return nil, endErr
		}
		input.Source = defaultSource(input.Source)
		resultID, err = upsertRecordID(ctx, r.pool, id, ownerID, "sleep_entries", `
			sleep_date=$3,sleep_start=$4,sleep_end=$5,is_nap=$6,time_in_bed_seconds=$7,
			actual_sleep_seconds=$8,awake_seconds=$9,rem_seconds=$10,deep_seconds=$11,light_seconds=$12,
			sleep_performance_percent=$13,efficiency_percent=$14,consistency_percent=$15,
			sleep_debt_seconds=$16,disturbances=$17,source=$18,external_id=$19,notes=$20,updated_at=now()`,
			[]any{day, start, end, input.IsNap, input.TimeInBedSeconds, input.ActualSleepSeconds, input.AwakeSeconds, input.REMSeconds, input.DeepSeconds, input.LightSeconds, input.SleepPerformancePercent, input.EfficiencyPercent, input.ConsistencyPercent, input.SleepDebtSeconds, input.Disturbances, input.Source, cleanExternalID(input.ExternalID), input.Notes})
	case "nutrition":
		var input nutritionInput
		if err := decodeStrict(raw, &input); err != nil {
			return nil, err
		}
		day, validation := parseDate(input.Date, loc, "date")
		if validation != nil {
			return nil, validation
		}
		input.Source = defaultSource(input.Source)
		resultID, err = upsertRecordID(ctx, r.pool, id, ownerID, "nutrition_days", `
			entry_date=$3,calories_kcal=$4,protein_g=$5,fat_g=$6,carbohydrates_g=$7,fiber_g=$8,
			sugar_g=$9,saturated_fat_g=$10,sodium_mg=$11,potassium_mg=$12,water_ml=$13,
			source=$14,external_id=$15,notes=$16,updated_at=now()`,
			[]any{day, input.CaloriesKcal, input.ProteinG, input.FatG, input.CarbohydratesG, input.FiberG, input.SugarG, input.SaturatedFatG, input.SodiumMg, input.PotassiumMg, input.WaterML, input.Source, cleanExternalID(input.ExternalID), input.Notes})
	case "body-measurements":
		var input bodyInput
		if err := decodeStrict(raw, &input); err != nil {
			return nil, err
		}
		if input.MeasuredAt == "" {
			input.MeasuredAt = input.Date
		}
		measuredAt, validation := parseRequiredTime(input.MeasuredAt, loc, "measured_at")
		if validation != nil {
			return nil, validation
		}
		if !bodyInputHasScalarMeasurement(input) {
			return nil, &ValidationError{Message: "invalid body measurement", Fields: map[string]string{"measurement": "provide at least one measurement"}}
		}
		if validation := validateInBodyInput(&input); validation != nil {
			return nil, validation
		}
		input.Source = defaultSource(input.Source)
		resultID, err = r.saveBodyMeasurement(ctx, ownerID, id, measuredAt, input)
	default:
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, mapPGError(err)
	}
	return r.getRecord(ctx, ownerID, resource, resultID, loc)
}

func (r *PostgresRepository) saveBodyMeasurement(
	ctx context.Context,
	ownerID int64,
	id int64,
	measuredAt time.Time,
	input bodyInput,
) (int64, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	resultID, err := upsertRecordID(ctx, tx, id, ownerID, "body_measurements", `
			measured_at=$3,weight_kg=$4,body_fat_percent=$5,fat_mass_kg=$6,lean_mass_kg=$7,
			skeletal_muscle_mass_kg=$8,total_body_water_l=$9,intracellular_water_l=$10,
			extracellular_water_l=$11,ecw_tbw_ratio=$12,protein_mass_kg=$13,mineral_mass_kg=$14,
			bmi=$15,visceral_fat_level=$16,visceral_fat_area_cm2=$17,basal_metabolic_rate_kcal=$18,
			inbody_score=$19,phase_angle_degrees=$20,waist_cm=$21,chest_cm=$22,biceps_cm=$23,thigh_cm=$24,
			source=$25,external_id=$26,notes=$27,updated_at=now()`,
		[]any{measuredAt, input.WeightKG, input.BodyFatPercent, input.FatMassKG, input.LeanMassKG,
			input.SkeletalMuscleMassKG, input.TotalBodyWaterL, input.IntracellularWaterL,
			input.ExtracellularWaterL, input.ECWTBWRatio, input.ProteinMassKG, input.MineralMassKG,
			input.BMI, input.VisceralFatLevel, input.VisceralFatAreaCM2, input.BasalMetabolicRateKcal,
			input.InBodyScore, input.PhaseAngleDegrees, input.WaistCM, input.ChestCM, input.BicepsCM,
			input.ThighCM, input.Source, cleanExternalID(input.ExternalID), input.Notes})
	if err != nil {
		return 0, err
	}
	if input.Segments != nil {
		if _, err := tx.Exec(ctx, `DELETE FROM body_segment_measurements
			WHERE body_measurement_id=$1 AND owner_id=$2`, resultID, ownerID); err != nil {
			return 0, err
		}
		for _, segment := range *input.Segments {
			if _, err := tx.Exec(ctx, `INSERT INTO body_segment_measurements (
				body_measurement_id,owner_id,segment,lean_mass_kg,lean_percent,fat_mass_kg,fat_percent)
				VALUES ($1,$2,$3,$4,$5,$6,$7)`, resultID, ownerID, segment.Segment,
				segment.LeanMassKG, segment.LeanPercent, segment.FatMassKG, segment.FatPercent); err != nil {
				return 0, err
			}
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, err
	}
	return resultID, nil
}

func bodyInputHasScalarMeasurement(input bodyInput) bool {
	return input.WeightKG != nil || input.BodyFatPercent != nil || input.FatMassKG != nil ||
		input.LeanMassKG != nil || input.SkeletalMuscleMassKG != nil || input.TotalBodyWaterL != nil ||
		input.IntracellularWaterL != nil || input.ExtracellularWaterL != nil || input.ECWTBWRatio != nil ||
		input.ProteinMassKG != nil || input.MineralMassKG != nil || input.BMI != nil ||
		input.VisceralFatLevel != nil || input.VisceralFatAreaCM2 != nil || input.BasalMetabolicRateKcal != nil ||
		input.InBodyScore != nil || input.PhaseAngleDegrees != nil || input.WaistCM != nil ||
		input.ChestCM != nil || input.BicepsCM != nil || input.ThighCM != nil
}

func validateInBodyInput(input *bodyInput) *ValidationError {
	fields := map[string]string{}
	validatePositiveBodyValue(fields, "total_body_water_l", input.TotalBodyWaterL)
	validatePositiveBodyValue(fields, "intracellular_water_l", input.IntracellularWaterL)
	validatePositiveBodyValue(fields, "extracellular_water_l", input.ExtracellularWaterL)
	validatePositiveBodyValue(fields, "bmi", input.BMI)
	validatePositiveBodyValue(fields, "basal_metabolic_rate_kcal", input.BasalMetabolicRateKcal)
	validateNonNegativeBodyValue(fields, "protein_mass_kg", input.ProteinMassKG)
	validateNonNegativeBodyValue(fields, "mineral_mass_kg", input.MineralMassKG)
	validateNonNegativeBodyValue(fields, "visceral_fat_area_cm2", input.VisceralFatAreaCM2)
	validateNonNegativeBodyValue(fields, "inbody_score", input.InBodyScore)
	if input.ECWTBWRatio != nil && (!finite(*input.ECWTBWRatio) || *input.ECWTBWRatio <= 0 || *input.ECWTBWRatio > 1) {
		fields["ecw_tbw_ratio"] = "must be greater than zero and at most one"
	}
	if input.VisceralFatLevel != nil && *input.VisceralFatLevel < 0 {
		fields["visceral_fat_level"] = "must be non-negative"
	}
	if input.PhaseAngleDegrees != nil && (!finite(*input.PhaseAngleDegrees) || *input.PhaseAngleDegrees <= 0 || *input.PhaseAngleDegrees >= 90) {
		fields["phase_angle_degrees"] = "must be greater than zero and less than 90"
	}
	if input.TotalBodyWaterL != nil {
		if input.IntracellularWaterL != nil && *input.IntracellularWaterL > *input.TotalBodyWaterL {
			fields["intracellular_water_l"] = "must not exceed total_body_water_l"
		}
		if input.ExtracellularWaterL != nil && *input.ExtracellularWaterL > *input.TotalBodyWaterL {
			fields["extracellular_water_l"] = "must not exceed total_body_water_l"
		}
	}
	if input.Segments != nil {
		seen := map[string]struct{}{}
		for index := range *input.Segments {
			segment := &(*input.Segments)[index]
			segment.Segment = strings.ToLower(strings.TrimSpace(segment.Segment))
			prefix := fmt.Sprintf("segments.%d", index)
			if !validBodySegment(segment.Segment) {
				fields[prefix+".segment"] = "use left_arm, right_arm, trunk, left_leg, or right_leg"
			}
			if _, duplicate := seen[segment.Segment]; duplicate {
				fields[prefix+".segment"] = "must be unique within a measurement"
			}
			seen[segment.Segment] = struct{}{}
			if segment.LeanMassKG == nil && segment.LeanPercent == nil && segment.FatMassKG == nil && segment.FatPercent == nil {
				fields[prefix] = "provide at least one segment measurement"
			}
			validateNonNegativeBodyValue(fields, prefix+".lean_mass_kg", segment.LeanMassKG)
			validateNonNegativeBodyValue(fields, prefix+".lean_percent", segment.LeanPercent)
			validateNonNegativeBodyValue(fields, prefix+".fat_mass_kg", segment.FatMassKG)
			validateNonNegativeBodyValue(fields, prefix+".fat_percent", segment.FatPercent)
		}
	}
	if len(fields) > 0 {
		return &ValidationError{Message: "invalid InBody measurement", Fields: fields}
	}
	return nil
}

func validatePositiveBodyValue(fields map[string]string, field string, value *float64) {
	if value != nil && (!finite(*value) || *value <= 0) {
		fields[field] = "must be a positive finite number"
	}
}

func validateNonNegativeBodyValue(fields map[string]string, field string, value *float64) {
	if value != nil && (!finite(*value) || *value < 0) {
		fields[field] = "must be a non-negative finite number"
	}
}

func finite(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0)
}

func validBodySegment(segment string) bool {
	switch segment {
	case "left_arm", "right_arm", "trunk", "left_leg", "right_leg":
		return true
	default:
		return false
	}
}

type rowQuerier interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}

func upsertRecordID(ctx context.Context, db rowQuerier, id, ownerID int64, table, assignments string, values []any) (int64, error) {
	args := []any{id, ownerID}
	args = append(args, values...)
	if id > 0 {
		var result int64
		query := fmt.Sprintf(`UPDATE %s SET %s WHERE id=$1 AND owner_id=$2 RETURNING id`, table, assignments)
		err := db.QueryRow(ctx, query, args...).Scan(&result)
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, ErrNotFound
		}
		return result, err
	}
	columns, placeholders := assignmentColumns(assignments)
	insertArgs := append([]any{ownerID}, values...)
	query := fmt.Sprintf(`INSERT INTO %s (owner_id,%s) VALUES ($1,%s) RETURNING id`, table, strings.Join(columns, ","), strings.Join(placeholders, ","))
	var result int64
	return result, db.QueryRow(ctx, query, insertArgs...).Scan(&result)
}

func assignmentColumns(assignments string) ([]string, []string) {
	parts := strings.Split(assignments, ",")
	columns := make([]string, 0, len(parts))
	placeholders := make([]string, 0, len(parts))
	for _, part := range parts {
		pieces := strings.SplitN(strings.TrimSpace(part), "=", 2)
		if len(pieces) != 2 || strings.Contains(pieces[0], "updated_at") {
			continue
		}
		columns = append(columns, strings.TrimSpace(pieces[0]))
		placeholders = append(placeholders, fmt.Sprintf("$%d", len(columns)+1))
	}
	return columns, placeholders
}

func decodeStrict(raw json.RawMessage, target any) error {
	decoder := json.NewDecoder(strings.NewReader(string(raw)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return &ValidationError{Message: "invalid JSON payload", Fields: map[string]string{"body": err.Error()}}
	}
	return nil
}

func parseDate(raw string, loc *time.Location, field string) (time.Time, error) {
	value, err := time.ParseInLocation("2006-01-02", strings.TrimSpace(raw), loc)
	if err != nil {
		return time.Time{}, &ValidationError{Message: "invalid date", Fields: map[string]string{field: "use YYYY-MM-DD"}}
	}
	return value, nil
}

func parseRequiredTime(raw string, loc *time.Location, field string) (time.Time, error) {
	if strings.TrimSpace(raw) == "" {
		return time.Time{}, &ValidationError{Message: "missing timestamp", Fields: map[string]string{field: "is required"}}
	}
	if value, err := time.Parse(time.RFC3339, raw); err == nil {
		return value, nil
	}
	if value, err := time.ParseInLocation("2006-01-02", raw, loc); err == nil {
		return value, nil
	}
	if value, err := time.ParseInLocation("2006-01-02T15:04", raw, loc); err == nil {
		return value, nil
	}
	if value, err := time.ParseInLocation("2006-01-02T15:04:05", raw, loc); err == nil {
		return value, nil
	}
	if value, err := time.ParseInLocation("2006-01-02 15:04:05", raw, loc); err == nil {
		return value, nil
	}
	return time.Time{}, &ValidationError{Message: "invalid timestamp", Fields: map[string]string{field: "use RFC3339, YYYY-MM-DD, or a local date-time"}}
}

func parseOptionalTime(raw *string, loc *time.Location, field string) (*time.Time, error) {
	if raw == nil || strings.TrimSpace(*raw) == "" {
		return nil, nil
	}
	value, err := parseRequiredTime(*raw, loc, field)
	if err != nil {
		return nil, err
	}
	return &value, nil
}

func defaultSource(source string) string {
	source = strings.TrimSpace(source)
	if source == "" {
		return "manual"
	}
	return source
}

func cleanExternalID(value *string) *string {
	if value == nil || strings.TrimSpace(*value) == "" {
		return nil
	}
	cleaned := strings.TrimSpace(*value)
	return &cleaned
}
