import { createContext, useContext, useState, useCallback, type ReactNode } from 'react';
import { tokens } from './design-tokens';

type ToastType = 'error' | 'success' | 'info';

interface ToastItem {
  id: number;
  message: string;
  type: ToastType;
}

interface ToastCtx {
  toast: (message: string, type?: ToastType) => void;
}

const ToastContext = createContext<ToastCtx>({ toast: () => {} });

let nextId = 0;

export function ToastProvider({ children }: { children: ReactNode }) {
  const [items, setItems] = useState<ToastItem[]>([]);

  const toast = useCallback((message: string, type: ToastType = 'error') => {
    const id = nextId++;
    setItems(prev => [...prev, { id, message, type }]);
    setTimeout(() => setItems(prev => prev.filter(i => i.id !== id)), 4000);
  }, []);

  const bgMap: Record<ToastType, string> = { error: tokens.error, success: tokens.cta, info: tokens.surface };
  const textMap: Record<ToastType, string> = { error: '#fff', success: tokens.ctaText, info: tokens.text };

  return (
    <ToastContext.Provider value={{ toast }}>
      {children}
      <div
        aria-live="polite"
        style={{ position: 'fixed', top: 16, right: 16, zIndex: 9999, display: 'flex', flexDirection: 'column', gap: 8, pointerEvents: 'none' }}
      >
        {items.map(item => (
          <div key={item.id} style={{
            background: bgMap[item.type], color: textMap[item.type],
            padding: '10px 16px', borderRadius: 6, fontSize: 14, fontWeight: 500,
            fontFamily: tokens.fontBody, maxWidth: 360,
            boxShadow: '0 4px 12px rgba(0,0,0,0.4)',
            animation: 'of-slide-in 200ms ease-out',
            pointerEvents: 'auto',
          }}>
            {item.message}
          </div>
        ))}
      </div>
    </ToastContext.Provider>
  );
}

export function useToast() { return useContext(ToastContext); }
