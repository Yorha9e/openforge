# JWT 下线 + Pipeline Active + Pro Mode 三合一修复计划

> 日期: 2026-05-26 | 状态: 规划中

**目标:** 修复三个关键安全/功能缺陷：
1. **JWT 下线** — 实现服务端 token 撤销，消除"退出后 token 仍可用"的安全漏洞
2. **Pipeline Active** — 完善活跃 pipeline 并发控制、实时推送、状态一致性
3. **Pro Mode** — 打通前端骨架与后端数据，实现可用的代码审查界面

**架构:** 后端 lightweight（最小侵入），优先修复安全性和数据绑定，WebSocket 增强在第二步。

---

## Part A: JWT 下线机制（安全）

### A.1 根因分析

| 问题 | 现状 | 风险 |
|------|------|------|
| 无服务端 token 撤销 | JWT 纯 HMAC-SHA256，无 jti，无黑名单 | 用户退出后 token 仍有效直到过期 |
| 前端 logout 仅清客户端 | `localStorage.removeItem` 不通知后端 | 攻击者持有 token 可持续访问 |
| AuthMiddleware 不检查撤销 | 仅验证签名+过期 | 被窃取的 token 无法失效 |
| 无 refresh_token 存储 | Refresh 直接验签重新签发 | 无法撤销 refresh token 链 |

### A.2 修复方案（最小侵入）

#### Task A1: 数据库新增 `revoked_tokens` 表

```sql
-- migrations/002_jwt_revoke.up.sql
CREATE TABLE IF NOT EXISTS revoked_token (
    jti         VARCHAR(64) PRIMARY KEY,
    user_id     VARCHAR(128) NOT NULL,
    expires_at  TIMESTAMPTZ NOT NULL,
    revoked_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_revoked_token_expires ON revoked_token(expires_at);

-- 定期清理过期记录（可在 profile bootstrap 中启动 goroutine）
```

#### Task A2: JWT Claims 增加 `jti` 字段

**文件:** `internal/auth/service/jwt.go`

```go
type Claims struct {
    JTI       string    `json:"jti"`
    UserID    string    `json:"uid"`
    Role      string    `json:"role"`
    ProjectID string    `json:"pid,omitempty"`
    IssuedAt  time.Time `json:"iat"`
    ExpiresAt time.Time `json:"exp"`
}
```

`Issue()` 方法生成 `jti = fmt.Sprintf("jti-%s", uuid.New().String())`。

#### Task A3: JWTService 增加 `Revoke()` 方法

```go
type TokenRevocationStore interface {
    Revoke(ctx context.Context, jti string, expiresAt time.Time) error
    IsRevoked(ctx context.Context, jti string) (bool, error)
}

func (s *JWTService) Revoke(ctx context.Context, token string) error {
    claims, err := s.Verify(token)
    if err != nil {
        return err // token already invalid, no-op
    }
    return s.revocationStore.Revoke(ctx, claims.JTI, claims.ExpiresAt)
}
```

#### Task A4: AuthMiddleware 增加黑名单检查

**文件:** `internal/server/middleware.go`

```go
func AuthMiddleware(jwtSvc *service.JWTService) func(http.HandlerFunc) http.HandlerFunc {
    return func(next http.HandlerFunc) http.HandlerFunc {
        return func(w http.ResponseWriter, r *http.Request) {
            // ... existing token extraction ...
            claims, err := jwtSvc.Verify(token)
            if err != nil {
                writeJSON(w, 401, map[string]string{"error": "invalid token"})
                return
            }
            // NEW: check revocation
            if revoked, _ := jwtSvc.IsRevoked(r.Context(), claims.JTI); revoked {
                writeJSON(w, 401, map[string]string{"error": "token has been revoked"})
                return
            }
            // ... existing context injection ...
        }
    }
}
```

#### Task A5: 新增 `POST /api/auth/logout` 端点

**文件:** `internal/server/routes.go`

```go
mux.HandleFunc("POST /api/auth/logout", authMw(handleLogout(jwtSvc)))
```

```go
func handleLogout(jwtSvc *service.JWTService) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        token := extractBearerToken(r)
        if token == "" {
            writeJSON(w, 200, map[string]string{"message": "logged out"})
            return
        }
        // Attempt revocation (best-effort; token might already be expired)
        _ = jwtSvc.Revoke(r.Context(), token)
        writeJSON(w, 200, map[string]string{"message": "logged out"})
    }
}
```

#### Task A6: 前端调用 logout API

**文件:** `frontend/src/shared/api.ts`

