# OpenForge 编码风格约定

> 适用: Go 协调层 · Node.js IO 层 · BFF · React 前端 · 通用
> 架构: 六边形 + 垂直切片混合 (Ports & Adapters + Vertical Slice)

---

## 一、项目目录结构

### 1.1 Go 协调层 (internal/)

```
internal/
├── shared/                              # ====== 共享内核 ======
│   ├── kernel/
│   │   ├── interfaces.go                # 10 个 port 接口: TaskQueue|EventBus|Notifier|Cache|Telemetry|...
│   │   └── types.go                     # 共享值对象: PipelineID, ProjectID, StageType, Level
│   ├── profile/
│   │   ├── loader.go                    # Profile YAML 加载 + Ed25519 签名校验
│   │   └── bootstrap.go                 # Bootstrap(profile) → 装配全部 adapter → 注入 domain
│   ├── pagination/                      # page_token encode/decode
│   └── errors/                          # 统一错误类型 + error code 注册表

├── pipeline/                            # ====== 垂直切片: Pipeline 引擎 (A 层) ======
│   ├── domain/
│   │   ├── pipeline.go                  # Pipeline 聚合根 (9 态 + L1-L4)
│   │   ├── stage.go                     # Stage 实体 (6 阶段 + Gate 判定)
│   │   ├── gate.go                      # Gate 审批值对象 (异步/草稿/超时)
│   │   ├── backtrack.go                 # DAG 回溯值对象
│   │   └── complexity.go               # 复杂度分级 (L1-L4 判定规则)
│   ├── port/
│   │   └── repository.go                # PipelineRepository|StageRepository|GateRepository
│   ├── adapter/
│   │   ├── pg_pipeline_repo.go
│   │   ├── pg_stage_repo.go
│   │   └── pg_gate_repo.go
│   └── service/
│       ├── pipeline_service.go          # 应用服务: 编排 domain + port
│       └── gate_service.go              # gRPC GateService 实现

├── agent/                               # ====== 垂直切片: Agent 协调 (B 层 Go 侧) ======
│   ├── domain/
│   │   ├── coordinator.go               # AgentCoordinator (goroutine=agent)
│   │   ├── checkpoint.go                # 检查点值对象 (3 内存 + 1 PG)
│   │   └── filelock.go                  # 文件锁值对象
│   ├── port/
│   │   ├── llm_client.go                # LLMRouterClient 接口
│   │   ├── tool_registry.go             # ToolRegistryClient 接口
│   │   └── learning_client.go           # LearningEngineClient 接口
│   ├── adapter/
│   │   ├── grpc_llm_client.go           # gRPC → Node.js LLM Router
│   │   ├── grpc_tool_client.go          # gRPC → Node.js Tool Registry
│   │   ├── grpc_learning_client.go      # gRPC → Node.js Learning Engine
│   │   ├── docker_runtime.go            # ContainerRuntime 实现
│   │   └── docker_runtime_test.go
│   ├── mock/                            # //go:generate mockgen
│   │   ├── mock_llm_client.go
│   │   └── mock_container_runtime.go
│   └── service/
│       ├── coordinator_service.go       # gRPC CoordinatorService 实现
│       └── csp_channel.go               # CSP Channel + WAL

├── auth/                                # ====== 垂直切片: 认证鉴权 ======
│   ├── domain/
│   │   ├── user.go
│   │   ├── role.go
│   │   └── permission_engine.go         # can(user, action, project) 纯函数
│   ├── port/
│   │   ├── user_repo.go
│   │   └── sso_provider.go              # OIDC/LDAP/SAML 接口
│   └── adapter/
│       ├── pg_user_repo.go
│       ├── oidc_provider.go
│       └── ldap_provider.go

├── observability/                       # ====== 垂直切片: 可观测性 ======
│   ├── domain/
│   │   └── metrics.go                   # 强类型 metric 定义
│   ├── port/
│   │   └── telemetry.go                 # Telemetry 接口
│   └── adapter/
│       ├── stdout_telemetry.go
│       ├── prometheus_telemetry.go
│       └── otel_telemetry.go

└── policy/                              # ====== 垂直切片: 策略/合规 ======
    ├── domain/
    │   ├── quota.go
    │   └── audit.go
    └── adapter/
        └── worm_audit_log.go
```

### 1.2 Node.js IO 层 (nodejs-io/)

```
nodejs-io/src/
├── kernel/
│   └── interfaces.ts                    # Node 侧 port 接口
├── llm/                                 # LLM Router 垂直切片
│   ├── domain/
│   │   └── model_selector.ts            # WFQ 优先级调度
│   ├── providers/
│   │   ├── anthropic.ts                 # ≤600 行
│   │   ├── ollama.ts
│   │   └── deepseek.ts
│   └── token_meter.ts                   # ring buffer → batch COPY
├── tools/                               # Tool Registry
│   ├── domain/
│   │   ├── registry.ts
│   │   └── embedding_index.ts
│   └── adapters/
│       └── mcp_adapter.ts
├── learning/
│   ├── domain/
│   │   ├── knowledge_engine.ts
│   │   └── ab_experiment.ts
│   └── snapshot_manager.ts
├── gen/                                 # Proto 生成 (tsconfig exclude)
└── tests/
```

