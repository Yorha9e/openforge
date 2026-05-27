# Phase5d: Prompt 系统重构与接入计划

> 日期: 2026-05-24 | 版本: v4（三次审计修订） | 设计文档: [2026-05-24-prompt-system-design.md](./2026-05-24-prompt-system-design.md)

---

## 0. 前置条件

### 0.1 最终架构

```
L1 (static.xml, 不可覆盖)
  →  L2 (项目融合: 偏好 + stageInstruction + KnowledgeQuerier + 元数据)
    →  ToolInjector (tools_stages.go 独立文件)
      →  L4 (对话, 每轮动态)
```

### 0.2 三次审计问题汇总

| ID | 来源 | 严重度 | 问题 |
|----|------|--------|------|
| C1 | 一审 | 致命 | 4类型+4构造函数在两个文件重复定义，`domain` 包无法编译 |
| C2 | 一审 | 致命 | `selectTemplate()` 调用小写 `getStageTemplate()`，6阶段模板是死代码 |
| C3 | 一审 | 致命 | clarify 模板 3 处重复定义且内容不一致 |
| C4 | 一审 | 致命 | L3 无独立数据源但作为独立层，StageLayer 与 selectTemplate 职责重叠 |
| N1 | 三审 | 致命 | `config/prompts/static.xml` 不存在，无文件读取机制 |
| N2 | 三审 | 致命 | `port.LearningEngineClient` 只有 `WriteKnowledge()`，缺 `QueryKnowledge`/`MatchTrajectory` |
| N3 | 三审 | 致命 | `Bootstrap()` 不创建 PromptBuilder，`OpenForge` struct 无此字段 |
| N4 | 三审 | 高 | Node.js 侧 `ChatRequest` 接口无 `systemPrompt` 字段 |
| N5 | 三审 | 中 | `ToolDefinition`(domain) vs `ToolInfo`(port) 字段名不一致 |
| H1 | 一审 | 高 | `removePattern` 是 no-op |
| H2 | 一审 | 高 | L2 缓存 10min TTL 应为 Pipeline 计数触发 |
| H3 | 一审 | 高 | `port.ChatRequest` 无 `SystemPrompt` |
| H4 | 二审 | 高 | StageLayer 和 selectTemplate 职责重叠 |
| H5 | 二审 | 高 | ws_handler 取 pipeline 上下文在 LLM 之后 |
| H6 | 二审 | 高 | Coordinator Chat/ChatStream 签名不接受 pipeline 上下文 |
| H7 | 二审 | 中 | KnowledgeInjector 自定接口与 port 包冲突 |
| H8 | 一审 | 中 | L1/L2 decompose 模板不应存在 |

---

## 一、Step 0: 架构简化（必须先做，阻断所有后续工作）

### 0.1 删除 prompt_builder.go 中的重复定义

**文件**: `internal/agent/domain/prompt_builder.go`

删除以下所有内容（保留 `PromptBuilder` struct 骨架、`PromptConfig`、`TokenBudget`、`BuildRequest`、`Prompt`、`TokenUsage`、`Message`、`TemplateData`、`PromptMetrics` 类型定义）：

| 删除行 | 内容 | 原因 |
|--------|------|------|
| L440-476 | `StaticLayer` + `NewStaticLayer` + `Build` + `buildStaticContent` | 改为读 static.xml |
| L500-562 | `ProjectLayer` + `NewProjectLayer` + `Build` + `buildProjectContent` | 改为 L2Builder |
| L564-604 | `StageLayer` + `NewStageLayer` + `initTemplates` + `Build` | L3 退化入 L2 |
| L606-636 | `ConversationLayer` + `NewConversationLayer` + `Build` | 改为纯函数 buildL4Content |
| L640-660 | `KnowledgeInjector` + `NewKnowledgeInjector` + `Inject` | 由 knowledge_querier.go 提供 |
| L662-730 | `ToolInjector` + `NewToolInjector` + `Inject` + helpers | 由 tools_stages.go 提供 |
| L731-737 | `ContextInjector` + `NewContextInjector` + `Inject` | 合并入 L2Builder.metadata() |
| L738-752 | `SecurityLayer` + `NewSecurityLayer` + `Sanitize` | 改为 sanitizePrompt 纯函数 |
| L754-767 | `truncateString` + `removePattern` (no-op) | 重写为有效实现 |
| L769-781 | `isReadOnlyTool()` | 迁至 tools_stages.go |
| L784-868 | 4 个 `const clarify{L1,L2,L3,L4}Template` | 由 stage_templates.go 提供 |
| L870-887 | `getStageTemplate()` | 改用 stage_templates.go 的 `GetStageTemplate()` |
| L889-895 | `getGenericTemplate()` | 不再需要 |
| L897-899 | `getGenericStageTemplate()` | 不再需要 |
| L392-406 | `StageTemplate.Sections` + `TemplateSection` + `Condition` | 从未使用 |

