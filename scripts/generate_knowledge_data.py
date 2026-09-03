#!/usr/bin/env python3
"""生成 N4/N5 知识点、题目映射和来源章节抽样记录。"""
from __future__ import annotations

import importlib.util
import json
import pathlib
import re
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


def correct_option_text(q: dict[str, Any]) -> str:
    answer = q.get("answer") or {}
    ids = answer.get("optionIds") or []
    if not ids:
        return ""
    wanted = str(ids[0]).lower()
    for option in q.get("options", []):
        if str(option.get("id", "")).lower() == wanted:
            return str(option.get("text", ""))
    return ""


def contains_form(text: str, patterns: tuple[str, ...]) -> bool:
    return any(re.search(pattern, text) for pattern in patterns)


def grammar_child(q: dict[str, Any]) -> tuple[str, str, float, str] | None:
    """只用题干和官方正确选项判断语法细类，不读取干扰项。"""
    stem = str(q.get("stem", ""))
    correct = correct_option_text(q)
    # 模拟题 OCR 的 stem 可能拼入相邻题目，只信任当前题的官方正确选项。
    text = correct if str(q.get("kind", "")).startswith("mock") else f"{stem} {correct}"
    if contains_form(text, (r"適当な形", r"適切な形", r"変えて", r"活用", r"て形", r"表の中")):
        return "conjugation", "stem", 0.98, "题干明确要求活用、变形或填写活用表。"
    if contains_form(text, (
        r"たい(?:です|だ|と思|んです|[。、]|$)", r"ほしい(?:です|だ|[。、]|$)",
        r"つもり(?:です|だ|で|に|[。、]|$)", r"予定(?:です|だ|で|に|[。、]|$)",
        r"かもしれ(?:ない|ません)", r"でしょう", r"よう(?:だ|です|に|な)",
        r"みたい(?:だ|です|に|な)", r"らしい(?:です|だ|[。、]|$)",
        r"はず(?:だ|です|が|の|[。、]|$)", r"と思(?:い|っ)")):
        return "modality", "stem_or_correct_answer", 0.94, "题干或官方正确选项明确出现愿望、计划或推量形式。"
    if contains_form(text, (
        r"(?:たら|なら|れば|なければ|ても|のに)(?![ぁ-んァ-ン])",
        r"ばいい", r"ならば")):
        return "condition", "stem_or_correct_answer", 0.94, "题干或官方正确选项明确出现条件、假定或让步形式。"
    if contains_form(text, (
        r"てください", r"てもいい", r"てはいけ", r"なければなら",
        r"いただ", r"ください", r"いらっしゃ", r"ございます")):
        return "benefactive", "stem_or_correct_answer", 0.94, "题干或官方正确选项明确出现请求、许可、授受或敬语形式。"
    if contains_form(text, (
        r"ている", r"ています", r"てある", r"まま", r"始め", r"続け",
        r"終わった", r"あとで", r"前に")):
        return "tense_aspect", "stem_or_correct_answer", 0.92, "题干或官方正确选项明确出现进行、状态或先后形式。"
    if contains_form(text, (
        r"ので(?:です|だ|[、。]|$)", r"から(?:です|だ|[、。]|$)",
        r"しかし", r"でも", r"たり", r"ながら", r"し、")):
        return "connectives", "stem_or_correct_answer", 0.92, "题干或官方正确选项明确出现原因、转折、并列或接续形式。"
    if correct in {"は", "が", "を", "に", "で", "と", "も", "の", "へ", "から", "まで", "や", "か"} and re.search(r"[_＿（(]", stem):
        return "particles", "correct_answer_and_blank", 0.90, "题干有填空位置，官方正确选项是明确的助词。"
    if correct in {"これ", "それ", "あれ", "どれ", "ここ", "そこ", "あそこ", "どこ", "この", "その", "あの", "どの", "こちら", "そちら", "あちら", "どちら", "こんな", "そんな", "あんな", "どんな", "だれ", "どなた"} and re.search(r"[_＿（(]", stem):
        return "structure", "correct_answer_and_blank", 0.90, "题干有填空位置，官方正确选项是指示词或疑问词。"
    return None


