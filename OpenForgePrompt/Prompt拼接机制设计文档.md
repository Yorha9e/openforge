# OpenForge Prompt 拼接机制设计文档

## 1. 概述

本文档为 OpenForge 项目设计一个最适合的 Prompt 拼接机制。基于 OpenForge 的三层架构（工作台、Pipeline 引擎、Agent Swarm）和企业级特性，我们设计了一个**分层模板系统**（Layered Template System），结合了结构化模板、Pipeline 阶段感知、动态注入和分层缓存机制。

## 2. 设计原则

### 2.1 核心原则

1. **Pipeline 阶段感知**：根据当前阶段（Clarify/Decompose/Implement/Test/Deploy/Verify）动态调整 prompt 内容
2. **复杂度感知**：根据 L1-L4 复杂度级别调整 prompt 详细程度
3. **权限模式感知**：根据权限模式（plan/auto/default/bypass）调整工具和操作说明
4. **分层缓存**：实现 L1-L4 四层缓存，优化 token 使用
5. **动态注入**：支持自学习知识、工具描述、上下文等动态内容注入
6. **安全隔离**：遵循 Sandwich Architecture，防止 prompt 注入

### 2.2 设计目标

- **可维护性**：结构化模板，易于更新和维护
- **可扩展性**：支持动态注入新内容（工具、技能、知识）
- **性能**：通过分层缓存减少 token 使用
- **一致性**：标准化接口确保 prompt 质量
- **安全性**：防止 prompt 注入，确保审计完整性

## 3. 架构设计

### 3.1 整体架构

```
┌─────────────────────────────────────────────────────────────┐
│                    PromptBuilder 核心引擎                      │
├─────────────────────────────────────────────────────────────┤
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐          │
│  │  L1 静态层   │  │  L2 项目层   │  │  L3 阶段层   │          │
│  │  (缓存)     │  │  (缓存)     │  │  (动态)     │          │
│  └─────────────┘  └─────────────┘  └─────────────┘          │
│         ↓               ↓               ↓                   │
│  ┌─────────────────────────────────────────────────────┐    │
│  │              L4 对话层 (动态)                         │    │
│  └─────────────────────────────────────────────────────┘    │
│                            ↓                                 │
│  ┌─────────────────────────────────────────────────────┐    │
│  │              模板组装引擎                              │    │
│  │  • 阶段模板选择    • 复杂度调整    • 权限过滤          │    │
│  └─────────────────────────────────────────────────────┘    │
│                            ↓                                 │
│  ┌─────────────────────────────────────────────────────┐    │
│  │              安全层 (Sandwich Architecture)            │    │
│  │  • System Zone  • Data Zone  • Output Zone           │    │
│  └─────────────────────────────────────────────────────┘    │
└─────────────────────────────────────────────────────────────┘
```

### 3.2 分层缓存架构

```
L1 静态层 (Static Layer):
  内容: 通用规则 + 代码规范 + 安全策略
  缓存策略: Anthropic Prompt Cache，高命中率
  更新频率: 极少更新（仅安全策略变更时）
  Token 预算: ~2000 tokens

L2 项目层 (Project Layer):
  内容: 项目偏好 + 模块索引 + 拓扑摘要
  缓存策略: 每 10 Pipeline 刷新
  更新频率: 中等频率
  Token 预算: ~1500 tokens

L3 阶段层 (Stage Layer):
  内容: 当前阶段模板 + 阶段特定规则 + 阶段产物摘要
  缓存策略: 每阶段刷新
  更新频率: 高频率
  Token 预算: ~2000 tokens

L4 对话层 (Conversation Layer):
  内容: 最近 5 轮对话 + 检查点上下文 + 自学习知识
  缓存策略: 每轮动态
  更新频率: 极高频率
  Token 预算: ~3000 tokens

总 Token 预算: ~8500 tokens (可根据模型调整)
```

## 4. 核心组件设计

### 4.1 PromptBuilder 类

