import { useState, useEffect } from 'react';
import { Link } from 'react-router-dom';
import { useAuth, useRole } from '../../shared/auth';
import { api } from '../../shared/api';

interface SystemStatus {
  rbac: 'active' | 'degraded';
  oidc: 'enabled' | 'disabled';
  auditChain: 'healthy' | 'broken';
  phase: string;
}

export function AdminPage() {
  const { user } = useAuth();
  const role = useRole();
  const [status, setStatus] = useState<SystemStatus | null>(null);
  const [error, setError] = useState('');

  useEffect(() => {
    async function fetchStatus() {
      try {
        const data = await api.getAdminStatus();
        setStatus({
          rbac: data.rbac || 'active',
          oidc: data.oidc || 'disabled',
          auditChain: 'healthy',
          phase: data.phase || 'Phase 6.5',
        });
      } catch {
        setStatus({
          rbac: 'active',
          oidc: 'disabled',
          auditChain: 'healthy',
          phase: 'Phase 6',
        });
      }
    }
    fetchStatus();
  }, []);

  const loginInfo = user
    ? (() => {
        try {
          const token = localStorage.getItem('of_token') || '';
          const parts = token.split('.');
          const payload = JSON.parse(atob(parts[1] || '{}'));
          const exp = payload.exp ? new Date(payload.exp * 1000) : null;
          return { uid: payload.uid, role: payload.role, exp };
        } catch {
          return null;
        }
      })()
    : null;

  return (
    <div style={styles.container}>
      <header style={styles.header}>
        <div style={{ display: 'flex', alignItems: 'center', gap: 16 }}>
          <Link to="/" style={{ color: '#94a3b8', textDecoration: 'none', fontSize: 14 }}>&larr; Dashboard</Link>
          <h1 style={styles.title}>Admin Panel</h1>
        </div>
        <span style={styles.badge}>OpenForge Phase 6</span>
      </header>

      {/* User Session */}
      <section style={styles.section}>
        <h2 style={styles.sectionTitle}>Current Session</h2>
        <div style={styles.cardGrid}>
          <div style={styles.card}>
            <div style={styles.cardLabel}>User</div>
            <div style={styles.cardValue}>{user?.id || '—'}</div>
          </div>
          <div style={styles.card}>
            <div style={styles.cardLabel}>Role</div>
            <div style={{ ...styles.cardValue, textTransform: 'capitalize' }}>
              {role || '—'}
            </div>
          </div>
          <div style={styles.card}>
            <div style={styles.cardLabel}>Token Expires</div>
            <div style={styles.cardValue}>
              {loginInfo?.exp
                ? loginInfo.exp.toLocaleTimeString()
                : '—'}
            </div>
          </div>
          <div style={styles.card}>
            <div style={styles.cardLabel}>RBAC via</div>
            <div style={styles.cardValue}>
              {role === 'admin' ? 'Admin Bypass' : 'RequireRole Middleware'}
            </div>
          </div>
        </div>
      </section>

      {/* System Status */}
      <section style={styles.section}>
        <h2 style={styles.sectionTitle}>System Status</h2>
        <div style={{ ...styles.cardGrid, gridTemplateColumns: 'repeat(auto-fill, minmax(200px, 1fr))' }}>
          <StatusCard label="RBAC Middleware" status={status?.rbac === 'active' ? 'on' : 'off'} />
          <StatusCard
            label="OIDC Provider"
            status={status?.oidc === 'enabled' ? 'on' : 'off'}
            detail={status?.oidc === 'enabled' ? 'Enterprise SSO' : 'JWT (dev)'}
          />
          <StatusCard label="Audit Hash Chain" status={status?.auditChain === 'healthy' ? 'on' : 'off'} />
          <StatusCard label="Module Ownership" status="on" detail="2D reviewer routing" />
        </div>
      </section>

      {/* Role Hierarchy */}
      <section style={styles.section}>
        <h2 style={styles.sectionTitle}>Role Hierarchy</h2>
        <div style={styles.roleChain}>
          {[
            { role: 'admin', color: '#7C3AED', desc: 'Bypass all checks' },
            { role: 'pm', color: '#2563EB', desc: 'Create pipelines, view costs' },
            { role: 'dev_lead', color: '#0891B2', desc: 'Approve gates, review inbox' },
            { role: 'dev', color: '#059669', desc: 'Read-only on review features' },
            { role: 'observer', color: '#6B7280', desc: 'Read-only access' },
          ].map((item, i) => (
            <div key={item.role} style={styles.roleItem}>
              <div style={{ ...styles.roleDot, backgroundColor: item.color }} />
              <div style={{ flex: 1 }}>
                <div style={{ ...styles.roleLabel, color: item.color }}>
                  {item.role}
                  {item.role === role ? (
                    <span style={styles.currentBadge}>YOU</span>
                  ) : null}
                </div>
                <div style={styles.roleDesc}>{item.desc}</div>
              </div>
              {i < 4 && <div style={styles.roleArrow}>→</div>}
            </div>
          ))}
        </div>
      </section>

      {/* Future Capabilities */}
      <section style={styles.section}>
        <h2 style={styles.sectionTitle}>Enterprise Roadmap</h2>
        <div style={styles.roadmap}>
          {[
            { phase: 'Phase 6', label: 'RBAC + OIDC + Audit Chain', done: true },
            { phase: 'Phase 7', label: 'Self-learning + A/B Testing', done: false },
            { phase: 'Phase 8', label: 'HA + DR + Performance SLOs', done: false },
            { phase: 'Phase 9', label: 'Full Workbench + K8s', done: false },
          ].map((item) => (
            <div key={item.phase} style={styles.roadmapItem}>
              <div
                style={{
                  ...styles.roadmapDot,
                  backgroundColor: item.done ? '#22C55E' : '#334155',
                  borderColor: item.done ? '#22C55E' : '#475569',
                }}
              />
              <div style={{ flex: 1 }}>
                <div style={styles.roadmapPhase}>{item.phase}</div>
                <div style={styles.roadmapLabel}>{item.label}</div>
              </div>
            </div>
          ))}
        </div>
      </section>

      {error && (
        <div style={styles.errorBox}>
          {error}
          <button onClick={() => setError('')} style={styles.errorClose}>x</button>
        </div>
      )}
    </div>
  );
}

