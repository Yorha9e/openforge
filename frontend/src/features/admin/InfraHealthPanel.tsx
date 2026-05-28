import { useState } from 'react';
import { tokens } from '../../shared/design-tokens';
import { SkeletonGrid } from '../../shared/skeleton';
import type { InfraComponent } from '../../shared/api';

/* ---- Types ---- */

interface InfraHealthPanelProps {
  components: InfraComponent[];
  loading?: boolean;
  lastCheck?: string;
}

/* ---- Status color config ---- */

const STATUS_CONFIG: Record<
  InfraComponent['status'],
  { dot: string; text: string; bg: string; label: string; icon: string }
> = {
  connected: {
    dot: '#22C55E',
    text: '#22C55E',
    bg: 'rgba(34,197,94,0.08)',
    label: 'Connected',
    icon: '🟢',
  },
  degraded: {
    dot: '#F59E0B',
    text: '#F59E0B',
    bg: 'rgba(245,158,11,0.08)',
    label: 'Degraded',
    icon: '🟡',
  },
  unavailable: {
    dot: '#EF4444',
    text: '#EF4444',
    bg: 'rgba(239,68,68,0.08)',
    label: 'Unavailable',
    icon: '🔴',
  },
  unused: {
    dot: '#6B7280',
    text: '#6B7280',
    bg: 'transparent',
    label: 'Not configured',
    icon: '⚪',
  },
};

/* ---- Uptime formatter ---- */

function formatUptime(seconds?: number): string {
  if (seconds === undefined || seconds === null) return '';
  const d = Math.floor(seconds / 86400);
  const h = Math.floor((seconds % 86400) / 3600);
  const m = Math.floor((seconds % 3600) / 60);
  if (d > 0) return `${d}d`;
  if (h > 0) return `${h}h`;
  if (m > 0) return `${m}m`;
  return `${Math.floor(seconds)}s`;
}

/* ---- Sort priority ---- */

const STATUS_ORDER: Record<InfraComponent['status'], number> = {
  unavailable: 0,
  degraded: 1,
  connected: 2,
  unused: 3,
};

function sortComponents(list: InfraComponent[]): InfraComponent[] {
  return [...list].sort((a, b) => STATUS_ORDER[a.status] - STATUS_ORDER[b.status]);
}

/* ---- Component ---- */

export default function InfraHealthPanel({ components, loading, lastCheck }: InfraHealthPanelProps) {
  if (loading) {
    return (
      <div>
        <SkeletonGrid count={8} variant="rect" />
      </div>
    );
  }

  if (!components || components.length === 0) {
    return (
      <div
        style={{
          textAlign: 'center',
          padding: 24,
          color: tokens.muted,
          fontSize: 13,
          border: `1px dashed ${tokens.border}`,
          borderRadius: 8,
        }}
      >
        Infrastructure data unavailable
      </div>
    );
  }

  const sorted = sortComponents(components);
  const stats = {
    connected: sorted.filter((c) => c.status === 'connected').length,
    degraded: sorted.filter((c) => c.status === 'degraded').length,
    unavailable: sorted.filter((c) => c.status === 'unavailable').length,
    unused: sorted.filter((c) => c.status === 'unused').length,
    total: sorted.length,
  };

  return (
    <div
      role="region"
      aria-label="Infrastructure Health"
      style={{
        background: 'linear-gradient(135deg, rgba(34,197,94,0.04), rgba(34,197,94,0.01))',
        border: '1px solid rgba(34,197,94,0.12)',
        borderRadius: 8,
        padding: 12,
      }}
    >
      {/* Summary bar */}
      <div
        style={{
          display: 'flex',
          alignItems: 'center',
          gap: 10,
          marginBottom: 10,
          paddingBottom: 10,
          borderBottom: `1px solid ${tokens.border}`,
        }}
      >
        <div
          style={{
            width: 28,
            height: 28,
            borderRadius: 6,
            background: 'linear-gradient(135deg, #22C55E, #16A34A)',
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
            fontSize: 13,
            fontWeight: 700,
            color: '#000',
            flexShrink: 0,
          }}
        >
          {stats.connected}/{stats.total}
        </div>
        <div style={{ flex: 1 }}>
          <div style={{ fontSize: 13, fontWeight: 700, color: tokens.text }}>
            Infrastructure Health
          </div>
          <div style={{ fontSize: 10, color: tokens.muted, display: 'flex', gap: 8 }}>
            {stats.connected > 0 && (
              <span style={{ color: '#22C55E' }}>{stats.connected} connected</span>
            )}
            {stats.degraded > 0 && (
              <span style={{ color: '#F59E0B' }}>{stats.degraded} degraded</span>
            )}
            {stats.unavailable > 0 && (
              <span style={{ color: '#EF4444' }}>{stats.unavailable} down</span>
            )}
            {stats.unused > 0 && (
              <span style={{ color: '#6B7280', fontSize: 9 }}>{stats.unused} unused</span>
            )}
          </div>
        </div>
      </div>

      {/* Component rows */}
      <div style={{ display: 'flex', flexDirection: 'column', gap: 4 }}>
        {sorted.map((comp) => (
          <InfraRow key={comp.key} component={comp} />
        ))}
      </div>

      {/* Last check timestamp footer */}
      {lastCheck && (
        <div
          style={{
            marginTop: 8,
            paddingTop: 6,
            borderTop: `1px solid ${tokens.border}`,
            fontSize: 9,
            color: tokens.muted,
            textAlign: 'right',
          }}
        >
          Last check: {lastCheck}
        </div>
      )}
    </div>
  );
}

