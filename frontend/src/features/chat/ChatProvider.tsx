import { createContext, useContext, useState, useCallback, useRef, useEffect, type ReactNode } from 'react';
import { useAuth } from '../../shared/auth';
import { useWebSocket } from './useWebSocket';
import type { ToolStatus } from './ToolCallCard';

interface Message {
  id: string;
  role: 'user' | 'agent' | 'system' | 'tool';
  content: string;
  timestamp: number;
  // Tool-specific fields (only when role === 'tool')
  toolName?: string;
  toolInput?: string;
  toolOutput?: string;
  toolOutputType?: string;
  toolStatus?: ToolStatus;
  toolDurationMs?: number;
  toolError?: string;
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
    const toolStartTimes = new Map<string, number>();

    const unsub5 = subscribe('tool.start', (p: any) => {
      const toolId = `tool-${++idCounter.current}`;
      toolStartTimes.set(toolId, Date.now());
      setMessages(prev => [...prev, {
        id: toolId,
        role: 'tool',
        content: `Running ${p?.tool_name}...`,
        timestamp: Date.now(),
        toolName: p?.tool_name,
        toolInput: p?.input,
        toolStatus: 'running',
      }]);
    });
    const unsub6 = subscribe('tool.done', (p: any) => {
      setMessages(prev => {
        // Find the last running tool message for this tool
        const lastToolIdx = [...prev].reverse().findIndex(
          m => m.role === 'tool' && m.toolName === p?.tool_name && m.toolStatus === 'running'
        );
        if (lastToolIdx === -1) {
          // No matching tool found, create new message
          return [...prev, {
            id: `tool-done-${++idCounter.current}`,
            role: 'tool' as const,
            content: `${p?.tool_name} completed`,
            timestamp: Date.now(),
            toolName: p?.tool_name,
            toolOutput: p?.output,
            toolOutputType: p?.output_type,
            toolStatus: 'success' as const,
          }];
        }
        // Update existing tool message
        const idx = prev.length - 1 - lastToolIdx;
        const toolMsg = prev[idx];
        const durationMs = toolStartTimes.get(toolMsg.id)
          ? Date.now() - toolStartTimes.get(toolMsg.id)!
          : undefined;
        toolStartTimes.delete(toolMsg.id);

        const updated = [...prev];
        updated[idx] = {
          ...toolMsg,
          content: `${p?.tool_name} completed`,
          toolOutput: p?.output,
          toolOutputType: p?.output_type,
          toolStatus: 'success',
          toolDurationMs: durationMs,
        };
        return updated;
      });
    });
    const unsub7 = subscribe('tool.error', (p: any) => {
      setMessages(prev => {
        // Find the last running tool message for this tool
        const lastToolIdx = [...prev].reverse().findIndex(
          m => m.role === 'tool' && m.toolName === p?.tool_name && m.toolStatus === 'running'
        );
        if (lastToolIdx === -1) {
          // No matching tool found, create new message
          return [...prev, {
            id: `tool-err-${++idCounter.current}`,
            role: 'tool' as const,
            content: `${p?.tool_name} failed`,
            timestamp: Date.now(),
            toolName: p?.tool_name,
            toolError: p?.error,
            toolStatus: 'error' as const,
          }];
        }
        // Update existing tool message
        const idx = prev.length - 1 - lastToolIdx;
        const toolMsg = prev[idx];
        const durationMs = toolStartTimes.get(toolMsg.id)
          ? Date.now() - toolStartTimes.get(toolMsg.id)!
          : undefined;
        toolStartTimes.delete(toolMsg.id);

        const updated = [...prev];
        updated[idx] = {
          ...toolMsg,
          content: `${p?.tool_name} failed`,
          toolError: p?.error,
          toolStatus: 'error',
          toolDurationMs: durationMs,
        };
        return updated;
      });
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
    const workDir = localStorage.getItem('openforge_work_dir') || '';
    wsSend('chat.send', { pipeline_id: pipelineId, message: content, work_dir: workDir });
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
