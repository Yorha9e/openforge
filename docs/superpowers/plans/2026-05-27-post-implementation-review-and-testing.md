# Post-Implementation Comprehensive Testing Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在 `feature-flags` 和 `enterprise-features-implementation` 两个计划完成后，进行系统性测试。

**Architecture:** 分层测试 — flag OFF 零影响验证 → flag ON 功能验证 → 全栈端到端。

**Note:** Phase A (Gap Review) 已移入两个计划各自的 `## 🔍 Pre-implementation Review` 节，作为实现前修正指南。

**Tech Stack:** Go 1.25 + PostgreSQL + React + TypeScript + curl + psql

---

## 分层测试

> 测试按 Feature Flag 分组，每组先测 flag OFF（零影响），再测 flag ON。

---

### Test Group 1: Feature Flags 基础设施

#### T1.1: Migration 测试

- [ ] **Step 1: 验证表结构**

```bash
psql -h localhost -U openforge -d openforge -c "\d feature_flags"
```
Expected: 3 列 (flag_key, enabled, updated_at)

- [ ] **Step 2: 验证迁移幂等**

```bash
psql -h localhost -U openforge -d openforge -f migrations/006_feature_flags.up.sql 2>&1
```
Expected: `NOTICE: relation "feature_flags" already exists, skipping`

- [ ] **Step 3: 验证回滚 + 重新创建**

```bash
psql -h localhost -U openforge -d openforge -f migrations/006_feature_flags.down.sql
psql -h localhost -U openforge -d openforge -f migrations/006_feature_flags.up.sql
psql -h localhost -U openforge -d openforge -c "SELECT COUNT(*) FROM feature_flags;"
```
Expected: count = 0（新创建的空表）