```go
// PromptBuilder 构建发送给 LLM 的完整 prompt
type PromptBuilder struct {
    // 配置
    config *PromptConfig
    
    // 缓存层
    staticLayer    *StaticLayer    // L1
    projectLayer   *ProjectLayer   // L2
    stageLayer     *StageLayer     // L3
    conversationLayer *ConversationLayer // L4
    
    // 动态注入器
    knowledgeInjector *KnowledgeInjector
    toolInjector      *ToolInjector
    contextInjector   *ContextInjector
    
    // 安全层
    securityLayer *SecurityLayer
}

// PromptConfig prompt 配置
type PromptConfig struct {
    // 模型配置
    ModelAlias     string
    MaxTokens      int
    Temperature    float64
    
    // Pipeline 上下文
    PipelineID     string
    ProjectID      string
    CurrentStage   string
    ComplexityLevel string
    PermissionMode string
    
    // 缓存配置
    CacheEnabled   bool
    CacheTTL       time.Duration
    
    // 安全配置
    SanitizationEnabled bool
    InjectionDefense    bool
}

// Build 构建完整 prompt
func (pb *PromptBuilder) Build(ctx context.Context, req *BuildRequest) (*Prompt, error) {
    // 1. 构建 L1 静态层
    staticContent, err := pb.staticLayer.Build(ctx)
    if err != nil {
        return nil, err
    }
    
    // 2. 构建 L2 项目层
    projectContent, err := pb.projectLayer.Build(ctx, req.ProjectID)
    if err != nil {
        return nil, err
    }
    
    // 3. 构建 L3 阶段层
    stageContent, err := pb.stageLayer.Build(ctx, req.CurrentStage, req.ComplexityLevel)
    if err != nil {
        return nil, err
    }
    
    // 4. 构建 L4 对话层
    conversationContent, err := pb.conversationLayer.Build(ctx, req.PipelineID)
    if err != nil {
        return nil, err
    }
    
    // 5. 动态注入
    knowledgeContent, err := pb.knowledgeInjector.Inject(ctx, req)
    if err != nil {
        return nil, err
    }
    
    toolContent, err := pb.toolInjector.Inject(ctx, req)
    if err != nil {
        return nil, err
    }
    
    // 6. 组装模板
    template := pb.selectTemplate(req.CurrentStage, req.ComplexityLevel)
    
    // 7. 填充模板
    prompt := pb.fillTemplate(template, &TemplateData{
        Static:      staticContent,
        Project:     projectContent,
        Stage:       stageContent,
        Conversation: conversationContent,
        Knowledge:   knowledgeContent,
        Tools:       toolContent,
        Config:      req,
    })
    
    // 8. 安全处理
    if pb.config.SanitizationEnabled {
        prompt = pb.securityLayer.Sanitize(prompt)
    }
    
    return prompt, nil
}
```

### 4.2 阶段模板系统

```go
// StageTemplate 阶段模板
type StageTemplate struct {
    Stage       string
    Complexity  string
    Template    string
    Sections    []TemplateSection
}

// TemplateSection 模板段落
type TemplateSection struct {
    ID          string
    Title       string
    Content     string
    Required    bool
    Conditions  []Condition
}

// 条件判断
type Condition struct {
    Field    string
    Operator string
    Value    interface{}
}

// 模板注册表
var stageTemplates = map[string]map[string]*StageTemplate{
    "clarify": {
        "L1": &StageTemplate{
            Stage:      "clarify",
            Complexity: "L1",
            Template:   clarifyL1Template,
            Sections: []TemplateSection{
                {ID: "role", Title: "角色定义", Required: true},
                {ID: "stage_objective", Title: "阶段目标", Required: true},
                {ID: "analysis_tasks", Title: "分析任务", Required: true},
                {ID: "output_format", Title: "输出格式", Required: true},
                {ID: "constraints", Title: "约束条件", Required: false},
            },
        },
        "L2": &StageTemplate{...},
        "L3": &StageTemplate{...},
        "L4": &StageTemplate{...},
    },
    "decompose": {...},
    "implement": {...},
    "test": {...},
    "deploy": {...},
    "verify": {...},
}
```

### 4.3 动态注入器

