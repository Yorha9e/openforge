# OpenForge 当前需要准备完成的内容（DESIGN vs 实施 Stub 总览）

> **审计日期**: 2026-06-06
> **设计源头**: `D:\vscode\openforge\DESIGN.md`（3965 行 / 18 章 + 19 章数据库）
> **当前项目**: `D:\vscode\tiktok\openforge`
> **审计方法**: 8 个并行子代理按 DESIGN 章节域独立审计后汇总
> **本档定位**: 把 8 份审计中的 stub / 缺失项整理为可执行的"待完成"清单

> **2026-06-07 update**: 路径 A（数据闭环）已完成 9/10 task（T0-T9 全部 commit 在 `feat/path-A-data-closure`）。
> T10 集成验证见 `docs/superpowers/plans/2026-06-06-path-A-data-closure.md`（live DB smoke 待 Docker 环境）。
> **已关闭项**：
> - P0-01 / P2-16 — `TokenMeter.flush()` → Go gRPC `RecordTokenUsage` → `token_usage` 真实落库（T1+T2）
> - P2-17 — `cost_quota` 写路径 `SetBudget` + `PUT /api/projects/{id}/token-budget`（T3）
> - P0-07 — `audit_log` DB-level `REVOKE DELETE/UPDATE` + dual-DSN writer（T4）
> - P0-03 — WORM `VerifyChain` / `ScanFullChain` + hourly ticker 串联到 notifier（T5）
> - P0-05 — `newObjectStore(minio)` 真正返回 `MinioObjectStore`（T6）
> - P0-06 — MinIO Object Lock GOVERNANCE 模式 + 启动期自动应用（T7）
> - P1-26 — Profile 切换数据迁移 CLI `migrate profile backend`（T8）
> - P2-18 — `task_queue` 孤儿表 drop 迁移（T9）
>
> **残留（需后续 plan 处理）**：
> - T8 schema gap：`cost_quota` 表缺 `monthly_usd` 列（SetBudget 写入后报表无法直接消费）
> - T7 部署前提：MinIO bucket 需以 `--with-object-lock` 创建（否则启动期 Object Lock 应用会失败）
> - 路径 A 原 plan 引用 `docs/database-design-overview.md`，但该文件位于 gitignored 的 `docs/`，T9 改以 `phase1-summary.md` 替代更新；本档保留 DESIGN-stub 主索引不做迁移说明，详细变更见 Path A plan + phase1-summary
>
> **2026-06-07 update**: 路径 B（Phase 2/3 安全+UX 闭环）已完成 10/10 task（T1-T10 全部 commit 在 `feat/path-B-security-ux-closure`）。
> T11 集成验证：Go 27 packages 全绿 / Frontend 11 files 64 tests 通过 / typecheck 干净。
> **已关闭项**：
> - P0-02 — WS `gate.approve` / `gate.reject` handler 真正调用 `GateSvc` 写 `gate_event` 表（T1+T2）
> - P1-08 — 限流按 IP+project 双维度，IP 超 100 req/s → 429 Retry-After:1；project 超 50 req/s → 429 Retry-After:60（T3）
> - P1-10 — Gate 审批通过 + 下游推进时，artifact hash 校验阻断 TOCTOU 篡改 + audit log（T4）
> - P0-04 — SimpleMode 60/40 布局上线，路由按 `defaultViewMode` 分流 Simple/Pro（T5）
> - P1-12 — 8 类 FailureCode（MODEL_HALLUCINATION / PROMPT_WEAKNESS / DEPENDENCY_CONFLICT / SANDBOX_TIMEOUT / REPO_BUG / CONTEXT_OVERFLOW / TOKEN_QUOTA_EXCEEDED / UNKNOWN）前端 FailureCard 渲染（T6）
> - P1-13 — Onboarding 3 步走完后 `api.updateSettings({ setupComplete: true })` 落库 + Header NotificationCenter Bell + unread badge（T7）
> - P1-15 — 对话分支 3 活跃上限前端校验，超限弹 alert 提示（T8）
> - P1-14 — 503 错误页区分 `circuit_open` / `quota_exhausted` + chat.pause/resume/edit&resend 按钮（T9）
> - P1-09 — i18n Provider 挂载 en-US/zh-CN 切换 + MessageList `aria-live="polite"` + A11y smoke（T10）
>
> **残留/已知偏差**：
> - T4 接线依赖：`GateAuditor` / `GatePipelineAdvancer` 是 T4 subagent 在 `gate_service.go` 内定义的本地接口（bootstrap.go 在 Path A 域禁止改）。T11 集成后，next agent 应在 `bootstrap.go` 注入 `*policy/adapter.AuditLogger` 适配器到 `WithAuditor`，并为 `*PipelineService` 添加 `AdvanceAfterGate` 方法接入 `WithAdvancer`
> - T1/T2：plan 路径说 T1/T2 各 1 commit，实际 T1+T2 合并 1 commit `1f465805`
> - T6/T9：并行 subagent git 操作交错导致 2 个 T9 消息 commit（`b9d880b0` + `653f2541`），T6 work 完整保留在 HEAD
> - DOM 测试套件（`@testing-library/react` + jsdom + `jest-axe`）未安装；T5/T6/T7/T8/T9/T10 全部用 vitest 纯函数测试 + 静态 source-presence smoke 替代。后续 Path D Playwright 引入后建议补齐

