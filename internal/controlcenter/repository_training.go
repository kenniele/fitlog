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

const sessionJSONExpression = `jsonb_build_object(
	'id', session.id,
	'date', COALESCE((session.started_at AT TIME ZONE $3)::date, (session.scheduled_at AT TIME ZONE $3)::date),
	'actual_date', (session.started_at AT TIME ZONE $3)::date,
	'scheduled_date', (session.scheduled_at AT TIME ZONE $3)::date,
	'calendar_date', COALESCE((session.scheduled_at AT TIME ZONE $3)::date, (session.started_at AT TIME ZONE $3)::date),
	'scheduled_at', session.scheduled_at,
	'started_at', session.started_at,
	'finished_at', session.finished_at,
	'duration_seconds', CASE WHEN session.started_at IS NOT NULL AND session.finished_at IS NOT NULL
		THEN extract(epoch FROM session.finished_at-session.started_at)::bigint END,
	'plan_id', program.id,
	'plan_name', program.name,
	'template_id', session.workout_template_id,
	'template_name', template.name,
	'program_name', session.program_name,
	'status', session.status,
	'strain', session.strain::double precision,
	'notes', session.notes,
	'source', session.source,
	'external_id', session.external_id,
	'working_sets', stats.working_sets,
	'warmup_sets', stats.warmup_sets,
	'total_reps', stats.total_reps,
	'volume_kg', stats.volume_kg,
	'average_rir', stats.average_rir,
	'has_progression_snapshot', snapshot.has_progression_snapshot,
	'exercises', COALESCE(exercises.items, '[]'::jsonb),
	'created_at', session.created_at,
	'updated_at', session.updated_at
)`

const sessionJSONJoins = `
	LEFT JOIN workout_templates template ON template.id=session.workout_template_id
	LEFT JOIN training_program_revisions revision ON revision.id=session.revision_id
	LEFT JOIN training_programs program ON program.id=revision.program_id
	LEFT JOIN LATERAL (
		SELECT
			count(*) FILTER (WHERE training_set.completed_at IS NOT NULL AND training_set.type IN ('working','drop'))::int AS working_sets,
			count(*) FILTER (WHERE training_set.completed_at IS NOT NULL AND training_set.type='warmup')::int AS warmup_sets,
			COALESCE(sum(training_set.actual_reps) FILTER (WHERE training_set.completed_at IS NOT NULL), 0)::int AS total_reps,
			COALESCE(sum(training_set.actual_weight_kg * training_set.actual_reps)
				FILTER (WHERE training_set.completed_at IS NOT NULL AND training_set.type IN ('working','drop')
					AND training_set.actual_weight_kg IS NOT NULL), 0)::double precision AS volume_kg,
			avg(training_set.actual_rir) FILTER (WHERE training_set.completed_at IS NOT NULL
				AND training_set.type IN ('working','drop'))::double precision AS average_rir
		FROM training_session_exercises session_exercise
		JOIN training_sets training_set ON training_set.session_exercise_id=session_exercise.id
		WHERE session_exercise.session_id=session.id
	) stats ON true
	LEFT JOIN LATERAL (
		SELECT EXISTS (
			SELECT 1
			FROM training_session_exercises snapshot_exercise
			WHERE snapshot_exercise.session_id=session.id
			  AND (
				snapshot_exercise.working_sets IS NOT NULL
				OR snapshot_exercise.min_reps IS NOT NULL
				OR snapshot_exercise.max_reps IS NOT NULL
				OR snapshot_exercise.target_rir IS NOT NULL
				OR snapshot_exercise.weight_step_kg IS NOT NULL
				OR snapshot_exercise.rest_between_sets_seconds IS NOT NULL
				OR snapshot_exercise.progression_type IS NOT NULL
				OR snapshot_exercise.warmup_plan <> '[]'::jsonb
				OR snapshot_exercise.recommendation <> '{}'::jsonb
				OR snapshot_exercise.planned_weight_kg IS NOT NULL
				OR snapshot_exercise.planned_min_reps IS NOT NULL
				OR snapshot_exercise.planned_max_reps IS NOT NULL
				OR snapshot_exercise.planned_working_sets IS NOT NULL
				OR snapshot_exercise.planned_target_rir IS NOT NULL
				OR snapshot_exercise.planned_rest_seconds IS NOT NULL
				OR snapshot_exercise.overridden
				OR EXISTS (
					SELECT 1 FROM training_sets snapshot_set
					WHERE snapshot_set.session_exercise_id=snapshot_exercise.id
					  AND (
						snapshot_set.planned_weight_kg IS NOT NULL
						OR snapshot_set.planned_min_reps IS NOT NULL
						OR snapshot_set.planned_max_reps IS NOT NULL
						OR snapshot_set.planned_rir IS NOT NULL
					  )
				)
			  )
		) AS has_progression_snapshot
	) snapshot ON true
	LEFT JOIN LATERAL (
		SELECT jsonb_agg(jsonb_build_object(
			'id', session_exercise.id,
			'exercise_id', session_exercise.exercise_id,
			'name', session_exercise.name,
			'exercise_name', session_exercise.name,
			'position', session_exercise.position,
			'note', session_exercise.note,
			'notes', session_exercise.note,
			'completed', session_exercise.complete,
			'working_sets', session_exercise.working_sets,
			'min_reps', session_exercise.min_reps,
			'max_reps', session_exercise.max_reps,
			'target_rir', session_exercise.target_rir::double precision,
			'planned_weight_kg', session_exercise.planned_weight_kg::double precision,
			'planned_min_reps', session_exercise.planned_min_reps,
			'planned_max_reps', session_exercise.planned_max_reps,
			'planned_working_sets', session_exercise.planned_working_sets,
			'planned_target_rir', session_exercise.planned_target_rir::double precision,
			'planned_rest_seconds', session_exercise.planned_rest_seconds,
			'source', session_exercise.source,
			'external_id', session_exercise.external_id,
			'rest_after_exercise_seconds', session_exercise.rest_after_exercise_seconds,
			'current_result', CASE WHEN sets.completed_working_sets > 0 THEN jsonb_build_object(
				'date', COALESCE((session.started_at AT TIME ZONE $3)::date, (session.scheduled_at AT TIME ZONE $3)::date),
				'working_sets', sets.completed_working_sets,
				'repetitions', sets.working_repetitions,
				'volume_kg', sets.working_volume_kg,
				'best_weight_kg', sets.best_weight_kg,
				'estimated_1rm', sets.estimated_1rm,
				'average_rir', sets.average_rir
			) END,
			'previous_result', previous.result,
			'sets', COALESCE(sets.items, '[]'::jsonb)
		) ORDER BY session_exercise.position) AS items
		FROM training_session_exercises session_exercise
		LEFT JOIN LATERAL (
			SELECT jsonb_agg(jsonb_build_object(
				'id', training_set.id,
				'position', training_set.position,
				'type', training_set.type,
				'weight_kg', training_set.actual_weight_kg::double precision,
				'actual_weight_kg', training_set.actual_weight_kg::double precision,
				'reps', training_set.actual_reps,
				'actual_reps', training_set.actual_reps,
				'rir', training_set.actual_rir::double precision,
				'actual_rir', training_set.actual_rir::double precision,
				'planned_weight_kg', training_set.planned_weight_kg::double precision,
				'planned_min_reps', training_set.planned_min_reps,
				'planned_max_reps', training_set.planned_max_reps,
				'planned_rir', training_set.planned_rir::double precision,
				'rest_seconds', training_set.rest_seconds,
				'completed', training_set.completed_at IS NOT NULL,
				'completed_at', training_set.completed_at,
				'comment', training_set.notes,
				'notes', training_set.notes,
				'source', training_set.source,
				'external_id', training_set.external_id
			) ORDER BY training_set.position) AS items,
			count(*) FILTER (WHERE training_set.completed_at IS NOT NULL
				AND training_set.type IN ('working','drop'))::int AS completed_working_sets,
			COALESCE(sum(training_set.actual_reps) FILTER (WHERE training_set.completed_at IS NOT NULL
				AND training_set.type IN ('working','drop')), 0)::int AS working_repetitions,
			COALESCE(sum(training_set.actual_weight_kg * training_set.actual_reps)
				FILTER (WHERE training_set.completed_at IS NOT NULL
					AND training_set.type IN ('working','drop')
					AND training_set.actual_weight_kg IS NOT NULL), 0)::double precision AS working_volume_kg,
			max(training_set.actual_weight_kg) FILTER (WHERE training_set.completed_at IS NOT NULL
				AND training_set.type IN ('working','drop'))::double precision AS best_weight_kg,
			max(training_set.actual_weight_kg * (1 + training_set.actual_reps::double precision / 30))
				FILTER (WHERE training_set.completed_at IS NOT NULL
					AND training_set.type IN ('working','drop')
					AND training_set.actual_weight_kg IS NOT NULL
					AND training_set.actual_reps BETWEEN 1 AND 12)::double precision AS estimated_1rm,
			avg(training_set.actual_rir) FILTER (WHERE training_set.completed_at IS NOT NULL
				AND training_set.type IN ('working','drop'))::double precision AS average_rir
			FROM training_sets training_set
			WHERE training_set.session_exercise_id=session_exercise.id
		) sets ON true
		LEFT JOIN LATERAL (
			SELECT jsonb_build_object(
				'session_id', previous_session.id,
				'date', (previous_session.started_at AT TIME ZONE $3)::date,
				'working_sets', count(previous_set.id)::int,
				'repetitions', COALESCE(sum(previous_set.actual_reps), 0)::int,
				'volume_kg', COALESCE(sum(previous_set.actual_weight_kg * previous_set.actual_reps)
					FILTER (WHERE previous_set.actual_weight_kg IS NOT NULL), 0)::double precision,
				'best_weight_kg', max(previous_set.actual_weight_kg)::double precision,
				'estimated_1rm', max(previous_set.actual_weight_kg * (1 + previous_set.actual_reps::double precision / 30))
					FILTER (WHERE previous_set.actual_weight_kg IS NOT NULL
						AND previous_set.actual_reps BETWEEN 1 AND 12)::double precision,
				'average_rir', avg(previous_set.actual_rir)::double precision
			) AS result
			FROM training_sessions previous_session
			JOIN training_session_exercises previous_exercise ON previous_exercise.session_id=previous_session.id
			JOIN training_sets previous_set ON previous_set.session_exercise_id=previous_exercise.id
				AND previous_set.completed_at IS NOT NULL
				AND previous_set.type IN ('working','drop')
			WHERE previous_session.owner_id=session.owner_id
			  AND previous_session.status='finished'
			  AND previous_session.started_at < COALESCE(session.started_at, session.scheduled_at)
			  AND (
				(session_exercise.exercise_id IS NOT NULL AND previous_exercise.exercise_id=session_exercise.exercise_id)
				OR (session_exercise.exercise_id IS NULL
					AND training_normalize_exercise_name(previous_exercise.name)=training_normalize_exercise_name(session_exercise.name))
			  )
			GROUP BY previous_session.id, previous_session.started_at
			ORDER BY previous_session.started_at DESC, previous_session.id DESC
			LIMIT 1
		) previous ON true
		WHERE session_exercise.session_id=session.id
	) exercises ON true`

