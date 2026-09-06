#!/usr/bin/env python3
"""Emit a transactional SQL snapshot for the cleaned blue N1-N3 JSON files."""

from __future__ import annotations

import hashlib
import json
import sys
import uuid
from collections import defaultdict
from pathlib import Path


ROOT = Path(__file__).resolve().parents[1]
INPUT_DIR = ROOT / "questions_json"
LEVELS = ("n1", "n2", "n3")
ALLOWED_KEYS = {
    "rawExcerpt", "materialKey", "type", "stem", "options", "materialTitle",
    "materialContent", "levelCode", "subjectCode", "difficulty",
    "knowledgePointNames", "sourceAnswer", "aiSuggestedAnswer", "anomalies",
}
SOURCE_NAMES = {level: f"蓝宝书{level.upper()}文法" for level in LEVELS}
NAMESPACE = uuid.NAMESPACE_URL


def stable_id(name: str) -> str:
    return str(uuid.uuid5(NAMESPACE, f"https://aishuati.local/blue123/{name}"))


def quote(value: str) -> str:
    return "'" + value.replace("'", "''") + "'"


def jsonb(value: object) -> str:
    return quote(json.dumps(value, ensure_ascii=False, separators=(",", ":"))) + "::jsonb"


def load() -> tuple[dict[str, list[dict]], dict[str, list[tuple[str, dict]]]]:
    by_level: dict[str, list[dict]] = {}
    by_file: dict[str, list[tuple[str, dict]]] = defaultdict(list)
    expected = {"n1": 1000, "n2": 925, "n3": 995}
    for level in LEVELS:
        files = sorted(INPUT_DIR.glob(f"蓝宝书{level.upper()}文法-*.json"))
        if not files:
            raise SystemExit(f"缺少 {level.upper()} JSON 分片")
        items: list[dict] = []
        for path in files:
            payload = json.loads(path.read_text(encoding="utf-8"))
            if set(payload) != {"items"} or not isinstance(payload["items"], list):
                raise SystemExit(f"JSON 根结构不合法: {path.name}")
            for q in payload["items"]:
                if set(q) != ALLOWED_KEYS:
                    raise SystemExit(f"字段不合法: {path.name}: {sorted(set(q) - ALLOWED_KEYS)}")
                validate(q, path.name)
                items.append(q)
                by_file[path.name].append((level, q))
        if len(items) != expected[level]:
            raise SystemExit(f"{level.upper()} 题数为 {len(items)}，预期 {expected[level]}")
        by_level[level] = items
    return by_level, by_file


def validate(q: dict, file_name: str) -> None:
    if q.get("levelCode") not in LEVELS or q.get("subjectCode") != "grammar":
        raise SystemExit(f"分类不合法: {file_name}")
    if q.get("type") not in {"single_choice", "multiple_choice", "fill_blank", "short_answer"}:
        raise SystemExit(f"题型不合法: {file_name}")
    if not str(q.get("stem", "")).strip() or not isinstance(q.get("difficulty"), int) or not 1 <= q["difficulty"] <= 5:
        raise SystemExit(f"题干或难度不合法: {file_name}")
    answer = q.get("sourceAnswer")
    if not isinstance(answer, dict) or answer.get("authority") != "official":
        raise SystemExit(f"缺少 official 答案: {file_name}")
    if q["type"] in {"single_choice", "multiple_choice"}:
        options = q.get("options") or []
        ids = [o.get("id") for o in options]
        if len(ids) < 4 or ids != list("abcdefghijklmnopqrstuvwxyz"[:len(ids)]):
            raise SystemExit(f"选项不合法: {file_name}")
        answer_ids = answer.get("value", {}).get("optionIds", [])
        if not answer_ids or any(option_id not in ids for option_id in answer_ids):
            raise SystemExit(f"答案引用不合法: {file_name}")
        if q["type"] == "single_choice" and len(answer_ids) != 1:
            raise SystemExit(f"单选答案不唯一: {file_name}")
    elif not (answer.get("value", {}).get("acceptable") or answer.get("value", {}).get("reference")):
        raise SystemExit(f"文字答案为空: {file_name}")


def unit_for(level: str, position: int) -> int:
    return (position - 1) // 50 + 1


def subject_id_sql(level: str) -> str:
    return "(SELECT s.id FROM subjects s JOIN exams e ON e.id = s.exam_id WHERE e.code = 'jlpt' AND s.code = 'grammar')"


def level_id_sql(level: str) -> str:
    return f"(SELECT l.id FROM exam_levels l JOIN exams e ON e.id = l.exam_id WHERE e.code = 'jlpt' AND l.code = {quote(level)})"


