-- +goose Up
ALTER TABLE training_ui_states
    DROP CONSTRAINT training_ui_states_mode_check,
    ADD COLUMN pending_exercise_name TEXT NOT NULL DEFAULT '',
    ADD CONSTRAINT training_ui_states_mode_check
        CHECK (mode IN ('', 'set', 'note', 'import_file', 'import_preview', 'exercise_rename', 'exercise_rename_confirm'));

-- +goose Down
ALTER TABLE training_ui_states
    DROP CONSTRAINT training_ui_states_mode_check,
    DROP COLUMN pending_exercise_name,
    ADD CONSTRAINT training_ui_states_mode_check
        CHECK (mode IN ('', 'set', 'note', 'import_file', 'import_preview', 'exercise_rename'));
