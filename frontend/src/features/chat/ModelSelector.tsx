import { useState, useEffect, useRef } from 'react';
import { api } from '../../shared/api';
import { tokens } from '../../shared/design-tokens';

interface ModelSelectorProps {
  current: string;
  onSelect: (model: string) => void;
}

function ChevronDown() {
  return (
    <svg width="12" height="12" viewBox="0 0 12 12" fill="none" aria-hidden="true">
      <path d="M3 5l3 3 3-3" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round" />
    </svg>
  );
}

export function ModelSelector({ current, onSelect }: ModelSelectorProps) {
  const [models, setModels] = useState<any[]>([]);
  const [open, setOpen] = useState(false);
  const ref = useRef<HTMLDivElement>(null);

  useEffect(() => { api.listModels().then(setModels).catch(() => {}); }, []);

  // Close on outside click
  useEffect(() => {
    if (!open) return;
    const handler = (e: MouseEvent) => {
      if (ref.current && !ref.current.contains(e.target as Node)) setOpen(false);
    };
    document.addEventListener('mousedown', handler);
    return () => document.removeEventListener('mousedown', handler);
  }, [open]);

  return (
    <div ref={ref} style={{ position: 'relative' }}>
      <button
        onClick={() => setOpen(!open)}
        aria-expanded={open}
        aria-haspopup="listbox"
        style={{
          display: 'flex', alignItems: 'center', gap: 6,
          background: tokens.surface,
          border: `1px solid ${tokens.border}`,
          borderRadius: 6,
          padding: '8px 12px',
          minHeight: 44,
          color: tokens.text,
          fontFamily: tokens.fontBody,
          fontSize: 13,
          cursor: 'pointer',
          transition: tokens.transition,
        }}
      >
        <span>{current}</span>
        <span style={{ color: tokens.muted, display: 'flex', alignItems: 'center', transform: open ? 'rotate(180deg)' : undefined, transition: 'transform 200ms' }}>
          <ChevronDown />
        </span>
      </button>
      {open && (
        <div
          role="listbox"
          style={{
            position: 'absolute', top: '100%', right: 0, marginTop: 4,
            background: tokens.surface, border: `1px solid ${tokens.border}`,
            borderRadius: 6, minWidth: 200, zIndex: 50, overflow: 'hidden',
            boxShadow: '0 4px 12px rgba(0,0,0,0.3)',
          }}
        >
          {models.map(m => (
            <button
              key={m.alias}
              role="option"
              aria-selected={m.alias === current}
              onClick={() => { onSelect(m.alias); setOpen(false); }}
              style={{
                display: 'flex', justifyContent: 'space-between', alignItems: 'center',
                width: '100%', padding: '10px 14px', minHeight: 44,
                background: m.alias === current ? tokens.bg : 'transparent',
                border: 'none', color: m.alias === current ? tokens.cta : tokens.text,
                fontFamily: tokens.fontBody, fontSize: 13, cursor: 'pointer',
                textAlign: 'left', transition: tokens.transition,
              }}
            >
              <span>{m.alias}</span>
              <span style={{ color: tokens.muted, fontSize: 11 }}>{m.provider}</span>
            </button>
          ))}
          {models.length === 0 && (
            <div style={{ padding: '10px 14px', color: tokens.muted, fontSize: 13 }}>
              No models available
            </div>
          )}
        </div>
      )}
    </div>
  );
}
