-- +goose Up

-- 知识点已在解析上方单独展示，移除 AI 解析中的重复段落。
UPDATE ai_generated_question_answers
SET explanation = regexp_replace(explanation, '(?im)^知识点[：:][^\r\n]*(?:\r?\n|$)', '', 'g')
WHERE explanation ~* '(?m)^知识点[：:]';

UPDATE question_ai_explanations
SET explanation = regexp_replace(explanation, '(?im)^知识点[：:][^\r\n]*(?:\r?\n|$)', '', 'g')
WHERE explanation ~* '(?m)^知识点[：:]';

UPDATE grading_results
SET explanation = regexp_replace(explanation, '(?im)^知识点[：:][^\r\n]*(?:\r?\n|$)', '', 'g')
WHERE explanation_source = 'ai'
  AND explanation ~* '(?m)^知识点[：:]';

-- +goose Down

-- 重复段落清理不可逆，保留已清理结果。
