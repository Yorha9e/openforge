# Current Status And Next Stage Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace stale Phase 1 guidance with a current, verified status baseline and a short execution path for stabilizing OpenForge before the next feature push.

**Architecture:** Treat the repository as a partially implemented multi-phase product, not a Phase 1 greenfield codebase. First preserve and verify the current working-tree changes, then update project guidance, then run a scoped Phase matrix audit, then close the highest-risk Phase 2/3 safety gaps.

**Tech Stack:** Go 1.22+, PostgreSQL migrations, React 19 + TypeScript + Vite, Node.js 20 + Vitest, PowerShell on Windows.

---

## Current Fact Baseline

Generated on 2026-06-03 from local repository state in `D:\vscode\tiktok\openforge`.

### Verified Checks

The following checks were run before this plan was written:

```powershell
$env:GOCACHE='D:\vscode\tiktok\openforge\.cache\go-build'; New-Item -ItemType Directory -Force -Path $env:GOCACHE | Out-Null; go test ./internal/...
cd frontend; npm run typecheck
cd frontend; npm test
cd nodejs-io; npm test
```

Observed result:

- Go internal tests passed.
- Frontend TypeScript check passed.
- Frontend Vitest tests passed: 2 files, 4 tests.
- Node.js Vitest tests passed: 2 files, 4 tests.

### Repository Status

`AGENTS.md` still says the project is "设计完成，待进入 Phase 1 编码" and instructs agents to create a Phase 1 implementation plan. That is stale.

`phase1-summary.md` says Phase 1 MVP is complete and records the original Phase 1 validation.

Recent git history shows work has continued into authentication, registration, invitation, RBAC, frontend, deployment, feature flags, and infrastructure:

```text
8327c24 feat: complete invitation UI, RBAC middleware, and auth refactor
3a15533 feat: add registration and invitation API endpoints (Task 6)
1e3d0fd chore: add .worktrees to gitignore
79381c4 fix: improve type safety and anthropic tool schema
67fbefc fix: type narrowing in ProModeContext, anthropic input_schema, peer deps
d359a19 feat: build frontend inside Dockerfile, no manual npm build needed
19ccb7b fix: skip feature flags seed when table does not exist yet
df652b2 feat: auto-run migrations on container startup via entrypoint script
```

Current tracked modifications, before this file was added:

```text
frontend/src/electron.d.ts
frontend/src/features/admin/AdminPage.tsx
frontend/src/features/chat/useWebSocket.ts
frontend/src/shared/api.ts
frontend/src/shared/auth.tsx
frontend/tsconfig.tsbuildinfo
internal/agent/domain/query_engine_buffer_test.go
internal/auth/adapter/pg_auth_repository_test.go
internal/pipeline/adapter/pg_repository.go
internal/pipeline/adapter/pg_repository_batch.go
internal/server/routes.go
internal/server/routes_test.go
internal/server/ws_handler.go
```

Current untracked items, before this file was added:

```text
AGENTS.md
migrations/010_gate_stage_width.down.sql
migrations/010_gate_stage_width.up.sql
nodejs-io/.claude/
projects/
```

### Phase Reading

The actual codebase is no longer at "start Phase 1".

| Area | Observed state | Next interpretation |
|------|----------------|---------------------|
| Phase 1 CLI/minimal/profile/proto/base domains | Implemented and previously audited in `phase1-summary.md` | Treat as complete; do not generate another Phase 1 plan |
| Phase 2 Web chat/Auth | Implemented beyond a minimal chat, including login/register/invite routes and WebSocket chat | Stabilize security and auth contracts before more surface area |
| Phase 3 Pipeline/Gate/Diff/Review | Present in backend and frontend routes/panels | Audit behavior against design before adding new panels |
| Phase 4 Sandbox/Deploy/Cost | Cost dashboard and sandbox/deploy services exist; runtime maturity unclear | Verify end-to-end before claiming full Phase 4 |
| Phase 5+ enterprise/learning/HA/admin | Many pieces exist earlier than the original roadmap | Classify as partial/experimental until verified |

