-- +goose Up
CREATE TABLE question_ai_explanations (
    question_version_id uuid PRIMARY KEY REFERENCES question_versions(id),
    prompt_version text NOT NULL,
    model text NOT NULL DEFAULT '',
    explanation text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE IF EXISTS question_ai_explanations;
