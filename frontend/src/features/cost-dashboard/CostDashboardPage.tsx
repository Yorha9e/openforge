import { useEffect, useState } from 'react';
import { useParams } from 'react-router-dom';
import { api } from '../../shared/api';
import { AppLayout } from '../../shared/AppLayout';
import { tokens } from '../../shared/design-tokens';
import { TokenUsageChart } from './TokenUsageChart';
import { CostBreakdown } from './CostBreakdown';
import { BudgetGauge } from './BudgetGauge';

export default function CostDashboardPage() {
  const { id } = useParams<{ id: string }>();
  const [usage, setUsage] = useState<any[]>([]);
  const [budget, setBudget] = useState<any>(null);
  const [days, setDays] = useState(30);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    if (!id) return;
    setLoading(true);
    Promise.all([
      api.getTokenUsage(id, days),
      api.getTokenBudget(id),
    ]).then(([u, b]) => { setUsage(u); setBudget(b); })
      .catch(console.error)
      .finally(() => setLoading(false));
  }, [id, days]);

  return (
    <AppLayout breadcrumbs={[
      { label: 'Project', to: `/project/${id}` },
      { label: 'Cost Dashboard' },
    ]}>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 24 }}>
        <h1 style={{ fontSize: 22, fontWeight: 700, fontFamily: tokens.fontHeading, margin: 0, color: tokens.text }}>Cost Dashboard</h1>
        <div style={{ display: 'flex', gap: 4 }}>
          {[7, 14, 30].map(d => (
            <button key={d} onClick={() => setDays(d)}
              style={{
                padding: '4px 12px',
                background: days === d ? tokens.cta : tokens.surface,
                color: days === d ? tokens.ctaText : tokens.text,
                border: 'none', borderRadius: 4, cursor: 'pointer', fontSize: 12, fontWeight: 600,
                transition: tokens.transition,
              }}>
              {d}d
            </button>
          ))}
        </div>
      </div>

      <div style={{ maxWidth: 1100, margin: '0 auto' }}>
        {loading ? <p style={{ color: tokens.muted }}>Loading...</p> : (
          <>
            {budget && <BudgetGauge budget={budget} />}
            <div style={{ display: 'grid', gridTemplateColumns: '2fr 1fr', gap: 20, marginTop: 20 }}>
              <TokenUsageChart data={usage} />
              <CostBreakdown data={usage} />
            </div>
          </>
        )}
      </div>
    </AppLayout>
  );
}
