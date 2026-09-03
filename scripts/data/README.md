# PDF 题库校对数据

`blue_questions.json` 和 `redblue_questions.json` 是本次导入使用的 OCR 结构化快照。
`manual_review.json` 保存已回看原始扫描页、确认属于 PDF 原版而非 OCR 误识别的异常。
`scripts/rebuild_question_bank.py` 会在生成 SQL 前应用已人工核对的异常修正，并对题号、选项、答案、分类和 OCR 噪声做硬校验；校验失败时不会生成可导入数据。

来源 PDF：

- `pdf/蓝宝书N4N5.pdf`
- `pdf/红蓝宝书1000题  新日本语能力考试N4N5文字.词汇.文法 练习+详解 .pdf`
# 题库数据与知识点映射

`blue_questions.json` 和 `redblue_questions.json` 是 OCR 清洗后的题目事实源，不能单独替代生成后的 SQL。

知识点相关文件由仓库根目录执行 `python3 scripts/generate_knowledge_data.py` 生成：

- `knowledge_points_n4n5.json`：只包含 N4/N5 的知识点树及正文。
- `question_knowledge_mapping.json`：两套书籍题的确定性映射、方法和置信度。
- `knowledge_mapping_review.json`：低置信度或数据库兜底映射，需人工确认。
- `source_section_samples.json`：每个来源章节的稳定抽样记录。
- `manual_review.json`：人工确认与来源章节抽样记录。

题库 SQL 由 `python3 scripts/rebuild_question_bank.py` 生成。已有数据库使用 `cd backend && make backfill-knowledge` 幂等回填；它会为需要补分类的发布题复制新题目版本和权威答案，再切换发布指针，不删除练习记录。

旧版本若没有知识点关系，不根据历史作答事实伪造映射；用户使用带可靠知识点的新版本题目后，才开始积累新的知识点统计。退休知识点只保留历史缓存，不再出现在学习端推荐和专项练习中。
