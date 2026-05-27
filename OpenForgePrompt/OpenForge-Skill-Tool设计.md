# OpenForge Skill/Tool 实现模式设计

## 1. 设计目标

为 OpenForge 设计一套适合企业级 AI 开发平台的 Skill/Tool 实现模式。

| 需求 | 说明 |
|------|------|
| Pipeline 阶段感知 | 根据 clarify/implement/test 等阶段动态调整 |
| 复杂度感知 | 支持 L1-L4 复杂度级别 |
| 权限模式 | plan/auto/default/bypass 四种模式 |
| 企业级特性 | Gate 审批、审计、安全隔离 |
| 动态扩展 | 支持自定义技能和工具 |

---

## 2. 架构设计

### 2.1 整体架构

```
┌─────────────────────────────────────────────────────────────┐
│                    PromptBuilder                             │
├─────────────────────────────────────────────────────────────┤
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐          │
│  │  L1 静态层   │  │  L2 项目层   │  │  L4 对话层   │          │
│  └─────────────┘  └─────────────┘  └─────────────┘          │
│                            ↓                                 │
│  ┌─────────────────────────────────────────────────────┐    │
│  │              Skill/Tool 注入层                        │    │
│  │  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐  │    │
│  │  │ SkillLoader │  │ToolRegistry │  │ MCP Gateway │  │    │
│  │  │  (DeerFlow) │  │(ClaudeCode) │  │  (DeerFlow) │  │    │
│  │  └─────────────┘  └─────────────┘  └─────────────┘  │    │
│  └─────────────────────────────────────────────────────┘    │
└─────────────────────────────────────────────────────────────┘
```

### 2.2 核心组件

| 组件 | 职责 | 数据源 |
|------|------|--------|
| SkillLoader | 加载和管理技能 | Markdown 文件 |
| ToolRegistry | 注册和过滤工具 | 硬编码 + MCP |
| MCP Gateway | 外部工具集成 | MCP 服务器 |

---

## 3. Skill 实现模式 (DeerFlow模式)

### 3.1 技能定义格式

```markdown
# 技能名称

## 描述
[技能的简要描述]

## 适用阶段
- clarify
- implement

## 复杂度
- L2
- L3

## 权限
- default
- auto

## 工作流程
1. [步骤1]
2. [步骤2]
3. [步骤3]

## 最佳实践
- [实践1]
- [实践2]
```

### 3.2 技能目录结构

```
config/skills/
├── public/                    # 内置技能（服务器端）
│   ├── clarify_analysis.md
│   ├── implement_coding.md
│   └── test_unit.md
└── custom/                    # 自定义技能（项目目录）
    └── my_custom_skill.md
```

### 3.3 技能加载器实现

```go
// SkillLoader 技能加载器
type SkillLoader struct {
    publicSkills  map[string]*Skill
    customSkills  map[string]*Skill
    mu            sync.RWMutex
}

// Skill 技能定义
type Skill struct {
    Name        string   `yaml:"name"`
    Description string   `yaml:"description"`
    Stages      []string `yaml:"stages"`      // 适用阶段
    Complexity  []string `yaml:"complexity"`   // L1-L4
    Permission  []string `yaml:"permission"`   // plan/auto/default
    Content     string   `yaml:"content"`      // Markdown内容
    Source      string   `yaml:"source"`       // public/custom
}

// LoadForContext 根据上下文加载技能
func (sl *SkillLoader) LoadForContext(ctx context.Context, stage, complexity, permission string) []*Skill {
    sl.mu.RLock()
    defer sl.mu.RUnlock()
    
    skillMap := make(map[string]*Skill)
    
    // 加载内置技能
    for name, skill := range sl.publicSkills {
        if sl.matchesContext(skill, stage, complexity, permission) {
            skillMap[name] = skill
        }
    }
    
    // 加载自定义技能（覆盖同名内置技能）
    for name, skill := range sl.customSkills {
        if sl.matchesContext(skill, stage, complexity, permission) {
            skillMap[name] = skill
        }
    }
    
    var skills []*Skill
    for _, skill := range skillMap {
        skills = append(skills, skill)
    }
    
    return skills
}
```

---

## 4. Tool 实现模式 (Claude Code模式)

### 4.1 工具定义格式

```go
// ToolDefinition 工具定义
type ToolDefinition struct {
    Name        string               `json:"name"`
    Description string               `json:"description"`
    Parameters  map[string]Parameter `json:"parameters"`
    ReadOnly    bool                 `json:"readOnly"`
    Stages      []string             `json:"stages"`      // 可用阶段
    Complexity  []string             `json:"complexity"`  // L1-L4
    Permission  []string             `json:"permission"`  // plan/auto/default
    Required    bool                 `json:"required"`
}

// Parameter 参数定义
type Parameter struct {
    Type        string      `json:"type"`
    Description string      `json:"description"`
    Required    bool        `json:"required"`
    Default     interface{} `json:"default,omitempty"`
}
```

### 4.2 工具注册表实现

