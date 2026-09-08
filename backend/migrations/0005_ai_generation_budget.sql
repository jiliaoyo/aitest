-- +goose Up

ALTER TABLE practice_sessions
    ADD COLUMN ai_generation_calls_used integer NOT NULL DEFAULT 0,
    ADD COLUMN ai_generation_call_budget integer NOT NULL DEFAULT 6,
    ADD COLUMN ai_generation_last_error text NOT NULL DEFAULT '';

ALTER TABLE practice_sessions
    ADD CONSTRAINT practice_sessions_ai_generation_budget_check
        CHECK (ai_generation_calls_used >= 0 AND ai_generation_call_budget > 0 AND ai_generation_calls_used <= ai_generation_call_budget);

-- +goose Down

ALTER TABLE practice_sessions
    DROP CONSTRAINT IF EXISTS practice_sessions_ai_generation_budget_check,
    DROP COLUMN IF EXISTS ai_generation_last_error,
    DROP COLUMN IF EXISTS ai_generation_call_budget,
    DROP COLUMN IF EXISTS ai_generation_calls_used;
