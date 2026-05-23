# OpenForge (原 SuperAgent) — AI 工程工具 设计文档

> 日期: 2026-05-24 | 比赛: Agent 辅助全栈挑战赛 | 题目: 题一「AI工程工具」 | 版本: v5 (Phase 5d Prompt 三层简化 + §4.2b 新增 + 文件清单更新)

---

## 一、概述

### 1.1 产品定义

OpenForge (原 SuperAgent) 是一个 AI 驱动的端到端全栈开发工作台。以 Conduit (RealWorld 前后端 monorepo) 为实验田，产品经理通过对话完成「需求澄清 → 方案拆解 → 实现(定位+生成) → 自动化测试 → 代码部署 → 验证反馈」全链路，各阶段均可人工介入。

### 1.2 核心设计原则

1. **三层正交** — 工作台(C)、Pipeline(A)、Agent Swarm(B) 三层职责严格分离，互不越界
2. **YAGNI 实现，接口预留** — 当前仅实现 Linux 部署，但架构层预留多 OS/多 Region/分库扩展点
3. **所有外部资源加 `of_` 前缀** — 防止与已有基础设施命名冲突
4. **不要训练模型权重** — 所有「学习」都是 prompt engineering + 规则
5. **每种语言做它最擅长的事** — Go 管协调，Node.js 管 IO
6. **业务不直接高频读写数据库** — 所有高频操作走内存缓冲/计数器，异步批量落盘；DB 仅承载低频写入

**高频 vs 低频边界**:

| 操作 | 频率 | 写入路径 | 一致性策略 |
|------|------|---------|-----------|
| Token 计数 | 每轮 LLM 对话 (~100次/Pipeline) | ring buffer → 500条或5s → 批量 COPY | 最终一致 (crash 丢失 <0.1%) |
| Token 超限检测 | 每轮 LLM 对话 | atomic counter (内存, 零DB查询) | 最终一致 (5s flush 窗口) |
| 检查点 | 每 10 轮 (~10次/Pipeline) | 内存缓冲 → 异步 flush PG | 保留最近 3 个内存 + 1 个 PG 已确认, crash 回退到 PG 确认的最近检查点 |
| Pipeline 状态变更 | 每阶段切换 (数次/Pipeline) | 直接 PG (低频) | 强一致 (同步写) |
| Gate 事件/审计 | 每 Gate (数次/Pipeline) | 直接 PG + MinIO WORM (低频) | 强一致 |
| 文件锁 | 每模块定位 (1次/Pipeline) | 内存为主, PG 异步备份 | 最终一致 (crash 后从活跃 worktree 重建) |
| 许可证校验 | 部署时 (1次/Pipeline) | 直接 MinIO (低频) | 强一致 |
| 知识回写 | 每 Pipeline 完成 (1次) | Delta → 异步合并索引 (低频) | 最终一致 |

### 缓存一致性分层策略

所有缓存按「准实时度」分三类，容忍度不同用不同策略:

```
P0 强一致 (不可丢失, 不可窗口):
  ├── Pipeline 状态 (Postgres 同步写, 事务保证)
  ├── Gate 审批事件 (Postgres + MinIO WORM 双写, 同步)
  └── 审计日志 (INSERT-only, 同步)

P1 准实时 (< 5s 窗口, crash 可恢复):
  ├── 检查点 (内存最近 3 个 → 异步 flush PG → crash 回退到已确认的)
  ├── 文件锁 (内存为主 → crash 后从活跃 worktree + PG 重建)
  └── Feature Flags (Redis 主读 → PG source of truth → 变更后主动 invalidate Redis)

P2 最终一致 (可容忍 < 60s 窗口, 精度 > 99%):
  ├── Token 计数 (ring buffer → batch flush, crash 丢失 ~0.1%)
  ├── Token 超限检测 (atomic counter, 5s flush)
  ├── Redis 配额计数器 (INCR in Redis → 每 30s flush PG → Redis crash 后从 PG 恢复)
  ├── 嵌入索引 (CoW 原子交换 → reader 无锁 → 新写入仅对后续查询可见)
  └── Session 缓存 (Redis, TTL 自动过期, 丢失只需重新登录)

一致性恢复:
  Coordinator 启动时 → 扫描 PG 中 running Pipeline → 从 PG 检查点恢复 Agent 状态
  → 从 active worktree 重建文件锁 → 从 PG 重建 quota 基数 → 从空 ring buffer 开始 Token 计数
```

### 1.3 竞争定位

> OpenForge 不是「帮开发者写代码」，而是「让 PM 能交付软件，开发做审查而非翻译」。企业级 Gate + 组织知识库 + 气隙内网 = 竞品无法触及的市场。

| 产品 | 面向谁 | 端到端能力 | 企业Gate | 组织知识库 | 气隙内网 |
|------|--------|-----------|----------|-----------|---------|
| Copilot/Cursor/Windsurf | 开发者 | ❌ | ❌ | ❌ | ❌ |
| Aider | 开发者 | ❌ | ❌ | ❌ | ❌ |
| Claude Code/Codex CLI | 开发者 | ⚠️ 需开发者驱动 | ❌ | ❌ | ❌ |
| Devin | 开发者 | ✅ | ❌ 人工介入弱 | ❌ | ❌ |
| **OpenForge** | **PM驱动, 开发审查** | **✅** | **✅ 硬件Gate** | **✅ 三层** | **✅** |

**三道护城河**: ① Pipeline + 硬件 Gate（不是可选审批）② 项目/团队/全局三级组织知识库 ③ 从第一天就设计为企业内网气隙部署

### 1.4 渐进运维路径

为保持「OpenForge」定位，Phase 1-6 仅需 5 个容器 (Go + Node + React + Postgres + Docker)，后续按需增加组件:

| Phase | 新增组件 | 总组件数 |
|-------|---------|---------|
| 1-4 (MVP) | Go, Node, React, Postgres, Docker | 5 |
| 5-6 | Redis (LLM 队列) | 6 |
| 7-8 | MinIO, OTel, PgBouncer | 9 |
| 9-10 | K8s, Vault, Grafana, DR Region | 13 |

Postgres 临时替代 Redis(队列/缓存) + MinIO(本地文件) + Grafana(内置/metrics/html)，延后但不砍掉。

---

## 二、总体架构

### 2.1 三层架构

```
┌─────────────────────────────────────────────────────┐
│              C层: 协作工作台 (BFF + 微前端)            │
│  📋 需求面板 │ 📝 代码审查(Diff+行级评论) │ 🚀 CI/CD  │
│  💬 Agent 对话面板 │ 👥 RBAC │ 📊 成本看板            │
├─────────────────────────────────────────────────────┤
│              A层: Pipeline 引擎 (Go)                  │
│  澄清 → 拆解 → 实现 → 测试 → 部署 → 验证              │
│  DAG 回溯 │ Gate 审批 │ 复杂度分级 │ 文件锁 │ 灰度    │
├─────────────────────────────────────────────────────┤
│              B层: Agent Swarm 运行时 (Go + Node.js)   │
│  Go: CSP Channel │ goroutine pool │ 检查点 │ WAL     │
│  Node: Tool Hub │ Skill Loader │ LLM Router │ Learn  │
└─────────────────────────────────────────────────────┘
```

### 2.2 跨切面关注点

- **全链路 OTel Trace ID** — 从 PM 提需求到部署完成
- **熔断** — LLM 调用失败超阈值 → 自动降级，Pipeline 挂起通知人工
- **并发** — 同项目同时最多 N 条 Pipeline (可配)，超出排队
- **`of_` 前缀** — Bucket / Redis Key / Metric / Git Branch 统一前缀

### 2.3 核心数据流

```
PM 输入需求
  → BFF (鉴权 + 限流)
  → A层: 复杂度判定 → 创建 Pipeline → 路由到 Coordinator 分片
  → B层: Agent Swarm 执行具体阶段
  → Artifact Store (MinIO) 持久化产物
  → Gate 通知人工审批 (飞书/钉钉)
  → 审批通过 → 下一阶段
  → 部署到 Staging → 验证反馈
  → 知识回写到 Learning Engine
```

### 2.4 Monorepo 拓扑分析

实现阶段启动时，Agent 先分析仓库生成跨栈拓扑图:

```
frontend/ + backend/ 在同一个 monorepo
→ 解析前端 HTTP 调用 + 后端路由注册
→ API 端点匹配 (GET /api/articles ↔ router.get('/api/articles', ...))
→ 合并为统一拓扑图 (前端组件 → API → 中间件 → 模型 → 数据库)
→ PM/Dev 审查确认模块范围 → Agent 开始实现
```

实现: B 层 `topology-analyzer/` (frontend-parser, backend-parser, cross-stack-linker, unified-builder)，C 层 `TopologyViewer.tsx` (Cytoscape.js/D3.js 渲染)。首次接入全量生成作为种子知识，后续增量更新。

底层 parser 支持 tree-sitter AST 级解析 (Phase 3)，替换纯正则匹配，提升 import/require/路由识别的准确率。LSP 保留用于符号跳转。

---

### 2.5 消息队列与事件驱动策略

**Phase 1-4 (MVP): 不使用 MQ。** PG SKIP LOCKED + CSP Channel + ring buffer + goroutine 足够。Pipeline 由 PM 手动创建，天然限速，不存在突发流量。

**Phase 5+ (多 Coordinator): 引入 Redis** 覆盖以下场景:

| 场景 | Phase 1-4 方案 (minimal) | Phase 5+ 方案 (standard) | 能力域 |
|------|---------------------------|---------------------------|--------|
| LLM 优先级调度 | WFQ 内存 (单节点) | Redis Streams 全局队列 | `TaskQueue` |
| Pipeline 事件扇出 | goroutine channel | Redis Pub/Sub → 多消费者 | `EventBus` |
| Gate 通知可靠送达 | HTTP 直连 + goroutine 重试 | Redis Streams 持久化 + 死信 | `Notifier` |
| 操作补偿重试 | goroutine 循环 | 死信队列 + 后台消费者 | `TaskQueue` |

**Phase 8+**: 如果运维环境已有 RabbitMQ/Kafka，可替换 Redis Streams，对接已有基础设施优先于自建。

---

## 三、A层: Pipeline 引擎

### 3.1 阶段定义 (5 阶段 + 验证)

```
L1 (原子变更): 澄清 → 实现 → 测试 → 部署 → 验证
L2 (简单修改): 澄清 → 实现 → 测试 → 部署 → 验证
L3 (功能开发): 澄清 → 拆解 → 实现 → 测试 → 部署 → 验证
L4 (架构变更): 澄清 → 拆解 → 实现 → 测试 → 部署 → 验证  (+ 架构评审 Gate)

**Deploy 阶段内部子步骤** `[Phase 3]`:
```
pre-apply dry-run → apply → post-apply verify → rollback on failure
  模拟部署        实际部署   健康检查+冒烟测试    回退到上一版本
```
回滚触发: verify 失败或健康检查不通过 → 自动 rollback + 通知 PM。
借鉴 Plandex `_apply.sh` 的 dry-run→apply→verify→rollback 模式。
```

**复杂度判定**: Agent 在需求澄清阶段输出 `{level: L1-L4, reasoning, estimated_files, estimated_modules}`，PM 可手动调整。

### 3.2 DAG 回溯机制

```
代码生成 → 发现模块定位遗漏 → 发起回溯请求 (目标 + 原因 + 证据)
  → Gate 审批者收到通知 → 批准
  → Pipeline 状态回退到目标阶段，保留部分产物
  → 目标阶段 Agent 收到「上次输出 + 新证据」→ 修正输出
  → 下游阶段继续执行

回溯上限: 同一 Pipeline 最多 3 次 → 超限升级为人工介入
```

### 3.3 Gate 审批模型

- **异步 Gate** — Pipeline 不占资源等审批，释放 Agent 到其他任务
- **草稿预执行** — 低风险阶段可预先执行下游 (标记为草稿)，审批通过转正式
- **超时自动流转** — 仅 L1/L2 的非关键 Gate，L3/L4 必须人工
- **验证阶段永不自动关闭** — 通知升级: PM → PM Lead → 项目负责人

**Gate Hook 拦截器** `[Phase 5]`: Gate 节点支持 `[]GateHook` (pre/post 拦截器)，复用现有 Gate 值对象无需新机制:
```go
type GateHook interface {
    PreApprove(ctx, pipeline, stage) error   // 审批前: LicenseChecker, SecurityScan
    PostApprove(ctx, pipeline, stage) error  // 审批后: AuditLogger, NotificationFanout
}
// 借鉴 DeerFlow Middleware 链模式 + Claude Code preToolUse/postToolUse hooks
```

### 3.4 Pipeline 高可用

- 状态持久化到 Postgres (pipeline_state 表)
- 协调层重启 → 扫描 `WHERE status='running'` → 从断点恢复
- 阶段内检查点 (双触发机制):
  ```
  自动检查点:  每 10 轮 Agent 对话自动保存 (~50KB)
  即时检查点:  chat.pause 触发 → 当前轮立即保存 (不等 10 轮)
  
  恢复: crash → 回到最新的检查点 (自动或即时)
        暂停 → resume 从即时检查点继续
  
  窗口: 最坏丢失 < 10 轮 (自动) / 0 轮 (即时暂停)
        即时检查点写入内存缓冲 → 异步 flush PG
  ```
- CSP Channel WAL: 跨 Agent 消息先写 WAL → 再投递 → crash 后回放去重
- 背压保护: 通道满时 → 消息写入 WAL 暂存 (非丢弃) → 等待消费 → 消费后从 WAL 移除。WAL 自身 CRC32 校验

### 3.5 文件锁

文件级并发锁: `{path, operation: write|read_only, lock_type: exclusive|shared}`
- write 锁: 同文件同时仅 1 Pipeline
- 死锁检测: 发现环 → 通知双方 PM + 自动解锁较晚者
- 超时释放: `estimated_duration * 2` → 自动释放

### 3.6 灰度发布

Agent Prompt / Tool 配置变更:
```yaml
canary:
  target: code-generate.v2
  percentage: 20
  projects: [proj-A]
  duration: 24h
  rollback_on:
    code_rejection_increase: 15%
```

### 3.7 负载丢弃

```
容量模型:
  C = min(
    goroutine_pool.available,
    sandbox_pool.warm_count,
    llm_queue.queue_depth < threshold,
    postgres.idle_connections > 5
  )

水位线:
  C > 30%:  NORMAL   — 全部接受
  C 10-30%: WARNING  — 仅 P0/P1/P2, P3 返回 429
  C < 10%:  CRITICAL — 仅 P0, 其余 429 + retry_after

429 响应: {retry_after_seconds, message}
  客户端: 自动倒计时 → 重新提交按钮 → 连续 3 次 429 → 建议闲时重试
```

### 3.8 Artifact Store

```
热数据 (Postgres):  pipeline_state, gate_events, audit_log
温数据 (MinIO/S3):  方案文档, 模块映射, 测试报告, Diff, 部署日志
冷归档 (MinIO tier): 超过保留期的完整快照 (.tar.gz)

保留策略:
  活跃 Pipeline:     全保留
  已完成:            90 天
  已驳回/废弃:       30 天
  冷归档:            1 年
  定时清理:          每天凌晨扫描 + 释放 worktree/sandbox
```

### 3.10 子 Pipeline 分支模型 (Git 分支 + CI 混合)

```
main
  ├── of-pipeline-42 (主: 新增标签过滤)
  │     ├── of-pipeline-42-1 (子: 修复 router.ts 校验缺失)
  │     │     └── CI: lint → test → 合入 of-pipeline-42
  │     └── of-pipeline-42-2 (子: 调整 article.ts 类型定义)
  └── of-pipeline-51 (主: 新增用户收藏)
```

Agent 在执行中发现额外问题 → 从当前分支创建子分支 (继承父 Pipeline 上下文)，独立 CI，合入时 rebase 父分支。子 Pipeline 生命周期通知:

| 事件 | 通知 | Gate |
|------|------|------|
| 创建子分支 | 通知父 Pipeline PM | 可选: 需 PM 确认 |
| 子分支请求合入 | 通知父 Pipeline 审批人 | 可选: 需审批人确认 (默认需要) |
| 合入冲突 (自动解决失败) | 升级通知审批人 + PM | **硬性人工确认** |

配置: `branch_policy.yaml` per project (auto_create_child, child_require_approval, max_child_pipelines)

### 3.11 二维 Pipeline 归属模型

Pipeline 不只属于项目，还属于受影响的模块（模块挂在开发团队下）:

```
Pipeline #42 归属:
  一维: proj-A (Conduit)
  二维: [frontend (前端组), backend-api (后端组)]
```

**模块注册表** (`module-ownership.yaml`): paths + team + reviewers + fallback_reviewer。自动路由: 受影响模块 → 对应团队的所有 reviewer 加入审批列表；知情者（间接影响模块）只通知不阻塞。

**审批人变更**: LDAP 同步检测离职 → 自动移除 + 切换 fallback；飞书/Outlook OOO 集成 → 自动跳过；>48h 无响应 → 自动绕过 + 审计记录。

### 3.12 Pipeline 回顾与跨 Pipeline 总结

**单 Pipeline 回顾**: 完成后 Agent 自动生成回顾报告（指标: 耗时/对话轮次/Token/驳回次数 + 经验沉淀: 做对了什么/可改进什么 + 知识回写更新清单），推送 PM + 开发负责人，存储 90 天。

**跨 Pipeline 总结**: 每周自动触发（或 N 条后），跨 Pipeline 分析驳回原因频次、知识有效模式、新趋势，自动执行晋升/淘汰/新建 A/B 实验，需人工裁决的问题推送给对应角色。

### 3.13 审批收件箱

```
统一审批收件箱 (跨项目聚合):
  Pipeline #42 │ proj-A │ Tags.tsx, ArticleList.tsx │ ⏳ 待审批 │ 主审
  Pipeline #44 │ proj-A │ article.ts               │ ⏳ 待审批 │ 主审
  Pipeline #51 │ proj-B │ comment.ts               │ ⏳ 需知晓 │ 知情者
```

批量审查: 同项目小变更可批处理。审核负载均衡: 多人有审批权限 → 先认领锁定，超时释放。

---

### 3.14 失败分类归因

Pipeline 失败时自动分类，避免学到噪声:

```
失败分类:
  level 0: Agent 执行异常 → 自动重试 → 仍失败 → Gate 阻塞通知人工
    ├── MODEL_HALLUCINATION    Agent 编造了不存在的 API/文件
    ├── PROMPT_WEAKNESS        提示不够精确，Agent 理解偏差
    ├── DEPENDENCY_CONFLICT    npm/pip 依赖版本冲突
    ├── SANDBOX_TIMEOUT        测试运行超时 (可能是死循环)
    ├── REPO_BUG               Conduit 仓库本身有 bug
    ├── CONTEXT_OVERFLOW       上下文溢出导致输出截断
    ├── TOKEN_QUOTA_EXCEEDED   Token 配额超限
    └── UNKNOWN                人工分类后回写规则

归因流程:
  失败 → 提取特征 (错误日志/失败阶段/模型/涉及文件/prompt版本)
  → 规则引擎 + 嵌入匹配 → 输出分类 + 建议修复方向
  → 写入 trajectories.jsonl (L3 自学习)
  → 人工确认/修正分类 → 回写规则引擎
```

### 3.15 多层自动错误恢复链 `[Phase 2 — v4 新增]`

> 采纳自 Claude Code 源码分析: 分类之后怎么做? 先走自动恢复链, 全失败再升级人工。

```
恢复链 (依次尝试, 任一成功即停止):

  Layer 1: TRANSIENT — 瞬时故障, 自动重试
    触发: API_TIMEOUT / RATE_LIMITED / OVERLOADED
    动作: 指数退避重试 (1s→2s→4s, max 30s, 最多 3 次)
    成功: 继续执行 → 不通知人工
    失败: 进入 Layer 2

  Layer 2: DEGRADABLE — 可降级恢复
    触发: CONTEXT_OVERFLOW / TOKEN_QUOTA_EXCEEDED
    动作:
      CONTEXT_OVERFLOW → 压缩上下文 (保留最近 5 轮 + 需求摘要) → 重试
      TOKEN_QUOTA_EXCEEDED → 降级模型 (Opus→Sonnet, Sonnet→DeepSeek) → 重试
    成功: 继续执行 → 通知 PM (信息: "已自动降级, 质量可能略降")
    失败: 进入 Layer 3

  Layer 3: RECOVERABLE — 需 Agent 自修复
    触发: MODEL_HALLUCINATION / PROMPT_WEAKNESS / DEPENDENCY_CONFLICT
    动作:
      HALLUCINATION → Agent 回退到仓库现有代码作为参考 → 重新生成
      PROMPT_WEAKNESS → Agent 自检需求 → 向 PM 发起澄清对话
      DEPENDENCY_CONFLICT → Agent 锁定版本 → 重新生成
    成功: 继续执行 → 记录反模式到自学习
    失败: → 升级人工 Gate

  Layer 4: FATAL — 不可自动恢复
    触发: SANDBOX_TIMEOUT(3次后) / REPO_BUG / UNKNOWN
    动作: 保存完整上下文 → Pipeline 暂停 → 通知人工 + Debug Trace 自动开启

恢复链实现 (~100行 Go):
  位置: internal/agent/domain/error_recovery.go
  模式: Chain of Responsibility
```

---

## 四、B层: Agent Swarm 运行时

### 4.1 Go 协调层

- **Agent Coordinator**: goroutine = agent 实例，select 多路复用 channel
- **CSP Channel**: buffer 固定 + 背压传播，下游慢 → 上游自动节流
- **Goroutine Pool** (ants): 全局上限 ~8000，按 Pipeline L1-L4 分级配额
- **进程守护**: `os/exec` + `cmd.Wait()` 自动重启 Node.js IO 层
- **Prompt 构建系统** (§4.2b): L1 static.xml → L2 项目融合 → L4 对话摘要, 三层简化架构。SystemPrompt 通过 proto → port → router → adapter 穿透至 LLM Provider

### 4.2 Query Engine (对话生命周期管理) `[Phase 1.5 — v5 spec 闭环, 评分 9.5/10]`

> 5 个独立 spec (QE-01~05) 合并，P0/P1 全闭环。采纳自 Claude Code 源码分析 + Gate 挂起评审 + QE Specs 审核。

Query Engine 管理单次对话(Pipeline 内一个 Stage)的完整生命周期。**状态机 (6 态)**:

```
IDLE → AWAITING_LLM → AWAITING_TOOLS → AWAITING_GATE → AWAITING_USER → IDLE
```

#### 4.2.1 QueryEngineConfig (QE-01)

