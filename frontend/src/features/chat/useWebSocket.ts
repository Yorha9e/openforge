import { useEffect, useRef, useCallback, useState } from 'react';
import { wsURL } from '../../shared/api';

type WSStatus = 'connecting' | 'open' | 'closed' | 'error';

export function useWebSocket(token: string | null) {
  const wsRef = useRef<WebSocket | null>(null);
  const [status, setStatus] = useState<WSStatus>('closed');
  const listenersRef = useRef<Map<string, Set<(payload: any) => void>>>(new Map());
  const reconnectTimer = useRef<number>(0);
  const reconnectDelay = useRef(1000);

  const connect = useCallback(() => {
    if (!token) return;
    const ws = new WebSocket(wsURL());
    wsRef.current = ws;
    setStatus('connecting');

    ws.onopen = () => {
      setStatus('open');
      reconnectDelay.current = 1000;
      ws.send(JSON.stringify({ type: 'auth', payload: { token } }));
    };

    ws.onclose = () => {
      setStatus('closed');
      reconnectTimer.current = window.setTimeout(() => {
        reconnectDelay.current = Math.min(reconnectDelay.current * 2, 30000);
        connect();
      }, reconnectDelay.current);
    };

    ws.onerror = () => setStatus('error');

    ws.onmessage = (event) => {
      try {
        const msg = JSON.parse(event.data);
        listenersRef.current.get(msg.type)?.forEach(fn => fn(msg.payload));
      } catch {}
    };
  }, [token]);

  useEffect(() => {
    connect();
    return () => {
      clearTimeout(reconnectTimer.current);
      wsRef.current?.close();
    };
  }, [connect]);

  const send = useCallback((type: string, payload?: any) => {
    if (wsRef.current?.readyState === WebSocket.OPEN) {
      wsRef.current.send(JSON.stringify({ type, payload }));
    }
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
