-- +goose Up
ALTER TABLE nutrition_days
    ADD COLUMN fiber_incomplete BOOLEAN NOT NULL DEFAULT false;

-- +goose Down
ALTER TABLE nutrition_days DROP COLUMN fiber_incomplete;
