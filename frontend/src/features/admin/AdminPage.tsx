import { useState, useEffect } from 'react';
import { useAuth, useRole } from '../../shared/auth';
import { api, AdminStatus, FeatureFlags } from '../../shared/api';
import { AppLayout } from '../../shared/AppLayout';
import { tokens } from '../../shared/design-tokens';
import SkillPanel from './SkillPanel';

export default function AdminPage() {
  const { user } = useAuth();
  const role = useRole();
  const [status, setStatus] = useState<AdminStatus | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [showSkillPanel, setShowSkillPanel] = useState(false);
  const [featureFlags, setFeatureFlags] = useState<FeatureFlags | null>(null);
  const [flagsLoading, setFlagsLoading] = useState(false);

  useEffect(() => {
    setLoading(true);
    setError(null);
    api.getAdminStatus()
      .then(data => {
        setStatus(data);
      })
      .catch(err => {
        setError(err instanceof Error ? err.message : 'Unable to fetch admin status');
        setStatus(null);
      })
      .finally(() => setLoading(false));

    api.getFeatureFlags()
      .then(ff => setFeatureFlags(ff))
      .catch(() => {}); // flags unavailable — show nothing
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

  if (showSkillPanel) {
    return <SkillPanel />;
  }

  return (
    <AppLayout>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 24 }}>
        <h1 style={{ fontSize: 24, fontWeight: 700, fontFamily: tokens.fontHeading, margin: 0, color: tokens.text }}>Admin Panel</h1>
        <button onClick={() => setShowSkillPanel(true)}
          style={{ padding: '8px 16px', background: tokens.surface, border: `1px solid ${tokens.border}`, borderRadius: 4, cursor: 'pointer', color: tokens.text, fontSize: 13 }}>
          Skill Management
        </button>
      </div>

      {/* Current Session */}
      <Section title="Current Session">
        <CardGrid>
          <InfoCard label="User" value={user?.id || '—'} />
          <InfoCard label="Role" value={role || '—'} capitalize />
          <InfoCard label="Token Expires" value={loginInfo?.exp ? loginInfo.exp.toLocaleTimeString() : '—'} />
          <InfoCard label="RBAC via" value={role === 'admin' ? 'Admin Bypass' : 'RequireRole Middleware'} />
        </CardGrid>
      </Section>

      {/* System Status */}
      <Section title="System Status">
        {loading ? (
          <div style={{ textAlign: 'center', padding: 32, color: tokens.muted, fontSize: 14 }}>
            Loading system status...
          </div>
        ) : error ? (
          <div style={{ textAlign: 'center', padding: 32, color: tokens.error, fontSize: 14 }}>
            {error}
          </div>
        ) : (
          <>
            <CardGrid>
              <StatusCard label="RBAC Middleware" status={status?.rbac === 'active' ? 'on' : status?.rbac ? 'error' : 'off'} />
              <StatusCard label="OIDC Provider" status={status?.oidc === 'enabled' ? 'on' : 'off'} detail={status?.oidc === 'enabled' ? 'Enterprise SSO' : 'JWT (dev)'} />
              <StatusCard label="Audit Hash Chain" status="on" />
              <StatusCard label="Module Ownership" status="on" detail="2D reviewer routing" />
              <StatusCard label="Profile" status="on" detail={status?.profile || '—'} />
              <StatusCard label="Security Tier" status="on" detail={status?.tier || '—'} />
            </CardGrid>

            {/* SLO data */}
            {status?.slo && (
              <div style={{ marginTop: 16 }}>
                <h3 style={{ fontSize: 13, color: tokens.muted, marginBottom: 8 }}>SLO Performance</h3>
                <CardGrid>
                  <InfoCard label="Total Requests" value={String(status.slo.total)} />
                  <InfoCard label="Success Rate" value={`${(status.slo.success_rate * 100).toFixed(1)}%`} />
                  {status.slo.p95_ms !== undefined && (
                    <InfoCard label="P95 Latency" value={`${status.slo.p95_ms}ms`} />
                  )}
                </CardGrid>
              </div>
            )}

            {/* HA data */}
            {status?.ha && (
              <div style={{ marginTop: 16 }}>
                <h3 style={{ fontSize: 13, color: tokens.muted, marginBottom: 8 }}>High Availability</h3>
                <CardGrid>
                  <InfoCard label="Task Queue" value={status.ha.task_queue} />
                  <InfoCard label="Hash Ring Nodes" value={String(status.ha.hash_ring_nodes)} />
                  <InfoCard label="Load Shedding" value={status.ha.load_shedding} />
                </CardGrid>
              </div>
            )}

            {/* Circuit Breakers */}
            {status?.circuit_breakers && Object.keys(status.circuit_breakers).length > 0 && (
              <div style={{ marginTop: 16 }}>
                <h3 style={{ fontSize: 13, color: tokens.muted, marginBottom: 8 }}>Circuit Breakers</h3>
                <CardGrid>
                  {Object.entries(status.circuit_breakers).map(([name, state]) => (
                    <StatusCard key={name} label={name} status={state === 'closed' ? 'on' : state === 'open' ? 'error' : 'off'} detail={state} />
                  ))}
                </CardGrid>
              </div>
            )}
          </>
        )}
      </Section>

      {/* Role Hierarchy */}
      <Section title="Role Hierarchy">
        <div style={{ display: 'flex', alignItems: 'center', flexWrap: 'wrap', gap: 0 }}>
          {[
            { role: 'admin', color: '#7C3AED', desc: 'Bypass all checks' },
            // pm and dev_lead are peers — side by side
          ].map((item, i, arr) => (
            <div key={item.role} style={{ display: 'flex', alignItems: 'center', gap: 10 }}>
              <div style={{
                background: tokens.surface, borderRadius: 10, padding: '12px 16px',
                border: `1px solid ${tokens.border}`, minWidth: 140, display: 'flex', alignItems: 'center', gap: 10,
              }}>
                <div style={{ width: 12, height: 12, borderRadius: '50%', backgroundColor: item.color, flexShrink: 0 }} />
                <div style={{ flex: 1 }}>
                  <div style={{ fontSize: 14, fontWeight: 700, color: item.color, display: 'flex', alignItems: 'center', gap: 6 }}>
                    {item.role}
                    {item.role === role && <span style={{ fontSize: 9, fontWeight: 700, color: tokens.text, background: tokens.border, padding: '1px 5px', borderRadius: 4 }}>YOU</span>}
                  </div>
                  <div style={{ fontSize: 11, color: '#64748B', marginTop: 2 }}>{item.desc}</div>
                </div>
              </div>
              {i < arr.length - 1 && <div style={{ fontSize: 18, color: '#475569', margin: '0 4px' }}>→</div>}
            </div>
          ))}
          {/* pm + dev_lead as peers */}
          <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'center', margin: '0 4px' }}>
            <div style={{ fontSize: 18, color: '#475569' }}>→</div>
          </div>
          <div style={{ display: 'flex', gap: 8 }}>
            {[
              { role: 'pm', color: '#2563EB', desc: 'Create pipelines, view costs' },
              { role: 'dev_lead', color: '#0891B2', desc: 'Approve gates, review inbox' },
            ].map(item => (
              <div key={item.role} style={{
                background: tokens.surface, borderRadius: 10, padding: '12px 16px',
                border: `1px solid ${tokens.border}`, minWidth: 140,
              }}>
                <div style={{ display: 'flex', alignItems: 'center', gap: 10 }}>
                  <div style={{ width: 12, height: 12, borderRadius: '50%', backgroundColor: item.color, flexShrink: 0 }} />
                  <div>
                    <div style={{ fontSize: 14, fontWeight: 700, color: item.color, display: 'flex', alignItems: 'center', gap: 6 }}>
                      {item.role}
                      {item.role === role && <span style={{ fontSize: 9, fontWeight: 700, color: tokens.text, background: tokens.border, padding: '1px 5px', borderRadius: 4 }}>YOU</span>}
                    </div>
                    <div style={{ fontSize: 11, color: '#64748B', marginTop: 2 }}>{item.desc}</div>
                  </div>
                </div>
              </div>
            ))}
          </div>
          <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'center', margin: '0 4px' }}>
            <div style={{ fontSize: 18, color: '#475569' }}>→</div>
          </div>
          {[
            { role: 'dev', color: '#059669', desc: 'Read-only on review features' },
          ].map((item, i, arr) => (
            <div key={item.role} style={{ display: 'flex', alignItems: 'center', gap: 10 }}>
              <div style={{
                background: tokens.surface, borderRadius: 10, padding: '12px 16px',
                border: `1px solid ${tokens.border}`, minWidth: 140, display: 'flex', alignItems: 'center', gap: 10,
              }}>
                <div style={{ width: 12, height: 12, borderRadius: '50%', backgroundColor: item.color, flexShrink: 0 }} />
                <div style={{ flex: 1 }}>
                  <div style={{ fontSize: 14, fontWeight: 700, color: item.color, display: 'flex', alignItems: 'center', gap: 6 }}>
                    {item.role}
                    {item.role === role && <span style={{ fontSize: 9, fontWeight: 700, color: tokens.text, background: tokens.border, padding: '1px 5px', borderRadius: 4 }}>YOU</span>}
                  </div>
                  <div style={{ fontSize: 11, color: '#64748B', marginTop: 2 }}>{item.desc}</div>
                </div>
              </div>
              {i < arr.length - 1 && <div style={{ fontSize: 18, color: '#475569', margin: '0 4px' }}>→</div>}
            </div>
          ))}
          <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'center', margin: '0 4px' }}>
            <div style={{ fontSize: 18, color: '#475569' }}>→</div>
          </div>
          {[
            { role: 'observer', color: '#6B7280', desc: 'Read-only access' },
          ].map(item => (
            <div key={item.role} style={{
              background: tokens.surface, borderRadius: 10, padding: '12px 16px',
              border: `1px solid ${tokens.border}`, minWidth: 140, display: 'flex', alignItems: 'center', gap: 10,
            }}>
              <div style={{ width: 12, height: 12, borderRadius: '50%', backgroundColor: item.color, flexShrink: 0 }} />
              <div style={{ flex: 1 }}>
                <div style={{ fontSize: 14, fontWeight: 700, color: item.color, display: 'flex', alignItems: 'center', gap: 6 }}>
                  {item.role}
                  {item.role === role && <span style={{ fontSize: 9, fontWeight: 700, color: tokens.text, background: tokens.border, padding: '1px 5px', borderRadius: 4 }}>YOU</span>}
                </div>
                <div style={{ fontSize: 11, color: '#64748B', marginTop: 2 }}>{item.desc}</div>
              </div>
            </div>
          ))}
        </div>
      </Section>

      {/* Phase Progress — All 10 Phases */}
      <Section title="Phase Progress">
        <div style={{ display: 'flex', flexDirection: 'column', gap: 4 }}>
          {[
            { phase: 'Phase 1', label: 'CLI + Minimal Profile + 10 API Stubs', done: true },
            { phase: 'Phase 2', label: 'Web Chat + BFF Auth (JWT/CORS/XSS)', done: true },
            { phase: 'Phase 3', label: 'Pipeline State Machine + Diff Preview + Gate/Approve', done: true },
            { phase: 'Phase 4', label: 'Docker Sandbox (5-Layer) + Cost Dashboard + Onboarding', done: true },
            { phase: 'Phase 5', label: 'Redis Task Queue + Gate Hooks + Multi-Coordinator', done: true },
            { phase: 'Phase 6', label: 'RBAC + SSO/OIDC + Audit Hash Chain + Learning Engine', done: true },
            { phase: 'Phase 7', label: 'OTel + ToolRegistry + Prometheus Metrics + AB Testing', done: true },
            { phase: 'Phase 8', label: 'HA + Circuit Breaker + Load Shedding + Sharding + SLO', done: true },
            { phase: 'Phase 9', label: 'Full Workbench + Enterprise Profile + K8s Container Runtime', done: false },
            { phase: 'Phase 10', label: 'Compliance Reports + Runbook + Offline Package + DR Region', done: false },
          ].map(item => (
            <div key={item.phase} style={{
              display: 'flex', alignItems: 'flex-start', gap: 14, padding: '14px 16px',
              background: tokens.surface, borderRadius: 10, border: `1px solid ${tokens.border}`,
            }}>
              <div style={{
                width: 14, height: 14, borderRadius: '50%', border: '2px solid',
                borderColor: item.done ? tokens.cta : '#F59E0B',
                backgroundColor: item.done ? tokens.cta : 'transparent',
                flexShrink: 0, marginTop: 2,
              }} />
              <div>
                <div style={{ fontSize: 13, fontWeight: 700, color: '#E2E8F0', display: 'flex', alignItems: 'center', gap: 8 }}>
                  {item.phase}
                  {item.done ? (
                    <span style={{ fontSize: 10, fontWeight: 600, color: tokens.cta, background: `${tokens.cta}18`, padding: '1px 7px', borderRadius: 4 }}>DONE</span>
                  ) : (
                    <span style={{ fontSize: 10, fontWeight: 600, color: '#F59E0B', background: '#F59E0B18', padding: '1px 7px', borderRadius: 4 }}>PLANNED</span>
                  )}
                </div>
                <div style={{ fontSize: 12, color: '#64748B', marginTop: 2 }}>{item.label}</div>
              </div>
            </div>
          ))}
        </div>
      </Section>

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
    </AppLayout>
  );
}

