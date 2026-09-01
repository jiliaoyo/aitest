-- +goose Up
CREATE TABLE exams (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    code text NOT NULL UNIQUE,
    name text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE exam_levels (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    exam_id uuid NOT NULL REFERENCES exams(id),
    code text NOT NULL,
    name text NOT NULL,
    sort_order integer NOT NULL DEFAULT 0,
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (exam_id, code)
);

CREATE TABLE subjects (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    exam_id uuid NOT NULL REFERENCES exams(id),
    code text NOT NULL,
    name text NOT NULL,
    sort_order integer NOT NULL DEFAULT 0,
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (exam_id, code)
);

CREATE TABLE knowledge_points (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    exam_id uuid NOT NULL REFERENCES exams(id),
    level_id uuid NOT NULL REFERENCES exam_levels(id),
    subject_id uuid NOT NULL REFERENCES subjects(id),
    parent_id uuid REFERENCES knowledge_points(id),
    name text NOT NULL,
    description text NOT NULL DEFAULT '',
    common_mistakes text NOT NULL DEFAULT '',
    examples text NOT NULL DEFAULT '',
    status text NOT NULL DEFAULT 'draft' CHECK (status IN ('draft', 'published', 'retired')),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX idx_knowledge_points_level_subject ON knowledge_points(level_id, subject_id, status);

-- +goose Down
DROP TABLE IF EXISTS knowledge_points;
DROP TABLE IF EXISTS subjects;
DROP TABLE IF EXISTS exam_levels;
DROP TABLE IF EXISTS exams;
