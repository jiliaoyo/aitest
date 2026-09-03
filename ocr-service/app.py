"""本地 OCR 导入服务：PDF -> DeepSeek-OCR -> 人工逐页审核 -> 文本 LLM 结构化 -> 导出 JSON。

导出的 JSON 直接对应后端 imports 模块的 .json 导入格式（管理页上传即可生成待审核草稿）。
运行：../.venv-ocr/bin/python -m uvicorn app:app --host 127.0.0.1 --port 8787
"""
from __future__ import annotations

import json
import os
import re
import shutil
import sys
import tempfile
import threading
import time
import uuid
from collections import Counter
from pathlib import Path

import httpx
import pymupdf
from fastapi import FastAPI, File, Form, HTTPException, UploadFile
from fastapi.responses import FileResponse
from fastapi.staticfiles import StaticFiles

ROOT = Path(__file__).resolve().parent
DATA = ROOT / "data"
PROMPT = (ROOT / "prompt_structure.md").read_text(encoding="utf-8")
sys.path.insert(0, str(ROOT.parent / "scripts"))

# 文本 LLM 配置（只做结构化，不做视觉识别；在 ocr-service/.env 或环境变量里配置）
def _load_env() -> None:
    env_file = ROOT / ".env"
    if not env_file.exists():
        return
    for line in env_file.read_text(encoding="utf-8").splitlines():
        line = line.strip()
        if line and not line.startswith("#") and "=" in line:
            key, _, value = line.partition("=")
            os_environ_setdefault(key.strip(), value.strip())

os_environ_setdefault = os.environ.setdefault
_load_env()

LLM_BASE_URL = os.environ.get("LLM_BASE_URL", "").rstrip("/")
LLM_API_KEY = os.environ.get("LLM_API_KEY", "")
LLM_MODEL = os.environ.get("LLM_MODEL", "")
CHUNK_CHARS = int(os.environ.get("STRUCTURE_CHUNK_CHARS", "6000"))
MAX_UPLOAD_BYTES = int(os.environ.get("OCR_MAX_UPLOAD_BYTES", str(100 * 1024 * 1024)))
MAX_PAGES = int(os.environ.get("OCR_MAX_PAGES", "500"))

app = FastAPI(title="本地 OCR 导入服务")
DATA.mkdir(parents=True, exist_ok=True)

_model_lock = threading.Lock()
_model = None
_ocr_thread: threading.Thread | None = None
_ocr_lock = threading.Lock()
_structure_lock = threading.Lock()
_structure_jobs: set[str] = set()
_meta_lock = threading.RLock()


def job_dir(job_id: str) -> Path:
    if not re.fullmatch(r"[0-9a-f]{12}", job_id):
        raise HTTPException(404, "任务不存在")
    path = DATA / job_id
    if not path.is_dir():
        raise HTTPException(404, "任务不存在")
    return path


def read_meta(path: Path) -> dict:
    with _meta_lock:
        return json.loads((path / "meta.json").read_text(encoding="utf-8"))


def write_meta(path: Path, meta: dict) -> None:
    with _meta_lock:
        write_json(path / "meta.json", meta)


def write_json(target: Path, value: object) -> None:
    temp = None
    try:
        with tempfile.NamedTemporaryFile("w", encoding="utf-8", dir=target.parent,
                                        prefix=f".{target.name}.", suffix=".tmp", delete=False) as out:
            temp = Path(out.name)
            json.dump(value, out, ensure_ascii=False, indent=1)
            out.flush()
            os.fsync(out.fileno())
        temp.replace(target)
    finally:
        if temp:
            temp.unlink(missing_ok=True)


def update_meta(path: Path, update) -> dict:
    with _meta_lock:
        meta = read_meta(path)
        update(meta)
        write_meta(path, meta)
        return meta


def page_text(path: Path, page: int) -> str:
    txt = path / "pages" / f"page{page:03d}.txt"
    return txt.read_text(encoding="utf-8") if txt.exists() else ""