```go
type QueryEngineConfig struct {
    MaxToolRounds      int                 // 默认 10
    TokenBudget        int                 // 默认 200000
    CheckpointInterval int                 // 默认 10 (0=禁用)
    GateTimeout        time.Duration       // 默认 5min
    ConflictResolution ConflictResolution  // abort(默认) | override
    ToolErrorPolicy    ToolErrorPolicy
    HistoryCompression HistoryCompressionConfig
    EnableMetrics      bool                // 默认 false (Phase 7)
    EnableTracing      bool                // 默认 false (Phase 8)
    AutoApprovalRules  []AutoApprovalRule  // P2: Phase 6+
}

type ConflictResolution string
const (
    ConflictAbort    ConflictResolution = "abort"
    ConflictOverride ConflictResolution = "override"
)

type ToolErrorPolicy struct {
    MaxRetries    int           // 默认 3
    RetryDelay    time.Duration // 默认 1s
    BackoffFactor float64       // 默认 2.0
    OnFailure     string        // "notify_llm"(默认) | "skip" | "abort"
}

type HistoryCompressionConfig struct {
    Enabled          bool    // Phase 1.5 禁用，Phase 3 开启
    TriggerThreshold float64 // 默认 0.8
    KeepRecentRounds int     // 默认 10
    MaxSummaryTokens int     // 默认 2000
}

var DefaultQueryEngineConfig = QueryEngineConfig{
    MaxToolRounds: 10, TokenBudget: 200000, CheckpointInterval: 10,
    GateTimeout: 5 * time.Minute, ConflictResolution: ConflictAbort,
    ToolErrorPolicy: ToolErrorPolicy{MaxRetries: 3, RetryDelay: 1 * time.Second, BackoffFactor: 2.0, OnFailure: "notify_llm"},
    HistoryCompression: HistoryCompressionConfig{TriggerThreshold: 0.8, KeepRecentRounds: 10, MaxSummaryTokens: 2000},
}
```

#### 4.2.2 核心数据结构 (QE-01 + QE-04)

```go
type QueryEngine struct {
    config       QueryEngineConfig
    llmClient    port.LLMRouterClient
    toolRegistry port.ToolRegistry
    gateRepo     GateRepository

    mu           sync.Mutex
    state        QueryState
    history      []port.Message
    tokenUsed    int
    pipelineID   string
    model        string
    currentStage string
    resumeRound  int                    // Resume 恢复轮数 (持久化)
    pendingGate  *PendingGateState
    resumeOnce   sync.Once
    cancel       context.CancelFunc
    toolCallRecords []ToolCallRecord    // 结构化文件记录

    logger       Logger
    metrics      MetricsCollector
}

// ── 提交结果 ──
type SubmitResult struct {
    Status      SubmitStatus     // "completed" | "pending_gate" | "error"
    Reply       string
    ToolCalls   []ToolCallRecord
    TokenUsed   int
    Checkpoint  *Checkpoint
    GateRequest *GateRequest
    Error       string
}

type SubmitStatus string
const (
    SubmitCompleted   SubmitStatus = "completed"
    SubmitPendingGate SubmitStatus = "pending_gate"
    SubmitError       SubmitStatus = "error"
)

// ── Gate 类型 ──
type GateRequest struct {
    PipelineID   string; Stage string; ToolName string; Reason string
    ChangedFiles []string; ArtifactHash string
    CreatedAt    time.Time; ExpiresAt time.Time
    Approvers    []string               // P2: Phase 6+
}

type GateResult struct {
    Approved bool; LineComments []LineComment; SummaryFeedback string
    ApprovedBy string; ApprovedAt time.Time
}

type PendingGateState struct {
    PendingID    string; GateRequest GateRequest; ToolCall ToolCallParsed
    History      []port.Message; TokenUsed int; RoundCount int
    ArtifactHash string; CreatedAt time.Time; ExpiresAt time.Time
    Status       string   // "pending" | "approved" | "rejected" | "timeout" | "expired"
}

// ── 工具类型 ──
type ToolCallParsed struct {
    ID string; Name string; Args map[string]interface{}; IsReadOnly bool
}

type ToolCallGroup struct {
    Tools    []ToolCallParsed; Parallel bool
}

type ToolResult struct {
    Output        interface{}; Err error; GateRequired bool
    ModifiedFiles []string                // 结构化文件记录 (替代文本解析)
}

type ToolCallRecord struct {
    Name string; Args map[string]interface{}
    Output interface{}; Err error; ModifiedFiles []string
}
```

#### 4.2.3 核心 API + runLLMLoop (QE-02, P0 锁修复)

```go
func (qe *QueryEngine) Submit(ctx context.Context, input string) (*SubmitResult, error)
func (qe *QueryEngine) Resume(ctx context.Context, gateResult GateResult) (*SubmitResult, error)
func (qe *QueryEngine) Cancel()

// runLLMLoop — P0: 锁仅在构建请求/处理结果时持有，LLM 调用期间释放
func (qe *QueryEngine) runLLMLoop(ctx context.Context) (*SubmitResult, error) {
    for round := qe.resumeRound; round < qe.config.MaxToolRounds; round++ {
        // 构建请求 (加锁)
        qe.mu.Lock()
        req := port.ChatRequest{Messages: qe.history, Config: qe.buildLLMConfig()}
        qe.mu.Unlock()                                          // ← 释放锁

        // LLM 调用 (无锁, 可能阻塞数十秒)
        qe.state = QueryStateAwaitingLLM
        response, err := qe.llmClient.ChatStream(ctx, req)
        if err != nil { return &SubmitResult{Status: SubmitError, Error: err.Error()}, nil }

        // 处理结果 (重新加锁)
        qe.mu.Lock()
        parsed := qe.parseStreamResponse(response)
        qe.tokenUsed += parsed.TokenUsed                        // ← P0 Token 累加
        if qe.tokenUsed >= qe.config.TokenBudget { qe.mu.Unlock(); return tokenExceededResult, nil }

        if len(parsed.ToolCalls) == 0 { /* 完成 */ qe.mu.Unlock(); break }

        // 权限预检 + 工具执行 (在锁内完成分类和 Gate 判断)
        gateTool := qe.executeToolGroupWithGateCheck(parsed.ToolCalls)
        qe.mu.Unlock()

        if gateTool != nil {
            return qe.handleGatePause(ctx, *gateTool), nil       // Gate 挂起
        }
    }
    return &SubmitResult{Status: SubmitCompleted, TokenUsed: qe.tokenUsed}, nil
}
```

#### 4.2.4 Gate 挂起与恢复 (QE-02, QE-04, P0 修复)

```go
// handleGatePause — 持久化 + 返回 pending_gate
func (qe *QueryEngine) handleGatePause(ctx context.Context, tc ToolCallParsed) *SubmitResult {
    now := time.Now()
    qe.pendingGate = &PendingGateState{
        PendingID: fmt.Sprintf("gate-%s-%d", qe.pipelineID, now.UnixNano()),
        GateRequest: GateRequest{
            PipelineID: qe.pipelineID, Stage: qe.currentStage, ToolName: tc.Name,
            Reason: tc.Name + " requires Gate approval",
            ChangedFiles: qe.collectChangedFiles(), ArtifactHash: qe.calcHistoryHash(),
            CreatedAt: now, ExpiresAt: now.Add(qe.config.GateTimeout),
        },
        ToolCall: tc, History: qe.snapshotHistory(),
        TokenUsed: qe.tokenUsed, RoundCount: qe.resumeRound,  // P0: 持久化轮数
        ArtifactHash: qe.calcHistoryHash(), CreatedAt: now, ExpiresAt: now.Add(qe.config.GateTimeout),
        Status: "pending",
    }
    if qe.gateRepo != nil { qe.gateRepo.Create(context.Background(), qe.pendingGate) }
    qe.state = QueryStateAwaitingGate
    return &SubmitResult{Status: SubmitPendingGate, GateRequest: &qe.pendingGate.GateRequest}
}

// resumeApproved — P0 修复: 先执行暂停的工具, 再继续 LLM 推理
func (qe *QueryEngine) resumeApproved(ctx context.Context, gr GateResult) (*SubmitResult, error) {
    result := qe.executeToolWithPolicy(ctx, qe.pendingGate.ToolCall)
    qe.history = append(qe.history, port.Message{Role: "tool", Content: qe.formatToolResult(qe.pendingGate.ToolCall.Name, result)})
    qe.history = append(qe.history, port.Message{Role: "system", Content: "Gate 审批通过，继续执行。"})
    qe.resumeRound = qe.pendingGate.RoundCount
    qe.state = QueryStateAwaitingLLM
    return qe.runLLMLoop(ctx)
}

// resumeRejected — 反馈注入 + needs_revision 标记
func (qe *QueryEngine) resumeRejected(ctx context.Context, gr GateResult) (*SubmitResult, error) {
    qe.history = append(qe.history, port.Message{Role: "system", Content: qe.formatGateFeedback(gr)})
    if len(gr.LineComments) > 0 {
        files := extractFilesFromComments(gr.LineComments)
        qe.history = append(qe.history, port.Message{Role: "system",
            Content: fmt.Sprintf("[needs_revision] 请仅修改以下文件: %s", strings.Join(files, ", "))})
    }
    qe.state = QueryStateAwaitingLLM
    return qe.runLLMLoop(ctx)
}
```

#### 4.2.5 GateRepository 接口 + PG 实现 (QE-03)

```go
// internal/agent/port/gate_repository.go
type GateRepository interface {
    Create(ctx context.Context, state *PendingGateState) error
    Get(ctx context.Context, pendingID string) (*PendingGateState, error)
    UpdateResult(ctx context.Context, pendingID string, result GateResult) error
    UpdateStatus(ctx context.Context, pendingID string, status string) error
    ListPending(ctx context.Context) ([]*PendingGateState, error)
}
```

PG DDL:

```sql
CREATE TABLE gate_request (
    pending_id    VARCHAR(64) PRIMARY KEY,
    pipeline_id   VARCHAR(64) NOT NULL REFERENCES pipeline(id),
    tool_name     VARCHAR(128) NOT NULL, reason TEXT,
    changed_files JSONB, artifact_hash VARCHAR(64) NOT NULL,
    history       JSONB NOT NULL, tool_call JSONB NOT NULL,
    token_used    INT DEFAULT 0, round_count INT DEFAULT 0,
    status        VARCHAR(16) DEFAULT 'pending' CHECK (status IN ('pending','approved','rejected','timeout','expired')),
    created_at    TIMESTAMPTZ DEFAULT NOW(), expires_at TIMESTAMPTZ NOT NULL,
    approved_at   TIMESTAMPTZ, approved_by VARCHAR(320), result JSONB
);
CREATE INDEX idx_gate_request_status ON gate_request(status);
CREATE INDEX idx_gate_request_expires ON gate_request(expires_at);
```

恢复: `NewQueryEngine()` 启动时 `recoverPendingGates()` — 扫描 pending, 已过期标记 expired, 恢复第一个到 `pendingGate`。

#### 4.2.6 错误映射 → §3.15 (QE-05)

```go
// FailureCode + ClassifyAndRecover 完整定义 (P0 闭环)
type FailureCode string
const (
    FailAPITimeout FailureCode = "API_TIMEOUT"; FailRateLimited = "RATE_LIMITED"
    FailOverloaded = "OVERLOADED"; FailContextOverflow = "CONTEXT_OVERFLOW"
    FailTokenQuotaExceeded = "TOKEN_QUOTA_EXCEEDED"
    FailModelHallucination = "MODEL_HALLUCINATION"; FailPromptWeakness = "PROMPT_WEAKNESS"
    FailDependencyConflict = "DEPENDENCY_CONFLICT"; FailSandboxTimeout = "SANDBOX_TIMEOUT"
    FailRepoBug = "REPO_BUG"; FailUnknown = "UNKNOWN"
)

func MapToolErrorToFailureCode(err error) FailureCode {
    msg := err.Error()
    switch {
    case reTimeout.MatchString(msg):    return FailAPITimeout
    case reRateLimit.MatchString(msg):  return FailRateLimited
    case reOverloaded.MatchString(msg): return FailOverloaded
    case reContextLen.MatchString(msg): return FailContextOverflow
    case reQuota.MatchString(msg):      return FailTokenQuotaExceeded
    case reNotFound.MatchString(msg):   return FailModelHallucination
    case reDependency.MatchString(msg): return FailDependencyConflict
    case rePermission.MatchString(msg): return FailRepoBug
    case reSandbox.MatchString(msg):    return FailSandboxTimeout
    default:                            return FailUnknown
    }
}

func ClassifyAndRecover(code FailureCode, attempt int) RecoveryResult {
    switch code {
    case FailAPITimeout, FailRateLimited, FailOverloaded:
        if attempt < 3 { return RecoveryResult{ActionRetry, fmt.Sprintf("attempt %d/3", attempt+1)} }
        return RecoveryResult{ActionEscalate, "TRANSIENT exhausted"}
    case FailContextOverflow:  return RecoveryResult{ActionCompress, "compressing context"}
    case FailTokenQuotaExceeded: return RecoveryResult{ActionDowngradeModel, "switching model"}
    case FailModelHallucination, FailDependencyConflict: return RecoveryResult{ActionSelfRepair, "auto-repair"}
    case FailPromptWeakness: return RecoveryResult{ActionClarify, "asking PM"}
    default: return RecoveryResult{ActionEscalate, fmt.Sprintf("FATAL: %s", code)}
    }
}
```

**错误匹配用正则** (P2): `reTimeout`, `reRateLimit`, `reOverloaded`, `reContextLen`, `reQuota`, `reNotFound`, `reDependency`, `rePermission`, `reSandbox` 共 9 个 `regexp.MustCompile`。

#### 4.2.7 Gate 超时 + 冲突检测 + 反馈格式化 (QE-02, QE-04)

- **超时:** goroutine 每 10s 检查 `pendingGate.ExpiresAt`, 超时自动拒绝 (状态切 AWAITING_LLM, system message 注入, status=timeout 持久化)
- **冲突:** Resume 时 calcHistoryHash() 比对, 不匹配 → abort(默认) 或 override(dev)
- **反馈:** `formatGateFeedback()` → 结构化 Markdown (总结 + 按文件分组的行级评论 + `[needs_revision]` 文件列表)

#### 4.2.8 文件清单

```
internal/agent/domain/
  query_engine.go           # QueryEngine 核心 (~200行, 6 态 + PromptBuilder 集成)
  query_engine_config.go    # QueryEngineConfig + 默认值
  query_engine_test.go      # Table-driven 测试
  prompt_builder.go         # PromptBuilder 核心 (~350行, L1/L2/L4 三层架构)
  prompt_builder_test.go    # PromptBuilder 测试
  stage_templates.go        # 6 阶段 × 4 复杂度服务器默认模板
  tools_stages.go           # StageToolMap + InjectTools (Phase 7→ToolRegistry)
  knowledge_querier.go      # KnowledgeQuerier — L2 附属, 对接 LearningEngine
  tool_call_parsed.go       # ToolCallParsed + ToolCallGroup + ToolResult
  error_mapping.go          # FailureCode + MapToolErrorToFailureCode + ClassifyAndRecover
  coordinator.go            # AgentCoordinator — goroutine pool 管理

internal/agent/port/
  gate_repository.go        # GateRepository 接口
  llm_client.go             # LLMRouterClient + ChatRequest (含 SystemPrompt)

internal/agent/adapter/
  pg_gate_repository.go     # PG 实现
  grpc_llm_client.go        # ConnectRPC → Node.js (传递 SystemPrompt)

config/prompts/
  static.xml                # L1 不可覆盖 — 身份/安全/代码规范

migrations/
  003_gate_request.up.sql   # gate_request DDL
```

### 4.2b Prompt 构建系统 `[Phase 5d — v2 简化]`

> 架构: L1 (static.xml 不可覆盖) → L2 (项目融合) → L4 (对话摘要)。L3 退化入 L2（无独立数据源）。注入器精简为 KnowledgeQuerier（L2 附属）+ 独立 tools_stages.go。

#### 4.2b.1 三层架构

```
PromptBuilder.Build(ctx, BuildRequest)
  │
  ├── 1. L1 静态安全层 (config/prompts/static.xml)
  │       启动加载，内存缓存，永不重读
  │       身份声明 + 安全规则 + 代码规范
  │       拼接在最末尾，不可被项目模板覆盖
  │
  ├── 2. L2Builder.Build(ctx, L2Request)
  │       ├── ProjectPrefs (of-prefs.yaml, mtime 热重载, Phase 6 实现)
  │       ├── stageInstruction(stage, level)
  │       │     → of-prefs.yaml 项目覆盖(优先)
  │       │     → stage_templates.go 服务器默认模板(兜底)
  │       │     → 通用 fallback 提示(最后)
  │       ├── KnowledgeQuerier.Query(ctx, projectID, userQuery)
  │       │     → 5min TTL 缓存 → LearningEngine (nil 时静默返回 "")
  │       └── Metadata (pipeline_id, project_id, current_time, 回溯/子Pipeline)
  │
  ├── 3. Tool 注入 (tools_stages.go)
  │       InjectTools(stage, permissionMode)
  │       → StageToolMap[stage] → plan模式过滤 → tool描述文本 + []ToolDefinition
  │       Phase 7: 替换为 ToolRegistry.Search()
  │
  └── 4. L4 对话摘要 (纯函数 buildL4Summary)
        最近 5 轮 (10 条消息) → <conversation_summary> XML
        注入 SystemPrompt，不影响 Messages 数组
        Messages 完全由 QueryEngine 独立管理
```

#### 4.2b.2 核心类型

```go
// PromptBuilder — 简化后核心
type PromptBuilder struct {
    l1Content string          // static.xml 内存缓存，启动加载
    l2Builder *L2Builder
    metrics   *PromptMetrics
    mu        sync.RWMutex
}

// L2Builder 负责 L2 层全部内容组装
type L2Builder struct {
    prefs     *ProjectPrefsLoader  // of-prefs.yaml 热重载
    knowledge *KnowledgeQuerier    // L2 附属，可为 nil
    mu        sync.RWMutex
}

// L2Request L2 层输入
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

// BuildRequest 完整请求
type BuildRequest struct {
    PipelineID, ProjectID, Stage, StageLevel string
    PermissionMode, UserRole, UserMessage    string
    ConversationHistory                      []Message
    BacktrackReason, BacktrackTarget         string
    ParentPipelineID                         string
}

// Prompt 构建输出 — System + Tools + Token，不含 Messages
type Prompt struct {
    System     string
    Tools      []ToolDefinition
    TokenUsage *TokenUsage
}
```

#### 4.2b.3 PromptBuilder 构造与核心方法

```go
// NewPromptBuilder 创建 PromptBuilder
// l1Path: config/prompts/static.xml 路径 (服务启动校验，缺失 fast-fail)
// knowledgeQuerier: 可为 nil (Phase 7 前知识查询静默返回空)
func NewPromptBuilder(l1Path string, knowledgeQuerier *KnowledgeQuerier) (*PromptBuilder, error)

// Build 执行完整构建链: L2 → Tools → L4 → L1 + sanitize + token估算
func (pb *PromptBuilder) Build(ctx context.Context, req *BuildRequest) (*Prompt, error)

// GetMetrics 返回构建指标 (Stage/Complexity/Permission/BuildDuration/Token 统计)
func (pb *PromptBuilder) GetMetrics() *PromptMetrics
```

#### 4.2b.4 tools_stages.go — 阶段工具映射

```go
// ToolDefinition 工具定义 (字段名与 port.ToolInfo 对齐，方便 Phase 7 迁移)
type ToolDefinition struct {
    Name        string
    Description string
    InputSchema map[string]interface{}
    ReadOnly    bool
}

// StageToolMap 硬编码 6 阶段工具列表 (Phase 7 → ToolRegistry.Search)
var StageToolMap = map[string][]ToolDefinition{
    "clarify":   {read_file, search_content, analyze_topology, lsp_symbols},
    "decompose": {read_file, search_content, analyze_topology, lsp_references},
    "implement": {acquire_file_lock, release_file_lock, read_file, edit_file, write_file, bash, lsp_*},
    "test":      {read_file, edit_file, bash, search_content},
    "deploy":    {bash, read_file, manage_sandbox},
    "verify":    {read_file, bash, write_knowledge_delta},
}

// PermissionFilter plan 模式下允许的工具白名单 (13 个只读工具)
var PermissionFilter = map[string][]string{"plan": {...}}

// InjectTools 返回 (工具描述 XML 文本, []ToolDefinition)
func InjectTools(stage, permissionMode string) (string, []ToolDefinition)
```

#### 4.2b.5 L1 不可覆盖规则

`config/prompts/static.xml` 在 `Build()` 拼接链最末尾。项目 `of-prefs.yaml` 不能覆盖其中的安全规则：

```xml
<system_prompt>
  <identity>
    <role>You are OpenForge, an AI-driven full-stack development agent.</role>
    <mission>Execute software engineering tasks across the complete lifecycle.</mission>
  </identity>
  <security>
    <audit>All operations are audited (WORM).</audit>
    <gate>Never bypass the Gate approval system.</gate>
    <license>Never generate GPL/AGPL code.</license>
  </security>
  <code_conventions>
    <convention>NO COMMENTS unless asked</convention>
    <convention>Follow existing code style</convention>
    <convention>Prefer editing existing files over creating new ones</convention>
    <convention>Never expose or log secrets/keys</convention>
  </code_conventions>
</system_prompt>
```

#### 4.2b.6 SystemPrompt 穿透链路

```
QueryEngine.SubmitMessage()
  → PromptBuilder.Build(ctx, buildReq)
    → SystemPrompt = L2 + Tools + L4 + L1
  → port.ChatRequest.SystemPrompt = prompt.System
    → llm.Router.Chat/ChatStream → internal ChatRequest.SystemPrompt
      → adapter.LLMClient.toProtoRequest() → proto LLMChatRequest.system_prompt
        → Node.js LLMRouterService.Chat → ChatRequest.systemPrompt → Provider
```

**涉及文件**: `query_engine.go` → `prompt_builder.go` → `port/llm_client.go` → `router.go` → `grpc_llm_client.go` → `llm.proto` → `nodejs-io`

#### 4.2b.7 KnowledgeQuerier (L2 附属)

```go
// KnowledgeQuerier 对接 LearningEngine，被 L2Builder 调用
type KnowledgeQuerier struct {
    learningEngine LearningEngine  // nil 时 Query() 静默返回 ""
    embeddingIndex EmbeddingIndex
    cache          sync.Map        // 5min TTL
}
// Query 并行查询偏好(QueryKnowledge) + 轨迹(MatchTrajectory)
func (kq *KnowledgeQuerier) Query(ctx context.Context, projectID, query string) (string, error)
```

**与旧设计的差异**: `knowledge_injector.go` 已删除。原 Tool/Context/Security 三个独立注入器 (L369-619) 全部移除。LearningEngine 接口的三个方法 (`QueryKnowledge`/`MatchTrajectory`/`WriteKnowledge`) 在 domain 内部自定，`port.LearningEngineClient` 的方法扩展延至 Phase 7。

