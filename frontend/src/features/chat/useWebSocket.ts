import { useEffect, useRef, useCallback, useState } from 'react';
import { wsURL } from '../../shared/api';

type WSStatus = 'connecting' | 'open' | 'closed' | 'error';

export function useWebSocket(token: string | null) {
  const wsRef = useRef<WebSocket | null>(null);
  const [status, setStatus] = useState<WSStatus>('closed');
  const listenersRef = useRef<Map<string, Set<(payload: any) => void>>>(new Map());
  const reconnectTimer = useRef<number>(0);
  const reconnectDelay = useRef(1000);
  const reconnectAttempts = useRef(0);
  // lastSeqRef tracks the highest sequence number we've consumed from a
  // sync.replay event. On reconnect we send it back to the server in a
  // sync.request so the server can replay any events we missed.
  const lastSeqRef = useRef(0);
  // activePipelineIdRef remembers the most recent pipeline_id we sent
  // chat.send for; needed to scope the sync.request on reconnect.
  const activePipelineIdRef = useRef<string | null>(null);

  const connect = useCallback(async () => {
    if (!token) return;
    reconnectAttempts.current += 1;
    const ws = new WebSocket(await wsURL(), ['openforge.auth', `bearer.${token}`]);
    wsRef.current = ws;
    setStatus('connecting');

    ws.onopen = () => {
      setStatus('open');
      reconnectDelay.current = 1000;
      reconnectAttempts.current = 0;
      // On reconnect, ask the server to replay any events we missed
      // since the last sequence number we successfully consumed.
      if (lastSeqRef.current > 0 && activePipelineIdRef.current) {
        ws.send(JSON.stringify({
          type: 'sync.request',
          payload: {
            pipeline_id: activePipelineIdRef.current,
            last_seq: lastSeqRef.current,
          },
        }));
      }
    };

    ws.onclose = () => {
      setStatus('closed');
      if (reconnectAttempts.current >= 3) {
        console.warn(
          '[WS] WebSocket failed to connect after multiple attempts — possible firewall or proxy blocking the connection.',
        );
      }
      reconnectTimer.current = window.setTimeout(() => {
        reconnectDelay.current = Math.min(reconnectDelay.current * 2, 30000);
        void connect();
      }, reconnectDelay.current);
    };

    ws.onerror = () => setStatus('error');

    ws.onmessage = (event) => {
      try {
        const msg = JSON.parse(event.data);
        // Track the highest sequence number we've seen from a
        // sync.replay so the next reconnect can ask for "events
        // newer than this".
        if (msg.type === 'sync.replay' && msg.payload && typeof msg.payload.seq === 'number') {
          if (msg.payload.seq > lastSeqRef.current) {
            lastSeqRef.current = msg.payload.seq;
          }
        }
        listenersRef.current.get(msg.type)?.forEach(fn => fn(msg.payload));
      } catch {}
    };
  }, [token]);

  useEffect(() => {
    void connect();
    return () => {
      clearTimeout(reconnectTimer.current);
      wsRef.current?.close();
    };
  }, [connect]);

  const send = useCallback((type: string, payload?: any): boolean => {
    if (wsRef.current?.readyState === WebSocket.OPEN) {
      wsRef.current.send(JSON.stringify({ type, payload }));
      // Remember the most recent pipeline_id we sent a chat for; the
      // reconnect sync.request needs to be scoped to a single pipeline.
      if (type === 'chat.send' && payload && typeof payload.pipeline_id === 'string') {
        activePipelineIdRef.current = payload.pipeline_id;
      }
      return true;
    }
    console.warn(`[WS] Cannot send "${type}": WebSocket not connected`);
    return false;
  }, []);

  const subscribe = useCallback((type: string, fn: (payload: any) => void) => {
    if (!listenersRef.current.has(type)) {
      listenersRef.current.set(type, new Set());
    }
    listenersRef.current.get(type)!.add(fn);
    return () => { listenersRef.current.get(type)?.delete(fn); };
  }, []);

  return { status, send, subscribe };
}
