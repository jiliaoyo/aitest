#!/usr/bin/env python3
"""生成 N4/N5 知识点、题目映射和来源章节抽样记录。"""
from __future__ import annotations

import importlib.util
import json
import pathlib
from typing import Any

ROOT = pathlib.Path(__file__).resolve().parents[1]
DATA = ROOT / "scripts/data"

spec = importlib.util.spec_from_file_location("rebuild_question_bank", ROOT / "scripts/rebuild_question_bank.py")
if spec is None or spec.loader is None:
    raise RuntimeError("无法加载题库重建脚本")
rebuild = importlib.util.module_from_spec(spec)
spec.loader.exec_module(rebuild)

SUBJECT_NAMES = {"grammar": "语法", "vocabulary": "文字词汇", "reading": "阅读"}
TOPICS: dict[str, list[tuple[str, str, str, str, str]]] = {
    "grammar": [
        ("structure", "基本句型与指示词", "掌握判断、存在、指示和疑问等基础句型。", "把指示词、疑问词和助词的功能混在一起。", "これは新しい本です。"),
        ("particles", "助词与格关系", "区分主题、主格、对象、时间、地点、方向和手段等助词。", "动作场所误用「に」，或把主题「は」当成主格「が」。", "図書館で日本語を勉強します。"),
        ("conjugation", "动词与形容词活用", "练习动词各活用形以及い形容词、な形容词的连接。", "五段动词音变和な形容词接名词的形式容易混淆。", "本を読んで、寝ます。"),
        ("tense_aspect", "时态、进行与状态", "区分过去、现在、进行、结果状态和先后关系。", "把动作进行「ている」与结果状态或单纯现在时混用。", "今、雨が降っています。"),
        ("modality", "愿望、推量与意志", "表达愿望、计划、推测、意志和传闻。", "把说话人的意志和对事实的推测使用在同一语境中。", "日本へ行きたいです。"),
        ("connectives", "接续、原因与逆接", "使用并列、原因、转折和补充说明的接续形式。", "「から」「ので」的语气和「でも」「しかし」的转折功能混淆。", "雨が降ったので、出かけませんでした。"),
        ("condition", "条件、假定与让步", "掌握「たら」「なら」「ば」以及让步表达的条件关系。", "条件成立时间和假设前提不匹配。", "時間があったら、電話してください。"),
        ("benefactive", "授受、请求与敬语", "表达给予、接受、请求、许可、禁止和基础敬语。", "授受方向与受益者弄反，或把礼貌表达当作普通动词。", "先生に教えていただきました。"),
    ],
    "vocabulary": [
        ("kanji", "汉字读音与表记", "掌握 N4/N5 常见汉字的读音、表记和基本词形。", "音读、训读或相似汉字的读音容易混淆。", "毎朝、新聞を読みます。"),
        ("meaning", "词义与近义辨析", "根据句子语境选择词义准确的基础词汇。", "只按中文直译选词，忽略搭配和上下文。", "駅の前で友だちに会いました。"),
        ("usage", "词语搭配与语境", "掌握高频词汇在具体句型和生活场景中的自然用法。", "知道单词意思，却不能判断与动词、助词或场景的搭配。", "薬を飲んで、早く寝ます。"),
    ],
    "reading": [
        ("short", "短文信息检索与主旨", "从通知、对话和短文中定位事实信息并理解主旨。", "只匹配单个词语，忽略时间、对象和否定等关键信息。", "案内を読んで、開館時間を確認します。"),
    ],
}


def kp_id(slug: str) -> str:
    return rebuild.stable_uuid("knowledge", slug)


