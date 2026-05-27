# OpenForge Skill 兼容性设计

## 1. 问题分析

当前OpenForge的skill格式有特有字段，无法直接兼容其他agent的skill：

| 字段 | OpenForge | DeerFlow | Claude Code | 其他Agent |
|------|-----------|----------|-------------|-----------|
| name | ✅ | ✅ | ✅ | ✅ |
| description | ✅ | ✅ | ✅ | ✅ |
| stages | ✅ | ❌ | ❌ | ❌ |
| complexity | ✅ | ❌ | ❌ | ❌ |
| permission | ✅ | ❌ | ❌ | ❌ |

---

## 2. 解决方案：适配器模式

### 2.1 架构设计

```
┌─────────────────────────────────────────────────────────────┐
│                    SkillLoader                               │
├─────────────────────────────────────────────────────────────┤
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐          │
│  │ OpenForge   │  │  DeerFlow   │  │ Claude Code │          │
│  │   Adapter   │  │   Adapter   │  │   Adapter   │          │
│  └─────────────┘  └─────────────┘  └─────────────┘          │
│         ↓               ↓               ↓                   │
│  ┌─────────────────────────────────────────────────────┐    │
│  │              统一 Skill 接口                          │    │
│  │  - Name, Description (通用)                          │    │
│  │  - Stages, Complexity, Permission (默认值填充)        │    │
│  └─────────────────────────────────────────────────────┘    │
└─────────────────────────────────────────────────────────────┘
```

### 2.2 统一 Skill 接口

```go
// UnifiedSkill 统一技能接口
type UnifiedSkill struct {
    // 通用字段
    Name        string `json:"name"`
    Description string `json:"description"`
    Content     string `json:"content"`
    
    // OpenForge 特有字段（可选，有默认值）
    Stages      []string `json:"stages,omitempty"`
    Complexity  []string `json:"complexity,omitempty"`
    Permission  []string `json:"permission,omitempty"`
    
    // 元数据
    Source      string `json:"source"`      // openforge/deerflow/claude/other
    Version     string `json:"version,omitempty"`
    Author      string `json:"author,omitempty"`
}

// GetStages 获取适用阶段，如果为空返回默认值
func (s *UnifiedSkill) GetStages() []string {
    if len(s.Stages) == 0 {
        return []string{"clarify", "implement", "test", "verify"} // 默认全部阶段
    }
    return s.Stages
}

// GetComplexity 获取复杂度，如果为空返回默认值
func (s *UnifiedSkill) GetComplexity() []string {
    if len(s.Complexity) == 0 {
        return []string{"L1", "L2", "L3", "L4"} // 默认全部复杂度
    }
    return s.Complexity
}

// GetPermission 获取权限，如果为空返回默认值
func (s *UnifiedSkill) GetPermission() []string {
    if len(s.Permission) == 0 {
        return []string{"default", "auto"} // 默认非plan权限
    }
    return s.Permission
}
```

---

## 3. 适配器实现

### 3.1 SkillAdapter 接口

```go
// SkillAdapter 技能适配器接口
type SkillAdapter interface {
    // CanHandle 判断是否能处理该文件
    CanHandle(filename string, content []byte) bool
    
    // Parse 解析技能文件
    Parse(filename string, content []byte) (*UnifiedSkill, error)
    
    // Source 返回来源标识
    Source() string
}
```

### 3.2 OpenForge 适配器

```go
// OpenForgeAdapter OpenForge原生格式适配器
type OpenForgeAdapter struct{}

func (a *OpenForgeAdapter) CanHandle(filename string, content []byte) bool {
    // 检查是否包含OpenForge特有字段
    return strings.Contains(string(content), "stages:") ||
           strings.Contains(string(content), "complexity:")
}

func (a *OpenForgeAdapter) Parse(filename string, content []byte) (*UnifiedSkill, error) {
    // 解析YAML front matter
    skill := &UnifiedSkill{
        Source: "openforge",
    }
    
    // 解析逻辑...
    // 提取stages、complexity、permission等字段
    
    return skill, nil
}

func (a *OpenForgeAdapter) Source() string {
    return "openforge"
}
```

### 3.3 DeerFlow 适配器

