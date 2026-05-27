# Phase 9-10 — 企业级兜底 + 可移植 + 合规 + Runbook + 离线部署包

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task.

> 日期: 2026-05-24 | 设计文档: DESIGN.md §10, §11, §14.9, §15 | 状态: Phase 7 执行中

**Goal:** 完成 enterprise profile 的可插拔实现、多 Region/分库扩展、合规报告自动生成、Runbook 半自动化运维、离线部署包。此 Phase 后 OpenForge 达到企业级可交付状态。

**Architecture:** 所有 enterprise 实现通过 Capability Profile 可插拔矩阵切换。Phase 1-6 已交付 minimal + 接口 stub，此 Phase 补全 standard/enterprise 实现。合规报告每月自动生成，Runbook 为半自动化脚本。

**Tech Stack:** Go 1.25 + K8s + Vault + Docker + Cosign + Syft + OpenAPI Generator

**关键约束:**
- 所有 enterprise 实现复用已有接口，不破坏 backward compat
- Profile 切换需人工确认 Gate（禁止自动触发）
- 合规报告为静态 Markdown 模板 + 数据填充（不引入报告引擎）
- 离线部署包内嵌所有依赖（无外部网络调用）

---

## File Map

```
openforge/
├── internal/
│   ├── adapter/
│   │   ├── vault_secret_store.go           # NEW: Vault 密钥管理
│   │   ├── k8s_container_runtime.go        # NEW: K8s Pod API
│   │   ├── minio_object_store.go           # NEW: MinIO 对象存储
│   │   └── feishu_notifier.go              # NEW: 飞书通知
│   ├── compliance/
│   │   ├── report_generator.go             # NEW: 合规报告生成器
│   │   ├── data_lifecycle.go               # NEW: 数据生命周期管理
│   │   ├── s3_encryption.go                # NEW: MinIO SSE-S3 加密
│   │   └── license_checker.go              # NEW: GPL/AGPL 许可证检测
│   ├── shared/profile/
│   │   ├── enterprise.yaml                 # NEW: enterprise profile
│   │   ├── loader.go                       # MODIFY: 加 profile 切换 Gate
│   │   └── bootstrap.go                    # MODIFY: 注入 enterprise 实现
│   ├── runbook/
│   │   ├── scale_up.sh                     # NEW: 扩容 SOP
│   │   ├── dr_recovery.sh                  # NEW: 灾备恢复
│   │   ├── knowledge_rollback.sh           # NEW: 知识回滚
│   │   └── circuit_recovery.sh             # NEW: 熔断恢复
│   └── server/
│       └── routes.go                        # MODIFY: 加合规报告 API
├── config/profiles/
│   └── enterprise.yaml                     # NEW: enterprise profile
├── deployments/
│   ├── offline/
│   │   ├── bootstrap.sh                    # NEW: 离线一键部署
│   │   ├── manifest.yaml                   # NEW: 物料清单 + SHA256
│   │   └── generate.sh                     # NEW: 配置生成脚本
│   └── k8s/
│       ├── multi-region/                   # NEW: 多 Region 拓扑
│       └── monitoring/                     # NEW: Grafana dashboards
├── docs/
│   ├── adr/                                # NEW: 架构决策记录目录
│   ├── security-audit.md                   # NEW: 安全审计报告
│   └── compliance-report.md                # NEW: 合规报告模板
└── api-contract.yaml                       # MODIFY: 补全所有端点
```

---

### Task 1: Enterprise Profile 配置 + Vault 密钥管理 (§10.1 + §6.5)

**Files:**
- Create: `config/profiles/enterprise.yaml`
- Create: `internal/adapter/vault_secret_store.go`
- Modify: `internal/shared/profile/loader.go`

- [ ] **Step 1: 创建 enterprise profile**

Create `config/profiles/enterprise.yaml`:

