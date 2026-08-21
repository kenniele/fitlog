-- +goose Up
-- Control Center extends the existing training schema in place. The Telegram
-- bot keeps using the same rows and defaults, while the web API gains stable
-- source identifiers and the missing analytical domains.

ALTER TABLE training_programs
    ADD COLUMN source TEXT NOT NULL DEFAULT 'manual' CHECK (btrim(source) <> ''),
    ADD COLUMN external_id TEXT;
CREATE UNIQUE INDEX training_programs_owner_source_external_uidx
    ON training_programs (owner_id, source, external_id)
    WHERE external_id IS NOT NULL;

ALTER TABLE workout_templates
    ADD COLUMN description TEXT NOT NULL DEFAULT '',
    ADD COLUMN source TEXT NOT NULL DEFAULT 'manual' CHECK (btrim(source) <> '');
CREATE INDEX workout_templates_owner_source_external_idx
    ON workout_templates (owner_id, source, external_id);

ALTER TABLE workout_template_exercises
    ADD COLUMN notes TEXT NOT NULL DEFAULT '';

ALTER TABLE training_exercises
    ADD COLUMN slug TEXT,
    ADD COLUMN primary_muscle_group TEXT,
    ADD COLUMN secondary_muscle_groups TEXT[] NOT NULL DEFAULT '{}',
    ADD COLUMN exercise_type TEXT,
    ADD COLUMN equipment TEXT,
    ADD COLUMN unilateral BOOLEAN NOT NULL DEFAULT false,
    ADD COLUMN notes TEXT NOT NULL DEFAULT '',
    ADD COLUMN source TEXT NOT NULL DEFAULT 'manual' CHECK (btrim(source) <> ''),
    ADD COLUMN external_id TEXT;
CREATE UNIQUE INDEX training_exercises_owner_source_external_uidx
    ON training_exercises (owner_id, source, external_id)
    WHERE external_id IS NOT NULL;

ALTER TABLE training_sessions
    DROP CONSTRAINT training_sessions_status_check,
    ADD CONSTRAINT training_sessions_status_check
        CHECK (status IN ('scheduled', 'active', 'finished', 'cancelled', 'excused')),
    ALTER COLUMN started_at DROP NOT NULL,
    ADD COLUMN scheduled_at TIMESTAMPTZ,
    ADD COLUMN strain NUMERIC(5, 2) CHECK (strain >= 0 AND strain <= 21),
    ADD COLUMN notes TEXT NOT NULL DEFAULT '',
    ADD COLUMN source TEXT NOT NULL DEFAULT 'manual' CHECK (btrim(source) <> ''),
    ADD COLUMN external_id TEXT,
    ADD COLUMN created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    ADD COLUMN updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    ADD CONSTRAINT training_sessions_lifecycle_check CHECK (
        (status IN ('active', 'finished') AND started_at IS NOT NULL)
        OR (status IN ('scheduled', 'cancelled', 'excused') AND scheduled_at IS NOT NULL)
    );
CREATE UNIQUE INDEX training_sessions_owner_source_external_uidx
    ON training_sessions (owner_id, source, external_id)
    WHERE external_id IS NOT NULL;
CREATE INDEX training_sessions_owner_status_date_idx
    ON training_sessions (owner_id, status, started_at DESC);
CREATE INDEX training_sessions_owner_scheduled_idx
    ON training_sessions (owner_id, scheduled_at)
    WHERE scheduled_at IS NOT NULL;

ALTER TABLE training_session_exercises
    ADD COLUMN source TEXT NOT NULL DEFAULT 'manual' CHECK (btrim(source) <> ''),
    ADD COLUMN external_id TEXT,
    ADD COLUMN updated_at TIMESTAMPTZ NOT NULL DEFAULT now();
CREATE UNIQUE INDEX training_session_exercises_source_external_uidx
    ON training_session_exercises (session_id, source, external_id)
    WHERE external_id IS NOT NULL;

