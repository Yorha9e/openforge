# Review Approval Loop Design

> Date: 2026-06-04
> Scope: Phase A of the next product increment, before Pipeline creation/chat execution work.

## Goal

Build the approval workflow from "usable" to "product-ready": a reviewer should be able to see pending gate work, understand what needs review, open the correct ProMode workspace, approve or reject, and see the work queue update.

This phase is intentionally split into three increments so the product can improve safely without turning into a large, tangled release.

## Increment A1: Review Data Contract

The current Review Inbox path works, but the pending gate records are too thin. The frontend can route with `project_id` only after prior stabilization work, but the backend pending gate response still originates from raw `gate_event` data and does not reliably carry project and pipeline display context.

A1 adds a stable review item contract:

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

Backend source of truth:

- Pending work remains `gate_event.event = 'awaiting'`.
- Display fields come from joining `pipeline` and `project` by `gate_event.pipeline_id`.
- `awaiting_since` mirrors `gate_event.created_at` for frontend clarity while preserving `created_at` compatibility.

Frontend source of truth:

- `api.getReviewInbox()` returns `ReviewInboxItem[]`.
- Review links use `/project/:project_id/pipeline/:pipeline_id`.
- Existing `created_at` display falls back to `awaiting_since` until all consumers move to the new name.

Acceptance:

- Backend route test proves `/api/review-inbox` includes project and pipeline display fields.
- Frontend type/test proves Review Inbox links and labels use separate project and pipeline IDs.
- Existing full verification still passes.

## Increment A2: Productized Review Inbox

A2 turns the Review Inbox from a simple list into a real work queue.

User-visible behavior:

- Loading state remains skeleton-based and does not jump layout.
- Empty state explains that no gate reviews are pending.
- Error state provides a retry path instead of only static text.
- Items show project name, pipeline title, stage, waiting time, and a clear Review action.
- Filtering supports stage and project name text search.
- Pending count is visible at the top.

Design constraints:

- Use existing `tokens`, `AppLayout`, `PageSkeleton`, and the current dark workbench style.
- Keep the page dense and operational, not marketing-like.
- Do not introduce new design libraries.

Acceptance:

- Frontend tests cover filtering, empty state, and retry behavior at the pure helper or component boundary available in the existing Vitest setup.
- `npm run typecheck` and `npm test` pass.

## Increment A3: Gate Action Closed Loop

A3 completes the approval action loop between ProMode and Review Inbox.

User-visible behavior:

- GatePanel disables actions while approve/reject is in flight.
- Success feedback is shown after approve or reject.
- GatePanel can notify the parent page when an action completes.
- ProMode refreshes pipeline data after an action so stage/status display updates.
- Review Inbox removes processed items on refresh.
- Dashboard pending review count reflects the updated queue after navigation or reload.

Backend behavior:

- Existing approve/reject endpoints remain the command API.
- The response can stay `{status: "approved" | "rejected"}` for A3 unless frontend needs a pipeline snapshot.
- If a route already updates pipeline status through `GateService`, do not duplicate state transitions in handlers.

Acceptance:

- Backend route/service tests prove approve/reject remove the item from pending review results.
- Frontend tests cover GatePanel success callback and Review Inbox refresh behavior where practical.
- Full acceptance verification passes.

## Out Of Scope

- New Pipeline creation UX. That is Phase B.
- Chat-driven pipeline execution. That is Phase C.
- Admin/cost/project-management enhancements. Those are Phase D.
- Full visual redesign of ProMode. Only targeted usability improvements are allowed in A.

## Rollout Order

1. A1 Review Data Contract.
2. A2 Productized Review Inbox.
3. A3 Gate Action Closed Loop.
4. Phase B Pipeline creation and state flow.
5. Phase C Chat-driven execution.
6. Phase D Peripheral capabilities.

## Risks

- `docs/` is ignored in this repository, so planning docs must be force-added when they are meant to be saved.
- Some design/contract source files are intentionally ignored as private repo artifacts; avoid force-adding them unless explicitly requested.
- Current frontend tests do not use a browser DOM environment broadly, so A2/A3 tests should prefer pure helpers or lightweight component seams unless the test setup is expanded deliberately.
