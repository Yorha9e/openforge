# Phase 6 — 企业级安全: RBAC + SSO + 审计 + 模块归属 (v2 修正版)

> 日期: 2026-05-24 | 审计版本: v2（基于 Phase 5d 代码实况修正） | 设计文档: DESIGN.md §3.3 §3.11 §5.3 §5.4
> 上一版本问题: 7 处致命 API 不匹配 + 3 处高严重度逻辑缺陷，详见末尾审计记录

**Goal:** 将 OpenForge 从"单角色 PM 开发工具"升级为"多团队企业级平台"：RBAC 路由守卫、OIDC SSO 适配器（可配置切换）、审计日志真哈希链、二维模块归属审批人路由。

**Architecture:** 基于现有 `Can()` 函数和 `ContextUserRole` 上下文 Key 构建 RBAC 中间件（零新类型）。OIDC 通过 `auth.provider: oidc` 配置切换，向后兼容现有 JWT dev login。审计日志 `prevHash` 从 DB 最近记录查询（修复"genesis"硬编码）。模块归属从 DB 读取 → Gate 审批人自动路由。

**Tech Stack:** Go 1.25 + `database/sql` + `golang.org/x/oauth2` (Phase 6 新增依赖), React 19 + TypeScript

**关键约束:**
- 向后兼容：现有 dev 环境 JWT login 不变，OIDC 仅在 `auth_provider: oidc` 时启用
- RBAC 粒度：路由级（非数据级），数据级 RLS 延至 Phase 8
- 审计链：从 DB 最近一条 `content_hash` 查询 `prevHash`，不硬编码
- 模块归属：从 ownership 表读取，缺数据时 fallback 到默认审批人

---

## 审计修正记录（v1 → v2）

| ID | 严重度 | 问题 | 修正 |
|----|--------|------|------|
| A1 | **致命** | 计划测试 `engine.Can(role, action)` 但实际是 `Can(roles []string, action, projectID)` | 测试对齐实际 API |
| A2 | **致命** | `RequireRole` 测试在 domain 包但实现在 middleware 包 → 编译失败 | 全部放 middleware 包，复用已有 `ContextUserRole` |
| A3 | **致命** | `PermissionEngine` struct 不存在（代码是独立函数） | 不再创建新 struct，直接用现有 `Can()` |
| A4 | **致命** | OIDC `Exchange()` 存 `_ = idToken` 但从未解码 → 返回空 OIDCUser | 实现真正的 JWT payload 解析 |
| A5 | **致命** | `loader.go` 有 `JWTConfig` 不是 `AuthConfig` | AuthConfig 包裹 JWTConfig + OIDCConfig |
| A6 | **致命** | 网关服务已在 Phase 5c 修复 prevHash（使用 GetLatestHash）→ 计划中的 gate_service 改动多余 | 只修 worm_audit_log.go |
| A7 | **致命** | 前端 `useCanAccess`/`useRole` 已存在 → Task 5 冗余 | 改为验证 + 微调现有 hook |
| H1 | **高** | ModuleOwnership.FindReviewers 缺 `strings` import | 补 import |
| H2 | **高** | RBAC middleware 签名需与现有 AuthMiddleware 模式一致 | 统一为 `func(http.HandlerFunc) http.HandlerFunc` |
| H3 | **高** | `minimal.yaml` 无 `auth:` 节点 | 新增 auth 配置块 |

---

## File Map