```go
// KnowledgeInjector 知识注入器
type KnowledgeInjector struct {
    learningEngine LearningEngine
    embeddingIndex EmbeddingIndex
}

// Inject 注入相关知识
func (ki *KnowledgeInjector) Inject(ctx context.Context, req *BuildRequest) (string, error) {
    // 1. 查询相关轨迹
    trajectories, err := ki.learningEngine.MatchTrajectory(ctx, &MatchRequest{
        ProjectID: req.ProjectID,
        Query:     req.UserMessage,
        TopK:      3,
        Level:     "L3",
    })
    if err != nil {
        return "", err
    }
    
    // 2. 查询相关偏好
    preferences, err := ki.learningEngine.QueryKnowledge(ctx, &QueryRequest{
        ProjectID: req.ProjectID,
        Query:     req.UserMessage,
        TopK:      5,
        Level:     "all",
    })
    if err != nil {
        return "", err
    }
    
    // 3. 构建知识内容
    content := ki.buildKnowledgeContent(trajectories, preferences)
    
    return content, nil
}

// ToolInjector 工具注入器
type ToolInjector struct {
    toolRegistry ToolRegistry
}

// Inject 注入相关工具
func (ti *ToolInjector) Inject(ctx context.Context, req *BuildRequest) (string, error) {
    // 1. 根据阶段筛选工具
    stageTools := ti.filterByStage(req.CurrentStage)
    
    // 2. 根据权限模式筛选工具
    permissionTools := ti.filterByPermission(req.PermissionMode)
    
    // 3. 取交集
    availableTools := ti.intersect(stageTools, permissionTools)
    
    // 4. 构建工具描述
    content := ti.buildToolDescriptions(availableTools)
    
    return content, nil
}
```

## 5. 模板设计

### 5.1 基础模板结构

```xml
<system_prompt>
  <identity>
    <role>You are OpenForge, an AI-driven full-stack development agent.</role>
    <architecture>Three-layer: Collaboration Workbench (C), Pipeline Engine (A), Agent Swarm Runtime (B)</architecture>
    <mission>Execute software engineering tasks across the complete lifecycle.</mission>
  </identity>
  
  <security>
    <audit>All operations are audited (WORM).</audit>
    <gate>Never bypass the Gate approval system.</gate>
    <attribution>Code changes attributed to approving human via `Author="<user> via OpenForge"`.</attribution>
    <injection_defense>Sandwich Architecture: System Zone, Data Zone, Output Zone.</injection_defense>
  </security>
  
  <current_context>
    <pipeline_id>{{.PipelineID}}</pipeline_id>
    <project_id>{{.ProjectID}}</project_id>
    <current_stage>{{.CurrentStage}}</current_stage>
    <complexity_level>{{.ComplexityLevel}}</complexity_level>
    <permission_mode>{{.PermissionMode}}</permission_mode>
    <user_role>{{.UserRole}}</user_role>
  </current_context>
  
  <stage_instructions>
    {{.StageInstructions}}
  </stage_instructions>
  
  <tools>
    {{.AvailableTools}}
  </tools>
  
  <knowledge>
    {{.RelevantKnowledge}}
  </knowledge>
  
  <conversation_history>
    {{.ConversationHistory}}
  </conversation_history>
  
  <constraints>
    {{.Constraints}}
  </constraints>
  
  <output_format>
    {{.OutputFormat}}
  </output_format>
</system_prompt>
```

### 5.2 Clarify 阶段模板 (L3)

```xml
<stage_instructions stage="clarify" level="L3">
  <objective>
    分析需求，理解上下文，提出澄清问题，估算复杂度。
  </objective>
  
  <tasks>
    <task order="1">分析项目结构和现有代码</task>
    <task order="2">理解需求上下文和约束</task>
    <task order="3">识别潜在问题和风险</task>
    <task order="4">提出澄清问题</task>
    <task order="5">估算复杂度级别 (L1-L4)</task>
  </tasks>
  
  <analysis_tools>
    <tool name="read_file" purpose="读取相关文件理解上下文" />
    <tool name="search_content" purpose="搜索相关代码和配置" />
    <tool name="analyze_topology" purpose="分析项目拓扑结构" />
    <tool name="lsp_symbols" purpose="获取文件符号信息" />
  </analysis_tools>
  
  <output_requirements>
    <requirement name="requirement_summary">需求摘要 (200-300 tokens)</requirement>
    <requirement name="constraints">约束条件列表</requirement>
    <requirement name="questions">澄清问题列表 (最多 5 个)</requirement>
    <requirement name="complexity_estimate">复杂度估算 JSON</requirement>
    <requirement name="affected_modules">影响模块列表</requirement>
  </output_requirements>
  
  <complexity_estimate_format>
    {
      "level": "L1|L2|L3|L4",
      "reasoning": "估算理由",
      "estimated_files": 5,
      "estimated_modules": ["module1", "module2"],
      "estimated_tokens": 10000,
      "estimated_duration": "2h"
    }
  </complexity_estimate_format>
  
  <constraints>
    <constraint>只读操作，不修改任何文件</constraint>
    <constraint>必须提出至少一个澄清问题</constraint>
    <constraint>复杂度估算必须基于实际分析</constraint>
    <constraint>输出必须符合指定格式</constraint>
  </constraints>
</stage_instructions>
```

