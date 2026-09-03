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
ocr-service/      本地 OCR 导入服务(扫描 PDF → 审核 → 结构化 JSON,见「扫描书导入」)
```

## 环境与常用命令

依赖:Go 1.24+、Node(前端)、本机 PostgreSQL。两个 Python venv:`.venv/`(pymupdf 等工具)、`.venv-ocr/`(mlx-vlm==0.3.10 + DeepSeek-OCR 模型,**必须锁 0.3.10**,0.6.x 与该模型不兼容会输出乱码)。后端启动时自动读取当前目录的 `.env`，从仓库根目录启动时也会读取 `backend/.env`；已有进程环境变量优先。开发环境内嵌 worker 默认开启，生产环境使用独立 `cmd/worker`。

AI 配置使用 `AI_BASE_URL`、`AI_API_KEY`、`AI_MODEL`、`AI_TIMEOUT`。不要把真实 API key 写入日志、提交记录或聊天内容。

```bash
# 后端(数据库默认 ai_shuati_dev,可用 PG_URL 覆盖)
cd backend
make migrate   # 应用迁移
make seed      # 写入演示数据(幂等,已有 jlpt 考试则跳过)
make dev       # 启动 API(开发环境默认内嵌 worker),监听 :8080
make server    # 启动 API；RUN_WORKER 可显式覆盖默认值
make worker    # 启动独立 worker
make test      # go test ./...
go build ./... # 编译检查

# 前端
cd front-web
npm run dev        # Vite dev server :5173,代理 /api -> :8080
npm run build      # vue-tsc 严格检查 + 构建
npm run test       # vitest
```

演示账号:`admin@example.com / [local seed password]`(管理员)、`learner@example.com / [local seed password]`。

## Docker 部署约定

- 根目录 `compose.yaml` 定义 `postgres`、`backend`、`worker`、`frontend` 四个服务；后端镜像默认启动 API，`/app/worker` 用于独立 worker，`/app/migrate` 用于迁移，`/app/seed` 用于演示数据初始化。
- 生产部署目录为 `<deployment-directory>`。后端映射宿主机 `9090 -> 8080`，前端 Nginx 映射宿主机 `9091 -> 80`；公网只通过现有入口 Nginx 的 `443` 虚拟主机按域名反代到 `127.0.0.1:9091`。宿主机 `80` 保留给现有服务，不要由本项目绑定。
- PostgreSQL 持久卷固定为 `ai_shuati_postgres_data`，上传文件卷固定为 `ai_shuati_uploads`。更新服务禁止使用 `docker compose down -v`；涉及数据同步前先做 `pg_dump` 备份。
- 首次初始化顺序：启动 `postgres` → `/app/migrate` → 挂载 `scripts/data/knowledge_points_n4n5.json` 执行 `/app/seed` → 导入 `backend/questions/blue_questions.sql` 和 `backend/questions/redblue_questions.sql` → 启动 `backend`、`worker`、`frontend`。题库 SQL 可幂等重跑，但不要执行题库重置脚本覆盖线上运行数据。
- 后端 `.env` 只放服务器环境变量和密钥，不提交、不打印；AI 服务必须同时配置 `AI_BASE_URL`、`AI_API_KEY`、`AI_MODEL`。前端 `nginx.conf` 使用 Docker DNS 动态解析 `backend`，避免后端容器重建后反代缓存旧 IP。
- 常规更新：`sudo docker compose build backend frontend && sudo docker compose up -d --force-recreate backend worker frontend`；完成后检查 `/health/live`、前端 `/api` 反代和容器健康状态。

### 线上更新流程

1. 本地先运行 `cd backend && go build ./...`、`cd front-web && npm run build`。
2. 将代码同步到 `<deployment-directory>`，排除所有 `.env`、`node_modules/`、`dist/`、`pdf/`、`.venv*/`；不要使用会删除远端文件的 `rsync --delete`。
3. 更新前备份数据库：`backup=backups/pre-update-$(date +%Y%m%d%H%M%S).dump && sudo docker compose exec -T postgres pg_dump -U ai_shuati -d ai_shuati --format=custom > "$backup" && chmod 600 "$backup"`。
4. 后端或迁移有变化时执行 `sudo docker compose build backend worker`，再执行 `sudo docker compose run --rm backend /app/migrate -dir /app/migrations`，最后 `sudo docker compose up -d --no-build --force-recreate backend worker frontend`。
5. 只有前端变化时只执行 `sudo docker compose build frontend && sudo docker compose up -d --no-build --force-recreate frontend`；只改服务器 AI 配置时修改远端 `.env` 后重建 `backend`、`worker`，不需要重新导入数据。
6. 更新后检查 `sudo docker compose ps`、`curl -fsS http://127.0.0.1:9090/health/live`、前端 `/api` 反代和 443 域名访问。禁止执行 `docker compose down -v`，禁止删除 `ai_shuati_postgres_data` 或 `ai_shuati_uploads`。

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
9. AI 分析按批次执行:新批次只入队一个 `analyze_practice_session_ai`，一次请求同时处理整批总结、未缓存题目解析和必要的 AI 判分；禁止恢复逐题 AI 解析入队。
10. 题目级 AI 解析写入 `question_ai_explanations`，以不可变的 `question_version_id` 为键；题目版本变化后才重新生成。批次结果写入 `practice_sessions.ai_summary`，学习端通过 `aiAnalysis` 展示。