#### 4.2b.8 Phase 7 预留扩展点

| 当前实现 | Phase 7 替换 |
|---------|-------------|
| `tools_stages.go` 硬编码 `StageToolMap` | 调用 `ToolRegistry.Search(ctx, stage, topK)` |
| `KnowledgeQuerier.learningEngine == nil` → 返回 "" | 对接真实 pgvector 嵌入索引 |
| `PermissionFilter` 白名单硬编码 | 调 `Tool.IsReadOnly()` 接口动态判定 |
| `ProjectPrefsLoader.Get()` 返回 "" | 读 `of-prefs.yaml` + mtime 热重载 |
| 服务器模板 Go string 常量 | `templates/stages/*.xml` + embed.FS 外部化 |

#### 4.2b.9 文件清单 (Phase 5d 变更)

```
新建:
  config/prompts/static.xml                              ← L1 不可覆盖
  internal/agent/domain/tools_stages.go                  ← StageToolMap + InjectTools
  internal/agent/domain/knowledge_querier.go             ← KnowledgeQuerier (替代 knowledge_injector.go)

重写:
  internal/agent/domain/prompt_builder.go                ← ~900→~350行, 4层→3层简化

删除:
  internal/agent/domain/knowledge_injector.go            ← 13 类型/11 方法移入 knowledge_querier.go

修改:
  internal/agent/domain/query_engine.go                  ← +promptBuilder +PipelineContext
  internal/agent/domain/query_engine_test.go             ← 对齐新 API
  internal/agent/port/llm_client.go                       ← ChatRequest.SystemPrompt
  internal/llm/router.go                                  ← SystemPrompt 穿透
  internal/agent/adapter/grpc_llm_client.go               ← toProtoRequest 传递
  internal/shared/profile/bootstrap.go                    ← OpenForge.PromptBuilder 字段 + 初始化
  internal/server/ws_handler.go                           ← PromptBuilder + PipelineContext 注入
  proto/agent/v1/llm.proto                                ← system_prompt 字段
  nodejs-io/src/kernel/interfaces.ts                      ← ChatRequest.systemPrompt 字段
```

### 4.3 Tool 接口标准化 `[Phase 1.5 — v4 新增]`

> 采纳自 Claude Code: 当前 OpenForge 工具仅在 Proto 层定义 RPC, Go 侧无 Tool 泛型接口, 每加一个 Tool 都是一次性代码。

```go
// internal/agent/port/tool.go

// Tool 泛型接口 — 所有工具的基础契约
type Tool[Input any, Output any] interface {
    Name()             string
    Description()      string
    InputSchema()      []byte                         // JSON Schema
    IsConcurrencySafe() bool                           // true=可并行, false=必须串行
    IsReadOnly()       bool                           // true=只读(plan模式自动放行)
    Execute(ctx context.Context, input Input) (Output, error)
}

// StreamingTool — 支持流式产出的工具 (如 Bash, Test Runner)
type StreamingTool[Input any, Output any] interface {
    Tool[Input, Output]
    ExecuteStream(ctx context.Context, input Input) (<-chan StreamChunk[Output], error)
}

// ToolRegistry — 工具注册表 (嵌入索引匹配)
type ToolRegistry interface {
    Register(tool Tool[any, any]) error
    Search(ctx context.Context, query string, topK int) ([]ToolMatch, error)
    Get(name string) (Tool[any, any], error)
}

// 工具状态机: QUEUED → EXECUTING → COMPLETED | YIELDED | FAILED
```

**并发规则**: `IsConcurrencySafe()==true` 的工具可并行执行（如 Read, Grep）；`==false` 必须串行（如 Write, Edit, Bash 写入操作）。级联中止: Bash 错误触发兄弟工具中止, 只读工具不受影响。

**Bash 作为 StreamingTool 实例**:

Bash 命令执行通过 `CommandExecutor` 能力域（第 12 个能力域, 见 §10.1.1）实现。`BashTool` 实现 `StreamingTool[BashInput, ExecOutput]`，Profile 决定执行策略:

```go
// internal/tool/bash_tool.go — BashTool 实现 StreamingTool

type BashInput struct {
    Command     string `json:"command"`
    Description string `json:"description"`
    WorkDir     string `json:"work_dir,omitempty"`
    TimeoutMs   int    `json:"timeout_ms,omitempty"`
}

type BashTool struct {
    executor CommandExecutor
}

func (t *BashTool) Name() string            { return "bash" }
func (t *BashTool) IsConcurrencySafe() bool { return false }
func (t *BashTool) IsReadOnly() bool        { return false }

func (t *BashTool) Execute(ctx context.Context, input BashInput) (ExecOutput, error) {
    return t.executor.Execute(ctx, input.Command, ExecOptions{
        WorkDir: input.WorkDir, Timeout: time.Duration(input.TimeoutMs) * time.Millisecond,
    })
}

func (t *BashTool) ExecuteStream(ctx context.Context, input BashInput) (<-chan StreamChunk, error) {
    return t.executor.ExecuteStream(ctx, input.Command, ExecOptions{...})
}
```

**执行路径** (Profile 感知):

| Profile | 实现 | 执行位置 | 安全边界 |
|---------|------|---------|---------|
| **minimal** | `LocalShellExecutor` | 宿主机直接 spawn (/bin/bash -c) | 危险命令硬阻断 + 路径限制 (project root + /tmp) |
| **standard** | `DockerSandboxExecutor` | Docker 容器内 | --read-only + --cap-drop=ALL + cgroup |
| **enterprise** | `DockerSandboxExecutor` + seccomp | Docker + 5 层纵深防护 | 同 standard + seccomp + 网络隔离 |

minimal profile 对标 Claude Code 体验 — `ls`/`grep`/`npm install`/`git status` 等开发工具链直接执行，无 Docker 依赖。危险命令 (`rm -rf`/`sudo`/`dd`/`mkfs`/`curl|bash`) 硬阻断。降级阻断: `enterprise→minimal` / `standard→minimal` → FATAL 拒绝启动。

**Terminal 面板** (复用现有 WebSocket 协议): 只读 Terminal 展示沙箱 stdout/stderr 流 (`terminal.output`)，调试 Terminal (仅 Dev + Tech Lead + 2FA) 接受输入 (`terminal.input`)。所有调试 Terminal 输入记 WORM 审计。

**错误恢复**: Bash 执行失败 → 分类 → 融入 DESIGN.md §3.15 四层恢复链:
- `TRANSIENT` (超时/沙箱不可用) → Layer 1 自动重试
- `DEGRADABLE` (输出超限) → Layer 2 截断
- `RECOVERABLE` (命令不存在) → Layer 3 Agent 自修正
- `FATAL` (权限/危险命令) → Layer 4 升级人工 Gate

### 4.4 权限模式（四级）`[Phase 1.5 — v4 新增]`

> 采纳自 Claude Code 的 default/plan/auto/bypass 四级权限, 适配 OpenForge 的 PM Pipeline + Gate 场景。

```
PermissionMode (按 Pipeline Stage 自动选择, PM 可手动升级):

  ┌────────────┬──────────────────────────────────────────────┐
  │ bypass     │ 管理员特权通道, 全部操作自动允许               │
  │            │ 使用条件: Admin 角色 + 仅紧急回溯/生产修复     │
  │            │ 全程审计 (WORM), 事后强制 Review              │
  ├────────────┼──────────────────────────────────────────────┤
  │ auto       │ 规则引擎自动判定 (只读放行, 写入需 Gate)       │
  │            │ 适用于: L1/L2 原子变更 (typo/文案/配置)       │
  │            │ 文件锁范围内自动允许                           │
  ├────────────┼──────────────────────────────────────────────┤
  │ plan       │ 仅允许只读操作 (文件读取/LSP/Grep/拓扑分析)   │
  │            │ 适用于: 需求澄清阶段 → Agent 分析仓库         │
  │            │ PM 可见操作日志, 不可写代码                    │
  ├────────────┼──────────────────────────────────────────────┤
  │ default    │ 所有操作需 Gate 审批                          │
  │            │ 适用于: L3/L4 功能开发+架构变更               │
  │            │ 写入操作全部阻断, 等待审批人通过                │
  └────────────┴──────────────────────────────────────────────┘

权限判定链 (per tool call):
  1. PermissionMode 判定 → bypass → 直接放行
  2. tool.IsReadOnly() + plan 模式 → 自动放行
  3. auto 规则引擎 → 文件锁/白名单内 → 放行
  4. 触发 Gate 审批 → 审批人确认 → 放行/拒绝
  5. 所有决策记录审计: {who, what, mode, decision, timestamp}
```

**与 RBAC 的关系**: RBAC 控制"谁能审批/谁能创建 Pipeline"，PermissionMode 控制"Agent 单次操作需要什么级别的许可"。两者正交。

### 4.5 LLM Router + 模型注册表 `[Phase 1.5 — v5 新增]`

> **架构决策**: LLM Router 从 Node.js IO 层移至 Go 协调层。理由: (1) Router 的本质是 HTTP 转发 + JSON 翻译，Go `net/http` 零依赖即满足; (2) minimal profile 下 Go 单二进制部署，无需 Node 运行时; (3) 与 Query Engine(Go)、Tool Registry(Go) 进程内调用，无序列化开销。

#### 4.5.1 模型注册表

Router 采用**表驱动**机制。每个模型一条记录，调用方只需传别名（如 `"sonnet"`），Router 从注册表查找实际 ModelID、BaseURL、API Key 引用、能力标记、降级链：

```go
// internal/llm/registry.go

type ModelEntry struct {
    Alias    string   // "sonnet" — 用户/Agent 使用的短名
    Provider string   // "anthropic" | "deepseek" | "openai" | "gemini"

    // 路由字段
    ModelID  string   // "claude-sonnet-4-6-20250514" — 发给 API 的实际 model 名
    BaseURL  string   // "https://api.anthropic.com"

    // API Key 引用 (不存 key 本身, 运行时从 SecretStore 获取)
    KeyRef   string   // "llm/anthropic/api_key"

    // 凭据解析链 (按优先级: env → file → vault → literal)
    CredentialChain []CredentialSource

    FeatureFlags
    RetryConfig
    Thinking   *ThinkingConfig

    // 降级与调度
    Fallback   []string // ["deepseek", "haiku"]
    Priority   int      // 0-3, WFQ 权重 (Phase 7)

    // 成本
    InputPricePer1K  float64
    OutputPricePer1K float64
}

type FeatureFlags struct {
    MessagesAPI      bool  // 支持 /v1/messages → 直通; false → 需要翻译层
    ToolUse          bool
    Thinking         bool
    PromptCaching    bool
    Vision           bool
    Streaming        bool
    StructuredOutput bool
    ContextWindow    int
    MaxOutputTokens  int
}

type ThinkingConfig struct {
    BudgetTokens int    // 默认 4096
    Mode         string // "enabled" | "auto"
}

type CredentialSource struct {
    Type     string // "env" | "file" | "vault" | "literal"
    Location string // "ANTHROPIC_AUTH_TOKEN" | "~/.openforge/credentials.json" | "llm/anthropic/api_key"
}

type RetryConfig struct {
    MaxRetries      int           // 默认 3
    BaseDelay       time.Duration // 默认 1s
    MaxDelay        time.Duration // 默认 30s
    RetryableStatus []int         // {408, 409, 425, 429, 500, 502, 503, 504}
    NonRetryable    []string      // "auth" | "quota"
}
```

**默认注册表** (Phase 1 硬编码，Phase 5 加 YAML 覆盖):

```go
var DefaultRegistry = ModelRegistry{
    // ── Anthropic 原生 (直通 /v1/messages) ──
    "opus": {
        Alias: "opus", Provider: "anthropic",
        ModelID: "claude-opus-4-7-20250514", BaseURL: "https://api.anthropic.com",
        KeyRef: "llm/anthropic/api_key", Priority: 0,
        Fallback: []string{"sonnet", "deepseek"},
        Features: FeatureFlags{MessagesAPI: true, ToolUse: true, Thinking: true,
            PromptCaching: true, Vision: true, Streaming: true,
            StructuredOutput: true, ContextWindow: 200000, MaxOutputTokens: 32000},
        Thinking: &ThinkingConfig{BudgetTokens: 4096, Mode: "enabled"},
        InputPricePer1K: 0.015, OutputPricePer1K: 0.075,
    },
    "sonnet": {
        Alias: "sonnet", Provider: "anthropic",
        ModelID: "claude-sonnet-4-6-20250514", BaseURL: "https://api.anthropic.com",
        KeyRef: "llm/anthropic/api_key", Priority: 1,
        Fallback: []string{"deepseek", "haiku"},
        Features: FeatureFlags{MessagesAPI: true, ToolUse: true, Thinking: true,
            PromptCaching: true, Vision: true, Streaming: true,
            StructuredOutput: true, ContextWindow: 200000, MaxOutputTokens: 32000},
        Thinking: &ThinkingConfig{BudgetTokens: 4096, Mode: "enabled"},
        InputPricePer1K: 0.003, OutputPricePer1K: 0.015,
    },
    "haiku": {
        Alias: "haiku", Provider: "anthropic",
        ModelID: "claude-haiku-4-5-20251001", BaseURL: "https://api.anthropic.com",
        KeyRef: "llm/anthropic/api_key", Priority: 3,
        Fallback: []string{"deepseek"},
        Features: FeatureFlags{MessagesAPI: true, ToolUse: true,
            Streaming: true, ContextWindow: 200000, MaxOutputTokens: 16000},
        InputPricePer1K: 0.0008, OutputPricePer1K: 0.004,
    },

    // ── DeepSeek (原生 /v1/messages, 直通) ──
    "deepseek": {
        Alias: "deepseek", Provider: "deepseek",
        ModelID: "deepseek-v4-pro", BaseURL: "https://api.deepseek.com",
        KeyRef: "llm/deepseek/api_key", Priority: 1,
        Fallback: []string{"deepseek-r1"},
        Features: FeatureFlags{MessagesAPI: true, ToolUse: true,
            Streaming: true, ContextWindow: 131072, MaxOutputTokens: 32000},
        InputPricePer1K: 0.0005, OutputPricePer1K: 0.002,
    },
    "deepseek-r1": {
        Alias: "deepseek-r1", Provider: "deepseek",
        ModelID: "deepseek-reasoner", BaseURL: "https://api.deepseek.com",
        KeyRef: "llm/deepseek/api_key", Priority: 0,
        Fallback: []string{"deepseek"},
        Features: FeatureFlags{MessagesAPI: true, ToolUse: false,
            Thinking: true, Streaming: true, ContextWindow: 131072, MaxOutputTokens: 32000},
        Thinking: &ThinkingConfig{BudgetTokens: 4096, Mode: "auto"},
        InputPricePer1K: 0.001, OutputPricePer1K: 0.006,
    },

    // ── OpenAI (不支持 /v1/messages → 需要翻译层, Phase 5) ──
    "gpt-5": {
        Alias: "gpt-5", Provider: "openai",
        ModelID: "gpt-5", BaseURL: "https://api.openai.com",
        KeyRef: "llm/openai/api_key", Priority: 2,
        Fallback: []string{"sonnet"},
        Features: FeatureFlags{MessagesAPI: false, ToolUse: true,
            Streaming: true, StructuredOutput: true, Vision: true,
            ContextWindow: 128000, MaxOutputTokens: 16000},
        InputPricePer1K: 0.0025, OutputPricePer1K: 0.01,
    },

    // ── Gemini (不支持 /v1/messages → 需要翻译层, Phase 5) ──
    "gemini": {
        Alias: "gemini", Provider: "gemini",
        ModelID: "gemini-2.5-pro", BaseURL: "https://generativelanguage.googleapis.com",
        KeyRef: "llm/gemini/api_key", Priority: 2,
        Fallback: []string{"sonnet"},
        Features: FeatureFlags{MessagesAPI: false, ToolUse: true,
            Vision: true, Streaming: true, ContextWindow: 1048576, MaxOutputTokens: 64000},
        InputPricePer1K: 0.00125, OutputPricePer1K: 0.005,
    },
}
```

#### 4.5.2 Router 核心

```go
// internal/llm/router.go

type Router struct {
    registry   ModelRegistry
    secrets    SecretStore
    translator *Translator        // nil until Phase 5
    clients    map[string]*http.Client // 按 BaseURL 复用连接池
}

// SendMessage — 唯一对外暴露的方法。调用方只需传 alias。
func (r *Router) SendMessage(ctx context.Context, alias string, req *AnthropicRequest) (*AnthropicResponse, error) {
    entry, err := r.registry.Lookup(alias)
    if err != nil {
        return nil, err
    }
    if entry.Features.MessagesAPI {
        return r.forward(ctx, entry, req)        // Phase 1: 直通
    }
    return r.translateAndForward(ctx, entry, req) // Phase 5: Anthropic→OpenAI/Gemini 翻译
}

func (r *Router) forward(ctx context.Context, entry ModelEntry, req *AnthropicRequest) (*AnthropicResponse, error) {
    apiKey, err := r.secrets.Get(ctx, entry.KeyRef)
    if err != nil {
        return nil, fmt.Errorf("secret %q: %w", entry.KeyRef, err)
    }
    resp, err := r.post(ctx, entry.BaseURL+"/v1/messages", apiKey, req)
    if err != nil {
        return r.fallback(ctx, entry, req, err)  // 遍历降级链
    }
    return resp, nil
}

func (r *Router) fallback(ctx context.Context, entry ModelEntry, req *AnthropicRequest, origErr error) (*AnthropicResponse, error) {
    for _, fbAlias := range entry.Fallback {
        fb, _ := r.registry.Lookup(fbAlias)
        if resp, err := r.forward(ctx, fb, req); err == nil {
            emit.ModelFallback(entry.Alias, fbAlias)
            return resp, nil
        }
    }
    return nil, fmt.Errorf("all fallbacks exhausted: %w", origErr)
}
```

#### 4.5.3 模型切换流程

```
PM 点击 [Sonnet ▾] → 选 [Opus]
  ↓ WebSocket: {type: "model.switch", payload: {model_alias: "opus"}}
  ↓ BFF: pipeline.config.model_alias = "opus"
  ↓ 下一轮: Query Engine → Router.SendMessage(ctx, "opus", req)
  ↓ Router.Lookup("opus") → 查表 (< 1μs)
  ↓ SecretStore.Get("llm/anthropic/api_key") → POST /v1/messages
```

#### 4.5.4 企业代理覆盖

环境变量覆盖注册表字段，一行配置路由全部 Anthropic 家族模型到自定义端点：

```
ANTHROPIC_BASE_URL="https://api.deepseek.com/anthropic"
  → 所有 Provider=="anthropic" 的 entry 的 BaseURL 被替换
ANTHROPIC_AUTH_TOKEN="sk-..."
  → 覆盖 KeyRef，直接使用 env 值
```

### 4.6 Node.js IO 层

- **Dynamic Tool Hub**: 嵌入索引 (all-MiniLM) → 按需匹配 top-3 tool → 0 schema 进上下文
- **Skill Loader**: 扫描 SKILL.md → YAML 解析 → 语义匹配 → 注入上下文
- **Token Metering**: 每次 Chat 调用推送到内存环形缓冲 (lock-free ring buffer, 1000 slots)，满足 buffer>500 或 >5s 触发批量写入 (COPY protocol, 500 rows ~10ms)，Prometheus Counter 用 atomic counter 零开销。crash 时 buffer 未 flush 数据可接受丢失 (精度 ~99.9%)。
  ```sql
  CREATE TABLE token_usage (
    pipeline_id  TEXT,
    project_id   TEXT,
    provider     TEXT,
    model        TEXT,
    prompt_tokens     BIGINT,
    completion_tokens BIGINT,
    estimated_cost    DECIMAL(10,4),
    timestamp    TIMESTAMPTZ DEFAULT NOW()
  );
  -- Prometheus: of_llm_token_usage_total{project_id, provider, model} (atomic counter)
  ```

  **Token 超限检测**: 也走内存计数器 (每轮 atomic add) + 5s定期 flush，不每轮查 DB。内存中当前计数值超过阈值 → 触发超限通知。

  **Token 超限状态机** (新增 `token_exceeded` 终态):
  ```
  单 Pipeline Token 超限 → 通知 PM (飞书/钉钉卡片，含三个操作):

    [继续执行] → PM 确认预算追加 → Token 上限提升至当前值 × 1.5 → 继续
    [终止任务] → Agent 生成当前阶段半成品摘要 → Artifact 归档
                 → Pipeline 标记 token_exceeded (与 completed/rejected 并列)
    [切换模型] → 降级到更便宜模型 (如 Opus→Sonnet, Sonnet→DeepSeek)
                 → 重试当前阶段

  默认: 通知 24h 无响应 → 自动终止 + 归档
  token_exceeded Pipeline 后续可发起新 Pipeline (继承需求描述, 不继承对话历史)
  ```

  **配套控制点**:
  - 月预算: 每项目月度 Token 上限 (可配, 超出排队或走特批)
  - 异常检测: Token 消耗速率突然 ×3 → 告警 PM (疑似死循环)
  - 按项目/团队/时间维度聚合统计，按模型拆分消耗趋势
  - C 层成本看板提前到 **Phase 4**
- **Learning Engine**: 四层自学习

#### 4.5.1 Sandbox Provider 生命周期 `[Phase 4]`

> 借鉴 DeerFlow `SandboxProvider` 模式: acquire→use→release + LRU 缓存。

复用现有 `ContainerRuntime` 接口，Provider 层加缓存管理:

```
acquire(threadID) → warm pool 取出 → bind mount → 执行 → release → reset → 归还 pool
LRU 驱逐: 空闲 > 10min → 销毁容器
```

接口不变 (`ContainerRuntime` 已有 Create/Start/Stop/Remove/List)，仅实现层加缓存。

#### 4.5.2 CommandExecutor — 第 12 能力域 `[Phase 1 — v5 新增]`

Profile 感知的命令执行，Phase 1 即可交付 (zero-dependency `os/exec`):

```go
type CommandExecutor interface {
    Execute(ctx, command, ExecOptions) (ExecOutput, error)
    ExecuteStream(ctx, command, ExecOptions) (<-chan StreamChunk, error)
    Validate(ctx, command, ExecOptions) error
}
```

| Profile | 实现 | 安全边界 |
|---------|------|---------|
| minimal | `LocalShellExecutor` (os/exec) | 危险命令硬阻断 + 模式匹配 + 只读白名单 |
| standard/enterprise | `DockerSandboxExecutor` | 5 层容器纵深 (复用 §6.2) |

**BashTool** 实现 `StreamingTool[BashInput, ExecOutput]`，融入 §4.3 Tool 体系 + §4.4 权限判定链 + §3.15 错误恢复链。流式输出复用 §5.5.4 `terminal.output` 事件，无新增 WS 协议。

