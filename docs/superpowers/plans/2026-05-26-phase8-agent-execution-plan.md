# Phase 8 HA/Concurrency Agent Execution Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Complete Phase 8 production-readiness work: circuit breaker wiring, file locking, load shedding, canary rollout services, Redis queue wiring, sandbox dependency cache, SLO/metrics, and standard deployment profile.

**Architecture:** Reuse the implementations that already exist in this repository instead of moving packages unnecessarily. Finish the missing service/wiring layers around existing domain components, add focused tests first, then integrate into `profile.Bootstrap`, HTTP middleware/routes, and frontend status panels. Keep `minimal` profile local/dev friendly while making `standard` profile use Redis + Docker sandbox + Phase 8 observability.

**Tech Stack:** Go 1.25, `database/sql`, `lib/pq`, Docker CLI, React 19 + TypeScript + Vitest, Prometheus text format.

---

## Current Audit Snapshot

Use this snapshot before implementation:

- Existing and reusable:
  - `internal/observability/domain/circuit_breaker.go` + tests
  - `internal/observability/domain/load_shedder.go` + tests
  - `internal/observability/domain/sharding.go` but no tests/integration
  - `internal/pipeline/domain/file_lock.go` + tests
  - `internal/pipeline/adapter/pg_file_lock.go`
  - `internal/policy/domain/canary.go` + tests
  - `internal/adapter/redis_task_queue.go` + tests
  - `internal/adapter/docker_sandbox_executor.go` + tests
  - `internal/observability/adapter/prometheus_exporter.go` + tests
  - `config/profiles/standard.yaml`
- Missing or incomplete:
  - Phase 8 components are not consistently wired into `internal/shared/profile/bootstrap.go`.
  - `newTaskQueue` still returns `noopTaskQueue`, even for `redis-streams`.
  - `PGFileLockStore.Acquire` does not report lock conflict via `RowsAffected`.
  - No `FileLockService` wrapper for acquire/release/timeout/deadlock policies.
  - No load-shedding middleware integration with 429 + `Retry-After`.
  - Hash ring lacks tests and is unused by orchestration.
  - Dependency cache is missing.
  - Prometheus exporter exists but is not exposed through the server profile/routes.
  - Phase 8 admin/status API and frontend panels are mostly static.

---

## Execution Rules for Agents

- Use TDD: write/modify tests first, run them and confirm they fail for the expected reason, then implement.
- Do not rename existing packages unless a task explicitly says so.
- Prefer small commits after each task.
- Do not commit generated binaries (`*.exe`) or screenshots.
- Before each commit run the task-specific tests listed in that task.
- After Task 10 run the full verification matrix.

---

## Task 1: Wire Redis TaskQueue in Bootstrap

**Why first:** This is a small, isolated Phase 8 wiring gap and validates profile-based component selection.

**Files:**
- Modify: `internal/shared/profile/bootstrap.go`
- Test: `internal/shared/profile/bootstrap_test.go` or extend existing `internal/shared/profile/loader_test.go`
- Existing dependency: `internal/adapter/redis_task_queue.go`

**Agent prompt:**

```text
Implement Task 1 from docs/superpowers/plans/2026-05-26-phase8-agent-execution-plan.md. Only work on Redis TaskQueue bootstrap wiring. Use TDD. Do not touch frontend or unrelated Phase 8 modules.
```

- [ ] **Step 1: Write failing test for `redis-streams` profile wiring**

Add a test that creates a `Config{TaskQueue: "redis-streams"}` and asserts `newTaskQueue(cfg)` returns `*adapter.RedisTaskQueue`, while default/minimal returns `noopTaskQueue`.

