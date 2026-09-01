-- +goose Up
CREATE TABLE sources (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    name text NOT NULL,
    kind text NOT NULL CHECK (kind IN ('book', 'past_exam', 'self_made', 'ai_generated')),
    author text NOT NULL DEFAULT '',
    publisher text NOT NULL DEFAULT '',
    year integer,
    license_note text NOT NULL DEFAULT '',
    internal_note text NOT NULL DEFAULT '',
    created_by uuid REFERENCES users(id),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE source_sections (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    source_id uuid NOT NULL REFERENCES sources(id) ON DELETE CASCADE,
    name text NOT NULL,
    sort_order integer NOT NULL DEFAULT 0,
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (source_id, name)
);

CREATE TABLE materials (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    current_version_id uuid,
    created_by uuid REFERENCES users(id),
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE material_versions (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    material_id uuid NOT NULL REFERENCES materials(id),
    version_no integer NOT NULL,
    title text NOT NULL DEFAULT '',
    content text NOT NULL,
    created_by uuid REFERENCES users(id),
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (material_id, version_no)
);
ALTER TABLE materials
    ADD CONSTRAINT fk_materials_current_version FOREIGN KEY (current_version_id) REFERENCES material_versions(id);

CREATE TABLE questions (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    current_version_id uuid,
    published_version_id uuid,
    status text NOT NULL DEFAULT 'draft' CHECK (status IN ('draft', 'in_review', 'published', 'retired')),
    has_answer boolean NOT NULL DEFAULT false,
    created_by uuid REFERENCES users(id),
    published_at timestamptz,
    retired_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE question_versions (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    question_id uuid NOT NULL REFERENCES questions(id),
    version_no integer NOT NULL,
    type text NOT NULL CHECK (type IN ('single_choice', 'multiple_choice', 'fill_blank', 'short_answer')),
    stem text NOT NULL,
    material_version_id uuid REFERENCES material_versions(id),
    options jsonb,
    level_id uuid NOT NULL REFERENCES exam_levels(id),
    subject_id uuid NOT NULL REFERENCES subjects(id),
    source_section_id uuid REFERENCES source_sections(id),
    difficulty integer NOT NULL DEFAULT 3 CHECK (difficulty BETWEEN 1 AND 5),
    created_by uuid REFERENCES users(id),
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (question_id, version_no)
);
ALTER TABLE questions
    ADD CONSTRAINT fk_questions_current_version FOREIGN KEY (current_version_id) REFERENCES question_versions(id),
    ADD CONSTRAINT fk_questions_published_version FOREIGN KEY (published_version_id) REFERENCES question_versions(id);

CREATE TABLE question_version_knowledge_points (
    question_version_id uuid NOT NULL REFERENCES question_versions(id),
    knowledge_point_id uuid NOT NULL REFERENCES knowledge_points(id),
    PRIMARY KEY (question_version_id, knowledge_point_id)
);

CREATE TABLE answer_keys (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    question_version_id uuid NOT NULL UNIQUE REFERENCES question_versions(id),
    value jsonb NOT NULL,
    authority text NOT NULL CHECK (authority IN ('official', 'human_verified')),
    explanation text NOT NULL DEFAULT '',
    created_by uuid REFERENCES users(id),
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX idx_questions_status ON questions(status, published_version_id);
CREATE INDEX idx_question_versions_level_subject ON question_versions(level_id, subject_id);

-- +goose Down
DROP TABLE IF EXISTS answer_keys;
DROP TABLE IF EXISTS question_version_knowledge_points;
DROP TABLE IF EXISTS questions;
DROP TABLE IF EXISTS question_versions;
DROP TABLE IF EXISTS material_versions;
DROP TABLE IF EXISTS materials;
DROP TABLE IF EXISTS source_sections;
DROP TABLE IF EXISTS sources;