type workoutSetInput struct {
	ID          *int64   `json:"id"`
	Position    int      `json:"position"`
	Type        string   `json:"type"`
	WeightKG    *float64 `json:"weight_kg"`
	Reps        *int     `json:"reps"`
	RIR         *float64 `json:"rir"`
	RestSeconds *int     `json:"rest_seconds"`
	Completed   *bool    `json:"completed"`
	CompletedAt *string  `json:"completed_at"`
	Comment     string   `json:"comment"`
	Notes       string   `json:"notes"`
	Source      string   `json:"source"`
	ExternalID  *string  `json:"external_id"`
}

type sessionExerciseInput struct {
	ID                       *int64            `json:"id"`
	ExerciseID               *int64            `json:"exercise_id"`
	Name                     string            `json:"name"`
	Position                 int               `json:"position"`
	Note                     string            `json:"note"`
	Notes                    string            `json:"notes"`
	Completed                *bool             `json:"completed"`
	RestAfterExerciseSeconds *int              `json:"rest_after_exercise_seconds"`
	Source                   string            `json:"source"`
	ExternalID               *string           `json:"external_id"`
	Sets                     []workoutSetInput `json:"sets"`
}

type workoutSessionInput struct {
	Date              string                 `json:"date"`
	PlanID            *int64                 `json:"plan_id"`
	TemplateID        *int64                 `json:"template_id"`
	WorkoutTemplateID *int64                 `json:"workout_template_id"`
	ProgramName       string                 `json:"program_name"`
	Status            string                 `json:"status"`
	ScheduledAt       *string                `json:"scheduled_at"`
	StartedAt         *string                `json:"started_at"`
	FinishedAt        *string                `json:"finished_at"`
	Strain            *float64               `json:"strain"`
	Notes             string                 `json:"notes"`
	Source            string                 `json:"source"`
	ExternalID        *string                `json:"external_id"`
	Exercises         []sessionExerciseInput `json:"exercises"`
}

