-- +goose Up

-- 合并同一账号尚未领取的学习统计重建任务，避免 JSONB 条件退化为全表扫描。
WITH ranked AS (
    SELECT id,
           row_number() OVER (
               PARTITION BY payload->>'userId'
               ORDER BY created_at DESC, id DESC
           ) AS position
    FROM jobs
    WHERE kind = 'rebuild_user_knowledge_stats' AND status = 'queued'
)
UPDATE jobs
SET status = 'succeeded',
    last_error = 'coalesced duplicate learning rebuild task',
    updated_at = now()
WHERE id IN (SELECT id FROM ranked WHERE position > 1);

CREATE UNIQUE INDEX uq_jobs_learning_rebuild_queued_user
    ON jobs ((payload->>'userId'))
    WHERE kind = 'rebuild_user_knowledge_stats' AND status = 'queued';

-- 让已有复习计划也立即按版本绑定和 AI 较短间隔重算，而不是等用户下一次作答。
INSERT INTO jobs (kind, payload)
SELECT 'rebuild_user_knowledge_stats', jsonb_build_object('userId', user_id::text)
FROM user_question_reviews
GROUP BY user_id
ON CONFLICT DO NOTHING;

-- +goose Down

DROP INDEX IF EXISTS uq_jobs_learning_rebuild_queued_user;
