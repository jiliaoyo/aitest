-- +goose Up

ALTER TABLE auth_sessions
    ADD COLUMN refresh_token_hash text,
    ADD COLUMN refresh_expires_at timestamptz;

ALTER TABLE auth_sessions
    ADD CONSTRAINT auth_sessions_refresh_token_hash_key UNIQUE (refresh_token_hash);

CREATE INDEX idx_auth_sessions_refresh_expires
    ON auth_sessions (refresh_expires_at)
    WHERE revoked_at IS NULL AND refresh_token_hash IS NOT NULL;

-- +goose Down

DROP INDEX IF EXISTS idx_auth_sessions_refresh_expires;
ALTER TABLE auth_sessions DROP CONSTRAINT IF EXISTS auth_sessions_refresh_token_hash_key;
ALTER TABLE auth_sessions DROP COLUMN IF EXISTS refresh_expires_at;
ALTER TABLE auth_sessions DROP COLUMN IF EXISTS refresh_token_hash;