## PDF 题库导入管线(scripts/,已完成首轮建库)

目标:把 `pdf/蓝宝书N4N5.pdf`(210 页扫描版,华东理工《蓝宝书》N5N4 文法,ISBN 978-7-5628-3205-8,官方自述 537 基础练习 + 510 实战练习)的 20 个单元练习题提取成符合后端 schema 的 SQL。

流程与脚本(均为一次性管线,可重跑):

1. **抽页**:`.venv/bin/python` + pymupdf,书页 = PDF 页 - 12;直接抽取内嵌原图(2096×2941 JPEG)到 `/tmp/n4n5/native/`。
2. **OCR**:`scripts/ocr_batch.py`(用 `.venv-ocr/` 跑,mlx-vlm 0.3.10 + `mlx-community/DeepSeek-OCR-8bit`,模型在 `~/.cache/huggingface`)。含空表格的页面易退化成重复 token,`scripts/ocr_retry.py` 用 `repetition_penalty=1.2→1.35→温度0.2→半页裁切` 自动重试;detokenizer 的 `KeyError('｜')` 由 `scripts/ocr_fixup.py` 的 BPE 类补丁兜底。输出 `/tmp/n4n5/ocr/bookNNN.txt`。
3. **清洗**:`scripts/ocr_clean.py` 去掉开头/结尾的退化 token 行。
4. **解析**:`scripts/parse_questions.py` → `/tmp/n4n5/parsed/questions.json` + `report.txt`。题型映射:表格题→`short_answer`(reference),共享选项框/四选一→`single_choice`,计数词·助词填空→`fill_blank`,并替题(もんだい2実戦)→`single_choice`(题干用参考句还原 `＿★＿＿` 槽位)。
5. **并替题 ★ 定位**:`scripts/star_layout.py` 像素分析(下划线段 + ★ 实心块),与答案页【参考句】互相验证;Vision 行级坐标辅助脚本 `scripts/ocr_lines.swift`(postgresOS Vision,日语;注意 postgresOS Vision 对这本书的扫描质量识别很差,只能用于坐标,不能用于内容)。

已知结果:蓝宝书 986 题 / 986 答案,红蓝宝书 1000 题 / 1000 答案,来源章节 213 个。重建基线会清理书籍题目及其依赖练习记录并保留自建演示练习；用户后来创建的练习记录属于运行数据,不作为题库建库基线。`backend/questions/*.sql` 只接受通过严格校验的记录；再次重建前先执行清理脚本。

原始 OCR 中间产物仍在 `/tmp/n4n5/`，不可作为唯一事实源；人工复核记录、异常和修正放在 `scripts/data/`，生成 SQL 和已导入数据库是可复现交付物。

## 扫描书导入(ocr-service/,通用管线)

针对任意扫描版 PDF 的常规导入路径:`ocr-service/` 是用 `.venv-ocr` 运行的 FastAPI 服务(`./run.sh`,127.0.0.1:8787),流程为 上传 PDF → pymupdf 抽页 → 本地 DeepSeek-OCR 逐页识别 → 网页端逐页审核编辑 → 文本 LLM(非视觉模型,`.env` 配 `LLM_BASE_URL/LLM_API_KEY/LLM_MODEL`)结构化 → 题目 JSON 再编辑 → 导出。

