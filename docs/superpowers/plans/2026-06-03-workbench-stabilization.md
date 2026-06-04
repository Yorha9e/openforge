# Workbench Stabilization Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Align the existing browser workbench, backend routes, and API contracts so OpenForge has one verified path from login to project chat to pipeline review.

**Architecture:** Keep changes vertical by workflow: backend route/service/repository, frontend API/component, and tests move together. Treat `docs/superpowers/specs/2026-06-03-phase-status-audit.md` and `docs/security/2026-06-03-phase2-security-gate.md` as the immediate source of truth, then reconcile `DESIGN.md` and `api-contract.yaml` where they drift from verified code.

**Tech Stack:** Go, PostgreSQL, React, TypeScript, Vitest, PowerShell.

---

## Context

The current status audit classifies Phase 2 and Phase 3 as partial, not missing. The security gate now requires `/ws/chat` to reject missing or invalid tokens before upgrade while avoiding URL query JWTs; the browser client does this through WebSocket subprotocols (`openforge.auth`, `bearer.<jwt>`).

`DESIGN.md` still states that WebSocket auth happens through a first-frame message. That is now a documented drift to resolve in this stabilization increment, not an instruction to revert the security gate.

## Tasks

### Task 1: Auth And Invitation Contract Audit

**Files:**
- Read: `api-contract.yaml`
- Read: `docs/security/2026-06-03-phase2-security-gate.md`
- Modify: `internal/server/routes.go`
- Modify: `frontend/src/shared/api.ts`
- Test: `internal/server/routes_test.go`
- Test: `frontend/src/shared/api.test.ts`

- [x] **Step 1: List auth and invitation endpoints from code**

Run:

```powershell
rg -n "api/auth|api/invitations|handleRegister|handleLogin|handleCreateInvitation|handleVerifyInvitation" internal/server frontend/src/shared/api.ts
```

Expected: the list includes login, register, refresh, invitation create/list/delete/verify, invitation registration, and join-project endpoints.

- [x] **Step 2: Compare response envelopes**

Run:

```powershell
rg -n "access_token|refresh_token|invitation|success|data" api-contract.yaml internal/server/routes.go internal/server/routes_test.go frontend/src/shared/api.ts
```

Expected: any mismatch between code responses and `api-contract.yaml` is recorded before edits.

- [x] **Step 3: Choose one response envelope per endpoint**

Use the currently verified route tests as the source of truth unless the API contract has a clearly safer or more complete shape. Update either route tests/code or `api-contract.yaml`, but do not leave frontend types expecting a different shape from backend tests.

- [x] **Step 4: Verify auth contract**

Run:

```powershell
$env:GOCACHE='D:\vscode\tiktok\openforge\.cache\go-build'
New-Item -ItemType Directory -Force -Path $env:GOCACHE | Out-Null
go test ./internal/server -run "TestHandle(Register|Login|VerifyInvitation|RegisterWithInvitation)"
cd frontend
npm run typecheck
npm test
cd ..
```

Expected: all commands exit 0.

### Task 2: Chat And WebSocket Contract Audit

**Files:**
- Read: `DESIGN.md`
- Read: `api-contract.yaml`
- Read: `docs/security/2026-06-03-phase2-security-gate.md`
- Modify: `internal/server/ws_handler.go`
- Modify: `frontend/src/features/chat/useWebSocket.ts`
- Modify: `DESIGN.md`
- Test: `internal/server/ws_handler_test.go`
- Test: `frontend/src/App.test.tsx`

- [x] **Step 1: Capture the verified WebSocket auth contract**

Run:

```powershell
rg -n "Sec-WebSocket-Protocol|bearer\\.|openforge.auth|first-frame|auth" internal/server/ws_handler.go internal/server/ws_handler_test.go frontend/src/features/chat/useWebSocket.ts DESIGN.md
```

Expected: implementation uses pre-upgrade verification; stale first-frame auth language appears only in docs before this task.

- [x] **Step 2: Update stale WebSocket auth documentation**

In `DESIGN.md`, replace the Phase 2/3 frontend-security sentence that says JWT auth is first-frame with language matching the implemented contract:

```markdown
**WebSocket 鉴权**: JWT 不放 URL query string。浏览器使用 `Sec-WebSocket-Protocol: openforge.auth, bearer.<token>` 在握手阶段传递 1h 短期令牌；服务端在 Upgrade 前校验，缺失或无效令牌返回 401。非浏览器客户端可用 `Authorization: Bearer <token>`。
```

- [x] **Step 3: Add a positive WebSocket token extraction test**

Add a test in `internal/server/ws_handler_test.go`:

```go
func TestBearerTokenFromSubprotocols(t *testing.T) {
	got := bearerTokenFromSubprotocols([]string{"openforge.auth, bearer.abc.def.ghi"})
	if got != "abc.def.ghi" {
		t.Fatalf("token = %q, want abc.def.ghi", got)
	}
}
```

- [x] **Step 4: Verify WebSocket contract**

Run:

```powershell
$env:GOCACHE='D:\vscode\tiktok\openforge\.cache\go-build'
New-Item -ItemType Directory -Force -Path $env:GOCACHE | Out-Null
go test ./internal/server -run "TestChatWS|TestBearerTokenFromSubprotocols"
cd frontend
npm run typecheck
npm test
cd ..
```

Expected: all commands exit 0.

### Task 3: Pipeline Review Contract Audit

**Files:**
- Read: `api-contract.yaml`
- Read: `DESIGN.md`
- Modify: `internal/server/routes.go`
- Modify: `frontend/src/features/code-review/ProModePage.tsx`
- Test: `internal/server/routes_test.go`
- Test: `frontend/src/App.test.tsx`

- [x] **Step 1: List pipeline/review routes**

Run:

```powershell
rg -n "pipelines|review-inbox|gate|diff|branches|messages" api-contract.yaml internal/server/routes.go frontend/src/features/code-review frontend/src/features/review-inbox frontend/src/shared/api.ts
```

Expected: route names and path parameters are visible for code and contract comparison.

- [x] **Step 2: Identify route drift**

Record mismatches such as `project_id` vs `id`, `pipeline_id` vs `pid`, `/token-quota` vs `/token-budget`, and `/ws` vs `/ws/chat`.

- [x] **Step 3: Fix the smallest blocking contract mismatch**

Pick the mismatch that blocks the login-to-project-chat-to-review path first. Update backend route, frontend API method, or `api-contract.yaml` so tests and types agree. Do not batch unrelated contract edits.

- [x] **Step 4: Verify pipeline review contract**

Run:

```powershell
$env:GOCACHE='D:\vscode\tiktok\openforge\.cache\go-build'
New-Item -ItemType Directory -Force -Path $env:GOCACHE | Out-Null
go test ./internal/server
cd frontend
npm run typecheck
npm test
cd ..
```

Expected: all commands exit 0.

## Execution Notes

Completed on 2026-06-04:

- Aligned auth and invitation contracts with verified backend response envelopes, including optional structured error fields.
- Documented and regression-tested WebSocket pre-upgrade authentication via `Sec-WebSocket-Protocol: openforge.auth, bearer.<token>`.
- Fixed Review Inbox navigation to use `project_id` and `pipeline_id` separately.
- Reconciled pipeline/review OpenAPI drift for `/models`, `/token-budget`, `/ws/chat`, review inbox arrays, messages, active pipelines, and diff responses.
- Acceptance verification passed: `go test ./internal/...`, frontend `npm run typecheck`, frontend `npm test`, and `nodejs-io npm test`.

## Acceptance Verification

Run before completing this stabilization increment:

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

Expected: all commands exit 0.
