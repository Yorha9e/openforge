# Phase 7 — 自学习系统 + Tool/Skill 处理 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

> 日期: 2026-05-25 (v3 — Phase 6.5 后更新) | 设计文档: DESIGN.md §3.12, §4.3, §4.6, §4.8–§4.11 | 状态: Phase 6.5 ✅ 已完成

**Goal:** 为 OpenForge 构建四层自学习系统 + Tool/Skill 嵌入匹配 + Pipeline 回顾报告 + LLM 优先级调度(WFQ)。

**Phase 6.5 已就绪的组件:**
- `SkillLoader` + `CapabilityInjector` + `UnifiedPriorityEngine` ✅ (Phase 6.5)
- `PromptBuilder` L1→L2→Capability→L4 新构建链 ✅
- `KnowledgeQuerier` 已有完整类型和 LearningEngine 接口 ✅
- Bootstrap 已有 Skill/Capability/Priority 三层注入 ✅
- UnifiedPriorityEngine 的`LearningFactor` 恒为 1.0 — Phase 7 接入 TrajectoryStore 后激活

**Architecture:** KnowledgeQuerier(domain) 已有完整类型和 LearningEngine 接口，learningEngine 为 nil。Phase 7 提供 PG 实现 + 服务层，注入 OpenForge。Tool Registry 从纯 keyword 升级为嵌入匹配。Skill 偏好通过 L2 反馈闭环学习。

**Tech Stack:** Go 1.25 + `database/sql` + `golang.org/x/crypto` (Ed25519), React 19 + TypeScript

**关键约束:**
- 嵌入匹配先走关键词+规则双通道，真实 embedding 模型延后 Phase 7.5
- LLM 队列仍用内存 WFQ，Redis 延后 Phase 8
- 统计检验用简化 t-test 近似
- 复用已有 KnowledgeQuerier、PromptBuilder、ToolRegistry 接口
- §3.12 回顾报告基于 trajectory + preference 数据生成，不依赖新外部服务

---

## Phase 6 审计修正清单

| # | 修正 | 状态 |
|---|------|------|
| 1 | `gate_service.go:38` Approve 的 `prevHash = "genesis"` → 改为空字符串（首条记录） | ✅ 已修复 `5f33abc` |
| 2 | `gate_service.go:80` Reject 的 `prevHash = "genesis"` → 同上 | ✅ 已修复 `5f33abc` |
| 3 | 文件锁 (§3.5) — 延后 Phase 8（依赖多 Coordinator 分片） | 📋 延后 |
| 4 | 沙箱安全 (§6.2) docker-sandbox 切换 — 延后 Phase 8 | 📋 延后 |
| 5 | 负载丢弃 (§3.7) — 延后 Phase 8 | 📋 延后 |
| 6 | 灰度发布 (§3.6) — 延后 Phase 8 | 📋 延后 |

---

## File Map

> **Phase 6.5 已建**: `skill.go`, `skill_loader.go`, `capability_injector.go`, `priority_engine.go`, `tools_stages.go`, `knowledge_querier.go`, `prompt_builder.go` (L1→L2→Cap→L4), `query_engine.go` (PromptBuilder 集成)

```
openforge/
├── migrations/
│   └── 004_learning_tables.up.sql          # NEW
├── internal/agent/
│   ├── domain/
│   │   ├── knowledge_querier.go            # MODIFY: 对接 LearningEngine PG 实现
│   │   ├── prompt_builder.go               # [DONE] L1→L2→Capability→L4 (Phase 6.5)
│   │   ├── skill.go                        # [DONE] Phase 6.5
│   │   ├── skill_loader.go                 # [DONE] Phase 6.5
│   │   ├── capability_injector.go          # [DONE] Phase 6.5
│   │   ├── priority_engine.go              # MODIFY: +TrajectoryStore LearningFactor
│   │   ├── preference_store.go             # NEW
│   │   ├── preference_store_test.go        # NEW
│   │   ├── trajectory_store.go             # NEW
│   │   ├── trajectory_store_test.go        # NEW
│   │   ├── knowledge_snapshot.go           # NEW
│   │   ├── knowledge_snapshot_test.go      # NEW
│   │   ├── ab_experiment.go                # NEW
│   │   ├── ab_experiment_test.go           # NEW
│   │   ├── llm_priority_queue.go           # NEW
│   │   ├── llm_priority_queue_test.go      # NEW
│   │   ├── retrospective.go                # NEW — Pipeline 回顾
│   │   └── retrospective_test.go           # NEW
│   ├── adapter/
│   │   ├── pg_preference_store.go          # NEW
│   │   ├── pg_trajectory_store.go          # NEW
│   │   ├── pg_knowledge_snapshot.go        # NEW
│   │   └── pg_ab_experiment.go             # NEW
│   ├── port/
│   │   ├── tool_registry.go                # MODIFY — 加嵌入搜索能力
│   │   └── learning_client.go              # [EXISTS]
│   └── service/
│       ├── learning_service.go             # NEW — LearningEngine 完整实现
│       ├── preference_service.go           # NEW — L2 + 冲突 resolution
│       ├── trajectory_service.go           # NEW — L3 轨迹 + 模式提取
│       ├── snapshot_service.go             # NEW — 快照/回滚
│       ├── ab_test_service.go              # NEW — A/B 实验
│       ├── retrospective_service.go        # NEW — Pipeline 回顾
│       └── tool_embedding_service.go       # NEW — Tool 嵌入匹配
├── internal/shared/profile/
│   └── bootstrap.go    # MODIFY: ADD Phase 7 learning components (Skill/Cap/Priority already wired)
└── frontend/src/
    └── features/admin/
        └── AdminPage.tsx                   # MODIFY: 追加学习状态卡片

---

### Task 1: 数据库迁移 — 学习表

**Files:**
- Create: `migrations/004_learning_tables.up.sql`
- Create: `migrations/004_learning_tables.down.sql`

- [ ] **Step 1: 创建 migration up SQL**

Create `migrations/004_learning_tables.up.sql`:

```sql
-- Phase 7: Learning Engine tables

CREATE TABLE preference (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id      TEXT NOT NULL REFERENCES project(id),
    key             VARCHAR(128) NOT NULL,
    value           TEXT NOT NULL,
    weight          DECIMAL(5,2) NOT NULL DEFAULT 0,
    source          VARCHAR(32) NOT NULL CHECK (source IN ('code_review','auto_detect','ab_experiment','manual','skill_usage','tool_success')),
    conflict_count  INT NOT NULL DEFAULT 0,
    last_activated  TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(project_id, key, value)
);
CREATE INDEX idx_preference_project ON preference(project_id);

