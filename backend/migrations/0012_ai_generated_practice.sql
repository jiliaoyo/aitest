-- +goose Up
ALTER TABLE practice_sessions
    DROP CONSTRAINT IF EXISTS practice_sessions_status_check;

ALTER TABLE practice_sessions
    ADD CONSTRAINT practice_sessions_status_check
    CHECK (status IN ('generating', 'active', 'grading', 'completed', 'analysis_failed', 'generation_failed'));

-- AI 生成题的答案不是 official / human_verified，不能写入 answer_keys。
CREATE TABLE ai_generated_question_answers (
    question_version_id uuid PRIMARY KEY REFERENCES question_versions(id) ON DELETE CASCADE,
    value jsonb NOT NULL,
    explanation text NOT NULL DEFAULT '',
    prompt_version text NOT NULL,
    model text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE IF EXISTS ai_generated_question_answers;
ALTER TABLE practice_sessions
    DROP CONSTRAINT IF EXISTS practice_sessions_status_check;
ALTER TABLE practice_sessions
    ADD CONSTRAINT practice_sessions_status_check
    CHECK (status IN ('active', 'grading', 'completed', 'analysis_failed'));