func (r *PostgresRepository) listSessions(ctx context.Context, ownerID int64, options Pagination, loc *time.Location) ([]json.RawMessage, int, error) {
	planID, err := optionalFilterID(options.Filters["plan_id"], "plan_id")
	if err != nil {
		return nil, 0, err
	}
	templateID, err := optionalFilterID(options.Filters["template_id"], "template_id")
	if err != nil {
		return nil, 0, err
	}
	exerciseID, err := optionalFilterID(options.Filters["exercise_id"], "exercise_id")
	if err != nil {
		return nil, 0, err
	}
	from, to := dateFilterArgs(options)
	search := strings.TrimSpace(options.Search)
	offset := (options.Page - 1) * options.PageSize
	where := ` WHERE session.owner_id=$1
		AND ($2::text='' OR session.status=$2)
		AND ($4::date IS NULL OR (CASE WHEN $10::text='calendar'
			THEN COALESCE((session.scheduled_at AT TIME ZONE $3)::date,(session.started_at AT TIME ZONE $3)::date)
			ELSE COALESCE((session.started_at AT TIME ZONE $3)::date,(session.scheduled_at AT TIME ZONE $3)::date) END) >= $4::date)
		AND ($5::date IS NULL OR (CASE WHEN $10::text='calendar'
			THEN COALESCE((session.scheduled_at AT TIME ZONE $3)::date,(session.started_at AT TIME ZONE $3)::date)
			ELSE COALESCE((session.started_at AT TIME ZONE $3)::date,(session.scheduled_at AT TIME ZONE $3)::date) END) <= $5::date)
		AND ($6::text='' OR session.program_name ILIKE '%' || $6 || '%' OR EXISTS (
			SELECT 1 FROM training_session_exercises searched WHERE searched.session_id=session.id AND searched.name ILIKE '%' || $6 || '%'))
		AND ($7::bigint IS NULL OR program.id=$7)
		AND ($8::bigint IS NULL OR session.workout_template_id=$8)
		AND ($9::bigint IS NULL OR EXISTS (
			SELECT 1 FROM training_session_exercises filtered WHERE filtered.session_id=session.id AND filtered.exercise_id=$9))`
	args := []any{ownerID, options.Filters["status"], loc.String(), from, to, search, planID, templateID, exerciseID, options.Filters["date_basis"]}
	rows, err := r.pool.Query(ctx, `SELECT `+sessionJSONExpression+` FROM training_sessions session `+sessionJSONJoins+where+
		` ORDER BY CASE WHEN $10::text='calendar' THEN COALESCE(session.scheduled_at,session.started_at)
		ELSE COALESCE(session.started_at,session.scheduled_at) END DESC, session.id DESC LIMIT $11 OFFSET $12`, append(args, options.PageSize, offset)...)
	if err != nil {
		return nil, 0, err
	}
	items, err := collectJSONRows(rows)
	if err != nil {
		return nil, 0, err
	}
	var total int
	if err := r.pool.QueryRow(ctx, `SELECT count(*) FROM training_sessions session
		LEFT JOIN training_program_revisions revision ON revision.id=session.revision_id
		LEFT JOIN training_programs program ON program.id=revision.program_id `+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func optionalFilterID(raw, field string) (any, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, nil
	}
	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || value <= 0 {
		return nil, &ValidationError{Message: "invalid filter", Fields: map[string]string{field: "must be a positive integer"}}
	}
	return value, nil
}

func (r *PostgresRepository) getSession(ctx context.Context, ownerID, id int64, loc *time.Location) (json.RawMessage, error) {
	var raw json.RawMessage
	err := r.pool.QueryRow(ctx, `SELECT `+sessionJSONExpression+` FROM training_sessions session `+sessionJSONJoins+
		` WHERE session.owner_id=$1 AND session.id=$2`, ownerID, id, loc.String()).Scan(&raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return raw, err
}

func (r *PostgresRepository) createSession(ctx context.Context, ownerID int64, raw json.RawMessage, loc *time.Location) (json.RawMessage, error) {
	return r.saveSession(ctx, ownerID, 0, raw, loc)
}

func (r *PostgresRepository) updateSession(ctx context.Context, ownerID, id int64, raw json.RawMessage, loc *time.Location) (json.RawMessage, error) {
	return r.saveSession(ctx, ownerID, id, raw, loc)
}

func (r *PostgresRepository) saveSession(ctx context.Context, ownerID, id int64, raw json.RawMessage, loc *time.Location) (json.RawMessage, error) {
	var input workoutSessionInput
	if err := decodeStrict(raw, &input); err != nil {
		return nil, err
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	creating := id == 0
	protectedSnapshot := false
	var existingTemplateID, existingRevisionID *int64
	if !creating {
		protectedSnapshot, err = hasProgressionSnapshot(ctx, tx, ownerID, id)
		if err != nil {
			return nil, err
		}
		if protectedSnapshot {
			if err := tx.QueryRow(ctx, `SELECT workout_template_id,revision_id FROM training_sessions
				WHERE id=$1 AND owner_id=$2`, id, ownerID).Scan(&existingTemplateID, &existingRevisionID); err != nil {
				if errors.Is(err, pgx.ErrNoRows) {
					return nil, ErrNotFound
				}
				return nil, err
			}
		}
	}

	templateID := input.TemplateID
	if templateID == nil {
		templateID = input.WorkoutTemplateID
	}
	resolvedTemplateID, revisionID, programName, err := resolveSessionPlan(ctx, tx, ownerID, input.PlanID, templateID)
	if err != nil {
		return nil, err
	}
	templateID = resolvedTemplateID
	if protectedSnapshot && (!sameOptionalID(templateID, existingTemplateID) || !sameOptionalID(revisionID, existingRevisionID)) {
		return nil, &ValidationError{Message: "invalid workout", Fields: map[string]string{
			"template_id": "cannot change the plan or template of a progression snapshot",
		}}
	}
	if strings.TrimSpace(input.ProgramName) != "" {
		programName = strings.TrimSpace(input.ProgramName)
	}
	if programName == "" {
		programName = "Свободная тренировка"
	}
	status := strings.TrimSpace(input.Status)
	if status == "" {
		if input.FinishedAt != nil {
			status = "finished"
		} else {
			status = "active"
		}
	}
	if !validSessionStatus(status) {
		return nil, &ValidationError{Message: "invalid workout", Fields: map[string]string{"status": "use scheduled, active, finished, cancelled, or excused"}}
	}
	scheduledAt, err := parseOptionalTime(input.ScheduledAt, loc, "scheduled_at")
	if err != nil {
		return nil, err
	}
	startedAt, err := parseOptionalTime(input.StartedAt, loc, "started_at")
	if err != nil {
		return nil, err
	}
	finishedAt, err := parseOptionalTime(input.FinishedAt, loc, "finished_at")
	if err != nil {
		return nil, err
	}
	if input.Date != "" {
		date, dateErr := parseDate(input.Date, loc, "date")
		if dateErr != nil {
			return nil, dateErr
		}
		if status == "scheduled" || status == "cancelled" || status == "excused" {
			if scheduledAt == nil {
				scheduledAt = &date
			} else if !sameLocalDate(*scheduledAt, date, loc) {
				return nil, &ValidationError{Message: "invalid workout", Fields: map[string]string{"date": "must match scheduled_at in the dashboard timezone"}}
			}
		} else if startedAt == nil {
			startedAt = &date
		} else if !sameLocalDate(*startedAt, date, loc) {
			return nil, &ValidationError{Message: "invalid workout", Fields: map[string]string{"date": "must match started_at in the dashboard timezone"}}
		}
	}
	if (status == "active" || status == "finished") && startedAt == nil {
		return nil, &ValidationError{Message: "invalid workout", Fields: map[string]string{"started_at": "is required for active or finished sessions"}}
	}
	if (status == "scheduled" || status == "cancelled" || status == "excused") && scheduledAt == nil {
		return nil, &ValidationError{Message: "invalid workout", Fields: map[string]string{"scheduled_at": "is required for scheduled, cancelled, or excused sessions"}}
	}
	if finishedAt != nil && startedAt != nil && finishedAt.Before(*startedAt) {
		return nil, &ValidationError{Message: "invalid workout", Fields: map[string]string{"finished_at": "must not be before started_at"}}
	}
	if status == "scheduled" || status == "cancelled" || status == "excused" {
		for exerciseIndex, exercise := range input.Exercises {
			for setIndex, set := range exercise.Sets {
				if (set.Completed != nil && *set.Completed) || strings.TrimSpace(valueOrEmpty(set.CompletedAt)) != "" {
					return nil, &ValidationError{Message: "invalid workout", Fields: map[string]string{
						fmt.Sprintf("exercises.%d.sets.%d.completed", exerciseIndex, setIndex): "planned, cancelled, or excused sessions cannot contain completed sets",
					}}
				}
			}
		}
	}
	input.Source = defaultSource(input.Source)
	patchSnapshot := false
	if id == 0 {
		err = tx.QueryRow(ctx, `INSERT INTO training_sessions (
			owner_id, workout_template_id, revision_id, program_name, status, current_position,
			scheduled_at, started_at, finished_at, strain, notes, source, external_id, updated_at)
			VALUES ($1,$2,$3,$4,$5,1,$6,$7,$8,$9,$10,$11,$12,now()) RETURNING id`,
			ownerID, templateID, revisionID, programName, status, scheduledAt, startedAt, finishedAt,
			input.Strain, input.Notes, input.Source, cleanExternalID(input.ExternalID)).Scan(&id)
	} else {
		command, updateErr := tx.Exec(ctx, `UPDATE training_sessions SET
			workout_template_id=$3,revision_id=$4,program_name=$5,status=$6,scheduled_at=$7,
			started_at=$8,finished_at=$9,strain=$10,notes=$11,source=$12,external_id=$13,updated_at=now()
			WHERE id=$1 AND owner_id=$2`, id, ownerID, templateID, revisionID, programName, status,
			scheduledAt, startedAt, finishedAt, input.Strain, input.Notes, input.Source, cleanExternalID(input.ExternalID))
		err = updateErr
		if err == nil && command.RowsAffected() == 0 {
			return nil, ErrNotFound
		}
		if err == nil && input.Exercises != nil {
			if protectedSnapshot {
				patchSnapshot = true
				err = patchProgressionSnapshot(ctx, tx, ownerID, id, input.Exercises, status, startedAt, loc)
			}
			if err == nil && !patchSnapshot {
				_, err = tx.Exec(ctx, `DELETE FROM training_session_exercises WHERE session_id=$1`, id)
			}
		}
	}
	if err != nil {
		return nil, mapPGError(err)
	}
	if input.Exercises != nil && !patchSnapshot {
		if err := insertSessionExercises(ctx, tx, ownerID, id, input.Exercises, input.Source, status, startedAt, loc); err != nil {
			return nil, err
		}
	} else if creating && templateID != nil {
		if err := materializeTemplateSnapshot(ctx, tx, ownerID, id, *templateID, input.Source); err != nil {
			return nil, err
		}
	} else if !creating && input.Exercises == nil {
		// Metadata-only update: keep every snapshot and manually entered set intact.
	}
	if status == "finished" {
		var completed bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS (
			SELECT 1 FROM training_sets training_set
			JOIN training_session_exercises exercise ON exercise.id=training_set.session_exercise_id
			JOIN training_sessions session ON session.id=exercise.session_id
			WHERE session.id=$1 AND session.owner_id=$2 AND training_set.completed_at IS NOT NULL
		)`, id, ownerID).Scan(&completed); err != nil {
			return nil, err
		}
		if !completed {
			return nil, &ValidationError{Message: "invalid workout", Fields: map[string]string{"exercises": "a finished session requires at least one completed set"}}
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, mapPGError(err)
	}
	return r.getSession(ctx, ownerID, id, loc)
}

func sameOptionalID(left, right *int64) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return *left == *right
}

func valueOrEmpty(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func sameLocalDate(left, right time.Time, loc *time.Location) bool {
	left = left.In(loc)
	right = right.In(loc)
	leftYear, leftMonth, leftDay := left.Date()
	rightYear, rightMonth, rightDay := right.Date()
	return leftYear == rightYear && leftMonth == rightMonth && leftDay == rightDay
}

func hasProgressionSnapshot(ctx context.Context, tx pgx.Tx, ownerID, sessionID int64) (bool, error) {
	var protected bool
	err := tx.QueryRow(ctx, `SELECT EXISTS (
		SELECT 1
		FROM training_session_exercises exercise
		JOIN training_sessions session ON session.id=exercise.session_id
		WHERE exercise.session_id=$1 AND session.owner_id=$2
		  AND (
			exercise.working_sets IS NOT NULL
			OR exercise.min_reps IS NOT NULL
			OR exercise.max_reps IS NOT NULL
			OR exercise.target_rir IS NOT NULL
			OR exercise.weight_step_kg IS NOT NULL
			OR exercise.rest_between_sets_seconds IS NOT NULL
			OR exercise.progression_type IS NOT NULL
			OR exercise.warmup_plan <> '[]'::jsonb
			OR exercise.recommendation <> '{}'::jsonb
			OR exercise.planned_weight_kg IS NOT NULL
			OR exercise.planned_min_reps IS NOT NULL
			OR exercise.planned_max_reps IS NOT NULL
			OR exercise.planned_working_sets IS NOT NULL
			OR exercise.planned_target_rir IS NOT NULL
			OR exercise.planned_rest_seconds IS NOT NULL
			OR exercise.overridden
			OR EXISTS (
				SELECT 1 FROM training_sets training_set
				WHERE training_set.session_exercise_id=exercise.id
				  AND (
					training_set.planned_weight_kg IS NOT NULL
					OR training_set.planned_min_reps IS NOT NULL
					OR training_set.planned_max_reps IS NOT NULL
					OR training_set.planned_rir IS NOT NULL
				  )
			)
		  )
	)`, sessionID, ownerID).Scan(&protected)
	return protected, err
}

func patchProgressionSnapshot(ctx context.Context, tx pgx.Tx, ownerID, sessionID int64, exercises []sessionExerciseInput, status string, startedAt *time.Time, loc *time.Location) error {
	for exerciseIndex, exercise := range exercises {
		if exercise.ID == nil || *exercise.ID <= 0 {
			return &ValidationError{Message: "invalid workout", Fields: map[string]string{fmt.Sprintf("exercises.%d.id", exerciseIndex): "is required for a progression snapshot"}}
		}
		var exists bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS (
			SELECT 1 FROM training_session_exercises snapshot_exercise
			JOIN training_sessions snapshot_session ON snapshot_session.id=snapshot_exercise.session_id
			WHERE snapshot_exercise.id=$1 AND snapshot_exercise.session_id=$2 AND snapshot_session.owner_id=$3
		)`, *exercise.ID, sessionID, ownerID).Scan(&exists); err != nil {
			return err
		}
		if !exists {
			return &ValidationError{Message: "invalid workout", Fields: map[string]string{fmt.Sprintf("exercises.%d.id", exerciseIndex): "does not belong to this session"}}
		}
		for setIndex, set := range exercise.Sets {
			if set.WeightKG != nil && *set.WeightKG <= 0 {
				set.WeightKG = nil
			}
			if set.Reps != nil && *set.Reps <= 0 {
				return &ValidationError{Message: "invalid workout", Fields: map[string]string{fmt.Sprintf("exercises.%d.sets.%d.reps", exerciseIndex, setIndex): "must be positive"}}
			}
			if set.RIR != nil && *set.RIR < 0 {
				return &ValidationError{Message: "invalid workout", Fields: map[string]string{fmt.Sprintf("exercises.%d.sets.%d.rir", exerciseIndex, setIndex): "must be non-negative"}}
			}
			if set.RestSeconds != nil && *set.RestSeconds < 0 {
				return &ValidationError{Message: "invalid workout", Fields: map[string]string{fmt.Sprintf("exercises.%d.sets.%d.rest_seconds", exerciseIndex, setIndex): "must be non-negative"}}
			}
			completed := set.Completed != nil && *set.Completed
			completedAt, err := parseOptionalTime(set.CompletedAt, loc, "completed_at")
			if err != nil {
				return err
			}
			if completed && completedAt == nil {
				value := time.Now()
				if startedAt != nil {
					value = *startedAt
				}
				completedAt = &value
			}
			if !completed {
				completedAt = nil
			}
			notes := set.Notes
			if notes == "" {
				notes = set.Comment
			}
			if set.ID != nil {
				command, err := tx.Exec(ctx, `UPDATE training_sets training_set SET
					actual_weight_kg=$1,actual_reps=$2,actual_rir=$3,started_at=$4,completed_at=$4,
					rest_seconds=$5,notes=$6,updated_at=now()
					FROM training_session_exercises snapshot_exercise
					JOIN training_sessions snapshot_session ON snapshot_session.id=snapshot_exercise.session_id
					WHERE training_set.id=$7 AND training_set.session_exercise_id=snapshot_exercise.id
						AND snapshot_exercise.id=$8 AND snapshot_session.id=$9 AND snapshot_session.owner_id=$10`,
					set.WeightKG, set.Reps, set.RIR, completedAt, set.RestSeconds, notes,
					*set.ID, *exercise.ID, sessionID, ownerID)
				if err != nil {
					return mapPGError(err)
				}
				if command.RowsAffected() == 0 {
					return &ValidationError{Message: "invalid workout", Fields: map[string]string{fmt.Sprintf("exercises.%d.sets.%d.id", exerciseIndex, setIndex): "does not belong to this exercise"}}
				}
				continue
			}
			setType := strings.TrimSpace(set.Type)
			if setType == "" {
				setType = "working"
			}
			if setType != "warmup" && setType != "working" && setType != "drop" {
				return &ValidationError{Message: "invalid workout", Fields: map[string]string{fmt.Sprintf("exercises.%d.sets.%d.type", exerciseIndex, setIndex): "use warmup, working, or drop"}}
			}
			position := set.Position
			if position <= 0 {
				if err := tx.QueryRow(ctx, `SELECT COALESCE(max(position),0)+1 FROM training_sets WHERE session_exercise_id=$1`, *exercise.ID).Scan(&position); err != nil {
					return err
				}
			}
			setSource := defaultSource(set.Source)
			if _, err := tx.Exec(ctx, `INSERT INTO training_sets (
				session_exercise_id,position,type,actual_weight_kg,actual_reps,actual_rir,
				started_at,completed_at,rest_seconds,notes,source,external_id,updated_at)
				VALUES ($1,$2,$3,$4,$5,$6,$7,$7,$8,$9,$10,$11,now())`, *exercise.ID, position,
				setType, set.WeightKG, set.Reps, set.RIR, completedAt, set.RestSeconds, notes,
				setSource, cleanExternalID(set.ExternalID)); err != nil {
				return mapPGError(err)
			}
		}
		note := exercise.Notes
		if note == "" {
			note = exercise.Note
		}
		complete := status == "finished"
		if exercise.Completed != nil {
			complete = *exercise.Completed
		}
		if _, err := tx.Exec(ctx, `UPDATE training_session_exercises SET note=$1,complete=$2,updated_at=now()
			WHERE id=$3 AND session_id=$4`, note, complete, *exercise.ID, sessionID); err != nil {
			return err
		}
	}
	return nil
}

func materializeTemplateSnapshot(ctx context.Context, tx pgx.Tx, ownerID, sessionID, templateID int64, source string) error {
	type warmupSet struct {
		WeightKG *float64 `json:"weight_kg"`
		Bar      bool     `json:"bar"`
		Reps     int      `json:"reps"`
	}
	type templateExercise struct {
		exerciseID                    *int64
		position                      int
		name                          string
		workingSets, minReps, maxReps *int
		targetRIR, weightStep         *float64
		startingWeight                *float64
		restSeconds, afterSeconds     *int
		progressionType               *string
		warmupJSON                    []byte
		warmups                       []warmupSet
	}
	rows, err := tx.Query(ctx, `SELECT template_exercise.id,template_exercise.exercise_id,
		template_exercise.position,template_exercise.name,template_exercise.working_sets,
		template_exercise.min_reps,template_exercise.max_reps,template_exercise.target_rir::double precision,
		template_exercise.weight_step_kg::double precision,template_exercise.starting_weight_kg::double precision,
		template_exercise.rest_between_sets_seconds,template_exercise.rest_after_exercise_seconds,
		template_exercise.progression_type,
		COALESCE((SELECT jsonb_agg(jsonb_build_object(
			'weight_kg',warmup.weight_kg::double precision,'bar',warmup.weight_mode='bar','reps',warmup.reps
		) ORDER BY warmup.position) FROM workout_template_warmup_sets warmup
		WHERE warmup.template_exercise_id=template_exercise.id),'[]'::jsonb)
		FROM workout_template_exercises template_exercise
		JOIN workout_templates template ON template.id=template_exercise.workout_template_id
		WHERE template.id=$1 AND template.owner_id=$2
		ORDER BY template_exercise.position`, templateID, ownerID)
	if err != nil {
		return err
	}
	defer rows.Close()
	exercises := make([]templateExercise, 0)
	for rows.Next() {
		var templateExerciseID int64
		var exercise templateExercise
		if err := rows.Scan(&templateExerciseID, &exercise.exerciseID, &exercise.position, &exercise.name,
			&exercise.workingSets, &exercise.minReps, &exercise.maxReps, &exercise.targetRIR,
			&exercise.weightStep, &exercise.startingWeight, &exercise.restSeconds,
			&exercise.afterSeconds, &exercise.progressionType, &exercise.warmupJSON); err != nil {
			return err
		}
		if err := json.Unmarshal(exercise.warmupJSON, &exercise.warmups); err != nil {
			return fmt.Errorf("decode template warmup snapshot: %w", err)
		}
		exercises = append(exercises, exercise)
		_ = templateExerciseID // Template child IDs never leak into the independent session snapshot.
	}
	if err := rows.Err(); err != nil {
		return err
	}
	rows.Close()
	if len(exercises) == 0 {
		return &ValidationError{Message: "invalid workout", Fields: map[string]string{"template_id": "template has no exercises"}}
	}

	// A pgx transaction owns one connection. Finish and close the SELECT before
	// issuing INSERTs; otherwise scheduling fails immediately with "conn busy".
	for _, exercise := range exercises {
		var sessionExerciseID int64
		if err := tx.QueryRow(ctx, `INSERT INTO training_session_exercises (
			session_id,position,name,exercise_id,working_sets,min_reps,max_reps,target_rir,weight_step_kg,
			rest_between_sets_seconds,rest_after_exercise_seconds,progression_type,warmup_plan,recommendation,
			planned_weight_kg,planned_min_reps,planned_max_reps,planned_working_sets,planned_target_rir,
			planned_rest_seconds,source,updated_at)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13::jsonb,'{}'::jsonb,
				$14,$6,$7,$5,$8,$10,$15,now()) RETURNING id`, sessionID, exercise.position, exercise.name,
			exercise.exerciseID, exercise.workingSets, exercise.minReps, exercise.maxReps,
			exercise.targetRIR, exercise.weightStep, exercise.restSeconds, exercise.afterSeconds,
			exercise.progressionType, exercise.warmupJSON, exercise.startingWeight, source).Scan(&sessionExerciseID); err != nil {
			return mapPGError(err)
		}
		setPosition := 0
		for _, warmup := range exercise.warmups {
			setPosition++
			plannedWeight := warmup.WeightKG
			if warmup.Bar {
				plannedWeight = nil
			}
			if _, err := tx.Exec(ctx, `INSERT INTO training_sets (
				session_exercise_id,position,type,planned_weight_kg,planned_min_reps,planned_max_reps,source,updated_at)
				VALUES ($1,$2,'warmup',$3,$4,$4,$5,now())`, sessionExerciseID, setPosition,
				plannedWeight, warmup.Reps, source); err != nil {
				return mapPGError(err)
			}
		}
		if exercise.workingSets != nil {
			for index := 0; index < *exercise.workingSets; index++ {
				setPosition++
				if _, err := tx.Exec(ctx, `INSERT INTO training_sets (
					session_exercise_id,position,type,planned_weight_kg,planned_min_reps,planned_max_reps,
					planned_rir,rest_seconds,source,updated_at)
					VALUES ($1,$2,'working',$3,$4,$5,$6,$7,$8,now())`, sessionExerciseID, setPosition,
					exercise.startingWeight, exercise.minReps, exercise.maxReps, exercise.targetRIR,
					exercise.restSeconds, source); err != nil {
					return mapPGError(err)
				}
			}
		}
	}
	return nil
}

func resolveSessionPlan(ctx context.Context, tx pgx.Tx, ownerID int64, planID, templateID *int64) (*int64, *int64, string, error) {
	var revisionID *int64
	var programName string
	if templateID != nil {
		err := tx.QueryRow(ctx, `SELECT template.revision_id, program.name
			FROM workout_templates template
			JOIN training_program_revisions revision ON revision.id=template.revision_id
			JOIN training_programs program ON program.id=revision.program_id
			WHERE template.id=$1 AND template.owner_id=$2`, *templateID, ownerID).Scan(&revisionID, &programName)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil, "", &ValidationError{Message: "invalid workout", Fields: map[string]string{"template_id": "does not belong to the dashboard owner"}}
		}
		return templateID, revisionID, programName, err
	}
	if planID != nil {
		var selectedTemplate int64
		err := tx.QueryRow(ctx, `SELECT program.active_revision_id, program.name, template.id
			FROM training_programs program
			JOIN workout_templates template ON template.revision_id=program.active_revision_id
			WHERE program.id=$1 AND program.owner_id=$2
			ORDER BY template.position LIMIT 1`, *planID, ownerID).Scan(&revisionID, &programName, &selectedTemplate)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil, "", &ValidationError{Message: "invalid workout", Fields: map[string]string{"plan_id": "does not have an active template"}}
		}
		if err != nil {
			return nil, nil, "", err
		}
		return &selectedTemplate, revisionID, programName, nil
	}
	return nil, nil, "", nil
}

