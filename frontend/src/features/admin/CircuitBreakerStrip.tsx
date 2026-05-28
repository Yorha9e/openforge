import { tokens } from '../../shared/design-tokens';

interface CircuitBreakerStripProps {
  breakers: Record<string, 'closed' | 'open' | 'half_open'>;
}

const breakerColors: Record<string, string> = {
  closed: tokens.cta,
  open: tokens.error,
  half_open: tokens.warning,
};

const breakerIcons: Record<string, string> = {
  closed: '●',
  open: '○',
  half_open: '◐',
};

export function CircuitBreakerStrip({ breakers }: CircuitBreakerStripProps) {
  const entries = Object.entries(breakers);
  if (entries.length === 0) return null;

  return (
    <div style={{ display: 'flex', gap: 6, flexWrap: 'wrap' }}>
      {entries.map(([name, state]) => (
        <span
          key={name}
          aria-label={`Circuit breaker ${name}: ${state}`}
          style={{
            padding: '3px 10px',
            borderRadius: 14,
            fontSize: 11,
            fontWeight: 600,
            border: `1px solid ${breakerColors[state]}44`,
            background: `${breakerColors[state]}08`,
            color: breakerColors[state],
            display: 'inline-flex',
            alignItems: 'center',
            gap: 4,
          }}
        >
          {breakerIcons[state]} {name}
          {state !== 'closed' && (
            <span style={{ fontSize: 9, fontWeight: 400 }}>({state.replace('_', ' ')})</span>
          )}
        </span>
      ))}
    </div>
  );
}
