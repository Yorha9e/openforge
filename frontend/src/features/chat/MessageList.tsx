import { useEffect, useRef } from 'react';
import { useChat } from './ChatProvider';
import { sanitizeHTML } from '../../shared/sanitize';
import { tokens } from '../../shared/design-tokens';
import { TextSkeleton } from '../../shared/skeleton';

export function MessageList() {
  const { messages, streaming } = useChat();
  const bottomRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    bottomRef.current?.scrollIntoView({ behavior: 'smooth' });
  }, [messages, streaming]);

  if (!messages) return <div style={{ padding: 16 }}><TextSkeleton lines={5} /></div>;

  return (
    <div style={{ flex: 1, overflowY: 'auto', padding: 16, fontFamily: tokens.fontBody }} aria-live="polite" role="log" aria-label="Chat messages">
      {messages.map(msg => (
        <div key={msg.id} style={{ display: 'flex', justifyContent: msg.role === 'user' ? 'flex-end' : 'flex-start', marginBottom: 12 }}>
          <div style={{
            maxWidth: '80%', borderRadius: 8, padding: '8px 16px',
            background: msg.role === 'user' ? tokens.userBubble : msg.role === 'system' ? 'rgba(185,28,28,0.3)' : tokens.surface,
            color: msg.role === 'system' ? tokens.error : tokens.text,
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
          <div style={{ maxWidth: '80%', borderRadius: 8, padding: '8px 16px', background: tokens.surface, color: tokens.text }}>
            <div dangerouslySetInnerHTML={{ __html: sanitizeHTML(streaming) }} />
            <span style={{ display: 'inline-block', width: 8, height: 16, background: tokens.muted, marginLeft: 4, animation: 'pulse 1s infinite' }} />
          </div>
        </div>
      )}
      <div ref={bottomRef} />
      <style>{`@keyframes pulse { 0%, 100% { opacity: 1; } 50% { opacity: 0.3; } }`}</style>
    </div>
  );
}