func validSessionStatus(status string) bool {
	switch status {
	case "scheduled", "active", "finished", "cancelled", "excused":
		return true
	default:
		return false
	}
}

func insertSessionExercises(ctx context.Context, tx pgx.Tx, ownerID, sessionID int64, exercises []sessionExerciseInput, sessionSource, sessionStatus string, startedAt *time.Time, loc *time.Location) error {
	for index, exercise := range exercises {
		if err := validateSessionExercise(exercise); err != nil {
			return err
		}
		position := exercise.Position
		if position <= 0 {
			position = index + 1
		}
		exerciseID, name, err := resolveExercise(ctx, tx, ownerID, exercise.ExerciseID, exercise.Name)
		if err != nil {
			return err
		}
		note := exercise.Notes
		if note == "" {
			note = exercise.Note
		}
		complete := false
		if exercise.Completed != nil {
			complete = *exercise.Completed
		} else if sessionStatus == "finished" && len(exercise.Sets) > 0 {
			complete = true
			for _, set := range exercise.Sets {
				if set.Completed == nil || !*set.Completed {
					complete = false
					break
				}
			}
		}
		source := defaultSource(exercise.Source)
		if exercise.Source == "" {
			source = sessionSource
		}
		var sessionExerciseID int64
		if err := tx.QueryRow(ctx, `INSERT INTO training_session_exercises (
			session_id,position,name,note,complete,exercise_id,rest_after_exercise_seconds,source,external_id,updated_at)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,now()) RETURNING id`, sessionID, position, name,
			note, complete, exerciseID, exercise.RestAfterExerciseSeconds, source, cleanExternalID(exercise.ExternalID)).Scan(&sessionExerciseID); err != nil {
			return mapPGError(err)
		}
		for setIndex, set := range exercise.Sets {
			setPosition := set.Position
			if setPosition <= 0 {
				setPosition = setIndex + 1
			}
			setType := strings.TrimSpace(set.Type)
			if setType == "" {
				setType = "working"
			}
			if setType != "warmup" && setType != "working" && setType != "drop" {
				return &ValidationError{Message: "invalid workout", Fields: map[string]string{"sets.type": "use warmup, working, or drop"}}
			}
			if set.WeightKG != nil && *set.WeightKG <= 0 {
				set.WeightKG = nil
			}
			completed := set.Completed != nil && *set.Completed
			completedAt, err := parseOptionalTime(set.CompletedAt, loc, "completed_at")
			if err != nil {
				return err
			}
			if completed && completedAt == nil {
				value := time.Now()
				if startedAt != nil {
					value = *startedAt
				}
				completedAt = &value
			}
			if !completed {
				completedAt = nil
			}
			notes := set.Notes
			if notes == "" {
				notes = set.Comment
			}
			setSource := defaultSource(set.Source)
			if set.Source == "" {
				setSource = source
			}
			if _, err := tx.Exec(ctx, `INSERT INTO training_sets (
				session_exercise_id,position,type,actual_weight_kg,actual_reps,actual_rir,
				started_at,completed_at,rest_seconds,notes,source,external_id,updated_at)
				VALUES ($1,$2,$3,$4,$5,$6,$7,$7,$8,$9,$10,$11,now())`, sessionExerciseID,
				setPosition, setType, set.WeightKG, set.Reps, set.RIR, completedAt, set.RestSeconds,
				notes, setSource, cleanExternalID(set.ExternalID)); err != nil {
				return mapPGError(err)
			}
		}
	}
	return nil
}

