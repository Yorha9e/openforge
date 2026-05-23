interface BudgetGaugeProps {
  budget: {
    monthly_limit: number;
    current_usage: number;
    cost_limit: number;
    current_cost: number;
  };
}

export function BudgetGauge({ budget }: BudgetGaugeProps) {
  const tokenPct = budget.monthly_limit > 0 ? Math.min(100, (budget.current_usage / budget.monthly_limit) * 100) : 0;
  const costPct = budget.cost_limit > 0 ? Math.min(100, (budget.current_cost / budget.cost_limit) * 100) : 0;

  const barColor = (pct: number) => pct > 90 ? '#EF4444' : pct > 70 ? '#F59E0B' : '#22C55E';

  return (
    <div style={{ background: '#1E293B', borderRadius: 8, padding: 20, marginBottom: 20 }}>
      <h2 style={{ fontSize: 16, fontWeight: 600, fontFamily: "'Fira Code', monospace", marginBottom: 16, color: '#F8FAFC' }}>Monthly Budget</h2>
      <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 20 }}>
        <div>
          <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: 4, fontSize: 13, color: '#94a3b8' }}>
            <span>Tokens</span>
            <span>{budget.current_usage.toLocaleString()} / {budget.monthly_limit.toLocaleString()}</span>
          </div>
          <div style={{ height: 8, background: '#334155', borderRadius: 4, overflow: 'hidden' }}>
            <div style={{ width: `${tokenPct}%`, height: '100%', background: barColor(tokenPct), borderRadius: 4, transition: 'width 300ms' }} />
          </div>
        </div>
        <div>
          <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: 4, fontSize: 13, color: '#94a3b8' }}>
            <span>Cost</span>
            <span>${budget.current_cost.toFixed(2)} / ${budget.cost_limit.toFixed(2)}</span>
          </div>
          <div style={{ height: 8, background: '#334155', borderRadius: 4, overflow: 'hidden' }}>
            <div style={{ width: `${costPct}%`, height: '100%', background: barColor(costPct), borderRadius: 4, transition: 'width 300ms' }} />
          </div>
        </div>
      </div>
    </div>
  );
}
