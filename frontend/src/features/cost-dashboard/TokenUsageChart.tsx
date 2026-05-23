import { AreaChart, Area, XAxis, YAxis, CartesianGrid, Tooltip, ResponsiveContainer } from 'recharts';

interface TokenUsageChartProps {
  data: Array<{ date: string; prompt_tokens: number; completion_tokens: number }>;
}

export function TokenUsageChart({ data }: TokenUsageChartProps) {
  const chartData = data.map(d => ({
    date: d.date?.slice(5) ?? d.date, // MM-DD
    Prompt: d.prompt_tokens,
    Completion: d.completion_tokens,
  })).reverse();

  return (
    <div style={{ background: '#1E293B', borderRadius: 8, padding: 20 }}>
      <h2 style={{ fontSize: 16, fontWeight: 600, fontFamily: "'Fira Code', monospace", marginBottom: 16, color: '#F8FAFC' }}>Token Usage</h2>
      <ResponsiveContainer width="100%" height={300}>
        <AreaChart data={chartData}>
          <CartesianGrid strokeDasharray="3 3" stroke="#334155" />
          <XAxis dataKey="date" stroke="#94a3b8" fontSize={12} />
          <YAxis stroke="#94a3b8" fontSize={12} />
          <Tooltip contentStyle={{ background: '#0F172A', border: '1px solid #334155', borderRadius: 4, color: '#F8FAFC' }} />
          <Area type="monotone" dataKey="Prompt" stroke="#22C55E" fill="#22C55E33" strokeWidth={2} />
          <Area type="monotone" dataKey="Completion" stroke="#3B82F6" fill="#3B82F633" strokeWidth={2} />
        </AreaChart>
      </ResponsiveContainer>
    </div>
  );
}