### 5.3 Implement 阶段模板 (L3)

```xml
<stage_instructions stage="implement" level="L3">
  <objective>
    根据需求生成或修改代码，应用补丁，确保代码质量。
  </objective>
  
  <tasks>
    <task order="1">分析影响范围和依赖关系</task>
    <task order="2">获取文件锁 (acquire_file_lock)</task>
    <task order="3">生成或修改代码</task>
    <task order="4">运行测试验证</task>
    <task order="5">提交 Gate 审批</task>
  </tasks>
  
  <implementation_tools>
    <tool name="acquire_file_lock" purpose="获取文件锁" required="true" />
    <tool name="read_file" purpose="读取现有代码" />
    <tool name="edit_file" purpose="修改现有文件" />
    <tool name="write_file" purpose="创建新文件" />
    <tool name="bash" purpose="运行构建和测试命令" />
    <tool name="lsp_hover" purpose="获取符号信息" />
    <tool name="lsp_definition" purpose="跳转到定义" />
  </implementation_tools>
  
  <code_conventions>
    <convention>NO COMMENTS unless asked</convention>
    <convention>Follow existing code style</convention>
    <convention>Check package.json / go.mod before using libraries</convention>
    <convention>Prefer editing existing files over creating new ones</convention>
    <convention>No backwards-compatibility hacks</convention>
  </code_conventions>
  
  <security_rules>
    <rule>Never expose or log secrets/keys</rule>
    <rule>Never commit secrets</rule>
    <rule>Follow security best practices</rule>
    <rule>Validate at system boundaries only</rule>
  </security_rules>
  
  <gate_approval>
    <trigger>After code changes are complete</trigger>
    <required_artifacts>
      <artifact>Changed files list</artifact>
      <artifact>Diff preview</artifact>
      <artifact>Test results</artifact>
      <artifact>Summary of changes</artifact>
    </required_artifacts>
  </gate_approval>
  
  <constraints>
    <constraint>Must acquire file lock before modifying any file</constraint>
    <constraint>Code changes must pass all tests</constraint>
    <constraint>Must request Gate approval after changes</constraint>
    <constraint>Follow code conventions strictly</constraint>
  </constraints>
</stage_instructions>
```

## 6. 实现细节

### 6.1 缓存实现

```go
// CacheLayer 缓存层接口
type CacheLayer interface {
    Get(ctx context.Context, key string) (string, error)
    Set(ctx context.Context, key string, value string, ttl time.Duration) error
    Invalidate(ctx context.Context, key string) error
}

// StaticLayerCache L1 静态层缓存
type StaticLayerCache struct {
    cache CacheLayer
    content string
    lastUpdated time.Time
}

// Get 获取静态层内容
func (slc *StaticLayerCache) Get(ctx context.Context) (string, error) {
    // 静态层内容极少变化，可以长期缓存
    if slc.content != "" && time.Since(slc.lastUpdated) < 24*time.Hour {
        return slc.content, nil
    }
    
    // 重新构建
    content := slc.buildStaticContent()
    slc.content = content
    slc.lastUpdated = time.Now()
    
    // 缓存到 Redis
    slc.cache.Set(ctx, "L1:static", content, 24*time.Hour)
    
    return content, nil
}

// ProjectLayerCache L2 项目层缓存
type ProjectLayerCache struct {
    cache CacheLayer
    learningEngine LearningEngine
}

// Get 获取项目层内容
func (plc *ProjectLayerCache) Get(ctx context.Context, projectID string) (string, error) {
    key := fmt.Sprintf("L2:project:%s", projectID)
    
    // 尝试从缓存获取
    cached, err := plc.cache.Get(ctx, key)
    if err == nil && cached != "" {
        return cached, nil
    }
    
    // 重新构建
    content, err := plc.buildProjectContent(ctx, projectID)
    if err != nil {
        return "", err
    }
    
    // 缓存，每 10 Pipeline 刷新
    plc.cache.Set(ctx, key, content, 10*time.Minute)
    
    return content, nil
}
```

### 6.2 阶段感知实现