---

## File Structure

This plan intentionally keeps implementation changes small and auditable.

- Create: `docs/superpowers/plans/2026-06-03-current-status-next-stage.md`
  - Records the current baseline and the execution plan.
- Modify: `AGENTS.md`
  - Replace stale Phase 1 instruction with current status and next-step policy.
- Modify: `phase1-summary.md`
  - Add a short supersession note pointing readers to the current status plan.
- Create: `docs/superpowers/specs/2026-06-03-phase-status-audit.md`
  - A source-of-truth matrix mapping design phases to actual code, tests, and gaps.
- Create: `docs/security/2026-06-03-phase2-security-gate.md`
  - A security gate document for JWT, CSP headers, XSS sanitization, WebSocket auth, and Node.js IO exposure.
- Modify: `internal/server/middleware.go`
  - Add or verify response security headers in one place.
- Test: `internal/server/middleware_test.go`
  - Assert security headers on authenticated and public routes.
- Modify: `internal/server/ws_handler.go`
  - Confirm WebSocket auth behavior is token-only, rejects missing/invalid tokens, and does not accept auth through unsafe URL patterns unless explicitly documented.
- Test: `internal/server/ws_handler_test.go`
  - Cover rejected WebSocket auth paths and accepted bearer-token path.
- Modify: `frontend/src/shared/sanitize.ts`
  - Confirm all Markdown/HTML-rendering paths share one sanitizer.
- Test: `frontend/src/shared/api.test.ts`, `frontend/src/App.test.tsx`
  - Keep existing smoke tests passing after route/doc changes.

---

## Tasks

### Task 1: Preserve Current Verified Worktree State

**Files:**
- Inspect: `frontend/src/electron.d.ts`
- Inspect: `frontend/src/features/admin/AdminPage.tsx`
- Inspect: `frontend/src/features/chat/useWebSocket.ts`
- Inspect: `frontend/src/shared/api.ts`
- Inspect: `frontend/src/shared/auth.tsx`
- Inspect: `internal/agent/domain/query_engine_buffer_test.go`
- Inspect: `internal/auth/adapter/pg_auth_repository_test.go`
- Inspect: `internal/pipeline/adapter/pg_repository.go`
- Inspect: `internal/pipeline/adapter/pg_repository_batch.go`
- Inspect: `internal/server/routes.go`
- Inspect: `internal/server/routes_test.go`
- Inspect: `internal/server/ws_handler.go`
- Inspect: `migrations/010_gate_stage_width.up.sql`
- Inspect: `migrations/010_gate_stage_width.down.sql`

- [ ] **Step 1: Capture exact working-tree status**

Run:

```powershell
git status --short --branch
git diff --stat
```

Expected:

```text
Tracked changes include frontend API/auth/WebSocket files, server routes/WebSocket files, pipeline repository files, and query/auth tests.
Untracked changes include AGENTS.md, migration 010 files, nodejs-io/.claude/, and projects/.
```

- [ ] **Step 2: Review tracked diffs by concern**

Run:

```powershell
git diff -- frontend/src/shared/api.ts frontend/src/features/chat/useWebSocket.ts frontend/src/shared/auth.tsx
git diff -- internal/server/routes.go internal/server/ws_handler.go internal/server/routes_test.go
git diff -- internal/pipeline/adapter/pg_repository.go internal/pipeline/adapter/pg_repository_batch.go
git diff -- internal/agent/domain/query_engine_buffer_test.go internal/auth/adapter/pg_auth_repository_test.go
```

Expected:

```text
Diffs group into Electron async base URL/WebSocket URL, invitation role validation, SLO recording, token cost SQL, and test expectation updates.
```

- [ ] **Step 3: Run verification before staging anything**

Run:

```powershell
$env:GOCACHE='D:\vscode\tiktok\openforge\.cache\go-build'
New-Item -ItemType Directory -Force -Path $env:GOCACHE | Out-Null
go test ./internal/...
cd frontend
npm run typecheck
npm test
cd ..\nodejs-io
npm test
cd ..
```

