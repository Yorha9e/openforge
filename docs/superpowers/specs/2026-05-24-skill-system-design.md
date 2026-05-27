# OpenForge Skill 系统设计规范

> 日期: 2026-05-24 | 版本: v1.0 | 状态: 设计完成

## 概要

为 OpenForge 的 PromptBuilder 新增 **Capability 能力层**（L2 与 L4 之间），统一管理 Skill 和 Tool 的注入。本文档仅覆盖 **Skill 机制**（Tool 机制已有完整实现，增强项另案处理）。

---

## §1 核心定义

### 1.1 Skill 的本质

Skill = **System Prompt 片段（主体）** + **可选工作流步骤（Agent 可自主偏离）**。

| 组成部分 | 作用 | 示例 |
|---------|------|------|
| Prompt 片段 | 注入 System Prompt，影响 Agent 行为和输出风格 | "使用 react-hook-form + zod 进行表单校验" |
| 工作流步骤 | 建议的执行顺序，Agent 可自主调整 | "1.检查现有校验 2.创建 schema 3.集成 useForm" |

### 1.2 文件格式

YAML frontmatter + Markdown body：

```markdown
---
name: react-form-pattern
version: "1.2.0"
stages: [impl, test]
complexity: [L2, L3, L4]
permission: [auto, default]
keywords: [form, validation, input, zod, react-hook-form]
triggers:
  file_patterns: ["*.tsx", "*Form*"]
  user_intent: [create form, add validation, fix input]
base_priority: 70
---

# React Form Pattern

## Prompt
使用 react-hook-form + zod 进行表单校验。遵循以下规范：
...

## Workflow (optional)
1. 检查现有表单组件的校验模式
2. 创建独立的 schema 定义文件
3. 在组件中集成 useForm + zodResolver
```

### 1.3 Frontmatter 字段

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `name` | string | ✅ | 唯一标识，同项目中不可重复 |
| `version` | string | ✅ | semver，如 "1.2.0" |
| `stages` | []string | ✅ | 适用阶段：clarify/decompose/impl/test/deploy/verify |
| `complexity` | []string | ❌ | 适用复杂度：L1-L4。不填 = 全适用 |
| `permission` | []string | ❌ | 权限模式：plan/auto/default/bypass。不填 = 全适用 |
| `keywords` | []string | ❌ | 关键词，用于匹配打分 |
| `triggers.file_patterns` | []string | ❌ | 文件模式触发，如 `["*.tsx"]` |
| `triggers.user_intent` | []string | ❌ | 用户意图触发，如 `["create form"]` |
| `base_priority` | int | ❌ | 0-100，默认 70。管理员可手动调整 |

### 1.4 仓库层级

```
config/skills/global/       ← 平台内置，随 OpenForge 发布
.openforge/team-skills/     ← 团队共享仓库
.openforge/skills/          ← 项目独有
```

加载优先级：**project > team > global**（同名 project 覆盖 team，team 覆盖 global）。

---

## §2 Skill 注册表

### 2.1 skill_config.yaml

每个层级目录（global/team/project）各有一个 `skill_config.yaml`，是该目录下所有 Skill 的运行时状态索引：

```yaml
skills:
  - name: react-form-pattern
    version: "1.2.0"
    file: react-form-pattern.md
    base_priority: 80
    current_priority: 82
    enabled: true
    is_latest: true
    deprecated: false

  - name: react-form-pattern
    version: "1.1.0"
    file: react-form-pattern-v1.1.0.md
    base_priority: 80
    current_priority: 0.12
    enabled: true
    is_latest: false
    deprecated: true
```

- `.md` 文件是 Source of Truth（内容）
- `skill_config.yaml` 是运行时状态（优先级、弃用标记、指标）
- SkillLoader 启动时 scan 目录拿到基础字段 → merge `skill_config.yaml` → 产出不可变 `skillSnapshot`

### 2.2 并发控制

- **进程内**：`atomic.Value` + `sync.RWMutex`（读无锁，写持 Mutex）
- **文件写入**：write-tmp + atomic rename（防止部分写入）
- **多进程部署**：文件 advisory lock（Phase 8 多 Coordinator 时启用）

---

## §3 SkillLoader 架构

### 3.1 核心结构

```go
type SkillLoader struct {
    scanDirs    []string              // [global, team, project]
    configPath  string                // skill_config.yaml 路径
    watcher     *fsnotify.Watcher     // 文件变更监听
    mu          sync.Mutex            // 保护 buildSnapshot + atomic swap
    current     atomic.Value          // *skillSnapshot (不可变, CoW)
    cache       sync.Map              // key: cacheKey(stage+msg+budget) → []Skill
    polling     *time.Ticker          // 30s fallback (Windows fsnotify 不稳定)
}

type skillSnapshot struct {
    skills    []Skill
    config    map[string]SkillConfig
    buildTime time.Time
}
```

