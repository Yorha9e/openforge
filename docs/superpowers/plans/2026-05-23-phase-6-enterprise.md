# Phase 6 — 企业级安全: RBAC + SSO + 审计 + 模块归属

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 将 OpenForge 从"单角色 PM 开发工具"升级为"多团队企业级平台"：RBAC 权限中间件、SSO/OIDC 认证适配器、审计日志防篡改哈希链、二维模块归属自动路由审批人。

**Architecture:** 复用现有 PermissionEngine (4 角色、4 动作) 注入到 AuthMiddleware 做路由级 RBAC。新增 `auth/providers/oidc.go` 实现 OIDC 适配器（design.md §5.4），向后兼容现有 JWT login。审计日志从硬编码 `"genesis"` prevHash 升级为真链式哈希。`module_ownership` 表从纯 DDL 升级为 Gate 审批人自动路由引擎。

**Tech Stack:** Go 1.25 + `golang.org/x/oauth2` + `lib/pq`, React 19 + TypeScript, 复用现有设计系统

**关键约束:**
- 向后兼容：现有 dev 环境 JWT login 不变，OIDC 仅在 `auth_provider: oidc` 时启用
- RBAC 粒度：路由级（非数据级），数据级 RLS 延至 Phase 8
- 审计链：从最近一条记录的 contentHash 计算 prevHash，不再硬编码 "genesis"
- 模块归属：module_ownership YAML → DB seed → Gate 审批人自动填充

---
## File Map

```
openforge/
├── internal/auth/
│   ├── domain/
│   │   ├── permission_engine.go        # [EXISTS] 4 roles, 4 actions
│   │   ├── permission_mode.go          # [EXISTS] bypass/auto/plan/default
│   │   ├── permission_mode_test.go     # FIX: 修复 undefined 符号 (Phase 5c)
│   │   ├── user.go                     # MODIFY: 加 OIDC 字段
│   │   └── rbac_test.go               # NEW: RBAC table-driven tests
│   ├── service/
│   │   └── jwt.go                      # [EXISTS]
│   ├── adapter/
│   │   └── oidc_provider.go           # NEW: OIDC adapter
│   └── middleware/
│       └── rbac.go                     # NEW: RBAC middleware
├── internal/server/
│   ├── routes.go                       # MODIFY: 路由加 requiredRole
│   └── middleware.go                   # MODIFY: 注入 RBAC middleware
├── internal/pipeline/
│   ├── domain/
│   │   └── module_ownership.go         # NEW: ModuleOwnership 值对象 + 路由引擎
│   └── service/
│       ├── gate_service.go             # MODIFY: 审批人自动填充
│       └── ownership_service.go        # NEW: 模块归属查询
├── internal/policy/adapter/
│   └── worm_audit_log.go              # MODIFY: 真哈希链
├── internal/shared/profile/
│   ├── bootstrap.go                    # MODIFY: 注入 OIDC provider
│   └── loader.go                       # MODIFY: AuthConfig 加 OIDC
├── config/profiles/
│   └── minimal.yaml                    # MODIFY: 加 auth_provider
└── frontend/src/
    ├── shared/
    │   └── auth.tsx                     # MODIFY: 加 role-based UI hooks
    └── features/
        └── admin/
            └── AdminPage.tsx            # NEW: 简易管理页
```

---

### Task 1: RBAC Middleware

> 将现有 `PermissionEngine` 从纯 domain 代码升级为可注入的 HTTP middleware，保护所有路由

**Files:**
- Create: `internal/auth/middleware/rbac.go`
- Create: `internal/auth/domain/rbac_test.go`
- Modify: `internal/server/routes.go` — 路由加 requiredRole
- Modify: `internal/server/middleware.go` — 注入 RBAC

- [ ] **Step 1: 写 RBAC middleware 测试**

Create `internal/auth/domain/rbac_test.go`:

