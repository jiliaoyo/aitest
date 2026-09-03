#!/usr/bin/env python3
"""分五个等级批次生成 JLPT N5-N1 的知识点候选，不直接发布到学习端。"""
from __future__ import annotations

import json
import os
import pathlib
import re
import time
import urllib.request
import uuid
from typing import Any

ROOT = pathlib.Path(__file__).resolve().parents[1]
DATA = ROOT / "scripts/data"
OUT = DATA / "knowledge_points_ai_batches"
LEVELS = ("n5", "n4", "n3", "n2", "n1")
SUBJECTS = ("grammar", "vocabulary", "reading")
TARGETS = {
    "n5": {"grammar": 50, "vocabulary": 50, "reading": 20},
    "n4": {"grammar": 70, "vocabulary": 70, "reading": 25},
    "n3": {"grammar": 90, "vocabulary": 90, "reading": 35},
    "n2": {"grammar": 110, "vocabulary": 110, "reading": 45},
    "n1": {"grammar": 130, "vocabulary": 130, "reading": 55},
}
KEY_RE = re.compile(r"^[a-z][a-z0-9_]{1,80}$")
CHUNK_SIZE = 20


def load_env() -> None:
    path = ROOT / "backend/.env"
    if not path.exists():
        return
    for line in path.read_text(encoding="utf-8").splitlines():
        line = line.strip()
        if not line or line.startswith("#"):
            continue
        key, separator, value = line.partition("=")
        if separator and key and key not in os.environ:
            os.environ[key] = value.strip().strip("'\"")


def stable_id(slug: str) -> str:
    return str(uuid.uuid5(uuid.NAMESPACE_URL, f"https://aishuati.local/knowledge/{slug}"))


def read_existing() -> list[dict[str, Any]]:
    path = DATA / "knowledge_points_n4n5.json"
    if not path.exists():
        return []
    return json.loads(path.read_text(encoding="utf-8")).get("points", [])


def prompt_for(level: str, subject: str, existing: list[dict[str, Any]], count: int, batch_no: int, batch_total: int) -> str:
    existing_lines = "\n".join(
        f"- {point['name']}"
        for point in existing
        if point.get("level") == level and point.get("subject") == subject
    )
    target = TARGETS[level][subject]
    subject_name = {"grammar": "语法", "vocabulary": "文字词汇", "reading": "阅读"}[subject]
    return f"""你是 JLPT 课程体系编辑。请只生成 {level.upper()} 级“{subject_name}”科的待审核知识点候选。

重要边界：JLPT 没有官方公开的一份固定、穷尽的知识点清单；你的输出是基于常见教材和考试范围整理的候选，不得声称是官方清单。知识点要足够细，能够用于学习统计和专项练习，但不要把每个单词机械列成知识点；文字词汇应覆盖汉字读音/表记、词义、近义辨析、搭配、词族和语境，阅读应覆盖不同文本、定位、主旨、推断、态度和结构能力。

这是 {batch_total} 个子批次中的第 {batch_no} 批；本批必须恰好生成 {count} 个 {subject_name} 知识点，不要生成其他科目。该科目最终目标约为 {target} 个。

已有知识点（不要重复这些名称；它们会与本批结果合并）：
{existing_lines}

生成规则：
1. 每项是一个原子、可教授、可统计的知识点，不要生成章节名、题型名、学习建议或空泛的“综合能力”。
2. 语法尽量覆盖具体句型、接续、功能、限制条件和易混表达；词汇覆盖可区分的语义/用法群；阅读覆盖可单独练习的阅读策略。
3. 所有项的 level 必须是 {level}，subject 必须是 {subject}。
4. key 必须是唯一的 ASCII 小写 snake_case，不超过 80 个字符；不要在 key 中写级别和科目前缀。
5. description、commonMistakes、examples 都必须是有实际内容的中文；examples 给出日语例句并简短说明。
6. 不要编造官方编号、官方词表或“全部覆盖”的断言；不要重复已有项或本批项。
7. 只输出合法 JSON，不要 Markdown、解释或额外字段。必须包含字符串 "json"。

输出格式：
{{"level":"{level}","subject":"{subject}","points":[{{"subject":"{subject}","key":"example_key","name":"知识点名称","description":"说明","commonMistakes":"常见误区","examples":"日语例句：……；说明：……"}}]}}"""


def call_ai(level: str, subject: str, existing: list[dict[str, Any]], count: int, batch_no: int, batch_total: int, attempt: int) -> dict[str, Any]:
    base_url = os.environ.get("AI_BASE_URL", "").rstrip("/")
    api_key = os.environ.get("AI_API_KEY", "")
    model = os.environ.get("AI_MODEL", "")
    if not base_url or not api_key or not model:
        raise RuntimeError("缺少 AI_BASE_URL、AI_API_KEY 或 AI_MODEL")
    payload = {
        "model": model,
        "messages": [
            {"role": "system", "content": prompt_for(level, subject, existing, count, batch_no, batch_total)},
            {"role": "user", "content": "请输出本批 JSON。" if attempt == 1 else "上一次输出未通过校验，请重新完整输出本批 JSON；务必避开已有知识点名称和 key，不要重复。"},
        ],
        "thinking": {"type": "disabled"},
        "response_format": {"type": "json_object"},
        "max_tokens": 16384,
        "temperature": 0.4 if attempt == 1 else 0.2,
    }
    req = urllib.request.Request(
        f"{base_url}/chat/completions",
        data=json.dumps(payload, ensure_ascii=False).encode("utf-8"),
        headers={"Content-Type": "application/json", "Authorization": f"Bearer {api_key}"},
        method="POST",
    )
    timeout = float(os.environ.get("AI_TIMEOUT", "300s").removesuffix("s"))
    with urllib.request.urlopen(req, timeout=timeout) as response:
        body = json.load(response)
    content = body["choices"][0]["message"]["content"]
    return json.loads(content)