### 3.2 生命周期

```
Init()
  → buildSnapshot()
    → scanDirs: 遍历三层目录，解析每个 .md 的 YAML frontmatter
    → merge skill_config.yaml: 覆盖 current_priority, deprecated, enabled
    → 返回不可变 *skillSnapshot
  → current.Store(snapshot)
  → go watchLoop()

Match(req):
  → 检查 sync.Map 缓存 (stage+msg+budget)
  → 缓存未命中 → snapshot.match(req) → 写入缓存
  → 返回 []Skill

watchLoop():
  → fsnotify Event → debounce 1s → Reload()
  → 30s polling ticker (fallback)
  → Reload(): mu.Lock → buildSnapshot → current.Store → 清空 cache

Reload():
  → 全量重建（非增量）。三层 override 关系 + 同名多版本使增量复杂且收益低
  → 重载期间 Match() 读旧 snapshot，无阻塞
  → 失败不回退，WARN 日志保留旧 snapshot
```

### 3.3 Match 缓存

- Key: `hash(stage + userMessage + tokenBudget)`
- 生命周期: 跟随 snapshot。新 snapshot → 旧缓存随旧 snapshot 被 GC
- 缓存不需要 TTL，因为 snapshot 不变则结果不变

---

## §4 匹配与排序

### 4.1 匹配链

```
候选集 = stage 精确过滤
       + 排除 deprecated == true (除非全部 deprecated)
       + 排除 permission 不匹配
       + 排除 enabled == false
    ↓
计算 finalScore:
    triggerPoints = file_pattern 命中(+30) + user_intent 命中(+20) + keywords 命中(+15×命中率)
    matchScore = triggerPoints / 50.0                   // 归化 0-1
    finalScore = skill.CurrentPriority × matchScore
    ↓
排序:
    1. finalScore 降序
    2. 同分 → project(3) > team(2) > global(1)
    3. 同分同层级 → CurrentPriority 高者优先
    4. 同分同层级同 priority → 版本号高者优先 (semver)
    5. 同版本 → CreatedAt 近者优先
    6. 最终 tiebreaker → Name 字母序（保证排序稳定）
    ↓
top-K: K = min(len(candidates), maxSkillInject)
       maxSkillInject 默认 5
       tokenBudget 不足时提前截断
```

### 4.2 同名多版本

同名 Skill（不同 version）只取 `CurrentPriority` 最高者注入：

```go
bestVersion := maxBy(sameNameSkills, func(s *Skill) float64 {
    return s.CurrentPriority
})
```

旧版本通过版本衰减自然降权，最终被弃用。

### 4.3 边界条件

| 场景 | 行为 |
|------|------|
| 无任何 Skill 命中 | 返回空列表，正常执行 |
| 所有候选均 deprecated | 返回空列表，WARN 日志 |
| `tokenBudget == 0` | 停止注入，返回 `pending_user` + 提示"是否注入 Skill？[是/跳过]" |
| 多触发器命中 > maxSkillInject | 截断至 maxSkillInject (默认 5) |

### 4.4 强制拉起

用户通过 `/skill <name>` 命令或下拉选择器强制拉起 Skill：

```go
func (sl *SkillLoader) MatchByName(name string) (*Skill, error) {
    snapshot := sl.current.Load().(*skillSnapshot)
    // 按 name 精确查找，无视 stage/complexity/permission
    // 同名多版本取 CurrentPriority 最高者
}
```

- 强制拉起 = 无条件 top-1
- **不参与 token budget 计算**
- 超支时 WARN 日志但照常注入
- 强制拉起的 Skill 走正常 Gate 审批链（不绕过权限，只绕过匹配）
- 同一 Pipeline 允许多次 `/skill` 累加

---

## §5 注入位置

### 5.1 新的 Prompt 构建链

```
L1 静态安全层 (static.xml)
  ↓
L2 项目融合层 (ProjectPrefs + stageInstruction + KnowledgeQuerier + Metadata)
  ↓
Capability 能力层 (Skill + Tool 统一注入)   ← NEW
  ↓
L4 对话摘要层 (recent N rounds)
```

### 5.2 CapabilityInjector

