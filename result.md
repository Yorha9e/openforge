# OpenForge 项目文件全览

## 目录结构 (14 个文件)

```
OpenForge/
├── CLAUDE.md                          # 项目上下文入口
├── DESIGN.md                          # 完整设计文档 (~2600行, 18章+19章DB)
├── STYLE_GUIDE.md                     # Go/TS/React 编码规范
├── api-contract.yaml                  # OpenAPI 3.1, 30 个 REST 端点
├── buf.yaml                           # Buf v2 配置 (lint/breaking规则)
├── buf.gen.yaml                       # Go 代码生成 (buf generate)
├── buf.lock                           # Buf 依赖锁定
├── .claude/settings.local.json        # 本地权限配置
└── proto/agent/v1/
    ├── coordinator.proto              # CoordinatorService — 核心协调 RPC
    ├── gate.proto                     # GateService — 审批流 RPC
    ├── llm.proto                      # LLMRouterService — LLM 路由 RPC
    ├── tools.proto                    # ToolRegistryService — 工具注册/搜索 RPC
    ├── learning.proto                 # LearningEngineService — 四层自学习 RPC
    ├── terminal.proto                 # TerminalService — Sandbox 终端 RPC
    └── buf.gen.ts.yaml                # TypeScript 代码生成配置
```

## 核心架构要点

**三层正交：**
- **C 层** (工作台) — BFF + React 微前端，PM 对话式操作
- **A 层** (Pipeline 引擎/Go) — 6 阶段状态机 + DAG 回溯 + Gate 审批
- **B 层** (Agent Swarm/Go+Node.js) — Go 管协调 (CSP Channel)，Node.js 管 IO (LLM Router, Tool Hub, Learning Engine)

**Proto 6 个 Service：**

| Service | 职责 | 关键 RPC |
|---------|------|----------|
| `CoordinatorService` | Agent 生命周期、Pipeline 阶段执行、对话流、Token 推送 | `Chat(stream)`, `ExecuteStage(stream)`, `PushTokenUsage(stream)` |
| `GateService` | 审批流 (不直写 DB) | `Approve`, `Reject`, `Claim`, `GetInbox` |
| `LLMRouterService` | 多模型路由 (Anthropic 标准) | `ChatStream(stream)`, `ListModels`, `SwitchModel` |
| `ToolRegistryService` | 动态工具注册+嵌入搜索 | `SearchTools`, `CallTool`, `RegisterTool` |
| `LearningEngineService` | 四层自学习 + A/B 实验 | `WriteKnowledge`, `RecordTrajectory`, `MatchTrajectory` |
| `TerminalService` | Sandbox 终端 (只读/调试两种模式) | `Open(stream)`, `Input`, `Close` |

**API 端点：** 30 个 REST 端点，覆盖项目管理、Pipeline 操作、Gate 审批、模型切换、Token/成本、Artifact 下载、用户设置、通知、管理后台。

**Phase 1-4 MVP 范围：** 5 容器 (Go + Node + React + PG + Docker)，Phase 1 先做 CLI + minimal profile + 10 接口 stub (仅 Go+Node，不写前端)。

**编码架构：** 六边形+垂直切片 (`domain → port ← adapter`)，Go/Node.js/React 代码规范齐全，table-driven tests 强制，CI 门禁规范。

### 轻微遗漏补充

> 以下 3 项在初次扫描时未覆盖，不影响 Phase 1 启动，但后续 Phase 需知悉。

**1. Capability Profile（适配大小公司的关键抽象）** — 详见 DESIGN.md §10.1
```
minimal:    Docker Compose 5 容器, .env 密钥, 本地文件存储 (Phase 1-4)
standard:   K8s 单 AZ, Vault Sidecar, MinIO 单节点, Redis (Phase 5-7)
enterprise: 多 Region K8s, Vault HA, MinIO Cluster, Multi-Region DR (Phase 9-10)
```
通过一个 YAML 切换全套基础设施实现，不改代码。10 个能力域接口全部定义在 `shared/kernel/interfaces.go`。

**2. 数据库 22 张表 + WORM 月分区** — 详见 DESIGN.md §19
```
核心表: pipeline / pipeline_stage / gate_event / checkpoint / conversation_message
审计:   audit_log (WORM, 月分区, SHA256 哈希链, 3 年保留)
自学习: preference / trajectory (pgvector) / knowledge_snapshot / ab_experiment
计量:   token_usage (月分区, 异步 ring buffer → COPY) / cost_quota
```
UUID v7 全线统一, 所有枚举列 CHECK 约束, 乐观锁 version 列 (Phase 3)。

**3. 设计文档经过 6 轮评审**
```
v1: 纯技术架构 → v2: 竞争定位+成本+DR+法律 (6.4分)
v3: + Capability Profile (6.5) → v4: + 缓存分层+MQ策略+安全护栏 (7.5)
v5: 补齐前端+29维度综合评审 (6.5, 3个阻塞项) → v6: 全部清零
```
某些决策有"为什么"的背景，翻阅 DESIGN.md 对应章节可追溯到评审意见。

项目当前状态是 **设计完成，待进入 Phase 1 编码**。
