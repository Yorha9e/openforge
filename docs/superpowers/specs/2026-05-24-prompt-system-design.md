# OpenForge Prompt 系统设计

> 日期: 2026-05-24 | 状态: 设计完成 | 替代: 原 Prompt拼接机制设计文档.md

---

## 一、概述

对 Prompt 拼接机制做架构简化：四层减为三层（L1/L2/L4），L3 退化入 L2，注入器精简为 KnowledgeQuerier（L2 附属）+ ToolInjector（独立文件）。

### 核心变更

| 原设计 | 新设计 | 理由 |
|--------|--------|------|
| L1-L4 四层独立构建 | L1/L2/L4 三层 | L3 无独立数据源，内容可由 L2 内部纯函数生成 |
| 三个注入器并列 | KnowledgeQuerier 入 L2，ToolInjector 保留 | ContextInjector 只加时间戳，并入 L2 |
| 模板散落 3 处 | 服务器 templates/ + 项目 of-prefs.yaml 两级覆盖 | 消除重复，建立优先级 |
| 工具硬编码在 injector 内 | 独立 tools_stages.go 文件 | 方便 Phase 7 替换为 ToolRegistry |

---

## 二、架构

```
┌──────────────────────────────────────────────────────────┐
│                    PromptBuilder.Build()                   │
│                                                          │
│  L1 (static.xml)  ──→  L2 (项目融合)  ──→  L4 (对话)     │
│   启动加载，不可覆盖      │                   每轮动态       │
│   身份+安全+规范         │                                │
│                          ├── ProjectPrefs (of-prefs.yaml) │
│                          ├── StageInstruction (纯函数)     │
│                          ├── KnowledgeQuerier (L2附属)    │
│                          └── Metadata (时间/ID)           │
│                                                          │
│  ToolInjector: tools_stages.go (独立文件)                 │
│    StageToolMap + PermissionFilter → 工具文本+定义列表     │
│    Phase 7 → ToolRegistry.Search()                       │
└──────────────────────────────────────────────────────────┘
```

### 数据流

```
PromptBuilder.Build(ctx, BuildRequest)
  │
  ├── 1. L1.内容（static.xml，内存缓存，启动加载）
  │
  ├── 2. L2Builder.Build(ctx, L2Request)
  │       ├── ProjectPrefs（of-prefs.yaml，内存缓存 + mtime 热重载）
  │       ├── stageInstruction(stage, level)
  │       │     → 查 of-prefs.yaml → 兜底服务器 templates/stages/*.xml
  │       ├── KnowledgeQuerier.Query(ctx, projectID, userQuery)
  │       │     → 5min TTL 缓存 → LearningEngine
  │       └── Metadata（pipeline_id, project_id, timestamp）
  │
  ├── 3. ToolInjector.Inject(stage, permissionMode)
  │       ├── 查 StageToolMap[stage]
  │       ├── plan模式 → 过滤 PermissionFilter["plan"]
  │       └── 返回 (工具描述文本, []ToolDefinition)
  │
  └── 4. L4.内容（最近5轮对话摘要 + 当前用户消息）
```

---

## 三、各层详细设计

### 3.1 L1 — 静态安全层（不可覆盖）

**文件**: `config/prompts/static.xml`

**加载**: 服务启动时 `embed.FS` 或 `os.ReadFile` 加载到内存，永不重新读取。

**不可覆盖规则**: L1 内容在 `fillTemplate` 时**最后拼接**，作为 System Prompt 的末尾。项目模板无法通过 `{{.X}}` 占位符覆盖 L1 内容。

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

### 3.2 L2 — 项目融合层

L2 内部由 `L2Builder` 统一管理，不再是多个独立注入器的拼装。

```go
// L2Builder 负责L2层全部内容组装
type L2Builder struct {
    prefs           *ProjectPrefsLoader
    knowledge       *KnowledgeQuerier
    serverTemplates map[string]map[string]string  // stage → level → XML
    mu              sync.RWMutex
}

type L2Request struct {
    ProjectID  string
    PipelineID string
    Stage      string
    Level      string
    UserQuery  string
}

func (l2 *L2Builder) Build(ctx context.Context, req *L2Request) (string, error) {
    var parts []string

    // 1. 项目偏好
    if prefs := l2.prefs.Get(req.ProjectID); prefs != "" {
        parts = append(parts, prefs)
    }

    // 2. 阶段指令（项目优先 → 服务器兜底）
    parts = append(parts, l2.stageInstruction(req.Stage, req.Level, req.ProjectID))

    // 3. 知识查询
    if knowledge, err := l2.knowledge.Query(ctx, req.ProjectID, req.UserQuery); err == nil && knowledge != "" {
        parts = append(parts, knowledge)
    }

    // 4. 元数据
    parts = append(parts, l2.metadata(req))

    return strings.Join(parts, "\n"), nil
}
```

#### 3.2.1 stageInstruction 查找优先级

