-- +goose Up
CREATE TABLE user_knowledge_stats (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id uuid NOT NULL REFERENCES users(id),
    knowledge_point_id uuid NOT NULL REFERENCES knowledge_points(id),
    confirmed_answered integer NOT NULL DEFAULT 0,
    confirmed_correct integer NOT NULL DEFAULT 0,
    recent_answered integer NOT NULL DEFAULT 0,
    recent_correct integer NOT NULL DEFAULT 0,
    ai_answered integer NOT NULL DEFAULT 0,
    ai_correct integer NOT NULL DEFAULT 0,
    consecutive_wrong integer NOT NULL DEFAULT 0,
    last_practiced_at timestamptz,
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (user_id, knowledge_point_id)
);

CREATE TABLE issue_reports (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id uuid NOT NULL REFERENCES users(id),
    question_id uuid NOT NULL REFERENCES questions(id),
    question_version_id uuid NOT NULL REFERENCES question_versions(id),
    practice_item_id uuid REFERENCES practice_items(id),
    session_id uuid REFERENCES practice_sessions(id),
    target_type text NOT NULL CHECK (target_type IN ('stem', 'answer', 'explanation', 'classification', 'ai_grading')),
    description text NOT NULL DEFAULT '',
    context jsonb NOT NULL DEFAULT '{}',
    status text NOT NULL DEFAULT 'open' CHECK (status IN ('open', 'resolved', 'dismissed')),
    resolution_note text NOT NULL DEFAULT '',
    handled_by uuid REFERENCES users(id),
    handled_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX idx_issue_reports_status ON issue_reports(status, created_at);

CREATE TABLE import_jobs (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    created_by uuid NOT NULL REFERENCES users(id),
    file_name text NOT NULL,
    stored_path text NOT NULL DEFAULT '',
    file_sha256 text NOT NULL DEFAULT '',
    mime_type text NOT NULL DEFAULT '',
    size_bytes bigint NOT NULL DEFAULT 0,
    status text NOT NULL DEFAULT 'uploaded' CHECK (status IN ('uploaded', 'extracting', 'structuring', 'review_ready', 'published', 'failed')),
    stage_error text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE import_items (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    import_job_id uuid NOT NULL REFERENCES import_jobs(id),
    position integer NOT NULL,
    raw_excerpt text NOT NULL DEFAULT '',
    ai_draft jsonb,
    anomalies jsonb NOT NULL DEFAULT '[]',
    review_status text NOT NULL DEFAULT 'pending' CHECK (review_status IN ('pending', 'approved', 'published', 'rejected')),
    published_question_id uuid REFERENCES questions(id),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (import_job_id, position)
);

CREATE TABLE jobs (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    kind text NOT NULL,
    payload jsonb NOT NULL,
    status text NOT NULL DEFAULT 'queued' CHECK (status IN ('queued', 'running', 'succeeded', 'failed')),
    attempts integer NOT NULL DEFAULT 0,
    max_attempts integer NOT NULL DEFAULT 3,
    available_at timestamptz NOT NULL DEFAULT now(),
    locked_by text,
    locked_until timestamptz,
    last_error text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX idx_jobs_claim ON jobs(status, available_at);

CREATE TABLE ai_runs (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    kind text NOT NULL,
    prompt_version text NOT NULL,
    model text NOT NULL DEFAULT '',
    input_ref text NOT NULL DEFAULT '',
    output jsonb,
    prompt_tokens integer NOT NULL DEFAULT 0,
    completion_tokens integer NOT NULL DEFAULT 0,
    duration_ms integer NOT NULL DEFAULT 0,
    error text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE audit_logs (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    actor_user_id uuid REFERENCES users(id),
    action text NOT NULL,
    object_type text NOT NULL,
    object_id text NOT NULL DEFAULT '',
    detail jsonb NOT NULL DEFAULT '{}',
    created_at timestamptz NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE IF EXISTS audit_logs;
DROP TABLE IF EXISTS ai_runs;
DROP TABLE IF EXISTS jobs;
DROP TABLE IF EXISTS import_items;
DROP TABLE IF EXISTS import_jobs;
DROP TABLE IF EXISTS issue_reports;
DROP TABLE IF EXISTS user_knowledge_stats;
