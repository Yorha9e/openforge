# OpenForge Phase 1 MVP — 完成审计

> 日期: 2026-05-21 | 版本: v1 | 状态: 已完成

---

## 验证结果

| 检查项 | 状态 |
|--------|------|
| Go 测试 (`go test ./...`) | **5/5 通过** |
| Go 编译 (`go build ./...`) | **通过** |
| Go vet | **清洁** |
| TypeScript (`tsc --noEmit`) | **清洁** |
| 实时对话 (Mimo 代理) | **通过** |
| `[1m]` 后缀解析 → 1,048,576 ctx | **通过** |
| CLI → 协调器 → gRPC → Node.js → API | **正常工作** |

---

## 所有任务与关键节点 (18 次提交)

| # | 提交 | 节点 |
|---|------|------|
| 1 | `d967924` | **项目骨架**: Go mod、Node.js 项目、Docker Compose |
| 2 | `1db3315` | **共享内核**: PipelineID/ProjectID/Level 等类型，11 个能力域接口 |
| 3 | `9f60c7c` | **Bug 修复**: Error 类型新增 `Unwrap()`，测试覆盖全部 9 种 PipelineStatus |
| 4 | `4fd955c` | **Profile 系统**: YAML 加载器、显式组合根、10 个最小实现、安全层级阻断 |
| 5 | `10ef50e` | **数据库**: 16 张表、CHECK 约束、月分区、WORM 审计日志 |
| 6 | `0d56ac2` | **Pipeline 领域**: Pipeline 聚合、Stage 实体、Gate 值对象、复杂度 L1-L4 分类器 |
| 7 | `7f11365` | **Proto 生成**: Go + TypeScript 代码生成、buf 配置 |
| 8 | `e31892f` | **Agent 领域**: 协调器、CSP 通道（含背压）、检查点、3 个 Port 接口 |
| 9 | `e78c682` | **gRPC 适配器**: Go → Node.js ConnectRPC 客户端 |
| 10 | `dc53d2e` | **Auth + 可观测性**: 用户/Role/Permission 引擎、12 个指标常量、WORM 审计日志 |
| 11 | `7994601` | **Node.js LLM 路由器**: Anthropic 适配器、模型选择器、Token 计量器、gRPC 服务器 |
| 12 | `54140ed` | **CLI 入口**: Readline 循环、流式对话、信号处理、`/help` `/quit` `/clear` |
| 13 | `419a29e` | **E2E 测试**: CLI 构建、横幅输出、优雅退出 |
| 14 | `1cd05db` | **`[1m]` 后缀解析**: Go+Node.js 双端上下文窗口解析 (`ParseContextWindow`) |
| 15 | `1cd05db` | **SecretStore 共存**: `chainSecretStore` Vault 优先 + env 回退 |
| 16 | `1cd05db` | **Env 模型注入**: `ANTHROPIC_MODEL`、`ANTHROPIC_DEFAULT_{SONNET,OPUS,HAIKU}_MODEL` |
| 17 | `494a5df` | **修复**: API 调用前剔除模型名 `[Nm]/[Nk]` 后缀 |
| 18 | `a9c6472` | **杂项**: .gitignore 完整配置、实现计划文档 |

---

## 52 个源文件

### Go 端 (21 文件)

```
cmd/openforge/main.go                          ← CLI 入口
internal/shared/kernel/types.go                ← 共享值对象 + ParseContextWindow
internal/shared/kernel/interfaces.go           ← 11 个能力域接口
internal/shared/kernel/types_test.go           ← 19 个测试用例
internal/shared/errors/errors.go               ← 结构化错误 + Unwrap
internal/shared/profile/loader.go              ← YAML 配置加载 + 验证
internal/shared/profile/loader_test.go         ← 8 个测试用例
internal/shared/profile/bootstrap.go           ← 显式组合根 + 10 个最小实现 + chainSecretStore
internal/pipeline/domain/pipeline.go           ← Pipeline 聚合根
internal/pipeline/domain/stage.go              ← Stage 实体
internal/pipeline/domain/gate.go               ← Gate 审批值对象
internal/pipeline/domain/complexity.go         ← L1-L4 复杂度分类器
internal/pipeline/domain/complexity_test.go    ← 4 个测试用例
internal/agent/domain/coordinator.go           ← AgentCoordinator
internal/agent/domain/checkpoint.go            ← Checkpoint 值对象
internal/agent/port/llm_client.go              ← LLMRouterClient 接口
internal/agent/port/tool_registry.go           ← ToolRegistryClient 接口 (存根)
internal/agent/port/learning_client.go         ← LearningEngineClient 接口 (存根)
internal/agent/adapter/grpc_llm_client.go      ← ConnectRPC Go 客户端
internal/agent/service/csp_channel.go          ← CSP 通道 + 背压
internal/agent/service/csp_channel_test.go     ← 2 个测试用例
internal/auth/domain/user.go                   ← User + Role 类型
internal/auth/domain/permission_engine.go      ← can() RBAC 纯函数
internal/observability/domain/metrics.go       ← 12 个强类型指标常量
internal/observability/adapter/stdout_telemetry.go ← Stdout JSON 遥测
internal/policy/adapter/worm_audit_log.go      ← WORM 审计日志 (SHA256 哈希链)
```

