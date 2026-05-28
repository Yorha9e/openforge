import { tokens } from '../../shared/design-tokens';

interface HaPill {
  label: string;
  status: 'active' | 'inactive' | 'warning';
}

interface HaPillsProps {
  pills: HaPill[];
}

const dotColors: Record<string, string> = {
  active: tokens.cta,
  inactive: '#6B7280',
  warning: tokens.warning,
};

export function HaPills({ pills }: HaPillsProps) {
  return (
    <div style={{ display: 'flex', gap: 6, flexWrap: 'wrap' }}>
      {pills.map((pill, i) => (
        <div
          key={`pill-${i}`}
          style={{
            display: 'flex',
            alignItems: 'center',
            gap: 5,
            padding: '4px 10px',
            borderRadius: 14,
            border: `1px solid ${tokens.border}`,
            fontSize: 11,
            color: tokens.text,
          }}
        >
          <span style={{ width: 6, height: 6, borderRadius: '50%', background: dotColors[pill.status], flexShrink: 0 }} />
          {pill.label}
        </div>
      ))}
    </div>
  );
}