ALTER TABLE training_sets
    DROP CONSTRAINT training_sets_type_check,
    ADD CONSTRAINT training_sets_type_check CHECK (type IN ('warmup', 'working', 'drop')),
    ADD COLUMN rest_seconds INTEGER CHECK (rest_seconds >= 0),
    ADD COLUMN notes TEXT NOT NULL DEFAULT '',
    ADD COLUMN source TEXT NOT NULL DEFAULT 'manual' CHECK (btrim(source) <> ''),
    ADD COLUMN external_id TEXT,
    ADD COLUMN updated_at TIMESTAMPTZ NOT NULL DEFAULT now();
CREATE UNIQUE INDEX training_sets_source_external_uidx
    ON training_sets (session_exercise_id, source, external_id)
    WHERE external_id IS NOT NULL;
CREATE INDEX training_sets_completed_at_idx
    ON training_sets (completed_at)
    WHERE completed_at IS NOT NULL;

CREATE TABLE recovery_entries (
    id                     BIGSERIAL PRIMARY KEY,
    owner_id               BIGINT NOT NULL,
    entry_date             DATE NOT NULL,
    recovery_score         NUMERIC(5, 2) CHECK (recovery_score >= 0 AND recovery_score <= 100),
    hrv_ms                 NUMERIC(10, 3) CHECK (hrv_ms >= 0),
    resting_heart_rate_bpm NUMERIC(7, 2) CHECK (resting_heart_rate_bpm > 0),
    respiratory_rate      NUMERIC(7, 3) CHECK (respiratory_rate > 0),
    spo2_percent           NUMERIC(5, 2) CHECK (spo2_percent >= 0 AND spo2_percent <= 100),
    skin_temperature_c    NUMERIC(6, 3),
    daily_strain          NUMERIC(5, 2) CHECK (daily_strain >= 0 AND daily_strain <= 21),
    source                 TEXT NOT NULL DEFAULT 'manual' CHECK (btrim(source) <> ''),
    external_id            TEXT,
    notes                  TEXT NOT NULL DEFAULT '',
    created_at             TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at             TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX recovery_entries_owner_date_idx ON recovery_entries (owner_id, entry_date DESC);
CREATE INDEX recovery_entries_external_idx ON recovery_entries (source, external_id)
    WHERE external_id IS NOT NULL;
CREATE UNIQUE INDEX recovery_entries_owner_source_external_uidx
    ON recovery_entries (owner_id, source, external_id)
    WHERE external_id IS NOT NULL;
CREATE UNIQUE INDEX recovery_entries_owner_date_manual_uidx
    ON recovery_entries (owner_id, entry_date, source)
    WHERE external_id IS NULL;

CREATE TABLE sleep_entries (
    id                        BIGSERIAL PRIMARY KEY,
    owner_id                  BIGINT NOT NULL,
    sleep_date                DATE NOT NULL,
    sleep_start               TIMESTAMPTZ,
    sleep_end                 TIMESTAMPTZ,
    is_nap                    BOOLEAN NOT NULL DEFAULT false,
    time_in_bed_seconds       BIGINT CHECK (time_in_bed_seconds >= 0),
    actual_sleep_seconds      BIGINT CHECK (actual_sleep_seconds >= 0),
    awake_seconds             BIGINT CHECK (awake_seconds >= 0),
    rem_seconds               BIGINT CHECK (rem_seconds >= 0),
    deep_seconds              BIGINT CHECK (deep_seconds >= 0),
    light_seconds             BIGINT CHECK (light_seconds >= 0),
    sleep_performance_percent NUMERIC(5, 2) CHECK (sleep_performance_percent >= 0 AND sleep_performance_percent <= 100),
    efficiency_percent        NUMERIC(5, 2) CHECK (efficiency_percent >= 0 AND efficiency_percent <= 100),
    consistency_percent       NUMERIC(5, 2) CHECK (consistency_percent >= 0 AND consistency_percent <= 100),
    sleep_debt_seconds        BIGINT CHECK (sleep_debt_seconds >= 0),
    disturbances              INTEGER CHECK (disturbances >= 0),
    source                    TEXT NOT NULL DEFAULT 'manual' CHECK (btrim(source) <> ''),
    external_id               TEXT,
    notes                     TEXT NOT NULL DEFAULT '',
    created_at                TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at                TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (sleep_end IS NULL OR sleep_start IS NULL OR sleep_end >= sleep_start)
);
CREATE INDEX sleep_entries_owner_date_idx ON sleep_entries (owner_id, sleep_date DESC);
CREATE INDEX sleep_entries_external_idx ON sleep_entries (source, external_id)
    WHERE external_id IS NOT NULL;
CREATE UNIQUE INDEX sleep_entries_owner_source_external_uidx
    ON sleep_entries (owner_id, source, external_id)
    WHERE external_id IS NOT NULL;
CREATE UNIQUE INDEX sleep_entries_owner_date_manual_uidx
    ON sleep_entries (owner_id, sleep_date, source, is_nap)
    WHERE external_id IS NULL;

CREATE TABLE nutrition_days (
    id                BIGSERIAL PRIMARY KEY,
    owner_id          BIGINT NOT NULL,
    entry_date        DATE NOT NULL,
    calories_kcal     NUMERIC(10, 3) CHECK (calories_kcal >= 0),
    protein_g         NUMERIC(10, 3) CHECK (protein_g >= 0),
    fat_g             NUMERIC(10, 3) CHECK (fat_g >= 0),
    carbohydrates_g   NUMERIC(10, 3) CHECK (carbohydrates_g >= 0),
    fiber_g           NUMERIC(10, 3) CHECK (fiber_g >= 0),
    sugar_g           NUMERIC(10, 3) CHECK (sugar_g >= 0),
    saturated_fat_g   NUMERIC(10, 3) CHECK (saturated_fat_g >= 0),
    sodium_mg         NUMERIC(12, 3) CHECK (sodium_mg >= 0),
    potassium_mg      NUMERIC(12, 3) CHECK (potassium_mg >= 0),
    water_ml          NUMERIC(12, 3) CHECK (water_ml >= 0),
    source            TEXT NOT NULL DEFAULT 'manual' CHECK (btrim(source) <> ''),
    external_id       TEXT,
    notes             TEXT NOT NULL DEFAULT '',
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX nutrition_days_owner_date_idx ON nutrition_days (owner_id, entry_date DESC);
CREATE INDEX nutrition_days_external_idx ON nutrition_days (source, external_id)
    WHERE external_id IS NOT NULL;
CREATE UNIQUE INDEX nutrition_days_owner_source_external_uidx
    ON nutrition_days (owner_id, source, external_id)
    WHERE external_id IS NOT NULL;
CREATE UNIQUE INDEX nutrition_days_owner_date_manual_uidx
    ON nutrition_days (owner_id, entry_date, source)
    WHERE external_id IS NULL;

CREATE TABLE body_measurements (
    id                      BIGSERIAL PRIMARY KEY,
    owner_id                BIGINT NOT NULL,
    measured_at             TIMESTAMPTZ NOT NULL,
    weight_kg               NUMERIC(8, 3) CHECK (weight_kg > 0),
    body_fat_percent        NUMERIC(5, 2) CHECK (body_fat_percent >= 0 AND body_fat_percent <= 100),
    fat_mass_kg             NUMERIC(8, 3) CHECK (fat_mass_kg >= 0),
    lean_mass_kg            NUMERIC(8, 3) CHECK (lean_mass_kg >= 0),
    skeletal_muscle_mass_kg NUMERIC(8, 3) CHECK (skeletal_muscle_mass_kg >= 0),
    waist_cm                NUMERIC(8, 2) CHECK (waist_cm > 0),
    chest_cm                NUMERIC(8, 2) CHECK (chest_cm > 0),
    biceps_cm               NUMERIC(8, 2) CHECK (biceps_cm > 0),
    thigh_cm                NUMERIC(8, 2) CHECK (thigh_cm > 0),
    source                  TEXT NOT NULL DEFAULT 'manual' CHECK (btrim(source) <> ''),
    external_id             TEXT,
    notes                   TEXT NOT NULL DEFAULT '',
    created_at              TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at              TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (weight_kg IS NOT NULL OR body_fat_percent IS NOT NULL OR fat_mass_kg IS NOT NULL
        OR lean_mass_kg IS NOT NULL OR skeletal_muscle_mass_kg IS NOT NULL OR waist_cm IS NOT NULL
        OR chest_cm IS NOT NULL OR biceps_cm IS NOT NULL OR thigh_cm IS NOT NULL)
);
CREATE INDEX body_measurements_owner_date_idx ON body_measurements (owner_id, measured_at DESC);
CREATE INDEX body_measurements_external_idx ON body_measurements (source, external_id)
    WHERE external_id IS NOT NULL;
CREATE UNIQUE INDEX body_measurements_owner_source_external_uidx
    ON body_measurements (owner_id, source, external_id)
    WHERE external_id IS NOT NULL;

CREATE TABLE dashboard_settings (
    owner_id                  BIGINT PRIMARY KEY,
    timezone                  TEXT NOT NULL DEFAULT 'UTC' CHECK (btrim(timezone) <> ''),
    units                     TEXT NOT NULL DEFAULT 'metric' CHECK (units = 'metric'),
    theme                     TEXT NOT NULL DEFAULT 'dark' CHECK (theme IN ('dark', 'light', 'system')),
    first_day_of_week         SMALLINT NOT NULL DEFAULT 1 CHECK (first_day_of_week BETWEEN 1 AND 7),
    calorie_target_kcal       NUMERIC(10, 2) CHECK (calorie_target_kcal > 0),
    protein_target_g          NUMERIC(10, 2) CHECK (protein_target_g > 0),
    fat_target_g              NUMERIC(10, 2) CHECK (fat_target_g > 0),
    carbohydrates_target_g    NUMERIC(10, 2) CHECK (carbohydrates_target_g > 0),
    sleep_target_min_seconds  INTEGER CHECK (sleep_target_min_seconds > 0),
    sleep_target_max_seconds  INTEGER CHECK (sleep_target_max_seconds > 0),
    recovery_ranges           JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at                TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at                TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (sleep_target_min_seconds IS NULL OR sleep_target_max_seconds IS NULL
        OR sleep_target_max_seconds >= sleep_target_min_seconds)
);

CREATE TABLE data_imports (
    id             BIGSERIAL PRIMARY KEY,
    owner_id       BIGINT NOT NULL,
    source         TEXT NOT NULL CHECK (btrim(source) <> ''),
    data_type      TEXT NOT NULL CHECK (data_type IN ('recovery', 'sleep', 'nutrition', 'body', 'workouts', 'sets')),
    filename       TEXT NOT NULL DEFAULT '',
    format         TEXT NOT NULL CHECK (format IN ('csv', 'json', 'demo')),
    status         TEXT NOT NULL CHECK (status IN ('running', 'completed', 'failed')),
    total_rows     INTEGER NOT NULL DEFAULT 0 CHECK (total_rows >= 0),
    imported_rows  INTEGER NOT NULL DEFAULT 0 CHECK (imported_rows >= 0),
    skipped_rows   INTEGER NOT NULL DEFAULT 0 CHECK (skipped_rows >= 0),
    failed_rows    INTEGER NOT NULL DEFAULT 0 CHECK (failed_rows >= 0),
    error_summary  JSONB NOT NULL DEFAULT '[]'::jsonb,
    started_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    completed_at   TIMESTAMPTZ
);
CREATE INDEX data_imports_owner_started_idx ON data_imports (owner_id, started_at DESC);

-- +goose Down
-- Refuse a lossy rollback if the web UI created set/session states the legacy
-- checks cannot represent. Existing active/finished Telegram history remains
-- untouched by a normal rollback.
-- +goose StatementBegin
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM recovery_entries)
        OR EXISTS (SELECT 1 FROM sleep_entries)
        OR EXISTS (SELECT 1 FROM nutrition_days)
        OR EXISTS (SELECT 1 FROM body_measurements)
        OR EXISTS (SELECT 1 FROM data_imports)
        OR EXISTS (SELECT 1 FROM dashboard_settings) THEN
        RAISE EXCEPTION 'cannot safely roll back control center: control center data exists';
    END IF;
    IF EXISTS (SELECT 1 FROM training_sessions WHERE status NOT IN ('active', 'finished')) THEN
        RAISE EXCEPTION 'cannot safely roll back control center: scheduled/cancelled/excused sessions exist';
    END IF;
    IF EXISTS (SELECT 1 FROM training_sets WHERE type = 'drop') THEN
        RAISE EXCEPTION 'cannot safely roll back control center: drop sets exist';
    END IF;
    IF EXISTS (SELECT 1 FROM training_sessions WHERE scheduled_at IS NOT NULL OR source <> 'manual' OR external_id IS NOT NULL OR strain IS NOT NULL OR notes <> '')
        OR EXISTS (SELECT 1 FROM training_sets WHERE source <> 'manual' OR external_id IS NOT NULL OR rest_seconds IS NOT NULL OR notes <> '')
        OR EXISTS (SELECT 1 FROM training_session_exercises WHERE source <> 'manual' OR external_id IS NOT NULL)
        OR EXISTS (SELECT 1 FROM training_exercises WHERE source <> 'manual' OR external_id IS NOT NULL OR notes <> '' OR slug IS NOT NULL
            OR primary_muscle_group IS NOT NULL OR cardinality(secondary_muscle_groups) > 0
            OR exercise_type IS NOT NULL OR equipment IS NOT NULL OR unilateral)
        OR EXISTS (SELECT 1 FROM training_programs WHERE source <> 'manual' OR external_id IS NOT NULL)
        OR EXISTS (SELECT 1 FROM workout_templates WHERE source <> 'manual' OR description <> '')
        OR EXISTS (SELECT 1 FROM workout_template_exercises WHERE notes <> '') THEN
        RAISE EXCEPTION 'cannot safely roll back control center: extended training metadata exists';
    END IF;
END $$;
-- +goose StatementEnd

DROP TABLE data_imports;
DROP TABLE dashboard_settings;
DROP TABLE body_measurements;
DROP TABLE nutrition_days;
DROP TABLE sleep_entries;
DROP TABLE recovery_entries;

DROP INDEX training_sets_completed_at_idx;
DROP INDEX training_sets_source_external_uidx;
ALTER TABLE training_sets
    DROP CONSTRAINT training_sets_type_check,
    ADD CONSTRAINT training_sets_type_check CHECK (type IN ('warmup', 'working')),
    DROP COLUMN updated_at,
    DROP COLUMN external_id,
    DROP COLUMN source,
    DROP COLUMN notes,
    DROP COLUMN rest_seconds;

DROP INDEX training_session_exercises_source_external_uidx;
ALTER TABLE training_session_exercises
    DROP COLUMN updated_at,
    DROP COLUMN external_id,
    DROP COLUMN source;

DROP INDEX training_sessions_owner_scheduled_idx;
DROP INDEX training_sessions_owner_status_date_idx;
DROP INDEX training_sessions_owner_source_external_uidx;
ALTER TABLE training_sessions
    DROP CONSTRAINT training_sessions_lifecycle_check,
    DROP CONSTRAINT training_sessions_status_check,
    ADD CONSTRAINT training_sessions_status_check CHECK (status IN ('active', 'finished')),
    ALTER COLUMN started_at SET NOT NULL,
    DROP COLUMN updated_at,
    DROP COLUMN created_at,
    DROP COLUMN external_id,
    DROP COLUMN source,
    DROP COLUMN notes,
    DROP COLUMN strain,
    DROP COLUMN scheduled_at;

DROP INDEX training_exercises_owner_source_external_uidx;
ALTER TABLE training_exercises
    DROP COLUMN external_id,
    DROP COLUMN source,
    DROP COLUMN notes,
    DROP COLUMN unilateral,
    DROP COLUMN equipment,
    DROP COLUMN exercise_type,
    DROP COLUMN secondary_muscle_groups,
    DROP COLUMN primary_muscle_group,
    DROP COLUMN slug;

ALTER TABLE workout_template_exercises DROP COLUMN notes;

DROP INDEX workout_templates_owner_source_external_idx;
ALTER TABLE workout_templates
    DROP COLUMN source,
    DROP COLUMN description;

DROP INDEX training_programs_owner_source_external_uidx;
ALTER TABLE training_programs
    DROP COLUMN external_id,
    DROP COLUMN source;
