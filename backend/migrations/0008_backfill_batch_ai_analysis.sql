-- +goose Up
WITH legacy_sessions AS (
    SELECT DISTINCT (payload->>'sessionId')::uuid AS session_id
    FROM jobs
    WHERE kind = 'explain_practice_item_ai'
      AND status IN ('queued', 'running', 'succeeded', 'failed')
      AND payload->>'sessionId' ~* '^[0-9a-f-]{36}$'
)
UPDATE practice_sessions ps
SET ai_summary_status = 'pending', updated_at = now()
FROM legacy_sessions ls
WHERE ps.id = ls.session_id
  AND ps.status <> 'active'
  AND ps.ai_summary_status = 'not_requested';

INSERT INTO jobs (kind, payload)
SELECT 'analyze_practice_session_ai', jsonb_build_object('sessionId', ps.id::text)
FROM practice_sessions ps
WHERE ps.ai_summary_status = 'pending'
  AND EXISTS (
      SELECT 1 FROM jobs j
      WHERE j.kind = 'explain_practice_item_ai'
        AND j.payload->>'sessionId' = ps.id::text
  )
  AND NOT EXISTS (
      SELECT 1 FROM jobs j
      WHERE j.kind = 'analyze_practice_session_ai'
        AND j.payload->>'sessionId' = ps.id::text
        AND j.status IN ('queued', 'running', 'succeeded')
  );

-- +goose Down
DELETE FROM jobs WHERE kind = 'analyze_practice_session_ai';
UPDATE practice_sessions ps
SET ai_summary_status = 'not_requested', ai_summary = '', updated_at = now()
WHERE ps.ai_summary_status = 'pending';
