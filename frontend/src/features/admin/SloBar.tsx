import { tokens } from '../../shared/design-tokens';

interface SloBarProps {
  label: string;
  value: string;
  max: number;
  current: number;
  target: string;
  color?: string;
}

export function SloBar({ label, value, max, current, target, color = tokens.cta }: SloBarProps) {
  const pct = max > 0 ? Math.min((current / max) * 100, 100) : 0;

  return (
    <div style={{ display: 'flex', alignItems: 'center', gap: 10, padding: '6px 0', fontSize: 12 }}>
      <span style={{ color: tokens.muted, minWidth: 65, fontWeight: 600, fontSize: 11 }}>{label}</span>
      <span style={{ fontWeight: 700, minWidth: 45, color: tokens.text }}>{value}</span>
      <div style={{ flex: 1, height: 5, background: tokens.border, borderRadius: 3, overflow: 'hidden' }}>
        <div style={{ height: '100%', width: `${pct}%`, background: color, borderRadius: 3, transition: 'width 250ms' }} />
      </div>
      <span style={{ fontSize: 11, color: tokens.muted, minWidth: 70, textAlign: 'right' }}>{target}</span>
    </div>
  );
}