CREATE TABLE trajectory (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id      TEXT NOT NULL REFERENCES project(id),
    pipeline_id     TEXT NOT NULL,
    stage_sequence  TEXT[] NOT NULL,
    total_chat_rounds INT NOT NULL,
    total_tokens    BIGINT NOT NULL,
    backtrack_count INT NOT NULL DEFAULT 0,
    rejection_count INT NOT NULL DEFAULT 0,
    failure_codes   TEXT[],
    successful_patterns TEXT[],
    tools_used      TEXT[],
    skills_matched  TEXT[],
    requirement_summary TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_trajectory_project ON trajectory(project_id);

CREATE TABLE knowledge_snapshot (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id      TEXT NOT NULL REFERENCES project(id),
    version         INT NOT NULL CHECK (version > 0),
    snapshot_data   JSONB NOT NULL,
    signature       VARCHAR(128),
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

CREATE TABLE pipeline_retrospective (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    pipeline_id     TEXT NOT NULL REFERENCES pipeline(id),
    project_id      TEXT NOT NULL REFERENCES project(id),
    duration_seconds INT,
    chat_rounds     INT,
    total_tokens    BIGINT,
    rejection_count INT,
    backtrack_count INT,
    lessons_learned TEXT[],
    improvement_actions TEXT[],
    knowledge_updates TEXT[],               -- knowledge IDs updated by this pipeline
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_retrospective_pipeline ON pipeline_retrospective(pipeline_id);
CREATE INDEX idx_retrospective_project ON pipeline_retrospective(project_id);
```

- [ ] **Step 2: 创建 migration down SQL**

Create `migrations/004_learning_tables.down.sql`:

```sql
DROP TABLE IF EXISTS pipeline_retrospective;
DROP TABLE IF EXISTS ab_experiment_assignment;
DROP TABLE IF EXISTS ab_experiment;
DROP TABLE IF EXISTS knowledge_snapshot;
DROP TABLE IF EXISTS trajectory;
DROP TABLE IF EXISTS preference;
```

- [ ] **Step 3: Commit**

```bash
git add migrations/004_learning_tables.up.sql migrations/004_learning_tables.down.sql
git commit -m "feat(learning): add Phase 7 learning engine DB tables

- preference: L2 feedback loop with conflict_count
- trajectory: L3/L4 pipeline execution with tools_used + skills_matched
- knowledge_snapshot: versioned knowledge state
- ab_experiment + ab_experiment_assignment: A/B test tracking
- pipeline_retrospective: §3.12 post-pipeline review reports
"
```

---

### Task 2: Preference Store + 冲突 Resolution (§4.9)

**Files:**
- Create: `internal/agent/domain/preference_store.go`
- Create: `internal/agent/domain/preference_store_test.go`
- Create: `internal/agent/adapter/pg_preference_store.go`

- [ ] **Step 1: 写 PreferenceStore 接口和冲突 resolution 逻辑**

Create `internal/agent/domain/preference_store.go`:

```go
package domain

import (
	"context"
	"sort"
	"sync"
	"time"
)

// PreferenceStore persists learned preferences (L2 feedback loop).
type PreferenceStore interface {
	Upsert(ctx context.Context, pref PreferenceRecord) error
	ListByProject(ctx context.Context, projectID string) ([]PreferenceRecord, error)
	Get(ctx context.Context, projectID, key string) ([]PreferenceRecord, error)
	ResolveConflict(ctx context.Context, projectID, key string) (*PreferenceRecord, error)
}

// PreferenceRecord is a persisted preference entry.
type PreferenceRecord struct {
	ID             string
	ProjectID      string
	Key            string
	Value          string
	Weight         float64
	Source         string
	ConflictCount  int
	LastActivated  string // ISO8601
}

// ResolveConflict applies §4.9 conflict rules: 频次 > 时间 > Agent 置信度.
// Returns the winning preference or nil if no records exist.
func ResolveConflict(records []PreferenceRecord) *PreferenceRecord {
	if len(records) == 0 {
		return nil
	}
	// Sort by: conflict_count desc → last_activated desc → weight desc
	sort.Slice(records, func(i, j int) bool {
		if records[i].ConflictCount != records[j].ConflictCount {
			return records[i].ConflictCount > records[j].ConflictCount
		}
		if records[i].LastActivated != records[j].LastActivated {
			return records[i].LastActivated > records[j].LastActivated
		}
		return records[i].Weight > records[j].Weight
	})
	return &records[0]
}

// MergeSimilarPreferences merges preferences with similarity > 0.95.
// Returns the merged list with deduplicated entries.
func MergeSimilarPreferences(prefs []PreferenceRecord) []PreferenceRecord {
	if len(prefs) <= 1 {
		return prefs
	}
	seen := make(map[string]bool)
	var result []PreferenceRecord
	for _, p := range prefs {
		fingerprint := p.Key + "::" + p.Value
		if seen[fingerprint] {
			continue
		}
		seen[fingerprint] = true
		result = append(result, p)
	}
	return result
}

// MemPreferenceStore is an in-memory implementation for testing.
type MemPreferenceStore struct {
	mu   sync.RWMutex
	data map[string][]PreferenceRecord
}

func NewMemPreferenceStore() *MemPreferenceStore {
	return &MemPreferenceStore{data: make(map[string][]PreferenceRecord)}
}

func (s *MemPreferenceStore) Upsert(_ context.Context, pref PreferenceRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	list := s.data[pref.ProjectID]
	for i, p := range list {
		if p.Key == pref.Key && p.Value == pref.Value {
			list[i].Weight = pref.Weight
			list[i].ConflictCount++
			list[i].LastActivated = time.Now().UTC().Format(time.RFC3339)
			return nil
		}
	}
	pref.LastActivated = time.Now().UTC().Format(time.RFC3339)
	s.data[pref.ProjectID] = append(list, pref)
	return nil
}

func (s *MemPreferenceStore) ListByProject(_ context.Context, projectID string) ([]PreferenceRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]PreferenceRecord, len(s.data[projectID]))
	copy(result, s.data[projectID])
	return result, nil
}

func (s *MemPreferenceStore) Get(_ context.Context, projectID, key string) ([]PreferenceRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var result []PreferenceRecord
	for _, p := range s.data[projectID] {
		if p.Key == key {
			result = append(result, p)
		}
	}
	return result, nil
}

func (s *MemPreferenceStore) ResolveConflict(_ context.Context, projectID, key string) (*PreferenceRecord, error) {
	records, _ := s.Get(context.Background(), projectID, key)
	return ResolveConflict(records), nil
}
```

- [ ] **Step 2: 写测试**

Create `internal/agent/domain/preference_store_test.go`:

```go
package domain

import (
	"context"
	"testing"
)

func TestMemPreferenceStore_UpsertAndList(t *testing.T) {
	store := NewMemPreferenceStore()
	ctx := context.Background()

	store.Upsert(ctx, PreferenceRecord{ProjectID: "proj-A", Key: "naming", Value: "camelCase", Weight: 5.0, Source: "code_review"})
	store.Upsert(ctx, PreferenceRecord{ProjectID: "proj-A", Key: "error_handling", Value: "try-catch", Weight: 3.0, Source: "auto_detect"})

	list, err := store.ListByProject(ctx, "proj-A")
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 {
		t.Fatalf("expected 2 preferences, got %d", len(list))
	}
}

func TestMemPreferenceStore_UpsertExisting(t *testing.T) {
	store := NewMemPreferenceStore()
	ctx := context.Background()

	store.Upsert(ctx, PreferenceRecord{ProjectID: "proj-A", Key: "naming", Value: "camelCase", Weight: 3.0})
	store.Upsert(ctx, PreferenceRecord{ProjectID: "proj-A", Key: "naming", Value: "camelCase", Weight: 7.0})

	list, _ := store.ListByProject(ctx, "proj-A")
	if len(list) != 1 {
		t.Fatalf("expected 1 preference after upsert, got %d", len(list))
	}
	if list[0].Weight != 7.0 {
		t.Errorf("expected weight 7.0, got %f", list[0].Weight)
	}
	if list[0].ConflictCount != 1 {
		t.Errorf("expected conflict_count 1, got %d", list[0].ConflictCount)
	}
}

func TestResolveConflict_FrequencyWins(t *testing.T) {
	records := []PreferenceRecord{
		{Key: "naming", Value: "camelCase", ConflictCount: 10, Weight: 3.0},
		{Key: "naming", Value: "snake_case", ConflictCount: 2, Weight: 9.0},
	}
	winner := ResolveConflict(records)
	if winner == nil || winner.Value != "camelCase" {
		t.Errorf("expected camelCase (higher frequency), got %v", winner)
	}
}

func TestResolveConflict_TimeBreaksTie(t *testing.T) {
	records := []PreferenceRecord{
		{Key: "naming", Value: "camelCase", ConflictCount: 5, LastActivated: "2026-05-20T00:00:00Z"},
		{Key: "naming", Value: "PascalCase", ConflictCount: 5, LastActivated: "2026-05-24T00:00:00Z"},
	}
	winner := ResolveConflict(records)
	if winner == nil || winner.Value != "PascalCase" {
		t.Errorf("expected PascalCase (more recent), got %v", winner)
	}
}

func TestMergeSimilarPreferences(t *testing.T) {
	prefs := []PreferenceRecord{
		{Key: "naming", Value: "camelCase"},
		{Key: "naming", Value: "camelCase"},  // duplicate
		{Key: "error_handling", Value: "try-catch"},
	}
	merged := MergeSimilarPreferences(prefs)
	if len(merged) != 2 {
		t.Errorf("expected 2 after merge, got %d", len(merged))
	}
}
```

- [ ] **Step 3: TDD 循环**

```bash
cd /d/vscode/tiktok/openforge
go test ./internal/agent/domain/ -v -run "TestMemPreference|TestResolveConflict|TestMergeSimilar" -count=1
```
Expected: FAIL → then PASS after Step 1 code is created.

- [ ] **Step 4: 实现 PG adapter**

Create `internal/agent/adapter/pg_preference_store.go`:

```go
package adapter

import (
	"context"
	"database/sql"

	"openforge/internal/agent/domain"
)

type PGPreferenceStore struct {
	db *sql.DB
}

func NewPGPreferenceStore(db *sql.DB) *PGPreferenceStore {
	return &PGPreferenceStore{db: db}
}

func (s *PGPreferenceStore) Upsert(ctx context.Context, pref domain.PreferenceRecord) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO preference (project_id, key, value, weight, source, conflict_count, last_activated)
		VALUES ($1, $2, $3, $4, $5, 1, NOW())
		ON CONFLICT (project_id, key, value) DO UPDATE SET
			weight = EXCLUDED.weight,
			conflict_count = preference.conflict_count + 1,
			last_activated = NOW()
	`, pref.ProjectID, pref.Key, pref.Value, pref.Weight, pref.Source)
	return err
}

func (s *PGPreferenceStore) ListByProject(ctx context.Context, projectID string) ([]domain.PreferenceRecord, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, project_id, key, value, weight, source, conflict_count,
			COALESCE(last_activated::text, '')
		FROM preference WHERE project_id = $1 ORDER BY weight DESC
	`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []domain.PreferenceRecord
	for rows.Next() {
		var r domain.PreferenceRecord
		if err := rows.Scan(&r.ID, &r.ProjectID, &r.Key, &r.Value, &r.Weight, &r.Source, &r.ConflictCount, &r.LastActivated); err != nil {
			return nil, err
		}
		result = append(result, r)
	}
	return result, rows.Err()
}

