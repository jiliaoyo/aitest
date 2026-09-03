-- +goose Up
ALTER TABLE practice_sessions
    ADD COLUMN ai_summary text NOT NULL DEFAULT '',
    ADD COLUMN ai_summary_status text NOT NULL DEFAULT 'not_requested'
        CHECK (ai_summary_status IN ('not_requested', 'pending', 'completed', 'failed'));

WITH queued_sessions AS (
    SELECT DISTINCT (payload->>'sessionId')::uuid AS session_id
    FROM jobs
    WHERE kind = 'explain_practice_item_ai'
      AND status = 'queued'
      AND payload->>'sessionId' ~* '^[0-9a-f-]{36}$'
)
UPDATE practice_sessions ps
SET ai_summary_status = 'pending'
FROM queued_sessions qs
WHERE ps.id = qs.session_id;

INSERT INTO jobs (kind, payload)
SELECT 'analyze_practice_session_ai', jsonb_build_object('sessionId', qs.session_id::text)
FROM (
    SELECT DISTINCT (payload->>'sessionId')::uuid AS session_id
    FROM jobs
    WHERE kind = 'explain_practice_item_ai'
      AND status = 'queued'
      AND payload->>'sessionId' ~* '^[0-9a-f-]{36}$'
) qs
JOIN practice_sessions ps ON ps.id = qs.session_id;

DELETE FROM jobs
WHERE kind = 'explain_practice_item_ai' AND status = 'queued';

-- +goose Down
DELETE FROM jobs WHERE kind = 'analyze_practice_session_ai';
ALTER TABLE practice_sessions
    DROP COLUMN IF EXISTS ai_summary_status,
    DROP COLUMN IF EXISTS ai_summary;
