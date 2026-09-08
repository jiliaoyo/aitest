-- +goose Up

ALTER TABLE ai_runs
    ADD COLUMN http_status integer,
    ADD COLUMN business_status text NOT NULL DEFAULT 'unknown',
    ADD COLUMN failure_kind text NOT NULL DEFAULT '';

ALTER TABLE ai_runs
    ADD CONSTRAINT ai_runs_http_status_check
        CHECK (http_status IS NULL OR (http_status >= 100 AND http_status <= 599)),
    ADD CONSTRAINT ai_runs_business_status_check
        CHECK (business_status = ANY (ARRAY['unknown', 'pending', 'succeeded', 'failed', 'not_applicable']));

-- +goose Down

ALTER TABLE ai_runs
    DROP CONSTRAINT IF EXISTS ai_runs_business_status_check,
    DROP CONSTRAINT IF EXISTS ai_runs_http_status_check,
    DROP COLUMN IF EXISTS failure_kind,
    DROP COLUMN IF EXISTS business_status,
    DROP COLUMN IF EXISTS http_status;
