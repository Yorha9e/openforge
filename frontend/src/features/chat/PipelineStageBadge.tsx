import { useChat } from './ChatProvider';
import { tokens } from '../../shared/design-tokens';

const STAGE_LABELS: Record<string, string> = {
  clarify: 'Clarify',
  decompose: 'Decompose',
  impl: 'Implement',
  test: 'Test',
};

const STAGE_COLORS: Record<string, string> = {
  clarify: '#6366f1',   // indigo
  decompose: '#8b5cf6', // violet
  impl: '#06b6d4',      // cyan
  test: '#10b981',      // emerald
};

function statusColor(status: string): string {
  switch (status) {
    case 'running': return '#f59e0b';  // amber
    case 'completed': return '#22c55e'; // green
    case 'failed': return '#ef4444';    // red
    default: return tokens.muted;       // pending
  }
}

export function PipelineStageBadge() {
  const { pipelineStage } = useChat();
  if (!pipelineStage || !pipelineStage.stage) return null;

  const { stage, status } = pipelineStage;
  const label = STAGE_LABELS[stage] || stage;
  const color = STAGE_COLORS[stage] || tokens.muted;

  return (
    <div style={{
      display: 'flex', alignItems: 'center', gap: 8,
      fontSize: 12, fontWeight: 500,
    }}>
      {/* Stage name with colored dot */}
      <span style={{
        display: 'inline-flex', alignItems: 'center', gap: 5,
        padding: '2px 10px', borderRadius: 4,
        background: `${color}18`, border: `1px solid ${color}40`,
      }}>
        <span style={{
          width: 7, height: 7, borderRadius: '50%',
          background: color, flexShrink: 0,
        }} />
        {label}
      </span>

      {/* Status badge */}
      {status && status !== 'pending' && (
        <span style={{
          padding: '2px 8px', borderRadius: 4,
          fontSize: 11,
          background: `${statusColor(status)}18`,
          color: statusColor(status),
          border: `1px solid ${statusColor(status)}40`,
        }}>
          {status}
        </span>
      )}

      {/* Running spinner */}
      {status === 'running' && (
        <span style={{
          display: 'inline-block', width: 12, height: 12,
          border: `2px solid ${tokens.border}`,
          borderTopColor: statusColor(status),
          borderRadius: '50%',
          animation: 'spin 0.8s linear infinite',
        }} />
      )}
    </div>
  );
}