func validateSessionExercise(exercise sessionExerciseInput) error {
	if exercise.RestAfterExerciseSeconds != nil && *exercise.RestAfterExerciseSeconds < 0 {
		return &ValidationError{
			Message: "invalid workout",
			Fields:  map[string]string{"exercises.rest_after_exercise_seconds": "must be non-negative"},
		}
	}
	return nil
}

func resolveExercise(ctx context.Context, tx pgx.Tx, ownerID int64, exerciseID *int64, suppliedName string) (int64, string, error) {
	if exerciseID != nil {
		var name string
		err := tx.QueryRow(ctx, `SELECT name FROM training_exercises WHERE id=$1 AND owner_id=$2`, *exerciseID, ownerID).Scan(&name)
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, "", &ValidationError{Message: "invalid workout", Fields: map[string]string{"exercise_id": "does not belong to the dashboard owner"}}
		}
		return *exerciseID, name, err
	}
	name := strings.TrimSpace(suppliedName)
	if name == "" {
		return 0, "", &ValidationError{Message: "invalid workout", Fields: map[string]string{"exercises.name": "is required when exercise_id is absent"}}
	}
	var id int64
	err := tx.QueryRow(ctx, `INSERT INTO training_exercises (owner_id,name)
		VALUES ($1,$2) ON CONFLICT (owner_id, training_normalize_exercise_name(name)) DO UPDATE
		SET name=EXCLUDED.name, updated_at=now() RETURNING id`, ownerID, name).Scan(&id)
	return id, name, err
}

