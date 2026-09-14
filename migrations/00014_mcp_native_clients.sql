-- +goose Up
-- Empty client_hash preserves the existing pre-registered web client's grants.
ALTER TABLE mcp_oauth_grants ADD COLUMN client_hash TEXT NOT NULL DEFAULT '';

-- +goose Down
-- Native grants must not become web grants when rolling back client isolation.
DELETE FROM mcp_oauth_grants WHERE client_hash <> '';
ALTER TABLE mcp_oauth_grants DROP COLUMN client_hash;