```go
// DeerFlowAdapter DeerFlow格式适配器
type DeerFlowAdapter struct{}

func (a *DeerFlowAdapter) CanHandle(filename string, content []byte) bool {
    // 检查是否是DeerFlow格式
    // DeerFlow通常有version、author等字段，但没有stages
    return strings.Contains(string(content), "version:") &&
           !strings.Contains(string(content), "stages:")
}

func (a *DeerFlowAdapter) Parse(filename string, content []byte) (*UnifiedSkill, error) {
    skill := &UnifiedSkill{
        Source: "deerflow",
        // DeerFlow没有stages/complexity/permission，使用默认值
        Stages:     []string{"clarify", "implement", "test", "verify"},
        Complexity: []string{"L1", "L2", "L3", "L4"},
        Permission: []string{"default", "auto"},
    }
    
    // 解析DeerFlow格式的元数据
    // 提取name、description、version、author等
    
    return skill, nil
}

func (a *DeerFlowAdapter) Source() string {
    return "deerflow"
}
```

### 3.4 Claude Code 适配器

```go
// ClaudeCodeAdapter Claude Code格式适配器
type ClaudeCodeAdapter struct{}

func (a *ClaudeCodeAdapter) CanHandle(filename string, content []byte) bool {
    // Claude Code使用JSON或特定格式
    return strings.HasSuffix(filename, ".json") &&
           strings.Contains(string(content), "\"type\":\"skill\"")
}

func (a *ClaudeCodeAdapter) Parse(filename string, content []byte) (*UnifiedSkill, error) {
    skill := &UnifiedSkill{
        Source: "claude",
        // Claude Code没有stages/complexity/permission，使用默认值
        Stages:     []string{"clarify", "implement", "test", "verify"},
        Complexity: []string{"L1", "L2", "L3", "L4"},
        Permission: []string{"default", "auto"},
    }
    
    // 解析Claude Code格式
    // 提取name、description、tools等
    
    return skill, nil
}

func (a *ClaudeCodeAdapter) Source() string {
    return "claude"
}
```

### 3.5 通用 Markdown 适配器（兜底）

```go
// GenericMarkdownAdapter 通用Markdown适配器
type GenericMarkdownAdapter struct{}

func (a *GenericMarkdownAdapter) CanHandle(filename string, content []byte) bool {
    // 兜底：所有.md文件都能处理
    return strings.HasSuffix(filename, ".md")
}

func (a *GenericMarkdownAdapter) Parse(filename string, content []byte) (*UnifiedSkill, error) {
    skill := &UnifiedSkill{
        Source:     "generic",
        Name:       strings.TrimSuffix(filename, ".md"),
        Content:    string(content),
        // 使用默认值
        Stages:     []string{"clarify", "implement", "test", "verify"},
        Complexity: []string{"L1", "L2", "L3", "L4"},
        Permission: []string{"default", "auto"},
    }
    
    // 尝试从内容中提取描述
    // 查找第一个段落作为描述
    
    return skill, nil
}

func (a *GenericMarkdownAdapter) Source() string {
    return "generic"
}
```

---

## 4. SkillLoader 增强

```go
// SkillLoader 增强版技能加载器
type SkillLoader struct {
    publicSkills  map[string]*UnifiedSkill
    customSkills  map[string]*UnifiedSkill
    adapters      []SkillAdapter
    mu            sync.RWMutex
}

// NewSkillLoader 创建技能加载器
func NewSkillLoader(publicDir, customDir string) *SkillLoader {
    sl := &SkillLoader{
        publicSkills: make(map[string]*UnifiedSkill),
        customSkills: make(map[string]*UnifiedSkill),
    }
    
    // 注册适配器（按优先级排序）
    sl.adapters = []SkillAdapter{
        &OpenForgeAdapter{},      // 优先：OpenForge原生格式
        &DeerFlowAdapter{},       // 其次：DeerFlow格式
        &ClaudeCodeAdapter{},     // 再次：Claude Code格式
        &GenericMarkdownAdapter{}, // 兜底：通用Markdown
    }
    
    // 加载技能
    sl.loadSkillsFromDir(publicDir, "public")
    sl.loadSkillsFromDir(customDir, "custom")
    
    return sl
}

// loadSkillsFromDir 从目录加载技能
func (sl *SkillLoader) loadSkillsFromDir(dir, source string) {
    files, err := os.ReadDir(dir)
    if err != nil {
        return
    }
    
    for _, file := range files {
        if file.IsDir() {
            continue
        }
        
        filename := file.Name()
        content, err := os.ReadFile(filepath.Join(dir, filename))
        if err != nil {
            continue
        }
        
        // 使用适配器解析
        skill, err := sl.parseWithAdapter(filename, content)
        if err != nil {
            continue
        }
        
        // 根据来源存储
        if source == "public" {
            sl.publicSkills[skill.Name] = skill
        } else {
            sl.customSkills[skill.Name] = skill
        }
    }
}

// parseWithAdapter 使用适配器解析
func (sl *SkillLoader) parseWithAdapter(filename string, content []byte) (*UnifiedSkill, error) {
    for _, adapter := range sl.adapters {
        if adapter.CanHandle(filename, content) {
            return adapter.Parse(filename, content)
        }
    }
    
    return nil, fmt.Errorf("no adapter found for %s", filename)
}

// LoadForContext 根据上下文加载技能
func (sl *SkillLoader) LoadForContext(ctx context.Context, stage, complexity, permission string) []*UnifiedSkill {
    sl.mu.RLock()
    defer sl.mu.RUnlock()
    
    skillMap := make(map[string]*UnifiedSkill)
    
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
    
    var skills []*UnifiedSkill
    for _, skill := range skillMap {
        skills = append(skills, skill)
    }
    
    return skills
}

// matchesContext 检查技能是否匹配上下文
func (sl *SkillLoader) matchesContext(skill *UnifiedSkill, stage, complexity, permission string) bool {
    if !contains(skill.GetStages(), stage) {
        return false
    }
    if !contains(skill.GetComplexity(), complexity) {
        return false
    }
    if !contains(skill.GetPermission(), permission) {
        return false
    }
    return true
}
```

