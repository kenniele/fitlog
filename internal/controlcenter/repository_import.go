package controlcenter

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

const compositeExternalSeparator = "\x1f"

var demoTemplateExercises = map[string][]string{
	"A": {
		"Тяга вертикального блока широким хватом",
		"Жим штанги лёжа",
		"Тяга штанги к поясу",
		"Махи гантелями в стороны",
		"Разгибания рук в блоке",
	},
	"B": {
		"Тяга вертикального блока нейтральным хватом",
		"Жим от груди в наклонном тренажёре",
		"Горизонтальная тяга",
		"Жим сидя над головой",
		"Сгибания рук с супинацией",
	},
	"C": {
		"Разгибания ног",
		"Сгибания ног",
		"Обратные разведения",
	},
}

func (r *PostgresRepository) ExistingExternalIDs(ctx context.Context, ownerID int64, dataType, source string, ids []string) (map[string]struct{}, error) {
	result := make(map[string]struct{})
	if len(ids) == 0 {
		return result, nil
	}
	var query string
	switch dataType {
	case "recovery":
		query = `SELECT external_id FROM recovery_entries WHERE owner_id=$1 AND source=$2 AND external_id=ANY($3::text[])`
	case "sleep":
		query = `SELECT external_id FROM sleep_entries WHERE owner_id=$1 AND source=$2 AND external_id=ANY($3::text[])`
	case "nutrition":
		query = `SELECT external_id FROM nutrition_days WHERE owner_id=$1 AND source=$2 AND external_id=ANY($3::text[])`
	case "body":
		query = `SELECT external_id FROM body_measurements WHERE owner_id=$1 AND source=$2 AND external_id=ANY($3::text[])`
	case "workouts":
		query = `SELECT external_id FROM training_sessions WHERE owner_id=$1 AND source=$2 AND external_id=ANY($3::text[])`
	case "sets":
		query = `SELECT session.external_id || chr(31) || training_set.external_id
			FROM training_sets training_set
			JOIN training_session_exercises exercise ON exercise.id=training_set.session_exercise_id
			JOIN training_sessions session ON session.id=exercise.session_id
			WHERE session.owner_id=$1 AND training_set.source=$2
				AND session.external_id || chr(31) || training_set.external_id=ANY($3::text[])`
	default:
		return nil, &ValidationError{Message: "unsupported import type", Fields: map[string]string{"data_type": "is not supported"}}
	}
	rows, err := r.pool.Query(ctx, query, ownerID, source, ids)
	if err != nil {
		return nil, fmt.Errorf("find import duplicates: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var externalID string
		if err := rows.Scan(&externalID); err != nil {
			return nil, err
		}
		result[externalID] = struct{}{}
	}
	return result, rows.Err()
}

func (r *PostgresRepository) ExecuteImport(ctx context.Context, ownerID int64, batch ImportBatch, loc *time.Location) (ImportResult, error) {
	totalRows := batch.TotalRows
	if totalRows == 0 {
		totalRows = len(batch.Rows) + batch.FailedRows
		if totalRows == 0 {
			totalRows = len(batch.Rows) + len(batch.Errors)
		}
	}
	failedRows := batch.FailedRows
	if failedRows == 0 {
		failedRows = len(batch.Errors)
	}
	result := ImportResult{
		Status: "completed", Total: totalRows, Failed: failedRows,
		Errors: append([]ImportRowError(nil), batch.Errors...),
	}
	// Journal creation is deliberately committed before the data transaction.
	// A process crash or database failure must leave an honest running/failed
	// record instead of rolling the entire audit trail back with imported rows.
	if err := r.pool.QueryRow(ctx, `INSERT INTO data_imports (
		owner_id,source,data_type,filename,format,status,total_rows,failed_rows,error_summary)
		VALUES ($1,$2,$3,$4,$5,'running',$6,$7,$8::jsonb) RETURNING id`, ownerID,
		batch.Source, batch.DataType, batch.Filename, batch.Format, result.Total, result.Failed,
		mustJSON(result.Errors)).Scan(&result.ID); err != nil {
		return ImportResult{}, fmt.Errorf("start import journal: %w", err)
	}
	fail := func(cause error) (ImportResult, error) {
		result.Status = "failed"
		if len(result.Errors) < MaxImportErrorSamples {
			result.Errors = append(result.Errors, ImportRowError{Message: cause.Error()})
		}
		failureCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		_, _ = r.pool.Exec(failureCtx, `UPDATE data_imports SET status='failed',imported_rows=$2,
			skipped_rows=$3,failed_rows=$4,error_summary=$5::jsonb,completed_at=now() WHERE id=$1`,
			result.ID, result.Imported, result.Skipped, result.Failed, mustJSON(result.Errors))
		return result, cause
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fail(fmt.Errorf("begin import: %w", err))
	}
	defer func() { _ = tx.Rollback(ctx) }()
	failTransaction := func(cause error) (ImportResult, error) {
		_ = tx.Rollback(context.WithoutCancel(ctx))
		// The outer transaction is atomic: none of the apparent per-row inserts
		// survived. Do not publish optimistic counters from rolled-back work.
		result.Imported = 0
		result.Skipped = 0
		result.Failed = result.Total
		return fail(cause)
	}
	for _, row := range batch.Rows {
		rowTx, beginErr := tx.Begin(ctx)
		if beginErr != nil {
			return failTransaction(fmt.Errorf("begin import row %d: %w", row.Row, beginErr))
		}
		inserted, rowErr := r.importRow(ctx, rowTx, ownerID, row, loc)
		if rowErr != nil {
			_ = rowTx.Rollback(ctx)
			if isImportRowError(rowErr) {
				result.Failed++
				if len(result.Errors) < MaxImportErrorSamples {
					result.Errors = append(result.Errors, ImportRowError{Row: row.Row, Message: rowErr.Error()})
				}
				continue
			}
			return failTransaction(fmt.Errorf("import row %d: %w", row.Row, rowErr))
		}
		if commitErr := rowTx.Commit(ctx); commitErr != nil {
			return failTransaction(fmt.Errorf("commit import row %d: %w", row.Row, commitErr))
		}
		if inserted {
			result.Imported++
		} else {
			result.Skipped++
		}
	}
	if result.Imported == 0 && result.Failed > 0 {
		result.Status = "failed"
	}
	// The terminal journal state commits atomically with imported rows. A client
	// disconnect after Commit can no longer leave durable data behind a forever
	// "running" journal entry.
	if _, err := tx.Exec(ctx, `UPDATE data_imports SET status=$2,imported_rows=$3,
		skipped_rows=$4,failed_rows=$5,error_summary=$6::jsonb,completed_at=now() WHERE id=$1`,
		result.ID, result.Status, result.Imported, result.Skipped, result.Failed, mustJSON(result.Errors)); err != nil {
		return failTransaction(fmt.Errorf("finish import journal: %w", err))
	}
	if err := tx.Commit(ctx); err != nil {
		return failTransaction(fmt.Errorf("commit import: %w", err))
	}
	return result, nil
}

func isImportRowError(err error) bool {
	return errors.Is(err, ErrValidation) || errors.Is(err, ErrNotFound) || errors.Is(err, ErrConflict)
}

func mustJSON(value any) []byte {
	encoded, err := json.Marshal(value)
	if err != nil {
		return []byte(`[]`)
	}
	return encoded
}

func (r *PostgresRepository) importRow(ctx context.Context, tx pgx.Tx, ownerID int64, row ImportRow, loc *time.Location) (bool, error) {
	source := defaultSource(row.Source)
	externalID := cleanExternalID(&row.ExternalID)
	values := row.Values
	switch row.DataType {
	case "recovery":
		day, err := parseDate(values["date"], loc, "date")
		if err != nil {
			return false, err
		}
		command, err := tx.Exec(ctx, `INSERT INTO recovery_entries (
			owner_id,entry_date,recovery_score,hrv_ms,resting_heart_rate_bpm,respiratory_rate,
			spo2_percent,skin_temperature_c,daily_strain,source,external_id,notes)
			VALUES ($1,$2,NULLIF($3,'')::numeric,NULLIF($4,'')::numeric,NULLIF($5,'')::numeric,
			NULLIF($6,'')::numeric,NULLIF($7,'')::numeric,NULLIF($8,'')::numeric,NULLIF($9,'')::numeric,
			$10,$11,$12) ON CONFLICT DO NOTHING`, ownerID, day, values["recovery_score"], values["hrv_ms"],
			values["resting_heart_rate_bpm"], values["respiratory_rate"], values["spo2_percent"],
			values["skin_temperature_c"], values["daily_strain"], source, externalID, values["notes"])
		return command.RowsAffected() > 0, mapPGError(err)
	case "sleep":
		day, err := parseDate(values["date"], loc, "date")
		if err != nil {
			return false, err
		}
		start, err := parseImportTime(values["sleep_start"], loc, "sleep_start")
		if err != nil {
			return false, err
		}
		end, err := parseImportTime(values["sleep_end"], loc, "sleep_end")
		if err != nil {
			return false, err
		}
		command, err := tx.Exec(ctx, `INSERT INTO sleep_entries (
			owner_id,sleep_date,sleep_start,sleep_end,is_nap,time_in_bed_seconds,actual_sleep_seconds,
			awake_seconds,rem_seconds,deep_seconds,light_seconds,sleep_performance_percent,
			efficiency_percent,consistency_percent,sleep_debt_seconds,disturbances,source,external_id,notes)
			VALUES ($1,$2,$3,$4,COALESCE(NULLIF($5,'')::boolean,false),NULLIF($6,'')::bigint,NULLIF($7,'')::bigint,NULLIF($8,'')::bigint,
			NULLIF($9,'')::bigint,NULLIF($10,'')::bigint,NULLIF($11,'')::bigint,NULLIF($12,'')::numeric,
			NULLIF($13,'')::numeric,NULLIF($14,'')::numeric,NULLIF($15,'')::bigint,NULLIF($16,'')::int,
			$17,$18,$19) ON CONFLICT DO NOTHING`, ownerID, day, start, end, values["is_nap"], values["time_in_bed_seconds"],
			values["actual_sleep_seconds"], values["awake_seconds"], values["rem_seconds"], values["deep_seconds"],
			values["light_seconds"], values["sleep_performance_percent"], values["efficiency_percent"],
			values["consistency_percent"], values["sleep_debt_seconds"], values["disturbances"], source, externalID, values["notes"])
		return command.RowsAffected() > 0, mapPGError(err)
	case "nutrition":
		day, err := parseDate(values["date"], loc, "date")
		if err != nil {
			return false, err
		}
		command, err := tx.Exec(ctx, `INSERT INTO nutrition_days (
			owner_id,entry_date,calories_kcal,protein_g,fat_g,carbohydrates_g,fiber_g,sugar_g,
			saturated_fat_g,sodium_mg,potassium_mg,water_ml,source,external_id,notes)
			VALUES ($1,$2,NULLIF($3,'')::numeric,NULLIF($4,'')::numeric,NULLIF($5,'')::numeric,
			NULLIF($6,'')::numeric,NULLIF($7,'')::numeric,NULLIF($8,'')::numeric,NULLIF($9,'')::numeric,
			NULLIF($10,'')::numeric,NULLIF($11,'')::numeric,NULLIF($12,'')::numeric,$13,$14,$15)
			ON CONFLICT DO NOTHING`, ownerID, day, values["calories_kcal"], values["protein_g"], values["fat_g"],
			values["carbohydrates_g"], values["fiber_g"], values["sugar_g"], values["saturated_fat_g"],
			values["sodium_mg"], values["potassium_mg"], values["water_ml"], source, externalID, values["notes"])
		return command.RowsAffected() > 0, mapPGError(err)
	case "body":
		measuredAt, err := parseRequiredTime(values["measured_at"], loc, "measured_at")
		if err != nil {
			return false, err
		}
		var bodyMeasurementID int64
		err = tx.QueryRow(ctx, `INSERT INTO body_measurements (
			owner_id,measured_at,weight_kg,body_fat_percent,fat_mass_kg,lean_mass_kg,
			skeletal_muscle_mass_kg,total_body_water_l,intracellular_water_l,extracellular_water_l,
			ecw_tbw_ratio,protein_mass_kg,mineral_mass_kg,bmi,visceral_fat_level,visceral_fat_area_cm2,
			basal_metabolic_rate_kcal,inbody_score,phase_angle_degrees,
			waist_cm,chest_cm,biceps_cm,thigh_cm,source,external_id,notes)
			VALUES ($1,$2,NULLIF($3,'')::numeric,NULLIF($4,'')::numeric,NULLIF($5,'')::numeric,
			NULLIF($6,'')::numeric,NULLIF($7,'')::numeric,NULLIF($8,'')::numeric,NULLIF($9,'')::numeric,
			NULLIF($10,'')::numeric,NULLIF($11,'')::numeric,NULLIF($12,'')::numeric,NULLIF($13,'')::numeric,
			NULLIF($14,'')::numeric,NULLIF($15,'')::smallint,NULLIF($16,'')::numeric,
			NULLIF($17,'')::numeric,NULLIF($18,'')::numeric,NULLIF($19,'')::numeric,
			NULLIF($20,'')::numeric,NULLIF($21,'')::numeric,NULLIF($22,'')::numeric,NULLIF($23,'')::numeric,
			$24,$25,$26) ON CONFLICT DO NOTHING RETURNING id`, ownerID,
			measuredAt, values["weight_kg"], values["body_fat_percent"], values["fat_mass_kg"], values["lean_mass_kg"],
			values["skeletal_muscle_mass_kg"], values["total_body_water_l"], values["intracellular_water_l"],
			values["extracellular_water_l"], values["ecw_tbw_ratio"], values["protein_mass_kg"],
			values["mineral_mass_kg"], values["bmi"], values["visceral_fat_level"], values["visceral_fat_area_cm2"],
			values["basal_metabolic_rate_kcal"], values["inbody_score"], values["phase_angle_degrees"],
			values["waist_cm"], values["chest_cm"], values["biceps_cm"], values["thigh_cm"],
			source, externalID, values["notes"]).Scan(&bodyMeasurementID)
		if errors.Is(err, pgx.ErrNoRows) {
			return false, nil
		}
		if err != nil {
			return false, mapPGError(err)
		}
		for _, segment := range []string{"left_arm", "right_arm", "trunk", "left_leg", "right_leg"} {
			leanMass, leanPercent := values[segment+"_lean_mass_kg"], values[segment+"_lean_percent"]
			fatMass, fatPercent := values[segment+"_fat_mass_kg"], values[segment+"_fat_percent"]
			if leanMass == "" && leanPercent == "" && fatMass == "" && fatPercent == "" {
				continue
			}
			if _, err := tx.Exec(ctx, `INSERT INTO body_segment_measurements (
				body_measurement_id,owner_id,segment,lean_mass_kg,lean_percent,fat_mass_kg,fat_percent)
				VALUES ($1,$2,$3,NULLIF($4,'')::numeric,NULLIF($5,'')::numeric,
					NULLIF($6,'')::numeric,NULLIF($7,'')::numeric)`, bodyMeasurementID, ownerID, segment,
				leanMass, leanPercent, fatMass, fatPercent); err != nil {
				return false, mapPGError(err)
			}
		}
		return true, nil
	case "workouts":
		startedAt, err := parseRequiredTime(values["started_at"], loc, "started_at")
		if err != nil {
			return false, err
		}
		scheduledAt, err := parseImportTime(values["scheduled_at"], loc, "scheduled_at")
		if err != nil {
			return false, err
		}
		finishedAt, err := parseImportTime(values["finished_at"], loc, "finished_at")
		if err != nil {
			return false, err
		}
		status := strings.TrimSpace(values["status"])
		if status == "" {
			status = "finished"
		}
		if !validSessionStatus(status) || status == "scheduled" || status == "cancelled" || status == "excused" {
			return false, &ValidationError{Message: "imported workouts must be active or finished", Fields: map[string]string{"status": "use active or finished"}}
		}
		command, err := tx.Exec(ctx, `INSERT INTO training_sessions (
			owner_id,program_name,status,current_position,scheduled_at,started_at,finished_at,strain,notes,source,external_id)
			VALUES ($1,$2,$3,1,$4,$5,$6,NULLIF($7,'')::numeric,$8,$9,$10) ON CONFLICT DO NOTHING`,
			ownerID, values["program_name"], status, scheduledAt, startedAt, finishedAt, values["strain"],
			values["notes"], source, externalID)
		return command.RowsAffected() > 0, mapPGError(err)
	case "sets":
		return r.importSet(ctx, tx, ownerID, row, loc)
	default:
		return false, &ValidationError{Message: "unsupported import type", Fields: map[string]string{"data_type": "is not supported"}}
	}
}

func parseImportTime(raw string, loc *time.Location, field string) (*time.Time, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, nil
	}
	value, err := parseRequiredTime(raw, loc, field)
	if err != nil {
		return nil, err
	}
	return &value, nil
}

