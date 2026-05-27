# Prompt 拼接与 Skill/Tool 整合设计

## 1. 当前架构回顾

### 1.1 Prompt 拼接流程（简化后）

```
L1 (静态) → L2 (项目) → ToolInjector → L4 (对话)
```

### 1.2 各层职责

| 层 | 职责 | 数据源 |
|----|------|--------|
| L1 | 身份 + 安全规则 + 代码规范 | static.xml |
| L2 | 项目偏好 + 模块索引 + stageInstruction + 知识查询 | of-prefs.yaml + LearningEngine |
| ToolInjector | 按stage+permission筛选工具 | 硬编码map |
| L4 | 最近5轮对话 + 当前用户消息 | QueryEngine |

---

## 2. 整合方案

### 2.1 新架构

```
L1 (静态) → L2 (项目) → Skill/Tool注入层 → L4 (对话)
                         │
                         ├─ SkillLoader (动态加载技能)
                         ├─ ToolRegistry (阶段感知工具)
                         └─ MCP Gateway (外部工具)
```

### 2.2 整合后的Prompt结构

```xml
<system_prompt>
  <!-- L1: 静态层 -->
  <identity>...</identity>
  <security>...</security>
  <code_conventions>...</code_conventions>
  
  <!-- L2: 项目层 -->
  <project_context>
    <preferences>...</preferences>
    <module_index>...</module_index>
    <stage_instructions>...</stage_instructions>
    <knowledge>...</knowledge>
    <timestamp>...</timestamp>
  </project_context>
  
  <!-- Skill/Tool注入层 -->
  <skills>
    <skill name="clarify_analysis" source="public">
      [技能内容]
    </skill>
    <skill name="custom_skill" source="custom">
      [技能内容]
    </skill>
  </skills>
  
  <available_tools>
    <tool name="read_file" required="true" readonly="true">
      <description>Read contents of a file</description>
      <parameters>
        <param name="filePath" type="string" required="true">Path to the file</param>
      </parameters>
    </tool>
    <tool name="edit_file" required="false" readonly="false">
      <description>Edit existing file</description>
      <parameters>
        <param name="filePath" type="string" required="true">Path to the file</param>
        <param name="old_str" type="string" required="true">String to replace</param>
        <param name="new_str" type="string" required="true">Replacement string</param>
      </parameters>
    </tool>
  </available_tools>
  
  <!-- L4: 对话层 -->
  <conversation_history>
    [最近5轮对话]
  </conversation_history>
  
  <current_message>
    [当前用户消息]
  </current_message>
</system_prompt>
```

---

## 3. 整合后的PromptBuilder

### 3.1 结构定义

```go
// PromptBuilder 整合后的Prompt构建器
type PromptBuilder struct {
    // L1: 静态层
    staticContent string
    
    // L2: 项目层
    projectBuilder *L2Builder
    
    // Skill/Tool注入层
    skillLoader   *SkillLoader
    toolRegistry  *ToolRegistry
    mcpGateway    *MCPGateway
    
    // L4: 对话层
    queryEngine *QueryEngine
    
    // 安全层
    securityLayer *SecurityLayer
}

// BuildRequest 构建请求
type BuildRequest struct {
    // Pipeline上下文
    PipelineID     string
    ProjectID      string
    CurrentStage   string
    ComplexityLevel string
    PermissionMode string
    UserRole       string
    
    // 用户输入
    UserMessage    string
}
```

### 3.2 Build 方法

```go
// Build 构建完整Prompt
func (pb *PromptBuilder) Build(ctx context.Context, req *BuildRequest) (*Prompt, error) {
    // 1. L1: 静态层（启动时加载，不变）
    staticCtx := pb.getStaticContext()
    
    // 2. L2: 项目层（内存缓存 + 热重载）
    projectCtx, err := pb.projectBuilder.Build(ctx, req.ProjectID, req.CurrentStage, req.ComplexityLevel)
    if err != nil {
        return nil, err
    }
    
    // 3. Skill/Tool注入层（动态加载）
    skills := pb.skillLoader.LoadForContext(ctx, req.CurrentStage, req.ComplexityLevel, req.PermissionMode)
    tools := pb.toolRegistry.GetToolsForContext(ctx, req.CurrentStage, req.ComplexityLevel, req.PermissionMode)
    mcpTools := pb.mcpGateway.GetTools(ctx)
    allTools := append(tools, mcpTools...)
    
    // 4. L4: 对话层（每轮动态）
    conversation := pb.queryEngine.GetRecentMessages(ctx, req.PipelineID, 5)
    
    // 5. 组装Prompt
    prompt := pb.assemble(staticCtx, projectCtx, skills, allTools, conversation, req)
    
    // 6. 安全处理
    prompt = pb.securityLayer.Sanitize(prompt)
    
    return prompt, nil
}
```

