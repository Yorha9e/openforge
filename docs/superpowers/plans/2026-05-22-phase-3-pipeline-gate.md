# Phase 3 — Pipeline 状态机 + Diff 预览 + Gate 审批 实现计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 交付 Pipeline 生命周期状态机、Gate 审批流程、Side-by-side Diff 代码审查面板、Dockview 多面板专业模式。对齐 v5 设计: Query Engine (§4.2), PermissionMode (§4.4), ErrorRecovery (§3.15)。

**Architecture:** Pipeline domain 增加状态转换方法（TDD 纯函数），PG adapter 实现 Pipeline/Stage/Gate 持久化，`impl` 阶段通过 QueryEngine.Submit() 驱动 Agent 执行，Gate 判定使用 §4.4 PermissionMode (替代硬编码 L3/L4)，Go HTTP server 增加审批 REST 端点，WebSocket 增加 stage/progress 下行事件，前端新增 ProMode 路由（Dockview 三面板：Chat + Diff + FileTree）和审批收件箱页面。

**Tech Stack:** Go 1.25 + `database/sql` + `lib/pq`, React 19 + TypeScript + dockview + @monaco-editor/react, Postgres 16

> ⚠️ **前端设计提醒**: Task 6-7 涉及 ProMode Dockview、Diff 面板、Gate 审批面板、审批收件箱 UI。**开始前端任务前必须先调用 `Skill("ui-ux-pro-max")`**，按 CLAUDE.md 流程：分析需求 → 生成设计系统 → 输出色板/字体/间距/组件规范。Stack: `react` + `shadcn`。禁止 Inter/Roboto 作为展示字体，禁止紫色→粉色→蓝色渐变。

**Phase 3 关键约束：**
- Pipeline 最多 3 次 backtrack，需 Gate 审批
- Gate 判定使用 §4.4 PermissionMode (替代硬编码 L3/L4: bypass 全部放行, auto/plan 只读放行, default 需 Gate)
- Gate 超时 5min 自动拒绝 (§4.2.4)，不区分 L1/L2/L3/L4
- Verify stage 永远不 auto-close
- Gate 审批含行级评论 + 总结反馈 + checklist
- 审批驳回仅重做标记为 needs_revision 的文件
- Gate 挂起/恢复走 §4.2: Submit() → pending_gate → Resume(GateResult)
- gate_request 表 (审批挂起持久化) 与 gate_event 表 (审计历史) 分离

---
## File Map

```
openforge/
├── internal/
│   ├── pipeline/
│   │   ├── domain/
│   │   │   ├── pipeline.go              # MODIFY: 增加状态转换方法
│   │   │   ├── pipeline_test.go         # NEW: 状态机 table-driven 测试
│   │   │   ├── stage.go                 # MODIFY: 增加 stage lifecycle
│   │   │   ├── gate.go                  # [EXISTS] Gate 值对象
│   │   │   └── complexity.go            # [EXISTS] 复杂度分类器
│   │   ├── port/
│   │   │   └── repository.go            # NEW: PipelineRepository 接口
│   │   ├── adapter/
│   │   │   └── pg_repository.go         # NEW: PG adapter
│   │   └── service/
│   │       ├── pipeline_service.go      # NEW: Pipeline 状态机服务
│   │       ├── pipeline_service_test.go # NEW: 集成测试
│   │       ├── gate_service.go          # NEW: Gate 审批服务
│   │       └── gate_service_test.go     # NEW: Gate 审批测试
│   ├── server/
│   │   ├── routes.go                    # MODIFY: 增加 gate/pipeline 端点
│   │   ├── ws_handler.go               # MODIFY: 增加 stage/gate 下行事件
│   │   └── middleware.go               # [EXISTS]
│   └── shared/profile/
│       └── bootstrap.go                # MODIFY: 注入 Repository
├── config/profiles/
│   └── minimal.yaml                    # MODIFY: 增加 database DSN
├── internal/
│   └── agent/
│       ├── port/
│       │   └── gate_repository.go          # NEW: §4.2.5 GateRepository 接口
│       └── adapter/
│           └── pg_gate_repository.go       # NEW: PG 实现
├── migrations/
│   ├── 001_init.up.sql                    # [EXISTS] 16 tables
│   └── 003_gate_request.up.sql            # NEW: gate_request 表 (§4.2.5)
└── frontend/src/
    ├── shared/
    │   └── api.ts                       # MODIFY: 增加 gate/pipeline API
    ├── features/
    │   ├── review-inbox/
    │   │   └── ReviewInboxPage.tsx      # NEW: 审批收件箱
    │   ├── code-review/
    │   │   ├── ProModePage.tsx          # NEW: Dockview 三面板容器
    │   │   ├── DiffPanel.tsx            # NEW: Monaco side-by-side diff
    │   │   ├── FileTreePanel.tsx        # NEW: 变更文件树
    │   │   └── GatePanel.tsx            # NEW: 审批操作面板
    │   └── chat/
    │       ├── ChatPanel.tsx            # MODIFY: 嵌入 ProMode 面板
    │       └── ChatProvider.tsx         # MODIFY: 增加 card 事件处理
    ├── App.tsx                          # MODIFY: 增加新路由
    └── package.json                     # MODIFY: 增加 dockview, monaco
```

---

### Task 1: Pipeline 状态机 (Domain)

**Files:**
- Modify: `internal/pipeline/domain/pipeline.go`
- Create: `internal/pipeline/domain/pipeline_test.go`

- [ ] **Step 1: 写状态机 table-driven 测试**