```go
// StageAwarePromptBuilder 阶段感知的 Prompt 构建器
type StageAwarePromptBuilder struct {
    templates map[string]map[string]*StageTemplate
    builder   *PromptBuilder
}

// Build 构建阶段特定的 prompt
func (sapb *StageAwarePromptBuilder) Build(ctx context.Context, req *BuildRequest) (*Prompt, error) {
    // 1. 选择阶段模板
    template := sapb.selectTemplate(req.CurrentStage, req.ComplexityLevel)
    
    // 2. 获取阶段特定配置
    stageConfig := sapb.getStageConfig(req.CurrentStage)
    
    // 3. 构建阶段特定内容
    stageContent := sapb.buildStageContent(ctx, req, stageConfig)
    
    // 4. 注入到构建器
    req.StageTemplate = template
    req.StageContent = stageContent
    
    // 5. 调用基础构建器
    return sapb.builder.Build(ctx, req)
}

// selectTemplate 选择阶段模板
func (sapb *StageAwarePromptBuilder) selectTemplate(stage, level string) *StageTemplate {
    if stageTemplates, ok := sapb.templates[stage]; ok {
        if template, ok := stageTemplates[level]; ok {
            return template
        }
        // 默认使用 L3 模板
        return stageTemplates["L3"]
    }
    return nil
}
```

### 6.3 安全层实现

```go
// SecurityLayer 安全层
type SecurityLayer struct {
    sanitizer *Sanitizer
    validator *Validator
}

// Sanitize 清理 prompt
func (sl *SecurityLayer) Sanitize(prompt *Prompt) *Prompt {
    // 1. 清理 System Zone
    prompt.System = sl.sanitizer.CleanSystemZone(prompt.System)
    
    // 2. 隔离 Data Zone
    prompt.Data = sl.sanitizer.IsolateDataZone(prompt.Data)
    
    // 3. 约束 Output Zone
    prompt.Output = sl.sanitizer.ConstrainOutputZone(prompt.Output)
    
    // 4. 移除潜在注入
    prompt = sl.removeInjectionAttempts(prompt)
    
    return prompt
}

// removeInjectionAttempts 移除注入尝试
func (sl *SecurityLayer) removeInjectionAttempts(prompt *Prompt) *Prompt {
    // 移除 "SYSTEM:" 标记
    prompt = sl.removePattern(prompt, `(?i)system:`)
    
    // 移除 "指令" 标记
    prompt = sl.removePattern(prompt, `(?i)指令`)
    
    // 移除角色扮演尝试
    prompt = sl.removePattern(prompt, `(?i)you are now`)
    
    return prompt
}
```

## 7. 集成方案

### 7.1 与现有系统集成

```go
// OpenForgeAgent OpenForge Agent 集成
type OpenForgeAgent struct {
    promptBuilder *PromptBuilder
    llmRouter     LLMRouter
    pipeline      *Pipeline
}

// ExecuteStage 执行 Pipeline 阶段
func (ofa *OpenForgeAgent) ExecuteStage(ctx context.Context, stage string) error {
    // 1. 构建 prompt
    req := &BuildRequest{
        PipelineID:     ofa.pipeline.ID,
        ProjectID:      ofa.pipeline.ProjectID,
        CurrentStage:   stage,
        ComplexityLevel: ofa.pipeline.Level,
        PermissionMode: ofa.getPermissionMode(stage),
        UserRole:       ofa.getCurrentUserRole(),
        UserMessage:    ofa.getUserMessage(),
    }
    
    prompt, err := ofa.promptBuilder.Build(ctx, req)
    if err != nil {
        return err
    }
    
    // 2. 调用 LLM
    response, err := ofa.llmRouter.SendMessage(ctx, &LLMRequest{
        Model:        ofa.pipeline.ModelAlias,
        SystemPrompt: prompt.System,
        Messages:     prompt.Messages,
        Tools:        prompt.Tools,
        MaxTokens:    prompt.MaxTokens,
    })
    if err != nil {
        return err
    }
    
    // 3. 处理响应
    return ofa.handleResponse(ctx, response, stage)
}
```

### 7.2 与 Learning Engine 集成

```go
// LearningEngineIntegration 学习引擎集成
type LearningEngineIntegration struct {
    learningEngine LearningEngine
    promptBuilder  *PromptBuilder
}

// EnhancePromptWithKnowledge 用知识增强 prompt
func (lei *LearningEngineIntegration) EnhancePromptWithKnowledge(ctx context.Context, req *BuildRequest) error {
    // 1. 查询相关知识
    knowledge, err := lei.learningEngine.QueryKnowledge(ctx, &QueryRequest{
        ProjectID: req.ProjectID,
        Query:     req.UserMessage,
        TopK:      5,
        Level:     "all",
    })
    if err != nil {
        return err
    }
    
    // 2. 查询相关轨迹
    trajectories, err := lei.learningEngine.MatchTrajectory(ctx, &MatchRequest{
        ProjectID: req.ProjectID,
        Query:     req.UserMessage,
        TopK:      3,
        Level:     "L3",
    })
    if err != nil {
        return err
    }
    
    // 3. 注入到 prompt
    req.Knowledge = lei.formatKnowledge(knowledge)
    req.Trajectories = lei.formatTrajectories(trajectories)
    
    return nil
}
```