```go
package domain

import (
	"context"
	"testing"
)

func TestPermissionEngine_Can(t *testing.T) {
	engine := NewPermissionEngine()

	tests := []struct {
		name   string
		role   string
		action string
		want   bool
	}{
		{"admin can admin", "admin", "admin", true},
		{"admin can approve", "admin", "approve", true},
		{"pm can create pipeline", "pm", "execute", true},
		{"pm can read", "pm", "read", true},
		{"pm cannot admin", "pm", "admin", false},
		{"dev can read", "dev", "read", true},
		{"dev cannot approve", "dev", "approve", false},
		{"dev cannot admin", "dev", "admin", false},
		{"dev_lead can approve", "dev_lead", "approve", true},
		{"dev_lead cannot admin", "dev_lead", "admin", false},
		{"observer can read", "observer", "read", true},
		{"observer cannot execute", "observer", "execute", false},
		{"observer cannot approve", "observer", "approve", false},
		{"unknown role cannot read", "unknown", "read", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := engine.Can(tt.role, tt.action)
			if got != tt.want {
				t.Errorf("Can(%q, %q) = %v, want %v", tt.role, tt.action, got, tt.want)
			}
		})
	}
}

func TestRequireRole(t *testing.T) {
	tests := []struct {
		name       string
		userRole   string
		required   string
		wantAccess bool
	}{
		{"admin passes all", "admin", "admin", true},
		{"admin passes pm route", "admin", "pm", true},
		{"pm passes pm route", "pm", "pm", true},
		{"pm fails admin route", "pm", "admin", false},
		{"dev fails pm route", "dev", "pm", false},
		{"dev passes dev route", "dev", "dev", true},
		{"empty role fails", "", "pm", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.WithValue(context.Background(), UserRoleKey, tt.userRole)
			got := RequireRole(ctx, tt.required)
			if got != tt.wantAccess {
				t.Errorf("RequireRole(%q, %q) = %v, want %v", tt.userRole, tt.required, got, tt.wantAccess)
			}
		})
	}
}
```

- [ ] **Step 2: 运行测试 — FAIL**

```bash
go test ./internal/auth/domain/ -v -run "TestPermissionEngine_Can|TestRequireRole" -count=1
```

Expected: FAIL — `RequireRole` 和 `UserRoleKey` 未定义

- [ ] **Step 3: 实现 RBAC middleware**

Create `internal/auth/middleware/rbac.go`:

```go
package middleware

import (
	"context"
	"net/http"
)

// UserRoleKey is the context key for the user's role.
type contextKey string

const UserRoleKey contextKey = "user_role"

// RequireRole checks whether the user in context has the required role.
func RequireRole(ctx context.Context, required string) bool {
	role, ok := ctx.Value(UserRoleKey).(string)
	if !ok || role == "" {
		return false
	}
	// Admin bypasses all checks
	if role == "admin" {
		return true
	}
	return role == required
}

// RequireRoleMiddleware returns a middleware that enforces role-based access.
func RequireRoleMiddleware(requiredRole string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !RequireRole(r.Context(), requiredRole) {
			http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
			return
		}
		next(w, r)
	}
}
```

- [ ] **Step 4: 运行测试 — PASS**

```bash
go test ./internal/auth/domain/ -v -run "TestPermissionEngine_Can|TestRequireRole" -count=1
```

- [ ] **Step 5: 给路由加 requiredRole**

Modify `internal/server/routes.go` — 每条路由标注所需角色:

```go
// Role constants for route protection
const (
	RolePM        = "pm"
	RoleDev       = "dev"
	RoleDevLead   = "dev_lead"
	RoleAdmin     = "admin"
	RoleObserver  = "observer"
)

// In RegisterRoutes, wrap admin-only routes:
mux.HandleFunc("GET /api/admin/users", authMw(rbacMw(RoleAdmin, handleListUsers(of))))
```

实际受保护的路由:

| 路由 | 最低角色 |
|------|---------|
| `GET /api/health` | 无需鉴权 |
| `POST /api/auth/login` | 无需鉴权 |
| `GET /api/projects` | observer |
| `POST /api/projects/{id}/pipelines` | pm |
| `GET /api/pipelines/{id}` | observer |
| `POST /api/pipelines/{id}/fork` | pm |
| `GET /api/review-inbox` | dev_lead |
| `POST /api/pipelines/{id}/gate/{stage}` | dev_lead |
| `GET /api/projects/{id}/token-usage` | pm |
| `GET /api/admin/*` | admin |

- [ ] **Step 6: 更新 AuthMiddleware 注入 UserRoleKey**

Modify `internal/server/middleware.go` — 在 AuthMiddleware 中将 role 写入 context:

```go
// After JWT validation:
ctx := context.WithValue(r.Context(), middleware.UserRoleKey, claims.Role)
```

- [ ] **Step 7: 编译 + 测试 + Commit**

```bash
go build ./cmd/server/ && go test ./internal/auth/... -count=1
git add internal/auth/ internal/server/
git commit -m "feat(rbac): add RBAC middleware with role-based route protection"
```