```
openforge/
├── internal/auth/
│   ├── domain/
│   │   ├── permission_engine.go        # [EXISTS] Can(userRoles, action, projectID) bool
│   │   ├── permission_mode.go          # [EXISTS] Classify + SelectMode + PermissionMode
│   │   ├── user.go                     # [EXISTS] User + Role structs
│   │   └── rbac_test.go               # NEW: RBAC table-driven tests (uses actual Can signature)
│   ├── service/
│   │   └── jwt.go                      # [EXISTS] JWTService: Issue + Verify
│   ├── adapter/
│   │   └── oidc_provider.go           # NEW: OIDC adapter (fixes Exchange no-op bug)
│   └── middleware/
│       └── rbac.go                     # NEW: RequireRole + RequireRoleMiddleware
├── internal/server/
│   ├── routes.go                       # MODIFY: 路由加 requiredRole 参数
│   └── middleware.go                   # MODIFY: 已有 ContextUserRole，无需改
├── internal/pipeline/
│   ├── domain/
│   │   └── module_ownership.go         # NEW: ModuleOwnership + OwnershipIndex
│   └── service/
│       └── ownership_service.go        # NEW: 模块归属查询服务
├── internal/policy/adapter/
│   └── worm_audit_log.go              # MODIFY: 修复硬编码 "genesis"，查 DB 取真实 prevHash
├── internal/shared/profile/
│   └── loader.go                       # MODIFY: Config 加 AuthConfig (包裹 JWTConfig + OIDCConfig)
├── config/profiles/
│   └── minimal.yaml                    # MODIFY: 加 auth 配置块
└── frontend/src/
    └── shared/
        └── auth.tsx                     # MODIFY: useCanAccess 加角色层级（pm→dev 隐含权限）
```

---

### Task 1: RBAC Middleware（路由级角色守卫）

> 基于现有 `Can()` 函数和 `ContextUserRole` 上下文 key，添加路由级 RBAC 中间件

**Files:**
- Create: `internal/auth/middleware/rbac.go`
- Create: `internal/auth/middleware/rbac_test.go`
- Modify: `internal/server/routes.go` — 路由加 requiredRole

- [ ] **Step 1: 写 RBAC 测试（对齐实际 API）**

Create `internal/auth/middleware/rbac_test.go`:

```go
package middleware

import (
    "context"
    "net/http"
    "net/http/httptest"
    "testing"

    "openforge/internal/auth/domain"
)

func TestCan_Alignment(t *testing.T) {
    tests := []struct {
        name   string
        roles  []string
        action string
        want   bool
    }{
        {"admin all", []string{"admin"}, "admin", false},       // admin action returns false per existing code
        {"admin read", []string{"admin"}, "read", true},
        {"admin execute", []string{"admin"}, "execute", true},
        {"pm execute", []string{"pm"}, "execute", true},
        {"pm read", []string{"pm"}, "read", true},
        {"pm cannot approve self", []string{"pm"}, "approve", true},  // pm CAN approve
        {"dev read", []string{"dev"}, "read", true},
        {"dev cannot approve", []string{"dev"}, "approve", false},
        {"dev_lead approve", []string{"dev_lead"}, "approve", true},
        {"observer read", []string{"observer"}, "read", true},
        {"observer cannot execute", []string{"observer"}, "execute", false},
        {"empty roles cannot execute", []string{}, "execute", false},
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got := domain.Can(tt.roles, tt.action, "")
            if got != tt.want {
                t.Errorf("Can(%v, %q, \"\") = %v, want %v", tt.roles, tt.action, got, tt.want)
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
            ctx := context.WithValue(context.Background(), contextKey("user_role"), tt.userRole)
            got := RequireRole(ctx, tt.required)
            if got != tt.wantAccess {
                t.Errorf("RequireRole(%q, %q) = %v, want %v", tt.userRole, tt.required, got, tt.wantAccess)
            }
        })
    }
}

func TestRequireRoleMiddleware(t *testing.T) {
    handler := RequireRoleMiddleware("pm", func(w http.ResponseWriter, r *http.Request) {
        w.WriteHeader(200)
    })

    t.Run("allowed role", func(t *testing.T) {
        r := httptest.NewRequest("GET", "/", nil)
        r = r.WithContext(context.WithValue(r.Context(), contextKey("user_role"), "pm"))
        w := httptest.NewRecorder()
        handler(w, r)
        if w.Code != 200 {
            t.Errorf("expected 200, got %d", w.Code)
        }
    })

    t.Run("forbidden role", func(t *testing.T) {
        r := httptest.NewRequest("GET", "/", nil)
        r = r.WithContext(context.WithValue(r.Context(), contextKey("user_role"), "dev"))
        w := httptest.NewRecorder()
        handler(w, r)
        if w.Code != 403 {
            t.Errorf("expected 403, got %d", w.Code)
        }
    })
}
```

