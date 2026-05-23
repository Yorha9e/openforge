import { PieChart, Pie, Cell, Tooltip, ResponsiveContainer, Legend } from 'recharts';

const COLORS = ['#22C55E', '#3B82F6', '#F59E0B', '#EF4444', '#8B5CF6', '#EC4899'];

interface CostBreakdownProps {
  data: Array<{ model: string; estimated_cost: number }>;
}

export function CostBreakdown({ data }: CostBreakdownProps) {
  // Aggregate by model
  const modelMap = new Map<string, number>();
  data.forEach(d => {
    modelMap.set(d.model, (modelMap.get(d.model) ?? 0) + (d.estimated_cost ?? 0));
  });
  const pieData = Array.from(modelMap.entries()).map(([name, value]) => ({ name, value }));

  return (
    <div style={{ background: '#1E293B', borderRadius: 8, padding: 20 }}>
      <h2 style={{ fontSize: 16, fontWeight: 600, fontFamily: "'Fira Code', monospace", marginBottom: 16, color: '#F8FAFC' }}>Cost by Model</h2>
      <ResponsiveContainer width="100%" height={300}>
        <PieChart>
          <Pie data={pieData} dataKey="value" nameKey="name" cx="50%" cy="50%" outerRadius={100} innerRadius={50}>
            {pieData.map((_, i) => <Cell key={i} fill={COLORS[i % COLORS.length]} />)}
          </Pie>
          <Tooltip contentStyle={{ background: '#0F172A', border: '1px solid #334155', borderRadius: 4, color: '#F8FAFC' }} />
          <Legend wrapperStyle={{ color: '#94a3b8', fontSize: 12 }} />
        </PieChart>
      </ResponsiveContainer>
    </div>
  );
}
