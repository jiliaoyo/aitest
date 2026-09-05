#!/usr/bin/env python3
"""把红蓝宝书 N1/N2/N3 扫描 PDF 转成后端 JSON 导入文件。

默认先 OCR 到 /tmp/redblue123，再从缓存构建 questions_json；不会连接数据库。
"""
from __future__ import annotations

import argparse
import json
import os
import re
import sys
import unicodedata
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
PDF_DIR = ROOT / "pdf"
OUT_DIR = ROOT / "questions_json"
WORK_DIR = Path("/tmp/redblue123")

BOOKS = {
    "n1": {
        "level": "n1",
        "pdf": PDF_DIR / "红蓝宝书1000题 新日本语能力考试N1文字.词汇.文法 练习+详解  .pdf",
        "pages": 341,
        "unit_counts": [36] * 18 + [40, 42],
        "mock_starts": [247, 263, 279, 295, 311, 327],
        "mock_ranges": [(731, 775), (776, 820), (821, 865), (866, 910), (911, 955), (956, 1000)],
        "mock_end": 341,
    },
    "n2": {
        "level": "n2",
        "pdf": PDF_DIR / "红蓝宝书1000题  新日本语能力考试N2文字.词汇.文法  练习+详解.pdf",
        "pages": 337,
        "unit_counts": [36] * 18 + [40, 42],
        "mock_starts": [247, 265, 283, 301, 319],
        "mock_ranges": [(731, 784), (785, 838), (839, 892), (893, 946), (947, 1000)],
        "mock_end": 336,
    },
    "n3": {
        "level": "n3",
        "pdf": PDF_DIR / "红蓝宝书1000题·新日本语能力考试N3文字·词汇·文法.pdf",
        "pages": 337,
        "unit_counts": [36] * 18 + [31, 31],
        "mock_starts": [248, 264, 280, 302, 319],
        "mock_ranges": [(711, 768), (769, 826), (827, 884), (885, 942), (943, 1000)],
        "mock_end": 336,
    },
}

ITEM_KEYS = {
    "rawExcerpt", "materialKey", "type", "stem", "options", "materialTitle",
    "materialContent", "levelCode", "subjectCode", "difficulty",
    "knowledgePointNames", "sourceAnswer", "aiSuggestedAnswer", "anomalies",
}
QUESTION_PROMPT = "<image>\nTranscribe every Japanese question number and all four answer choices exactly. Do not summarize."
ANSWER_PROMPT = "<image>\nRead only the answer-key row at the top of the page. Return the question range and its answer digits, for example: 001-006: 2 3 1 4 2 1. Ignore all explanations."
QUESTION_RE = re.compile(r"(?m)^\s*(\d{3,4})(?=\s|[.．、)])")
OPTION_RE = re.compile(r"(?m)(?:^|\s)([1-4])(?:[.．、:：)]|[ \t]+|(?=[\u3040-\u30ff\u3400-\u9fffA-Za-z]))")
SECTION_RE = re.compile(r"(?:問題|问题)\s*([1-9])")
ANSWER_RE = re.compile(
    r"(?<!\d)(\d{3,4})\s*(?:\[?【?\s*答\s*案\s*】?\]?|答\s*案)\s*[:：]?\s*([1-4])(?!\d)"
)
NOISE_RE = re.compile(r"(?:aws|attsu?|角和|image|text|hydraulic|press|\ufffd|[<>])", re.I)


def normalize_text(value: str) -> str:
    value = unicodedata.normalize("NFKC", value)
    value = value.replace("\r", " ").replace("\u00a0", " ")
    value = value.replace("**", "")
    value = re.sub(r"```[^\n]*|```", "", value)
    value = re.sub(r"<br\s*/?>", " ", value, flags=re.I)
    lines = []
    for line in value.splitlines():
        line = re.sub(r"^\s*#{1,6}\s*", "", line)
        line = line.strip(" |`*")
        if line:
            lines.append(line)
    return "\n".join(lines)


def compact(value: str) -> str:
    value = normalize_text(value)
    value = re.sub(r"\s+", " ", value).strip(" |-*#")
    return value


def page_number(path: Path) -> int:
    match = re.search(r"page(\d+)", path.name)
    if not match:
        raise ValueError(path)
    return int(match.group(1))


