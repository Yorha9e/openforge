import { tokens } from './design-tokens';

interface SkeletonProps {
  width?: number | string;
  height?: number | string;
  borderRadius?: number;
  style?: React.CSSProperties;
}

/** Single shimmer line/block for loading placeholders. */
export function Skeleton({ width = '100%', height = 16, borderRadius = 4, style }: SkeletonProps) {
  return (
    <div
      aria-hidden="true"
      style={{
        width,
        height,
        borderRadius,
        background: '#151D2B',
        animation: 'of-shimmer 1.5s ease-in-out infinite',
        ...style,
      }}
    />
  );
}

/** Card-sized skeleton matching ProjectCard dimensions. */
export function CardSkeleton() {
  return (
    <div style={{ background: '#151D2B', border: `1px solid ${tokens.border}`, borderRadius: 8, padding: 16 }}>
      <Skeleton width="60%" height={20} />
      <div style={{ height: 8 }} />
      <Skeleton width="80%" height={14} />
      <div style={{ height: 4 }} />
      <Skeleton width="40%" height={12} />
    </div>
  );
}

/** Text block skeleton for message/paragraph content. */
export function TextSkeleton({ lines = 3 }: { lines?: number }) {
  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
      {Array.from({ length: lines }).map((_, i) => (
        <Skeleton key={i} width={i === lines - 1 ? '60%' : '100%'} height={14} />
      ))}
    </div>
  );
}

/** Full page skeleton: header + grid of cards. */
export function PageSkeleton({ cards = 3 }: { cards?: number }) {
  return (
    <div style={{ minHeight: '100vh', background: tokens.bg }}>
      <div style={{ padding: '12px 24px', borderBottom: `1px solid ${tokens.border}`, display: 'flex', alignItems: 'center', gap: 16 }}>
        <Skeleton width={120} height={20} />
        <div style={{ marginLeft: 'auto' }}>
          <Skeleton width={80} height={14} />
        </div>
      </div>
      <div style={{ maxWidth: 960, margin: '0 auto', padding: 24 }}>
        <Skeleton width={140} height={28} style={{ marginBottom: 24 }} />
        <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fill, minmax(320px, 1fr))', gap: 16 }}>
          {Array.from({ length: cards }).map((_, i) => <CardSkeleton key={i} />)}
        </div>
      </div>
    </div>
  );
}