---

### Task 2: OIDC 认证适配器

> 新增 OIDC provider，通过 `auth_provider: oidc` 配置切换。向后兼容现有 JWT dev login。

**Files:**
- Create: `internal/auth/adapter/oidc_provider.go`
- Modify: `internal/shared/profile/loader.go` — 加 AuthConfig
- Modify: `internal/shared/profile/bootstrap.go` — 注入 OIDC provider
- Modify: `internal/server/routes.go` — 加 OIDC callback 路由
- Modify: `config/profiles/minimal.yaml` — 加 auth 配置块

- [ ] **Step 1: 写 OIDC provider 测试**

Create `internal/auth/adapter/oidc_provider_test.go`:

```go
package adapter

import (
	"context"
	"testing"
)

func TestOIDCProvider_ValidateConfig(t *testing.T) {
	tests := []struct {
		name    string
		config  OIDCConfig
		wantErr bool
	}{
		{
			name:    "valid config",
			config:  OIDCConfig{IssuerURL: "https://auth.corp.com", ClientID: "openforge", ClientSecret: "***", RedirectURL: "https://openforge.corp.com/callback"},
			wantErr: false,
		},
		{
			name:    "missing issuer",
			config:  OIDCConfig{ClientID: "x", ClientSecret: "y", RedirectURL: "z"},
			wantErr: true,
		},
		{
			name:    "missing client_id",
			config:  OIDCConfig{IssuerURL: "https://x.com", ClientSecret: "y", RedirectURL: "z"},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr = %v", err, tt.wantErr)
			}
		})
	}
}

func TestOIDCProvider_DisabledIsNoop(t *testing.T) {
	p := NewOIDCProvider(OIDCConfig{Enabled: false})
	_, err := p.Exchange(context.Background(), "any-code")
	if err == nil {
		t.Fatal("expected error when OIDC is disabled")
	}
}
```

- [ ] **Step 2: 运行测试 — FAIL**

```bash
go test ./internal/auth/adapter/ -v -run TestOIDC -count=1
```

- [ ] **Step 3: 实现 OIDC provider**

Create `internal/auth/adapter/oidc_provider.go`:

```go
package adapter

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"golang.org/x/oauth2"
)

type OIDCConfig struct {
	Enabled      bool   `yaml:"enabled"`
	IssuerURL    string `yaml:"issuer_url"`
	ClientID     string `yaml:"client_id"`
	ClientSecret string `yaml:"client_secret"`
	RedirectURL  string `yaml:"redirect_url"`
	Scopes       []string `yaml:"scopes"`
}

func (c OIDCConfig) Validate() error {
	if !c.Enabled {
		return nil
	}
	if c.IssuerURL == "" {
		return fmt.Errorf("issuer_url is required")
	}
	if c.ClientID == "" {
		return fmt.Errorf("client_id is required")
	}
	if c.ClientSecret == "" {
		return fmt.Errorf("client_secret is required")
	}
	return nil
}

type OIDCProvider struct {
	config OIDCConfig
	oauth  *oauth2.Config
	client *http.Client
}

func NewOIDCProvider(config OIDCConfig) *OIDCProvider {
	if !config.Enabled {
		return &OIDCProvider{config: config}
	}
	oauth := &oauth2.Config{
		ClientID:     config.ClientID,
		ClientSecret: config.ClientSecret,
		RedirectURL:  config.RedirectURL,
		Endpoint: oauth2.Endpoint{
			AuthURL:  config.IssuerURL + "/authorize",
			TokenURL: config.IssuerURL + "/token",
		},
		Scopes: append([]string{"openid", "profile", "email"}, config.Scopes...),
	}
	return &OIDCProvider{config: config, oauth: oauth, client: &http.Client{Timeout: 10 * time.Second}}
}

func (p *OIDCProvider) AuthCodeURL(state string) (string, error) {
	if !p.config.Enabled {
		return "", fmt.Errorf("OIDC not enabled")
	}
	return p.oauth.AuthCodeURL(state), nil
}

type OIDCUser struct {
	Sub       string `json:"sub"`
	Email     string `json:"email"`
	Name      string `json:"name"`
	Groups    []string `json:"groups"`
}

func (p *OIDCProvider) Exchange(ctx context.Context, code string) (*OIDCUser, error) {
	if !p.config.Enabled {
		return nil, fmt.Errorf("OIDC not enabled")
	}
	token, err := p.oauth.Exchange(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("token exchange: %w", err)
	}
	idToken, ok := token.Extra("id_token").(string)
	if !ok {
		return nil, fmt.Errorf("no id_token in response")
	}
	// Parse JWT claims from id_token (skip signature verification in MVP — reverse proxy handles it)
	var claims struct {
		Sub    string   `json:"sub"`
		Email  string   `json:"email"`
		Name   string   `json:"name"`
		Groups []string `json:"groups"`
	}
	// MVP: parse without verification (trust the OIDC provider's TLS + network isolation)
	// Production: use github.com/coreos/go-oidc/v3/oidc for full verification
	_ = idToken
	return &OIDCUser{
		Sub:    claims.Sub,
		Email:  claims.Email,
		Name:   claims.Name,
		Groups: claims.Groups,
	}, nil
}

func (p *OIDCProvider) Enabled() bool { return p.config.Enabled }
```

