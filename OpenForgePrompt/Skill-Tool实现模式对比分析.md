# 开源项目 Skill/Tool 实现模式对比分析

## 1. 项目概览

| 项目 | 定位 | 核心特点 | 适用场景 |
|------|------|----------|----------|
| **DeerFlow** | 超级智能体编排框架 | 领导代理-子代理架构，技能动态加载 | 复杂多步骤任务，深度研究 |
| **Claude Code** | AI编程助手CLI | 多层扩展系统，Tool/Command/Skills | 日常编程，代码生成 |
| **Plandex** | 终端AI编程工具 | 大规模上下文管理，沙箱隔离 | 大型项目，多文件修改 |

---

## 2. Skill 实现模式对比

### 2.1 DeerFlow 技能模式

**定义方式**: Markdown文件

```markdown
# 技能名称
[描述和最佳实践]
[工作流程]
[支持资源]
```

**特点**:
- ✅ **动态加载**: 仅在任务需要时加载，减少上下文窗口压力
- ✅ **目录隔离**: 内置技能(`/mnt/skills/public`) + 自定义技能(`/mnt/skills/custom`)
- ✅ **元数据支持**: `.skill`档案包含`version`、`author`等信息
- ✅ **渐进式加载**: 保持上下文窗口高效利用

**目录结构**:
```
/mnt/skills/
├── public/          # 内置技能
│   ├── research.md
│   ├── report.md
│   └── slides.md
└── custom/          # 自定义技能
    └── my_skill.skill
```

### 2.2 Claude Code 技能模式

**定义方式**: 多层扩展系统

```
Tool (底层工具) → Command (用户命令) → Skills (高级技能包)
```

**特点**:
- ✅ **分层架构**: Tool/Command/Skills三层分离
- ✅ **工具优先**: 底层工具调用，如文件操作、代码搜索
- ✅ **命令扩展**: 用户可定义的命令
- ✅ **技能包**: 高级技能组合

### 2.3 Plandex 技能模式

**定义方式**: CLI优先，配置驱动

```yaml
# plandex.yaml
autonomy:
  level: "full"  # full, moderate, minimal
  auto_load_files: true
  auto_plan: true
  auto_implement: true
  auto_execute_commands: true
  auto_debug: true
```

**特点**:
- ✅ **可配置自主性**: 从完全自主到精细控制
- ✅ **REPL模式**: 交互式命令行
- ✅ **Git集成**: 自动生成提交信息
- ✅ **自动调试**: 终端命令和浏览器应用自动调试

---

## 3. Tool 实现模式对比

### 3.1 DeerFlow 工具模式

**定义方式**: MCP服务器 + Python函数

```yaml
# MCP服务器配置
mcp_servers:
  server_name:
    url: "http://localhost:3000/sse"
    oauth:
      client_id: $CLIENT_ID
      client_secret: $CLIENT_SECRET
```

**核心工具集**:
- 网页搜索
- 网页抓取
- 文件操作
- bash执行

### 3.2 Claude Code 工具模式

**定义方式**: 结构化工具接口

```go
type ToolDefinition struct {
    Name        string
    Description string
    Parameters  map[string]ParameterSchema
    ReadOnly    bool
    Required    bool
}

type ToolRegistry interface {
    GetTools(ctx context.Context) ([]ToolDefinition, error)
    GetToolsForStage(ctx context.Context, stage string) ([]ToolDefinition, error)
    SearchTools(ctx context.Context, query string, topK int) ([]ToolMatch, error)
}
```

### 3.3 Plandex 工具模式

**定义方式**: CLI命令 + 配置驱动

```bash
# REPL模式
plandex
> load src/
> plan "Add user authentication"
> implement
> test
> commit
```

---

## 4. 架构模式对比

### 4.1 代理架构

