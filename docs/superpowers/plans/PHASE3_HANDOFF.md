# Phase 3 Handoff — 当前完成状态

> **日期**: 2026-05-23 | **Phase 3 计划**: `docs/superpowers/plans/2026-05-22-phase-3-pipeline-gate.md` | **v5 设计对齐**

## v5 关键设计变更 (Phase 1.5 已交付)

Phase 3 执行前必须知悉的 v5 设计:

| 设计 | 位置 | Phase 3 影响 |
|------|------|-------------|
| **Query Engine Gate-pause** | §4.2 | Gate 审批不再直接改 pipeline 状态，而是通过 Submit→pending_gate→Resume(GateResult) |
| **PermissionMode** | §4.4 | Gate 判定用 PermissionMode (替代硬编码 L3/L4) |
| **GateRepository** | §4.2.5 | 新增 `gate_request` 表 (审批挂起持久化)，与 `gate_event` 审计表分离 |
| **ErrorRecovery** | §3.15 + §4.2.6 | ClassifyAndRecover 完整 4 层，MapToolErrorToFailureCode 正则匹配 |
| **LLM Router (Go)** | §4.5 | 模型注册表 8 模型，表驱动，Phase 1.5 已实现 |

## ⚠️ 前端设计提醒

**Phase 3 涉及 ProMode Dockview 多面板 UI、Diff 面板、Gate 审批面板、审批收件箱等前端页面。开始前端任务（Task 6-7）前，必须先调用 Skill：**

```
Skill("ui-ux-pro-max")
```

按照 CLAUDE.md 规定的流程：分析需求 → 生成设计系统 → 输出色板/字体/间距/组件规范。OpenForge 专属 Stack 映射为 `react` + `shadcn`。禁止 Inter/Roboto 作为展示字体，禁止紫色→粉色→蓝色渐变。

---

## 已完成 Phase 概况

| Phase | 核心交付 | 状态 |
|-------|---------|------|
| Phase 1 | CLI 对话 + 10 能力域接口 + Profile 系统 | ✅ |
| Phase 1.5 | CSP WAL, Tool 泛型接口, 权限模式, 错误恢复链, Query Engine, CommandExecutor, LLM Router Go 迁移 | ✅ |
| Phase 2 | Web 聊天框 + JWT Auth + CSP/XSS/CORS 清零 | ✅ |

## Phase 2 最终架构

```
Browser (React SPA :5173)
    │
    ├─ REST /api/* → Go Server (:8030)
    │   ├── POST /api/auth/login       (dev: 任意用户名)
    │   ├── POST /api/auth/refresh
    │   ├── GET  /api/health
    │   ├── GET  /api/projects         (stub)
    │   ├── GET  /api/projects/{id}    (stub)
    │   ├── POST /api/projects/{id}/pipelines  (stub)
    │   ├── GET  /api/pipelines/{id}   (stub)
    │   └── GET  /api/pipelines/{id}/messages  (stub)
    │
    └─ WS /ws/chat → Go Server (:8030)
        ├── 首帧鉴权: {"type":"auth","payload":{"token":"JWT"}}
        ├── 上行: chat.send, ping
        └── 下行: chat.stream, chat.stream_done, error, pong
            (Phase 3 需加: stage_change, gate.notify, msg.card 等)
```

## 关键配置

### 环境变量
```powershell
$env:ANTHROPIC_AUTH_TOKEN = "sk-e85f132f45aa406f8a9949bf5e4990d5"
```

### config/profiles/minimal.yaml
```yaml
profile: minimal
security_tier: dev
llm:
  default_provider: deepseek
  default_model: deepseek            # → Registry 查找 Alias="deepseek"
jwt:
  secret: "dev-secret-change-in-production-32b!"
  access_ttl: "1h"
  refresh_ttl: "24h"
command_executor: local-shell
grpc:
  nodejs_io_addr: 127.0.0.1:50051
  coordinator_addr: 127.0.0.1:50052
```

### LLM Registry（`internal/llm/registry.go`）
| Alias | Provider | ModelID | BaseURL | KeyRef |
|-------|----------|---------|---------|--------|
| sonnet | anthropic | claude-sonnet-4-6-20250514 | api.anthropic.com | ANTHROPIC_AUTH_TOKEN |
| haiku | anthropic | claude-haiku-4-5-20251001 | api.anthropic.com | ANTHROPIC_AUTH_TOKEN |
| deepseek | deepseek | deepseek-v4-pro[1m] | api.deepseek.com/anthropic | ANTHROPIC_AUTH_TOKEN |
| ollama | ollama | qwen3 | localhost:11434 | (空) |

> **注意**: DeepSeek 使用 Anthropic 兼容 API endpoint (`/anthropic` 后缀)。
> 所有 KeyRef 改为 `ANTHROPIC_AUTH_TOKEN`（匹配用户环境变量）。

### 端口
| 服务 | 端口 | 备注 |
|------|------|------|
| Go Server | 8030 | 8080 被占用 |
| Vite Dev | 5173 | `/api` 和 `/ws` proxy 到 8030 |
| Postgres | 5432 | Phase 3 需要 |

## 前端路由

