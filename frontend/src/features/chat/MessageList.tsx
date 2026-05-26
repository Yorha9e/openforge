import { useEffect, useRef, useMemo } from 'react';
import { useChat } from './ChatProvider';
import { sanitizeHTML } from '../../shared/sanitize';
import { tokens } from '../../shared/design-tokens';
import { TextSkeleton } from '../../shared/skeleton';
import { marked } from 'marked';
import { ToolCallCard } from './ToolCallCard';

// Configure marked for safety
marked.setOptions({
  breaks: true,
  gfm: true,
});

function renderMarkdown(text: string): string {
  try {
    const html = marked.parse(text) as string;
    return sanitizeHTML(html);
  } catch {
    return sanitizeHTML(text);
  }
}

export function MessageList() {
  const { messages, streaming } = useChat();
  const bottomRef = useRef<HTMLDivElement>(null);
  const listRef = useRef<HTMLDivElement>(null);

  // Auto-scroll to bottom when new messages arrive (only if already near bottom)
  useEffect(() => {
    const el = listRef.current;
    if (!el) return;
    const nearBottom = el.scrollHeight - el.scrollTop - el.clientHeight < 100;
    if (nearBottom) {
      bottomRef.current?.scrollIntoView({ behavior: 'smooth' });
    }
  }, [messages, streaming]);

  if (!messages) return <div style={{ padding: 16 }}><TextSkeleton lines={5} /></div>;

  return (
    <div ref={listRef} style={{ flex: 1, overflowY: 'auto', padding: '16px 24px', fontFamily: tokens.fontBody }} aria-live="polite" role="log" aria-label="Chat messages">
      {messages.map(msg => (
        <div key={msg.id} style={{ display: 'flex', justifyContent: msg.role === 'user' ? 'flex-end' : 'flex-start', marginBottom: 12 }}>
          {msg.role === 'tool' ? (
            <div style={{ maxWidth: '80%', minWidth: 0, width: '100%' }}>
              <ToolCallCard
                tool={msg.toolName || 'unknown'}
                input={msg.toolInput || ''}
                output={msg.toolOutput}
                error={msg.toolError}
                outputType={msg.toolOutputType}
                status={msg.toolStatus}
                durationMs={msg.toolDurationMs}
              />
            </div>
          ) : (
            <div style={{
              maxWidth: '80%', borderRadius: 8, padding: '10px 16px',
              background: msg.role === 'user' ? tokens.userBubble : msg.role === 'system' ? 'rgba(185,28,28,0.3)' : tokens.surface,
              color: msg.role === 'system' ? tokens.error : tokens.text,
              fontSize: 14, lineHeight: 1.6,
              overflowWrap: 'break-word',
              overflowX: 'auto',
              minWidth: 0,
            }}>
              {msg.role === 'user' ? (
                <p style={{ whiteSpace: 'pre-wrap', margin: 0 }}>{msg.content}</p>
              ) : (
                <div
                  className="markdown-body"
                  dangerouslySetInnerHTML={{ __html: renderMarkdown(msg.content) }}
                  style={{
                    // Minimal markdown styling that stays within design system
                    wordBreak: 'break-word',
                  }}
                />
              )}
            </div>
          )}
        </div>
      ))}
      {streaming && (
        <div style={{ display: 'flex', justifyContent: 'flex-start', marginBottom: 12 }}>
          <div style={{ maxWidth: '80%', borderRadius: 8, padding: '10px 16px', background: tokens.surface, color: tokens.text, fontSize: 14, lineHeight: 1.6, overflowX: 'auto', minWidth: 0 }}>
            <div
              className="markdown-body"
              dangerouslySetInnerHTML={{ __html: renderMarkdown(streaming) }}
            />
            <span style={{ display: 'inline-block', width: 8, height: 16, background: tokens.cta, marginLeft: 4, verticalAlign: 'text-bottom' }} />
          </div>
        </div>
      )}
      <div ref={bottomRef} />
      <style>{`
        .markdown-body p { margin: 0 0 8px 0; }
        .markdown-body p:last-child { margin-bottom: 0; }
        .markdown-body code { background: ${tokens.bg}; padding: 2px 6px; border-radius: 3px; font-size: 13px; font-family: ${tokens.fontHeading}; }
        .markdown-body pre { background: ${tokens.bg}; padding: 12px; border-radius: 6px; overflow-x: auto; margin: 8px 0; max-width: 100%; }
        .markdown-body pre code { background: none; padding: 0; white-space: pre; }
        .markdown-body ul, .markdown-body ol { padding-left: 20px; margin: 4px 0; }
        .markdown-body blockquote { border-left: 3px solid ${tokens.cta}; padding-left: 12px; margin: 8px 0; color: ${tokens.muted}; overflow-x: auto; }
        .markdown-body h1, .markdown-body h2, .markdown-body h3 { margin: 12px 0 4px 0; font-family: ${tokens.fontHeading}; }
        .markdown-body h1 { font-size: 1.4em; } .markdown-body h2 { font-size: 1.2em; } .markdown-body h3 { font-size: 1.05em; }
        .markdown-body a { color: ${tokens.cta}; word-break: break-all; }
        .markdown-body table { border-collapse: collapse; margin: 8px 0; display: block; overflow-x: auto; max-width: 100%; }
        .markdown-body th, .markdown-body td { border: 1px solid ${tokens.border}; padding: 6px 10px; text-align: left; font-size: 13px; white-space: nowrap; }
        .markdown-body th { background: ${tokens.surface}; font-weight: 600; }
        .markdown-body hr { border: none; border-top: 1px solid ${tokens.border}; margin: 12px 0; }
        .markdown-body img { max-width: 100%; height: auto; }
        .markdown-body pre::-webkit-scrollbar,
        .markdown-body table::-webkit-scrollbar { height: 6px; }
        .markdown-body pre::-webkit-scrollbar-track,
        .markdown-body table::-webkit-scrollbar-track { background: transparent; }
        .markdown-body pre::-webkit-scrollbar-thumb,
        .markdown-body table::-webkit-scrollbar-thumb { background: ${tokens.border}; border-radius: 3px; }`}
      </style>
    </div>
  );
}
