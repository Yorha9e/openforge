# Enterprise Features 统一实现计划

> **目标：** 实现 Phase 9-10 全部 13 项企业级功能，按 4 个 Feature Flag 分组。
> **前提：** Phase 1（Flags 基础设施）和 Phase 2（Gate 接入骨架）已完成。
> **原则：** 每个功能独立可测、flag 关闭时零影响、启动不阻塞、失败不 panic。

---

## 🔍 Pre-implementation Review — 审查缺漏与修正

> 以下缺漏在 2026-05-27 审查中发现，**必须在实现前修正**。

| # | 级别 | 缺漏 | 修正方案 | 影响位置 |
|---|:--:|------|----------|:--:|
| **G10** | 🔴 | **Docker SDK 不在 `go.mod`** — 计划声称"已有间接依赖"但实际无 | 追加 **Task 0**: `go get github.com/docker/docker/client` | Feature 1.3 |
| **G11** | 🔴 | **bootstrap.go 工厂函数修改无具体代码** — 只说 "修改 newSecretStore" 但不给完整修改 | 每个接入点追加修改前后完整代码（见各 Feature 接入点） | Part 1 全部 |
| **G12** | 🔴 | **无独立 go.mod 更新任务** — Vault + MinIO + Docker 三个 `go get` 操作分散在任务描述中 | 追加 **Task 0**: 统一安装全部新依赖 | Feature 1.1-1.3 |
| **G13** | 🟡 | **PG DR `pg_dump`/`pg_restore` 路径硬编码** — Docker 容器中可能无此命令 | 添加 `pg_tools_path` config 字段 + Dockerfile 安装说明 | Feature 1.4 |
| **G14** | 🟡 | **Vault KV v1 vs v2 未检测** — 默认可能是 v1 | 自动检测 `/v1/sys/internal/ui/mounts`，fallback v2 | Feature 1.1 |
| **G15** | 🟡 | **FeishuNotifier 测试代码缺失** — 说"单元测试"但无具体代码 | 追加 mock webhook server 测试用例 | Feature 3.1 |
| **G16** | 🟡 | **无 graceful shutdown 清理** — `DataLifecycle.Stop()` 和 adapter 连接未在 shutdown hook 调用 | 追加 shutdown 注册步骤 | Part 1/2 |
| **G17** | 🟡 | **前端占位页面无功能** — 3 个页面只是 `<h1>` + `<p>` | 至少加 loading skeleton + API 错误处理 | Feature 2.3/3.3/4.2 |
| **G18** | 🟡 | **`Loader` → `LoadBalancer` 命名** — 代码中两处混用 | 统一使用 `LoadBalancer` | Feature 1.5 |
| **G19** | 🟢 | Vault/MinIO 初始化失败时不记录结构化日志 | 追加 `slog.Warn("adapter unavailable", ...)` | Feature 1.1-1.2 |
| **G20** | 🟢 | 无 latency budget 定义 | 在配置中加 `timeout_sec` 默认值（Vault 3s / MinIO 5s） | Feature 1.1-1.2 |

---

## Task 0: 前置依赖安装 (G10+G12)

> **必须先于所有 Feature 实现。**

- [ ] **Step 1: 安装全部新 Go 依赖**

```bash
go get github.com/hashicorp/vault/api
go get github.com/minio/minio-go/v7
go get github.com/docker/docker/client
```

- [ ] **Step 2: 验证 go.mod 更新**

```bash
grep -E "vault/api|minio/minio-go|docker/docker" go.mod
```

Expected: 至少三行匹配

- [ ] **Step 3: 验证编译（空引入，无破坏）**

```bash
go build ./...
```

- [ ] **Step 4: Commit**

```bash
git add go.mod go.sum
git commit -m "chore(deps): add vault/api + minio/minio-go/v7 + docker/docker/client

Required by enterprise_platform flag adapters:
- VaultSecretStore, MinIOObjectStore, DockerContainerRuntime
"
```

---

## 全局 File Map