```go
type CapabilityInjector struct {
    skillLoader  *SkillLoader
    toolRegistry port.ToolRegistryClientFull
}

func (ci *CapabilityInjector) Inject(ctx context.Context, req CapabilityRequest) (*CapabilityResult, error) {
    // 1. Skill 匹配
    skills := ci.skillLoader.Match(req)
    
    // 2. Tool 注入
    tools := ci.toolRegistry.SearchTools(ctx, req.Stage, topK)
    
    // 3. tokenBudget 检查
    if req.TokenBudget == 0 && len(skills) > 0 {
        return &CapabilityResult{PendingUser: true, Message: "是否注入 Skill？"}, nil
    }
    
    // 4. 组装 XML: Skills 在前（指导行为），Tools 在后（可用工具）
    return &CapabilityResult{Skills: skillRecords, Tools: toolRecords}, nil
}

type CapabilityResult struct {
    Skills      []SkillInjectionRecord
    Tools       []ToolInjectionRecord
    PendingUser bool
    Message     string
}

type SkillInjectionRecord struct {
    Name             string  `json:"name"`
    Version          string  `json:"version"`
    Source           string  `json:"source"`            // "project" | "team" | "global"
    TriggerScore     float64 `json:"trigger_score"`
    CurrentPriority  float64 `json:"current_priority"`
    TriggeredBy      string  `json:"triggered_by"`      // "stage_filter" | "user_command" | "keyword_match" | "file_pattern"
    TokenCost        int     `json:"token_cost"`
}
```

---

## §6 优先级算法

### 6.1 核心公式

```
CurrentPriority = BasePriority × VersionFactor × LearningFactor

BasePriority:    默认 70 (0-100)，管理员可手动调整
VersionFactor:   旧版本自动衰减，latest 版本 = 1.0
LearningFactor:  基于近期采纳率和使用频率，无数据时 = 1.0
```

### 6.2 版本衰减因子

```
is_latest == true  → VersionFactor = 1.0
is_latest == false → 指数衰减: e^(-0.1 × days_since_latest_published)
                     最小值 = 0.05（不归零）
```

### 6.3 学习反馈因子

```
UsageCount < 10  → LearningFactor = 1.0（冷启动中性）

UsageCount ≥ 10  → LearningFactor = AcceptRate × 0.7 + UsageNorm × 0.3
  AcceptRate = recentAcceptCount / recentTotalDecisions (7 天窗口)
  UsageNorm   = sigmoid(recentUsageCount, midpoint=10, steepness=0.5)
              = 1 / (1 + e^(-0.5 × (count - 10)))
  取值范围: [0.1, 2.0]
```

### 6.4 自动弃用

```
条件: CurrentPriority < 0.15（连续 3 天）
动作: deprecated = true, 写入 skill_config.yaml
      C 层面板展示"待确认弃用"

人工审核:
  确认弃用 → 移除 Skill 文件 + skill_config.yaml
  恢复      → CurrentPriority = base_priority × 0.3
  30d 不响应 → 自动移除

全部 deprecated: 同名 Skill 所有版本均 deprecated → WARN 提醒 "Skill X 已无可用版本"
```

### 6.5 每日更新任务

```
Cron: 每天 03:00

步骤:
  1. 加载所有 Skill 的最新 Metrics (7d 窗口, 从 TrajectoryStore 聚合)
  2. 逐个计算 CurrentPriority
  3. 检查弃用阈值 (连续 3 天低于阈值)
  4. 生成变更 diff: {skill, old_priority, new_priority, reason}
  5. 原子写入 skill_config.yaml (write-tmp + rename)
  6. 推送 C 层通知: "N 个 Skill 优先级更新，M 个待确认弃用"
  7. SkillLoader.Watch() → Reload()
```

### 6.6 初期排序策略（优先级相同时）

当所有 Skill 的 `CurrentPriority` 相同（冷启动）时，排序退化为：

```go
func initialSort(skills []SkillCandidate) {
    sort.Slice(skills, func(i, j int) bool {
        if skills[i].TriggerScore != skills[j].TriggerScore {
            return skills[i].TriggerScore > skills[j].TriggerScore
        }
        if skills[i].Level != skills[j].Level {
            return skills[i].Level > skills[j].Level  // project(3) > team(2) > global(1)
        }
        if !skills[i].CreatedAt.Equal(skills[j].CreatedAt) {
            return skills[i].CreatedAt.After(skills[j].CreatedAt)
        }
        return skills[i].Name < skills[j].Name
    })
}
```

---

## §7 前端可见性

### 7.1 Skill 调用标签

Agent 消息旁显示 Skill 标签：`🧩 react-form-pattern v1.2`

Hover 显示详情：匹配原因、trigger_score、CurrentPriority、token 消耗。

### 7.2 管理面板 (只读)

路由: `/admin/skills`