```go
func TestNewTaskQueue_RedisStreams(t *testing.T) {
	cfg := &Config{TaskQueue: "redis-streams"}
	q := newTaskQueue(cfg)
	if fmt.Sprintf("%T", q) != "*adapter.RedisTaskQueue" {
		t.Fatalf("expected *adapter.RedisTaskQueue, got %T", q)
	}
}

func TestNewTaskQueue_DefaultNoop(t *testing.T) {
	q := newTaskQueue(&Config{})
	if fmt.Sprintf("%T", q) != "*profile.noopTaskQueue" {
		t.Fatalf("expected noopTaskQueue, got %T", q)
	}
}
```

Run:

```bash
go test ./internal/shared/profile -run "TestNewTaskQueue" -count=1
```

Expected before implementation: first test fails because `newTaskQueue` returns noop.

- [ ] **Step 2: Implement minimal wiring**

Modify `newTaskQueue` in `internal/shared/profile/bootstrap.go`:

```go
func newTaskQueue(cfg *Config) kernel.TaskQueue {
	switch cfg.TaskQueue {
	case "redis-streams", "redis-cluster-streams":
		return adapter.NewRedisTaskQueue(cfg.Redis.Host+":"+cfg.Redis.Port, cfg.Redis.Password)
	default:
		return &noopTaskQueue{}
	}
}
```

If `Config.Redis.Port` is not a string field, inspect `loader.go` and construct the address from the actual fields.

- [ ] **Step 3: Verify and commit**

```bash
go test ./internal/shared/profile ./internal/adapter -count=1
git add internal/shared/profile/bootstrap.go internal/shared/profile/*_test.go
git commit -m "feat(phase8): wire redis task queue for standard profile"
```

---

## Task 2: FileLockService + PG Lock Conflict Fix

**Why:** Phase 8 requires file-level concurrency safety before high-concurrency orchestration.

**Files:**
- Modify: `internal/pipeline/adapter/pg_file_lock.go`
- Create: `internal/pipeline/service/file_lock_service.go`
- Create: `internal/pipeline/service/file_lock_service_test.go`
- Optional test: `internal/pipeline/adapter/pg_file_lock_test.go`

**Agent prompt:**

```text
Implement Task 2 from docs/superpowers/plans/2026-05-26-phase8-agent-execution-plan.md. Fix PG file lock conflict detection and add FileLockService. Use TDD. Keep public APIs small and do not integrate into chat tools yet.
```

- [ ] **Step 1: Add service tests**

Create `internal/pipeline/service/file_lock_service_test.go` with a fake store:

```go
type fakeFileLockStore struct {
	locks map[string]*domain.FileLock
	failAcquire bool
}

func (s *fakeFileLockStore) Acquire(ctx context.Context, lock *domain.FileLock) error {
	if s.failAcquire { return errors.New("lock conflict") }
	s.locks[lock.ProjectID+":"+lock.FilePath] = lock
	return nil
}
func (s *fakeFileLockStore) Release(ctx context.Context, projectID, filePath string) error {
	delete(s.locks, projectID+":"+filePath)
	return nil
}
func (s *fakeFileLockStore) ListByProject(ctx context.Context, projectID string) ([]*domain.FileLock, error) {
	var out []*domain.FileLock
	for _, l := range s.locks { if l.ProjectID == projectID { out = append(out, l) } }
	return out, nil
}
func (s *fakeFileLockStore) DetectDeadlock(ctx context.Context, projectID string) ([]domain.GraphCycle, error) {
	return nil, nil
}
```

Required test names:

```go
func TestFileLockService_AcquireWriteLock(t *testing.T) {}
func TestFileLockService_ReleaseLock(t *testing.T) {}
func TestFileLockService_RejectsConflict(t *testing.T) {}
func TestFileLockService_ExpireTimeoutLocks(t *testing.T) {}
```

Run:

```bash
go test ./internal/pipeline/service -run TestFileLockService -count=1
```

Expected before implementation: package/file missing.

- [ ] **Step 2: Implement service**

Create `internal/pipeline/service/file_lock_service.go`:

```go
package service

import (
	"context"
	"time"

	"openforge/internal/pipeline/domain"
)

type FileLockService struct { store domain.FileLockStore }

func NewFileLockService(store domain.FileLockStore) *FileLockService {
	return &FileLockService{store: store}
}

func (s *FileLockService) AcquireWriteLock(ctx context.Context, pipelineID, projectID, filePath string, ttl time.Duration) error {
	return s.store.Acquire(ctx, domain.LockWrite(pipelineID, projectID, filePath, ttl))
}

func (s *FileLockService) AcquireReadOnlyLock(ctx context.Context, pipelineID, projectID, filePath string, ttl time.Duration) error {
	return s.store.Acquire(ctx, domain.LockReadOnly(pipelineID, projectID, filePath, ttl))
}

func (s *FileLockService) ReleaseLock(ctx context.Context, projectID, filePath string) error {
	return s.store.Release(ctx, projectID, filePath)
}

func (s *FileLockService) ExpiredLocks(ctx context.Context, projectID string, now time.Time) ([]*domain.FileLock, error) {
	locks, err := s.store.ListByProject(ctx, projectID)
	if err != nil { return nil, err }
	var expired []*domain.FileLock
	for _, l := range locks { if l.ExpiresAt.Before(now) || l.ExpiresAt.Equal(now) { expired = append(expired, l) } }
	return expired, nil
}
```

- [ ] **Step 3: Fix PG conflict detection**

In `PGFileLockStore.Acquire`, capture `sql.Result`, call `RowsAffected`, and return `domain.ErrFileLockConflict` or a new typed error when rows affected is `0`.

If no domain error exists, add:

```go
var ErrFileLockConflict = errors.New("file lock conflict")
```

- [ ] **Step 4: Verify and commit**

```bash
go test ./internal/pipeline/domain ./internal/pipeline/adapter ./internal/pipeline/service -count=1
git add internal/pipeline/domain internal/pipeline/adapter internal/pipeline/service
git commit -m "feat(phase8): add file lock service and conflict detection"
```

---

## Task 3: HashRing Tests + Bootstrap Field

**Why:** Existing `HashRing` is untested and unused; Phase 8 requires consistent sharding.

**Files:**
- Modify: `internal/observability/domain/sharding.go`
- Create: `internal/observability/domain/sharding_test.go`
- Modify: `internal/shared/profile/bootstrap.go`

**Agent prompt:**

```text
Implement Task 3 from docs/superpowers/plans/2026-05-26-phase8-agent-execution-plan.md. Add HashRing tests, fix duplicate AddNode behavior if needed, and expose it through OpenForge bootstrap. Use existing package path.
```

- [ ] **Step 1: Write tests**

Create tests:

```go
func TestHashRing_GetNodeIsStable(t *testing.T) {}
func TestHashRing_RemoveNodeRemapsOnlyRemovedNodeKeys(t *testing.T) {}
func TestHashRing_AddNodeTwiceDoesNotDuplicateVirtualNodes(t *testing.T) {}
func TestHashRing_EmptyRingReturnsEmptyNode(t *testing.T) {}
```

Run:

```bash
go test ./internal/observability/domain -run TestHashRing -count=1
```

Expected: duplicate-add test likely fails.

- [ ] **Step 2: Fix duplicate add**

At the start of `AddNode`, remove the existing node if present:

```go
func (h *HashRing) AddNode(nodeID string, virtualNodes int) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if _, exists := h.nodes[nodeID]; exists {
		h.removeNodeLocked(nodeID)
	}
	// add virtual nodes...
}
```

Extract a private `removeNodeLocked` to avoid recursive locking.

- [ ] **Step 3: Add bootstrap field**

Add to `OpenForge` struct:

```go
HashRing *observabilitydomain.HashRing
```

Initialize in `Bootstrap`:

```go
of.HashRing = observabilitydomain.NewHashRing()
of.HashRing.AddNode("local", 128)
```

- [ ] **Step 4: Verify and commit**