def classify(q: dict[str, Any], namespace: str) -> dict[str, Any]:
    level = q["part"] if namespace == "blue" else q["level"]
    if namespace == "blue" and q["dan"] == 2 and q["mondai"] == 3:
        return {"level": level, "subject": "reading", "leaf": "short", "confidence": 0.98,
                "method": "source_material", "basis": "共享阅读材料可确定为阅读题。"}
    if namespace == "redblue" and q.get("subject") == "reading":
        return {"level": level, "subject": "reading", "leaf": "short", "confidence": 0.98,
                "method": "source_section", "basis": "来源章节和共享材料可确定为阅读题。"}
    if namespace == "redblue" and q.get("subject") == "vocabulary":
        if "文字" in q.get("category", ""):
            return {"level": level, "subject": "vocabulary", "leaf": "kanji", "confidence": 0.99,
                    "method": "source_section", "basis": "来源章节明确标为文字。"}
        if "語彙" in q.get("category", ""):
            return {"level": level, "subject": "vocabulary", "leaf": "usage", "confidence": 0.95,
                    "method": "source_section", "basis": "来源章节明确标为语汇用法。"}

    child = grammar_child(q) if q.get("subject") == "grammar" else None
    if child:
        leaf, method, confidence, basis = child
        if leaf == "conjugation":
            return {"level": level, "subject": q["subject"], "leaf": None, "confidence": 1,
                    "method": "scope_root", "basis": "活用题在当前来源中的样本不足，正式映射只保留级别和科目根节点。",
                    "suggestion": leaf, "suggestedConfidence": confidence,
                    "reviewReason": "活用细分类当前样本不足 10 题，需人工确认后再用于专项练习。"}
        return {"level": level, "subject": q["subject"], "leaf": leaf, "confidence": confidence,
                "method": method, "basis": basis}

    # 细分类无法从来源、题干和官方答案可靠确定时，只发布级别/科目根节点。
    suggestion = None
    if q.get("subject") == "grammar" and q.get("type") == "single_choice":
        suggestion = "structure"
    elif q.get("subject") == "vocabulary":
        suggestion = "meaning"
    result = {"level": level, "subject": q["subject"], "leaf": None, "confidence": 1,
              "method": "scope_root", "basis": "来源只能可靠确定级别和科目，细分类保留为待复核建议。"}
    if suggestion:
        result.update({"suggestion": suggestion, "suggestedConfidence": 0.6,
                       "reviewReason": "细分类缺少明确来源或题干依据，不能直接写入正式映射。"})
    return result


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
            classified = classify(q, namespace)
            level = classified["level"]
            subject = classified["subject"]
            root = by_slug[f"{level}-{subject}"]
            child = by_slug.get(f"{level}-{subject}-{classified['leaf']}") if classified.get("leaf") else None
            q_key = key(namespace, q)
            formal_ids = [root["id"]] + ([child["id"]] if child else [])
            suggestion_fields = {}
            if classified.get("suggestion"):
                suggestion_fields = {
                    "suggestedKnowledgePointIds": [by_slug[f"{level}-{subject}-{classified['suggestion']}"]["id"]],
                    "suggestedConfidence": classified["suggestedConfidence"],
                    "suggestedReviewStatus": "pending",
                }
            mapping_item = {
                "source": namespace,
                "questionId": rebuild.stable_uuid(namespace, "question:" + ":".join(map(str, q_key))),
                "key": q_key,
                "level": level,
                "subject": subject,
                "knowledgePointIds": formal_ids,
                "method": classified["method"],
                "confidence": classified["confidence"],
                **suggestion_fields,
                "reviewStatus": "not_required",
                **({"reviewReason": classified["reviewReason"]} if classified.get("reviewReason") else {}),
                "basis": classified["basis"],
            }
            mappings.append(mapping_item)
            section_key = (namespace, q["category"])
            if section_key not in seen_sections:
                seen_sections.add(section_key)
                samples.append({
                    "book": namespace, "key": q_key, "sourceSection": q["category"], "status": "pending",
                    "checks": ["stem", "options", "answer", "source_section", "material_relation"],
                })
    samples.append({
        "book": "seed", "key": [], "sourceSection": "第 1 章 基础语法", "status": "pending",
        "checks": ["stem", "options", "answer", "source_section", "material_relation"],
    })

    if len(mappings) != 1986 or len({(m["source"], tuple(m["key"])) for m in mappings}) != len(mappings):
        raise ValueError("书籍题知识点映射数量或键不稳定")
    mapped_children = {p["id"]: 0 for p in points if p["parentId"] is not None}
    for item in mappings:
        for point_id in item["knowledgePointIds"][1:]:
            mapped_children[point_id] += 1
    used_children = [count for count in mapped_children.values() if count]
    if used_children and min(used_children) < 10:
        raise ValueError("知识点分类过细，存在少于 10 道题的主题")

    (DATA / "knowledge_points_n4n5.json").write_text(
        json.dumps({"version": 1, "scope": ["n4", "n5"], "points": points}, ensure_ascii=False, indent=2) + "\n",
        encoding="utf-8",
    )
    (DATA / "question_knowledge_mapping.json").write_text(
        json.dumps({"version": 2, "knowledgePointsVersion": 1, "questions": mappings}, ensure_ascii=False, indent=2) + "\n",
        encoding="utf-8",
    )
    review_path = DATA / "knowledge_mapping_review.json"
    existing_reviews = json.loads(review_path.read_text(encoding="utf-8")) if review_path.exists() else {"items": []}
    database_reviews = []
    for item in existing_reviews.get("items", []):
        if item.get("source") == "database_fallback":
            normalized = {}
            for field in ("source", "questionId", "key", "level", "subject", "stem", "knowledgePointIds",
                          "method", "confidence", "suggestedKnowledgePointIds", "suggestedConfidence",
                          "suggestedReviewStatus", "reviewStatus", "reviewReason", "basis"):
                value = "pending" if field == "reviewStatus" else item.get(field)
                if field == "source":
                    value = "database_fallback"
                if field == "knowledgePointIds" and isinstance(value, list):
                    value = value[:1]
                if field == "suggestedKnowledgePointIds" and not value and len(item.get("knowledgePointIds", [])) > 1:
                    value = item["knowledgePointIds"][1:]
                if field == "suggestedConfidence" and value in (None, "") and len(item.get("knowledgePointIds", [])) > 1:
                    value = item.get("confidence", 0.5)
                if field == "suggestedReviewStatus" and value in (None, "") and len(item.get("knowledgePointIds", [])) > 1:
                    value = "pending"
                if value not in (None, "", []):
                    normalized[field] = value
            normalized["reviewStatus"] = "pending"
            database_reviews.append(normalized)
    review_items = []
    for item in mappings:
        if item.get("suggestedKnowledgePointIds"):
            review_items.append({**item, "reviewStatus": "pending"})
    for item in database_reviews:
        item["reviewStatus"] = "pending"
        review_items.append(item)
    review_items.sort(key=lambda item: (str(item.get("questionId", "")), str(item.get("source", "")), str(item.get("stem", ""))))
    review_path.write_text(
        json.dumps({"version": 2, "items": review_items}, ensure_ascii=False, indent=2) + "\n",
        encoding="utf-8",
    )
    (DATA / "source_section_samples.json").write_text(
        json.dumps({"version": 1, "items": samples}, ensure_ascii=False, indent=2) + "\n",
        encoding="utf-8",
    )
    root_only = sum(len(item["knowledgePointIds"]) == 1 for item in mappings)
    confirmed_fine = len(mappings) - root_only
    pending_suggestions = len(review_items) - len(database_reviews)
    print(f"knowledge_points={len(points)} formal_mappings={len(mappings)} root_only={root_only} confirmed_fine={confirmed_fine} pending_suggestions={pending_suggestions} samples={len(samples)}")


if __name__ == "__main__":
    main()