func (s *PGPreferenceStore) Get(ctx context.Context, projectID, key string) ([]domain.PreferenceRecord, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, project_id, key, value, weight, source, conflict_count,
			COALESCE(last_activated::text, '')
		FROM preference WHERE project_id = $1 AND key = $2 ORDER BY conflict_count DESC, last_activated DESC
	`, projectID, key)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []domain.PreferenceRecord
	for rows.Next() {
		var r domain.PreferenceRecord
		if err := rows.Scan(&r.ID, &r.ProjectID, &r.Key, &r.Value, &r.Weight, &r.Source, &r.ConflictCount, &r.LastActivated); err != nil {
			return nil, err
		}
		result = append(result, r)
	}
	return result, rows.Err()
}

func (s *PGPreferenceStore) ResolveConflict(ctx context.Context, projectID, key string) (*domain.PreferenceRecord, error) {
	records, err := s.Get(ctx, projectID, key)
	if err != nil {
		return nil, err
	}
	return domain.ResolveConflict(records), nil
}

var _ domain.PreferenceStore = (*PGPreferenceStore)(nil)
```

- [ ] **Step 5: 编译 + 测试 + Commit**

```bash
go build ./... && go test ./internal/agent/... -count=1
git add internal/agent/domain/preference_store.go internal/agent/domain/preference_store_test.go internal/agent/adapter/pg_preference_store.go
git commit -m "feat(learning): add PreferenceStore with conflict resolution (§4.9)

- PreferenceStore interface with ResolveConflict
- Conflict rules: frequency > recency > agent confidence
- MergeSimilarPreferences for deduplication
- PGPreferenceStore with ON CONFLICT upsert
"
```

---

### Task 3: Trajectory Store — L3 轨迹学习 + Tool/Skill 跟踪

**Files:**
- Create: `internal/agent/domain/trajectory_store.go`
- Create: `internal/agent/domain/trajectory_store_test.go`
- Create: `internal/agent/adapter/pg_trajectory_store.go`

- [ ] **Step 1: 写 TrajectoryStore 接口**

Create `internal/agent/domain/trajectory_store.go`:

```go
package domain

import (
	"context"
	"sync"
)

// TrajectoryStore persists pipeline execution trajectories (L3).
type TrajectoryStore interface {
	Record(ctx context.Context, t TrajectoryRecord) error
	ListByProject(ctx context.Context, projectID string) ([]TrajectoryRecord, error)
	GetByPipeline(ctx context.Context, pipelineID string) (*TrajectoryRecord, error)
	SimilarPatterns(ctx context.Context, projectID string, failureCodes []string, topK int) ([]TrajectoryRecord, error)
	SuccessfulTools(ctx context.Context, projectID string, stage string, topK int) ([]string, error)
	MatchedSkills(ctx context.Context, projectID string, requirement string, topK int) ([]string, error)
}

// TrajectoryRecord is a persisted pipeline execution trajectory.
type TrajectoryRecord struct {
	ID                 string
	ProjectID          string
	PipelineID         string
	StageSequence      []string
	TotalChatRounds    int
	TotalTokens        int64
	BacktrackCount     int
	RejectionCount     int
	FailureCodes       []string
	SuccessfulPatterns []string
	ToolsUsed          []string
	SkillsMatched      []string
	RequirementSummary string
}

// MemTrajectoryStore is an in-memory implementation for testing.
type MemTrajectoryStore struct {
	mu   sync.RWMutex
	data map[string][]TrajectoryRecord
}

func NewMemTrajectoryStore() *MemTrajectoryStore {
	return &MemTrajectoryStore{data: make(map[string][]TrajectoryRecord)}
}

func (s *MemTrajectoryStore) Record(_ context.Context, t TrajectoryRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data[t.ProjectID] = append(s.data[t.ProjectID], t)
	return nil
}

func (s *MemTrajectoryStore) ListByProject(_ context.Context, projectID string) ([]TrajectoryRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]TrajectoryRecord, len(s.data[projectID]))
	copy(result, s.data[projectID])
	return result, nil
}

func (s *MemTrajectoryStore) GetByPipeline(_ context.Context, pipelineID string) (*TrajectoryRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, list := range s.data {
		for _, t := range list {
			if t.PipelineID == pipelineID {
				return &t, nil
			}
		}
	}
	return nil, nil
}

func (s *MemTrajectoryStore) SimilarPatterns(_ context.Context, projectID string, failureCodes []string, topK int) ([]TrajectoryRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	codeSet := make(map[string]bool, len(failureCodes))
	for _, c := range failureCodes {
		codeSet[c] = true
	}
	var matches []TrajectoryRecord
	for _, t := range s.data[projectID] {
		overlap := 0
		for _, fc := range t.FailureCodes {
			if codeSet[fc] {
				overlap++
			}
		}
		if overlap > 0 {
			matches = append(matches, t)
			if len(matches) >= topK {
				break
			}
		}
	}
	return matches, nil
}

func (s *MemTrajectoryStore) SuccessfulTools(_ context.Context, projectID string, stage string, topK int) ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	toolCounts := make(map[string]int)
	for _, t := range s.data[projectID] {
		if len(t.FailureCodes) == 0 {
			for _, tool := range t.ToolsUsed {
				toolCounts[tool]++
			}
		}
	}
	type kv struct{ k string; v int }
	var sorted []kv
	for k, v := range toolCounts {
		sorted = append(sorted, kv{k, v})
	}
	// Sort descending by count
	for i := 0; i < len(sorted); i++ {
		for j := i + 1; j < len(sorted); j++ {
			if sorted[j].v > sorted[i].v {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}
	result := make([]string, 0, topK)
	for i := 0; i < len(sorted) && i < topK; i++ {
		result = append(result, sorted[i].k)
	}
	return result, nil
}

func (s *MemTrajectoryStore) MatchedSkills(_ context.Context, projectID string, requirement string, topK int) ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	skillCounts := make(map[string]int)
	for _, t := range s.data[projectID] {
		for _, skill := range t.SkillsMatched {
			skillCounts[skill]++
		}
	}
	type kv struct{ k string; v int }
	var sorted []kv
	for k, v := range skillCounts {
		sorted = append(sorted, kv{k, v})
	}
	for i := 0; i < len(sorted); i++ {
		for j := i + 1; j < len(sorted); j++ {
			if sorted[j].v > sorted[i].v {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}
	result := make([]string, 0, topK)
	for i := 0; i < len(sorted) && i < topK; i++ {
		result = append(result, sorted[i].k)
	}
	return result, nil
}
```

- [ ] **Step 2: 写测试**

Create `internal/agent/domain/trajectory_store_test.go`:

```go
package domain

import (
	"context"
	"testing"
)

func TestMemTrajectoryStore_RecordAndList(t *testing.T) {
	store := NewMemTrajectoryStore()
	ctx := context.Background()

	store.Record(ctx, TrajectoryRecord{
		ProjectID: "proj-A", PipelineID: "pipe-1",
		ToolsUsed: []string{"bash", "write_file"},
		SkillsMatched: []string{"react-pattern", "testing"},
		FailureCodes: []string{"MODEL_HALLUCINATION"},
	})

	list, err := store.ListByProject(ctx, "proj-A")
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1, got %d", len(list))
	}
}

func TestMemTrajectoryStore_SuccessfulTools(t *testing.T) {
	store := NewMemTrajectoryStore()
	ctx := context.Background()

	store.Record(ctx, TrajectoryRecord{ProjectID: "proj-A", PipelineID: "p1", ToolsUsed: []string{"bash", "grep"}})
	store.Record(ctx, TrajectoryRecord{ProjectID: "proj-A", PipelineID: "p2", ToolsUsed: []string{"bash", "write_file"}, FailureCodes: []string{"ERR"}})
	store.Record(ctx, TrajectoryRecord{ProjectID: "proj-A", PipelineID: "p3", ToolsUsed: []string{"bash"}})

	tools, err := store.SuccessfulTools(ctx, "proj-A", "", 3)
	if err != nil {
		t.Fatal(err)
	}
	if tools[0] != "bash" {
		t.Errorf("most successful tool should be bash, got %v", tools)
	}
}

func TestMemTrajectoryStore_SimilarPatterns(t *testing.T) {
	store := NewMemTrajectoryStore()
	ctx := context.Background()

	store.Record(ctx, TrajectoryRecord{
		ProjectID: "proj-A", PipelineID: "pipe-1",
		FailureCodes: []string{"MODEL_HALLUCINATION", "CONTEXT_OVERFLOW"},
	})
	store.Record(ctx, TrajectoryRecord{
		ProjectID: "proj-A", PipelineID: "pipe-2",
		FailureCodes: []string{"DEPENDENCY_CONFLICT"},
	})

	matches, _ := store.SimilarPatterns(ctx, "proj-A", []string{"MODEL_HALLUCINATION"}, 10)
	if len(matches) != 1 {
		t.Fatalf("expected 1 match, got %d", len(matches))
	}
}
```

- [ ] **Step 3: 实现 PG adapter**

Create `internal/agent/adapter/pg_trajectory_store.go`:

```go
package adapter

import (
	"context"
	"database/sql"

	"github.com/lib/pq"

	"openforge/internal/agent/domain"
)

type PGTrajectoryStore struct {
	db *sql.DB
}

func NewPGTrajectoryStore(db *sql.DB) *PGTrajectoryStore {
	return &PGTrajectoryStore{db: db}
}

func (s *PGTrajectoryStore) Record(ctx context.Context, t domain.TrajectoryRecord) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO trajectory (project_id, pipeline_id, stage_sequence, total_chat_rounds,
			total_tokens, backtrack_count, rejection_count, failure_codes, successful_patterns,
			tools_used, skills_matched, requirement_summary)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
	`, t.ProjectID, t.PipelineID, pq.Array(t.StageSequence), t.TotalChatRounds,
		t.TotalTokens, t.BacktrackCount, t.RejectionCount,
		pq.Array(t.FailureCodes), pq.Array(t.SuccessfulPatterns),
		pq.Array(t.ToolsUsed), pq.Array(t.SkillsMatched), t.RequirementSummary)
	return err
}