def job_summary(path: Path) -> dict:
    meta = read_meta(path)
    pages = []
    for n in range(1, meta["pageCount"] + 1):
        pages.append({
            "page": n,
            "recognized": (path / "pages" / f"page{n:03d}.txt").stat().st_size > 0
            if (path / "pages" / f"page{n:03d}.txt").exists() else False,
            "approved": n in meta.get("approved", []),
        })
    item_count = meta.get("itemCount")
    if item_count is None:
        item_count = len(read_items(path))
    return {**meta, "pages": pages, "itemCount": item_count}


# ---------- OCR ----------

def load_model():
    global _model
    with _model_lock:
        if _model is None:
            from ocr_utils import load_ocr
            _model = load_ocr()
        return _model


def ocr_page(model, processor, generate, image: Path, penalty: float) -> str:
    result = generate(
        model, processor,
        prompt="<image>\nFree OCR.",
        image=str(image),
        max_tokens=2600,
        verbose=False,
        repetition_penalty=penalty,
    )
    return result.text if hasattr(result, "text") else str(result)


def looks_degenerate(text: str) -> bool:
    lines = [line.strip() for line in text.splitlines() if line.strip()]
    if not lines:
        return True
    most_common = Counter(lines).most_common(1)[0][1]
    return most_common >= 8 and most_common >= len(lines) // 2


def run_ocr(job_id: str) -> None:
    path = job_dir(job_id)
    meta = read_meta(path)
    try:
        model, processor, generate = load_model()
        for n in range(1, meta["pageCount"] + 1):
            if page_text(path, n).strip():
                continue
            if not read_meta(path)["ocr"].get("running"):
                break  # 被停止
            update_meta(path, lambda current: current["ocr"].update(current=n))
            image = path / "pages" / f"page{n:03d}.png"
            try:
                text = ocr_page(model, processor, generate, image, penalty=1.2)
                if looks_degenerate(text):
                    retry = ocr_page(model, processor, generate, image, penalty=1.35)
                    if not looks_degenerate(retry) or len(retry) > len(text):
                        text = retry
                (path / "pages" / f"page{n:03d}.txt").write_text(text.strip(), encoding="utf-8")
            except Exception as exc:  # 单页失败不拖垮整批，审核界面可重跑该页
                update_meta(path, lambda current: current["ocr"].update(error=f"第 {n} 页识别失败: {exc}"))
    except Exception as exc:
        update_meta(path, lambda current: current["ocr"].update(error=f"OCR 中断: {exc}"))
    finally:
        try:
            update_meta(path, lambda current: current["ocr"].update(running=False, current=0))
        finally:
            global _ocr_thread
            with _ocr_lock:
                _ocr_thread = None


# ---------- 结构化 ----------

ALLOWED_ITEM_KEYS = {"rawExcerpt", "materialKey", "type", "stem", "options", "materialTitle",
                     "materialContent", "levelCode", "subjectCode", "difficulty",
                     "knowledgePointNames", "sourceAnswer", "aiSuggestedAnswer", "anomalies"}
ALLOWED_OPTION_KEYS = {"id", "label", "text"}


def normalize_answer(answer):
    """接受裸答案值（{"optionIds":["a"]}）或完整 AnswerInput，统一补全为后端格式。"""
    if not isinstance(answer, dict) or not answer:
        return None
    if "value" in answer:
        out = dict(answer)
    else:
        out = {"value": answer}
    out.setdefault("authority", "official")
    out.setdefault("explanation", "")
    return out


def normalize_suggestion(suggestion):
    if not isinstance(suggestion, dict) or not suggestion:
        return None
    if "value" in suggestion:
        out = dict(suggestion)
    else:
        out = {"value": suggestion}
    out.setdefault("explanation", "AI 建议答案，请人工确认。")
    return out