| 功能 | 说明 |
|------|------|
| 列表视图 | 搜索、按来源/状态/阶段过滤、按优先级排序 |
| 详情视图 | 完整 frontmatter + Prompt 预览 + 版本历史 + 优先级分解（base × version × learning） |
| 匹配日志 | 每个 Skill 的匹配历史：Pipeline ID、匹配原因、trigger_score |
| 弃用确认 | "待确认弃用"列表 → [确认弃用] [保留恢复] |
| 人工调权 | 管理员手动修改 base_priority（输入新值 + 原因 → 审计日志） |

### 7.3 强制拉起入口

- `/skill <name>` 对话命令
- 输入框旁 `🧩` 下拉选择器，列出当前阶段可用 Skill

### 7.4 API

```
GET    /api/admin/skills                      # Skill 列表（搜索/过滤/排序）
GET    /api/admin/skills/:name                # Skill 详情 + 指标 + 匹配日志
GET    /api/admin/skills/:name/versions       # 所有版本历史
PUT    /api/admin/skills/:name/priority       # 手动调整 base_priority {value, reason}
POST   /api/admin/skills/:name/confirm-deprecation  # 确认弃用 {action: "confirm"|"restore"}
GET    /api/admin/skills/pending-deprecations       # 待确认弃用列表
GET    /api/pipelines/:pid/skills             # Pipeline 被注入的 Skill 追溯
```

---

## §8 Learning Engine 集成

```
Pipeline 完成
  → TrajectoryStore.Record(skills_injected, skills_used, acceptance_rate)
  → 每日 03:00 UnifiedPriorityEngine.RunDailyUpdate()
    → 读 TrajectoryStore 聚合 7d 指标
    → 计算每个 Skill 的 CurrentPriority = BasePriority × VersionFactor × LearningFactor
    → 检查弃用阈值
    → 原子写入 skill_config.yaml
    → 推送通知
    → SkillLoader.Reload()
```

不新增接口。复用 Phase 7 `TrajectoryStore`，`UnifiedPriorityEngine` 是独立 goroutine。

---

## §9 降级策略

| 故障 | 行为 |
|------|------|
| SkillLoader 启动失败 | `current = emptySnapshot`，Capability 层仅注入 Tool，不阻断 Pipeline |
| Reload 失败 | 保留旧 snapshot，WARN 日志 |
| skill_config.yaml 损坏 | 从 .md 文件重建（base_priority=70, 无学习因子），ERROR 日志 |
| fsnotify 不可用 | 退化为 30s 轮询 |
| 强制拉起不存在的 Skill | 返回错误消息，不阻断对话 |
| token budget = 0 时注入 | 返回 pending_user，等待用户选择 |

---

## §10 文件清单 (Phase 7.5)

```
新建:
  config/skills/global/                         ← 平台内置 Skill 目录
  internal/agent/domain/skill_loader.go          ← SkillLoader 核心 (~300行)
  internal/agent/domain/skill_loader_test.go     ← Test
  internal/agent/domain/capability_injector.go   ← CapabilityInjector (~150行)
  internal/agent/domain/capability_injector_test.go
  internal/agent/domain/priority_engine.go       ← UnifiedPriorityEngine (~200行)
  internal/agent/domain/priority_engine_test.go

修改:
  internal/agent/domain/prompt_builder.go        ← +CapabilityInjector 调用
  internal/agent/domain/prompt_builder_test.go
  internal/agent/domain/query_engine.go          ← +"/skill" 命令解析
  internal/shared/profile/bootstrap.go           ← +SkillLoader + CapabilityInjector 注入
  frontend/src/features/admin/SkillPanel.tsx      ← 管理面板
  frontend/src/features/chat/SkillBadge.tsx       ← Skill 标签
  frontend/src/features/chat/ChatInput.tsx        ← +🧩 下拉选择器

不修改:
  internal/agent/domain/tools_stages.go          ← 保留，CapabilityInjector 内部调用
  internal/tool/registry.go                      ← 不变
  proto/agent/v1/tools.proto                     ← 不变
```

---

## §11 配置

```yaml
# config/skill_engine.yaml — 应用级全局配置（区别于 per-directory 的 skill_config.yaml）
skill:
  max_inject: 5                    # 单次最大注入 Skill 数
  avg_skill_tokens: 500            # 单 Skill 平均 token 估算
  snapshot_cache_enabled: true     # Match 缓存开关
  
  # 优先级引擎参数
  priority:
    default_base: 70
    decay_mode: "exponential"
    decay_rate: 0.1
    max_decay_days: 60
    min_version_factor: 0.05
    window_days: 7
    accept_weight: 0.7
    usage_weight: 0.3
    deprecate_threshold: 0.15
    min_usage_for_learning: 10

  # 文件监听
  watch:
    debounce_ms: 1000
    poll_fallback_s: 30
```