const planJSONExpression = `jsonb_build_object(
	'id', program.id, 'name', program.name, 'description', program.description,
	'days_per_week', program.days_per_week, 'active', program.active_revision_id IS NOT NULL,
	'source', program.source, 'external_id', program.external_id,
	'templates', COALESCE(templates.items, '[]'::jsonb),
	'historical_templates', COALESCE(historical_templates.items, '[]'::jsonb),
	'created_at', program.created_at, 'updated_at', program.updated_at)`

const planJSONJoin = ` LEFT JOIN LATERAL (
	SELECT jsonb_agg(jsonb_build_object(
		'id', template.id, 'name', template.name, 'description', template.description,
		'position', template.position, 'external_id', template.external_id,
		'exercises', COALESCE(exercises.items, '[]'::jsonb)
	) ORDER BY template.position) AS items
	FROM workout_templates template
	LEFT JOIN LATERAL (
		SELECT jsonb_agg(jsonb_build_object(
			'id', template_exercise.id, 'exercise_id', template_exercise.exercise_id,
			'name', template_exercise.name, 'position', template_exercise.position,
			'notes', template_exercise.notes, 'working_sets', template_exercise.working_sets,
			'min_reps', template_exercise.min_reps, 'max_reps', template_exercise.max_reps,
			'target_rir', template_exercise.target_rir::double precision,
			'weight_step_kg', template_exercise.weight_step_kg::double precision,
			'starting_weight_kg', template_exercise.starting_weight_kg::double precision,
			'progression_type', template_exercise.progression_type,
			'warmup_sets', COALESCE(warmup.items, '[]'::jsonb),
			'rest_seconds', template_exercise.rest_between_sets_seconds,
			'after_seconds', template_exercise.rest_after_exercise_seconds,
			'rest_after_exercise_seconds', template_exercise.rest_after_exercise_seconds
		) ORDER BY template_exercise.position) AS items
		FROM workout_template_exercises template_exercise
		LEFT JOIN LATERAL (
			SELECT jsonb_agg(jsonb_build_object(
				'position', warmup_set.position,
				'weight_kg', warmup_set.weight_kg::double precision,
				'weight_mode', warmup_set.weight_mode,
				'bar', warmup_set.weight_mode='bar',
				'reps', warmup_set.reps
			) ORDER BY warmup_set.position) AS items
			FROM workout_template_warmup_sets warmup_set
			WHERE warmup_set.template_exercise_id=template_exercise.id
		) warmup ON true
		WHERE template_exercise.workout_template_id=template.id
	) exercises ON true
	WHERE template.revision_id=program.active_revision_id
) templates ON true
LEFT JOIN LATERAL (
	SELECT jsonb_agg(jsonb_build_object(
		'id', historical_template.id,
		'name', historical_template.name,
		'position', historical_template.position,
		'revision_id', historical_template.revision_id
	) ORDER BY historical_revision.created_at DESC, historical_template.position) AS items
	FROM workout_templates historical_template
	JOIN training_program_revisions historical_revision ON historical_revision.id=historical_template.revision_id
	WHERE historical_revision.program_id=program.id
		AND historical_template.revision_id <> program.active_revision_id
		AND EXISTS (
			SELECT 1 FROM training_sessions historical_session
			WHERE historical_session.owner_id=program.owner_id
				AND historical_session.workout_template_id=historical_template.id
		)
) historical_templates ON true`

type planWarmupSetInput struct {
	Position   int      `json:"position"`
	WeightKG   *float64 `json:"weight_kg"`
	WeightMode string   `json:"weight_mode"`
	Bar        bool     `json:"bar"`
	Reps       int      `json:"reps"`
}

type planExerciseInput struct {
	ExerciseID               *int64               `json:"exercise_id"`
	Name                     string               `json:"name"`
	Position                 int                  `json:"position"`
	Notes                    string               `json:"notes"`
	WorkingSets              *int                 `json:"working_sets"`
	MinReps                  *int                 `json:"min_reps"`
	MaxReps                  *int                 `json:"max_reps"`
	TargetRIR                *float64             `json:"target_rir"`
	WeightStepKG             *float64             `json:"weight_step_kg"`
	StartingWeightKG         *float64             `json:"starting_weight_kg"`
	ProgressionType          *string              `json:"progression_type"`
	WarmupSets               []planWarmupSetInput `json:"warmup_sets"`
	RestSeconds              *int                 `json:"rest_seconds"`
	AfterSeconds             *int                 `json:"after_seconds"`
	RestAfterExerciseSeconds *int                 `json:"rest_after_exercise_seconds"`
}

type planTemplateInput struct {
	Name        string              `json:"name"`
	Description string              `json:"description"`
	Position    int                 `json:"position"`
	ExternalID  string              `json:"external_id"`
	Exercises   []planExerciseInput `json:"exercises"`
}

type planInput struct {
	Name        string              `json:"name"`
	Description string              `json:"description"`
	DaysPerWeek *int                `json:"days_per_week"`
	Source      string              `json:"source"`
	ExternalID  *string             `json:"external_id"`
	Templates   []planTemplateInput `json:"templates"`
}

func (r *PostgresRepository) listPlans(ctx context.Context, ownerID int64, options Pagination) ([]json.RawMessage, int, error) {
	offset := (options.Page - 1) * options.PageSize
	rows, err := r.pool.Query(ctx, `SELECT `+planJSONExpression+` FROM training_programs program `+planJSONJoin+`
		WHERE program.owner_id=$1 AND ($2::text='' OR program.name ILIKE '%' || $2 || '%')
		ORDER BY program.name,program.id LIMIT $3 OFFSET $4`, ownerID, options.Search, options.PageSize, offset)
	if err != nil {
		return nil, 0, err
	}
	items, err := collectJSONRows(rows)
	if err != nil {
		return nil, 0, err
	}
	var total int
	err = r.pool.QueryRow(ctx, `SELECT count(*) FROM training_programs WHERE owner_id=$1
		AND ($2::text='' OR name ILIKE '%' || $2 || '%')`, ownerID, options.Search).Scan(&total)
	return items, total, err
}