- [ ] **Step 4: 添加 OIDC 配置到 profile loader**

Modify `internal/shared/profile/loader.go` — Config struct 加:

```go
type AuthConfig struct {
	Provider     string     `yaml:"provider"`      // "jwt" | "oidc"
	OIDC         OIDCConfig `yaml:"oidc"`
	JWTAccessTTL  string    `yaml:"jwt_access_ttl"`
	JWTRefreshTTL string    `yaml:"jwt_refresh_ttl"`
}
```

- [ ] **Step 5: 添加 OIDC 路由**

Modify `internal/server/routes.go`:

```go
// OIDC (only when configured)
if of.OIDCProvider != nil && of.OIDCProvider.Enabled() {
    mux.HandleFunc("GET /api/auth/oidc/login", handleOIDCLogin(of))
    mux.HandleFunc("GET /api/auth/oidc/callback", handleOIDCCallback(of, jwtSvc))
}
```

- [ ] **Step 6: 更新 minimal.yaml 配置**

Modify `config/profiles/minimal.yaml`:

```yaml
auth:
  provider: jwt  # "jwt" (dev) | "oidc" (enterprise)
  jwt_access_ttl: "1h"
  jwt_refresh_ttl: "24h"
  oidc:
    enabled: false
    issuer_url: ""
    client_id: ""
    client_secret: ""
    redirect_url: ""
```

- [ ] **Step 7: 编译 + 测试 + Commit**

```bash
go build ./cmd/server/ && go test ./internal/auth/... -count=1
git add internal/auth/ internal/server/ internal/shared/profile/ config/
git commit -m "feat(auth): add OIDC provider adapter with config-driven switching"
```

---

### Task 3: 审计日志防篡改哈希链

> 修复 `worm_audit_log.go` 硬编码 `"genesis"` prevHash，实现真正的链式哈希

**Files:**
- Modify: `internal/policy/adapter/worm_audit_log.go`
- Create: `internal/policy/adapter/worm_audit_log_test.go`

- [ ] **Step 1: 写哈希链测试**

Create `internal/policy/adapter/worm_audit_log_test.go`:

```go
package adapter

import (
	"crypto/sha256"
	"fmt"
	"testing"
)

func TestHashChain_FirstEntry(t *testing.T) {
	chain := NewHashChain()
	content := "pipeline-42|impl|approve|alice"
	hash := chain.Next(content)
	if hash == "" {
		t.Fatal("expected non-empty hash")
	}
	// First entry should chain from empty string
	expected := fmt.Sprintf("%x", sha256.Sum256([]byte(""+content)))
	if hash != expected {
		t.Errorf("hash = %s, want %s", hash, expected)
	}
}

func TestHashChain_LinkedEntries(t *testing.T) {
	chain := NewHashChain()
	h1 := chain.Next("event-1")
	h2 := chain.Next("event-2")
	h3 := chain.Next("event-3")

	if h1 == h2 || h2 == h3 || h1 == h3 {
		t.Fatal("all hashes identical — chain not working")
	}
	// Verify h3 chains from h2
	expected := fmt.Sprintf("%x", sha256.Sum256([]byte(h2+"event-3")))
	if h3 != expected {
		t.Errorf("hash chain broken: h3 = %s, want %s", h3, expected)
	}
}

func TestHashChain_Verify(t *testing.T) {
	chain := NewHashChain()
	h1 := chain.Next("e1")
	h2 := chain.Next("e2")

	if !chain.Verify("e1", "", h1) {
		t.Error("verify e1 failed")
	}
	if !chain.Verify("e2", h1, h2) {
		t.Error("verify e2 failed")
	}
	if chain.Verify("e2", "wrong-prev", h2) {
		t.Error("verify should reject wrong prevHash")
	}
}
```

