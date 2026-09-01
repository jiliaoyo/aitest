-- +goose Up
CREATE TABLE practice_sessions (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id uuid NOT NULL REFERENCES users(id),
    status text NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'grading', 'completed', 'analysis_failed')),
    level_id uuid NOT NULL REFERENCES exam_levels(id),
    subject_id uuid REFERENCES subjects(id),
    scope jsonb NOT NULL DEFAULT '{}',
    requested_count integer NOT NULL,
    submit_key text,
    submit_hash text,
    submitted_at timestamptz,
    completed_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX uq_practice_sessions_submit_key ON practice_sessions(submit_key) WHERE submit_key IS NOT NULL;
CREATE INDEX idx_practice_sessions_user ON practice_sessions(user_id, status, created_at);

CREATE TABLE practice_items (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    session_id uuid NOT NULL REFERENCES practice_sessions(id),
    question_id uuid NOT NULL REFERENCES questions(id),
    question_version_id uuid NOT NULL REFERENCES question_versions(id),
    position integer NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (session_id, position),
    UNIQUE (session_id, question_id)
);

CREATE TABLE user_answers (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    session_id uuid NOT NULL REFERENCES practice_sessions(id),
    item_id uuid NOT NULL UNIQUE REFERENCES practice_items(id),
    user_id uuid NOT NULL REFERENCES users(id),
    value jsonb,
    marked_for_review boolean NOT NULL DEFAULT false,
    saved_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE grading_results (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    session_id uuid NOT NULL REFERENCES practice_sessions(id),
    item_id uuid NOT NULL REFERENCES practice_items(id),
    source text NOT NULL CHECK (source IN ('deterministic', 'ai')),
    status text NOT NULL CHECK (status IN ('pending', 'correct', 'incorrect', 'unanswered', 'failed')),
    answer_authority text CHECK (answer_authority IN ('official', 'human_verified')),
    correct_value jsonb,
    user_value jsonb,
    explanation text,
    explanation_source text CHECK (explanation_source IN ('official', 'human_verified', 'ai')),
    ai_run_id uuid,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (item_id, source)
);
CREATE INDEX idx_grading_results_session ON grading_results(session_id);

-- +goose Down
DROP TABLE IF EXISTS grading_results;
DROP TABLE IF EXISTS user_answers;
DROP TABLE IF EXISTS practice_items;
DROP TABLE IF EXISTS practice_sessions;
