package controlcenter

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"
)

var importSchemas = map[string][]string{
	"recovery":  {"date", "recovery_score", "hrv_ms", "resting_heart_rate_bpm", "respiratory_rate", "spo2_percent", "skin_temperature_c", "daily_strain", "external_id", "notes"},
	"sleep":     {"date", "sleep_start", "sleep_end", "is_nap", "time_in_bed_seconds", "actual_sleep_seconds", "awake_seconds", "rem_seconds", "deep_seconds", "light_seconds", "sleep_performance_percent", "efficiency_percent", "consistency_percent", "sleep_debt_seconds", "disturbances", "external_id", "notes"},
	"nutrition": {"date", "calories_kcal", "protein_g", "fat_g", "carbohydrates_g", "fiber_g", "sugar_g", "saturated_fat_g", "sodium_mg", "potassium_mg", "water_ml", "external_id", "notes"},
	"body": {
		"measured_at", "weight_kg", "body_fat_percent", "fat_mass_kg", "lean_mass_kg", "skeletal_muscle_mass_kg",
		"total_body_water_l", "intracellular_water_l", "extracellular_water_l", "ecw_tbw_ratio",
		"protein_mass_kg", "mineral_mass_kg", "bmi", "visceral_fat_level", "visceral_fat_area_cm2",
		"basal_metabolic_rate_kcal", "inbody_score", "phase_angle_degrees",
		"waist_cm", "chest_cm", "biceps_cm", "thigh_cm",
		"left_arm_lean_mass_kg", "left_arm_lean_percent", "left_arm_fat_mass_kg", "left_arm_fat_percent",
		"right_arm_lean_mass_kg", "right_arm_lean_percent", "right_arm_fat_mass_kg", "right_arm_fat_percent",
		"trunk_lean_mass_kg", "trunk_lean_percent", "trunk_fat_mass_kg", "trunk_fat_percent",
		"left_leg_lean_mass_kg", "left_leg_lean_percent", "left_leg_fat_mass_kg", "left_leg_fat_percent",
		"right_leg_lean_mass_kg", "right_leg_lean_percent", "right_leg_fat_mass_kg", "right_leg_fat_percent",
		"external_id", "notes",
	},
	"workouts": {"program_name", "scheduled_at", "started_at", "finished_at", "status", "strain", "external_id", "notes"},
	"sets":     {"session_external_id", "exercise_name", "position", "type", "weight_kg", "reps", "rir", "rest_seconds", "completed_at", "external_id", "notes"},
}

var requiredImportFields = map[string][]string{
	"recovery": {"date"}, "sleep": {"date"}, "nutrition": {"date"},
	"body": {"measured_at"}, "workouts": {"program_name", "started_at"},
	"sets": {"session_external_id", "exercise_name", "type", "reps"},
}

func parseImport(request ImportRequest) (ImportBatch, ImportPreview, error) {
	request.DataType = strings.ToLower(strings.TrimSpace(request.DataType))
	request.Format = strings.ToLower(strings.TrimSpace(request.Format))
	request.Source = strings.TrimSpace(request.Source)
	if request.Source == "" {
		request.Source = "file"
	}
	if _, ok := importSchemas[request.DataType]; !ok {
		return ImportBatch{}, ImportPreview{}, &ValidationError{Message: "unsupported import type", Fields: map[string]string{"data_type": "use recovery, sleep, nutrition, body, workouts, or sets"}}
	}
	if request.Format != "csv" && request.Format != "json" {
		return ImportBatch{}, ImportPreview{}, &ValidationError{Message: "unsupported import format", Fields: map[string]string{"format": "use csv or json"}}
	}
	if len(request.Content) > MaxImportSize {
		return ImportBatch{}, ImportPreview{}, &ValidationError{Message: "import is too large", Fields: map[string]string{"content": "maximum size is 5 MiB"}}
	}
	rows, columns, err := decodeImportRows(request.Format, request.Content)
	if err != nil {
		return ImportBatch{}, ImportPreview{}, &ValidationError{Message: "cannot parse import", Fields: map[string]string{"content": err.Error()}}
	}
	mapping := suggestedMapping(request.DataType, columns)
	explicitMapping := make(map[string]string, len(request.Mapping))
	for target, source := range request.Mapping {
		target = normalizeColumn(target)
		source = strings.TrimSpace(source)
		mapping[target] = source
		explicitMapping[target] = source
	}
	batch := ImportBatch{DataType: request.DataType, Filename: request.Filename, Format: request.Format, Source: request.Source, TotalRows: len(rows)}
	preview := ImportPreview{
		DataType: request.DataType, Format: request.Format, Columns: columns,
		TargetFields:     append([]string(nil), importSchemas[request.DataType]...),
		RequiredFields:   append([]string(nil), requiredImportFields[request.DataType]...),
		SuggestedMapping: mapping, TotalRows: len(rows),
	}
	for index, raw := range rows {
		values := make(map[string]string)
		for _, target := range importSchemas[request.DataType] {
			source, explicit := explicitMapping[target]
			if !explicit {
				source = mapping[target]
				if source == "" {
					source = target
				}
			}
			if source != "" {
				values[target] = strings.TrimSpace(raw[source])
			} else {
				values[target] = ""
			}
		}
		rowError := validateImportRow(request.DataType, index+1, values)
		if rowError != nil {
			batch.FailedRows++
			if len(batch.Errors) < MaxImportErrorSamples {
				batch.Errors = append(batch.Errors, *rowError)
			}
			if len(preview.Errors) < MaxImportErrorSamples {
				preview.Errors = append(preview.Errors, *rowError)
			}
			continue
		}
		// PostgreSQL numeric input uses a dot decimal separator. Validation accepts
		// comma decimals for common CSV exports, so persist the canonical form that
		// was actually validated instead of forwarding the raw locale spelling.
		for _, field := range numericImportFields(request.DataType) {
			if values[field] == "" {
				continue
			}
			parsed, _ := importNumber(values[field])
			values[field] = strconv.FormatFloat(parsed, 'f', -1, 64)
		}
		batch.Rows = append(batch.Rows, ImportRow{
			Row: index + 1, DataType: request.DataType, Source: request.Source,
			ExternalID: values["external_id"], Values: values,
		})
		if len(preview.Rows) < 10 {
			preview.Rows = append(preview.Rows, values)
		}
	}
	preview.ValidRows = len(batch.Rows)
	preview.InvalidRows = batch.FailedRows
	return batch, preview, nil
}

