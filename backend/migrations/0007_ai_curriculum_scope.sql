-- +goose Up

-- N5 的“意志”表述曾让 AI 把 N4 的「～ために」误当成 N5 范围；把边界写回知识点正文，供管理端和生成提示词共同使用。
UPDATE knowledge_points kp
SET name = '愿望、计划与基础推量（不含意志形）',
    description = '只练习 N5 可用的愿望、计划与基础推量表达，如「～たい」「～ほしい」「～ましょう」「～でしょう」「～かもしれない」。不包含意志形「～（よ）う」、目的表达「～ために」或更高级句型。',
    common_mistakes = '把愿望、基础推量和说话人的意志混在一起；将 N4 以上的「～ために」或意志形套入 N5。',
    updated_at = now()
FROM exam_levels l
JOIN subjects s ON s.exam_id = l.exam_id
WHERE kp.level_id = l.id
  AND kp.subject_id = s.id
  AND l.code = 'n5'
  AND s.code = 'grammar'
  AND kp.name = '愿望、推量与意志';

-- +goose Down

UPDATE knowledge_points kp
SET name = '愿望、推量与意志',
    description = '表达愿望、计划、推测、意志和传闻。',
    common_mistakes = '把说话人的意志和对事实的推测使用在同一语境中。',
    updated_at = now()
FROM exam_levels l
JOIN subjects s ON s.exam_id = l.exam_id
WHERE kp.level_id = l.id
  AND kp.subject_id = s.id
  AND l.code = 'n5'
  AND s.code = 'grammar'
  AND kp.name = '愿望、计划与基础推量（不含意志形）';