## 8. 性能优化

### 8.1 Token 优化策略

1. **分层缓存**：L1/L2 层内容缓存，减少重复构建
2. **摘要压缩**：对话历史压缩为摘要
3. **选择性注入**：只注入相关知识和工具
4. **动态裁剪**：根据 token 预算动态裁剪内容

### 8.2 缓存命中率优化

1. **L1 静态层**：24 小时缓存，命中率 > 95%
2. **L2 项目层**：10 Pipeline 刷新，命中率 > 80%
3. **L3 阶段层**：每阶段刷新，命中率 > 60%
4. **L4 对话层**：每轮动态，命中率 > 40%

### 8.3 并发优化

1. **并行构建**：L1-L4 层并行构建
2. **异步注入**：知识和工具异步注入
3. **预加载**：预加载下阶段模板

## 9. 监控和调试

### 9.1 监控指标

```go
// PromptMetrics prompt 监控指标
type PromptMetrics struct {
    // 构建指标
    BuildDuration    time.Duration
    BuildSuccess     bool
    BuildError       string
    
    // 缓存指标
    L1CacheHit       bool
    L2CacheHit       bool
    L3CacheHit       bool
    L4CacheHit       bool
    
    // Token 指标
    TotalTokens      int
    StaticTokens     int
    ProjectTokens    int
    StageTokens      int
    ConversationTokens int
    KnowledgeTokens  int
    ToolTokens       int
    
    // 阶段指标
    Stage            string
    ComplexityLevel  string
    PermissionMode   string
}
```

### 9.2 调试工具

```go
// PromptDebugger prompt 调试器
type PromptDebugger struct {
    enabled bool
    output  io.Writer
}

// Debug 输出调试信息
func (pd *PromptDebugger) Debug(prompt *Prompt, metrics *PromptMetrics) {
    if !pd.enabled {
        return
    }
    
    fmt.Fprintf(pd.output, "=== Prompt Debug Info ===\n")
    fmt.Fprintf(pd.output, "Stage: %s\n", metrics.Stage)
    fmt.Fprintf(pd.output, "Complexity: %s\n", metrics.ComplexityLevel)
    fmt.Fprintf(pd.output, "Permission: %s\n", metrics.PermissionMode)
    fmt.Fprintf(pd.output, "Total Tokens: %d\n", metrics.TotalTokens)
    fmt.Fprintf(pd.output, "Build Duration: %v\n", metrics.BuildDuration)
    fmt.Fprintf(pd.output, "\n=== Prompt Content ===\n")
    fmt.Fprintf(pd.output, "System:\n%s\n", prompt.System)
    fmt.Fprintf(pd.output, "Messages: %d\n", len(prompt.Messages))
    fmt.Fprintf(pd.output, "Tools: %d\n", len(prompt.Tools))
}
```

## 10. 测试策略

### 10.1 单元测试

```go
func TestPromptBuilder_Build(t *testing.T) {
    builder := NewPromptBuilder(config)
    
    req := &BuildRequest{
        PipelineID:     "test-pipeline",
        ProjectID:      "test-project",
        CurrentStage:   "clarify",
        ComplexityLevel: "L3",
        PermissionMode: "plan",
        UserRole:       "dev",
        UserMessage:    "Add user authentication",
    }
    
    prompt, err := builder.Build(context.Background(), req)
    
    assert.NoError(t, err)
    assert.NotEmpty(t, prompt.System)
    assert.Contains(t, prompt.System, "clarify")
    assert.Contains(t, prompt.System, "L3")
}
```

### 10.2 集成测试

```go
func TestOpenForgeAgent_ExecuteStage(t *testing.T) {
    agent := NewOpenForgeAgent(config)
    
    // 创建测试 Pipeline
    pipeline := &Pipeline{
        ID:        "test-pipeline",
        ProjectID: "test-project",
        Level:     "L3",
        CurrentStage: "clarify",
    }
    
    err := agent.ExecuteStage(context.Background(), "clarify")
    
    assert.NoError(t, err)
    // 验证 Gate 审批请求
    assert.True(t, agent.HasPendingGateApproval())
}
```

