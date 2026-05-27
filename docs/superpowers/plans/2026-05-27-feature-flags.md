# Feature Flags — 企业功能开关 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 实现混合 Feature Flags 系统 — Profile YAML 设默认值 + DB 运行时覆盖 + Admin 面板自由开关，将 Phase 9-10 的 12 项企业功能合并为 4 个可控开关。

**Architecture:** 启动时从 Profile YAML 加载默认值写入内存，首次运行时写入 DB（若无记录），后续启动时 DB 值覆盖 YAML 默认值。Admin API（GET/PUT）允许管理员实时切换，修改后同时更新 DB 和内存，无需重启。

**Tech Stack:** Go 1.25 + PostgreSQL + React + TypeScript

---

## 🔍 Pre-implementation Review — 审查缺漏与修正

> 以下缺漏在 2026-05-27 审查中发现，**必须在实现前修正**，否则会导致运行时 bug。

| # | 级别 | 缺漏 | 修正方案 | 影响 Task |
|---|:--:|------|----------|:--:|
| **G1** | 🔴 | **并发安全缺失** — `OpenForge.FeatureFlags` 无 `sync.RWMutex`，HTTP handler 和后台 goroutine 可能同时读写 | 添加 `sync.RWMutex` 保护，或包装到带锁的 struct | Task 5 |
| **G2** | 🔴 | **Task 5 Step 1 含废弃死代码** — 第一个代码块（行452-458）含 `_ = raw` 和 "let me rewrite this" 注释 | 已删除第一个代码块，只保留第二版（已修正） | Task 5 |
| **G3** | 🔴 | **Task 9-12 引用未定义工厂函数** — `newVaultSecretStore`/`newK8sContainerRuntime` 等在 enterprise 计划才实现 | 已加注释说明，条件分支骨架先占位，函数在 enterprise 计划完成后可用 | Task 9 |
| **G4** | 🟡 | **无 Go 单元测试** — Task 2 只有 `go build`，缺少 store 测试 | 追加 Task 2.1: 创建 `store_test.go`，含 Save/Load/SeedDefaults 测试 | Task 2 |
| **G5** | 🟡 | **`SeedDefaults` 语义陷阱** — `ON CONFLICT DO NOTHING` 导致 YAML 默认值变更时不更新已存在 DB 行 | 文档化此行为：首次启动写默认值，之后不会覆盖手动修改 | Task 2 |
| **G6** | 🟡 | **Task 8 Step 4 验证场景不清晰** — 未说明如何在 standard profile 上验证 DB 覆盖 | 已补充明确步骤：先 ON → 切 OFF → 重启 → 确认仍 OFF | Task 8 |
| **G7** | 🟡 | **PUT handler 4 次独立 DB 调用，无事务** — 单次 UPSERT 更合理 | 改为单次 SQL batch UPSERT（见 Task 5 修正后代码） | Task 5 |
| **G8** | 🟢 | `AllFlags()` 和 `Clone()` 定义了但未使用 | 在 Admin UI 动态渲染 flags 列表时使用 `AllFlags()` | Task 7 |
| **G9** | 🟢 | `handleUpdateFeatureFlags` 不支持 partial update — client 需发送全部 4 字段 | 已文档化，注释说明全量更新语义 | Task 5 |

---

## File Map

```
openforge/
├── migrations/
│   ├── 006_feature_flags.up.sql           # NEW: feature_flags 表
│   └── 006_feature_flags.down.sql         # NEW: 回滚
├── internal/
│   ├── shared/
│   │   ├── featureflags/
│   │   │   ├── flags.go                   # NEW: FeatureFlags 类型定义
│   │   │   └── store.go                   # NEW: DB 读写层
│   │   └── profile/
│   │       ├── loader.go                  # MODIFY: Config 加 FeatureFlags
│   │       └── bootstrap.go               # MODIFY: OpenForge 加 FeatureFlags, 初始化逻辑
│   └── server/
│       ├── routes.go                      # MODIFY: 注册 admin feature-flags 路由
│       └── admin_feature_flags.go         # NEW: GET/PUT handler
├── config/profiles/
│   ├── minimal.yaml                       # MODIFY: 加 feature_flags 节
│   └── standard.yaml                      # MODIFY: 加 feature_flags 节
└── frontend/src/
    ├── shared/api.ts                      # MODIFY: 加 getFeatureFlags / updateFeatureFlags
    └── features/admin/AdminPage.tsx       # MODIFY: Phase 9-10 卡片改为可交互开关
```

---

### Task 1: 数据库 Migration — feature_flags 表

**Files:**
- Create: `migrations/006_feature_flags.up.sql`
- Create: `migrations/006_feature_flags.down.sql`

- [ ] **Step 1: 创建 up migration**

Create `migrations/006_feature_flags.up.sql`:

```sql
-- Feature flags: Admin-togglable enterprise capabilities.
-- Defaults are set by profile YAML; DB rows override on startup and at runtime.

CREATE TABLE IF NOT EXISTS feature_flags (
    flag_key    TEXT        PRIMARY KEY,
    enabled     BOOLEAN     NOT NULL DEFAULT false,
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

COMMENT ON TABLE feature_flags IS 'Runtime-overridable feature toggles for enterprise capabilities';
COMMENT ON COLUMN feature_flags.flag_key IS 'Flag identifier: enterprise_platform, compliance_suite, production_ops, distribution_artifacts';
COMMENT ON COLUMN feature_flags.enabled IS 'Whether the feature group is currently active';
```

- [ ] **Step 2: 创建 down migration**

Create `migrations/006_feature_flags.down.sql`:

```sql
DROP TABLE IF EXISTS feature_flags;
```

- [ ] **Step 3: Commit**

```bash
git add migrations/006_feature_flags.up.sql migrations/006_feature_flags.down.sql
git commit -m "feat(feature-flags): add feature_flags table migration

- flag_key (PK) + enabled + updated_at
- Runtime-overridable enterprise capability toggles
"
```

---

### Task 2: FeatureFlags 类型定义 + DB Store

**Files:**
- Create: `internal/shared/featureflags/flags.go`
- Create: `internal/shared/featureflags/store.go`

- [ ] **Step 1: 定义 FeatureFlags 类型**

Create `internal/shared/featureflags/flags.go`:

```go
package featureflags

import "sync"

// FeatureFlags groups enterprise capabilities into 4 toggleable switches.
// Each flag controls a cluster of related Phase 9-10 features.
// G1 FIX: embedded sync.RWMutex for HTTP handler + goroutine concurrency safety.
type FeatureFlags struct {
	mu                     sync.RWMutex
	EnterprisePlatform     bool `json:"enterprise_platform" yaml:"enterprise_platform"`
	ComplianceSuite        bool `json:"compliance_suite" yaml:"compliance_suite"`
	ProductionOps          bool `json:"production_ops" yaml:"production_ops"`
	DistributionArtifacts  bool `json:"distribution_artifacts" yaml:"distribution_artifacts"`
}

// Lock/Unlock/RLock/RUnlock delegates for external use (e.g. PUT handler).
func (f *FeatureFlags) Lock()    { f.mu.Lock() }
func (f *FeatureFlags) Unlock()  { f.mu.Unlock() }
func (f *FeatureFlags) RLock()   { f.mu.RLock() }
func (f *FeatureFlags) RUnlock() { f.mu.RUnlock() }

// Defaults returns the hardcoded zero-value defaults (all false).
// Profile YAML values override these at bootstrap time.
func Defaults() *FeatureFlags {
	return &FeatureFlags{}
}

// Clone returns a deep copy.
func (f *FeatureFlags) Clone() *FeatureFlags {
	c := *f
	return &c
}

// AllFlags returns the 4 flag keys in canonical order.
func AllFlags() []string {
	return []string{
		"enterprise_platform",
		"compliance_suite",
		"production_ops",
		"distribution_artifacts",
	}
}
```

