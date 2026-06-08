import { createContext, useContext, useState, useCallback, type ReactNode } from "react";

export type NotificationLevel = "info" | "warn" | "error";

export interface Notification {
  id: string;
  level: NotificationLevel;
  title: string;
  body?: string;
  ts: number;
  read: boolean;
}

export type NotificationInput = Omit<Notification, "id" | "ts" | "read">;

export interface NotificationContextValue {
  items: Notification[];
  push: (n: NotificationInput) => void;
  markAllRead: () => void;
  clear: () => void;
  unreadCount: number;
}

const NotificationCtx = createContext<NotificationContextValue | null>(null);

const MAX_ITEMS = 100;

function generateId(): string {
  if (typeof crypto !== "undefined" && typeof crypto.randomUUID === "function") {
    return crypto.randomUUID();
  }
  // Fallback for environments without crypto.randomUUID (e.g. older test envs).
  return `n-${Date.now().toString(36)}-${Math.random().toString(36).slice(2, 10)}`;
}

/**
 * In-memory notification store. Notifications live only in this tab;
 * a future change can wire `push` to a websocket backend.
 */
export function NotificationProvider({ children }: { children: ReactNode }) {
  const [items, setItems] = useState<Notification[]>([]);

  const push = useCallback((n: NotificationInput) => {
    setItems((prev) => [
      { ...n, id: generateId(), ts: Date.now(), read: false },
      ...prev,
    ].slice(0, MAX_ITEMS));
  }, []);

  const markAllRead = useCallback(() => {
    setItems((prev) => {
      // Avoid triggering a re-render when nothing changes.
      if (prev.every((i) => i.read)) return prev;
      return prev.map((i) => (i.read ? i : { ...i, read: true }));
    });
  }, []);

  const clear = useCallback(() => {
    setItems([]);
  }, []);

  const unreadCount = items.reduce((acc, i) => acc + (i.read ? 0 : 1), 0);

  const value: NotificationContextValue = {
    items,
    push,
    markAllRead,
    clear,
    unreadCount,
  };

  return <NotificationCtx.Provider value={value}>{children}</NotificationCtx.Provider>;
}

export function useNotification(): NotificationContextValue {
  const ctx = useContext(NotificationCtx);
  if (!ctx) {
    throw new Error("useNotification must be used inside <NotificationProvider>");
  }
  return ctx;
}