```bash
go test ./internal/observability/domain ./internal/shared/profile -count=1
git add internal/observability/domain/sharding.go internal/observability/domain/sharding_test.go internal/shared/profile/bootstrap.go
git commit -m "feat(phase8): stabilize hash ring sharding primitive"
```

---

## Task 4: Load-Shedding Middleware Integration

**Why:** Domain logic exists but is not connected to HTTP ingress.

**Files:**
- Modify: `internal/server/middleware.go`
- Modify: `internal/server/routes.go` if middleware composition lives there
- Test: `internal/server/middleware_test.go`

**Agent prompt:**

```text
Implement Task 4 from docs/superpowers/plans/2026-05-26-phase8-agent-execution-plan.md. Add load shedding middleware around HTTP API requests using existing observability/domain LoadShedder. Use TDD and keep defaults permissive for minimal profile.
```

- [ ] **Step 1: Write middleware tests**

Required tests:

```go
func TestLoadShedMiddleware_AllowsNormal(t *testing.T) {}
func TestLoadShedMiddleware_RejectsCriticalNonP0(t *testing.T) {}
func TestLoadShedMiddleware_SetsRetryAfter(t *testing.T) {}
```

Use `httptest.NewRecorder` and a fake snapshot provider returning `ResourceSnapshot`.

- [ ] **Step 2: Implement middleware**

Add a provider type:

```go
type ResourceSnapshotProvider interface {
	Snapshot() observabilitydomain.ResourceSnapshot
}
```

Add middleware:

```go
func LoadShedMiddleware(ls *observabilitydomain.LoadShedder, provider ResourceSnapshotProvider, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		priority := priorityFromRequest(r)
		decision := ls.Shed(provider.Snapshot(), priority)
		if !decision.Accept {
			w.Header().Set("Retry-After", fmt.Sprintf("%d", int(decision.RetryAfter.Seconds())))
			writeError(w, http.StatusTooManyRequests, decision.Reason)
			return
		}
		next.ServeHTTP(w, r)
	})
}
```

Keep `priorityFromRequest` simple: `X-OpenForge-Priority: 0..3`, default `3`.

- [ ] **Step 3: Wire only when provider exists**

In route setup, keep minimal profile safe. Do not break existing auth middleware.

- [ ] **Step 4: Verify and commit**

```bash
go test ./internal/server ./internal/observability/domain -count=1
git add internal/server internal/observability/domain
git commit -m "feat(phase8): enforce load shedding at HTTP ingress"
```

---

## Task 5: Circuit Breaker Runtime Wiring

**Why:** Breaker exists but no production call path uses it.

**Files:**
- Modify: `internal/shared/profile/bootstrap.go`
- Modify: one safe dependency wrapper first, preferably LLM router adapter or command executor wrapper
- Test: add focused test near wrapper package

**Agent prompt:**

```text
Implement Task 5 from docs/superpowers/plans/2026-05-26-phase8-agent-execution-plan.md. Wire existing circuit breaker into OpenForge runtime and protect at least LLM and Docker command execution. Use TDD; do not rewrite breaker internals unless tests require it.
```

- [ ] **Step 1: Add `BreakerPool` to `OpenForge`**

```go
BreakerPool *observabilitydomain.BreakerPool
```

Initialize:

```go
of.BreakerPool = observabilitydomain.NewBreakerPool(observabilitydomain.BreakerConfig{
	MaxFailures: 3,
	OpenDuration: 60 * time.Second,
	HalfOpenMaxReqs: 1,
})
```

- [ ] **Step 2: Add wrapper tests**

Create a small wrapper around `kernel.CommandExecutor`:

```go
type breakerCommandExecutor struct {
	name string
	breaker *observabilitydomain.Breaker
	next kernel.CommandExecutor
}
```

Tests:

```go
func TestBreakerCommandExecutor_DelegatesWhenClosed(t *testing.T) {}
func TestBreakerCommandExecutor_RejectsWhenOpen(t *testing.T) {}
```

