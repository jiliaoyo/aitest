-- +goose Up

-- 0017 的重分析已完成，但旧批次仍保留 analysis_failed 状态；统一收敛为完成。
UPDATE practice_sessions
SET status = 'completed',
    completed_at = COALESCE(completed_at, now()),
    updated_at = now()
WHERE status = 'analysis_failed'
  AND ai_summary_status = 'completed';

-- +goose Down

-- 状态修复不可逆，不回滚为失败状态。
