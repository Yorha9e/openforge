# Admin 基础设施健康面板重新设计计划

> 创建日期：2026-05-28
> 关联：AdminPage 右侧 "Milestones Complete" → "基础设施健康" 重构
> 设计参考：ui-ux-pro-max skill (Dark OLED Dashboard + Infrastructure Monitoring UX)

---

## 一、背景与动机

### 当前问题

- 右侧「Milestones Complete」展示的是 **Phase 完成情况**，但 Phase 1~10 已全部完成，信息价值大幅降低
- 右侧空间（2fr 列，约占页面 40%）被低价值内容占据
- **缺少对生产级中间件/基础设施运行状态的直观监控**

### 用户需求

> 替换为「企业级中间件/基础设施是否接入当前业务运作、健康状态、运行时长」的监控面板

### 设计目标

| # | 目标 | 描述 |
|---|------|------|
| 1 | **实时可见** | 一眼看到各基础设施是否存活、健康 |
| 2 | **接入状态** | 区分已启用/未配置/降级三种接入态 |
| 3 | **运行时长** | 展示各组件自启动以来的 uptime |
| 4 | **告警层次** | 健康(绿)/降级(黄)/故障(红)/未使用(灰) 四色体系 |
| 5 | **信息密度** | 在有限空间内展示 8~12 个基础设施组件 |

---

## 二、可用基础设施清单

根据项目 `kernel/interfaces.go` + Profile 体系，梳理以下 12 个组件：

| # | 组件 | 内核接口 | Profile 可见性 | 健康检查方式 |
|---|------|---------|----------------|--------------|
| 1 | **PostgreSQL** | N/A (直接连接) | 所有 Profile | `pg_isready` / 连接池状态 / 断路器 |
| 2 | **Redis** | `Cache` / `TaskQueue` / `EventBus` | standard+ | PING 命令 / 断路器 |
| 3 | **Docker** | `ContainerRuntime` | 所有 Profile | `client.Ping()` / 断路器 |
| 4 | **MinIO / S3** | `ObjectStore` | standard+ | `BucketExists()` / 断路器 |
| 5 | **Vault** | `SecretStore` | standard+ | `Sys().Health()` API |
| 6 | **Nginx** | `LoadBalancer` | standard | `HealthCheck()` 后端可用性 |
| 7 | **gRPC IO** | N/A (LLM 服务) | 所有 Profile | `/api/health` HTTP 端点 |
| 8 | **Sandbox Provider** | `CommandExecutor` | standard+ | 预热池可用数 / 断路器 |
| 9 | **Feishu Notifier** | `Notifier` | standard+ | Webhook 可达性 |
| 10 | **K8s** | `ContainerRuntime` / `ServiceRegistry` | enterprise | 接口预留（尚未完全实现） |
| 11 | **Telemetry** | `Telemetry` | standard+ (prometheus-loki) / enterprise (otel) | 端点可达性 |
| 12 | **Disaster Recovery** | `DisasterRecovery` | 所有 Profile | `Status()` 检查 24h 内备份 |

---

## 三、现有数据源分析

### 当前 `/api/admin/status` 返回结构

```typescript
type AdminStatus = {
  phase: string;                    // "Phase 8"
  profile: string;                  // "minimal" | "standard" | "enterprise"
  tier: string;                     // security tier
  skills: number;                   // skill count
  rbac: string;                     // "active"
  oidc: string;                     // "enabled" | "disabled"
  auth_provider: string;            // "oidc" | "jwt"
  models: number;                   // LLM model count
  circuit_breakers?: Record<string, string>;  // e.g. { "postgres": "closed", "docker": "open" }
  slo?: { total: number; success_rate: number; p95_ms?: number };
  ha?: {
    task_queue: string;
    hash_ring_nodes: number;
    load_shedding: string;
  };
}
```

### 可推导信息 (无需后端改动)