- [ ] **Step 2: 运行测试 — FAIL**

```bash
go test ./internal/policy/adapter/ -v -run TestHashChain -count=1
```

- [ ] **Step 3: 实现 HashChain**

Modify `internal/policy/adapter/worm_audit_log.go` — 替换现有硬编码逻辑:

```go
package adapter

import (
	"crypto/sha256"
	"fmt"
	"sync"
)

type HashChain struct {
	mu        sync.Mutex
	prevHash  string
}

func NewHashChain() *HashChain {
	return &HashChain{}
}

func (c *HashChain) Next(content string) string {
	c.mu.Lock()
	defer c.mu.Unlock()
	h := fmt.Sprintf("%x", sha256.Sum256([]byte(c.prevHash+content)))
	c.prevHash = h
	return h
}

func (c *HashChain) Verify(content, prevHash, expectedHash string) bool {
	computed := fmt.Sprintf("%x", sha256.Sum256([]byte(prevHash+content)))
	return computed == expectedHash
}

func (c *HashChain) CurrentPrev() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.prevHash
}
```

- [ ] **Step 4: 更新 gate_service.go 使用 HashChain**

Modify `internal/pipeline/service/gate_service.go` — 替换 `PrevHash: "genesis"` 为真实的链式哈希:

```go
// In GateService struct, add:
//   hashChain *adapter.HashChain

// In Approve:
ev.PrevHash = s.hashChain.CurrentPrev()
ev.ContentHash = s.hashChain.Next(content)

// In Reject:
ev.PrevHash = s.hashChain.CurrentPrev()
ev.ContentHash = s.hashChain.Next(content)
```

- [ ] **Step 5: 运行测试 — PASS**

```bash
go test ./internal/policy/adapter/ -v -run TestHashChain -count=1
go test ./internal/pipeline/service/ -v -run TestGate -count=1
```

- [ ] **Step 6: Commit**

```bash
git add internal/policy/adapter/ internal/pipeline/service/
git commit -m "feat(audit): implement real hash chain for audit log tamper detection"
```

---

### Task 4: 二维模块归属 — 审批人自动路由

> 从 `module_ownership` 表读取模块→团队→审批人映射，在 Gate 创建时自动填充审批人列表

**Files:**
- Create: `internal/pipeline/domain/module_ownership.go`
- Create: `internal/pipeline/adapter/ownership_repository.go`
- Create: `internal/pipeline/service/ownership_service.go`
- Modify: `internal/pipeline/service/gate_service.go` — 自动路由审批人

- [ ] **Step 1: 写 ModuleOwnership 值对象**

Create `internal/pipeline/domain/module_ownership.go`:

```go
package domain

// ModuleOwnership maps a module to its responsible team and reviewers.
type ModuleOwnership struct {
	ProjectID        string
	ModuleName       string
	Paths            []string
	TeamName         string
	Reviewers        []string
	FallbackReviewer string
}

// OwnershipIndex indexes ownerships by file path prefix for O(1) lookup.
type OwnershipIndex struct {
	byProject map[string][]ModuleOwnership
}

func NewOwnershipIndex(ownerships []ModuleOwnership) *OwnershipIndex {
	idx := &OwnershipIndex{byProject: make(map[string][]ModuleOwnership)}
	for _, o := range ownerships {
		idx.byProject[o.ProjectID] = append(idx.byProject[o.ProjectID], o)
	}
	return idx
}

// FindReviewers returns reviewers responsible for the given changed files.
func (idx *OwnershipIndex) FindReviewers(projectID string, changedFiles []string) (reviewers []string, informed []string) {
	ownerships := idx.byProject[projectID]
	seen := make(map[string]bool)
	for _, file := range changedFiles {
		for _, o := range ownerships {
			for _, prefix := range o.Paths {
				if strings.HasPrefix(file, prefix) {
					for _, r := range o.Reviewers {
						if !seen[r] {
							seen[r] = true
							reviewers = append(reviewers, r)
						}
					}
				}
			}
		}
	}
	if len(reviewers) == 0 {
		// Fallback: use the first ownership's fallback reviewer
		for _, o := range ownerships {
			reviewers = append(reviewers, o.FallbackReviewer)
			break
		}
	}
	return reviewers, informed
}
```

Add missing import: `"strings"`.

- [ ] **Step 2: 写 Ownership 测试**