- [ ] **Step 2: 实现 DB Store**

Create `internal/shared/featureflags/store.go`:

```go
package featureflags

import (
	"context"
	"database/sql"
	"fmt"
)

// Store persists and retrieves feature flag overrides from the feature_flags table.
type Store struct {
	db *sql.DB
}

func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

// Load reads all flag rows from the DB and returns a FeatureFlags struct.
// Flags not present in the DB are left at their zero value (caller should merge
// with YAML defaults).
func (s *Store) Load(ctx context.Context) (*FeatureFlags, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT flag_key, enabled FROM feature_flags`)
	if err != nil {
		return nil, fmt.Errorf("featureflags load: %w", err)
	}
	defer rows.Close()

	result := Defaults()
	for rows.Next() {
		var key string
		var enabled bool
		if err := rows.Scan(&key, &enabled); err != nil {
			return nil, fmt.Errorf("featureflags scan: %w", err)
		}
		switch key {
		case "enterprise_platform":
			result.EnterprisePlatform = enabled
		case "compliance_suite":
			result.ComplianceSuite = enabled
		case "production_ops":
			result.ProductionOps = enabled
		case "distribution_artifacts":
			result.DistributionArtifacts = enabled
		}
	}
	return result, rows.Err()
}

// Save upserts a single flag value into the DB.
func (s *Store) Save(ctx context.Context, key string, enabled bool) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO feature_flags (flag_key, enabled, updated_at)
		 VALUES ($1, $2, NOW())
		 ON CONFLICT (flag_key) DO UPDATE SET enabled = $2, updated_at = NOW()`,
		key, enabled)
	if err != nil {
		return fmt.Errorf("featureflags save %s=%v: %w", key, enabled, err)
	}
	return nil
}

// SaveAll persists all 4 flags in a single transaction (G7: avoids partial-update).
func (s *Store) SaveAll(ctx context.Context, f *FeatureFlags) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("featureflags SaveAll begin: %w", err)
	}
	defer tx.Rollback()

	entries := map[string]bool{
		"enterprise_platform":    f.EnterprisePlatform,
		"compliance_suite":        f.ComplianceSuite,
		"production_ops":          f.ProductionOps,
		"distribution_artifacts":  f.DistributionArtifacts,
	}
	for key, enabled := range entries {
		_, err := tx.ExecContext(ctx,
			`INSERT INTO feature_flags (flag_key, enabled, updated_at)
			 VALUES ($1, $2, NOW())
			 ON CONFLICT (flag_key) DO UPDATE SET enabled = $2, updated_at = NOW()`,
			key, enabled)
		if err != nil {
			return fmt.Errorf("featureflags SaveAll %s: %w", key, err)
		}
	}
	return tx.Commit()
}

// SeedDefaults writes the YAML-default flags to the DB (idempotent —
// uses ON CONFLICT DO NOTHING so existing user overrides are preserved).
func (s *Store) SeedDefaults(ctx context.Context, defaults *FeatureFlags) error {
	entries := map[string]bool{
		"enterprise_platform":    defaults.EnterprisePlatform,
		"compliance_suite":        defaults.ComplianceSuite,
		"production_ops":          defaults.ProductionOps,
		"distribution_artifacts":  defaults.DistributionArtifacts,
	}
	for key, enabled := range entries {
		_, err := s.db.ExecContext(ctx,
			`INSERT INTO feature_flags (flag_key, enabled, updated_at)
			 VALUES ($1, $2, NOW())
			 ON CONFLICT (flag_key) DO NOTHING`,
			key, enabled)
		if err != nil {
			return fmt.Errorf("featureflags seed %s: %w", key, err)
		}
	}
	return nil
}
```

- [ ] **Step 3: Build 验证**

```bash
go build ./internal/shared/featureflags/...
```
Expected: 编译通过（无 main 函数无法生成可执行文件属正常警告）

- [ ] **Step 4: Commit**

```bash
git add internal/shared/featureflags/flags.go internal/shared/featureflags/store.go
git commit -m "feat(feature-flags): add FeatureFlags type + DB store

- FeatureFlags struct: 4 enterprise capability groups with sync.RWMutex
- Store: Load/Save/SaveAll/SeedDefaults for feature_flags table
- SeedDefaults is idempotent (ON CONFLICT DO NOTHING)
"
```

---

### Task 2.1: Store 单元测试 (G4 — 追加)

> **原因：** 审查发现 Task 2 只有 `go build`，缺少 DB 层单元测试。

**Files:**
- Create: `internal/shared/featureflags/store_test.go`

- [ ] **Step 1: 创建测试文件**

Create `internal/shared/featureflags/store_test.go`:

```go
package featureflags

import (
	"context"
	"database/sql"
	"os"
	"testing"
	_ "github.com/lib/pq"
)

func testDB(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("TEST_DSN")
	if dsn == "" {
		dsn = "host=localhost port=5432 user=openforge password=openforge dbname=openforge sslmode=disable"
	}
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Skipf("db open failed: %v", err)
	}
	if err := db.Ping(); err != nil {
		t.Skipf("db unreachable: %v", err)
	}
	db.Exec("DELETE FROM feature_flags")
	return db
}

func TestStore_SaveAndLoad(t *testing.T) {
	db := testDB(t)
	store := NewStore(db)
	ctx := context.Background()

	if err := store.Save(ctx, "enterprise_platform", true); err != nil {
		t.Fatalf("save: %v", err)
	}
	flags, err := store.Load(ctx)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if !flags.EnterprisePlatform {
		t.Error("EnterprisePlatform should be true")
	}
	if flags.ComplianceSuite {
		t.Error("unsaved flag should be false")
	}
}

func TestStore_SeedDefaults_Idempotent(t *testing.T) {
	db := testDB(t)
	store := NewStore(db)
	ctx := context.Background()

	defaults := &FeatureFlags{ComplianceSuite: true, ProductionOps: true}
	if err := store.SeedDefaults(ctx, defaults); err != nil {
		t.Fatalf("seed1: %v", err)
	}
	// Second seed with different values should NOT overwrite (ON CONFLICT DO NOTHING).
	if err := store.SeedDefaults(ctx, &FeatureFlags{}); err != nil {
		t.Fatalf("seed2: %v", err)
	}
	flags, _ := store.Load(ctx)
	if !flags.ComplianceSuite || !flags.ProductionOps {
		t.Error("SeedDefaults overwrote existing — idempotency broken")
	}
}

func TestStore_SaveAll_Transactional(t *testing.T) {
	db := testDB(t)
	store := NewStore(db)
	ctx := context.Background()

	f := &FeatureFlags{
		EnterprisePlatform: true, ComplianceSuite: true,
		ProductionOps: false, DistributionArtifacts: false,
	}
	if err := store.SaveAll(ctx, f); err != nil {
		t.Fatalf("SaveAll: %v", err)
	}
	flags, _ := store.Load(ctx)
	if !flags.EnterprisePlatform || !flags.ComplianceSuite {
		t.Error("SaveAll did not persist all flags")
	}
}

func TestStore_Save_Overwrite(t *testing.T) {
	db := testDB(t)
	store := NewStore(db)
	ctx := context.Background()

	store.Save(ctx, "distribution_artifacts", true)
	store.Save(ctx, "distribution_artifacts", false)
	flags, _ := store.Load(ctx)
	if flags.DistributionArtifacts {
		t.Error("overwrite should set to false")
	}
}
```

