-- +goose Up
CREATE FUNCTION training_normalize_exercise_name(value TEXT)
RETURNS TEXT
LANGUAGE SQL
IMMUTABLE
STRICT
PARALLEL SAFE
RETURN lower(translate(
    btrim(value),
    'АБВГДЕЁЖЗИЙКЛМНОПРСТУФХЦЧШЩЪЫЬЭЮЯ',
    'абвгдеёжзийклмнопрстуфхцчшщъыьэюя'
));

CREATE TABLE training_exercises (
    id         BIGSERIAL PRIMARY KEY,
    owner_id   BIGINT      NOT NULL,
    name       TEXT        NOT NULL CHECK (btrim(name) <> ''),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX training_exercises_owner_name_idx
    ON training_exercises (owner_id, training_normalize_exercise_name(name));

INSERT INTO training_exercises (owner_id, name)
SELECT owner_id, min(name)
FROM (
    SELECT p.owner_id, btrim(e.name) AS name
    FROM training_program_exercises e
    JOIN training_programs p ON p.id = e.program_id
    WHERE btrim(e.name) <> ''
    UNION ALL
    SELECT s.owner_id, btrim(e.name) AS name
    FROM training_session_exercises e
    JOIN training_sessions s ON s.id = e.session_id
    WHERE btrim(e.name) <> ''
) names
GROUP BY owner_id, training_normalize_exercise_name(name);

ALTER TABLE training_program_exercises
    ADD COLUMN exercise_id BIGINT REFERENCES training_exercises(id);
UPDATE training_program_exercises e
SET exercise_id = catalog.id,
    name = catalog.name
FROM training_programs p, training_exercises catalog
WHERE p.id = e.program_id
  AND catalog.owner_id = p.owner_id
  AND training_normalize_exercise_name(catalog.name) = training_normalize_exercise_name(e.name);
CREATE INDEX training_program_exercises_exercise_idx
    ON training_program_exercises (exercise_id);

ALTER TABLE training_session_exercises
    ADD COLUMN exercise_id BIGINT REFERENCES training_exercises(id);
UPDATE training_session_exercises e
SET exercise_id = catalog.id,
    name = catalog.name
FROM training_sessions s, training_exercises catalog
WHERE s.id = e.session_id
  AND catalog.owner_id = s.owner_id
  AND training_normalize_exercise_name(catalog.name) = training_normalize_exercise_name(e.name);
CREATE INDEX training_session_exercises_exercise_idx
    ON training_session_exercises (exercise_id);

ALTER TABLE training_ui_states
    DROP CONSTRAINT training_ui_states_mode_check,
    ADD COLUMN pending_exercise_id BIGINT REFERENCES training_exercises(id) ON DELETE SET NULL,
    ADD CONSTRAINT training_ui_states_mode_check
        CHECK (mode IN ('', 'set', 'note', 'import_file', 'import_preview', 'exercise_rename'));

-- +goose Down
ALTER TABLE training_ui_states
    DROP CONSTRAINT training_ui_states_mode_check,
    DROP COLUMN pending_exercise_id,
    ADD CONSTRAINT training_ui_states_mode_check
        CHECK (mode IN ('', 'set', 'note', 'import_file', 'import_preview'));
DROP INDEX training_session_exercises_exercise_idx;
ALTER TABLE training_session_exercises DROP COLUMN exercise_id;
DROP INDEX training_program_exercises_exercise_idx;
ALTER TABLE training_program_exercises DROP COLUMN exercise_id;
DROP TABLE training_exercises;
DROP FUNCTION training_normalize_exercise_name(TEXT);
