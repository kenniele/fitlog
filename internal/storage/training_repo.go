package storage

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"fitlog/internal/training"
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
		       pending_program_exercise_id, pending_target_exercise_id
		FROM training_ui_states
		WHERE owner_id = $1`
	var state training.UIState
	var mode string
	var pending []byte
	var pendingExerciseID pgtype.Int8
	var pendingProgramExerciseID, pendingTargetExerciseID pgtype.Int8
	err := r.pool.QueryRow(ctx, q, ownerID).Scan(
		&state.OwnerID, &state.ChatID, &state.MessageID, &mode, &pending,
		&pendingExerciseID, &state.PendingExerciseName,
		&pendingProgramExerciseID, &pendingTargetExerciseID,
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
	const q = `
		INSERT INTO training_ui_states (
			owner_id, chat_id, message_id, mode, pending_import,
			pending_exercise_id, pending_exercise_name,
			pending_program_exercise_id, pending_target_exercise_id, updated_at
		)
		VALUES ($1, $2, $3, $4, $5::jsonb, $6, $7, $8, $9, now())
		ON CONFLICT (owner_id) DO UPDATE SET
			chat_id = EXCLUDED.chat_id,
			message_id = EXCLUDED.message_id,
			mode = EXCLUDED.mode,
			pending_import = EXCLUDED.pending_import,
			pending_exercise_id = EXCLUDED.pending_exercise_id,
			pending_exercise_name = EXCLUDED.pending_exercise_name,
			pending_program_exercise_id = EXCLUDED.pending_program_exercise_id,
			pending_target_exercise_id = EXCLUDED.pending_target_exercise_id,
			updated_at = now()`
	if _, err := r.pool.Exec(
		ctx, q,
		state.OwnerID, state.ChatID, state.MessageID, string(state.Mode), pending,
		state.PendingExerciseID, state.PendingExerciseName,
		state.PendingProgramExerciseID, state.PendingTargetExerciseID,
	); err != nil {
		return fmt.Errorf("upsert training UI state: %w", err)
	}
	return nil
}

func (r *TrainingRepo) ListPrograms(ctx context.Context, ownerID int64) ([]training.Program, error) {
	const q = `
		SELECT p.id, p.owner_id, p.name, e.id, e.exercise_id, e.position, e.name
		FROM training_programs p
		LEFT JOIN training_program_exercises e ON e.program_id = p.id
		WHERE p.owner_id = $1
		ORDER BY p.sort_order, p.id, e.position`
	rows, err := r.pool.Query(ctx, q, ownerID)
	if err != nil {
		return nil, fmt.Errorf("select training programs: %w", err)
	}
	defer rows.Close()

	programs := make([]training.Program, 0)
	indexes := make(map[int64]int)
	for rows.Next() {
		var id, rowOwnerID int64
		var name string
		var programExerciseID, exerciseID pgtype.Int8
		var position pgtype.Int4
		var exerciseName pgtype.Text
		if err := rows.Scan(
			&id, &rowOwnerID, &name, &programExerciseID, &exerciseID, &position, &exerciseName,
		); err != nil {
			return nil, fmt.Errorf("scan training program: %w", err)
		}
		index, ok := indexes[id]
		if !ok {
			index = len(programs)
			indexes[id] = index
			programs = append(programs, training.Program{ID: id, OwnerID: rowOwnerID, Name: name})
		}
		if exerciseName.Valid {
			item := training.ProgramExercise{
				ID: programExerciseID.Int64, Position: int(position.Int32), Name: exerciseName.String,
			}
			if exerciseID.Valid {
				value := exerciseID.Int64
				item.ExerciseID = &value
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

	for i, program := range programs {
		const upsertProgram = `
			INSERT INTO training_programs (owner_id, name, sort_order)
			VALUES ($1, $2, $3)
			ON CONFLICT (owner_id, (lower(name))) DO UPDATE SET
				name = EXCLUDED.name,
				sort_order = EXCLUDED.sort_order,
				updated_at = now()
			RETURNING id`
		var programID int64
		if err := tx.QueryRow(ctx, upsertProgram, ownerID, program.Name, i+1).Scan(&programID); err != nil {
			return fmt.Errorf("upsert training program %q: %w", program.Name, err)
		}
		if _, err := tx.Exec(ctx, `DELETE FROM training_program_exercises WHERE program_id = $1`, programID); err != nil {
			return fmt.Errorf("clear exercises for %q: %w", program.Name, err)
		}
		for j, exercise := range program.Exercises {
			catalogID, catalogName, err := ensureTrainingExercise(ctx, tx, ownerID, exercise)
			if err != nil {
				return fmt.Errorf("ensure exercise %q: %w", exercise, err)
			}
			const insertExercise = `
				INSERT INTO training_program_exercises (program_id, position, name, exercise_id)
				VALUES ($1, $2, $3, $4)`
			if _, err := tx.Exec(ctx, insertExercise, programID, j+1, catalogName, catalogID); err != nil {
				return fmt.Errorf("insert exercise %q: %w", exercise, err)
			}
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit program import: %w", err)
	}
	return nil
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
		LEFT JOIN training_program_exercises pe ON pe.exercise_id = e.id
		LEFT JOIN training_programs p ON p.id = pe.program_id
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
		LEFT JOIN training_program_exercises pe ON pe.exercise_id = e.id
		LEFT JOIN training_programs p ON p.id = pe.program_id
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
		LEFT JOIN training_program_exercises pe ON pe.exercise_id = e.id
		LEFT JOIN training_programs p ON p.id = pe.program_id
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
		UPDATE training_program_exercises e
		SET exercise_id = $1, name = $2
		FROM training_programs p
		WHERE p.id = e.program_id AND p.owner_id = $3
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
		FROM training_program_exercises pe
		JOIN training_programs p ON p.id = pe.program_id
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
		FROM training_program_exercises pe
		JOIN training_programs p ON p.id = pe.program_id
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
			  AND s.program_id = $2
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
			  AND s.program_id = $4
			  AND s.status = 'finished'
			  AND se.position = $5
			  AND (se.exercise_id = $6 OR (se.exercise_id IS NULL AND training_normalize_exercise_name(se.name) = training_normalize_exercise_name($7)))`,
			targetID, canonicalName, ownerID, programID, position, currentExerciseID, currentName,
		); err != nil {
			return training.ProgramExerciseReplaceResult{}, fmt.Errorf("replace exercise in finished program sessions: %w", err)
		}
	}

	if _, err := tx.Exec(ctx, `
		UPDATE training_program_exercises
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
	if err := tx.QueryRow(ctx,
		`SELECT name FROM training_programs WHERE id = $1 AND owner_id = $2`, programID, ownerID,
	).Scan(&programName); errors.Is(err, pgx.ErrNoRows) {
		return training.Session{}, training.ErrNotFound
	} else if err != nil {
		return training.Session{}, fmt.Errorf("select program for session: %w", err)
	}

	var sessionID int64
	const insertSession = `
		INSERT INTO training_sessions (owner_id, program_id, program_name, started_at)
		VALUES ($1, $2, $3, $4)
		RETURNING id`
	if err := tx.QueryRow(ctx, insertSession, ownerID, programID, programName, now).Scan(&sessionID); err != nil {
		if isUniqueViolation(err) {
			return training.Session{}, training.ErrActiveSession
		}
		return training.Session{}, fmt.Errorf("insert training session: %w", err)
	}
	const copyExercises = `
		INSERT INTO training_session_exercises (session_id, position, name, exercise_id)
		SELECT $1, position, name, exercise_id
		FROM training_program_exercises
		WHERE program_id = $2
		ORDER BY position`
	result, err := tx.Exec(ctx, copyExercises, sessionID, programID)
	if err != nil {
		return training.Session{}, fmt.Errorf("copy session exercises: %w", err)
	}
	if result.RowsAffected() == 0 {
		return training.Session{}, training.ErrNoPrograms
	}
	if err := tx.Commit(ctx); err != nil {
		return training.Session{}, fmt.Errorf("commit training session: %w", err)
	}
	return r.loadSession(ctx, r.pool, ownerID, sessionID)
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
	if _, err := tx.Exec(ctx,
		`INSERT INTO training_sets (session_exercise_id, position, reps, weight_kg) VALUES ($1, $2, $3, $4)`,
		exerciseID, position, input.Reps, input.WeightKG,
	); err != nil {
		return training.Session{}, fmt.Errorf("insert training set: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return training.Session{}, fmt.Errorf("commit training set: %w", err)
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
			SET status = 'finished', finished_at = COALESCE(finished_at, $1)
			WHERE id = $2`, now, sessionID,
		); err != nil {
			return training.Session{}, fmt.Errorf("finish training session: %w", err)
		}
	} else if err != nil {
		return training.Session{}, fmt.Errorf("select next exercise: %w", err)
	} else {
		if _, err := tx.Exec(ctx,
			`UPDATE training_sessions SET current_position = $1 WHERE id = $2`, nextPosition, sessionID,
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
		SELECT previous.started_at, previous.program_name,
		       sets.id, sets.position, sets.reps, sets.weight_kg::double precision
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
		var weight pgtype.Float8
		if err := rows.Scan(
			&startedAt, &programName, &set.ID, &set.Position, &set.Reps, &weight,
		); err != nil {
			return nil, fmt.Errorf("scan previous exercise: %w", err)
		}
		if previous == nil {
			previous = &training.PreviousExercise{StartedAt: startedAt, ProgramName: programName}
		}
		if weight.Valid {
			value := weight.Float64
			set.WeightKG = &value
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
		SELECT id, owner_id, program_id, program_name, status, current_position,
		       started_at, finished_at, published_chat_id, published_message_id
		FROM training_sessions
		WHERE id = $1 AND owner_id = $2`
	var session training.Session
	var programID, publishedChatID pgtype.Int8
	var finishedAt pgtype.Timestamptz
	var publishedMessageID pgtype.Int4
	err := db.QueryRow(ctx, sessionQuery, sessionID, ownerID).Scan(
		&session.ID, &session.OwnerID, &programID, &session.ProgramName, &session.Status,
		&session.CurrentPosition, &session.StartedAt, &finishedAt, &publishedChatID, &publishedMessageID,
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

	const exercisesQuery = `
		SELECT e.id, e.exercise_id, e.position, e.name, e.note, e.complete,
		       s.id, s.position, s.reps, s.weight_kg::double precision
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
		var setPosition, reps pgtype.Int4
		var weight pgtype.Float8
		if err := rows.Scan(
			&exercise.ID, &exerciseID, &exercise.Position, &exercise.Name, &exercise.Note, &exercise.Complete,
			&setID, &setPosition, &reps, &weight,
		); err != nil {
			return training.Session{}, fmt.Errorf("scan session exercise: %w", err)
		}
		index, ok := indexes[exercise.ID]
		if !ok {
			if exerciseID.Valid {
				value := exerciseID.Int64
				exercise.ExerciseID = &value
			}
			index = len(session.Exercises)
			indexes[exercise.ID] = index
			session.Exercises = append(session.Exercises, exercise)
		}
		if setID.Valid {
			set := training.WorkoutSet{ID: setID.Int64, Position: int(setPosition.Int32), Reps: int(reps.Int32)}
			if weight.Valid {
				value := weight.Float64
				set.WeightKG = &value
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