func decodeImportRows(format, content string) ([]map[string]string, []string, error) {
	if strings.TrimSpace(content) == "" {
		return nil, nil, fmt.Errorf("content is empty")
	}
	if format == "json" {
		var rawRows []map[string]any
		if err := json.Unmarshal([]byte(content), &rawRows); err != nil {
			var envelope struct {
				Rows []map[string]any `json:"rows"`
			}
			if envelopeErr := json.Unmarshal([]byte(content), &envelope); envelopeErr != nil {
				return nil, nil, err
			}
			rawRows = envelope.Rows
		}
		columnSet := map[string]struct{}{}
		rows := make([]map[string]string, 0, len(rawRows))
		for _, raw := range rawRows {
			row := make(map[string]string, len(raw))
			for key, value := range raw {
				column := normalizeColumn(key)
				columnSet[column] = struct{}{}
				switch typed := value.(type) {
				case nil:
					row[column] = ""
				case string:
					row[column] = typed
				case float64:
					row[column] = strconv.FormatFloat(typed, 'f', -1, 64)
				case bool:
					row[column] = strconv.FormatBool(typed)
				default:
					encoded, _ := json.Marshal(typed)
					row[column] = string(encoded)
				}
			}
			rows = append(rows, row)
		}
		columns := make([]string, 0, len(columnSet))
		for column := range columnSet {
			columns = append(columns, column)
		}
		sort.Strings(columns)
		return rows, columns, nil
	}

	delimiter := ','
	firstLine := content
	if index := strings.IndexByte(content, '\n'); index >= 0 {
		firstLine = content[:index]
	}
	if strings.Count(firstLine, ";") > strings.Count(firstLine, ",") {
		delimiter = ';'
	}
	reader := csv.NewReader(bytes.NewBufferString(content))
	reader.Comma = delimiter
	reader.TrimLeadingSpace = true
	reader.FieldsPerRecord = -1
	header, err := reader.Read()
	if err != nil {
		return nil, nil, err
	}
	columns := make([]string, len(header))
	for index, value := range header {
		columns[index] = normalizeColumn(value)
	}
	var rows []map[string]string
	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, nil, err
		}
		row := make(map[string]string, len(columns))
		for index, column := range columns {
			if index < len(record) {
				row[column] = record[index]
			}
		}
		rows = append(rows, row)
	}
	return rows, columns, nil
}

