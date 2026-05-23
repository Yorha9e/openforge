# Phase 5c — 打磨修复：测试补完 + Bug 修复 + 代码清理

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 修复已知 bug 和编译错误，补完缺失测试，清理硬编码和 TODO，消除 5b 阶段遗留的技术债务。

**Architecture:** 逐个修复审计发现的问题：permission_mode_test.go 编译错误、硬编码 "genesis" prevHash、硬编码 "pm" 角色、3 个 Docker 测试跳过、无测试覆盖的新增端点。

**Tech Stack:** Go 1.25 + `database/sql`, React 19 + TypeScript

**关键约束:**
- 不引入新功能，只修现有代码
- 每个修复独立 commit
- 所有现有测试必须继续通过

---
## File Map

```
openforge/
├── internal/auth/domain/
│   ├── permission_mode.go            # FIX: 加 PermissionContext/PermissionDecision
│   └── permission_mode_test.go       # FIX: 修复编译错误
├── internal/pipeline/service/
│   └── gate_service.go              # FIX: genesis → 真实 prevHash
├── internal/server/
│   ├── routes.go                     # FIX: 移除硬编码 "pm" 角色
│   └── routes_test.go               # NEW: fork endpoint 集成测试
├── internal/pipeline/domain/
│   └── pipeline_test.go             # NEW: Fork 方法测试
├── internal/adapter/
│   └── docker_sandbox_executor_test.go # FIX: 添加 t.Skip 原因说明
├── internal/shared/profile/
│   ├── bootstrap.go                  # FIX: 清理 TODO
│   └── loader.go                     # FIX: 清理 TODO
├── frontend/src/
│   ├── shared/
│   │   └── auth.tsx                  # FIX: 移除硬编码 "pm" → 从 JWT claims 读取
│   └── features/chat/
│       └── ChatPanel.tsx             # FIX: AgentPanel 接入真实数据（预留接口）
```

---

### Task 1: 修复 permission_mode_test.go 编译错误

> `internal/auth/domain/permission_mode_test.go` 引用 undefined: `PermissionContext`, `PermissionDecision`, `DecisionAllow`, `DecisionAskGate`

**Files:**
- Modify: `internal/auth/domain/permission_mode_test.go`
- Modify: `internal/auth/domain/permission_mode.go` — 补全缺失类型

- [ ] **Step 1: 补全缺失类型定义**

Modify `internal/auth/domain/permission_mode.go` — 在现有内容尾部追加:

```go
// PermissionContext holds the context for a permission check.
type PermissionContext struct {
	PipelineID string
	Stage      string
	ToolName   string
	IsReadOnly bool
	Level      string
}

// PermissionDecision is the result of a permission check.
type PermissionDecision string

const (
	DecisionAllow    PermissionDecision = "allow"
	DecisionAskGate  PermissionDecision = "ask_gate"
	DecisionDeny     PermissionDecision = "deny"
)

// EvaluatePermission checks whether a tool call should be allowed based on mode.
func EvaluatePermission(mode PermissionMode, ctx PermissionContext) PermissionDecision {
	switch mode {
	case "bypass":
		return DecisionAllow
	case "auto":
		if ctx.IsReadOnly {
			return DecisionAllow
		}
		return DecisionAskGate
	case "plan":
		if ctx.IsReadOnly {
			return DecisionAllow
		}
		return DecisionDeny
	case "default":
		return DecisionAskGate
	default:
		return DecisionDeny
	}
}
```

- [ ] **Step 2: 修复测试文件中的引用**

Modify `internal/auth/domain/permission_mode_test.go` — 如果使用了不匹配的字段名/方法，对齐到 Step 1 定义:

确保测试使用正确的函数签名 `EvaluatePermission(mode, PermissionContext{...})` 并检查返回值类型 `PermissionDecision`。

- [ ] **Step 3: 运行测试 — PASS**

```bash
go test ./internal/auth/domain/ -v -run TestPermission -count=1
```

Expected: PASS (所有测试通过)

- [ ] **Step 4: Commit**

```bash
git add internal/auth/domain/permission_mode.go internal/auth/domain/permission_mode_test.go
git commit -m "fix(auth): add missing PermissionContext/PermissionDecision types, fix test compilation"
```

---