function Section({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <section style={{ marginBottom: 32 }}>
      <h2 style={{ fontSize: 16, fontWeight: 600, color: tokens.muted, marginBottom: 16, letterSpacing: '-0.01em' }}>{title}</h2>
      {children}
    </section>
  );
}

function CardGrid({ children }: { children: React.ReactNode }) {
  return (
    <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fill, minmax(180px, 1fr))', gap: 12 }}>
      {children}
    </div>
  );
}

function InfoCard({ label, value, capitalize }: { label: string; value: string; capitalize?: boolean }) {
  return (
    <div style={{ background: tokens.surface, borderRadius: 8, padding: '16px 20px', border: `1px solid ${tokens.border}` }}>
      <div style={{ fontSize: 11, fontWeight: 500, color: '#64748B', textTransform: 'uppercase', letterSpacing: '0.04em', marginBottom: 8 }}>{label}</div>
      <div style={{ fontSize: 18, fontWeight: 600, color: '#F1F5F9', textTransform: capitalize ? 'capitalize' : undefined }}>{value}</div>
    </div>
  );
}

function StatusCard({ label, status, detail }: { label: string; status: 'on' | 'off' | 'error'; detail?: string }) {
  return (
    <div style={{ background: tokens.surface, borderRadius: 8, padding: '16px 20px', border: `1px solid ${tokens.border}`, display: 'flex', flexDirection: 'column', gap: 8 }}>
      <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
        <div style={{
          width: 10, height: 10, borderRadius: '50%', flexShrink: 0,
          backgroundColor: status === 'on' ? tokens.cta : status === 'error' ? tokens.error : '#6B7280',
        }} />
        <div style={{ fontSize: 13, fontWeight: 600, color: '#E2E8F0' }}>{label}</div>
      </div>
      <div style={{ fontSize: 12, fontWeight: 500, color: '#94A3B8' }}>
        {status === 'on' ? 'Active' : status === 'error' ? 'Error' : 'Disabled'}
      </div>
      {detail && <div style={{ fontSize: 11, color: '#64748B', marginTop: -2 }}>{detail}</div>}
    </div>
  );
}
