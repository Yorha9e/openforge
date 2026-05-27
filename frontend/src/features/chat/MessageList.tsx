import { useEffect, useRef, useMemo, useState } from 'react';
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

// --- Elegant thinking / tool-use folding logic ---

interface ContentPart {
  type: 'text' | 'thinking';
  content: string;
  toolName?: string;
  toolInput?: any;
}

function parseAgentContent(content: string): ContentPart[] {
  const parts: ContentPart[] = [];
  if (!content) return parts;

  // 1. Try parsing as Anthropic multi-part JSON (raw array of blocks)
  try {
    if (content.trim().startsWith('[') && content.trim().endsWith(']')) {
      const blocks = JSON.parse(content);
      if (Array.isArray(blocks)) {
        blocks.forEach((b: any) => {
          if (b.type === 'text' && b.text) {
            parts.push({ type: 'text', content: b.text });
          } else if (b.type === 'tool_use') {
            parts.push({
              type: 'thinking',
              content: `Calling tool: ${b.name}`,
              toolName: b.name,
              toolInput: b.input,
            });
          }
        });
        if (parts.length > 0) return parts;
      }
    }
  } catch {
    // Ignore parse errors, fall back to fenced-block regex
  }

  // 2. Fenced code block regex: extract ```json [...] ``` containing "tool_use"
  const fencedRe = /```(?:json)?\s*(\[\s*\{\s*"type"\s*:\s*"tool_use"[\s\S]*?\])\s*```/g;
  let match;
  let lastIndex = 0;

  while ((match = fencedRe.exec(content)) !== null) {
    const textBefore = content.substring(lastIndex, match.index);
    if (textBefore.trim()) {
      parts.push({ type: 'text', content: textBefore });
    }

    try {
      const jsonStr = match[1];
      if (!jsonStr) continue;
      const toolUseList = JSON.parse(jsonStr);
      if (Array.isArray(toolUseList)) {
        toolUseList.forEach((tu: any) => {
          parts.push({
            type: 'thinking',
            content: `Calling tool: ${tu.name}`,
            toolName: tu.name,
            toolInput: tu.input,
          });
        });
      } else {
        parts.push({ type: 'thinking', content: 'Tool execution logs' });
      }
    } catch {
      parts.push({ type: 'thinking', content: match[0] });
    }
    lastIndex = fencedRe.lastIndex;
  }

  const textAfter = content.substring(lastIndex);
  if (textAfter.trim() || parts.length === 0) {
    parts.push({ type: 'text', content: textAfter || content });
  }

  return parts;
}

function AgentThinkingBlock({ part }: { part: ContentPart }) {
  const [expanded, setExpanded] = useState(false);
  const hasInput = part.toolInput && Object.keys(part.toolInput).length > 0;

  return (
    <div style={{
      margin: '6px 0',
      background: 'rgba(30, 41, 59, 0.4)',
      border: `1px solid ${tokens.border}`,
      borderRadius: 6,
      overflow: 'hidden',
      fontSize: 12,
      fontFamily: tokens.fontHeading,
      transition: tokens.transition,
    }}>
      <div
        onClick={() => hasInput && setExpanded(!expanded)}
        style={{
          padding: '6px 12px',
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'space-between',
          cursor: hasInput ? 'pointer' : 'default',
          userSelect: 'none',
          color: tokens.muted,
        }}
      >
        <div style={{ display: 'flex', alignItems: 'center', gap: 6 }}>
          <span style={{
            display: 'inline-block',
            width: 6,
            height: 6,
            borderRadius: '50%',
            background: tokens.cta,
            boxShadow: `0 0 4px ${tokens.cta}`,
          }} />
          <span>
            Thinking: Executing <strong>{part.toolName || 'tool'}</strong>
          </span>
        </div>
        {hasInput && (
          <span style={{ fontSize: 10, color: tokens.muted, opacity: 0.7 }}>
            {expanded ? 'Collapse ▴' : 'Expand ▾'}
          </span>
        )}
      </div>
      {expanded && hasInput && (
        <div style={{
          padding: '8px 12px',
          borderTop: `1px solid ${tokens.border}`,
          background: 'rgba(15, 23, 42, 0.3)',
          maxHeight: 200,
          overflowY: 'auto',
          whiteSpace: 'pre-wrap',
          color: tokens.muted,
          opacity: 0.85,
        }}>
          <code>{JSON.stringify(part.toolInput, null, 2)}</code>
        </div>
      )}
    </div>
  );
}

export function MessageList() {
  const { messages, streaming, thinking } = useChat();
  const bottomRef = useRef<HTMLDivElement>(null);
  const listRef = useRef<HTMLDivElement>(null);

  // Auto-scroll to bottom when new messages arrive (only if already near bottom)
  useEffect(() => {
    try {
      const el = listRef.current;
      if (!el) return;
      const nearBottom = el.scrollHeight - el.scrollTop - el.clientHeight < 100;
      if (nearBottom) {
        bottomRef.current?.scrollIntoView({ behavior: 'smooth' });
      }
    } catch {
      // Ignore scroll errors — element may have been detached during transition
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
                <div>
                  {parseAgentContent(msg.content).map((part, pIdx) => {
                    if (part.type === 'thinking') {
                      return <AgentThinkingBlock key={pIdx} part={part} />;
                    }
                    return (
                      <div
                        key={pIdx}
                        className="markdown-body"
                        dangerouslySetInnerHTML={{ __html: renderMarkdown(part.content) }}
                        style={{
                          wordBreak: 'break-word',
                        }}
                      />
                    );
                  })}
                </div>
              )}
            </div>
          )}
        </div>
      ))}
      {thinking && !streaming && (
        <div style={{ display: 'flex', justifyContent: 'flex-start', marginBottom: 12 }}>
          <div style={{
            maxWidth: '80%', borderRadius: 8, padding: '10px 16px',
            background: tokens.surface, color: tokens.muted, fontSize: 14,
            lineHeight: 1.6, minWidth: 0,
            display: 'flex', alignItems: 'center', gap: 4,
          }}>
            Thinking
            <span className="thinking-dots">
              <span>.</span><span>.</span><span>.</span>
            </span>
          </div>
        </div>
      )}
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
        .markdown-body table::-webkit-scrollbar-thumb { background: ${tokens.border}; border-radius: 3px; }
        .thinking-dots span { display: inline-block; opacity: 0; animation: dot-blink 1.4s infinite; }
        .thinking-dots span:nth-child(1) { animation-delay: 0s; }
        .thinking-dots span:nth-child(2) { animation-delay: 0.2s; }
        .thinking-dots span:nth-child(3) { animation-delay: 0.4s; }
        @keyframes dot-blink { 0%, 80%, 100% { opacity: 0; } 40% { opacity: 1; } }`}
      </style>
    </div>
  );
}