- [ ] **Step 2: 运行测试**

```bash
go test ./internal/shared/featureflags/ -v -count=1
```

Expected: 4 tests PASS

- [ ] **Step 3: Commit**

```bash
git add internal/shared/featureflags/store_test.go
git commit -m "test(feature-flags): add Store unit tests — Save/Load/SaveAll/SeedDefaults"
```

---

### Task 3: Profile Config 扩展 — YAML 默认值

**Files:**
- Modify: `internal/shared/profile/loader.go`
- Modify: `config/profiles/minimal.yaml`
- Modify: `config/profiles/standard.yaml`

- [ ] **Step 1: Config 结构体加 FeatureFlags 字段**

Modify `internal/shared/profile/loader.go` — 在 `Config` struct 中添加字段（在 `CommandExecutor` 字段之后，`Database` 字段之前）：

```go
	CommandExecutor  string `yaml:"command_executor"`

	// FeatureFlags: YAML-level defaults for enterprise capability toggles.
	// Runtime overrides are stored in the feature_flags DB table.
	FeatureFlags FeatureFlagsConfig `yaml:"feature_flags"`

	Database DatabaseConfig `yaml:"database"`
```

并在文件顶部添加 import 和类型定义。在 `Config` struct 之后、`AuthConfig` 之前添加：

```go
// FeatureFlagsConfig groups the YAML-level defaults for feature toggles.
type FeatureFlagsConfig struct {
	EnterprisePlatform    bool `yaml:"enterprise_platform"`
	ComplianceSuite       bool `yaml:"compliance_suite"`
	ProductionOps         bool `yaml:"production_ops"`
	DistributionArtifacts bool `yaml:"distribution_artifacts"`
}
```

- [ ] **Step 2: 更新 minimal.yaml**

Modify `config/profiles/minimal.yaml` — 在 `command_executor: local-shell` 和 `dependency_cache: none` 之间添加：

```yaml
command_executor: local-shell
dependency_cache: none  # minimal 无共享缓存, 每 sandbox 独立下载

feature_flags:
  enterprise_platform: false
  compliance_suite: false
  production_ops: false
  distribution_artifacts: false
```

- [ ] **Step 3: 更新 standard.yaml**

Modify `config/profiles/standard.yaml` — 在 `command_executor: docker-sandbox` 和 `docker:` 之间添加：

```yaml
command_executor: docker-sandbox

feature_flags:
  enterprise_platform: false
  compliance_suite: true
  production_ops: true
  distribution_artifacts: false
```

- [ ] **Step 4: Build 验证**

```bash
go build ./...
```
Expected: 编译通过

- [ ] **Step 5: Commit**

```bash
git add internal/shared/profile/loader.go config/profiles/minimal.yaml config/profiles/standard.yaml
git commit -m "feat(feature-flags): add FeatureFlagsConfig to profile YAML

- Config.FeatureFlags YAML binding
- minimal: all false (personal/dev mode)
- standard: compliance_suite=true, production_ops=true, rest false
"
```

---

### Task 4: Bootstrap 集成 — 初始化 FeatureFlags

**Files:**
- Modify: `internal/shared/profile/bootstrap.go`

- [ ] **Step 1: OpenForge 结构体加 FeatureFlags 字段**

Modify `internal/shared/profile/bootstrap.go` — 在 `OpenForge` struct 中，`Config *Config` 之后添加：

```go
	Config          *Config
	FeatureFlags    *featureflags.FeatureFlags   // Phase 10: runtime-overridable enterprise toggles
	PromptBuilder   *domain.PromptBuilder
```

并在文件顶部 import 区域添加：

```go
	"openforge/internal/shared/featureflags"
```

- [ ] **Step 2: Bootstrap 函数中初始化 FeatureFlags**

Modify `internal/shared/profile/bootstrap.go` — 在 `Bootstrap` 函数中 `of.DB = db` 之后、`of.PipelineRepo = ...` 之前，添加初始化逻辑：

```go
	of.DB = db

	// Feature flags: load YAML defaults → seed DB → DB override → memory.
	ffStore := featureflags.NewStore(db)
	yamlDefaults := &featureflags.FeatureFlags{
		EnterprisePlatform:    cfg.FeatureFlags.EnterprisePlatform,
		ComplianceSuite:       cfg.FeatureFlags.ComplianceSuite,
		ProductionOps:         cfg.FeatureFlags.ProductionOps,
		DistributionArtifacts: cfg.FeatureFlags.DistributionArtifacts,
	}
	// Seed DB with YAML defaults (idempotent — preserves existing overrides).
	if err := ffStore.SeedDefaults(context.Background(), yamlDefaults); err != nil {
		return nil, fmt.Errorf("featureflags seed: %w", err)
	}
	// Load from DB (which may have runtime overrides taking priority).
	ff, err := ffStore.Load(context.Background())
	if err != nil {
		return nil, fmt.Errorf("featureflags load: %w", err)
	}
	of.FeatureFlags = ff

	of.PipelineRepo = pipelineadapter.NewPGRepository(db)
```

- [ ] **Step 3: Build 验证**

```bash
go build ./...
```
Expected: 编译通过

- [ ] **Step 4: 启动验证**

```bash
go run ./cmd/server/ --config config/profiles/minimal.yaml
```
Expected: 正常启动，日志中无明显错误

- [ ] **Step 5: Commit**

```bash
git add internal/shared/profile/bootstrap.go
git commit -m "feat(feature-flags): wire FeatureFlags into Bootstrap

- OpenForge.FeatureFlags field
- Load path: YAML defaults → SeedDefaults (idempotent) → DB Load (override)
- DB values take priority over YAML at runtime
"
```

---

### Task 5: Admin API — GET/PUT feature-flags 端点

**Files:**
- Create: `internal/server/admin_feature_flags.go`
- Modify: `internal/server/routes.go`

- [ ] **Step 1: 实现 GET 和 PUT handler**

Create `internal/server/admin_feature_flags.go`:

```go
package server

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"openforge/internal/shared/featureflags"
	"openforge/internal/shared/profile"
)

// handleGetFeatureFlags returns the current feature flag state.
func handleGetFeatureFlags(of *profile.OpenForge) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, of.FeatureFlags)
	}
}

// handleUpdateFeatureFlags accepts a full FeatureFlags JSON body and persists
// all 4 flags to the DB in a single transaction, then syncs the in-memory state.
// REVISED (G1+G7): Added sync.RWMutex for concurrency safety + batch UPSERT in one tx.
func handleUpdateFeatureFlags(of *profile.OpenForge, store *featureflags.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req featureflags.FeatureFlags
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, 400, "invalid request body")
			return
		}

		// Persist all 4 flags in a single DB transaction.
		if err := store.SaveAll(r.Context(), &req); err != nil {
			slog.Error("featureflags save failed", "error", err)
			writeError(w, 500, sanitizeError(err))
			return
		}

		// Sync in-memory state with write lock (G1: concurrency safe).
		of.FeatureFlags.Lock()
		of.FeatureFlags.EnterprisePlatform = req.EnterprisePlatform
		of.FeatureFlags.ComplianceSuite = req.ComplianceSuite
		of.FeatureFlags.ProductionOps = req.ProductionOps
		of.FeatureFlags.DistributionArtifacts = req.DistributionArtifacts
		of.FeatureFlags.Unlock()

		writeJSON(w, 200, of.FeatureFlags)
	}
}
```

- [ ] **Step 2: 在 routes.go 中注册路由**

Modify `internal/server/routes.go` — 在 `// Admin status (admin)` 行之后添加：

```go
	// Admin: Feature flags (admin-only, runtime toggles without restart)
	ffStore := featureflags.NewStore(of.DB)
	mux.HandleFunc("GET /api/admin/feature-flags", withAdmin(handleGetFeatureFlags(of)))
	mux.HandleFunc("PUT /api/admin/feature-flags", withAdmin(handleUpdateFeatureFlags(of, ffStore)))
```

并在文件顶部 import 区域添加：
```go
	"openforge/internal/shared/featureflags"
```

- [ ] **Step 3: Build 验证**

```bash
go build ./...
```
Expected: 编译通过

- [ ] **Step 4: API 测试**

启动服务器后：

```bash
# GET current flags
curl -s http://localhost:8030/api/admin/feature-flags \
  -H "Authorization: Bearer <admin-token>" | jq .

# PUT update
curl -s -X PUT http://localhost:8030/api/admin/feature-flags \
  -H "Authorization: Bearer <admin-token>" \
  -H "Content-Type: application/json" \
  -d '{"enterprise_platform":false,"compliance_suite":true,"production_ops":true,"distribution_artifacts":false}' | jq .
```
Expected: GET 返回 4 个 bool 字段；PUT 返回更新后的状态

- [ ] **Step 5: Commit**

```bash
git add internal/server/admin_feature_flags.go internal/server/routes.go
git commit -m "feat(feature-flags): add GET/PUT /api/admin/feature-flags endpoints

- GET: returns current in-memory FeatureFlags
- PUT: persists all 4 flags to DB + syncs memory immediately
- Admin-only (withAdmin wrapper)
"
```

---

### Task 6: 前端 — API 客户端扩展

**Files:**
- Modify: `frontend/src/shared/api.ts`

- [ ] **Step 1: 添加 FeatureFlags 类型和方法**

Modify `frontend/src/shared/api.ts` — 在 `AdminStatus` 类型定义之后、`export const api` 之前添加：

```typescript
export type FeatureFlags = {
  enterprise_platform: boolean;
  compliance_suite: boolean;
  production_ops: boolean;
  distribution_artifacts: boolean;
};
```

在 `export const api = {` 块中，`getAdminStatus` 之后添加：

```typescript
  // Feature Flags
  getFeatureFlags: () => request<FeatureFlags>('/admin/feature-flags'),
  updateFeatureFlags: (flags: FeatureFlags) =>
    request<FeatureFlags>('/admin/feature-flags', {
      method: 'PUT',
      body: JSON.stringify(flags),
    }),
```

- [ ] **Step 2: TypeScript 编译验证**

```bash
cd frontend && npx tsc --noEmit
```
Expected: 无类型错误

- [ ] **Step 3: Commit**

```bash
git add frontend/src/shared/api.ts
git commit -m "feat(feature-flags): add FeatureFlags type and API methods to frontend

- FeatureFlags type: 4 boolean fields
- getFeatureFlags / updateFeatureFlags api methods
"
```

---

### Task 7: 前端 — Admin 面板开关 UI

**Files:**
- Modify: `frontend/src/features/admin/AdminPage.tsx`

- [ ] **Step 1: 添加 FeatureFlags 状态和加载逻辑**

Modify `frontend/src/features/admin/AdminPage.tsx` — 在现有 import 后添加类型导入：

```typescript
import { FeatureFlags } from '../../shared/api';
```

在组件函数顶部，`const [showSkillPanel, setShowSkillPanel] = useState(false);` 之后添加：

```typescript
  const [featureFlags, setFeatureFlags] = useState<FeatureFlags | null>(null);
  const [flagsLoading, setFlagsLoading] = useState(false);
```

在 `useEffect` 块中（`api.getAdminStatus()` 调用之后），添加：

```typescript
    api.getFeatureFlags()
      .then(ff => setFeatureFlags(ff))
      .catch(() => {}); // flags unavailable — show nothing
```

- [ ] **Step 2: 添加切换处理函数**

在组件函数内部、`const loginInfo = ...` 之前添加：

```typescript
  const handleToggleFlag = async (key: keyof FeatureFlags) => {
    if (!featureFlags) return;
    const updated = { ...featureFlags, [key]: !featureFlags[key] };
    setFlagsLoading(true);
    try {
      const result = await api.updateFeatureFlags(updated);
      setFeatureFlags(result);
    } catch {
      // revert on failure
    } finally {
      setFlagsLoading(false);
    }
  };
```

- [ ] **Step 3: 改造 Phase 9-10 卡片为开关**

将 `{/* Phase 9-10 Planned Features */}` Section 中的 12 张静态卡片替换为 4 组带开关的卡片。

定位到 `{/* Phase 9-10 Planned Features */}` 注释行（约第 263 行），将其后的整个 Section（从 `<Section>` 开始到对应 `</Section>` 结束）替换为：