func (s *PGTrajectoryStore) ListByProject(ctx context.Context, projectID string) ([]domain.TrajectoryRecord, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, project_id, pipeline_id, stage_sequence, total_chat_rounds,
			total_tokens, backtrack_count, rejection_count, failure_codes, successful_patterns,
			tools_used, skills_matched, COALESCE(requirement_summary, '')
		FROM trajectory WHERE project_id = $1 ORDER BY created_at DESC
	`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanTrajectories(rows)
}

func (s *PGTrajectoryStore) GetByPipeline(ctx context.Context, pipelineID string) (*domain.TrajectoryRecord, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, project_id, pipeline_id, stage_sequence, total_chat_rounds,
			total_tokens, backtrack_count, rejection_count, failure_codes, successful_patterns,
			tools_used, skills_matched, COALESCE(requirement_summary, '')
		FROM trajectory WHERE pipeline_id = $1
	`, pipelineID)
	r := &domain.TrajectoryRecord{}
	err := row.Scan(&r.ID, &r.ProjectID, &r.PipelineID, pq.Array(&r.StageSequence),
		&r.TotalChatRounds, &r.TotalTokens, &r.BacktrackCount, &r.RejectionCount,
		pq.Array(&r.FailureCodes), pq.Array(&r.SuccessfulPatterns),
		pq.Array(&r.ToolsUsed), pq.Array(&r.SkillsMatched), &r.RequirementSummary)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return r, err
}