### 4.3 Go ↔ Node 通信

- Protobuf IDL 契约 (`coordinator.proto`, `llm.proto`, `tools.proto`, `learning.proto`)
- buf breaking change 检测
- 集成测试: Go 端调用真实 Node 层验证

### 4.4 上下文窗口管理

```
阶段 A 完整上下文 (150轮 ~45K tokens)
  → compress →
阶段 B 只接收: 需求摘要(300t) + 约束列表(200t) + 关键决策(200t) = ~700 tokens

每阶段结束 → Agent 自动生成 Stage 摘要 → 下游只收摘要 + 当前输入
```

### 4.5 Prompt 分层缓存

> Phase 5d 已将 Prompt 构建从四层简化为三层 (L1/L2/L4), L3 退化入 L2。详见 §4.2b。

```
L1 静态层: config/prompts/static.xml 启动加载 → 内存缓存, 永不重读
L2 项目层: of-prefs.yaml + stage_templates.go + KnowledgeQuerier → 
           Pipeline 内缓存 (Phase 6 计数触发刷新, Phase 5d 始终重建)
L4 对话层: buildL4Summary() 最近 5 轮 → 注入 SystemPrompt → 每轮动态

Anthropic Prompt Cache 断点 (cache_control): Phase 6 (含 L1/L2 prefix 标注)
节省预估: ~40% token (L4 受 Anthropic 5-min TTL 限制, 实际命中率低于 L1/L2)
```

### 4.6 LLM 优先级调度

Weighted Fair Queueing (WFQ):
```
P0: 灰度验证 / crash恢复  → 抢占式
P1: L4架构 / 生产修复     → weight 4
P2: L3功能开发 / L2修改    → weight 2
P3: L1原子变更 / 后台整理  → weight 1
```

### 4.7 嵌入索引 (增量 + CoW)

```
Delta + Base 结构:
  Base Index (全量, vN) + Delta Index (增量, vN.M)
  写入: O(变更数) 而非 O(全量)
  合并: Delta > 1000 条 或 Base > 24h → 异步合并

Copy-on-Write:
  读: 活跃索引 (无锁)
  写: 克隆 → 写入 → 原子交换指针 → 旧版等 reader 归零 → 释放
```

### 4.8 四层自学习

```
L1 静态规则: AST 分析 + git diff 统计 → {缩进, 命名, 框架, 测试模式}
L2 反馈闭环: Agent产出 → 用户修改 → diff对比 → 更新偏好 profile
L3 轨迹学习: trajectories.jsonl → 提取偏好 → prompt 前缀注入
L4 嵌入匹配: 当前任务 → 嵌入相似度 → 匹配历史轨迹 → 提取偏好

全本地运行, all-MiniLM-L6-v2 / bge-small-zh, 不依赖外部 LLM
```

### 4.9 知识库维护

```
轨迹去重:    嵌入相似度 > 0.95 → 合并 + 计数器
偏好冲突:    频次 > 时间 > Agent 置信度
定期清理:    每周合并相似偏好 + 清理过期 + 快照压缩
知识回滚:    快照版本管理 + 自动回滚 (代码接受率低于基线 20%)
全局准入:    每月人工审核晋升/降级/删除
种子知识库:   新项目自动注入 Conduit + General 种子
多租户隔离:   项目级 namespace + 全局晋升 Gate
```

### 4.10 自学习 A/B 测试

新知识进入 Learning Engine 后必须经过实验验证:

```
Cohort A (90% Pipeline): 使用新知识 K 作为 prompt 前缀
Cohort B (10% Pipeline): 不使用 K (对照组)

实验周期: min(50 Pipeline, 7天)
判定:
  K 组接受率 > B 组且 p < 0.05 → K 晋升为正式知识
  K 组接受率 ≤ B 组 → K 标记为无效, 归档
  K 组接受率 < B 组且差异 > 10% → K 标记为有害, 立即撤回 + 记录反模式
```

种子知识标记为 `trusted` 直接注入（不进入实验），积累 100 Pipeline 后重新评估。L1 静态规则不进入实验（直接注入）。C 层新增 A/B 实验看板。

### 4.11 跨 Pipeline 总结报告

每周自动触发，跨 Pipeline 分析驳回原因频次分布、识别有效/无效知识模式、检测新趋势，自动执行晋升/淘汰/新建 A/B 实验。需人工裁决的问题推送给对应角色。详见 A 层 3.12 节。

### 4.12 版本兼容策略

```
Pipeline Artifact Schema:
  v1 初始 → v2 新增字段 (带默认值, v1 兼容读取)
  → Postgres 存储带 schema_version 字段

Prompt Template:
  code-generate.v1 → code-generate.v2
  灰度期间两版共存, v2 稳定后 v1 deprecation window 30 天 → 强制迁移 → 删除

Learning Snapshot:
  header: {version, created_at, checksum}
  v1 快照可被 v2 读取 (向前兼容), v2 新字段 v1 忽略 (向后兼容)

破坏性变更 4 步流程:
  1. 新版本 + 迁移脚本
  2. 双写 (新旧格式) 验证
  3. 切换到新格式读 → 停止写旧格式
  4. 清理旧格式
```

### 4.14 LLM Router Anthropic 标准选择理由

```
选用 Anthropic Messages API 作为内部统一标准 (详见 §4.5 LLM Router + 模型注册表):
  1. tool_use 是 content block — 支持文本+工具调用交替出现,
     更适合 tool-heavy agent 场景 (>90% 的 Agent 对话包含 tool_use)
  2. 国产 LLM (DeepSeek 等) 为兼容 Claude Code 已普遍提供 /v1/messages 端点
      → 注册表中 Features.MessagesAPI==true → 直通, 零翻译开销
  3. 仅对不兼容 Anthropic 格式的 provider 写翻译层 (~80 行/provider, 边际成本低)
     → 注册表中 Features.MessagesAPI==false → 走 translateAndForward()

Router 位于 Go 协调层(非 Node.js IO 层):
  - minimal profile: Go 单二进制, 零 Node 依赖, LLM Router 进程内调用 Query Engine
  - 模型切换: O(1) map 查找, 不重启 Pipeline, 不重建连接
  - 凭据: SecretStore 统一管理, 注册表仅存 KeyRef 引用

翻译范围: OpenAI tool_calls ↔ Anthropic tool_use, Gemini functionCall ↔ Anthropic tool_use
```

---

## 五、C层: 协作工作台

### 5.1 技术栈与 BFF API 契约

**API 端点摘要** (REST + WebSocket):

```
项目管理:
  GET    /api/projects                           # 项目列表
  GET    /api/projects/:id                       # 项目详情 + Pipeline 列表
  POST   /api/projects/:id/pipelines             # 创建 Pipeline
  GET    /api/projects/:id/pipelines/:pid        # Pipeline 详情 (状态/阶段/产物)
  POST   /api/projects/:id/pipelines/:pid/cancel # 取消 Pipeline
  POST   /api/projects/:id/pipelines/:pid/modify # 修改需求 (触发回溯)
  GET    /api/projects/:id/pipelines/:pid/branches # 对话分支列表

对话:
  GET    /api/pipelines/:pid/messages             # 消息历史 (分页)
  POST   /api/pipelines/:pid/messages/retry       # 从指定消息重试

Gate 审批:
  GET    /api/review-inbox                       # 审批收件箱 (跨项目聚合)
  POST   /api/pipelines/:pid/gate/:stage/approve # 审批通过
  POST   /api/pipelines/:pid/gate/:stage/reject  # 驳回 + 反馈
  POST   /api/pipelines/:pid/gate/:stage/claim   # 认领审批

模型:
  GET    /api/pipelines/:pid/models              # 可用模型列表
  POST   /api/pipelines/:pid/model/switch        # 切换模型

Token/成本:
  GET    /api/projects/:id/token-usage           # Token 消耗 (按模型/时间)
  GET    /api/projects/:id/token-quota           # Token 配额余量

Artifact:
  GET    /api/pipelines/:pid/artifacts            # 产物列表
  GET    /api/pipelines/:pid/artifacts/:aid/download # 下载产物

WebSocket:
  wss://<host>/ws  (协议见 5.5.4)

用户:
  GET    /api/user/settings                      # 获取设置
  PUT    /api/user/settings                      # 更新设置
  GET    /api/user/layout                        # 获取面板布局
  PUT    /api/user/layout                        # 持久化布局
```

- **前端**: BFF 聚合层 + 微前端模块
- **BFF**: tRPC/gRPC 网关 + REST + WebSocket
- **Auth**: SSO/OIDC/LDAP Adapter + JWT + RBAC

### 5.2 代码审查交互

```
Diff View (side-by-side) + 文件树(changed files)
行级评论: 精确到行号的代码反馈
文件级标记: accept / needs revision / needs discussion
总结性反馈: 自然语言整体指导

驳回后 Agent 收到: 行级评论 + 文件标记 + 总结
  → 仅重新生成标记为 'needs revision' 的文件
  → 反馈写入 preferences.json (L2 自学习)
```

### 5.3 RBAC 权限

```
管理员:      全局 (系统配置/用户管理/项目管理)
PM:          所属项目 (创建需求/澄清/审方案/确认部署)
开发负责人:   所属项目+指定模块 (审核代码/管理测试用例)
开发:        指定模块 (查看 Diff/提交修改意见)
观察者:      指定项目 (只读)
```

### 5.4 企业认证对接 (BFF 鉴权中间件)

鉴权统一在 BFF 层完成，A/B 层信任 BFF 注入的身份 Header (内网 mTLS 不可伪造):

```
请求 → Nginx(TLS+限流) → BFF Auth Middleware
  ├── Step 1: JWT 验证 (SSO/OIDC/LDAP/SAML)
  ├── Step 2: Session 验证 + 滑动窗口续期
  ├── Step 3: Permission 判定 can(user, action, project) → true/false/limited
  ├── Step 4: 注入 Header: X-Authenticated-User, X-User-Roles
  └── Step 5: 转发 A 层 或 拒绝 (401/403)
```

BFF Auth 结构: `middleware.ts` → `providers/{oidc,ldap,saml}.ts` → `role-mapper.ts` (LDAP OU → RBAC 角色映射) → `permission-engine.ts`。

Token 生命周期: 用户 Session 8h (滑动续期) / Pipeline Token 最长 24h (完成即吊销) / Git Bot Token 长期 (30d 轮换, Vault 管理)。

### 5.5 前端设计

> **Phase 映射总览**:
> Phase 2: 5.5.1(仅简约模式) / 5.5.2(仅 3 路由) / 5.5.3(仅基础消息+流式) / 5.5.4(基础 WS)
> Phase 3: 5.5.1(全部) / 5.5.2(全部路由) / 5.5.3(卡片+分支+进度) / 5.5.4(全部 WS) / 5.5.8 / 5.5.9 / 5.5.10
> Phase 4: 5.5.5 / 5.5.6 / 5.5.7
> Phase 6: 5.5.12
> Phase 7: 5.5.11 / 5.5.13 / 5.5.17
> Phase 8: 5.5.14 / 5.5.15 / 5.5.16

#### 5.5.1 双模式布局 `[Phase 2-4]`

**简约模式 (PM 默认)** — 左侧对话区 + 右侧信息卡片网格，拖拽可自由调整 60/40 占比:

```
┌────────────────────────────┬──────────────────────────────┐
│ 对话区                      │ 信息卡片区                    │
│ ┌────────────────────────┐ │ ┌──────────┐ ┌───────────┐ │
│ │ 对话历史 (可滚动)       │ │ │ 🗺 模块拓扑│ │ 💰 预估    │ │
│ │ PM: "给文章加标签过滤"   │ │ │ [依赖图] │ │ L3, 80K   │ │
│ │ Agent: "已分析需求..."   │ │ │ 点击放大  │ │ Token, ~2h │ │
│ │ ┌ Diff 卡片 ──────────┐ │ │ └──────────┘ └───────────┘ │
│ │ │ auth.ts +45/-12     │ │ │ ┌──────────┐ ┌───────────┐ │
│ │ │ [查看详情]          │ │ │ │ 📊 流程图  │ │ 📋 模块    │ │
│ │ └────────────────────┘ │ │ │ │ [泳道图] │ │ 3模块5文件 │ │
│ │ Agent: "测试通过 ✅"    │ │ │ │ 点击放大  │ │ 点击展开  │ │
│ └────────────────────────┘ │ └──────────┘ └───────────┘ │
│ ┌────────────────────────┐ │ ┌──────────┐ ┌───────────┐ │
│ │ 输入框          [发送] │ │ │ 🔔 Gate   │ │ 📜 审查    │ │
│ └────────────────────────┘ │ │ 等待审批  │ │ 拒绝3次    │ │
└────────────────────────────┘ └──────────┘ └───────────┘
```

**专业模式 (开发默认)** — 完全自由拖拽面板 (Dockview, MIT 协议):

```
┌──────────┬──────────────────────┬──────────┐
│ 文件树    │  Diff View            │ AI 对话   │
│ (面板A)  │  (面板B)              │ (面板C)   │
├──────────┴──────────────────────┴──────────┤
│  拓扑图 (面板D)    │  测试报告 (面板E)       │
├────────────────────┼───────────────────────┤
│  行级评论 (面板F)   │  Terminal (面板G)      │
└────────────────────┴───────────────────────┘
```

**面板清单 (全部可自由拖拽/拆分/合并/Tab/浮动/最大化/关闭)**:

| 面板 | 内容 | 可开实例数 |
|------|------|-----------|
| 文件树 | 本次变更文件列表 (带变更标记) | 1 |
| Diff View | Monaco side-by-side diff | 多个 (可同时打开不同文件) |
| AI 对话 | 实时 Agent 问答 (可 @Agent 提问) | 多个 |
| 拓扑图 | Cytoscape.js 交互式依赖图 | 1 |
| 流程图 | 当前需求泳道图 (阶段 + Gate 进度) | 1 |
| 测试报告 | 实时测试结果 + 失败用例列表 | 1 |
| 行级评论 | 审查反馈面板 (按文件分组) | 1 |
| Terminal (只读) | Sandbox 输出日志 (只读观察, 不可输入) | 1 | — |
| Terminal (调试) | Sandbox 手动操作 (**仅 Dev 环境**, require 二次确认) | 1 | Phase 4+ |
| Gate 面板 | 审批操作 (批准/驳回/评论) | 1 |
| 对话历史 | 对话消息树形列表 (含分支) | 1 |

**面板规则**: 拖拽标签到相邻 → 分屏；拖拽标签到另一个面板 → Tab 合并；关闭的面板从「面板菜单」重新打开；布局持久化 localStorage 按用户 + 按项目；预置布局切换 (代码审查/调试/PM 审查)。

#### 5.5.2 路由与组件树 `[Phase 2-4]`

```
页面路由:
  /                          → 项目列表 (Dashboard)
  /project/:id               → 项目主页 (Pipeline 列表 + 统计)
  /project/:id/chat           → 简约模式 (PM 对话 + 信息卡)
  /project/:id/pipeline/:pid  → 专业模式 (可拖拽面板工作台)
  /project/:id/review/:pid    → 代码审查 (默认专业模式布局)
  /review-inbox               → 审批收件箱 (跨项目聚合)
  /admin                      → 管理后台

组件树:
  App
  ├── NotificationCenter (全局通知铃铛 + 下拉列表)
  ├── ProjectListPage
  ├── ProjectPage
  │   ├── PipelineDashboard (列表 + 图表 + Token 趋势)
  │   ├── SimpleMode (简约布局容器)
  │   │   ├── ChatPanel (对话 + 消息流 + 输入区)
  │   │   └── InfoCardGrid (拓扑/预估/流程图/模块卡)
  │   └── ProMode (Dockview 布局容器)
  │       ├── FileTreePanel, DiffPanel, ChatPanel
  │       ├── TopologyPanel, FlowchartPanel
  │       ├── TestReportPanel, CommentPanel
  │       └── TerminalPanel, GatePanel, ChatHistoryPanel
  ├── ReviewInboxPage (聚合审批队列)
  └── AdminPage

状态管理: React Context + useReducer (Phase 1-4), Phase 5+ Zustand
面板库: Dockview (MIT) — 支持拆分/合并/Tab/浮动/最大化/持久化
实时通信: WebSocket (流式输出 + 状态推送 + 通知)
```

#### 5.5.3 对话面板交互 `[Phase 2-4]`

**消息类型**:

```
💬 文本消息 (Markdown 渲染)
📝 代码卡片 (Diff 语法高亮 + [查看全文件] [审查Diff] [复制代码])
🗺 拓扑图卡片 (交互式, 点击节点跳转代码, [展开全屏])
🚦 Gate 审批卡片 (变更文件/测试结果 + [批准] [驳回] [仅评论])
⚠️ 错误卡片 (失败用例列表 + [查看详情] [自动修复] [手动修复])
```

**流式输出与打断**:

```
Agent 输出中: 光标闪烁 + 实时渲染 + [⏹ 停止] [⏸ 暂停]
  停止 → 终止 LLM 流 → 已生成内容保留 → 输入框激活
  暂停 → 当前状态存为检查点 → PM 可插入新指令 → [▶ 继续]
```

**消息操作 (hover 悬浮)**:

```
Agent 消息: ✏️编辑  🔄重试(丢弃后续)  📋复制  ⭐收藏  🚩标记问题
用户消息:   ✏️编辑  ↩️撤回(撤回本条及之后所有)
```

**对话分支**:

```
编辑第N条消息重发 → 自动创建分支:
  对话分支 (前端概念) ↔ DAG 回溯 (后端概念) 映射:
  
  前端编辑第 3 条消息 → Agent 判断影响范围:
    L1/L2 变更: 不回退 Pipeline 阶段, 仅修正实现 → 对话分支 = 轻量修正
    L3/L4 变更: 触发 DAG 回溯到对应阶段 → 对话分支 = DAG 回溯实例
    → 前端分支在左侧历史面板可视化, 与 A 层回溯状态通过 WS 同步
  
  分支上限: 同一 Pipeline 最多 3 个活跃分支 (对齐 DAG 回溯上限)
  分支对比: 两个分支的 Diff 可 side-by-side 对比

右侧对话历史面板: 树形结构, 可切换可对比
[🌿 创建分支] 按钮在任意消息上
```

**进度指示器** (对话顶部常驻, 可折叠):

```
Pipeline #42 — ●澄清✅ ●拆解✅ ◉实现⏳ ○测试 ○部署 ○验证
已耗时: 1h23min | 预计剩余: 45min | Token: 42K/80K | 驳回: 0
[查看详情] [取消 Pipeline]
```

**逃生舱**:

```
[取消 Pipeline] → 确认 → 保存半成品 Artifact → 终止
[修改需求]     → 修订 → Agent 判断回溯级别
[切换模型]     → 下拉选便宜模型继续
[手动介入]     → 打开 Terminal 面板直接操作 Sandbox
```

**通知中心**:

```
Header 铃铛 [N] → 下拉:
  ⚠️ Pipeline #42 等待审批 ｜5分钟前
  ✅ Pipeline #38 验证通过 ｜2小时前
  ❌ Pipeline #36 Token超限 ｜昨天
  📊 本周总结报告已生成    ｜昨天
```

#### 5.5.4 WebSocket 实时协议 `[Phase 2-4]`

```
端点: wss://<host>/ws
鉴权: 连接后首帧发送 {"type":"auth","payload":{"token":"<short-lived-token>"}}
      token 为 BFF 签发的 1h 短期令牌 (不可放 URL query string,
      防止被服务器日志/反向代理/浏览器历史记录泄露)
      服务器 10s 内未收到有效 auth → 断开连接

统一 JSON 帧格式:
  { "type": "...", "seq": 42, "timestamp": "ISO8601", "pipeline_id": "pipe-42", "payload": {...} }
```

**客户端 → 服务端 (上行)**:

| 事件 | Payload | 说明 |
|------|---------|------|
| `auth` | `{token}` | 首帧鉴权 (1h 短期 token, 不放 URL) |
| `chat.send` | `{msg_text, branch_id?, reply_to_msg_seq?}` | 发送消息 |
| `chat.edit` | `{msg_seq, new_text}` | 编辑历史消息 → 触发新分支 |
| `chat.stop` | `{}` | 停止当前流式输出 |
| `chat.pause` | `{}` | 暂停 → Agent 保存检查点 |
| `chat.resume` | `{}` | 从暂停检查点继续 |
| `chat.retry` | `{from_msg_seq}` | 从指定消息起重新生成 |
| `chat.cancel_branch` | `{branch_id}` | 废弃一个对话分支 |
| `gate.approve` | `{gate_stage, checklist}` | 审批通过 |
| `gate.reject` | `{gate_stage, line_comments, summary}` | 驳回 + 行级反馈 |
| `pipeline.cancel` | `{reason}` | 取消整个 Pipeline |
| `pipeline.modify_scope` | `{new_requirement}` | 修改需求触发回溯 |
| `model.switch` | `{provider, model}` | 切换模型继续 |
| `panel.layout.save` | `{layout_json}` | 持久化面板布局 |
| `terminal.input` | `{container_id, input}` | Sandbox 终端输入 |

**服务端 → 客户端 (下行)**:

| 事件 | Payload | 说明 |
|------|---------|------|
| `chat.stream` | `{msg_seq, delta_text, is_done}` | Agent 流式输出，前端逐 token 渲染 |
| `chat.stream_done` | `{msg_seq, full_text, token_count}` | 单条消息流式输出完成 |
| `chat.branch_created` | `{branch_id, parent_msg_seq}` | 编辑重发触发新分支 |
| `msg.card` | `{msg_seq, card_type, card_data}` | 代码/Diff/Gate/拓扑/错误卡片 |
| `pipeline.stage_change` | `{from, to, status}` | 阶段流转 |
| `pipeline.progress` | `{elapsed, est_remaining, token_used, token_limit}` | 进度更新 (每 5s) |
| `pipeline.token_warning` | `{used, limit, rate_x3}` | Token 消耗速率异常告警 |
| `pipeline.finished` | `{status, summary}` | Pipeline 终态 |
| `gate.notify` | `{gate_stage, reviewer, deadline}` | 通知审批人 |
| `gate.claimed` | `{reviewer}` | 审批已被认领 (防止重复审批) |
| `test.report` | `{passed, failed, skipped, summary}` | 测试报告实时推送 |
| `terminal.output` | `{container_id, output}` | Sandbox 终端输出流 |
| `notification` | `{level, title, body, action_url}` | 全局通知 |

**断线重连**:
```
重连 → client 发 {type: "sync.request", seq: 37}
     → server 回 {type: "sync.replay", events: [{seq:38,...}, {seq:39,...}]}
     → client 增量重放到当前, 补推期间丢失的卡片/通知
```

#### 5.5.5 新手引导 (Onboarding) `[Phase 4]`

