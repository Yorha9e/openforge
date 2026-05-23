import { useState, type FormEvent, type KeyboardEvent } from 'react';
import { useChat } from './ChatProvider';
import { useSearchParams } from 'react-router-dom';
import { tokens } from '../../shared/design-tokens';

export function MessageInput() {
  const [input, setInput] = useState('');
  const { send, connected } = useChat();
  const [params] = useSearchParams();
  const pipelineId = params.get('pipeline') || 'default';
  const [inputFocused, setInputFocused] = useState(false);
  const [btnHovered, setBtnHovered] = useState(false);

  const doSend = () => {
    if (!input.trim() || !connected) return;
    send(pipelineId, input.trim());
    setInput('');
  };

  const handleSubmit = (e: FormEvent) => {
    e.preventDefault();
    doSend();
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
          placeholder={connected ? 'Type a message...' : 'Connecting...'}
          disabled={!connected}
          rows={2}
          aria-label="Message input"
          style={{
            flex: 1, padding: '8px 12px', background: tokens.bg, border: `1px solid ${tokens.border}`, borderRadius: 4, color: tokens.text,
            resize: 'none', opacity: connected ? 1 : 0.5, fontFamily: tokens.fontBody,
            outline: inputFocused ? '2px solid' : 'none', outlineColor: tokens.cta, outlineOffset: 2,
            transition: tokens.transition,
          }} />
        <button
          type="submit"
          disabled={!connected || !input.trim()}
          onMouseEnter={() => setBtnHovered(true)}
          onMouseLeave={() => setBtnHovered(false)}
          aria-label="Send message"
          style={{
            padding: '8px 16px', background: btnHovered && connected && input.trim() ? tokens.ctaHover : tokens.cta,
            color: tokens.ctaText, border: 'none', borderRadius: 4, fontWeight: 500,
            cursor: connected && input.trim() ? 'pointer' : 'default',
            opacity: connected && input.trim() ? 1 : 0.5, alignSelf: 'flex-end',
            transition: tokens.transition,
          }}>
          Send
        </button>
      </div>
    </form>
  );
}
