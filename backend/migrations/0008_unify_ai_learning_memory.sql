-- +goose Up

-- 版本化任务名避免发布切换期的旧 worker 用旧规则提前消费；新 worker 会恢复旧版失败的 AI 客观题并重建统一记忆。
INSERT INTO jobs (kind, payload, max_attempts)
SELECT 'rebuild_user_learning_memory_v2', jsonb_build_object('userId', ps.user_id::text), 20
FROM practice_sessions ps
WHERE ps.submitted_at IS NOT NULL
  AND ps.scope->>'mode' = 'ai_generated'
GROUP BY ps.user_id;

-- +goose Down

DELETE FROM jobs WHERE kind = 'rebuild_user_learning_memory_v2';