### 0.2 重命名 knowledge_injector.go → knowledge_querier.go

**文件变更**: `internal/agent/domain/knowledge_injector.go` → `knowledge_querier.go`

**内容变更**:
- `KnowledgeInjector` → `KnowledgeQuerier`
- 删除 `ToolInjector` struct + `NewToolInjector` + `Inject` + `getToolsForStage` + `filterByPermission`（L369-524）
- 删除 `ContextInjector` struct + `NewContextInjector` + `Inject`（L527-566）
- 删除 `SecurityLayer` struct + `NewSecurityLayer` + `Sanitize` + helpers（L569-619）
- 删除 `ToolRegistry` 接口定义 + `isToolRequired`（L375-523）
- 保留 `LearningEngine` 接口（内部定义 `QueryKnowledge`/`MatchTrajectory`/`WriteKnowledge`）
- 保留 `EmbeddingIndex` 接口 + `KnowledgeCache` + `Query` 方法
- 保留 `isReadOnlyTool()` + `truncateStringWithEllipsis` helper
- `Query` 方法签名: `func (kq *KnowledgeQuerier) Query(ctx context.Context, projectID, query string) (string, error)`

> **注意**: `port.LearningEngineClient` 当前只有 `WriteKnowledge()`。Phase 5d 期间 KnowledgeQuerier 使用 domain 内部自定的 `LearningEngine` 接口（含全部三个方法），但 `learningEngine` 字段为 nil 时静默返回空字符串。`port.LearningEngineClient` 的方法扩展延后到 Phase 7（与真实 LearningEngine 实现一起交付）。

### 0.3 新建 tools_stages.go

**文件**: `internal/agent/domain/tools_stages.go`

