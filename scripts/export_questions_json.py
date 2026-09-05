#!/usr/bin/env python3
"""把本地数据库题库导出为 imports 模块接受的结构化 JSON（{"items": [...]}）。

用法：
    .venv/bin/python scripts/export_questions_json.py [输出目录]

- 数据源：PG_URL 环境变量或默认 ai_shuati_dev 库，取每题 current_version。
- 输出：只导出书籍来源，每本书一个 JSON 文件。
- 约定：human_verified 答案导出为 official（导入格式只接受 official）；
  无答案题目 sourceAnswer 为 null；materialKey 用材料 ID 以便重新导入时共享材料。
"""

import json
import os
import re
import subprocess
import sys
from pathlib import Path

REPO = Path(__file__).resolve().parent.parent
DEFAULT_OUT = REPO / "questions_json"
QUERY = r"""
SELECT json_build_object(
  'questionId', q.id,
  'status', q.status,
  'type', qv.type,
  'stem', qv.stem,
  'options', COALESCE(qv.options, '[]'::jsonb),
  'levelCode', l.code,
  'subjectCode', sub.code,
  'difficulty', qv.difficulty,
  'materialKey', COALESCE(mv.material_id::text, ''),
  'materialTitle', COALESCE(mv.title, ''),
  'materialContent', COALESCE(mv.content, ''),
  'sourceName', COALESCE(src.name, ''),
  'sectionName', COALESCE(ss.name, ''),
  'knowledgePointNames', COALESCE((
      SELECT json_agg(kp.name ORDER BY kp.name)
      FROM question_version_knowledge_points qvkp
      JOIN knowledge_points kp ON kp.id = qvkp.knowledge_point_id
      WHERE qvkp.question_version_id = qv.id), '[]'::json),
  'sourceAnswer', (
      SELECT json_build_object('value', ak.value, 'authority', ak.authority,
                               'explanation', ak.explanation)
      FROM answer_keys ak WHERE ak.question_version_id = qv.id)
)
FROM questions q
JOIN question_versions qv ON qv.id = q.current_version_id
JOIN exam_levels l ON l.id = qv.level_id
JOIN subjects sub ON sub.id = qv.subject_id
LEFT JOIN material_versions mv ON mv.id = qv.material_version_id
JOIN source_sections ss ON ss.id = qv.source_section_id
JOIN sources src ON src.id = ss.source_id AND src.kind = 'book'
ORDER BY src.name, ss.name, qv.source_order NULLS LAST, q.created_at;
"""


def main() -> None:
    out_dir = Path(sys.argv[1]) if len(sys.argv) > 1 else DEFAULT_OUT
    pg_url = os.environ.get("PG_URL", "ai_shuati_dev")
    rows = subprocess.run(
        ["psql", pg_url, "-At", "-c", QUERY],
        check=True, capture_output=True, text=True,
    ).stdout.splitlines()

    questions = [json.loads(line) for line in rows if line.strip()]
    if not questions:
        sys.exit("数据库里没有书籍题目")

    # 分组：书籍来源；答案 authority 归一为 official（导入格式硬约束）。
    groups: dict[str, list[dict]] = {}
    human_verified = 0
    for q in questions:
        answer = q["sourceAnswer"]
        if answer is not None:
            if answer["authority"] == "human_verified":
                human_verified += 1
            answer["authority"] = "official"
        item = {
            "rawExcerpt": "",
            "materialKey": q["materialKey"],
            "type": q["type"],
            "stem": q["stem"],
            "options": q["options"],
            "materialTitle": q["materialTitle"],
            "materialContent": q["materialContent"],
            "levelCode": q["levelCode"],
            "subjectCode": q["subjectCode"],
            "difficulty": q["difficulty"],
            "knowledgePointNames": q["knowledgePointNames"],
            "sourceAnswer": answer,
            "aiSuggestedAnswer": None,
            "anomalies": [],
        }
        groups.setdefault(q["sourceName"], []).append(item)

    out_dir.mkdir(parents=True, exist_ok=True)
    for old_file in out_dir.glob("*.json"):
        old_file.unlink()

    total = 0
    for source_name, items in sorted(groups.items()):
        name = re.sub(r'[\\/:*?"<>|]', "_", source_name).strip() or "未命名书籍"
        (out_dir / f"{name}.json").write_text(
            json.dumps({"items": items}, ensure_ascii=False, indent=1), encoding="utf-8")
        total += len(items)
        print(f"  {name}.json: {len(items)}")

    print(f"导出 {total} 题 -> {out_dir}（{len(groups)} 本书，human_verified 归一 {human_verified} 题）")


if __name__ == "__main__":
    main()