func (s *PGTrajectoryStore) SimilarPatterns(ctx context.Context, projectID string, failureCodes []string, topK int) ([]domain.TrajectoryRecord, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, project_id, pipeline_id, stage_sequence, total_chat_rounds,
			total_tokens, backtrack_count, rejection_count, failure_codes, successful_patterns,
			tools_used, skills_matched, COALESCE(requirement_summary, '')
		FROM trajectory WHERE project_id = $1 AND failure_codes && $2
		ORDER BY created_at DESC LIMIT $3
	`, projectID, pq.Array(failureCodes), topK)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanTrajectories(rows)
}

func (s *PGTrajectoryStore) SuccessfulTools(ctx context.Context, projectID string, stage string, topK int) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT unnest(tools_used) AS tool, COUNT(*) as c
		FROM trajectory
		WHERE project_id = $1 AND failure_codes = '{}'
		GROUP BY tool ORDER BY c DESC LIMIT $2
	`, projectID, topK)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []string
	for rows.Next() {
		var tool string
		var count int
		rows.Scan(&tool, &count)
		result = append(result, tool)
	}
	return result, rows.Err()
}

func (s *PGTrajectoryStore) MatchedSkills(ctx context.Context, projectID string, requirement string, topK int) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT unnest(skills_matched) AS skill, COUNT(*) as c
		FROM trajectory
		WHERE project_id = $1 AND skills_matched IS NOT NULL
		GROUP BY skill ORDER BY c DESC LIMIT $2
	`, projectID, topK)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []string
	for rows.Next() {
		var skill string
		var count int
		rows.Scan(&skill, &count)
		result = append(result, skill)
	}
	return result, rows.Err()
}

func scanTrajectories(rows *sql.Rows) ([]domain.TrajectoryRecord, error) {
	var result []domain.TrajectoryRecord
	for rows.Next() {
		var r domain.TrajectoryRecord
		if err := rows.Scan(&r.ID, &r.ProjectID, &r.PipelineID, pq.Array(&r.StageSequence),
			&r.TotalChatRounds, &r.TotalTokens, &r.BacktrackCount, &r.RejectionCount,
			pq.Array(&r.FailureCodes), pq.Array(&r.SuccessfulPatterns),
			pq.Array(&r.ToolsUsed), pq.Array(&r.SkillsMatched), &r.RequirementSummary); err != nil {
			return nil, err
		}
		result = append(result, r)
	}
	return result, rows.Err()
}

var _ domain.TrajectoryStore = (*PGTrajectoryStore)(nil)
```

- [ ] **Step 4: 编译 + 测试 + Commit**

```bash
go build ./... && go test ./internal/agent/... -count=1
git add internal/agent/domain/trajectory_store.go internal/agent/domain/trajectory_store_test.go internal/agent/adapter/pg_trajectory_store.go
git commit -m "feat(learning): add TrajectoryStore with tool/skill tracking

- TrajectoryRecord includes tools_used and skills_matched
- SuccessfulTools() ranks tools by success rate
- MatchedSkills() ranks skills by usage frequency
- SimilarPatterns() matches failure patterns across trajectories
"
```

---

### Task 4: Knowledge Snapshots — 版本管理与回滚 (§4.9)

**Files:**
- Create: `internal/agent/domain/knowledge_snapshot.go`
- Create: `internal/agent/domain/knowledge_snapshot_test.go`
- Create: `internal/agent/adapter/pg_knowledge_snapshot.go`

代码与 v1 计划相同（略）。包含 Ed25519 Sign/Verify、SnapshotStore 接口、Mem/PG 实现。

Commit:
```bash
git add internal/agent/domain/knowledge_snapshot.go internal/agent/domain/knowledge_snapshot_test.go internal/agent/adapter/pg_knowledge_snapshot.go
git commit -m "feat(learning): add KnowledgeSnapshot with Ed25519 signing (§4.9)"
```

---

### Task 5: A/B Testing Engine (§4.10)

**Files:**
- Create: `internal/agent/domain/ab_experiment.go`
- Create: `internal/agent/domain/ab_experiment_test.go`
- Create: `internal/agent/adapter/pg_ab_experiment.go`

代码与 v1 计划相同（略）。包含 SimpleTTest、DetermineVerdict、ExperimentStore。

Commit:
```bash
git add internal/agent/domain/ab_experiment.go internal/agent/domain/ab_experiment_test.go internal/agent/adapter/pg_ab_experiment.go
git commit -m "feat(learning): add A/B testing engine with statistical verdict (§4.10)"
```

---