| 字段 | 可推导结论 |
|------|-----------|
| `profile` | 确定哪些中间件已配置（minimal=基础 / standard=生产 / enterprise=全栈） |
| `circuit_breakers` | 已有断路器状态的组件（postgres/docker/minio/llm） |
| `ha.task_queue` | Redis 是否作为任务队列启用 |
| `slo` | 整体 SLO 指标 |

### 需要后端扩展

| 新增字段 | 类型 | 说明 |
|---------|------|------|
| `infrastructure` | `InfraComponent[]` | 各组件独立健康状态 |
| `infrastructure[].name` | `string` | 组件名 (postgres, redis, docker, ...) |
| `infrastructure[].healthy` | `boolean` | 是否健康 |
| `infrastructure[].status` | `'connected'\|'degraded'\|'unavailable'\|'unused'` | 接入状态 |
| `infrastructure[].uptime_seconds` | `number?` | 运行时长(秒) |
| `infrastructure[].latency_ms` | `number?` | 最后检查延迟(毫秒) |
| `infrastructure[].message` | `string?` | 附加信息（如 "3.2GB / 10GB"） |
| `infrastructure[].version` | `string?` | 版本号 |

---

## 四、设计方案

### 4.1 布局位置

```
┌──────────────────────────────────────────────────┐
│                 AdminPage (grid: 3fr 2fr)          │
├─────────────────────────┬────────────────────────┤
│   LEFT (3fr)            │   RIGHT (2fr)          │
│                          │                        │
│   Current Session        │   Role Hierarchy       │
│   System Status          │                        │
│   (StatusBadges/SLO/HA/  │   ▼ NEW SECTION ▼      │
│    CircuitBreakers)     │                        │
│                          │   Infrastructure      │
│                          │   Health               │
│                          │   ┌─────────────────┐ │
│                          │   │ 🟢 PostgreSQL   │ │
│                          │   │    Connected    │ │
│                          │   │    18d 4h       │ │
│                          │   ├─────────────────┤ │
│                          │   │ 🟢 Redis        │ │
│                          │   │    Connected    │ │
│                          │   │    18d 4h       │ │
│                          │   ├─────────────────┤ │
│                          │   │ 🟡 MinIO        │ │
│                          │   │    Degraded     │ │
│                          │   │    CB: half_open│ │
│                          │   ├─────────────────┤ │
│                          │   │ ⚪ K8s          │ │
│                          │   │    Not Used     │ │
│                          │   └─────────────────┘ │
│                          │                        │
│   Enterprise Feature     │                        │
│   Toggles (full width)   │                        │
└─────────────────────────┴────────────────────────┘
```

### 4.2 视觉设计

#### 颜色体系 (基于现有 Dark OLED tokens)

| 状态 | 颜色 | 含义 | 视觉效果 |
|------|------|------|---------|
| `connected` | `#22C55E` (CTA Green) | 健康运行 | 绿色脉冲圆点 + 正常文字 |
| `degraded` | `#F59E0B` (Amber) | 降级但可用（断路器半开、高延迟） | 黄色圆点 + 警告图标 |
| `unavailable` | `#EF4444` (Red) | 不可用/断路 | 红色圆点 + 错误状态 |
| `unused` | `#6B7280` (Muted Gray) | 当前 Profile 未启用 | 灰色圆点 + 低对比度文字 |

#### 每个组件卡片结构

```
┌──────────────────────────────────────────┐
│  🟢  PostgreSQL                   16-alpine │  ← 颜色指示器 + 名称 + 版本
│      Connected  ·  Uptime: 18d 4h 12m      │  ← 状态 + Uptime
│      Latency: 2ms  ·  CB: Closed           │  ← 可选元数据行
│      ████████████████░░░  3.2G / 10G       │  ← 可选进度条（磁盘/内存）
└──────────────────────────────────────────┘
```

#### 紧凑模式 (推荐，节省空间)

