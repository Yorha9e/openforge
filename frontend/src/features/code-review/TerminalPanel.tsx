import { useEffect, useRef } from 'react';
import { useProMode } from './ProModeContext';
import { tokens } from '../../shared/design-tokens';

export function TerminalPanel() {
  const { executionLogs, clearLogs } = useProMode();
  const containerRef = useRef<HTMLDivElement>(null);
  const isAutoScrollRef = useRef(true);

  useEffect(() => {
    if (isAutoScrollRef.current && containerRef.current) {
      containerRef.current.scrollTop = containerRef.current.scrollHeight;
    }
  }, [executionLogs]);

  const handleScroll = () => {
    if (!containerRef.current) return;
    const { scrollTop, scrollHeight, clientHeight } = containerRef.current;
    isAutoScrollRef.current = scrollHeight - scrollTop - clientHeight < 50;
  };

  const getTypeColor = (type: string) => {
    switch (type) {
      case 'tool.start': return '#00bcd4';
      case 'tool.done': return '#4caf50';
      case 'tool.error': return '#f44336';
      case 'stage_change': return '#9c27b0';
      case 'context.compress': return '#ff9800';
      case 'pipeline.finished': return '#4caf50';
      default: return tokens.text;
    }
  };

  const getTypeIcon = (type: string) => {
    switch (type) {
      case 'tool.start': return '▶';
      case 'tool.done': return '✓';
      case 'tool.error': return '✗';
      case 'stage_change': return '→';
      case 'context.compress': return '📦';
      case 'pipeline.finished': return '🏁';
      default: return '•';
    }
  };

  const formatTimestamp = (timestamp: number) => {
    const date = new Date(timestamp);
    return date.toLocaleTimeString('en-US', { hour12: false });
  };

  return (
    <div style={{
      height: '100%',
      display: 'flex',
      flexDirection: 'column',
      background: '#0d1117',
      color: '#c9d1d9',
      fontFamily: 'Consolas, Monaco, "Courier New", monospace',
      fontSize: 13,
    }}>
      <div style={{
        padding: '8px 12px',
        borderBottom: '1px solid #21262d',
        display: 'flex',
        justifyContent: 'space-between',
        alignItems: 'center',
      }}>
        <span style={{ color: '#8b949e' }}>Terminal</span>
        <button
          onClick={clearLogs}
          style={{
            background: 'transparent',
            border: '1px solid #30363d',
            borderRadius: 4,
            padding: '2px 8px',
            color: '#8b949e',
            cursor: 'pointer',
            fontSize: 11,
          }}
        >
          Clear
        </button>
      </div>
      <div
        ref={containerRef}
        onScroll={handleScroll}
        style={{
          flex: 1,
          overflow: 'auto',
          padding: '8px 12px',
        }}
      >
        {executionLogs.length === 0 ? (
          <div style={{ color: '#484f58', fontStyle: 'italic' }}>
            Waiting for execution logs...
          </div>
        ) : (
          executionLogs.map((log, index) => (
            <div
              key={index}
              style={{
                display: 'flex',
                gap: 8,
                marginBottom: 4,
                lineHeight: 1.4,
              }}
            >
              <span style={{ color: '#484f58', minWidth: 70 }}>
                {formatTimestamp(log.timestamp)}
              </span>
              <span style={{ color: getTypeColor(log.type), minWidth: 16 }}>
                {getTypeIcon(log.type)}
              </span>
              <span style={{ color: getTypeColor(log.type) }}>
                {log.type}
              </span>
              <span style={{ color: '#8b949e' }}>
                {log.tool_name || log.stage || ''}
              </span>
              {log.error && (
                <span style={{ color: '#f44336' }}>
                  {log.error}
                </span>
              )}
            </div>
          ))
        )}
      </div>
    </div>
  );
}