```go
// tools_stages.go
// Phase 7: 替换为 ToolRegistry 动态查询

package domain

// ToolDefinition 工具定义
// 注意：字段名与 port.ToolInfo 保持对齐（InputSchema 而非 Parameters），
// 方便 Phase 7 切 ToolRegistry 时消除类型转换
type ToolDefinition struct {
    Name        string
    Description string
    InputSchema map[string]interface{} // ← 与 port.ToolInfo 对齐
    ReadOnly    bool
}

// StageToolMap 硬编码工具列表，按 stage 组织
var StageToolMap = map[string][]ToolDefinition{
    "clarify": {
        {Name: "read_file", Description: "Read file contents", ReadOnly: true},
        {Name: "search_content", Description: "Search with regex", ReadOnly: true},
        {Name: "analyze_topology", Description: "Analyze project topology", ReadOnly: true},
        {Name: "lsp_symbols", Description: "Get document symbols", ReadOnly: true},
    },
    "decompose": {
        {Name: "read_file", Description: "Read file contents", ReadOnly: true},
        {Name: "search_content", Description: "Search with regex", ReadOnly: true},
        {Name: "analyze_topology", Description: "Analyze project topology", ReadOnly: true},
        {Name: "lsp_references", Description: "Find all references", ReadOnly: true},
    },
    "implement": {
        {Name: "acquire_file_lock", Description: "Acquire lock before modification", ReadOnly: false},
        {Name: "release_file_lock", Description: "Release file lock", ReadOnly: false},
        {Name: "read_file", Description: "Read file contents", ReadOnly: true},
        {Name: "edit_file", Description: "Edit existing file", ReadOnly: false},
        {Name: "write_file", Description: "Write new file", ReadOnly: false},
        {Name: "bash", Description: "Execute shell command", ReadOnly: false},
        {Name: "lsp_hover", Description: "Get symbol info", ReadOnly: true},
        {Name: "lsp_definition", Description: "Go to definition", ReadOnly: true},
        {Name: "lsp_references", Description: "Find all references", ReadOnly: true},
    },
    "test": {
        {Name: "read_file", Description: "Read file contents", ReadOnly: true},
        {Name: "edit_file", Description: "Edit file to fix failures", ReadOnly: false},
        {Name: "bash", Description: "Run tests and lint", ReadOnly: false},
        {Name: "search_content", Description: "Search test patterns", ReadOnly: true},
    },
    "deploy": {
        {Name: "bash", Description: "Run deployment commands", ReadOnly: false},
        {Name: "read_file", Description: "Read deployment logs", ReadOnly: true},
        {Name: "manage_sandbox", Description: "Manage deployment sandbox", ReadOnly: false},
    },
    "verify": {
        {Name: "read_file", Description: "Read changed files", ReadOnly: true},
        {Name: "bash", Description: "Run verification scripts", ReadOnly: false},
        {Name: "write_knowledge_delta", Description: "Write learned preferences", ReadOnly: false},
    },
}

// PermissionFilter plan 模式下允许的工具白名单
var PermissionFilter = map[string][]string{
    "plan": {
        "read_file", "search_content", "search_files",
        "analyze_topology", "lsp_hover", "lsp_symbols",
        "lsp_references", "lsp_definition",
        "list_models", "check_token_budget",
        "query_module_ownership", "validate_artifact_hash",
        "generate_artifact_url",
    },
}

func InjectTools(stage, permissionMode string) (string, []ToolDefinition) {
    tools := StageToolMap[stage]
    if tools == nil {
        return "", nil
    }
    if permissionMode == "plan" {
        tools = filterByPermission(tools)
    }
    return buildToolDescription(tools), tools
}

func filterByPermission(tools []ToolDefinition) []ToolDefinition {
    allowed := PermissionFilter["plan"]
    allowedSet := make(map[string]bool, len(allowed))
    for _, name := range allowed {
        allowedSet[name] = true
    }
    var filtered []ToolDefinition
    for _, t := range tools {
        if allowedSet[t.Name] {
            filtered = append(filtered, t)
        }
    }
    return filtered
}
```

### 0.4 重写 PromptBuilder 核心

**文件**: `internal/agent/domain/prompt_builder.go`