### Task 2: 修复硬编码 "genesis" prevHash

> `gate_service.go` 中 Approve/Reject 都硬编码 `PrevHash: "genesis"`，需替换为从 DB 最近一条记录计算的真实 hash

**Files:**
- Modify: `internal/pipeline/service/gate_service.go`
- Modify: `internal/pipeline/port/repository.go` — 加 `GetLatestHash` 方法

- [ ] **Step 1: 写 GetLatestHash 测试**

Modify `internal/pipeline/service/gate_service_test.go` — 追加:

```go
func TestGateService_PrevHashChaining(t *testing.T) {
	repo := &stubGateRepo{latestHash: "abc123"}
	pipeRepo := &stubPipeRepo{}
	svc := NewGateService(repo, pipeRepo)

	// First approve should use latestHash from repo
	ctx := context.Background()
	err := svc.Approve(ctx, "pipe-1", "impl", "alice", domain.GateChecklist{}, "ok")
	if err != nil {
		t.Fatal(err)
	}

	// Second approve should chain from first
	err = svc.Approve(ctx, "pipe-1", "test", "alice", domain.GateChecklist{}, "ok")
	if err != nil {
		t.Fatal(err)
	}

	if repo.created[0].PrevHash != "abc123" {
		t.Errorf("first event prevHash = %q, want abc123", repo.created[0].PrevHash)
	}
	if repo.created[1].PrevHash == "genesis" {
		t.Error("second event should not use 'genesis' prevHash")
	}
}
```

更新 `stubGateRepo` — 加 `latestHash` 字段和 `created []*domain.GateEvent`:

```go
type stubGateRepo struct {
	events     []*domain.GateEvent
	latestHash string
	created    []*domain.GateEvent
	err        error
}

func (s *stubGateRepo) CreateEvent(ctx context.Context, ev *domain.GateEvent) error {
	s.created = append(s.created, ev)
	s.latestHash = ev.ContentHash
	return s.err
}
```

- [ ] **Step 2: 运行测试 — FAIL**

```bash
go test ./internal/pipeline/service/ -v -run TestGateService_PrevHashChaining -count=1
```

- [ ] **Step 3: 修改 gate_service.go**

Modify `internal/pipeline/service/gate_service.go` — Approve 方法:

```go
func (s *GateService) Approve(ctx context.Context, pipelineID, stage, actor string, checklist domain.GateChecklist, summary string) error {
	// ... existing code ...

	content := fmt.Sprintf("%s|%s|%s|approve", pipelineID, stage, actor)
	prevHash := s.gateRepo.GetLatestHash(ctx) // replaces hardcoded "genesis"
	ev := &domain.GateEvent{
		// ... existing fields ...
		PrevHash:    prevHash,
		ContentHash: fmt.Sprintf("%x", sha256.Sum256([]byte(prevHash+content))),
	}

	// ... rest unchanged ...
}
```

同样修改 Reject 方法。

- [ ] **Step 4: 在 GateRepository 接口加 GetLatestHash**

Modify `internal/pipeline/port/repository.go`:

```go
type GateRepository interface {
	// ... existing methods ...
	GetLatestHash(ctx context.Context) string
}
```

在 `internal/pipeline/adapter/pg_repository.go` 实现:

```go
func (r *PGRepository) GetLatestHash(ctx context.Context) string {
	var hash string
	err := r.db.QueryRowContext(ctx, `
		SELECT content_hash FROM gate_event
		ORDER BY created_at DESC LIMIT 1
	`).Scan(&hash)
	if err != nil {
		return "" // empty chain for first entry
	}
	return hash
}
```

- [ ] **Step 5: 运行测试 + Commit**

```bash
go test ./internal/pipeline/service/ -v -run TestGate -count=1
git add internal/pipeline/
git commit -m "fix(audit): replace hardcoded 'genesis' prevHash with real DB chain"
```

---

### Task 3: 移除硬编码 "pm" 角色

> `routes.go:102` 和 `auth.tsx:33` 在创建 JWT 时硬编码 `role: "pm"`

**Files:**
- Modify: `internal/server/routes.go` — handleLogin
- Modify: `frontend/src/shared/auth.tsx` — 从 JWT 解析 role

- [ ] **Step 1: 修复后端 — 支持多角色登录**