```typescript
      {/* Phase 9-10 Feature Toggles */}
      <Section title="Phase 9-10 — Feature Toggles">
        {featureFlags === null ? (
          <div style={{ textAlign: 'center', padding: 24, color: tokens.muted, fontSize: 14 }}>
            Feature flags unavailable — check admin access
          </div>
        ) : (
          <div style={{ display: 'flex', flexDirection: 'column', gap: 12 }}>
            {[
              {
                key: 'enterprise_platform' as keyof FeatureFlags,
                title: 'Enterprise Platform',
                desc: 'Enterprise-grade infrastructure stack',
                items: ['Vault Secret Store', 'K8s Container Runtime', 'MinIO Object Store', 'Multi-Region DR', 'K8s Helm Charts'],
                color: '#7C3AED',
              },
              {
                key: 'compliance_suite' as keyof FeatureFlags,
                title: 'Compliance Suite',
                desc: 'Regulatory compliance & data governance',
                items: ['Monthly Compliance Reports', 'Data Lifecycle Manager (90d/365d/7yr)'],
                color: '#0891B2',
              },
              {
                key: 'production_ops' as keyof FeatureFlags,
                title: 'Production Operations',
                desc: 'Monitoring, alerting & operational runbooks',
                items: ['Grafana Dashboards', 'Semi-automated Runbooks', 'Multi-Channel Notifier (Feishu/DingTalk)'],
                color: '#F59E0B',
              },
              {
                key: 'distribution_artifacts' as keyof FeatureFlags,
                title: 'Distribution & Docs',
                desc: 'Offline deployment & architecture documentation',
                items: ['Offline Deployment Package', 'ADR + OpenAPI Contract'],
                color: '#10B981',
              },
            ].map(group => (
              <div key={group.key} style={{
                background: tokens.surface, borderRadius: 10, padding: '16px 20px',
                border: `1px solid ${featureFlags[group.key] ? group.color : tokens.border}`,
                display: 'flex', alignItems: 'flex-start', gap: 16,
                opacity: flagsLoading ? 0.5 : 1,
                transition: 'border-color 0.2s, opacity 0.2s',
              }}>
                {/* Toggle switch */}
                <button
                  onClick={() => handleToggleFlag(group.key)}
                  disabled={flagsLoading}
                  style={{
                    width: 44, height: 24, borderRadius: 12, border: 'none',
                    cursor: flagsLoading ? 'not-allowed' : 'pointer',
                    backgroundColor: featureFlags[group.key] ? group.color : '#334155',
                    position: 'relative', flexShrink: 0, marginTop: 4,
                    transition: 'background-color 0.2s',
                  }}
                >
                  <div style={{
                    width: 18, height: 18, borderRadius: '50%',
                    backgroundColor: '#fff',
                    position: 'absolute', top: 3,
                    left: featureFlags[group.key] ? 23 : 3,
                    transition: 'left 0.2s',
                  }} />
                </button>

                {/* Content */}
                <div style={{ flex: 1 }}>
                  <div style={{ display: 'flex', alignItems: 'center', gap: 8, marginBottom: 4 }}>
                    <div style={{ fontSize: 14, fontWeight: 700, color: '#F1F5F9' }}>{group.title}</div>
                    <span style={{
                      fontSize: 10, fontWeight: 600, padding: '1px 7px', borderRadius: 4,
                      color: featureFlags[group.key] ? group.color : '#64748B',
                      background: featureFlags[group.key] ? `${group.color}18` : '#334155',
                    }}>
                      {featureFlags[group.key] ? 'ON' : 'OFF'}
                    </span>
                  </div>
                  <div style={{ fontSize: 12, color: '#94A3B8', marginBottom: 8 }}>{group.desc}</div>
                  <div style={{ display: 'flex', flexWrap: 'wrap', gap: 6 }}>
                    {group.items.map(item => (
                      <span key={item} style={{
                        fontSize: 11, color: '#64748B',
                        background: '#1E293B', padding: '2px 8px', borderRadius: 4,
                      }}>
                        {item}
                      </span>
                    ))}
                  </div>
                </div>
              </div>
            ))}
          </div>
        )}
      </Section>
```

- [ ] **Step 4: TypeScript + Vite 编译验证**

```bash
cd frontend && npx tsc --noEmit && npx vite build
```
Expected: 无类型错误，build 成功

- [ ] **Step 5: Commit**

```bash
git add frontend/src/features/admin/AdminPage.tsx
git commit -m "feat(feature-flags): replace Phase 9-10 static cards with interactive toggle switches

- 4 toggle groups replace 12 static cards
- Real-time DB sync via PUT /api/admin/feature-flags
- Color-coded ON/OFF state with animated toggle
- Loading/error states handled
"
```

---

### Task 8: 端到端验证 + 收尾

- [ ] **Step 1: 全量编译**

```bash
go build ./cmd/server/ && go build ./cmd/openforge/
go vet ./...
```

Expected: 全部通过

- [ ] **Step 2: 启动 minimal 模式验证**

```bash
go run ./cmd/server/ --config config/profiles/minimal.yaml
```

然后访问 Admin 面板，确认 4 个开关全部显示为 OFF。

- [ ] **Step 3: 启动 standard 模式验证**

```bash
go run ./cmd/server/ --config config/profiles/standard.yaml
```

确认 `compliance_suite` 和 `production_ops` 为 ON，其余为 OFF。

- [ ] **Step 4: 切换开关 + 重启验证 (G6 — 已明确)**

1. 以 `standard.yaml` 启动 → `compliance_suite` 初始为 ON（YAML 默认值）
2. 在 Admin 面板中将 `compliance_suite` 改为 **OFF**
3. 重启服务器（仍用 `standard.yaml`）
4. 确认 `compliance_suite` 仍为 **OFF** — DB 值覆盖了 YAML 默认值（standard.yaml 写的是 `true`，但 DB 中的 `false` 优先）
5. 再将 `compliance_suite` 改回 ON → 重启 → 确认仍为 ON

- [ ] **Step 5: DB 直接查询验证**

```bash
psql -h localhost -U openforge -d openforge -c "SELECT * FROM feature_flags;"
```

Expected: 4 rows，对应 4 个 flag

- [ ] **Step 6: Final commit**

```bash
git add -A
git commit -m "feat(feature-flags): final verification — all toggles working end-to-end

- DB migration: 006_feature_flags
- Profile YAML: minimal (all false), standard (compliance+ops=true)
- Bootstrap: YAML → SeedDefaults → DB override
- Admin API: GET/PUT /api/admin/feature-flags (admin-only)
- Frontend: 4 interactive toggle groups in Admin panel
- Restart persistence: DB values survive restart
"
```

---

## Acceptance Criteria

| # | Criterion | Verification |
|---|-----------|-------------|
| 1 | `feature_flags` 表创建成功 | `\d feature_flags` in psql |
| 2 | minimal.yaml 启动后 4 个 flag 全为 false | GET /api/admin/feature-flags |
| 3 | standard.yaml 启动后 compliance_suite + production_ops 为 true | GET /api/admin/feature-flags |
| 4 | Admin 面板切换后 DB 即时更新 | `SELECT * FROM feature_flags` |
| 5 | 切换后不重启立即生效（内存同步） | 两次 GET 之间 PUT 后立即返回新值 |
| 6 | 重启后 DB 值优先于 YAML 默认值 | 改 flag → 重启 → 验证值保留 |
| 7 | 非 admin 用户访问返回 403 | curl without admin token |
| 8 | `go build ./...` + `go vet ./...` 通过 | automated |
| 9 | `npx tsc --noEmit` + `npx vite build` 通过 | automated |

---

---

## Phase 2: Feature Gates 接入 — 开关接线

> **目标：** 让 Admin 面板的 4 个开关真正生效，根据 flag 状态切换内核实现和路由注册。

---

### Task 9: `enterprise_platform` Gate 接入 — 先 noop 后替换

**Files:**
- Modify: `internal/shared/profile/bootstrap.go`

**设计原则：** 内核接口初始化发生在 DB 连接之前，因此采用"先 noop，后替换"模式。启动时所有接口用 noop stub 初始化，FeatureFlags 加载完成后，若 flag 为 ON，替换为真实 enterprise 实现。如果 DB 挂了，系统以 noop 模式正常运行，不会崩溃。