- [ ] **Step 3: Implement wrapper and wire Docker/local shell execution through it**

Wrap after executor creation:

```go
if of.BreakerPool != nil {
	of.CommandExec = newBreakerCommandExecutor("command_executor", of.BreakerPool.Get("command_executor"), of.CommandExec)
}
```

- [ ] **Step 4: Expose breaker states in admin status**

Add to `/api/admin/status` response:

```go
"circuit_breakers": of.BreakerPool.All(),
```

- [ ] **Step 5: Verify and commit**

```bash
go test ./internal/shared/profile ./internal/server ./internal/observability/domain -count=1
git add internal/shared/profile internal/server internal/observability/domain
git commit -m "feat(phase8): wire circuit breakers into runtime status"
```

---

## Task 6: DependencyCache + Docker Sandbox Mount Integration

**Why:** Phase 8 requires shared read-only dependency layer to reduce sandbox I/O.

**Files:**
- Create: `internal/adapter/dependency_cache.go`
- Create: `internal/adapter/dependency_cache_test.go`
- Modify: `internal/adapter/docker_sandbox_executor.go`
- Modify: `internal/adapter/docker_sandbox_executor_test.go`
- Modify: `internal/shared/profile/bootstrap.go`

**Agent prompt:**

```text
Implement Task 6 from docs/superpowers/plans/2026-05-26-phase8-agent-execution-plan.md. Add DependencyCache and mount it read-only in DockerSandboxExecutor. Use deterministic path hashing and no network calls in unit tests.
```

- [ ] **Step 1: Write dependency cache tests**

Required tests:

```go
func TestDependencyCache_LayerIsDeterministic(t *testing.T) {}
func TestDependencyCache_WarmCreatesLayerDirectory(t *testing.T) {}
func TestDependencyCache_EvictRemovesOldLayers(t *testing.T) {}
```

- [ ] **Step 2: Implement cache**

Create:

```go
type DependencySpec struct {
	ProjectID string
	Runtime string
	LockfileHash string
}

type DependencyCache struct { rootDir string }

func NewDependencyCache(rootDir string) *DependencyCache { return &DependencyCache{rootDir: rootDir} }
func (dc *DependencyCache) Layer(ctx context.Context, spec DependencySpec) (string, error) {}
func (dc *DependencyCache) Warm(ctx context.Context, spec DependencySpec) error {}
func (dc *DependencyCache) Evict(ctx context.Context, retention time.Duration) error {}
```

Use `sha256` of `runtime + projectID + lockfileHash` for deterministic directory names.

- [ ] **Step 3: Add Docker mount test**

Assert `buildDockerCmd` includes:

```text
-v <cache-layer>:/cache/deps:ro
```

- [ ] **Step 4: Wire in bootstrap for standard profile**

Add to `OpenForge`:

```go
DepCache *adapter.DependencyCache
```

Initialize when profile is not minimal or config says shared cache:

```go
of.DepCache = adapter.NewDependencyCache("/cache/deps")
```

- [ ] **Step 5: Verify and commit**

```bash
go test ./internal/adapter ./internal/shared/profile -count=1
git add internal/adapter internal/shared/profile/bootstrap.go
git commit -m "feat(phase8): add dependency cache for docker sandboxes"
```

---

## Task 7: SLO Tracker + Prometheus Route Wiring

**Why:** Exporter exists, but Phase 8 requires SLO metrics and reachable `/metrics`.

**Files:**
- Create: `internal/observability/domain/slo_tracker.go`
- Create: `internal/observability/domain/slo_tracker_test.go`
- Modify: `internal/observability/adapter/prometheus_exporter.go` if needed
- Modify: `internal/server/routes.go`
- Modify: `internal/shared/profile/bootstrap.go`

**Agent prompt:**

