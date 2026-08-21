package controlcenter

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresRepository struct {
	pool *pgxpool.Pool
}

const importDetailJSONProjection = `jsonb_build_object(
	'id', id, 'source', source, 'data_type', data_type, 'filename', filename,
	'format', format, 'status', status, 'total_rows', total_rows,
	'imported_rows', imported_rows, 'skipped_rows', skipped_rows,
	'failed_rows', failed_rows, 'errors', error_summary,
	'started_at', started_at, 'completed_at', completed_at)`

var _ Store = (*PostgresRepository)(nil)

func NewRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository { return NewRepository(pool) }

func (r *PostgresRepository) List(
	ctx context.Context,
	ownerID int64,
	resource string,
	options Pagination,
	loc *time.Location,
) (ListResult, error) {
	if options.Page < 1 {
		options.Page = 1
	}
	if options.PageSize < 1 || options.PageSize > MaxPageSize {
		options.PageSize = 25
	}
	offset := (options.Page - 1) * options.PageSize
	var items []json.RawMessage
	var total int
	var err error
	switch resource {
	case "workout-sessions":
		items, total, err = r.listSessions(ctx, ownerID, options, loc)
	case "workout-plans":
		items, total, err = r.listPlans(ctx, ownerID, options)
	case "exercises":
		items, total, err = r.listExercises(ctx, ownerID, options)
	case "recovery", "sleep", "nutrition", "body-measurements":
		items, total, err = r.listRecords(ctx, ownerID, resource, options, loc)
	case "imports":
		var rows pgx.Rows
		rows, err = r.pool.Query(ctx, `
			SELECT jsonb_build_object(
				'id', id, 'source', source, 'data_type', data_type, 'filename', filename,
				'format', format, 'status', status, 'total_rows', total_rows,
				'imported_rows', imported_rows, 'skipped_rows', skipped_rows,
				'failed_rows', failed_rows,
				'started_at', started_at, 'completed_at', completed_at)
			FROM data_imports
			WHERE owner_id = $1
			ORDER BY started_at DESC, id DESC LIMIT $2 OFFSET $3`, ownerID, options.PageSize, offset)
		if err == nil {
			items, err = collectJSONRows(rows)
		}
		if err == nil {
			err = r.pool.QueryRow(ctx, `SELECT count(*) FROM data_imports WHERE owner_id = $1`, ownerID).Scan(&total)
		}
	default:
		return ListResult{}, ErrNotFound
	}
	if err != nil {
		return ListResult{}, fmt.Errorf("list %s: %w", resource, err)
	}
	return ListResult{Items: nonNilRaw(items), Total: total, Page: options.Page, PageSize: options.PageSize}, nil
}

func (r *PostgresRepository) Get(
	ctx context.Context,
	ownerID int64,
	resource string,
	id int64,
	loc *time.Location,
) (json.RawMessage, error) {
	var raw json.RawMessage
	var err error
	switch resource {
	case "workout-sessions":
		raw, err = r.getSession(ctx, ownerID, id, loc)
	case "workout-plans":
		raw, err = r.getPlan(ctx, ownerID, id)
	case "exercises":
		err = r.pool.QueryRow(ctx, exerciseJSONQuery+` WHERE e.owner_id = $1 AND e.id = $2`, ownerID, id).Scan(&raw)
	case "recovery", "sleep", "nutrition", "body-measurements":
		raw, err = r.getRecord(ctx, ownerID, resource, id, loc)
	case "imports":
		err = r.pool.QueryRow(ctx, `SELECT `+importDetailJSONProjection+`
			FROM data_imports WHERE owner_id = $1 AND id = $2`, ownerID, id).Scan(&raw)
	default:
		return nil, ErrNotFound
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get %s: %w", resource, err)
	}
	return raw, nil
}

func (r *PostgresRepository) Create(
	ctx context.Context,
	ownerID int64,
	resource string,
	raw json.RawMessage,
	loc *time.Location,
) (json.RawMessage, error) {
	switch resource {
	case "workout-sessions":
		return r.createSession(ctx, ownerID, raw, loc)
	case "workout-plans":
		return r.createPlan(ctx, ownerID, raw)
	case "exercises":
		return r.createExercise(ctx, ownerID, raw, loc)
	case "recovery", "sleep", "nutrition", "body-measurements":
		return r.createRecord(ctx, ownerID, resource, raw, loc)
	default:
		return nil, ErrNotFound
	}
}