- [ ] **Step 1: 在 Bootstrap 中添加 enterprise 实现替换逻辑**

Modify `internal/shared/profile/bootstrap.go` — 在 `of.FeatureFlags = ff` 行之后、`of.PipelineRepo = ...` 之前，添加：

```go
	of.FeatureFlags = ff

	// --- Feature Gates: enterprise_platform ---
	// Replace noop stubs with real enterprise implementations when flag is ON.
	if ff.EnterprisePlatform {
		of.Secrets = newVaultSecretStore(cfg)
		of.Container = newK8sContainerRuntime(cfg)
		of.Object = newMinioObjectStore(cfg)
		of.DR = newMultiRegionDR(cfg)
		of.LB = newK8sIngressLB(cfg)
	}

	of.PipelineRepo = pipelineadapter.NewPGRepository(db)
```

**注意：** `newVaultSecretStore`、`newK8sContainerRuntime`、`newMinioObjectStore`、`newMultiRegionDR`、`newK8sIngressLB` 这 5 个函数在 enterprise 实现完成后才可用。当前阶段先添加条件分支骨架，函数调用在对应 Task 中实现。

- [ ] **Step 2: 添加条件路由注册**

Modify `internal/server/routes.go` — 在路由注册区域，根据 flag 条件注册企业级端点：

```go
	// Enterprise platform routes (conditionally registered)
	if of.FeatureFlags.EnterprisePlatform {
		mux.HandleFunc("GET /api/vault/status", withAdmin(handleVaultStatus(of)))
		mux.HandleFunc("GET /api/k8s/status", withAdmin(handleK8sStatus(of)))
		mux.HandleFunc("GET /api/storage/status", withAdmin(handleStorageStatus(of)))
	}
```

- [ ] **Step 3: Build 验证**

```bash
go build ./...
```
Expected: 编译通过（函数引用可能临时报 undefined，等待 enterprise 实现）

- [ ] **Step 4: Commit**

```bash
git add internal/shared/profile/bootstrap.go internal/server/routes.go
git commit -m "feat(feature-gates): wire enterprise_platform flag — noop→enterprise swap

- Bootstrap: replace 5 noop stubs with real enterprise implementations when flag ON
- Routes: conditionally register /api/vault/status, /api/k8s/status, /api/storage/status
- Safe default: DB failure keeps noop stubs, system stays functional
"
```

---

### Task 10: `compliance_suite` Gate 接入 — 条件注册

**Files:**
- Create: `internal/server/admin_audit.go`
- Modify: `internal/server/routes.go`
- Create: `internal/compliance/data_lifecycle.go`
- Modify: `internal/shared/profile/bootstrap.go`
- Modify: `frontend/src/App.tsx`
- Create: `frontend/src/features/compliance/ComplianceReportPage.tsx`

**设计原则：** 纯增量，不需要替换现有实现。三个接入点均为条件注册：审计导出 API、数据生命周期 cron、合规报告前端路由。

- [ ] **Step 1: 实现审计导出 API handler**

Create `internal/server/admin_audit.go`:

```go
package server

import (
	"database/sql"
	"encoding/csv"
	"net/http"
	"time"

	"openforge/internal/shared/profile"
)

// handleAuditExport exports the audit_log table as CSV.
func handleAuditExport(of *profile.OpenForge) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		rows, err := of.DB.QueryContext(r.Context(),
			`SELECT event, actor, action, resource, result, project_id, created_at
			 FROM audit_log ORDER BY created_at DESC LIMIT 10000`)
		if err != nil {
			writeError(w, 500, sanitizeError(err))
			return
		}
		defer rows.Close()

		w.Header().Set("Content-Type", "text/csv")
		w.Header().Set("Content-Disposition",
			"attachment; filename=audit_export_"+time.Now().Format("20060102")+".csv")

		writer := csv.NewWriter(w)
		writer.Write([]string{"event", "actor", "action", "resource", "result", "project_id", "created_at"})

		for rows.Next() {
			var event, actor, action, resource, result, projectID string
			var createdAt time.Time
			if err := rows.Scan(&event, &actor, &action, &resource, &result, &projectID, &createdAt); err != nil {
				continue
			}
			writer.Write([]string{event, actor, action, resource, result, projectID,
				createdAt.Format(time.RFC3339)})
		}
		writer.Flush()
	}
}
```

- [ ] **Step 2: 实现数据生命周期 cron**

Create `internal/compliance/data_lifecycle.go`:

```go
package compliance

import (
	"context"
	"database/sql"
	"log/slog"
	"time"
)

// DataLifecycle runs periodic cleanup of expired data.
type DataLifecycle struct {
	db     *sql.DB
	stopCh chan struct{}
}

func NewDataLifecycle(db *sql.DB) *DataLifecycle {
	return &DataLifecycle{db: db, stopCh: make(chan struct{})}
}

// Start begins the daily cleanup goroutine. Call Stop to shut down.
func (d *DataLifecycle) Start() {
	go func() {
		ticker := time.NewTicker(24 * time.Hour)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				d.cleanup(context.Background())
			case <-d.stopCh:
				return
			}
		}
	}()
	slog.Info("data-lifecycle started")
}

func (d *DataLifecycle) Stop() {
	close(d.stopCh)
	slog.Info("data-lifecycle stopped")
}

func (d *DataLifecycle) cleanup(ctx context.Context) {
	// 90 days: soft-deleted projects
	if _, err := d.db.ExecContext(ctx,
		`DELETE FROM audit_log WHERE created_at < NOW() - INTERVAL '365 days'`); err != nil {
		slog.Error("data-lifecycle audit cleanup failed", "error", err)
	}
	// Future: cleanup sandbox artifacts, expired tokens, etc.
}
```

- [ ] **Step 3: 在 Bootstrap 中条件启动 DataLifecycle**

Modify `internal/shared/profile/bootstrap.go` — 在 `of.FeatureFlags = ff` 行的 enterprise_platform gate 块之后添加：

```go
	// --- Feature Gate: compliance_suite ---
	if ff.ComplianceSuite {
		of.DataLifecycle = compliance.NewDataLifecycle(db)
		of.DataLifecycle.Start()
	}
```

并在 `OpenForge` struct 中添加字段：

```go
	FeatureFlags    *featureflags.FeatureFlags
	DataLifecycle   *compliance.DataLifecycle  // lifecycle cron (compliance_suite gate)
```

- [ ] **Step 4: 在 routes.go 中条件注册审计导出路由**

Modify `internal/server/routes.go`:

```go
	// Compliance suite routes (conditionally registered)
	if of.FeatureFlags.ComplianceSuite {
		mux.HandleFunc("GET /api/admin/audit/export", withAdmin(handleAuditExport(of)))
	}
```

- [ ] **Step 5: 前端条件路由 — 合规报告页**

Create `frontend/src/features/compliance/ComplianceReportPage.tsx` — 占位页面：

```tsx
export function ComplianceReportPage() {
  return (
    <div style={{ padding: 24, color: '#F1F5F9' }}>
      <h1>Compliance Report</h1>
      <p>Automated compliance report generation.</p>
    </div>
  );
}
```