```text
Implement Task 7 from docs/superpowers/plans/2026-05-26-phase8-agent-execution-plan.md. Add SLOTracker and expose Prometheus metrics through server routes. Use existing PrometheusExporter where possible.
```

- [ ] **Step 1: Write SLO tests**

Required tests:

```go
func TestSLOTracker_RecordsPipelineDuration(t *testing.T) {}
func TestSLOTracker_ErrorBudgetCalculation(t *testing.T) {}
func TestSLOTracker_SnapshotIsCopy(t *testing.T) {}
```

- [ ] **Step 2: Implement SLOTracker**

Minimal API:

```go
type SLOTracker struct { /* mutex + counters */ }
func NewSLOTracker() *SLOTracker {}
func (s *SLOTracker) RecordPipeline(duration time.Duration, success bool) {}
func (s *SLOTracker) Snapshot() SLOSnapshot {}
```

- [ ] **Step 3: Add `/metrics` route**

If exporter currently listens on a separate port, also expose it in the main mux:

```go
mux.HandleFunc("GET /metrics", of.PrometheusExporter.HandleHTTP)
```

If no exported handler exists, add:

```go
func (pe *PrometheusExporter) HandleHTTP(w http.ResponseWriter, r *http.Request) { pe.handleMetrics(w, r) }
```

- [ ] **Step 4: Admin status includes SLO summary**

Return `slo` object from `/api/admin/status`.

- [ ] **Step 5: Verify and commit**

```bash
go test ./internal/observability/... ./internal/server ./internal/shared/profile -count=1
git add internal/observability internal/server internal/shared/profile/bootstrap.go
git commit -m "feat(phase8): expose slo and prometheus metrics"
```

---

## Task 8: CanaryService Runtime Wrapper

**Why:** Canary domain exists but no service owns configs or exposes runtime evaluation.

**Files:**
- Create: `internal/pipeline/service/canary_service.go`
- Create: `internal/pipeline/service/canary_service_test.go`
- Modify: `internal/shared/profile/bootstrap.go`
- Modify: `internal/server/routes.go` if adding admin endpoint

**Agent prompt:**

```text
Implement Task 8 from docs/superpowers/plans/2026-05-26-phase8-agent-execution-plan.md. Add CanaryService around existing policy/domain CanaryEngine and expose read-only admin evaluation/status. Use TDD.
```

- [ ] **Step 1: Write service tests**

Required tests:

```go
func TestCanaryService_EvaluateDelegatesToEngine(t *testing.T) {}
func TestCanaryService_UpdateConfigsReplacesEngine(t *testing.T) {}
func TestCanaryService_StatusReturnsConfigs(t *testing.T) {}
```

- [ ] **Step 2: Implement service**

Minimal API:

```go
type CanaryService struct {
	mu sync.RWMutex
	configs []*policydomain.CanaryConfig
	engine *policydomain.CanaryEngine
}

func NewCanaryService(configs ...*policydomain.CanaryConfig) *CanaryService {}
func (s *CanaryService) Evaluate(pipelineID, projectID string, current, baseline float64, sampleSize int) []policydomain.EvaluateResult {}
func (s *CanaryService) Configs() []*policydomain.CanaryConfig {}
func (s *CanaryService) Replace(configs []*policydomain.CanaryConfig) {}
```

- [ ] **Step 3: Wire bootstrap and admin status**

Add `CanarySvc *pipelineservice.CanaryService` to `OpenForge` and initialize empty.

- [ ] **Step 4: Verify and commit**

```bash
go test ./internal/policy/domain ./internal/pipeline/service ./internal/shared/profile ./internal/server -count=1
git add internal/pipeline/service internal/shared/profile/bootstrap.go internal/server/routes.go
git commit -m "feat(phase8): add canary rollout service wiring"
```

---

## Task 9: Standard Deployment Profile and Compose

**Why:** `standard.yaml` exists, but Phase 8 needs reproducible deployment artifacts.