Expected:

```text
All commands exit 0.
```

- [ ] **Step 4: Remove local Go cache created by verification**

Run:

```powershell
$target = Resolve-Path -LiteralPath '.cache'
$root = Resolve-Path -LiteralPath '.'
if ($target.Path.StartsWith($root.Path + [IO.Path]::DirectorySeparatorChar)) {
  Remove-Item -LiteralPath $target.Path -Recurse -Force
} else {
  throw "refusing to remove outside workspace: $($target.Path)"
}
```

Expected:

```text
.cache no longer appears in git status.
```

- [ ] **Step 5: Decide staging groups**

Use these groups if the diffs review cleanly:

```text
Group A: Electron API and WebSocket URL async changes
  frontend/src/electron.d.ts
  frontend/src/features/chat/useWebSocket.ts
  frontend/src/shared/api.ts
  frontend/src/shared/auth.tsx

Group B: Invitation, route, and WebSocket backend hardening
  internal/server/routes.go
  internal/server/routes_test.go
  internal/server/ws_handler.go
  internal/auth/adapter/pg_auth_repository_test.go

Group C: Cost and pipeline repository fixes
  internal/pipeline/adapter/pg_repository.go
  internal/pipeline/adapter/pg_repository_batch.go

Group D: Test robustness
  internal/agent/domain/query_engine_buffer_test.go

Group E: Migration 010
  migrations/010_gate_stage_width.up.sql
  migrations/010_gate_stage_width.down.sql
```

- [ ] **Step 6: Commit only reviewed groups**

Run one commit per coherent group:

```powershell
git add frontend/src/electron.d.ts frontend/src/features/chat/useWebSocket.ts frontend/src/shared/api.ts frontend/src/shared/auth.tsx
git commit -m "fix: support electron api and websocket base urls"

git add internal/server/routes.go internal/server/routes_test.go internal/server/ws_handler.go internal/auth/adapter/pg_auth_repository_test.go
git commit -m "fix: harden invitation routes and websocket metrics"

git add internal/pipeline/adapter/pg_repository.go internal/pipeline/adapter/pg_repository_batch.go
git commit -m "fix: align token cost queries with schema"

git add internal/agent/domain/query_engine_buffer_test.go
git commit -m "test: stabilize query engine buffer assertions"

git add migrations/010_gate_stage_width.up.sql migrations/010_gate_stage_width.down.sql
git commit -m "fix: widen gate stage migration fields"
```

Expected:

```text
Each commit succeeds only after its files have been reviewed. Do not stage nodejs-io/.claude/ or projects/ unless the user explicitly confirms they are intended source artifacts.
```

### Task 2: Update Stale Agent Guidance

**Files:**
- Modify: `AGENTS.md`
- Modify: `phase1-summary.md`

- [ ] **Step 1: Read the current guidance**

Run:

```powershell
Get-Content -Encoding UTF8 -LiteralPath AGENTS.md
Get-Content -Encoding UTF8 -TotalCount 40 -LiteralPath phase1-summary.md
```

Expected:

```text
AGENTS.md says Phase 1 is not started.
phase1-summary.md says Phase 1 is complete.
```

- [ ] **Step 2: Edit `AGENTS.md` status block**

Replace:

```markdown
> 状态: 设计完成，待进入 Phase 1 编码
```

With:

```markdown
> 状态: Phase 1 已完成；当前重点是工作区收尾、Phase 状态审计、Phase 2/3 安全与稳定性闭环
```

Replace the final "下一步" section with:

```markdown
## 下一步

当前 `phase1-summary.md` 已记录 Phase 1 完成。新会话不要再生成 Phase 1 实现计划。

下一步按优先级执行：

1. Review 并提交当前已验证的工作区补丁。
2. 维护 `docs/superpowers/specs/2026-06-03-phase-status-audit.md`，把 DESIGN/API/proto/代码实际状态对齐。
3. 优先关闭 Phase 2/3 安全门禁：JWT/CSP/XSS/WebSocket 鉴权/Node.js IO 暴露面。
4. 再进入新的功能实现计划。
```