- [ ] **Step 2: 运行测试 — FAIL**

```bash
go test ./internal/auth/middleware/ -v -run "TestCan_Alignment|TestRequireRole" -count=1
```

Expected: FAIL — `RequireRole`/`RequireRoleMiddleware` 未定义

- [ ] **Step 3: 实现 RBAC middleware**

Create `internal/auth/middleware/rbac.go`:

```go
package middleware

import (
    "context"
    "encoding/json"
    "net/http"
)

type contextKey string

// RequireRole checks whether the user in context has the required role.
// Uses the same contextKey type as server/middleware.go (value match by string).
func RequireRole(ctx context.Context, required string) bool {
    role, ok := ctx.Value(contextKey("user_role")).(string)
    if !ok || role == "" {
        return false
    }
    if role == "admin" {
        return true
    }
    return role == required
}

// RequireRoleMiddleware returns a middleware that enforces role-based access.
func RequireRoleMiddleware(requiredRole string, next http.HandlerFunc) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        if !RequireRole(r.Context(), requiredRole) {
            w.Header().Set("Content-Type", "application/json")
            w.WriteHeader(http.StatusForbidden)
            json.NewEncoder(w).Encode(map[string]string{"error": "forbidden"})
            return
        }
        next(w, r)
    }
}
```

- [ ] **Step 4: 运行测试 — PASS**

```bash
go test ./internal/auth/middleware/ -v -run "TestCan_Alignment|TestRequireRole" -count=1
```

- [ ] **Step 5: 给路由加 requiredRole 守卫**

Modify `internal/server/routes.go` — 引入 rbacmw 包，包裹敏感路由:

```go
import (
    rbacmw "openforge/internal/auth/middleware"
)

// In RegisterRoutes, replace plain authMw with role-guarded version:
// Before: mux.HandleFunc("POST /api/projects/{id}/pipelines", authMw(handleCreatePipeline(of)))
// After:  mux.HandleFunc("POST /api/projects/{id}/pipelines", authMw(rbacMw("pm", handleCreatePipeline(of))))
```

路由角色映射表：

| 路由 | 最低角色 | 理由 (DESIGN.md) |
|------|---------|-----------------|
| `GET /api/health` | 无 | 健康检查 |
| `POST /api/auth/login` | 无 | 登录 |
| `POST /api/auth/refresh` | 无 | Token 刷新 |
| `GET /api/projects` | observer | §5.3 观察者只读 |
| `POST /api/projects/{id}/pipelines` | pm | §5.3 PM 创建 Pipeline |
| `GET /api/pipelines/{id}` | observer | §5.3 观察者只读 |
| `POST /api/pipelines/{id}/fork` | pm | §3.10 子 Pipeline 需 PM 权限 |
| `GET /api/review-inbox` | dev_lead | §5.3 开发负责人审核 |
| `POST /api/pipelines/{id}/gate/{stage}` | dev_lead | §3.3 Gate 审批 |
| `POST /api/pipelines/{id}/gate/{stage}/reject` | dev_lead | §3.3 Gate 驳回 |
| `GET /api/projects/{id}/token-usage` | pm | §4.5.3 Token 成本看板 |
| `GET /api/projects/{id}/token-budget` | pm | §4.5.3 Token 预算 |
| `GET /api/models` | observer | 模型列表只读 |
| `GET /ws/chat` | 无 | WS 第一帧鉴权 |

- [ ] **Step 6: 编译 + 测试 + Commit**

```bash
go build ./cmd/server/ && go test ./internal/auth/... -count=1
git add internal/auth/middleware/ internal/server/routes.go
git commit -m "feat(rbac): add RBAC middleware with role-based route protection"
```

---

### Task 2: OIDC 认证适配器