func (r *PostgresRepository) getPlan(ctx context.Context, ownerID, id int64) (json.RawMessage, error) {
	var raw json.RawMessage
	err := r.pool.QueryRow(ctx, `SELECT `+planJSONExpression+` FROM training_programs program `+planJSONJoin+
		` WHERE program.owner_id=$1 AND program.id=$2`, ownerID, id).Scan(&raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return raw, err
}

func (r *PostgresRepository) createPlan(ctx context.Context, ownerID int64, raw json.RawMessage) (json.RawMessage, error) {
	return r.savePlan(ctx, ownerID, 0, raw)
}

func (r *PostgresRepository) updatePlan(ctx context.Context, ownerID, id int64, raw json.RawMessage) (json.RawMessage, error) {
	return r.savePlan(ctx, ownerID, id, raw)
}

func (r *PostgresRepository) savePlan(ctx context.Context, ownerID, id int64, raw json.RawMessage) (json.RawMessage, error) {
	var input planInput
	if err := decodeStrict(raw, &input); err != nil {
		return nil, err
	}
	input.Name = strings.TrimSpace(input.Name)
	if input.Name == "" {
		return nil, &ValidationError{Message: "invalid workout plan", Fields: map[string]string{"name": "is required"}}
	}
	if input.DaysPerWeek != nil && (*input.DaysPerWeek < 1 || *input.DaysPerWeek > 7) {
		return nil, &ValidationError{Message: "invalid workout plan", Fields: map[string]string{"days_per_week": "must be between 1 and 7"}}
	}
	if len(input.Templates) == 0 {
		return nil, &ValidationError{Message: "invalid workout plan", Fields: map[string]string{"templates": "at least one template is required"}}
	}
	input.Source = defaultSource(input.Source)
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if id == 0 {
		err = tx.QueryRow(ctx, `INSERT INTO training_programs
			(owner_id,name,description,days_per_week,source,external_id)
			VALUES ($1,$2,$3,$4,$5,$6) RETURNING id`, ownerID, input.Name, input.Description,
			input.DaysPerWeek, input.Source, cleanExternalID(input.ExternalID)).Scan(&id)
	} else {
		command, updateErr := tx.Exec(ctx, `UPDATE training_programs SET name=$3,description=$4,
			days_per_week=$5,source=$6,external_id=$7,updated_at=now() WHERE id=$1 AND owner_id=$2`,
			id, ownerID, input.Name, input.Description, input.DaysPerWeek, input.Source, cleanExternalID(input.ExternalID))
		err = updateErr
		if err == nil && command.RowsAffected() == 0 {
			return nil, ErrNotFound
		}
	}
	if err != nil {
		return nil, mapPGError(err)
	}
	var revisionNumber int
	if err := tx.QueryRow(ctx, `SELECT COALESCE(max(revision),0)+1 FROM training_program_revisions WHERE program_id=$1`, id).Scan(&revisionNumber); err != nil {
		return nil, err
	}
	var revisionID int64
	if err := tx.QueryRow(ctx, `INSERT INTO training_program_revisions (program_id,revision,raw_source)
		VALUES ($1,$2,$3) RETURNING id`, id, revisionNumber, string(raw)).Scan(&revisionID); err != nil {
		return nil, err
	}
	for index, template := range input.Templates {
		template.Name = strings.TrimSpace(template.Name)
		if template.Name == "" {
			return nil, &ValidationError{Message: "invalid workout plan", Fields: map[string]string{"templates.name": "is required"}}
		}
		if len(template.Exercises) == 0 {
			return nil, &ValidationError{Message: "invalid workout plan", Fields: map[string]string{"templates.exercises": "at least one exercise is required"}}
		}
		position := template.Position
		if position <= 0 {
			position = index + 1
		}
		externalID := strings.TrimSpace(template.ExternalID)
		if externalID == "" {
			externalID = fmt.Sprintf("web-%d-%d", revisionNumber, position)
		}
		var templateID int64
		if err := tx.QueryRow(ctx, `INSERT INTO workout_templates
			(owner_id,name,sort_order,revision_id,external_id,position,description,source)
			VALUES ($1,$2,$3,$4,$5,$3,$6,$7) RETURNING id`, ownerID, template.Name, position,
			revisionID, externalID, template.Description, input.Source).Scan(&templateID); err != nil {
			return nil, mapPGError(err)
		}
		for exerciseIndex, exercise := range template.Exercises {
			if err := validatePlanExercise(exercise); err != nil {
				return nil, err
			}
			exercisePosition := exercise.Position
			if exercisePosition <= 0 {
				exercisePosition = exerciseIndex + 1
			}
			exerciseID, name, err := resolveExercise(ctx, tx, ownerID, exercise.ExerciseID, exercise.Name)
			if err != nil {
				return nil, err
			}
			progressionType := normalizedProgressionType(exercise)
			var templateExerciseID int64
			if err := tx.QueryRow(ctx, `INSERT INTO workout_template_exercises (
				workout_template_id,position,name,exercise_id,working_sets,min_reps,max_reps,
				target_rir,weight_step_kg,starting_weight_kg,rest_between_sets_seconds,
				rest_after_exercise_seconds,notes,progression_type)
				VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,NULLIF($14,''))
				RETURNING id`, templateID, exercisePosition,
				name, exerciseID, exercise.WorkingSets, exercise.MinReps, exercise.MaxReps,
				exercise.TargetRIR, exercise.WeightStepKG, exercise.StartingWeightKG, exercise.RestSeconds,
				firstNonNilInt(exercise.RestAfterExerciseSeconds, exercise.AfterSeconds), exercise.Notes,
				progressionType).Scan(&templateExerciseID); err != nil {
				return nil, mapPGError(err)
			}
			for warmupIndex, warmup := range exercise.WarmupSets {
				position := warmup.Position
				if position <= 0 {
					position = warmupIndex + 1
				}
				mode := normalizedWarmupMode(warmup)
				if _, err := tx.Exec(ctx, `INSERT INTO workout_template_warmup_sets
					(template_exercise_id,position,weight_kg,weight_mode,reps)
					VALUES ($1,$2,$3,$4,$5)`, templateExerciseID, position, warmup.WeightKG, mode, warmup.Reps); err != nil {
					return nil, mapPGError(err)
				}
			}
		}
	}
	if _, err := tx.Exec(ctx, `UPDATE training_programs SET active_revision_id=$1,updated_at=now() WHERE id=$2`, revisionID, id); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, mapPGError(err)
	}
	return r.getPlan(ctx, ownerID, id)
}

