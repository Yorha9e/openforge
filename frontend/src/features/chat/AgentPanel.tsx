import { useState } from 'react';
import { tokens } from '../../shared/design-tokens';

interface AgentInfo {
  id: string;
  role: string;
  pipeline_id: string;
  parent_id: string;
}

export function AgentPanel({ agents }: { agents: AgentInfo[] }) {
  const [collapsed, setCollapsed] = useState(false);

  return (
    <div style={{ borderBottom: `1px solid ${tokens.border}`, background: tokens.surface }}>
      <button
        onClick={() => setCollapsed(!collapsed)}
        aria-expanded={!collapsed}
        style={{
          width: '100%', padding: '8px 12px', background: 'none', border: 'none',
          color: tokens.muted, fontFamily: tokens.fontBody, fontSize: 12,
          cursor: 'pointer', textAlign: 'left', display: 'flex', justifyContent: 'space-between', alignItems: 'center',
        }}
      >
        Agents ({agents.length})
        <svg width="12" height="12" viewBox="0 0 12 12" fill="none" aria-hidden="true"
          style={{ transform: collapsed ? undefined : 'rotate(180deg)', transition: 'transform 200ms' }}>
          <path d="M3 5l3 3 3-3" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round" />
        </svg>
      </button>
      {!collapsed && (
        <div style={{ padding: '0 12px 8px' }}>
          {agents.map(a => (
            <div key={a.id} style={{ display: 'flex', gap: 8, alignItems: 'center', padding: '4px 0', fontSize: 12, color: tokens.muted }}>
              <span style={{
                width: 8, height: 8, borderRadius: '50%', background: tokens.cta,
                display: 'inline-block', opacity: a.role === 'pm' ? 1 : 0.5,
              }} />
              <span style={{ fontFamily: tokens.fontHeading, color: tokens.text }}>{a.id}</span>
              <span>{a.role}</span>
              {a.parent_id && <span style={{ color: tokens.muted }}>← {a.parent_id}</span>}
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