func (r *PostgresRepository) Update(
	ctx context.Context,
	ownerID int64,
	resource string,
	id int64,
	raw json.RawMessage,
	loc *time.Location,
) (json.RawMessage, error) {
	switch resource {
	case "workout-sessions":
		return r.updateSession(ctx, ownerID, id, raw, loc)
	case "workout-plans":
		return r.updatePlan(ctx, ownerID, id, raw)
	case "exercises":
		return r.updateExercise(ctx, ownerID, id, raw, loc)
	case "recovery", "sleep", "nutrition", "body-measurements":
		return r.updateRecord(ctx, ownerID, resource, id, raw, loc)
	default:
		return nil, ErrNotFound
	}
}

func (r *PostgresRepository) Delete(ctx context.Context, ownerID int64, resource string, id int64) error {
	var command pgconn.CommandTag
	var err error
	switch resource {
	case "workout-sessions":
		command, err = r.pool.Exec(ctx, `DELETE FROM training_sessions WHERE owner_id = $1 AND id = $2`, ownerID, id)
	case "workout-plans":
		command, err = r.pool.Exec(ctx, `DELETE FROM training_programs WHERE owner_id = $1 AND id = $2`, ownerID, id)
	case "exercises":
		command, err = r.pool.Exec(ctx, `DELETE FROM training_exercises WHERE owner_id = $1 AND id = $2`, ownerID, id)
	case "recovery":
		command, err = r.pool.Exec(ctx, `DELETE FROM recovery_entries WHERE owner_id = $1 AND id = $2`, ownerID, id)
	case "sleep":
		command, err = r.pool.Exec(ctx, `DELETE FROM sleep_entries WHERE owner_id = $1 AND id = $2`, ownerID, id)
	case "nutrition":
		command, err = r.pool.Exec(ctx, `DELETE FROM nutrition_days WHERE owner_id = $1 AND id = $2`, ownerID, id)
	case "body-measurements":
		command, err = r.pool.Exec(ctx, `DELETE FROM body_measurements WHERE owner_id = $1 AND id = $2`, ownerID, id)
	default:
		return ErrNotFound
	}
	if err != nil {
		return mapPGError(err)
	}
	if command.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *PostgresRepository) Settings(ctx context.Context, ownerID int64, defaultTimezone string) (Settings, error) {
	var settings Settings
	var recovery []byte
	var updated time.Time
	err := r.pool.QueryRow(ctx, `
		SELECT timezone, units, theme, first_day_of_week,
		       calorie_target_kcal::double precision, protein_target_g::double precision,
		       fat_target_g::double precision, carbohydrates_target_g::double precision,
		       sleep_target_min_seconds, sleep_target_max_seconds, recovery_ranges, updated_at
		FROM dashboard_settings WHERE owner_id = $1`, ownerID).Scan(
		&settings.Timezone, &settings.Units, &settings.Theme, &settings.FirstDayOfWeek,
		&settings.CalorieTargetKcal, &settings.ProteinTargetG, &settings.FatTargetG,
		&settings.CarbohydratesTargetG, &settings.SleepTargetMinSeconds,
		&settings.SleepTargetMaxSeconds, &recovery, &updated,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return Settings{
			Timezone: defaultTimezone, Units: "metric", Theme: "dark", FirstDayOfWeek: 1,
			RecoveryRanges: json.RawMessage(`{}`),
		}, nil
	}
	if err != nil {
		return Settings{}, fmt.Errorf("get dashboard settings: %w", err)
	}
	settings.RecoveryRanges = recovery
	settings.UpdatedAt = &updated
	return settings, nil
}