Create `internal/pipeline/domain/pipeline_test.go`:
```go
package domain

import "testing"

func TestPipelineStateTransitions(t *testing.T) {
	tests := []struct {
		name      string
		initial   string
		action    string
		want      string
		wantErr   bool
	}{
		// Normal forward flow
		{"pending → running", "pending", "start", "running", false},
		{"running → awaiting_review", "running", "complete_stage", "awaiting_review", false},
		{"awaiting_review → running (approved)", "awaiting_review", "gate_approve", "running", false},
		{"awaiting_review → rejected", "awaiting_review", "gate_reject", "rejected", false},
		// Pause/resume
		{"running → paused", "running", "pause", "paused", false},
		{"paused → running", "paused", "resume", "running", false},
		// Cancellation
		{"running → cancelled", "running", "cancel", "cancelled", false},
		{"pending → cancelled", "pending", "cancel", "cancelled", false},
		// Terminal states reject transitions
		{"completed rejects start", "completed", "start", "", true},
		{"rejected rejects start", "rejected", "start", "", true},
		{"cancelled rejects start", "cancelled", "start", "", true},
		// Token exceeded check
		{"running → token_exceeded", "running", "exceed_token", "token_exceeded", false},
		// Invalid actions
		{"pending rejects pause", "pending", "pause", "", true},
		{"awaiting_review rejects pause", "awaiting_review", "pause", "", true},
		// Backtrack (L3/L4 only)
		{"running → dormant", "running", "backtrack", "dormant", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &Pipeline{Status: tt.initial}
			err := p.Transition(tt.action)
			if (err != nil) != tt.wantErr {
				t.Fatalf("Transition(%q) error = %v, wantErr = %v", tt.action, err, tt.wantErr)
			}
			if !tt.wantErr && p.Status != tt.want {
				t.Errorf("status = %q, want %q", p.Status, tt.want)
			}
		})
	}
}

func TestPipelineAdvanceStage(t *testing.T) {
	p := NewPipeline("p1", "proj1", "Test", "user1", 6, 4) // L3
	if len(p.Stages) != 6 {
		t.Fatalf("L3 should have 6 stages, got %d", len(p.Stages))
	}
	if p.CurrentStage != "clarify" {
		t.Errorf("initial stage = %q, want clarify", p.CurrentStage)
	}

	// Advance through all stages
	expected := []string{"clarify", "decompose", "impl", "test", "deploy", "verify"}
	for _, exp := range expected {
		if p.CurrentStage != exp {
			t.Errorf("stage = %q, want %q", p.CurrentStage, exp)
		}
		p.AdvanceStage()
	}
	if p.Status != "completed" {
		t.Errorf("final status = %q, want completed", p.Status)
	}
}

func TestPipelineBacktrackLimit(t *testing.T) {
	p := NewPipeline("p1", "proj1", "Test", "user1", 10, 5) // L4
	p.Status = "running"
	p.CurrentStage = "impl"

	// 3 backtracks allowed
	for i := 0; i < 3; i++ {
		err := p.Transition("backtrack")
		if err != nil {
			t.Fatalf("backtrack %d should succeed: %v", i, err)
		}
		p.Status = "running" // reset for next test
	}

	// 4th should fail
	err := p.Transition("backtrack")
	if err == nil {
		t.Fatal("4th backtrack should fail")
	}
}
```

- [ ] **Step 2: 运行测试 — 失败**

Run: `go test ./internal/pipeline/domain/ -v -run TestPipeline -count=1`
Expected: FAIL — Transition/AdvanceStage 未定义

- [ ] **Step 3: 实现状态机**

在 `internal/pipeline/domain/pipeline.go` 追加:
```go
import "fmt"

// Transition validates and applies a state transition.
func (p *Pipeline) Transition(action string) error {
	next, ok := validTransitions[p.Status][action]
	if !ok {
		return fmt.Errorf("invalid transition: %q from %q", action, p.Status)
	}
	p.Status = next
	return nil
}

// AdvanceStage moves the pipeline to the next stage.
// If at the last stage, marks pipeline completed.
func (p *Pipeline) AdvanceStage() {
	currentIdx := p.currentStageIndex()
	if currentIdx < 0 || currentIdx >= len(p.Stages) {
		p.Status = "completed"
		return
	}
	p.Stages[currentIdx].Status = "passed"
	nextIdx := currentIdx + 1
	if nextIdx >= len(p.Stages) {
		p.Status = "completed"
		p.CurrentStage = ""
		return
	}
	p.Stages[nextIdx].Status = "running"
	p.CurrentStage = p.Stages[nextIdx].Type
	p.Status = "running"
}

func (p *Pipeline) currentStageIndex() int {
	for i, s := range p.Stages {
		if s.Type == p.CurrentStage {
			return i
		}
	}
	return -1
}

// CanBacktrack returns true if the pipeline can still backtrack.
func (p *Pipeline) CanBacktrack() bool {
	return p.BacktrackCount < 3
}

var validTransitions = map[string]map[string]string{
	"pending": {
		"start":  "running",
		"cancel": "cancelled",
	},
	"running": {
		"complete_stage": "awaiting_review",
		"pause":          "paused",
		"cancel":         "cancelled",
		"exceed_token":   "token_exceeded",
		"backtrack":      "dormant",
	},
	"paused": {
		"resume": "running",
		"cancel": "cancelled",
	},
	"awaiting_review": {
		"gate_approve":  "running",
		"gate_reject":   "rejected",
	},
	"dormant": {
		"resume": "running",
	},
	"token_exceeded": {
		"resume": "running",
	},
}
```

- [ ] **Step 4: 运行测试 — 通过**

Run: `go test ./internal/pipeline/domain/ -v -run TestPipeline -count=1`
Expected: PASS (~20 subtests)

- [ ] **Step 5: 增加 BuildImplPrompt + PermissionMode 引用**

在 `pipeline.go` 追加:
```go
import "openforge/internal/auth/domain" as auth

// BuildImplPrompt 构建实现阶段的 Agent 输入 prompt
func (p *Pipeline) BuildImplPrompt() string {
	return fmt.Sprintf("需求: %s\n模块: %+v", p.Title, p.Stages)
}

// NeedsGate 使用 §4.4 PermissionMode 判定是否需要 Gate 审批
func (p *Pipeline) NeedsGate() bool {
	mode := auth.SelectMode(p.Level, p.CurrentStage)
	return mode == auth.PermissionModeDefault  // bypass/auto/plan 跳过，default 需 Gate
}
```

- [ ] **Step 6: Commit**

```bash
git add internal/pipeline/domain/pipeline.go internal/pipeline/domain/pipeline_test.go
git commit -m "feat(pipeline): add state machine (9 transitions), stage advancement, backtrack limit, PermissionMode gate check"
```

---

### Task 2: Pipeline Repository + PG Adapter

**Files:**
- Create: `internal/pipeline/port/repository.go`
- Create: `internal/pipeline/adapter/pg_repository.go`

- [ ] **Step 1: 安装 lib/pq**

```bash
go get github.com/lib/pq
```

- [ ] **Step 2: 定义 Repository 接口**