```
openforge/
├── internal/
│   ├── adapter/
│   │   ├── vault_secret_store.go / _test.go        # NEW
│   │   ├── minio_object_store.go / _test.go        # NEW
│   │   ├── docker_container_runtime.go / _test.go  # NEW
│   │   ├── feishu_notifier.go / _test.go           # NEW
│   │   ├── pg_disaster_recovery.go                 # NEW
│   │   ├── nginx_load_balancer.go                  # NEW
│   │   └── redis_cache.go                          # NEW
│   ├── compliance/data_lifecycle.go                # NEW
│   ├── server/
│   │   ├── admin_audit.go / admin_runbook.go / admin_download.go  # NEW
│   │   └── routes.go                               # MODIFY
│   └── shared/profile/
│       ├── loader.go    # MODIFY: +VaultConfig MinioConfig NotifierConfig K8sConfig
│       └── bootstrap.go # MODIFY: 所有工厂函数接线
├── config/profiles/
│   └── standard.yaml    # MODIFY: +vault minio notifier 配置节
├── frontend/src/
│   ├── features/{compliance,monitoring,adr}/*.tsx  # NEW
│   └── App.tsx          # MODIFY: 条件路由
└── go.mod               # MODIFY: +hashicorp/vault/api +minio/minio-go/v7
```

---

# Part 1: `enterprise_platform` — 5 个内核接口

> **模式：** 先 noop 后替换 · **兜底：** 初始化失败 → 保持 noop，系统不崩

## Feature 1.1: Vault Secret Store

**接口：** `kernel.SecretStore` — `Get(ctx, key) ([]byte, error)`
**依赖：** `go get github.com/hashicorp/vault/api`
**新建：** `vault_secret_store.go` / `_test.go`

### Task 1.1.1: 加 VaultConfig（`loader.go`）

```go
type VaultConfig struct {
    Addr       string `yaml:"addr"`        // "http://vault:8200"
    RoleID     string `yaml:"role_id"`     // AppRole
    SecretID   string `yaml:"secret_id"`
    AutoUnseal bool   `yaml:"auto_unseal"`
    Token      string `yaml:"token"`       // dev mode
    EnginePath string `yaml:"engine_path"` // default "secret"
    EngineVersion string `yaml:"engine_version"` // G14: "v1" or "v2", empty = auto-detect
    TimeoutSec int    `yaml:"timeout_sec"` // default 3
}
```

`Config` struct 加 `Vault VaultConfig \`yaml:"vault"\``，加 `EnginePathOrDefault()` / `TimeoutOrDefault()` helpers.

### Task 1.1.2: 实现 VaultSecretStore（`vault_secret_store.go`）

```go
type VaultSecretStore struct {
    client  *vault.Client; mount string; timeout time.Duration; enabled bool
}

func NewVaultSecretStore(cfg profile.VaultConfig) *VaultSecretStore {
    // 空地址 → enabled=false → slog.Warn (G19: 结构化日志)
    // 创建 client → AppRole 认证 → Token fallback
    // 失败 → enabled=false → slog.Warn("vault unavailable", "reason", err)（不 panic）
    // G14: 自动检测 engine version — GET /v1/sys/internal/ui/mounts/<mount>
    //   若 data["type"]=="kv" && data["options"]["version"]=="1" → use KV v1 read path
    //   否则 → KV v2 (default)
}

func (v *VaultSecretStore) Get(ctx context.Context, key string) ([]byte, error) {
    // KV v2: GET /v1/<mount>/data/<key> → 解析 double-wrapped data
    // 返回 "value" 字段 或 第一个 string 字段
}

var _ kernel.SecretStore = (*VaultSecretStore)(nil)
```

**接入点（`bootstrap.go`）：** 修改 `newSecretStore` 的 `vault-sidecar` / `vault-ha` case — Vault 可用时 prepend 到 chain 前面。

### Task 1.1.3: 测试（`vault_secret_store_test.go`）

- `TestVaultSecretStore_Get_KVv2_Success` — 写 seed → 读验证
- `TestVaultSecretStore_Get_NotFound`
- `TestVaultSecretStore_Disabled_OnBadAddress/Unreachable`
- `vaultAvailable()` → 无 Vault 自动 `t.Skip`

---

## Feature 1.2: MinIO Object Store