func suggestedMapping(dataType string, columns []string) map[string]string {
	aliases := map[string]string{
		"day": "date", "entry_date": "date", "sleep_date": "date", "timestamp": "measured_at",
		"calories": "calories_kcal", "protein": "protein_g", "fat": "fat_g", "carbs": "carbohydrates_g",
		"carbohydrate": "carbohydrates_g", "weight": "weight_kg", "body_fat": "body_fat_percent",
		"hrv": "hrv_ms", "rhr": "resting_heart_rate_bpm", "recovery": "recovery_score",
		"skin_temperature_celsius": "skin_temperature_c",
		"session_id":               "session_external_id", "exercise": "exercise_name", "comment": "notes",
	}
	if dataType == "body" {
		for alias, target := range map[string]string{
			"tbw": "total_body_water_l", "total_body_water": "total_body_water_l",
			"icw": "intracellular_water_l", "intracellular_water": "intracellular_water_l",
			"ecw": "extracellular_water_l", "extracellular_water": "extracellular_water_l",
			"ecw_tbw": "ecw_tbw_ratio", "protein": "protein_mass_kg", "minerals": "mineral_mass_kg",
			"visceral_fat_area": "visceral_fat_area_cm2", "vfa": "visceral_fat_area_cm2",
			"bmr": "basal_metabolic_rate_kcal", "score": "inbody_score", "phase_angle": "phase_angle_degrees",
		} {
			aliases[alias] = target
		}
	}
	allowed := make(map[string]struct{}, len(importSchemas[dataType]))
	for _, field := range importSchemas[dataType] {
		allowed[field] = struct{}{}
	}
	mapping := make(map[string]string)
	for _, column := range columns {
		target := column
		if alias := aliases[column]; alias != "" {
			target = alias
		}
		if _, ok := allowed[target]; ok {
			mapping[target] = column
		}
	}
	return mapping
}

func validateImportRow(dataType string, row int, values map[string]string) *ImportRowError {
	fields := map[string]string{}
	for _, field := range requiredImportFields[dataType] {
		if strings.TrimSpace(values[field]) == "" {
			fields[field] = "is required"
		}
	}
	dateFields := []string{"date", "measured_at", "scheduled_at", "started_at", "finished_at", "sleep_start", "sleep_end", "completed_at"}
	for _, field := range dateFields {
		value := values[field]
		if value == "" {
			continue
		}
		if !validImportDate(value) {
			fields[field] = "use YYYY-MM-DD or RFC3339"
		}
	}
	if setType := values["type"]; setType != "" && setType != "warmup" && setType != "working" && setType != "drop" {
		fields["type"] = "use warmup, working, or drop"
	}
	if isNap := values["is_nap"]; isNap != "" {
		if _, err := strconv.ParseBool(isNap); err != nil {
			fields["is_nap"] = "must be true or false"
		}
	}
	for _, field := range numericImportFields(dataType) {
		if value := values[field]; value != "" {
			parsed, err := importNumber(value)
			if err != nil || parsed < 0 {
				fields[field] = "must be a non-negative number"
			}
		}
	}
	for field, maximum := range map[string]float64{
		"recovery_score": 100, "spo2_percent": 100, "daily_strain": 21,
		"sleep_performance_percent": 100, "efficiency_percent": 100,
		"consistency_percent": 100, "body_fat_percent": 100, "strain": 21, "ecw_tbw_ratio": 1,
	} {
		if value := values[field]; value != "" {
			if parsed, err := importNumber(value); err == nil && parsed > maximum {
				fields[field] = fmt.Sprintf("must be at most %s", strconv.FormatFloat(maximum, 'f', -1, 64))
			}
		}
	}
	for _, field := range integerImportFields(dataType) {
		if value := values[field]; value != "" {
			parsed, err := importNumber(value)
			if err == nil && parsed != float64(int64(parsed)) {
				fields[field] = "must be a whole number"
			}
		}
	}
	for _, field := range positiveImportFields(dataType) {
		if value := values[field]; value != "" {
			if parsed, err := importNumber(value); err == nil && parsed <= 0 {
				fields[field] = "must be greater than zero"
			}
		}
	}
	if dataType == "workouts" {
		status := strings.TrimSpace(values["status"])
		if status != "" && status != "active" && status != "finished" {
			fields["status"] = "use active or finished"
		}
		validateImportTimeOrder(fields, values, "started_at", "finished_at")
	}
	if dataType == "sleep" {
		validateImportTimeOrder(fields, values, "sleep_start", "sleep_end")
	}
	if dataType == "body" {
		hasMeasurement := false
		for _, field := range bodyScalarImportFields() {
			if strings.TrimSpace(values[field]) != "" {
				hasMeasurement = true
				break
			}
		}
		if !hasMeasurement {
			fields["measurement"] = "provide at least one body measurement"
		}
		if value := values["phase_angle_degrees"]; value != "" {
			if parsed, err := importNumber(value); err == nil && parsed >= 90 {
				fields["phase_angle_degrees"] = "must be less than 90"
			}
		}
		if total, totalErr := importNumber(values["total_body_water_l"]); values["total_body_water_l"] != "" && totalErr == nil {
			for _, field := range []string{"intracellular_water_l", "extracellular_water_l"} {
				if value, err := importNumber(values[field]); values[field] != "" && err == nil && value > total {
					fields[field] = "must not exceed total_body_water_l"
				}
			}
		}
	}
	if len(fields) == 0 {
		return nil
	}
	return &ImportRowError{Row: row, Message: "row is invalid", Fields: fields}
}