def sanitize_items(items: list) -> list[dict]:
    out = []
    for raw in items:
        if not isinstance(raw, dict):
            continue
        item = {k: v for k, v in raw.items() if k in ALLOWED_ITEM_KEYS}
        options = item.get("options")
        item["options"] = [{k: v for k, v in opt.items() if k in ALLOWED_OPTION_KEYS}
                           for opt in options if isinstance(opt, dict)] if isinstance(options, list) else []
        for key, default in (("rawExcerpt", ""), ("materialKey", ""), ("type", ""), ("stem", ""),
                             ("materialTitle", ""), ("materialContent", ""), ("levelCode", ""),
                             ("subjectCode", ""), ("difficulty", 3)):
            item.setdefault(key, default)
        item["sourceAnswer"] = normalize_answer(item.get("sourceAnswer"))
        item["aiSuggestedAnswer"] = normalize_suggestion(item.get("aiSuggestedAnswer"))
        for key in ("knowledgePointNames", "anomalies"):
            if not isinstance(item.get(key), list):
                item[key] = []
        out.append(item)
    return out


def call_llm(text: str, level_code: str, subject_code: str) -> list[dict]:
    if not (LLM_BASE_URL and LLM_MODEL):
        raise RuntimeError("未配置文本模型：请在 ocr-service/.env 设置 LLM_BASE_URL、LLM_API_KEY、LLM_MODEL")
    user = f"默认级别代码：{level_code}\n默认科目代码：{subject_code}\n\nOCR 文本：\n{text}"
    body = {"model": LLM_MODEL, "temperature": 0.1,
            "response_format": {"type": "json_object"},
            "messages": [{"role": "system", "content": PROMPT}, {"role": "user", "content": user}]}
    headers = {"Authorization": f"Bearer {LLM_API_KEY}"} if LLM_API_KEY else {}
    with httpx.Client(timeout=300) as client:
        resp = client.post(f"{LLM_BASE_URL}/chat/completions", json=body, headers=headers)
        resp.raise_for_status()
    content = resp.json()["choices"][0]["message"]["content"]
    try:
        data = json.loads(content)
    except (TypeError, json.JSONDecodeError) as exc:
        raise ValueError("模型没有返回合法 JSON") from exc
    if not isinstance(data, dict):
        raise ValueError("模型返回的 JSON 根节点必须是对象")
    items = data.get("items")
    if not isinstance(items, list):
        raise ValueError("模型返回缺少 items 数组")
    return items


def split_chunks(pages: list[tuple[int, str]]) -> list[str]:
    chunks, current, size = [], [], 0
    for n, text in pages:
        block = f"【第 {n} 页】\n{text}\n"
        if current and size + len(block) > CHUNK_CHARS:
            chunks.append("\n".join(current))
            current, size = [], 0
        current.append(block)
        size += len(block)
    if current:
        chunks.append("\n".join(current))
    return chunks


def read_items(path: Path) -> list[dict]:
    items_file = path / "items.json"
    if not items_file.exists():
        return []
    return json.loads(items_file.read_text(encoding="utf-8")).get("items", [])


def write_items(path: Path, items: list[dict]) -> None:
    with _meta_lock:
        write_json(path / "items.json", {"items": items})
        meta = read_meta(path)
        meta["itemCount"] = len(items)
        write_meta(path, meta)


def recover_running_jobs() -> None:
    for path in DATA.iterdir():
        if not path.is_dir() or not (path / "meta.json").exists():
            continue
        try:
            meta = read_meta(path)
        except (OSError, json.JSONDecodeError):
            continue
        changed = False
        for key in ("ocr", "structure"):
            state = meta.get(key)
            if isinstance(state, dict) and state.get("running"):
                state["running"] = False
                state["error"] = "服务重启，任务已停止，请重新开始"
                if key == "ocr":
                    state["current"] = 0
                changed = True
        if changed:
            write_meta(path, meta)


recover_running_jobs()