```
首次登录 → 3 步引导:

Step 1: 角色选择
  ┌──────────────────────────────────────┐
  │ 欢迎使用 OpenForge 🎉               │
  │                                      │
  │ 我的主要角色是:                       │
  │ [📋 产品经理]  [💻 开发者]  [🔧 管理] │
  └──────────────────────────────────────┘

Step 2: 项目接入 (如果是第一个项目)
  ┌──────────────────────────────────────┐
  │ 接入项目                              │
  │                                      │
  │ Git 仓库地址: [_____________]        │
  │ 仓库类型: [Monorepo ▾]               │
  │                                      │
  │ 检测到 monorepo → 自动识别前后端子项目 │
  │ ✅ frontend/ (React + TypeScript)     │
  │ ✅ backend/  (Express + TypeScript)   │
  │                                      │
  │ [开始分析仓库] → 生成拓扑图 + 种子知识 │
  └──────────────────────────────────────┘

Step 3: 首次对话示范
  ┌──────────────────────────────────────┐
  │ 试试看:                               │
  │                                      │
  │ 聊天框预填示范需求 (可编辑):           │
  │ "给文章列表增加标签过滤功能"           │
  │                                      │
  │ [发送] — Agent 开始分析并展示拓扑卡片  │
  └──────────────────────────────────────┘

引导完成后 → 跳到对应模式的默认布局
后续可通过 [帮助] 菜单 → [重新走引导]
```

**上下文帮助**: 每个复杂面板右上角 [?] → 点击弹出该面板的功能说明 Tooltip (3-5 句 + GIF 动图)。

#### 5.5.6 错误页与熔断感知 `[Phase 4]`

```
404:
  ┌──────────────────────────────────┐
  │ 🔍 页面未找到                     │
  │                                  │
  │ 返回 [项目列表]                   │
  └──────────────────────────────────┘

500:
  ┌──────────────────────────────────┐
  │ ⚠️ 系统暂时出错了                 │
  │                                  │
  │ 错误 ID: err-20260518-0042       │
  │ 已自动通知运维团队                 │
  │                                  │
  │ [返回项目列表]  [复制错误ID]      │
  └──────────────────────────────────┘

503 (熔断 OPEN):
  ┌──────────────────────────────────┐
  │ 🔧 Agent 引擎暂时降级             │
  │                                  │
  │ 原因: LLM 推理服务连续 5 次超时    │
  │ 正在自动恢复，预计 30 秒后重试      │
  │                                  │
  │ 您的 Pipeline 已自动挂起，恢复后继续│
  │ 无需重新提交需求                   │
  │                                  │
  │ [通知我恢复后]  [查看详情]         │
  └──────────────────────────────────┘

503 (Token 配额耗尽):
  ┌──────────────────────────────────┐
  │ 💰 本月 Token 配额已用尽           │
  │                                  │
  │ 已用: 50M / 50M                  │
  │ 下月重置: 2026-06-01 (14 天后)    │
  │                                  │
  │ [申请特批额度 ▸]  [切换到更便宜模型]│
  └──────────────────────────────────┘
```

#### 5.5.7 用户设置页 `[Phase 4]`

```
/user/settings

┌──────────────────────────────────────────────────┐
│ ⚙️ 设置                                          │
│                                                  │
│ ┌─ 通知偏好 ────────────────────────────────────┐│
│ │ 飞书通知:    [✅ Pipeline审批] [✅ 审批超时提醒]││
│ │              [✅ Token告警]   [❌ 每周总结]     ││
│ │ 邮件通知:    [❌ 全部关闭]                      ││
│ │ 浏览器通知:  [✅ 全部开启]                      ││
│ └────────────────────────────────────────────────┘│
│ ┌─ 默认布局 ────────────────────────────────────┐│
│ │ 默认模式:   [简约 ▾]                           ││
│ │ 审查时自动: [切换到专业模式 ✅]                  ││
│ │ 编辑器字体: [JetBrains Mono ▾] 字号: [14]      ││
│ │ 主题:       [跟随系统 ▾]                       ││
│ │ [重置所有布局到默认]                            ││
│ └────────────────────────────────────────────────┘│
│ ┌─ 语言与地区 ──────────────────────────────────┐│
│ │ 界面语言:   [中文 ▾]                           ││
│ │ 代码注释语言: [跟随界面]                        ││
│ └────────────────────────────────────────────────┘│
└──────────────────────────────────────────────────┘
```

#### 5.5.8 失败分类 → 人类可读 `[Phase 3]`

```
内部码 → 用户侧卡片展示:

MODEL_HALLUCINATION → 
  🤖 "AI 生成了不存在的方法/文件"
  建议: Agent 已自动回退，将使用仓库现有代码作为参考重新生成

PROMPT_WEAKNESS → 
  📝 "需求描述不够精确，AI 理解偏差"
  建议: PM 尝试将需求拆分为更小的步骤，或 @开发者 帮忙澄清

DEPENDENCY_CONFLICT → 
  📦 "新代码与现有依赖版本冲突"
  建议: 人工介入 — 可能需要升级某个 npm 包或锁定版本

SANDBOX_TIMEOUT → 
  ⏰ "测试运行超时 (可能死循环)"
  建议: Agent 已自动重试一次，仍失败则需开发者检查

REPO_BUG → 
  🐛 "仓库本身存在 Bug，不是 AI 的问题"
  建议: 先在 main 分支复现，确认是否为已有问题

CONTEXT_OVERFLOW → 
  📄 "本轮需求涉及文件太多，上下文溢出"
  建议: 拆分为多个小需求分别执行

TOKEN_QUOTA_EXCEEDED → 
  💰 "Token 配额不足"
  建议: 等待下月重置 / 申请特批 / 切换到更便宜模型

UNKNOWN → 
  ❓ "未识别的原因"
  建议: [人工排查 ▸] → 开发者分类后回写规则
```

#### 5.5.9 拓扑图分级视图 `[Phase 3]`

```
拓扑图显示分级 (右上角切换):

L1 — 业务视图 (PM 默认):
  ┌────────────────────────────┐
  │  [文章列表页] ──→ [文章API] │
  │       │              │     │
  │       └──── [标签系统] ←── │
  │                              │
  │  仅显示: 页面 → API → 功能   │
  │  隐藏: 中间件/模型/数据库细节  │
  └────────────────────────────┘

L2 — 技术视图 (开发默认):
  ┌──────────────────────────────┐
  │  [ArticleList.tsx]            │
  │       │                       │
  │  [routes/articles.ts]         │
  │       │                       │
  │  [middleware/auth.ts]         │
  │       │                       │
  │  [models/Article.ts]          │
  │       │                       │
  │  [db/articles table]          │
  │                                │
  │  显示: 完整文件级依赖链路       │
  └────────────────────────────────┘

L3 — 数据流视图 (架构师):
  ┌──────────────────────────────┐
  │  前端: GET /api/articles?tag=│
  │    ↓ HTTP                    │
  │  后端: router.get()          │
  │    ↓ JWT verify              │
  │  中间件: auth.validate()      │
  │    ↓ SQL query               │
  │  数据: SELECT ... WHERE tag= │
  │    ↓ JSON response           │
  │  前端: setState(articles)    │
  │                                │
  │  显示: 请求→响应 完整数据流     │
  └────────────────────────────────┘

实现: 同一份拓扑数据，前端按 level 过滤节点类型
```

#### 5.5.10 前端安全 `[Phase 2-3]`

**WebSocket 鉴权**: JWT 不放 URL query string，改为连接后首帧 `{type:"auth", token}`，token 为 BFF 签发的 1h 短期令牌 (过期前 5min 静默续期)。

**XSS 防护**: 流式输出渲染前经 DOMPurify (白名单: h1-h6/p/code/pre/ul/ol/li/strong/em/a/table/blockquote)，script/iframe/object/embed/on* 全部 strip。代码块用 Monaco 只读模式渲染。CSP: `default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; connect-src 'self' wss:`。

**WS 心跳与退避重连**: 每 30s ping → 10s 内 pong, 3 次失败断开。重连: 指数退避 1s→2s→4s→8s→16s→30s(max)。连后发 `sync.request` 恢复丢失事件。

**Monaco 内存管控**: 最多 4 实例，超出 LRU 淘汰。逐 token 渲染经 rAF 批处理 (16ms 窗口合并 → 单帧单 DOM 更新)。

#### 5.5.11 可访问性 (A11y) `[Phase 7]`

```
键盘导航:
  ├── 拓扑图: Tab 在节点间跳转, Enter 选中, ←↑↓→ 移动焦点,
  │           + 键放大, - 键缩小, 0 重置视图
  ├── 面板: Ctrl+数字 切换面板, Ctrl+W 关闭当前面板,
  │         Ctrl+Shift+Arrow 面板间移动焦点
  └── 对话: ↑↓ 在消息间跳转, R 回复, Esc 关闭面板

ARIA 标注:
  ├── 面板容器: role="tablist" + aria-label 标注面板名
  ├── 拓扑图 SVG: role="img" + aria-description="交互式模块依赖图"
  │   每个节点: role="button" + aria-label="模块: articles.ts, 依赖于 auth.ts"
  └── 消息流: role="log" + aria-live="polite" (新消息自动朗读)

屏幕阅读器:
  ├── 流式输出: aria-live="polite" 只朗读完整句子 (非逐 token)
  ├── 卡片: 代码/Diff 卡片标记 aria-label="代码变更: +45 -12 行"
  └── 通知: role="alert" + aria-live="assertive"
```

#### 5.5.12 浏览器兼容矩阵 `[Phase 6]` — Phase 6

```
最低支持:
  Chrome 90+ / Edge 90+ / Firefox 90+ / Safari 15+
  (基于 WebSocket + CSS Grid + ResizeObserver 支持)

企业防火墙场景:
  WebSocket 被阻断 → 自动降级 SSE (EventSource)
  SSE 也不可用 → 降级 HTTP long-polling (30s timeout)
  → 前端 BFF 自动检测并切换 (feature detection, 非 UA sniffing)

移动端 (Phase 9+):
  响应式适配: 小屏 (<768px) 仅显示对话面板，其余面板全屏叠加
  手势: 左滑返回, 长按消息操作菜单
```

#### 5.5.13 渐进增强 `[Phase 7]` — Phase 7

```
JS 禁用: 显示 "OpenForge 需要 JavaScript 运行" 提示页
          + 静态链接到 FAQ/管理员联系方式

WebSocket 不可用: 见 5.5.12 降级策略

Monaco 加载失败: fallback → 纯文本 <pre> 块 + 语法高亮 (Prism.js)
                 (Prism 在 HTML 中静态加载, 不依赖 JS 动态 import)

CSS Grid 不支持 (IE/旧浏览器): 降级 → flexbox 单列布局
```

#### 5.5.14 离线与弱网 `[Phase 8]` — Phase 8

```
Service Worker:
  ├── 缓存策略: App Shell (HTML/JS/CSS) → Cache-First
  │             API 数据 → Network-First, 5s 超时 → 缓存
  │             静态资源 (字体/图片) → Stale-While-Revalidate
  └── 离线页: 显示 "当前离线" + 已缓存的 Pipeline 列表和对话记录

localStorage:
  ├── 草稿: 输入框内容每 5s 自动保存 → 刷新页面不丢
  ├── 布局: 面板布局实时持久化 (现有)
  └── 离线队列: 无网络时代理 → 网络恢复后自动重放
                (仅非关键操作: 布局保存/设置变更, 不含审批/Gate)

IndexedDB:
  └── 对话历史缓存 (最近 100 条消息) → 离线可回顾
```

#### 5.5.15 前端可观测性 (RUM) `[Phase 8]` — Phase 8

```
Web Vitals:
  ├── LCP (Largest Contentful Paint): p99 < 2.5s
  ├── INP (Interaction to Next Paint): p99 < 200ms
  ├── CLS (Cumulative Layout Shift): p99 < 0.1
  └── 采集: web-vitals 库 → 每 5min 批量 POST /api/rum/metrics

错误监控:
  ├── 未捕获异常: window.onerror → POST /api/rum/errors
  ├── Promise rejection: unhandledrejection → 同上
  └── React Error Boundary: 组件级 500 → 显示降级 UI

W3C Trace Context (前端侧):
  ├── 页面加载时生成 W3C traceparent → 注入 WS 首帧 headers
  ├── 用户点击事件生成 span → 待后端 span 关联
  └── 存储在 performance.mark() API, RUM SDK 定期上报

行为埋点:
  ├── 关键路径: Pipeline 创建/审批/部署 成功率漏斗
  ├── 用户行为: 面板切换/拓扑交互/分支切换 频率
  └── 隐私: 不记录具体消息内容/代码 Diff 内容
```

#### 5.5.16 国际化 (i18n) `[Phase 8]` — Phase 8

```
语言:
  Phase 1-7: 中文 (zh-CN) 默认
  Phase 8: + 英文 (en-US), 日文 (ja-JP)
  架构: react-intl / i18next, ICU MessageFormat

日期/时间: Intl.DateTimeFormat (自动跟随用户 locale)
数字: Intl.NumberFormat (Token 计数: "80,000" / "8万")
RTL (阿拉伯语/希伯来语): CSS logical properties
  margin-left → margin-inline-start, text-align → dir="auto"
  Phase 9+ 启用

翻译: 界面文案提取到 JSON key → 人工翻译
  Agent 对话内容: 不翻译 (保持原始语言)
```

#### 5.5.17 面板注册表 + 主题体系 `[Phase 7]` — Phase 7

```
面板注册表 (可扩展):
  panelRegistry = {
    'file-tree':   { component: FileTreePanel,   icon: 'folder',  title: '文件树' },
    'diff-view':   { component: DiffPanel,        icon: 'diff',    title: 'Diff' },
    'chat':        { component: ChatPanel,        icon: 'chat',    title: 'AI 对话' },
    'topology':    { component: TopologyPanel,    icon: 'graph',   title: '拓扑图' },
    'flowchart':   { component: FlowchartPanel,   icon: 'flow',    title: '流程图' },
    // 第三方插件注册: PanelRegistry.register('custom-analyzer', MyPanel)
  }

主题 Token (CSS 变量):
  :root {
    --of-bg-primary: #ffffff;
    --of-bg-secondary: #f5f5f5;
    --of-text-primary: #1a1a1a;
    --of-accent: #2563eb;
    --of-border: #e5e5e5;
    --of-monaco-bg: #1e1e1e;
  }
  [data-theme="dark"] {
    --of-bg-primary: #1e1e1e;
    --of-bg-secondary: #252525;
    --of-text-primary: #e0e0e0;
    --of-accent: #3b82f6;
    --of-border: #333333;
    --of-monaco-bg: #0d0d0d;
  }

主题: 浅色/深色/高对比度 (WCAG AAA) → 用户设置页可选
组件强制使用 var(--of-*) 不使用硬编码颜色
```
- JWT 不放入 URL query string (会被服务器日志/反向代理/浏览器历史记录泄露)
- 改为连接后首帧发送 `{type:"auth", token}` (channel-level auth)
- Token 为 BFF 签发的 1h 短期令牌，前端每 55min 静默续期

**XSS 防护**:
- Agent 流式输出: 渲染前经过 `DOMPurify.sanitize(md, {ALLOWED_TAGS:[...]})`
- 白名单: `h1-h6/p/code/pre/ul/ol/li/strong/em/a/table/blockquote`
- `<script>/<iframe>/<object>/<embed>/on*` 全部 strip
- 代码块: Monaco Editor (只读模式) 渲染，非 innerHTML
- CSP Header: `default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; connect-src 'self' wss:`

**WebSocket 心跳与退避重连**:
```
心跳: client 每 30s → ping, server 10s 内 → pong
      3 次 ping 无 pong → 断开 → 重连
重连: 指数退避 1s→2s→4s→8s→16s→30s(max)
      reconnect → {type:"sync.request", seq:last_seq}
```

**Monaco 多实例内存管控**:
- Diff View 最多同时打开 4 个文件
- 超出 LRU 淘汰 (关闭最久未使用的 Diff)
- 每个 Monaco 实例 Web Worker 独立，共享 TypeScript 语言服务

**逐 token 渲染防抖** (rAF 批处理):
- 每 16ms (1 帧) 批量 apply pending deltas → 单帧单 DOM 操作
- 避免 100 tokens/s → 100 次 DOM 更新/秒 的渲染风暴

---

## 六、安全性

> **Phase 2 前置**: 本章节 JWT/XSS/CSP 3 个高危项必须在 Phase 2 前端上线前闭环。

### 6.1 Prompt 注入防护 (三明治架构)

```
System Zone (不可被用户内容污染) |
Data Zone (用户/代码/文件内容)    | 消息组装时隔离
Output Zone (Agent 输出受限结构)  |

输入清洗: 过滤 "SYSTEM:" / "指令" 等标记
模型层: Agent 操作仅通过 tool_use content block 执行
```

### 6.2 沙箱逃逸防护 (5 层纵深)

```
L1: Trivy 漏洞扫描 + 基础镜像白名单
L2: --read-only + --cap-drop=ALL + cgroup 限制 (2G/2CPU/100pids)
L3: seccomp profile (禁用 mount/kexec/reboot/ptrace)
L4: 网络隔离 (仅内网 registry + localhost)

Terminal 安全分级:
  只读 Terminal:  所有用户可用，仅展示沙箱 stdout/stderr, 不接受输入
                  前端 `readonly` 属性 + BFF 拒绝 terminal.input 事件
  调试 Terminal:  仅 Dev 环境 + Tech Lead 角色
                  每次打开需输入二次确认码 (2FA),
                  所有输入记录完整审计 (WORM)
```

### 6.3 供应链安全

- 依赖白名单 + Version pinned + 包名 typo-squatting 检测
- Cosign 镜像签名验证
- npm audit 自动阻断 Critical/High

### 6.4 鉴权链路

```
用户(SSO) → BFF(JWT) → Pipeline(created_by透传) →
  Agent操作: Git(Bot Token, author="用户 via OpenForge")
  审计日志: actor = 用户身份 (透传)
```

### 6.5 密钥管理

- HashiCorp Vault 管理全部密钥 (LLM Key / Git Token / Docker Cred / DB Password)
- Vault Agent Sidecar 注入 → 应用层零硬编码
- 全链路日志脱敏 (API Key / Token / Password 正则替换)

**Ed25519 密钥生命周期** (Phase 7):

```
Profile 签名私钥 → offline 生成 → encrypted 存储
公钥 → 部署时注入 (环境变量 / ConfigMap, 可读但不可写)
私钥 → 仅 CI/CD 签名步骤可访问, 运行时不可访问

轮换: 每 90 天, 旧公钥保留 180 天 (过渡期内新旧签名均有效)
撤销: 私钥泄露 → 立即轮换, 旧公钥 immediate 撤销
验证: 启动时验签 profile → 验签失败 → 拒绝启动
      运行时验签: 每 24h 重新验签 (防止运行时篡改)

信任锚: 私钥持有者 = Tech Lead + DevOps Lead (两人分持, 需双人操作)
       签名操作记录完整审计 (谁/何时/对哪个 profile 签名)
```

### 6.6 审计日志防篡改

- WORM 双写 + 超级用户也无法绕过:
  - Postgres: REVOKE DELETE/UPDATE/TRUNCATE/DROP on audit_log (DB-level)
    - 应用用户仅 INSERT 权限
    - DBA 角色无 DELETE/UPDATE 权限 (权限分离, 安全管理员 vs 数据库管理员)
    - 任何对 audit_log 的 DDL 操作触发实时告警 #of-security + 锁定账户
  - MinIO: Object Lock GOVERNANCE mode (Retention 7 年)
    - GOVERNANCE: 无 `s3:BypassGovernanceRetention` 权限的任何用户 (含 root) 无法删除
    - 该权限仅授予安全审计员角色, 且每次使用记录完整审计
  - 完整性哈希链: `{event_id, prev_hash, content_hash}` → 链式可验证
    - 链断裂检测: 每小时自动扫描全链, 断裂 → P0 告警 + 冻结 Pipeline 创建

**审计事件五元组** (所有审计事件强制包含):
```
{who, what, when, where, result}

who:   actor (用户/Agent 身份)
what:  action + resource (如 gate.approve + pipeline-42/stage-impl)
when:  ISO8601 timestamp (毫秒精度)
where: source_ip + user_agent + region
result: outcome (success/failure) + error_code (if failure) + artifact_hash
```

### 6.7 Gate TOCTOU 防护

```
审批时: artifact_hash = SHA256(all_files)
下游使用前: 重新计算 → 不匹配 → 阻断 + 告警
关联 Artifact: 审批后只读挂载
```

### 6.8 数据隔离

- MinIO Bucket Policy per project
- 嵌入索引 namespace 前缀强制验证
- BFF middleware: 每请求验证 project membership

### 6.9 Gateway 防护

```
限流: 50 req/s/IP + 100 req/s/project
WS鉴权: 连接时 JWT 验证 + 30s ping/pong + max 3 conn/user
CORS: 仅工作台域名
CSP: default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; connect-src 'self' wss:
HSTS: max-age=31536000
TLS: 1.2+, 内网 CA 签发, cert-manager 自动续期
```

### 6.10 法律/IP 风险 ⚖️

**AI 生成代码的 License 污染**: Agent System Prompt 注入"不生成 GPL/AGPL 代码"指令 + CI 代码相似度检测 + SBOM 生成阻断 (Syft, GPL/AGPL 阻断) + Gate 审批人确认 Checklist ("已审查代码，未发现 License 冲突")。

**代码语义相似**: 除文本比对外，做 AST 结构相似度比对。每段生成代码记录溯源上下文 (基于哪个需求/偏好生成，未引用外部仓库)。

**代码归属权**: Git Author="用户 (via OpenForge)", Committer="pipeline-bot"。使用条款明确"审批该代码的自然人/企业持有版权"。GDPR 预留: 用户离职可删除 PII + 匿名化审计 actor。

**审批人法律责任**: Gate 审批 Checklist 明确提示"作为审批人，你对此审批决定承担最终责任"，审批记录 WORM 留痕。

---

## 七、高可用

### 7.1 级联故障防护

每个外部依赖独立熔断器 (参数经实测修正):
```
LLM:     timeout 120s, 连续5次失败→OPEN(120s), 降级: Pipeline 挂起 + 自动重试
         half-open: 120s后放 1 个探测请求 → 成功 CLOSE / 失败重置 120s
Docker:  timeout 120s, 连续3次→OPEN(60s),  降级: 沙箱排队 + 通知运维扩容
MinIO:   timeout 30s,  连续3次→OPEN(60s),  降级: 本地缓冲写 + 恢复后异步同步
Postgres: timeout 10s/query, pool max 50 (PgBouncer 事务级复用),
          主库故障 → 自动切只读副本 (读路径) + 写返回 503
CSP:     buffer上限+背压, 不 goroutine 泄漏
WAL:     消息去重 (msg_hash 幂等), 自身 CRC32 校验 → 损坏消息跳过 + 告警
```

