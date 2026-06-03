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
| Stale Phase 1 guidance | Docs | `AGENTS.md` conflicts with `phase1-summary.md` before the current-status update | Task 2 in current-stage plan |
| Security gate not re-audited after frontend expansion | Backend/frontend | Routes and pages exist beyond original Phase 2 | Task 4 in current-stage plan |
| Current worktree not committed | Process | `git status --short` showed tracked and untracked changes before Task 1 | Task 1 in current-stage plan |
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