#### T1.2: Store 单元测试

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
	if err := store.SeedDefaults(ctx, &FeatureFlags{}); err != nil {
		t.Fatalf("seed2: %v", err)
	}
	flags, _ := store.Load(ctx)
	if !flags.ComplianceSuite || !flags.ProductionOps {
		t.Error("SeedDefaults overwrote existing — idempotency broken")
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
		t.Error("overwrite should have set false")
	}
}
```

- [ ] **Step 2: 运行测试**

```bash
go test ./internal/shared/featureflags/ -v -count=1
```
Expected: 3 tests PASS

#### T1.3: Admin API 测试

> **前提：** 服务器以 minimal profile 运行

- [ ] **Step 1: 获取 token + GET flags**

```bash
TOKEN=$(curl -s http://localhost:8030/api/auth/login -H "Content-Type: application/json" -d '{"username":"admin","password":"admin123"}' | jq -r '.token')
curl -s http://localhost:8030/api/admin/feature-flags -H "Authorization: Bearer $TOKEN" | jq .
```
Expected: 4 个字段均为 false

- [ ] **Step 2: PUT 更新 + 验证内存即时生效**

```bash
curl -s -X PUT http://localhost:8030/api/admin/feature-flags -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" -d '{"enterprise_platform":false,"compliance_suite":true,"production_ops":true,"distribution_artifacts":false}' | jq .
# 立即再次 GET
curl -s http://localhost:8030/api/admin/feature-flags -H "Authorization: Bearer $TOKEN" | jq .
```
Expected: 两次返回一致，compliance_suite=true, production_ops=true

- [ ] **Step 3: 验证非 admin 返回 403**

```bash
curl -s -o /dev/null -w "%{http_code}" http://localhost:8030/api/admin/feature-flags
```
Expected: 401 或 403

- [ ] **Step 4: 验证 DB 持久化**

```bash
psql -h localhost -U openforge -d openforge -c "SELECT flag_key, enabled FROM feature_flags ORDER BY flag_key;"
```
Expected: 4 行，enabled 值与 PUT 一致

- [ ] **Step 5: 验证重启后 DB 值优先于 YAML 默认值**

```bash
# 将 standard.yaml 中默认 ON 的 compliance_suite 改为 OFF
curl -s -X PUT http://localhost:8030/api/admin/feature-flags -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" -d '{"enterprise_platform":false,"compliance_suite":false,"production_ops":false,"distribution_artifacts":false}'
# 重启服务器后再次 GET
curl -s http://localhost:8030/api/admin/feature-flags -H "Authorization: Bearer $TOKEN" | jq .
```
Expected: 所有 false（DB 值覆盖 standard.yaml 默认 ON 值）

---

### Test Group 2: `enterprise_platform` ON/OFF

#### T2.1: Flag OFF — 零影响验证

- [ ] **Step 1: 确认 all flags OFF 后系统正常启动**

```bash
# 用 minimal.yaml 启动
go run ./cmd/server/ --config config/profiles/minimal.yaml &
sleep 5
curl -s http://localhost:8030/api/health | jq .
```
Expected: health check 200 OK

- [ ] **Step 2: 企业路由 404**

```bash
curl -s -o /dev/null -w "%{http_code}" http://localhost:8030/api/vault/status
curl -s -o /dev/null -w "%{http_code}" http://localhost:8030/api/k8s/status
curl -s -o /dev/null -w "%{http_code}" http://localhost:8030/api/storage/status
```
Expected: 全部 404（路由未注册）

#### T2.2: Flag ON — 企业功能验证

> **前提：** Vault、MinIO、Docker 已在 Docker Compose 中启动

- [ ] **Step 1: 确认 Docker Compose 中 Vault/MinIO 可用**

```bash
docker compose -f deployments/docker-compose.standard.yaml ps | grep -E "vault|minio"
```
Expected: 服务状态 healthy（如果有 Vault/MinIO 容器）

- [ ] **Step 2: 设置 enterprise_platform=ON**

```bash
TOKEN=$(curl -s http://localhost:8030/api/auth/login -H "Content-Type: application/json" -d '{"username":"admin","password":"admin123"}' | jq -r '.token')
curl -s -X PUT http://localhost:8030/api/admin/feature-flags -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" -d '{"enterprise_platform":true,"compliance_suite":false,"production_ops":false,"distribution_artifacts":false}'
```

- [ ] **Step 3: Vault Secret Store 测试**

```bash
# 如果 Vault 可用：
go test -run TestVaultSecretStore ./internal/adapter/ -v -count=1
```
Expected: PASS 或 SKIP（Vault 不可用时）

- [ ] **Step 4: MinIO Object Store 测试**

```bash
go test -run TestMinioObjectStore ./internal/adapter/ -v -count=1
```
Expected: PASS 或 SKIP

- [ ] **Step 5: Docker Container Runtime 测试**

```bash
go test -run TestDockerContainerRuntime ./internal/adapter/ -v -count=1
```
Expected: PASS 或 SKIP

- [ ] **Step 6: 验证不 panic 降级**

```bash
# 停止 Vault 和 MinIO，重启服务器
docker compose -f deployments/docker-compose.standard.yaml stop vault minio
go run ./cmd/server/ --config config/profiles/standard.yaml &
sleep 5
curl -s http://localhost:8030/api/health | jq .
```
Expected: 200 OK（系统正常运行，企业功能降级为 noop）

---

### Test Group 3: `compliance_suite` ON/OFF

- [ ] **Step 1: Flag OFF → 审计导出 404**

```bash
curl -s -o /dev/null -w "%{http_code}" http://localhost:8030/api/admin/audit/export -H "Authorization: Bearer $TOKEN"
```
Expected: 404

- [ ] **Step 2: Flag ON → 审计导出下载 CSV**

```bash
curl -s -X PUT http://localhost:8030/api/admin/feature-flags -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" -d '{"enterprise_platform":false,"compliance_suite":true,"production_ops":false,"distribution_artifacts":false}'

curl -s -o /tmp/audit.csv -w "%{http_code}" http://localhost:8030/api/admin/audit/export -H "Authorization: Bearer $TOKEN"
head -3 /tmp/audit.csv
```
Expected: 200，CSV header: `event,actor,action,resource,result,project_id,created_at`

- [ ] **Step 3: 验证 DataLifecycle cron 启动**

```bash
# 检查服务器日志
grep "data-lifecycle" server.log
```
Expected: 日志包含 "data-lifecycle started"

---

### Test Group 4: `production_ops` ON/OFF

- [ ] **Step 1: Flag OFF → Runbook 404**

```bash
curl -s -o /dev/null -w "%{http_code}" http://localhost:8030/api/runbook -H "Authorization: Bearer $TOKEN"
```
Expected: 404

- [ ] **Step 2: Flag ON → Runbook API 可用**

```bash
curl -s -X PUT http://localhost:8030/api/admin/feature-flags -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" -d '{"enterprise_platform":false,"compliance_suite":false,"production_ops":true,"distribution_artifacts":false}'

curl -s http://localhost:8030/api/runbook -H "Authorization: Bearer $TOKEN" | jq .
```
Expected: 返回 4 个 runbook 条目

- [ ] **Step 3: 飞书 Notifier 发送测试**

```bash
go test -run TestFeishuNotifier ./internal/adapter/ -v -count=1
```
Expected: PASS（使用 mock webhook）或 SKIP

---

### Test Group 5: `distribution_artifacts` ON/OFF

- [ ] **Step 1: Flag OFF → 下载 API 404**

```bash
curl -s -o /dev/null -w "%{http_code}" http://localhost:8030/api/download/offline -H "Authorization: Bearer $TOKEN"
```
Expected: 404

- [ ] **Step 2: Flag ON → 下载 API 可用**

```bash
curl -s -X PUT http://localhost:8030/api/admin/feature-flags -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" -d '{"enterprise_platform":false,"compliance_suite":false,"production_ops":false,"distribution_artifacts":true}'

# 无 bundle → 404
curl -s http://localhost:8030/api/download/offline -H "Authorization: Bearer $TOKEN" | jq .
```
Expected: 404 + error message "run generate.sh first"

---

### Test Group 6: 全栈端到端

#### T6.1: 前端页面验证

- [ ] **Step 1: 登录 + 访问 Admin**

打开 `https://localhost/` → 登录 → Admin 页面 → 确认 4 个 toggle 可见且可交互

- [ ] **Step 2: 前端条件路由验证**

开启 `compliance_suite` → 访问 `/compliance` → 确认页面显示（非 404）
关闭 `compliance_suite` → 访问 `/compliance` → 确认 404 或重定向

- [ ] **Step 3: 前端条件路由（全部页面）**

同样验证 `/monitoring`（production_ops）和 `/adr`（distribution_artifacts）

#### T6.2: 切换 flag 不需要重启

- [ ] **Step 1: 验证 4 个 flag 全部切换后功能即时生效**

```bash
# 全部 OFF → 全部 ON
curl -s -X PUT ... -d '{"enterprise_platform":true,"compliance_suite":true,"production_ops":true,"distribution_artifacts":true}'
# 立即验证所有企业路由可用
for ep in "/api/vault/status" "/api/admin/audit/export" "/api/runbook" "/api/download/offline"; do
  code=$(curl -s -o /dev/null -w "%{http_code}" http://localhost:8030$ep -H "Authorization: Bearer $TOKEN")
  echo "$ep → $code (expected: 200)"
done
```
Expected: 所有端点 200（Vault 可能 503 如果 Vault 不可用）

#### T6.3: Docker Compose 全栈部署

- [ ] **Step 1: 启动全部服务**

```bash
cd deployments
docker compose -f docker-compose.standard.yaml up -d --build
```

- [ ] **Step 2: 验证所有容器健康**

```bash
docker compose -f docker-compose.standard.yaml ps
```
Expected: nginx(healthy), server(healthy), postgres(healthy), redis(healthy), nodejs-io(healthy)

- [ ] **Step 3: HTTPS 健康检查**

```bash
curl -k https://localhost/api/health | jq .
```
Expected: 200 OK

- [ ] **Step 4: WebSocket 连接验证**

```bash
# 在浏览器 console:
const ws = new WebSocket('wss://localhost/ws');
ws.onopen = () => console.log('WS connected');
ws.onmessage = (e) => console.log('WS msg:', e.data);
```
Expected: 连接成功

- [ ] **Step 5: 全量编译验证**

```bash
go build ./cmd/server/ && go build ./cmd/openforge/
go vet ./...
cd frontend && npx tsc --noEmit && npx vite build
```
Expected: 全部零错误

---

## Acceptance Criteria

| # | Criterion | Group |
|---|-----------|-------|
| 1 | `feature_flags` 表创建 + 迁移可回滚 | T1.1 |
| 2 | Store 单元测试全部通过 | T1.2 |
| 3 | GET/PUT /api/admin/feature-flags 正常工作 | T1.3 |
| 4 | 非 admin 用户访问返回 401/403 | T1.3 |
| 5 | 重启后 DB 值覆盖 YAML 默认值 | T1.3 |
| 6 | 所有 flag OFF 时系统正常运行（零影响） | T2.1, T3, T4, T5 |
| 7 | enterprise_platform ON → Vault/MinIO/Docker 可用 | T2.2 |
| 8 | 外部依赖不可用 → 不 panic，系统降级运行 | T2.2 |
| 9 | compliance_suite ON → audit export + cron | T3 |
| 10 | production_ops ON → runbook API + Grafana 页 | T4 |
| 11 | distribution_artifacts ON → 下载 API + ADR 页 | T5 |
| 12 | 前端 4 个 toggle 交互正常 | T6.1 |
| 13 | 前端条件路由正确 | T6.1 |
| 14 | 切换 flag 不需要重启（即时生效） | T6.2 |
| 15 | Docker Compose 全栈部署成功 | T6.3 |
| 16 | `go build ./... && go vet ./...` 零错误 | T6.3 |
| 17 | `npx tsc --noEmit && npx vite build` 零错误 | T6.3 |

---

## Execution

**Test plan saved to `docs/superpowers/plans/2026-05-27-post-implementation-review-and-testing.md`.**

**执行顺序：** 先完成两个计划的实现（注意各自 `## 🔍 Pre-implementation Review` 节的修正）→ 再执行本测试计划。
