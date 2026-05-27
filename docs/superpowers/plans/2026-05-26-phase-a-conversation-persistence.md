# Phase A: 对话历史持久化与回溯分支 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 实现 Pipeline 的对话历史在 PostgreSQL 数据库中的持久化，并支持对话分支（分支分叉与回溯），使用户刷新页面或断开连接后仍能完美恢复历史记录。

**Architecture:** 
1. 在 `internal/pipeline` 增加对话仓库接口，并在 `internal/pipeline/adapter/pg_repository.go` 中实现。
2. 在 `QueryEngine` 中进行桥接，使每次接收消息和流式生成完毕后，自动将消息写入 `conversation_message` 表。
3. 修改 WebSocket 及 REST 端点，在建立连接或拉取历史时直接从数据库加载最新的消息，替换现有的内存临时存储与硬编码返回。
4. 在需要 Backtrack（回溯）时，自动创建并切入 `conversation_branch`，保留完整的历史搜索树。

**Tech Stack:** Go, PostgreSQL (lib/pq), WebSocket, REST API

---

## 文件的结构调整与责任划分

为了保持微服务/领域驱动设计的整洁性，我们将修改/新建以下文件：
1. **Modify**: `internal/pipeline/port/repository.go` — 添加 `ConversationRepository` 接口
2. **Modify**: `internal/pipeline/adapter/pg_repository.go` — 实现 `ConversationRepository`
3. **Modify**: `internal/agent/domain/query_engine.go` — 注入 `ConversationRepository`，在提交和保存消息时触发持久化
4. **Modify**: `internal/server/routes.go` — 将原先硬编码返回 `[]any{}` 的 `handleGetMessages` 替换为真实的 DB 读取
5. **Modify**: `internal/server/ws_handler.go` — 在 WebSocket 连接创建时从数据库载入最新分支的历史，而非仅保留内存临时实例

---

## 详细实施步骤

### Task 1: 定义与实现数据库 ConversationRepository

**Files:**
- Modify: `internal/pipeline/port/repository.go`
- Modify: `internal/pipeline/adapter/pg_repository.go`
- Test: `internal/pipeline/adapter/pg_repository_test.go` (新建或在现有文件中追加测试)

- [ ] **Step 1: 在 `repository.go` 增加接口定义**

在 `internal/pipeline/port/repository.go` 中，添加以下接口定义：

```go
type DBMessage struct {
	ID         string    `json:"id"`
	PipelineID string    `json:"pipeline_id"`
	BranchID   string    `json:"branch_id"`
	MsgSeq     int       `json:"msg_seq"`
	Role       string    `json:"role"`
	MsgType    string    `json:"msg_type"`
	Content    string    `json:"content"`
	TokenCount int       `json:"token_count"`
	CreatedAt  time.Time `json:"created_at"`
}

type DBBranch struct {
	ID           string    `json:"id"`
	PipelineID   string    `json:"pipeline_id"`
	ParentBranch string    `json:"parent_branch"`
	ForkMsgSeq   int       `json:"fork_msg_seq"`
	Status       string    `json:"status"`
	CreatedBy    string    `json:"created_by"`
	CreatedAt    time.Time `json:"created_at"`
}

type ConversationRepository interface {
	SaveMessage(ctx context.Context, msg *DBMessage) error
	GetMessages(ctx context.Context, pipelineID string, branchID string) ([]*DBMessage, error)
	CreateBranch(ctx context.Context, branch *DBBranch) error
	GetBranch(ctx context.Context, branchID string) (*DBBranch, error)
	GetActiveBranch(ctx context.Context, pipelineID string) (*DBBranch, error)
}
```

- [ ] **Step 2: 在 `pg_repository.go` 中实现该接口**

在 `internal/pipeline/adapter/pg_repository.go` 中追加实现上述方法：