```go
// ============================================================
// 类型定义（保留/新增）
// ============================================================

// PromptBuilder 简化后的核心
type PromptBuilder struct {
    l1Content string          // static.xml 内存缓存，启动加载
    l2Builder *L2Builder
    metrics   *PromptMetrics
    mu        sync.RWMutex
}

// L2Builder 负责 L2 层全部内容组装
type L2Builder struct {
    prefs     *ProjectPrefsLoader   // of-prefs.yaml 热重载
    knowledge *KnowledgeQuerier     // L2附属
    mu        sync.RWMutex
}

// L2Request L2 层需要的输入
type L2Request struct {
    ProjectID        string
    PipelineID       string
    Stage            string
    Level            string
    UserQuery        string
    BacktrackReason  string
    BacktrackTarget  string
    ParentPipelineID string
}

// Prompt 构建输出 — 只有 System 文本 + Tools + Token 统计
// Messages 由 QueryEngine 独立管理，不属于 PromptBuilder 职责
type Prompt struct {
    System     string           // 完整 System Prompt（L2 + Tools + L4摘要 + L1）
    Tools      []ToolDefinition // 当前阶段可用工具
    TokenUsage *TokenUsage      // Token 估算
}

// PromptConfig / TokenBudget / BuildRequest / TokenUsage / PromptMetrics
// 保留现有定义，BuildRequest 扩展回溯/子Pipeline字段（见 Step 5.1）

// ============================================================
// 构造器
// ============================================================

// NewPromptBuilder 创建 PromptBuilder
// l1Path: config/prompts/static.xml 路径
// knowledgeQuerier: 可为 nil（Phase 7 前知识查询静默返回空）
func NewPromptBuilder(l1Path string, knowledgeQuerier *KnowledgeQuerier) (*PromptBuilder, error) {
    l1Content, err := os.ReadFile(l1Path)
    if err != nil {
        return nil, fmt.Errorf("read static.xml: %w", err)
    }
    return &PromptBuilder{
        l1Content: string(l1Content),
        l2Builder: &L2Builder{
            prefs:     NewProjectPrefsLoader(30 * time.Second),
            knowledge: knowledgeQuerier,
        },
    }, nil
}

// ============================================================
// 核心 Build 方法
// ============================================================

func (pb *PromptBuilder) Build(ctx context.Context, req *BuildRequest) (*Prompt, error) {
    // 1. L2 构建
    l2Content, err := pb.l2Builder.Build(ctx, &L2Request{
        ProjectID:        req.ProjectID,
        PipelineID:       req.PipelineID,
        Stage:            req.Stage,
        Level:            req.Level,
        UserQuery:        req.UserMessage,
        BacktrackReason:  req.BacktrackReason,
        BacktrackTarget:  req.BacktrackTarget,
        ParentPipelineID: req.ParentPipelineID,
    })
    if err != nil {
        return nil, fmt.Errorf("l2 build: %w", err)
    }

    // 2. Tool 注入
    toolText, toolDefs := InjectTools(req.Stage, req.PermissionMode)

    // 3. L4 对话摘要（注入 SystemPrompt，不单独返回 Messages）
    l4Summary := buildL4Summary(req.ConversationHistory)

    // 4. 拼接 System: L2 + Tools + L4摘要 + L1（L1 最后，不可覆盖）
    systemPrompt := strings.Join([]string{l2Content, toolText, l4Summary, pb.l1Content}, "\n")

    // 5. 安全清理
    systemPrompt = sanitizePrompt(systemPrompt)

    // 6. 计算 token
    tokenUsage := calcTokenUsage(systemPrompt, req.ConversationHistory, toolDefs)

    return &Prompt{
        System:     systemPrompt,
        Tools:      toolDefs,
        TokenUsage: tokenUsage,
    }, nil
}

// ============================================================
// L2Builder 内部方法
// ============================================================

func (l2 *L2Builder) Build(ctx context.Context, req *L2Request) (string, error) {
    var parts []string

    // 1. 项目偏好（Phase 5d 返回默认值，Phase 6 读 of-prefs.yaml）
    if prefs := l2.prefs.Get(req.ProjectID); prefs != "" {
        parts = append(parts, prefs)
    }

    // 2. 阶段指令（项目覆盖优先 → 服务器模板兜底 → 通用 fallback）
    parts = append(parts, l2.stageInstruction(req.Stage, req.Level, req.ProjectID))

    // 3. 知识查询（learningEngine 为 nil 时静默返回空）
    if knowledge, err := l2.knowledge.Query(ctx, req.ProjectID, req.UserQuery); err == nil && knowledge != "" {
        parts = append(parts, knowledge)
    }

    // 4. 元数据
    parts = append(parts, l2.metadata(req))

    return strings.Join(parts, "\n"), nil
}

// stageInstruction 阶段指令：项目覆盖 > 服务器默认 > 通用 fallback
func (l2 *L2Builder) stageInstruction(stage, level, projectID string) string {
    // 1. 查 of-prefs.yaml 项目覆盖
    if override := l2.prefs.GetStageOverride(projectID, stage, level); override != "" {
        return override
    }
    // 2. 兜底服务器模板（直接引用 stage_templates.go 全局变量）
    if tmpl := GetStageTemplate(stage, level); tmpl != nil {
        return tmpl.Template
    }
    // 3. 通用 fallback
    return fmt.Sprintf("<stage_instructions stage=\"%s\" level=\"%s\">Execute according to Pipeline rules.</stage_instructions>", stage, level)
}

// metadata 构建元数据上下文
func (l2 *L2Builder) metadata(req *L2Request) string {
    var parts []string
    parts = append(parts, fmt.Sprintf("<pipeline_id>%s</pipeline_id>", req.PipelineID))
    parts = append(parts, fmt.Sprintf("<project_id>%s</project_id>", req.ProjectID))
    parts = append(parts, fmt.Sprintf("<current_time>%s</current_time>", time.Now().Format(time.RFC3339)))

    if req.BacktrackReason != "" {
        parts = append(parts, fmt.Sprintf("<backtrack_context reason=\"%s\" target=\"%s\"/>", req.BacktrackReason, req.BacktrackTarget))
    }
    if req.ParentPipelineID != "" {
        parts = append(parts, fmt.Sprintf("<parent_pipeline id=\"%s\"/>", req.ParentPipelineID))
    }

    return "<context>\n" + strings.Join(parts, "\n") + "\n</context>"
}

// ============================================================
// ProjectPrefsLoader（Phase 5d 返回默认值，Phase 6 接 of-prefs.yaml + mtime 热重载）
// ============================================================

type ProjectPrefsLoader struct {
    interval time.Duration
    mu       sync.RWMutex
}

func NewProjectPrefsLoader(interval time.Duration) *ProjectPrefsLoader {
    return &ProjectPrefsLoader{interval: interval}
}

// Get 返回项目偏好内容（Phase 5d 返回空，Phase 6 读文件 + 缓存 + 热重载）
func (pl *ProjectPrefsLoader) Get(projectID string) string {
    return "" // Phase 6 实现
}

// GetStageOverride 返回项目覆盖的阶段指令（Phase 5d 返回空）
func (pl *ProjectPrefsLoader) GetStageOverride(projectID, stage, level string) string {
    return "" // Phase 6 实现
}

// ============================================================
// L4 纯函数
// ============================================================

// buildL4Summary 构建对话摘要（注入 SystemPrompt，不返回 Messages）
// Messages 数组完全由 QueryEngine 独立管理
func buildL4Summary(history []port.Message) string {
    if len(history) == 0 {
        return ""
    }
    // 取最近 5 轮（10 条消息）
    recent := history
    if len(recent) > 10 {
        recent = recent[len(recent)-10:]
    }
    var b strings.Builder
    b.WriteString("<conversation_summary>\n")
    for i, msg := range recent {
        content := msg.Content
        if len(content) > 200 {
            content = content[:200] + "..."
        }
        b.WriteString(fmt.Sprintf("<msg seq=\"%d\" role=\"%s\">%s</msg>\n", i+1, msg.Role, content))
    }
    b.WriteString("</conversation_summary>")
    return b.String()
}

// ============================================================
// 安全清理（原 removePattern no-op 修复）
// ============================================================

func sanitizePrompt(content string) string {
    patterns := []string{
        "SYSTEM:", "指令", "you are now",
        "ignore previous", "disregard instructions",
    }
    for _, p := range patterns {
        content = strings.ReplaceAll(content, p, "")
    }
    return content
}

// ============================================================
// Token 估算
// ============================================================

func calcTokenUsage(system string, messages []port.Message, tools []ToolDefinition) *TokenUsage {
    usage := &TokenUsage{}
    usage.StaticTokens = len(system) / 4
    for _, msg := range messages {
        usage.ConversationTokens += len(msg.Content) / 4
    }
    for _, t := range tools {
        usage.ToolTokens += len(t.Description) / 4
    }
    usage.TotalTokens = usage.StaticTokens + usage.ConversationTokens + usage.ToolTokens
    return usage
}
```