func (r *PostgresRepository) importSet(ctx context.Context, tx pgx.Tx, ownerID int64, row ImportRow, loc *time.Location) (bool, error) {
	values := row.Values
	source := defaultSource(row.Source)
	var sessionID int64
	var sessionStartedAt *time.Time
	err := tx.QueryRow(ctx, `SELECT id,started_at FROM training_sessions
		WHERE owner_id=$1 AND source=$2 AND external_id=$3`, ownerID, source, values["session_external_id"]).Scan(&sessionID, &sessionStartedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, fmt.Errorf("%w: referenced workout external_id %q", ErrNotFound, values["session_external_id"])
	}
	if err != nil {
		return false, err
	}
	externalID := cleanExternalID(&row.ExternalID)
	if externalID != nil {
		// The public import contract scopes a set external ID to the workout,
		// not to the mutable exercise-name match. Serialize that key across
		// concurrent imports, then check the whole session before resolving or
		// creating an exercise so a renamed exercise cannot duplicate the set.
		lockKey := strconv.FormatInt(sessionID, 10) + compositeExternalSeparator + source + compositeExternalSeparator + *externalID
		if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`, lockKey); err != nil {
			return false, err
		}
		var exists bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS (
			SELECT 1 FROM training_sets training_set
			JOIN training_session_exercises exercise ON exercise.id=training_set.session_exercise_id
			WHERE exercise.session_id=$1 AND training_set.source=$2 AND training_set.external_id=$3
		)`, sessionID, source, *externalID).Scan(&exists); err != nil {
			return false, err
		}
		if exists {
			return false, nil
		}
	}
	name := strings.TrimSpace(values["exercise_name"])
	var sessionExerciseID int64
	err = tx.QueryRow(ctx, `SELECT id FROM training_session_exercises
		WHERE session_id=$1 AND training_normalize_exercise_name(name)=training_normalize_exercise_name($2)
		ORDER BY position LIMIT 1`, sessionID, name).Scan(&sessionExerciseID)
	if errors.Is(err, pgx.ErrNoRows) {
		exerciseID, catalogName, resolveErr := resolveExercise(ctx, tx, ownerID, nil, name)
		if resolveErr != nil {
			return false, resolveErr
		}
		if err := tx.QueryRow(ctx, `INSERT INTO training_session_exercises (
			session_id,position,name,exercise_id,source,external_id)
			VALUES ($1,(SELECT COALESCE(max(position),0)+1 FROM training_session_exercises WHERE session_id=$1),$2,$3,$4,$5)
			RETURNING id`, sessionID, catalogName, exerciseID, source, "exercise:"+strings.ToLower(catalogName)).Scan(&sessionExerciseID); err != nil {
			return false, err
		}
	} else if err != nil {
		return false, err
	}
	position := 0
	if values["position"] != "" {
		position, err = strconv.Atoi(values["position"])
		if err != nil || position <= 0 {
			return false, &ValidationError{Message: "invalid set position", Fields: map[string]string{"position": "must be positive"}}
		}
	} else if err := tx.QueryRow(ctx, `SELECT COALESCE(max(position),0)+1 FROM training_sets WHERE session_exercise_id=$1`, sessionExerciseID).Scan(&position); err != nil {
		return false, err
	}
	completedAt, err := parseImportTime(values["completed_at"], loc, "completed_at")
	if err != nil {
		return false, err
	}
	if completedAt == nil {
		completedAt = sessionStartedAt
	}
	command, err := tx.Exec(ctx, `INSERT INTO training_sets (
		session_exercise_id,position,type,actual_weight_kg,actual_reps,actual_rir,started_at,completed_at,
		rest_seconds,notes,source,external_id)
		VALUES ($1,$2,$3,NULLIF(NULLIF($4,'')::numeric,0),NULLIF($5,'')::int,NULLIF($6,'')::numeric,$7,$7,
		NULLIF($8,'')::int,$9,$10,$11) ON CONFLICT DO NOTHING`, sessionExerciseID, position, values["type"],
		values["weight_kg"], values["reps"], values["rir"], completedAt, values["rest_seconds"],
		values["notes"], source, externalID)
	return command.RowsAffected() > 0, mapPGError(err)
}