def book_dir(book: str) -> Path:
    path = WORK_DIR / "ocr" / book
    path.mkdir(parents=True, exist_ok=True)
    return path


def render_page(doc, number: int, target: Path) -> None:
    target.parent.mkdir(parents=True, exist_ok=True)
    if target.exists() and target.stat().st_size:
        return
    doc[number - 1].get_pixmap(dpi=200, alpha=False).save(target)


def load_ocr_model():
    os.environ["HF_HUB_OFFLINE"] = "1"
    sys.path.insert(0, str(ROOT / "scripts"))
    from ocr_utils import load_ocr

    return load_ocr()


def ocr(model, processor, generate, image: Path, *, tokens: int, penalty: float) -> str:
    result = generate(
        model,
        processor,
        prompt="<image>\nFree OCR.",
        image=str(image),
        max_tokens=tokens,
        verbose=False,
        repetition_penalty=penalty,
    )
    return result.text if hasattr(result, "text") else str(result)


def ocr_question(model, processor, generate, image: Path) -> str:
    result = generate(
        model,
        processor,
        prompt=QUESTION_PROMPT,
        image=str(image),
        max_tokens=2600,
        verbose=False,
        repetition_penalty=1.2,
    )
    return result.text if hasattr(result, "text") else str(result)


def ocr_answer_key(model, processor, generate, image: Path) -> str:
    result = generate(
        model,
        processor,
        prompt=ANSWER_PROMPT,
        image=str(image),
        max_tokens=240,
        verbose=False,
        repetition_penalty=1.2,
    )
    return result.text if hasattr(result, "text") else str(result)


def write_if_missing(path: Path, value: str) -> None:
    if not path.exists() or not path.read_text(encoding="utf-8").strip():
        path.write_text(value.strip(), encoding="utf-8")


def crop_top(image: Path, target: Path) -> None:
    from PIL import Image, ImageEnhance

    with Image.open(image) as source:
        width, height = source.size
        crop = source.crop((0, 0, width, int(height * 0.18))).convert("L")
        crop = ImageEnhance.Contrast(crop).enhance(1.4)
        crop.save(target)


def crop_halves(image: Path, directory: Path, number: int) -> tuple[Path, Path]:
    from PIL import Image

    with Image.open(image) as source:
        width, height = source.size
        paths = []
        for index, (top, bottom) in enumerate(((0, int(height * 0.56)), (int(height * 0.44), height)), 1):
            target = directory / f"page{number:03d}.part{index}.png"
            if not target.exists():
                source.crop((0, top, width, bottom)).save(target)
            paths.append(target)
    return paths[0], paths[1]


def ocr_content_pages(book: str, model, processor, generate) -> None:
    config = BOOKS[book]
    import pymupdf

    root = book_dir(book)
    image_dir = WORK_DIR / "images" / book
    image_dir.mkdir(parents=True, exist_ok=True)
    doc = pymupdf.open(config["pdf"])
    # 普通单元的题页/答案页并不总是严格按固定页数排列；N3 后两个单元
    # 还会因详解长度出现额外页。用顶部标题识别页面类型比硬编码页表稳。
    for page in range(7, config["mock_starts"][0]):
        image = image_dir / f"page{page:03d}.png"
        top_image = image_dir / f"page{page:03d}.top.png"
        top_output = root / f"page{page:03d}.top.txt"
        render_page(doc, page, image)
        if not top_image.exists():
            crop_top(image, top_image)
        if not top_output.exists() or not top_output.read_text(encoding="utf-8").strip():
            top_output.write_text(ocr(model, processor, generate, top_image, tokens=300, penalty=1.2).strip(), encoding="utf-8")
        top = top_output.read_text(encoding="utf-8")
        if book == "n1":
            is_answer = "問題" not in doc[page - 1].get_text("text")
        elif book == "n2":
            # 这两本普通单元固定为题页、答案页交替；顶部装饰会让 OCR 偶尔
            # 漏掉“解答”，页序本身更可靠。
            is_answer = (page - 7) % 2 == 1
        else:
            is_answer = bool(re.search(r"解答|答案|正解|解説", top))
        (root / f"page{page:03d}.q.txt").unlink(missing_ok=True) if is_answer else None
        (root / f"page{page:03d}.akey.txt").unlink(missing_ok=True) if not is_answer else None
        if is_answer:
            output = root / f"page{page:03d}.akey.txt"
            if not output.exists() or not output.read_text(encoding="utf-8").strip():
                output.write_text(top, encoding="utf-8")
            key_output = root / f"page{page:03d}.key.txt"
            if not key_output.exists() or not key_output.read_text(encoding="utf-8").strip():
                key_output.write_text(ocr_answer_key(model, processor, generate, top_image).strip(), encoding="utf-8")
            kind = "answer"
        else:
            output = root / f"page{page:03d}.q.txt"
            if not output.exists() or not output.read_text(encoding="utf-8").strip():
                full = ocr_question(model, processor, generate, image).strip()
                # 顶部 OCR 偶尔漏掉“解答”，从整页结果兜底纠正页面类型。
                if re.search(r"解答|答案|正解|解説", full[:500]) and not re.search(r"(?:^|\n)\s*問題", full[:500]):
                    (root / f"page{page:03d}.akey.txt").write_text(full, encoding="utf-8")
                    kind = "answer"
                else:
                    output.write_text(full, encoding="utf-8")
                    kind = "question"
            else:
                kind = "question"
        print(f"{book} content page {page} {kind}", flush=True)