- [ ] **Step 3: Add supersession note to `phase1-summary.md`**

Add this block after the date/status line:

```markdown
> 后续状态更新: 2026-06-03 起，当前项目状态以 `docs/superpowers/plans/2026-06-03-current-status-next-stage.md` 和 `docs/superpowers/specs/2026-06-03-phase-status-audit.md` 为准。Phase 1 不再是下一步入口。
```

- [ ] **Step 4: Verify documentation diff**

Run:

```powershell
git diff -- AGENTS.md phase1-summary.md
```

Expected:

```text
Only status and next-step guidance changed. No design details are removed.
```

- [ ] **Step 5: Commit guidance update**

Run:

```powershell
git add AGENTS.md phase1-summary.md docs/superpowers/plans/2026-06-03-current-status-next-stage.md
git commit -m "docs: update current project status and next steps"
```

Expected:

```text
Commit succeeds after the new plan and stale-guidance edits are reviewed.
```

### Task 3: Create The Phase Status Audit Matrix

**Files:**
- Create: `docs/superpowers/specs/2026-06-03-phase-status-audit.md`

- [ ] **Step 1: Create the audit document skeleton**

Create `docs/superpowers/specs/2026-06-03-phase-status-audit.md` with this content:

```markdown
# OpenForge Phase Status Audit

> Date: 2026-06-03
> Source docs: `DESIGN.md`, `api-contract.yaml`, `proto/agent/v1/`, `phase1-summary.md`, current repository tree.

## Status Legend

| Status | Meaning |
|--------|---------|
| Complete | Code, tests, and docs agree enough to treat as delivered |
| Partial | Code exists but behavior, integration, docs, or tests are incomplete |
| Experimental | Code exists ahead of roadmap but should not be treated as stable |
| Missing | Required surface not found locally |
| Blocked | Cannot verify without secret, service, or user decision |

## Phase Matrix

| Phase | Design promise | Evidence in repo | Status | Next action |
|-------|----------------|------------------|--------|-------------|
| Phase 1 | CLI + minimal profile + capability stubs + LLM conversation | `phase1-summary.md`, `cmd/openforge`, `internal/shared/profile`, `internal/agent`, `nodejs-io`, tests | Complete | Keep as historical baseline |
| Phase 2 | Web chat + BFF Auth + basic WebSocket + frontend safety | `frontend/src/features/chat`, `frontend/src/features/login`, `frontend/src/features/register`, `internal/server/routes.go`, `internal/server/ws_handler.go` | Partial | Run security gate and WebSocket auth audit |
| Phase 3 | Pipeline state machine + diff + approval + review inbox | `internal/pipeline`, `frontend/src/features/code-review`, `frontend/src/features/review-inbox`, gate routes | Partial | Verify gate, diff, branch, and review workflows end-to-end |
| Phase 4 | Docker sandbox + deploy + validation + cost dashboard | `internal/adapter/docker_*`, `internal/pipeline/service/deploy_service.go`, `frontend/src/features/cost-dashboard` | Partial | Prove sandbox/deploy/cost loop with local environment |
| Phase 5+ | Multi-agent, learning, enterprise, HA, admin | `internal/agent/application`, `internal/observability`, `frontend/src/features/admin`, `migrations/004_learning_tables.*` | Experimental | Mark feature flags and avoid promising stability |

## Immediate Gaps

| Gap | Area | Evidence | Owner task |
|-----|------|----------|------------|
| Stale Phase 1 guidance | Docs | `AGENTS.md` conflicts with `phase1-summary.md` | Task 2 in current-stage plan |
| Security gate not re-audited after frontend expansion | Backend/frontend | Routes and pages exist beyond original Phase 2 | Task 4 in current-stage plan |
| Current worktree not committed | Process | `git status --short` shows tracked and untracked changes | Task 1 in current-stage plan |
| Phase 4 loop not proven | Product | Sandbox/deploy/cost code exists, but no current E2E proof recorded | Future Phase 4 verification plan |

## Verification Commands

```powershell
$env:GOCACHE='D:\vscode\tiktok\openforge\.cache\go-build'
New-Item -ItemType Directory -Force -Path $env:GOCACHE | Out-Null
go test ./internal/...
cd frontend
npm run typecheck
npm test
cd ..\nodejs-io
npm test
cd ..
```
```