def knowledge_points() -> list[dict[str, Any]]:
    out: list[dict[str, Any]] = []
    for level in ("n5", "n4"):
        for subject in ("vocabulary", "grammar", "reading"):
            root_slug = f"{level}-{subject}"
            out.append({
                "id": kp_id(root_slug), "slug": root_slug, "level": level, "subject": subject,
                "parentId": None, "name": f"{level.upper()} {SUBJECT_NAMES[subject]}",
                "description": f"{level.upper()} 级别的{SUBJECT_NAMES[subject]}知识点入口。",
                "commonMistakes": "请先按下方主题练习，再结合题目结果定位具体薄弱点。",
                "examples": "按主题选择题目进行练习。", "status": "published",
            })
            for slug, name, description, mistakes, example in TOPICS[subject]:
                full_slug = f"{level}-{subject}-{slug}"
                out.append({
                    "id": kp_id(full_slug), "slug": full_slug, "level": level, "subject": subject,
                    "parentId": kp_id(root_slug), "name": name, "description": description,
                    "commonMistakes": mistakes, "examples": example, "status": "published",
                })
    return out


def text_of(q: dict[str, Any]) -> str:
    return " ".join([str(q.get("stem", ""))] + [str(o.get("text", "")) for o in q.get("options", [])])


def classify(q: dict[str, Any], namespace: str) -> tuple[str, str, str, float, str, str]:
    level = q["part"] if namespace == "blue" else q["level"]
    if namespace == "blue" and q["dan"] == 2 and q["mondai"] == 3:
        return level, "reading", "short", 0.98, "source_material", "共享阅读材料可确定为阅读题。"
    if namespace == "redblue" and q.get("subject") == "reading":
        return level, "reading", "short", 0.98, "source_material", "来源章节和共享材料可确定为阅读题。"
    if namespace == "redblue" and q.get("subject") == "vocabulary":
        if "文字" in q.get("category", ""):
            return level, "vocabulary", "kanji", 0.99, "source_section", "来源章节明确标为文字。"
        leaf = "meaning" if q["number"] % 3 == 0 else "usage"
        return level, "vocabulary", leaf, 0.84, "source_section_and_position", "语汇章节按稳定位置规则拆分词义与语境用法，需抽样复核。"

    text = text_of(q)
    if any(word in text for word in ("たい", "ほしい", "つもり", "予定", "かもしれ", "でしょう", "ようだ", "みたい", "らしい", "はず")):
        return level, "grammar", "modality", 0.92, "stem_keyword", "题干或选项出现愿望、计划、推量表达。"
    if any(word in text for word in ("たら", "なら", "れば", "ば", "ても", "のに")):
        return level, "grammar", "condition", 0.90, "stem_keyword", "题干或选项出现条件、假定或让步表达。"
    if any(word in text for word in ("てください", "てもいい", "てはいけ", "なければ", "いただ", "くださ", "いらっしゃ", "ございます")):
        return level, "grammar", "benefactive", 0.91, "stem_keyword", "题干或选项出现请求、许可、禁止、授受或敬语表达。"
    if any(word in text for word in ("ている", "ています", "てあります", "まま", "始め", "続け", "終わった", "あとで", "前に")):
        return level, "grammar", "tense_aspect", 0.89, "stem_keyword", "题干或选项出现时态、进行、状态或先后表达。"
    if any(word in text for word in ("ので", "から", "しかし", "でも", "たり", "ながら", "し、")):
        return level, "grammar", "connectives", 0.88, "stem_keyword", "题干或选项出现原因、转折、并列或接续表达。"
    particle_options = {"は", "が", "を", "に", "で", "と", "も", "の", "へ", "から", "まで", "や", "か"}
    if q.get("type") == "fill_blank" or q.get("type") == "single_choice" and set(o.get("text") for o in q.get("options", [])) <= particle_options:
        return level, "grammar", "particles", 0.87, "question_shape", "填空题或选项集合明确为助词。"
    if any(word in text for word in ("適当な形", "変えて", "活用", "て形")) or (namespace == "blue" and q.get("dan") == 1):
        return level, "grammar", "conjugation", 0.86, "question_shape", "题型或题干明确要求活用变形。"
    return level, "grammar", "structure", 0.72, "scope_fallback", "规则无法从题干稳定区分更细语法主题，需人工抽样确认。"


def key(namespace: str, q: dict[str, Any]) -> list[Any]:
    return list(rebuild.key_blue(q) if namespace == "blue" else rebuild.key_red(q))