def ocr_mocks(book: str, model, processor, generate) -> None:
    config = BOOKS[book]
    import pymupdf

    root = book_dir(book)
    image_dir = WORK_DIR / "images" / book
    image_dir.mkdir(parents=True, exist_ok=True)
    doc = pymupdf.open(config["pdf"])
    start = config["mock_starts"][0]
    for page in range(start, config["mock_end"] + 1):
        image = image_dir / f"page{page:03d}.png"
        top_image = image_dir / f"page{page:03d}.mocktop.png"
        top_output = root / f"page{page:03d}.mocktop.txt"
        render_page(doc, page, image)
        if not top_image.exists():
            crop_top(image, top_image)
        if not top_output.exists() or not top_output.read_text(encoding="utf-8").strip():
            top_output.write_text(ocr(model, processor, generate, top_image, tokens=300, penalty=1.2).strip(), encoding="utf-8")
        top = top_output.read_text(encoding="utf-8")
        fixed_kind = None
        if book == "n1":
            fixed_kind = "question" if any(s <= page < s + 7 for s in config["mock_starts"]) else "answer"
        elif book == "n2":
            fixed_kind = "question" if any(s <= page < s + 9 for s in config["mock_starts"]) else "answer"
        is_answer = fixed_kind == "answer" if fixed_kind else bool(re.search(r"解答|答案|正解|解説", top))
        (root / f"page{page:03d}.q.txt").unlink(missing_ok=True) if is_answer else None
        (root / f"page{page:03d}.a.txt").unlink(missing_ok=True) if not is_answer else None
        if is_answer:
            output = root / f"page{page:03d}.a.txt"
            if not output.exists() or not output.read_text(encoding="utf-8").strip():
                upper, lower = crop_halves(image, image_dir, page)
                upper_text = ocr(model, processor, generate, upper, tokens=1800, penalty=1.25)
                lower_text = ocr(model, processor, generate, lower, tokens=1800, penalty=1.25)
                output.write_text((upper_text + "\n" + lower_text).strip(), encoding="utf-8")
            kind = "answer"
        else:
            output = root / f"page{page:03d}.q.txt"
            if not output.exists() or not output.read_text(encoding="utf-8").strip():
                output.write_text(ocr_question(model, processor, generate, image).strip(), encoding="utf-8")
            kind = "question"
        print(f"{book} mock page {page} {kind}", flush=True)


def ocr_book(book: str) -> None:
    model, processor, generate = load_ocr_model()
    ocr_content_pages(book, model, processor, generate)
    ocr_mocks(book, model, processor, generate)


def option_matches(block: str) -> list[re.Match[str]]:
    return list(OPTION_RE.finditer(block))