Modify `frontend/src/App.tsx` — 在 `<Routes>` 的 `</Routes>` 之前添加条件路由（需要从 context 获取 featureFlags）：

```tsx
const ComplianceReportPage = lazy(() => import('./features/compliance/ComplianceReportPage'));

// Inside <Routes>:
{featureFlags?.compliance_suite && (
  <Route path="/compliance" element={
    <ProtectedRoute><Suspense fallback={<LoadingFallback />}><ComplianceReportPage /></Suspense></ProtectedRoute>
  } />
)}
```

- [ ] **Step 6: Build 验证**

```bash
go build ./... && cd frontend && npx tsc --noEmit
```
Expected: 无错误

- [ ] **Step 7: Commit**

```bash
git add internal/server/admin_audit.go internal/compliance/data_lifecycle.go \
        internal/shared/profile/bootstrap.go internal/server/routes.go \
        frontend/src/features/compliance/ComplianceReportPage.tsx frontend/src/App.tsx
git commit -m "feat(feature-gates): wire compliance_suite flag — audit export + data lifecycle + report page

- Audit export: CSV download at GET /api/admin/audit/export (admin-only)
- Data lifecycle: daily cleanup cron for audit_log (365d retention)
- Frontend: /compliance route conditionally rendered
- All gated behind compliance_suite flag
"
```

---

### Task 11: `production_ops` Gate 接入 — Notifier 替换 + 条件路由

**Files:**
- Modify: `internal/shared/profile/bootstrap.go`
- Modify: `internal/server/routes.go`
- Create: `internal/server/admin_runbook.go`
- Create: `frontend/src/features/monitoring/GrafanaPage.tsx`
- Modify: `frontend/src/App.tsx`

**设计原则：** 混合模式 — Notifier 使用"先 noop 后替换"，Runbook API 和 Grafana 面板使用"条件注册"。

- [ ] **Step 1: 在 Bootstrap 中替换 Notifier 实现**

Modify `internal/shared/profile/bootstrap.go` — 在 compliance_suite gate 块之后添加：

```go
	// --- Feature Gate: production_ops ---
	if ff.ProductionOps {
		of.Notifier = newProductionNotifier(cfg)
	}
```

**注意：** `newProductionNotifier` 根据 Profile 的 `notifier` 配置（`feishu-webhook` / `multi-channel`）实例化真实通知实现。当前阶段先添加骨架，具体实现在后续 Task 完成。

- [ ] **Step 2: 实现 Runbook API handler**

Create `internal/server/admin_runbook.go`:

```go
package server

import (
	"net/http"

	"openforge/internal/shared/profile"
)

// handleRunbookList returns available runbook entries.
func handleRunbookList(of *profile.OpenForge) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		entries := []map[string]string{
			{"id": "scale-up", "title": "Scale Up", "desc": "Horizontal scaling SOP"},
			{"id": "dr-recovery", "title": "DR Recovery", "desc": "Disaster recovery procedure"},
			{"id": "circuit-recovery", "title": "Circuit Recovery", "desc": "Circuit breaker recovery"},
			{"id": "knowledge-rollback", "title": "Knowledge Rollback", "desc": "Agent knowledge rollback"},
		}
		writeJSON(w, 200, entries)
	}
}

// handleRunbookDetail returns a specific runbook content.
func handleRunbookDetail(of *profile.OpenForge) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		writeJSON(w, 200, map[string]string{
			"id":      id,
			"content": "# Runbook: " + id + "\n\n## Steps\n1. Verify status\n2. Execute procedure\n3. Validate recovery",
		})
	}
}
```

- [ ] **Step 3: 在 routes.go 中条件注册 runbook 路由**

Modify `internal/server/routes.go`:

```go
	// Production ops routes (conditionally registered)
	if of.FeatureFlags.ProductionOps {
		mux.HandleFunc("GET /api/runbook", withAdmin(handleRunbookList(of)))
		mux.HandleFunc("GET /api/runbook/{id}", withAdmin(handleRunbookDetail(of)))
	}
```

- [ ] **Step 4: 前端条件路由 — Grafana 面板页**

Create `frontend/src/features/monitoring/GrafanaPage.tsx`:

```tsx
export function GrafanaPage() {
  return (
    <div style={{ padding: 24, color: '#F1F5F9' }}>
      <h1>Monitoring Dashboard</h1>
      <p>Grafana dashboards for production monitoring.</p>
      {/* Future: iframe embed Grafana */}
    </div>
  );
}
```

Modify `frontend/src/App.tsx`:

```tsx
const GrafanaPage = lazy(() => import('./features/monitoring/GrafanaPage'));

// Inside <Routes>:
{featureFlags?.production_ops && (
  <Route path="/monitoring" element={
    <ProtectedRoute><Suspense fallback={<LoadingFallback />}><GrafanaPage /></Suspense></ProtectedRoute>
  } />
)}
```

- [ ] **Step 5: Build 验证**

```bash
go build ./... && cd frontend && npx tsc --noEmit
```
Expected: 无错误

- [ ] **Step 6: Commit**

```bash
git add internal/shared/profile/bootstrap.go internal/server/routes.go \
        internal/server/admin_runbook.go \
        frontend/src/features/monitoring/GrafanaPage.tsx frontend/src/App.tsx
git commit -m "feat(feature-gates): wire production_ops flag — Notifier swap + Runbook API + Grafana panel

- Notifier: stdout → feishu-webhook/multi-channel when flag ON
- Runbook: GET /api/runbook + /api/runbook/{id} (admin-only)
- Frontend: /monitoring Grafana page conditionally rendered
- All gated behind production_ops flag
"
```

---

### Task 12: `distribution_artifacts` Gate 接入 — 纯增量条件路由

**Files:**
- Create: `internal/server/admin_download.go`
- Modify: `internal/server/routes.go`
- Create: `frontend/src/features/adr/ADRPage.tsx`
- Modify: `frontend/src/App.tsx`

**设计原则：** 最简单的 flag — 纯增量，无需替换任何现有实现。两个接入点：离线部署包下载 API + ADR 文档浏览页。

- [ ] **Step 1: 实现离线部署包下载 API**

Create `internal/server/admin_download.go`:

```go
package server

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"openforge/internal/shared/profile"
)

// handleDownloadOffline packages the offline deployment bundle as a zip.
func handleDownloadOffline(of *profile.OpenForge) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Check if offline bundle exists
		bundleDir := filepath.Join(of.Config.DataDir, "offline")
		if _, err := os.Stat(bundleDir); os.IsNotExist(err) {
			writeError(w, 404, "offline deployment bundle not found. Run generate.sh first.")
			return
		}

		buf := new(bytes.Buffer)
		zw := zip.NewWriter(buf)

		err := filepath.Walk(bundleDir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if info.IsDir() {
				return nil
			}
			relPath, _ := filepath.Rel(bundleDir, path)
			f, err := zw.Create(relPath)
			if err != nil {
				return err
			}
			src, err := os.Open(path)
			if err != nil {
				return err
			}
			defer src.Close()
			io.Copy(f, src)
			return nil
		})
		if err != nil {
			writeError(w, 500, fmt.Sprintf("packaging failed: %v", err))
			return
		}
		zw.Close()

		w.Header().Set("Content-Type", "application/zip")
		w.Header().Set("Content-Disposition",
			"attachment; filename=openforge-offline-bundle.zip")
		w.Write(buf.Bytes())
	}
}
```