/* ---- Single Row ---- */

function InfraRow({ component }: { component: InfraComponent }) {
  const [expanded, setExpanded] = useState(false);
  const cfg = STATUS_CONFIG[component.status];
  const isUnused = component.status === 'unused';
  const uptimeLabel = formatUptime(component.uptime_seconds);
  const latencyLabel = component.latency_ms !== undefined ? `${component.latency_ms}ms` : null;
  const cbLabel = component.circuit_breaker_state
    ? `CB: ${component.circuit_breaker_state.replace('_', ' ')}`
    : null;
  const metaItems = [latencyLabel, cbLabel].filter(Boolean);
  const isInteractive = !isUnused;
  const rowOpacity = isUnused ? 0.5 : 1;

  return (
    <div
      role="status"
      aria-label={`${component.name}: ${cfg.label}`}
      tabIndex={isInteractive ? 0 : undefined}
      onKeyDown={
        isInteractive
          ? (e) => {
              if (e.key === 'Enter' || e.key === ' ') {
                e.preventDefault();
                setExpanded((v) => !v);
              }
            }
          : undefined
      }
      onClick={isInteractive ? () => setExpanded((v) => !v) : undefined}
      title={
        isUnused
          ? `${component.name} — not enabled in current profile`
          : `${component.name}: ${cfg.label}${uptimeLabel ? ` · Uptime: ${uptimeLabel}` : ''}${latencyLabel ? ` · ${latencyLabel}` : ''}`
      }
      style={{
        display: 'flex',
        alignItems: 'center',
        gap: 8,
        padding: '6px 10px',
        borderRadius: 4,
        position: 'relative',
        background: isUnused ? 'transparent' : component.status === 'unavailable' ? 'rgba(239,68,68,0.04)' : component.status === 'degraded' ? 'rgba(245,158,11,0.04)' : 'transparent',
        border: `1px solid ${
          component.status === 'unavailable'
            ? 'rgba(239,68,68,0.18)'
            : component.status === 'degraded'
              ? 'rgba(245,158,11,0.18)'
              : tokens.border
        }`,
        cursor: isInteractive ? 'pointer' : 'default',
        opacity: rowOpacity,
        fontSize: 12,
        transition: 'background 200ms, border-color 200ms',
        userSelect: 'none',
        minHeight: 32,
      }}
      onMouseEnter={() => {}}
    >
      {/* Status dot with pulse animation */}
      <span
        className={component.status === 'connected' ? 'infra-pulse' : ''}
        style={{
          width: 6,
          height: 6,
          borderRadius: '50%',
          background: cfg.dot,
          flexShrink: 0,
        }}
      />

      {/* Component name */}
      <span
        style={{
          fontWeight: 600,
          width: 72,
          flexShrink: 0,
          color: isUnused ? tokens.muted : tokens.text,
          fontSize: 11,
          overflow: 'hidden',
          textOverflow: 'ellipsis',
          whiteSpace: 'nowrap',
        }}
      >
        {component.name}
      </span>

      {/* Status label */}
      <span
        style={{
          color: cfg.text,
          width: 88,
          flexShrink: 0,
          fontSize: 10,
          fontWeight: 500,
          opacity: isUnused ? 0.6 : 1,
        }}
      >
        {cfg.label}
      </span>

      {/* Version badge */}
      {component.version && (
        <span
          style={{
            fontSize: 9,
            padding: '1px 5px',
            borderRadius: 3,
            background: tokens.surface,
            color: tokens.muted,
            border: `1px solid ${tokens.border}`,
            whiteSpace: 'nowrap',
            flexShrink: 0,
          }}
        >
          {component.version}
        </span>
      )}

      {/* Uptime */}
      {uptimeLabel && (
        <span
          style={{
            color: tokens.muted,
            flex: 1,
            textAlign: 'right',
            fontSize: 10,
            fontFamily: "'Fira Code', monospace",
            whiteSpace: 'nowrap',
          }}
        >
          {uptimeLabel}
        </span>
      )}

      {/* Expand indicator */}
      {isInteractive && metaItems.length > 0 && (
        <span
          style={{
            color: tokens.muted,
            fontSize: 8,
            marginLeft: 2,
            flexShrink: 0,
          }}
        >
          {expanded ? '▲' : '▼'}
        </span>
      )}

      {/* Expanded metadata row */}
      {expanded && metaItems.length > 0 && (
        <div
          style={{
            position: 'absolute',
            right: 40,
            top: '100%',
            marginTop: 4,
            display: 'flex',
            gap: 6,
            zIndex: 10,
            background: tokens.surface,
            border: `1px solid ${tokens.border}`,
            borderRadius: 4,
            padding: '4px 8px',
            fontSize: 10,
            color: tokens.muted,
            whiteSpace: 'nowrap',
            pointerEvents: 'none',
          }}
        >
          {metaItems.map((item, i) => (
            <span key={i}>{item}</span>
          ))}
        </div>
      )}
    </div>
  );
}
