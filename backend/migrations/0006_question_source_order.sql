-- +goose Up
ALTER TABLE question_versions ADD COLUMN source_order integer;
CREATE INDEX idx_question_versions_source_order ON question_versions(source_section_id, source_order);

-- +goose Down
DROP INDEX IF EXISTS idx_question_versions_source_order;
ALTER TABLE question_versions DROP COLUMN IF EXISTS source_order;
