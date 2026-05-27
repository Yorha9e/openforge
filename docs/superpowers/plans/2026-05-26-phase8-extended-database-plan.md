# Phase 8 数据库扩展实施计划 (Phase 8.1 - 8.4)

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 完成 OpenForge 系统中数据库设计中所有挂起/未实现的 DDL 和表关联，使整个系统的多租户项目、用户权限、对话历史、审批挂起、学习引擎闭环等完全落地。

**Architecture:** 
- **Phase 8.1**: 对话历史持久化与回溯分支。
- **Phase 8.2**: 项目管理与多租户权限隔离。
- **Phase 8.3**: 待审批门禁请求持久化。
- **Phase 8.4**: 学习引擎深化与 A/B 实验数据闭环。

**Tech Stack:** Go, PostgreSQL (lib/pq), WebSockets, Gux/Mux REST API

---

## 整体演进路线

```
【Phase 8.1: 聊天与回溯】 ──► 【Phase 8.2: 多租户权限】 ──► 【Phase 8.3: 审批挂起】 ──► 【Phase 8.4: A/B 实验与回顾】
 (conversation_message)       (project, user, user_role)    (gate_request 门禁)           (ab_experiment & retrospective)
```

---

## Phase 8.1: 对话历史持久化与回溯分支 (Conversation Persistence)

### 目标
实现 Pipeline 的聊天记录在 `conversation_message` 和 `conversation_branch` 表中的持久化，保证刷新页面后聊天记录不丢失，并且在回溯退回（Backtrack）时分叉保存分支。

### 涉及文件
- Modify: `internal/pipeline/port/repository.go` — 定义 `ConversationRepository` 接口
- Modify: `internal/pipeline/adapter/pg_repository.go` — 实现 SQL 插入与拉取
- Modify: `internal/agent/domain/query_engine.go` — 桥接持久化方法并在消息提交和回溯时写入
- Modify: `internal/server/routes.go` — 替换硬编码返回 `[]` 的 `handleGetMessages` 接口
- Modify: `internal/server/ws_handler.go` — WebSocket 连接建立时从数据库载入最新历史消息

### 关键步骤
- [ ] **Step 1: 在 `repository.go` 中定义接口与数据模型**
- [ ] **Step 2: 在 `pg_repository.go` 中编写 SQL 实现**
  - 使用 `INSERT INTO conversation_message ... ON CONFLICT (pipeline_id, branch_id, msg_seq) DO UPDATE`
- [ ] **Step 3: 改造 `QueryEngine.SubmitMessage` 进行实时保存**
- [ ] **Step 4: 改造 `routes.go` 实现 REST 消息历史返回**
- [ ] **Step 5: 改造 `ws_handler.go` 的连接初始化流程，从 DB 预载历史**

---

## Phase 8.2: 项目管理与多租户权限隔离 (Multitenancy & Roles)

### 目标
落地项目 (`project`)、用户 (`user`)、用户角色 (`user_role`) 和模块所有权 (`module_ownership`) 的 CRUD 及其拦截层，保证 API 和 Pipeline 执行受租户和角色（Admin/PM/Dev_Lead/Dev）的安全控制。

### 涉及文件
- Create: `internal/auth/port/repository.go` — 定义 `AuthRepository` 接口
- Create: `internal/auth/adapter/pg_auth_repository.go` — 实现租户、用户及角色的 DB 操作
- Modify: `internal/server/middleware.go` — 增加租户和角色上下文拦截器（Tenant & Role Check）
- Modify: `internal/pipeline/service/pipeline_service.go` — 在创建和执行 Pipeline 时校验创建人的角色和所有权

### 关键步骤
- [ ] **Step 1: 编写 `AuthRepository` 并实现 `project`、`user`、`user_role` 的增删改查**
- [ ] **Step 2: 在 `middleware.go` 编写 `TenantContextMiddleware` 提取 `X-Project-ID` 与 `X-User-ID`**
- [ ] **Step 3: 在 `middleware.go` 编写 `RBACMiddleware` 校验用户是否有执行操作的 Role 权限**
- [ ] **Step 4: 结合 `module_ownership` 表，在 Pipeline 执行或审批修改特定路径文件时，校验其是否属于该用户的归属 Team**

---

## Phase 8.3: 待审批门禁请求持久化 (Gate Request Tracking)

### 目标
将 L3/L4 Pipeline 遇到人工审批点（Gate）时的挂起状态，从原本不可追踪的状态转移至持久化存储表 `gate_request` 中，记录其审批超时时间、认领人和决策结果。

### 涉及文件
- Modify: `internal/pipeline/port/repository.go` — 增加 `GateRequestRepository` 接口
- Create: `internal/pipeline/adapter/pg_gate_request_repository.go` — 实现 `gate_request` 表的读写
- Modify: `internal/pipeline/service/gate_service.go` — 重构审批服务的 `Approve`/`Reject` 逻辑，同时更新挂起请求与写入 `gate_event` 审计日志
- Modify: `internal/agent/domain/query_engine.go` — 在遇到 Approval 挂起时，触发持久化写入一条状态为 `pending` 的 gate_request。

### 关键步骤
- [ ] **Step 1: 实现 `gate_request` 表的 Repository，支持 `Create`、`UpdateStatus`、`GetPending` 和 `ListByPipeline`**
- [ ] **Step 2: 在 `QueryEngine` 中，当触发 `NeedsGate()` 并暂停时，同步写入一条 `gate_request` 记录，默认包含 5 分钟超时时间。**
- [ ] **Step 3: 在 `GateService` 处理来自 PM 的 REST API 审批时，校验、认领并更新该 `gate_request` 的状态为 `approved` 或 `rejected`。**
- [ ] **Step 4: 异步后台定时任务（Worker）扫描 `gate_request`，将超时的 pending 请求状态更改为 `timeout`，并注入失败通知让 Pipeline 中断。**

---

## Phase 8.4: 学习引擎深化与 A/B 实验数据闭环 (Learning Engine & A/B Testing)

### 目标
落地 `knowledge_snapshot`（知识库版本快照）、`ab_experiment` 与 `ab_experiment_assignment`（用于策略演进的 A/B 实验分流）以及 `pipeline_retrospective`（事后复盘与提炼）。

### 涉及文件
- Modify: `internal/agent/domain/knowledge_snapshot.go` — 增加并实现 `KnowledgeSnapshotStore`
- Modify: `internal/agent/domain/ab_experiment.go` — 增加并实现 `ExperimentStore`
- Modify: `internal/agent/domain/retrospective.go` — 增加并实现 `RetrospectiveStore`
- Modify: `internal/agent/service/learning_service.go` — 当 Pipeline 运行结束（Completed / Rejected）时，自动触发回顾分析（Retrospective），抽取 lessons_learned 写入 DB，同时评估 A/B 实验对象的代码接受率以决定是否 Promote。

### 关键步骤
- [ ] **Step 1: 为 `knowledge_snapshot`、`ab_experiment`、`pipeline_retrospective` 编写 PG 数据库持久化适配器**
- [ ] **Step 2: 编写 A/B 实验分流路由器：当 Pipeline 启动时，分配 Cohort 'A' 或 'B'，并在执行期间注入不同的 Preference 策略。**
- [ ] **Step 3: 在 Pipeline 运行结束后，由 `LearningService` 调用 LLM 提炼本次 Pipeline 运行的成功/失败模式，插入 `pipeline_retrospective` 中。**
- [ ] **Step 4: 实现 A/B 实验结果分析器，根据两组分配的 Pipeline 的最终代码接受率（Code Acceptance Rate）进行假设检验，自动更新或推广性能更佳的知识版本。**