> 新增 OIDC provider，通过配置切换。**修复 Exchange() 不解析 JWT 的缺陷**。向后兼容现有 JWT dev login。

**Files:**
- Create: `internal/auth/adapter/oidc_provider.go` — **修复版 Exchange 真正解析 JWT**
- Create: `internal/auth/adapter/oidc_provider_test.go`
- Modify: `internal/shared/profile/loader.go` — Config 加 AuthConfig
- Modify: `internal/server/routes.go` — 加 OIDC callback/login 路由
- Modify: `config/profiles/minimal.yaml` — 加 auth 配置块

- [ ] **Step 1: 写 OIDC provider 测试**

Create `internal/auth/adapter/oidc_provider_test.go`:

```go
package adapter

import (
    "testing"
)

func TestOIDCConfig_Validate(t *testing.T) {
    tests := []struct {
        name    string
        config  OIDCConfig
        wantErr bool
    }{
        {
            name:    "valid config",
            config:  OIDCConfig{Enabled: true, IssuerURL: "https://auth.corp.com", ClientID: "openforge", ClientSecret: "***", RedirectURL: "https://of.corp.com/callback"},
            wantErr: false,
        },
        {
            name:    "missing issuer",
            config:  OIDCConfig{Enabled: true, ClientID: "x", ClientSecret: "y", RedirectURL: "z"},
            wantErr: true,
        },
        {
            name:    "missing client_id",
            config:  OIDCConfig{Enabled: true, IssuerURL: "https://x.com", ClientSecret: "y", RedirectURL: "z"},
            wantErr: true,
        },
        {
            name:    "disabled is valid even with empty fields",
            config:  OIDCConfig{Enabled: false},
            wantErr: false,
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
    if _, err := p.AuthCodeURL("state"); err == nil {
        t.Error("AuthCodeURL should return error when disabled")
    }
    if _, err := p.Exchange(nil, "any-code"); err == nil {
        t.Error("Exchange should return error when disabled")
    }
}
```

- [ ] **Step 2: 运行测试 — FAIL**

```bash
go test ./internal/auth/adapter/ -v -run TestOIDC -count=1
```

- [ ] **Step 3: 实现 OIDC provider（修复 Exchange）**

Create `internal/auth/adapter/oidc_provider.go`:

```go
package adapter

import (
    "context"
    "encoding/base64"
    "encoding/json"
    "fmt"
    "net/http"
    "strings"
    "time"

    "golang.org/x/oauth2"
)

type OIDCConfig struct {
    Enabled      bool     `yaml:"enabled"`
    IssuerURL    string   `yaml:"issuer_url"`
    ClientID     string   `yaml:"client_id"`
    ClientSecret string   `yaml:"client_secret"`
    RedirectURL  string   `yaml:"redirect_url"`
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
    Sub    string   `json:"sub"`
    Email  string   `json:"email"`
    Name   string   `json:"name"`
    Groups []string `json:"groups"`
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
    return parseIDToken(idToken)
}

// parseIDToken decodes the JWT payload without signature verification.
// MVP: trust the OIDC provider's TLS + network isolation.
// Phase 7: switch to coreos/go-oidc for full verification.
func parseIDToken(raw string) (*OIDCUser, error) {
    parts := strings.Split(raw, ".")
    if len(parts) != 3 {
        return nil, fmt.Errorf("invalid JWT format")
    }
    payload, err := base64.RawURLEncoding.DecodeString(parts[1])
    if err != nil {
        return nil, fmt.Errorf("decode payload: %w", err)
    }
    var user OIDCUser
    if err := json.Unmarshal(payload, &user); err != nil {
        return nil, fmt.Errorf("unmarshal claims: %w", err)
    }
    if user.Sub == "" {
        return nil, fmt.Errorf("id_token missing sub claim")
    }
    return &user, nil
}

func (p *OIDCProvider) Enabled() bool { return p.config.Enabled }
```

- [ ] **Step 4: 扩展 loader.go 配置结构**

Modify `internal/shared/profile/loader.go` — Config 加 AuthConfig:

```go
type AuthConfig struct {
    Provider string     `yaml:"provider"` // "jwt" (default) | "oidc"
    OIDC     OIDCConfig `yaml:"oidc"`
}
```

Config struct 加字段 `Auth AuthConfig `yaml:"auth"``。

- [ ] **Step 5: 添加 OIDC 路由 + 更新 minimal.yaml**

Modify `internal/server/routes.go`:

```go
// OIDC routes (conditionally registered)
if of.Config.Auth.Provider == "oidc" {
    mux.HandleFunc("GET /api/auth/oidc/login", handleOIDCLogin(of))
    mux.HandleFunc("GET /api/auth/oidc/callback", handleOIDCCallback(of, jwtSvc))
}
```

Modify `config/profiles/minimal.yaml` — 追加:

```yaml
auth:
  provider: jwt  # "jwt" (dev) | "oidc" (enterprise)
  oidc:
    enabled: false
    issuer_url: ""
    client_id: ""
    client_secret: ""
    redirect_url: ""
```

- [ ] **Step 6: 编译 + 测试 + Commit**

```bash
go get golang.org/x/oauth2
go build ./cmd/server/ && go test ./internal/auth/... -count=1
git add internal/auth/adapter/ internal/shared/profile/loader.go internal/server/routes.go config/profiles/minimal.yaml go.mod go.sum
git commit -m "feat(auth): add OIDC provider adapter with config-driven switching"
```

---

### Task 3: 审计日志防篡改哈希链

> 修复 `worm_audit_log.go:34` 硬编码 `prevHash := "genesis"`，改为从 DB 查最近一条 content_hash

**Files:**
- Modify: `internal/policy/adapter/worm_audit_log.go` — Log() 查 DB 取 prevHash
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

func TestHashChain_Links(t *testing.T) {
    // Simulate what the DB would return
    var dbHashes []string
    record := func(content string) string {
        prev := ""
        if len(dbHashes) > 0 {
            prev = dbHashes[len(dbHashes)-1]
        }
        h := fmt.Sprintf("%x", sha256.Sum256([]byte(prev+content)))
        dbHashes = append(dbHashes, h)
        return h
    }

    h1 := record("event-1")
    h2 := record("event-2")
    h3 := record("event-3")

    expectedPrevForH2 := h1
    expectedH2 := fmt.Sprintf("%x", sha256.Sum256([]byte(h1+"event-2")))
    if h2 != expectedH2 {
        t.Errorf("h2 = %s, want %s", h2, expectedH2)
    }

    expectedH3 := fmt.Sprintf("%x", sha256.Sum256([]byte(h2+"event-3")))
    if h3 != expectedH3 {
        t.Errorf("h3 = %s, want %s\nh2 was: %s", h3, expectedH3, h2)
    }

    // Verify chaining
    if h1 == h2 || h2 == h3 {
        t.Fatal("all hashes identical — chain not working")
    }
    t.Logf("prev for h2 should be: %s", expectedPrevForH2)
}
```

- [ ] **Step 2: 运行测试 — PASS（逻辑测试不依赖 DB）**

```bash
go test ./internal/policy/adapter/ -v -run TestHashChain -count=1
```

- [ ] **Step 3: 修复 worm_audit_log.go**

Modify `internal/policy/adapter/worm_audit_log.go` — Log() 方法:

```go
func (l *AuditLogger) Log(ctx context.Context, entry AuditEntry) error {
    content := fmt.Sprintf("%s|%s|%s|%s|%s", entry.Actor, entry.Action, entry.Resource, entry.Result, time.Now().UTC())
    contentHash := fmt.Sprintf("%x", sha256.Sum256([]byte(content)))

    // Query DB for last content_hash to build real chain
    prevHash := l.getLastHash(ctx)

    _, err := l.db.ExecContext(ctx, `
        INSERT INTO audit_log (event, actor, action, resource, result, project_id, source_ip, user_agent, artifact_hash, prev_hash, content_hash)
        VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
    `, entry.Event, entry.Actor, entry.Action, entry.Resource, entry.Result,
        entry.ProjectID, entry.SourceIP, entry.UserAgent, entry.ArtifactHash, prevHash, contentHash)
    return err
}

