# PDF 题库校对数据

`blue_questions.json` 和 `redblue_questions.json` 是本次导入使用的 OCR 结构化快照。
`manual_review.json` 保存已回看原始扫描页、确认属于 PDF 原版而非 OCR 误识别的异常。
`scripts/rebuild_question_bank.py` 会在生成 SQL 前应用已人工核对的异常修正，并对题号、选项、答案、分类和 OCR 噪声做硬校验；校验失败时不会生成可导入数据。

来源 PDF：

- `pdf/蓝宝书N4N5.pdf`
- `pdf/红蓝宝书1000题  新日本语能力考试N4N5文字.词汇.文法 练习+详解 .pdf`