**接口：** `kernel.ObjectStore` — `Put/Get/Delete/List`
**依赖：** `go get github.com/minio/minio-go/v7`
**新建：** `minio_object_store.go` / `_test.go`

### Task 1.2.1: 加 MinioConfig（`loader.go`）

```go
type MinioConfig struct {
    Endpoint        string `yaml:"endpoint"`         // "minio:9000"
    AccessKeyID     string `yaml:"access_key_id"`
    SecretAccessKey string `yaml:"secret_access_key"`
    Bucket          string `yaml:"bucket"`           // default "openforge"
    UseSSL          bool   `yaml:"use_ssl"`
    Region          string `yaml:"region"`           // default "us-east-1"
    TimeoutSec      int    `yaml:"timeout_sec"`      // default 5
}
```

### Task 1.2.2: 实现 MinioObjectStore

```go
type MinioObjectStore struct {
    client *minio.Client; bucket string; timeout time.Duration; enabled bool
}

func NewMinioObjectStore(cfg profile.MinioConfig) *MinioObjectStore {
    // 空 endpoint → enabled=false
    // minio.New() → MakeBucket (幂等, 忽略 AlreadyExists)
    // 失败 → enabled=false
}
// Put → client.PutObject(ctx, bucket, key, reader, -1, ...)
// Get → client.GetObject(ctx, bucket, key, ...)
// Delete → client.RemoveObject(...)
// List → client.ListObjects(ctx, bucket, ListObjectsOptions{Prefix: prefix})
```

**接入点（`bootstrap.go`）：** `newObjectStore` — `minio`/`minio-cluster` case 替换 noop。

### Task 1.2.3: 测试

- `TestMinioObjectStore_PutGet_CRUD` / `_Disabled_EmptyEndpoint` / `_NotFound`
- `minioAvailable()` → Docker MinIO 不可用自动 skip

---

## Feature 1.3: Docker Container Runtime

**接口：** `kernel.ContainerRuntime` — `Create/Start/Stop/Remove/List`
**依赖：** Docker SDK（已有间接依赖）
**新建：** `docker_container_runtime.go` / `_test.go`

```go
type DockerContainerRuntime struct {
    client *docker.Client; enabled bool
}

func NewDockerContainerRuntime(host string) *DockerContainerRuntime {
    // docker.NewClientWithOpts(docker.FromEnv, docker.WithHost(host))
    // ping 验证 → 失败 → enabled=false
}

// Create → client.ContainerCreate(image, env, cmd, workdir)
// Start → client.ContainerStart
// Stop → client.ContainerStop(10s timeout)
// Remove → client.ContainerRemove(force=true)
// List → client.ContainerList(all=true)
```

**接入点（`bootstrap.go`）：** `newContainerRuntime` — `docker`/`k8s-pod` case 替换 noop。

---

## Feature 1.4: PG Disaster Recovery

**接口：** `kernel.DisasterRecovery` — `Backup/Restore/Status`
**依赖：** 无（`os/exec` 调 `pg_dump`/`pg_restore`）
**新建：** `pg_disaster_recovery.go`

```go
type PGDisasterRecovery struct {
    db *sql.DB; dsn string; backupDir string; pgToolsPath string  // G13: configurable pg_dump path
    lastBackup, lastRestore time.Time; mu sync.RWMutex
}

func NewPGDisasterRecovery(db *sql.DB, dsn, backupDir, pgToolsPath string) *PGDisasterRecovery
// NOTE: Dockerfile must include postgresql-client for pg_dump/pg_restore:
//   RUN apt-get update && apt-get install -y postgresql-client && rm -rf /var/lib/apt/lists/*

// Backup → <pgToolsPath>/pg_dump -Fc -f <dir>/<timestamp>.dump <dsn>，保留最近 7 个
// Restore → 找最近备份文件 → <pgToolsPath>/pg_restore -d <dsn> <file>
// Status → 检查备份文件存在 + 24h 内新鲜度
```

**接入点（`bootstrap.go`）：** `newDisasterRecovery` — `pg-streaming`/`multi-region` case 替换 noop（需传 `db` 参数）。

---

## Feature 1.5: Nginx LoadBalancer