### 1.3 BFF 层 (bff/)

```
bff/src/
├── routes/                              # API 路由 (按 domain 拆分)
│   ├── projects.ts
│   ├── pipelines.ts
│   ├── gates.ts
│   └── admin.ts
├── middleware/
│   ├── auth.ts                          # JWT + SSO/OIDC/LDAP/SAML
│   ├── rbac.ts                          # permission-engine
│   └── rate_limiter.ts
├── grpc/                                # gRPC client → Go 协调层
│   ├── coordinator_client.ts
│   ├── llm_client.ts
│   └── learning_client.ts
└── ws/
    └── connection_manager.ts
```

### 1.4 前端 (frontend/)

```
frontend/src/
├── shared/                              # 共享 UI 组件 + hooks
├── features/                            # 垂直切片 (自包含)
│   ├── pipeline-dashboard/
│   │   ├── components/ + hooks/ + stores/ + types.ts
│   ├── chat-panel/
│   │   ├── components/ + hooks/ + stores/ + types.ts
│   ├── code-review/
│   │   ├── components/ + hooks/ + stores/ + types.ts
│   ├── review-inbox/
│   │   ├── components/ + hooks/ + stores/ + types.ts
│   ├── topology-viewer/
│   ├── admin-panel/
│   └── notifications/
├── gen/                                 # Proto TS 生成 + API types
└── App.tsx
```

### 1.5 依赖方向

```
frontend/features → BFF/routes → internal/xxx/service
       ↓                ↓                ↓
  shared/hooks    middleware/grpc   internal/xxx/domain
                                        ↕ (port 接口)
                                  internal/xxx/adapter
                                        ↓
                                 nodejs-io/src/xxx/domain
                                        ↓
                                 nodejs-io/src/xxx/providers

规则:
  domain/ 不依赖 adapter/ (纯逻辑)
  port/ 只定义接口, 不 import 实现
  adapter/ 实现 port/ 接口, 依赖 domain 类型
  service/ 编排 domain + port, 不直接碰 adapter
  垂直切片间只通过 port 接口通信
  shared/kernel 可被所有切片引用
```

---

## 二、Go 编码规范

### 命名

```go
// 导出: CamelCase
type PipelineStateMachine struct{}
func NewPipelineStateMachine() *PipelineStateMachine {}

// 非导出: camelCase
type coordinator struct{}

// 常量
const MaxBacktrackCount = 3
const defaultChannelBuffer = 256

// 接口: 动词-er / 名词-able, 放在 port/ 目录
type TaskQueue interface { Enqueue(); Dequeue() }
type RotatableSecretStore interface { SecretStore; Rotate() }

// Mock 生成
//go:generate mockgen -destination=mock/mock_task_queue.go -package=mock . TaskQueue
```

### 错误处理

```go
result, err := doSomething()
if err != nil {
    return fmt.Errorf("pipeline %s: %w", pipelineID, err)
}
// 仅在 cmd/ 使用 log.Fatal; internal/ 返回 error
```

### 并发

```go
ctx, cancel := context.WithTimeout(parentCtx, 120*time.Second)
defer cancel()

select {
case ch <- msg:
default: // 背压处理
}
```

### Table-Driven Tests (必需)

```go
func TestComplexityClassifier(t *testing.T) {
    tests := []struct {
        name  string
        files int
        modules int
        want  Level
    }{
        {"typo fix",      1, 1, LevelL1},
        {"small feature", 3, 2, LevelL2},
        {"new module",    6, 4, LevelL3},
        {"refactor",      10, 5, LevelL4},
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got := classifyComplexity(tt.files, tt.modules)
            if got != tt.want {
                t.Errorf("classify(%d, %d) = %v, want %v", tt.files, tt.modules, got, tt.want)
            }
        })
    }
}
```

### Benchmark 命名

```go
func BenchmarkPipelineStage_Decompose(b *testing.B) { ... }
func BenchmarkCSPChannel_Latency(b *testing.B)      { ... }
func BenchmarkEmbeddingIndex_Search(b *testing.B)   { ... }
```

### Go 特定规则

- `gofmt` + `go vet` + `staticcheck` 强制
- 单文件 ≤ 500 行, 单函数 ≤ 80 行
- Context 永远第一个参数
- Mock 放在 `mock/` 子目录
- pprof 标签: goroutine 池 > 1000 时加

---

## 三、TypeScript (Node.js IO + BFF) 编码规范

### 配置

```json
{
  "compilerOptions": {
    "target": "ES2022", "module": "NodeNext",
    "strict": true, "noUncheckedIndexedAccess": true,
    "noImplicitReturns": true
  },
  "exclude": ["src/gen"]
}
```

### 命名

