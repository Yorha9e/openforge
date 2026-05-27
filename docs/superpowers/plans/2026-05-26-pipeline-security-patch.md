# Pipeline Security Patch Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 修复 API 接口中的越权漏洞（BOLA/IDOR），确保只有对应 Project 内的授权用户（PM/Observer 等角色）才能拉取和软删除对应的 Pipeline。

**Architecture:** 
- 在 API 处理器内部，执行操作前先通过 `PipelineRepo.GetByID` 获取 Pipeline 实体。
- 提取并校验 Pipeline 实例的 `ProjectID`，与上下文中的 `ProjectID`（通过 JWT 解析）进行比对。
- 如果上下文存在 `ProjectID` 且与 Pipeline 属性不符，拒绝请求并返回 403 错误（越权保护）。

**Tech Stack:** Go (Standard Library http.ServeMux, context)

---

### Task 1: 修复 handleGetMessages 越权漏洞

**Files:**
- Modify: `d:/vscode/tiktok/openforge/internal/server/routes.go:618-635`

- [ ] **Step 1: 修改 handleGetMessages 增加 ProjectID 越权校验**

在 `internal/server/routes.go` 中，重构 `handleGetMessages` 处理器。先从 `PipelineRepo` 根据 ID 读取 Pipeline。如果存在上下文 ProjectID，必须与 Pipeline 的 ProjectID 保持一致。

```go
func handleGetMessages(of *profile.OpenForge) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		pipelineID := r.PathValue("pid")
		branchID := r.URL.Query().Get("branch_id")
		if branchID == "" {
			branchID = "main"
		}

		// 1. 获取 Pipeline 详情，以便验证归属项目
		p, err := of.PipelineRepo.GetByID(r.Context(), pipelineID)
		if err != nil {
			writeError(w, http.StatusNotFound, "pipeline not found")
			return
		}

		// 2. 越权校验: 如果上下文有关联 ProjectID，进行一致性校验
		ctxProjectID, _ := r.Context().Value(domain.ProjectIDContextKey).(string)
		if ctxProjectID != "" && p.ProjectID != ctxProjectID {
			writeError(w, http.StatusForbidden, "forbidden: access to this pipeline is denied")
			return
		}

		msgs, err := of.PipelineRepo.GetMessages(r.Context(), pipelineID, branchID)
		if err != nil {
			writeError(w, 500, sanitizeError(err))
			return
		}
		if msgs == nil {
			msgs = []*port2.DBMessage{}
		}
		writeJSON(w, 200, msgs)
	}
}
```

---

### Task 2: 修复 handleDeletePipeline 越权漏洞

**Files:**
- Modify: `d:/vscode/tiktok/openforge/internal/server/routes.go:400-410` (对应 handleDeletePipeline 的位置)

- [ ] **Step 1: 修改 handleDeletePipeline 增加 ProjectID 越权校验**

在 `internal/server/routes.go` 中，重构 `handleDeletePipeline` 处理器，加入鉴权保护：

```go
func handleDeletePipeline(of *profile.OpenForge) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")

		// 1. 获取 Pipeline 详情
		p, err := of.PipelineRepo.GetByID(r.Context(), id)
		if err != nil {
			writeError(w, http.StatusNotFound, "pipeline not found")
			return
		}

		// 2. 越权校验
		ctxProjectID, _ := r.Context().Value(domain.ProjectIDContextKey).(string)
		if ctxProjectID != "" && p.ProjectID != ctxProjectID {
			writeError(w, http.StatusForbidden, "forbidden: deletion of this pipeline is denied")
			return
		}

		// 3. 执行软删除
		if err := of.PipelineRepo.Delete(r.Context(), id); err != nil {
			writeError(w, 500, sanitizeError(err))
			return
		}
		writeJSON(w, 200, map[string]string{"status": "deleted"})
	}
}
```

- [ ] **Step 2: 运行编译和 Linter 校验**

在终端确认项目无编译报错。