## 11. 部署和配置

### 11.1 配置文件

```yaml
# prompt_config.yaml
prompt:
  # 缓存配置
  cache:
    enabled: true
    l1_ttl: 24h
    l2_ttl: 10m
    l3_ttl: 5m
    l4_ttl: 1m
  
  # Token 预算
  token_budget:
    total: 8500
    static: 2000
    project: 1500
    stage: 2000
    conversation: 3000
  
  # 安全配置
  security:
    sanitization_enabled: true
    injection_defense: true
    audit_logging: true
  
  # 模板配置
  templates:
    directory: "./templates"
    reload_interval: 5m
  
  # 监控配置
  metrics:
    enabled: true
    export_interval: 1m
```

### 11.2 部署步骤

1. **准备模板文件**：将阶段模板文件放到配置目录
2. **配置缓存**：配置 Redis 缓存连接
3. **配置学习引擎**：配置 Learning Engine 连接
4. **启动服务**：启动 PromptBuilder 服务
5. **验证集成**：验证与现有系统的集成

## 12. 总结

本设计文档为 OpenForge 项目设计了一个**分层模板系统**的 Prompt 拼接机制，具有以下特点：

1. **Pipeline 阶段感知**：根据当前阶段动态调整 prompt 内容
2. **复杂度感知**：根据 L1-L4 复杂度调整 prompt 详细程度
3. **权限模式感知**：根据权限模式调整工具和操作说明
4. **分层缓存**：实现 L1-L4 四层缓存，优化 token 使用
5. **动态注入**：支持自学习知识、工具描述、上下文等动态内容注入
6. **安全隔离**：遵循 Sandwich Architecture，防止 prompt 注入

该设计充分考虑了 OpenForge 的企业级特性，与现有架构高度一致，可维护性和可扩展性强，是一个适合 OpenForge 项目的 Prompt 拼接机制。

## 13. 实现代码

### 13.1 核心实现文件

本设计已实现以下核心文件：

1. **`internal/agent/domain/prompt_builder.go`** - PromptBuilder 核心类
   - 实现分层模板系统架构
   - 包含 L1-L4 缓存层实现
   - 实现动态注入器（知识、工具、上下文）
   - 实现安全层（Sandwich Architecture）

2. **`internal/agent/domain/stage_templates.go`** - 阶段模板定义
   - 包含所有 6 个阶段的模板（Clarify, Decompose, Implement, Test, Deploy, Verify）
   - 每个阶段支持 L1-L4 复杂度级别
   - 使用 XML 结构化模板格式

3. **`internal/agent/domain/knowledge_injector.go`** - 知识注入器
   - 集成 Learning Engine 进行知识查询
   - 集成 Embedding Index 进行语义搜索
   - 实现缓存机制优化性能

### 13.2 使用示例

```go
// 创建 PromptBuilder
config := &PromptConfig{
    CacheEnabled:        true,
    SanitizationEnabled: true,
    InjectionDefense:   true,
    TokenBudget:        DefaultTokenBudget(),
}

builder := NewPromptBuilder(config)

// 设置依赖（在实际应用中由依赖注入完成）
builder.knowledgeInjector.SetLearningEngine(learningEngine)
builder.knowledgeInjector.SetEmbeddingIndex(embeddingIndex)
builder.toolInjector.SetToolRegistry(toolRegistry)

// 构建 prompt
req := &BuildRequest{
    PipelineID:     "pipeline-123",
    ProjectID:      "project-456",
    CurrentStage:   "implement",
    ComplexityLevel: "L3",
    PermissionMode: "default",
    UserRole:       "dev",
    UserMessage:    "Add user authentication with JWT tokens",
}

prompt, err := builder.Build(context.Background(), req)
if err != nil {
    log.Fatal(err)
}

// 使用 prompt 调用 LLM
response, err := llmRouter.SendMessage(ctx, &LLMRequest{
    Model:        "sonnet",
    SystemPrompt: prompt.System,
    Messages:     prompt.Messages,
    Tools:        prompt.Tools,
    MaxTokens:    4096,
})
```

### 13.3 与 OpenForge Pipeline 集成

