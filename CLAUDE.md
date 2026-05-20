# OpenForge Project Context

> 比赛: Agent 辅助全栈挑战赛 | 题目: 题一「AI工程工具」
> 状态: 设计完成，待进入 Phase 1 编码

## 项目概要

OpenForge 是一个 AI 驱动的端到端全栈开发工作台。以 Conduit (RealWorld 前后端 monorepo) 为实验田，PM 通过对话完成「需求澄清 → 方案拆解 → 实现 → 自动化测试 → 代码部署 → 验证反馈」全链路。

## 已完成的交付物

| 文件 | 内容 | 评审 |
|------|------|------|
| `DESIGN.md` | 完整设计文档 (~2600行)，18章 + 19章数据库 | 6轮评审 |
| `api-contract.yaml` | OpenAPI 3.1，30 端点，完整 Request/Response | 1轮评审 |
| `proto/agent/v1/` | 6 个 .proto + buf.yaml/buf.lock/buf.gen.yaml | 3轮评审 |
| `STYLE_GUIDE.md` | Go/TS/React 编码规范 + 六边形垂直切片架构 | 1轮评审 |

## 架构关键决策

1. **三层正交**: C层(工作台) → A层(Pipeline引擎/Go) → B层(Agent Swarm/Go+Node.js)
2. **六边形+垂直切片**: `domain → port ← adapter`，详见 STYLE_GUIDE.md 第一章
3. **渐进运维**: Phase 1-4 仅 5 容器 (Go+Node+React+PG+Docker)，Phase 10 全企业级
4. **Capability Profile**: minimal/standard/enterprise 三档 YAML 切换
5. **DB 22 表**: UUID v7, WORM 月分区, CHECK 约束全覆盖, 分片预留
6. **Proto 6 Service**: Coordinator/Gate/LLM/Tool/Learning/Terminal
7. **CSP 通信**: Go goroutine+channel 原生, Phase 5+ Redis Streams

## 前端设计规范

### 主力 Skill：`ui-ux-pro-max`（全局已启用）

**所有前端 UI 工作必须首先调用此 Skill。** 覆盖场景：
- 页面设计（Dashboard、Admin、Pipeline 工作台、设置页、审批面板）
- 组件创建（按钮、模态框、表单、表格、图表、导航）
- 设计系统决策（色板、字体、间距、动效）
- UI Review（可访问性、UX 一致性、视觉质量）

**强制工作流：**

1. **Step 1: 分析需求** — 提取产品类型（Developer Tool / Dashboard）、目标用户、风格关键词
2. **Step 2: 生成设计系统（必须）** — 调用 `Skill("ui-ux-pro-max")` 并使用 `--design-system -p "OpenForge"` 参数
3. **Step 2b: 持久化（跨会话复用）** — 加 `--persist` 保存到 `design-system/MASTER.md`
4. **Step 3: 按需补充搜索** — 特定组件/图表/UX 规则用 `--domain` 精确查询
5. **Step 4: 技术栈指南** — `--stack react` 获取 React 特定最佳实践
6. **交付前检查** — 对照 SKILL.md §Pre-Delivery Checklist 逐项验证

**OpenForge 专属 Stack 映射：**

| 前端场景 | Stack |
|---------|-------|
| 工作台主界面 | `react` + `shadcn` |
| Pipeline 可视化 | `react` (DAG/拓扑图) |
| 数据图表面板 | `react` + chart domain |
| 移动端（预留） | `react-native` |

### 补充规范：来自 web-design-engineer 的方法论

以下两个方法论直接纳入前端开发流程，不依赖 web-design-engineer Skill：

**1. 品牌资产协议（Brand Asset Protocol）**

```
资产优先级: Logo (SVG/PNG) > UI 截图 > 色板 > 字体
- Logo 必须真实，禁止 CSS 手绘/文字替代
- 无 Logo 时使用占位符 [logo-pending]，不造假
- 来源优先级: 官方 press kit → 品牌官网 → Wikimedia Commons
```

**2. Critique 自检模式（5 维度评分）**

每次 UI 交付前自检：
| 维度 | 检查点 |
|------|--------|
| 理念对齐 | 是否贯彻了设计系统？有无风格漂移？ |
| 视觉层级 | Squint test 通过？标题/正文比例 ≥ 2.5×？ |
| 工艺质量 | 像素对齐、间距系统一致、颜色 ≤ 4 种、字体 ≤ 2 族 |
| 功能性 | 每个元素是否必要？删掉它设计会变差吗？ |
| 原创性 | 有"意外但正确"的决策吗？还是纯模板输出？ |

### 禁止事项（AI Clichés 红线）

- ❌ 紫色→粉色→蓝色渐变（"AI 风"公式化配色）
- ❌ Emoji 作为图标替代
- ❌ CSS 手绘 Logo / 产品图 / 人物
- ❌ Inter/Roboto/Arial 作为展示字体（标题必须另选）
- ❌ 伪造数据（统计数字、Logo墙、用户评价）
- ❌ `const styles = { ... }` 全局变量名（React 中多文件会覆盖）

## Phase 1-4 MVP 范围

| Phase | 交付 | 关键约束 |
|-------|------|---------|
| **Phase 1** | CLI + minimal profile + 10 接口 stub | 仅 Go+Node, 不写前端 |
| **Phase 2** | 极简 Web 聊天框 + BFF Auth | JWT/XSS/CSP 三高危清零后才能上线 |
| **Phase 3** | Pipeline 状态机 + Diff 预览 + 审批 | Dockview 2-3 面板 |
| **Phase 4** | Docker Sandbox + 一键部署 + Token 成本看板 | 完整闭环 MVP |

## 下一步

下一步: 阅读 `DESIGN.md` → 使用 Skill(`superpowers:writing-plans`) 为 Phase 1 生成实现计划。