func importNumber(value string) (float64, error) {
	parsed, err := strconv.ParseFloat(strings.ReplaceAll(strings.TrimSpace(value), ",", "."), 64)
	if err == nil && (math.IsNaN(parsed) || math.IsInf(parsed, 0)) {
		return 0, fmt.Errorf("number must be finite")
	}
	return parsed, err
}

func integerImportFields(dataType string) []string {
	fields := map[string][]string{
		"sleep": {"time_in_bed_seconds", "actual_sleep_seconds", "awake_seconds", "rem_seconds", "deep_seconds", "light_seconds", "sleep_debt_seconds", "disturbances"},
		"sets":  {"position", "reps", "rest_seconds"},
		"body":  {"visceral_fat_level"},
	}
	return fields[dataType]
}

func positiveImportFields(dataType string) []string {
	fields := map[string][]string{
		"recovery": {"resting_heart_rate_bpm", "respiratory_rate"},
		"body": {"weight_kg", "total_body_water_l", "intracellular_water_l", "extracellular_water_l",
			"ecw_tbw_ratio", "bmi", "basal_metabolic_rate_kcal", "phase_angle_degrees",
			"waist_cm", "chest_cm", "biceps_cm", "thigh_cm"},
		"sets": {"position", "reps"},
	}
	return fields[dataType]
}

func validateImportTimeOrder(fields map[string]string, values map[string]string, startField, endField string) {
	start, startOK := comparableImportTime(values[startField])
	end, endOK := comparableImportTime(values[endField])
	if startOK && endOK && end.Before(start) {
		fields[endField] = "must not be before " + startField
	}
}

func comparableImportTime(raw string) (time.Time, bool) {
	for _, layout := range []string{time.RFC3339, "2006-01-02T15:04", "2006-01-02T15:04:05", "2006-01-02 15:04:05", "2006-01-02"} {
		if value, err := time.Parse(layout, strings.TrimSpace(raw)); err == nil {
			return value, true
		}
	}
	return time.Time{}, false
}

func validImportDate(value string) bool {
	for _, layout := range []string{"2006-01-02", time.RFC3339, "2006-01-02T15:04", "2006-01-02T15:04:05", "2006-01-02 15:04:05"} {
		if _, err := time.Parse(layout, value); err == nil {
			return true
		}
	}
	return false
}

func numericImportFields(dataType string) []string {
	fields := map[string][]string{
		"recovery":  {"recovery_score", "hrv_ms", "resting_heart_rate_bpm", "respiratory_rate", "spo2_percent", "daily_strain"},
		"sleep":     {"time_in_bed_seconds", "actual_sleep_seconds", "awake_seconds", "rem_seconds", "deep_seconds", "light_seconds", "sleep_performance_percent", "efficiency_percent", "consistency_percent", "sleep_debt_seconds", "disturbances"},
		"nutrition": {"calories_kcal", "protein_g", "fat_g", "carbohydrates_g", "fiber_g", "sugar_g", "saturated_fat_g", "sodium_mg", "potassium_mg", "water_ml"},
		"body": append(bodyScalarImportFields(),
			"left_arm_lean_mass_kg", "left_arm_lean_percent", "left_arm_fat_mass_kg", "left_arm_fat_percent",
			"right_arm_lean_mass_kg", "right_arm_lean_percent", "right_arm_fat_mass_kg", "right_arm_fat_percent",
			"trunk_lean_mass_kg", "trunk_lean_percent", "trunk_fat_mass_kg", "trunk_fat_percent",
			"left_leg_lean_mass_kg", "left_leg_lean_percent", "left_leg_fat_mass_kg", "left_leg_fat_percent",
			"right_leg_lean_mass_kg", "right_leg_lean_percent", "right_leg_fat_mass_kg", "right_leg_fat_percent"),
		"workouts": {"strain"}, "sets": {"position", "weight_kg", "reps", "rir", "rest_seconds"},
	}
	return fields[dataType]
}

func bodyScalarImportFields() []string {
	return []string{
		"weight_kg", "body_fat_percent", "fat_mass_kg", "lean_mass_kg", "skeletal_muscle_mass_kg",
		"total_body_water_l", "intracellular_water_l", "extracellular_water_l", "ecw_tbw_ratio",
		"protein_mass_kg", "mineral_mass_kg", "bmi", "visceral_fat_level", "visceral_fat_area_cm2",
		"basal_metabolic_rate_kcal", "inbody_score", "phase_angle_degrees",
		"waist_cm", "chest_cm", "biceps_cm", "thigh_cm",
	}
}

func normalizeColumn(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	replacer := strings.NewReplacer(" ", "_", "-", "_", ".", "_", "/", "_", "ё", "е")
	return replacer.Replace(value)
}
