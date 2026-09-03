# AGENTS.md — AI 协作指引

本文件给在本仓库工作的 AI 编码代理(以及新加入的人类)提供必要背景、约束和常用命令。改动代码前请先读完「硬性约束」一节。

## 项目是什么

「AI 刷题」:一个 JLPT(日语能力考试)刷题平台。核心产品理念见 `docs/` 下的四份规范文档,**改动设计前先读对应文档**:

| 文档 | 内容 |
|---|---|
| `docs/AI刷题App-产品需求文档.md` | 产品需求:批量刷题、延迟反馈、AI/权威答案分层 |
| `docs/前端框架与设计规范.md` | Vue 3 + TS + Vite,禁用 UI 组件库,oklch 设计 token |
| `docs/后端框架结构规范.md` | Go 模块化单体 + PostgreSQL + 同库 worker |
| `docs/AI协作交接提示词.md` | 阶段划分与交接约定 |

### 目录结构

```
backend/          Go 1.24 模块化单体(module 名 github.com/aishuati/backend)
  migrations/     goose 格式迁移(用自带的 cmd/migrate 跑,不是 goose CLI)
  internal/       auth / catalog / content / practice / learning / jobs / ai / httpapi / store / config / app
  cmd/            server / worker / migrate / seed
  api/openapi.yaml 全部 Phase-1 接口契约
front-web/        Vue 3 + TS(strict) + Vite + vue-router,无 Pinia(shallowRef session)
docs/             上述规范文档
pdf/              蓝宝书 N4N5 扫描版(题库来源,见「PDF 题库导入管线」)
scripts/          OCR / 解析 / 建库脚本(见下文)
```

## 环境与常用命令

依赖:Go 1.24+、Node(前端)、本机 PostgreSQL。两个 Python venv:`.venv/`(pymupdf 等工具)、`.venv-ocr/`(mlx-vlm==0.3.10 + DeepSeek-OCR 模型,**必须锁 0.3.10**,0.6.x 与该模型不兼容会输出乱码)。

```bash
# 后端(数据库默认 ai_shuati_dev,可用 PG_URL 覆盖)
cd backend
make migrate   # 应用迁移
make seed      # 写入演示数据(幂等,已有 jlpt 考试则跳过)
make dev       # 迁移 + seed + 启动 API(内嵌 worker),监听 :8080
make test      # go test ./...
go build ./... # 编译检查

# 前端
cd front-web
npm run dev        # Vite dev server :5173,代理 /api -> :8080
npm run build      # vue-tsc 严格检查 + 构建
npm run test       # vitest
```

演示账号:`admin@example.com / [local seed password]`(管理员)、`learner@example.com / [local seed password]`。

## 硬性约束(违反等于返工)

1. **术语红线:产品里只有"刷题/答题/提交本批练习",绝对不允许出现"交卷""做卷子"这类措辞。** 这是用户明确的措辞要求,适用于所有 UI 文案、注释、文档、API 字段名设计。`试卷`一词只允许在"题目来源是书籍/真题"的语义下出现。
2. **答案隔离**:`answer_keys` 是私有表,任何面向学习者的接口(尤其是 pre-submit)不得泄漏答案字段。前端已有一份针对泄漏词的 DTO 测试(`front-web/tests/`),改 DTO 后要跑。
3. **题目版本不可变**:改题 = 新建 `question_versions` 版本 + 迁移 `published_version_id`/`current_version_id` 指针,禁止原地 UPDATE 题干/选项。
4. **答案分层**:`grading_results` 里确定性判定与 AI 判定分来源;`confirmed` 统计只含确定性+权威答案,AI 结果单独呈现且必须标注"AI 判定(可能有误)"。accuracy 类统计是可重建的缓存,一律通过 `rebuild_user_knowledge_stats` 任务重算,不要在答题路径上直接 UPDATE。
5. **无 ORM**:pgx v5 手写 SQL。注意两个已踩过的坑:
   - SQL 里出现未使用的 `$n` 参数 pgx 直接报错(哪怕逻辑上想复用 CTE);
   - data-modifying CTE 共享同一快照,`WITH ... INSERT ... UPDATE 同表` 读不到新行,拆成多条语句。