func (r *PostgresRepository) SaveSettings(ctx context.Context, ownerID int64, settings Settings) (Settings, error) {
	recovery := settings.RecoveryRanges
	if len(recovery) == 0 {
		recovery = json.RawMessage(`{}`)
	}
	if !json.Valid(recovery) {
		return Settings{}, &ValidationError{Message: "invalid settings", Fields: map[string]string{"recovery_ranges": "must be valid JSON"}}
	}
	_, err := r.pool.Exec(ctx, `
		INSERT INTO dashboard_settings (
			owner_id, timezone, units, theme, first_day_of_week,
			calorie_target_kcal, protein_target_g, fat_target_g, carbohydrates_target_g,
			sleep_target_min_seconds, sleep_target_max_seconds, recovery_ranges, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12::jsonb,now())
		ON CONFLICT (owner_id) DO UPDATE SET
			timezone=EXCLUDED.timezone, units=EXCLUDED.units, theme=EXCLUDED.theme,
			first_day_of_week=EXCLUDED.first_day_of_week,
			calorie_target_kcal=EXCLUDED.calorie_target_kcal,
			protein_target_g=EXCLUDED.protein_target_g, fat_target_g=EXCLUDED.fat_target_g,
			carbohydrates_target_g=EXCLUDED.carbohydrates_target_g,
			sleep_target_min_seconds=EXCLUDED.sleep_target_min_seconds,
			sleep_target_max_seconds=EXCLUDED.sleep_target_max_seconds,
			recovery_ranges=EXCLUDED.recovery_ranges, updated_at=now()`,
		ownerID, settings.Timezone, settings.Units, settings.Theme, settings.FirstDayOfWeek,
		settings.CalorieTargetKcal, settings.ProteinTargetG, settings.FatTargetG,
		settings.CarbohydratesTargetG, settings.SleepTargetMinSeconds,
		settings.SleepTargetMaxSeconds, recovery,
	)
	if err != nil {
		return Settings{}, mapPGError(err)
	}
	return r.Settings(ctx, ownerID, settings.Timezone)
}

func (r *PostgresRepository) Sources(ctx context.Context, ownerID int64) ([]SourceStatus, error) {
	rows, err := r.pool.Query(ctx, `
		WITH known(source, label) AS (VALUES ('whoop','WHOOP'), ('fatsecret','FatSecret')),
		provider_sync AS (
			SELECT source, max(updated_at) AS synced_at FROM (
				SELECT source, external_id, updated_at FROM recovery_entries WHERE owner_id=$1
				UNION ALL SELECT source, external_id, updated_at FROM sleep_entries WHERE owner_id=$1
				UNION ALL SELECT source, external_id, updated_at FROM nutrition_days WHERE owner_id=$1
			) synced
			WHERE source IN ('whoop','fatsecret') AND external_id LIKE 'sync:%'
			GROUP BY source
		),
		last_updates AS (
			SELECT source, max(updated_at) AS last_synced_at FROM (
				SELECT source, updated_at FROM recovery_entries WHERE owner_id=$1
				UNION ALL SELECT source, updated_at FROM sleep_entries WHERE owner_id=$1
				UNION ALL SELECT source, updated_at FROM nutrition_days WHERE owner_id=$1
				UNION ALL SELECT source, updated_at FROM body_measurements WHERE owner_id=$1
				UNION ALL SELECT source, updated_at FROM training_sessions WHERE owner_id=$1
				UNION ALL
				SELECT training_set.source, training_set.updated_at
				FROM training_sets training_set
				JOIN training_session_exercises exercise ON exercise.id=training_set.session_exercise_id
				JOIN training_sessions session ON session.id=exercise.session_id
				WHERE session.owner_id=$1
			) records GROUP BY source
		)
		SELECT known.source, known.label, last_updates.last_synced_at,
			(provider_sync.synced_at IS NOT NULL) AS has_manual_sync
		FROM known
		LEFT JOIN last_updates ON last_updates.source = known.source
		LEFT JOIN provider_sync ON provider_sync.source = known.source
		ORDER BY known.source`, ownerID)
	if err != nil {
		return nil, fmt.Errorf("list sources: %w", err)
	}
	defer rows.Close()
	statuses := make([]SourceStatus, 0, 2)
	for rows.Next() {
		var status SourceStatus
		var hasManualSync bool
		if err := rows.Scan(&status.Source, &status.Label, &status.LastSyncedAt, &hasManualSync); err != nil {
			return nil, fmt.Errorf("scan source: %w", err)
		}
		status.Connected = false
		if hasManualSync {
			status.Status = "manual_sync"
			status.Detail = "One-shot API backfill; no background synchronization"
		} else {
			status.Status = "file_import_only"
			status.Detail = "File import only; bot OAuth does not sync dashboard"
		}
		statuses = append(statuses, status)
	}
	return statuses, rows.Err()
}