```
1. of-prefs.yaml → stages.{stage}.{level}.*   (项目覆盖)
2. templates/stages/{stage}_{level}.xml        (服务器默认)
3. 都不存在 → WARN 日志 + 通用提示
```

#### 3.2.2 热重载

```go
// ProjectPrefsLoader 项目偏好加载器
type ProjectPrefsLoader struct {
    cache    sync.Map                        // projectID → *prefEntry
    interval time.Duration                   // 轮询间隔，默认 30s
}

type prefEntry struct {
    content   string
    mtime     time.Time
    loadedAt  time.Time
}

func (pl *ProjectPrefsLoader) Get(projectID string) string {
    entry, _ := pl.cache.Load(projectID)
    if entry == nil {
        return pl.load(projectID)
    }
    e := entry.(*prefEntry)
    // 检查文件 mtime 是否变化
    if stat, err := os.Stat(pl.prefPath(projectID)); err == nil {
        if stat.ModTime().After(e.mtime) {
            return pl.load(projectID)
        }
    }
    return e.content
}
```

### 3.3 KnowledgeQuerier（L2 附属）

**文件**: `internal/agent/domain/knowledge_querier.go`（由 `knowledge_injector.go` 重命名）

```go
// KnowledgeQuerier 知识查询器
// 职责：对接LearningEngine，查询相关知识和轨迹
// 被L2Builder调用，不是独立注入器
type KnowledgeQuerier struct {
    learningEngine LearningEngine
    cache          sync.Map   // key → *queryCacheEntry (5min TTL)
}

type queryCacheEntry struct {
    content   string
    expiresAt time.Time
}

func (kq *KnowledgeQuerier) Query(ctx context.Context, projectID, query string) (string, error) {
    key := fmt.Sprintf("%s:%s", projectID, query)
    if entry, ok := kq.cache.Load(key); ok {
        e := entry.(*queryCacheEntry)
        if time.Now().Before(e.expiresAt) {
            return e.content, nil
        }
    }

    if kq.learningEngine == nil {
        return "", nil
    }

    // 并行查询偏好 + 轨迹
    prefs, _ := kq.learningEngine.QueryKnowledge(ctx, &QueryKnowledgeRequest{
        ProjectID: projectID, Query: query, TopK: 5, Level: "all",
    })
    trajs, _ := kq.learningEngine.MatchTrajectory(ctx, &MatchTrajectoryRequest{
        ProjectID: projectID, Query: query, TopK: 3, Level: "L3_trajectory",
    })

    content := kq.format(prefs, trajs)
    kq.cache.Store(key, &queryCacheEntry{content: content, expiresAt: time.Now().Add(5 * time.Minute)})
    return content, nil
}
```

### 3.4 ToolInjector（独立文件）

**文件**: `internal/agent/domain/tools_stages.go`

```go
// tools_stages.go
// Phase 7: 替换为 ToolRegistry 动态查询

// StageToolMap 硬编码工具列表（按 stage 组织）
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

// PermissionFilter plan模式下允许的工具白名单
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

type ToolDefinition struct {
    Name        string
    Description string
    ReadOnly    bool
}

// Inject 注入工具列表（被PromptBuilder调用）
func InjectTools(stage, permissionMode string) (string, []ToolDefinition) {
    tools := StageToolMap[stage]
    if permissionMode == "plan" {
        tools = filterByPermission(tools)
    }
    return buildToolDescription(tools), tools
}
```

### 3.5 L4 — 对话层

- 最近 5 轮对话（10 条消息），压缩为 `<conversation_summary>` XML 摘要
- 摘要注入 SystemPrompt，**不单独返回 Messages 数组**
- Messages（user/assistant 对话）完全由 QueryEngine 独立管理
- 不做缓存，每轮动态构建

---

## 四、文件结构

```
config/prompts/
  static.xml                           ← L1 不可覆盖内容
  stages/                              ← 服务器默认阶段模板
    clarify_L1.xml
    clarify_L2.xml
    clarify_L3.xml
    clarify_L4.xml
    decompose_L3.xml                   ← 无 L1/L2
    decompose_L4.xml
    implement_L1.xml
    implement_L2.xml
    implement_L3.xml
    implement_L4.xml
    test_L1.xml
    test_L3.xml
    deploy_L1.xml
    deploy_L3.xml
    verify_L1.xml
    verify_L3.xml

internal/agent/domain/
  prompt_builder.go                    ← PromptBuilder 核心 + L1/L2/L4 Build
  stage_templates.go                   ← 服务器默认模板常量（删 decompose L1/L2 + 删重复）
  tools_stages.go                      ← StageToolMap + PermissionFilter（独立文件）
  knowledge_querier.go                 ← KnowledgeQuerier（L2 附属, 5min 缓存）
  query_engine.go                      ← 接入 PromptBuilder
  coordinator.go                       ← 接入 PromptBuilder

项目目录/
  of-prefs.yaml                        ← 项目偏好 + 可选 stages 覆盖
```

### of-prefs.yaml 示例

