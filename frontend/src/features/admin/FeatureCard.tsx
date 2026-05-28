import { tokens } from '../../shared/design-tokens';

interface FeatureCardProps {
  title: string;
  description: string;
  features: string[];
  enabled: boolean;
  color: string;
  icon: string;
  onToggle: () => void;
  loading?: boolean;
}

export function FeatureCard({ title, description, features, enabled, color, icon, onToggle, loading }: FeatureCardProps) {
  return (
    <div
      style={{
        background: '#111B2A',
        border: `1px solid ${enabled ? `${color}55` : tokens.border}`,
        borderRadius: 8,
        padding: 18,
        transition: 'border-color 200ms, opacity 200ms',
        opacity: loading ? 0.5 : 1,
      }}
    >
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start', marginBottom: 6 }}>
        <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
          <span style={{ fontSize: 20, lineHeight: 1 }}>{icon}</span>
          <span style={{ fontSize: 14, fontWeight: 700, color: enabled ? color : tokens.muted }}>{title}</span>
        </div>
        <button
          onClick={onToggle}
          disabled={loading}
          aria-label={`Toggle ${title}: currently ${enabled ? 'ON' : 'OFF'}`}
          style={{
            width: 40, height: 22, borderRadius: 11, border: 'none',
            cursor: loading ? 'not-allowed' : 'pointer',
            background: enabled ? color : '#3D4F66',
            position: 'relative', flexShrink: 0,
            transition: 'background-color 200ms',
          }}
        >
          <div style={{
            width: 16, height: 16, borderRadius: '50%', background: '#fff',
            position: 'absolute', top: 3,
            left: enabled ? 21 : 3,
            transition: 'left 200ms ease-out',
          }} />
        </button>
      </div>
      <div style={{ fontSize: 11, color: tokens.muted, marginBottom: 10 }}>{description}</div>
      <div style={{ display: 'flex', flexWrap: 'wrap', gap: 5 }}>
        {features.map(f => (
          <span key={f} style={{
            fontSize: 10, padding: '2px 7px', borderRadius: 4,
            background: 'rgba(125, 140, 165, 0.06)', color: '#8C9AB3',
          }}>
            {f}
          </span>
        ))}
      </div>
    </div>
  );
}
