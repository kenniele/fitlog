-- +goose Up
CREATE TABLE training_programs (
    id         BIGSERIAL PRIMARY KEY,
    owner_id   BIGINT      NOT NULL,
    name       TEXT        NOT NULL CHECK (btrim(name) <> ''),
    sort_order INTEGER     NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX training_programs_owner_name_idx
    ON training_programs (owner_id, lower(name));

CREATE TABLE training_program_exercises (
    id         BIGSERIAL PRIMARY KEY,
    program_id BIGINT  NOT NULL REFERENCES training_programs(id) ON DELETE CASCADE,
    position   INTEGER NOT NULL CHECK (position > 0),
    name       TEXT    NOT NULL CHECK (btrim(name) <> ''),
    UNIQUE (program_id, position)
);

CREATE TABLE training_sessions (
    id                   BIGSERIAL PRIMARY KEY,
    owner_id             BIGINT      NOT NULL,
    program_id           BIGINT      REFERENCES training_programs(id) ON DELETE SET NULL,
    program_name         TEXT        NOT NULL,
    status               TEXT        NOT NULL DEFAULT 'active'
                                  CHECK (status IN ('active', 'finished')),
    current_position     INTEGER     NOT NULL DEFAULT 1 CHECK (current_position > 0),
    started_at           TIMESTAMPTZ NOT NULL,
    finished_at          TIMESTAMPTZ,
    published_chat_id    BIGINT,
    published_message_id INTEGER
);
CREATE UNIQUE INDEX training_one_active_session_idx
    ON training_sessions (owner_id) WHERE status = 'active';
CREATE INDEX training_sessions_history_idx
    ON training_sessions (owner_id, started_at DESC);

CREATE TABLE training_session_exercises (
    id         BIGSERIAL PRIMARY KEY,
    session_id BIGINT  NOT NULL REFERENCES training_sessions(id) ON DELETE CASCADE,
    position   INTEGER NOT NULL CHECK (position > 0),
    name       TEXT    NOT NULL,
    note       TEXT    NOT NULL DEFAULT '',
    complete   BOOLEAN NOT NULL DEFAULT false,
    UNIQUE (session_id, position)
);

CREATE TABLE training_sets (
    id                  BIGSERIAL PRIMARY KEY,
    session_exercise_id BIGINT         NOT NULL REFERENCES training_session_exercises(id) ON DELETE CASCADE,
    position            INTEGER        NOT NULL CHECK (position > 0),
    reps                INTEGER        NOT NULL CHECK (reps > 0),
    weight_kg           NUMERIC(8, 2)  CHECK (weight_kg > 0),
    created_at          TIMESTAMPTZ    NOT NULL DEFAULT now(),
    UNIQUE (session_exercise_id, position)
);

CREATE TABLE training_ui_states (
    owner_id       BIGINT      PRIMARY KEY,
    chat_id        BIGINT      NOT NULL DEFAULT 0,
    message_id     INTEGER     NOT NULL DEFAULT 0,
    mode           TEXT        NOT NULL DEFAULT ''
                               CHECK (mode IN ('', 'set', 'note', 'import_file', 'import_preview')),
    pending_import JSONB,
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE training_ui_states;
DROP TABLE training_sets;
DROP TABLE training_session_exercises;
DROP TABLE training_sessions;
DROP TABLE training_program_exercises;
DROP TABLE training_programs;