### Task 6: LLM Priority Queue — WFQ 调度器 (§4.6)

**Files:**
- Create: `internal/agent/domain/llm_priority_queue.go`
- Create: `internal/agent/domain/llm_priority_queue_test.go`

代码与 v1 计划相同（略）。P0/P1/P2/P3 四级权重，binary heap 实现。

Commit:
```bash
git add internal/agent/domain/llm_priority_queue.go internal/agent/domain/llm_priority_queue_test.go
git commit -m "feat(learning): add WFQ LLM priority queue (§4.6)"
```

---

### Task 7: Pipeline Retrospective Service (§3.12 + §3.14)

**Files:**
- Create: `internal/agent/domain/retrospective.go`
- Create: `internal/agent/domain/retrospective_test.go`
- Create: `internal/agent/service/retrospective_service.go`

- [ ] **Step 1: 写 Retrospective 值对象**

Create `internal/agent/domain/retrospective.go`:

```go
package domain

import "time"

// PipelineRetrospective captures post-pipeline analysis (§3.12).
type PipelineRetrospective struct {
	ID                  string
	PipelineID          string
	ProjectID           string
	DurationSeconds     int
	ChatRounds          int
	TotalTokens         int64
	RejectionCount      int
	BacktrackCount      int
	FailureCodes        []string
	LessonsLearned      []string
	ImprovementActions  []string
	KnowledgeUpdates    []string
	CreatedAt           time.Time
}

// RetrospectiveStore persists pipeline retrospectives.
type RetrospectiveStore interface {
	Create(ctx context.Context, r *PipelineRetrospective) error
	ListByProject(ctx context.Context, projectID string, limit int) ([]PipelineRetrospective, error)
	GetByPipeline(ctx context.Context, pipelineID string) (*PipelineRetrospective, error)
}

// RetrospectiveSummary is the cross-pipeline weekly analysis output.
type RetrospectiveSummary struct {
	ProjectID            string
	PeriodStart          time.Time
	PeriodEnd            time.Time
	TotalPipelines       int
	RejectionFrequency   map[string]int    // failure_code → count
	TopLessons           []string
	SuggestedExperiments []string
	PromotedKnowledge    []string
	DiscardedKnowledge   []string
}
```

- [ ] **Step 2: 写测试**

Create `internal/agent/domain/retrospective_test.go`:

```go
package domain

import (
	"testing"
	"time"
)

func TestPipelineRetrospective_Fields(t *testing.T) {
	r := &PipelineRetrospective{
		PipelineID:        "pipe-42",
		ProjectID:         "proj-A",
		DurationSeconds:   3600,
		ChatRounds:        25,
		TotalTokens:       120000,
		RejectionCount:    2,
		LessonsLearned:    []string{"use zod for validation", "add error boundaries"},
		ImprovementActions: []string{"set up zod schema template", "document error boundary pattern"},
		FailureCodes:      []string{"MODEL_HALLUCINATION"},
		CreatedAt:         time.Now(),
	}
	if r.PipelineID != "pipe-42" {
		t.Errorf("expected pipe-42, got %s", r.PipelineID)
	}
	if len(r.LessonsLearned) != 2 {
		t.Errorf("expected 2 lessons, got %d", len(r.LessonsLearned))
	}
}
```

- [ ] **Step 3: 实现服务**

Create `internal/agent/service/retrospective_service.go`:

```go
package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"openforge/internal/agent/domain"
)

type RetrospectiveService struct {
	retroStore  domain.RetrospectiveStore
	trajStore   domain.TrajectoryStore
	prefStore   domain.PreferenceStore
	expStore    domain.ExperimentStore
}

func NewRetrospectiveService(
	retroStore domain.RetrospectiveStore,
	trajStore domain.TrajectoryStore,
	prefStore domain.PreferenceStore,
	expStore domain.ExperimentStore,
) *RetrospectiveService {
	return &RetrospectiveService{
		retroStore: retroStore,
		trajStore:  trajStore,
		prefStore:  prefStore,
		expStore:   expStore,
	}
}

// Generate creates a retrospective for a completed pipeline.
func (s *RetrospectiveService) Generate(ctx context.Context, projectID, pipelineID string) (*domain.PipelineRetrospective, error) {
	traj, err := s.trajStore.GetByPipeline(ctx, pipelineID)
	if err != nil || traj == nil {
		return nil, fmt.Errorf("trajectory %s: %w", pipelineID, err)
	}

	// Extract lessons from failure patterns
	var lessons, actions []string
	for _, code := range traj.FailureCodes {
		lessons = append(lessons, fmt.Sprintf("Failure %s: future pipelines should verify before proceeding", code))
		switch code {
		case "MODEL_HALLUCINATION":
			actions = append(actions, "Add API existence verification before code generation")
		case "DEPENDENCY_CONFLICT":
			actions = append(actions, "Lock dependency versions in sandbox pre-check")
		case "CONTEXT_OVERFLOW":
			actions = append(actions, "Reduce scope per pipeline; split large features")
		}
	}
	if len(traj.SuccessfulPatterns) > 0 {
		lessons = append(lessons, "Successful patterns: "+strings.Join(traj.SuccessfulPatterns, ", "))
	}

	// Extract knowledge update IDs from preferences changed during this pipeline
	var knowledgeIDs []string
	prefs, _ := s.prefStore.ListByProject(ctx, projectID)
	for _, p := range prefs {
		if p.LastActivated != "" {
			knowledgeIDs = append(knowledgeIDs, fmt.Sprintf("%s=%s", p.Key, p.Value))
		}
	}

	r := &domain.PipelineRetrospective{
		PipelineID:         pipelineID,
		ProjectID:          projectID,
		DurationSeconds:    0,
		ChatRounds:         traj.TotalChatRounds,
		TotalTokens:        traj.TotalTokens,
		RejectionCount:     traj.RejectionCount,
		BacktrackCount:     traj.BacktrackCount,
		FailureCodes:       traj.FailureCodes,
		LessonsLearned:     lessons,
		ImprovementActions: actions,
		KnowledgeUpdates:   knowledgeIDs,
		CreatedAt:          time.Now(),
	}
	if err := s.retroStore.Create(ctx, r); err != nil {
		return nil, err
	}
	return r, nil
}

// CrossPipelineSummary generates a weekly cross-pipeline analysis (§3.12).
func (s *RetrospectiveService) CrossPipelineSummary(ctx context.Context, projectID string, days int) (*domain.RetrospectiveSummary, error) {
	retros, err := s.retroStore.ListByProject(ctx, projectID, days)
	if err != nil {
		return nil, err
	}

	summary := &domain.RetrospectiveSummary{
		ProjectID:          projectID,
		TotalPipelines:     len(retros),
		RejectionFrequency: make(map[string]int),
	}

	for _, r := range retros {
		for _, code := range r.FailureCodes {
			summary.RejectionFrequency[code]++
		}
		if len(r.LessonsLearned) > 0 {
			summary.TopLessons = append(summary.TopLessons, r.LessonsLearned[0])
		}
	}

	// Auto-suggest experiments for frequent failure patterns
	for code, count := range summary.RejectionFrequency {
		if count >= 3 {
			summary.SuggestedExperiments = append(summary.SuggestedExperiments,
				fmt.Sprintf("AB test for %s (occurred %d times)", code, count))
		}
	}

	return summary, nil
}
```

