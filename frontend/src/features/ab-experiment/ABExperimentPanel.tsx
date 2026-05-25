import { useState, useEffect } from 'react';
import { tokens } from '../../shared/design-tokens';
import { api } from '../../shared/api';

interface Experiment {
  id: string;
  knowledgeId: string;
  status: 'running' | 'completed' | 'aborted';
  verdict?: 'promoted' | 'invalid' | 'harmful';
  pValue?: number;
  effectSize?: number;
  cohortARatio: number;
  startedAt: string;
}

const MOCK_EXPERIMENTS: Experiment[] = [
  {
    id: 'exp-001',
    knowledgeId: 'react-pattern-recommendations',
    status: 'running',
    cohortARatio: 0.5,
    startedAt: '2026-05-20T10:00:00Z',
  },
  {
    id: 'exp-002',
    knowledgeId: 'error-handling-strategies',
    status: 'completed',
    verdict: 'promoted',
    pValue: 0.023,
    effectSize: 0.31,
    cohortARatio: 0.5,
    startedAt: '2026-05-10T08:00:00Z',
  },
  {
    id: 'exp-003',
    knowledgeId: 'code-style-enforcement',
    status: 'aborted',
    verdict: 'invalid',
    pValue: 0.45,
    effectSize: 0.02,
    cohortARatio: 0.33,
    startedAt: '2026-05-01T12:00:00Z',
  },
];

const statusBadge: Record<string, React.CSSProperties> = {
  running: { background: 'rgba(59,130,246,0.15)', color: '#60A5FA' },
  completed: { background: 'rgba(34,197,94,0.15)', color: '#4ADE80' },
  aborted: { background: 'rgba(239,68,68,0.15)', color: '#F87171' },
};

const verdictStyle: Record<string, React.CSSProperties> = {
  promoted: { color: tokens.cta, fontWeight: 700 },
  invalid: { color: tokens.muted },
  harmful: { color: tokens.error, fontWeight: 700 },
};

export function ABExperimentPanel() {
  const [experiments, setExperiments] = useState<Experiment[]>([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    api
      .listExperiments()
      .then((data: Experiment[]) =>
        setExperiments(Array.isArray(data) ? data : []),
      )
      .catch(() => setExperiments(MOCK_EXPERIMENTS))
      .finally(() => setLoading(false));
  }, []);

  if (loading) {
    return (
      <div
        style={{
          padding: 24,
          color: tokens.muted,
          fontFamily: tokens.fontBody,
        }}
      >
        Loading experiments...
      </div>
    );
  }

  return (
    <div
      role="region"
      aria-label="A/B Experiments"
      style={{ padding: 24, color: tokens.text, fontFamily: tokens.fontBody }}
    >
      <div
        style={{
          display: 'flex',
          justifyContent: 'space-between',
          alignItems: 'center',
          marginBottom: 20,
        }}
      >
        <h2 style={{ fontSize: 18, fontWeight: 600, margin: 0 }}>
          A/B Experiments
        </h2>
        {experiments.length > 0 && (
          <span style={{ fontSize: 13, color: tokens.muted }}>
            {experiments.length} experiments
          </span>
        )}
      </div>

      {experiments.length === 0 ? (
        <p style={{ color: tokens.muted, fontSize: 13 }}>
          No active experiments.
        </p>
      ) : (
        <table
          style={{
            width: '100%',
            borderCollapse: 'collapse',
            fontSize: 13,
          }}
        >
          <thead>
            <tr
              style={{
                borderBottom: `2px solid ${tokens.border}`,
                textAlign: 'left',
              }}
            >
              <th style={{ padding: '8px 12px' }}>Knowledge</th>
              <th style={{ padding: '8px 12px' }}>Status</th>
              <th style={{ padding: '8px 12px' }}>Cohort Split</th>
              <th style={{ padding: '8px 12px' }}>Verdict</th>
              <th style={{ padding: '8px 12px' }}>p-Value</th>
            </tr>
          </thead>
          <tbody>
            {experiments.map((exp) => (
              <tr
                key={exp.id}
                style={{ borderBottom: `1px solid ${tokens.border}` }}
              >
                <td style={{ padding: '10px 12px', fontWeight: 500 }}>
                  {exp.knowledgeId}
                </td>
                <td style={{ padding: '10px 12px' }}>
                  <span
                    style={{
                      ...statusBadge[exp.status],
                      display: 'inline-block',
                      padding: '2px 8px',
                      borderRadius: 4,
                      fontSize: 12,
                      fontWeight: 500,
                    }}
                  >
                    {exp.status}
                  </span>
                </td>
                <td style={{ padding: '10px 12px', color: tokens.muted }}>
                  A:{(exp.cohortARatio * 100).toFixed(0)}% / B:
                  {((1 - exp.cohortARatio) * 100).toFixed(0)}%
                </td>
                <td style={{ padding: '10px 12px' }}>
                  {exp.verdict ? (
                    <span style={verdictStyle[exp.verdict]}>
                      {exp.verdict}
                    </span>
                  ) : (
                    <span style={{ color: tokens.muted }}>—</span>
                  )}
                </td>
                <td style={{ padding: '10px 12px', color: tokens.muted }}>
                  {exp.pValue != null ? exp.pValue.toFixed(4) : '—'}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
    </div>
  );
}
