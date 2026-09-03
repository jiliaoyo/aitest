-- +goose Up
CREATE TABLE user_learning_memory (
    user_id uuid PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    reset_at timestamptz,
    ai_advice text NOT NULL DEFAULT '',
    ai_advice_status text NOT NULL DEFAULT 'not_requested'
        CHECK (ai_advice_status IN ('not_requested', 'pending', 'completed', 'failed')),
    ai_advice_updated_at timestamptz,
    updated_at timestamptz NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE IF EXISTS user_learning_memory;
