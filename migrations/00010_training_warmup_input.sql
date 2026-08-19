-- +goose Up
ALTER TABLE training_ui_states
    DROP CONSTRAINT training_ui_states_mode_check,
    ADD CONSTRAINT training_ui_states_mode_check
        CHECK (mode IN (
            '', 'set', 'warmup', 'note', 'import_file', 'import_preview', 'exercise_rename',
            'program_exercise_choice', 'program_exercise_new', 'program_exercise_confirm',
            'rir', 'override'
        ));

-- +goose Down
UPDATE training_ui_states SET mode = '' WHERE mode = 'warmup';
ALTER TABLE training_ui_states
    DROP CONSTRAINT training_ui_states_mode_check,
    ADD CONSTRAINT training_ui_states_mode_check
        CHECK (mode IN (
            '', 'set', 'note', 'import_file', 'import_preview', 'exercise_rename',
            'program_exercise_choice', 'program_exercise_new', 'program_exercise_confirm',
            'rir', 'override'
        ));