```
🟢 PostgreSQL  Connected  18d 4h   ·  2ms
🟡 MinIO       Degraded  17d 2h   ·  CB: half_open
🔴 Docker      Down       1h 32m  ·  CB: open
⚪  K8s         Not configured
```

**采用紧凑单行模式**，在 2fr 列宽内可并排展示 10+ 个组件。

### 4.3 组件接口设计

```typescript
// shared/api.ts - AdminStatus 扩展
interface InfraComponent {
  name: string;                              // "PostgreSQL"
  key: string;                               // "postgres"
  icon: string;                              // emoji or SVG key
  healthy: boolean;
  status: 'connected' | 'degraded' | 'unavailable' | 'unused';
  uptime_seconds?: number;
  latency_ms?: number;
  message?: string;                          // "3.2G / 10G" or "CB: open"
  version?: string;                          // "16-alpine"
  circuit_breaker_state?: 'closed' | 'open' | 'half_open';
}

interface AdminStatus {
  // ... existing fields ...
  infrastructure: InfraComponent[];
}
```

### 4.4 新组件

#### `InfraHealthPanel.tsx` (新建)

```typescript
// 输入
interface InfraHealthPanelProps {
  components: InfraComponent[];
  loading?: boolean;
}

// 行为
- 渲染紧凑网格 (grid: 1fr, gap: 8px)
- 每个组件行: [状态灯] [名称] [状态标签] [Uptime] [元数据]
- 按健康 → 降级 → 故障 → 未使用的优先级排序
- 支持 tooltip 显示详细信息 (延迟、断路器状态、消息)
- 加载态使用 Skeleton 占位
- 空状态: "Infrastructure data unavailable"
```

---

## 五、实现计划

### Phase 1: 数据层 (后端)

| 任务 | 文件 | 工作量 |
|------|------|--------|
| 扩展 `AdminStatus` 结构体，添加 `Infrastructure []InfraStatus` | `internal/server/routes.go` | 小 |
| 从 `profile.OpenForge` 收集各适配器健康状态 | `internal/server/routes.go` `handleAdminStatus()` | 中 |
| 为已有健康检查的适配器添加统一 `HealthCheck()` 方法 | 各 `internal/adapter/*.go` | 中 |
| 各适配器记录启动时间实现 `Uptime()` | 各 `internal/adapter/*.go` | 小 |

**备选方案（无需后端改动）**：前端根据 `profile` + `circuit_breakers` + `ha` 推导组件状态，适用于快速原型。

### Phase 2: 前端类型 & API

| 任务 | 文件 | 工作量 |
|------|------|--------|
| 扩展 `AdminStatus` TypeScript 类型 | `frontend/src/shared/api.ts` | 小 |
| (可选) 添加前端推导逻辑 `deriveInfraHealth(AdminStatus)` | 新文件或 AdminPage 内 | 小 |

### Phase 3: 新组件

| 任务 | 文件 | 工作量 |
|------|------|--------|
| 创建 `InfraHealthPanel` 组件 | `frontend/src/features/admin/InfraHealthPanel.tsx` | 中 |
| 实现 4 色状态体系 + tooltip | 同上 | 小 |
| 实现 Uptime 格式化 (`18d 4h 12m` → `18d`) | 同上 | 小 |
| 无障碍：aria-label, focus, keyboard nav | 同上 | 小 |

### Phase 4: 集成到 AdminPage

| 任务 | 文件 | 工作量 |
|------|------|--------|
| 替换 `Milestones Complete` section | `frontend/src/features/admin/AdminPage.tsx` | 小 |
| 清理 `PHASES` / `milestoneGroups` / `doneCount` 相关代码 | 同上 | 小 |
| 确保 TypeScript 编译通过 | - | 小 |

### 组件设计详案

#### `InfraHealthPanel` 紧凑行设计

