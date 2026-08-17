-- +goose Up
-- The pre-v1 training_programs table represented one workout day. Preserve it
-- as workout_templates and introduce the long-lived program/revision layer.
ALTER TABLE training_programs RENAME TO workout_templates;
ALTER INDEX training_programs_pkey RENAME TO workout_templates_pkey;
ALTER INDEX training_programs_owner_name_idx RENAME TO workout_templates_owner_name_idx;

ALTER TABLE training_program_exercises RENAME TO workout_template_exercises;
ALTER TABLE workout_template_exercises RENAME COLUMN program_id TO workout_template_id;
ALTER INDEX training_program_exercises_pkey RENAME TO workout_template_exercises_pkey;
ALTER INDEX training_program_exercises_exercise_idx RENAME TO workout_template_exercises_exercise_idx;

CREATE TABLE training_programs (
    id                 BIGSERIAL PRIMARY KEY,
    owner_id           BIGINT      NOT NULL,
    name               TEXT        NOT NULL CHECK (btrim(name) <> ''),
    description        TEXT        NOT NULL DEFAULT '',
    days_per_week      INTEGER     CHECK (days_per_week BETWEEN 1 AND 7),
    active_revision_id BIGINT,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at         TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX training_programs_owner_name_idx
    ON training_programs (owner_id, training_normalize_exercise_name(name));

CREATE TABLE training_program_revisions (
    id         BIGSERIAL PRIMARY KEY,
    program_id BIGINT      NOT NULL REFERENCES training_programs(id) ON DELETE CASCADE,
    revision   INTEGER     NOT NULL CHECK (revision > 0),
    raw_source TEXT        NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (program_id, revision)
);
ALTER TABLE training_programs
    ADD CONSTRAINT training_programs_active_revision_fk
    FOREIGN KEY (active_revision_id) REFERENCES training_program_revisions(id) ON DELETE SET NULL;

ALTER TABLE workout_templates
    ADD COLUMN revision_id BIGINT REFERENCES training_program_revisions(id) ON DELETE CASCADE,
    ADD COLUMN external_id TEXT,
    ADD COLUMN position INTEGER NOT NULL DEFAULT 1 CHECK (position > 0);

INSERT INTO training_programs (owner_id, name)
SELECT owner_id, min(name)
FROM workout_templates
GROUP BY owner_id, training_normalize_exercise_name(name)
ORDER BY owner_id, training_normalize_exercise_name(name);

INSERT INTO training_program_revisions (program_id, revision, raw_source)
SELECT id, 1, ''
FROM training_programs;

UPDATE workout_templates template
SET revision_id = revision.id,
    external_id = 'legacy_' || template.id,
    position = GREATEST(template.sort_order, 1)
FROM training_programs program
JOIN training_program_revisions revision ON revision.program_id = program.id AND revision.revision = 1
WHERE program.owner_id = template.owner_id
  AND training_normalize_exercise_name(program.name) = training_normalize_exercise_name(template.name);

UPDATE training_programs program
SET active_revision_id = revision.id
FROM training_program_revisions revision
WHERE revision.program_id = program.id AND revision.revision = 1;

ALTER TABLE workout_templates
    ALTER COLUMN revision_id SET NOT NULL,
    ALTER COLUMN external_id SET NOT NULL;
DROP INDEX workout_templates_owner_name_idx;
CREATE UNIQUE INDEX workout_templates_revision_external_idx
    ON workout_templates (revision_id, external_id);
CREATE INDEX workout_templates_owner_active_idx
    ON workout_templates (owner_id, revision_id, position);

ALTER TABLE workout_template_exercises
    ADD COLUMN working_sets INTEGER CHECK (working_sets > 0),
    ADD COLUMN min_reps INTEGER CHECK (min_reps > 0),
    ADD COLUMN max_reps INTEGER CHECK (max_reps > 0),
    ADD COLUMN target_rir NUMERIC(4, 1) CHECK (target_rir >= 0),
    ADD COLUMN weight_step_kg NUMERIC(8, 2) CHECK (weight_step_kg > 0),
    ADD COLUMN starting_weight_kg NUMERIC(8, 2) CHECK (starting_weight_kg > 0),
    ADD COLUMN rest_between_sets_seconds INTEGER CHECK (rest_between_sets_seconds >= 0),
    ADD COLUMN rest_after_exercise_seconds INTEGER CHECK (rest_after_exercise_seconds >= 0),
    ADD COLUMN progression_type TEXT CHECK (progression_type IN ('double'));

CREATE TABLE workout_template_warmup_sets (
    id                   BIGSERIAL PRIMARY KEY,
    template_exercise_id BIGINT NOT NULL REFERENCES workout_template_exercises(id) ON DELETE CASCADE,
    position             INTEGER NOT NULL CHECK (position > 0),
    weight_kg            NUMERIC(8, 2) CHECK (weight_kg > 0),
    weight_mode          TEXT NOT NULL CHECK (weight_mode IN ('kg', 'bar')),
    reps                 INTEGER NOT NULL CHECK (reps > 0),
    UNIQUE (template_exercise_id, position),
    CHECK ((weight_mode = 'bar' AND weight_kg IS NULL) OR (weight_mode = 'kg' AND weight_kg IS NOT NULL))
);

ALTER TABLE training_sessions
    RENAME COLUMN program_id TO workout_template_id;
ALTER TABLE training_sessions
    ADD COLUMN revision_id BIGINT REFERENCES training_program_revisions(id) ON DELETE SET NULL,
    ADD COLUMN rest_until TIMESTAMPTZ;
UPDATE training_sessions session
SET revision_id = template.revision_id
FROM workout_templates template
WHERE template.id = session.workout_template_id;

ALTER TABLE training_session_exercises
    ADD COLUMN working_sets INTEGER CHECK (working_sets > 0),
    ADD COLUMN min_reps INTEGER CHECK (min_reps > 0),
    ADD COLUMN max_reps INTEGER CHECK (max_reps > 0),
    ADD COLUMN target_rir NUMERIC(4, 1) CHECK (target_rir >= 0),
    ADD COLUMN weight_step_kg NUMERIC(8, 2) CHECK (weight_step_kg > 0),
    ADD COLUMN rest_between_sets_seconds INTEGER CHECK (rest_between_sets_seconds >= 0),
    ADD COLUMN rest_after_exercise_seconds INTEGER CHECK (rest_after_exercise_seconds >= 0),
    ADD COLUMN progression_type TEXT CHECK (progression_type IN ('double')),
    ADD COLUMN warmup_plan JSONB NOT NULL DEFAULT '[]'::jsonb,
    ADD COLUMN recommendation JSONB NOT NULL DEFAULT '{}'::jsonb,
    ADD COLUMN planned_weight_kg NUMERIC(8, 2) CHECK (planned_weight_kg > 0),
    ADD COLUMN planned_min_reps INTEGER CHECK (planned_min_reps > 0),
    ADD COLUMN planned_max_reps INTEGER CHECK (planned_max_reps > 0),
    ADD COLUMN planned_working_sets INTEGER CHECK (planned_working_sets > 0),
    ADD COLUMN planned_target_rir NUMERIC(4, 1) CHECK (planned_target_rir >= 0),
    ADD COLUMN planned_rest_seconds INTEGER CHECK (planned_rest_seconds >= 0),
    ADD COLUMN overridden BOOLEAN NOT NULL DEFAULT false;

ALTER TABLE training_sets RENAME COLUMN reps TO actual_reps;
ALTER TABLE training_sets RENAME COLUMN weight_kg TO actual_weight_kg;
ALTER TABLE training_sets
    ALTER COLUMN actual_reps DROP NOT NULL,
    ADD COLUMN type TEXT NOT NULL DEFAULT 'working' CHECK (type IN ('warmup', 'working')),
    ADD COLUMN planned_weight_kg NUMERIC(8, 2) CHECK (planned_weight_kg > 0),
    ADD COLUMN planned_min_reps INTEGER CHECK (planned_min_reps > 0),
    ADD COLUMN planned_max_reps INTEGER CHECK (planned_max_reps > 0),
    ADD COLUMN planned_rir NUMERIC(4, 1) CHECK (planned_rir >= 0),
    ADD COLUMN actual_rir NUMERIC(4, 1) CHECK (actual_rir >= 0),
    ADD COLUMN started_at TIMESTAMPTZ,
    ADD COLUMN completed_at TIMESTAMPTZ,
    ADD COLUMN rest_until TIMESTAMPTZ;
UPDATE training_sets
SET completed_at = created_at,
    started_at = created_at;

ALTER TABLE training_ui_states
    DROP CONSTRAINT training_ui_states_mode_check,
    ADD COLUMN pending_set JSONB,
    ADD CONSTRAINT training_ui_states_mode_check
        CHECK (mode IN (
            '', 'set', 'note', 'import_file', 'import_preview', 'exercise_rename',
            'program_exercise_choice', 'program_exercise_new', 'program_exercise_confirm',
            'rir', 'override'
        ));

-- +goose Down
UPDATE training_ui_states SET mode = '', pending_set = NULL WHERE mode IN ('rir', 'override');
ALTER TABLE training_ui_states
    DROP CONSTRAINT training_ui_states_mode_check,
    DROP COLUMN pending_set,
    ADD CONSTRAINT training_ui_states_mode_check
        CHECK (mode IN (
            '', 'set', 'note', 'import_file', 'import_preview', 'exercise_rename',
            'program_exercise_choice', 'program_exercise_new', 'program_exercise_confirm'
        ));

DELETE FROM training_sets WHERE actual_reps IS NULL;
ALTER TABLE training_sets
    DROP COLUMN type,
    DROP COLUMN planned_weight_kg,
    DROP COLUMN planned_min_reps,
    DROP COLUMN planned_max_reps,
    DROP COLUMN planned_rir,
    DROP COLUMN actual_rir,
    DROP COLUMN started_at,
    DROP COLUMN completed_at,
    DROP COLUMN rest_until,
    ALTER COLUMN actual_reps SET NOT NULL;
ALTER TABLE training_sets RENAME COLUMN actual_reps TO reps;
ALTER TABLE training_sets RENAME COLUMN actual_weight_kg TO weight_kg;

ALTER TABLE training_session_exercises
    DROP COLUMN working_sets,
    DROP COLUMN min_reps,
    DROP COLUMN max_reps,
    DROP COLUMN target_rir,
    DROP COLUMN weight_step_kg,
    DROP COLUMN rest_between_sets_seconds,
    DROP COLUMN rest_after_exercise_seconds,
    DROP COLUMN progression_type,
    DROP COLUMN warmup_plan,
    DROP COLUMN recommendation,
    DROP COLUMN planned_weight_kg,
    DROP COLUMN planned_min_reps,
    DROP COLUMN planned_max_reps,
    DROP COLUMN planned_working_sets,
    DROP COLUMN planned_target_rir,
    DROP COLUMN planned_rest_seconds,
    DROP COLUMN overridden;

ALTER TABLE training_sessions
    DROP COLUMN revision_id,
    DROP COLUMN rest_until;
ALTER TABLE training_sessions RENAME COLUMN workout_template_id TO program_id;

DROP TABLE workout_template_warmup_sets;
ALTER TABLE workout_template_exercises
    DROP COLUMN working_sets,
    DROP COLUMN min_reps,
    DROP COLUMN max_reps,
    DROP COLUMN target_rir,
    DROP COLUMN weight_step_kg,
    DROP COLUMN starting_weight_kg,
    DROP COLUMN rest_between_sets_seconds,
    DROP COLUMN rest_after_exercise_seconds,
    DROP COLUMN progression_type;

-- The legacy schema can retain only the active revision of each program.
DELETE FROM workout_templates template
USING training_programs program
WHERE template.revision_id <> program.active_revision_id
  AND template.revision_id IN (
      SELECT id FROM training_program_revisions WHERE program_id = program.id
  );
-- Multiple v1 programs may legitimately use the same workout display name,
-- while the legacy schema allowed only one such name per owner.
DELETE FROM workout_templates duplicate
USING workout_templates keeper
WHERE duplicate.owner_id = keeper.owner_id
  AND lower(duplicate.name) = lower(keeper.name)
  AND duplicate.id > keeper.id;

DROP INDEX workout_templates_owner_active_idx;
DROP INDEX workout_templates_revision_external_idx;
ALTER TABLE workout_templates
    DROP COLUMN revision_id,
    DROP COLUMN external_id,
    DROP COLUMN position;
CREATE UNIQUE INDEX workout_templates_owner_name_idx
    ON workout_templates (owner_id, lower(name));

ALTER TABLE training_programs DROP CONSTRAINT training_programs_active_revision_fk;
DROP TABLE training_program_revisions;
DROP TABLE training_programs;

ALTER TABLE workout_template_exercises RENAME COLUMN workout_template_id TO program_id;
ALTER TABLE workout_template_exercises RENAME TO training_program_exercises;
ALTER INDEX workout_template_exercises_pkey RENAME TO training_program_exercises_pkey;
ALTER INDEX workout_template_exercises_exercise_idx RENAME TO training_program_exercises_exercise_idx;

ALTER TABLE workout_templates RENAME TO training_programs;
ALTER INDEX workout_templates_pkey RENAME TO training_programs_pkey;
ALTER INDEX workout_templates_owner_name_idx RENAME TO training_programs_owner_name_idx;
