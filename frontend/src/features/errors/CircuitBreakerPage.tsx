import { useState, useEffect } from 'react';
import { useSearchParams, Link, useNavigate } from 'react-router-dom';
import { tokens } from '../../shared/design-tokens';

export default function CircuitBreakerPage() {
  const [searchParams] = useSearchParams();
  const navigate = useNavigate();
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
        background: tokens.bg,
        color: tokens.text,
        fontFamily: tokens.fontBody,
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        padding: 24,
      }}
    >
      <div
        style={{
          background: tokens.surface,
          border: `1px solid ${tokens.border}`,
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
          stroke={tokens.warning}
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
            fontFamily: tokens.fontHeading,
            fontSize: 20,
            fontWeight: 600,
            margin: '0 0 8px',
            color: tokens.text,
          }}
        >
          Service Temporarily Unavailable
        </h1>

        <p
          style={{
            fontSize: 14,
            color: tokens.muted,
            lineHeight: 1.6,
            margin: '0 0 20px',
          }}
        >
          {reason}
        </p>

        {/* Auto-recovery countdown */}
        <div
          style={{
            fontFamily: tokens.fontHeading,
            fontSize: 36,
            fontWeight: 700,
            color: remaining === 0 ? tokens.cta : tokens.warning,
            marginBottom: 4,
          }}
        >
          {formatTime(remaining)}
        </div>
        <p
          style={{
            fontSize: 12,
            color: tokens.muted,
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
              background: notifyText === 'Notification Requested' ? tokens.surface : tokens.cta,
              color: notifyText === 'Notification Requested' ? tokens.muted : tokens.ctaText,
              border: 'none',
              borderRadius: 6,
              cursor: notifyText === 'Notification Requested' ? 'default' : 'pointer',
              fontSize: 14,
              fontWeight: 600,
              fontFamily: tokens.fontBody,
              transition: tokens.transition,
            }}
            onMouseEnter={(e) => {
              if (notifyText !== 'Notification Requested') e.currentTarget.style.background = tokens.ctaHover;
            }}
            onMouseLeave={(e) => {
              if (notifyText !== 'Notification Requested') e.currentTarget.style.background = tokens.cta;
            }}
          >
            {notifyText}
          </button>

          <Link
            to="/"
            style={{
              padding: '10px 24px',
              background: 'transparent',
              color: tokens.text,
              border: `1px solid ${tokens.border}`,
              borderRadius: 6,
              cursor: 'pointer',
              fontSize: 14,
              fontWeight: 600,
              fontFamily: tokens.fontBody,
              textDecoration: 'none',
              display: 'inline-flex',
              alignItems: 'center',
              transition: tokens.transition,
            }}
            onMouseEnter={(e) => {
              e.currentTarget.style.borderColor = tokens.cta;
              e.currentTarget.style.background = `${tokens.cta}14`;
            }}
            onMouseLeave={(e) => {
              e.currentTarget.style.borderColor = tokens.border;
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