func (r *PostgresRepository) ExportSessionsCSV(ctx context.Context, ownerID int64, dateRange DateRange, filters Pagination, loc *time.Location) ([]byte, error) {
	planID, err := optionalFilterID(filters.Filters["plan_id"], "plan_id")
	if err != nil {
		return nil, err
	}
	templateID, err := optionalFilterID(filters.Filters["template_id"], "template_id")
	if err != nil {
		return nil, err
	}
	exerciseID, err := optionalFilterID(filters.Filters["exercise_id"], "exercise_id")
	if err != nil {
		return nil, err
	}
	rows, err := r.pool.Query(ctx, `
		SELECT session.id, session.program_name, session.status, session.scheduled_at,
		       session.started_at, session.finished_at,
		       exercise.position, exercise.name, exercise.note,
		       set.position, set.type, set.actual_weight_kg::double precision,
		       set.actual_reps, set.actual_rir::double precision, set.rest_seconds, set.completed_at, set.notes
		FROM training_sessions session
		LEFT JOIN training_program_revisions revision ON revision.id=session.revision_id
		LEFT JOIN training_programs program ON program.id=revision.program_id
		LEFT JOIN training_session_exercises exercise ON exercise.session_id=session.id
		LEFT JOIN training_sets set ON set.session_exercise_id=exercise.id
		WHERE session.owner_id=$1
		  AND (CASE WHEN $10::text='calendar'
		       THEN COALESCE((session.scheduled_at AT TIME ZONE $2)::date,(session.started_at AT TIME ZONE $2)::date)
		       ELSE COALESCE((session.started_at AT TIME ZONE $2)::date,(session.scheduled_at AT TIME ZONE $2)::date)
		       END) BETWEEN $3::date AND $4::date
		  AND ($5::text='' OR session.status=$5)
		  AND ($6::text='' OR session.program_name ILIKE '%' || $6 || '%' OR EXISTS (
		      SELECT 1 FROM training_session_exercises searched
		      WHERE searched.session_id=session.id AND searched.name ILIKE '%' || $6 || '%'))
		  AND ($7::bigint IS NULL OR program.id=$7)
		  AND ($8::bigint IS NULL OR session.workout_template_id=$8)
		  AND ($9::bigint IS NULL OR EXISTS (
		      SELECT 1 FROM training_session_exercises filtered
		      WHERE filtered.session_id=session.id AND filtered.exercise_id=$9))
		ORDER BY CASE WHEN $10::text='calendar' THEN COALESCE(session.scheduled_at,session.started_at)
		         ELSE COALESCE(session.started_at,session.scheduled_at) END,
		         session.id, exercise.position, set.position`,
		ownerID, loc.String(), dateRange.From, dateRange.To, filters.Filters["status"],
		strings.TrimSpace(filters.Search), planID, templateID, exerciseID, filters.Filters["date_basis"])
	if err != nil {
		return nil, fmt.Errorf("export sessions: %w", err)
	}
	defer rows.Close()
	var output bytes.Buffer
	writer := csv.NewWriter(&output)
	_ = writer.Write([]string{"session_id", "program_name", "status", "scheduled_at", "started_at", "finished_at", "exercise_position", "exercise_name", "exercise_notes", "set_position", "set_type", "weight_kg", "reps", "rir", "rest_seconds", "completed_at", "set_notes"})
	for rows.Next() {
		var sessionID int64
		var programName, status string
		var scheduled, started, finished, completed *time.Time
		var exercisePosition, setPosition, reps, rest *int
		var exerciseName, exerciseNotes, setType, setNotes *string
		var weight, rir *float64
		if err := rows.Scan(&sessionID, &programName, &status, &scheduled, &started, &finished, &exercisePosition, &exerciseName, &exerciseNotes, &setPosition, &setType, &weight, &reps, &rir, &rest, &completed, &setNotes); err != nil {
			return nil, fmt.Errorf("scan session export: %w", err)
		}
		_ = writer.Write([]string{
			strconv.FormatInt(sessionID, 10), programName, status, formatTimePtr(scheduled, loc), formatTimePtr(started, loc), formatTimePtr(finished, loc),
			formatIntPtr(exercisePosition), stringPtr(exerciseName), stringPtr(exerciseNotes), formatIntPtr(setPosition), stringPtr(setType),
			formatFloatPtr(weight), formatIntPtr(reps), formatFloatPtr(rir), formatIntPtr(rest), formatTimePtr(completed, loc), stringPtr(setNotes),
		})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	writer.Flush()
	return output.Bytes(), writer.Error()
}

func (r *PostgresRepository) ExportAll(ctx context.Context, ownerID int64, loc *time.Location) (json.RawMessage, error) {
	resources := []string{"workout-sessions", "workout-plans", "exercises", "recovery", "sleep", "nutrition", "body-measurements", "imports"}
	export := make(map[string]any, len(resources)+2)
	for _, resource := range resources {
		if resource == "imports" {
			items, err := r.exportImports(ctx, ownerID)
			if err != nil {
				return nil, err
			}
			export[resource] = items
			continue
		}
		items := make([]json.RawMessage, 0)
		for page := 1; ; page++ {
			result, err := r.List(ctx, ownerID, resource, Pagination{Page: page, PageSize: MaxPageSize}, loc)
			if err != nil {
				return nil, err
			}
			items = append(items, result.Items...)
			if len(items) >= result.Total || len(result.Items) == 0 {
				break
			}
		}
		export[strings.ReplaceAll(resource, "-", "_")] = items
	}
	settings, err := r.Settings(ctx, ownerID, loc.String())
	if err != nil {
		return nil, err
	}
	export["settings"] = settings
	export["exported_at"] = time.Now().UTC()
	encoded, err := json.Marshal(export)
	return encoded, err
}

func (r *PostgresRepository) exportImports(ctx context.Context, ownerID int64) ([]json.RawMessage, error) {
	rows, err := r.pool.Query(ctx, `SELECT `+importDetailJSONProjection+`
		FROM data_imports
		WHERE owner_id=$1
		ORDER BY started_at DESC, id DESC`, ownerID)
	if err != nil {
		return nil, fmt.Errorf("export imports: %w", err)
	}
	items, err := collectJSONRows(rows)
	if err != nil {
		return nil, fmt.Errorf("export imports: %w", err)
	}
	return nonNilRaw(items), nil
}

func (r *PostgresRepository) DeleteAll(ctx context.Context, ownerID int64) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	statements := []string{
		`DELETE FROM data_imports WHERE owner_id=$1`,
		`DELETE FROM dashboard_settings WHERE owner_id=$1`, `DELETE FROM body_measurements WHERE owner_id=$1`,
		`DELETE FROM nutrition_days WHERE owner_id=$1`, `DELETE FROM sleep_entries WHERE owner_id=$1`,
		`DELETE FROM recovery_entries WHERE owner_id=$1`, `DELETE FROM training_ui_states WHERE owner_id=$1`,
		`DELETE FROM training_sessions WHERE owner_id=$1`, `DELETE FROM training_programs WHERE owner_id=$1`,
		`DELETE FROM training_exercises WHERE owner_id=$1`,
	}
	for _, statement := range statements {
		if _, err := tx.Exec(ctx, statement, ownerID); err != nil {
			return fmt.Errorf("delete control center data: %w", err)
		}
	}
	return tx.Commit(ctx)
}

