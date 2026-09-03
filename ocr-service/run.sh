#!/bin/zsh
# 启动本地 OCR 导入服务（必须用大模型所在的 .venv-ocr）
cd "$(dirname "$0")"
exec ../.venv-ocr/bin/python -m uvicorn app:app --host 127.0.0.1 --port 8787
