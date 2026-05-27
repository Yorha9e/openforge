import { useState, type FormEvent, type KeyboardEvent, useRef, useEffect } from 'react';
import { useChat } from './ChatProvider';
import { useSearchParams } from 'react-router-dom';
import { tokens } from '../../shared/design-tokens';

interface SkillOption {
  name: string;
  version: string;
  description: string;
}

const availableSkills: SkillOption[] = [
  { name: 'conduit-backend', version: '1.0.0', description: 'Conduit Express + TypeScript backend patterns' },
  { name: 'conduit-frontend', version: '1.0.0', description: 'Conduit React + TypeScript frontend patterns' },
];

export function MessageInput() {
  const [input, setInput] = useState('');
  const { send, cancel, connected, thinking, streaming } = useChat();
  const [params] = useSearchParams();
  const pipelineId = params.get('pipeline') || 'default';
  const [inputFocused, setInputFocused] = useState(false);
  const [btnHovered, setBtnHovered] = useState(false);
  const [showSkills, setShowSkills] = useState(false);
  const dropdownRef = useRef<HTMLDivElement>(null);

  const generating = thinking || !!streaming;

  useEffect(() => {
    function handleClick(e: MouseEvent) {
      if (dropdownRef.current && !dropdownRef.current.contains(e.target as Node)) {
        setShowSkills(false);
      }
    }
    document.addEventListener('mousedown', handleClick);
    return () => document.removeEventListener('mousedown', handleClick);
  }, []);

  const doSend = () => {
    if (!input.trim() || !connected) return;
    send(pipelineId, input.trim());
    setInput('');
  };

  const insertSkill = (skillName: string) => {
    setInput(`/skill ${skillName} `);
    setShowSkills(false);
  };

  const handleSubmit = (e: FormEvent) => {
    e.preventDefault();
    if (generating) {
      cancel();
    } else {
      doSend();
    }
  };

  const handleKeyDown = (e: KeyboardEvent<HTMLTextAreaElement>) => {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault();
      doSend();
    }
  };

  return (
    <form onSubmit={handleSubmit} style={{ borderTop: `1px solid ${tokens.border}`, padding: 16 }}>
      <div style={{ maxWidth: 640, margin: '0 auto', display: 'flex', gap: 8 }}>
        <textarea
          value={input}
          onChange={e => setInput(e.target.value)}
          onKeyDown={handleKeyDown}
          onFocus={() => setInputFocused(true)}
          onBlur={() => setInputFocused(false)}
          placeholder={connected ? 'Type a message or /skill <name>...' : 'Connecting...'}
          disabled={!connected}
          rows={2}
          aria-label="Message input"
          style={{
            flex: 1, padding: '8px 12px', background: tokens.bg, border: `1px solid ${tokens.border}`, borderRadius: 4, color: tokens.text,
            resize: 'none', opacity: connected ? 1 : 0.5, fontFamily: tokens.fontBody,
            outlineWidth: inputFocused ? 2 : 0, outlineStyle: inputFocused ? 'solid' : 'none', outlineColor: tokens.cta, outlineOffset: 2,
            transition: tokens.transition,
          }} />
        <div ref={dropdownRef} style={{ position: 'relative' }}>
          <button
            type="button"
            onClick={() => setShowSkills(!showSkills)}
            disabled={!connected}
            aria-label="Select skill"
            title="Select skill"
            style={{
              padding: '8px 10px', background: 'transparent', border: `1px solid ${tokens.border}`,
              borderRadius: 4, cursor: connected ? 'pointer' : 'default',
              opacity: connected ? 1 : 0.5, alignSelf: 'flex-end', fontSize: 14,
              color: tokens.text, transition: tokens.transition,
            }}>
            <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
              <path d="M12 2L2 7l10 5 10-5-10-5z"/><path d="M2 17l10 5 10-5"/><path d="M2 12l10 5 10-5"/>
            </svg>
          </button>
          {showSkills && (
            <div style={{
              position: 'absolute', bottom: '100%', right: 0, marginBottom: 4,
              background: tokens.bg, border: `1px solid ${tokens.border}`, borderRadius: 4,
              minWidth: 220, zIndex: 100, boxShadow: '0 4px 12px rgba(0,0,0,0.15)',
              padding: 4,
            }}>
              {availableSkills.map(skill => (
                <button
                  key={skill.name}
                  type="button"
                  onClick={() => insertSkill(skill.name)}
                  style={{
                    display: 'block', width: '100%', textAlign: 'left', padding: '8px 12px',
                    border: 'none', background: 'transparent', color: tokens.text,
                    cursor: 'pointer', borderRadius: 2, fontSize: 13,
                  }}
                  onMouseEnter={e => { (e.target as HTMLElement).style.background = tokens.border; }}
                  onMouseLeave={e => { (e.target as HTMLElement).style.background = 'transparent'; }}
                >
                  <div style={{ fontWeight: 500 }}>{skill.name} <span style={{ fontSize: 11, color: tokens.muted }}>v{skill.version}</span></div>
                  <div style={{ fontSize: 11, color: tokens.muted }}>{skill.description}</div>
                </button>
              ))}
              <div style={{ padding: '4px 12px', fontSize: 11, color: tokens.muted, borderTop: `1px solid ${tokens.border}` }}>
                Type /skill {'<'}name{'>'} or select above
              </div>
            </div>
          )}
        </div>
        <button
          type="submit"
          disabled={!connected || (!input.trim() && !generating)}
          onMouseEnter={() => setBtnHovered(true)}
          onMouseLeave={() => setBtnHovered(false)}
          onClick={(e) => {
            if (generating) {
              e.preventDefault();
              cancel();
              return;
            }
          }}
          aria-label={generating ? 'Stop generation' : 'Send message'}
          title={generating ? 'Stop generation' : 'Send message'}
          style={{
            padding: '8px 16px', minWidth: 60, minHeight: 44,
            background: generating
              ? (btnHovered ? tokens.error : tokens.warning)
              : (btnHovered && connected && input.trim() ? tokens.ctaHover : tokens.cta),
            color: generating ? '#fff' : tokens.ctaText,
            border: 'none', borderRadius: 4, fontWeight: 500,
            cursor: generating ? 'pointer' : (connected && input.trim() ? 'pointer' : 'default'),
            opacity: generating ? 1 : (connected && input.trim() ? 1 : 0.5),
            alignSelf: 'flex-end',
            transition: tokens.transition,
            display: 'flex', alignItems: 'center', justifyContent: 'center', gap: 6,
          }}>
          {generating ? (
            <svg width="14" height="14" viewBox="0 0 24 24" fill="currentColor" aria-hidden="true">
              <rect x="4" y="4" width="16" height="16" rx="2" />
            </svg>
          ) : (
            <>
              <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
                <line x1="22" y1="2" x2="11" y2="13" /><polygon points="22 2 15 22 11 13 2 9 22 2" />
              </svg>
              Send
            </>
          )}
        </button>
      </div>
    </form>
  );
}