def choose_question_matches(text: str, first: int, last: int) -> tuple[list[tuple[re.Match[str], re.Match[str] | None]], list[int]]:
    # 题号有时被 OCR 粘在段落中（尤其是文章填空题），按预期连续题号
    # 查找比“必须行首”更能容错；三位/四位题号不会与选项编号混淆。
    selected: list[re.Match[str]] = []
    missing: list[int] = []
    cursor = 0
    for number in range(first, last + 1):
        patterns = [re.compile(rf"(?<!\d)({number:03d})(?!\d)")]
        if number >= 100:
            patterns = [
                re.compile(rf"(?<!\d)(0*{number})(?!\d)"),
                re.compile(rf"(?m)^\s*[^\d\n]{{0,8}}(0*{number})"),
            ]
        else:
            patterns.append(re.compile(rf"(?m)^\s*({number})(?=\s|[.．、)])"))
        match = next((candidate.search(text, cursor) for candidate in patterns if candidate.search(text, cursor)), None)
        if match is None:
            missing.append(number)
            continue
        selected.append(match)
        cursor = match.end()
    selected_with_end = []
    for index, match in enumerate(selected):
        end = selected[index + 1] if index + 1 < len(selected) else None
        selected_with_end.append((match, end))
    return selected_with_end, missing


def parse_question_blocks(text: str, first: int, last: int, level: str, mock: bool) -> tuple[list[dict], list[int]]:
    text = normalize_text(text)
    selected, missing = choose_question_matches(text, first, last)
    items = []
    for match, next_match in selected:
        end = next_match.start() if next_match else len(text)
        block = text[match.end():end]
        options = option_matches(block)
        stem = compact(block[: options[0].start() if options else len(block)])
        option_values = []
        for index, option in enumerate(options[:4]):
            end_option = options[index + 1].start() if index + 1 < len(options) else len(block)
            value = compact(block[option.end():end_option])
            if value:
                option_values.append(value)
        number = int(match.group(1))
        anomalies = []
        if len(options) != 4 or len(option_values) != 4:
            anomalies.append("OCR 未识别出完整四选项，请人工核对原题")
        if len(stem) < 2:
            anomalies.append("OCR 题干过短，请人工核对原题")
        if NOISE_RE.search(stem) or any(NOISE_RE.search(x) for x in option_values):
            anomalies.append("题干或选项含疑似 OCR 噪声，请人工核对原题")
        section = None
        if mock:
            sections = list(SECTION_RE.finditer(text[:match.start()]))
            if sections:
                section = int(sections[-1].group(1))
            else:
                anomalies.append("模拟题小节未能从 OCR 文本识别，请人工核对科目")
        if mock:
            subject = "grammar" if section is not None and section >= 6 else "vocabulary"
        else:
            subject = "grammar" if (number - 1) % 6 >= 4 else "vocabulary"
        items.append({
            "number": number,
            "stem": stem,
            "options": [{"id": "abcd"[i], "label": "ABCD"[i], "text": option_values[i] if i < len(option_values) else ""}
                        for i in range(4)],
            "levelCode": level,
            "subjectCode": subject,
            "anomalies": anomalies,
        })
    return items, missing


def parse_unit_answers(root: Path) -> dict[int, int]:
    answers: dict[int, int] = {}
    paths = (list(root.glob("page*.akey.txt")) + list(root.glob("page*.key.txt"))
             + list(root.glob("page*.keytight.txt")))
    for path in sorted(paths, key=page_number):
        text = normalize_text(path.read_text(encoding="utf-8"))
        range_match = re.search(
            r"(\d{1,4})\s*[-~－一]\s*(\d{1,4})\s*(?:正解|正确|答案)?\s*[:：]?",
            text,
        )
        if not range_match:
            continue
        start, end = int(range_match.group(1)), int(range_match.group(2))
        after = text[range_match.end():].splitlines()[0] if text[range_match.end():] else ""
        digits = [int(x) for x in re.findall(r"(?<!\d)([1-4])(?!\d)", after)]
        if len(digits) != end - start + 1:
            nearby = text[range_match.end():range_match.end() + 100]
            digits = [int(x) for x in re.findall(r"(?<!\d)([1-4])(?!\d)", nearby)]
        for number, answer in zip(range(start, end + 1), digits):
            answers[number] = answer
    return answers