**类型别名**: `prompt_builder.go` 中 `type Message = port.Message`，消除 domain 与 port 之间的 Message 转换。`BuildRequest.ConversationHistory` 类型统一为 `[]port.Message`（值切片，非指针）。

### 0.5 新建 L1 静态文件

**文件**: `config/prompts/static.xml`（新建）

```xml
<system_prompt>
  <identity>
    <role>You are OpenForge, an AI-driven full-stack development agent.</role>
    <mission>Execute software engineering tasks across the complete lifecycle.</mission>
  </identity>

  <security>
    <audit>All operations are audited (WORM).</audit>
    <gate>Never bypass the Gate approval system.</gate>
    <attribution>Code changes attributed to approving human via Author="&lt;user&gt; via OpenForge".</attribution>
    <sandwich>Sandwich Architecture: System Zone / Data Zone / Output Zone.</sandwich>
    <license>Never generate GPL/AGPL code. All agent actions execute ONLY through tool_use content blocks.</license>
  </security>

  <code_conventions>
    <convention>NO COMMENTS unless asked</convention>
    <convention>Follow existing code style</convention>
    <convention>Check package.json / go.mod before using libraries</convention>
    <convention>Prefer editing existing files over creating new ones</convention>
    <convention>No backwards-compatibility hacks</convention>
    <convention>Never expose or log secrets/keys</convention>
    <convention>Never commit secrets to the repository</convention>
  </code_conventions>
</system_prompt>
```