def validate_batch(level: str, subject: str, raw: dict[str, Any], existing: list[dict[str, Any]], expected_count: int, batch_no: int) -> list[dict[str, Any]]:
    if raw.get("level") != level or raw.get("subject") != subject or not isinstance(raw.get("points"), list):
        raise ValueError("顶层 level 或 points 不合法")
    existing_names = {str(point.get("name", "")).strip() for point in existing if point.get("level") == level and point.get("subject") == subject}
    existing_slugs = {str(point.get("slug", "")).strip() for point in existing if point.get("level") == level and point.get("subject") == subject}
    seen_keys: set[str] = set()
    seen_names: set[str] = set()
    output: list[dict[str, Any]] = []
    for item in raw["points"]:
        if not isinstance(item, dict):
            raise ValueError("知识点项不是对象")
        item_subject = item.get("subject")
        key = str(item.get("key", "")).strip()
        if not key and str(item.get("slug", "")).startswith(f"{level}-{subject}-"):
            key = str(item["slug"])[len(f"{level}-{subject}-"):]
        name = str(item.get("name", "")).strip()
        slug = f"{level}-{item_subject}-{key}"
        if item_subject != subject or not KEY_RE.fullmatch(key):
            raise ValueError(f"知识点科目或 key 不合法: {item}")
        if key in seen_keys or name in seen_names or name in existing_names or slug in existing_slugs:
            continue
        if len(name) < 2 or any(not str(item.get(field, "")).strip() for field in ("description", "commonMistakes", "examples")):
            raise ValueError(f"知识点内容不完整: {name}")
        if len(name) > 80 or len(str(item["description"])) > 500 or len(str(item["commonMistakes"])) > 500 or len(str(item["examples"])) > 500:
            raise ValueError(f"知识点文本过长: {name}")
        seen_keys.add(key)
        seen_names.add(name)
        output.append({
            "id": stable_id(slug), "slug": slug, "level": level, "subject": item_subject,
            "parentId": stable_id(f"{level}-{item_subject}"), "name": name,
            "description": str(item["description"]).strip(),
            "commonMistakes": str(item["commonMistakes"]).strip(),
            "examples": str(item["examples"]).strip(), "status": "draft",
            "source": "ai_candidate", "sourceBatch": f"{level}_{subject}",
        })
    if len(output) > expected_count:
        raise ValueError(f"{subject} 第 {batch_no} 批数量超出: 需要 {expected_count}，实际 {len(output)}")
    return output


def main() -> None:
    load_env()
    existing = read_existing()
    OUT.mkdir(parents=True, exist_ok=True)
    manifest = {"version": 1, "scope": list(LEVELS), "subjects": list(SUBJECTS), "status": "draft", "batches": []}
    for level in LEVELS:
        for subject in SUBJECTS:
            target = TARGETS[level][subject]
            batch_total = (target + CHUNK_SIZE - 1) // CHUNK_SIZE
            generated: list[dict[str, Any]] = []
            batch_no = 1
            while len(generated) < target:
                count = min(CHUNK_SIZE, target - len(generated))
                path = OUT / f"{level}_{subject}_{batch_no:02d}.json"
                if path.exists():
                    saved = json.loads(path.read_text(encoding="utf-8"))
                    saved_count = len(saved.get("points", []))
                    points = validate_batch(level, subject, saved, existing + generated, saved_count, batch_no)
                    if len(points) == saved_count:
                        generated.extend(points)
                        manifest["batches"].append({"level": level, "subject": subject, "batch": batch_no, "file": path.name, "count": len(points)})
                        print(f"{level}/{subject}/{batch_no:02d}: {len(points)} points (已存在)", flush=True)
                        batch_no += 1
                        continue
                    print(f"{level}/{subject}/{batch_no:02d}: 已有文件含重复项，重新生成", flush=True)
                collected: list[dict[str, Any]] = []
                last_error = ""
                for attempt in range(1, 9):
                    try:
                        known = existing + generated + collected
                        remaining = count - len(collected)
                        raw = call_ai(level, subject, known, remaining, batch_no, batch_total, attempt)
                        points = validate_batch(level, subject, raw, known, remaining, batch_no)
                        if not points:
                            raise ValueError("本批没有新的知识点")
                        collected.extend(points)
                        print(f"{level}/{subject}/{batch_no:02d}: {len(collected)}/{count} points", flush=True)
                        if len(collected) == count:
                            break
                    except Exception as exc:  # noqa: BLE001 - 单批失败后重试一次并保留明确错误
                        last_error = str(exc)
                        time.sleep(2 * attempt)
                if not collected:
                    raise RuntimeError(f"{level}/{subject}/{batch_no:02d} 批次生成失败: {last_error}")
                path.write_text(json.dumps({"version": 1, "level": level, "subject": subject, "batch": batch_no, "status": "draft", "source": "DeepSeek", "points": collected}, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
                generated.extend(collected)
                manifest["batches"].append({"level": level, "subject": subject, "batch": batch_no, "file": path.name, "count": len(collected)})
                print(f"{level}/{subject}/{batch_no:02d}: {len(collected)}/{count} points 已保存", flush=True)
                batch_no += 1
    manifest["batches"] = []
    for path in sorted(OUT.glob("n[1-5]_*.json")):
        data = json.loads(path.read_text(encoding="utf-8"))
        manifest["batches"].append({
            "level": data["level"], "subject": data["subject"], "batch": data["batch"],
            "file": path.name, "count": len(data["points"]),
        })
    (OUT / "manifest.json").write_text(json.dumps(manifest, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")


if __name__ == "__main__":
    main()
