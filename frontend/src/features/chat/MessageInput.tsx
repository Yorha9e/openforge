import { useState, type FormEvent, type KeyboardEvent } from 'react';
import { useChat } from './ChatProvider';
import { useSearchParams } from 'react-router-dom';

export function MessageInput() {
  const [input, setInput] = useState('');
  const { send, connected } = useChat();
  const [params] = useSearchParams();
  const pipelineId = params.get('pipeline') || 'default';

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
    <form onSubmit={handleSubmit} style={{ borderTop: '1px solid #262626', padding: 16 }}>
      <div style={{ maxWidth: 640, margin: '0 auto', display: 'flex', gap: 8 }}>
        <textarea value={input} onChange={e => setInput(e.target.value)}
          onKeyDown={handleKeyDown}
          placeholder={connected ? 'Type a message...' : 'Connecting...'}
          disabled={!connected}
          rows={2}
          style={{ flex: 1, padding: '8px 12px', background: '#262626', border: '1px solid #404040', borderRadius: 4, color: '#fff', resize: 'none', opacity: connected ? 1 : 0.5 }} />
        <button type="submit" disabled={!connected || !input.trim()}
          style={{ padding: '8px 16px', background: '#2563eb', color: '#fff', border: 'none', borderRadius: 4, fontWeight: 500, cursor: connected && input.trim() ? 'pointer' : 'default', opacity: connected && input.trim() ? 1 : 0.5, alignSelf: 'flex-end' }}>
          Send
        </button>
      </div>
    </form>
  );
}
