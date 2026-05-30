import { useState, useEffect } from 'react';
import { useNavigate } from 'react-router-dom';
import { useAuth, useRole } from '../../shared/auth';
import { api, AdminStatus, FeatureFlags } from '../../shared/api';
import { AppLayout } from '../../shared/AppLayout';
import { tokens } from '../../shared/design-tokens';
import { Skeleton, SkeletonGrid } from '../../shared/skeleton';
import { StatusBadge } from '../../shared/StatusBadge';
import { RolePyramid } from './RolePyramid';
import { FeatureCard } from './FeatureCard';
import { SloBar } from './SloBar';
import { HaPills } from './HaPills';
import { CircuitBreakerStrip } from './CircuitBreakerStrip';
import InfraHealthPanel from './InfraHealthPanel';
import { deriveInfraHealth } from '../../shared/api';

const FEATURE_GROUPS = [
  {
    key: 'enterprise_platform' as keyof FeatureFlags,
    title: 'Enterprise Platform',
    desc: 'Enterprise-grade infrastructure stack',
    items: ['Vault', 'K8s', 'MinIO', 'Multi-Region DR', 'Helm'],
    color: '#7C3AED',
    icon: '\u{1F3E2}',
  },
  {
    key: 'compliance_suite' as keyof FeatureFlags,
    title: 'Compliance Suite',
    desc: 'Regulatory compliance & data governance',
    items: ['Monthly Reports', 'Data Lifecycle'],
    color: '#0891B2',
    icon: '\u{1F4CB}',
  },
  {
    key: 'production_ops' as keyof FeatureFlags,
    title: 'Production Operations',
    desc: 'Monitoring, alerting & operational runbooks',
    items: ['Grafana', 'Runbooks', 'Notifier'],
    color: '#F59E0B',
    icon: '\u{2699}\u{FE0F}',
  },
  {
    key: 'distribution_artifacts' as keyof FeatureFlags,
    title: 'Distribution & Docs',
    desc: 'Offline deployment & documentation',
    items: ['Offline Package', 'ADR + OpenAPI'],
    color: '#10B981',
    icon: '\u{1F4E6}',
  },
];

const ROLE_NODES = [
  { role: 'admin', color: '#7C3AED', description: 'Bypass all checks' },
  {
    role: '', color: '', description: '',
    peers: [
      { role: 'pm', color: '#2563EB', description: 'Pipeline & costs' },
      { role: 'dev_lead', color: '#0891B2', description: 'Approve & review' },
    ],
  },
  { role: 'dev', color: '#059669', description: 'Read-only on reviews' },
  { role: 'observer', color: '#6B7280', description: 'Read-only access' },
];

