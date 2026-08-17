package storage

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"fitlog/internal/training"
	"fitlog/internal/training/progression"
)

type TrainingRepo struct {
	pool *pgxpool.Pool
}

func NewTrainingRepo(pool *pgxpool.Pool) *TrainingRepo {
	return &TrainingRepo{pool: pool}
}

func (r *TrainingRepo) GetUIState(ctx context.Context, ownerID int64) (training.UIState, error) {
	const q = `
		SELECT owner_id, chat_id, message_id, mode, pending_import,
		       pending_exercise_id, pending_exercise_name,
		       pending_program_exercise_id, pending_target_exercise_id,
		       pending_set
		FROM training_ui_states
		WHERE owner_id = $1`
	var state training.UIState
	var mode string
	var pending, pendingSet []byte
	var pendingExerciseID pgtype.Int8
	var pendingProgramExerciseID, pendingTargetExerciseID pgtype.Int8
	err := r.pool.QueryRow(ctx, q, ownerID).Scan(
		&state.OwnerID, &state.ChatID, &state.MessageID, &mode, &pending,
		&pendingExerciseID, &state.PendingExerciseName,
		&pendingProgramExerciseID, &pendingTargetExerciseID, &pendingSet,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return training.UIState{}, training.ErrNotFound
	}
	if err != nil {
		return training.UIState{}, fmt.Errorf("select training UI state: %w", err)
	}
	state.Mode = training.InputMode(mode)
	if pendingExerciseID.Valid {
		value := pendingExerciseID.Int64
		state.PendingExerciseID = &value
	}
	if pendingProgramExerciseID.Valid {
		value := pendingProgramExerciseID.Int64
		state.PendingProgramExerciseID = &value
	}
	if pendingTargetExerciseID.Valid {
		value := pendingTargetExerciseID.Int64
		state.PendingTargetExerciseID = &value
	}
	if len(pending) > 0 {
		var preview training.ImportPreview
		if err := json.Unmarshal(pending, &preview); err != nil {
			return training.UIState{}, fmt.Errorf("decode pending training import: %w", err)
		}
		state.PendingImport = &preview
	}
	if len(pendingSet) > 0 {
		var set training.PendingSet
		if err := json.Unmarshal(pendingSet, &set); err != nil {
			return training.UIState{}, fmt.Errorf("decode pending training set: %w", err)
		}
		state.PendingSet = &set
	}
	return state, nil
}

func (r *TrainingRepo) SaveUIState(ctx context.Context, state training.UIState) error {
	var pending []byte
	var err error
	if state.PendingImport != nil {
		pending, err = json.Marshal(state.PendingImport)
		if err != nil {
			return fmt.Errorf("encode pending training import: %w", err)
		}
	}
	var pendingSet []byte
	if state.PendingSet != nil {
		pendingSet, err = json.Marshal(state.PendingSet)
		if err != nil {
			return fmt.Errorf("encode pending training set: %w", err)
		}
	}
	const q = `
		INSERT INTO training_ui_states (
			owner_id, chat_id, message_id, mode, pending_import,
			pending_exercise_id, pending_exercise_name,
			pending_program_exercise_id, pending_target_exercise_id, pending_set, updated_at
		)
		VALUES ($1, $2, $3, $4, $5::jsonb, $6, $7, $8, $9, $10::jsonb, now())
		ON CONFLICT (owner_id) DO UPDATE SET
			chat_id = EXCLUDED.chat_id,
			message_id = EXCLUDED.message_id,
			mode = EXCLUDED.mode,
			pending_import = EXCLUDED.pending_import,
			pending_exercise_id = EXCLUDED.pending_exercise_id,
			pending_exercise_name = EXCLUDED.pending_exercise_name,
			pending_program_exercise_id = EXCLUDED.pending_program_exercise_id,
			pending_target_exercise_id = EXCLUDED.pending_target_exercise_id,
			pending_set = EXCLUDED.pending_set,
			updated_at = now()`
	if _, err := r.pool.Exec(
		ctx, q,
		state.OwnerID, state.ChatID, state.MessageID, string(state.Mode), pending,
		state.PendingExerciseID, state.PendingExerciseName,
		state.PendingProgramExerciseID, state.PendingTargetExerciseID, pendingSet,
	); err != nil {
		return fmt.Errorf("upsert training UI state: %w", err)
	}
	return nil
}