Create `internal/pipeline/port/repository.go`:
```go
package port

import (
	"context"
	"time"

	"openforge/internal/pipeline/domain"
)

type PipelineRepository interface {
	Create(ctx context.Context, p *domain.Pipeline) error
	GetByID(ctx context.Context, id string) (*domain.Pipeline, error)
	ListByProject(ctx context.Context, projectID string) ([]*domain.Pipeline, error)
	UpdateStatus(ctx context.Context, id string, status string, version int) error
	IncrementBacktrack(ctx context.Context, id string) error
}

type GateRepository interface {
	CreateEvent(ctx context.Context, ev *domain.GateEvent) error
	ListByPipeline(ctx context.Context, pipelineID string) ([]*domain.GateEvent, error)
	ListPending(ctx context.Context, actor string) ([]*domain.GateEvent, error)
	Claim(ctx context.Context, pipelineID, stage, actor string, ttl time.Duration) error
	ReleaseClaim(ctx context.Context, pipelineID, stage, actor string) error
}
```

- [ ] **Step 3: 实现 PG adapter**

Create `internal/pipeline/adapter/pg_repository.go`:
```go
package adapter

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"openforge/internal/pipeline/domain"
	"openforge/internal/pipeline/port"
)

type PGRepository struct {
	db *sql.DB
}

func NewPGRepository(db *sql.DB) *PGRepository {
	return &PGRepository{db: db}
}

var _ port.PipelineRepository = (*PGRepository)(nil)
var _ port.GateRepository = (*PGRepository)(nil)

// --- PipelineRepository ---

func (r *PGRepository) Create(ctx context.Context, p *domain.Pipeline) error {
	stagesJSON, _ := json.Marshal(p.Stages)
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO pipeline (id, project_id, title, level, status, current_stage, created_by, config)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`, p.ID, p.ProjectID, p.Title, p.Level, p.Status, p.CurrentStage, p.CreatedBy, stagesJSON)
	return err
}

func (r *PGRepository) GetByID(ctx context.Context, id string) (*domain.Pipeline, error) {
	var p domain.Pipeline
	var config []byte
	err := r.db.QueryRowContext(ctx, `
		SELECT id, project_id, title, level, status, current_stage, created_by,
		       backtrack_count, version, created_at, updated_at, config
		FROM pipeline WHERE id = $1 AND deleted_at IS NULL
	`, id).Scan(&p.ID, &p.ProjectID, &p.Title, &p.Level, &p.Status,
		&p.CurrentStage, &p.CreatedBy, &p.BacktrackCount, &p.Version,
		&p.CreatedAt, &p.UpdatedAt, &config)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("pipeline %q not found", id)
	}
	if err != nil {
		return nil, err
	}
	json.Unmarshal(config, &p.Stages)
	return &p, nil
}

func (r *PGRepository) ListByProject(ctx context.Context, projectID string) ([]*domain.Pipeline, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, project_id, title, level, status, current_stage, created_by,
		       backtrack_count, version, created_at
		FROM pipeline WHERE project_id = $1 AND deleted_at IS NULL
		ORDER BY created_at DESC
	`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []*domain.Pipeline
	for rows.Next() {
		var p domain.Pipeline
		if err := rows.Scan(&p.ID, &p.ProjectID, &p.Title, &p.Level, &p.Status,
			&p.CurrentStage, &p.CreatedBy, &p.BacktrackCount, &p.Version, &p.CreatedAt); err != nil {
			return nil, err
		}
		result = append(result, &p)
	}
	return result, nil
}

func (r *PGRepository) UpdateStatus(ctx context.Context, id string, status string, version int) error {
	result, err := r.db.ExecContext(ctx, `
		UPDATE pipeline SET status = $2, version = version + 1, updated_at = NOW()
		WHERE id = $1 AND version = $3 AND deleted_at IS NULL
	`, id, status, version)
	if err != nil {
		return err
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return fmt.Errorf("pipeline %q: optimistic lock conflict (version %d)", id, version)
	}
	return nil
}

func (r *PGRepository) IncrementBacktrack(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE pipeline SET backtrack_count = backtrack_count + 1, updated_at = NOW()
		WHERE id = $1 AND backtrack_count < 3
	`, id)
	return err
}

// --- GateRepository ---

func (r *PGRepository) CreateEvent(ctx context.Context, ev *domain.GateEvent) error {
	comments, _ := json.Marshal(ev.LineComments)
	checklist, _ := json.Marshal(ev.Checklist)
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO gate_event (pipeline_id, stage, event, actor, decision,
			line_comments, summary_feedback, checklist, artifact_hash, prev_hash, content_hash)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
	`, ev.PipelineID, ev.Stage, ev.Event, ev.Actor, ev.Decision,
		comments, ev.SummaryFeedback, checklist,
		ev.ArtifactHash, ev.PrevHash, ev.ContentHash)
	return err
}

func (r *PGRepository) ListByPipeline(ctx context.Context, pipelineID string) ([]*domain.GateEvent, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT pipeline_id, stage, event, actor, decision,
		       line_comments, summary_feedback, checklist, artifact_hash, created_at
		FROM gate_event WHERE pipeline_id = $1 ORDER BY created_at DESC
	`, pipelineID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []*domain.GateEvent
	for rows.Next() {
		var ev domain.GateEvent
		var comments, checklist []byte
		if err := rows.Scan(&ev.PipelineID, &ev.Stage, &ev.Event, &ev.Actor,
			&ev.Decision, &comments, &ev.SummaryFeedback, &checklist,
			&ev.ArtifactHash, &ev.CreatedAt); err != nil {
			return nil, err
		}
		json.Unmarshal(comments, &ev.LineComments)
		json.Unmarshal(checklist, &ev.Checklist)
		events = append(events, &ev)
	}
	return events, nil
}

func (r *PGRepository) ListPending(ctx context.Context, actor string) ([]*domain.GateEvent, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT pipeline_id, stage, event, actor, decision, line_comments,
		       summary_feedback, checklist, artifact_hash, created_at
		FROM gate_event WHERE event = 'awaiting'
		ORDER BY created_at ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []*domain.GateEvent
	for rows.Next() {
		var ev domain.GateEvent
		var comments, checklist []byte
		if err := rows.Scan(&ev.PipelineID, &ev.Stage, &ev.Event, &ev.Actor,
			&ev.Decision, &comments, &ev.SummaryFeedback, &checklist,
			&ev.ArtifactHash, &ev.CreatedAt); err != nil {
			return nil, err
		}
		json.Unmarshal(comments, &ev.LineComments)
		json.Unmarshal(checklist, &ev.Checklist)
		events = append(events, &ev)
	}
	return events, nil
}