func (r *PostgresRepository) SeedDemo(ctx context.Context, ownerID int64, now time.Time, loc *time.Location) (DemoSeedResult, error) {
	if loc == nil {
		loc = time.UTC
	}
	end := now.In(loc)
	end = time.Date(end.Year(), end.Month(), end.Day(), 8, 0, 0, 0, loc)
	start := end.AddDate(0, 0, -89)
	batches := map[string]ImportBatch{}
	for _, dataType := range []string{"recovery", "sleep", "nutrition", "body", "workouts", "sets"} {
		batches[dataType] = ImportBatch{DataType: dataType, Filename: "fitlog-demo-90d", Format: "demo", Source: "demo"}
	}
	workoutIndex := 0
	for offset := 0; offset < 90; offset++ {
		day := start.AddDate(0, 0, offset)
		date := day.Format("2006-01-02")
		if offset%13 != 0 {
			batch := batches["recovery"]
			batch.Rows = append(batch.Rows, demoRow("recovery", len(batch.Rows)+1, date,
				map[string]string{"date": date, "recovery_score": fmt.Sprintf("%d", 52+(offset*7)%42),
					"hrv_ms": fmt.Sprintf("%d", 48+(offset*3)%25), "resting_heart_rate_bpm": fmt.Sprintf("%d", 54+(offset*5)%10),
					"daily_strain": fmt.Sprintf("%.1f", 7+float64(offset%9)*0.8)}))
			batches["recovery"] = batch
		}
		if offset%11 != 0 {
			batch := batches["sleep"]
			sleepStart := day.Add(-8 * time.Hour)
			sleepSeconds := 6*3600 + (offset%7)*900
			batch.Rows = append(batch.Rows, demoRow("sleep", len(batch.Rows)+1, date,
				map[string]string{"date": date, "sleep_start": sleepStart.Format(time.RFC3339),
					"sleep_end":           sleepStart.Add(time.Duration(sleepSeconds+1800) * time.Second).Format(time.RFC3339),
					"time_in_bed_seconds": strconv.Itoa(sleepSeconds + 1800), "actual_sleep_seconds": strconv.Itoa(sleepSeconds),
					"awake_seconds": "1800", "rem_seconds": strconv.Itoa(sleepSeconds / 4),
					"deep_seconds": strconv.Itoa(sleepSeconds / 5), "light_seconds": strconv.Itoa(sleepSeconds - sleepSeconds/4 - sleepSeconds/5),
					"sleep_performance_percent": fmt.Sprintf("%d", 72+offset%22), "efficiency_percent": "91",
					"consistency_percent": fmt.Sprintf("%d", 70+offset%25), "sleep_debt_seconds": strconv.Itoa((offset % 5) * 1200),
					"disturbances": strconv.Itoa(offset % 4)}))
			batches["sleep"] = batch
		}
		if offset%7 != 0 {
			batch := batches["nutrition"]
			batch.Rows = append(batch.Rows, demoRow("nutrition", len(batch.Rows)+1, date,
				map[string]string{"date": date, "calories_kcal": fmt.Sprintf("%d", 2050+(offset%9)*55),
					"protein_g": fmt.Sprintf("%d", 125+(offset%6)*7), "fat_g": fmt.Sprintf("%d", 65+(offset%5)*4),
					"carbohydrates_g": fmt.Sprintf("%d", 220+(offset%8)*12), "fiber_g": fmt.Sprintf("%d", 24+offset%8),
					"water_ml": fmt.Sprintf("%d", 2100+(offset%6)*200)}))
			batches["nutrition"] = batch
		}
		if offset%17 != 0 {
			batch := batches["body"]
			weight := 82.4 - float64(offset)*0.025
			batch.Rows = append(batch.Rows, demoRow("body", len(batch.Rows)+1, date,
				map[string]string{"measured_at": day.Format(time.RFC3339), "weight_kg": fmt.Sprintf("%.2f", weight),
					"body_fat_percent": fmt.Sprintf("%.2f", 19.2-float64(offset)*0.015),
					"lean_mass_kg":     fmt.Sprintf("%.2f", weight*(1-(19.2-float64(offset)*0.015)/100))}))
			batches["body"] = batch
		}
		weekday := day.Weekday()
		if weekday == time.Monday || weekday == time.Wednesday || weekday == time.Friday {
			letter := map[time.Weekday]string{time.Monday: "A", time.Wednesday: "B", time.Friday: "C"}[weekday]
			externalID := "demo-workout-" + date
			started := time.Date(day.Year(), day.Month(), day.Day(), 18, 30, 0, 0, loc)
			workouts := batches["workouts"]
			workouts.Rows = append(workouts.Rows, demoRow("workouts", len(workouts.Rows)+1, externalID,
				map[string]string{"program_name": "Full Body " + letter,
					"scheduled_at": started.Add(-30 * time.Minute).Format(time.RFC3339), "started_at": started.Format(time.RFC3339),
					"finished_at": started.Add(70 * time.Minute).Format(time.RFC3339), "status": "finished",
					"strain": fmt.Sprintf("%.1f", 10+float64(workoutIndex%7)*0.7)}))
			batches["workouts"] = workouts
			week := workoutIndex / 3
			exercises := demoTemplateExercises[letter]
			sets := batches["sets"]
			for exerciseIndex, exercise := range exercises {
				for setIndex := 0; setIndex < 4; setIndex++ {
					setType := "working"
					weight := 35 + exerciseIndex*12 + week*2
					if setIndex == 0 {
						setType, weight = "warmup", weight/2
					}
					setExternal := fmt.Sprintf("%s-e%d-s%d", externalID, exerciseIndex+1, setIndex+1)
					sets.Rows = append(sets.Rows, demoRow("sets", len(sets.Rows)+1, setExternal,
						map[string]string{"session_external_id": externalID, "exercise_name": exercise,
							"position": strconv.Itoa(setIndex + 1), "type": setType, "weight_kg": strconv.Itoa(weight),
							"reps": strconv.Itoa(8 + (week+setIndex)%4), "rir": fmt.Sprintf("%.1f", 3-float64(setIndex%3)*0.5),
							"rest_seconds": strconv.Itoa(75 + setIndex*30), "completed_at": started.Add(time.Duration(10+exerciseIndex*18+setIndex*3) * time.Minute).Format(time.RFC3339)}))
				}
			}
			batches["sets"] = sets
			workoutIndex++
		}
	}
	seed := DemoSeedResult{Days: 90}
	if _, err := r.pool.Exec(ctx, `INSERT INTO dashboard_settings (
		owner_id,timezone,calorie_target_kcal,protein_target_g,fat_target_g,carbohydrates_target_g,
		sleep_target_min_seconds,sleep_target_max_seconds)
		VALUES ($1,$2,2300,145,75,260,25200,32400) ON CONFLICT (owner_id) DO NOTHING`, ownerID, loc.String()); err != nil {
		return DemoSeedResult{}, fmt.Errorf("seed demo settings: %w", err)
	}
	for _, dataType := range []string{"recovery", "sleep", "nutrition", "body", "workouts", "sets"} {
		batch := batches[dataType]
		result, err := r.ExecuteImport(ctx, ownerID, batch, loc)
		if err != nil {
			return DemoSeedResult{}, fmt.Errorf("seed demo %s: %w", dataType, err)
		}
		switch dataType {
		case "recovery":
			seed.RecoveryEntries = result.Imported
		case "sleep":
			seed.SleepEntries = result.Imported
		case "nutrition":
			seed.NutritionEntries = result.Imported
		case "body":
			seed.BodyMeasurements = result.Imported
		case "workouts":
			seed.WorkoutSessions = result.Imported
		}
	}
	if err := r.ensureDemoPlan(ctx, ownerID); err != nil {
		return DemoSeedResult{}, err
	}
	return seed, nil
}

