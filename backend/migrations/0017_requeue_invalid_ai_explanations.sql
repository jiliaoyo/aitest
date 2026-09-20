-- +goose Up

-- 旧版本把语言异常的 AI 解析写成了普通已完成成绩，导致结果页没有重试入口。
WITH affected AS (
    SELECT DISTINCT session_id
    FROM grading_results
    WHERE source = 'ai'
      AND explanation LIKE 'AI 解析语言异常%'
), updated AS (
    UPDATE practice_sessions ps
    SET status = 'analysis_failed',
        ai_summary_status = 'failed',
        ai_summary = 'AI 解析语言异常，待重新分析。',
        completed_at = COALESCE(ps.completed_at, now()),
        updated_at = now()
    FROM affected
    WHERE ps.id = affected.session_id
      AND ps.status IN ('completed', 'analysis_failed')
    RETURNING ps.id
)
INSERT INTO jobs (kind, payload)
SELECT 'analyze_practice_session_ai', jsonb_build_object('sessionId', updated.id::text)
FROM updated
WHERE NOT EXISTS (
    SELECT 1
    FROM jobs j
    WHERE j.kind = 'analyze_practice_session_ai'
      AND j.status IN ('queued', 'running')
      AND j.payload->>'sessionId' = updated.id::text
);

-- +goose Down

-- 数据修复不可逆，保留已重新入队的任务和结果。
