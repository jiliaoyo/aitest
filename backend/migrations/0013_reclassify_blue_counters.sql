-- +goose Up
-- 蓝宝书 N5 第一单元问题 3 是助数词填空，修正为文字词汇；复制版本以保留旧练习引用。
DO $$
DECLARE
    item record;
    new_version uuid;
    vocabulary_subject uuid;
    vocabulary_root uuid;
BEGIN
    SELECT s.id INTO vocabulary_subject
    FROM subjects s JOIN exams e ON e.id = s.exam_id
    WHERE e.code = 'jlpt' AND s.code = 'vocabulary';

    SELECT kp.id INTO vocabulary_root
    FROM knowledge_points kp
    JOIN exam_levels l ON l.id = kp.level_id
    JOIN subjects s ON s.id = kp.subject_id
    WHERE l.code = 'n5' AND s.code = 'vocabulary' AND kp.parent_id IS NULL AND kp.status = 'published';

    IF vocabulary_subject IS NULL OR vocabulary_root IS NULL THEN
        RETURN;
    END IF;

    FOR item IN
        SELECT q.id AS question_id, v.*
        FROM questions q
        JOIN question_versions v ON v.id = q.published_version_id
        JOIN source_sections ss ON ss.id = v.source_section_id
        JOIN sources src ON src.id = ss.source_id
        JOIN exam_levels l ON l.id = v.level_id
        WHERE q.status = 'published'
          AND src.name = '蓝宝书N4N5文法'
          AND ss.name = 'N5·第1单元·基础练习·問題3'
          AND l.code = 'n5'
          AND v.type = 'fill_blank'
          AND v.subject_id <> vocabulary_subject
    LOOP
        INSERT INTO question_versions
            (question_id, version_no, source_order, type, stem, material_version_id, options,
             level_id, subject_id, source_section_id, difficulty, created_by)
        SELECT item.question_id, coalesce(max(version_no), 0) + 1, item.source_order, item.type,
               item.stem, item.material_version_id, item.options, item.level_id, vocabulary_subject,
               item.source_section_id, item.difficulty, item.created_by
        FROM question_versions
        WHERE question_id = item.question_id
        RETURNING id INTO new_version;

        INSERT INTO answer_keys (question_version_id, value, authority, explanation, created_by)
        SELECT new_version, ak.value, ak.authority, ak.explanation, ak.created_by
        FROM answer_keys ak
        WHERE ak.question_version_id = item.id;

        INSERT INTO question_version_knowledge_points (question_version_id, knowledge_point_id)
        VALUES (new_version, vocabulary_root);

        UPDATE questions
        SET current_version_id = new_version, published_version_id = new_version, updated_at = now()
        WHERE id = item.question_id;

        INSERT INTO audit_logs (action, object_type, object_id, detail)
        VALUES ('question_reclassified', 'question', item.question_id::text,
                jsonb_build_object('fromVersion', item.id, 'toVersion', new_version,
                                    'reason', 'N5 第一单元问题 3 为助数词题'));
    END LOOP;
END $$;

-- +goose Down
-- 题目版本不可逆回退；旧版本保留供历史练习引用。