| 路径 | 组件 | 说明 |
|------|------|------|
| `/login` | LoginPage | 任意用户名登录 |
| `/` | DashboardPage | 项目列表（stub 空） |
| `/project/:id` | ProjectPage | 创建 Pipeline 表单 |
| `/project/:id/chat?pipeline=:pid` | ChatPanel | WebSocket 流式对话 |
| `/project/:id/pipeline/:pid` | ProModePage | Phase 3 新增，Dockview 多面板 |

## Phase 2 文件清单

### Go 后端（新增 7 文件）
```
cmd/server/main.go                     # HTTP 服务入口
internal/auth/service/jwt.go           # JWT HS256 issue/verify
internal/auth/service/jwt_test.go      # 4 tests
internal/server/routes.go              # REST 路由注册
internal/server/middleware.go          # Auth/CSP/CORS/Logging/RateLimit
internal/server/ws_handler.go          # WS 首帧鉴权 + chat.send 流式
internal/server/ws_handler_test.go     # 3 tests
```

### Go 后端（修改 3 文件）
```
internal/shared/profile/loader.go      # +JWTConfig, +JWT 字段
internal/shared/profile/bootstrap.go   # [Phase 1.5 已修改] 无 Phase 2 额外改动
internal/llm/registry.go              # KeyRef → ANTHROPIC_AUTH_TOKEN, DeepSeek URL/Model 修正
config/profiles/minimal.yaml          # +jwt, +command_executor, default_model→deepseek
```

### 前端（18 文件）
```
frontend/package.json                  # react, react-router-dom, dompurify, vite
frontend/tsconfig.json
frontend/vite.config.ts                # proxy /api→8030, /ws→ws://8030
frontend/index.html                    # CSP meta
frontend/src/main.tsx                  # React entry
frontend/src/App.tsx                   # 4 routes + ProtectedRoute
frontend/src/shared/api.ts             # REST + wsURL()
frontend/src/shared/auth.tsx           # AuthProvider + useAuth
frontend/src/shared/sanitize.ts        # DOMPurify
frontend/src/features/login/LoginPage.tsx
frontend/src/features/dashboard/DashboardPage.tsx
frontend/src/features/dashboard/ProjectCard.tsx
frontend/src/features/project/ProjectPage.tsx
frontend/src/features/chat/ChatPanel.tsx
frontend/src/features/chat/ChatProvider.tsx
frontend/src/features/chat/MessageList.tsx
frontend/src/features/chat/MessageInput.tsx
frontend/src/features/chat/useWebSocket.ts
```

## 启动命令

```powershell
# Terminal 1: Go
$env:ANTHROPIC_AUTH_TOKEN = "sk-e85f132f45aa406f8a9949bf5e4990d5"
go run ./cmd/server/ --addr :8030

# Terminal 2: 前端
cd frontend && npm run dev
# 打开 http://localhost:5173
```

## 已知 Quirk / 注意点

1. **WebSocket 鉴权**: 浏览器 WebSocket 不支持自定义 HTTP Header，所以 `/ws/chat` 路由 **不使用** AuthMiddleware。鉴权完全通过首帧 `{"type":"auth","payload":{"token":"JWT"}}` 完成，5 秒超时。

2. **Dashboard 空**: `GET /api/projects` 返回 `[]`，因为 Phase 2 没有真正的 project 表数据。Dashboard 显示 "No projects yet."

3. **Vite proxy**: `vite.config.ts` 的 proxy 指向 `localhost:8030`。如果换端口需要同步修改 + 重启 Vite。

4. **Dockview 导入**: Phase 3 使用 `dockview` npm 包时，需要 `import { DockviewReact } from 'dockview'`。如果 TypeScript 报类型错误，可能需要 `@types/dockview` 或 `declare module 'dockview'`。

5. **PG 连接**: Phase 3 需要 Postgres。`minimal.yaml` 的 database 配置尚未被 Bootstrap 读取（Phase 2 没有数据库依赖）。Phase 3 Task 4 会添加 `sql.Open`。

6. **前端无 CSS 框架**: Phase 2 全部内联样式。Phase 3 可继续此模式或引入 CSS。

## Phase 3 计划速览

**文件**: `docs/superpowers/plans/2026-05-22-phase-3-pipeline-gate.md`

**任务 (v5 对齐后)**:
1. Pipeline 状态机（9 种转换 + NeedsGate() = PermissionMode 判定 + backtrack 限制）
2. Repository 接口 + PG Adapter（PipelineRepository + GateRepository，乐观锁）
3. PipelineService + GateService（含 gate_request 持久化 + QueryEngine Resume 桥接）
4. Server REST 端点（替换 stub，加 gate/review-inbox，Gate-pause 桥接）
5. WebSocket — QueryEngine.Submit() 对接 + 下行事件
6. ProMode Dockview 布局（Chat + Diff + FileTree）
7. Gate 审批 UI + ReviewInboxPage
8. E2E 验证

**Phase 3 新增依赖**: `dockview`, `@monaco-editor/react`, `lib/pq`

**Phase 3 新增表**: `gate_request` (§4.2.5 DDL)

**Phase 3.1 延后**: 拓扑图 (Cytoscape), 对话分支, 完整 WS 协议 (edit/stop/pause), Backtrack DAG, 模型切换 UI