def run_structure(job_id: str) -> None:
    path = job_dir(job_id)
    meta = read_meta(path)
    try:
        approved = set(meta.get("approved", []))
        pages = [(n, page_text(path, n)) for n in range(1, meta["pageCount"] + 1)
                 if n in approved and page_text(path, n).strip()]
        if not pages:
            raise RuntimeError("还没有已确认的识别页面")
        items: list[dict] = []
        for chunk in split_chunks(pages):
            items.extend(call_llm(chunk, meta["levelCode"], meta["subjectCode"]))
        items = sanitize_items(items)
        if not items:
            raise RuntimeError("模型没有拆出任何题目")
        write_items(path, items)
        update_meta(path, lambda current: current["structure"].update(
            finishedAt=time.strftime("%Y-%m-%dT%H:%M:%S")))
    except Exception as exc:
        update_meta(path, lambda current: current["structure"].update(error=str(exc)[:500]))
    finally:
        try:
            update_meta(path, lambda current: current["structure"].update(running=False))
        finally:
            with _structure_lock:
                _structure_jobs.discard(job_id)


# ---------- 路由 ----------

@app.post("/api/jobs")
def create_job(file: UploadFile = File(...), level_code: str = Form("n5"),
               subject_code: str = Form("grammar")):
    if not file.filename or not file.filename.lower().endswith(".pdf"):
        raise HTTPException(400, "只支持 PDF 文件")
    job_id = uuid.uuid4().hex[:12]
    path = DATA / job_id
    (path / "pages").mkdir(parents=True)
    pdf_path = path / "source.pdf"
    try:
        with pdf_path.open("wb") as out:
            size = 0
            while chunk := file.file.read(1024 * 1024):
                size += len(chunk)
                if size > MAX_UPLOAD_BYTES:
                    raise HTTPException(413, "PDF 文件过大")
                out.write(chunk)
    except Exception:
        shutil.rmtree(path, ignore_errors=True)
        raise
    try:
        with pymupdf.open(pdf_path) as doc:
            page_count = len(doc)
            if page_count < 1 or page_count > MAX_PAGES:
                raise HTTPException(400, f"PDF 页数必须在 1 到 {MAX_PAGES} 页之间")
            for index, page in enumerate(doc, 1):
                page.get_pixmap(dpi=200).save(path / "pages" / f"page{index:03d}.png")
    except Exception:
        shutil.rmtree(path, ignore_errors=True)
        raise
    write_meta(path, {"id": job_id, "fileName": file.filename, "levelCode": level_code,
                      "subjectCode": subject_code, "pageCount": page_count, "approved": [],
                      "ocr": {"running": False, "current": 0, "error": ""},
                      "structure": {"running": False, "error": "", "finishedAt": ""},
                      "createdAt": time.strftime("%Y-%m-%dT%H:%M:%S")})
    return job_summary(path)


@app.get("/api/jobs")
def list_jobs():
    jobs = [job_summary(path) for path in DATA.iterdir() if path.is_dir() and (path / "meta.json").exists()]
    return sorted(jobs, key=lambda job: job["createdAt"], reverse=True)


@app.get("/api/jobs/{job_id}")
def get_job(job_id: str):
    return job_summary(job_dir(job_id))


@app.post("/api/jobs/{job_id}/ocr")
def start_ocr(job_id: str):
    global _ocr_thread
    path = job_dir(job_id)
    with _ocr_lock:
        if _ocr_thread and _ocr_thread.is_alive():
            raise HTTPException(409, "已有 OCR 任务在运行")
        update_meta(path, lambda current: current.update(
            ocr={"running": True, "current": 0, "error": ""}))
        _ocr_thread = threading.Thread(target=run_ocr, args=(job_id,), daemon=True)
        _ocr_thread.start()
    return job_summary(path)


@app.post("/api/jobs/{job_id}/ocr/stop")
def stop_ocr(job_id: str):
    path = job_dir(job_id)
    update_meta(path, lambda current: current["ocr"].update(running=False))
    return job_summary(path)