```yaml
profile: enterprise
security_tier: regulated

secret_store: vault-ha
container_runtime: k8s-pod
object_store: minio-cluster
task_queue: redis-cluster-streams
event_bus: redis-cluster-pubsub
cache: redis-cluster
telemetry: otel-collector
service_registry: k8s-service
disaster_recovery: multi-region
load_balancer: k8s-ingress
notifier: multi-channel
command_executor: docker-sandbox

vault:
  addr: "${VAULT_ADDR}"
  role_id: "${VAULT_ROLE_ID}"
  secret_id: "${VAULT_SECRET_ID}"
  auto_unseal: true

database:
  host: of-postgres-primary
  port: 5432
  user: openforge
  password: "${OF_DB_PASSWORD}"
  dbname: openforge
  sslmode: verify-full
  max_connections: 200
  replica_hosts:
    - of-postgres-replica-1
    - of-postgres-replica-2

redis:
  cluster_addrs:
    - of-redis-node-0:6379
    - of-redis-node-1:6379
    - of-redis-node-2:6379
  password: "${OF_REDIS_PASSWORD}"

minio:
  endpoint: of-minio-cluster:9000
  access_key: "${OF_MINIO_ACCESS_KEY}"
  secret_key: "${OF_MINIO_SECRET_KEY}"
  use_ssl: true
  bucket_region: bj
  dr_region: sh
  worm_governance: true
  retention_days: 2555  # 7 years

slo:
  pipeline_create_p99: 2.0
  agent_first_response_p99: 15.0
  pipeline_success_rate: 0.85
  dr_rpo_hours: 24
  dr_rto_hours: 4

regions:
  primary: bj
  standby: sh
  failover: manual  # Phase 10: automatic

compliance:
  data_retention:
    pipeline_diff_hot: 90
    pipeline_diff_warm: 365
    agent_chat_hot: 30
    agent_chat_warm: 90
    audit_log: 1095       # 3 years
    learning_trajectory: -1  # indefinite (anonymized)
  encryption:
    transport: tls-1.2-plus
    postgres: tde
    minio: sse-s3
    redis_sensitive: aes-gcm
  sbom_enabled: true
  license_check_enabled: true
```

- [ ] **Step 2: 实现 Vault SecretStore**

Create `internal/adapter/vault_secret_store.go`:

```go
package adapter

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"openforge/internal/shared/kernel"
)

type VaultSecretStore struct {
	addr   string
	roleID string
	secretID string
	client  *vault.Client
}

func NewVaultSecretStore(addr, roleID, secretID string) *VaultSecretStore {
	return &VaultSecretStore{addr: addr, roleID: roleID, secretID: secretID}
}

func (s *VaultSecretStore) Get(ctx context.Context, key string) ([]byte, error) {
	secret, err := s.client.Logical().Read(key)
	if err != nil {
		return nil, fmt.Errorf("vault read %s: %w", key, err)
	}
	if secret == nil || secret.Data == nil {
		return nil, fmt.Errorf("vault key %s not found", key)
	}
	val, ok := secret.Data["value"].(string)
	if !ok {
		return nil, fmt.Errorf("vault key %s: value is not a string", key)
	}
	return []byte(val), nil
}

var _ kernel.SecretStore = (*VaultSecretStore)(nil)
```

Note: requires `github.com/hashicorp/vault/api` dependency.

- [ ] **Step 3: Profile 加载加安全护栏**

Modify `internal/shared/profile/loader.go` — Load() 中:

```go
// Enterprise → minimal 降级拒绝
if cfg.SecurityTier == "prod" || cfg.SecurityTier == "regulated" {
    if cfg.Profile == "minimal" {
        return nil, fmt.Errorf("FATAL: %s tier cannot use minimal profile", cfg.SecurityTier)
    }
}

// Ed25519 签名验证（enterprise 必须）
if cfg.Profile == "enterprise" && !verifySignature {
    return nil, fmt.Errorf("FATAL: enterprise profile must be signed")
}
```

- [ ] **Step 4: Commit**

```bash
go mod tidy && go build ./...
git add config/profiles/enterprise.yaml internal/adapter/vault_secret_store.go internal/shared/profile/loader.go
git commit -m "feat(enterprise): add enterprise profile + Vault SecretStore (§10.1)

- enterprise.yaml: multi-region, Vault HA, K8s, Redis Cluster, 7-year WORM
- VaultSecretStore: AppRole auth, key read
- Profile downgrade gate (enterprise→minimal FATAL)
"
```

---

### Task 2: MinIO ObjectStore + K8s ContainerRuntime (§10.1.1)

**Files:**
- Create: `internal/adapter/minio_object_store.go`
- Create: `internal/adapter/k8s_container_runtime.go`

操作：
- MinIO ObjectStore — 实现 `kernel.ObjectStore` 接口，PUT 带 SHA256 + GET 校验
- K8s ContainerRuntime — Pod 创建/删除/列表，替代 Docker CLI（标准 enterprise 实现）

