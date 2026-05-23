import { useEffect, useState } from 'react';
import { useParams, Link } from 'react-router-dom';
import { api } from '../../shared/api';
import { TokenUsageChart } from './TokenUsageChart';
import { CostBreakdown } from './CostBreakdown';
import { BudgetGauge } from './BudgetGauge';

export function CostDashboardPage() {
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
    <div style={{ minHeight: '100vh', background: '#0F172A', color: '#F8FAFC', fontFamily: "'Fira Sans', sans-serif" }}>
      <header style={{ padding: '12px 24px', borderBottom: '1px solid #334155', display: 'flex', alignItems: 'center', gap: 16 }}>
        <Link to={`/project/${id}`} style={{ color: '#94a3b8', textDecoration: 'none' }}>&larr; Project</Link>
        <h1 style={{ fontSize: 18, fontWeight: 700, fontFamily: "'Fira Code', monospace" }}>Cost Dashboard</h1>
        <div style={{ marginLeft: 'auto', display: 'flex', gap: 4 }}>
          {[7, 14, 30].map(d => (
            <button key={d} onClick={() => setDays(d)}
              style={{ padding: '4px 12px', background: days === d ? '#22C55E' : '#1E293B', color: days === d ? '#0F172A' : '#F8FAFC', border: 'none', borderRadius: 4, cursor: 'pointer', fontSize: 12, fontWeight: 600, transition: 'background 200ms' }}>
              {d}d
            </button>
          ))}
        </div>
      </header>
      <main style={{ maxWidth: 1100, margin: '0 auto', padding: 24 }}>
        {loading ? <p style={{ color: '#94a3b8' }}>Loading...</p> : (
          <>
            {budget && <BudgetGauge budget={budget} />}
            <div style={{ display: 'grid', gridTemplateColumns: '2fr 1fr', gap: 20, marginTop: 20 }}>
              <TokenUsageChart data={usage} />
              <CostBreakdown data={usage} />
            </div>
          </>
        )}
      </main>
    </div>
  );
}