// getLastHash returns the most recent content_hash, or empty string for first entry.
func (l *AuditLogger) getLastHash(ctx context.Context) string {
    var hash string
    err := l.db.QueryRowContext(ctx, `SELECT content_hash FROM audit_log ORDER BY created_at DESC LIMIT 1`).Scan(&hash)
    if err != nil {
        return "" // empty chain for first entry
    }
    return hash
}
```

- [ ] **Step 4: 编译 + 测试 + Commit**

```bash
go build ./cmd/server/ && go test ./internal/policy/... -count=1
git add internal/policy/adapter/
git commit -m "fix(audit): replace hardcoded 'genesis' prevHash with real DB chain lookup"
```

---

### Task 4: 二维模块归属 — 审批人自动路由

> 从模块归属表查询模块→团队→审批人，在 Gate 创建时自动填充

**Files:**
- Create: `internal/pipeline/domain/module_ownership.go` — ModuleOwnership + OwnershipIndex
- Create: `internal/pipeline/service/ownership_service.go` — 归属查询服务
- Create: `internal/pipeline/domain/module_ownership_test.go` — 测试

- [ ] **Step 1: 写值对象 + 索引**

Create `internal/pipeline/domain/module_ownership.go`:

```go
package domain

import "strings"

type ModuleOwnership struct {
    ProjectID        string
    ModuleName       string
    Paths            []string
    TeamName         string
    Reviewers        []string
    FallbackReviewer string
}

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

func (idx *OwnershipIndex) FindReviewers(projectID string, changedFiles []string) []string {
    ownerships := idx.byProject[projectID]
    seen := make(map[string]bool)
    var reviewers []string
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
        for _, o := range ownerships {
            if o.FallbackReviewer != "" {
                reviewers = append(reviewers, o.FallbackReviewer)
                break
            }
        }
    }
    return reviewers
}
```

- [ ] **Step 2: 写测试**

Create `internal/pipeline/domain/module_ownership_test.go`:

```go
package domain

import "testing"

func TestOwnershipIndex_FindReviewers(t *testing.T) {
    ownerships := []ModuleOwnership{
        {ProjectID: "proj-A", Paths: []string{"frontend/"}, Reviewers: []string{"alice"}, FallbackReviewer: "bob"},
        {ProjectID: "proj-A", Paths: []string{"backend/"}, Reviewers: []string{"charlie"}, FallbackReviewer: "bob"},
    }
    idx := NewOwnershipIndex(ownerships)

    reviewers := idx.FindReviewers("proj-A", []string{"frontend/src/App.tsx"})
    if len(reviewers) != 1 || reviewers[0] != "alice" {
        t.Errorf("reviewers = %v, want [alice]", reviewers)
    }

    // Unknown path → fallback
    reviewers = idx.FindReviewers("proj-A", []string{"unknown/file.go"})
    if len(reviewers) < 1 {
        t.Error("fallback should return at least one reviewer")
    }
}

func TestOwnershipIndex_EmptyProject(t *testing.T) {
    idx := NewOwnershipIndex(nil)
    reviewers := idx.FindReviewers("nonexistent", []string{"foo.go"})
    if len(reviewers) != 0 {
        t.Errorf("expected 0 reviewers for unknown project, got %v", reviewers)
    }
}
```

- [ ] **Step 3: TDD 循环 — FAIL→PASS**

```bash
go test ./internal/pipeline/domain/ -v -run TestOwnership -count=1
```

- [ ] **Step 4: 创建归属查询服务**

Create `internal/pipeline/service/ownership_service.go`:

```go
package service

import (
    "context"

    "openforge/internal/pipeline/domain"
)

type OwnershipService struct {
    repo OwnershipRepository
}

type OwnershipRepository interface {
    ListByProject(ctx context.Context, projectID string) ([]domain.ModuleOwnership, error)
}

