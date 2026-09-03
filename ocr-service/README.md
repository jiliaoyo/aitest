# 本地 OCR 导入服务（ocr-service）

把扫描版 PDF 变成可导入题库的 JSON：**本地 DeepSeek-OCR 识别 → 人工逐页审核 → 文本 LLM 结构化 → 人工编辑题目 JSON → 导出**。
远程视觉模型一次都不调用；唯一的外部调用是把审核后的纯文本发给文本 LLM 做结构化（成本很低，也可以指向本地模型）。

## 启动

```bash
./run.sh        # 等价于 ../.venv-ocr/bin/python -m uvicorn app:app --host 127.0.0.1 --port 8787
```

打开 http://127.0.0.1:8787 。首次 OCR 时会加载本地 DeepSeek-OCR 模型（`~/.cache/huggingface`，mlx-vlm 0.3.10），需要等一两分钟。

## 配置

复制 `.env.example` 为 `.env`，填写文本 LLM 的 `LLM_BASE_URL` / `LLM_API_KEY` / `LLM_MODEL`（OpenAI 兼容接口）。
不配也能用：OCR 和逐页审核不依赖它，只有「生成结构化草稿」会报未配置错误。

## 流程

1. **新建识别任务**：上传 PDF，填默认级别/科目代码（对应后端 `exam_levels.code` / `subjects.code`，如 `n5` / `grammar`）。服务用 pymupdf 把每页渲染成 PNG。
2. **识别未识别页**：后台逐页 OCR，识别结果存为 `data/<job>/pages/pageNNN.txt`；退化输出（重复行）会自动用更高的 repetition_penalty 重试一次。单页失败可稍后「重新识别本页」。
3. **逐页审核**：左边页列表，中间原图，右边可编辑文本；「保存并确认本页」后该页进入结构化候选。
4. **生成结构化草稿**：把已确认页的文本按 `STRUCTURE_CHUNK_CHARS` 分块发给文本 LLM（prompt 见 `prompt_structure.md`），合并为题目草稿。
5. **题目草稿**：整体 JSON 编辑（保存时自动过滤多余字段，保证与后端导入格式一致）。
6. **导出 JSON**：下载后到管理端「导入任务」页上传，直接生成待审核草稿（不经过后端 AI 结构化），逐题审核后发布上架。

## 导出格式

`{"items": [...]}`，每个 item 字段：`rawExcerpt, materialKey, type, stem, options[{id,label,text}], materialTitle, materialContent, levelCode, subjectCode, difficulty, knowledgePointNames, sourceAnswer, aiSuggestedAnswer, anomalies`。
知识点用名称表示，后端按（名称 + 级别）解析，匹配不上只记入 anomalies 不会拒导。

## 注意

- 数据都在 `data/` 下，删任务直接删对应目录。
- 一次只跑一个 OCR 线程（模型独占 GPU）；结构化是独立线程，可以和 OCR 并行。
- `.venv-ocr` 的 mlx-vlm 必须保持 0.3.10，升级会导致 DeepSeek-OCR 输出乱码。
