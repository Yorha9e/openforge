import { useState, useEffect } from 'react';
import { useSearchParams, Link } from 'react-router-dom';

export function CircuitBreakerPage() {
  const [searchParams] = useSearchParams();
  const reason = searchParams.get('reason') || 'The upstream service is currently unavailable due to repeated failures.';
  const retryAfterSec = Math.min(Math.max(Number(searchParams.get('retryAfter')) || 30, 10), 300);
  const [remaining, setRemaining] = useState(retryAfterSec);
  const [notifyText, setNotifyText] = useState('Notify Me When Ready');

  useEffect(() => {
    if (remaining <= 0) return;
    const timer = setInterval(() => {
      setRemaining((prev) => Math.max(prev - 1, 0));
    }, 1000);
    return () => clearInterval(timer);
  }, [remaining]);

  const formatTime = (seconds: number): string => {
    const m = Math.floor(seconds / 60);
    const s = seconds % 60;
    return `${m}:${s.toString().padStart(2, '0')}`;
  };

  const handleNotify = () => {
    setNotifyText('Notification Requested');
    setTimeout(() => setNotifyText('Notify Me When Ready'), 3000);
  };

  return (
    <div
      style={{
        minHeight: '100vh',
        background: '#0F172A',
        color: '#F8FAFC',
        fontFamily: "'Fira Sans', sans-serif",
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        padding: 24,
      }}
    >
      <div
        style={{
          background: '#1E293B',
          border: '1px solid #334155',
          borderRadius: 12,
          padding: '48px 40px',
          maxWidth: 480,
          width: '100%',
          textAlign: 'center',
        }}
      >
        {/* Warning icon — SVG exclamation triangle, no emoji */}
        <svg
          width={48}
          height={48}
          viewBox="0 0 24 24"
          fill="none"
          stroke="#F59E0B"
          strokeWidth={2}
          strokeLinecap="round"
          strokeLinejoin="round"
          style={{ marginBottom: 16 }}
        >
          <path d="M10.29 3.86L1.82 18a2 2 0 001.71 3h16.94a2 2 0 001.71-3L13.71 3.86a2 2 0 00-3.42 0z" />
          <line x1={12} y1={9} x2={12} y2={13} />
          <line x1={12} y1={17} x2={12.01} y2={17} />
        </svg>

        <h1
          style={{
            fontFamily: "'Fira Code', monospace",
            fontSize: 20,
            fontWeight: 600,
            margin: '0 0 8px',
            color: '#F8FAFC',
          }}
        >
          Service Temporarily Unavailable
        </h1>

        <p
          style={{
            fontSize: 14,
            color: '#94a3b8',
            lineHeight: 1.6,
            margin: '0 0 20px',
          }}
        >
          {reason}
        </p>

        {/* Auto-recovery countdown */}
        <div
          style={{
            fontFamily: "'Fira Code', monospace",
            fontSize: 36,
            fontWeight: 700,
            color: remaining === 0 ? '#22C55E' : '#F59E0B',
            marginBottom: 4,
          }}
        >
          {formatTime(remaining)}
        </div>
        <p
          style={{
            fontSize: 12,
            color: '#64748b',
            margin: '0 0 24px',
          }}
        >
          {remaining === 0 ? 'Recovery expected — service should be available now' : 'Estimated time until recovery'}
        </p>

        <div style={{ display: 'flex', gap: 12, justifyContent: 'center', flexWrap: 'wrap' }}>
          <button
            onClick={handleNotify}
            disabled={notifyText === 'Notification Requested'}
            style={{
              padding: '10px 24px',
              background: notifyText === 'Notification Requested' ? '#334155' : '#22C55E',
              color: notifyText === 'Notification Requested' ? '#94a3b8' : '#0F172A',
              border: 'none',
              borderRadius: 6,
              cursor: notifyText === 'Notification Requested' ? 'default' : 'pointer',
              fontSize: 14,
              fontWeight: 600,
              fontFamily: "'Fira Sans', sans-serif",
              transition: 'background 200ms',
            }}
            onMouseEnter={(e) => {
              if (notifyText !== 'Notification Requested') e.currentTarget.style.background = '#16A34A';
            }}
            onMouseLeave={(e) => {
              if (notifyText !== 'Notification Requested') e.currentTarget.style.background = '#22C55E';
            }}
          >
            {notifyText}
          </button>

          <Link
            to="/"
            style={{
              padding: '10px 24px',
              background: 'transparent',
              color: '#F8FAFC',
              border: '1px solid #334155',
              borderRadius: 6,
              cursor: 'pointer',
              fontSize: 14,
              fontWeight: 600,
              fontFamily: "'Fira Sans', sans-serif",
              textDecoration: 'none',
              display: 'inline-flex',
              alignItems: 'center',
              transition: 'border-color 200ms, background 200ms',
            }}
            onMouseEnter={(e) => {
              e.currentTarget.style.borderColor = '#22C55E';
              e.currentTarget.style.background = 'rgba(34, 197, 94, 0.08)';
            }}
            onMouseLeave={(e) => {
              e.currentTarget.style.borderColor = '#334155';
              e.currentTarget.style.background = 'transparent';
            }}
          >
            Go to Dashboard
          </Link>
        </div>
      </div>
    </div>
  );
}
