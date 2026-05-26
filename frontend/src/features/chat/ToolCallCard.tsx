import { tokens } from '../../shared/design-tokens';
import { useState, useMemo } from 'react';
import { FileTreeRenderer } from './FileTreeRenderer';

export type ToolStatus = 'running' | 'success' | 'error';

export interface ToolCallCardProps {
  tool: string;
  input: string;
  output?: string;
  error?: string;
  outputType?: string;
  status?: ToolStatus;
  durationMs?: number;
}

/**
 * Status icons for tool execution
 */
const StatusIcon = ({ status }: { status: ToolStatus }) => {
  switch (status) {
    case 'running':
      return (
        <svg width="14" height="14" viewBox="0 0 14 14" fill="none" aria-hidden="true"
          style={{ animation: 'spin 1s linear infinite' }}>
          <circle cx="7" cy="7" r="5.5" stroke={tokens.cta} strokeWidth="1.5" strokeDasharray="20 10" />
        </svg>
      );
    case 'success':
      return (
        <svg width="14" height="14" viewBox="0 0 14 14" fill="none" aria-hidden="true">
          <circle cx="7" cy="7" r="6" fill={tokens.cta} opacity={0.2} />
          <path d="M4.5 7L6.5 9L9.5 5" stroke={tokens.cta} strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round" />
        </svg>
      );
    case 'error':
      return (
        <svg width="14" height="14" viewBox="0 0 14 14" fill="none" aria-hidden="true">
          <circle cx="7" cy="7" r="6" fill={tokens.error} opacity={0.2} />
          <path d="M5 5L9 9M9 5L5 9" stroke={tokens.error} strokeWidth="1.5" strokeLinecap="round" />
        </svg>
      );
  }
};

/**
 * Formats duration in milliseconds to human-readable string.
 */
function formatDuration(ms?: number): string {
  if (ms === undefined || ms === null) return '';
  if (ms < 1000) return `${ms}ms`;
  if (ms < 60000) return `${(ms / 1000).toFixed(1)}s`;
  const minutes = Math.floor(ms / 60000);
  const seconds = Math.floor((ms % 60000) / 1000);
  return `${minutes}m ${seconds}s`;
}

/**
 * Generates a user-friendly summary based on tool name and input.
 */
function generateSummary(tool: string, input: string): string {
  try {
    const parsed = JSON.parse(input);
    switch (tool) {
      case 'read_file':
        return `Reading ${parsed.path || parsed.filePath || 'file'}...`;
      case 'write_file':
        return `Writing to ${parsed.path || parsed.filePath || 'file'}...`;
      case 'ls':
      case 'list_dir':
        return `Listing ${parsed.path || parsed.directory || 'directory'}...`;
      case 'bash':
        return `Running: ${parsed.command?.substring(0, 50) || 'command'}${(parsed.command?.length || 0) > 50 ? '...' : ''}`;
      case 'grep':
      case 'search_file':
        return `Searching for "${parsed.pattern || parsed.query || ''}"...`;
      case 'glob':
        return `Finding files matching "${parsed.pattern || ''}"...`;
      default:
        return `Executing ${tool}...`;
    }
  } catch {
    // If input is not JSON, show truncated raw input
    const truncated = input.substring(0, 50);
    return `${tool}: ${truncated}${input.length > 50 ? '...' : ''}`;
  }
}

/**
 * ToolCallCard - Displays tool execution with summary, status, and expandable details.
 *
 * Features:
 * - Compact summary line with status icon and duration
 * - Expandable to show full input/output
 * - FileTreeRenderer for file_listing output type
 * - Accessible with proper ARIA attributes
 */
export function ToolCallCard({
  tool,
  input,
  output,
  error,
  outputType,
  status = 'success',
  durationMs,
}: ToolCallCardProps) {
  const [expanded, setExpanded] = useState(false);

  const summary = useMemo(() => generateSummary(tool, input), [tool, input]);

  const renderOutput = () => {
    if (!output) return null;

    // Use FileTreeRenderer for file listing outputs
    if (outputType === 'file_listing') {
      return (
        <div style={{ marginTop: 8 }}>
          <div style={{ color: tokens.muted, marginBottom: 4, fontSize: 12 }}>Files:</div>
          <FileTreeRenderer content={output} maxHeight={400} />
        </div>
      );
    }

    // Default: render as code block
    return (
      <div style={{ marginTop: 8 }}>
        <div style={{ color: tokens.muted, marginBottom: 4, fontSize: 12 }}>Output:</div>
        <pre style={{
          background: tokens.bg,
          padding: 8,
          borderRadius: 4,
          color: tokens.text,
          fontSize: 12,
          overflow: 'auto',
          maxHeight: 300,
          margin: 0,
          whiteSpace: 'pre-wrap',
          wordBreak: 'break-word',
        }}>
          {output}
        </pre>
      </div>
    );
  };

  return (
    <div style={{
      margin: '8px 0',
      padding: 12,
      borderRadius: 8,
      background: tokens.surface,
      border: `1px solid ${status === 'error' ? tokens.error : tokens.border}`,
      fontFamily: tokens.fontBody,
      fontSize: 13,
    }}>
      {/* Header with summary line */}
      <button
        type="button"
        onClick={() => setExpanded(!expanded)}
        aria-expanded={expanded}
        aria-label={`${expanded ? 'Collapse' : 'Expand'} ${tool} details`}
        style={{
          display: 'flex',
          alignItems: 'center',
          gap: 8,
          cursor: 'pointer',
          width: '100%',
          background: 'none',
          border: 'none',
          color: 'inherit',
          padding: 0,
          font: 'inherit',
          minHeight: 44,
          textAlign: 'left',
        }}
      >
        {/* Status icon */}
        <StatusIcon status={status} />

        {/* Summary text */}
        <span style={{
          flex: 1,
          color: status === 'error' ? tokens.error : tokens.text,
          overflow: 'hidden',
          textOverflow: 'ellipsis',
          whiteSpace: 'nowrap',
        }}>
          {summary}
        </span>

        {/* Duration */}
        {durationMs !== undefined && (
          <span style={{ color: tokens.muted, fontSize: 11, flexShrink: 0 }}>
            {formatDuration(durationMs)}
          </span>
        )}

        {/* Expand chevron */}
        <svg width="12" height="12" viewBox="0 0 12 12" fill="none" aria-hidden="true"
          style={{ transform: expanded ? 'rotate(180deg)' : undefined, transition: 'transform 200ms', flexShrink: 0 }}>
          <path d="M3 5l3 3 3-3" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round" />
        </svg>
      </button>

      {/* Expanded content */}
      {expanded && (
        <div style={{ marginTop: 8 }}>
          {/* Input section */}
          <div style={{ color: tokens.muted, marginBottom: 4, fontSize: 12 }}>Input:</div>
          <pre style={{
            background: tokens.bg,
            padding: 8,
            borderRadius: 4,
            color: tokens.text,
            fontSize: 12,
            overflow: 'auto',
            maxHeight: 120,
            margin: 0,
            whiteSpace: 'pre-wrap',
            wordBreak: 'break-word',
          }}>
            {input}
          </pre>

          {/* Output section */}
          {renderOutput()}

          {/* Error section */}
          {error && (
            <div style={{ color: tokens.error, marginTop: 8, fontSize: 12 }}>
              Error: {error}
            </div>
          )}
        </div>
      )}

      {/* CSS animation for spinner */}
      <style>{`
        @keyframes spin {
          from { transform: rotate(0deg); }
          to { transform: rotate(360deg); }
        }
      `}</style>
    </div>
  );
}