---

## 5. 兼容性矩阵

| 格式 | 适配器 | stages | complexity | permission | 说明 |
|------|--------|--------|------------|------------|------|
| OpenForge | OpenForgeAdapter | ✅ 原生 | ✅ 原生 | ✅ 原生 | 完全支持 |
| DeerFlow | DeerFlowAdapter | 默认值 | 默认值 | 默认值 | 自动填充默认值 |
| Claude Code | ClaudeCodeAdapter | 默认值 | 默认值 | 默认值 | 自动填充默认值 |
| 通用Markdown | GenericMarkdownAdapter | 默认值 | 默认值 | 默认值 | 兜底方案 |

---

## 6. 使用示例

### 6.1 OpenForge 原生格式

```yaml
---
name: clarify_analysis
description: 分析需求，理解上下文
stages: [clarify]
complexity: [L2, L3, L4]
permission: [default, auto]
---

# Clarify 分析技能

## 工作流程
1. 读取相关文件
2. 分析项目结构
3. 提出澄清问题
```

### 6.2 DeerFlow 格式（自动兼容）

```yaml
---
name: research
description: Deep research skill
version: 1.0
author: DeerFlow Team
---

# Research Skill

## 工作流程
1. 搜索相关信息
2. 整理研究结果
3. 生成研究报告
```

**自动转换为**：
```go
UnifiedSkill{
    Name:       "research",
    Description: "Deep research skill",
    Version:    "1.0",
    Author:     "DeerFlow Team",
    Source:     "deerflow",
    Stages:     ["clarify", "implement", "test", "verify"],  // 默认值
    Complexity: ["L1", "L2", "L3", "L4"],                    // 默认值
    Permission: ["default", "auto"],                          // 默认值
}
```

### 6.3 通用 Markdown（自动兼容）

```markdown
# My Custom Skill

This is a custom skill for specific tasks.

## Steps
1. Do something
2. Do something else
```

**自动转换为**：
```go
UnifiedSkill{
    Name:       "My Custom Skill",
    Description: "This is a custom skill for specific tasks.",
    Source:     "generic",
    Stages:     ["clarify", "implement", "test", "verify"],  // 默认值
    Complexity: ["L1", "L2", "L3", "L4"],                    // 默认值
    Permission: ["default", "auto"],                          // 默认值
}
```

---

## 7. 总结

### 7.1 兼容性保证

| 机制 | 说明 |
|------|------|
| **适配器模式** | 每种格式有专用适配器 |
| **默认值填充** | 缺失字段自动填充合理默认值 |
| **优先级排序** | OpenForge > DeerFlow > Claude > 通用 |
| **兜底方案** | 通用Markdown适配器处理所有.md文件 |

### 7.2 扩展性

- 新增格式：只需实现 `SkillAdapter` 接口
- 自定义默认值：可在配置文件中调整
- 格式转换：适配器可输出统一格式

### 7.3 向后兼容

- ✅ DeerFlow skill 直接可用
- ✅ 通用 Markdown skill 直接可用
- ✅ OpenForge 特有功能完整支持

---

**文档版本**: v1.0  
**最后更新**: 2026-05-24