### 0.6 删除 decompose 的 L1/L2 模板

**文件**: `internal/agent/domain/stage_templates.go`

删除 `decomposeTemplates["L1"]` 和 `decomposeTemplates["L2"]` 条目。

### 0.7 编译验证

```bash
go build ./internal/agent/domain/...   # 必须通过
go vet ./internal/agent/domain/...     # 零 warning
```

---

## 二、数据模型打通

### Layer 1: 数据模型（自底向上）

| # | 文件 | 变更 |
|---|------|------|
| 1.1 | `proto/agent/v1/llm.proto` | `LLMChatRequest` 加 `string system_prompt = 6;` |
| 1.2 | `internal/agent/port/llm_client.go` | `ChatRequest` 加 `SystemPrompt string` |
| 1.3 | `gen/go/agent/v1/` | `buf generate` 重新生成 Go 代码 |
| 1.4 | `nodejs-io/src/gen/agent/v1/llm_pb.ts` | `buf generate` 重新生成 TypeScript 代码 |

### Layer 2: 适配层穿透

| # | 文件 | 变更 |
|---|------|------|
| 2.1 | `internal/llm/router.go` | `Chat()` L49 + `ChatStream()` L110 的 `llmReq` 加 `SystemPrompt: req.SystemPrompt` |
| 2.2 | `internal/agent/adapter/grpc_llm_client.go` | `toProtoRequest()` L88 加 `SystemPrompt: req.SystemPrompt` |
| 2.3 | `nodejs-io/src/kernel/interfaces.ts` | `ChatRequest` 接口加 `systemPrompt?: string` |
| 2.4 | `nodejs-io/src/` LLM Router 实现 | 从 `ChatRequest.systemPrompt` 读取并传递给 Provider |

### Layer 3: QueryEngine 接入

| # | 文件 | 变更 |
|---|------|------|
| 3.1 | `query_engine.go` | 加 `promptBuilder *PromptBuilder` + `pipelineCtx PipelineContext` 字段 |
| 3.2 | `query_engine.go` | 加 `PipelineContext{PipelineID, ProjectID, Stage, Level, PermissionMode, UserRole}` 类型 |
| 3.3 | `query_engine.go` | `NewQueryEngine(llmClient, config, promptBuilder, pipelineCtx)` |
| 3.4 | `query_engine.go` | `SubmitMessage()` 内调用 `pb.Build(ctx, req)` → `ChatRequest.SystemPrompt = prompt.System` |
| 3.5 | `query_engine.go` | `domain.Message` → `type Message = port.Message` 类型别名，消除转换 |

### Layer 4: 入口点 + Bootstrap

| # | 文件 | 变更 |
|---|------|------|
| 4.1 | `internal/shared/profile/bootstrap.go` | `OpenForge` struct 加 `PromptBuilder *domain.PromptBuilder` 字段 |
| 4.2 | `internal/shared/profile/bootstrap.go` | `Bootstrap()` 中 `NewPromptBuilder("config/prompts/static.xml", knowledgeQuerier)` → 挂到 `of.PromptBuilder` |
| 4.3 | `internal/server/ws_handler.go` | `handleMessage` 中 `chat.send`：**提前** `PipelineRepo.GetByID()` 到 LLM 之前，取 stage/level/status 构造 `PipelineContext` |
| 4.4 | `internal/server/ws_handler.go` | `wsConn` 通过 `c.of.PromptBuilder` 获取；`getOrCreateEngine()` 传入 `PromptBuilder` + `PipelineContext` |
| 4.5 | `internal/agent/domain/coordinator.go` | `Chat()` L122 / `ChatStream()` L130 的 `port.ChatRequest` 加 `SystemPrompt`；Coordinator 加 `promptBuilder *PromptBuilder` 字段 + `NewCoordinator` 签名加参 |
| 4.6 | `cmd/openforge/main.go` | 创建 `PromptBuilder`，构造 `port.ChatRequest` 时补 `SystemPrompt` |
| 4.7 | `cmd/server/main.go` | 确认 `of.PromptBuilder` 已由 Bootstrap 注入，无需额外处理 |

