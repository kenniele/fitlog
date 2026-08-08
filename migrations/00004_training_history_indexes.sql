-- +goose Up
CREATE INDEX training_session_exercises_normalized_name_idx
    ON training_session_exercises (lower(btrim(name)));
CREATE INDEX training_sets_exercise_position_idx
    ON training_sets (session_exercise_id, position);

-- +goose Down
DROP INDEX training_sets_exercise_position_idx;
DROP INDEX training_session_exercises_normalized_name_idx;
