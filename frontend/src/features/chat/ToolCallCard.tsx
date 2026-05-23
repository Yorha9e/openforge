import { tokens } from '../../shared/design-tokens';
import { useState } from 'react';

export function ToolCallCard({ tool, input, output, error }: {
  tool: string; input: string; output?: string; error?: string;
}) {
  const [expanded, setExpanded] = useState(false);
  return (
    <div style={{
      margin: '8px 0', padding: 12, borderRadius: 8,
      background: tokens.surface, border: `1px solid ${tokens.border}`,
      fontFamily: tokens.fontBody, fontSize: 13,
    }}>
      <div style={{ display: 'flex', justifyContent: 'space-between', cursor: 'pointer' }}
        onClick={() => setExpanded(!expanded)}>
        <span style={{ fontFamily: tokens.fontHeading, color: tokens.cta, fontWeight: 600, display: 'flex', alignItems: 'center', gap: 6 }}>
          <svg width="14" height="14" viewBox="0 0 14 14" fill="none" aria-hidden="true">
            <path d="M2.5 3.5l3 3-3 3M7.5 3.5l3 3-3 3" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round" />
          </svg>
          {tool}
        </span>
        <svg width="12" height="12" viewBox="0 0 12 12" fill="none" aria-hidden="true"
          style={{ transform: expanded ? 'rotate(180deg)' : undefined, transition: 'transform 200ms' }}>
          <path d="M3 5l3 3 3-3" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round" />
        </svg>
      </div>
      {expanded && (
        <div style={{ marginTop: 8 }}>
          <div style={{ color: tokens.muted, marginBottom: 4 }}>Input:</div>
          <pre style={{ background: tokens.bg, padding: 8, borderRadius: 4, color: tokens.text, fontSize: 12, overflow: 'auto', maxHeight: 120 }}>
            {input}
          </pre>
          {output && (<><div style={{ color: tokens.muted, marginBottom: 4, marginTop: 8 }}>Output:</div>
            <pre style={{ background: tokens.bg, padding: 8, borderRadius: 4, color: tokens.text, fontSize: 12, overflow: 'auto', maxHeight: 120 }}>
              {output}
            </pre></>
          )}
          {error && <div style={{ color: tokens.error, marginTop: 4 }}>Error: {error}</div>}
        </div>
      )}
    </div>
  );
}