**接口：** `kernel.LoadBalancer` — `AddBackend/RemoveBackend/HealthCheck`
**依赖：** 无（内存 pool + nginx conf 文件）
**新建：** `nginx_load_balancer.go`

```go
type NginxLoadBalancer struct {
    mu sync.RWMutex; backends map[string][]string; configPath string
}

func NewNginxLoadBalancer(configPath string) *NginxLoadBalancer

// AddBackend → 加到内存 pool（去重）+ 可选重写 nginx upstream conf
// RemoveBackend → 从 pool 移除
// HealthCheck → pool 非空即健康
```

**接入点（`bootstrap.go`）：** `newLoadBalancer` — `nginx`/`k8s-ingress` case 替换 noop。

---

# Part 2: `compliance_suite` — 3 个合规模块

> **模式：** 纯增量条件注册
> **兜底：** flag OFF → 路由 404 / cron 不启动 / 页面不渲染

## Feature 2.1: 审计日志导出 API（`admin_audit.go`）

```go
func handleAuditExport(of *profile.OpenForge) http.HandlerFunc {
    // SELECT event, actor, action, resource, result, project_id, created_at
    //   FROM audit_log ORDER BY created_at DESC LIMIT 10000
    // → CSV: Content-Type text/csv + Content-Disposition attachment
}
```
**接入：** `routes.go` — `compliance_suite` ON → `GET /api/admin/audit/export`.

## Feature 2.2: 数据生命周期 Cron（`compliance/data_lifecycle.go`）

```go
type DataLifecycle struct { db *sql.DB; stopCh chan struct{} }
// Start → 24h ticker goroutine
// cleanup → DELETE audit_log WHERE created_at < NOW() - INTERVAL '365 days'
// Stop → close(stopCh)
```
**接入：** `bootstrap.go` — `compliance_suite` ON → `of.DataLifecycle = compliance.NewDataLifecycle(db); of.DataLifecycle.Start()`.
**OpenForge 新字段：** `DataLifecycle *compliance.DataLifecycle`.

## Feature 2.3: 合规报告页面（`ComplianceReportPage.tsx`）

占位页面（G17: 含 loading skeleton + error handling）：

```tsx
import { useEffect, useState } from 'react';
import { api } from '../../shared/api';

export function ComplianceReportPage() {
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    // Future: load actual report data
    const t = setTimeout(() => { setLoading(false); }, 800);
    return () => clearTimeout(t);
  }, []);

  if (error) return <ErrorBanner message={error} />;

  return (
    <div style={{ padding: 24 }}>
      <h1 style={{ color: '#F1F5F9', fontSize: 20 }}>Compliance Report</h1>
      {loading ? (
        <div style={{ marginTop: 16 }}>
          <SkeletonRow /><SkeletonRow /><SkeletonRow />
        </div>
      ) : (
        <p style={{ color: '#94A3B8', marginTop: 12 }}>
          Automated compliance report generation coming soon.
        </p>
      )}
    </div>
  );
}
```
**接入：** `App.tsx` → `featureFlags?.compliance_suite && <Route path="/compliance">`.

---

# Part 3: `production_ops` — 3 个运维模块

> **模式：** 混合（Notifier 先 noop 后替换 + 其余条件注册）

## Feature 3.1: Feishu / Multi-Channel Notifier

**接口：** `kernel.Notifier` — `Send/SendWithRetry`
**依赖：** 无（`net/http` POST JSON）
**新建：** `feishu_notifier.go` / `_test.go`

### Task 3.1.1: 加 NotifierConfig（`loader.go`）

```go
type NotifierConfig struct {
    FeishuWebhook string `yaml:"feishu_webhook"`
    DingtalkURL   string `yaml:"dingtalk_url"`
    EmailSMTP     string `yaml:"email_smtp"`
}
```

### Task 3.1.2: 实现

```go
type FeishuNotifier struct { webhookURL string }
// Send → POST JSON to webhook (飞书卡片消息), 3s timeout
// SendWithRetry → 指数退避: 1s → 2s → 4s

type MultiChannelNotifier struct { channels []kernel.Notifier }
// Send → 广播到所有 channels（飞书 + 钉钉 + 邮件）
```

