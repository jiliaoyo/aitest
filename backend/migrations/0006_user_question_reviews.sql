-- +goose Up

CREATE TABLE user_question_reviews (
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    question_id uuid NOT NULL REFERENCES questions(id) ON DELETE CASCADE,
    stage integer NOT NULL DEFAULT 0,
    next_review_at timestamp with time zone NOT NULL,
    last_grading_result_id uuid,
    last_status text NOT NULL,
    updated_at timestamp with time zone NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, question_id),
    CONSTRAINT user_question_reviews_stage_check CHECK (stage >= 0 AND stage <= 4),
    CONSTRAINT user_question_reviews_status_check CHECK (last_status IN ('incorrect', 'unanswered', 'correct'))
);

CREATE INDEX idx_user_question_reviews_due
    ON user_question_reviews (user_id, next_review_at);

-- +goose Down

DROP INDEX IF EXISTS idx_user_question_reviews_due;
DROP TABLE IF EXISTS user_question_reviews;
