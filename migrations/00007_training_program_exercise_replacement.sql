-- +goose Up
UPDATE training_ui_states
SET mode = '',
    pending_exercise_id = NULL,
    pending_exercise_name = ''
WHERE mode = 'exercise_rename_confirm';

ALTER TABLE training_ui_states
    DROP CONSTRAINT training_ui_states_mode_check,
    ADD COLUMN pending_program_exercise_id BIGINT REFERENCES training_program_exercises(id) ON DELETE SET NULL,
    ADD COLUMN pending_target_exercise_id BIGINT REFERENCES training_exercises(id) ON DELETE SET NULL,
    ADD CONSTRAINT training_ui_states_mode_check
        CHECK (mode IN (
            '', 'set', 'note', 'import_file', 'import_preview', 'exercise_rename',
            'program_exercise_choice', 'program_exercise_new', 'program_exercise_confirm'
        ));

-- +goose Down
UPDATE training_ui_states
SET mode = '',
    pending_exercise_name = ''
WHERE mode IN ('program_exercise_choice', 'program_exercise_new', 'program_exercise_confirm');

ALTER TABLE training_ui_states
    DROP CONSTRAINT training_ui_states_mode_check,
    DROP COLUMN pending_program_exercise_id,
    DROP COLUMN pending_target_exercise_id,
    ADD CONSTRAINT training_ui_states_mode_check
        CHECK (mode IN (
            '', 'set', 'note', 'import_file', 'import_preview', 'exercise_rename', 'exercise_rename_confirm'
        ));