**Files:**
- Modify: `config/profiles/standard.yaml`
- Create: `deployments/docker-compose.standard.yaml`
- Optional: `deployments/README.md`
- Test: `internal/shared/profile/loader_test.go`

**Agent prompt:**

```text
Implement Task 9 from docs/superpowers/plans/2026-05-26-phase8-agent-execution-plan.md. Ensure standard profile loads and add a standard docker compose deployment file. Do not change minimal profile behavior.
```

- [ ] **Step 1: Add/extend profile loader test**

Assert:

```go
if cfg.Profile != "standard" { t.Fatal(...) }
if cfg.TaskQueue != "redis-streams" { t.Fatal(...) }
if cfg.CommandExecutor != "docker-sandbox" { t.Fatal(...) }
```

- [ ] **Step 2: Create compose file**

`deployments/docker-compose.standard.yaml` must include services:

```yaml
services:
  postgres:
  redis:
  server:
  nodejs-io:
  frontend:
```

The `server` service must mount Docker socket only for `standard` sandbox mode:

```yaml
volumes:
  - /var/run/docker.sock:/var/run/docker.sock
  - dependency-cache:/cache/deps
```

- [ ] **Step 3: Validate YAML shape**

```bash
go test ./internal/shared/profile -run TestLoadStandardProfile -count=1
docker compose -f deployments/docker-compose.standard.yaml config
```

If Docker is unavailable, document the skipped compose validation in the commit body.

- [ ] **Step 4: Commit**

```bash
git add config/profiles/standard.yaml deployments/docker-compose.standard.yaml internal/shared/profile/loader_test.go
git commit -m "chore(phase8): add standard deployment profile"
```

---

## Task 10: Frontend Phase 8 Status Panels

**Why:** Admin UI currently marks Phase 8 incomplete and circuit breaker page is not wired to live status.

**Files:**
- Modify: `frontend/src/shared/api.ts`
- Modify: `frontend/src/features/admin/AdminPage.tsx`
- Modify: `frontend/src/features/errors/CircuitBreakerPage.tsx`
- Test: `frontend/src/shared/api.test.ts` or new component tests if existing test setup supports it

**Agent prompt:**

```text
Implement Task 10 from docs/superpowers/plans/2026-05-26-phase8-agent-execution-plan.md. Wire admin and circuit breaker UI to /api/admin/status Phase 8 fields. Keep styling consistent with existing pages.
```

- [ ] **Step 1: Extend API types**

Add type:

```ts
export type AdminStatus = {
  phase: string;
  profile: string;
  circuit_breakers?: Record<string, string>;
  slo?: { total: number; success_rate: number; p95_ms?: number };
  ha?: { task_queue: string; hash_ring_nodes: number; load_shedding: string };
};
```

- [ ] **Step 2: Update admin page**

Render Phase 8 row as active/ready based on fields returned by `/api/admin/status`.

- [ ] **Step 3: Update circuit breaker page**

Fetch `api.getAdminStatus()` and render `circuit_breakers` map with states: `CLOSED`, `OPEN`, `HALF_OPEN`.

- [ ] **Step 4: Verify and commit**

```bash
cd frontend
npm test -- --run
npm run typecheck
npm run build
cd ..
git add frontend/src/shared/api.ts frontend/src/features/admin/AdminPage.tsx frontend/src/features/errors/CircuitBreakerPage.tsx
git commit -m "feat(frontend): show phase8 ha and circuit status"
```

---

## Task 11: Final Wiring and Full Verification

**Why:** Phase 8 spans multiple systems; final verification prevents partial green status.

**Files:**
- Modify as needed: `internal/shared/profile/bootstrap.go`, `internal/server/routes.go`, `docs/superpowers/plans/2026-05-24-phase-8-ha-concurrency.md`
- Optional: create `docs/superpowers/phase8-verification-report.md`

**Agent prompt:**

```text
Implement Task 11 from docs/superpowers/plans/2026-05-26-phase8-agent-execution-plan.md. Do not add new features. Only finalize wiring, update docs/status, and run verification. Report exact failures if any.
```