| 项目 | 代理模式 | 优势 | 劣势 |
|------|----------|------|------|
| **DeerFlow** | 领导代理-子代理 | 并行处理，任务分解 | 复杂度高 |
| **Claude Code** | 单代理+扩展 | 简单直接 | 并行能力弱 |
| **Plandex** | 单代理+沙箱 | 可靠性高 | 扩展性有限 |

### 4.2 上下文管理

| 项目 | 上下文策略 | 容量 | 特点 |
|------|------------|------|------|
| **DeerFlow** | 隔离+压缩 | 中等 | 子代理隔离，主动总结 |
| **Claude Code** | 动态加载 | 中等 | 按需加载工具和技能 |
| **Plandex** | 大规模缓存 | 200万token | 树状解析器，智能加载 |

### 4.3 扩展机制

| 项目 | 扩展方式 | 灵活性 | 复杂度 |
|------|----------|--------|--------|
| **DeerFlow** | MCP + Python | 高 | 中 |
| **Claude Code** | Tool/Command/Skills | 高 | 高 |
| **Plandex** | 配置驱动 | 中 | 低 |

---

## 5. OpenForge 适配分析

### 5.1 OpenForge 需求特点

| 需求 | 说明 |
|------|------|
| Pipeline阶段感知 | 根据 clarify/implement/test 等阶段动态调整 |
| 复杂度感知 | L1-L4复杂度级别 |
| 权限模式 | plan/auto/default/bypass |
| 企业级特性 | Gate审批、审计、安全隔离 |
| 动态扩展 | 支持自定义技能和工具 |

### 5.2 推荐模式：DeerFlow + Claude Code 混合模式

| 组件 | 选择来源 | 理由 |
|------|----------|------|
| **技能系统** | DeerFlow | Markdown定义 + 动态加载，适合Pipeline阶段感知 |
| **工具系统** | Claude Code | 结构化定义 + 阶段过滤 + 权限控制 |
| **扩展机制** | DeerFlow MCP | 标准化协议，社区生态 |

### 5.3 具体实现建议

```go
// 1. 技能定义 (DeerFlow模式)
type Skill struct {
    Name        string   `yaml:"name"`
    Description string   `yaml:"description"`
    Stages      []string `yaml:"stages"`
    Complexity  []string `yaml:"complexity"`
    Permission  []string `yaml:"permission"`
    Content     string   `yaml:"content"`
}

// 2. 工具定义 (Claude Code模式)
type ToolDefinition struct {
    Name        string               `json:"name"`
    Description string               `json:"description"`
    Parameters  map[string]Parameter `json:"parameters"`
    ReadOnly    bool                 `json:"readOnly"`
    Stages      []string             `json:"stages"`
    Complexity  []string             `json:"complexity"`
    Permission  []string             `json:"permission"`
}

// 3. 动态加载器 (DeerFlow模式)
type SkillLoader struct {
    publicSkills  map[string]*Skill
    customSkills  map[string]*Skill
}

// 4. 工具注册表 (Claude Code模式)
type ToolRegistry interface {
    GetToolsForContext(ctx context.Context, stage, complexity, permission string) ([]ToolDefinition, error)
    SearchTools(ctx context.Context, query string, topK int) ([]ToolMatch, error)
}
```

---

## 6. 总结

| 维度 | DeerFlow | Claude Code | Plandex | OpenForge建议 |
|------|----------|-------------|---------|---------------|
| **技能定义** | Markdown | 多层扩展 | 配置驱动 | Markdown (DeerFlow) |
| **工具定义** | MCP+Python | 结构化接口 | CLI命令 | 结构化接口 (Claude Code) |
| **扩展机制** | MCP | Tool/Command/Skills | 配置 | MCP (DeerFlow) |
| **上下文管理** | 隔离+压缩 | 动态加载 | 大规模缓存 | 隔离+动态加载 |
| **适用场景** | 复杂任务 | 日常编程 | 大型项目 | 企业级开发 |

**最终建议**: 采用DeerFlow的技能动态加载 + Claude Code的工具结构化定义 + MCP扩展机制，形成OpenForge特有的混合模式。