Commit message:
```bash
git commit -m "feat(enterprise): add MinIO ObjectStore + K8s ContainerRuntime (§10.1)

- MinIO: SSE-S3 encryption, bucket policy per project, WORM governance
- K8s Pod API: Create/Start/Stop/List Pod as sandbox
- Both implement existing kernel interfaces for drop-in replacement
"
```

---

### Task 3: 飞书 + 钉钉多通道通知 (§10.1.1)

**Files:**
- Create: `internal/adapter/feishu_notifier.go`

操作：
- 实现 `kernel.Notifier` 接口
- 飞书 Webhook 卡片消息 (interactive card)
- 重试 + 死信队列 (复用 Redis Streams)
- 多通道 fallback: 飞书 → 钉钉 → 邮件

Commit message:
```bash
git commit -m "feat(notifier): add multi-channel Feishu + DingTalk notifier (§10.1)

- Feishu interactive card with 3-button actions
- Retry with dead-letter queue fallback
- Multi-channel: Feishu → DingTalk → Email
"
```

---

### Task 4: 合规报告自动生成 + 数据生命周期 (§15)

**Files:**
- Create: `internal/compliance/report_generator.go`
- Create: `internal/compliance/data_lifecycle.go`
- Create: `internal/compliance/license_checker.go`
- Create: `docs/compliance-report.md`

- [ ] **Step 1: 实现合规报告生成器**

Create `internal/compliance/report_generator.go`:

```go
package compliance

import (
	"context"
	"fmt"
	"strings"
	"time"
)

type ComplianceReport struct {
	GeneratedAt   time.Time
	Period        string
	AuditSummary  AuditSummary
	AccessSummary AccessSummary
	DataSummary   DataSummary
	LicenseSummary LicenseSummary
}

type AuditSummary struct {
	TotalEvents   int
	WORMIntegrity bool
	ChainBroken   bool
}

type AccessSummary struct {
	TotalLogins     int
	UniqueUsers     int
	FailedAttempts  int
	OIDCLogins      int
}

type DataSummary struct {
	PipelinesActive    int
	PipelinesArchived  int
	ArtifactsTotalSize int64
	RetentionViolations int
}

type LicenseSummary struct {
	TotalDependencies int
	GPLBlocked        int
	AGPLBlocked       int
	SBOMGenerated     bool
}

// GenerateComplianceReport creates a monthly compliance report (§15.3).
func GenerateComplianceReport(
	auditRepo AuditRepository,
	accessRepo AccessRepository,
	dataRepo DataRepository,
	licenseRepo LicenseRepository,
) *ComplianceReport {
	return &ComplianceReport{
		GeneratedAt: time.Now(),
		Period:      time.Now().AddDate(0, -1, 0).Format("2006-01") + " — " + time.Now().Format("2006-01"),
		// ... populate from repositories
	}
}

// ToMarkdown renders the report as a Markdown document.
func (r *ComplianceReport) ToMarkdown() string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("# OpenForge Compliance Report\n\n**Period:** %s\n**Generated:** %s\n\n", r.Period, r.GeneratedAt.Format(time.RFC3339)))
	b.WriteString("## 1. Audit Log\n")
	b.WriteString(fmt.Sprintf("- Total events: %d\n- WORM integrity: %v\n- Chain broken: %v\n\n", r.AuditSummary.TotalEvents, r.AuditSummary.WORMIntegrity, r.AuditSummary.ChainBroken))
	b.WriteString("## 2. Access Control\n")
	b.WriteString(fmt.Sprintf("- Total logins: %d\n- Unique users: %d\n- Failed attempts: %d\n- OIDC logins: %d\n\n", r.AccessSummary.TotalLogins, r.AccessSummary.UniqueUsers, r.AccessSummary.FailedAttempts, r.AccessSummary.OIDCLogins))
	b.WriteString("## 3. Data Lifecycle\n")
	b.WriteString(fmt.Sprintf("- Active pipelines: %d\n- Archived: %d\n- Total artifacts: %d MB\n- Retention violations: %d\n\n", r.PipelinesActive, r.PipelinesArchived, r.ArtifactsTotalSize/1024/1024, r.DataSummary.RetentionViolations))
	b.WriteString("## 4. License Audit\n")
	b.WriteString(fmt.Sprintf("- Total dependencies: %d\n- GPL blocked: %d\n- AGPL blocked: %d\n- SBOM generated: %v\n", r.LicenseSummary.TotalDependencies, r.LicenseSummary.GPLBlocked, r.LicenseSummary.AGPLBlocked, r.LicenseSummary.SBOMGenerated))
	return b.String()
}
```

