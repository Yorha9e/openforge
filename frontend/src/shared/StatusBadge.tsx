import { tokens } from './design-tokens';

interface StatusBadgeProps {
  label: string;
  status: 'active' | 'inactive' | 'error' | 'warning';
  detail?: string;
  size?: 'sm' | 'md';
}

const statusColors: Record<StatusBadgeProps['status'], { dot: string; bg: string; text: string; label: string }> = {
  active: { dot: tokens.cta, bg: 'rgba(34, 197, 94, 0.1)', text: tokens.cta, label: 'Active' },
  inactive: { dot: '#6B7280', bg: 'rgba(108, 122, 137, 0.08)', text: '#8C9AB3', label: 'Disabled' },
  error: { dot: tokens.error, bg: 'rgba(239, 68, 68, 0.1)', text: tokens.error, label: 'Error' },
  warning: { dot: tokens.warning, bg: 'rgba(245, 158, 11, 0.1)', text: tokens.warning, label: 'Warning' },
};

export function StatusBadge({ label, status, detail, size = 'sm' }: StatusBadgeProps) {
  const c = statusColors[status];
  const isSm = size === 'sm';
  return (
    <span
      aria-label={`${label}: ${c!.label}`}
      title={detail}
      style={{
        display: 'inline-flex',
        alignItems: 'center',
        gap: 5,
        padding: isSm ? '3px 10px' : '6px 14px',
        borderRadius: 12,
        fontSize: isSm ? 11 : 13,
        fontWeight: 600,
        background: c!.bg,
        color: c!.text,
        whiteSpace: 'nowrap',
        transition: tokens.transition,
      }}
    >
      <span style={{ width: 6, height: 6, borderRadius: '50%', background: c!.dot, flexShrink: 0 }} />
      {label}
    </span>
  );
}