6. **前端禁用 UI 组件库**;样式用 `front-web/src/styles/tokens.css` 的 oklch token;触控目标 ≥44px。
7. 迁移只增不改:已应用的迁移文件不允许修改,需要变更就新增一个迁移文件。
8. 写操作接口都要过 Origin 校验 + 会话 Cookie(HttpOnly),新增公开路由要显式在 `internal/app/app.go` 挂载。

## PDF 题库导入管线(scripts/,进行中)

目标:把 `pdf/蓝宝书N4N5.pdf`(210 页扫描版,华东理工《蓝宝书》N5N4 文法,ISBN 978-7-5628-3205-8,官方自述 537 基础练习 + 510 实战练习)的 20 个单元练习题提取成符合后端 schema 的 SQL。

流程与脚本(均为一次性管线,可重跑):

1. **抽页**:`.venv/bin/python` + pymupdf,书页 = PDF 页 - 12;直接抽取内嵌原图(2096×2941 JPEG)到 `/tmp/n4n5/native/`。
2. **OCR**:`scripts/ocr_batch.py`(用 `.venv-ocr/` 跑,mlx-vlm 0.3.10 + `mlx-community/DeepSeek-OCR-8bit`,模型在 `~/.cache/huggingface`)。含空表格的页面易退化成重复 token,`scripts/ocr_retry.py` 用 `repetition_penalty=1.2→1.35→温度0.2→半页裁切` 自动重试;detokenizer 的 `KeyError('｜')` 由 `scripts/ocr_fixup.py` 的 BPE 类补丁兜底。输出 `/tmp/n4n5/ocr/bookNNN.txt`。
3. **清洗**:`scripts/ocr_clean.py` 去掉开头/结尾的退化 token 行。
4. **解析**:`scripts/parse_questions.py` → `/tmp/n4n5/parsed/questions.json` + `report.txt`。题型映射:表格题→`short_answer`(reference),共享选项框/四选一→`single_choice`,计数词·助词填空→`fill_blank`,并替题(もんだい2実戦)→`single_choice`(题干用参考句还原 `＿★＿＿` 槽位)。
5. **并替题 ★ 定位**:`scripts/star_layout.py` 像素分析(下划线段 + ★ 实心块),与答案页【参考句】互相验证;Vision 行级坐标辅助脚本 `scripts/ocr_lines.swift`(postgresOS Vision,日语;注意 postgresOS Vision 对这本书的扫描质量识别很差,只能用于坐标,不能用于内容)。

已知进度:OCR 全部 102 页完成,答案页中 N5 全部 + N4 大部分已人工校对转录(答案是最关键数据,OCR 不稳的页一律看图手抄)。解析器仍在迭代(目标 ≈1047 题)。

`/tmp/n4n5/` 是临时目录,重建方法:重新执行 1-2 步即可,人工转录的答案页内容以 `git grep` 能搜到的 `cat > /tmp/...` 无从恢复——所以**这些转录内容如有需要应挪进仓库**(暂存于 ocr txt,后续随 SQL 一起归档)。

## 代码风格

- Go:标准 gofmt;错误要包一层上下文;JSON 日志用 slog;注释解释「为什么」而不是「做什么」。
- Vue SFC 全部 `<script setup lang="ts">`;API 调用统一走 `src/api/client.ts` 的 `request<T>`(自动处理 401/ApiError);状态文本用 `src/app/format.ts` 的映射,不要在组件里硬编码。
- 提交信息/PR 描述用中文,说明「改了什么、为什么」。

## 测试底线

- 后端:`make test` 全绿(含 grading 纯函数表测、practice DTO 泄漏检查)。
- 前端:`npm run test`(vitest,jsdom,注意 `tests/setup.ts` 里有 Node 的 localStorage shim,别删)+ `npm run build`(vue-tsc strict 必须零错误)。
- 涉及刷题闭环的改动,用 `curl`/HTTP 走一遍:create session → autosave → submit(幂等键)→ result 的冒烟,再下结论。