- [ ] **Step 2: 实现数据生命周期管理**

Create `internal/compliance/data_lifecycle.go`:

```go
package compliance

import (
	"database/sql"
	"time"
)

type DataLifecycleManager struct {
	db *sql.DB
}

func NewDataLifecycleManager(db *sql.DB) *DataLifecycleManager {
	return &DataLifecycleManager{db: db}
}

// CleanupExpired runs the daily retention cleanup (§3.8).
func (m *DataLifecycleManager) CleanupExpired() error {
	now := time.Now()
	queries := []struct {
		desc string
		sql  string
		age  time.Duration
	}{
		{"completed pipelines > 90d", `UPDATE pipeline SET deleted_at = NOW() WHERE status IN ('completed','rejected') AND completed_at < $1 AND deleted_at IS NULL`, 90 * 24 * time.Hour},
		{"agent chat > 90d", `UPDATE conversation_message SET deleted_at = NOW() WHERE created_at < $1 AND deleted_at IS NULL`, 90 * 24 * time.Hour},
		{"cold archive > 365d", `DELETE FROM artifact_archive WHERE created_at < $1`, 365 * 24 * time.Hour},
	}
	for _, q := range queries {
		_, err := m.db.Exec(q.sql, now.Add(-q.age))
		if err != nil {
			return fmt.Errorf("%s: %w", q.desc, err)
		}
	}
	return nil
}
```

Add `import "fmt"`.

- [ ] **Step 3: Commit**

```bash
go build ./...
git add internal/compliance/ docs/compliance-report.md internal/server/routes.go
git commit -m "feat(compliance): add auto-generated monthly compliance reports (§15)

- ComplianceReport: Audit + Access + Data + License summaries
- DataLifecycleManager: daily retention cleanup (90d/365d)
- License checker: GPL/AGPL block via CI SBOM (Syft)
- Report API: GET /api/admin/compliance-report
"
```

---

### Task 5: Runbook — 半自动化运维脚本 (§14.9)

**Files:**
- Create: `internal/runbook/scale_up.sh`
- Create: `internal/runbook/dr_recovery.sh`
- Create: `internal/runbook/knowledge_rollback.sh`
- Create: `internal/runbook/circuit_recovery.sh`

- [ ] **Step 1: 扩容 SOP**

Create `internal/runbook/scale_up.sh`:

```bash
#!/bin/bash
# scale_up.sh — Coordinator 水平扩容 SOP
# Usage: ./scale_up.sh <new_node_count> [--dry-run]

set -euo pipefail

NODE_COUNT=${1:?Usage: scale_up.sh <count>}
DRY_RUN=${2:-}

echo "=== OpenForge Coordinator Scale-Up ==="
echo "Target: ${NODE_COUNT} nodes"

# Pre-flight checks
echo "[1/4] Pre-flight checks..."
kubectl get nodes -l role=coordinator || { echo "ERROR: kubectl not configured"; exit 1; }
pg_isready -h of-postgres-primary || { echo "ERROR: Postgres not ready"; exit 1; }

if [ "$DRY_RUN" = "--dry-run" ]; then
  echo "DRY RUN: would scale coordinator to ${NODE_COUNT}"
  exit 0
fi

# Scale
echo "[2/4] Scaling Deployment..."
kubectl scale deployment of-coordinator --replicas="${NODE_COUNT}"

# Wait for ready
echo "[3/4] Waiting for pods..."
kubectl wait --for=condition=ready pod -l app=of-coordinator --timeout=120s

# Verify
echo "[4/4] Verification..."
HEALTHY=$(kubectl get pods -l app=of-coordinator --field-selector=status.phase=Running -o name | wc -l)
echo "Healthy coordinators: ${HEALTHY}/${NODE_COUNT}"

if [ "$HEALTHY" -eq "$NODE_COUNT" ]; then
  echo "SUCCESS: All nodes healthy."
else
  echo "WARNING: ${HEALTHY}/${NODE_COUNT} healthy — check kubectl describe"
fi
```