- [ ] **Step 1: Run backend focused tests**

```bash
go test ./internal/observability/... ./internal/pipeline/... ./internal/policy/... ./internal/adapter ./internal/shared/profile ./internal/server -count=1
```

Expected: PASS.

- [ ] **Step 2: Run full Go test suite**

```bash
go test ./... -count=1
```

Expected: PASS. If unrelated pre-existing failures appear, document exact package and error; do not hide failures.

- [ ] **Step 3: Run Go build and vet**

```bash
go build ./cmd/server ./cmd/openforge
go vet ./...
```

Expected: PASS.

- [ ] **Step 4: Run frontend checks**

```bash
cd frontend
npm test -- --run
npm run typecheck
npm run build
cd ..
```

Expected: PASS.

- [ ] **Step 5: Run nodejs-io checks**

```bash
cd nodejs-io
npm test -- --run
npm run typecheck
npm run build
cd ..
```

Expected: PASS.

- [ ] **Step 6: Update Phase 8 plan status**

In `docs/superpowers/plans/2026-05-24-phase-8-ha-concurrency.md`, add a short status block near the top:

```markdown
> Implementation status updated 2026-05-26: Phase 8 runtime wiring completed. See `docs/superpowers/plans/2026-05-26-phase8-agent-execution-plan.md` for execution trace and verification commands.
```

- [ ] **Step 7: Commit final docs/status**

```bash
git add docs/superpowers/plans/2026-05-24-phase-8-ha-concurrency.md docs/superpowers/plans/2026-05-26-phase8-agent-execution-plan.md
git commit -m "docs(phase8): record ha concurrency execution plan"
```

---

## Recommended Subagent Dispatch Order

Dispatch one fresh subagent per task, review diff after each task:

1. Task 1 — Redis TaskQueue wiring
2. Task 2 — FileLockService
3. Task 3 — HashRing tests/bootstrap
4. Task 4 — Load shedding middleware
5. Task 5 — Circuit breaker runtime wiring
6. Task 6 — DependencyCache + Docker sandbox mount
7. Task 7 — SLO + Prometheus route
8. Task 8 — CanaryService
9. Task 9 — Standard deployment profile
10. Task 10 — Frontend status panels
11. Task 11 — Full verification and docs

Parallelism guidance:

- Safe to run in parallel after Task 1: Task 2, Task 3, Task 8, Task 9.
- Do not run Task 4 and Task 7 in parallel with Task 5 if all modify `routes.go`/`bootstrap.go`; sequence them to avoid merge conflicts.
- Run Task 10 after Task 5 and Task 7 because it depends on `/api/admin/status` fields.
- Always run Task 11 last.

---

## Phase 8 Acceptance Checklist

- [x] Redis queue selected for `standard` profile.
- [x] File lock conflicts return explicit errors.
- [x] FileLockService supports acquire/release/expired lock listing.
- [x] FileLockService wired into OpenForge bootstrap.
- [x] HashRing has deterministic tests and no duplicate virtual-node accumulation.
- [x] Load shedding middleware can return 429 + `Retry-After`.
- [x] LoadShedMiddleware activated in routes.go middleware chain.
- [x] BreakerPool is available from `OpenForge` and protects at least command execution.
- [x] DependencyCache creates deterministic cache layers and Docker mounts them read-only.
- [x] Docker sandbox executor properly wired (falls back to local shell when Docker unavailable).
- [x] `/metrics` endpoint returns Prometheus text.
- [x] `/api/admin/status` includes Phase 8 HA fields.
- [x] Frontend displays circuit breaker and HA/SLO status from live API.
- [x] `go test ./... -count=1` passes or documented pre-existing failures are listed.
- [x] SLOTracker constructor typo fixed (removed LOTracker alias workaround).
- [x] Profile loader tests extended with command_executor field for standard profile.
