# Review Data Contract Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make `/api/review-inbox` return product-ready pending review items with project and pipeline display context.

**Architecture:** Keep pending review source-of-truth in `gate_event`, but enrich the BFF response by joining pipeline and project data before returning JSON. Keep frontend changes narrow: add a typed Review Inbox item contract and use it for labels and routing.

**Tech Stack:** Go `net/http`, PostgreSQL, React, TypeScript, Vitest, PowerShell.

---

## Context

This plan implements A1 from `docs/superpowers/specs/2026-06-04-review-approval-loop-design.md`.

Current state:

- `internal/server/routes.go::handleReviewInbox` returns raw `[]*domain.GateEvent`.
- `internal/pipeline/adapter/pg_repository.go::ListPending` reads pending `gate_event` rows without pipeline/project display fields.
- `frontend/src/features/review-inbox/ReviewInboxPage.tsx` already routes with `project_id` and `pipeline_id`, but the backend contract is not yet guaranteed to include project context.
- `frontend/src/shared/api.ts::getReviewInbox` currently returns `any[]`.

Target response item:

```ts
type ReviewInboxItem = {
  pipeline_id: string;
  project_id: string;
  project_name: string;
  pipeline_title: string;
  stage: string;
  event: "awaiting";
  actor: string;
  decision: string;
  artifact_hash: string;
  created_at: string;
  awaiting_since: string;
  claimed_by?: string;
};
```

## File Map

- `internal/server/routes.go`: define BFF review item shape and enrich pending gate events with pipeline/project display context.
- `internal/server/routes_test.go`: add handler-level test for the enriched review inbox response using in-memory fakes where possible.
- `frontend/src/shared/api.ts`: export `ReviewInboxItem` and type `getReviewInbox`.
- `frontend/src/features/review-inbox/ReviewInboxPage.tsx`: consume `ReviewInboxItem`, show project/pipeline labels, and keep existing route helper.
- `frontend/src/features/review-inbox/ReviewInboxPage.test.ts`: expand pure helper tests for labels/time/link behavior.
- `docs/superpowers/plans/2026-06-04-review-data-contract.md`: mark progress during execution.

## Task 1: Backend Review Inbox Contract

**Files:**
- Modify: `internal/server/routes.go`
- Test: `internal/server/routes_test.go`

- [x] **Step 1: Write a failing backend test for enriched review items**

Add a test named `TestHandleReviewInboxEnrichesPendingEvents` to `internal/server/routes_test.go`.

Use the smallest fake `profile.OpenForge` dependencies needed by `handleReviewInbox`. If constructing `GateSvc` with repository fakes is simpler than mocking the handler, define focused fake repositories in the test file.

The assertion must decode the response and check these fields:

```go
assert.Equal(t, http.StatusOK, w.Code)
assert.Equal(t, "pipe-1", item["pipeline_id"])
assert.Equal(t, "proj-1", item["project_id"])
assert.Equal(t, "Conduit", item["project_name"])
assert.Equal(t, "Add tag filters", item["pipeline_title"])
assert.Equal(t, "impl", item["stage"])
assert.Equal(t, "awaiting", item["event"])
assert.NotEmpty(t, item["created_at"])
assert.Equal(t, item["created_at"], item["awaiting_since"])
```

- [x] **Step 2: Run the backend test and verify RED**

Run:

```powershell
$env:GOCACHE='D:\vscode\tiktok\openforge\.cache\go-build'
New-Item -ItemType Directory -Force -Path $env:GOCACHE | Out-Null
go test ./internal/server -run TestHandleReviewInboxEnrichesPendingEvents
```

Expected: FAIL because raw pending gate events do not include `project_id`, `project_name`, `pipeline_title`, or `awaiting_since`.

- [x] **Step 3: Implement the enriched BFF shape**

In `internal/server/routes.go`, add a small response struct near `handleReviewInbox`:

```go
type reviewInboxItem struct {
	PipelineID    string    `json:"pipeline_id"`
	ProjectID     string    `json:"project_id"`
	ProjectName   string    `json:"project_name"`
	PipelineTitle string    `json:"pipeline_title"`
	Stage         string    `json:"stage"`
	Event         string    `json:"event"`
	Actor         string    `json:"actor"`
	Decision      string    `json:"decision"`
	ArtifactHash  string    `json:"artifact_hash"`
	CreatedAt     time.Time `json:"created_at"`
	AwaitingSince time.Time `json:"awaiting_since"`
	ClaimedBy     string    `json:"claimed_by,omitempty"`
}
```

Update `handleReviewInbox` so it:

1. Calls `of.GateSvc.ListPending(r.Context())`.
2. For each event, calls `of.PipelineRepo.GetByID(r.Context(), ev.PipelineID)`.
3. Uses pipeline fields for `project_id` and `pipeline_title`.
4. Loads project name from `of.DB` with:

```go
SELECT name FROM project WHERE id = $1 AND deleted_at IS NULL
```

5. Falls back to empty `project_name` if the project query returns no row.
6. Returns `[]reviewInboxItem{}` when there are no pending events.