```
┌─────────────────────────────────────┐
│  Infrastructure Health              │  ← Section Header (11px muted uppercase)
│                                     │
│  🟢 PostgreSQL    Connected  18d    │  ← 紧凑行，hover 展开
│  🟢 Redis         Connected  18d    │
│  🟢 Docker        Connected  18d    │
│  🟢 gRPC IO       Connected  18d    │
│  🟡 MinIO         Degraded   17d    │  ← 断路器半开
│  🟢 Nginx         Connected  18d    │
│  🟢 Sandbox       Connected  18d    │
│  ⚪  Vault         Not configured    │  ← 未使用 Profile
│  ⚪  Feishu        Not configured    │
│  ⚪  K8s           Not configured    │
│  🟢 Telemetry     Connected  18d    │
│  🟢 DR Backup     OK         18d    │  ← 24h 内有备份
│                                     │
│  Last check: 10:29:45               │  ← 刷新时间
└─────────────────────────────────────┘
```

#### 每个 Row 的交互设计

```tsx
// 默认态：单行紧凑
<div style={{
  display: 'flex', alignItems: 'center', gap: 8,
  padding: '6px 10px', borderRadius: 4,
  background: '#111B2A', border: `1px solid ${tokens.border}`,
  cursor: 'pointer',
  fontSize: 12,
}}>
  {/* 状态灯 (6×6px 圆点, 健康时有 pulse 动画) */}
  <span className={healthy ? 'pulse-green' : ''} style={{
    width: 6, height: 6, borderRadius: '50%',
    background: statusColor, flexShrink: 0,
  }} />
  {/* 组件名 */}
  <span style={{ fontWeight: 600, width: 80, flexShrink: 0 }}>{name}</span>
  {/* 状态标签 */}
  <span style={{ color: statusColor, width: 90, flexShrink: 0, fontSize: 11 }}>
    {statusLabel}
  </span>
  {/* Uptime */}
  <span style={{ color: tokens.muted, flex: 1, textAlign: 'right', fontSize: 11 }}>
    {uptimeLabel}
  </span>
</div>

// Hover 态：在 row 下方展开 metadata tooltip
// 显示 Latency, Circuit Breaker, 版本号, 最后检查时间
```

#### 健康脉冲动画

```css
@keyframes infraPulse {
  0%, 100% { box-shadow: 0 0 0 0 rgba(34, 197, 94, 0.4); }
  50% { box-shadow: 0 0 0 4px rgba(34, 197, 94, 0); }
}
.pulse-green {
  animation: infraPulse 2s ease-in-out infinite;
}
```

---

## 六、UX 规范检查清单

基于 ui-ux-pro-max skill 规范：

### Accessibility (§1)
- [ ] 每个组件行有 `role="status"` 和 `aria-label="{name}: {status}"`
- [ ] 状态颜色不是唯一指示器（配合文字标签）
- [ ] Keyboard navigable (`tabIndex={0}`, `onKeyDown` Enter 展开详情)
- [ ] `prefers-reduced-motion` 时禁用 pulse 动画

### Touch & Interaction (§2)
- [ ] Row 高度 ≥ 32px（满足触控最小 44px 需要 padding 补偿）
- [ ] Tooltip/Hover 展开不依赖 hover only（mobile 用 tap）

### Performance (§3)
- [ ] 加载态使用 Skeleton（Skeleton variant="rect" 8 行）
- [ ] 列表虚拟化（12 个以内不需要，但预留接口）

### Color (§6)
- [ ] 绿色 `#22C55E` 在 dark bg `#0F172A` 上对比度 ≥ 4.5:1 ✓
- [ ] 黄色 `#F59E0B` 需搭配图标/文字，不单独用颜色传达信息
- [ ] 灰色 `#6B7280` 未使用组件降低不透明度至 0.5