Modify `internal/server/routes.go` — handleLogin:

```go
func handleLogin(jwtSvc *service.JWTService, cfg *profile.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Username string `json:"username"`
			Password string `json:"password"`
			Role     string `json:"role"`     // NEW: 可选角色字段
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, 400, "invalid request body")
			return
		}
		if req.Username == "" {
			writeError(w, 400, "username required")
			return
		}
		role := req.Role
		if role == "" {
			role = "pm" // default for dev mode
		}
		token, err := jwtSvc.Issue(req.Username, role, "")
		if err != nil {
			writeError(w, 500, "failed to issue token")
			return
		}
		writeJSON(w, 200, token)
	}
}
```

- [ ] **Step 2: 修复前端 — 从 JWT 解析真实 role**

Modify `frontend/src/shared/auth.tsx`:

```tsx
// REMOVE hardcoded line:
// const role = 'pm'; // ← DELETE THIS

// ADD JWT role parsing:
function parseRole(token: string): string {
  try {
    const payload = JSON.parse(atob(token.split('.')[1]));
    return payload.role || 'pm';
  } catch {
    return 'pm';
  }
}
```

更新 AuthProvider 使用 `parseRole(token)` 替代硬编码 `'pm'`。

- [ ] **Step 3: 验证**

```bash
# 1. 不带 role → 默认 pm
curl -X POST :8030/api/auth/login -d '{"username":"test"}' | python3 -c "import sys,json; t=json.load(sys.stdin)['access_token']; print(__import__('base64').b64decode(t.split('.')[1]+'=='))"

# 2. 带 role=admin
curl -X POST :8030/api/auth/login -d '{"username":"admin","role":"admin"}' | python3 -c "import sys,json; t=json.load(sys.stdin)['access_token']; print(__import__('base64').b64decode(t.split('.')[1]+'=='))"
```

- [ ] **Step 4: Commit**

```bash
git add internal/server/routes.go frontend/src/shared/auth.tsx
git commit -m "fix(auth): remove hardcoded 'pm' role, support multi-role login"
```

---

### Task 4: Fork Pipeline 测试补完

> 新增的 `Fork` 方法和 `POST /api/pipelines/{id}/fork` 端点缺少单元测试

**Files:**
- Create: `internal/pipeline/domain/pipeline_fork_test.go`
- Create: `internal/server/routes_fork_test.go`

- [ ] **Step 1: 写 Fork 域逻辑测试**

Create `internal/pipeline/domain/pipeline_fork_test.go`:

```go
package domain

import "testing"

func TestPipeline_Fork(t *testing.T) {
	parent := NewPipeline("pipe-1", "proj-A", "Parent", "alice", 5, 3)
	parent.Region = "bj"
	parent.Config = PipelineConfig{Language: "go", Framework: "gin", MaxAgents: 3}

	child := parent.Fork("pipe-2", "Child Fork", "bob")

	if child.ID != "pipe-2" {
		t.Errorf("child ID = %q, want pipe-2", child.ID)
	}
	if child.ProjectID != parent.ProjectID {
		t.Errorf("child ProjectID = %q, want %q", child.ProjectID, parent.ProjectID)
	}
	if child.Level != "L2" {
		t.Errorf("child Level = %q, want L2 (parent is L1)", child.Level)
	}
	if *child.ParentPipelineID != "pipe-1" {
		t.Errorf("child ParentPipelineID = %q, want pipe-1", *child.ParentPipelineID)
	}
	if child.Region != "bj" {
		t.Errorf("child Region = %q, want bj", child.Region)
	}
	if child.Config.Language != "go" {
		t.Errorf("child Config not inherited: %+v", child.Config)
	}
	if child.Status != "pending" {
		t.Errorf("child Status = %q, want pending", child.Status)
	}
	if len(child.Stages) == 0 {
		t.Fatal("child has no stages")
	}
	if child.CurrentStage == "" {
		t.Error("child CurrentStage is empty — will fail DB CHECK constraint")
	}
}

func TestPipeline_IsSubPipeline(t *testing.T) {
	parent := NewPipeline("pipe-1", "proj-A", "Parent", "alice", 1, 1)
	if parent.IsSubPipeline() {
		t.Error("root pipeline should not be sub-pipeline")
	}

	child := parent.Fork("pipe-2", "Child", "bob")
	if !child.IsSubPipeline() {
		t.Error("forked pipeline should be sub-pipeline")
	}
}

func TestPipeline_Fork_L1ParentYieldsL2Child(t *testing.T) {
	parent := &Pipeline{ID: "p1", ProjectID: "proj-A", Level: "L1"}
	child := parent.Fork("p2", "Child", "alice")
	if child.Level != "L2" {
		t.Errorf("L1 parent → child level = %q, want L2", child.Level)
	}
}

func TestPipeline_Fork_L2ParentYieldsL3Child(t *testing.T) {
	parent := &Pipeline{ID: "p1", ProjectID: "proj-A", Level: "L2"}
	child := parent.Fork("p2", "Child", "alice")
	if child.Level != "L3" {
		t.Errorf("L2 parent → child level = %q, want L3", child.Level)
	}
}
```

