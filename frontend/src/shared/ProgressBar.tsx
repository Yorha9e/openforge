import { tokens } from './design-tokens';

interface ProgressBarProps {
  current: number;
  total: number;
  showPercent?: boolean;
  label?: string;
  color?: string;
  badges?: { label: string; done: boolean }[];
}

export function ProgressBar({ current, total, showPercent, label, color = tokens.cta, badges }: ProgressBarProps) {
  const pct = total > 0 ? Math.round((current / total) * 100) : 0;

  return (
    <div>
      {label && (
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 8 }}>
          <div style={{ fontSize: 14, fontWeight: 600, color: tokens.text }}>{label}</div>
          <div style={{ fontSize: 14, fontWeight: 700, color }}>{current} / {total}{showPercent && ` (${pct}%)`}</div>
        </div>
      )}
      <div style={{ height: 8, background: tokens.border, borderRadius: 4, overflow: 'hidden' }}>
        <div style={{
          height: '100%',
          width: `${pct}%`,
          background: `linear-gradient(90deg, ${color}, ${color}dd)`,
          borderRadius: 4,
          transition: 'width 250ms ease-out',
        }} />
      </div>
      {badges && (
        <div style={{ display: 'flex', gap: 5, flexWrap: 'wrap', marginTop: 10 }}>
          {badges.map(b => (
            <span
              key={b.label}
              style={{
                fontSize: 10,
                fontWeight: 600,
                padding: '2px 7px',
                borderRadius: 4,
                background: b.done ? `${tokens.cta}14` : `${tokens.warning}14`,
                color: b.done ? tokens.cta : tokens.warning,
              }}
            >
              {b.label} {b.done ? '✓' : '⏳'}
            </span>
          ))}
        </div>
      )}
    </div>
  );
}