### Layer 5: 扩展字段

| # | 文件 | 变更 |
|---|------|------|
| 5.1 | `prompt_builder.go` | `BuildRequest` 加 `BacktrackReason/Target/Evidence` + `ParentPipelineID` |
| 5.2 | `prompt_builder.go` | `L2Builder.metadata()` 检测非空回溯/子Pipeline字段，注入对应 XML 段 |
| 5.3 | `prompt_builder.go` | `ProjectPrefsLoader` 实现：定时轮询 mtime 热重载 + 内存缓存 |

---

## 三、验证清单

### 编译

```
[ ] go build ./internal/agent/domain/...    ← Step 0 后必须通过
[ ] go build ./cmd/server/...               ← Step 4 后必须通过
[ ] go build ./cmd/openforge/...            ← Step 4 后必须通过
[ ] go build ./...                           ← 全量编译
[ ] go vet ./...                             ← 零 warning
[ ] go test ./internal/agent/domain/...      ← 全部通过
```

### 功能路径

```
[ ] ws_handler clarify → PromptBuilder.Build() → SystemPrompt → LLM 正常返回
[ ] ws_handler implement → SystemPrompt含 acquire_file_lock 指令 → LLM 正常返回
[ ] ws_handler deploy → SystemPrompt含 dry-run→apply→verify→rollback
[ ] ws_handler verify → SystemPrompt含 write_knowledge_delta
[ ] CLI 路径 → PromptBuilder(minimal) → SystemPrompt注入 → LLM 正常返回
[ ] Coordinator 路径 → SystemPrompt 传递 → LLM 正常返回
[ ] ConnectRPC 路径(Go) → proto system_prompt 字段有值
[ ] ConnectRPC 路径(Node.js) → ChatRequest 接收 systemPrompt → Provider 使用
```

### 场景

```
[ ] plan 模式 → 只返回 PermissionFilter 白名单工具
[ ] 回溯场景 → prompt 含 <backtrack_context reason="..." target="..."/>
[ ] 子 Pipeline → prompt 含 <parent_pipeline id="..."/>
[ ] 模型切换 → TokenBudget 随模型 ContextWindow 重算
[ ] L1 不可覆盖 → static.xml 的 security/code_conventions 段在拼接最末尾
[ ] stage 模板覆盖 → of-prefs.yaml 自定义 implement.L3 生效（Phase 6 验证）
[ ] decompose L1 不存在 → GetStageTemplate("decompose", "L1") == nil
[ ] 不存在 stage/level → WARN 日志 + 通用 fallback 提示
[ ] static.xml 缺失 → NewPromptBuilder 返回 error，服务启动失败（fast-fail）
[ ] KnowledgeQuerier.learningEngine == nil → Query() 静默返回 ""，不影响 Build
[ ] Messages 管理 → Prompt 不含 Messages 字段，QueryEngine 独立管理 ChatRequest.Messages
[ ] 对话摘要 → SystemPrompt 中含 <conversation_summary> 最近 5 轮
```

---

## 四、影响文件总览

