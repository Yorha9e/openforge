# Admin Panel Audit Fixes Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Fix critical bugs and gaps in the Admin page frontend (`AdminPage.tsx`, `SkillPanel.tsx`) discovered during audit, making Skill management functional and Admin status display accurate.

**Tech Stack:** React 19 + TypeScript, no backend changes needed.

---

## Current Audit Snapshot

### 🔴 Critical (2 issues)

1. **SkillPanel — Deprecate/Restore 按钮无实际操作**
   - Confirm deprecation 只清空了 `deprecateConfirm` 状态，从未调用 API
   - 需要在前端补充 API 调用，或确认后端是否有对应端点

2. **SkillPanel — `priorityEdits` 没有提交入口**
   - 优先级编辑只有 UI，没有 "Save Priorities" 按钮或自动保存
   - 刷新后所有修改丢失

### 🟡 Medium (5 issues)

3. **AdminPage — `auditChain` 硬编码为 `'healthy'`**
   - 成功和失败分支都写死 `'healthy'`，后端可能返回了实际值

4. **AdminPage — API 失败时 fallback 状态有误导性**
   - `.catch()` 中显示 `rbac: 'active'`、`auditChain: 'healthy'`，实际 Admin API 已不可用

5. **AdminPage — 后端返回 SLO/HA/CircuitBreaker 数据未展示**
   - `api.ts` 的 `AdminStatus` 类型已定义这些字段，但页面未渲染

6. **AdminPage — Role Hierarchy 可视化与实际权限不匹配**
   - 页面展示 `admin → pm → dev_lead → dev → observer` 线性链
   - 实际 `pm` 和 `dev_lead` 是平级关系

7. **AdminPage — Roadmap Phase 6-8 全部标记 `done: true`**
   - Phase 8 还在开发中，Roadmap 可能过期

### 🟢 Low (3 issues)

8. AdminPage — 无 `loading` 状态，数据未就绪时 System Status 区域为空
9. SkillPanel — 搜索输入未做防抖
10. SkillPanel — Cancel 按钮误触发了 confirm 行为

---

## Execution Rules for Agents

- Use TDD: write/modify tests first, run them and confirm they fail for the expected reason, then implement.
- Do not rename existing packages unless a task explicitly says so.
- Prefer small commits after each task.
- Do not commit generated binaries (`*.exe`) or screenshots.
- All changes are frontend-only; no backend modifications needed.

---

## Task 1: Fix SkillPanel — Add Deprecate/Restore API Integration

- [ ] Confirm backend has `PATCH /api/admin/skills/:name` with `{ deprecated: boolean }` body, or add the endpoint if missing
- [ ] In `SkillPanel.tsx`, add `handleDeprecateToggle(skillName, deprecated)` that calls the API
- [ ] Wire Confirm Deprecation button to `handleDeprecateToggle(selectedSkill.name, true)`
- [ ] Wire Restore button to `handleDeprecateToggle(selectedSkill.name, false)`
- [ ] Add loading/error state for the API call
- [ ] **Test:** Write a test that simulates clicking Confirm → verifies API call is made with `{ deprecated: true }`

## Task 2: Fix SkillPanel — Add Priority Save

- [ ] Confirm backend has `PUT /api/admin/skills/priorities` with `{ priorities: Record<string, number> }` body, or add the endpoint if missing
- [ ] Add a "Save Priorities" button below the skills table, visible when `priorityEdits` is non-empty
- [ ] Implement `handleSavePriorities()` that sends the API call
- [ ] On success, reset `priorityEdits` and refresh skills list
- [ ] Add loading/error state
- [ ] **Test:** Write a test that edits a priority → clicks Save → verifies API call

## Task 3: Fix AdminPage — Wire Real Status Data

- [ ] Check actual response shape of `GET /api/admin/status` and map `auditChain` from real data
- [ ] Replace hardcoded `'healthy'` with actual value; if backend doesn't return it, display as "Unknown"
- [ ] Fix `.catch()` fallback: show "Error" / "Unavailable" instead of faking healthy status
- [ ] Render SLO data (success rate, P95) from `status.slo` if available
- [ ] Render HA data (task queue, overload protection) from `status.ha` if available
- [ ] Render circuit breaker states from `status.circuitBreakers` if available
- [ ] **Test:** Write a test for successful data rendering and error fallback states

## Task 4: Fix AdminPage — Correct Role Hierarchy Display

- [ ] Replace linear chain visualization with side-by-side layout showing `pm` and `dev_lead` as peers
- [ ] Visual: `admin → [pm | dev_lead] → dev → observer`
- [ ] **Test:** Verify the rendered component matches the actual `roleHierarchy` from `auth.tsx`

## Task 5: Fix AdminPage — Update Roadmap

- [ ] Review current Phase 6-8 actual completion status
- [ ] Update `done` flags for phases still in progress
- [ ] Add any newly started phases to the roadmap

## Task 6: Low Priority Polish

- [ ] Add `loading` state to AdminPage's `useEffect`, show spinner while data loads
- [ ] Add debounce (200ms) to SkillPanel search input to reduce re-renders
- [ ] Fix SkillPanel Cancel button so it resets `deprecateConfirm` without triggering confirm behavior
- [ ] **Test:** Verify loading spinner appears, search debounce works

---

## Verification Matrix

| Task | Lint Check | Unit Tests | Manual Smoke |
|------|-----------|------------|-------------|
| 1 - Deprecate/Restore | ✅ | ✅ | Click deprecate → confirm → verify API called |
| 2 - Priority Save | ✅ | ✅ | Edit priority → Save → refresh → priority persisted |
| 3 - Real Status Data | ✅ | ✅ | Load Admin page → SLO/HA/CB data visible |
| 4 - Role Hierarchy | ✅ | ✅ | Visual matches auth.tsx hierarchy |
| 5 - Roadmap | ✅ | N/A | Phases reflect actual status |
| 6 - Polish | ✅ | ✅ | Spinner visible, search doesn't lag |