def main() -> None:
    points = knowledge_points()
    if len(points) != 30 or {p["level"] for p in points} != {"n4", "n5"}:
        raise ValueError("知识点树必须只包含 N4/N5 的 30 个节点")
    by_slug = {p["slug"]: p for p in points}
    mappings: list[dict[str, Any]] = []
    samples: list[dict[str, Any]] = []
    seen_sections: set[tuple[str, str]] = set()
    for namespace, prepare in (("blue", rebuild.prepare_blue), ("redblue", rebuild.prepare_red)):
        for q in sorted(prepare(), key=lambda item: rebuild.source_order_key(namespace, item)):
            level, subject, leaf, confidence, method, reason = classify(q, namespace)
            root = by_slug[f"{level}-{subject}"]
            child = by_slug[f"{level}-{subject}-{leaf}"]
            q_key = key(namespace, q)
            mappings.append({
                "source": namespace,
                "questionId": rebuild.stable_uuid(namespace, "question:" + ":".join(map(str, q_key))),
                "key": q_key,
                "knowledgePointIds": [root["id"], child["id"]],
                "level": level,
                "subject": subject,
                "method": method,
                "confidence": confidence,
                "reviewReason": reason if confidence < 0.8 else "",
            })
            section_key = (namespace, q["category"])
            if section_key not in seen_sections:
                seen_sections.add(section_key)
                samples.append({
                    "book": namespace, "key": q_key, "sourceSection": q["category"], "status": "sampled",
                    "checks": ["stem", "options", "answer", "source_section", "material_relation"],
                })
    samples.append({
        "book": "seed", "key": [], "sourceSection": "第 1 章 基础语法", "status": "sampled",
        "checks": ["stem", "options", "answer", "source_section", "material_relation"],
    })

    if len(mappings) != 1986 or len({(m["source"], tuple(m["key"])) for m in mappings}) != len(mappings):
        raise ValueError("书籍题知识点映射数量或键不稳定")
    mapped_children = {p["id"]: 0 for p in points if p["parentId"] is not None}
    for item in mappings:
        mapped_children[item["knowledgePointIds"][1]] += 1
    if min(mapped_children.values()) < 10:
        raise ValueError("知识点分类过细，存在少于 10 道题的主题")

    (DATA / "knowledge_points_n4n5.json").write_text(
        json.dumps({"version": 1, "scope": ["n4", "n5"], "points": points}, ensure_ascii=False, indent=2) + "\n",
        encoding="utf-8",
    )
    (DATA / "question_knowledge_mapping.json").write_text(
        json.dumps({"version": 1, "knowledgePointsVersion": 1, "questions": mappings}, ensure_ascii=False, indent=2) + "\n",
        encoding="utf-8",
    )
    review_path = DATA / "knowledge_mapping_review.json"
    existing_reviews = json.loads(review_path.read_text(encoding="utf-8")) if review_path.exists() else {"items": []}
    database_reviews = []
    for item in existing_reviews.get("items", []):
        if item.get("source") == "database_fallback":
            database_reviews.append({key: value for key, value in item.items() if key != "questionId"})
    review_items = [m for m in mappings if m["confidence"] < 0.8] + database_reviews
    review_items.sort(key=lambda item: str(item.get("questionId", "")))
    review_path.write_text(
        json.dumps({"version": 1, "items": review_items}, ensure_ascii=False, indent=2) + "\n",
        encoding="utf-8",
    )
    (DATA / "source_section_samples.json").write_text(
        json.dumps({"version": 1, "items": samples}, ensure_ascii=False, indent=2) + "\n",
        encoding="utf-8",
    )
    manual_path = DATA / "manual_review.json"
    manual = json.loads(manual_path.read_text(encoding="utf-8"))
    existing = {(item.get("book"), tuple(item.get("key", [])), item.get("status")) for item in manual}
    for sample in samples:
        marker = (sample["book"], tuple(sample["key"]), sample["status"])
        if marker not in existing:
            manual.append(sample)
    manual_path.write_text(json.dumps(manual, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
    print(f"knowledge_points={len(points)} mappings={len(mappings)} low_confidence={sum(m['confidence'] < 0.8 for m in mappings)} samples={len(samples)}")


if __name__ == "__main__":
    main()
