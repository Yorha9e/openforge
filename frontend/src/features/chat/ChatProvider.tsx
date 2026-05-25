import { createContext, useContext, useState, useCallback, useRef, useEffect, type ReactNode } from 'react';
import { useAuth } from '../../shared/auth';
import { useWebSocket } from './useWebSocket';

interface Message {
  id: string;
  role: 'user' | 'agent' | 'system';
  content: string;
  timestamp: number;
}

interface ChatState {
  messages: Message[];
  streaming: string;
  connected: boolean;
  send: (pipelineId: string, content: string) => void;
  clear: () => void;
}

const ChatContext = createContext<ChatState>({
  messages: [], streaming: '', connected: false,
  send: () => {}, clear: () => {},
});

export function ChatProvider({ pipelineId, children }: { pipelineId: string; children: ReactNode }) {
  const { token } = useAuth();
  const { status, send: wsSend, subscribe } = useWebSocket(token);
  const [messages, setMessages] = useState<Message[]>([]);
  const [streaming, setStreaming] = useState('');
  const streamingRef = useRef('');
  const idCounter = useRef(0);

  useEffect(() => {
    const unsub1 = subscribe('chat.stream', (p: any) => {
      streamingRef.current += p?.delta || '';
      setStreaming(streamingRef.current);
    });
    const unsub2 = subscribe('chat.stream_done', (p: any) => {
      const finalContent = p?.content || streamingRef.current;
      setMessages(prev => [...prev, {
        id: `agent-${++idCounter.current}`, role: 'agent',
        content: finalContent, timestamp: Date.now(),
      }]);
      setStreaming('');
      streamingRef.current = '';
    });
    const unsub3 = subscribe('error', (p: any) => {
      setMessages(prev => [...prev, {
        id: `err-${++idCounter.current}`, role: 'system',
        content: `Error: ${p?.message || 'Unknown error'}`, timestamp: Date.now(),
      }]);
      setStreaming('');
      streamingRef.current = '';
    });
    const unsub4 = subscribe('msg.card', (p: any) => {
      setMessages(prev => [...prev, {
        id: `card-${++idCounter.current}`,
        role: 'system',
        content: `[${p?.card_type || 'card'}] ${p?.title || ''}`,
        timestamp: Date.now(),
      }]);
    });
    // Pipeline stage/gate events are shown in the progress indicator, not as chat messages.

    // Tool execution events
    const unsub5 = subscribe('tool.start', (p: any) => {
      setMessages(prev => [...prev, {
        id: `tool-start-${++idCounter.current}`,
        role: 'system',
        content: `Running ${p?.tool_name}...`,
        timestamp: Date.now(),
      }]);
    });
    const unsub6 = subscribe('tool.done', (p: any) => {
      setMessages(prev => [...prev, {
        id: `tool-done-${++idCounter.current}`,
        role: 'system',
        content: `${p?.tool_name}: ${p?.status} (${p?.duration_ms}ms)`,
        timestamp: Date.now(),
      }]);
    });
    const unsub7 = subscribe('tool.error', (p: any) => {
      setMessages(prev => [...prev, {
        id: `tool-err-${++idCounter.current}`,
        role: 'system',
        content: `${p?.tool_name} failed: ${p?.error_code} — ${p?.message}`,
        timestamp: Date.now(),
      }]);
    });

    // Context compression notification
    const unsub8 = subscribe('context.compress', (p: any) => {
      setMessages(prev => [...prev, {
        id: `compress-${++idCounter.current}`,
        role: 'system',
        content: `Context compressed: ${p?.before_tokens} → ${p?.after_tokens} tokens (${p?.rounds_compressed} rounds)`,
        timestamp: Date.now(),
      }]);
    });

    return () => { unsub1(); unsub2(); unsub3(); unsub4(); unsub5(); unsub6(); unsub7(); unsub8(); };
  }, [subscribe]);

  const send = useCallback((_pid: string, content: string) => {
    setMessages(prev => [...prev, {
      id: `user-${++idCounter.current}`, role: 'user',
      content, timestamp: Date.now(),
    }]);
    wsSend('chat.send', { pipeline_id: pipelineId, message: content });
  }, [wsSend, pipelineId]);

  const clear = useCallback(() => {
    setMessages([]); setStreaming(''); streamingRef.current = '';
  }, []);

  return (
    <ChatContext.Provider value={{ messages, streaming, connected: status === 'open', send, clear }}>
      {children}
    </ChatContext.Provider>
  );
}

export function useChat() { return useContext(ChatContext); }
