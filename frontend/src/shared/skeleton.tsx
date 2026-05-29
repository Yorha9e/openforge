import { tokens } from './design-tokens';

interface SkeletonProps {
  variant?: 'text' | 'card' | 'circle' | 'rect';
  width?: number | string;
  height?: number;
  radius?: number;
  lines?: number;
  style?: React.CSSProperties;
}

function ShimmerBlock({
  width,
  height,
  radius,
  style,
}: {
  width?: number | string;
  height?: number | string;
  radius?: number | string;
  style?: React.CSSProperties;
}) {
  return (
    <div
      aria-hidden="true"
      style={{
        width,
        height,
        borderRadius: radius,
        background: tokens.surface,
        animation: 'of-shimmer 1.5s ease-in-out infinite',
        ...style,
      }}
    />
  );
}

export function Skeleton({
  variant = 'rect',
  width = '100%',
  height,
  radius,
  lines,
  style,
}: SkeletonProps) {
  if (variant === 'text' && lines && lines > 1) {
    return (
      <div style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
        {Array.from({ length: lines }).map((_, i) => (
          <ShimmerBlock
            key={i}
            width={i === lines - 1 ? '60%' : '100%'}
            height={height ?? 14}
            radius={radius ?? 4}
            style={style}
          />
        ))}
      </div>
    );
  }

  if (variant === 'circle') {
    const size = typeof width === 'number' ? width : 40;
    return <ShimmerBlock width={size} height={size} radius="50%" style={style} />;
  }

  if (variant === 'card') {
    return (
      <div
        style={{
          background: tokens.surface,
          border: '1px solid ' + tokens.border,
          borderRadius: 8,
          padding: 16,
        }}
      >
        <ShimmerBlock width="60%" height={20} radius={4} />
        <div style={{ height: 8 }} />
        <ShimmerBlock width="80%" height={14} radius={4} />
        <div style={{ height: 4 }} />
        <ShimmerBlock width="40%" height={12} radius={4} />
      </div>
    );
  }

  return (
    <ShimmerBlock
      width={width}
      height={height ?? 16}
      radius={radius ?? 4}
      style={style}
    />
  );
}

export function CardSkeleton() {
  return <Skeleton variant="card" />;
}

export function TextSkeleton({ lines = 3 }: { lines?: number }) {
  return <Skeleton variant="text" lines={lines} />;
}

export function PageSkeleton({ cards = 3 }: { cards?: number }) {
  return (
    <div style={{ minHeight: '100vh', background: tokens.bg }}>
      <div
        style={{
          padding: '12px 24px',
          borderBottom: '1px solid ' + tokens.border,
          display: 'flex',
          alignItems: 'center',
          gap: 16,
        }}
      >
        <Skeleton width={120} height={20} />
        <div style={{ marginLeft: 'auto' }}>
          <Skeleton width={80} height={14} />
        </div>
      </div>
      <div style={{ maxWidth: 960, margin: '0 auto', padding: 24 }}>
        <Skeleton width={140} height={28} style={{ marginBottom: 24 }} />
        <div
          style={{
            display: 'grid',
            gridTemplateColumns: 'repeat(auto-fill, minmax(320px, 1fr))',
            gap: 16,
          }}
        >
          {Array.from({ length: cards }).map((_, i) => (
            <CardSkeleton key={i} />
          ))}
        </div>
      </div>
    </div>
  );
}

export function SkeletonGrid({
  count = 4,
  variant = 'card',
}: {
  count?: number;
  variant?: 'card' | 'rect' | 'text';
}) {
  return (
    <div
      aria-busy="true"
      aria-label="Loading"
      style={{
        display: 'grid',
        gridTemplateColumns: 'repeat(auto-fill, minmax(180px, 1fr))',
        gap: 12,
      }}
    >
      {Array.from({ length: count }).map((_, i) => (
        <Skeleton key={i} variant={variant} />
      ))}
    </div>
  );
}
