-- +goose Up

ALTER TABLE user_question_reviews
    ADD COLUMN mastered_at timestamp with time zone;

CREATE INDEX idx_user_question_reviews_mastered
    ON user_question_reviews (user_id, mastered_at)
    WHERE mastered_at IS NOT NULL;

-- +goose Down

DROP INDEX IF EXISTS idx_user_question_reviews_mastered;
ALTER TABLE user_question_reviews
    DROP COLUMN IF EXISTS mastered_at;