func (r *PGRepository) Claim(ctx context.Context, pipelineID, stage, actor string, ttl time.Duration) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO gate_event (pipeline_id, stage, event, actor, prev_hash, content_hash)
		VALUES ($1, $2, 'claimed', $3, 'genesis', 'genesis')
	`, pipelineID, stage, actor)
	return err
}

func (r *PGRepository) ReleaseClaim(ctx context.Context, pipelineID, stage, actor string) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE gate_event SET event = 'awaiting', actor = ''
		WHERE pipeline_id = $1 AND stage = $2 AND event = 'claimed' AND actor = $3
	`, pipelineID, stage, actor)
	return err
}
```

- [ ] **Step 4: 编译验证**

Run: `go build ./internal/pipeline/...`
Expected: 编译通过

- [ ] **Step 5: Commit**

```bash
git add internal/pipeline/port/repository.go internal/pipeline/adapter/pg_repository.go go.mod go.sum
git commit -m "feat(pipeline): add PipelineRepository + GateRepository interfaces and PG adapter"
```

---

### Task 3: Pipeline + Gate Services

**Files:**
- Create: `internal/pipeline/service/pipeline_service.go`
- Create: `internal/pipeline/service/pipeline_service_test.go`
- Create: `internal/pipeline/service/gate_service.go`
- Create: `internal/pipeline/service/gate_service_test.go`

- [ ] **Step 1: 实现 PipelineService**

Create `internal/pipeline/service/pipeline_service.go`:
```go
package service

import (
	"context"
	"fmt"

	"openforge/internal/pipeline/domain"
	"openforge/internal/pipeline/port"
)

type PipelineService struct {
	repo port.PipelineRepository
}

func NewPipelineService(repo port.PipelineRepository) *PipelineService {
	return &PipelineService{repo: repo}
}

func (s *PipelineService) Start(ctx context.Context, id string) error {
	p, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if err := p.Transition("start"); err != nil {
		return err
	}
	p.Stages[0].Status = "running"
	return s.repo.UpdateStatus(ctx, id, p.Status, p.Version)
}

func (s *PipelineService) AdvanceStage(ctx context.Context, id string) error {
	p, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}

	// 使用 §4.4 PermissionMode 判定 (替代硬编码 L3/L4)
	if p.NeedsGate() {
		// auto/plan 模式跳过, default 模式需 Gate 审批
		if err := p.Transition("complete_stage"); err != nil {
			return err
		}
		return s.repo.UpdateStatus(ctx, id, p.Status, p.Version)
	}

	// L1/L2 或 auto/plan 模式: 直接推进 (无需 Gate)
	p.AdvanceStage()
	return s.repo.UpdateStatus(ctx, id, p.Status, p.Version)
}

func (s *PipelineService) Pause(ctx context.Context, id string) error {
	p, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if err := p.Transition("pause"); err != nil {
		return err
	}
	return s.repo.UpdateStatus(ctx, id, p.Status, p.Version)
}

func (s *PipelineService) Resume(ctx context.Context, id string) error {
	p, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if err := p.Transition("resume"); err != nil {
		return err
	}
	return s.repo.UpdateStatus(ctx, id, p.Status, p.Version)
}

func (s *PipelineService) Cancel(ctx context.Context, id string) error {
	p, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if err := p.Transition("cancel"); err != nil {
		return err
	}
	return s.repo.UpdateStatus(ctx, id, p.Status, p.Version)
}
```

- [ ] **Step 2: 实现 GateService (含 gate_request 持久化)**

> v5 对齐: `gate_request` 表用于审批挂起持久化 (§4.2.5), `gate_event` 表用于审计历史 (已有)。GateService 的 approve/reject 先写 gate_request (更新 status+result)，再写 gate_event (审计记录)，最后触发 QueryEngine.Resume()。

Create `internal/pipeline/service/gate_service.go`:

Create `internal/pipeline/service/gate_service.go`:
```go
package service

import (
	"context"
	"crypto/sha256"
	"fmt"
	"time"

	"openforge/internal/pipeline/domain"
	"openforge/internal/pipeline/port"
)

type GateService struct {
	gateRepo  port.GateRepository
	pipeRepo  port.PipelineRepository
	l1l2TTL   time.Duration
}

func NewGateService(gateRepo port.GateRepository, pipeRepo port.PipelineRepository) *GateService {
	return &GateService{
		gateRepo: gateRepo,
		pipeRepo: pipeRepo,
		l1l2TTL:  30 * time.Minute,
	}
}

func (s *GateService) Approve(ctx context.Context, pipelineID, stage, actor string, checklist domain.GateChecklist, summary string) error {
	p, err := s.pipeRepo.GetByID(ctx, pipelineID)
	if err != nil {
		return err
	}

	// Record gate event
	content := fmt.Sprintf("%s|%s|%s|approve", pipelineID, stage, actor)
	ev := &domain.GateEvent{
		PipelineID:      pipelineID,
		Stage:           stage,
		Event:           "approved",
		Actor:           actor,
		Decision:        "approve",
		SummaryFeedback: summary,
		Checklist:       checklist,
		ContentHash:     fmt.Sprintf("%x", sha256.Sum256([]byte(content))),
		PrevHash:        "genesis",
	}
	if err := s.gateRepo.CreateEvent(ctx, ev); err != nil {
		return err
	}

	// Advance pipeline past gate
	if err := p.Transition("gate_approve"); err != nil {
		return err
	}
	p.AdvanceStage()
	return s.pipeRepo.UpdateStatus(ctx, pipelineID, p.Status, p.Version)
}

func (s *GateService) Reject(ctx context.Context, pipelineID, stage, actor string, comments []domain.LineComment, summary string) error {
	p, err := s.pipeRepo.GetByID(ctx, pipelineID)
	if err != nil {
		return err
	}

	ev := &domain.GateEvent{
		PipelineID:      pipelineID,
		Stage:           stage,
		Event:           "rejected",
		Actor:           actor,
		Decision:        "reject",
		LineComments:    comments,
		SummaryFeedback: summary,
		ContentHash:     fmt.Sprintf("%x", sha256.Sum256([]byte(fmt.Sprintf("%s|%s|%s|reject", pipelineID, stage, actor)))),
		PrevHash:        "genesis",
	}
	if err := s.gateRepo.CreateEvent(ctx, ev); err != nil {
		return err
	}

	if err := p.Transition("gate_reject"); err != nil {
		return err
	}
	return s.pipeRepo.UpdateStatus(ctx, pipelineID, p.Status, p.Version)
}

func (s *GateService) Claim(ctx context.Context, pipelineID, stage, actor string) error {
	return s.gateRepo.Claim(ctx, pipelineID, stage, actor, 30*time.Minute)
}

func (s *GateService) Release(ctx context.Context, pipelineID, stage, actor string) error {
	return s.gateRepo.ReleaseClaim(ctx, pipelineID, stage, actor)
}