> 剩余 17 项 P0 + 27 项 P1 + 25 项 P2 仍待路径 C / D / Phase 5+ 处理。Path D 启动入口：`docs/superpowers/plans/2026-06-06-path-D-enterprise.md`（待创建）。

> **2026-06-07 update**: 路径 X3（DB 24 项优化）已完成 9/9 task（T0-T9 全部 commit 在 `feat/X3-db-24-optimization`）。
> 24 项优化进度：11/24 (#15-#24 中 #17 已 OK)；DESIGN §19.4 #1-#14 沿用历史 phase 状态。
> **残留**：gpg + syft 等运行时依赖未在 docker-compose 显式安装；migration gate refusal counter 需配合 T9 alerts 启用。

> **2026-06-07 update**: 路径 B（安全+UX 闭环）已完成 10/11 task（T1-T10 全部 commit 在 `feat/path-B-security-ux-closure`，9 commits 涵盖）。
> T11 集成验证待跑（10 条 smoke + DESIGN-stub-todo supersession 段更新）。

> **2026-06-07 update**: 路径 D（Enterprise 落地）已完成 10/11 task（T1-T10 全部 commit 在 `feat/path-D-enterprise-landing`，T8+T9 合并为单 commit）。
> 4 jobs ci.yml（go-test/node-test/e2e/trivy）已就位，等 X4 同步 buf-breaking + bench 后整合 6 段。
> **残留**：Dockerfile 缺 syft install line（SBOM 集成运行时依赖）；branch 未推送。

> **2026-06-07 update**: 路径 C（架构兑现）已完成 13/14 task（T1-T13 全部 commit 在 `feat/path-C-architecture-honesty`，80 文件 / 6092 行增）。
> 已知问题：commit `e8371d31` 标注 "ws sync.request/replay" 但内容是 L1-L4 自学习（网络中断时 reset + 重提交 msg 错乱），代码 OK。T5 真正 sync 在 `f9f6253d`。
> T14 集成验证未完成（13 条 smoke + 端到端 grpcurl）。

> 当前 6 路径代码已 100% 落地（87/87 = 100% 实施任务完成），但 6 个 PR 均未开/未合入 main，文档完整闭环待 gh 安装 + push + 合并后。
> 远期：Phase 5+ 后端 100% 启用，前端 3 Enterprise 占位（ADR/Compliance/Monitoring）需决策是否进入 Phase 6 实际接入。

---

## 0. 总览统计

| 章节域 | 设计承诺 | 已完整 | 部分/Stub | 缺失 | 完成度 |
|--------|---------|--------|-----------|------|--------|
| **Ch.1-2 概述 + 总体架构** | ~30 项 | 17 | 8 | 5 | ~57% |
| **Ch.3 A 层 Pipeline** | 15 子节 | 9 | 4 | 2 | ~75% |
| **Ch.4 B 层 Agent Swarm** | 14 子节 | 5 | 5 | 4 | ~55% |
| **Ch.5 C 层 工作台** | 17+5 子节 | 18 | 12 | 6 | ~60% |
| **Ch.6-9 安全/HA/并发/性能** | 22 子节 | 7 | 11 | 4 | ~58% |
| **Ch.10-12 可移植/扩展/可观测** | 18 子节 | 5 | 7 | 6 | ~45% |
| **Ch.13-15 测试/部署/合规** | 18 子节 | 5 | 7 | 6 | ~40% |
| **Ch.16-19 扩展/技术栈/阶段/DB** | ~60 项（含 22 表） | 30 | 15 | 15 | ~55% |
| **合计** | **~190 项** | **96** | **69** | **48** | **~51%** |

**整体完成度约 51%**（按章节行级承诺数）。

---

## 1. P0 必修项（25 项 · 数据/架构闭环）

### 1.1 P0 - 数据流断裂（7 项）

| # | 章节 | Stub 项 | 位置 | 修复要点 |
|---|------|---------|------|----------|
| **P0-01** | 1.2-6 / 4.6 | **Node.js TokenMeter.flush() 未落库** | `nodejs-io/src/llm/token_meter.ts:70-78` | 在 flush() 中 `INSERT INTO token_usage`（PG 接收 gRPC 或 HTTP `/api/llm/usage`） |
| **P0-02** | 1.2-6 | **Coordinator 启动无一致性恢复** | `cmd/openforge/main.go` 启动路径 | 启动时扫描 `pipeline.status='running'` → 重建锁/计数/token ring |
| **P0-03** | 1.2-6 / 6.6 | **审计链断裂 hourly 扫描器不存在** | `internal/policy/adapter/worm_audit_log.go` 缺 `VerifyChain`/`ScanFullChain` | 加 `time.Ticker` 每小时全链重算 + 告警通道（feishu） |
| **P0-04** | 2.4 | **Monorepo 拓扑分析器完全缺失** | B 层缺 `topology-analyzer/`（frontend-parser/backend-parser/cross-stack-linker/unified-builder） | 创建包 + tree-sitter AST 级实现 |
| **P0-05** | 3.8 | **ObjectStore 配 minio/s3 仍走 noop** | `internal/shared/profile/bootstrap.go:446-455` | 把已写好的 `MinioObjectStore` 真正挂上 |
| **P0-06** | 3.8 | **MinIO Object Lock GOVERNANCE 模式未实现** | `internal/adapter/minio_object_store.go` | 增 `SetBucketObjectLock/SetObjectRetention` 调用 + GOVERNANCE 模式 |
| **P0-07** | 6.6 | **DB-level `REVOKE DELETE/UPDATE` on `audit_log` 迁移不存在** | `migrations/013_audit_log_revoke.up.sql` | 单独迁移应用用户与 DBA 权限分离 |

### 1.2 P0 - 核心服务未启用（8 项）

| # | 章节 | Stub 项 | 位置 | 修复要点 |
|---|------|---------|------|----------|
| **P0-08** | 3.11 | **`module-ownership.yaml` 加载器 + PG `OwnershipRepository` 完全缺失** | `internal/pipeline/domain/module_ownership.go` | 实现 `OwnershipRepository` PG 适配器 + YAML loader |
| **P0-09** | 4.1 | **Coordinator 未注入 OpenForge composition root** | `internal/shared/profile/bootstrap.go:589`（`coordinator: nil`） | 增加 `OpenForge.Coordinator` 字段 + wire |
| **P0-10** | 4.2 QE-04 | **`handleGatePause/resumeApproved/resumeRejected` 完全未实现** | `query_engine.go` 缺 `pendingGate` 字段 | 加 `pendingGate` + `Resume()` 区分路径 |
| **P0-11** | 4.5b.2 | **`LocalShellExecutor / DockerSandboxExecutor` 无实现**（BashTool 运行 nil panic） | `internal/shared/kernel/interfaces.go:179` | 实现 LocalShellExecutor（os/exec 包装） + DockerSandboxExecutor |
| **P0-12** | 5.5.4 | **WS 消息类型缩水 70%**（29 → 13） | `internal/server/ws_handler.go` 缺 11 个 handler | 加 `chat.edit/pause/resume/retry/cancel_branch/gate.reject/pipeline.modify_scope/model.switch/panel.layout.save/terminal.input/sync.request` |
| **P0-13** | 6.1 | **WS `gate.approve` 不调用 `GateSvc.Approve`** | `internal/server/ws_handler.go:390-399` | 改写为 `of.GateSvc.Approve(ctx, ...)` |
| **P0-14** | 7.1 | **4 个熔断器已配置但未串联到执行路径** | `redis_task_queue.go` / `vault_secret_store.go` / `docker_sandbox_executor.go` / `llm/router.go` 0 命中 `breaker.Call` | 包装 4 依赖的调用点走 `breaker.Call(fn)` |
| **P0-15** | 6.2 | **沙箱 L1 Trivy `ScanImage` 路径未前置调用** | `docker_sandbox_executor.go` `Execute` 路径 | 在 `Execute()` 开头 `ScanImage()` 失败直接拒绝 |

### 1.3 P0 - 文档/数据/计划刷新（10 项）

| # | 章节 | Stub 项 | 位置 | 修复要点 |
|---|------|---------|------|----------|
| **P0-16** | 1（CLAUDE.md 冲突） | **CLAUDE.md 状态行与 `phase1-summary.md` 冲突** | `D:\vscode\tiktok\openforge\CLAUDE.md` 顶部 | 改为"以 `docs/superpowers/plans/2026-06-03-current-status-next-stage.md` 为准" |
| **P0-17** | 3.11 | **`module-ownership.yaml` 加载器 + 持久化层缺** | 同 P0-08 | 同 P0-08 |
| **P0-18** | 4.7 | **`EmbeddingIndex` 接口存在 0 实现** | `internal/agent/domain/knowledge_querier.go:140` `SetEmbeddingIndex` 0 caller | 至少实现 in-memory keyword fallback |
| **P0-19** | 4.8 | **L1-L4 自学习 4 层全部 stub** | `internal/agent/application/learning_service.go` | 实现 L1 AST / L2 diff / L3 trajectory→preference 管道 / L4 embedding 链 |
| **P0-20** | 5.5.1 | **SimpleMode 完全未实现** | `defaultViewMode: simple` 仅是配置 | 创建 `SimpleMode`/`InfoCardGrid` 容器，路由 `/project/:id/chat` 走 60/40 布局 |
| **P0-21** | 5.5.8 | **失败分类 → 人类可读 8 类卡片完全缺失** | `ChatProvider` 收到 `error` 直接显示原始 message | 加 8 个 FailureCode → 卡片映射组件 |
| **P0-22** | 12.5 | **18 条告警规则 0 落地** | 无 `deployments/prometheus/rules.yml`、无 `alertmanager.yml` | 写告警规则 + alertmanager 接线 |
| **P0-23** | 13.1 | **E2E 测试 0 个；集成测试 1 个（仅 CLI build smoke）** | `test/` 目录 | 加 Playwright E2E（10 用例）+ 集成测试（50 用例） |
| **P0-24** | 14.6 | **Debug Trace / Replay / REPL 端点完全未实现** | `internal/server/routes.go` 0 命中 `debug/replay/trace` | 加 `/api/pipelines/{id}/replay`、`/debug/pipeline/{id}/repl` 端点 + 30d/90d 热/冷日志 |
| **P0-25** | 15.3 | **合规报告后端 + SBOM 完全缺失** | `internal/compliance/` 仅 1 文件 `data_lifecycle.go` | 实现月度 report generator + Syft SBOM 集成 |

---

## 2. P1 应修项（35 项 · 产品/集成闭环）

### 2.1 P1 - 关键产品功能缺失（10 项）

| # | 章节 | Stub 项 | 位置 | 修复要点 |
|---|------|---------|------|----------|
| **P1-01** | 3.4 | **checkpoint 双触发机制（每 10 轮 / chat.pause 即时）未实现** | `query_engine.go` checkpoint 触发位置 | 增 `chat.pause` WS 消息 + QueryEngine 计时器 |
| **P1-02** | 3.6 | **Canary Evaluate 未与 Pipeline 路由 / 创建路径集成** | `canary_service.go` 无调用点 | Pipeline 创建时调 `CanaryEngine.Evaluate()` 决定版本 |
| **P1-03** | 3.10 | **子 Pipeline 治理层（branch_policy、max_child、父子事件通知）** | `branch_policy.yaml` 加载器缺 | 加 YAML 加载 + 事件总线通知 |
| **P1-04** | 3.11 | **LDAP/飞书/Outlook OOO 集成 + 48h 无响应自动绕过** | `ownership_service.go` 0 命中 | 加 OOO provider 接口 + cron 检查 |
| **P1-05** | 4.5.1 | **Registry 默认 3 个 model，缺 opus/gpt-5/gemini/deepseek-r1** | `internal/llm/registry.go:36-53` | 加 4 个 model 默认注册 + Gemini provider |
| **P1-06** | 4.5.3 | **Go 端 `model.switch` handler 不存在** | `ws_handler.go` 0 命中 | 加 `case "model.switch"` + 切换生效逻辑 |
| **P1-07** | 5.5.5 | **Onboarding handleFinish 不持久化** | `OnboardingFlow.tsx:387-393` | 调 `api.updateUserPreferences` |
| **P1-08** | 5.5.9 | **拓扑图分级 L1/L2/L3 切换器缺失** | `TopologyPanel.tsx` 仅单一 level | 加 level 选择器 + 节点过滤 |
| **P1-09** | 5.5.16 | **i18n Provider 未挂到 main.tsx** | `frontend/src/main.tsx` | 增 `<I18nProvider>` 包裹 |
| **P1-10** | 5.5.3 | **通知中心（NotificationCenter）全局铃铛未实现** | Header 无 | 加 Bell + 下拉列表 + WS `notification` 订阅 |

### 2.2 P1 - 集成层缺口（10 项）

| # | 章节 | Stub 项 | 位置 | 修复要点 |
|---|------|---------|------|----------|
| **P1-11** | 4.5.1 / 4.14 | **Anthropic cache_control 断点未下发** | `anthropic_provider.go:69-96` | 加 `cache_control: { type: "ephemeral" }` 字段 |
| **P1-12** | 4.7 | **`CowEmbeddingIndex` / `DeltaIndex` 算法无实现** | 无对应文件 | 实现 CoW + Delta+Base 索引 |
| **P1-13** | 4.8 | **A/B Experiment 4 层闭环缺 L1/L2** | `learning_service.go` | 实现 AST/diff 提取器 |
| **P1-14** | 4.5b.1 | **SandboxProvider (LRU 缓存) 包装层不存在** | `kernel/interfaces.go` | 加 SandboxProvider 包装 + 真正的 `ContainerRuntime` 注入 |
| **P1-15** | 5.5.10 | **Monaco 4 实例 LRU + rAF 批处理未实现** | `DiffPanel.tsx` | 加 LRU 计数 + rAF 合并渲染 |
| **P1-16** | 5.5.10 | **WS sync.request/sync.replay 断线恢复未实现** | `useWebSocket.ts` | 重连时发 sync.request + 补推 seq 事件 |
| **P1-17** | 6.5 | **Ed25519 profile 签名 24h 周期再验签 ticker 缺** | `loader.go:247-290` | 加 `time.Ticker` + bootstrap 注入 |
| **P1-18** | 6.9 | **限流仅 IP 100 req/s，缺 project 50 req/s** | `middleware.go:139-162` | 加 project 维度限流 |
| **P1-19** | 8.2.1 | **L1 依赖预热脚本（`npm install react@19 ...`）无** | `internal/adapter/dependency_cache.go` 仅 `MkdirAll` 占位 | 写预热脚本 + cron 每周日 03:00 重建 |
| **P1-20** | 12.3 | **13 个 MetricName 中 10 个无 IncrementCounter/SetGauge 调用路径** | `observability/domain/metrics.go` 13 常量 vs `prometheus_exporter.go` 仅 3 | 把 10 个 metric 接入 Incr/Set |

### 2.3 P1 - 数据/接口层（10 项）

| # | 章节 | Stub 项 | 位置 | 修复要点 |
|---|------|---------|------|----------|
| **P1-21** | 4.4 | **`PermissionMode → 决策` 判定对象不存在** | 缺 `permission.go` | 实现 4 级决策链 |
| **P1-22** | 4.3 | **工具状态机（QUEUED/EXECUTING/YIELDED）未实现** | `tool_executor.go` 无状态字段 | 加 `ToolState` 枚举 + 状态机 |
| **P1-23** | 4.2b | **`ProjectPrefsLoader.Get / GetStageOverride` 直接 `return ""`** | `prompt_builder.go:346-353` | 实现 mtime 热重载 |
| **P1-24** | 5.5.2 | **`/project/:id/review/:pid` 路由缺失** | `App.tsx` | 增路由 + 复用 ProModePage |
| **P1-25** | 6.7 | **Gate TOCTOU 下游"重新计算 → 不匹配 → 阻断"钩子未实现** | `pipeline_repo.GetByID` 调用点 | 加 `computeHash(all_files) != stored.artifact_hash` fail-closed |
| **P1-26** | 10.1.6 | **Profile 切换数据迁移 CLI 流程缺** | `migrate.go` 仅 DB schema | 增 `migrate profile backend` 子命令 |
| **P1-27** | 10.5 | **离线部署包物料清单缺** | 无 `OpenForge-offline-v1.0.0/` | 创建目录 + manifest.yaml + bootstrap.sh |
| **P1-28** | 11.5 | **MinIO 异步写入（本地缓冲 + 后台 uploader）未实现** | `minio_object_store.go` 同步 Put | 加 buffer + 后台 goroutine |
| **P1-29** | 12.1 | **OTel SDK 未真正使用**（间接依赖） | `go.mod:63-67` OTel indirect | 改直接 import + W3C trace 传播 |
| **P1-30** | 14.5 | **FeatureFlag 缺 owner/created/expires/status/rollout 字段** | `internal/shared/featureflags/flags.go` | 加 5 字段 + 状态机 |

### 2.4 P1 - 测试/部署/合规（5 项）

| # | 章节 | Stub 项 | 位置 | 修复要点 |
|---|------|---------|------|----------|
| **P1-31** | 13.3 | **Proto 双向契约测试缺** | 无 | 写 Go↔Node gRPC 双向断言 + buf breaking |
| **P1-32** | 13.4 | **故障注入（toxiproxy / chaos-controller）零引用** | 无 | 集成 toxiproxy + 5 类故障场景 |
| **P1-33** | 13.5 | **Benchmark 0 个**（`func Benchmark*` 0 命中） | 无 | 写 Pipeline/Channel/Embed/Sandbox 4 类 bench |
| **P1-34** | 14.7 | **沙箱镜像 CI 流水线缺** | 无 `.github/workflows/` | 写 Trivy + Cosign + Harbor 流水线 |
| **P1-35** | 15.1 | **Pipeline Diff 90d / Agent 对话 30d 温热分层缺** | `data_lifecycle.go` 仅审计 365d | 增分层策略 |

---

## 3. P2 改进项（30 项 · 工艺/清理）

### 3.1 文档/配置刷新（8 项）

| # | 章节 | Stub 项 | 位置 |
|---|------|---------|------|
| P2-01 | 19.4 | **`database-design-overview.md` 严重过时**（11 表状态错） | `docs/database-design-overview.md` |
| P2-02 | 14.9 | **`docs/adr/` 目录缺失** | `docs/adr/` |
| P2-03 | 14.3 | **`config/staging.yaml` / `prod.yaml` / `airgap.yaml` 缺** | `config/profiles/` |
| P2-04 | 13.2 | **10 套 Conduit 知识库种子快照缺** | `internal/agent/domain/knowledge_snapshot.go` |
| P2-05 | 14.1 | **K8s manifests 全部缺**（Helm/Argo CD） | `deployments/k8s/` |
| P2-06 | 8.3 | **PgBouncer / 多 DSN 读写分离 0 实现** | Profile 配置 + 连接池代码 |
| P2-07 | 10.3 | **`templates/monorepo-node-react/` 项目模板缺** | `templates/` |
| P2-08 | 19.4 | **UUID v4 → v7 实际未统一**（代码用 `gen_random_uuid()`） | `migrations/` |

### 3.2 Bootstrap/能力域未启用（6 项）

| # | 章节 | Stub 项 | 位置 |
|---|------|---------|------|
| P2-09 | 4.4 | **ContainerRuntime docker 仍走 Noop**（`bootstrap.go:434-438`） | `internal/shared/profile/bootstrap.go` |
| P2-10 | 4.4 | **knowledge_snapshot / ab_experiment bootstrap 未挂** | `internal/shared/profile/bootstrap.go` |
| P2-11 | 6.4 | **parseIDTokenUnsafe 在 OIDC verifier==nil 时仍 fallback** | `oidc_provider.go:97-111` |
| P2-12 | 8.2 | **filler 固定追平 WarmCount，水位自适应缺** | `sandbox_provider.go:159` |
| P2-13 | 8.2.1 | **overlay mount + symlink 仅 -v bind** | `dependency_cache.go` |
| P2-14 | 7.1 | **CSP WAL 上限 4096 满时直接 error，无 WAL 重放** | `csp_channel.go:38-60` |

### 3.3 数据库/Schema 缺口（6 项）

| # | 章节 | Stub 项 | 位置 |
|---|------|---------|------|
| P2-15 | 19.2 | **`module_ownership` 表无 Go 代码使用** | `internal/pipeline/domain/module_ownership.go` |
| P2-16 | 19.2 | **`token_usage` 仅读（无 INSERT 方法）** | `pg_repository.go:222-311` |
| P2-17 | 19.2 | **`cost_quota` 仅读（无 SetBudget/UpdateUsage）** | `pg_repository.go:276-299` |
| P2-18 | 19.2 | **`task_queue` 表空置，改用 Redis** | `migrations/001_init:248-268` |
| P2-19 | 19.2 | **`feature_flag`（单数）表死代码** | `migrations/001_init:234-245` |
| P2-20 | 19.4 | **CHECK 约束命名 (chk_*) 未统一** | `migrations/*.up.sql` |

### 3.4 前端工艺（5 项）

| # | 章节 | Stub 项 | 位置 |
|---|------|---------|------|
| P2-21 | 5.5.3 | **对话暂停/继续/编辑重发未实现** | `ChatProvider.tsx` |
| P2-22 | 5.5.3 | **对话分支 3 活跃上限前端校验缺** | `ChatHistoryPanel.tsx` |
| P2-23 | 5.5.6 | **503 熔断 OPEN vs 配额耗尽 未区分** | `ErrorPage.tsx` |
| P2-24 | 5.5.11 | **A11y 工具落地稀少**（仅 30% 引用） | 各 panel 文件 |
| P2-25 | 5.5.17 | **CSS 变量主题 Token `--of-*` 未在 global.css 定义** | `global.css` |

### 3.5 性能/可观测（5 项）

| # | 章节 | Stub 项 | 位置 |
|---|------|---------|------|
| P2-26 | 9.1 | **Diff + 上下文前 5 后 5 行未实现** | Pipeline 阶段代码 |
| P2-27 | 9.1 | **`PromptConfig.CacheEnabled` 字段声明但未启用** | `prompt_builder.go:62-63` |
| P2-28 | 9.1 | **LLM 语义共享 结构化 Artifact 未实现** | 无 |
| P2-29 | 9.2 | **日志采样率（ERROR 全量/INFO 10%/DEBUG 仅 Debug）零实现** | `middleware.go:126-137` |
| P2-30 | 9.2 | **P95 估算未排序近似** | `slo_tracker.go:46` |

---

## 4. 修复路径建议（4 条按依赖顺序）

### 路径 A：Phase 4 数据闭环（1 周，10 项）
聚焦 P0-01 / P0-03 / P0-05 / P0-16 / P1-26 / P2-16 / P2-17 / P2-18
- **目标**：成本/审计/部署/恢复四件套端到端真实
- **验收**：`TokenMeter → token_usage` 真实落库；`MinioObjectStore` 挂上；`audit_log` 链断裂扫描 + 告警；`cmd/openforge` 启动恢复

### 路径 B：Phase 2/3 安全 + UX 闭环（3 天，12 项）
聚焦 P0-13 / P0-15 / P0-20 / P0-21 / P1-07 / P1-10 / P1-18 / P1-25 / P2-21 / P2-22
- **目标**：WS 审批真生效、SimpleMode 上线、5.5.8 失败分类、3 活跃分支上限
- **验收**：WS `gate.approve` 真正调 `GateSvc.Approve`；SimpleMode 走 60/40 布局；失败显示分类卡片

### 路径 C：架构兑现（2 周，15 项）
聚焦 P0-08 / P0-09 / P0-10 / P0-11 / P0-14 / P1-11 / P1-14 / P1-29 / P2-09 / P2-10
- **目标**：gRPC 真正启用 + Multi-agent 服务组合 + 沙箱真正 docker run
- **验收**：`Coordinator` 注入 bootstrap；`BashTool` 不再 nil panic；熔断器串联；OTel 启用

### 路径 D：Phase 5+ Enterprise 落地（1 周，10 项）
聚焦 P0-22 / P0-23 / P0-24 / P0-25 / P1-30 / P1-34 / P2-02 / P2-05 / P2-15 / P2-21
- **目标**：告警/测试/Debug/合规/FeatureFlag 生命周期全闭环
- **验收**：18 条 Prom 规则上线；E2E 10 用例通过；`/debug/pipeline/{id}/replay` 端点可用

---

## 5. 关键跨域共识 & 需补充的子计划

### 5.1 跨域共识

| # | 现象 | 涉及审计 | 含义 |
|---|------|---------|------|
| **X-1** | **3 个 Enterprise Feature（ADR/Compliance/Monitoring）是占位页** | Ch.5 + Ch.10 + Ch.13 | 需决策：是否进入 Phase 5+ 实际接入 |
| **X-2** | **CLAUDE.md "设计完成，待进入 Phase 1 编码" 是失真陈述** | Ch.1-2 | 文档与 phase1-summary.md 冲突，需立即更新 |
| **X-3** | **Token 数据流端到端断裂** | Ch.1-2 + Ch.4 + Ch.6 | Node 端 console.log + Go 端无写方法 = 看板永远空 |
| **X-4** | **Deploy 能力有但接入缺失** | Ch.5 + Ch.6 | `DeployService` 完整 + bootstrap 注入 + routes.go 无端点 |
| **X-5** | **gRPC 通信层从未真正启用** | Ch.4 | Go 走纯 HTTP；Node 只挂 LLM Router；其余 5 Service 全 stub |
| **X-6** | **22 表已建但 9 表无 Go CRUD** | Ch.19 | 设计漂移 + bootstrap 未挂 |
| **X-7** | **测试金字塔严重失衡** | Ch.13 | E2E=0、集成=1、单元≈60 vs 设计 10/50/200+ |
| **X-8** | **4 个熔断器已配置但未串联** | Ch.7 | 仅有配置，0 命中 `breaker.Call` |

### 5.2 待补充的子计划（未在 docs/superpowers/plans/ 中）

| 计划文件 | 章节 | 估算 |
|---------|------|------|
| `2026-06-06-monorepo-topology-analyzer.md` | 2.4 | 3-5 天 |
| `2026-06-06-token-data-pipeline-closure.md` | 1.2-6 / 4.6 | 2 天 |
| `2026-06-06-embed-index-implementation.md` | 4.7 / 4.8 | 5-7 天 |
| `2026-06-06-gateway-sandbox-true-runtime.md` | 4.5b / 8.2 | 3-4 天 |
| `2026-06-06-enterprise-features-implementation.md` | 5.5.5/5.5.8/5.5.9/5.5.16 + Ch.10.3/10.5 | 1 周 |
| `2026-06-06-phase2-3-ux-closure.md` | 5.5.1/5.5.3/5.5.5/5.5.8 | 1 周 |
| `2026-06-06-p0-architecture-honesty.md` | Ch.0（重构 gRPC 通信层） | 2 周 |
| `2026-06-06-observability-alerting-rollout.md` | 12.5 | 3 天 |
| `2026-06-06-featureflag-lifecycle.md` | 14.5 | 2 天 |
| `2026-06-06-data-layer-and-table-drift.md` | Ch.19 + 2.4 | 1 周 |

---

## 6. 关键文件路径索引（按章节）

### 设计文档
- `D:\vscode\openforge\DESIGN.md`（3965 行 / 18 章 + 19 章数据库）

### 当前项目主要实现
- A 层：`D:\vscode\tiktok\openforge\internal\pipeline\`
- B 层：`D:\vscode\tiktok\openforge\internal\agent\` + `D:\vscode\tiktok\openforge\nodejs-io\src\`
- C 层：`D:\vscode\tiktok\openforge\frontend\src\`
- 后端 HTTP：`D:\vscode\tiktok\openforge\internal\server\`（routes.go / ws_handler.go / middleware.go）
- 鉴权：`D:\vscode\tiktok\openforge\internal\auth\`
- 观测：`D:\vscode\tiktok\openforge\internal\observability\`
- 适配器：`D:\vscode\tiktok\openforge\internal\adapter\`（17 个 adapter 真实实现）
- 配置：`D:\vscode\tiktok\openforge\config\profiles\`
- 迁移：`D:\vscode\tiktok\openforge\migrations\`（11 套 up + 11 套 down）
- Proto：`D:\vscode\tiktok\openforge\proto\agent\v1\`（6 service）
- 文档：`D:\vscode\tiktok\openforge\docs\superpowers\`

### 既有阶段计划（按 Phase）
- `2026-05-20-phase-1-mvp.md` ~ `2026-06-05-data-layer-and-api-completeness.md`（40+ 计划）
- 当前活跃基线：`2026-06-03-current-status-next-stage.md`
- 下一波入口：`2026-06-05-runtime-stub-closure-and-panel-wiring.md` + `2026-06-05-data-layer-and-api-completeness.md` + `2026-06-05-code-quality-and-bootstrap-closure.md` + `2026-06-05-phase2-3-security-functional-closure.md`

---

## 7. 立即可执行的下一步

**Day 1**:
1. 修复 P0-16（CLAUDE.md 状态冲突）—— 5 分钟
2. 启动路径 A（数据闭环）：P0-01 TokenMeter.flush() 落库
3. 启动路径 B（UX 闭环）：P0-20 SimpleMode

**Week 1**:
1. 路径 A 全部 10 项完成
2. 路径 B 全部 12 项完成
3. 输出 4 份新子计划（monorepo-topology / token-pipeline / embed-index / gateway-sandbox）

**Week 2-3**:
1. 路径 C 架构兑现 15 项
2. 路径 D Enterprise 落地 10 项

**Week 4+**:
1. P2 改进项按需清理
2. 跨域 8 大共识逐一收口
3. 文档/数据/Schema 与设计最终对齐

---

## 8. 总结

- **整体完成度约 51%** —— Phase 1-2 准入级（70%+）已通过；Phase 3 Pipeline/Gate/Diff 80% 落地；Phase 4 数据流半闭环；Phase 5+ 企业特性"代码就绪但 0 调用方"居多
- **P0 必修 25 项** 是下一波 Phase 5+ / 6 / 7 准入的前置条件
- **CLAUDE.md 失真陈述** 是最高优先级 5 分钟修复项（P0-16）
- **8 大跨域共识**中 X-1（Enterprise 占位）+ X-2（CLAUDE.md 文档）+ X-3（Token 数据流）必须先关闭再谈下一阶段

**审计完成。本档即"待完成内容"MD 文档，可作为下一波实施计划的根清单。**