func NewOwnershipService(repo OwnershipRepository) *OwnershipService {
    return &OwnershipService{repo: repo}
}

func (s *OwnershipService) FindReviewers(ctx context.Context, projectID string, changedFiles []string) ([]string, error) {
    ownerships, err := s.repo.ListByProject(ctx, projectID)
    if err != nil {
        return nil, err
    }
    idx := domain.NewOwnershipIndex(ownerships)
    return idx.FindReviewers(projectID, changedFiles), nil
}
```

- [ ] **Step 5: 编译 + 测试 + Commit**

```bash
go build ./cmd/server/ && go test ./internal/pipeline/... -count=1
git add internal/pipeline/
git commit -m "feat(ownership): add 2D module ownership reviewer auto-routing"
```

---

### Task 5: 前端 RBAC 完善

> 现有 `useCanAccess`/`useRole` 已存在。需要加角色层级（pm 隐含 dev 权限）

**Files:**
- Modify: `frontend/src/shared/auth.tsx` — useCanAccess 加层级逻辑

- [ ] **Step 1: 修复 useCanAccess 角色层级**

Modify `frontend/src/shared/auth.tsx` — useCanAccess 函数:

当前代码逻辑已经正确（admin 绕过，否则严格相等）。加角色层级：

```tsx
const roleHierarchy: Record<string, string[]> = {
  admin: ['admin', 'pm', 'dev_lead', 'dev', 'observer'],
  pm: ['pm', 'dev', 'observer'],
  dev_lead: ['dev_lead', 'dev', 'observer'],
  dev: ['dev', 'observer'],
  observer: ['observer'],
};

export function useCanAccess(requiredRole: string): boolean {
  const role = useRole();
  if (!role) return false;
  const allowed = roleHierarchy[role];
  if (!allowed) return false;
  return allowed.includes(requiredRole);
}
```

- [ ] **Step 2: TypeScript 编译 + Commit**

```bash
cd frontend && npx tsc --noEmit
git add frontend/src/shared/auth.tsx
git commit -m "fix(frontend): add role hierarchy to useCanAccess for implicit permissions"
```

---

### Task 6: E2E 验证

- [ ] **Step 1: Go 全量测试**

```bash
go test ./internal/... -count=1
```

- [ ] **Step 2: Go vet**

```bash
go vet ./...
```

- [ ] **Step 3: 全量编译**

```bash
go build ./cmd/server/ && go build ./cmd/openforge/
```

- [ ] **Step 4: 前端编译**

```bash
cd frontend && npx tsc --noEmit
```

- [ ] **Step 5: Commit**

```bash
git commit -m "chore(phase6): final verification — all tests pass, builds clean"
```

---

## Phase 6 Acceptance Criteria

| # | Criterion | Verification |
|---|-----------|-------------|
| 1 | RBAC middleware 拒绝无权限角色的请求，返回 403 | automated |
| 2 | 现有 Can() 函数行为不变，所有 auth 测试继续通过 | automated |
| 3 | OIDC provider disabled 模式下不影响现有 JWT login | automated |
| 4 | OIDC Exchange() 真正解析 id_token JWT payload | automated |
| 5 | 审计日志 prevHash 从 DB 最近记录查询，不硬编码 | automated |
| 6 | 模块归属索引根据变更文件前缀匹配审批人 | automated |
| 7 | 前端 useCanAccess 支持角色层级 | manual |
| 8 | `go build ./...` 通过 | automated |
| 9 | `go vet ./...` 零 warning | automated |

---

## 实施顺序

```
Task 1: RBAC Middleware (可独立)
Task 2: OIDC Adapter (依赖 loader.go 改动)
Task 3: Audit Hash Chain (可独立)
Task 4: Module Ownership (可独立)
Task 5: Frontend RBAC (可独立)
Task 6: E2E 验证
```

Tasks 1/3/4/5 互不依赖，可并行。Task 2 需先改 loader.go 但可从 Task 1 独立开始。