func (s *GateService) ListPending(ctx context.Context) ([]*domain.GateEvent, error) {
	return s.gateRepo.ListPending(ctx, "")
}
```

- [ ] **Step 3: 编译验证**

Run: `go build ./internal/pipeline/...`
Expected: 编译通过

- [ ] **Step 4: Commit**

```bash
git add internal/pipeline/service/
git commit -m "feat(pipeline): add PipelineService (start/advance/pause/cancel) and GateService (approve/reject/claim)"
```

---

### Task 4: Server Routes — Gate + Pipeline 端点

**Files:**
- Modify: `internal/server/routes.go`
- Modify: `internal/shared/profile/bootstrap.go:35` — 注入 PG DB + Repository

- [ ] **Step 1: 更新 routes.go — 替换 stub 为真实逻辑**

Read current `routes.go` then replace the pipeline/gate stub handlers with real implementations that use `*profile.OpenForge` (will have repo fields after bootstrap changes).

Append these new handlers to `routes.go`:
```go
func handleListProjects(of *profile.OpenForge) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		projects, err := of.PipelineRepo.ListByProject(ctx, "") // Phase 3: all projects
		if err != nil {
			writeError(w, 500, err.Error())
			return
		}
		writeJSON(w, 200, projects)
	}
}

func handleGetPipeline(of *profile.OpenForge) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		p, err := of.PipelineRepo.GetByID(r.Context(), id)
		if err != nil {
			writeError(w, 404, err.Error())
			return
		}
		writeJSON(w, 200, p)
	}
}

func handleCreatePipeline(of *profile.OpenForge) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Title string `json:"title"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, 400, "invalid body")
			return
		}
		projectID := r.PathValue("id")
		userID := UserIDFromContext(r.Context())
		p := domain.NewPipeline(
			"pipe-"+fmt.Sprintf("%d", time.Now().UnixNano()),
			projectID, req.Title, userID, 1, 1,
		)
		if err := of.PipelineRepo.Create(r.Context(), p); err != nil {
			writeError(w, 500, err.Error())
			return
		}
		writeJSON(w, 201, p)
	}
}

func handleApproveGate(of *profile.OpenForge) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		pipelineID := r.PathValue("id")
		stage := r.PathValue("stage")
		actor := UserIDFromContext(r.Context())

		var req struct {
			Checklist domain.GateChecklist `json:"checklist"`
			Summary   string               `json:"summary_feedback"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, 400, "invalid body")
			return
		}
		if err := of.GateSvc.Approve(r.Context(), pipelineID, stage, actor, req.Checklist, req.Summary); err != nil {
			writeError(w, 500, err.Error())
			return
		}
		writeJSON(w, 200, map[string]string{"status": "approved"})
	}
}