- [ ] **Step 2: 运行 Fork 测试 — PASS**

```bash
go test ./internal/pipeline/domain/ -v -run TestPipeline_Fork -count=1
```

- [ ] **Step 3: 写 Fork endpoint 集成测试**

Create `internal/server/routes_fork_test.go`:

```go
package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"openforge/internal/pipeline/domain"
	pipelineadapter "openforge/internal/pipeline/adapter"
	"openforge/internal/pipeline/service"
	"openforge/internal/shared/profile"
)

func TestHandleForkPipeline_Success(t *testing.T) {
	// Setup: Create a parent pipeline in-memory
	repo := &stubPipelineRepo{
		pipelines: map[string]*domain.Pipeline{
			"pipe-parent": domain.NewPipeline("pipe-parent", "proj-A", "Parent", "alice", 1, 1),
		},
	}
	of := &profile.OpenForge{
		PipelineSvc: service.NewPipelineService(repo),
	}

	body, _ := json.Marshal(map[string]string{"title": "My Fork"})
	req := httptest.NewRequest("POST", "/api/pipelines/pipe-parent/fork", bytes.NewReader(body))
	req.SetPathValue("id", "pipe-parent")
	req = req.WithContext(withTestUser(req.Context(), "bob"))
	w := httptest.NewRecorder()

	handleForkPipeline(of)(w, req)

	if w.Code != 201 {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
	var child domain.Pipeline
	json.Unmarshal(w.Body.Bytes(), &child)
	if child.Title != "My Fork" {
		t.Errorf("title = %q, want My Fork", child.Title)
	}
	if child.IsSubPipeline() != true {
		t.Error("forked pipeline should be sub-pipeline")
	}
}

func TestHandleForkPipeline_ParentNotFound(t *testing.T) {
	repo := &stubPipelineRepo{pipelines: map[string]*domain.Pipeline{}}
	of := &profile.OpenForge{
		PipelineSvc: service.NewPipelineService(repo),
	}

	body, _ := json.Marshal(map[string]string{"title": "Ghost Fork"})
	req := httptest.NewRequest("POST", "/api/pipelines/nonexistent/fork", bytes.NewReader(body))
	req.SetPathValue("id", "nonexistent")
	req = req.WithContext(withTestUser(req.Context(), "bob"))
	w := httptest.NewRecorder()

	handleForkPipeline(of)(w, req)

	if w.Code != 500 {
		t.Errorf("expected 500 for not found, got %d", w.Code)
	}
}
```

- [ ] **Step 4: 运行 endpoint 测试 — fix stub 然后 PASS**

```bash
go test ./internal/server/ -v -run TestHandleForkPipeline -count=1
```

- [ ] **Step 5: Commit**

```bash
git add internal/pipeline/domain/pipeline_fork_test.go internal/server/routes_fork_test.go
git commit -m "test: add unit tests for Fork domain logic and endpoint"
```

---

### Task 5: 清理 TODO + 死代码

> 清理 2 个 TODO `bootstrap.go:110` 和 `loader.go:104`

**Files:**
- Modify: `internal/shared/profile/bootstrap.go:110`
- Modify: `internal/shared/profile/loader.go:104`

- [ ] **Step 1: 修复 bootstrap.go TODO**

Modify `internal/shared/profile/bootstrap.go` — 替换:

```go
of.LLMRouter.RegisterProvider("openai", llm.NewOpenAIProvider(
    "https://api.openai.com", string(antAPIKey))) // TODO: use OPENAI_API_KEY in production
```

改为:

```go
// Resolve OpenAI API key: prefer dedicated env var, fallback to Anthropic key
openAIKey, errOAI := of.Secrets.Get(context.Background(), "OPENAI_API_KEY")
if errOAI != nil {
    openAIKey = antAPIKey // fallback for dev
}
of.LLMRouter.RegisterProvider("openai", llm.NewOpenAIProvider(
    "https://api.openai.com", string(openAIKey)))
```

- [ ] **Step 2: 修复 loader.go TODO**

Modify `internal/shared/profile/loader.go:104` — 替换:

```go
// TODO: implement ed25519.Verify(...) once key format is finalized
```

改为:

```go
// Ed25519 signature verification deferred to Phase 8 (per DESIGN.md §6.5).
// When enabled, verify profile signature before bootstrapping.
if cfg.VerifySignature {
    return nil, fmt.Errorf("profile signature verification not yet implemented (Phase 8)")
}
```

- [ ] **Step 3: 编译验证**

```bash
go build ./cmd/server/ && go test ./internal/shared/profile/... -count=1
```

- [ ] **Step 4: Commit**

```bash
git add internal/shared/profile/bootstrap.go internal/shared/profile/loader.go
git commit -m "chore: resolve TODO comments — OpenAI key resolution + Ed25519 deferral"
```

---

### Task 6: AgentPanel 接入真实数据预留

> `ChatPanel.tsx` 中 `AgentPanel agents={[]}` 是空数组占位。添加从后端获取 Agent 列表的接口。

**Files:**
- Modify: `frontend/src/features/chat/ChatPanel.tsx`

- [ ] **Step 1: 添加 Agent 数据获取 hook**

Modify `frontend/src/features/chat/ChatPanel.tsx`:

```tsx
import { useState, useEffect } from 'react';

interface AgentInfo {
  id: string;
  role: string;
  pipeline_id: string;
  parent_id: string;
}

function useAgents(pipelineId: string): AgentInfo[] {
  const [agents, setAgents] = useState<AgentInfo[]>([]);
  useEffect(() => {
    // Phase 6+: fetch from GET /api/pipelines/{id}/agents
    // For now, return empty — agents API not yet exposed
    setAgents([]);
  }, [pipelineId]);
  return agents;
}

export function ChatPanel() {
  // ... existing code ...
  const agents = useAgents(pipelineId);

  return (
    <ChatProvider pipelineId={pipelineId}>
      <div ...>
        <header ...>...</header>
        <AgentPanel agents={agents} />
        <MessageList />
        <MessageInput />
      </div>
    </ChatProvider>
  );
}
```

- [ ] **Step 2: TypeScript 编译**

```bash
cd frontend && npx tsc --noEmit
```

- [ ] **Step 3: Commit**

```bash
git add frontend/src/features/chat/ChatPanel.tsx
git commit -m "feat(frontend): add useAgents hook placeholder for real agent data"
```

---

### Task 7: E2E 验证

- [ ] **Step 1: Go 全量测试**

```bash
go -C /d/vscode/tiktok/openforge test ./internal/... -count=1
```

- [ ] **Step 2: 前端编译 + 构建**

```bash
cd frontend && npx tsc --noEmit && npm run build
```

- [ ] **Step 3: Commit**

```bash
git commit -m "chore(phase5c): final verification — all tests pass, zero build errors"
```

---

## Phase 5c Acceptance Criteria

| # | Criterion | Verification |
|---|-----------|-------------|
| 1 | `permission_mode_test.go` 编译通过，所有测试 PASS | automated |
| 2 | `gate_service.go` prevHash 不再硬编码 "genesis" | automated |
| 3 | login 支持可选 `role` 字段，默认 "pm" | manual |
| 4 | Fork 方法有单元测试覆盖 | automated |
| 5 | Fork endpoint 有集成测试 | automated |
| 6 | 0 个 TODO/FIXME 残留在生产代码中 | automated |
| 7 | `go test ./internal/...` 零失败 | automated |
| 8 | `npm run build` 零错误 | automated |
