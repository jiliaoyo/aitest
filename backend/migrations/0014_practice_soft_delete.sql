-- +goose Up
ALTER TABLE practice_sessions ADD COLUMN deleted_at timestamptz;
ALTER TABLE practice_items ADD COLUMN deleted_at timestamptz;

CREATE INDEX idx_practice_sessions_visible_user_created
    ON practice_sessions(user_id, created_at DESC) WHERE deleted_at IS NULL;
CREATE INDEX idx_practice_items_visible_session
    ON practice_items(session_id) WHERE deleted_at IS NULL;

-- +goose Down
DROP INDEX IF EXISTS idx_practice_items_visible_session;
DROP INDEX IF EXISTS idx_practice_sessions_visible_user_created;
ALTER TABLE practice_items DROP COLUMN IF EXISTS deleted_at;
ALTER TABLE practice_sessions DROP COLUMN IF EXISTS deleted_at;