- [ ] **Step 4: 编译 + 测试 + Commit**

```bash
go build ./... && go test ./internal/agent/... -count=1
git add internal/agent/domain/retrospective.go internal/agent/domain/retrospective_test.go internal/agent/service/retrospective_service.go
git commit -m "feat(learning): add Pipeline Retrospective + Cross-Pipeline Summary (§3.12)

- PipelineRetrospective captures lessons learned and improvement actions
- Auto-classifies failure codes into actionable recommendations
- CrossPipelineSummary generates weekly analysis with experiment suggestions
"
```

---

### Task 8: Tool Registry 嵌入匹配升级 + Skill 偏好学习

**Files:**
- Modify: `internal/agent/port/tool_registry.go` — 加 ToolEmbeddingSearcher 接口
- Create: `internal/agent/service/tool_embedding_service.go` — Tool/Skill 嵌入匹配

- [ ] **Step 1: 扩展 ToolRegistry 接口**

Modify `internal/agent/port/tool_registry.go` — 在现有 ToolSearcher 接口后加:

```go
// ToolEmbeddingSearcher extends ToolSearcher with embedding-based matching.
// Phase 7: keyword+rule dual-channel (Phase 7.5: real embedding vectors).
type ToolEmbeddingSearcher interface {
	ToolSearcher
	// SearchWithContext ranks tools by relevance to the current task context.
	SearchWithContext(ctx context.Context, query string, taskContext TaskContext, topK int) ([]ToolMatch, error)
	// LearnFromUsage records tool success/failure for future ranking.
	LearnFromUsage(ctx context.Context, toolName string, stage string, success bool) error
}

// TaskContext provides semantic context for tool matching.
type TaskContext struct {
	Stage       string
	Level       string
	FailureCode string
	ProjectID   string
}
```

- [ ] **Step 2: 实现 Tool/Skill 嵌入服务**

Create `internal/agent/service/tool_embedding_service.go`:

```go
package service

import (
	"context"
	"sync"

	"openforge/internal/agent/domain"
	"openforge/internal/agent/port"
)

// ToolEmbeddingService implements ToolEmbeddingSearcher with keyword+rule dual-channel.
type ToolEmbeddingService struct {
	toolSearcher    port.ToolSearcher
	trajStore       domain.TrajectoryStore
	toolSuccessRates map[string]float64 // toolName → success rate
	mu              sync.RWMutex
}

func NewToolEmbeddingService(searcher port.ToolSearcher, trajStore domain.TrajectoryStore) *ToolEmbeddingService {
	return &ToolEmbeddingService{
		toolSearcher:     searcher,
		trajStore:        trajStore,
		toolSuccessRates: make(map[string]float64),
	}
}

func (s *ToolEmbeddingService) Search(ctx context.Context, query string, topK int) ([]port.ToolMatch, error) {
	return s.toolSearcher.Search(ctx, query, topK)
}

func (s *ToolEmbeddingService) SearchTools(ctx context.Context, query string, topK int) ([]port.ToolMatch, error) {
	return s.toolSearcher.SearchTools(ctx, query, topK)
}

func (s *ToolEmbeddingService) Register(ctx context.Context, info port.ToolInfo) error {
	return s.toolSearcher.Register(ctx, info)
}

func (s *ToolEmbeddingService) List(ctx context.Context) ([]port.ToolInfo, error) {
	return s.toolSearcher.List(ctx)
}

// SearchWithContext boosts tools known to succeed in similar contexts.
func (s *ToolEmbeddingService) SearchWithContext(ctx context.Context, query string, taskCtx port.TaskContext, topK int) ([]port.ToolMatch, error) {
	matches, err := s.toolSearcher.Search(ctx, query, topK*2)
	if err != nil {
		return nil, err
	}

	// Boost successful tools for this stage
	successfulTools, _ := s.trajStore.SuccessfulTools(ctx, taskCtx.ProjectID, taskCtx.Stage, 10)
	boostSet := make(map[string]float64, len(successfulTools))
	for i, t := range successfulTools {
		boostSet[t] = 0.2 * float64(len(successfulTools)-i) / float64(len(successfulTools))
	}

	for i := range matches {
		if boost, ok := boostSet[matches[i].Name]; ok {
			matches[i].Score += boost
		}
	}

	// Re-sort and trim
	for i := 0; i < len(matches); i++ {
		for j := i + 1; j < len(matches); j++ {
			if matches[j].Score > matches[i].Score {
				matches[i], matches[j] = matches[j], matches[i]
			}
		}
	}
	if len(matches) > topK {
		matches = matches[:topK]
	}
	return matches, nil
}

// LearnFromUsage records tool outcome for future ranking.
func (s *ToolEmbeddingService) LearnFromUsage(ctx context.Context, toolName string, stage string, success bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	rate, ok := s.toolSuccessRates[toolName]
	if !ok {
		rate = 0.5
	}
	if success {
		s.toolSuccessRates[toolName] = rate*0.9 + 0.1
	} else {
		s.toolSuccessRates[toolName] = rate * 0.9
	}
	return nil
}

// SkillPreferenceLearner records which skills were matched and their outcomes
// to update L2 preferences (skill → tool mapping).
type SkillPreferenceLearner struct {
	prefStore domain.PreferenceStore
}

func NewSkillPreferenceLearner(prefStore domain.PreferenceStore) *SkillPreferenceLearner {
	return &SkillPreferenceLearner{prefStore: prefStore}
}

// RecordSkillUsage records that a skill was matched and whether it helped.
func (l *SkillPreferenceLearner) RecordSkillUsage(ctx context.Context, projectID, skillName, outcome string) error {
	weight := 1.0
	if outcome == "success" {
		weight = 2.0
	} else if outcome == "failure" {
		weight = -1.0
	}
	return l.prefStore.Upsert(ctx, domain.PreferenceRecord{
		ProjectID: projectID,
		Key:       "skill_match",
		Value:     skillName,
		Weight:    weight,
		Source:    "skill_usage",
	})
}
```

- [ ] **Step 3: 编译 + 测试 + Commit**

```bash
go build ./... && go test ./internal/agent/... -count=1
git add internal/agent/port/tool_registry.go internal/agent/service/tool_embedding_service.go
git commit -m "feat(learning): add Tool embedding matching + Skill preference learning

- ToolEmbeddingSearcher interface with context-aware ranking
- SearchWithContext boosts tools successful in similar pipeline stages
- LearnFromUsage updates tool success rates online
- SkillPreferenceLearner records skill→tool mappings as L2 preferences
"
```

