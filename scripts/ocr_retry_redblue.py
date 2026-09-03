#!/usr/bin/env python3
"""只重识别已知退化的红蓝宝书题目页；按上下半页避免整页重复输出。"""
from __future__ import annotations

import argparse
from pathlib import Path

from PIL import Image

from ocr_utils import load_ocr


ROOT = Path(__file__).resolve().parents[1]
INPUT = Path("/tmp/redblue/question_images")
OUTPUT = Path("/tmp/redblue/question_ocr_retry")
DEFAULT_PAGES = (130, 294, 295, 310, 316, 317)


def ocr(model, processor, generate, image: Path, penalty: float = 1.25) -> str:
    result = generate(
        model,
        processor,
        prompt="<image>\nFree OCR.",
        image=str(image),
        max_tokens=2600,
        verbose=False,
        repetition_penalty=penalty,
    )
    return result.text if hasattr(result, "text") else str(result)


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--pages", nargs="*", type=int, default=list(DEFAULT_PAGES))
    args = parser.parse_args()
    OUTPUT.mkdir(parents=True, exist_ok=True)
    model, processor, generate = load_ocr()
    for number in args.pages:
        source = INPUT / f"page{number:03d}.png"
        if not source.exists():
            raise FileNotFoundError(source)
        with Image.open(source) as image:
            width, height = image.size
            parts = []
            for index, (top, bottom) in enumerate(((0, int(height * 0.54)), (int(height * 0.48), height)), 1):
                crop = OUTPUT / f"page{number:03d}_part{index}.png"
                image.crop((0, top, width, bottom)).save(crop)
                parts.append(ocr(model, processor, generate, crop))
                crop.unlink(missing_ok=True)
        (OUTPUT / f"page{number:03d}.txt").write_text("\n".join(parts), encoding="utf-8")
        print(number, "ok")


if __name__ == "__main__":
    main()