- [ ] **Step 2: Check that all referenced files exist**

Run:

```powershell
Test-Path DESIGN.md
Test-Path api-contract.yaml
Test-Path proto/agent/v1/coordinator.proto
Test-Path phase1-summary.md
Test-Path frontend/src/features/chat/ChatPanel.tsx
Test-Path internal/server/ws_handler.go
Test-Path internal/pipeline/service/deploy_service.go
Test-Path frontend/src/features/cost-dashboard/CostDashboardPage.tsx
```

Expected:

```text
Every command prints True.
```

- [ ] **Step 3: Commit the audit matrix**

Run:

```powershell
git add docs/superpowers/specs/2026-06-03-phase-status-audit.md
git commit -m "docs: add current phase status audit"
```

Expected:

```text
Commit succeeds after the file is reviewed.
```

### Task 4: Build The Phase 2/3 Security Gate

**Files:**
- Create: `docs/security/2026-06-03-phase2-security-gate.md`
- Modify: `internal/server/middleware.go`
- Test: `internal/server/middleware_test.go`
- Modify: `internal/server/ws_handler.go`
- Test: `internal/server/ws_handler_test.go`
- Modify: `frontend/src/shared/sanitize.ts`

- [ ] **Step 1: Write the security gate document**

Create `docs/security/2026-06-03-phase2-security-gate.md`:

```markdown
# Phase 2/3 Security Gate

> Date: 2026-06-03

## Gate Rule

OpenForge should not expand the browser workbench surface until these controls are verified in code and tests.

## Required Controls

| Control | Required behavior | Verification |
|---------|-------------------|--------------|
| JWT location | Browser API calls use `Authorization: Bearer`, not URL query tokens | `frontend/src/shared/api.ts`, route tests |
| WebSocket auth | `/ws/chat` rejects missing or invalid tokens before upgrade | `internal/server/ws_handler_test.go` |
| CSP header | HTTP responses include a restrictive Content-Security-Policy | `internal/server/middleware_test.go` |
| XSS sanitizer | Markdown/HTML rendering uses shared sanitizer | `frontend/src/shared/sanitize.ts` and component review |
| Node.js IO exposure | Node service is not treated as public browser API | deployment and server config review |

## Acceptance Commands

```powershell
$env:GOCACHE='D:\vscode\tiktok\openforge\.cache\go-build'
New-Item -ItemType Directory -Force -Path $env:GOCACHE | Out-Null
go test ./internal/server -run "TestSecurityHeaders|TestChatWSAuth"
cd frontend
npm run typecheck
npm test
```
```

- [ ] **Step 2: Write failing middleware tests for security headers**

Add to `internal/server/middleware_test.go`:

```go
func TestSecurityHeadersMiddleware(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	rec := httptest.NewRecorder()

	SecurityHeaders(next).ServeHTTP(rec, req)

	if got := rec.Header().Get("Content-Security-Policy"); got == "" {
		t.Fatal("Content-Security-Policy header is required")
	}
	if got := rec.Header().Get("X-Content-Type-Options"); got != "nosniff" {
		t.Fatalf("X-Content-Type-Options = %q, want nosniff", got)
	}
	if got := rec.Header().Get("Referrer-Policy"); got == "" {
		t.Fatal("Referrer-Policy header is required")
	}
}
```

Run:

```powershell
$env:GOCACHE='D:\vscode\tiktok\openforge\.cache\go-build'
New-Item -ItemType Directory -Force -Path $env:GOCACHE | Out-Null
go test ./internal/server -run TestSecurityHeadersMiddleware
```

