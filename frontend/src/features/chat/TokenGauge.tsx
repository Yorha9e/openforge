import { useChat } from './ChatProvider';
import { tokens } from '../../shared/design-tokens';

const BAR_WIDTH = 80;
const PULSE_BAR_WIDTH = 100;

function formatTokens(n: number): string {
  if (n >= 1000) {
    const k = n / 1000;
    return k >= 100 ? Math.round(k) + 'K' : k.toFixed(1) + 'K';
  }
  return String(n);
}

function usageColor(pct: number): string {
  if (pct > 0.95) return '#ef4444';   // red
  if (pct > 0.80) return '#f97316';   // orange
  if (pct > 0.50) return '#eab308';   // yellow
  return '#22c55e';                    // green
}

export function TokenGauge() {
  const { tokenUsed, tokenBudget } = useChat();

  if (tokenBudget <= 0) return null;

  const pct = Math.min(tokenUsed / tokenBudget, 1);
  const color = usageColor(pct);
  const barWidth = BAR_WIDTH;

  const trackStyle: React.CSSProperties = {
    width: barWidth,
    height: 6,
    borderRadius: 3,
    background: tokens.border,
    overflow: 'hidden',
    position: 'relative',
  };
  const fillStyle: React.CSSProperties = {
    height: '100%',
    borderRadius: 3,
    background: color,
    width: `${pct * 100}%`,
    transition: 'width 0.4s ease, background 0.3s ease',
  };
  const labelStyle: React.CSSProperties = {
    fontSize: 11,
    fontWeight: 500,
    color: tokens.muted,
    whiteSpace: 'nowrap',
    lineHeight: '16px',
  };

  if (tokenUsed === 0) return null;

  return (
    <div style={{
      display: 'flex', alignItems: 'center', gap: 8,
      padding: '2px 10px', borderRadius: 6,
      background: 'rgba(255,255,255,0.03)',
    }}>
      <span style={labelStyle}>
        {formatTokens(tokenUsed)} / {formatTokens(tokenBudget)}
      </span>
      <div style={trackStyle}>
        <div style={fillStyle} />
      </div>
      {pct > 0.80 && (
        <span style={{ ...labelStyle, color, fontWeight: 600 }}>
          {Math.round(pct * 100)}%
        </span>
      )}
    </div>
  );
}