```typescript
// PascalCase: 类型/接口/类
interface LLMConfig { provider: string; model: string }

// camelCase: 变量/函数/方法
const router = new LLMRouter();
```

### Result 模式

```typescript
type Result<T, E = Error> = { ok: true; value: T } | { ok: false; error: E };
```

### Provider 适配器模板

```typescript
// 基类减少重复代码
abstract class BaseLLMProvider implements LLMProvider {
  abstract chat(req: ChatRequest): Promise<Result<ChatResponse>>;
  abstract chatStream(req: ChatRequest): AsyncIterable<ChatChunk>;

  protected buildPrompt(messages: Message[]): string { /* ... */ }
}
// Anthropic/Ollama/DeepSeek 继承 BaseLLMProvider, 只覆盖差异部分
```

### TypeScript 特定规则

- ESLint + `@typescript-eslint/strict` + Prettier
- 单文件 ≤ 400 行 (provider 适配器允许 ≤600)
- `gen/` 排除 strict + lint
- 异步显式标注 `async` / `Promise`

---

## 四、React / TSX (前端) 编码规范

### 组件

```tsx
interface ChatPanelProps {
  pipelineId: string;
  readonly messages: ChatMessage[];
  onSend(message: string): void;
}

export const ChatPanel = memo(function ChatPanel(props: ChatPanelProps) {
  return <div>...</div>;
});
```

### 状态管理

```tsx
// Phase 1-4: Context + useReducer
// Phase 5+: Zustand
const useProjectStore = create<ProjectState>((set) => ({ ... }));
```

### Error Boundary & Suspense

```tsx
// 每个 feature 模块必须有 Error Boundary
<ErrorBoundary fallback={<PipelineErrorCard />}>
  <Suspense fallback={<PipelineSkeleton />}>
    <PipelineDashboard pipelineId={id} />
  </Suspense>
</ErrorBoundary>
```

### React 特定规则

- 单组件 ≤ 300 行 (ChatPanel ≤500)
- 低频更新组件强制 `React.memo` (拓扑图/文件树/用户设置)
- 禁止 `any`, 使用 `unknown` + 类型守卫
- `useEffect` 依赖数组不可省略
- 不可变更新

---

## 五、通用规则

### Git Commit

```
<type>(<scope>): <subject>

[optional body — why + what]

BREAKING CHANGE: <description>       # 破坏性变更
Closes: #42                           # 关联 Issue

例:
feat(pipeline): add DAG backtrack to state machine
fix(sandbox): prevent --privileged flag in sandbox containers

  Container creation now strips all capabilities and enforces
  seccomp profile. Previously a missing flag allowed escalation.

  BREAKING CHANGE: ContainerRuntime.Create() now requires
  SecurityProfile parameter.
```

### 日志格式

```json
{"ts":"2026-05-18T10:30:00Z","level":"info","msg":"stage completed",
 "trace_id":"abc","span_id":"def","pipeline_id":"pipe-42",
 "stage":"impl","duration_ms":1234}
```

### 安全规范

- secret/API key 不落地到日志/文件/env
- 日志脱敏: `sk-[a-zA-Z0-9]{32,}` → `[REDACTED]`
- 输入校验: BFF middleware 层统一做; domain 层不做输入校验 (信任上游)
- Go 端 `LLMConfig.api_key` wire 传输后调用方调用方清零

### 测试策略

```
单元测试: domain 层纯函数, table-driven, mock adapter 注入
  Go:   go test ./... -race
  Node: vitest --coverage
  React: vitest + @testing-library/react

集成测试: 真实 adapter (PG/MinIO/gRPC) 但 mock 外部 LLM
  Go:   环境变量切换 adapter 注册
  CI:   docker compose up -d pg minio && go test -tags=integration ./...

契约测试: Go↔Node gRPC 双向 + BFF↔Go REST
E2E:     Mock LLM + 真实 Conduit → 全链路 Pipeline
```

### 通用禁止项

- `TODO` 必须带跟踪: `// TODO(PHASE-3): ...`
- 禁止魔法数字: 命名常量
- 禁止 `console.log` → 走 Telemetry
- 禁止硬编码超时/重试 → 从 config 读取
- 禁止跨切片 import adapter/ (编译器可 enforce)

---

## 六、Lint/Format 配置速查

### Go

```yaml
# .golangci.yml
linters:
  enable: [errcheck, gosimple, govet, ineffassign, staticcheck, unused, gofmt, goimports, misspell]
```

### Node.js / React

```json
{
  "scripts": {
    "lint": "eslint src/ --ext .ts,.tsx --ignore-pattern src/gen",
    "format": "prettier --write 'src/**/*.{ts,tsx}'",
    "typecheck": "tsc --noEmit"
  }
}
```

### CI 门禁

- Go: `golangci-lint run` + `go test ./... -race`
- Node: `npm run lint && npm run typecheck && npm test`
- React: `npm run lint && npm run typecheck && npm run build`
- Proto: `buf lint` (Phase 1) + `buf breaking --against main` (Phase 3+)