### 7.2 优雅降级 (三级 + MinIO 补充)

```
GREEN:   全功能
YELLOW:  LLM不可用 → Pipeline挂起+排队
         Docker不足 → 沙箱排队 + 通知运维
         MinIO不可用 → 本地缓冲 (读 fallback 本地, 写落本地)
RED:     Postgres主库不可达 → 只读副本读, 新Pipeline返回503
         MinIO + 本地缓冲同时不可用 → Pipeline 暂停, 等待存储恢复
```

### 7.3 健康检查

```
/health/live:  进程存活 + goroutine 死锁检测 (30s ping all channels)
/health/ready: Postgres可达 + MinIO可达 + LLM可达 + Docker可达
/health/debug: pprof + learning rollback + force GC
```

### 7.4 部署拓扑

```
K8s 多活:
  BFF × 2+ (LB)
  Go Coordinator × 3 (一致性哈希分片, 各自 Master)
  Node.js IO × 2+ (无状态)
  Postgres 主从 + PgBouncer
  MinIO + Redis
```

### 7.5 数据完整性

- MinIO: PUT 带 SHA256, GET 校验 → 不匹配重试 + 副本拉取
- 学习快照: Ed25519 签名, 加载前验签 → 不通过回退

### 7.6 灾备 (DR) 计划

HA 解决单组件故障，DR 解决整个 Region 挂掉:

| 等级 | 场景 | RPO | RTO | 恢复方式 |
|------|------|-----|-----|---------|
| L1 | 单组件故障 | 0 (同步复制) | < 1min | 自动切 Replica |
| L2 | 单机故障 | 检查点 10轮 | < 2min | 自动切 Standby |
| L3 | 单 AZ 故障 | < 6h | < 30min | 切备 AZ |
| L4 | 单 Region 故障 | < 24h | < 4h | 切备 Region |
| L5 | 全 Region 损毁 | < 24h | < 24h | 异地冷备恢复 |

备 Region 常驻 PG Standby + MinIO Mirror，切换时冷启动其余组件 → DNS/LB 切流量 → 从检查点恢复飞行中 Pipeline。每季度 L3 演练，每年 L4 演练，记录实际 RTO 更新 Runbook。每周备 Region 冒烟测试。

---

## 八、高并发

### 8.1 Orchestrator 去中心化

```
按 project_id hash 分片 → 每个 Coordinator 独立管理分片
Pipeline 实例自治 (创建后不经过中心)
吞吐提升 ~10x
```

### 8.2 Sandbox 预热池

```
warm: 10 containers (idle, 预装依赖层缓存)
分配: warm取出 → bind mount worktree (~100ms) → hot运行
归还: reset → 回 warm pool
水位自适应: warm<3 → 预热5个, cold>30 → 回收闲置
```

### 8.3 Postgres 读写分离

```
写: Primary (pipeline_state, gate_events, audit_log)
读: Replica×2 (配额查询, dashboard, 审计查询)
缓存: Redis (配额计数器, 限流窗口, Session, 热配置)
连接池: PgBouncer 事务级复用
```

### 8.4 文件级并发锁

见 A 层 3.5 节。

---

## 九、性能效率

### 9.1 关键优化

| 瓶颈 | 方案 | 节省 |
|------|------|------|
| 全量文件→LLM | Diff + 前5后5行上下文 | token -93% |
| 嵌入全量重建 | Delta + Base 增量 | O(n)→O(1) |
| 上下文重复注入 | Prompt 4层缓存 (L1-L4, L3/L4 受 5-min TTL 限制) | token ~40% |
| 测试串行 | 沙箱内并行波次 | 时间 -60% |
| LLM 语义不共享 | 结构化 Artifact + Embedding 传递 | 下游推理 -30% |
| 沙箱冷启动 | 预热池 + 依赖层缓存 | 5s→100ms |

### 9.2 日志采样

```
ERROR/WARN: 全量
INFO: 10% 采样
DEBUG: 仅 Debug 模式
Pipeline LLM 对话: 仅记 metadata, 完整内容仅在 Debug Trace
```

---

## 十、可移植性

### 10.1 基础设施能力剖面 (Capability Profile)

每个基础设施依赖封装为接口 + 多实现，通过 profile 配置文件一键切换，适配大/中/小公司需求：

```
config/profiles/
  ├── minimal.yaml    # 单机 Docker Compose, <10 人小团队
  ├── standard.yaml   # 单 AZ K8s, 50-200 人中型团队
  └── enterprise.yaml # 多 Region K8s, 大型企业
```

#### 10.1.1 完整可插拔矩阵

| 能力域 | 接口 | minimal 实现 | standard 实现 | enterprise 实现 |
|--------|------|-------------|--------------|-----------------|
| 容器编排 | `ContainerRuntime` | Docker CLI | Docker API (远程) | K8s Pod API |
| 密钥管理 | `SecretStore` | `.env` 文件 | Vault Agent Sidecar | Vault HA + Auto-unseal |
| 对象存储 | `ObjectStore` | 本地文件系统 | MinIO 单节点 | MinIO Cluster / S3 |
| 任务队列 | `TaskQueue` | PG SKIP LOCKED | Redis Streams | Redis Cluster Streams |
| 事件总线 | `EventBus` | goroutine channel (内存) | Redis Pub/Sub | Redis Cluster Pub/Sub |
| 缓存 | `Cache` | 内存 Map | Redis 单节点 | Redis Cluster |
| 可观测性 | `Telemetry` | stdout JSON 日志 | Prometheus + Loki | OTel Collector + Grafana |
| 服务发现 | `ServiceRegistry` | 静态配置 (YAML) | DNS SRV | K8s Service + DNS |
| 灾备 | `DisasterRecovery` | 本地 pg_dump | PG 流复制 Standby | Multi-Region Active-Passive |
| 负载均衡 | `LoadBalancer` | 单实例（无） | Nginx 反代 | K8s Ingress + Service Mesh |
| 命令执行 | `CommandExecutor` | local-shell (os/exec) | docker-sandbox | docker-sandbox + 5层纵深 |
| 审批通知 | `Notifier` | 终端 stdout | 飞书 Webhook (带重试) | 飞书 + 钉钉 + 邮件多通道 + 死信队列 |
| 命令执行 | `CommandExecutor` | `LocalShellExecutor` (宿主机直接 spawn) | `DockerSandboxExecutor` (容器内执行) | `DockerSandboxExecutor` + seccomp + 5 层纵深防护 |

> **C1 修复**: `MessageQueue` 拆为 `TaskQueue`(点对点FIFO) + `EventBus`(发布-订阅广播)。`Notifier` 增加重试/死信机制确保可靠送达。
>
> **v5 新增**: `CommandExecutor` — Bash 命令执行能力域。minimal 走本地 shell (对标 Claude Code 体验)，standard/enterprise 走 Docker Sandbox (对标 DeerFlow 安全模型)。危险命令硬阻断、路径限制、Profile 降级阻断。

#### 10.1.2 Go 注册表 + 启动装配

```go
// 接口定义（按实际通信模式拆分，避免语义泄漏）

// 任务队列: 点对点，FIFO/优先级，exactly-once 投递 (LLM 请求排队、Pipeline 创建排队)
type TaskQueue interface {
    Enqueue(ctx context.Context, topic string, msg Message, priority int) error
    Dequeue(ctx context.Context, topic string) (Message, error)   // 阻塞消费
    Ack(ctx context.Context, topic string, msgID string) error     // 确认消费完成
}

// 事件总线: 发布-订阅广播，at-least-once (Pipeline 完成 → 多消费者并行)
type EventBus interface {
    Publish(ctx context.Context, topic string, event Event) error
    Subscribe(ctx context.Context, topic string) (<-chan Event, error)
}

// 审批通知: 持久化 + 重试 + 死信 (Gate 通知不可丢失)
type Notifier interface {
    Send(ctx context.Context, target Target, msg Notification) error
    SendWithRetry(ctx context.Context, target Target, msg Notification, maxRetries int) error
}

// 命令执行: Bash 命令执行 (Agent 开发工具链 / 测试 / 构建)
// Profile 决定宿主机直接 spawn 还是 Docker 容器隔离
type CommandExecutor interface {
    Execute(ctx context.Context, command string, opts ExecOptions) (ExecOutput, error)
    ExecuteStream(ctx context.Context, command string, opts ExecOptions) (<-chan StreamChunk, error)
    Validate(ctx context.Context, command string, opts ExecOptions) error
}

// 其余接口 (无变化)
type ContainerRuntime interface { Create(ctx, spec) (Container, error); Start(id) error; Stop(id) error; Remove(id) error; List() ([]Container, error) }
type SecretStore      interface { Get(key) ([]byte, error) }
type ObjectStore      interface { Put(ctx, key, reader) error; Get(ctx, key) (io.ReadCloser, error); Delete(ctx, key) error; List(ctx, prefix) ([]string, error) }
type Cache            interface { Get(key) (any, error); Set(ctx, key, val, ttl) error; Del(key) error }
type Telemetry        interface { Trace(ctx, name) (Span, error); Metric(name, value, tags); Log(level, msg, fields) }
type ServiceRegistry  interface { Register(name, addr) error; Discover(name) ([]string, error); Watch(name) (<-chan Event, error) }
type DisasterRecovery interface { Backup(ctx) error; Restore(ctx, point) error; Status() (DRStatus, error) }
type LoadBalancer     interface { AddBackend(name, addr) error; RemoveBackend(name, addr) error; HealthCheck(name) (bool, error) }

// 启动装配 — 显式组成根 (Explicit Composition Root)
// 不使用全局注册表: 避免并行 UT 相互覆盖 + 编译器验证完整性
func Bootstrap(profile Profile) *OpenForge {
    return &OpenForge{
        Secrets:    newSecretStore(profile),
        Container:  newContainerRuntime(profile),
        TaskQ:      newTaskQueue(profile),
        EventBus:   newEventBus(profile),
        Cache:      newCache(profile),
        Telemetry:  newTelemetry(profile),
        Registry:   newServiceRegistry(profile),
        DR:         newDisasterRecovery(profile),
        LB:         newLoadBalancer(profile),
        Notifier:   newNotifier(profile),
        CommandExec: newCommandExecutor(profile),
    }
}

// 每个工厂函数根据 profile 字段选择实现, 无全局状态
func newTaskQueue(p Profile) TaskQueue {
    switch p.TaskQueue {
    case "pg-skip-locked":  return NewPGTaskQueue(p.DB)
    case "redis-streams":   return NewRedisTaskQueue(p.Redis)
    default: panic("unknown task_queue: " + p.TaskQueue)
    }
}

func newEventBus(p Profile) EventBus {
    switch p.EventBus {
    case "goroutine-chan": return NewGoroutineEventBus()
    case "redis-pubsub":   return NewRedisEventBus(p.Redis)
    default: panic("unknown event_bus: " + p.EventBus)
    }
}
// 测试时直接注入 Mock: &OpenForge{TaskQ: mockTaskQ, EventBus: mockEventBus, ...}
```

#### 10.1.3 Profile 配置示例

```yaml
# minimal.yaml — 单机开发/小团队
profile: minimal
secret_store: envfile
container_runtime: docker
object_store: localfs
task_queue: pg-skip-locked      # TaskQueue: PG 轮询队列
event_bus: goroutine-chan       # EventBus: 进程内 channel
cache: memory
telemetry: stdout
service_registry: static
disaster_recovery: local-backup
load_balancer: none
notifier: stdout
command_executor: local-shell   # 宿主机直接 spawn, 对标 Claude Code

# enterprise.yaml — 大型企业多 Region
profile: enterprise
secret_store: vault-ha
container_runtime: k8s-pod
object_store: minio-cluster
task_queue: redis-cluster-streams  # TaskQueue: 跨节点优先级队列
event_bus: redis-cluster-pubsub    # EventBus: 跨节点广播
cache: redis-cluster
telemetry: otel-collector
service_registry: k8s-service
disaster_recovery: multi-region
load_balancer: k8s-ingress
notifier: multi-channel
command_executor: docker-sandbox   # 容器内执行 + seccomp + 5 层纵深防护
```

#### 10.1.4 Profile 安全护栏

```
安全等级标注:
  minimal:     security_tier=dev        # 仅开发/本地, 禁止生产
  standard:    security_tier=prod       # 生产可用
  enterprise:  security_tier=regulated  # 合规监管行业

降级阻断:
  enterprise → standard: 允许 (告警 + 审计记录)
  enterprise → minimal:  运行时拒绝 (FATAL: "enterprise→minimal prohibited in prod")
  standard  → minimal:   运行时拒绝 (FATAL: "standard→minimal prohibited in prod")

完整性校验:
  部署时: ./OpenForge-go serve --config profiles/standard.yaml --verify-signature
    → 读取 profiles/standard.yaml.sig (Ed25519 签名)
    → 验签失败 → 拒绝启动 + 告警 + 审计记录
  签名: Ed25519 (运维使用私钥对 profile + 实现版本列表签名)
  防篡改: 修改 profile → 签名失效 → 无法启动
```

#### 10.1.5 Profile 安全级别标注

```yaml
# minimal.yaml
profile: minimal
security_tier: dev             # ⚠️ 仅开发 — .env 明文密钥
secret_store: envfile          # Secrets 从 .env 读取 (开发用)
# ...

# standard.yaml
profile: standard
security_tier: prod            # 生产可用
secret_store: vault-sidecar    # Secrets 从 Vault 注入
# ...

# enterprise.yaml
profile: enterprise
security_tier: regulated       # 合规监管行业
secret_store: vault-ha         # Vault HA + Auto-unseal
# ...
```

#### 10.1.6 渐进升级路径

```
Phase 1-4 (开发验证):
  → minimal profile → 5 容器, 单机 Docker Compose
  验证: 核心闭环跑通

Phase 5-6 (内部试用):
  → minimal → standard 过渡
  验证: Redis 替换 PG 队列, MinIO 替换本地文件

Phase 7-8 (小规模生产):
  → standard profile → K8s 部署, HA 就绪
  验证: 按团队规模 SLO 达标

Phase 9-10 (企业级):
  → enterprise profile → 多 Region, 全合规
  验证: 大公司安全审计通过
```

每个阶段改的不是代码，是 profile 配置文件。Phase 1 仅需交付 minimal 实现 + 接口 stub（enterprise 实现随 Phase 9-10 引入），不要求 Phase 1 就写完所有实现。

**Profile 切换数据迁移** (minimal→standard→enterprise 存储后端变更):

```
迁移需要人工确认 Gate (P0 运维操作, 不允许自动触发):

切换流程:
  1. 运维修改 profile → 启动时检测存储后端变更
     → 输出迁移计划 (涉及哪些能力域 / 预估耗时 / 风险)
     → 等待人工确认 (CLI: "Proceed with migration? [y/N]")
  2. 逐个能力域迁移 (获得确认后):
     ├── localfs→MinIO:  后台异步同步 → 验证完整性 (SHA256 比对)
     ├── PG队列→Redis:   新消息写 Redis, PG 旧消息消费完毕后切换
     ├── 内存→Redis:     预热回放 PG → 验证命中率 > 95% → 切换
     └── stdout→OTel:    双写 → 验证 → 停止 stdout
  3. 迁移完成 → 原子切换 → 旧后端保留 72h → 确认无误后清理
  4. 异常 → 停止迁移 → 人工选择 [回滚/跳过/重试]

预检项完整清单 (Phase 7):
  部署前:
    ├── schema drift:    DB 迁移版本一致性检查
    ├── 端口冲突:        9100-9200 端口可用性
    ├── Vault 可达性:    vault status
    ├── 证书过期:        TLS 证书剩余天数 > 30
    ├── DNS 解析:        of-api.corp.internal 可解析
    ├── 磁盘空间:        /var/OpenForge > 20G
    ├── Docker 可用:     docker info
    └── Profile 签名:    Ed25519 验签通过
```

#### 10.1.7 接口演化预案

Go 接口新增方法会破坏所有现有实现。使用**组合接口**模式:

```go
// Phase 1: 基础接口 (不会改变)
type SecretStore interface {
    Get(ctx context.Context, key string) ([]byte, error)
}

// Phase 9+: 扩展接口 (组合, 不破坏基础接口)
type RotatableSecretStore interface {
    SecretStore                           // 嵌入基础接口
    Rotate(ctx context.Context, key string) ([]byte, error)
}

// 使用方: 类型断言检查扩展能力
func rotateIfSupported(store SecretStore, key string) ([]byte, error) {
    if rs, ok := store.(RotatableSecretStore); ok {
        return rs.Rotate(ctx, key)
    }
    return nil, ErrNotSupported  // 优雅降级
}
```

**所有 10 个能力域接口均采用此模式**: 基础接口不可变（Phase 1 定义后冻结），未来能力通过组合接口 + 类型断言扩展。

#### 10.1.8 额外收益

- **可测试性** — 测试时注入 Mock 实现，无需真 Docker/K8s
- **SaaS 化** — `enterprise` profile 即生产配置
- **开源友好** — 开源版发 `minimal` profile，企业版发 `enterprise` profile，同一份代码
- **零锁定** — 每个接口 2+ 实现，不被任何供应商绑定

### 10.2 嵌入模型注册中心

```yaml
default: bge-small-zh (中文)
fallback: all-MiniLM-L6-v2 (英文)
支持热切换 + 多模型按语言选择
```

### 10.3 项目模板 (Conduit 解耦)

```
templates/monorepo-node-react/
  module-map.yaml + seed-knowledge/ + sandbox.Dockerfile

自动模板生成: 分析仓库 → 识别技术栈 → 提取风格 → 人工审核确认
```

### 10.4 平台适配

```
Go: runtime.GOOS → 自动选适配器 (linux/darwin/windows)
Node: process.platform → 动态 import (当前仅实现 Linux)
```

### 10.5 离线部署包

```
OpenForge-offline-v1.0.0/
  ├── binaries/                    # Go Coordinator (linux-amd64/arm64)
  ├── nodejs/                      # Node.js IO 层 (含依赖 + 嵌入模型.onnx)
  ├── containers/                  # Sandbox 镜像导出 (.tar.gz)
  ├── config/                      # {base,airgap}.yaml + generate.sh
  ├── migrations/                  # Postgres 迁移脚本
  ├── docs/                        # airgap-deploy-guide.md
  ├── bootstrap.sh                 # 一键部署脚本
  └── manifest.yaml                # 物料清单 + SHA256 校验和

bootstrap.sh 流程:
  1. 预检: Docker ≥ 24, Postgres ≥ 16, 内存 ≥ 16G, 磁盘 ≥ 50G
  2. 加载镜像: docker load < containers/sandbox-node.tar.gz
  3. 初始化 DB: psql -f migrations/001-init.sql
  4. 配置生成: config/generate.sh --company corp --region bj
  5. 启动: ./OpenForge-go serve --config config/airgap.yaml
  6. 自检: curl :9091/health/ready → 200 OK
```

---

## 十一、扩展性

### 11.1 水平扩展

```
BFF:        无状态, LB 直接扩展
Go Coordinator: 一致性哈希分片, rebalance 迁移 (秒级)
Node.js IO: 无状态, 多实例
Sandbox:    独立节点 + pool-agent 注册
```

### 11.2 多 Region 预留

```
region 字段已在 config/DB schema 中
RPC 调用带 X-Region 路由头
MinIO Bucket: of-{region}-artifacts
LLM Router: 就近调度 + 跨 region failover (预留, 不实现)
```

### 11.3 Postgres 分库预留

```
connection 接口已抽象支持多实例
pipeline_state 表已带 project_id 分库键
跨项目查询走 Redis 缓存聚合
```

### 11.4 LLM 推理扩容

```
多 GPU 节点 + 负载感知调度 (最少排队/最低利用率)
P3 降级: GPU 满 → CPU 推理 (llama.cpp)
```

### 11.5 MinIO 异步写入

```
本地缓冲 (Coordinator 磁盘) → 返回成功 → 后台异步上传
读取优先本地 → 未命中 MinIO → 自动 GC
```

---

## 十二、可观测性

### 12.1 遥测栈

```
OTel Trace: W3C Trace Context 跨 Go-Node 传播
  traceparent: 00-{trace_id}-{span_id}-01
  tracestate: of_pipeline_id=pipe-42,of_stage=impl

结构化日志: JSON Lines → stdout → Loki/Promtail 采集
  { "ts":"ISO8601", "level":"info", "msg":"...",
    "trace_id":"abc", "span_id":"def",
    "pipeline_id":"pipe-42", "stage":"impl",
    "project_id":"proj-A", "user":"alice@corp" }

Prometheus Metrics (:9090)
Grafana Dashboards (导入 JSON, 不自建实例)
```

### 12.2 核心 SLO/SLI (Phase 8 目标值, 待实测修正)

| SLI | Phase 8 SLO 目标 | 测量点 | Phase 2-4 测量开始 |
|-----|-----------------|--------|-------------------|
| Pipeline 创建延迟 | p99 < 2s | BFF → Coordinator | Phase 3 |
| Agent 首轮响应 | p99 < 15s | LLM Router | Phase 2 |
| Gate 通知送达 | p99 < 30s | BFF → 飞书/钉钉 | Phase 3 |
| Staging 部署耗时 | p99 < 5min | Docker + CI | Phase 4 |
| Pipeline 成功率 | > 85% | pipeline_state | Phase 3 |
| 知识回写延迟 | p99 < 10min | Learning Engine | Phase 7 |

**SLO 基线测量计划** (每 Phase 基于实测更新):

```
Phase 2 (Web 对话上线即开始):
  → 收集 100 次 Agent 对话的首轮响应时间分布
  → 人均对话轮次分布
  → LLM API 失败率 (按 provider/model)
  → 设定 Phase 2 专用宽松 SLO 窗口

Phase 3 (Pipeline 状态机 + Diff):
  → Pipeline 端到端耗时分布 (按 L1-L4 分级)
  → 代码驳回率分布
  → 沙箱执行成功率
  → 更新 SLO 为实际基线值

Phase 4 (完整闭环):
  → 首版正式 SLO 基于 500+ Pipeline 真实数据
  → 后续每个 Phase 结束时基于新增数据更新 SLO
```

### 12.3 Metrics 注册表 (Phase 7 启用基数控制)

> **基数控制**: 移除 `pipeline_id` 作为 label (高基数: 每个 Pipeline 唯一值 → 百万级时间序列)。改用 project/level/provider/model 等有界标签 (< 1000 唯一值)。