```go
// ToolRegistry 工具注册表
type ToolRegistry struct {
    tools    map[string]ToolDefinition
    stageMap map[string][]string
    mu       sync.RWMutex
}

// GetToolsForContext 根据上下文获取工具
func (tr *ToolRegistry) GetToolsForContext(ctx context.Context, stage, complexity, permission string) []ToolDefinition {
    tr.mu.RLock()
    defer tr.mu.RUnlock()
    
    var tools []ToolDefinition
    
    for _, tool := range tr.tools {
        if tr.matchesContext(tool, stage, complexity, permission) {
            tools = append(tools, tool)
        }
    }
    
    return tools
}

// matchesContext 检查工具是否匹配上下文
func (tr *ToolRegistry) matchesContext(tool ToolDefinition, stage, complexity, permission string) bool {
    if !contains(tool.Stages, stage) {
        return false
    }
    if !contains(tool.Complexity, complexity) {
        return false
    }
    if !contains(tool.Permission, permission) {
        return false
    }
    // plan模式下只允许只读工具
    if permission == "plan" && !tool.ReadOnly {
        return false
    }
    return true
}
```

### 4.3 内置工具示例

```go
// Clarify 阶段工具
ToolDefinition{
    Name:        "read_file",
    Description: "Read contents of a file",
    ReadOnly:    true,
    Stages:      []string{"clarify", "decompose", "implement", "test", "verify"},
    Complexity:  []string{"L1", "L2", "L3", "L4"},
    Permission:  []string{"plan", "auto", "default"},
}

// Implement 阶段工具
ToolDefinition{
    Name:        "edit_file",
    Description: "Edit existing file",
    ReadOnly:    false,
    Stages:      []string{"implement", "test", "verify"},
    Complexity:  []string{"L1", "L2", "L3", "L4"},
    Permission:  []string{"auto", "default"},
}
```

---

## 5. MCP 集成 (DeerFlow模式)

### 5.1 MCP 配置格式

```yaml
# config/mcp_servers.yaml
mcp_servers:
  github:
    url: "https://api.github.com/mcp"
    oauth:
      client_id: $GITHUB_CLIENT_ID
      client_secret: $GITHUB_CLIENT_SECRET
    
  jira:
    url: "https://company.atlassian.net/mcp"
    api_token: $JIRA_API_TOKEN
```

### 5.2 MCP Gateway 实现

```go
// MCPGateway MCP网关
type MCPGateway struct {
    servers   map[string]*MCPServer
    toolCache map[string]ToolDefinition
    mu        sync.RWMutex
}

// GetTools 获取MCP工具
func (gw *MCPGateway) GetTools(ctx context.Context) []ToolDefinition {
    gw.mu.RLock()
    defer gw.mu.RUnlock()
    
    var tools []ToolDefinition
    for _, tool := range gw.toolCache {
        tools = append(tools, tool)
    }
    
    return tools
}
```

---

## 6. 与 PromptBuilder 集成

### 6.1 集成架构

```go
// PromptBuilder 集成 Skill/Tool
type PromptBuilder struct {
    // ... 其他字段 ...
    
    skillLoader   *SkillLoader
    toolRegistry  *ToolRegistry
    mcpGateway    *MCPGateway
}

// Build 构建Prompt
func (pb *PromptBuilder) Build(ctx context.Context, req *BuildRequest) (*Prompt, error) {
    // 1. 加载技能
    skills := pb.skillLoader.LoadForContext(ctx, req.CurrentStage, req.ComplexityLevel, req.PermissionMode)
    
    // 2. 获取工具
    tools := pb.toolRegistry.GetToolsForContext(ctx, req.CurrentStage, req.ComplexityLevel, req.PermissionMode)
    
    // 3. 获取MCP工具
    mcpTools := pb.mcpGateway.GetTools(ctx)
    
    // 4. 合并工具
    allTools := append(tools, mcpTools...)
    
    // 5. 构建技能内容
    skillContent := pb.buildSkillContent(skills)
    
    // 6. 构建工具描述
    toolContent := pb.buildToolDescriptions(allTools)
    
    // 7. 组装Prompt
    prompt := pb.assemble(skillContent, toolContent, req)
    
    return prompt, nil
}
```

---

## 7. 配置文件结构

### 7.1 目录结构

```
openforge/
├── config/
│   ├── prompts/
│   │   ├── static.xml              # L1静态内容
│   │   └── stages/                 # 阶段模板
│   ├── skills/
│   │   ├── public/                 # 内置技能
│   │   └── custom/                 # 自定义技能
│   └── mcp_servers.yaml            # MCP服务器配置
├── internal/
│   └── agent/
│       └── domain/
│           ├── prompt_builder.go
│           ├── skill_loader.go     # 技能加载器
│           ├── tool_registry.go    # 工具注册表
│           └── mcp_gateway.go      # MCP网关
└── 项目目录/
    ├── of-prefs.yaml               # 项目偏好
    └── .openforge/
        └── skills/                 # 项目自定义技能
```

---

## 8. 总结

### 8.1 设计优势

| 优势 | 说明 |
|------|------|
| **阶段感知** | 技能和工具按Pipeline阶段动态加载 |
| **复杂度适配** | L1-L4复杂度级别自动适配 |
| **权限控制** | plan/auto/default/bypass精细控制 |
| **动态扩展** | 支持自定义技能和MCP工具 |
| **企业级** | 支持Gate审批、审计、安全隔离 |

### 8.2 实施路径

| 阶段 | 内容 |
|------|------|
| Phase 1 | 实现 SkillLoader + ToolRegistry，集成到 PromptBuilder |
| Phase 2 | 实现技能动态加载、阶段感知过滤、权限控制 |
| Phase 3 | 实现 MCP Gateway，支持外部工具集成 |
| Phase 4 | 集成 Gate 审批、审计日志、安全隔离增强 |

---

**文档版本**: v1.0  
**最后更新**: 2026-05-24