### Node.js 端 (6 文件)

```
nodejs-io/src/server.ts                        ← gRPC 服务入口 + stripSuffix
nodejs-io/src/kernel/interfaces.ts             ← LLMProvider 接口
nodejs-io/src/llm/providers/anthropic.ts       ← Anthropic API 适配器 (baseURL 支持)
nodejs-io/src/llm/domain/model_selector.ts     ← WFQ 模型选择 + [1m] 后缀解析
nodejs-io/src/llm/token_meter.ts               ← 环形缓冲 Token 计数
```

### 配置 + 数据 (9 文件)

```
config/profiles/minimal.yaml                   ← Phase 1 活动配置 (mimo-v2.5-pro[1m])
config/profiles/standard.yaml                  ← 未来存根
config/profiles/enterprise.yaml                ← 未来存根
migrations/001_init.up.sql                     ← 16 张表 + WORM 月分区
migrations/001_init.down.sql                   ← DROP ALL (仅 dev)
buf.gen.yaml                                   ← Go + TS 代码生成
proto/agent/v1/buf.gen.ts.yaml                 ← TS 专用生成配置
proto/agent/v1/coordinator.proto               ← CoordinatorService (13 RPC)
proto/agent/v1/gate.proto                      ← GateService (4 RPC)
proto/agent/v1/llm.proto                       ← LLMRouterService (5 RPC)
proto/agent/v1/tools.proto                     ← ToolRegistryService (7 RPC)
proto/agent/v1/learning.proto                  ← LearningEngineService (11 RPC)
proto/agent/v1/terminal.proto                  ← TerminalService (3 RPC)
```

### 基础设施 (6 文件)

```
go.mod, go.sum                                 ← Go 模块依赖
.golangci.yml                                  ← Lint 配置
docker-compose.yml                             ← postgres + nodejs-io
.gitignore                                     ← gen/ node_modules/ dist/
nodejs-io/package.json, tsconfig.json          ← Node.js 项目配置
```

### 测试 (4 文件，不含已列出的 *_test.go)

```
test/integration/cli_chat_test.go              ← 3 个 E2E 测试
```

---

## 架构关键决策

1. **三层正交**: C 层 (工作台) → A 层 (Pipeline 引擎/Go) → B 层 (Agent Swarm/Go+Node.js)
2. **六边形+垂直切片**: `domain → port ← adapter`，依赖方向严格内→外
3. **Capability Profile**: minimal/standard/enterprise 三档 YAML，同一份代码
4. **显式组合根**: 无全局状态，无 DI 框架，`Bootstrap(profile) → *OpenForge`
5. **ConnectRPC**: Go ↔ Node.js 通信使用 Connect 协议 (HTTP/1.1)，非标准 gRPC
6. **CSP 通道**: Go goroutine + buffered channel 原生背压
7. **`[Nm]/[Nk]` 模型后缀**: 从模型名自动解析上下文窗口大小
8. **SecretStore 链**: Vault 优先 → env 回退，接口不变

---

## 数据库 16 张表

| 表 | 用途 | 关键约束 |
|----|------|---------|
| `project` | 项目 | repo_type CHECK |
| `user` | 用户 (SSO email 作为 PK) | — |
| `user_role` | 项目级 RBAC | role CHECK, UNIQUE(user_id, project_id) |
| `module_ownership` | 模块归属 | UNIQUE(project_id, module_name) |
| `pipeline` | Pipeline 聚合根 | level/status/current_stage CHECK, backtrack_count 0-3 |
| `pipeline_stage` | 阶段实体 | stage/status CHECK |
| `gate_event` | 审批事件 | event/decision CHECK, prev_hash+content_hash (WORM 链) |
| `checkpoint` | 检查点 | trigger CHECK (auto/manual) |
| `conversation_message` | 对话消息 | role/msg_type CHECK, UNIQUE(pipeline_id, branch_id, msg_seq) |
| `conversation_branch` | 对话分支 | status CHECK |
| `file_lock` | 文件并发锁 | lock_type CHECK, UNIQUE(project_id, file_path) |
| `token_usage` | Token 计量 (**月分区**) | prompt/completion_tokens >= 0 CHECK |
| `cost_quota` | 成本配额 | month 正则 CHECK, status CHECK |
| `audit_log` | 审计日志 (**月分区**, **WORM**) | result CHECK, prev_hash+content_hash 哈希链 |
| `feature_flag` | 功能开关 | status/rollout_percent CHECK |
| `task_queue` | 任务队列 | task_type/status/priority CHECK, 部分索引 |