---

### Task 9: Wiring — Bootstrap 追加学习组件 + AdminPage 前端

> **Phase 6.5 已就绪**: `SkillLoader`, `CapabilityInjector`, `UnifiedPriorityEngine`, `PromptBuilder` 已在 `bootstrap.go` 中注入。Task 9 仅追加 Phase 7 的新学习组件，不重复注入。

**Files:**
- Modify: `internal/shared/profile/bootstrap.go` — 追加学习字段
- Modify: `frontend/src/features/admin/AdminPage.tsx` — 追加状态卡

- [ ] **Step 1: OpenForge struct 追加学习组件**

在已有 `SkillLoader`, `CapabilityInjector`, `PriorityEngine` 字段下方追加:

```go
	// Phase 7: Learning engine
	PreferenceStore   *agentadapter.PGPreferenceStore
	TrajectoryStore   *agentadapter.PGTrajectoryStore
	SnapshotStore     *agentadapter.PGKnowledgeSnapshotStore
	ExperimentStore   *agentadapter.PGExperimentStore
	LLMPriorityQueue  *agentdomain.LLMPriorityQueue
	RetrospectiveSvc  *agentservice.RetrospectiveService
	LearningEngineSvc *agentservice.LearningEngineService
```

在 `of.PriorityEngine.Start()` 之后追加注入:

```go
	// Phase 7: Learning engine wiring
	of.PreferenceStore = agentadapter.NewPGPreferenceStore(db)
	of.TrajectoryStore = agentadapter.NewPGTrajectoryStore(db)
	of.SnapshotStore = agentadapter.NewPGKnowledgeSnapshotStore(db)
	of.ExperimentStore = agentadapter.NewPGExperimentStore(db)
	of.LLMPriorityQueue = agentdomain.NewLLMPriorityQueue()
	of.LearningEngineSvc = agentservice.NewLearningEngineService(of.PreferenceStore, of.TrajectoryStore, of.SnapshotStore)
	of.RetrospectiveSvc = agentservice.NewRetrospectiveService(of.SnapshotStore, of.TrajectoryStore, of.PreferenceStore, of.ExperimentStore)
	// Wire TrajectoryStore into UnifiedPriorityEngine for LearningFactor
	of.PriorityEngine.SetTrajectoryStore(of.TrajectoryStore)
```

- [ ] **Step 2: 更新 AdminPage 前端**

在 `frontend/src/features/admin/AdminPage.tsx` 追加状态卡:

```tsx
<StatusCard label="Learning Engine" status="on" detail="L1/L2/L3/L4 active" />
<StatusCard label="LLM Priority Queue" status="on" detail="WFQ: P0/P1/P2/P3" />
<StatusCard label="Skill Learning" status="on" detail="Skill→tool preference" />
```

- [ ] **Step 3: 编译 + 前端 + Commit**

```bash
go build ./... && go test ./... -count=1
cd frontend && npx tsc --noEmit
git add internal/shared/profile/bootstrap.go frontend/src/features/admin/AdminPage.tsx
git commit -m "feat(learning): wire Phase 7 learning components into OpenForge

- Append PreferenceStore, TrajectoryStore, SnapshotStore, ExperimentStore
- Inject LLMPriorityQueue, RetrospectiveService, LearningEngineService
- Wire TrajectoryStore into UnifiedPriorityEngine for LearningFactor
- Update AdminPage with learning status indicators
"
```

---

### Task 10: E2E 验证

- [ ] **Step 1: Go 全量测试**

```bash
cd /d/vscode/tiktok/openforge
go test ./... -count=1
```

- [ ] **Step 2: Go vet + build**

```bash
go vet ./...
go build ./cmd/server/ && go build ./cmd/openforge/
```

- [ ] **Step 3: 前端编译**

```bash
cd frontend && npx tsc --noEmit && npx vite build
```

- [ ] **Step 4: Commit**

```bash
git commit --allow-empty -m "chore(phase7): final verification — all tests pass, builds clean

- go build ./cmd/server/ ✓
- go build ./cmd/openforge/ ✓
- go vet ./... ✓
- go test ./... (all packages pass) ✓
- frontend tsc --noEmit ✓
- frontend vite build ✓
"
```

---

## 实施顺序

```
Task 1: DB Migration          (独立)
Task 2: Preference Store      (独立)
Task 3: Trajectory Store      (独立) — 关键：后续 Task 7/8 依赖
Task 4: Knowledge Snapshots   (独立)
Task 5: A/B Testing Engine    (独立)
Task 6: LLM Priority Queue    (独立)
Task 7: Retrospective         (依赖 Task 3)
Task 8: Tool/Skill Embedding  (依赖 Task 3)
Task 9: Wiring + Frontend     (依赖 Task 2-8)
       └─ bootstrap 只追加学习组件（Skill/Capability/Priority 已注入不重复）
       └─ priority_engine.go 接入 TrajectoryStore 激活 LearningFactor
Task 10: E2E Verification     (依赖 Task 1-9)
```

Tasks 1/2/3/4/5/6 互不依赖，可并行。Task 7/8 依赖 TrajectoryStore (Task 3)。

**Phase 6.5 组件状态**: `skill.go`, `skill_loader.go`, `capability_injector.go`, `priority_engine.go` 已就绪，无需重建。Phase 7 的工作量相比原定计划减少约 30%。

## Phase 7 Acceptance Criteria

| # | Criterion | Verification |
|---|-----------|-------------|
| 1 | PreferenceStore 持久化 L2 偏好 + 冲突 resolution (频次>时间>置信度) | automated |
| 2 | TrajectoryStore 记录 pipeline 轨迹含 tools_used + skills_matched | automated |
| 3 | KnowledgeSnapshot Ed25519 签名/验签 | automated |
| 4 | A/B 实验 cohort 分配 + 统计判定 (promoted/invalid/harmful) | automated |
| 5 | LLM 优先级队列 WFQ 调度 (P0 抢先，同优先级 FIFO) | automated |
| 6 | Pipeline Retrospective 自动生成 + CrossPipelineSummary 周报 | automated |
| 7 | ToolEmbeddingSearcher 根据 stage 上下文调整工具排名 | automated |
| 8 | SkillPreferenceLearner 记录 skill→tool 映射为 L2 偏好 | automated |
| 9 | 学习组件追加注入 OpenForge（不重复已有 Skill/Capability/Priority 注入） | automated |
| 10 | 前端 AdminPage 显示完整学习引擎状态 | manual |
| 11 | `go build ./...` + `go vet ./...` 通过 | automated |
| 12 | **Phase 6.5 集成**: UnifiedPriorityEngine.LearningFactor 读取 TrajectoryStore 数据 | automated |
| 13 | **Phase 6.5 集成**: SkillLoader 匹配记录写入 TrajectoryStore.skills_matched | automated |
