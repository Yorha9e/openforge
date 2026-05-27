import { createContext, useContext, useState, useCallback, useRef, useEffect, type ReactNode } from 'react';
import { useAuth } from '../../shared/auth';
import { useWebSocket } from './useWebSocket';
import { api } from '../../shared/api';
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

interface PipelineStageInfo {
  stage: string;
  status: string;
  pipelineId: string;
}

interface ChatState {
  messages: Message[];
  streaming: string;
  thinking: boolean;
  connected: boolean;
  pipelineStage: PipelineStageInfo | null;
  send: (pipelineId: string, content: string) => void;
  cancel: () => void;
  clear: () => void;
}

const ChatContext = createContext<ChatState>({
  messages: [], streaming: '', thinking: false, connected: false, pipelineStage: null,
  send: () => {}, cancel: () => {}, clear: () => {},
});

export function ChatProvider({ pipelineId, children }: { pipelineId: string; children: ReactNode }) {
  const { token } = useAuth();
  const { status, send: wsSend, subscribe } = useWebSocket(token);
  const [messages, setMessages] = useState<Message[]>([]);
  const [streaming, setStreaming] = useState('');
  const [thinking, setThinking] = useState(false);
  const [pipelineStage, setPipelineStage] = useState<PipelineStageInfo | null>(null);
  const streamingRef = useRef('');
  const idCounter = useRef(0);

  // Load historical messages from DB when pipeline changes
  useEffect(() => {
    setMessages([]);
    setStreaming('');
    setThinking(false);
    streamingRef.current = '';
    idCounter.current = 0;

    if (!pipelineId || pipelineId === 'default') return;
    api.getMessages(pipelineId).then((msgs: any[]) => {
      if (!Array.isArray(msgs)) return;
      const historical: Message[] = msgs.map((m: any, idx: number) => {
        const base: Message = {
          id: m.id || `hist-${idx}`,
          role: m.role as Message['role'],
          content: m.content || '',
          timestamp: m.created_at ? new Date(m.created_at).getTime() : Date.now(),
        };
        if (m.role === 'tool') {
          base.toolName = m.toolName;
          base.toolInput = m.toolInput;
          base.toolOutput = m.toolOutput;
          base.toolOutputType = m.toolOutputType;
          base.toolStatus = (m.toolStatus || 'success') as ToolStatus;
          base.toolDurationMs = m.toolDurationMs;
          base.toolError = m.toolError;
        }
        return base;
      });
      if (historical.length > 0) {
        idCounter.current = historical.length;
      }
      setMessages(historical);
    }).catch(() => { /* silent, show empty chat */ });
  }, [pipelineId]);

  useEffect(() => {
    const unsub1 = subscribe('chat.stream', (p: any) => {
      setThinking(false);
      streamingRef.current += p?.delta || '';
      setStreaming(streamingRef.current);
    });
    const unsub2 = subscribe('chat.stream_done', (p: any) => {
      setThinking(false);
      const finalContent = p?.content || streamingRef.current;
      setMessages(prev => [...prev, {
        id: `agent-${++idCounter.current}`, role: 'agent',
        content: finalContent, timestamp: Date.now(),
      }]);
      setStreaming('');
      streamingRef.current = '';
    });
    const unsub3 = subscribe('error', (p: any) => {
      setThinking(false);
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
    // Pipeline stage/gate events
    const unsub9 = subscribe('pipeline.stage_change', (p: any) => {
      setPipelineStage({
        stage: p?.stage || '',
        status: p?.status || '',
        pipelineId: p?.pipeline_id || '',
      });
      setMessages(prev => [...prev, {
        id: `stage-${++idCounter.current}`, role: 'system',
        content: `Pipeline stage changed: ${p?.stage || 'unknown'} (${p?.status || 'unknown'})`,
        timestamp: Date.now(),
      }]);
    });
    const unsub10 = subscribe('pipeline.token_warning', (p: any) => {
      const used = p?.used ?? 0;
      const budget = p?.budget ?? 4096;
      setMessages(prev => [...prev, {
        id: `tokwarn-${++idCounter.current}`, role: 'system',
        content: `Token usage warning: ${used}/${budget} tokens used`,
        timestamp: Date.now(),
      }]);
    });
    const unsub11 = subscribe('pipeline.finished', (p: any) => {
      setPipelineStage(prev => prev ? { ...prev, status: p?.status || 'completed' } : null);
      setMessages(prev => [...prev, {
        id: `pf-${++idCounter.current}`, role: 'system',
        content: `Pipeline finished: ${p?.status || 'completed'}`,
        timestamp: Date.now(),
      }]);
    });

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

    return () => { unsub1(); unsub2(); unsub3(); unsub4(); unsub5(); unsub6(); unsub7(); unsub8(); unsub9(); unsub10(); unsub11(); };
  }, [subscribe]);

  const send = useCallback((_pid: string, content: string) => {
    if (status !== 'open') {
      setMessages(prev => [...prev, {
        id: `err-${++idCounter.current}`, role: 'system',
        content: 'Cannot send message: WebSocket is not connected. Please wait for reconnection...',
        timestamp: Date.now(),
      }]);
      return;
    }
    setThinking(true);
    setMessages(prev => [...prev, {
      id: `user-${++idCounter.current}`, role: 'user',
      content, timestamp: Date.now(),
    }]);
    const workDir = localStorage.getItem('openforge_work_dir') || '';
    wsSend('chat.send', { pipeline_id: pipelineId, message: content, work_dir: workDir });
  }, [wsSend, pipelineId, status]);

  const cancel = useCallback(() => {
    setThinking(false);
    if (status !== 'open') return;
    wsSend('chat.stop', {});
  }, [wsSend, status]);

  const clear = useCallback(() => {
    setMessages([]); setStreaming(''); setThinking(false); streamingRef.current = '';
  }, []);

  return (
    <ChatContext.Provider value={{ messages, streaming, thinking, connected: status === 'open', pipelineStage, send, cancel, clear }}>
      {children}
    </ChatContext.Provider>
  );
}

export function useChat() { return useContext(ChatContext); }