**接入点（`bootstrap.go`）：** `newNotifier` — `feishu` → `FeishuNotifier`, `multi-channel` → `MultiChannelNotifier`.

### Task 3.1.3: 测试 (G15 — 追加)

Create `internal/adapter/feishu_notifier_test.go`:

```go
package adapter

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestFeishuNotifier_Send_Success(t *testing.T) {
	received := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]interface{}
		json.NewDecoder(r.Body).Decode(&body)
		if body["msg_type"] != nil {
			received = true
		}
		w.WriteHeader(200)
		w.Write([]byte(`{"code":0,"msg":"ok"}`))
	}))
	defer srv.Close()

	n := NewFeishuNotifier(profile.NotifierConfig{FeishuWebhook: srv.URL})
	err := n.Send(context.Background(), "test message")
	if err != nil {
		t.Fatalf("send failed: %v", err)
	}
	if !received {
		t.Error("webhook not called")
	}
}

func TestFeishuNotifier_Send_RetryOnFailure(t *testing.T) {
	attempts := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts < 3 {
			w.WriteHeader(500)
			return
		}
		w.WriteHeader(200)
		w.Write([]byte(`{"code":0}`))
	}))
	defer srv.Close()

	n := NewFeishuNotifier(profile.NotifierConfig{FeishuWebhook: srv.URL})
	err := n.SendWithRetry(context.Background(), "retry test", 3)
	if err != nil {
		t.Fatalf("retry failed: %v", err)
	}
	if attempts != 3 {
		t.Errorf("expected 3 attempts, got %d", attempts)
	}
}

func TestFeishuNotifier_Disabled_EmptyWebhook(t *testing.T) {
	n := NewFeishuNotifier(profile.NotifierConfig{})
	if n.enabled {
		t.Error("should be disabled when webhook is empty")
	}
}
```

## Feature 3.2: Runbook API（`admin_runbook.go`）

```go
func handleRunbookList → 返回硬编码条目列表
func handleRunbookDetail → 按 id 返回具体 runbook 内容（后续加载 YAML/MD）
```
**接入：** `routes.go` → `GET /api/runbook` + `GET /api/runbook/{id}`.

## Feature 3.3: Grafana 监控页面（`GrafanaPage.tsx`）

占位页面 → 后续 iframe 嵌入 Grafana dashboard。
**接入：** `App.tsx` → `<Route path="/monitoring">`.

---

# Part 4: `distribution_artifacts` — 2 个发布模块

> **模式：** 纯增量条件注册

## Feature 4.1: 离线部署包下载（`admin_download.go`）

```go
func handleDownloadOffline → 打包 /data/offline 目录为 zip
    // 目录不存在 → 404 "run generate.sh first"
```
**接入：** `routes.go` → `GET /api/download/offline`.

## Feature 4.2: ADR 文档页面（`ADRPage.tsx`）

占位页面 → 后续渲染 `docs/adr/` markdown。
**接入：** `App.tsx` → `<Route path="/adr">`.

---

## Graceful Shutdown 清理 (G16)

> **原因：** 审查发现 `DataLifecycle.Stop()` 和 adapter 连接未在 server shutdown 时调用。

- [ ] **Step 1: 在 Bootstrap 返回后注册 shutdown hook**

在 `internal/shared/profile/bootstrap.go` 的 `Bootstrap()` 返回前：

```go
// G16: Register cleanup on shutdown.
of.Shutdown = func() {
    if of.DataLifecycle != nil {
        of.DataLifecycle.Stop()
    }
    // Close enterprise adapter connections.
    if vs, ok := of.Secrets.(io.Closer); ok { vs.Close() }
    if ms, ok := of.Object.(io.Closer); ok { ms.Close() }
    if dc, ok := of.Container.(io.Closer); ok { dc.Close() }
    slog.Info("enterprise adapters shutdown complete")
}
```

- [ ] **Step 2: 在 OpenForge struct 中添加 Shutdown 字段**

```go
Shutdown       func()        // G16: graceful shutdown callback
```

- [ ] **Step 3: 在 server main 的 `signal.Notify` 中调用**