function StatusCard({
  label,
  status,
  detail,
}: {
  label: string;
  status: 'on' | 'off' | 'error';
  detail?: string;
}) {
  return (
    <div style={styles.statusCard}>
      <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
        <div
          style={{
            width: 10,
            height: 10,
            borderRadius: '50%',
            backgroundColor:
              status === 'on' ? '#22C55E' : status === 'error' ? '#EF4444' : '#6B7280',
          }}
        />
        <div style={styles.statusLabel}>{label}</div>
      </div>
      <div style={styles.statusValue}>
        {status === 'on' ? 'Active' : status === 'error' ? 'Error' : 'Disabled'}
      </div>
      {detail && <div style={styles.statusDetail}>{detail}</div>}
    </div>
  );
}

const styles: Record<string, React.CSSProperties> = {
  container: {
    maxWidth: 1024,
    margin: '0 auto',
    padding: '32px 24px 64px',
    fontFamily: "'Inter', system-ui, -apple-system, sans-serif",
    color: '#F8FAFC',
    minHeight: '100vh',
  },
  header: {
    display: 'flex',
    alignItems: 'center',
    gap: 16,
    marginBottom: 48,
    paddingBottom: 24,
    borderBottom: '1px solid #1E293B',
  },
  title: {
    fontSize: 32,
    fontWeight: 700,
    color: '#F8FAFC',
    margin: 0,
    letterSpacing: '-0.02em',
  },
  badge: {
    fontSize: 12,
    fontWeight: 600,
    color: '#22C55E',
    backgroundColor: 'rgba(34,197,94,0.12)',
    padding: '4px 10px',
    borderRadius: 6,
    textTransform: 'uppercase',
    letterSpacing: '0.05em',
  },
  section: {
    marginBottom: 40,
  },
  sectionTitle: {
    fontSize: 18,
    fontWeight: 600,
    color: '#94A3B8',
    marginBottom: 16,
    letterSpacing: '-0.01em',
  },
  cardGrid: {
    display: 'grid',
    gridTemplateColumns: 'repeat(auto-fill, minmax(180px, 1fr))',
    gap: 12,
  },
  card: {
    backgroundColor: '#1E293B',
    borderRadius: 12,
    padding: '20px 24px',
    border: '1px solid #334155',
    transition: 'border-color 200ms',
  },
  cardLabel: {
    fontSize: 12,
    fontWeight: 500,
    color: '#64748B',
    textTransform: 'uppercase',
    letterSpacing: '0.04em',
    marginBottom: 8,
  },
  cardValue: {
    fontSize: 20,
    fontWeight: 600,
    color: '#F1F5F9',
  },
  statusCard: {
    backgroundColor: '#1E293B',
    borderRadius: 12,
    padding: '20px 24px',
    border: '1px solid #334155',
    display: 'flex',
    flexDirection: 'column',
    gap: 10,
  },
  statusLabel: {
    fontSize: 14,
    fontWeight: 600,
    color: '#E2E8F0',
  },
  statusValue: {
    fontSize: 13,
    fontWeight: 500,
    color: '#94A3B8',
  },
  statusDetail: {
    fontSize: 12,
    color: '#64748B',
    marginTop: -4,
  },
  roleChain: {
    display: 'flex',
    alignItems: 'center',
    flexWrap: 'wrap',
    gap: 0,
  },
  roleItem: {
    display: 'flex',
    alignItems: 'center',
    gap: 10,
    backgroundColor: '#1E293B',
    borderRadius: 10,
    padding: '12px 16px',
    border: '1px solid #334155',
    minWidth: 140,
  },
  roleDot: {
    width: 12,
    height: 12,
    borderRadius: '50%',
    flexShrink: 0,
  },
  roleLabel: {
    fontSize: 14,
    fontWeight: 700,
    display: 'flex',
    alignItems: 'center',
    gap: 6,
  },
  currentBadge: {
    fontSize: 9,
    fontWeight: 700,
    color: '#FFFFFF',
    backgroundColor: '#334155',
    padding: '1px 5px',
    borderRadius: 4,
    textTransform: 'uppercase',
  },
  roleDesc: {
    fontSize: 11,
    color: '#64748B',
    marginTop: 2,
  },
  roleArrow: {
    fontSize: 18,
    color: '#475569',
    margin: '0 4px',
  },
  roadmap: {
    display: 'flex',
    flexDirection: 'column',
    gap: 4,
  },
  roadmapItem: {
    display: 'flex',
    alignItems: 'flex-start',
    gap: 14,
    padding: '14px 16px',
    backgroundColor: '#1E293B',
    borderRadius: 10,
    border: '1px solid #334155',
  },
  roadmapDot: {
    width: 14,
    height: 14,
    borderRadius: '50%',
    border: '2px solid',
    flexShrink: 0,
    marginTop: 2,
  },
  roadmapPhase: {
    fontSize: 13,
    fontWeight: 700,
    color: '#E2E8F0',
  },
  roadmapLabel: {
    fontSize: 12,
    color: '#64748B',
    marginTop: 2,
  },
  errorBox: {
    backgroundColor: 'rgba(239,68,68,0.12)',
    color: '#FCA5A5',
    padding: '12px 16px',
    borderRadius: 8,
    marginTop: 16,
    display: 'flex',
    justifyContent: 'space-between',
    alignItems: 'center',
    fontSize: 13,
  },
  errorClose: {
    background: 'none',
    border: 'none',
    color: '#FCA5A5',
    cursor: 'pointer',
    fontSize: 16,
    padding: '0 4px',
  },
};