func validatePlanExercise(exercise planExerciseInput) error {
	fieldError := func(field, message string) error {
		return &ValidationError{Message: "invalid workout plan", Fields: map[string]string{"templates.exercises." + field: message}}
	}
	if exercise.WorkingSets != nil && *exercise.WorkingSets <= 0 {
		return fieldError("working_sets", "must be greater than zero")
	}
	if exercise.MinReps != nil && *exercise.MinReps <= 0 {
		return fieldError("min_reps", "must be greater than zero")
	}
	if exercise.MaxReps != nil && *exercise.MaxReps <= 0 {
		return fieldError("max_reps", "must be greater than zero")
	}
	if exercise.MinReps != nil && exercise.MaxReps != nil && *exercise.MaxReps < *exercise.MinReps {
		return fieldError("max_reps", "must be greater than or equal to min_reps")
	}
	if exercise.TargetRIR != nil && *exercise.TargetRIR < 0 {
		return fieldError("target_rir", "must be non-negative")
	}
	if exercise.WeightStepKG != nil && *exercise.WeightStepKG <= 0 {
		return fieldError("weight_step_kg", "must be greater than zero")
	}
	if exercise.StartingWeightKG != nil && *exercise.StartingWeightKG <= 0 {
		return fieldError("starting_weight_kg", "must be greater than zero")
	}
	if exercise.RestSeconds != nil && *exercise.RestSeconds < 0 {
		return fieldError("rest_seconds", "must be non-negative")
	}
	after := firstNonNilInt(exercise.RestAfterExerciseSeconds, exercise.AfterSeconds)
	if after != nil && *after < 0 {
		return fieldError("rest_after_exercise_seconds", "must be non-negative")
	}
	progression := ""
	if exercise.ProgressionType != nil {
		progression = strings.TrimSpace(*exercise.ProgressionType)
		if progression != "" && progression != "double" {
			return fieldError("progression_type", "use double")
		}
	}
	structured := exercise.WorkingSets != nil || exercise.MinReps != nil || exercise.MaxReps != nil ||
		exercise.TargetRIR != nil || exercise.WeightStepKG != nil || exercise.StartingWeightKG != nil ||
		progression != "" || len(exercise.WarmupSets) > 0
	if structured && (exercise.WorkingSets == nil || exercise.MinReps == nil || exercise.MaxReps == nil || exercise.WeightStepKG == nil) {
		return fieldError("weight_step_kg", "structured progression requires working_sets, min_reps, max_reps, and weight_step_kg")
	}
	for _, warmup := range exercise.WarmupSets {
		if warmup.Reps <= 0 {
			return fieldError("warmup_sets.reps", "must be greater than zero")
		}
		mode := normalizedWarmupMode(warmup)
		if mode != "bar" && mode != "kg" {
			return fieldError("warmup_sets.weight_mode", "use kg or bar")
		}
		if mode == "bar" && warmup.WeightKG != nil {
			return fieldError("warmup_sets.weight_kg", "must be empty for bar warmup")
		}
		if mode == "kg" && (warmup.WeightKG == nil || *warmup.WeightKG <= 0) {
			return fieldError("warmup_sets.weight_kg", "must be greater than zero for kg warmup")
		}
	}
	return nil
}

func normalizedProgressionType(exercise planExerciseInput) string {
	if exercise.ProgressionType != nil && strings.TrimSpace(*exercise.ProgressionType) != "" {
		return strings.TrimSpace(*exercise.ProgressionType)
	}
	if exercise.WorkingSets != nil && exercise.MinReps != nil && exercise.MaxReps != nil && exercise.WeightStepKG != nil {
		return "double"
	}
	return ""
}

func normalizedWarmupMode(warmup planWarmupSetInput) string {
	if mode := strings.TrimSpace(warmup.WeightMode); mode != "" {
		return mode
	}
	if warmup.Bar {
		return "bar"
	}
	return "kg"
}

func firstNonNilInt(primary, fallback *int) *int {
	if primary != nil {
		return primary
	}
	return fallback
}

const exerciseJSONQuery = `SELECT jsonb_build_object(
	'id',e.id,'name',e.name,'slug',e.slug,'primary_muscle_group',e.primary_muscle_group,
	'secondary_muscle_groups',e.secondary_muscle_groups,
	'muscle_groups', CASE WHEN e.primary_muscle_group IS NULL THEN e.secondary_muscle_groups
		ELSE array_prepend(e.primary_muscle_group,e.secondary_muscle_groups) END,
	'exercise_type',e.exercise_type,'equipment',e.equipment,'unilateral',e.unilateral,
	'notes',e.notes,'source',e.source,'external_id',e.external_id,
	'created_at',e.created_at,'updated_at',e.updated_at) FROM training_exercises e`

type exerciseInput struct {
	Name                  string          `json:"name"`
	Slug                  *string         `json:"slug"`
	PrimaryMuscleGroup    *string         `json:"primary_muscle_group"`
	SecondaryMuscleGroups []string        `json:"secondary_muscle_groups"`
	MuscleGroups          json.RawMessage `json:"muscle_groups"`
	ExerciseType          *string         `json:"exercise_type"`
	Equipment             *string         `json:"equipment"`
	Unilateral            bool            `json:"unilateral"`
	Notes                 string          `json:"notes"`
	Source                string          `json:"source"`
	ExternalID            *string         `json:"external_id"`
}

func (r *PostgresRepository) listExercises(ctx context.Context, ownerID int64, options Pagination) ([]json.RawMessage, int, error) {
	offset := (options.Page - 1) * options.PageSize
	rows, err := r.pool.Query(ctx, exerciseJSONQuery+` WHERE e.owner_id=$1
		AND ($2::text='' OR e.name ILIKE '%' || $2 || '%' OR e.primary_muscle_group ILIKE '%' || $2 || '%')
		ORDER BY e.name,e.id LIMIT $3 OFFSET $4`, ownerID, options.Search, options.PageSize, offset)
	if err != nil {
		return nil, 0, err
	}
	items, err := collectJSONRows(rows)
	if err != nil {
		return nil, 0, err
	}
	var total int
	err = r.pool.QueryRow(ctx, `SELECT count(*) FROM training_exercises e WHERE e.owner_id=$1
		AND ($2::text='' OR e.name ILIKE '%' || $2 || '%' OR e.primary_muscle_group ILIKE '%' || $2 || '%')`, ownerID, options.Search).Scan(&total)
	return items, total, err
}

func (r *PostgresRepository) createExercise(ctx context.Context, ownerID int64, raw json.RawMessage, loc *time.Location) (json.RawMessage, error) {
	return r.saveExercise(ctx, ownerID, 0, raw, loc)
}

func (r *PostgresRepository) updateExercise(ctx context.Context, ownerID, id int64, raw json.RawMessage, loc *time.Location) (json.RawMessage, error) {
	return r.saveExercise(ctx, ownerID, id, raw, loc)
}

func (r *PostgresRepository) saveExercise(ctx context.Context, ownerID, id int64, raw json.RawMessage, _ *time.Location) (json.RawMessage, error) {
	var input exerciseInput
	if err := decodeStrict(raw, &input); err != nil {
		return nil, err
	}
	input.Name = strings.TrimSpace(input.Name)
	if input.Name == "" {
		return nil, &ValidationError{Message: "invalid exercise", Fields: map[string]string{"name": "is required"}}
	}
	if len(input.MuscleGroups) > 0 && string(input.MuscleGroups) != "null" {
		var groups []string
		if err := json.Unmarshal(input.MuscleGroups, &groups); err != nil {
			var joined string
			if stringErr := json.Unmarshal(input.MuscleGroups, &joined); stringErr != nil {
				return nil, &ValidationError{Message: "invalid exercise", Fields: map[string]string{"muscle_groups": "must be a string or string array"}}
			}
			for _, group := range strings.Split(joined, ",") {
				if trimmed := strings.TrimSpace(group); trimmed != "" {
					groups = append(groups, trimmed)
				}
			}
		}
		if len(groups) > 0 && input.PrimaryMuscleGroup == nil {
			input.PrimaryMuscleGroup = &groups[0]
			input.SecondaryMuscleGroups = groups[1:]
		}
	}
	input.Source = defaultSource(input.Source)
	var err error
	if id == 0 {
		err = r.pool.QueryRow(ctx, `INSERT INTO training_exercises (
			owner_id,name,slug,primary_muscle_group,secondary_muscle_groups,exercise_type,
			equipment,unilateral,notes,source,external_id)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11) RETURNING id`, ownerID, input.Name,
			input.Slug, input.PrimaryMuscleGroup, input.SecondaryMuscleGroups, input.ExerciseType,
			input.Equipment, input.Unilateral, input.Notes, input.Source, cleanExternalID(input.ExternalID)).Scan(&id)
	} else {
		command, updateErr := r.pool.Exec(ctx, `UPDATE training_exercises SET name=$3,slug=$4,
			primary_muscle_group=$5,secondary_muscle_groups=$6,exercise_type=$7,equipment=$8,
			unilateral=$9,notes=$10,source=$11,external_id=$12,updated_at=now()
			WHERE id=$1 AND owner_id=$2`, id, ownerID, input.Name, input.Slug, input.PrimaryMuscleGroup,
			input.SecondaryMuscleGroups, input.ExerciseType, input.Equipment, input.Unilateral,
			input.Notes, input.Source, cleanExternalID(input.ExternalID))
		err = updateErr
		if err == nil && command.RowsAffected() == 0 {
			return nil, ErrNotFound
		}
	}
	if err != nil {
		return nil, mapPGError(err)
	}
	var result json.RawMessage
	if err := r.pool.QueryRow(ctx, exerciseJSONQuery+` WHERE e.owner_id=$1 AND e.id=$2`, ownerID, id).Scan(&result); err != nil {
		return nil, err
	}
	return result, nil
}