```yaml
# 项目偏好
project:
  name: "Conduit"
  language: "typescript"
  framework: "express+react"

preferences:
  auth_validation: "zod"
  error_handling: "try-catch"
  naming: "camelCase"

# 可选：覆盖服务器默认阶段模板
stages:
  implement:
    L3:
      objective: "根据Conduit代码规范实现功能"
      constraints:
        - "使用 zod 做输入校验"
        - "API 返回格式遵循 RealWorld spec"
```

---

## 五、删除清单

| 文件 | 删除项 |
|------|--------|
| `prompt_builder.go` | `KnowledgeInjector` struct + `NewKnowledgeInjector` + `Inject` |
| `prompt_builder.go` | `ToolInjector` struct + `NewToolInjector` + `Inject` + `getToolsForStage` + `filterByPermission` |
| `prompt_builder.go` | `ContextInjector` struct + `NewContextInjector` |
| `prompt_builder.go` | `SecurityLayer` struct + `NewSecurityLayer` + `Sanitize` |
| `prompt_builder.go` | `StageLayer` struct + `NewStageLayer` + `initTemplates` + `Build` |
| `prompt_builder.go` | `StageTemplate.Sections` + `TemplateSection` + `Condition` |
| `prompt_builder.go` | `isReadOnlyTool()` |
| `prompt_builder.go` | `const clarify{L1,L2,L3,L4}Template` 4 个常量 |
| `prompt_builder.go` | `getStageTemplate()` / `getGenericTemplate()` / `getGenericStageTemplate()` |
| `prompt_builder.go` | `removePattern` no-op |
| `knowledge_injector.go` | `ToolInjector` + `ContextInjector` + `SecurityLayer` 类型及其方法 |
| `knowledge_injector.go` | → 重命名为 `knowledge_querier.go`，类型重命名为 `KnowledgeQuerier` |
| `stage_templates.go` | `decomposeTemplates["L1"]` / `decomposeTemplates["L2"]` |

---

## 六、PromptBuilder 核心接口

```go
// PromptBuilder 简化后的核心
type PromptBuilder struct {
    l1Content   string              // static.xml 内存缓存
    l2Builder   *L2Builder          // L2 项目融合
    l4Builder   *L4Builder          // L4 对话
    tokenBudget *TokenBudget
    mu          sync.RWMutex
}

type BuildRequest struct {
    PipelineID     string
    ProjectID      string
    Stage          string
    Level          string
    PermissionMode string
    UserRole       string
    UserMessage    string

    // 对话上下文
    ConversationHistory []*port.Message

    // 回溯上下文
    BacktrackReason  string
    BacktrackTarget  string
    BacktrackEvidence string

    // 子Pipeline上下文
    ParentPipelineID string
}

type Prompt struct {
    System     string           // 完整 System Prompt（L2 + Tools + L4摘要 + L1）
    Tools      []ToolDefinition // 可用工具列表
    TokenUsage *TokenUsage      // Token 估算
}
// 注意：Prompt 不含 Messages 字段
// Messages（user/assistant 对话）完全由 QueryEngine 独立管理
```

---

## 七、与现有系统的集成点

同 Phase5d 计划，核心变更：

1. `port.ChatRequest` 加 `SystemPrompt string`
2. `llm.Router` / `adapter.LLMClient` 穿透传递 `SystemPrompt`
3. `proto/agent/v1/llm.proto` 加 `system_prompt` 字段
4. `QueryEngine` 持有 `PromptBuilder`，`SubmitMessage()` 内调用 `Build()`
5. `ws_handler` 提前取 pipeline 上下文，传入 `QueryEngine`

---

## 八、Phase 7 预留扩展点

| 当前实现 | Phase 7 替换 |
|---------|-------------|
| `tools_stages.go` 硬编码 `StageToolMap` | 改为调 `ToolRegistry.Search(ctx, stage, topK)` |
| `KnowledgeQuerier` 空实现（LearningEngine stub） | 对接真实 pgvector 嵌入索引 |
| 服务器模板 `templates/stages/*.xml` 硬编码 | 可热重载的文件系统模板 |
| `PermissionFilter` 白名单硬编码 | 调 `Tool.IsReadOnly()` 接口动态判定 |

---

## 九、设计决策记录

| 决策 | 理由 |
|------|------|
| L3 退化入 L2 | L3 无独立数据源，本质是查表函数 |
| L1 不可覆盖 | 安全规则必须全局强制，不能被项目 `of-prefs.yaml` 污染 |
| L2 stage 模板项目优先 | PM 可按项目定制阶段行为，服务器提供默认 |
| XML 格式 | 结构化好，未来可做条件渲染 |
| ToolInjector 独立文件 | Phase 7 替换时只需改 `tools_stages.go` 一个文件 |
| KnowledgeQuerier 为 L2 附属 | 知识查询结果本质是项目偏好的延伸 |
| 硬编码工具列表保留至 Phase 7 | ToolRegistry 嵌入索引尚未实现 |
