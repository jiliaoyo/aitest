-- +goose Up

ALTER TABLE ai_runs
    ADD COLUMN user_id uuid,
    ADD COLUMN estimated_cost_usd numeric(18, 8);

-- 旧调用用 session/item ID 作为 input_ref，迁移时补齐账号归属。
UPDATE ai_runs ar
SET user_id = ps.user_id
FROM practice_sessions ps
WHERE ar.user_id IS NULL
  AND ar.input_ref = ps.id::text;

UPDATE ai_runs ar
SET user_id = ps.user_id
FROM practice_items pi
JOIN practice_sessions ps ON ps.id = pi.session_id
WHERE ar.user_id IS NULL
  AND ar.input_ref = pi.id::text;

ALTER TABLE ai_runs
    ADD CONSTRAINT ai_runs_user_id_fkey FOREIGN KEY (user_id) REFERENCES users(id);

CREATE INDEX idx_ai_runs_user_created ON ai_runs (user_id, created_at DESC);

-- +goose Down

DROP INDEX IF EXISTS idx_ai_runs_user_created;
ALTER TABLE ai_runs DROP CONSTRAINT IF EXISTS ai_runs_user_id_fkey;
ALTER TABLE ai_runs DROP COLUMN IF EXISTS estimated_cost_usd;
ALTER TABLE ai_runs DROP COLUMN IF EXISTS user_id;