```typescript
logout: () => request('/auth/logout', { method: 'POST' }),
```

**文件:** `frontend/src/shared/auth.tsx`

```typescript
const logout = useCallback(async () => {
    // Notify server first (best-effort)
    try { await api.logout(); } catch { /* ignore network errors */ }
    // Clear local state
    setAccessToken(null); setRefreshToken(null); setUser(null);
    localStorage.removeItem('of_token');
    localStorage.removeItem('of_refresh');
    localStorage.removeItem('of_user');
    setToken(null);
}, []);
```

### A.3 验收标准

| # | 标准 | 验证方法 |
|---|------|----------|
| A1 | logout 端点返回 200 | `curl -X POST /api/auth/logout -H "Authorization: Bearer <token>"` |
| A2 | 已撤销 token 后续请求返回 401 | `curl /api/projects -H "Authorization: Bearer <revoked-token>"` |
| A3 | 有效 token 不受影响 | 正常登录后 API 调用正常 |
| A4 | 前端 logout 按钮清除状态 | UI 测试：退出后回到登录页 |

---

## Part B: Pipeline Active 机制完善

### B.1 根因分析

| 问题 | 严重度 | 影响 |
|------|--------|------|
| 无并发 pipeline 限制 | **严重** | 资源滥用风险 |
| WebSocket `gate.approve` 为 no-op | **严重** | 前端通过 WS 审批无效 |
| 无 WebSocket 广播 | **中等** | 状态变更不实时同步多客户端 |
| ProjectPage 过滤器遗漏 `awaiting_review` | **中等** | UI 不一致 |
| 无 running 超时检测 | **中等** | 卡住的 pipeline 永久占用 |
| 创建后不自动启动 | **轻微** | pending 对用户不可见 |

### B.2 修复方案

#### Task B1: 增加并发 pipeline 限制

**文件:** `internal/server/routes.go` — `handleCreatePipeline`

在创建 pipeline 前检查同一项目下活跃 pipeline 数量：

```go
// Check active pipeline count for this project
var activeCount int
err := tx.QueryRowContext(r.Context(),
    `SELECT COUNT(*) FROM pipeline 
     WHERE project_id = $1 
     AND status IN ('running','paused','awaiting_review','pending')
     AND deleted_at IS NULL`, 
    projectID).Scan(&activeCount)

maxActive := 10 // default; could be per-project config
if activeCount >= maxActive {
    writeError(w, 429, fmt.Sprintf("max active pipelines (%d) reached for project", maxActive))
    return
}
```

#### Task B2: 修复 WebSocket gate.approve

**文件:** `internal/server/ws_handler.go`

当前 gate.approve handler 是 echo。修复为实际调用：

```go
// ws_handler.go 中 gate.approve case:
case "gate.approve":
    var req struct {
        PipelineID string `json:"pipeline_id"`
        Stage      string `json:"stage"`
        Comment    string `json:"comment"`
    }
    json.Unmarshal(msg.Data, &req)
    // Validate user has pm/dev_lead role
    err := of.GateSvc.Approve(wsCtx, req.PipelineID, req.Stage, userID, 
        domain.GateChecklist{}, req.Comment)
    if err != nil {
        sendJSON(conn, "gate.error", map[string]string{"error": err.Error()})
        return
    }
    // Broadcast to all connections watching this pipeline
    broadcastPipelineUpdate(of, req.PipelineID)
```

#### Task B3: WebSocket 广播机制

**文件:** `internal/server/ws_handler.go` 新增 Hub

```go
type Hub struct {
    mu          sync.RWMutex
    connections map[string]map[*websocket.Conn]bool // pipelineID -> conns
}

func (h *Hub) Subscribe(pipelineID string, conn *websocket.Conn) { ... }
func (h *Hub) Unsubscribe(pipelineID string, conn *websocket.Conn) { ... }
func (h *Hub) Broadcast(pipelineID string, msg WSEvent) { ... }
```

在 `profile.OpenForge` 中注入 `Hub` 单例。pipeline 状态变更（审批/取消/完成）时广播。

#### Task B4: 修复 ProjectPage 过滤器

**文件:** `frontend/src/features/project/ProjectPage.tsx`

```typescript
// 修复前: ['running', 'pending', 'paused']
// 修复后:
const activeStatuses = ['running', 'pending', 'paused', 'awaiting_review'];
```

#### Task B5: running 超时检测后台任务

**文件:** `internal/server/gate_timeout.go` 扩展

在现有的 gate timeout checker 中增加 running 超时检测：