def parse_mock_answers(root: Path) -> dict[int, int]:
    answers: dict[int, int] = {}
    for path in sorted(root.glob("page*.a.txt"), key=page_number):
        text = normalize_text(path.read_text(encoding="utf-8"))
        for match in ANSWER_RE.finditer(text):
            number, answer = int(match.group(1)), int(match.group(2))
            if 1 <= number <= 1000:
                answers[number] = answer
    return answers


def question_page_text(path: Path, book: str, target_questions: int, pdf_doc=None) -> str:
    raw = path.read_text(encoding="utf-8")
    if book != "n1" or pdf_doc is None:
        return raw
    try:
        pdf_text = pdf_doc[page_number(path) - 1].get_text("text")
    except Exception:
        return raw

    def score(value: str) -> tuple[int, int, int]:
        normalized = normalize_text(value)
        q_count = len(re.findall(r"(?m)^\s*(?:[^\d\n]{0,8})\d{3,4}(?=\s|[.．、)])", normalized))
        option_count = len(option_matches(normalized))
        noise = len(NOISE_RE.findall(normalized))
        return (
            min(option_count, target_questions * 4) * 10 - abs(q_count - target_questions) * 12,
            -noise,
            -abs(q_count - target_questions),
        )

    return max((raw, pdf_text), key=score)


def merge_question_sources(primary: list[dict], secondary: list[dict], first: int, last: int) -> tuple[list[dict], list[int]]:
    by_number: dict[int, list[dict]] = {}
    for item in primary + secondary:
        by_number.setdefault(item["number"], []).append(item)

    def quality(item: dict) -> tuple[int, int, int, int]:
        texts = [item["stem"], *(option["text"] for option in item["options"])]
        filled = sum(bool(option["text"]) for option in item["options"])
        noise = len(NOISE_RE.findall(" ".join(texts)))
        return filled, -noise, min(len(item["stem"]), 200), -len(item["anomalies"])

    merged = []
    missing = []
    for number in range(first, last + 1):
        candidates = by_number.get(number, [])
        if not candidates:
            missing.append(number)
            continue
        merged.append(max(candidates, key=quality))
    return merged, missing


def parse_question_sources(raw_text: str, pdf_text: str, first: int, last: int,
                          level: str, mock: bool) -> tuple[list[dict], list[int]]:
    primary, _ = parse_question_blocks(raw_text, first, last, level, mock)
    secondary = []
    if pdf_text.strip():
        secondary, _ = parse_question_blocks(pdf_text, first, last, level, mock)
    return merge_question_sources(primary, secondary, first, last)


def build_item(parsed: dict, answer: int | None) -> dict:
    stem = parsed["stem"]
    anomalies = list(dict.fromkeys(parsed["anomalies"]))
    if answer is None:
        anomalies.append("答案页 OCR 未识别，请人工核对原书答案")
    item = {
        "rawExcerpt": stem[:5000],
        "materialKey": "",
        "type": "single_choice",
        "stem": stem,
        "options": parsed["options"],
        "materialTitle": "",
        "materialContent": "",
        "levelCode": parsed["levelCode"],
        "subjectCode": parsed["subjectCode"],
        "difficulty": 3,
        "knowledgePointNames": [],
        "sourceAnswer": None if answer is None else {
            "value": {"optionIds": ["abcd"[answer - 1]]},
            "authority": "official",
            "explanation": "",
        },
        "aiSuggestedAnswer": None,
        "anomalies": anomalies,
    }
    return item


def validate_items(items: list[dict], level: str) -> list[str]:
    errors = []
    if len(items) != 1000:
        errors.append(f"题目数量为 {len(items)}，预期 1000")
    numbers = [int(x["number"]) for x in items]
    if numbers != list(range(1, 1001)):
        errors.append("题号不是 1 到 1000 的连续序列")
    for index, item in enumerate(items, 1):
        if set(item) - {"number"} != ITEM_KEYS:
            errors.append(f"第 {index} 题字段集合不符合导入契约")
        if item["levelCode"] != level or item["type"] != "single_choice":
            errors.append(f"第 {index} 题级别或题型不符合预期")
        if len(item["options"]) != 4 or any(not option["text"] for option in item["options"]):
            errors.append(f"第 {index} 题选项不完整")
        answer = item["sourceAnswer"]
        if answer is not None:
            ids = answer.get("value", {}).get("optionIds", [])
            if answer.get("authority") != "official" or len(ids) != 1 or ids[0] not in {"a", "b", "c", "d"}:
                errors.append(f"第 {index} 题答案结构不合法")
    return errors


