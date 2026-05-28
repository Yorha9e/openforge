import { tokens } from '../../shared/design-tokens';

interface PrioritySliderProps {
  value: number;
  max?: number;
  onChange?: (value: number) => void;
  readOnly?: boolean;
  showLabels?: boolean;
  baseValue?: number;
}

export function PrioritySlider({ value, max = 100, onChange, readOnly, showLabels, baseValue }: PrioritySliderProps) {
  const pct = max > 0 ? (value / max) * 100 : 0;

  return (
    <div>
      {showLabels && baseValue !== undefined && (
        <div style={{ display: 'flex', justifyContent: 'space-between', fontSize: 12, marginBottom: 6 }}>
          <span style={{ color: tokens.muted }}>Base: {baseValue}</span>
          <span style={{ color: tokens.cta, fontWeight: 700 }}>Current: {value}</span>
        </div>
      )}
      <div style={{ height: 6, background: tokens.border, borderRadius: 3, position: 'relative', marginTop: 4 }}>
        <div style={{
          height: '100%',
          width: `${pct}%`,
          background: `linear-gradient(90deg, ${tokens.cta}, #10B981)`,
          borderRadius: 3,
          transition: 'width 200ms',
        }} />
        {!readOnly && onChange && (
          <input
            type="range"
            min={0}
            max={max}
            value={value}
            onChange={e => onChange(Number(e.target.value))}
            aria-label="Set priority"
            style={{
              position: 'absolute',
              top: -5,
              left: 0,
              width: '100%',
              height: 16,
              opacity: 0,
              cursor: 'pointer',
              margin: 0,
            }}
          />
        )}
      </div>
    </div>
  );
}