@app.post("/api/jobs/{job_id}/pages/{page}/reocr")
def reocr_page(job_id: str, page: int):
    path = job_dir(job_id)
    if _ocr_thread and _ocr_thread.is_alive():
        raise HTTPException(409, "已有 OCR 任务在运行")
    image = path / "pages" / f"page{page:03d}.png"
    if not image.exists():
        raise HTTPException(404, "页面不存在")
    model, processor, generate = load_model()
    text = ocr_page(model, processor, generate, image, penalty=1.2)
    if looks_degenerate(text):
        retry = ocr_page(model, processor, generate, image, penalty=1.35)
        if not looks_degenerate(retry) or len(retry) > len(text):
            text = retry
    (path / "pages" / f"page{page:03d}.txt").write_text(text.strip(), encoding="utf-8")
    return {"page": page, "text": text.strip()}


@app.get("/api/jobs/{job_id}/pages/{page}")
def get_page(job_id: str, page: int):
    path = job_dir(job_id)
    meta = read_meta(path)
    if page < 1 or page > meta["pageCount"]:
        raise HTTPException(404, "页面不存在")
    return {"page": page, "text": page_text(path, page), "approved": page in meta.get("approved", [])}


@app.put("/api/jobs/{job_id}/pages/{page}")
def save_page(job_id: str, page: int, body: dict):
    path = job_dir(job_id)
    meta = read_meta(path)
    if page < 1 or page > meta["pageCount"]:
        raise HTTPException(404, "页面不存在")
    text = str(body.get("text", ""))
    if len(text) > 20000:
        raise HTTPException(400, "单页文本过长")
    (path / "pages" / f"page{page:03d}.txt").write_text(text, encoding="utf-8")
    approved = set()
    def update(current):
        approved.update(current.get("approved", []))
        (approved.add if body.get("approved") else approved.discard)(page)
        current["approved"] = sorted(approved)
    update_meta(path, update)
    return {"page": page, "approved": page in approved}


@app.get("/api/jobs/{job_id}/pages/{page}/image")
def page_image(job_id: str, page: int):
    image = job_dir(job_id) / "pages" / f"page{page:03d}.png"
    if not image.exists():
        raise HTTPException(404, "页面不存在")
    return FileResponse(image)


@app.post("/api/jobs/{job_id}/structure")
def start_structure(job_id: str):
    path = job_dir(job_id)
    with _structure_lock:
        meta = read_meta(path)
        if job_id in _structure_jobs or meta["structure"].get("running"):
            raise HTTPException(409, "正在生成结构化草稿")
        meta = update_meta(path, lambda current: current.update(
            structure={"running": True, "error": "", "finishedAt": ""}))
        _structure_jobs.add(job_id)
    try:
        threading.Thread(target=run_structure, args=(job_id,), daemon=True).start()
    except Exception:
        with _structure_lock:
            _structure_jobs.discard(job_id)
        update_meta(path, lambda current: current["structure"].update(running=False))
        raise
    return job_summary(path)


@app.get("/api/jobs/{job_id}/items")
def get_items(job_id: str):
    return {"items": read_items(job_dir(job_id))}


@app.put("/api/jobs/{job_id}/items")
def save_items(job_id: str, body: dict):
    path = job_dir(job_id)
    items = sanitize_items(body.get("items", []))
    if not items:
        raise HTTPException(400, "题目列表为空或格式不正确")
    write_items(path, items)
    return {"items": items}


@app.get("/api/jobs/{job_id}/export")
def export_items(job_id: str):
    path = job_dir(job_id)
    items_file = path / "items.json"
    if not items_file.exists():
        raise HTTPException(404, "还没有生成结构化草稿")
    meta = read_meta(path)
    name = Path(meta["fileName"]).stem + ".json"
    return FileResponse(items_file, filename=name, media_type="application/json")


app.mount("/", StaticFiles(directory=ROOT / "static", html=True), name="static")