def build_book(book: str) -> tuple[list[dict], list[str]]:
    config = BOOKS[book]
    root = book_dir(book)
    parsed_by_number: dict[int, dict] = {}
    errors: list[str] = []
    pdf_doc = None
    if book == "n1":
        import pymupdf
        pdf_doc = pymupdf.open(config["pdf"])
    q_paths = [
        path for path in sorted(root.glob("page*.q.txt"), key=page_number)
        if page_number(path) < config["mock_starts"][0]
    ]
    q_text = "\n".join(
        path.read_text(encoding="utf-8")
        for path in q_paths
    )
    q_pdf_text = "\n".join(
        (pdf_doc[page_number(path) - 1].get_text("text") if pdf_doc is not None else "")
        for path in q_paths
    )
    q_start = 1
    for unit, count in enumerate(config["unit_counts"], 1):
        first, last = q_start, q_start + count - 1
        parsed, missing = parse_question_sources(q_text, q_pdf_text, first, last, config["level"], mock=False)
        if missing:
            errors.append(f"unit {unit} 缺少题号: {missing[:12]}")
        parsed_by_number.update({item["number"]: item for item in parsed})
        q_start = last + 1
    mock_paths = [
        path for path in sorted(root.glob("page*.q.txt"), key=page_number)
        if page_number(path) >= config["mock_starts"][0]
    ]
    mock_q_text = "\n".join(path.read_text(encoding="utf-8") for path in mock_paths)
    mock_pdf_text = "\n".join(
        (pdf_doc[page_number(path) - 1].get_text("text") if pdf_doc is not None else "")
        for path in mock_paths
    )
    for first, last in config["mock_ranges"]:
        parsed, missing = parse_question_sources(mock_q_text, mock_pdf_text, first, last, config["level"], mock=True)
        if missing:
            errors.append(f"mock {first}-{last} 缺少题号: {missing[:12]}")
        parsed_by_number.update({item["number"]: item for item in parsed})
    unit_answers = parse_unit_answers(root)
    mock_answers = parse_mock_answers(root)
    answers = {**unit_answers, **mock_answers}
    items = []
    for number in range(1, 1001):
        parsed = parsed_by_number.get(number)
        if parsed is None:
            errors.append(f"题号 {number} 没有可结构化题干")
            continue
        items.append({"number": number, **build_item(parsed, answers.get(number))})
    validation = validate_items(items, config["level"])
    errors.extend(validation)
    return items, list(dict.fromkeys(errors))


def write_book(book: str, items: list[dict]) -> None:
    config = BOOKS[book]
    OUT_DIR.mkdir(parents=True, exist_ok=True)
    for path in OUT_DIR.glob(f"红蓝宝书1000题{book.upper()}-*.json"):
        path.unlink()
    for index in range(2):
        chunk = [{key: value for key, value in item.items() if key != "number"}
                 for item in items[index * 500:(index + 1) * 500]]
        target = OUT_DIR / f"红蓝宝书1000题{book.upper()}-{index + 1:02d}.json"
        target.write_text(json.dumps({"items": chunk}, ensure_ascii=False, indent=1) + "\n", encoding="utf-8")
        print(f"{target.name}: {len(chunk)}", flush=True)


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--book", choices=sorted(BOOKS))
    parser.add_argument("--ocr-only", action="store_true")
    parser.add_argument("--build-only", action="store_true")
    args = parser.parse_args()
    books = [args.book] if args.book else list(BOOKS)
    if not args.build_only:
        for book in books:
            ocr_book(book)
    if not args.ocr_only:
        for book in books:
            items, errors = build_book(book)
            print(f"{book}: parsed={len(items)} answers={sum(item['sourceAnswer'] is not None for item in items)}", flush=True)
            if errors:
                print("VALIDATION_ERRORS", json.dumps(errors[:30], ensure_ascii=False), flush=True)
                raise SystemExit(1)
            write_book(book, items)


if __name__ == "__main__":
    main()