Keep the implementation narrow. Do not change `GateService` or `PGRepository.ListPending` in A1 unless the handler cannot be tested otherwise.

- [x] **Step 4: Run the backend test and verify GREEN**

Run:

```powershell
$env:GOCACHE='D:\vscode\tiktok\openforge\.cache\go-build'
New-Item -ItemType Directory -Force -Path $env:GOCACHE | Out-Null
go test ./internal/server -run TestHandleReviewInboxEnrichesPendingEvents
```

Expected: PASS.

- [x] **Step 5: Run server package tests**

Run:

```powershell
$env:GOCACHE='D:\vscode\tiktok\openforge\.cache\go-build'
New-Item -ItemType Directory -Force -Path $env:GOCACHE | Out-Null
go test ./internal/server
```

Expected: PASS.

## Task 2: Frontend Review Inbox Type Contract

**Files:**
- Modify: `frontend/src/shared/api.ts`
- Modify: `frontend/src/features/review-inbox/ReviewInboxPage.tsx`
- Test: `frontend/src/features/review-inbox/ReviewInboxPage.test.ts`

- [x] **Step 1: Write failing frontend helper tests**

Extend `frontend/src/features/review-inbox/ReviewInboxPage.test.ts` with helper expectations:

```ts
import { reviewLinkForItem, reviewTitleForItem, reviewWaitingSinceForItem } from './ReviewInboxPage';

it('formats review item title from project and pipeline context', () => {
  expect(reviewTitleForItem({
    project_name: 'Conduit',
    pipeline_title: 'Add tag filters',
    pipeline_id: 'pipe-1',
  })).toBe('Conduit / Add tag filters');
});

it('prefers awaiting_since for waiting time source', () => {
  expect(reviewWaitingSinceForItem({
    awaiting_since: '2026-06-04T00:00:00Z',
    created_at: '2026-06-03T00:00:00Z',
  })).toBe('2026-06-04T00:00:00Z');
});
```

These helper names do not exist yet, so this test must fail first.

- [x] **Step 2: Run the frontend test and verify RED**

Run:

```powershell
cd frontend
npm test -- src/features/review-inbox/ReviewInboxPage.test.ts
cd ..
```

Expected: FAIL because `reviewTitleForItem` and `reviewWaitingSinceForItem` are not exported.

- [x] **Step 3: Export the frontend contract type**

In `frontend/src/shared/api.ts`, add:

```ts
export type ReviewInboxItem = {
  pipeline_id: string;
  project_id: string;
  project_name: string;
  pipeline_title: string;
  stage: string;
  event: 'awaiting';
  actor: string;
  decision: string;
  artifact_hash: string;
  created_at: string;
  awaiting_since: string;
  claimed_by?: string;
};
```

Change:

```ts
getReviewInbox: () => request<any[]>('/review-inbox'),
```

to:

```ts
getReviewInbox: () => request<ReviewInboxItem[]>('/review-inbox'),
```

- [x] **Step 4: Implement Review Inbox helpers and labels**

In `frontend/src/features/review-inbox/ReviewInboxPage.tsx`:

1. Import `ReviewInboxItem` from `../../shared/api`.
2. Remove the local `type ReviewInboxItem`.
3. Add:

```ts
export function reviewTitleForItem(item: Pick<ReviewInboxItem, 'project_name' | 'pipeline_title' | 'pipeline_id'>): string {
  const title = item.pipeline_title || item.pipeline_id;
  return item.project_name ? `${item.project_name} / ${title}` : title;
}

export function reviewWaitingSinceForItem(item: Pick<ReviewInboxItem, 'awaiting_since' | 'created_at'>): string {
  return item.awaiting_since || item.created_at;
}
```

4. Change the visible title line to use `reviewTitleForItem(item)`.
5. Change waiting time to:

```tsx
Awaiting since {new Date(reviewWaitingSinceForItem(item)).toLocaleString()}
```

- [x] **Step 5: Run the frontend tests and typecheck**

Run:

```powershell
cd frontend
npm test -- src/features/review-inbox/ReviewInboxPage.test.ts
npm run typecheck
cd ..
```

Expected: both commands exit 0.

## Task 3: A1 Acceptance Verification

**Files:**
- Verify only.

- [x] **Step 1: Run focused backend and frontend verification**

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

- [x] **Step 2: Run full project verification**

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

Expected: all commands exit 0.

- [x] **Step 3: Clean temporary Go cache**

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

Expected: `.cache` no longer appears in `git status`.

- [x] **Step 4: Commit A1**

Run:

```powershell
git status --short
git add internal/server/routes.go internal/server/routes_test.go frontend/src/shared/api.ts frontend/src/features/review-inbox/ReviewInboxPage.tsx frontend/src/features/review-inbox/ReviewInboxPage.test.ts docs/superpowers/plans/2026-06-04-review-data-contract.md
git commit -m "feat: enrich review inbox contract"
```

Expected: commit succeeds. Do not stage `DESIGN.md` or `api-contract.yaml` unless the user explicitly requests private design docs to be versioned.