func handleRejectGate(of *profile.OpenForge) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		pipelineID := r.PathValue("id")
		stage := r.PathValue("stage")
		actor := UserIDFromContext(r.Context())

		var req struct {
			Comments []domain.LineComment `json:"line_comments"`
			Summary  string               `json:"summary_feedback"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, 400, "invalid body")
			return
		}
		if err := of.GateSvc.Reject(r.Context(), pipelineID, stage, actor, req.Comments, req.Summary); err != nil {
			writeError(w, 500, err.Error())
			return
		}
		writeJSON(w, 200, map[string]string{"status": "rejected"})
	}
}

func handleReviewInbox(of *profile.OpenForge) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		events, err := of.GateSvc.ListPending(r.Context())
		if err != nil {
			writeError(w, 500, err.Error())
			return
		}
		writeJSON(w, 200, events)
	}
}
```

- [ ] **Step 2: 注册新路由**

在 `RegisterRoutes` 函数的 `mux.HandleFunc` 区域，**替换** stub 处理器并**新增** gate 路由:
```go
	// Pipeline (authenticated, real logic)
	mux.HandleFunc("GET /api/pipelines/{id}", authMw(handleGetPipeline(of)))
	mux.HandleFunc("POST /api/projects/{id}/pipelines", authMw(handleCreatePipeline(of)))

	// Gate approval
	mux.HandleFunc("GET /api/review-inbox", authMw(handleReviewInbox(of)))
	mux.HandleFunc("POST /api/pipelines/{id}/gate/{stage}", authMw(handleApproveGate(of)))
	mux.HandleFunc("POST /api/pipelines/{id}/gate/{stage}/reject", authMw(handleRejectGate(of)))
```

- [ ] **Step 3: 注入依赖到 Bootstrap**

Modify `internal/shared/profile/bootstrap.go`:
```go
// Add imports:
import (
	"database/sql"
	_ "github.com/lib/pq"
	"openforge/internal/pipeline/adapter"
	pipesvc "openforge/internal/pipeline/service"
)

// Add to OpenForge struct:
	PipelineRepo *adapter.PGRepository
	PipelineSvc  *pipesvc.PipelineService
	GateSvc      *pipesvc.GateService
	DB           *sql.DB

// In Bootstrap(), before return:
	db, err := sql.Open("postgres", cfg.Database.DSN())
	if err != nil {
		return nil, fmt.Errorf("db: %w", err)
	}
	of.DB = db
	of.PipelineRepo = adapter.NewPGRepository(db)
	of.PipelineSvc = pipesvc.NewPipelineService(of.PipelineRepo)
	of.GateSvc = pipesvc.NewGateService(of.PipelineRepo, of.PipelineRepo)
```

- [ ] **Step 4: 实现 Gate-pause 桥接 (QueryEngine ↔ GateService)**

> v5 对齐: REST approve/reject 收到后 → 查找 pending GateRequest → 更新 gate_request 状态 + 写 gate_event 审计 → Resume QueryEngine

在 `routes.go` 的 `handleApproveGate` 中:
```go
func handleApproveGate(of *profile.OpenForge) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        pipelineID := r.PathValue("id")
        stage := r.PathValue("stage")
        actor := UserIDFromContext(r.Context())

        var req struct {
            Checklist domain.GateChecklist `json:"checklist"`
            Summary   string               `json:"summary_feedback"`
        }
        json.NewDecoder(r.Body).Decode(&req)

        // 1. 查找 pending gate_request (来自 QueryEngine.handleGatePause)
        pending, err := of.GateRepo.GetByPipeline(ctx, pipelineID, stage)
        if err != nil { writeError(w, 404, "no pending gate"); return }

        // 2. 更新 gate_request 状态
        gr := domain.GateResult{Approved: true, ApprovedBy: actor, ApprovedAt: time.Now()}
        of.GateRepo.UpdateResult(ctx, pending.PendingID, gr)

        // 3. 写 gate_event 审计 (已有逻辑)
        of.GateSvc.RecordApprove(ctx, pipelineID, stage, actor, req.Checklist, req.Summary)

        // 4. Resume QueryEngine (通过 WS connection manager 找到对应连接)
        of.WSManager.ResumeQueryEngine(ctx, pipelineID, gr)

        writeJSON(w, 200, map[string]string{"status": "approved"})
    }
}
```

- [ ] **Step 5: 实现 GateRepository + gate_request DDL**

Create `internal/agent/adapter/pg_gate_repository.go` (PG 实现 — Create/Get/UpdateResult/UpdateStatus/ListPending/GetByPipeline).
Create `migrations/003_gate_request.up.sql` (§4.2.5 DDL).

- [ ] **Step 6: 编译验证**

Run: `go build ./cmd/server/`
Expected: 编译通过

- [ ] **Step 7: Commit**

```bash
git add internal/server/routes.go internal/agent/adapter/pg_gate_repository.go migrations/003_gate_request.up.sql internal/shared/profile/bootstrap.go
git commit -m "feat(server): wire pipeline + gate REST endpoints, GateRepository, gate_request DDL, QueryEngine Resume bridge"
```

---

### Task 5: WebSocket — QueryEngine 对接 + Gate 通知

> v5 对齐: `chat.send` 从 `coordinator.ChatStream()` 改为 `QueryEngine.Submit()`。

**Files:**
- Modify: `internal/server/ws_handler.go`

- [ ] **Step 1: chat.send 改为 QueryEngine.Submit()**

替换当前 `chat.send` case 中的 `coordinator.ChatStream()` 调用:
```go
case "chat.send":
    var req struct {
        PipelineID string `json:"pipeline_id"`
        Input      string `json:"msg_text"`
    }
    json.Unmarshal(msg.Payload, &req)

    // 替换: coordinator.ChatStream(ctx, messages, config)
    // 改为: QueryEngine.Submit() — 对齐 v5 §4.2
    pipeline, _ := c.of.PipelineRepo.GetByID(ctx, req.PipelineID)
    qe := domain.NewQueryEngine(
        domain.DefaultQueryEngineConfig,
        c.of.LLMRouter,  // Go-side LLM Router (Phase 1.5 Task 11)
        c.of.ToolReg,    // Tool Registry
    )
    qe.SetMode(auth.SelectMode(pipeline.Level, pipeline.CurrentStage))

    result, err := qe.Submit(ctx, req.Input)
    if err != nil {
        recovery := domain.ClassifyAndRecover(domain.MapErrorCode(err), 0)
        if recovery.Action == domain.ActionEscalate {
            c.writeError("llm error: " + err.Error()); return
        }
        // 自动恢复: 结果已包含恢复后的输出
    }

    // SubmitResult → WS 事件流
    c.write(map[string]any{"type": "chat.stream_done", "payload": map[string]any{
        "msg_seq": msgSeq, "full_text": result.Reply, "token_count": result.TokenUsed,
    }})
    for _, tc := range result.ToolCalls {
        c.write(map[string]any{"type": "msg.card", "payload": map[string]any{
            "msg_seq": msgSeq, "card_type": "tool", "card_data": tc,
        }})
    }

    // stage_change 事件 (LLM 响应完成后)
```

- [ ] **Step 2: 增加下行事件**
```go
	// After stream loop completes:
	if p.CompletedAt != nil {
		c.write(map[string]any{
			"type": "pipeline.stage_change",
			"payload": map[string]string{
				"pipeline_id": p.PipelineID,
				"stage":       p.CurrentStage,
				"status":      p.Status,
			},
		})
	}
```

在 `case "chat.send":` 末尾增加 `token_warning`:
```go
	if qe.TokenUsed() > qe.TokenBudget()*8/10 {
		c.write(map[string]any{
			"type": "pipeline.token_warning",
			"payload": map[string]int{
				"used":   qe.TokenUsed(),
				"budget": qe.TokenBudget(),
			},
		})
	}
```

增加新 `case` 处理:
```go
	case "gate.approve":
		var p struct {
			PipelineID string `json:"pipeline_id"`
			Stage      string `json:"stage"`
		}
		json.Unmarshal(msg.Payload, &p)
		// Gate approval via REST; WS just notifies
		c.write(map[string]any{"type": "gate.notify", "payload": map[string]string{
			"pipeline_id": p.PipelineID, "stage": p.Stage, "event": "approved",
		}})

	case "pipeline.cancel":
		var p struct {
			PipelineID string `json:"pipeline_id"`
		}
		json.Unmarshal(msg.Payload, &p)
		c.of.PipelineSvc.Cancel(context.Background(), p.PipelineID)
		c.write(map[string]any{"type": "pipeline.finished", "payload": map[string]string{
			"pipeline_id": p.PipelineID, "status": "cancelled",
		}})
```

- [ ] **Step 2: 编译验证**

Run: `go build ./cmd/server/`
Expected: 编译通过

- [ ] **Step 3: Commit**

```bash
git add internal/server/ws_handler.go
git commit -m "feat(ws): add pipeline.stage_change, pipeline.token_warning, gate.notify, pipeline.finished events"
```

---

### Task 6: Frontend — ProMode Dockview 布局

> ⚠️ **开始前调用 `Skill("ui-ux-pro-max")`** 生成设计系统。

**Files:**
- Modify: `frontend/package.json` — 加 dockview, @monaco-editor/react
- Create: `frontend/src/features/code-review/ProModePage.tsx`
- Create: `frontend/src/features/code-review/DiffPanel.tsx`
- Create: `frontend/src/features/code-review/FileTreePanel.tsx`
- Modify: `frontend/src/App.tsx` — 加路由

- [ ] **Step 1: 安装依赖**

```bash
cd frontend && npm install dockview @monaco-editor/react
```

- [ ] **Step 2: 创建 ProModePage (Dockview 容器)**

Create `frontend/src/features/code-review/ProModePage.tsx`:
```tsx
import { DockviewReact, DockviewReadyEvent, IDockviewPanelProps } from 'dockview';
import { ChatPanel } from '../chat/ChatPanel';
import { DiffPanel } from './DiffPanel';

const components = {
  chat: (props: IDockviewPanelProps) => (
    <div style={{ height: '100%', overflow: 'hidden' }}>
      <ChatPanel />
    </div>
  ),
  diff: (props: IDockviewPanelProps) => (
    <div style={{ height: '100%' }}>
      <DiffPanel />
    </div>
  ),
  filetree: (props: IDockviewPanelProps) => (
    <div style={{ height: '100%', background: '#1a1a1a', color: '#fff', padding: 12 }}>
      <FileTreePanel />
    </div>
  ),
};

export function ProModePage() {
  const onReady = (event: DockviewReadyEvent) => {
    const api = event.api;
    api.addPanel({ id: 'chat', component: 'chat', title: 'AI Chat' });
    api.addPanel({ id: 'diff', component: 'diff', title: 'Diff View', position: { direction: 'right' } });
    api.addPanel({ id: 'files', component: 'filetree', title: 'Files', position: { direction: 'left' } });
  };

  return (
    <div style={{ height: '100vh', background: '#0f0f0f' }}>
      <DockviewReact components={components} onReady={onReady} className="dockview-theme-dark" />
    </div>
  );
}
```

- [ ] **Step 3: 创建 DiffPanel (Monaco)**

Create `frontend/src/features/code-review/DiffPanel.tsx`:
```tsx
import { DiffEditor } from '@monaco-editor/react';

export function DiffPanel() {
  const original = '// Original code\nfunction hello() {\n  console.log("hello");\n}';
  const modified = '// Modified code\nfunction hello() {\n  console.log("hello world");\n}';

  return (
    <DiffEditor
      height="100%"
      original={original}
      modified={modified}
      language="typescript"
      theme="vs-dark"
      options={{ readOnly: true, minimap: { enabled: false } }}
    />
  );
}
```

- [ ] **Step 4: 创建 FileTreePanel**

Create `frontend/src/features/code-review/FileTreePanel.tsx`:
```tsx
import { useState } from 'react';

interface FileNode {
  path: string;
  status: 'added' | 'modified' | 'deleted';
}

const DEMO_FILES: FileNode[] = [
  { path: 'src/components/Header.tsx', status: 'modified' },
  { path: 'src/utils/api.ts', status: 'modified' },
  { path: 'src/pages/Home.tsx', status: 'added' },
];

const statusColors: Record<string, string> = { added: '#4ade80', modified: '#facc15', deleted: '#f87171' };

export function FileTreePanel() {
  const [selected, setSelected] = useState<string | null>(null);

  return (
    <div>
      <h3 style={{ fontSize: 14, fontWeight: 600, marginBottom: 8, color: '#a3a3a3' }}>Changed Files</h3>
      {DEMO_FILES.map(f => (
        <div
          key={f.path}
          onClick={() => setSelected(f.path)}
          style={{
            padding: '4px 8px', cursor: 'pointer', borderRadius: 4,
            fontSize: 13, color: '#e5e5e5',
            background: selected === f.path ? '#262626' : 'transparent',
            display: 'flex', alignItems: 'center', gap: 6,
          }}
        >
          <span style={{ color: statusColors[f.status], fontSize: 10 }}>●</span>
          <span>{f.path}</span>
        </div>
      ))}
    </div>
  );
}
```

- [ ] **Step 5: 增加路由**

在 `App.tsx` 加:
```tsx
import { ProModePage } from './features/code-review/ProModePage';
// ...inside Routes:
<Route path="/project/:id/pipeline/:pid" element={<ProtectedRoute><ProModePage /></ProtectedRoute>} />
```

- [ ] **Step 6: 编译验证**

```bash
cd frontend && npx tsc --noEmit && npm run build
```
Expected: 无错误

- [ ] **Step 7: Commit**

```bash
git add frontend/
git commit -m "feat(frontend): add ProMode Dockview layout with Chat + Diff + FileTree panels"
```

---

### Task 7: Frontend — Gate 审批 + 审批收件箱

> ⚠️ **开始前调用 `Skill("ui-ux-pro-max")`** 生成设计系统。

**Files:**
- Create: `frontend/src/features/code-review/GatePanel.tsx`
- Create: `frontend/src/features/review-inbox/ReviewInboxPage.tsx`
- Modify: `frontend/src/shared/api.ts` — 增加 gate API
- Modify: `frontend/src/App.tsx` — 加 review-inbox 路由
- Modify: `frontend/src/features/chat/ChatProvider.tsx` — 处理 gate/card 事件

- [ ] **Step 1: 增加 Gate API 到 api.ts**

在 `frontend/src/shared/api.ts` 追加:
```typescript
  // Gate
  getReviewInbox: () => request<any[]>('/review-inbox'),

  approveGate: (pipelineId: string, stage: string, checklist: any, summary: string) =>
    request<any>(`/pipelines/${pipelineId}/gate/${stage}`, {
      method: 'POST',
      body: JSON.stringify({ checklist, summary_feedback: summary }),
    }),

  rejectGate: (pipelineId: string, stage: string, comments: any[], summary: string) =>
    request<any>(`/pipelines/${pipelineId}/gate/${stage}/reject`, {
      method: 'POST',
      body: JSON.stringify({ line_comments: comments, summary_feedback: summary }),
    }),
```

- [ ] **Step 2: 创建 GatePanel**

Create `frontend/src/features/code-review/GatePanel.tsx`:
```tsx
import { useState } from 'react';
import { api } from '../../shared/api';

interface Props {
  pipelineId: string;
  stage: string;
}

export function GatePanel({ pipelineId, stage }: Props) {
  const [summary, setSummary] = useState('');
  const [loading, setLoading] = useState(false);

  const handleApprove = async () => {
    setLoading(true);
    try {
      await api.approveGate(pipelineId, stage,
        { code_reviewed: true, security_checked: true, license_cleared: true, coding_standard_met: true },
        summary,
      );
    } catch (err) { console.error(err); }
    finally { setLoading(false); }
  };

  const handleReject = async () => {
    setLoading(true);
    try {
      await api.rejectGate(pipelineId, stage, [], summary);
    } catch (err) { console.error(err); }
    finally { setLoading(false); }
  };

  return (
    <div style={{ padding: 16, color: '#fff' }}>
      <h3 style={{ fontSize: 16, fontWeight: 600, marginBottom: 12 }}>Gate Review — {stage}</h3>
      <textarea
        placeholder="Review summary..."
        value={summary}
        onChange={e => setSummary(e.target.value)}
        rows={3}
        style={{ width: '100%', padding: 8, background: '#262626', border: '1px solid #404040', borderRadius: 4, color: '#fff', resize: 'vertical', marginBottom: 12, boxSizing: 'border-box' }}
      />
      <div style={{ display: 'flex', gap: 8 }}>
        <button onClick={handleApprove} disabled={loading}
          style={{ flex: 1, padding: '8px 0', background: '#16a34a', color: '#fff', border: 'none', borderRadius: 4, fontWeight: 600, cursor: 'pointer' }}>
          Approve
        </button>
        <button onClick={handleReject} disabled={loading}
          style={{ flex: 1, padding: '8px 0', background: '#dc2626', color: '#fff', border: 'none', borderRadius: 4, fontWeight: 600, cursor: 'pointer' }}>
          Reject
        </button>
      </div>
    </div>
  );
}
```

- [ ] **Step 3: 创建 ReviewInboxPage**

Create `frontend/src/features/review-inbox/ReviewInboxPage.tsx`:
```tsx
import { useEffect, useState } from 'react';
import { Link } from 'react-router-dom';
import { api } from '../../shared/api';

export function ReviewInboxPage() {
  const [items, setItems] = useState<any[]>([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    api.getReviewInbox().then(setItems).catch(console.error).finally(() => setLoading(false));
  }, []);

  return (
    <div style={{ minHeight: '100vh', background: '#0f0f0f', color: '#fff' }}>
      <header style={{ padding: '12px 24px', borderBottom: '1px solid #262626', display: 'flex', alignItems: 'center', gap: 16 }}>
        <Link to="/" style={{ color: '#a3a3a3', textDecoration: 'none' }}>&larr; Dashboard</Link>
        <h1 style={{ fontSize: 18, fontWeight: 700 }}>Review Inbox</h1>
      </header>
      <main style={{ maxWidth: 720, margin: '0 auto', padding: 24 }}>
        {loading ? <p style={{ color: '#a3a3a3' }}>Loading...</p>
        : items.length === 0 ? <p style={{ color: '#a3a3a3' }}>No pending reviews.</p>
        : items.map(item => (
          <div key={item.pipeline_id + item.stage} style={{ background: '#1a1a1a', border: '1px solid #262626', borderRadius: 8, padding: 16, marginBottom: 12 }}>
            <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
              <div>
                <p style={{ fontWeight: 600 }}>Pipeline {item.pipeline_id} — {item.stage}</p>
                <p style={{ color: '#a3a3a3', fontSize: 13 }}>Awaiting since {new Date(item.created_at).toLocaleString()}</p>
              </div>
              <Link to={`/project/${item.pipeline_id}/pipeline/${item.pipeline_id}`}
                style={{ padding: '6px 12px', background: '#2563eb', color: '#fff', borderRadius: 4, textDecoration: 'none', fontSize: 13 }}>
                Review
              </Link>
            </div>
          </div>
        ))}
      </main>
    </div>
  );
}
```

- [ ] **Step 4: 加路由**

在 `App.tsx`:
```tsx
import { ReviewInboxPage } from './features/review-inbox/ReviewInboxPage';
// ...
<Route path="/review-inbox" element={<ProtectedRoute><ReviewInboxPage /></ProtectedRoute>} />
```

- [ ] **Step 5: ChatProvider 增加 card 事件处理**

在 `ChatProvider.tsx` 的 `useEffect` 中增加订阅:
```typescript
    const unsub4 = subscribe('msg.card', (p: any) => {
      setMessages(prev => [...prev, {
        id: `card-${++idCounter.current}`,
        role: 'system',
        content: `[${p?.card_type || 'card'}] ${p?.title || ''}`,
        timestamp: Date.now(),
      }]);
    });
    const unsub5 = subscribe('pipeline.stage_change', (p: any) => {
      setMessages(prev => [...prev, {
        id: `stage-${++idCounter.current}`,
        role: 'system',
        content: `Stage: ${p?.stage} → ${p?.status}`,
        timestamp: Date.now(),
      }]);
    });
    const unsub6 = subscribe('gate.notify', (p: any) => {
      setMessages(prev => [...prev, {
        id: `gate-${++idCounter.current}`,
        role: 'system',
        content: `Gate ${p?.stage}: ${p?.event}`,
        timestamp: Date.now(),
      }]);
    });
    return () => { unsub1(); unsub2(); unsub3(); unsub4(); unsub5(); unsub6(); };
```

- [ ] **Step 6: 编译验证**

```bash
cd frontend && npx tsc --noEmit && npm run build
```
Expected: 无错误

- [ ] **Step 7: Commit**

```bash
git add frontend/
git commit -m "feat(frontend): add gate approval panel, review inbox page, card/stage/gate WS events"
```

---

### Task 8: E2E + Final Verification

- [ ] **Step 1: 运行全部 Go 测试**

```bash
go test ./... -count=1
```
Expected: ALL PASS

- [ ] **Step 2: 运行 go vet**

```bash
go vet ./...
```
Expected: clean

- [ ] **Step 3: 前端编译**

```bash
cd frontend && npx tsc --noEmit && npm run build
```
Expected: 无错误，dist/ 生成

- [ ] **Step 4: 启动完整栈测试**

```bash
# Terminal 1: 启动 PG（如有 docker）
docker compose up -d postgres

# Terminal 2: Go server
go run ./cmd/server/ --addr :8030

# Terminal 3: 验证端点
curl -s http://localhost:8030/api/health
curl -s -X POST http://localhost:8030/api/auth/login -H "Content-Type: application/json" -d '{"username":"test","password":"x"}'
# 用返回的 token 测试受保护端点
TOKEN="<access_token>"
curl -s http://localhost:8030/api/review-inbox -H "Authorization: Bearer $TOKEN"
```

- [ ] **Step 5: Commit**

```bash
git add internal/ frontend/ go.mod go.sum
git commit -m "chore(phase3): final verification — all tests pass, frontend builds"
```

---

## Phase 3 Acceptance Criteria

| # | Criterion | Verification |
|---|-----------|-------------|
| 1 | `go test ./...` ALL PASS | automated |
| 2 | `go vet ./...` clean | automated |
| 3 | `cd frontend && npx tsc --noEmit` clean | automated |
| 4 | `cd frontend && npm run build` succeeds | automated |
| 5 | Pipeline 状态机 9 种转换全部通过测试 | automated |
| 6 | Pipeline 最多 3 次 backtrack，超限拒绝 | automated |
| 7 | Gate approve → pipeline 进入下一 stage | manual |
| 8 | Gate reject → pipeline 标记 rejected | manual |
| 9 | Review inbox 显示待审批列表 | manual |
| 10 | ProMode 页面三面板 Dockview 布局渲染 | manual |
| 11 | Diff 面板 Monaco side-by-side 渲染 | manual |
| 12 | FileTree 面板显示变更文件列表 | manual |
| 13 | WS pipeline.stage_change / gate.notify 事件 | manual |
| 14 | REST /review-inbox 返回 gate events | manual |
