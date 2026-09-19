-- +goose Up

-- ponytail: 只处理明显缺少中文说明词的整段日语，混合中文说明与日语例句的解析保留。
UPDATE ai_generated_question_answers
SET explanation = 'AI 解析语言异常，已留待重新分析。'
WHERE explanation ~ '[ぁ-ゖァ-ヺ]'
  AND explanation !~ '(根据|本题|题干|表示|因为|所以|因此|用于|误用|选择|选项|语法|词义|符合|不能|正确|错误|这里|说明|区别|意思|接在|接续|解析|句意|注意|场景|动作|时间|地点|助词|动词|形容词|名词|连接)';

UPDATE grading_results
SET status = 'failed',
    explanation = 'AI 解析语言异常，已留待重新分析。',
    explanation_source = 'ai',
    updated_at = now()
WHERE source = 'ai'
  AND explanation_source = 'ai'
  AND status IN ('correct', 'incorrect', 'unanswered', 'failed')
  AND explanation ~ '[ぁ-ゖァ-ヺ]'
  AND explanation !~ '(根据|本题|题干|表示|因为|所以|因此|用于|误用|选择|选项|语法|词义|符合|不能|正确|错误|这里|说明|区别|意思|接在|接续|解析|句意|注意|场景|动作|时间|地点|助词|动词|形容词|名词|连接)';

-- +goose Down

-- 异常文本已被替换，无法安全恢复原始 AI 输出。
