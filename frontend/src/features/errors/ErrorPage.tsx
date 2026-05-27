import { useSearchParams, useNavigate } from 'react-router-dom';
import { tokens } from '../../shared/design-tokens';

type ErrorCode = 404 | 500 | 503;

interface ErrorConfig {
  code: ErrorCode;
  title: string;
  message: string;
  action: { label: string; to: string };
  details?: string;
}

function getErrorConfig(code: ErrorCode): ErrorConfig {
  switch (code) {
    case 404:
      return {
        code: 404,
        title: 'Page Not Found',
        message: 'The page you are looking for does not exist or has been moved.',
        action: { label: 'Go to Dashboard', to: '/' },
      };
    case 500:
      return {
        code: 500,
        title: 'System Error',
        message: 'Something went wrong. Our team has been notified.',
        details: `Error ID: ${crypto.randomUUID().slice(0, 8).toUpperCase()}`,
        action: { label: 'Try Again', to: -1 as any },
      };
    case 503:
      return {
        code: 503,
        title: 'Token Quota Exhausted',
        message: 'Your token quota for this billing period has been reached.',
        details: 'Usage: 100% (500,000 / 500,000 tokens)',
        action: { label: 'View Usage', to: '/settings' },
      };
  }
}

export default function ErrorPage() {
  const [searchParams] = useSearchParams();
  const navigate = useNavigate();
  const codeParam = searchParams.get('code');
  const code: ErrorCode = codeParam === '404' ? 404 : codeParam === '500' ? 500 : codeParam === '503' ? 503 : 404;
  const config = getErrorConfig(code);

  const handleAction = () => {
    if (code === 500) {
      navigate(-1);
    } else {
      navigate(config.action.to);
    }
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
        <div
          style={{
            fontFamily: tokens.fontHeading,
            fontSize: 72,
            fontWeight: 700,
            lineHeight: 1,
            color: tokens.error,
            marginBottom: 8,
          }}
        >
          {config.code}
        </div>

        <h1
          style={{
            fontFamily: tokens.fontHeading,
            fontSize: 20,
            fontWeight: 600,
            margin: '0 0 8px',
            color: tokens.text,
          }}
        >
          {config.title}
        </h1>

        <p
          style={{
            fontSize: 14,
            color: tokens.muted,
            lineHeight: 1.6,
            margin: '0 0 16px',
          }}
        >
          {config.message}
        </p>

        {config.details && (
          <p
            style={{
              fontSize: 12,
              color: '#64748b',
              fontFamily: tokens.fontHeading,
              margin: '0 0 24px',
              padding: '8px 12px',
              background: tokens.bg,
              borderRadius: 6,
              display: 'inline-block',
            }}
          >
            {config.details}
          </p>
        )}

        <div style={{ display: 'flex', gap: 12, justifyContent: 'center' }}>
          <button
            onClick={handleAction}
            style={{
              padding: '10px 24px',
              background: tokens.cta,
              color: tokens.ctaText,
              border: 'none',
              borderRadius: 6,
              cursor: 'pointer',
              fontSize: 14,
              fontWeight: 600,
              fontFamily: tokens.fontBody,
              transition: tokens.transition,
            }}
            onMouseEnter={(e) => (e.currentTarget.style.background = tokens.ctaHover)}
            onMouseLeave={(e) => (e.currentTarget.style.background = tokens.cta)}
          >
            {config.action.label}
          </button>
        </div>
      </div>
    </div>
  );
}