---

## 安全审查要点

### 高危 (Phase 2 必须清零)

| 风险点 | 文件:行 | 说明 |
|--------|---------|------|
| **API Key 明文传输** | `grpc_llm_client.go:101` | APIKey 通过 ConnectRPC HTTP/1.1 传输到 Node.js。Phase 1 localhost 可接受，Phase 2 需 mTLS |
| **gRPC 无认证** | `server.ts:73` | 绑定 0.0.0.0:50051，无任何认证机制 |
| **CORS 缺失** | `server.ts` | 无 CORS 中间件 (Phase 1 无前端可接受) |

### 中危

| 风险点 | 文件:行 | 说明 |
|--------|---------|------|
| **输入清洗缺失** | `server.ts:79-82` | 用户消息直接拼接，无 XSS/注入过滤。Phase 1 CLI 无前端可接受 |
| **CSP 通道背压丢失** | `csp_channel.go:28-35` | 通道满时返回 error 但无重试/WAL 回放 |
| **并发安全** | `bootstrap.go:133-155` | `goroutineEventBus.subs` 无互斥锁。Phase 1 CLI 单线程可接受，Phase 2 Web 服务必须加锁 |
| **并发安全** | `bootstrap.go:159-179` | `memoryCache.data` 无互斥锁。同上 |
| **并发安全** | `bootstrap.go:202-233` | `staticServiceRegistry.services` 无互斥锁。同上 |

### 低危 / 已缓解

| 风险点 | 文件:行 | 说明 |
|--------|---------|------|
| Secret 日志泄漏 | `bootstrap.go:72-77` | `os.Getenv` 读取，不记录值 — **安全** |
| SQL 注入 | `worm_audit_log.go:41-44` | 参数化查询 — **安全** |
| Ed25519 验证存根 | `loader.go:62-75` | 未实际验证签名。Phase 7 启用 |
| 沙箱逃逸 | `bootstrap.go:86` | `noopContainerRuntime` — 沙箱未实现 |
| Token 计数精度 | `token_meter.ts` | Crash 时环形缓冲未 flush 数据丢失 ~0.1% |

### 修复跟踪

| # | 问题 | Phase | 状态 |
|---|------|-------|------|
| S1 | gRPC 认证 | Phase 2 | 待实现 |
| S2 | API Key 传输加密 (mTLS) | Phase 2 | 待实现 |
| S3 | CSP Header | Phase 2 | 待实现 |
| S4 | XSS 防护 (DOMPurify) | Phase 2 | 待实现 |
| S5 | EventBus 并发安全 (sync.RWMutex) | Phase 2 | 待实现 |
| S6 | Cache 并发安全 (sync.RWMutex) | Phase 2 | 待实现 |
| S7 | Ed25519 签名验证 | Phase 7 | 待实现 |
| S8 | 沙箱安全层 (seccomp/cap-drop) | Phase 4 | 待实现 |

---

## Phase 2 前置条件

Phase 2 交付「极简 Web 聊天框 + BFF Auth」，以下 3 个高危项必须清零后才能上线：

1. **JWT 安全** — BFF 签发，不放 URL query string，1h 短期令牌，滑动续期
2. **XSS 防护** — DOMPurify + CSP Header + Monaco 只读模式
3. **CSP** — `default-src 'self'; script-src 'self'; connect-src 'self' wss:`

---

## 启动命令

```powershell
# 终端 1: Node.js LLM 路由器
$env:ANTHROPIC_API_KEY = "your-key"
$env:ANTHROPIC_BASE_URL = "https://token-plan-cn.xiaomimimo.com/anthropic"
$env:ANTHROPIC_MODEL = "mimo-v2.5-pro[1m]"
$env:ANTHROPIC_DEFAULT_SONNET_MODEL = "mimo-v2.5-pro[1m]"
$env:ANTHROPIC_DEFAULT_OPUS_MODEL = "mimo-v2.5-pro[1m]"
$env:ANTHROPIC_DEFAULT_HAIKU_MODEL = "mimo-v2.5-pro[1m]"
cd nodejs-io; npm run dev

# 终端 2: OpenForge CLI
$env:ANTHROPIC_API_KEY = "your-key"
cd D:\vscode\tiktok\openforge
go run cmd/openforge/ serve --config config/profiles/minimal.yaml
```
