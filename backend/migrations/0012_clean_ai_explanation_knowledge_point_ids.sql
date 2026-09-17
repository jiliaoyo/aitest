-- +goose Up

-- 早期 AI 解析把知识点内部 UUID 写进了学习者可见文本；保留名称，去掉内部 ID。
UPDATE ai_generated_question_answers
SET explanation = regexp_replace(
    regexp_replace(
        explanation,
        '(?im)^(知识点：[[:space:]]*)[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}[[:space:]]*',
        E'\\1',
        'g'
    ),
    '(?im)^(知识点：[[:space:]]*)（(.*)）$',
    E'\\1\\2',
    'g'
)
WHERE explanation ~* '(?m)^知识点：[[:space:]]*[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}';

UPDATE question_ai_explanations
SET explanation = regexp_replace(
    regexp_replace(
        explanation,
        '(?im)^(知识点：[[:space:]]*)[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}[[:space:]]*',
        E'\\1',
        'g'
    ),
    '(?im)^(知识点：[[:space:]]*)（(.*)）$',
    E'\\1\\2',
    'g'
)
WHERE explanation ~* '(?m)^知识点：[[:space:]]*[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}';

UPDATE grading_results
SET explanation = regexp_replace(
    regexp_replace(
        explanation,
        '(?im)^(知识点：[[:space:]]*)[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}[[:space:]]*',
        E'\\1',
        'g'
    ),
    '(?im)^(知识点：[[:space:]]*)（(.*)）$',
    E'\\1\\2',
    'g'
)
WHERE explanation_source = 'ai'
  AND explanation ~* '(?m)^知识点：[[:space:]]*[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}';

-- +goose Down

-- 文本清理不可逆，保留已清理结果。