Expected before implementation:

```text
FAIL because SecurityHeaders is not defined or required headers are missing.
```

- [ ] **Step 3: Implement `SecurityHeaders`**

Add to `internal/server/middleware.go`:

```go
func SecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; connect-src 'self' ws: wss:; object-src 'none'; base-uri 'self'; frame-ancestors 'none'")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		w.Header().Set("X-Frame-Options", "DENY")
		next.ServeHTTP(w, r)
	})
}
```

Then wrap the registered mux in the server composition point. If the composition is in `cmd/server/main.go`, apply:

```go
handler := server.SecurityHeaders(mux)
```

Use the local variable names already present in `cmd/server/main.go`; do not create a second HTTP server.

- [ ] **Step 4: Verify security headers**

Run:

```powershell
$env:GOCACHE='D:\vscode\tiktok\openforge\.cache\go-build'
New-Item -ItemType Directory -Force -Path $env:GOCACHE | Out-Null
go test ./internal/server -run TestSecurityHeadersMiddleware
```

Expected:

```text
PASS
```

- [ ] **Step 5: Add WebSocket auth tests**

In `internal/server/ws_handler_test.go`, add tests covering:

```go
func TestChatWSRejectsMissingToken(t *testing.T) {
	// Build a request to /ws/chat without Authorization and without an allowed token source.
	// Expected: handler returns an HTTP error before upgrading.
}

func TestChatWSRejectsInvalidToken(t *testing.T) {
	// Build a request to /ws/chat with an invalid bearer token.
	// Expected: handler returns an HTTP error before upgrading.
}
```

Use the existing JWT helper and test server style already present in `internal/server/ws_handler_test.go`. The implementation must assert the response status is not `101 Switching Protocols`.

- [ ] **Step 6: Verify WebSocket auth tests**

Run:

```powershell
$env:GOCACHE='D:\vscode\tiktok\openforge\.cache\go-build'
New-Item -ItemType Directory -Force -Path $env:GOCACHE | Out-Null
go test ./internal/server -run TestChatWS
```

Expected:

```text
PASS
```

- [ ] **Step 7: Confirm frontend sanitizer usage**

Run:

```powershell
rg -n "dangerouslySetInnerHTML|marked|DOMPurify|sanitize" frontend/src
```

Expected:

```text
Every Markdown/HTML rendering path either calls frontend/src/shared/sanitize.ts or is a false positive that does not render user content.
```

If a component renders untrusted HTML directly, change it to call the shared sanitizer:

```tsx
import { sanitizeHtml } from '../../shared/sanitize';

<div dangerouslySetInnerHTML={{ __html: sanitizeHtml(renderedMarkdown) }} />
```

- [ ] **Step 8: Run full security-gate verification**

Run:

```powershell
$env:GOCACHE='D:\vscode\tiktok\openforge\.cache\go-build'
New-Item -ItemType Directory -Force -Path $env:GOCACHE | Out-Null
go test ./internal/server
go test ./internal/...
cd frontend
npm run typecheck
npm test
cd ..
```

Expected:

```text
All commands exit 0.
```

- [ ] **Step 9: Commit the security gate**

Run:

```powershell
git add docs/security/2026-06-03-phase2-security-gate.md internal/server/middleware.go internal/server/middleware_test.go internal/server/ws_handler.go internal/server/ws_handler_test.go frontend/src/shared/sanitize.ts
git commit -m "fix: add phase 2 browser security gate"
```

Expected:

```text
Commit succeeds only if tests pass and sanitizer review is complete.
```

### Task 5: Create The Workbench Stabilization Plan

**Files:**
- Read: `docs/superpowers/specs/2026-06-03-phase-status-audit.md`
- Read: `docs/security/2026-06-03-phase2-security-gate.md`
- Read: `DESIGN.md`
- Read: `api-contract.yaml`
- Create: `docs/superpowers/plans/2026-06-03-workbench-stabilization.md`