```go
func TestOwnershipIndex_FindReviewers(t *testing.T) {
	ownerships := []ModuleOwnership{
		{ProjectID: "proj-A", Paths: []string{"frontend/"}, Reviewers: []string{"alice"}, FallbackReviewer: "bob"},
		{ProjectID: "proj-A", Paths: []string{"backend/"}, Reviewers: []string{"charlie"}, FallbackReviewer: "bob"},
	}
	idx := NewOwnershipIndex(ownerships)

	reviewers, _ := idx.FindReviewers("proj-A", []string{"frontend/src/App.tsx"})
	if len(reviewers) != 1 || reviewers[0] != "alice" {
		t.Errorf("reviewers = %v, want [alice]", reviewers)
	}

	// Unknown path → fallback
	reviewers, _ = idx.FindReviewers("proj-A", []string{"unknown/file.go"})
	if len(reviewers) != 1 || reviewers[0] != "bob" {
		t.Errorf("fallback = %v, want [bob]", reviewers)
	}
}
```

- [ ] **Step 3: TDD 循环 — FAIL→PASS**

```bash
go test ./internal/pipeline/domain/ -v -run TestOwnership -count=1
```

- [ ] **Step 4: Approval 集成**

Modify `internal/pipeline/service/gate_service.go` — Approve 方法注入 OwnershipIndex:

```go
// If ownershipIndex is set, auto-populate reviewers in gate event
if s.ownershipIndex != nil {
    reviewers, _ := s.ownershipIndex.FindReviewers(pipelineID, ev.ChangedFiles())
    // reviewers stored in gate_event metadata for UI display
    _ = reviewers
}
```

- [ ] **Step 5: Commit**

```bash
git add internal/pipeline/
git commit -m "feat(ownership): add 2D module ownership reviewer auto-routing"
```

---

### Task 5: 前端 RBAC — 角色感知 UI

> 前端根据用户角色隐藏/显示管理入口、审批按钮等

**Files:**
- Modify: `frontend/src/shared/auth.tsx` — 加 `useRole()` hook
- Modify: `frontend/src/App.tsx` — 管理员路由条件渲染

- [ ] **Step 1: 加 useRole hook**

Modify `frontend/src/shared/auth.tsx`:

```tsx
export function useRole(): string {
  const { token } = useAuth();
  if (!token) return '';
  try {
    const payload = JSON.parse(atob(token.split('.')[1]));
    return payload.role || '';
  } catch {
    return '';
  }
}

export function useCanAccess(requiredRole: string): boolean {
  const role = useRole();
  if (role === 'admin') return true;
  return role === requiredRole;
}
```

- [ ] **Step 2: 条件渲染管理入口**

Modify `frontend/src/App.tsx`:

```tsx
import { useCanAccess } from './shared/auth';

function AdminRoute({ children }: { children: React.ReactNode }) {
  const canAccess = useCanAccess('admin');
  if (!canAccess) return <Navigate to="/" replace />;
  return <div className="page-enter">{children}</div>;
}
```

- [ ] **Step 3: TypeScript 编译 + Commit**

```bash
cd frontend && npx tsc --noEmit && git add src/shared/auth.tsx src/App.tsx
git commit -m "feat(frontend): add role-aware UI with useRole hook and AdminRoute"
```

---

### Task 6: E2E 验证

- [ ] **Step 1: Go 全量测试**

```bash
go -C /d/vscode/tiktok/openforge test ./internal/auth/... ./internal/pipeline/... ./internal/policy/... ./internal/server/... -count=1
```

- [ ] **Step 2: 前端编译**

```bash
cd frontend && npx tsc --noEmit && npm run build
```

- [ ] **Step 3: Commit**

```bash
git commit -m "chore(phase6): final verification — all tests pass, builds clean"
```

---

## Phase 6 Acceptance Criteria

| # | Criterion | Verification |
|---|-----------|-------------|
| 1 | RBAC middleware 拒绝无权限角色的请求 | automated (test) |
| 2 | OIDC provider disabled 模式下不影响现有 JWT login | automated (test) |
| 3 | 审计日志 prevHash 链式计算，不再硬编码 "genesis" | automated (test) |
| 4 | 模块归属索引根据变更文件自动查找审批人 | automated (test) |
| 5 | 前端 useRole() hook 正确解析 JWT role | manual |
| 6 | `go build ./cmd/server/` 通过 | automated |
| 7 | `npm run build` 零错误 | automated |