func (r *TrainingRepo) ListPrograms(ctx context.Context, ownerID int64) ([]training.Program, error) {
	const q = `
		SELECT template.id, template.owner_id, program.id, revision.id, revision.revision,
		       template.name, program.name, template.external_id,
		       exercise.id, exercise.exercise_id, exercise.position, exercise.name,
		       exercise.working_sets, exercise.min_reps, exercise.max_reps,
		       exercise.target_rir::double precision,
		       exercise.weight_step_kg::double precision,
		       exercise.starting_weight_kg::double precision,
		       exercise.rest_between_sets_seconds, exercise.rest_after_exercise_seconds,
		       exercise.progression_type,
		       COALESCE((
		           SELECT jsonb_agg(jsonb_build_object(
		               'weight', warmup.weight_kg::double precision,
		               'bar', warmup.weight_mode = 'bar',
		               'reps', warmup.reps
		           ) ORDER BY warmup.position)
		           FROM workout_template_warmup_sets warmup
		           WHERE warmup.template_exercise_id = exercise.id
		       ), '[]'::jsonb)
		FROM workout_templates template
		JOIN training_program_revisions revision ON revision.id = template.revision_id
		JOIN training_programs program ON program.id = revision.program_id
		LEFT JOIN workout_template_exercises exercise ON exercise.workout_template_id = template.id
		WHERE template.owner_id = $1 AND program.active_revision_id = template.revision_id
		ORDER BY template.position, template.id, exercise.position`
	rows, err := r.pool.Query(ctx, q, ownerID)
	if err != nil {
		return nil, fmt.Errorf("select training programs: %w", err)
	}
	defer rows.Close()

	programs := make([]training.Program, 0)
	indexes := make(map[int64]int)
	for rows.Next() {
		var id, rowOwnerID, planID, revisionID int64
		var revisionNumber int
		var name, planName, workoutKey string
		var programExerciseID, exerciseID pgtype.Int8
		var position pgtype.Int4
		var exerciseName pgtype.Text
		var workingSets, minReps, maxReps, restSeconds, afterSeconds pgtype.Int4
		var targetRIR, weightStep, startingWeight pgtype.Float8
		var progressionType pgtype.Text
		var warmupJSON []byte
		if err := rows.Scan(
			&id, &rowOwnerID, &planID, &revisionID, &revisionNumber,
			&name, &planName, &workoutKey,
			&programExerciseID, &exerciseID, &position, &exerciseName,
			&workingSets, &minReps, &maxReps, &targetRIR, &weightStep, &startingWeight,
			&restSeconds, &afterSeconds, &progressionType, &warmupJSON,
		); err != nil {
			return nil, fmt.Errorf("scan training program: %w", err)
		}
		index, ok := indexes[id]
		if !ok {
			index = len(programs)
			indexes[id] = index
			programs = append(programs, training.Program{
				ID: id, OwnerID: rowOwnerID, PlanID: planID, RevisionID: revisionID, Revision: revisionNumber,
				Name: name, PlanName: planName, WorkoutKey: workoutKey,
			})
		}
		if exerciseName.Valid {
			item := training.ProgramExercise{
				ID: programExerciseID.Int64, Position: int(position.Int32), Name: exerciseName.String,
			}
			if exerciseID.Valid {
				value := exerciseID.Int64
				item.ExerciseID = &value
			}
			if workingSets.Valid {
				item.Prescription.WorkingSets = int(workingSets.Int32)
				item.Prescription.Reps = training.RepRange{Min: int(minReps.Int32), Max: int(maxReps.Int32)}
				item.Prescription.TargetRIR = targetRIR.Float64
				item.Prescription.WeightStepKG = weightStep.Float64
				item.Prescription.RestSeconds = int(restSeconds.Int32)
				item.Prescription.AfterSeconds = int(afterSeconds.Int32)
				item.Prescription.Progression = progressionType.String
				if startingWeight.Valid {
					value := startingWeight.Float64
					item.Prescription.StartingWeight = &value
				}
				if err := json.Unmarshal(warmupJSON, &item.Prescription.Warmup); err != nil {
					return nil, fmt.Errorf("decode warmup for %q: %w", exerciseName.String, err)
				}
			}
			programs[index].Exercises = append(programs[index].Exercises, exerciseName.String)
			programs[index].ExerciseItems = append(programs[index].ExerciseItems, item)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate training programs: %w", err)
	}
	return programs, nil
}

func (r *TrainingRepo) Program(ctx context.Context, ownerID, programID int64) (training.Program, error) {
	programs, err := r.ListPrograms(ctx, ownerID)
	if err != nil {
		return training.Program{}, err
	}
	for _, program := range programs {
		if program.ID == programID {
			return program, nil
		}
	}
	return training.Program{}, training.ErrNotFound
}

func (r *TrainingRepo) ReplacePrograms(ctx context.Context, ownerID int64, programs []training.ProgramInput) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin program import: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	groups := groupProgramInputs(programs)
	for _, group := range groups {
		first := group[0]
		planName := first.PlanName
		if planName == "" {
			planName = first.Name
		}
		const upsertPlan = `
			INSERT INTO training_programs (owner_id, name, description, days_per_week)
			VALUES ($1, $2, $3, NULLIF($4, 0))
			ON CONFLICT (owner_id, (training_normalize_exercise_name(name))) DO UPDATE SET
				name = EXCLUDED.name,
				description = EXCLUDED.description,
				days_per_week = EXCLUDED.days_per_week,
				updated_at = now()
			RETURNING id`
		var planID int64
		if err := tx.QueryRow(
			ctx, upsertPlan, ownerID, planName, first.PlanDescription, first.DaysPerWeek,
		).Scan(&planID); err != nil {
			return fmt.Errorf("upsert training program %q: %w", planName, err)
		}
		var revisionNumber int
		if err := tx.QueryRow(ctx, `
			SELECT COALESCE(max(revision), 0) + 1
			FROM training_program_revisions
			WHERE program_id = $1`, planID,
		).Scan(&revisionNumber); err != nil {
			return fmt.Errorf("select next revision for %q: %w", planName, err)
		}
		var revisionID int64
		if err := tx.QueryRow(ctx, `
			INSERT INTO training_program_revisions (program_id, revision, raw_source)
			VALUES ($1, $2, $3)
			RETURNING id`, planID, revisionNumber, first.RawSource,
		).Scan(&revisionID); err != nil {
			return fmt.Errorf("insert revision %d for %q: %w", revisionNumber, planName, err)
		}

		for workoutIndex, program := range group {
			workoutKey := program.WorkoutKey
			if workoutKey == "" {
				workoutKey = "legacy"
			}
			var templateID int64
			if err := tx.QueryRow(ctx, `
				INSERT INTO workout_templates (
					owner_id, name, sort_order, revision_id, external_id, position
				)
				VALUES ($1, $2, $3, $4, $5, $3)
				RETURNING id`, ownerID, program.Name, workoutIndex+1, revisionID, workoutKey,
			).Scan(&templateID); err != nil {
				return fmt.Errorf("insert workout %q: %w", program.Name, err)
			}
			for exerciseIndex, exercise := range program.Exercises {
				catalogID, catalogName, err := ensureTrainingExercise(ctx, tx, ownerID, exercise)
				if err != nil {
					return fmt.Errorf("ensure exercise %q: %w", exercise, err)
				}
				var prescription training.ExercisePrescription
				if exerciseIndex < len(program.Prescriptions) {
					prescription = program.Prescriptions[exerciseIndex]
				}
				var templateExerciseID int64
				if err := tx.QueryRow(ctx, `
					INSERT INTO workout_template_exercises (
						workout_template_id, position, name, exercise_id,
						working_sets, min_reps, max_reps, target_rir,
						weight_step_kg, starting_weight_kg,
						rest_between_sets_seconds, rest_after_exercise_seconds, progression_type
					)
					VALUES (
						$1, $2, $3, $4,
						NULLIF($5, 0), NULLIF($6, 0), NULLIF($7, 0), $8,
						$9, $10, $11, $12, NULLIF($13, '')
					)
					RETURNING id`,
					templateID, exerciseIndex+1, catalogName, catalogID,
					prescription.WorkingSets, prescription.Reps.Min, prescription.Reps.Max, nullableStructuredFloat(prescription, prescription.TargetRIR),
					nullableStructuredFloat(prescription, prescription.WeightStepKG), prescription.StartingWeight,
					nullableStructuredInt(prescription, prescription.RestSeconds), nullableStructuredInt(prescription, prescription.AfterSeconds), prescription.Progression,
				).Scan(&templateExerciseID); err != nil {
					return fmt.Errorf("insert exercise %q: %w", exercise, err)
				}
				for warmupIndex, warmup := range prescription.Warmup {
					mode := "kg"
					if warmup.Bar {
						mode = "bar"
					}
					if _, err := tx.Exec(ctx, `
						INSERT INTO workout_template_warmup_sets (
							template_exercise_id, position, weight_kg, weight_mode, reps
						) VALUES ($1, $2, $3, $4, $5)`,
						templateExerciseID, warmupIndex+1, warmup.WeightKG, mode, warmup.Reps,
					); err != nil {
						return fmt.Errorf("insert warmup for %q: %w", exercise, err)
					}
				}
			}
		}
		if _, err := tx.Exec(ctx, `
			UPDATE training_programs SET active_revision_id = $1, updated_at = now() WHERE id = $2`,
			revisionID, planID,
		); err != nil {
			return fmt.Errorf("activate revision %d for %q: %w", revisionNumber, planName, err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit program import: %w", err)
	}
	return nil
}

func groupProgramInputs(programs []training.ProgramInput) [][]training.ProgramInput {
	groups := make([][]training.ProgramInput, 0)
	indexes := make(map[string]int)
	for _, program := range programs {
		name := program.PlanName
		if name == "" {
			name = program.Name
		}
		key := strings.ToLower(strings.TrimSpace(name))
		if index, ok := indexes[key]; ok {
			groups[index] = append(groups[index], program)
			continue
		}
		indexes[key] = len(groups)
		groups = append(groups, []training.ProgramInput{program})
	}
	return groups
}

func nullableStructuredFloat(plan training.ExercisePrescription, value float64) any {
	if !plan.Structured() {
		return nil
	}
	return value
}

func nullableStructuredInt(plan training.ExercisePrescription, value int) any {
	if !plan.Structured() {
		return nil
	}
	return value
}

func (r *TrainingRepo) ListExercises(ctx context.Context, ownerID int64, limit, offset int) ([]training.Exercise, int, error) {
	if limit <= 0 || limit > 100 {
		limit = 5
	}
	if offset < 0 {
		offset = 0
	}
	var total int
	if err := r.pool.QueryRow(ctx, `SELECT count(*) FROM training_exercises WHERE owner_id = $1`, ownerID).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count training exercises: %w", err)
	}
	rows, err := r.pool.Query(ctx, `
		SELECT e.id, e.owner_id, e.name,
		       COALESCE(array_agg(DISTINCT p.name ORDER BY p.name) FILTER (WHERE p.id IS NOT NULL), '{}')
		FROM training_exercises e
		LEFT JOIN workout_template_exercises pe ON pe.exercise_id = e.id
		LEFT JOIN workout_templates p ON p.id = pe.workout_template_id
		WHERE e.owner_id = $1
		GROUP BY e.id
		ORDER BY training_normalize_exercise_name(e.name), e.id
		LIMIT $2 OFFSET $3`, ownerID, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("select training exercises: %w", err)
	}
	defer rows.Close()
	items := make([]training.Exercise, 0, limit)
	for rows.Next() {
		var item training.Exercise
		if err := rows.Scan(&item.ID, &item.OwnerID, &item.Name, &item.Programs); err != nil {
			return nil, 0, fmt.Errorf("scan training exercise: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate training exercises: %w", err)
	}
	return items, total, nil
}

func (r *TrainingRepo) SimilarExercises(ctx context.Context, ownerID int64, name string, limit int) ([]training.Exercise, error) {
	if limit <= 0 || limit > 20 {
		limit = 5
	}
	rows, err := r.pool.Query(ctx, `
		SELECT e.id, e.owner_id, e.name,
		       COALESCE(array_agg(DISTINCT p.name ORDER BY p.name) FILTER (WHERE p.id IS NOT NULL), '{}')
		FROM training_exercises e
		LEFT JOIN workout_template_exercises pe ON pe.exercise_id = e.id
		LEFT JOIN workout_templates p ON p.id = pe.workout_template_id
		WHERE e.owner_id = $1
		  AND (
		      training_normalize_exercise_name(e.name) = training_normalize_exercise_name($2)
		      OR training_normalize_exercise_name(e.name) LIKE '%' || training_normalize_exercise_name($2) || '%'
		      OR training_normalize_exercise_name($2) LIKE '%' || training_normalize_exercise_name(e.name) || '%'
		      OR EXISTS (
		          SELECT 1
		          FROM regexp_split_to_table(training_normalize_exercise_name($2), '\s+') token
		          WHERE length(token) >= 4 AND training_normalize_exercise_name(e.name) LIKE '%' || token || '%'
		      )
		  )
		GROUP BY e.id
		ORDER BY (training_normalize_exercise_name(e.name) = training_normalize_exercise_name($2)) DESC,
		         (training_normalize_exercise_name(e.name) LIKE '%' || training_normalize_exercise_name($2) || '%') DESC,
		         training_normalize_exercise_name(e.name), e.id
		LIMIT $3`, ownerID, name, limit)
	if err != nil {
		return nil, fmt.Errorf("select similar training exercises: %w", err)
	}
	defer rows.Close()
	var items []training.Exercise
	for rows.Next() {
		var item training.Exercise
		if err := rows.Scan(&item.ID, &item.OwnerID, &item.Name, &item.Programs); err != nil {
			return nil, fmt.Errorf("scan similar training exercise: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate similar training exercises: %w", err)
	}
	return items, nil
}

func (r *TrainingRepo) Exercise(ctx context.Context, ownerID, exerciseID int64) (training.Exercise, error) {
	var item training.Exercise
	err := r.pool.QueryRow(ctx, `
		SELECT e.id, e.owner_id, e.name,
		       COALESCE(array_agg(DISTINCT p.name ORDER BY p.name) FILTER (WHERE p.id IS NOT NULL), '{}')
		FROM training_exercises e
		LEFT JOIN workout_template_exercises pe ON pe.exercise_id = e.id
		LEFT JOIN workout_templates p ON p.id = pe.workout_template_id
		WHERE e.id = $1 AND e.owner_id = $2
		GROUP BY e.id`, exerciseID, ownerID,
	).Scan(&item.ID, &item.OwnerID, &item.Name, &item.Programs)
	if errors.Is(err, pgx.ErrNoRows) {
		return training.Exercise{}, training.ErrNotFound
	}
	if err != nil {
		return training.Exercise{}, fmt.Errorf("select training exercise: %w", err)
	}
	return item, nil
}

func (r *TrainingRepo) RenameExercise(ctx context.Context, ownerID, exerciseID int64, name string) (training.RenameResult, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return training.RenameResult{}, fmt.Errorf("begin rename exercise: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var currentName string
	if err := tx.QueryRow(ctx, `
		SELECT name FROM training_exercises WHERE id = $1 AND owner_id = $2 FOR UPDATE`, exerciseID, ownerID,
	).Scan(&currentName); errors.Is(err, pgx.ErrNoRows) {
		return training.RenameResult{}, training.ErrNotFound
	} else if err != nil {
		return training.RenameResult{}, fmt.Errorf("lock exercise to rename: %w", err)
	}

	rows, err := tx.Query(ctx, `
		SELECT DISTINCT s.id
		FROM training_sessions s
		JOIN training_session_exercises e ON e.session_id = s.id
		WHERE s.owner_id = $1
		  AND (e.exercise_id = $2 OR (e.exercise_id IS NULL AND training_normalize_exercise_name(e.name) = training_normalize_exercise_name($3)))
		  AND s.published_chat_id IS NOT NULL AND s.published_message_id IS NOT NULL`, ownerID, exerciseID, currentName)
	if err != nil {
		return training.RenameResult{}, fmt.Errorf("select published sessions for rename: %w", err)
	}
	var publishedIDs []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return training.RenameResult{}, fmt.Errorf("scan published session for rename: %w", err)
		}
		publishedIDs = append(publishedIDs, id)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return training.RenameResult{}, fmt.Errorf("iterate published sessions for rename: %w", err)
	}
	rows.Close()

	var targetID int64
	var canonicalName string
	err = tx.QueryRow(ctx, `
		SELECT id, name
		FROM training_exercises
		WHERE owner_id = $1 AND training_normalize_exercise_name(name) = training_normalize_exercise_name($2) AND id <> $3
		FOR UPDATE`, ownerID, name, exerciseID).Scan(&targetID, &canonicalName)
	merged := err == nil
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return training.RenameResult{}, fmt.Errorf("select rename target: %w", err)
	}
	if !merged {
		targetID = exerciseID
		canonicalName = name
		if _, err := tx.Exec(ctx, `UPDATE training_exercises SET name = $1, updated_at = now() WHERE id = $2`, canonicalName, exerciseID); err != nil {
			return training.RenameResult{}, fmt.Errorf("rename catalog exercise: %w", err)
		}
	}
	if _, err := tx.Exec(ctx, `
		UPDATE workout_template_exercises e
		SET exercise_id = $1, name = $2
		FROM workout_templates p
		WHERE p.id = e.workout_template_id AND p.owner_id = $3
		  AND (e.exercise_id = $4 OR (e.exercise_id IS NULL AND training_normalize_exercise_name(e.name) = training_normalize_exercise_name($5)))`,
		targetID, canonicalName, ownerID, exerciseID, currentName,
	); err != nil {
		return training.RenameResult{}, fmt.Errorf("rename exercise in programs: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		UPDATE training_session_exercises e
		SET exercise_id = $1, name = $2
		FROM training_sessions s
		WHERE s.id = e.session_id AND s.owner_id = $3
		  AND (e.exercise_id = $4 OR (e.exercise_id IS NULL AND training_normalize_exercise_name(e.name) = training_normalize_exercise_name($5)))`,
		targetID, canonicalName, ownerID, exerciseID, currentName,
	); err != nil {
		return training.RenameResult{}, fmt.Errorf("rename exercise in sessions: %w", err)
	}
	if merged {
		if _, err := tx.Exec(ctx, `DELETE FROM training_exercises WHERE id = $1`, exerciseID); err != nil {
			return training.RenameResult{}, fmt.Errorf("remove merged exercise: %w", err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return training.RenameResult{}, fmt.Errorf("commit exercise rename: %w", err)
	}

	result := training.RenameResult{Merged: merged}
	result.Exercise, err = r.Exercise(ctx, ownerID, targetID)
	if err != nil {
		return training.RenameResult{}, err
	}
	for _, id := range publishedIDs {
		session, loadErr := r.loadSession(ctx, r.pool, ownerID, id)
		if loadErr != nil {
			return training.RenameResult{}, loadErr
		}
		result.PublishedSessions = append(result.PublishedSessions, session)
	}
	return result, nil
}

func (r *TrainingRepo) ProgramExercise(
	ctx context.Context,
	ownerID, programExerciseID int64,
) (training.ProgramExerciseReplacement, error) {
	var programID int64
	var current training.ProgramExercise
	var exerciseID pgtype.Int8
	err := r.pool.QueryRow(ctx, `
		SELECT p.id, pe.id, pe.exercise_id, pe.position, pe.name
		FROM workout_template_exercises pe
		JOIN workout_templates p ON p.id = pe.workout_template_id
		WHERE pe.id = $1 AND p.owner_id = $2`, programExerciseID, ownerID,
	).Scan(&programID, &current.ID, &exerciseID, &current.Position, &current.Name)
	if errors.Is(err, pgx.ErrNoRows) {
		return training.ProgramExerciseReplacement{}, training.ErrNotFound
	}
	if err != nil {
		return training.ProgramExerciseReplacement{}, fmt.Errorf("select program exercise: %w", err)
	}
	if exerciseID.Valid {
		value := exerciseID.Int64
		current.ExerciseID = &value
	}
	program, err := r.Program(ctx, ownerID, programID)
	if err != nil {
		return training.ProgramExerciseReplacement{}, err
	}
	return training.ProgramExerciseReplacement{Program: program, Current: current}, nil
}

func (r *TrainingRepo) ReplaceProgramExercise(
	ctx context.Context,
	ownerID, programExerciseID int64,
	targetExerciseID *int64,
	targetName string,
	replaceHistory bool,
) (training.ProgramExerciseReplaceResult, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return training.ProgramExerciseReplaceResult{}, fmt.Errorf("begin program exercise replacement: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var programID int64
	var position int
	var currentName string
	var currentExerciseID pgtype.Int8
	err = tx.QueryRow(ctx, `
		SELECT p.id, pe.position, pe.name, pe.exercise_id
		FROM workout_template_exercises pe
		JOIN workout_templates p ON p.id = pe.workout_template_id
		WHERE pe.id = $1 AND p.owner_id = $2
		FOR UPDATE OF p, pe`, programExerciseID, ownerID,
	).Scan(&programID, &position, &currentName, &currentExerciseID)
	if errors.Is(err, pgx.ErrNoRows) {
		return training.ProgramExerciseReplaceResult{}, training.ErrNotFound
	}
	if err != nil {
		return training.ProgramExerciseReplaceResult{}, fmt.Errorf("lock program exercise: %w", err)
	}

	var targetID int64
	var canonicalName string
	if targetExerciseID == nil {
		targetID, canonicalName, err = ensureTrainingExercise(ctx, tx, ownerID, targetName)
		if err != nil {
			return training.ProgramExerciseReplaceResult{}, fmt.Errorf("ensure replacement exercise: %w", err)
		}
	} else {
		err = tx.QueryRow(ctx, `
			SELECT id, name FROM training_exercises WHERE id = $1 AND owner_id = $2`,
			*targetExerciseID, ownerID,
		).Scan(&targetID, &canonicalName)
		if errors.Is(err, pgx.ErrNoRows) {
			return training.ProgramExerciseReplaceResult{}, training.ErrNotFound
		}
		if err != nil {
			return training.ProgramExerciseReplaceResult{}, fmt.Errorf("select replacement exercise: %w", err)
		}
	}

	var publishedIDs []int64
	if replaceHistory {
		rows, queryErr := tx.Query(ctx, `
			SELECT DISTINCT s.id
			FROM training_sessions s
			JOIN training_session_exercises se ON se.session_id = s.id
			WHERE s.owner_id = $1
			  AND s.workout_template_id = $2
			  AND s.status = 'finished'
			  AND se.position = $3
			  AND (se.exercise_id = $4 OR (se.exercise_id IS NULL AND training_normalize_exercise_name(se.name) = training_normalize_exercise_name($5)))
			  AND s.published_chat_id IS NOT NULL
			  AND s.published_message_id IS NOT NULL`,
			ownerID, programID, position, currentExerciseID, currentName,
		)
		if queryErr != nil {
			return training.ProgramExerciseReplaceResult{}, fmt.Errorf("select published sessions for program replacement: %w", queryErr)
		}
		for rows.Next() {
			var sessionID int64
			if scanErr := rows.Scan(&sessionID); scanErr != nil {
				rows.Close()
				return training.ProgramExerciseReplaceResult{}, fmt.Errorf("scan published session for program replacement: %w", scanErr)
			}
			publishedIDs = append(publishedIDs, sessionID)
		}
		queryErr = rows.Err()
		rows.Close()
		if queryErr != nil {
			return training.ProgramExerciseReplaceResult{}, fmt.Errorf("iterate published sessions for program replacement: %w", queryErr)
		}

		if _, err := tx.Exec(ctx, `
			UPDATE training_session_exercises se
			SET exercise_id = $1, name = $2
			FROM training_sessions s
			WHERE s.id = se.session_id
			  AND s.owner_id = $3
			  AND s.workout_template_id = $4
			  AND s.status = 'finished'
			  AND se.position = $5
			  AND (se.exercise_id = $6 OR (se.exercise_id IS NULL AND training_normalize_exercise_name(se.name) = training_normalize_exercise_name($7)))`,
			targetID, canonicalName, ownerID, programID, position, currentExerciseID, currentName,
		); err != nil {
			return training.ProgramExerciseReplaceResult{}, fmt.Errorf("replace exercise in finished program sessions: %w", err)
		}
	}

	if _, err := tx.Exec(ctx, `
		UPDATE workout_template_exercises
		SET exercise_id = $1, name = $2
		WHERE id = $3`, targetID, canonicalName, programExerciseID,
	); err != nil {
		return training.ProgramExerciseReplaceResult{}, fmt.Errorf("replace exercise in program: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return training.ProgramExerciseReplaceResult{}, fmt.Errorf("commit program exercise replacement: %w", err)
	}

	program, err := r.Program(ctx, ownerID, programID)
	if err != nil {
		return training.ProgramExerciseReplaceResult{}, err
	}
	result := training.ProgramExerciseReplaceResult{Program: program}
	for _, sessionID := range publishedIDs {
		session, loadErr := r.loadSession(ctx, r.pool, ownerID, sessionID)
		if loadErr != nil {
			return training.ProgramExerciseReplaceResult{}, loadErr
		}
		result.PublishedSessions = append(result.PublishedSessions, session)
	}
	return result, nil
}

func (r *TrainingRepo) StartSession(ctx context.Context, ownerID, programID int64, now time.Time) (training.Session, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return training.Session{}, fmt.Errorf("begin training session: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var programName string
	var revisionID int64
	if err := tx.QueryRow(ctx, `
		SELECT template.name, template.revision_id
		FROM workout_templates template
		JOIN training_program_revisions revision ON revision.id = template.revision_id
		JOIN training_programs program ON program.id = revision.program_id
		WHERE template.id = $1 AND template.owner_id = $2
		  AND program.active_revision_id = template.revision_id`, programID, ownerID,
	).Scan(&programName, &revisionID); errors.Is(err, pgx.ErrNoRows) {
		return training.Session{}, training.ErrNotFound
	} else if err != nil {
		return training.Session{}, fmt.Errorf("select program for session: %w", err)
	}

	var sessionID int64
	const insertSession = `
		INSERT INTO training_sessions (owner_id, workout_template_id, revision_id, program_name, started_at)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id`
	if err := tx.QueryRow(ctx, insertSession, ownerID, programID, revisionID, programName, now).Scan(&sessionID); err != nil {
		if isUniqueViolation(err) {
			return training.Session{}, training.ErrActiveSession
		}
		return training.Session{}, fmt.Errorf("insert training session: %w", err)
	}
	rows, err := tx.Query(ctx, `
		SELECT exercise.id, exercise.exercise_id, exercise.position, exercise.name,
		       exercise.working_sets, exercise.min_reps, exercise.max_reps,
		       exercise.target_rir::double precision,
		       exercise.weight_step_kg::double precision,
		       exercise.starting_weight_kg::double precision,
		       exercise.rest_between_sets_seconds, exercise.rest_after_exercise_seconds,
		       exercise.progression_type,
		       COALESCE((
		           SELECT jsonb_agg(jsonb_build_object(
		               'weight', warmup.weight_kg::double precision,
		               'bar', warmup.weight_mode = 'bar',
		               'reps', warmup.reps
		           ) ORDER BY warmup.position)
		           FROM workout_template_warmup_sets warmup
		           WHERE warmup.template_exercise_id = exercise.id
		       ), '[]'::jsonb)
		FROM workout_template_exercises exercise
		WHERE exercise.workout_template_id = $1
		ORDER BY exercise.position`, programID)
	if err != nil {
		return training.Session{}, fmt.Errorf("select workout snapshot: %w", err)
	}
	type snapshotExercise struct {
		exerciseID *int64
		position   int
		name       string
		plan       training.ExercisePrescription
	}
	var snapshots []snapshotExercise
	for rows.Next() {
		var templateExerciseID int64
		var catalogID pgtype.Int8
		var position int
		var name string
		var workingSets, minReps, maxReps, restSeconds, afterSeconds pgtype.Int4
		var targetRIR, weightStep, startingWeight pgtype.Float8
		var progressionType pgtype.Text
		var warmupJSON []byte
		if err := rows.Scan(
			&templateExerciseID, &catalogID, &position, &name,
			&workingSets, &minReps, &maxReps, &targetRIR, &weightStep, &startingWeight,
			&restSeconds, &afterSeconds, &progressionType, &warmupJSON,
		); err != nil {
			rows.Close()
			return training.Session{}, fmt.Errorf("scan workout snapshot: %w", err)
		}
		snapshot := snapshotExercise{position: position, name: name}
		if catalogID.Valid {
			value := catalogID.Int64
			snapshot.exerciseID = &value
		}
		if workingSets.Valid {
			snapshot.plan = training.ExercisePrescription{
				WorkingSets: int(workingSets.Int32),
				Reps:        training.RepRange{Min: int(minReps.Int32), Max: int(maxReps.Int32)},
				TargetRIR:   targetRIR.Float64, WeightStepKG: weightStep.Float64,
				RestSeconds: int(restSeconds.Int32), AfterSeconds: int(afterSeconds.Int32),
				Progression: progressionType.String,
			}
			if startingWeight.Valid {
				value := startingWeight.Float64
				snapshot.plan.StartingWeight = &value
			}
			if err := json.Unmarshal(warmupJSON, &snapshot.plan.Warmup); err != nil {
				rows.Close()
				return training.Session{}, fmt.Errorf("decode workout warmup: %w", err)
			}
		}
		snapshots = append(snapshots, snapshot)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return training.Session{}, fmt.Errorf("iterate workout snapshot: %w", err)
	}
	rows.Close()
	if len(snapshots) == 0 {
		return training.Session{}, training.ErrNoPrograms
	}
	engine := progression.New()
	for _, snapshot := range snapshots {
		var recommendation training.Recommendation
		if snapshot.plan.Structured() {
			previous, err := previousProgressionSession(ctx, tx, ownerID, snapshot.exerciseID, snapshot.name, now)
			if err != nil {
				return training.Session{}, err
			}
			history := []progression.PreviousSession(nil)
			if previous != nil {
				history = append(history, *previous)
			}
			recommendation, err = engine.Recommend(ctx, progression.Input{
				Exercise: prescriptionConfig(snapshot.plan), History: history,
			})
			if err != nil {
				return training.Session{}, fmt.Errorf("recommend %q: %w", snapshot.name, err)
			}
		}
		warmupJSON, err := json.Marshal(snapshot.plan.Warmup)
		if err != nil {
			return training.Session{}, fmt.Errorf("encode warmup snapshot: %w", err)
		}
		recommendationJSON, err := json.Marshal(recommendation)
		if err != nil {
			return training.Session{}, fmt.Errorf("encode recommendation snapshot: %w", err)
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO training_session_exercises (
				session_id, position, name, exercise_id,
				working_sets, min_reps, max_reps, target_rir, weight_step_kg,
				rest_between_sets_seconds, rest_after_exercise_seconds, progression_type,
				warmup_plan, recommendation,
				planned_weight_kg, planned_min_reps, planned_max_reps,
				planned_working_sets, planned_target_rir, planned_rest_seconds
			) VALUES (
				$1, $2, $3, $4,
				NULLIF($5, 0), NULLIF($6, 0), NULLIF($7, 0), $8, $9,
				$10, $11, NULLIF($12, ''),
				$13::jsonb, $14::jsonb,
				$15, NULLIF($16, 0), NULLIF($17, 0),
				NULLIF($18, 0), $19, $20
			)`,
			sessionID, snapshot.position, snapshot.name, snapshot.exerciseID,
			snapshot.plan.WorkingSets, snapshot.plan.Reps.Min, snapshot.plan.Reps.Max,
			nullableStructuredFloat(snapshot.plan, snapshot.plan.TargetRIR), nullableStructuredFloat(snapshot.plan, snapshot.plan.WeightStepKG),
			nullableStructuredInt(snapshot.plan, snapshot.plan.RestSeconds), nullableStructuredInt(snapshot.plan, snapshot.plan.AfterSeconds), snapshot.plan.Progression,
			warmupJSON, recommendationJSON,
			recommendation.WeightKG, recommendation.MinReps, recommendation.MaxReps,
			recommendation.WorkingSets, nullableStructuredFloat(snapshot.plan, recommendation.TargetRIR), nullableStructuredInt(snapshot.plan, recommendation.RestSeconds),
		); err != nil {
			return training.Session{}, fmt.Errorf("insert exercise snapshot %q: %w", snapshot.name, err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return training.Session{}, fmt.Errorf("commit training session: %w", err)
	}
	return r.loadSession(ctx, r.pool, ownerID, sessionID)
}

func prescriptionConfig(plan training.ExercisePrescription) progression.ExerciseConfig {
	return progression.ExerciseConfig{
		WorkingSets: plan.WorkingSets, MinReps: plan.Reps.Min, MaxReps: plan.Reps.Max,
		TargetRIR: plan.TargetRIR, WeightStepKG: plan.WeightStepKG, StartingWeight: plan.StartingWeight,
		RestSeconds: plan.RestSeconds, AfterSeconds: plan.AfterSeconds, Progression: plan.Progression,
	}
}

func previousProgressionSession(
	ctx context.Context,
	tx pgx.Tx,
	ownerID int64,
	exerciseID *int64,
	exerciseName string,
	before time.Time,
) (*progression.PreviousSession, error) {
	rows, err := tx.Query(ctx, `
		WITH previous AS (
			SELECT exercise.id
			FROM training_sessions session
			JOIN training_session_exercises exercise ON exercise.session_id = session.id
			WHERE session.owner_id = $1
			  AND session.status = 'finished'
			  AND session.started_at < $2
			  AND (
			      ($3::bigint IS NOT NULL AND exercise.exercise_id = $3)
			      OR training_normalize_exercise_name(exercise.name) = training_normalize_exercise_name($4)
			  )
			  AND EXISTS (
			      SELECT 1
			      FROM training_sets candidate
			      WHERE candidate.session_exercise_id = exercise.id
			        AND candidate.type = 'working'
			        AND candidate.completed_at IS NOT NULL
			        AND candidate.actual_reps IS NOT NULL
			  )
			ORDER BY session.started_at DESC, session.id DESC
			LIMIT 1
		)
		SELECT set.type, set.actual_weight_kg::double precision,
		       set.actual_reps, set.actual_rir::double precision
		FROM previous
		JOIN training_sets set ON set.session_exercise_id = previous.id
		WHERE set.completed_at IS NOT NULL AND set.actual_reps IS NOT NULL
		ORDER BY set.position`, ownerID, before, exerciseID, exerciseName)
	if err != nil {
		return nil, fmt.Errorf("select progression history for %q: %w", exerciseName, err)
	}
	defer rows.Close()
	var previous *progression.PreviousSession
	for rows.Next() {
		var set progression.Set
		var setType string
		var weight, rir pgtype.Float8
		if err := rows.Scan(&setType, &weight, &set.Reps, &rir); err != nil {
			return nil, fmt.Errorf("scan progression history for %q: %w", exerciseName, err)
		}
		if previous == nil {
			previous = &progression.PreviousSession{}
		}
		set.Type = progression.SetType(setType)
		if weight.Valid {
			value := weight.Float64
			set.WeightKG = &value
		}
		if rir.Valid {
			value := rir.Float64
			set.RIR = &value
		}
		previous.Sets = append(previous.Sets, set)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate progression history for %q: %w", exerciseName, err)
	}
	return previous, nil
}

func (r *TrainingRepo) ActiveSession(ctx context.Context, ownerID int64) (training.Session, error) {
	var sessionID int64
	err := r.pool.QueryRow(ctx,
		`SELECT id FROM training_sessions WHERE owner_id = $1 AND status = 'active'`, ownerID,
	).Scan(&sessionID)
	if errors.Is(err, pgx.ErrNoRows) {
		return training.Session{}, training.ErrNoActiveSession
	}
	if err != nil {
		return training.Session{}, fmt.Errorf("select active training session: %w", err)
	}
	return r.loadSession(ctx, r.pool, ownerID, sessionID)
}

func (r *TrainingRepo) Session(ctx context.Context, ownerID, sessionID int64) (training.Session, error) {
	return r.loadSession(ctx, r.pool, ownerID, sessionID)
}

func (r *TrainingRepo) AddSet(ctx context.Context, ownerID int64, input training.SetInput) (training.Session, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return training.Session{}, fmt.Errorf("begin add set: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	sessionID, exerciseID, err := lockCurrentExercise(ctx, tx, ownerID)
	if err != nil {
		return training.Session{}, err
	}
	var position int
	if err := tx.QueryRow(ctx,
		`SELECT COALESCE(max(position), 0) + 1 FROM training_sets WHERE session_exercise_id = $1`, exerciseID,
	).Scan(&position); err != nil {
		return training.Session{}, fmt.Errorf("select next set position: %w", err)
	}
	setType := input.Type
	if setType == "" {
		setType = training.SetTypeWorking
	}
	completedAt := input.CompletedAt
	if completedAt.IsZero() {
		completedAt = time.Now()
	}
	var planWeight, targetRIR pgtype.Float8
	var minReps, maxReps, restSeconds pgtype.Int4
	if err := tx.QueryRow(ctx, `
		SELECT planned_weight_kg::double precision, planned_min_reps, planned_max_reps,
		       planned_target_rir::double precision,
		       planned_rest_seconds
		FROM training_session_exercises
		WHERE id = $1`, exerciseID,
	).Scan(&planWeight, &minReps, &maxReps, &targetRIR, &restSeconds); err != nil {
		return training.Session{}, fmt.Errorf("select set plan: %w", err)
	}
	var plannedWeight any
	var plannedMin, plannedMax any
	var plannedRIR any
	if setType == training.SetTypeWarmup {
		plannedWeight = input.WeightKG
		plannedMin, plannedMax = input.Reps, input.Reps
	} else {
		if planWeight.Valid {
			plannedWeight = planWeight.Float64
		}
		if minReps.Valid {
			plannedMin = int(minReps.Int32)
		}
		if maxReps.Valid {
			plannedMax = int(maxReps.Int32)
		}
		if targetRIR.Valid {
			plannedRIR = targetRIR.Float64
		}
	}
	var restUntil *time.Time
	if restSeconds.Valid && restSeconds.Int32 > 0 {
		value := completedAt.Add(time.Duration(restSeconds.Int32) * time.Second)
		restUntil = &value
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO training_sets (
			session_exercise_id, position, type,
			planned_weight_kg, planned_min_reps, planned_max_reps, planned_rir,
			actual_weight_kg, actual_reps, actual_rir,
			started_at, completed_at, rest_until
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $11, $12)`,
		exerciseID, position, string(setType),
		plannedWeight, plannedMin, plannedMax, plannedRIR,
		input.WeightKG, input.Reps, input.RIR,
		completedAt, restUntil,
	); err != nil {
		return training.Session{}, fmt.Errorf("insert training set: %w", err)
	}
	if _, err := tx.Exec(ctx, `UPDATE training_sessions SET rest_until = $1 WHERE id = $2`, restUntil, sessionID); err != nil {
		return training.Session{}, fmt.Errorf("update training rest: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return training.Session{}, fmt.Errorf("commit training set: %w", err)
	}
	return r.loadSession(ctx, r.pool, ownerID, sessionID)
}

func (r *TrainingRepo) OverrideCurrentExercise(
	ctx context.Context,
	ownerID int64,
	override training.ExerciseOverride,
) (training.Session, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return training.Session{}, fmt.Errorf("begin exercise override: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	sessionID, exerciseID, err := lockCurrentExercise(ctx, tx, ownerID)
	if err != nil {
		return training.Session{}, err
	}
	if _, err := tx.Exec(ctx, `
		UPDATE training_session_exercises
		SET planned_weight_kg = $1,
		    planned_min_reps = $2,
		    planned_max_reps = $3,
		    planned_working_sets = $4,
		    planned_target_rir = $5,
		    planned_rest_seconds = $6,
		    overridden = true
		WHERE id = $7`,
		override.WeightKG, override.Reps.Min, override.Reps.Max, override.WorkingSets,
		override.TargetRIR, override.RestSeconds, exerciseID,
	); err != nil {
		return training.Session{}, fmt.Errorf("override exercise recommendation: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return training.Session{}, fmt.Errorf("commit exercise override: %w", err)
	}
	return r.loadSession(ctx, r.pool, ownerID, sessionID)
}

func (r *TrainingRepo) SetCurrentExerciseNote(ctx context.Context, ownerID int64, note string) (training.Session, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return training.Session{}, fmt.Errorf("begin exercise note: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	sessionID, exerciseID, err := lockCurrentExercise(ctx, tx, ownerID)
	if err != nil {
		return training.Session{}, err
	}
	if _, err := tx.Exec(ctx, `UPDATE training_session_exercises SET note = $1 WHERE id = $2`, note, exerciseID); err != nil {
		return training.Session{}, fmt.Errorf("update exercise note: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return training.Session{}, fmt.Errorf("commit exercise note: %w", err)
	}
	return r.loadSession(ctx, r.pool, ownerID, sessionID)
}

func (r *TrainingRepo) FinishCurrentExercise(ctx context.Context, ownerID int64, now time.Time) (training.Session, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return training.Session{}, fmt.Errorf("begin finish exercise: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	sessionID, exerciseID, err := lockCurrentExercise(ctx, tx, ownerID)
	if err != nil {
		return training.Session{}, err
	}
	var afterSeconds pgtype.Int4
	if err := tx.QueryRow(ctx, `
		SELECT rest_after_exercise_seconds FROM training_session_exercises WHERE id = $1`, exerciseID,
	).Scan(&afterSeconds); err != nil {
		return training.Session{}, fmt.Errorf("select exercise rest: %w", err)
	}
	if _, err := tx.Exec(ctx, `UPDATE training_session_exercises SET complete = true WHERE id = $1`, exerciseID); err != nil {
		return training.Session{}, fmt.Errorf("complete exercise: %w", err)
	}
	var nextPosition int
	err = tx.QueryRow(ctx, `
		SELECT position
		FROM training_session_exercises
		WHERE session_id = $1 AND complete = false
		ORDER BY position
		LIMIT 1`, sessionID).Scan(&nextPosition)
	if errors.Is(err, pgx.ErrNoRows) {
		if _, err := tx.Exec(ctx, `
			UPDATE training_sessions
			SET status = 'finished', finished_at = COALESCE(finished_at, $1), rest_until = NULL
			WHERE id = $2`, now, sessionID,
		); err != nil {
			return training.Session{}, fmt.Errorf("finish training session: %w", err)
		}
	} else if err != nil {
		return training.Session{}, fmt.Errorf("select next exercise: %w", err)
	} else {
		var restUntil *time.Time
		if afterSeconds.Valid && afterSeconds.Int32 > 0 {
			value := now.Add(time.Duration(afterSeconds.Int32) * time.Second)
			restUntil = &value
		}
		if _, err := tx.Exec(ctx,
			`UPDATE training_sessions SET current_position = $1, rest_until = $2 WHERE id = $3`, nextPosition, restUntil, sessionID,
		); err != nil {
			return training.Session{}, fmt.Errorf("advance training session: %w", err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return training.Session{}, fmt.Errorf("commit finish exercise: %w", err)
	}
	return r.loadSession(ctx, r.pool, ownerID, sessionID)
}

func (r *TrainingRepo) ReopenExercise(
	ctx context.Context,
	ownerID, sessionID, exerciseID int64,
) (training.Session, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return training.Session{}, fmt.Errorf("begin reopen exercise: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var status string
	var currentPosition, position int
	var complete bool
	err = tx.QueryRow(ctx, `
		SELECT s.status, s.current_position, e.position, e.complete
		FROM training_sessions s
		JOIN training_session_exercises e ON e.session_id = s.id
		WHERE s.id = $1 AND s.owner_id = $2 AND e.id = $3
		FOR UPDATE OF s, e`, sessionID, ownerID, exerciseID,
	).Scan(&status, &currentPosition, &position, &complete)
	if errors.Is(err, pgx.ErrNoRows) {
		return training.Session{}, training.ErrNotFound
	}
	if err != nil {
		return training.Session{}, fmt.Errorf("select exercise to reopen: %w", err)
	}
	if status == "active" && !complete && position != currentPosition {
		return training.Session{}, training.ErrNotEditable
	}
	if status == "finished" {
		var activeID int64
		err = tx.QueryRow(ctx, `
			SELECT id
			FROM training_sessions
			WHERE owner_id = $1 AND status = 'active' AND id <> $2
			LIMIT 1`, ownerID, sessionID).Scan(&activeID)
		if err == nil {
			return training.Session{}, training.ErrActiveSession
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return training.Session{}, fmt.Errorf("select active session before reopen: %w", err)
		}
	}
	if status != "active" && status != "finished" {
		return training.Session{}, training.ErrNotEditable
	}

	if _, err := tx.Exec(ctx,
		`UPDATE training_session_exercises SET complete = false WHERE id = $1`, exerciseID,
	); err != nil {
		return training.Session{}, fmt.Errorf("reopen exercise: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		UPDATE training_sessions
		SET status = 'active', current_position = $1
		WHERE id = $2`, position, sessionID,
	); err != nil {
		if isUniqueViolation(err) {
			return training.Session{}, training.ErrActiveSession
		}
		return training.Session{}, fmt.Errorf("reactivate training session: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		if isUniqueViolation(err) {
			return training.Session{}, training.ErrActiveSession
		}
		return training.Session{}, fmt.Errorf("commit reopen exercise: %w", err)
	}
	return r.loadSession(ctx, r.pool, ownerID, sessionID)
}

func (r *TrainingRepo) PreviousExercise(
	ctx context.Context,
	ownerID, sessionID int64,
	exerciseName string,
) (*training.PreviousExercise, error) {
	rows, err := r.pool.Query(ctx, `
		WITH previous AS (
			SELECT s.started_at, s.program_name, e.id AS exercise_id
			FROM training_sessions s
			JOIN training_session_exercises e ON e.session_id = s.id
			WHERE s.owner_id = $1
			  AND s.id <> $2
			  AND s.status = 'finished'
			  AND s.started_at < (
				SELECT started_at FROM training_sessions WHERE id = $2 AND owner_id = $1
			  )
			  AND training_normalize_exercise_name(e.name) = training_normalize_exercise_name($3)
			  AND EXISTS (
				SELECT 1 FROM training_sets candidate WHERE candidate.session_exercise_id = e.id
			  )
			ORDER BY s.started_at DESC, s.id DESC
			LIMIT 1
		)
		SELECT previous.started_at, previous.program_name, previous.exercise_id,
		       sets.id, sets.position, sets.type,
		       sets.actual_reps, sets.actual_weight_kg::double precision,
		       sets.actual_rir::double precision, sets.completed_at
		FROM previous
		JOIN training_sets sets ON sets.session_exercise_id = previous.exercise_id
		ORDER BY sets.position`, ownerID, sessionID, exerciseName)
	if err != nil {
		return nil, fmt.Errorf("select previous exercise: %w", err)
	}
	defer rows.Close()

	var previous *training.PreviousExercise
	for rows.Next() {
		var startedAt time.Time
		var programName string
		var set training.WorkoutSet
		var weight, rir pgtype.Float8
		var completedAt pgtype.Timestamptz
		if err := rows.Scan(
			&startedAt, &programName, &set.SessionExerciseID, &set.ID, &set.Position, &set.Type,
			&set.Reps, &weight, &rir, &completedAt,
		); err != nil {
			return nil, fmt.Errorf("scan previous exercise: %w", err)
		}
		if previous == nil {
			previous = &training.PreviousExercise{StartedAt: startedAt, ProgramName: programName}
		}
		if weight.Valid {
			value := weight.Float64
			set.WeightKG = &value
			set.ActualWeightKG = &value
		}
		actualReps := set.Reps
		set.ActualReps = &actualReps
		if rir.Valid {
			value := rir.Float64
			set.RIR = &value
			set.ActualRIR = &value
		}
		if completedAt.Valid {
			value := completedAt.Time
			set.CompletedAt = &value
		}
		previous.Sets = append(previous.Sets, set)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate previous exercise: %w", err)
	}
	return previous, nil
}

func (r *TrainingRepo) RecentSessions(ctx context.Context, ownerID int64, limit, offset int) ([]training.Session, int, error) {
	if limit <= 0 || limit > 50 {
		limit = 5
	}
	if offset < 0 {
		offset = 0
	}
	var total int
	if err := r.pool.QueryRow(ctx,
		`SELECT count(*) FROM training_sessions WHERE owner_id = $1 AND status = 'finished'`, ownerID,
	).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count training history: %w", err)
	}
	rows, err := r.pool.Query(ctx,
		`SELECT id FROM training_sessions WHERE owner_id = $1 AND status = 'finished' ORDER BY started_at DESC, id DESC LIMIT $2 OFFSET $3`,
		ownerID, limit, offset,
	)
	if err != nil {
		return nil, 0, fmt.Errorf("select training history: %w", err)
	}
	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return nil, 0, fmt.Errorf("scan training history ID: %w", err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, 0, fmt.Errorf("iterate training history: %w", err)
	}
	rows.Close()

	sessions := make([]training.Session, 0, len(ids))
	for _, id := range ids {
		session, err := r.loadSession(ctx, r.pool, ownerID, id)
		if err != nil {
			return nil, 0, err
		}
		sessions = append(sessions, session)
	}
	return sessions, total, nil
}

func (r *TrainingRepo) MarkPublished(ctx context.Context, ownerID, sessionID, chatID int64, messageID int) error {
	result, err := r.pool.Exec(ctx, `
		UPDATE training_sessions
		SET published_chat_id = $1, published_message_id = $2
		WHERE id = $3 AND owner_id = $4 AND status = 'finished'`,
		chatID, messageID, sessionID, ownerID,
	)
	if err != nil {
		return fmt.Errorf("mark training published: %w", err)
	}
	if result.RowsAffected() == 0 {
		return training.ErrNotFound
	}
	return nil
}

func (r *TrainingRepo) DeleteSession(ctx context.Context, ownerID, sessionID int64) error {
	result, err := r.pool.Exec(ctx,
		`DELETE FROM training_sessions WHERE id = $1 AND owner_id = $2`, sessionID, ownerID,
	)
	if err != nil {
		return fmt.Errorf("delete training session: %w", err)
	}
	if result.RowsAffected() == 0 {
		return training.ErrNotFound
	}
	return nil
}

type trainingQuerier interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

func (r *TrainingRepo) loadSession(ctx context.Context, db trainingQuerier, ownerID, sessionID int64) (training.Session, error) {
	const sessionQuery = `
		SELECT id, owner_id, workout_template_id, revision_id, program_name, status, current_position,
		       started_at, finished_at, published_chat_id, published_message_id, rest_until
		FROM training_sessions
		WHERE id = $1 AND owner_id = $2`
	var session training.Session
	var programID, revisionID, publishedChatID pgtype.Int8
	var finishedAt, restUntil pgtype.Timestamptz
	var publishedMessageID pgtype.Int4
	err := db.QueryRow(ctx, sessionQuery, sessionID, ownerID).Scan(
		&session.ID, &session.OwnerID, &programID, &revisionID, &session.ProgramName, &session.Status,
		&session.CurrentPosition, &session.StartedAt, &finishedAt, &publishedChatID, &publishedMessageID, &restUntil,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return training.Session{}, training.ErrNotFound
	}
	if err != nil {
		return training.Session{}, fmt.Errorf("select training session: %w", err)
	}
	if programID.Valid {
		value := programID.Int64
		session.ProgramID = &value
	}
	if revisionID.Valid {
		value := revisionID.Int64
		session.RevisionID = &value
	}
	if finishedAt.Valid {
		value := finishedAt.Time
		session.FinishedAt = &value
	}
	if publishedChatID.Valid {
		value := publishedChatID.Int64
		session.PublishedChatID = &value
	}
	if publishedMessageID.Valid {
		value := int(publishedMessageID.Int32)
		session.PublishedMessageID = &value
	}
	if restUntil.Valid {
		value := restUntil.Time
		session.RestUntil = &value
	}

	const exercisesQuery = `
		SELECT e.id, e.exercise_id, e.position, e.name, e.note, e.complete,
		       e.warmup_plan, e.recommendation,
		       e.planned_weight_kg::double precision,
		       e.planned_min_reps, e.planned_max_reps, e.planned_working_sets,
		       e.planned_target_rir::double precision, e.planned_rest_seconds, e.overridden,
		       s.id, s.position, s.type,
		       s.planned_weight_kg::double precision,
		       s.planned_min_reps, s.planned_max_reps, s.planned_rir::double precision,
		       s.actual_weight_kg::double precision, s.actual_reps, s.actual_rir::double precision,
		       s.started_at, s.completed_at, s.rest_until
		FROM training_session_exercises e
		LEFT JOIN training_sets s ON s.session_exercise_id = e.id
		WHERE e.session_id = $1
		ORDER BY e.position, s.position`
	rows, err := db.Query(ctx, exercisesQuery, sessionID)
	if err != nil {
		return training.Session{}, fmt.Errorf("select session exercises: %w", err)
	}
	defer rows.Close()
	indexes := make(map[int64]int)
	for rows.Next() {
		var exercise training.SessionExercise
		var exerciseID pgtype.Int8
		var setID pgtype.Int8
		var warmupJSON, recommendationJSON []byte
		var planWeight, planRIR pgtype.Float8
		var planMin, planMax, planSets, planRest pgtype.Int4
		var setPosition pgtype.Int4
		var setType pgtype.Text
		var setPlanWeight, setPlanRIR, actualWeight, actualRIR pgtype.Float8
		var setPlanMin, setPlanMax, actualReps pgtype.Int4
		var startedAt, completedAt, setRestUntil pgtype.Timestamptz
		if err := rows.Scan(
			&exercise.ID, &exerciseID, &exercise.Position, &exercise.Name, &exercise.Note, &exercise.Complete,
			&warmupJSON, &recommendationJSON,
			&planWeight, &planMin, &planMax, &planSets, &planRIR, &planRest, &exercise.Overridden,
			&setID, &setPosition, &setType,
			&setPlanWeight, &setPlanMin, &setPlanMax, &setPlanRIR,
			&actualWeight, &actualReps, &actualRIR,
			&startedAt, &completedAt, &setRestUntil,
		); err != nil {
			return training.Session{}, fmt.Errorf("scan session exercise: %w", err)
		}
		index, ok := indexes[exercise.ID]
		if !ok {
			if exerciseID.Valid {
				value := exerciseID.Int64
				exercise.ExerciseID = &value
			}
			if err := json.Unmarshal(warmupJSON, &exercise.Warmup); err != nil {
				return training.Session{}, fmt.Errorf("decode session warmup: %w", err)
			}
			if len(recommendationJSON) > 0 && string(recommendationJSON) != "{}" {
				if err := json.Unmarshal(recommendationJSON, &exercise.Recommendation); err != nil {
					return training.Session{}, fmt.Errorf("decode session recommendation: %w", err)
				}
			}
			exercise.Plan = exercise.Recommendation
			if planWeight.Valid {
				value := planWeight.Float64
				exercise.Plan.WeightKG = &value
			} else {
				exercise.Plan.WeightKG = nil
			}
			if planMin.Valid {
				exercise.Plan.MinReps = int(planMin.Int32)
			}
			if planMax.Valid {
				exercise.Plan.MaxReps = int(planMax.Int32)
			}
			if planSets.Valid {
				exercise.Plan.WorkingSets = int(planSets.Int32)
			}
			if planRIR.Valid {
				exercise.Plan.TargetRIR = planRIR.Float64
			}
			if planRest.Valid {
				exercise.Plan.RestSeconds = int(planRest.Int32)
			}
			index = len(session.Exercises)
			indexes[exercise.ID] = index
			session.Exercises = append(session.Exercises, exercise)
		}
		if setID.Valid {
			set := training.WorkoutSet{
				ID: setID.Int64, SessionExerciseID: exercise.ID,
				Position: int(setPosition.Int32), Type: training.SetType(setType.String),
			}
			if setPlanWeight.Valid {
				value := setPlanWeight.Float64
				set.PlannedWeightKG = &value
			}
			if setPlanMin.Valid {
				value := int(setPlanMin.Int32)
				set.PlannedMinReps = &value
			}
			if setPlanMax.Valid {
				value := int(setPlanMax.Int32)
				set.PlannedMaxReps = &value
			}
			if setPlanRIR.Valid {
				value := setPlanRIR.Float64
				set.PlannedRIR = &value
			}
			if actualWeight.Valid {
				value := actualWeight.Float64
				set.ActualWeightKG = &value
				set.WeightKG = &value
			}
			if actualReps.Valid {
				value := int(actualReps.Int32)
				set.ActualReps = &value
				set.Reps = value
			}
			if actualRIR.Valid {
				value := actualRIR.Float64
				set.ActualRIR = &value
				set.RIR = &value
			}
			if startedAt.Valid {
				value := startedAt.Time
				set.StartedAt = &value
			}
			if completedAt.Valid {
				value := completedAt.Time
				set.CompletedAt = &value
			}
			if setRestUntil.Valid {
				value := setRestUntil.Time
				set.RestUntil = &value
			}
			session.Exercises[index].Sets = append(session.Exercises[index].Sets, set)
		}
	}
	if err := rows.Err(); err != nil {
		return training.Session{}, fmt.Errorf("iterate session exercises: %w", err)
	}
	return session, nil
}

func ensureTrainingExercise(ctx context.Context, tx pgx.Tx, ownerID int64, name string) (int64, string, error) {
	var id int64
	var canonicalName string
	err := tx.QueryRow(ctx, `
		INSERT INTO training_exercises (owner_id, name)
		VALUES ($1, btrim($2))
		ON CONFLICT (owner_id, (training_normalize_exercise_name(name))) DO UPDATE SET updated_at = training_exercises.updated_at
		RETURNING id, name`, ownerID, name,
	).Scan(&id, &canonicalName)
	return id, canonicalName, err
}

func lockCurrentExercise(ctx context.Context, tx pgx.Tx, ownerID int64) (int64, int64, error) {
	const q = `
		SELECT s.id, e.id
		FROM training_sessions s
		JOIN training_session_exercises e
		  ON e.session_id = s.id AND e.position = s.current_position
		WHERE s.owner_id = $1 AND s.status = 'active'
		FOR UPDATE OF s, e`
	var sessionID, exerciseID int64
	err := tx.QueryRow(ctx, q, ownerID).Scan(&sessionID, &exerciseID)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, 0, training.ErrNoActiveSession
	}
	if err != nil {
		return 0, 0, fmt.Errorf("lock current exercise: %w", err)
	}
	return sessionID, exerciseID, nil
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