```go
func (r *PGRepository) SaveMessage(ctx context.Context, msg *DBMessage) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO conversation_message (pipeline_id, branch_id, msg_seq, role, msg_type, content, token_count)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (pipeline_id, branch_id, msg_seq) DO UPDATE SET
			content = EXCLUDED.content,
			token_count = EXCLUDED.token_count
	`, msg.PipelineID, msg.BranchID, msg.MsgSeq, msg.Role, msg.MsgType, msg.Content, msg.TokenCount)
	return err
}

func (r *PGRepository) GetMessages(ctx context.Context, pipelineID string, branchID string) ([]*DBMessage, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, pipeline_id, branch_id, msg_seq, role, msg_type, content, COALESCE(token_count, 0), created_at
		FROM conversation_message
		WHERE pipeline_id = $1 AND branch_id = $2
		ORDER BY msg_seq ASC
	`, pipelineID, branchID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var msgs []*DBMessage
	for rows.Next() {
		var m DBMessage
		if err := rows.Scan(&m.ID, &m.PipelineID, &m.BranchID, &m.MsgSeq, &m.Role, &m.MsgType, &m.Content, &m.TokenCount, &m.CreatedAt); err != nil {
			return nil, err
		}
		msgs = append(msgs, &m)
	}
	return msgs, nil
}

func (r *PGRepository) CreateBranch(ctx context.Context, b *DBBranch) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO conversation_branch (id, pipeline_id, parent_branch, fork_msg_seq, status, created_by)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, b.ID, b.PipelineID, b.ParentBranch, b.ForkMsgSeq, b.Status, b.CreatedBy)
	return err
}

func (r *PGRepository) GetBranch(ctx context.Context, branchID string) (*DBBranch, error) {
	var b DBBranch
	err := r.db.QueryRowContext(ctx, `
		SELECT id, pipeline_id, parent_branch, fork_msg_seq, status, created_by, created_at
		FROM conversation_branch WHERE id = $1
	`, branchID).Scan(&b.ID, &b.PipelineID, &b.ParentBranch, &b.ForkMsgSeq, &b.Status, &b.CreatedBy, &b.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return &b, err
}

func (r *PGRepository) GetActiveBranch(ctx context.Context, pipelineID string) (*DBBranch, error) {
	var b DBBranch
	err := r.db.QueryRowContext(ctx, `
		SELECT id, pipeline_id, parent_branch, fork_msg_seq, status, created_by, created_at
		FROM conversation_branch
		WHERE pipeline_id = $1 AND status = 'active'
		ORDER BY created_at DESC LIMIT 1
	`, pipelineID).Scan(&b.ID, &b.PipelineID, &b.ParentBranch, &b.ForkMsgSeq, &b.Status, &b.CreatedBy, &b.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return &b, err
}
```

- [ ] **Step 3: 运行并验证单元测试**

执行命令验证 `pg_repository` 是否有编译冲突或测试失败：
Run: `cmd /c "go test -v ./internal/pipeline/adapter/..."`
Expected: PASS

---

### Task 2: 改造 QueryEngine 以自动保存与切入分支

**Files:**
- Modify: `internal/agent/domain/query_engine.go`

- [ ] **Step 1: 在 `QueryEngine` 中添加对话持久化支持**

向 `QueryEngine` struct 添加 `convRepo` 依赖，以及追踪当前分支的 `activeBranchID` 变量：

```go
type QueryEngine struct {
    // ... 保持其他已有字段 ...
    convRepo       port.ConversationRepository  // 注入对话仓储
    activeBranchID string                       // 当前会话分支，默认为 "main"
}
```

- [ ] **Step 2: 在 `SubmitMessage` 时同步持久化消息到 DB**

在 `query_engine.go` 的 `SubmitMessage` 中追加用户消息时，进行持久化：

```go
// 1. 用户输入消息追加到内存
qe.messages = append(qe.messages, agentport.Message{Role: "user", Content: msg})

// 2. 新增：计算下一条 Seq 并持久化到数据库
msgSeq := len(qe.messages)
_ = qe.convRepo.SaveMessage(context.Background(), &port.DBMessage{
    PipelineID: qe.pipelineCtx.PipelineID,
    BranchID:   qe.activeBranchID,
    MsgSeq:     msgSeq,
    Role:       "user",
    MsgType:    "text",
    Content:    msg,
})
```

同理，在流式生成 assistant 消息或工具调用结果生成完成后，也向 `SaveMessage` 写入对应的返回结果。

- [ ] **Step 3: 改造 Backtrack (回溯) 以分叉出新 Branch**

当 Pipeline 触发回溯（例如测试失败退回修改）时，不要直接丢弃后续消息，而是执行 `CreateBranch` 创建一个分叉：

```go
func (qe *QueryEngine) ForkBranch(createdBy string) error {
    newBranchID := fmt.Sprintf("fork_%d", time.Now().UnixNano())
    forkSeq := len(qe.messages)
    
    branch := &port.DBBranch{
        ID:           newBranchID,
        PipelineID:   qe.pipelineCtx.PipelineID,
        ParentBranch: qe.activeBranchID,
        ForkMsgSeq:   forkSeq,
        Status:       "active",
        CreatedBy:    createdBy,
    }
    
    err := qe.convRepo.CreateBranch(context.Background(), branch)
    if err == nil {
        qe.activeBranchID = newBranchID
    }
    return err
}
```

---

### Task 3: 路由及端点替换 (REST API & WS)

**Files:**
- Modify: `internal/server/routes.go`
- Modify: `internal/server/ws_handler.go`

- [ ] **Step 1: 实现真实的 `handleGetMessages` 接口**

在 `internal/server/routes.go` 中，将原硬编码的返回：

```go
func handleGetMessages(of *profile.OpenForge) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// 原先：writeJSON(w, 200, []any{})
        
        // 替换为：
        pipelineID := mux.Vars(r)["id"] // 获取路径中的 pipeline 标识
        branchID := r.URL.Query().Get("branch_id")
        if branchID == "" {
            branchID = "main" // 默认 main 分支
        }
        
        msgs, err := of.PipelineRepo.GetMessages(r.Context(), pipelineID, branchID)
        if err != nil {
            writeError(w, 500, err.Error())
            return
        }
        writeJSON(w, 200, msgs)
	}
}
```

- [ ] **Step 2: 改造 WebSocket 处理器，在新建会话时加载历史**

在 `internal/server/ws_handler.go` 中，当客户端连上 WS 且没有内存引擎实例时，从 DB 恢复所有的消息历史至内存 `QueryEngine` 中，确保会话无缝接轨：

```go
// 1. 获取当前活跃的分支，默认为 main
activeBranch, _ := of.PipelineRepo.GetActiveBranch(ctx, pipelineID)
branchID := "main"
if activeBranch != nil {
    branchID = activeBranch.ID
}

// 2. 从 DB 恢复该分支历史
dbMsgs, _ := of.PipelineRepo.GetMessages(ctx, pipelineID, branchID)
var recoveredMsgs []agentport.Message
for _, m := range dbMsgs {
    recoveredMsgs = append(recoveredMsgs, agentport.Message{
        Role:    m.Role,
        Content: m.Content,
    })
}

// 3. 将 recoveredMsgs 重新注入到新初始化的 QueryEngine 中
engine.SetHistory(recoveredMsgs)
```

---

## 交付自审清单

- [ ] 1. 运行 `go test ./internal/...` 所有测试编译通过。
- [ ] 2. 刷新前端页面，点击任意 Pipeline，控制台能发出 `/api/pipelines/{id}/messages` 请求，且能够获取到之前聊天的 JSON 数组（不再是空 `[]`）。
- [ ] 3. 强行关闭后端，重启后端并重新开启 WebSocket 连接，之前的聊天气泡在 UI 上仍可以完整渲染，实现了数据持久化。
