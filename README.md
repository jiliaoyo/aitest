# AI 刷题 App

以“批次练习、延迟反馈、长期薄弱点追踪”为核心的 JLPT（首发）刷题平台。

- 后端：Go 模块化单体 + PostgreSQL + 同库异步 worker（`backend/`）
- 前端：Vue 3 + TypeScript + Vite，无 UI 框架依赖（`front-web/`）
- 契约：`backend/api/openapi.yaml`（JSON camelCase、枚举 snake_case、游标分页、统一错误结构）
- 需求与规范：`docs/`（PRD、前端规范、后端规范）

## 已实现范围（规范中的阶段 1 + 学习档案 + 管理端基础）

- 账号：邮箱注册/登录/退出/找回密码；bcrypt + SHA-256 哈希的 opaque session token（HttpOnly Cookie）；登录限流；learner/admin 角色。
- 题库：来源/章节、材料版本、题目不可变版本、知识点多对多、**独立私有答案表 answer_keys**（学习端查询根本不连接该表）。
- 刷题闭环：可用题量查询 → 创建批次（综合/知识点/错题重练，优先最近未做过的题）→ 自动保存（单飞 + 本地草稿恢复）→ 标记待检查 → 携带全部最终答案 + `Idempotency-Key` 幂等提交 → 确定性判分（单选/多选/填空归一化/未答规则）→ 分层结果页（已确认正确率 vs AI 判定 vs 待分析）→ 历史/错题本。
- 学习档案：`user_knowledge_stats` 由原始判分全量重算（可重建）；仪表盘推荐使用可解释规则（近 30 天已确认作答 ≥5 题 → 正确率升序 → 连错降序）；知识点列表/详情 + 专项练习。
- 异步任务：PostgreSQL 任务表 + worker（`FOR UPDATE SKIP LOCKED`、租约、指数退避、失败保留错误摘要）；AI 无答案题判定 / AI 解析 / 统计重算三类处理器。
- AI：OpenAI 兼容 HTTP 客户端、版本化 prompt（`go:embed`）、严格 JSON 解码、`ai_runs` 审计；**AI 不可用时确定性判分完全不受影响**（失败题标 failed，批次进入 analysis_failed）。
- 管理端：内容概览、题目 CRUD（编辑产生新版本、发布切换 `published_version_id`、下架保留历史快照）、知识点/来源管理、举报处理、`audit_logs` 审计。
- 安全：角色中间件 + SQL 内带 user_id 条件（越权一律 404）、非 GET 校验 Origin、请求体大小限制 + 严格 JSON 解码、结构化访问日志与 request_id。

**未实现（规范中的阶段 3，仅保留表结构与边界）**：文件导入/AI 整理/导入审核、扫描件 OCR、AI 补题、邮箱通知。

## 快速开始

```bash
# 1. 后端（需 Go 1.24+ 与 PostgreSQL 16）
cd backend
createdb ai_shuati_dev            # 如已存在可跳过
make migrate                      # 应用迁移（兼容 goose 格式）
make seed                         # 演示数据 + 两个账号
make dev                          # 启动 API :8080（内嵌 worker，RUN_WORKER=true）

# 2. 前端（需 Node 20+）
cd ../front-web
npm install
npm run dev                       # http://localhost:5173（/api 代理到 :8080）
```

演示账号：

| 角色 | 邮箱 | 密码 |
| --- | --- | --- |
| 学习者 | learner@example.com | [local seed password] |
| 管理员 | admin@example.com | [local seed password] |

## 常用命令

```bash
cd backend
make test      # Go 测试（判分表驱动 + DTO 泄漏防护）
go build ./... # 编译

cd front-web
npm test       # Vitest（自动保存竞态/草稿恢复 + 结果分层展示）
npm run build  # vue-tsc 严格类型检查 + 产物构建
```

独立 worker（生产形态）：`make worker`；API 与 worker 共用领域代码，仅进程分离。

## 关键设计边界（与规范一一对应）

1. **提交前不泄漏**：答题前 DTO 由独立结构体构造（`practice.PreSubmitSession`），并有单元测试断言序列化结果不含任何答案相关字段。
2. **幂等提交**：交卷事务内 `SELECT ... FOR UPDATE` 锁批次 → 校验幂等键与请求体哈希 → 覆盖最终答案 → 确定性判分 → AI 任务入队（同事务）。
3. **历史可复现**：`practice_items` 引用不可变 `question_version_id`；编辑创建新版本，发布切换 `published_version_id`，旧批次仍读旧版本。
4. **答案隔离**：`answer_keys.authority` 仅 `official` / `human_verified`；AI 结果只写 `grading_results`，永不回写答案表。
5. **统计可重算**：正式正确率只聚合确定性 + 权威答案；AI 判定单独计数，不混入。
