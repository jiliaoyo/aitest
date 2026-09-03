-- 只清理两本书的题目及其依赖；自建题和自建练习不在目标集合内。
BEGIN;

CREATE TEMP TABLE target_sources ON COMMIT DROP AS
SELECT id
FROM sources
WHERE kind = 'book'
  AND name IN (
    '蓝宝书N4N5文法',
    '红蓝宝书1000题 新日本语能力考试N4N5（练习+详解）'
  );

DO $$
BEGIN
  IF (SELECT count(*) FROM target_sources) <> 2 THEN
    RAISE EXCEPTION 'expected exactly 2 book sources, found %', (SELECT count(*) FROM target_sources);
  END IF;
END
$$;

CREATE TEMP TABLE target_questions ON COMMIT DROP AS
SELECT DISTINCT qv.question_id AS id
FROM question_versions qv
JOIN source_sections ss ON ss.id = qv.source_section_id
JOIN target_sources src ON src.id = ss.source_id;

CREATE TEMP TABLE target_versions ON COMMIT DROP AS
SELECT qv.id
FROM question_versions qv
JOIN target_questions q ON q.id = qv.question_id;

CREATE TEMP TABLE target_book_items ON COMMIT DROP AS
SELECT pi.id
FROM practice_items pi
JOIN target_questions q ON q.id = pi.question_id;

CREATE TEMP TABLE target_sessions ON COMMIT DROP AS
SELECT DISTINCT pi.session_id AS id
FROM practice_items pi
JOIN target_book_items ti ON ti.id = pi.id;

CREATE TEMP TABLE target_items ON COMMIT DROP AS
SELECT pi.id
FROM practice_items pi
JOIN target_sessions ts ON ts.id = pi.session_id;

CREATE TEMP TABLE target_materials ON COMMIT DROP AS
SELECT DISTINCT mv.material_id AS id
FROM question_versions qv
JOIN target_versions tv ON tv.id = qv.id
JOIN material_versions mv ON mv.id = qv.material_version_id
WHERE qv.material_version_id IS NOT NULL;

DELETE FROM issue_reports ir
WHERE ir.session_id IN (SELECT id FROM target_sessions)
   OR ir.practice_item_id IN (SELECT id FROM target_items)
   OR ir.question_id IN (SELECT id FROM target_questions)
   OR ir.question_version_id IN (SELECT id FROM target_versions);

DELETE FROM jobs
WHERE payload ->> 'sessionId' IN (SELECT id::text FROM target_sessions)
   OR payload ->> 'itemId' IN (SELECT id::text FROM target_items);

DELETE FROM user_answers WHERE session_id IN (SELECT id FROM target_sessions);
DELETE FROM grading_results WHERE session_id IN (SELECT id FROM target_sessions);
DELETE FROM practice_items WHERE id IN (SELECT id FROM target_items);
DELETE FROM practice_sessions WHERE id IN (SELECT id FROM target_sessions);

DELETE FROM import_items WHERE published_question_id IN (SELECT id FROM target_questions);
DELETE FROM question_version_knowledge_points
WHERE question_version_id IN (SELECT id FROM target_versions);
DELETE FROM answer_keys WHERE question_version_id IN (SELECT id FROM target_versions);

UPDATE questions
SET current_version_id = NULL, published_version_id = NULL
WHERE id IN (SELECT id FROM target_questions);

DELETE FROM question_versions WHERE id IN (SELECT id FROM target_versions);
DELETE FROM questions WHERE id IN (SELECT id FROM target_questions);

UPDATE materials SET current_version_id = NULL
WHERE id IN (SELECT id FROM target_materials);
DELETE FROM material_versions WHERE material_id IN (SELECT id FROM target_materials);
DELETE FROM materials WHERE id IN (SELECT id FROM target_materials);

DELETE FROM source_sections
WHERE source_id IN (SELECT id FROM target_sources);
DELETE FROM sources WHERE id IN (SELECT id FROM target_sources);

COMMIT;