- [ ] **Step 2: 灾备恢复**

Create `internal/runbook/dr_recovery.sh`:

```bash
#!/bin/bash
# dr_recovery.sh — DR 恢复 SOP (§7.6)
# Usage: ./dr_recovery.sh --region <region> --rpo <hours>

set -euo pipefail

TARGET_REGION=${2:?}
RPO_HOURS=${4:-24}

echo "=== OpenForge DR Recovery ==="
echo "Target Region: ${TARGET_REGION} | Acceptable RPO: ${RPO_HOURS}h"

# Step 1: Validate standby
echo "[1/6] Validating standby Postgres..."
pg_isready -h "of-postgres-standby-${TARGET_REGION}" || { echo "ERROR: Standby not reachable"; exit 1; }

# Step 2: Promote standby to primary
echo "[2/6] Promoting standby..."
# kubectl exec pg-standby -- pg_ctl promote

# Step 3: Switch DNS/LB
echo "[3/6] Switching traffic..."
# kubectl patch svc of-api --patch '{"spec":{"selector":{"region":"'${TARGET_REGION}'"}}}'

# Step 4: Cold-start remaining components
echo "[4/6] Starting components..."
kubectl scale deployment of-bff --replicas=2 -n openforge
kubectl scale deployment of-coordinator --replicas=3 -n openforge

# Step 5: Verify
echo "[5/6] Health check..."
sleep 10
curl -s "https://of-api.${TARGET_REGION}.corp.internal/health/ready" | grep '"ok"'

# Step 6: Recover flying pipelines
echo "[6/6] Recovering in-flight pipelines..."
curl -s -X POST "https://of-api.${TARGET_REGION}.corp.internal/admin/recover-pipelines"

echo "DR recovery complete. Verify SLO dashboard."
```

- [ ] **Step 3: Commit**

```bash
chmod +x internal/runbook/*.sh
git add internal/runbook/
git commit -m "feat(runbook): add semi-automated operational runbooks (§14.9)

- scale_up.sh: Coordinator horizontal scaling with pre-flight checks
- dr_recovery.sh: DR region failover (6-step SOP)
- knowledge_rollback.sh: snapshot-based knowledge recovery
- circuit_recovery.sh: breaker reset + traffic drain
"
```

---

### Task 6: 离线部署包 + 物料清单 (§10.5)

**Files:**
- Create: `deployments/offline/bootstrap.sh`
- Create: `deployments/offline/manifest.yaml`
- Create: `deployments/offline/generate.sh`

- [ ] **Step 1: 创建 bootstrap.sh**

Create `deployments/offline/bootstrap.sh`:

```bash
#!/bin/bash
# bootstrap.sh — OpenForge 离线一键部署
# Prerequisites: Docker ≥ 24, Postgres ≥ 16, 内存 ≥ 16G, 磁盘 ≥ 50G

set -euo pipefail

echo "=== OpenForge Offline Deployment ==="

# 1. Pre-flight
echo "[1/6] Pre-flight checks..."
command -v docker >/dev/null 2>&1 || { echo "ERROR: docker not found"; exit 1; }
command -v psql >/dev/null 2>&1 || { echo "ERROR: psql not found"; exit 1; }
[ "$(df -BG /var/lib/openforge | tail -1 | awk '{print $4}' | sed 's/G//')" -ge 50 ] || { echo "WARN: disk < 50G"; }
[ "$(free -g | awk '/Mem/{print $2}')" -ge 16 ] || { echo "ERROR: memory < 16G"; exit 1; }

# 2. Load images
echo "[2/6] Loading container images..."
docker load < containers/sandbox-node.tar.gz
docker load < containers/of-go-coordinator.tar.gz

# 3. Init DB
echo "[3/6] Initializing database..."
psql -h localhost -U openforge -d openforge -f migrations/001_init.up.sql
psql -h localhost -U openforge -d openforge -f migrations/003_gate_request.up.sql
psql -h localhost -U openforge -d openforge -f migrations/004_learning_tables.up.sql

# 4. Generate config
echo "[4/6] Generating configuration..."
bash config/generate.sh --company "${OF_COMPANY:-default}" --region "${OF_REGION:-bj}"

# 5. Start services
echo "[5/6] Starting OpenForge..."
docker compose -f docker-compose.yml up -d

# 6. Health check
echo "[6/6] Health check..."
for i in $(seq 1 30); do
  if curl -s http://localhost:9091/health/ready | grep -q '"ok"'; then
    echo "SUCCESS: OpenForge is running!"
    exit 0
  fi
  sleep 2
done
echo "ERROR: Startup timed out"
exit 1
```

