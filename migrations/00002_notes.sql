-- +goose Up
CREATE TABLE notes (
    id    BIGSERIAL PRIMARY KEY,
    ts    TIMESTAMPTZ NOT NULL DEFAULT now(),
    kind  TEXT        NOT NULL,
    value DOUBLE PRECISION,
    body  TEXT        NOT NULL DEFAULT ''
);
CREATE INDEX notes_ts_idx ON notes (ts DESC);

-- +goose Down
DROP TABLE notes;