### 3.3 assemble 方法

```go
// assemble 组装Prompt
func (pb *PromptBuilder) assemble(
    staticCtx string,
    projectCtx string,
    skills []*UnifiedSkill,
    tools []ToolDefinition,
    conversation []Message,
    req *BuildRequest,
) *Prompt {
    var systemPrompt strings.Builder
    
    // L1: 静态层
    systemPrompt.WriteString(staticCtx)
    systemPrompt.WriteString("\n")
    
    // L2: 项目层
    systemPrompt.WriteString(projectCtx)
    systemPrompt.WriteString("\n")
    
    // Skill注入
    if len(skills) > 0 {
        systemPrompt.WriteString("<skills>\n")
        for _, skill := range skills {
            systemPrompt.WriteString(fmt.Sprintf("<skill name=\"%s\" source=\"%s\">\n", skill.Name, skill.Source))
            systemPrompt.WriteString(skill.Content)
            systemPrompt.WriteString("</skill>\n")
        }
        systemPrompt.WriteString("</skills>\n")
    }
    
    // Tool注入
    if len(tools) > 0 {
        systemPrompt.WriteString("<available_tools>\n")
        for _, tool := range tools {
            systemPrompt.WriteString(fmt.Sprintf("<tool name=\"%s\" required=\"%t\" readonly=\"%t\">\n",
                tool.Name, tool.Required, tool.ReadOnly))
            systemPrompt.WriteString(fmt.Sprintf("  <description>%s</description>\n", tool.Description))
            systemPrompt.WriteString("  <parameters>\n")
            for name, param := range tool.Parameters {
                systemPrompt.WriteString(fmt.Sprintf("    <param name=\"%s\" type=\"%s\" required=\"%t\">%s</param>\n",
                    name, param.Type, param.Required, param.Description))
            }
            systemPrompt.WriteString("  </parameters>\n")
            systemPrompt.WriteString("</tool>\n")
        }
        systemPrompt.WriteString("</available_tools>\n")
    }
    
    // L4: 对话层
    systemPrompt.WriteString("<conversation_history>\n")
    for _, msg := range conversation {
        systemPrompt.WriteString(fmt.Sprintf("<message role=\"%s\">%s</message>\n", msg.Role, msg.Content))
    }
    systemPrompt.WriteString("</conversation_history>\n")
    
    systemPrompt.WriteString(fmt.Sprintf("<current_message>%s</current_message>\n", req.UserMessage))
    
    return &Prompt{
        System: systemPrompt.String(),
    }
}
```

---

## 4. 文件结构调整

### 4.1 新目录结构

```
internal/agent/domain/
├── prompt_builder.go      ← PromptBuilder核心 + 组装逻辑
├── l2_builder.go          ← L2项目层构建
├── skill_loader.go        ← 技能加载器
├── tool_registry.go       ← 工具注册表
├── mcp_gateway.go         ← MCP网关
├── security_layer.go      ← 安全层
├── stage_templates.go     ← 阶段模板加载器
└── query_engine.go        ← 对话引擎

config/prompts/
├── static.xml             ← L1静态内容
└── stages/                ← 阶段模板
    ├── clarify_L3.xml
    └── implement_L3.xml

config/skills/
├── public/                ← 内置技能
│   ├── clarify_analysis.md
│   └── implement_coding.md
└── custom/                ← 自定义技能

config/
└── mcp_servers.yaml       ← MCP服务器配置
```

### 4.2 代码文件职责

| 文件 | 职责 |
|------|------|
| prompt_builder.go | PromptBuilder核心，组装L1+L2+Skill/Tool+L4 |
| l2_builder.go | L2项目层构建（项目偏好+模块索引+stageInstruction+知识查询） |
| skill_loader.go | 技能加载和管理（适配器模式，兼容多格式） |
| tool_registry.go | 工具注册和过滤（阶段感知+权限控制） |
| mcp_gateway.go | MCP外部工具集成 |
| security_layer.go | Sandwich Architecture安全处理 |
| stage_templates.go | 阶段模板加载（服务器默认+项目覆盖） |