### Animation (§7)
- [ ] Pulse 动画仅用于 healthy 状态灯，duration 2s infinite
- [ ] Hover 展开使用 opacity/transform，duration 200ms ease-out
- [ ] `@media (prefers-reduced-motion: reduce)` 禁用所有动画

---

## 七、文件变更清单

| 操作 | 文件 | 说明 |
|------|------|------|
| NEW | `frontend/src/features/admin/InfraHealthPanel.tsx` | 基础设施健康面板组件 |
| MODIFY | `frontend/src/features/admin/AdminPage.tsx` | 替换 Milestones Complete 为 InfraHealthPanel |
| MODIFY | `frontend/src/shared/api.ts` | 扩展 AdminStatus 类型（或推导逻辑） |
| MODIFY | `internal/server/routes.go` | 扩展 `/api/admin/status` 返回值（选做） |

---

## 八、风险评估

| 风险 | 级别 | 缓解措施 |
|------|------|---------|
| 后端无 per-service 健康数据 | 中 | 前端根据 profile + circuit_breakers 推导；后续再扩展后端 API |
| 紧凑布局信息过密 | 低 | 默认仅显示 2 行核心信息，hover 展开详情 |
| 组件过多超出 2fr 列高 | 低 | 12 个组件 × 28px ≈ 336px + 间距 ≈ 约 400px，适配当前布局 |
| 开发者模式看不到全部组件 | 低 | unused 状态组件以低对比度显示，保持布局完整性 |

---

## 九、执行方式

按照此计划文档，下一步将：

1. **先不修改后端**，前端自行推导基础设施状态（快速原型）
2. 创建 `InfraHealthPanel.tsx` 组件
3. 修改 `AdminPage.tsx` 替换 Milestones Complete
4. 验证 TypeScript 编译
5. 后续可选：扩展后端 API 获取真实 per-service 数据

---

## 十、附录：前端推导逻辑 (fallback)

当后端不提供 `infrastructure` 字段时，前端根据现有数据推导：

```typescript
function deriveInfraHealth(status: AdminStatus): InfraComponent[] {
  const { profile, circuit_breakers, ha } = status;
  const isStandard = profile === 'standard' || profile === 'enterprise';
  const isEnterprise = profile === 'enterprise';

  return [
    { key: 'postgres', name: 'PostgreSQL', /* always used */
      status: circuit_breakers?.postgres === 'open' ? 'unavailable'
            : circuit_breakers?.postgres === 'half_open' ? 'degraded' : 'connected' },
    { key: 'redis', name: 'Redis',
      status: isStandard ? (circuit_breakers?.redis === 'open' ? 'unavailable' : 'connected') : 'unused' },
    { key: 'docker', name: 'Docker',
      status: circuit_breakers?.docker === 'open' ? 'unavailable'
            : circuit_breakers?.docker === 'half_open' ? 'degraded' : 'connected' },
    { key: 'minio', name: 'MinIO',
      status: isStandard ? (circuit_breakers?.minio === 'open' ? 'unavailable'
            : circuit_breakers?.minio === 'half_open' ? 'degraded' : 'connected') : 'unused' },
    { key: 'vault', name: 'Vault',
      status: isStandard ? 'connected' : 'unused' },  // 无独立断路器
    { key: 'nginx', name: 'Nginx',
      status: isStandard ? 'connected' : 'unused' },
    { key: 'grpc_io', name: 'gRPC IO',
      status: 'connected' },
    { key: 'sandbox', name: 'Sandbox',
      status: isStandard ? 'connected' : 'unused' },
    { key: 'feishu', name: 'Feishu',
      status: isStandard ? 'connected' : 'unused' },
    { key: 'k8s', name: 'K8s',
      status: isEnterprise ? 'connected' : 'unused' },
    { key: 'telemetry', name: 'Telemetry',
      status: isStandard ? 'connected' : 'unused' },
    { key: 'dr_backup', name: 'DR Backup',
      status: 'connected' },  // PG 备份总是配置的
  ];
}
```