```go
sig := make(chan os.Signal, 1)
signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
go func() {
    <-sig
    of.Shutdown()
    // ... existing shutdown logic ...
}()
```

---

## 依赖汇总

| 功能 | Go 依赖 | 负担 |
|------|--------|------|
| Vault Secret Store | `hashicorp/vault/api` | ~3MB |
| MinIO Object Store | `minio/minio-go/v7` | ~2MB |
| Docker Runtime | `docker/docker/client` | ~10MB (G10 FIX: 非间接依赖，需显式 go get) |
| DR / LB / Notifier / Audit / Lifecycle / Runbook / Download | 无新增 | 0 |
| **总计** | **3 个新依赖** | **~15MB** |

---

## 测试策略

| 层级 | 策略 | 适用 |
|------|------|------|
| 集成测试（需外部服务） | `t.Skip` 当服务不可用时 | Vault, MinIO, Docker |
| 单元测试（无需外部服务） | 直接测试逻辑 | DR, LB, Notifier |
| 禁用状态测试 | 验证 `enabled=false` 路径 | 全部 |

---

## 验收标准

### enterprise_platform

| # | Criterion | Verification |
|---|-----------|-------------|
| 1 | Vault 可用 → secret 正确读取 | 集成测试 PASS |
| 2 | Vault 不可用 → 系统 fallback envfile 正常 | 无 Vault 启动无报错 |
| 3 | MinIO CRUD 正常 | 集成测试 PASS |
| 4 | MinIO 不可用 → noop 降级 | 无 MinIO 启动无报错 |
| 5 | Docker 容器 Create/Start/Stop 正常 | 集成测试 PASS |
| 6 | DR Backup 生成 pg_dump | 手动触发 |
| 7 | LB AddBackend/RemoveBackend 正常 | 单元测试 |

### compliance_suite

| # | Criterion |
|---|-----------|
| 8 | `GET /api/admin/audit/export` → CSV 下载 |
| 9 | flag OFF → 404 |
| 10 | cron 启动日志可见 |
| 11 | `/compliance` 页面可访问 |

### production_ops

| # | Criterion |
|---|-----------|
| 12 | 飞书 webhook → 群消息 |
| 13 | Notifier 失败 → 日志 + 重试 |
| 14 | `/api/runbook` 返回条目 |
| 15 | `/monitoring` 页面可访问 |

### distribution_artifacts

| # | Criterion |
|---|-----------|
| 16 | `GET /api/download/offline` → zip |
| 17 | 无 bundle → 404 提示 |
| 18 | `/adr` 页面可访问 |

---

## 设计笔记

**为什么 `enabled` flag 而非 init() panic？**
所有企业适配器初始化失败时返回 `enabled=false`，不 panic。开发环境可能没 Vault/MinIO，K8s Pod 重启时外部依赖可能还没就绪。

**为什么 PG DR 用 `pg_dump` 而非 WAL streaming？**
`pg_dump -Fc` 零依赖、家用电脑可测、CI 可跑。WAL streaming 可后续替换（不改接口）。

**为什么 LB 用内存 pool 而非 K8s API？**
K8s Ingress API 依赖集群环境和 RBAC，家用电脑不可测。内存 pool 方案测试够用，生产可替换（不改接口）。

---

## 推荐实现顺序

```
Phase 1+2 完成（Flags + Gate 骨架）
    ↓
1.1 VaultSecretStore         ← 接口最简单：只有 Get()
1.2 MinioObjectStore         ← 独立
1.3 DockerContainerRuntime   ← 独立
3.1 FeishuNotifier           ← 独立（webhook 测试）
1.4 PGDisasterRecovery       ← 需要 db 参数
1.5 NginxLoadBalancer        ← 独立
    ↓
2.1 Audit Export API         ← 读 DB audit_log
2.2 Data Lifecycle           ← DB cron
3.2 Runbook API              ← 静态内容
    ↓
4.1 Offline Download         ← 文件系统
    ↓
{2.3, 3.3, 4.2} 前端页面    ← 一次性批量创建
```

每完成一个 → `go build ./...; go vet ./...` → commit → 下一个。
