import { useState, useEffect } from 'react';
import { tokens } from '../../shared/design-tokens';

interface DeployStatus {
  pipelineId: string;
  environment: 'staging' | 'production';
  status: 'pending' | 'deploying' | 'success' | 'failed' | 'rolled_back';
  version: number;
  deployedAt?: string;
}

const MOCK_DEPLOYMENTS: DeployStatus[] = [
  { pipelineId: 'pipe-edge-ml',   environment: 'production', status: 'success',     version: 42, deployedAt: '2026-05-25T03:12:00Z' },
  { pipelineId: 'pipe-gate-api',  environment: 'staging',    status: 'deploying',   version: 17, deployedAt: '2026-05-25T03:30:00Z' },
  { pipelineId: 'pipe-cost-dash', environment: 'production', status: 'failed',      version:  9, deployedAt: '2026-05-24T22:15:00Z' },
  { pipelineId: 'pipe-auth-svc',  environment: 'staging',    status: 'pending',     version:  5, deployedAt: undefined },
  { pipelineId: 'pipe-agent-rnr', environment: 'production', status: 'rolled_back', version:  3, deployedAt: '2026-05-24T14:00:00Z' },
];

const STATUS_COLORS: Record<string, string> = {
  pending:      tokens.muted,
  deploying:    tokens.warning,
  success:      tokens.cta,
  failed:       tokens.error,
  rolled_back:  tokens.warning,
};

const STATUS_ICON: Record<string, string> = {
  pending:     '⏳',
  deploying:   '🔄',
  success:     '✅',
  failed:      '❌',
  rolled_back: '↩️',
};

function timeAgo(dateStr?: string): string {
  if (!dateStr) return '—';
  const ms = Date.now() - new Date(dateStr).getTime();
  const min = Math.floor(ms / 60000);
  if (min < 1) return 'just now';
  if (min < 60) return `${min}m ago`;
  const hr = Math.floor(min / 60);
  if (hr < 24) return `${hr}h ago`;
  return `${Math.floor(hr / 24)}d ago`;
}

export function CICDDashboard() {
  const [deployments, setDeployments] = useState<DeployStatus[]>([]);
  const [errored, setErrored] = useState(false);

  useEffect(() => {
    fetch('/api/deployments')
      .then<DeployStatus[]>(r => { if (!r.ok) throw new Error(); return r.json(); })
      .then(setDeployments)
      .catch(() => { setDeployments(MOCK_DEPLOYMENTS); setErrored(true); });
  }, []);

  return (
    <div role="region" aria-label="CI/CD Dashboard" style={{
      padding: 16, color: tokens.text, background: tokens.bg, height: '100%', overflow: 'auto',
      fontFamily: tokens.fontBody,
    }}>
      <h2 style={{ fontSize: 16, fontWeight: 600, marginBottom: 12, fontFamily: tokens.fontHeading }}>
        Deployments
      </h2>

      {errored && (
        <p style={{ fontSize: 12, color: tokens.muted, marginBottom: 8, fontStyle: 'italic' }}>
          Live API unreachable &mdash; showing mock data
        </p>
      )}

      <div role="table" aria-label="Deployment status">
        <div role="rowgroup" style={{
          fontSize: 12, color: tokens.muted, display: 'flex',
          borderBottom: `1px solid ${tokens.border}`, paddingBottom: 6, marginBottom: 8,
        }}>
          <div role="columnheader" style={{ flex: 2 }}>Pipeline</div>
          <div role="columnheader" style={{ flex: 1 }}>Env</div>
          <div role="columnheader" style={{ flex: 1 }}>Status</div>
          <div role="columnheader" style={{ flex: 1, textAlign: 'right' }}>When</div>
        </div>

        {deployments.length === 0 ? (
          <p style={{ color: tokens.muted, fontSize: 13 }}>No deployments yet.</p>
        ) : (
          deployments.map(d => (
            <div key={d.pipelineId} role="row" style={{
              display: 'flex', padding: '6px 0', fontSize: 13, alignItems: 'center',
              borderBottom: `1px solid ${tokens.border}`, transition: tokens.transition,
            }}>
              <div role="cell" style={{ flex: 2, fontFamily: tokens.fontHeading, fontSize: 12 }}>
                {d.pipelineId}
              </div>
              <div role="cell" style={{ flex: 1 }}>{d.environment}</div>
              <div role="cell" style={{ flex: 1, display: 'flex', alignItems: 'center', gap: 4 }}>
                <span style={{ color: STATUS_COLORS[d.status], fontSize: 14 }}>{STATUS_ICON[d.status]}</span>
                <span style={{ color: STATUS_COLORS[d.status], fontWeight: 500 }}>{d.status}</span>
              </div>
              <div role="cell" style={{ flex: 1, textAlign: 'right', color: tokens.muted, fontSize: 12 }}>
                {timeAgo(d.deployedAt)}
              </div>
            </div>
          ))
        )}
      </div>
    </div>
  );
}