---

## 5. 数据流图

```
用户输入
    ↓
┌─────────────────────────────────────────────────────────────┐
│                    PromptBuilder.Build()                      │
├─────────────────────────────────────────────────────────────┤
│  1. getStaticContext()           → L1静态内容（缓存）         │
│  2. projectBuilder.Build()       → L2项目内容（内存+热重载）   │
│  3. skillLoader.LoadForContext() → 技能内容（动态加载）        │
│  4. toolRegistry.GetToolsForContext() → 工具列表（阶段过滤）  │
│  5. mcpGateway.GetTools()        → MCP工具（外部集成）        │
│  6. queryEngine.GetRecentMessages() → 对话历史（最近5轮）     │
│  7. assemble()                   → 组装完整Prompt             │
│  8. securityLayer.Sanitize()     → 安全处理                   │
└─────────────────────────────────────────────────────────────┘
    ↓
完整Prompt（System + Messages + Tools）
    ↓
LLM调用
```

---

## 6. 与原架构的对比

### 6.1 原架构（硬编码）

```go
// 原ToolInjector：硬编码工具列表
func (ti *ToolInjector) getToolsForStage(stage, permissionMode string) []*ToolDefinition {
    stageTools := map[string][]*ToolDefinition{
        "clarify": {
            {Name: "read_file", Description: "Read contents of a file"},
            // ...
        },
    }
    // ...
}
```

### 6.2 新架构（动态加载）

```go
// 新ToolRegistry：阶段感知 + 权限控制
func (tr *ToolRegistry) GetToolsForContext(ctx context.Context, stage, complexity, permission string) []ToolDefinition {
    var tools []ToolDefinition
    for _, tool := range tr.tools {
        if tr.matchesContext(tool, stage, complexity, permission) {
            tools = append(tools, tool)
        }
    }
    return tools
}
```

### 6.3 改进点

| 维度 | 原架构 | 新架构 |
|------|--------|--------|
| 工具定义 | 硬编码 | 结构化定义 |
| 阶段感知 | 简单map | 动态过滤 |
| 权限控制 | 基础 | 细粒度控制 |
| 技能支持 | 无 | 动态加载 |
| 扩展性 | 差 | MCP + 适配器 |
| 兼容性 | 无 | 多格式兼容 |

---

## 7. 实施步骤

### Phase 1: 基础整合
- [ ] 修改PromptBuilder，集成SkillLoader和ToolRegistry
- [ ] 实现assemble方法，支持Skill和Tool注入
- [ ] 替换原ToolInjector

### Phase 2: Skill系统
- [ ] 实现SkillLoader + 适配器模式
- [ ] 创建内置技能（clarify_analysis, implement_coding等）
- [ ] 支持自定义技能

### Phase 3: Tool系统
- [ ] 实现ToolRegistry + 阶段感知
- [ ] 迁移硬编码工具到ToolRegistry
- [ ] 实现权限控制

### Phase 4: MCP集成
- [ ] 实现MCPGateway
- [ ] 配置MCP服务器
- [ ] 支持外部工具

### Phase 5: 测试验证
- [ ] 单元测试
- [ ] 集成测试
- [ ] 性能测试

---

## 8. 总结

### 8.1 整合要点

| 要点 | 说明 |
|------|------|
| **Skill注入位置** | L2之后，作为独立的`<skills>`标签 |
| **Tool注入位置** | Skill之后，作为`<available_tools>`标签 |
| **替换原ToolInjector** | 用ToolRegistry替换硬编码 |
| **新增Skill支持** | 用SkillLoader实现动态加载 |

### 8.2 最终Prompt结构

```
L1 (静态) → L2 (项目) → Skills → Tools → L4 (对话)
   ↓           ↓          ↓        ↓        ↓
static.xml  of-prefs   .md文件   注册表   对话历史
            + 知识查询   动态加载  阶段过滤  最近5轮
```

### 8.3 优势

- ✅ **模块化**：各组件职责清晰
- ✅ **可扩展**：支持自定义技能和MCP工具
- ✅ **兼容性**：适配器模式兼容多格式
- ✅ **企业级**：阶段感知+权限控制+安全隔离

---

**文档版本**: v1.0  
**最后更新**: 2026-05-24