func (r *PostgresRepository) ensureDemoPlan(ctx context.Context, ownerID int64) error {
	const externalID = "control-center-90d"
	var exists bool
	if err := r.pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM training_programs
		WHERE owner_id=$1 AND source='demo' AND external_id=$2)`, ownerID, externalID).Scan(&exists); err != nil {
		return fmt.Errorf("check demo plan: %w", err)
	}
	if !exists {
		workingSets, minReps, maxReps, rest, after := 3, 8, 12, 120, 180
		targetRIR, weightStep := 2.0, 2.5
		progression := "double"
		templates := make([]planTemplateInput, 0, 3)
		for templateIndex, letter := range []string{"A", "B", "C"} {
			exercises := make([]planExerciseInput, 0, len(demoTemplateExercises[letter]))
			for exerciseIndex, name := range demoTemplateExercises[letter] {
				exercises = append(exercises, planExerciseInput{
					Name: name, Position: exerciseIndex + 1, WorkingSets: &workingSets,
					MinReps: &minReps, MaxReps: &maxReps, TargetRIR: &targetRIR,
					WeightStepKG: &weightStep, ProgressionType: &progression,
					WarmupSets:  []planWarmupSetInput{{WeightMode: "bar", Reps: 10}},
					RestSeconds: &rest, RestAfterExerciseSeconds: &after,
				})
			}
			templates = append(templates, planTemplateInput{
				Name: letter, Position: templateIndex + 1,
				ExternalID: "demo-" + strings.ToLower(letter), Exercises: exercises,
			})
		}
		daysPerWeek := 3
		raw, err := json.Marshal(planInput{
			Name: "Control Center A/B/C", Description: "Deterministic three-day demo progression",
			DaysPerWeek: &daysPerWeek, Source: "demo", ExternalID: stringPointer(externalID), Templates: templates,
		})
		if err != nil {
			return fmt.Errorf("encode demo plan: %w", err)
		}
		if _, err := r.createPlan(ctx, ownerID, raw); err != nil {
			return fmt.Errorf("create demo plan: %w", err)
		}
	}
	if _, err := r.pool.Exec(ctx, `UPDATE training_sessions session SET
		workout_template_id=template.id,revision_id=template.revision_id,updated_at=now()
		FROM training_programs program
		JOIN workout_templates template ON template.revision_id=program.active_revision_id
		WHERE session.owner_id=$1 AND session.source='demo'
			AND program.owner_id=$1 AND program.source='demo' AND program.external_id=$2
			AND template.name=right(session.program_name,1)
			AND (session.workout_template_id IS DISTINCT FROM template.id OR session.revision_id IS DISTINCT FROM template.revision_id)`, ownerID, externalID); err != nil {
		return fmt.Errorf("link demo sessions to plan: %w", err)
	}
	return nil
}

func stringPointer(value string) *string { return &value }

func demoRow(dataType string, row int, externalID string, values map[string]string) ImportRow {
	values["external_id"] = externalID
	return ImportRow{Row: row, DataType: dataType, Source: "demo", ExternalID: externalID, Values: values}
}