```go
// Check for pipelines stuck in 'running' for > 30 minutes
rows, err := of.DB.QueryContext(ctx,
    `SELECT id, project_id, title, updated_at FROM pipeline
     WHERE status = 'running' AND updated_at < $1 AND deleted_at IS NULL`,
    time.Now().Add(-30*time.Minute))

// Auto-pause stuck pipelines and notify
for rows.Next() {
    // Transition to paused or send alert
}
```

### B.3 验收标准

| # | 标准 |
|---|------|
| B1 | 超过并发上限时创建 pipeline 返回 429 |
| B2 | WS gate.approve 实际调用 GateSvc.Approve() |
| B3 | 审批通过后 Dashboard 实时刷新（广播） |
| B4 | ProjectPage Active 标签显示 awaiting_review 的 pipeline |
| B5 | 运行超过30分钟的 pipeline 被自动暂停 |

---

## Part C: Pro Mode 数据绑定与完善

### C.1 根因分析

| 问题 | 现状 | 缺失 |
|------|------|------|
| Pipeline 无 `changed_files` | Domain 模型不包含此字段 | 需加 JSONB 列 |
| 无 diff 内容 API | Agent 内部 git diff，不暴露 | 需新端点 |
| DiffPanel 无内容 | `original`/`modified` 未传入 | 需 fetch + 传入 |
| ChatPanel pipelineId 错误 | ProModePage 用路径参数，ChatPanel 用查询参数 | 绑定修复 |
| GatePanel 未集成 | 组件存在但不在布局中 | 加入 dockview |
| 无实时阶段更新 | ProModePage 初始加载后不刷新 | WebSocket 监听 |

### C.2 修复方案

#### Task C1: Pipeline 领域模型新增 `ChangedFiles`

**文件:** `internal/pipeline/domain/pipeline.go`

```go
type Pipeline struct {
    // ... existing fields ...
    ChangedFiles []ChangedFile `json:"changed_files,omitempty"`
}

type ChangedFile struct {
    Path   string `json:"path"`
    Status string `json:"status"` // added, modified, deleted
}
```

**文件:** `migrations/002_jwt_revoke.up.sql`（复用同一个 migration）

```sql
ALTER TABLE pipeline ADD COLUMN IF NOT EXISTS changed_files JSONB DEFAULT '[]';
```

**文件:** `internal/pipeline/adapter/pg_repository.go` — 更新查询和写入。

#### Task C2: 新增 `GET /api/pipelines/{id}/diff` 端点

```go
// routes.go
mux.HandleFunc("GET /api/pipelines/{id}/diff", withRole("observer", handleGetDiff(of)))
```

```go
func handleGetDiff(of *profile.OpenForge) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        pipelineID := r.PathValue("id")
        filePath := r.URL.Query().Get("file")
        
        // Fetch diff content from worktree or artifact store
        // For now, return changed_files with before/after from the pipeline config
        p, err := of.PipelineRepo.GetByID(r.Context(), pipelineID)
        if err != nil {
            writeError(w, 404, "pipeline not found")
            return
        }
        
        // Return unified diff for the requested file
        writeJSON(w, 200, map[string]string{
            "file":         filePath,
            "pipeline_id":  pipelineID,
            "original":     p.GetOriginalContent(filePath),
            "modified":     p.GetModifiedContent(filePath),
        })
    }
}
```

#### Task C3: 修复 ProModePage ChatPanel 绑定

**文件:** `frontend/src/features/code-review/ProModePage.tsx`

```tsx
export function ProModePage() {
  const { id, pid } = useParams<{ id: string; pid: string }>();
  
  // Pass pipelineId directly to ChatPanel via URL search params
  // Or create a wrapper that sets the search param context
  const chatPanel = useMemo(() => (
    <ChatPanel embedded pipelineId={pid!} projectId={id!} />
  ), [pid, id]);
  
  // ... rest of dockview setup
}
```

同时修改 `ChatPanel.tsx` 接受显式 `pipelineId` prop（兼容 embedded 模式）：

```tsx
export function ChatPanel({ embedded, pipelineId: pidFromProps, projectId: pidFromProps }: { 
    embedded?: boolean; 
    pipelineId?: string;
    projectId?: string;
}) {
  const { id } = useParams<{ id: string }>();
  const [params] = useSearchParams();
  const pipelineId = pidFromProps || params.get('pipeline') || 'default';
  // ...
}
```

#### Task C4: DiffPanel 连接真实数据

**文件:** `frontend/src/features/code-review/ProModePage.tsx`