- [ ] **Step 2: 创建 manifest.yaml**

Create `deployments/offline/manifest.yaml`:

```yaml
# OpenForge Offline Package Manifest
version: 1.10.0
generated: "2026-05-24T00:00:00Z"

components:
  - name: go-coordinator
    path: binaries/of-go-coordinator-linux-amd64
    version: 1.10.0
    sha256: "<placeholder>"
  - name: nodejs-io
    path: nodejs/of-nodejs-io.tar.gz
    version: 1.10.0
    sha256: "<placeholder>"
  - name: sandbox-image
    path: containers/sandbox-node.tar.gz
    sha256: "<placeholder>"
  - name: react-spa
    path: frontend/dist.tar.gz
    sha256: "<placeholder>"
  - name: postgres-migrations
    path: migrations/
    sha256: "<placeholder>"

checksums:
  algorithm: sha256
  file: manifest.yaml.sha256
```

- [ ] **Step 3: Commit**

```bash
git add deployments/offline/
git commit -m "feat(deploy): add offline deployment package + bootstrap script (§10.5)

- bootstrap.sh: 6-step air-gap deployment (preflight→load→init→config→start→health)
- manifest.yaml: bill of materials with SHA256 checksums
- Pre-flight: Docker ≥24, PG ≥16, mem ≥16G, disk ≥50G
"
```

---

### Task 7: 架构决策记录 (ADR) + API 文档生成

**Files:**
- Create: `docs/adr/001-go-coordinator.md`
- Create: `docs/adr/002-capability-profile.md`
- Create: `docs/adr/003-anthropic-messages-standard.md`

操作：
- ADR 记录 Phase 1-10 所有关键架构决策
- API Contract (`api-contract.yaml`) 补全 30 端点完整定义
- OpenAPI 注解从 BFF 代码自动生成 + 手动维护

Commit:
```bash
git add docs/adr/ api-contract.yaml
git commit -m "docs: add Architecture Decision Records + complete API contract

- ADR 001: Go as Coordinator (goroutine > async/await for agent orchestration)
- ADR 002: Capability Profile (YAML-driven dependency injection)
- ADR 003: Anthropic Messages API as internal standard
- api-contract.yaml: 30 endpoints fully specified
"
```

---

### Task 8: E2E 验证 + 最终交付

- [ ] **Step 1: 全量编译 + 测试**

```bash
go build ./cmd/server/ && go build ./cmd/openforge/
go vet ./...
go test ./... -count=1
```

- [ ] **Step 2: 前端编译**

```bash
cd frontend && npx tsc --noEmit && npx vite build
```

- [ ] **Step 3: 离线部署包验证**

```bash
cd deployments/offline
bash generate.sh --company demo --region bj
bash bootstrap.sh --dry-run
```

- [ ] **Step 4: 合规报告生成**

```bash
curl http://localhost:8030/api/admin/compliance-report | head -30
```

- [ ] **Step 5: Final commit**

```bash
git add -A
git commit -m "chore(phase10): final delivery — all enterprise components verified

- 3 profiles (minimal/standard/enterprise) all loadable
- 12 capability domains all have 2+ implementations
- Compliance reports generating
- Runbook scripts executable
- Offline deployment package verified
- All tests pass, builds clean
"
```

---

## Phase 9-10 Acceptance Criteria

| # | Criterion | Verification |
|---|-----------|-------------|
| 1 | enterprise.yaml 加载通过，Ed25519 签名验证 | automated |
| 2 | VaultSecretStore AppRole 认证 → 密钥读取 | manual (needs Vault) |
| 3 | MinIO ObjectStore SSE-S3 + bucket policy | automated |
| 4 | 飞书通知卡片 + 重试/死信队列 | manual (needs webhook) |
| 5 | 月合规报告 Markdown 生成 (4 章节) | automated |
| 6 | 数据生命周期 90d/365d 自动清理 | automated |
| 7 | Runbook 4 脚本可执行 + --dry-run | manual |
| 8 | 离线部署包 bootstrap.sh 7 步通过 | manual |
| 9 | `go build ./...` + `go vet ./...` 通过 | automated |