```
Pipeline:
  of_pipeline_created_total{project, level}         # Counter
  of_pipeline_completed_total{project, level}       # Counter
  of_pipeline_duration_seconds{project, level}      # Histogram
  of_pipeline_stage_duration_seconds{stage}         # Histogram
  of_pipeline_active_count{project}                 # Gauge

Agent:
  of_agent_llm_call_duration_seconds{provider,model}# Histogram
  of_agent_llm_call_errors_total{provider,error}    # Counter
  of_agent_llm_token_usage_total{project,provider,model}# Counter
  of_agent_tool_call_total{tool}                    # Counter
  of_agent_backtrack_total{project}                 # Counter (从 pipeline_id 降级为 project)

Gate:
  of_gate_wait_duration_seconds{stage}              # Histogram
  of_gate_approve_total{stage,project}              # Counter
  of_gate_reject_total{stage,project}               # Counter
  of_gate_timeout_total{stage}                      # Counter
  of_code_acceptance_rate{project}                  # Gauge

Sandbox:
  of_sandbox_pool_warm_count                        # Gauge
  of_sandbox_pool_cold_count                        # Gauge
  of_sandbox_startup_duration_seconds               # Histogram
  of_sandbox_test_duration_seconds                  # Histogram
  of_sandbox_test_results{result}                   # Counter

Learning:
  of_learning_snapshot_size_bytes                   # Gauge
  of_learning_ab_experiment_active_count            # Gauge
  of_learning_knowledge_promoted_total              # Counter
  of_learning_knowledge_discarded_total             # Counter
  of_learning_rollback_total                        # Counter

Infra:
  of_coordinator_goroutine_count                    # Gauge
  of_coordinator_channel_depth{name}                # Gauge (name 有界: < 50)
  of_coordinator_deadlock_detected                  # Counter
  of_circuit_breaker_state{component}               # Gauge (0=CLOSED, 1=OPEN, 2=HALF_OPEN)
```

### 12.4 核心 SLO/SLI (详见 12.2) + 补充 SLI

| SLI | SLO | 测量方式 |
|-----|-----|---------|
| Pipeline 端到端耗时 | L1: p99<15min, L2: p99<1h, L3: p99<4h | `of_pipeline_duration_seconds{level}` |
| 代码审查延迟 | p99 < 2h (从 Gate 通知到审批完成) | `of_gate_wait_duration_seconds` |
| 文件锁等待 | p99 < 30s | 从锁请求到获取的 Δt |

### 12.5 告警规则 (完整)

```
Pipeline:
  Pipeline 成功率 < 85% (1h窗口)          → #of-ops warning
  单 Pipeline 回溯 > 3 次                  → #of-ops + PM
  活跃 Pipeline 数 < 预期 (CRITICAL 水位)  → #of-ops critical

Agent/LLM:
  LLM 调用失败率 > 10% (5min)             → #of-ops warning
  LLM 熔断器 OPEN                         → #of-ops + 飞书 @oncall
  Token 消耗速率 ×3 (基线对比)              → PM + #of-ops
  单 Pipeline Token 超限                  → PM (三选一卡片)

Sandbox:
  Sandbox 预热可用 < 2                     → #of-ops critical
  Sandbox 启动延迟 p99 > 2s               → #of-ops warning

Gate:
  Gate 审批超时 (>48h)                     → PM Lead 升级
  Gate 审批认领冲突                       → 审批双方 + PM
  代码接受率 < 50% (10 Pipeline 窗口)      → Tech Lead 升级 ⚠️ 产品止损

文件锁:
  文件锁死锁检测触发                       → 双方 PM + #of-ops
  文件锁等待 > 2× 预估时间                 → #of-ops warning

Learning:
  学习快照验签失败                        → #of-security critical
  知识回滚自动触发                        → #of-ops + Tech Lead
  AB 实验连续 3 个有害结论                → Tech Lead 升级

MinIO:
  MinIO 积压 > 1000 文件或 > 10min       → #of-ops warning
  MinIO 副本不同步 (>5min)                → #of-ops critical

审计:
  审计日志写入失败 (>0, 5min)             → #of-security critical
  审计哈希链验证断裂                      → #of-security critical
```

---

## 十三、可测试性

### 13.1 测试金字塔

```
E2E (10条):    完整 Pipeline 端到端 (Mock LLM + 真实 Conduit)
集成 (50条):    Go↔Node 契约测试 + Postgres/MinIO 集成 + Sandbox + Auth + HA 恢复
单元 (200+条):  Go (Channel/文件锁/状态机) + Node (LLM Router/Tool/索引/学习)
```

### 13.2 Mock 策略

```
LLM Mock:     录制回放 (按 prompt_template 版本管理录制, 变更时重新录制)
              → 覆盖: 正常/幻觉/超时/截断/注入 5 场景
Sandbox Mock: Docker-in-Docker 或 Mock Docker API
              → 安全层 (seccomp/cap-drop/net-iso) 无法 Mock,
                 依赖 CI 中真 Docker 集成测试覆盖
知识库 Mock:  预置快照 → 恢复 → 验证学习行为
              → 种子知识 10 套快照 (Conduit 各模块)
```

### 13.3 契约测试 (双向)

```
Go→Node:  Go client 调 Node HTTP → 断言 proto 响应
Node→Go:  Node 回调 Go (如 LLM 完成通知) → 断言 Go 正确处理
每次 Go/Node PR 都在 CI 中运行

向后兼容: buf breaking change 检测设为 FILE 级别 —
          新增字段/方法允许, 删除/重命名字段拒绝
```

### 13.4 故障注入 (扩展覆盖)

```
工具: toxiproxy (网络) + 自定义 chaos-controller (应用层)

场景:
  网络: LLM 超时 / MinIO 延迟 / Postgres 连接断开 / 网络分区
  资源: 磁盘写满 / CPU 满载 / 内存 OOM / FD 耗尽
  时间: 时钟偏移 (NTP 故障) / 证书过期
  进程: Coordinator 随机 kill / Node.js OOM / Docker daemon 重启
  数据: Postgres WAL 损坏 / MinIO bit rot / 快照签名不匹配

断言: 熔断/降级/恢复行为符合预期 + 不丢数据 + 审计日志完整
```

### 13.5 性能基准

```
Pipeline 吞吐 (>50/min) / Channel 延迟 (p99<10ms)
嵌入查询 (p99<50ms) / 沙箱分配 (p99<500ms)
CI 中每次 PR 与 baseline 对比 → 退化 >15% 且重现 3 次 → 失败
(防止单次环境噪声误报)
```

### 13.6 公认难测组件

以下组件承认测试覆盖有限，标注风险:

| 组件 | 难点 | 缓解 |
|------|------|------|
| 自学习 A/B 测试 | 需 50 Pipeline+ 才能得出结论 → UT 无法验证 | 预置快照回放 + CI 中模拟实验 |
| 嵌入索引语义正确性 | "正确匹配" 无真值 → 依赖人工评估 | Phase 2 起每周人工抽样 20 条评估 |
| Pipeline 回溯组合爆炸 | 3 次回溯 × N 阶段 → 状态空间大 | 等价类划分 + 边界测试 |
| 多 Coordinator 分片 | 并发 race condition 难以重现 | Go race detector + CI stress test |
| LLM 流式中断恢复 | 网络中断时机不可控 | toxiproxy 注入 + replay 验证 |
| Sandbox 安全层 | seccomp/cap-drop 不可 Mock | 仅 CI 真 Docker 环境覆盖 |

---

## 十四、可部署性 / 运维性

### 14.1 容器化

```
BFF + Go Coordinator + Node.js IO → 各自 Dockerfile
K8s Deployment + Service + Ingress
配置通过 ConfigMap + Vault 注入
```

### 14.2 滚动更新

```
maxSurge: 1-2, maxUnavailable: 0 (零停机)
Go Coordinator: 优雅下线 (完成检查点 → 迁移分片 → 释放锁 → 退出)
terminationGracePeriodSeconds: 60-120s
```

### 14.3 配置中心

```
config/{base,dev,staging,prod,airgap}.yaml → viper (Go) + envsubst (Node/Nginx)
敏感值走 Vault
Feature Flag: 热加载, 无需重启
```

### 14.4 数据库迁移

```
迁移脚本: {version}_{name}.up.sql + .down.sql
部署前: 自动 Up (事务)
部署失败: 自动 Down
生产: Up-only (回滚需人工)
```

### 14.5 Feature Flag 机制与生命周期治理

运行时开关，热加载无需重启，Phase 7 加入生命周期管理:

```yaml
feature_flags:
  - name: dsl_pipeline
    owner: platform-team
    created: 2026-05-01
    expires: 2026-09-01
    status: stable              # experimental → beta → stable → deprecated
    rollout: 100
  - name: learning_v4
    owner: ml-team
    created: 2026-06-01
    expires: 2026-10-01
    status: beta
    rollout: 30
  - name: debug_repl
    owner: platform-team
    status: dev-only            # 仅 dev/staging, 生产永 false
```

生命周期: experimental (30d) → beta (60d) → stable (永久) → deprecated (30d 缓冲) → 移除。过期前 30d 告警 owner。C 层 `/admin/flags` 管理 UI。
存储: Postgres + Redis 缓存，变更走配置中心 → 各层热加载。

### 14.6 可调试性 (Debug Trace + REPL)

Debug Trace 默认关闭，按需开启 (避免存储膨胀):

```
开启策略:
  正常 Pipeline:  仅记 metadata (轮次/耗时/token), 不含完整内容
  异常 Pipeline:  自动开启 (失败/驳回/回溯 > 1 次)
  人工请求:       PM/Dev 在 Pipeline 设置中勾选 "完整调试日志"
  保留:           30 天热存储 → 压缩归档 90 天

Debug Trace 内容 (开启时):
  ├── 每轮 Agent 对话 (user/assistant/system 完整记录)
  ├── 每次 Tool 调用 (tool_name, args, result)
  ├── Channel 消息流 (谁→谁, 消息体)
  ├── Gate 事件 (谁审批, 审批意见, 时间)
  └── 错误/异常 (完整堆栈 + 上下文)

回放: :9092/debug/pipeline/:id/replay
  → 完整执行轨迹 (JSON Lines, 可下载)
  → 开发者在本地用 replay-viewer 逐轮回放

REPL 模式 (仅 Dev 环境):
  :9092/debug/pipeline/:id/repl
  → 交互式调试会话
  → 手动注入 prompt / 修改 Agent 状态 / 跳过某轮
```

### 14.7 沙箱镜像 CI Pipeline

```
触发: 每周一 03:00 + 基础镜像更新时

Step 1: docker build --no-cache
  ├── FROM node:22-alpine (版本锁定)
  ├── 预装 Conduit 测试依赖 (npm test deps)
  └── 预装 golangci-lint / 安全扫描工具

Step 2: Trivy scan → Critical/High 阻断
Step 3: Cosign sign → Harbor sandbox-images/
Step 4: 冒烟验证 — npm test / go test → 通过 → 标记 :vYYYYMMDD + :latest
Step 5: 通知 #ops "沙箱镜像 v20260519 已发布"

镜像版本锁定: Sandbox Pool 使用 :vYYYYMMDD (非 :latest)
灰度升级: 新 Pipeline 用新镜像, 运行中不受影响
回滚: 配置改回旧标签 → 重启预热池
```

### 14.8 部署前冲突检查

```yaml
# preflight.yaml — 部署前自动扫描
checks:
  - name: minio_bucket_conflict
    command: "mc ls minio/of-pipeline-artifacts || true"
    expect: "Bucket does not exist"
  - name: redis_key_conflict
    command: "redis-cli KEYS 'sa:*' | wc -l"
    expect: "0"
  - name: prometheus_metric_conflict
    command: "curl -s prometheus:9090/api/v1/label/__name__/values | grep 'of_'"
    expect: ""
  - name: git_branch_protection
    command: "gitlab project-protected-branches --pattern 'of-*'"
    expect: "No protected branches"
```

### 14.9 Runbook

```
扩容 SOP / 灾备恢复 / 知识库回滚 / 熔断恢复 / 僵尸资源清理
半自动化: 一键操作 + 预设脚本 + 人工确认节点
架构决策记录 (ADR): docs/adr/ 记录关键架构决策及原因
API 文档: OpenAPI 从 BFF 代码注解自动生成
```

---

## 十五、合规性

### 15.1 数据生命周期

| 数据类型 | 热存储 | 温存储 | 删除 |
|---------|--------|--------|------|
| Pipeline Diff | 进行中+90d | 1年 | 1年后 |
| Agent 对话 | 30d | 90d | 90天后 |
| 审计日志 | 全量(WORM) | 3年归档 | 依法规 |
| 学习轨迹 | 全量 | — | 去个性化保留 |
| 成本数据 | 30d | 1年 | 1年后聚合 |

### 15.2 加密

```
传输: TLS 1.2+ (外部) + mTLS (内部)
静态: Postgres TDE + MinIO SSE-S3 + Redis AES-GCM (敏感key)
GDPR 预留: 用户离职 → 删除 PII + 匿名化审计日志 actor
```

### 15.3 合规报告 (每月自动)

```
审计报告 + 访问报告 + 数据报告 + License 审计
SBOM 生成 (Syft) → GPL/AGPL 阻断
```

---

## 十六、扩展路径

| Phase | 范围 | 部署拓扑 |
|-------|------|---------|
| Phase 1 | 单机全栈 (开发验证) | Go + Node + PG + MinIO + Redis 全单机 |
| Phase 2 | 服务分离 (小规模生产) | BFF×2 + Go×1 + Node×2 + PG主从 + MinIO |
| Phase 3 | 水平扩展 (中规模) | BFF×N + Go×M分片 + Node×K + PG主从+副本 + Redis Cluster |
| Phase 4 | 推理扩容 (GPU 密集) | 加 vLLM 节点 + LLM Router 就近调度 + 优先级队列分离 |

---

## 十七、技术栈

| 层 | 技术 | 理由 |
|----|------|------|
| BFF | Node.js (tRPC/gRPC) | 与 IO 层同语言, 减少上下文切换 |
| 前端 | React + Monaco Editor + WebSocket | 代码审查(Diff View) + 实时推送 |
| Pipeline 引擎 | Go | goroutine=实例, channel 原生, 单二进制 |
| Agent 运行时 | Go (协调) + Node.js (IO) | Go 管并发, Node 管 MCP+LLM+Skill |
| 存储 | Postgres + MinIO + Redis | 关系型状态 + 对象产物 + 缓存/队列 |
| 推理 | vLLM/Ollama (内网) | 气隙兼容 |
| 嵌入 | all-MiniLM-L6 / bge-small-zh | 本地 CPU, 不花钱 |
| 容器 | Docker + Sandbox Pool | 预热池, 100ms 分配 |
| 监控 | OTel + Prometheus + Loki | 对接已有平台 |

---

## 十九、数据库 Schema

### 19.1 ER 概览

```
project ──1:N──→ pipeline ──1:N──→ pipeline_stage
   │                  │                    │
   │                  ├──1:N──→ gate_event ├──1:N──→ checkpoint
   │                  ├──1:N──→ conversation_message
   │                  │              └──1:N──→ conversation_branch
   │                  └──1:N──→ file_lock
   │
   ├──1:N──→ user_role (per project RBAC)
   │
   └──1:N──→ module_ownership
   
"user" ──1:N──→ user_role

Global:
  audit_log       (WORM, 月分区)
  token_usage     (月分区)
  feature_flag    (global)
  knowledge_snapshot (per project, Phase 7)
  ab_experiment + ab_experiment_assignment (Phase 7)
  cost_quota      (per project)
  task_queue      (minimal: PG SKIP LOCKED)
```

### 19.2 完整建表语句

> **命名约定**: 表名统一单数 (rows 语义) | CHECK 命名: `chk_<table>_<column>` | FK 命名: `fk_<table>_<ref>` | PK 命名: `pk_<table>`
> 
> **Phase 1 范围**: 仅建以下表 — projects/users/user_roles/module_ownership/pipelines/pipeline_stages/gate_events/checkpoints/conversation_messages/conversation_branches/file_locks/token_usage/cost_quotas/audit_log/feature_flags/task_queue
> ab_experiments/ab_experiment_assignments/knowledge_snapshots 延至 Phase 7