```tsx
const [diffContent, setDiffContent] = useState<{original: string; modified: string} | null>(null);

const handleSelectFile = useCallback(async (file: ChangedFile) => {
    setSelectedFile(file.path);
    try {
        const data = await api.getPipelineDiff(pid!, file.path);
        setDiffContent({ original: data.original, modified: data.modified });
    } catch { /* fallback to placeholder */ }
}, [pid]);

// In dockview:
diff: (_props: IDockviewPanelProps) => (
    <DiffPanel 
        fileName={selectedFile ?? undefined}
        original={diffContent?.original}
        modified={diffContent?.modified}
    />
),
```

**文件:** `frontend/src/shared/api.ts`

```typescript
getPipelineDiff: (pipelineId: string, filePath: string) =>
    request<any>(`/pipelines/${pipelineId}/diff?file=${encodeURIComponent(filePath)}`),
```

#### Task C5: GatePanel 集成到 Pro Mode 布局

**文件:** `frontend/src/features/code-review/ProModePage.tsx`

```tsx
// Add GatePanel as a bottom panel in the dockview
api.addPanel({
    id: 'gate',
    component: 'gate',
    title: 'Gate Approval',
    position: { direction: 'below', referencePanel: 'chat' },
});
```

#### Task C6: WebSocket 实时阶段更新

**文件:** `frontend/src/features/chat/ChatProvider.tsx`

在 WebSocket 消息处理中增加 `pipeline.stage_change` 和 `pipeline.files_changed` 的处理：

```typescript
case 'pipeline.stage_change':
    // Update ProModePage pipeline state via callback
    onStageChange?.(data.stage);
    break;
case 'pipeline.files_changed':
    // Refresh file tree
    onFilesChanged?.(data.files);
    break;
```

### C.3 验收标准

| # | 标准 |
|---|------|
| C1 | Pipeline API 返回 `changed_files` 数组 |
| C2 | `GET /api/pipelines/{id}/diff?file=...` 返回 diff 内容 |
| C3 | Pro Mode 中 ChatPanel 连接到正确的 pipeline |
| C4 | 选择文件后 DiffPanel 显示实际 diff |
| C5 | Gate 面板在 Pro Mode 中可用 |
| C6 | 阶段变更通过 WebSocket 实时推送 |

---

## 执行顺序

由于三个 Part 互不依赖，可以**并行执行**：

```
Phase 1 (并行):  Task A1→A4 (后端 JWT revoke 基础设施)
                 Task B1→B2 (并发限制 + WS gate.approve)
                 Task C1→C2 (后端 changed_files + diff API)

Phase 2 (并行):  Task A5→A6 (logout 端点 + 前端)
                 Task B3→B4 (广播 + 过滤器修复)
                 Task C3→C6 (前端数据绑定)

Phase 3 (收尾):  全量验证 go build + go test + frontend build
```

### 文件变更清单

| Part | 文件 | 操作 |
|------|------|------|
| A | `migrations/002_jwt_revoke.up.sql` | CREATE |
| A | `internal/auth/service/jwt.go` | MODIFY (jti + Revoke) |
| A | `internal/auth/port/repository.go` | MODIFY (TokenRevocationStore) |
| A | `internal/auth/adapter/pg_auth_repository.go` | MODIFY (Revoke/IsRevoked) |
| A | `internal/server/routes.go` | MODIFY (logout 端点 + middleware) |
| A | `internal/server/middleware.go` | MODIFY (黑名单检查) |
| A | `frontend/src/shared/api.ts` | MODIFY (logout) |
| A | `frontend/src/shared/auth.tsx` | MODIFY (logout) |
| B | `internal/server/routes.go` | MODIFY (并发限制) |
| B | `internal/server/ws_handler.go` | MODIFY (gate.approve + Hub) |
| B | `internal/server/gate_timeout.go` | MODIFY (running 超时) |
| B | `frontend/src/features/project/ProjectPage.tsx` | MODIFY (过滤器) |
| C | `internal/pipeline/domain/pipeline.go` | MODIFY (ChangedFiles) |
| C | `internal/pipeline/adapter/pg_repository.go` | MODIFY (changed_files 读写) |
| C | `internal/server/routes.go` | MODIFY (diff 端点) |
| C | `frontend/src/features/code-review/ProModePage.tsx` | MODIFY (完整数据绑定) |
| C | `frontend/src/features/chat/ChatPanel.tsx` | MODIFY (pipelineId prop) |
| C | `frontend/src/features/chat/ChatProvider.tsx` | MODIFY (WS 事件) |
| C | `frontend/src/shared/api.ts` | MODIFY (getPipelineDiff) |