```go
// OpenForgeAgent 使用 PromptBuilder
type OpenForgeAgent struct {
    promptBuilder *PromptBuilder
    llmRouter     LLMRouter
    pipeline      *Pipeline
}

func (ofa *OpenForgeAgent) ExecuteStage(ctx context.Context, stage string) error {
    // 1. 构建 prompt
    req := &BuildRequest{
        PipelineID:     ofa.pipeline.ID,
        ProjectID:      ofa.pipeline.ProjectID,
        CurrentStage:   stage,
        ComplexityLevel: ofa.pipeline.Level,
        PermissionMode: ofa.getPermissionMode(stage),
        UserRole:       ofa.getCurrentUserRole(),
        UserMessage:    ofa.getUserMessage(),
    }
    
    prompt, err := ofa.promptBuilder.Build(ctx, req)
    if err != nil {
        return err
    }
    
    // 2. 调用 LLM
    response, err := ofa.llmRouter.SendMessage(ctx, &LLMRequest{
        Model:        ofa.pipeline.ModelAlias,
        SystemPrompt: prompt.System,
        Messages:     prompt.Messages,
        Tools:        prompt.Tools,
        MaxTokens:    prompt.TokenUsage.TotalTokens,
    })
    if err != nil {
        return err
    }
    
    // 3. 处理响应
    return ofa.handleResponse(ctx, response, stage)
}
```

## 14. 与其他项目的对比

### 14.1 与 DeerFlow 对比

| 维度 | OpenForge | DeerFlow |
|------|-----------|----------|
| **模板结构** | XML 标签 + Pipeline 阶段感知 | XML 标签 + 角色定义 |
| **缓存机制** | L1-L4 四层缓存 | 无明确缓存机制 |
| **动态注入** | 知识 + 工具 + 上下文 | 技能 + 子代理 + 记忆 |
| **阶段感知** | 强（6 阶段 × 4 复杂度） | 弱（通用模板） |
| **安全机制** | Sandwich Architecture | 基础安全 |

### 14.2 与 Claude Code 对比

| 维度 | OpenForge | Claude Code |
|------|-----------|-------------|
| **架构** | 分层模板系统 | 多层扩展系统 |
| **扩展机制** | 模板 + 注入器 | Tool/Command/Skills |
| **缓存策略** | L1-L4 四层缓存 | 无明确缓存 |
| **阶段感知** | 强 | 弱（通用） |
| **企业特性** | 完整（Gate/审计/权限） | 有限 |

### 14.3 与 Plandex 对比

| 维度 | OpenForge | Plandex |
|------|-----------|---------|
| **机制类型** | 结构化模板系统 | CLI 直接输入 |
| **复杂度** | 高 | 低 |
| **可扩展性** | 高 | 低 |
| **阶段感知** | 强 | 弱 |
| **适用场景** | 企业级复杂项目 | 简单项目 |

## 15. 最佳实践

### 15.1 模板设计最佳实践

1. **保持模板简洁**：每个模板专注于一个阶段和复杂度级别
2. **使用 XML 结构**：便于解析和维护
3. **明确输出格式**：确保 LLM 输出符合预期
4. **添加约束条件**：防止 LLM 偏离任务

### 15.2 缓存策略最佳实践

1. **分层缓存**：根据内容更新频率设置不同 TTL
2. **预热缓存**：在系统启动时预热 L1/L2 缓存
3. **监控命中率**：根据命中率调整缓存策略
4. **处理缓存失效**：优雅处理缓存失效场景

### 15.3 安全最佳实践

1. **启用 Sandwich Architecture**：防止 prompt 注入
2. **输入清理**：清理用户输入中的潜在注入
3. **输出验证**：验证 LLM 输出符合预期格式
4. **审计日志**：记录所有 prompt 构建和 LLM 调用

## 16. 未来扩展

### 16.1 计划功能

1. **模板版本管理**：支持模板版本控制和 A/B 测试
2. **动态模板生成**：根据项目特点动态生成模板
3. **多语言支持**：支持中英文等多语言模板
4. **模板市场**：社区贡献的模板市场

### 16.2 性能优化

1. **并行构建**：L1-L4 层并行构建
2. **预编译模板**：预编译 XML 模板为 Go 模板
3. **增量更新**：支持模板增量更新
4. **分布式缓存**：支持 Redis 分布式缓存

### 16.3 集成扩展

1. **MCP 集成**：支持 Model Context Protocol
2. **插件系统**：支持自定义注入器插件
3. **API 网关**：支持 API 网关集成
4. **监控集成**：集成 Prometheus/Grafana 监控

---

**文档版本**: v1.1  
**最后更新**: 2026-05-23  
**作者**: OpenForge Design Team  
**实现状态**: 核心功能已实现，可在此基础上扩展和完善