- [ ] **Step 2: 在 routes.go 中条件注册下载路由**

Modify `internal/server/routes.go`:

```go
	// Distribution artifacts routes (conditionally registered)
	if of.FeatureFlags.DistributionArtifacts {
		mux.HandleFunc("GET /api/download/offline", withAdmin(handleDownloadOffline(of)))
	}
```

- [ ] **Step 3: 前端条件路由 — ADR 文档页**

Create `frontend/src/features/adr/ADRPage.tsx`:

```tsx
export function ADRPage() {
  return (
    <div style={{ padding: 24, color: '#F1F5F9' }}>
      <h1>Architecture Decision Records</h1>
      <p>Browse architecture decisions and design rationale.</p>
      {/* Future: load ADR markdown files from docs/adr/ */}
    </div>
  );
}
```

Modify `frontend/src/App.tsx`:

```tsx
const ADRPage = lazy(() => import('./features/adr/ADRPage'));

// Inside <Routes>:
{featureFlags?.distribution_artifacts && (
  <Route path="/adr" element={
    <ProtectedRoute><Suspense fallback={<LoadingFallback />}><ADRPage /></Suspense></ProtectedRoute>
  } />
)}
```

- [ ] **Step 4: Build 验证**

```bash
go build ./... && cd frontend && npx tsc --noEmit
```
Expected: 无错误

- [ ] **Step 5: 端到端 Git 端验证**

```bash
go vet ./...
```
Expected: 无问题

- [ ] **Step 6: Commit**

```bash
git add internal/server/admin_download.go internal/server/routes.go \
        frontend/src/features/adr/ADRPage.tsx frontend/src/App.tsx
git commit -m "feat(feature-gates): wire distribution_artifacts flag — offline download + ADR page

- Download: GET /api/download/offline zips the offline deployment bundle (admin-only)
- Frontend: /adr Architecture Decision Records page conditionally rendered
- All gated behind distribution_artifacts flag
"
```

---

### Task 13: 前端 — FeatureFlag Context（让条件路由工作）

**Files:**
- Create: `frontend/src/shared/featureFlags.tsx`
- Modify: `frontend/src/App.tsx`

**设计原则：** Task 10-12 中 App.tsx 使用 `featureFlags?.compliance_suite` 等条件渲染路由，但 App.tsx 自身没有 featureFlags 状态。通过 Context 在应用顶层加载 flags，供所有组件使用。

- [ ] **Step 1: 创建 FeatureFlag Context**

Create `frontend/src/shared/featureFlags.tsx`:

```tsx
import { createContext, useContext, useEffect, useState, type ReactNode } from 'react';
import { api, type FeatureFlags } from './api';

const FeatureFlagsContext = createContext<FeatureFlags | null>(null);

export function FeatureFlagsProvider({ children }: { children: ReactNode }) {
  const [flags, setFlags] = useState<FeatureFlags | null>(null);

  useEffect(() => {
    api.getFeatureFlags()
      .then(setFlags)
      .catch(() => setFlags(null)); // non-admin users get null
  }, []);

  return (
    <FeatureFlagsContext.Provider value={flags}>
      {children}
    </FeatureFlagsContext.Provider>
  );
}

export function useFeatureFlags(): FeatureFlags | null {
  return useContext(FeatureFlagsContext);
}
```

- [ ] **Step 2: 在 App.tsx 中包裹 Provider**

Modify `frontend/src/App.tsx`:

```tsx
import { FeatureFlagsProvider, useFeatureFlags } from './shared/featureFlags';
```

将 `<Routes>` 包裹在 Provider 中：

```tsx
export function App() {
  return (
    <FeatureFlagsProvider>
      <AppRoutes />
    </FeatureFlagsProvider>
  );
}

function AppRoutes() {
  const featureFlags = useFeatureFlags();
  return (
    <Routes>
      {/* ... existing routes ... */}
      {featureFlags?.compliance_suite && (
        <Route path="/compliance" element={...} />
      )}
      {/* ... */}
    </Routes>
  );
}
```

- [ ] **Step 3: TypeScript + Build 验证**

```bash
cd frontend && npx tsc --noEmit && npx vite build
```
Expected: 无类型错误，build 成功

- [ ] **Step 4: Commit**

```bash
git add frontend/src/shared/featureFlags.tsx frontend/src/App.tsx
git commit -m "feat(feature-gates): add FeatureFlagsProvider context for conditional routing

- FeatureFlagsProvider loads flags on mount via GET /api/admin/feature-flags
- useFeatureFlags() hook for any component to read current flag state
- App.tsx refactored to AppRoutes with conditional route rendering
"
```

---

## Phase 2 Acceptance Criteria

| # | Criterion | Verification |
|---|-----------|-------------|
| 10 | `enterprise_platform` ON → 5个内核接口替换为真实实现 | Bootstrap log / status API |
| 11 | `enterprise_platform` OFF → 全部保持 noop stub | 系统功能正常无报错 |
| 12 | `compliance_suite` ON → `/api/admin/audit/export` 可访问 | curl 下载 CSV |
| 13 | `compliance_suite` ON → cron job 启动日志可见 | log "data-lifecycle started" |
| 14 | `compliance_suite` OFF → audit export 返回 404 | curl 验证 |
| 15 | `production_ops` ON → Runbook API + Grafana 页面可用 | curl + 前端访问 |
| 16 | `production_ops` ON → Notifier 切换到飞书/多渠道 | 日志确认 Notifier 类型 |
| 17 | `distribution_artifacts` ON → 下载 API + ADR 页面可用 | curl + 前端访问 |
| 18 | 切换 flag 后不需要重启（内存即时同步） | Admin 面板切 ON → 功能立即可用 |

---

## Design Notes

**为什么是 4 个 flag 而不是更细粒度？**
12 个独立的 Phase 9-10 功能之间有强依赖关系。例如 Vault → K8s → MinIO → DR 是层层依赖的栈，拆开反而增加管理负担。4 个 flag 在灵活性和简洁性之间取得平衡。

**为什么 DB 覆盖 YAML 而不是反过来？**
因为 DB 值代表 Admin 的显式意图（"我手动关了这个"），应该比 YAML（可能是模板默认值）优先级更高。首次启动时 DB 为空，SeedDefaults 写入 YAML 默认值；之后 Admin 的任何修改都是显式覆盖。

**Gate 接入模式总结：**

| Flag | 模式 | 原因 |
|------|------|------|
| `enterprise_platform` | 先 noop 后替换 | 内核接口在 DB 之前初始化 |
| `compliance_suite` | 纯增量条件注册 | 3个接入点全为新增功能 |
| `production_ops` | 混合（替换 + 条件注册） | Notifier 需替换，Runbook/Grafana 为新增 |
| `distribution_artifacts` | 纯增量条件注册 | ADR + 下载全为新增功能 |

**安全兜底：** 所有 flag OFF 时，系统完全回退到现有行为。DB 故障时，内核接口保持 noop stub，系统正常运行。不存在 flag 导致启动失败的路径。
