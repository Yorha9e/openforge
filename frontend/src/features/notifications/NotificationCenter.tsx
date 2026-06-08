import { useEffect, useRef, useState } from "react";
import { useNotification, type Notification } from "../../shared/notification";
import { tokens } from "../../shared/design-tokens";

function BellIcon({ size = 18 }: { size?: number }) {
  return (
    <svg
      width={size}
      height={size}
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="1.5"
      strokeLinecap="round"
      strokeLinejoin="round"
      aria-hidden="true"
    >
      <path d="M18 8A6 6 0 006 8c0 7-3 9-3 9h18s-3-2-3-9" />
      <path d="M13.73 21a2 2 0 01-3.46 0" />
    </svg>
  );
}

function formatTs(ts: number): string {
  try {
    return new Date(ts).toLocaleString();
  } catch {
    return "";
  }
}

function NotificationRow({ item }: { item: Notification }) {
  const accent =
    item.level === "error"
      ? tokens.error
      : item.level === "warn"
        ? "#F59E0B"
        : tokens.cta;
  return (
    <div
      data-testid="notification-item"
      data-level={item.level}
      data-read={item.read ? "true" : "false"}
      style={{
        padding: "10px 14px",
        borderBottom: `1px solid ${tokens.border}`,
        background: item.read ? tokens.bg : `${tokens.cta}08`,
      }}
    >
      <div
        style={{
          display: "flex",
          alignItems: "center",
          gap: 8,
          fontSize: 13,
          fontWeight: 600,
          color: tokens.text,
          fontFamily: tokens.fontHeading,
        }}
      >
        <span
          aria-hidden="true"
          style={{
            width: 8,
            height: 8,
            borderRadius: "50%",
            background: accent,
            flexShrink: 0,
          }}
        />
        <span style={{ flex: 1, minWidth: 0 }}>{item.title}</span>
      </div>
      {item.body && (
        <div
          style={{
            fontSize: 12,
            color: tokens.muted,
            marginTop: 4,
            lineHeight: 1.4,
            wordBreak: "break-word",
          }}
        >
          {item.body}
        </div>
      )}
      <div
        style={{
          fontSize: 11,
          color: tokens.muted,
          marginTop: 4,
          fontFamily: tokens.fontBody,
        }}
      >
        {formatTs(item.ts)}
      </div>
    </div>
  );
}

export function NotificationCenter() {
  const { items, markAllRead, unreadCount } = useNotification();
  const [open, setOpen] = useState(false);
  const containerRef = useRef<HTMLDivElement | null>(null);

  // Close on outside click / Escape.
  useEffect(() => {
    if (!open) return;
    function onDocClick(e: MouseEvent) {
      if (!containerRef.current) return;
      if (!containerRef.current.contains(e.target as Node)) {
        setOpen(false);
      }
    }
    function onKey(e: KeyboardEvent) {
      if (e.key === "Escape") setOpen(false);
    }
    document.addEventListener("mousedown", onDocClick);
    document.addEventListener("keydown", onKey);
    return () => {
      document.removeEventListener("mousedown", onDocClick);
      document.removeEventListener("keydown", onKey);
    };
  }, [open]);

  const handleToggle = () => {
    setOpen((prev) => {
      const next = !prev;
      if (next) markAllRead();
      return next;
    });
  };

  return (
    <div
      ref={containerRef}
      data-testid="notification-center"
      style={{ position: "relative", display: "inline-block" }}
    >
      <button
        type="button"
        aria-label={`Notifications (${unreadCount} unread)`}
        aria-haspopup="dialog"
        aria-expanded={open}
        onClick={handleToggle}
        data-testid="notification-bell"
        style={{
          position: "relative",
          padding: 6,
          background: "transparent",
          border: `1px solid ${tokens.border}`,
          borderRadius: 6,
          color: tokens.muted,
          cursor: "pointer",
          display: "inline-flex",
          alignItems: "center",
          justifyContent: "center",
          transition: tokens.transition,
          minHeight: 36,
          minWidth: 36,
        }}
        onMouseEnter={(e) => {
          e.currentTarget.style.color = tokens.text;
          e.currentTarget.style.borderColor = tokens.text;
        }}
        onMouseLeave={(e) => {
          e.currentTarget.style.color = tokens.muted;
          e.currentTarget.style.borderColor = tokens.border;
        }}
      >
        <BellIcon />
        {unreadCount > 0 && (
          <span
            data-testid="unread-badge"
            style={{
              position: "absolute",
              top: -4,
              right: -4,
              minWidth: 18,
              height: 18,
              padding: "0 4px",
              borderRadius: 9,
              background: tokens.error,
              color: "#fff",
              fontSize: 10,
              fontWeight: 700,
              display: "flex",
              alignItems: "center",
              justifyContent: "center",
              lineHeight: 1,
            }}
          >
            {unreadCount > 99 ? "99+" : unreadCount}
          </span>
        )}
      </button>

      {open && (
        <div
          role="dialog"
          aria-label="Notifications"
          data-testid="notification-panel"
          style={{
            position: "absolute",
            right: 0,
            top: "calc(100% + 6px)",
            width: 320,
            maxHeight: 384,
            overflow: "auto",
            background: tokens.bg,
            border: `1px solid ${tokens.border}`,
            borderRadius: 8,
            boxShadow: "0 8px 24px rgba(0,0,0,0.4)",
            zIndex: 50,
          }}
        >
          <div
            style={{
              padding: "10px 14px",
              borderBottom: `1px solid ${tokens.border}`,
              display: "flex",
              alignItems: "center",
              justifyContent: "space-between",
            }}
          >
            <span
              style={{
                fontSize: 13,
                fontWeight: 600,
                color: tokens.text,
                fontFamily: tokens.fontHeading,
              }}
            >
              Notifications
            </span>
            <span style={{ fontSize: 11, color: tokens.muted }}>
              {items.length} total
            </span>
          </div>
          {items.length === 0 ? (
            <div
              data-testid="notification-empty"
              style={{
                padding: 24,
                fontSize: 13,
                color: tokens.muted,
                textAlign: "center",
              }}
            >
              No notifications
            </div>
          ) : (
            items.map((n) => <NotificationRow key={n.id} item={n} />)
          )}
        </div>
      )}
    </div>
  );
}
