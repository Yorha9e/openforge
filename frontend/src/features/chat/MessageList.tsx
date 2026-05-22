import { useEffect, useRef } from 'react';
import { useChat } from './ChatProvider';
import { sanitizeHTML } from '../../shared/sanitize';

export function MessageList() {
  const { messages, streaming } = useChat();
  const bottomRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    bottomRef.current?.scrollIntoView({ behavior: 'smooth' });
  }, [messages, streaming]);

  return (
    <div style={{ flex: 1, overflowY: 'auto', padding: 16 }}>
      {messages.map(msg => (
        <div key={msg.id} style={{ display: 'flex', justifyContent: msg.role === 'user' ? 'flex-end' : 'flex-start', marginBottom: 12 }}>
          <div style={{
            maxWidth: '80%', borderRadius: 8, padding: '8px 16px',
            background: msg.role === 'user' ? '#2563eb' : msg.role === 'system' ? 'rgba(185,28,28,0.3)' : '#262626',
            color: msg.role === 'system' ? '#fca5a5' : '#fff',
          }}>
            {msg.role === 'agent'
              ? <div dangerouslySetInnerHTML={{ __html: sanitizeHTML(msg.content) }} />
              : <p style={{ whiteSpace: 'pre-wrap', margin: 0 }}>{msg.content}</p>
            }
          </div>
        </div>
      ))}
      {streaming && (
        <div style={{ display: 'flex', justifyContent: 'flex-start', marginBottom: 12 }}>
          <div style={{ maxWidth: '80%', borderRadius: 8, padding: '8px 16px', background: '#262626', color: '#fff' }}>
            <div dangerouslySetInnerHTML={{ __html: sanitizeHTML(streaming) }} />
            <span style={{ display: 'inline-block', width: 8, height: 16, background: '#a3a3a3', marginLeft: 4, animation: 'pulse 1s infinite' }} />
          </div>
        </div>
      )}
      <div ref={bottomRef} />
      <style>{`@keyframes pulse { 0%, 100% { opacity: 1; } 50% { opacity: 0.3; } }`}</style>
    </div>
  );
}