导出的 `{"items":[...]}` 直接上传到管理端「导入任务」页:后端 imports 模块**只接受这种结构化 JSON**(不在刷题项目里做 PDF/DOCX 等格式识别,也没有 AI 结构化和失败重试),解析 `levelCode`/`subjectCode`/`knowledgePointNames` 为内部 ID(知识点名称匹配不上只记入 anomalies,不拒导),复用全部草稿校验后同步生成待审核条目(review_ready),之后走既有的逐题审核 → 发布流程。JSON 题目格式定义在 `backend/internal/imports/json_import.go`,字段增删要同步 `ocr-service/app.py` 的 `ALLOWED_ITEM_KEYS`。

## 刷题与 AI 结果约定

- 创建练习支持 `sourceId` 数据来源筛选；不传表示全部来源。默认 `source_order` 按 PDF 来源顺序，`random` 才随机。
- 答题前 DTO 永不返回答案；答题后结果通过 `sourceSectionName` 展示来源章节，通过 `aiAnalysis` 展示整批 AI 总结。
- AI 私有出题接口是 `POST /ai-practice-sessions`：默认 20 题，`generationMode` 支持 `memory`（账号记忆）和 `level`（指定 JLPT 级别），`questionType` 支持混合、单项选择、多项选择、填空和简答，`difficulty` 支持 `easy`、`normal`、`hard`、`mixed`。
- AI 按级别出题时，服务端从所选级别的已发布知识点中随机取样，并把级别代码（如 `n5`）传给模型；AI 返回的题型和难度会在服务端再次校验。
- `question_ai_explanations` 只缓存与题目本身无关用户作答的权威题目解析；主观题或依赖具体作答的 AI 判定不写入题目缓存。
- 批次 AI 请求应去重共享材料，并严格校验模型返回的题目 ID、结论和文本长度；失败任务保留在 jobs 中，不能静默写入残缺结果。
- 练习历史和错题本支持软删除：历史通过 `DELETE /practice-sessions/{id}` 隐藏整批，错题本通过 `DELETE /wrong-items/{id}` 隐藏单题；`practice_sessions.deleted_at` 和 `practice_items.deleted_at` 只影响用户视图与错题重练，不删除原始作答、成绩或统计事实。正在生成的练习不能删除，答题中的练习可以删除。
- `/settings` 是个人中心，当前支持默认级别、学习记忆、退出登录和修改密码；修改密码使用 `POST /auth/change-password`，校验当前密码后更新 bcrypt 哈希并撤销其他会话，当前会话继续有效。
- 最新迁移为 `0014_practice_soft_delete.sql`；新增字段或缓存策略只能追加迁移。

前端排版约定：AI 文本使用 `src/app/format.ts` 的 `formatAIText` 和 `src/styles/base.css` 的 `.ai-text`；选项行统一使用 `src/styles/utilities.css` 的 `.option-row`，其单选框/复选框已做共享的垂直居中处理，不要在页面内重复覆盖。

## 代码风格

- Go:标准 gofmt;错误要包一层上下文;JSON 日志用 slog;注释解释「为什么」而不是「做什么」。
- Vue SFC 全部 `<script setup lang="ts">`;API 调用统一走 `src/api/client.ts` 的 `request<T>`(自动处理 401/ApiError);状态文本用 `src/app/format.ts` 的映射,不要在组件里硬编码。
- 全局原生 `select` 样式在 `src/styles/base.css` 维护，使用项目 token 和自定义箭头，不在单个页面恢复浏览器默认外观。
- 提交信息/PR 描述用中文,说明「改了什么、为什么」。

## 测试底线

- 后端:`make test` 全绿(含 grading 纯函数表测、practice DTO 泄漏检查)。
- 前端:`npm run test`(vitest,jsdom,注意 `tests/setup.ts` 里有 Node 的 localStorage shim,别删)+ `npm run build`(vue-tsc strict 必须零错误)。
- 涉及刷题闭环的改动,用 `curl`/HTTP 走一遍:create session → autosave → submit(幂等键)→ result 的冒烟,并确认批次 AI 任务数量为 1、结果页可轮询到 `aiAnalysis`。
