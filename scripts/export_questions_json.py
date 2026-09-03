#!/usr/bin/env python3
"""把本地数据库题库导出为 imports 模块接受的结构化 JSON（{"items": [...]}）。

用法：
    .venv/bin/python scripts/export_questions_json.py [输出目录]

- 数据源：PG_URL 环境变量或默认 ai_shuati_dev 库，取每题 current_version。
- 输出：按 级别-科目 分组、每组最多 500 题（与后端导入上限一致）拆成多个文件。
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
CHUNK = 500  # backend internal/imports/json_import.go 限制单批 1-500 题

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
LEFT JOIN source_sections ss ON ss.id = qv.source_section_id
LEFT JOIN sources src ON src.id = ss.source_id
ORDER BY l.code, sub.code, ss.name NULLS LAST, qv.source_order NULLS LAST, q.created_at;
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
        sys.exit("数据库里没有题目")

    # 分组：级别-科目；答案 authority 归一为 official（导入格式硬约束）。
    groups: dict[tuple[str, str], list[dict]] = {}
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
        groups.setdefault((q["levelCode"], q["subjectCode"]), []).append((q, item))

    out_dir.mkdir(parents=True, exist_ok=True)
    total = 0
    manifest = []
    for (level, subject), pairs in sorted(groups.items()):
        for part in range(0, len(pairs), CHUNK):
            chunk = pairs[part:part + CHUNK]
            suffix = f"-{part // CHUNK + 1}" if len(pairs) > CHUNK else ""
            name = f"{level}-{subject}{suffix}.json"
            payload = {"items": [item for _, item in chunk]}
            (out_dir / name).write_text(
                json.dumps(payload, ensure_ascii=False, indent=1), encoding="utf-8")
            total += len(chunk)
            manifest.append({
                "file": name, "count": len(chunk),
                "published": sum(1 for q, _ in chunk if q["status"] == "published"),
                "draft": sum(1 for q, _ in chunk if q["status"] != "published"),
            })

    (out_dir / "manifest.json").write_text(
        json.dumps({"total": total, "files": manifest,
                    "note": "human_verified 答案已按导入格式要求导出为 official"},
                   ensure_ascii=False, indent=1), encoding="utf-8")
    print(f"导出 {total} 题 -> {out_dir}（{len(manifest)} 个文件，human_verified 归一 {human_verified} 题）")
    for m in manifest:
        print(f"  {m['file']}: {m['count']}（发布 {m['published']} / 草稿 {m['draft']}）")


if __name__ == "__main__":
    main()
