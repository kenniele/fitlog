-- +goose Up
CREATE TABLE oauth_tokens (
    source        TEXT PRIMARY KEY,
    access_token  BYTEA NOT NULL,
    refresh_token BYTEA NOT NULL,
    expires_at    TIMESTAMPTZ NOT NULL,
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE oauth_tokens;