- [ ] **Step 1: Create the next plan file**

Create `docs/superpowers/plans/2026-06-03-workbench-stabilization.md` with this header:

```markdown
# Workbench Stabilization Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Align the existing browser workbench, backend routes, and API contracts so OpenForge has one verified path from login to project chat to pipeline review.

**Architecture:** Keep changes vertical by workflow: backend route/service/repository, frontend API/component, and tests move together.

**Tech Stack:** Go, PostgreSQL, React, TypeScript, Vitest, PowerShell.

---
```

- [ ] **Step 2: Add the first three stabilization tasks**

Add these task headings to `docs/superpowers/plans/2026-06-03-workbench-stabilization.md`:

```markdown
### Task 1: Auth And Invitation Contract Audit

**Files:**
- Read: `api-contract.yaml`
- Modify: `internal/server/routes.go`
- Modify: `frontend/src/shared/api.ts`
- Test: `internal/server/routes_test.go`
- Test: `frontend/src/shared/api.test.ts`

### Task 2: Chat And WebSocket Contract Audit

**Files:**
- Read: `api-contract.yaml`
- Modify: `internal/server/ws_handler.go`
- Modify: `frontend/src/features/chat/useWebSocket.ts`
- Test: `internal/server/ws_handler_test.go`
- Test: `frontend/src/App.test.tsx`

### Task 3: Pipeline Review Contract Audit

**Files:**
- Read: `api-contract.yaml`
- Modify: `internal/server/routes.go`
- Modify: `frontend/src/features/code-review/ProModePage.tsx`
- Test: `internal/server/routes_test.go`
- Test: `frontend/src/App.test.tsx`
```

- [ ] **Step 3: Include acceptance verification**

Every next feature plan must end with:

```powershell
$env:GOCACHE='D:\vscode\tiktok\openforge\.cache\go-build'
New-Item -ItemType Directory -Force -Path $env:GOCACHE | Out-Null
go test ./internal/...
cd frontend
npm run typecheck
npm test
cd ..\nodejs-io
npm test
cd ..
```

- [ ] **Step 4: Commit the selected next plan**

Run:

```powershell
git add docs/superpowers/plans/2026-06-03-workbench-stabilization.md
git commit -m "docs: plan next verified product increment"
```

Expected:

```text
Commit succeeds after the workbench stabilization plan references the audit and security gate.
```

---

## Recommended Execution Order

1. Task 1: Preserve and commit the already verified worktree changes.
2. Task 2: Fix stale project guidance so future agents stop restarting Phase 1.
3. Task 3: Create the Phase status audit matrix.
4. Task 4: Close the browser security gate.
5. Task 5: Create the workbench stabilization plan from the audit evidence.

Do not start a new feature before Tasks 1-4 are complete. The project has enough surface area now that stale docs and unverified security assumptions are more dangerous than missing another panel.

## Self-Review

### Spec Coverage

| Requirement | Covered by |
|-------------|------------|
| Create a new current plan | This file |
| Reflect actual project state, not stale Phase 1 state | Current Fact Baseline, Phase Reading |
| Provide next-step execution order | Recommended Execution Order |
| Keep future work testable | Each task includes commands and expected results |
| Avoid accidental staging of unrelated user files | Task 1 staging groups |

### Placeholder Scan

The plan uses concrete file paths, commands, and expected results. Future feature work is routed into a named workbench stabilization plan after the audit.

### Type And Command Consistency

All commands use PowerShell paths and the repository root `D:\vscode\tiktok\openforge`. Go cache usage is explicit and followed by a cleanup step. File paths match the current repository layout observed on 2026-06-03.

## Execution Handoff

Plan complete and saved to `docs/superpowers/plans/2026-06-03-current-status-next-stage.md`. Two execution options:

**1. Subagent-Driven (recommended)** - Dispatch a fresh subagent per task, review between tasks, fast iteration.

**2. Inline Execution** - Execute tasks in this session using `superpowers:executing-plans`, batch execution with checkpoints.
