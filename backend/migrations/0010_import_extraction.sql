-- +goose Up
ALTER TABLE import_jobs
    ADD COLUMN extracted_text text NOT NULL DEFAULT '',
    ADD COLUMN stage_times jsonb NOT NULL DEFAULT '{}';

-- +goose Down
ALTER TABLE import_jobs
    DROP COLUMN IF EXISTS stage_times,
    DROP COLUMN IF EXISTS extracted_text;