```
新建:
  config/prompts/static.xml                              ← L1 不可覆盖内容
  internal/agent/domain/tools_stages.go                  ← StageToolMap + InjectTools

重写:
  internal/agent/domain/prompt_builder.go                ← 删除 ~400行 + 新 PromptBuilder/L2Builder

重命名:
  internal/agent/domain/knowledge_injector.go            ← → knowledge_querier.go
    (KnowledgeInjector → KnowledgeQuerier, 删 Tool/Context/Security)

修改:
  internal/agent/domain/stage_templates.go               ← 删 decompose L1/L2
  internal/agent/domain/query_engine.go                  ← +PromptBuilder +PipelineContext
  internal/agent/domain/coordinator.go                   ← +promptBuilder 字段 + 签名变更
  proto/agent/v1/llm.proto                               ← +system_prompt 字段
  gen/go/agent/v1/llm.pb.go                              ← buf generate（自动）
  gen/go/agent/v1/agentv1connect/llm.connect.go          ← buf generate（自动）
  internal/agent/port/llm_client.go                       ← ChatRequest.SystemPrompt
  internal/llm/router.go                                  ← 2处 SystemPrompt 传递
  internal/agent/adapter/grpc_llm_client.go               ← toProtoRequest 传递
  internal/shared/profile/bootstrap.go                    ← OpenForge.PromptBuilder + 初始化
  internal/server/ws_handler.go                           ← 提前取pipeline + 传入PromptBuilder
  cmd/openforge/main.go                                   ← PromptBuilder + SystemPrompt
  cmd/server/main.go                                      ← 确认（通常无需改动）
  nodejs-io/src/gen/agent/v1/llm_pb.ts                   ← buf generate（自动）
  nodejs-io/src/kernel/interfaces.ts                      ← ChatRequest.systemPrompt
  nodejs-io/src/llm/                                     ← LLM Router 传递 systemPrompt
```

共 **20 个文件**（10 新建/重写 + 10 修改），不含 proto/TS 自动生成。

---

## 五、实施顺序

```
Step 0: 架构简化 (0.1-0.7)
  ├── 0.1 删除 prompt_builder.go 重复定义（~400行）
  ├── 0.2 knowledge_injector.go → knowledge_querier.go（删 Tool/Context/Security）
  ├── 0.3 新建 tools_stages.go（StageToolMap + InjectTools）
  ├── 0.4 重写 PromptBuilder 核心（L1+L2Builder+L4+buildMessages）
  ├── 0.5 新建 config/prompts/static.xml
  ├── 0.6 删 decompose L1/L2 模板
  └── 0.7 go build ./internal/agent/domain/... ✅
  
  ⚠️ 0.1-0.6 全部在 domain 包内，无外部依赖，可独立完成

Step 1: 数据模型 + Proto (1.1-1.4)
  ⚠️ 1.1-1.2 零依赖，1.3-1.4 依赖 1.1

Step 2: 适配层穿透 (2.1-2.4)
  ⚠️ 2.1-2.2 Go 侧，零依赖；2.3-2.4 Node.js 侧，依赖 1.4

Step 3: Bootstrap + QueryEngine (4.1-4.2, 3.1-3.5)
  ⚠️ Bootstrap 先改（4.1-4.2），再改 QueryEngine（依赖 Step 2）

Step 4: 入口点接入 (4.3-4.7)
  ⚠️ ws_handler 依赖 Step 3，Coordinator 独立，CLI 独立

Step 5: 扩展字段 (5.1-5.3)
  ⚠️ 零依赖，最后加

Step 6: 全链路验证
```

---

## 六、延后项

| 项目 | 目标 Phase | 说明 |
|------|-----------|------|
| `port.LearningEngineClient` 加 `QueryKnowledge`/`MatchTrajectory` | Phase 7 | 等真实 LearningEngine 实现 |
| 服务器模板外部化（Go string → `templates/stages/*.xml` + embed.FS） | Phase 6 | |
| ToolRegistry 嵌入索引替换 `tools_stages.go` 硬编码 | Phase 7 | |
| Prompt 灰度发布（canary YAML） | Phase 7 | 依赖模板外部化 |
| Anthropic Prompt Cache 断点（cache_control） | Phase 6 | |
| SecurityLayer Zone 结构隔离 | Phase 6 | |
| Stage 间摘要压缩（~700 tokens） | Phase 6 | |
| Token 估算对接 ring buffer | Phase 7 | |
| `of-prefs.yaml` 文件格式定义 + 解析器 | Phase 6 | Phase5d 仅定义接口，返回硬编码默认值 |