```sql
-- ============================================================
-- 1. 项目与用户
-- ============================================================

CREATE TABLE project (
    id          TEXT CONSTRAINT pk_project PRIMARY KEY,  -- "proj-A"
    name        VARCHAR(255) NOT NULL,
    git_url     VARCHAR(512) NOT NULL,
    repo_type   VARCHAR(64) NOT NULL CHECK (repo_type IN ('monorepo-node-react','custom')),
    template    VARCHAR(64) NOT NULL DEFAULT 'custom',
    deleted_at  TIMESTAMPTZ,              -- 软删除 (Phase 2)
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    config      JSONB NOT NULL DEFAULT '{}'
);

CREATE TABLE "user" (                     -- 保留字引号
    id          VARCHAR(320) PRIMARY KEY,  -- "alice@corp" (SSO email, RFC 5321)
    display_name VARCHAR(128) NOT NULL,
    avatar_url  VARCHAR(512),
    disabled_at TIMESTAMPTZ,              -- 离职/GDPR 禁用 (Phase 2)
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_login  TIMESTAMPTZ
);

CREATE TABLE user_role (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     VARCHAR(320) NOT NULL REFERENCES "user"(id),
    project_id  TEXT NOT NULL REFERENCES project(id),
    role        VARCHAR(16) NOT NULL CHECK (role IN ('admin','pm','dev_lead','dev','observer')),
    modules     TEXT[] NOT NULL DEFAULT '{}',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(user_id, project_id)
);

CREATE TABLE module_ownership (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id  TEXT NOT NULL REFERENCES project(id),
    module_name VARCHAR(64) NOT NULL,
    paths       TEXT[] NOT NULL,
    team_name   VARCHAR(128) NOT NULL,
    reviewers   TEXT[] NOT NULL,
    fallback_reviewer VARCHAR(320) NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(project_id, module_name)
);

-- ============================================================
-- 2. Pipeline 核心
-- ============================================================

CREATE TABLE pipeline (
    id              TEXT PRIMARY KEY,          -- "pipe-42"
    project_id      TEXT NOT NULL REFERENCES project(id),
    parent_pipeline_id TEXT REFERENCES pipeline(id),
    title           VARCHAR(512) NOT NULL,
    level           VARCHAR(4) NOT NULL CHECK (level IN ('L1','L2','L3','L4')),
    status          VARCHAR(16) NOT NULL DEFAULT 'pending'
                    CHECK (status IN ('pending','running','paused','awaiting_review',
                                      'completed','rejected','token_exceeded','cancelled','dormant')),
    current_stage   VARCHAR(10) CHECK (current_stage IN ('clarify','decompose','impl','test','deploy','verify')),
    created_by      VARCHAR(320) NOT NULL,
    deleted_at      TIMESTAMPTZ,              -- 软删除, 区分"正常终结"和"用户删除" (Phase 2)
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at    TIMESTAMPTZ,
    region          VARCHAR(16) NOT NULL DEFAULT 'bj',
    config          JSONB NOT NULL DEFAULT '{}',
    backtrack_count INT NOT NULL DEFAULT 0 CHECK (backtrack_count >= 0 AND backtrack_count <= 3),
    version         INT NOT NULL DEFAULT 1    -- 乐观锁 (Phase 3)
);

CREATE INDEX idx_pipeline_project_status ON pipeline(project_id, status);
CREATE INDEX idx_pipeline_parent ON pipeline(parent_pipeline_id);
CREATE INDEX idx_pipeline_created_by ON pipeline(created_by, created_at);  -- Phase 2

CREATE TABLE pipeline_stage (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    pipeline_id     TEXT NOT NULL REFERENCES pipeline(id),
    stage           VARCHAR(10) NOT NULL CHECK (stage IN ('clarify','decompose','impl','test','deploy','verify')),
    status          VARCHAR(16) NOT NULL DEFAULT 'pending'
                    CHECK (status IN ('pending','running','awaiting_gate','passed','failed','skipped')),
    -- StageContext 结构化字段 (对应 Proto StageContext):
    requirement_summary TEXT,                 -- 阶段输入摘要
    constraints         TEXT[],               -- 约束列表
    preference_profile  TEXT,                 -- 项目偏好 (~500 tokens)
    module_index_subset TEXT,                 -- 模块索引子集
    -- 输出:
    summary         TEXT,                     -- 阶段输出摘要 (~700 tokens)
    artifact_ref    TEXT NOT NULL DEFAULT '', -- MinIO object key (content addressed: "artifacts/<hash>.tar.gz")
    artifact_hash   VARCHAR(64),              -- SHA256 of artifact (不可变)
    started_at      TIMESTAMPTZ,
    completed_at    TIMESTAMPTZ,
    schema_version  INT NOT NULL DEFAULT 1,
    version         INT NOT NULL DEFAULT 1     -- 乐观锁 (Phase 3)
);

CREATE INDEX idx_pipeline_stage_pipeline ON pipeline_stage(pipeline_id);

CREATE TABLE gate_event (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    pipeline_id     TEXT NOT NULL REFERENCES pipeline(id),
    stage           VARCHAR(8) NOT NULL,
    event           VARCHAR(16) NOT NULL
                    CHECK (event IN ('awaiting','approved','rejected','claimed','timeout','auto_bypassed')),
    actor           VARCHAR(320) NOT NULL,
    decision        VARCHAR(8) CHECK (decision IN ('approve','reject','comment')),
    line_comments   JSONB,
    summary_feedback TEXT,
    checklist       JSONB,
    artifact_hash   VARCHAR(64),
    prev_hash       VARCHAR(64) NOT NULL,
    content_hash    VARCHAR(64) NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
-- REVOKE DELETE/UPDATE/TRUNCATE/DROP on gate_event (WORM, Phase 6)

CREATE INDEX idx_gate_event_pipeline ON gate_event(pipeline_id);
CREATE INDEX idx_gate_event_actor ON gate_event(actor, event, created_at);  -- Phase 3 审批收件箱

-- ============================================================
-- 3. 检查点
-- ============================================================

CREATE TABLE checkpoint (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    pipeline_id     TEXT NOT NULL REFERENCES pipeline(id),
    stage           VARCHAR(8) NOT NULL,
    seq             INT NOT NULL,
    data            JSONB NOT NULL,
    trigger         VARCHAR(8) NOT NULL CHECK (trigger IN ('auto','manual')),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_checkpoint_pipeline_stage ON checkpoint(pipeline_id, stage DESC);

-- ============================================================
-- 4. 对话与分支
-- ============================================================

CREATE TABLE conversation_message (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    pipeline_id     TEXT NOT NULL REFERENCES pipeline(id),
    branch_id       VARCHAR(32) NOT NULL DEFAULT 'main',
    msg_seq         INT NOT NULL,
    role            VARCHAR(8) NOT NULL CHECK (role IN ('user','agent','system')),
    msg_type        VARCHAR(16) NOT NULL DEFAULT 'text'
                    CHECK (msg_type IN ('text','code_card','topo_card','gate_card','error_card')),
    content         TEXT NOT NULL,
    token_count     INT,
    reply_to_seq    INT,
    deleted_at      TIMESTAMPTZ,              -- 软删除(撤回), 保留 msg_seq 完整性 (Phase 2)
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(pipeline_id, branch_id, msg_seq)
);
-- UNIQUE 索引已覆盖 (pipeline_id, branch_id), 无需额外 idx_conv_msgs_pipeline

CREATE TABLE conversation_branch (
    id              VARCHAR(32) PRIMARY KEY,
    pipeline_id     TEXT NOT NULL REFERENCES pipeline(id),
    parent_branch   VARCHAR(32) NOT NULL DEFAULT 'main',
    fork_msg_seq    INT NOT NULL,
    status          VARCHAR(16) NOT NULL DEFAULT 'active'
                    CHECK (status IN ('active','merged','abandoned')),
    created_by      VARCHAR(320) NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- ============================================================
-- 5. 文件锁
-- ============================================================

CREATE TABLE file_lock (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    pipeline_id     TEXT NOT NULL REFERENCES pipeline(id),
    project_id      TEXT NOT NULL REFERENCES project(id),
    file_path       VARCHAR(512) NOT NULL,
    lock_type       VARCHAR(10) NOT NULL CHECK (lock_type IN ('write','read_only')),
    estimated_duration INT NOT NULL,
    locked_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at      TIMESTAMPTZ NOT NULL,
    UNIQUE(project_id, file_path)
);

CREATE INDEX idx_file_lock_project ON file_lock(project_id);

-- ============================================================
-- 6. Token 计量 (异步批量写入)
-- ============================================================

CREATE TABLE token_usage (
    id              BIGSERIAL NOT NULL,
    pipeline_id     TEXT NOT NULL,
    project_id      TEXT NOT NULL REFERENCES project(id),
    provider        VARCHAR(32) NOT NULL,
    model           VARCHAR(64) NOT NULL,
    prompt_tokens     BIGINT NOT NULL CHECK (prompt_tokens >= 0),
    completion_tokens BIGINT NOT NULL CHECK (completion_tokens >= 0),
    estimated_cost    DECIMAL(10,4) NOT NULL CHECK (estimated_cost >= 0),
    batch_id        UUID,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (id, created_at)               -- 分区表 PK 必须含分区键
) PARTITION BY RANGE (created_at);
-- 月分区: token_usage_2026_05, _06, ...

CREATE INDEX idx_token_usage_pipeline ON token_usage(pipeline_id);
CREATE INDEX idx_token_usage_project ON token_usage(project_id, created_at);

CREATE TABLE cost_quota (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id      TEXT NOT NULL REFERENCES project(id),
    month           VARCHAR(7) NOT NULL CHECK (month ~ '^\d{4}-\d{2}$'),  -- "2026-05"
    token_limit     BIGINT NOT NULL CHECK (token_limit > 0),
    token_used      BIGINT NOT NULL DEFAULT 0,
    status          VARCHAR(16) NOT NULL DEFAULT 'active'
                    CHECK (status IN ('active','exceeded','special_approved')),
    UNIQUE(project_id, month)
);

-- ============================================================
-- 7. 审计日志 (WORM, 月分区)
-- ============================================================

CREATE TABLE audit_log (
    id              UUID DEFAULT gen_random_uuid() NOT NULL,
    event           VARCHAR(64) NOT NULL,
    actor           VARCHAR(320) NOT NULL,
    action          VARCHAR(128) NOT NULL,
    resource        VARCHAR(256) NOT NULL,
    result          VARCHAR(16) NOT NULL DEFAULT 'success'
                    CHECK (result IN ('success','failure')),
    error_code      VARCHAR(32),
    source_ip       INET,
    user_agent      VARCHAR(512),
    project_id      TEXT,
    region          VARCHAR(16) NOT NULL DEFAULT 'bj',
    artifact_hash   VARCHAR(64),
    prev_hash       VARCHAR(64) NOT NULL,
    content_hash    VARCHAR(64) NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (id, created_at)               -- 分区表 PK 必须含分区键
) PARTITION BY RANGE (created_at);
-- 月分区: audit_log_2026_05, _06, ...
-- 应用用户仅 INSERT; REVOKE DELETE/UPDATE/TRUNCATE/DROP
-- 每小時全链完整性扫描 (跨分区顺序验证)
-- 3 年 WORM 保留 (超出可归档但不可删除)

CREATE INDEX idx_audit_log_created ON audit_log(created_at);
CREATE INDEX idx_audit_log_actor ON audit_log(actor, created_at);

-- ============================================================
-- 7a. 偏好存储 (对应 Proto PreferenceDelta, Phase 7)
-- ============================================================

CREATE TABLE preference (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id      TEXT NOT NULL REFERENCES project(id),
    key             VARCHAR(128) NOT NULL,     -- "auth_validation" | "error_handling" | "naming"
    value           TEXT NOT NULL,             -- "zod" | "try-catch" | "camelCase"
    weight          DECIMAL(5,2) NOT NULL DEFAULT 0,
    source          VARCHAR(32) NOT NULL CHECK (source IN ('code_review','auto_detect','ab_experiment','manual')),
    last_activated  TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(project_id, key, value)
);

CREATE INDEX idx_preference_project ON preference(project_id);

-- ============================================================
-- 7b. 轨迹存储 (对应 Proto Trajectory, Phase 7)
-- ============================================================

CREATE TABLE trajectory (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id      TEXT NOT NULL REFERENCES project(id),
    pipeline_id     TEXT NOT NULL,
    stage_sequence  TEXT[] NOT NULL,
    total_chat_rounds INT NOT NULL,
    total_tokens    BIGINT NOT NULL,
    backtrack_count INT NOT NULL DEFAULT 0,
    rejection_count INT NOT NULL DEFAULT 0,
    failure_codes    TEXT[],
    successful_patterns TEXT[],
    embedding       BYTEA,                    -- pgvector 扩展 (Phase 7)
    requirement_summary TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_trajectory_project ON trajectory(project_id);
CREATE INDEX idx_trajectory_embedding ON trajectory USING ivfflat (embedding vector_cosine_ops)
    WITH (lists = 100);

-- ============================================================
-- 8. 自学习快照与 A/B 实验 (Phase 7 启用)
-- ============================================================

CREATE TABLE knowledge_snapshot (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id      TEXT NOT NULL REFERENCES project(id),
    version         INT NOT NULL CHECK (version > 0),
    snapshot_data   JSONB NOT NULL,
    signature       VARCHAR(128),           -- Ed25519
    health_baseline BOOLEAN NOT NULL DEFAULT false,
    code_acceptance_rate DECIMAL(5,2) CHECK (code_acceptance_rate >= 0 AND code_acceptance_rate <= 100),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(project_id, version)
);

CREATE TABLE ab_experiment (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    knowledge_id    TEXT NOT NULL,
    cohort_a_ratio  DECIMAL(3,2) NOT NULL DEFAULT 0.90
                    CHECK (cohort_a_ratio > 0 AND cohort_a_ratio < 1),
    status          VARCHAR(16) NOT NULL DEFAULT 'running'
                    CHECK (status IN ('running','completed','aborted')),
    verdict         VARCHAR(8) CHECK (verdict IN ('promoted','invalid','harmful')),
    p_value         DECIMAL(6,4) CHECK (p_value >= 0 AND p_value <= 1),
    effect_size     DECIMAL(6,4),
    started_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at    TIMESTAMPTZ
);

CREATE TABLE ab_experiment_assignment (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    experiment_id   UUID NOT NULL REFERENCES ab_experiment(id),
    pipeline_id     TEXT NOT NULL REFERENCES pipeline(id),
    cohort          CHAR(1) NOT NULL CHECK (cohort IN ('A','B')),
    code_acceptance_rate DECIMAL(5,2),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- ============================================================
-- 9. Feature Flags
-- ============================================================

CREATE TABLE feature_flag (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name            VARCHAR(64) NOT NULL UNIQUE,
    owner           VARCHAR(128) NOT NULL,
    status          VARCHAR(16) NOT NULL DEFAULT 'experimental'
                    CHECK (status IN ('experimental','beta','stable','deprecated')),
    rollout_percent INT NOT NULL DEFAULT 0 CHECK (rollout_percent >= 0 AND rollout_percent <= 100),
    description     VARCHAR(512),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at      TIMESTAMPTZ,
    deprecated_at   TIMESTAMPTZ
);

-- ============================================================
-- 10. Task Queue (minimal: PG SKIP LOCKED)
-- ============================================================

CREATE TABLE task_queue (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    pipeline_id     TEXT NOT NULL,
    project_id      TEXT NOT NULL,          -- Phase 5 分片路由键
    task_type       VARCHAR(32) NOT NULL CHECK (task_type IN ('llm_request','sandbox_run','notification')),
    priority        INT NOT NULL DEFAULT 2 CHECK (priority >= 0 AND priority <= 3),
    payload         JSONB NOT NULL,
    status          VARCHAR(16) NOT NULL DEFAULT 'pending'
                    CHECK (status IN ('pending','claimed','running','completed','failed')),
    claimed_by      VARCHAR(64),
    claimed_at      TIMESTAMPTZ,
    started_at      TIMESTAMPTZ,
    completed_at    TIMESTAMPTZ,
    retry_count     INT NOT NULL DEFAULT 0,
    max_retries     INT NOT NULL DEFAULT 3,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_task_queue_dequeue ON task_queue(status, priority DESC, created_at DESC)
    WHERE status = 'pending';

-- 原子出队 (Phase 3 文档化):
-- UPDATE task_queue SET status='claimed', claimed_by=$coordinator_id, claimed_at=NOW()
--   WHERE id = (SELECT id FROM task_queue WHERE status='pending'
--     ORDER BY priority DESC, created_at LIMIT 1 FOR UPDATE SKIP LOCKED)
--   RETURNING *;
```

### 19.3 迁移脚本

```
migrations/
  ├── 001_init.up.sql          # Phase 1 表 (project/user/user_role/.../task_queue)
  ├── 001_init.down.sql        # DROP ALL (仅 dev)
  ├── 002_token_partition_2026_06.up.sql   # 预创建月分区
  ├── 003_soft_delete.up.sql   # Phase 2 软删除列
  ├── 004_ab_experiment.up.sql # Phase 7 自学习表
  └── ...
  
迁移方式:
  开发: 自动 Up/Down (goose / golang-migrate)
  Staging: 自动 Up, 手动 Down (需运维确认)
  生产: Up-only, 大表变更用三阶段 + pg_try_advisory_lock(迁移ID) 防并发
        pg_try_advisory_lock → 获取失败拒绝第二个迁移实例
```

### 19.4 数据库优化路线图 (24 项修复追踪)

| # | 修复项 | Phase | 状态 |
|---|--------|-------|------|
| 1 | audit_log 月分区 (不可逆) | Phase 1 | ✅ DDL 已含 |
| 2 | 9 种枚举 CHECK 约束 | Phase 1 | ✅ DDL 已含 |
| 3 | rollout/ratio 范围 CHECK | Phase 1 | ✅ DDL 已含 |
| 4 | FK 补全 (file_lock/knowledge_snapshot) | Phase 1 | ✅ DDL 已含 |
| 5 | audit_log 索引 | Phase 1 | ✅ DDL 已含 |
| 6 | pipeline(created_by) 索引 | Phase 2 | ✅ DDL 已含 |
| 7 | 冗余索引删除 (conv_msg) | Phase 1 | ✅ 已移除 |
| 8 | 软删除列 (pipeline/user/conv_msg) | Phase 2 | ✅ DDL 已含 |
| 9 | 命名统一 (单数) | Phase 1 | ✅ DDL 已改 |
| 10 | CHECK 约束命名 (chk_*) | Phase 1 | ✅ DDL 已含 |
| 11 | 迁移拆分 (Phase 1 不建 Phase 7 表) | Phase 1 | ✅ 19.3 已标注 |
| 12 | 原子 UPDATE+RETURNING 文档化 | Phase 3 | ✅ task_queue 已加 |
| 13 | 状态变更加 version 列 (乐观锁) | Phase 3 | ✅ DDL 已含 |
| 14 | 迁移并发控制 (pg_try_advisory_lock) | Phase 3 | ✅ 19.3 已加 |
| 15 | UUID v7 全线统一 (分片前) | Phase 5 | ✅ DDL 已用 gen_random_uuid() |
| 16 | audit_log 加 project_id | Phase 5 | ✅ DDL 已含 |
| 17 | task_queue 跨 project 调度 | Phase 5 | ✅ project_id 列已加 |
| 18 | RLS 行级安全 (project_id 隔离) | Phase 6 | 待实现 |
| 19 | 应用/迁移用户分离 + Replica 只读 | Phase 6 | 待实现 |
| 20 | TDE + 列级加密矩阵 | Phase 6 | 待实现 |
| 21 | 备份加密策略 | Phase 6 | 待实现 |
| 22 | 非核心表变更审计 | Phase 6 | 待实现 |
| 23 | 表级同步/异步复制策略矩阵 | Phase 8 | 待实现 |
| 24 | failover 前置 Migration Gate | Phase 8 | 待实现 |


---

## 十八、开发阶段 (重排后: UI 尽早交付)

| Phase | 内容 | 用户可见交付 | 组件数 |
|-------|------|-------------|--------|
| **Phase 1** | 项目骨架 + 单 Agent LLM 对话 (CLI) + **12 个能力域 (4 真实 + 8 stub)** + **LLM Router (Go) + 模型注册表 (Anthropic/DeepSeek 直通)** + **BashTool (local-shell)** | CLI: "帮我写 Hello World" + Agent 可执行 ls/grep/npm install (仅 minimal profile) | 1 (Go 单二进制) |
| **Phase 2** | 极简 Web 对话界面 (聊天框 + Markdown + 拓扑图) + BFF Auth + Terminal (只读) 面板 | PM 浏览器与 Agent 对话, 查看模块拓扑, 观察命令输出 | 3 (Go + Node/BFF + React) |
| **Phase 3** | Pipeline 状态机 + 实现阶段 + Diff 预览 (side-by-side) + 行级评论 + 审批收件箱 + **模型切换 UI** | 开发能审查代码, PM 能看阶段进度, 前端切换模型 | 5 |
| **Phase 4** | Docker Sandbox + 自动化测试 + 一键 Staging 部署 + 验证反馈 + **Token 成本看板** | **完整闭环**: 需求→代码→部署→PM 体验→反馈 + 成本可见 | 5 |
| Phase 5 | CSP 多 Agent 协作 + Tool Registry + 嵌入索引 + 子 Pipeline 分支 + **Node.js IO 层启动** + **OpenAI/Gemini 翻译层** | Agent 能力增强 (后台), 模型选择扩展到非 Anthropic 提供商 | 6 |
| Phase 6 | 企业级: RBAC + SSO + 审计 + 沙箱安全 + 二维模块归属 + **CommandExecutor 切换 docker-sandbox** | 多团队可用, 安全合规 | 6 |
| Phase 7 | 自学习 4 层 + A/B 测试 + 知识回滚 + 回顾/总结报告 + LLM 优先级调度 (WFQ) | 知识积累, 效率可见提升 | 7 |
| Phase 8 | 性能优化 + 高并发 (预热池/分片/读写分离) + 高可用 (熔断/降级/检查点) + DR | 压测通过 SLO, 灾备就绪 | 9 |
| Phase 9 | 完整工作台 (需求面板 / CI-CD 看板 / 成本看板 / A/B 实验看板) | 全功能工作台 | 11 |
| Phase 9-10 | 可移植 + 扩展 + 合规报告 + Runbook + 离线部署包 + **enterprise 实现交付** | 企业级就绪 | 13 |

> **C3 修复**: Phase 1 仅交付 minimal 实现 + 接口 stub。enterprise 实现 (Vault HA / K8s Pod API / Multi-Region DR / DockerSandboxExecutor) 随 Phase 9-10 引入，不要求 Phase 1 就写完。
>
> **v5 新增**: 能力域从 11 个扩展为 12 个 (新增 `CommandExecutor`)。LLM Router 从 Node.js IO 层移至 Go 协调层，采用表驱动模型注册表。Phase 1 模型注册表覆盖 Anthropic + DeepSeek (直通 /v1/messages)，Phase 5 加 OpenAI/Gemini 翻译层。

**核心变化**: Phase 2-4 每个阶段都交付可用的 UI 增量。Phase 4 已是完整闭环产品，后续 Phase 加深度而非补基础。MVP 用户从 Phase 2 即可介入体验。

### 18.1 Phase 1 实现分解 `[v5 细化]`

Phase 1 交付一个 **Go 单二进制 CLI**，不依赖 Docker、不依赖 Node.js。用户通过终端与 Agent 对话，Agent 可执行只读命令和开发工具链。

**核心原则**:
- LLM Router 在 Go 内（Phase 1 无 Node.js 运行时，仅保留 Proto 契约定义供 Phase 5 使用）
- 模型注册表硬编码，Phase 5 加 YAML 覆盖
- Bash 走 `local-shell`，对标 Claude Code 体验
- 12 个能力域: 4 个真实实现 + 8 个 stub

#### 18.1.1 文件清单

```
internal/
├── llm/
│   ├── registry.go              # ModelRegistry + ModelEntry + DefaultRegistry (Anthropic/DeepSeek)
│   ├── router.go                # Router.SendMessage() → forward() 直通 /v1/messages
│   └── anthropic/
│       └── client.go            # HTTP POST + SSE 流式解析 (net/http, 零第三方依赖)
├── port/
│   ├── llm_client.go            # LLMRouterClient 接口 (Chat + ChatStream)
│   ├── command_executor.go      # CommandExecutor 接口 (Execute + ExecuteStream + Validate)
│   ├── tool_registry.go         # Tool + StreamingTool + ToolRegistry 接口 (已有)
│   └── learning_client.go       # LearningEngine 接口 (stub, Phase 7)
├── adapter/
│   ├── local_shell_executor.go  # LocalShellExecutor: os/exec + 危险命令阻断 + 路径限制
│   └── local_shell_executor_test.go
├── tool/
│   ├── bash_tool.go             # BashTool — 实现 StreamingTool[BashInput, ExecOutput]
│   ├── read_tool.go             # ReadFileTool
│   ├── write_tool.go            # WriteFileTool
│   ├── grep_tool.go             # GrepTool
│   └── glob_tool.go             # GlobTool
├── agent/
│   ├── domain/
│   │   ├── query_engine.go      # QueryEngine 状态机 (已有)
│   │   ├── query_engine_test.go
│   │   └── coordinator.go       # AgentCoordinator (已有, 需改: 注入 Router + BashTool)
│   └── service/
│       └── csp_channel.go       # CSP Channel (已有, 单 Agent 仅需单 channel)
├── config/
│   └── profiles/
│       └── minimal.yaml         # Phase 1 唯一使用的 profile
├── secret/
│   └── envfile_store.go         # SecretStore minimal 实现 — 读 .env
cmd/
└── server/
    └── main.go                  # CLI 入口: Bootstrap(profile) → Agent Loop (stdin/stdout)
```

**不需要的文件**（延后到 Phase 5）:
- `adapter/grpc_llm_client.go` — LLM Router 已在 Go 内，无需 gRPC 到 Node
- `adapter/docker_sandbox_executor.go` — Phase 4
- `internal/agent/adapter/*_learning_client.go` — Phase 7
- Node.js IO 层全部代码 — Phase 5

**保留但不运行时使用的文件**:
- `proto/agent/v1/*.proto` — 保留 IDL 定义，供 Phase 5 Node.js IO 层生成；Phase 1 **不启动** ConnectRPC server
- `proto/agent/v1/llm.proto` — 保留 LLMConfig.model_alias 字段定义

#### 18.1.2 能力域实现状态

| 能力域 | Phase 1 状态 | 实现 |
|--------|-------------|------|
| `SecretStore` | **真实** | `envfile_store.go` — 读 `.env` 文件, Router 依赖 |
| `CommandExecutor` | **真实** | `local_shell_executor.go` — 宿主机 spawn, BashTool 依赖 |
| `Telemetry` | **真实** | stdout JSON 日志, 所有组件依赖 |
| `Notifier` | **真实** | stdout 输出, 通知/错误可见 |
| `ContainerRuntime` | stub | 返回 "not implemented", Phase 4 |
| `ObjectStore` | stub | 返回 "not implemented", Phase 4 |
| `TaskQueue` | stub | 返回 "not implemented", Phase 5 |
| `EventBus` | stub | 返回 "not implemented", Phase 5 |
| `Cache` | stub | 返回 "not implemented", Phase 5 |
| `ServiceRegistry` | stub | 返回 "not implemented", Phase 5 |
| `DisasterRecovery` | stub | 返回 "not implemented", Phase 8 |
| `LoadBalancer` | stub | 返回 "not implemented", Phase 8 |

#### 18.1.3 Agent Loop (CLI 核心)

```
启动: ./openforge serve --profile minimal
  ↓
Bootstrap(profile) → 注入 4 真实 + 8 stub 能力域
  ↓
QueryEngine.Start()
  ↓
┌─ Agent Loop (stdin/stdout) ──────────────────────────────┐
│ You: 帮我在当前项目加一个 eslint 配置                        │
│   ↓                                                       │
│ QueryEngine.SubmitMessage() → Router.SendMessage("sonnet")│
│   ↓ LLM 返回 tool_use: {name:"bash", input:{command:"ls"}}│
│ BashTool.Execute("ls") → LocalShellExecutor.spawn()       │
│   ↓ LLM 继续推理 → tool_use: {name:"write", ...}          │
│ WriteTool.Execute(".eslintrc.json", content)               │
│   ↓ LLM 返回 tool_use: {name:"bash", input:{npm install}} │
│ BashTool.Execute("npm install ...") → 流式输出到终端       │
│   ↓                                                       │
│ Agent: eslint 配置已完成 ✅                                 │
└──────────────────────────────────────────────────────────┘
```

#### 18.1.4 Phase 1 自检清单

```
[ ] Go 单二进制编译通过 (go build ./cmd/server)
[ ] Router 直通 Anthropic /v1/messages (真实 API Key)
[ ] Router 直通 DeepSeek /v1/messages (真实 API Key)
[ ] 模型降级: opus 不可用 → 自动 fallback sonnet → deepseek
[ ] BashTool: ls / grep / git status 可执行
[ ] BashTool: npm install / go build 可执行 (流式输出)
[ ] BashTool: rm -rf / 被阻断 (危险命令)
[ ] BashTool: curl | bash 被阻断 (管道注入)
[ ] BashTool: 超时 60s → SIGTERM → 2s → SIGKILL
[ ] Agent Loop: 3 轮对话不崩溃 (需求→实现→测试)
[ ] Agent Loop: Ctrl+C 优雅退出, 清理子进程
[ ] Token Metering: ring buffer 记录每轮 token
```
