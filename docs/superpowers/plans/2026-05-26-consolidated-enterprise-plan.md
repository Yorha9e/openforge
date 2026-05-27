# 2026-05-26-consolidated-enterprise-plan

这是一个涵盖 4 个核心企业级/专业版需求的完整架构设计与落地实施计划：
1. **简单的注册机制**：在原登录链路中支持用户注册与密码 Hash 持久化。
2. **项目级权限展示隔离**：保证各 Project 在列表展示和 API 层面只对拥有权限（`user_role`）的用户可见。
3. **Active Pipeline（活跃流水线）功能落地**：在 Dashboard 中落地跨项目的活跃/待审批流水线全局监控与实时进度展示。
4. **Pro Mode（专业开发模式）深度设计**：设计结合 Dockview、Diff 差分引擎、受控文件树与 AI 协同的双向联动页面。

---

## 目录
- [Task 1: 简单的注册机制](#task-1-简单的注册机制)
- [Task 2: 项目级权限展示隔离](#task-2-项目级权限展示隔离)
- [Task 3: Active Pipeline 功能落地](#task-3-active-pipeline-功能落地)
- [Task 4: Pro Mode 专业开发模式深度设计](#task-4-pro-mode-专业开发模式深度设计)
- [Task 5: 验证与联调计划](#task-5-验证与联调计划)

---

## Task 1: 简单的注册机制

### 1.1 数据库结构变更
注册机制需要保存用户的密码 Hash，我们需要在 `"user"` 数据表中新增密码存储列（或在 auth 模块中建立独立的凭证表）。
- **SQL 变更**：
  ```sql
  ALTER TABLE "user" ADD COLUMN IF NOT EXISTS password_hash VARCHAR(255);
  ```

### 1.2 后端 API 设计 (`POST /api/auth/register`)
- **路径**：`internal/server/routes.go`
- **逻辑**：
  1. 接收 `Username`, `Password`, `DisplayName`, `AvatarURL`。
  2. 检查 `Username` 在 `"user"` 表中是否已存在，若存在返回 `400 Username already exists`。
  3. 使用 `bcrypt` 或内置的安全性较高的加密算法生成 `password_hash`。
  4. 插入记录到 `"user"` 表：
     ```go
     INSERT INTO "user" (id, display_name, avatar_url, password_hash, created_at)
     VALUES ($1, $2, $3, $4, NOW())
     ```
  5. 注册成功后自动为用户初始化默认角色或全局 `observer` 角色，随后生成 JWT token 直接返回，完成“注册即登录”。

### 1.3 前端 UI 改造
- **修改文件**：`frontend/src/features/login/LoginPage.tsx`
- **改造内容**：
  - 新增 `isRegister` 状态（布尔值），在登录页表单底部提供 `切换到注册` 链接。
  - 注册状态下，额外展示 `Display Name` 输入框。
  - 调用 `api.register(...)` 注册接口，注册成功后调用 `login` 保存 token。

---

## Task 2: 项目级权限展示隔离

### 2.1 鉴权上下文注入与多租户机制
我们需要在 `routes.go` 中识别并提取当前 JWT 中携带的 `user_id`。
- 当前路由中：
  ```go
  claims, err := jwtSvc.Verify(tokenString) // 从 Authorization Header 中解出 UserID
  ```
- 我们需要将 `claims.UserID` 及 `claims.Role` 注入到当前 `http.Request` 的 `Context` 中。

### 2.2 修改 `handleListProjects`（项目过滤）
- **修改文件**：`internal/server/routes.go` 中的 `handleListProjects`
- **逻辑重构**：
  - 从 `r.Context()` 中获取 `user_id` 和用户角色 `role`。
  - **超级管理员模式**：如果用户是全局管理员（例如系统配置文件中定义的全局 `admin`），则允许拉取所有项目。
  - **普通用户隔离模式**：只拉取用户被显式授权关联的项目：
    ```sql
    SELECT p.id, p.name, p.git_url, p.created_at
    FROM project p
    INNER JOIN user_role ur ON p.id = ur.project_id
    WHERE ur.user_id = $1 AND p.deleted_at IS NULL
    ORDER BY p.created_at DESC
    ```
  - 通过该 INNER JOIN，非该项目授权成员（在 `user_role` 中无对应关系）的请求将在 API 层直接被隔离过滤，从而在 Dashboard 列表中彻底不可见。

### 2.3 单项目详情接口（`GET /api/projects/{id}`）拦截
- 任何试图请求特定项目或该项目下 Pipeline 的 API 接口，都必须在入口处检查当前 `user_id` 是否拥有该项目的 `user_role`。
- 若不属于该项目，直接返回 `403 Forbidden`。

---

## Task 3: Active Pipeline 功能落地

“Active Pipeline” 指的是**正在执行中、等待人工审批或被挂起**的 Pipeline（`status` 状态为 `'running'`, `'paused'`, `'awaiting_review'`）。

### 3.1 后端 API 实现
- **新增接口**：`GET /api/pipelines/active`
- **查询逻辑**：
  - 从当前上下文中取得 `user_id`。
  - 查询当前用户**有权限的所有项目**下，且状态处于活跃状态的流水线：
    ```sql
    SELECT pl.id, pl.project_id, pr.name as project_name, pl.title, pl.status, pl.current_stage, pl.updated_at
    FROM pipeline pl
    INNER JOIN project pr ON pl.project_id = pr.id
    INNER JOIN user_role ur ON pr.id = ur.project_id
    WHERE ur.user_id = $1 
      AND pl.status IN ('running', 'paused', 'awaiting_review')
      AND pl.deleted_at IS NULL
    ORDER BY pl.updated_at DESC
    ```

### 3.2 前端全局活跃仪表盘看板
- **修改/新增文件**：`frontend/src/features/dashboard/DashboardPage.tsx`
- **设计展示**：
  - **顶部 Active Overview 栏**：在原项目列表上方，设计一个精致的 “Active Pipelines (活跃流水线)” 网格看板。
  - **卡片样式 (Bento Card)**：
    - 显示所属项目名称（带有小标）、流水线标题。
    - **Stage 进度指示条**：以极简的阶梯节点形式渲染 `clarify -> decompose -> impl -> test -> deploy -> verify`。当前 Stage 呈现高亮呼吸闪烁动画，已通过的 Stage 显示绿色，未执行的显示灰色。
    - **待审批呼唤**：若状态为 `awaiting_review` 或 `paused`，卡片外框呈现琥珀黄色（`tokens.warning`）微光边缘，并带有一个快速跳转至 Pro Mode 审批的 “Go to Review” 动作按钮。
  - **实时状态流**：在 WebSocket 连接中，下发 `pipeline.stage_change` 和 `pipeline.finished` 事件，当前页面自动订阅并无刷更新进度条，实现完全实时化（Real-time Pipeline Tracker）。

---

## Task 4: Pro Mode 专业开发模式深度设计

Pro Mode（`/project/:id/pipeline/:pid`）是专为代码编写与多步骤演进设计的沉浸式 IDE 级开发页面。它采用 Dockview 实现高度自定义的多面板工作区。

### 4.1 核心版面设计与双向联动
Pro Mode 页面由四大区域组成，支持可拖拽的拖动条以满足高度专业的操作：

1. **左侧：Changed Files 树状图 (受控文件树面板)**
   - 精致渲染当前 Pipeline 生成/修改的代码变更列表。
   - 文件节点旁带红/绿/黄小标（表示 `Deleted`/`Added`/`Modified`）及变更行数。
   - 用户双击文件后，右侧的 Diff 面板自动切换。

2. **右侧：Interactive Diff Viewer (差分比对面板)**
   - 使用 `monaco-editor` 差分模式（`DiffEditor`）或精美封装的 React Diff 视图，提供 `Original` vs `Modified` 双栏/单栏比对。
   - 支持高亮、代码折叠与行号精确定位。

3. **中间：Contextual Copilot Chat (AI 协同对话面板)**
   - 完美嵌入之前优化的 **Thinking & Tool-use Folding 渲染器** 和 **发包防并发拦截器**。
   - 带有专门的交互快捷气泡。例如：“解释此段修改”、“一键运行单元测试”、“重新生成此文件”。

4. **顶部：Stage Status & Backtrack Control (状态控制头部)**
   - **Backtrack (回滚追踪)**：图形化显示回滚次数 `Backtrack Count` 剩余格数。若 AI 执行不佳导致 Test/Verify 失败，此处以橙色警示。
   - **Gate Decision (准入表决)**：如果流水线在 `impl` -> `test` 或 `test` -> `deploy` 阶段触发了审批 Gate（代码审计），直接在顶部栏显示 “Awaiting Your Approval” 大横幅。提供 **Approve (同意并合并到主干)** 与 **Reject (驳回重写并附言)** 的双向操作面板。

---

## Task 5: 验证与联调计划

1. **第一阶段：后端存储与鉴权机制落地**
   - 运行 migration 引入密码哈希并暴露 `POST /api/auth/register`。
   - 重构 `handleListProjects`，引入 `user_role` 的 INNER JOIN 查询隔离。
   - 运行 `go test ./...` 确保后端接口和测试逻辑全部通过。
2. **第二阶段：前端页面及权限测试**
   - 在前端 LoginPage 增加 Register Tab，支持无感注册和拦截跳转。
   - 验证：非项目成员强行访问 `/project/:id` 时是否能被拦截。
3. **第三阶段：Active Pipelines 交互测试**
   - 在 Dashboard 建立 Active 模块，启动一条 Pipeline 观察状态条的呼吸闪烁。
   - 通过 WebSocket 确认 Stage 转换事件正确推送并驱动前端 UI 重绘。
4. **第四阶段：Pro Mode 联调与收尾**
   - 完美衔接 Dockview 内部组件数据流。
   - 确认 Diff Panel 与 FileTree Panel 双向联动体验流畅。