export default function AdminPage() {
  const { user } = useAuth();
  const role = useRole();
  const navigate = useNavigate();
  const [status, setStatus] = useState<AdminStatus | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [featureFlags, setFeatureFlags] = useState<FeatureFlags | null>(null);
  const [flagsLoading, setFlagsLoading] = useState(false);

  useEffect(() => {
    setLoading(true);
    setError(null);
    api.getAdminStatus()
      .then(data => { setStatus(data); })
      .catch(err => {
        setError(err instanceof Error ? err.message : 'Unable to fetch admin status');
        setStatus(null);
      })
      .finally(() => setLoading(false));

    api.getFeatureFlags()
      .then(ff => setFeatureFlags(ff))
      .catch(() => {});
  }, []);

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

  const loginInfo = user ? (() => {
    try {
      const token = localStorage.getItem('of_token') || '';
      const parts = token.split('.');
      const payload = JSON.parse(atob(parts[1] || '{}'));
      return { uid: payload.uid, role: payload.role, exp: payload.exp ? new Date(payload.exp * 1000) : null };
    } catch { return null; }
  })() : null;

  const infraComponents = status ? deriveInfraHealth(status) : [];

  return (
    <AppLayout>
      {/* Header */}
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 24 }}>
        <h1 style={{ fontSize: 24, fontWeight: 700, fontFamily: tokens.fontHeading, margin: 0, color: tokens.text }}>Admin Panel</h1>
        <div style={{ display: 'flex', gap: 8 }}>
          <button onClick={() => navigate('/admin/invitations')}
            style={{ padding: '8px 16px', background: tokens.surface, border: `1px solid ${tokens.border}`, borderRadius: 4, cursor: 'pointer', color: tokens.text, fontSize: 13 }}>
            Invitations →
          </button>
          <button onClick={() => navigate('/admin/skills')}
            style={{ padding: '8px 16px', background: tokens.surface, border: `1px solid ${tokens.border}`, borderRadius: 4, cursor: 'pointer', color: tokens.text, fontSize: 13 }}>
            Skill Management →
          </button>
        </div>
      </div>

      {/* Current Session */}
      <Section title="Current Session">
        <div style={{ display: 'grid', gridTemplateColumns: 'repeat(4, 1fr)', gap: 10 }}>
          <InfoCard label="User" value={user?.id || '—'} />
          <InfoCard label="Role" value={role || '—'} capitalize />
          <InfoCard label="Token Expires" value={loginInfo?.exp ? loginInfo.exp.toLocaleTimeString() : '—'} />
          <InfoCard label="RBAC via" value={role === 'admin' ? 'Admin Bypass' : 'RequireRole Middleware'} />
        </div>
      </Section>

      {/* Two-column layout */}
      <div style={{ display: 'grid', gridTemplateColumns: '3fr 2fr', gap: 24 }}>
        {/* LEFT COLUMN */}
        <div>
          {/* System Status */}
          <Section title="System Status">
            {loading ? (
              <SkeletonGrid count={6} variant="card" />
            ) : error ? (
              <div role="alert" style={{ textAlign: 'center', padding: 32, color: tokens.error, fontSize: 14 }}>
                ⚠️ {error}
              </div>
            ) : (
              <>
                {/* Compact Status Badges */}
                <div style={{ display: 'flex', flexWrap: 'wrap', gap: 6, marginBottom: 16 }}>
                  <StatusBadge label="RBAC" status={status?.rbac === 'active' ? 'active' : status?.rbac ? 'error' : 'inactive'} detail="Role-based access" />
                  <StatusBadge label="OIDC" status={status?.oidc === 'enabled' ? 'active' : 'inactive'} detail={status?.oidc === 'enabled' ? 'Enterprise SSO' : 'JWT (dev)'} />
                  <StatusBadge label="Audit Chain" status="active" detail="Hash chain running" />
                  <StatusBadge label="Module Ownership" status="active" detail="2D reviewer routing" />
                  <StatusBadge label="Profile" status="active" detail={status?.profile || '—'} />
                  <StatusBadge label="Security Tier" status="active" detail={status?.tier || '—'} />
                </div>

                {/* SLO Mini Bars */}
                {status?.slo && (
                  <div style={{ marginBottom: 16 }}>
                    <h3 style={{ fontSize: 11, fontWeight: 600, color: tokens.muted, textTransform: 'uppercase', letterSpacing: '0.06em', marginBottom: 8 }}>SLO Performance</h3>
                    <SloBar label="Total" value={String(status.slo.total)} max={100000} current={status.slo.total} target="max 100K" color="#2563EB" />
                    <SloBar label="Success" value={`${(status.slo.success_rate * 100).toFixed(1)}%`} max={100} current={status.slo.success_rate * 100} target="SLO ≥99.5%" />
                    {status.slo.p95_ms !== undefined && (
                      <SloBar label="P95 Lat." value={`${status.slo.p95_ms}ms`} max={200} current={status.slo.p95_ms} target="SLO ≤200ms" color="#0891B2" />
                    )}
                  </div>
                )}

                {/* HA Pills */}
                {status?.ha && (
                  <div style={{ marginBottom: 16 }}>
                    <h3 style={{ fontSize: 11, fontWeight: 600, color: tokens.muted, textTransform: 'uppercase', letterSpacing: '0.06em', marginBottom: 8 }}>High Availability</h3>
                    <HaPills pills={[
                      { label: `Queue: ${status.ha.task_queue}`, status: 'active' },
                      { label: `Nodes: ${status.ha.hash_ring_nodes}`, status: 'active' },
                      { label: `Load: ${status.ha.load_shedding}`, status: status.ha.load_shedding.includes('%') && parseInt(status.ha.load_shedding) > 80 ? 'warning' : 'active' },
                    ]} />
                  </div>
                )}

                {/* Infrastructure Health */}
                <div style={{ marginTop: 16 }}>
                  <InfraHealthPanel components={infraComponents} loading={loading} />
                </div>
              </>
            )}
          </Section>
        </div>

        {/* RIGHT COLUMN */}
        <div>
          {/* Role Hierarchy */}
          <Section title="Role Hierarchy">
            <RolePyramid nodes={ROLE_NODES} currentUserRole={role || undefined} />
          </Section>

          {/* Circuit Breakers */}
          <Section title="Circuit Breakers">
            {status?.circuit_breakers && Object.keys(status.circuit_breakers).length > 0 ? (
              <CircuitBreakerStrip breakers={status.circuit_breakers as Record<string, 'closed' | 'open' | 'half_open'>} />
            ) : (
              <div style={{ textAlign: 'center', padding: 16, color: tokens.muted, fontSize: 12, border: `1px dashed ${tokens.border}`, borderRadius: 8 }}>
                No circuit breaker data
              </div>
            )}
          </Section>
        </div>
      </div>

      {/* Enterprise Feature Toggles — 2×2 */}
      <Section title="Enterprise Feature Toggles">
        {featureFlags === null ? (
          <div style={{ textAlign: 'center', padding: 24, color: tokens.muted, fontSize: 14 }}>
            Feature flags unavailable — check admin access
          </div>
        ) : (
          <>
            <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 10 }}>
              {FEATURE_GROUPS.map(group => (
                <FeatureCard
                  key={group.key}
                  title={group.title}
                  description={group.desc}
                  features={group.items}
                  enabled={!!featureFlags[group.key]}
                  color={group.color}
                  icon={group.icon}
                  onToggle={() => handleToggleFlag(group.key)}
                  loading={flagsLoading}
                />
              ))}
            </div>
            {/* Batch operations */}
            <div style={{ display: 'flex', gap: 8, marginTop: 12 }}>
              <button
                onClick={() => FEATURE_GROUPS.forEach(g => handleToggleFlag(g.key).catch(() => {}))}
                style={{ padding: '5px 14px', border: `1px solid ${tokens.border}`, borderRadius: 4, background: 'transparent', color: tokens.text, fontSize: 12, cursor: 'pointer' }}
              >
                Enable All
              </button>
              <button
                onClick={() => FEATURE_GROUPS.forEach(g => {
                  if (featureFlags?.[g.key]) handleToggleFlag(g.key);
                })}
                style={{ padding: '5px 14px', border: `1px solid ${tokens.border}`, borderRadius: 4, background: 'transparent', color: tokens.text, fontSize: 12, cursor: 'pointer' }}
              >
                Disable All
              </button>
            </div>
          </>
        )}
      </Section>
    </AppLayout>
  );
}

/* ----- Helper components ----- */

function Section({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <section style={{ marginBottom: 28 }}>
      <h2 style={{ fontSize: 11, fontWeight: 600, color: tokens.muted, textTransform: 'uppercase', letterSpacing: '0.06em', marginBottom: 12 }}>
        {title}
      </h2>
      {children}
    </section>
  );
}

function InfoCard({ label, value, capitalize }: { label: string; value: string; capitalize?: boolean }) {
  return (
    <div style={{ background: '#111B2A', border: `1px solid ${tokens.border}`, borderRadius: 8, padding: '14px 18px' }}>
      <div style={{ fontSize: 10, fontWeight: 600, color: tokens.muted, textTransform: 'uppercase', letterSpacing: '0.05em', marginBottom: 6 }}>{label}</div>
      <div style={{ fontSize: 16, fontWeight: 700, color: tokens.text, textTransform: capitalize ? 'capitalize' : undefined }}>{value}</div>
    </div>
  );
}
