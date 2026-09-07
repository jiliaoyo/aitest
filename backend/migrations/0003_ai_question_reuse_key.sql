-- +goose Up

ALTER TABLE question_versions
    ADD COLUMN ai_reuse_key text;

CREATE UNIQUE INDEX idx_question_versions_ai_reuse_key
    ON question_versions (ai_reuse_key)
    WHERE ai_reuse_key IS NOT NULL;

-- +goose Down

DROP INDEX IF EXISTS idx_question_versions_ai_reuse_key;
ALTER TABLE question_versions DROP COLUMN IF EXISTS ai_reuse_key;