func collectJSONRows(rows pgx.Rows) ([]json.RawMessage, error) {
	defer rows.Close()
	items := make([]json.RawMessage, 0)
	for rows.Next() {
		var raw json.RawMessage
		if err := rows.Scan(&raw); err != nil {
			return nil, err
		}
		items = append(items, raw)
	}
	return items, rows.Err()
}

func nonNilRaw(items []json.RawMessage) []json.RawMessage {
	if items == nil {
		return []json.RawMessage{}
	}
	return items
}

func mapPGError(err error) error {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return err
	}
	switch pgErr.Code {
	case "23505", "23503":
		return fmt.Errorf("%w: %s", ErrConflict, pgErr.ConstraintName)
	case "23502", "23514", "22P02", "22007":
		return &ValidationError{Message: "database rejected the input", Fields: map[string]string{"record": pgErr.Message}}
	default:
		return err
	}
}

func formatTimePtr(value *time.Time, loc *time.Location) string {
	if value == nil {
		return ""
	}
	return value.In(loc).Format(time.RFC3339)
}

func formatIntPtr(value *int) string {
	if value == nil {
		return ""
	}
	return strconv.Itoa(*value)
}

func formatFloatPtr(value *float64) string {
	if value == nil {
		return ""
	}
	return strconv.FormatFloat(*value, 'f', -1, 64)
}

func stringPtr(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