def main() -> None:
    by_level, by_file = load()
    lines = [
        "-- 由清理后的蓝宝书 N1/N2/N3 JSON 生成；仅新增这些来源，不删除既有数据。",
        "BEGIN;",
        "DO $$ BEGIN IF NOT EXISTS (SELECT 1 FROM users WHERE role = 'admin') THEN RAISE EXCEPTION '缺少管理员用户'; END IF; END $$;",
    ]
    for level in LEVELS:
        source_id = stable_id(f"{level}:source")
        source_name = SOURCE_NAMES[level]
        lines.append(f"SELECT pg_advisory_xact_lock(hashtext({quote('blue123:' + level)}));")
        lines.append(
            "DO $$ BEGIN IF EXISTS (SELECT 1 FROM sources WHERE name = "
            f"{quote(source_name)} AND id <> {quote(source_id)}::uuid) THEN "
            f"RAISE EXCEPTION '来源名称已存在: {source_name}'; END IF; END $$;"
        )
        lines.append(
            "INSERT INTO sources (id, name, kind, author, publisher, license_note, internal_note, created_by) VALUES "
            f"({quote(source_id)}::uuid, {quote(source_name)}, 'book', '许小明', '华东理工大学出版社', "
            f"'用户提供的本地 PDF；请确认版权与使用授权。', '蓝宝书 N1/N2/N3 文法；题目经 OCR 清洗与原图核对后导入，异常仍保留在 JSON 文件。', "
            "(SELECT id FROM users WHERE role = 'admin' ORDER BY created_at, id LIMIT 1)) ON CONFLICT (id) DO NOTHING;"
        )
        max_unit = max(unit_for(level, len(by_level[level])), 1)
        section_ids = {}
        for unit in range(1, max_unit + 1):
            section_id = stable_id(f"{level}:section:{unit:02d}")
            section_ids[unit] = section_id
            lines.append(
                "INSERT INTO source_sections (id, source_id, name, sort_order) VALUES "
                f"({quote(section_id)}::uuid, {quote(source_id)}::uuid, {quote(f'{level.upper()}·第{unit:02d}单元·文法')}, {unit}) "
                "ON CONFLICT (id) DO NOTHING;"
            )

        materials: dict[tuple[str, str], tuple[str, str]] = {}
        for q in by_level[level]:
            title, content = str(q.get("materialTitle") or ""), str(q.get("materialContent") or "")
            if content:
                materials[(title, content)] = (title, content)
        material_versions = {}
        for (title, content), value in materials.items():
            digest = hashlib.sha256((title + "\0" + content).encode()).hexdigest()
            material_id = stable_id(f"{level}:material:{digest}")
            version_id = stable_id(f"{level}:material-version:{digest}")
            material_versions[(title, content)] = version_id
            lines.append(
                "INSERT INTO materials (id, created_by) VALUES "
                f"({quote(material_id)}::uuid, (SELECT id FROM users WHERE role = 'admin' ORDER BY created_at, id LIMIT 1)) ON CONFLICT (id) DO NOTHING;"
            )
            lines.append(
                "INSERT INTO material_versions (id, material_id, version_no, title, content, created_by) VALUES "
                f"({quote(version_id)}::uuid, {quote(material_id)}::uuid, 1, {quote(title)}, {quote(content)}, "
                "(SELECT id FROM users WHERE role = 'admin' ORDER BY created_at, id LIMIT 1)) ON CONFLICT (id) DO NOTHING;"
            )
            lines.append(f"UPDATE materials SET current_version_id = {quote(version_id)}::uuid WHERE id = {quote(material_id)}::uuid;")

        for position, q in enumerate(by_level[level], 1):
            unit = unit_for(level, position)
            question_id = stable_id(f"{level}:question:{position:04d}")
            version_id = stable_id(f"{level}:version:{position:04d}")
            content_key = (str(q.get("materialTitle") or ""), str(q.get("materialContent") or ""))
            material_version = material_versions.get(content_key)
            material_sql = "NULL" if not material_version else f"{quote(material_version)}::uuid"
            lines.append(
                "INSERT INTO questions (id, status, has_answer, created_by) VALUES "
                f"({quote(question_id)}::uuid, 'published', true, (SELECT id FROM users WHERE role = 'admin' ORDER BY created_at, id LIMIT 1)) ON CONFLICT (id) DO NOTHING;"
            )
            lines.append(
                "INSERT INTO question_versions (id, question_id, version_no, source_order, type, stem, material_version_id, options, level_id, subject_id, source_section_id, difficulty, created_by) VALUES "
                f"({quote(version_id)}::uuid, {quote(question_id)}::uuid, 1, {position}, {quote(q['type'])}, {quote(q['stem'])}, {material_sql}, "
                f"{jsonb(q['options'])}, {level_id_sql(level)}, {subject_id_sql(level)}, {quote(section_ids[unit])}::uuid, {q['difficulty']}, "
                "(SELECT id FROM users WHERE role = 'admin' ORDER BY created_at, id LIMIT 1)) ON CONFLICT (id) DO NOTHING;"
            )
            lines.append(
                "INSERT INTO answer_keys (question_version_id, value, authority, explanation, created_by) VALUES "
                f"({quote(version_id)}::uuid, {jsonb(q['sourceAnswer']['value'])}, 'official', "
                f"{quote(q['sourceAnswer'].get('explanation', ''))}, (SELECT id FROM users WHERE role = 'admin' ORDER BY created_at, id LIMIT 1)) "
                "ON CONFLICT (question_version_id) DO NOTHING;"
            )
            lines.append(
                "UPDATE questions SET current_version_id = "
                f"{quote(version_id)}::uuid, published_version_id = {quote(version_id)}::uuid, status = 'published', "
                f"published_at = COALESCE(published_at, now()), updated_at = now() WHERE id = {quote(question_id)}::uuid AND current_version_id IS NULL;"
            )

    lines.append("COMMIT;")
    sys.stdout.write("\n".join(lines) + "\n")
    print(f"-- blue123 SQL: {sum(map(len, by_level.values()))} questions", file=sys.stderr)


if __name__ == "__main__":
    main()
