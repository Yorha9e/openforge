import { useCallback, useEffect, useRef, useState, type FormEvent } from 'react';
import { useNavigate, useSearchParams } from 'react-router-dom';
import { api } from '../../shared/api';
import { useAuth } from '../../shared/auth';
import { tokens } from '../../shared/design-tokens';
import { useToast } from '../../shared/toast';

interface InvitationInfo {
  valid: boolean;
  role: string;
  project_id: string;
  project_name: string;
  expires_at: string;
  error?: string;
}

export function InvitePage() {
  const { registerWithInvitation, token, user } = useAuth();
  const navigate = useNavigate();
  const [searchParams] = useSearchParams();
  const { toast } = useToast();
  const [invitation, setInvitation] = useState<InvitationInfo | null>(null);
  const [loading, setLoading] = useState(true);
  const [joining, setJoining] = useState(false);
  const [error, setError] = useState('');
  const [username, setUsername] = useState('');
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [displayName, setDisplayName] = useState('');
  const [submitting, setSubmitting] = useState(false);
  const [usernameFocused, setUsernameFocused] = useState(false);
  const [emailFocused, setEmailFocused] = useState(false);
  const [passwordFocused, setPasswordFocused] = useState(false);
  const [displayNameFocused, setDisplayNameFocused] = useState(false);
  const [btnHovered, setBtnHovered] = useState(false);
  const autoJoinAttempted = useRef(false);

  const inviteToken = searchParams.get('token');
  const isLoggedIn = !!token;

  // Join handler for logged-in users (used by both auto-join and manual button)
  const handleJoinProject = useCallback(async () => {
    if (!inviteToken) return;
    setError('');
    setJoining(true);
    try {
      const result = await api.joinProjectWithInvitation(inviteToken);
      if (result.success) {
        toast(`Successfully joined project: ${result.project_name}`, 'success');
        navigate(`/project/${result.project_id}`, { replace: true });
      }
    } catch (err) {
      const msg = err instanceof Error ? err.message : 'Failed to join project';
      if (msg.includes('already have access')) {
        toast('You already have access to this project', 'success');
        // Navigate to the project if we know the ID, otherwise to dashboard
        if (invitation?.project_id) {
          navigate(`/project/${invitation.project_id}`, { replace: true });
        } else {
          navigate('/', { replace: true });
        }
        return;
      }
      setError(msg);
      toast(msg);
    } finally {
      setJoining(false);
    }
  }, [inviteToken, invitation?.project_id, navigate, toast]);

  // Auto-join for logged-in users — don't wait for verify; join API does its own validation
  useEffect(() => {
    if (!inviteToken || !isLoggedIn || autoJoinAttempted.current) {
      return;
    }
    autoJoinAttempted.current = true;
    handleJoinProject();
  }, [inviteToken, isLoggedIn, handleJoinProject]);

  // Verify invitation (for display info; non-logged-in users need this for the registration form)
  useEffect(() => {
    if (!inviteToken) {
      setError('No invitation token provided');
      setLoading(false);
      return;
    }

    const verifyInvitation = async () => {
      try {
        const result = await api.verifyInvitation(inviteToken);
        if (!result.valid) {
          setError(result.error || 'This invitation is invalid or has expired.');
        }
        setInvitation(result);
      } catch (err) {
        // Logged-in users can still attempt to join; only surface error for non-logged-in users
        if (!isLoggedIn) {
          setError(err instanceof Error ? err.message : 'Failed to verify invitation');
        }
      } finally {
        setLoading(false);
      }
    };

    verifyInvitation();
  }, [inviteToken, isLoggedIn]);

  if (loading || joining) {
    return (
      <div
        style={{
          minHeight: '100vh',
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'center',
          background: tokens.bg,
          fontFamily: tokens.fontBody,
        }}
      >
        <div style={{ color: tokens.muted }}>
          {joining ? 'Joining project...' : 'Verifying invitation...'}
        </div>
      </div>
    );
  }

  // If no token provided, show error
  if (!inviteToken) {
    return (
      <div style={{ minHeight: '100vh', display: 'flex', alignItems: 'center', justifyContent: 'center', background: tokens.bg, fontFamily: tokens.fontBody }}>
        <div style={{ background: tokens.surface, padding: 32, borderRadius: 8, width: '100%', maxWidth: 400, textAlign: 'center' }}>
          <h1 style={{ fontSize: 24, fontWeight: 700, color: tokens.error, marginBottom: 16, fontFamily: tokens.fontHeading }}>Invalid Invitation</h1>
          <p style={{ color: tokens.muted, marginBottom: 24 }}>No invitation token provided.</p>
          <button onClick={() => navigate('/login')} style={{ padding: '10px 20px', background: tokens.cta, color: tokens.ctaText, border: 'none', borderRadius: 4, fontWeight: 500, cursor: 'pointer', fontSize: 14 }}>Go to Login</button>
        </div>
      </div>
    );
  }

  // For logged-in users: show join page even if verify failed (join API does its own validation)
  // For non-logged-in users: show error only if verify failed AND invitation is not valid
  if (!isLoggedIn && (error || !invitation?.valid)) {
    return (
      <div style={{ minHeight: '100vh', display: 'flex', alignItems: 'center', justifyContent: 'center', background: tokens.bg, fontFamily: tokens.fontBody }}>
        <div style={{ background: tokens.surface, padding: 32, borderRadius: 8, width: '100%', maxWidth: 400, textAlign: 'center' }}>
          <h1 style={{ fontSize: 24, fontWeight: 700, color: tokens.error, marginBottom: 16, fontFamily: tokens.fontHeading }}>Invalid Invitation</h1>
          <p style={{ color: tokens.muted, marginBottom: 24 }}>
            {error || invitation?.error || 'This invitation is invalid or has expired.'}
          </p>
          <button onClick={() => navigate('/login')} style={{ padding: '10px 20px', background: tokens.cta, color: tokens.ctaText, border: 'none', borderRadius: 4, fontWeight: 500, cursor: 'pointer', fontSize: 14 }}>Go to Login</button>
        </div>
      </div>
    );
  }

  const handleSubmit = async (e: FormEvent) => {
    e.preventDefault();
    setError('');
    setSubmitting(true);

    if (!username || !email || !password) {
      setError('Username, email, and password are required');
      setSubmitting(false);
      return;
    }

    try {
      const projectId = await registerWithInvitation(inviteToken!, username, password, displayName || username, email);
      if (projectId) {
        navigate(`/project/${projectId}`, { replace: true });
      } else {
        navigate('/', { replace: true });
      }
    } catch (err) {
      const msg = err instanceof Error ? err.message : 'Registration failed';
      setError(msg);
      toast(msg);
    } finally {
      setSubmitting(false);
    }
  };

  const inputStyle = (focused: boolean): React.CSSProperties => ({
    width: '100%',
    padding: '10px 12px',
    background: tokens.bg,
    border: `1px solid ${tokens.border}`,
    borderRadius: 4,
    color: tokens.text,
    boxSizing: 'border-box',
    outlineWidth: focused ? 2 : 0,
    outlineStyle: focused ? 'solid' : 'none',
    outlineColor: tokens.cta,
    outlineOffset: 2,
    transition: tokens.transition,
    fontSize: 14,
  });

  return (
    <div
      style={{
        minHeight: '100vh',
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        background: tokens.bg,
        fontFamily: tokens.fontBody,
      }}
    >
      <div
        style={{
          background: tokens.surface,
          padding: 32,
          borderRadius: 8,
          width: '100%',
          maxWidth: 400,
        }}
      >
        <h1
          style={{
            fontSize: 24,
            fontWeight: 700,
            color: tokens.text,
            textAlign: 'center',
            marginBottom: 16,
            fontFamily: tokens.fontHeading,
          }}
        >
          {isLoggedIn ? 'Join Project' : 'Create Account & Join'}
        </h1>
        {invitation && (
          <div
            style={{
              background: tokens.bg,
              padding: 16,
              borderRadius: 4,
              marginBottom: 24,
              border: `1px solid ${tokens.border}`,
            }}
          >
            <div style={{ marginBottom: 8 }}>
              <span style={{ color: tokens.muted, fontSize: 13 }}>Role: </span>
              <span style={{ color: tokens.text, fontWeight: 500 }}>{invitation.role}</span>
            </div>
            {invitation.project_name && (
              <div style={{ marginBottom: 8 }}>
                <span style={{ color: tokens.muted, fontSize: 13 }}>Project: </span>
                <span style={{ color: tokens.text, fontWeight: 500 }}>{invitation.project_name}</span>
              </div>
            )}
            <div>
              <span style={{ color: tokens.muted, fontSize: 13 }}>Expires: </span>
              <span style={{ color: tokens.text, fontWeight: 500 }}>
                {new Date(invitation.expires_at).toLocaleDateString()}
              </span>
            </div>
          </div>
        )}
        {error && (
          <p
            role="alert"
            style={{ color: tokens.error, fontSize: 14, marginBottom: 12 }}
          >
            {error}
          </p>
        )}

        {/* For logged-in users: show join button as fallback */}
        {isLoggedIn ? (
          <>
            <p style={{ color: tokens.muted, fontSize: 14, marginBottom: 16, textAlign: 'center' }}>
              You are logged in as <strong style={{ color: tokens.text }}>{user?.id}</strong>.
            </p>
            <button
              onClick={handleJoinProject}
              disabled={joining}
              onMouseEnter={() => setBtnHovered(true)}
              onMouseLeave={() => setBtnHovered(false)}
              style={{
                width: '100%',
                padding: '10px 0',
                background: btnHovered && !joining ? tokens.ctaHover : tokens.cta,
                color: tokens.ctaText,
                border: 'none',
                borderRadius: 4,
                fontWeight: 500,
                cursor: joining ? 'default' : 'pointer',
                opacity: joining ? 0.5 : 1,
                transition: tokens.transition,
                fontSize: 14,
              }}
            >
              {joining ? 'Joining...' : 'Join Project'}
            </button>
          </>
        ) : (
          // Not logged in: show registration form
          <form onSubmit={handleSubmit}>
            <div style={{ marginBottom: 12 }}>
              <label
                htmlFor="invite-username"
                style={{
                  display: 'block',
                  fontSize: 13,
                  color: tokens.muted,
                  marginBottom: 4,
                  fontWeight: 500,
                }}
              >
                Username
              </label>
              <input
                id="invite-username"
                type="text"
                value={username}
                onChange={e => setUsername(e.target.value)}
                onFocus={() => setUsernameFocused(true)}
                onBlur={() => setUsernameFocused(false)}
                style={inputStyle(usernameFocused)}
                autoFocus
              />
            </div>
            <div style={{ marginBottom: 12 }}>
              <label
                htmlFor="invite-email"
                style={{
                  display: 'block',
                  fontSize: 13,
                  color: tokens.muted,
                  marginBottom: 4,
                  fontWeight: 500,
                }}
              >
                Email
              </label>
              <input
                id="invite-email"
                type="email"
                value={email}
                onChange={e => setEmail(e.target.value)}
                onFocus={() => setEmailFocused(true)}
                onBlur={() => setEmailFocused(false)}
                style={inputStyle(emailFocused)}
              />
            </div>
            <div style={{ marginBottom: 12 }}>
              <label
                htmlFor="invite-displayname"
                style={{
                  display: 'block',
                  fontSize: 13,
                  color: tokens.muted,
                  marginBottom: 4,
                  fontWeight: 500,
                }}
              >
                Display Name
              </label>
              <input
                id="invite-displayname"
                type="text"
                value={displayName}
                onChange={e => setDisplayName(e.target.value)}
                onFocus={() => setDisplayNameFocused(true)}
                onBlur={() => setDisplayNameFocused(false)}
                style={inputStyle(displayNameFocused)}
              />
            </div>
            <div style={{ marginBottom: 16 }}>
              <label
                htmlFor="invite-password"
                style={{
                  display: 'block',
                  fontSize: 13,
                  color: tokens.muted,
                  marginBottom: 4,
                  fontWeight: 500,
                }}
              >
                Password
              </label>
              <input
                id="invite-password"
                type="password"
                value={password}
                onChange={e => setPassword(e.target.value)}
                onFocus={() => setPasswordFocused(true)}
                onBlur={() => setPasswordFocused(false)}
                style={inputStyle(passwordFocused)}
              />
            </div>
            <button
              type="submit"
              disabled={submitting}
              aria-label="Create Account"
              onMouseEnter={() => setBtnHovered(true)}
              onMouseLeave={() => setBtnHovered(false)}
              style={{
                width: '100%',
                padding: '10px 0',
                background: btnHovered && !submitting ? tokens.ctaHover : tokens.cta,
                color: tokens.ctaText,
                border: 'none',
                borderRadius: 4,
                fontWeight: 500,
                cursor: submitting ? 'default' : 'pointer',
                opacity: submitting ? 0.5 : 1,
                transition: tokens.transition,
                fontSize: 14,
              }}
            >
              {submitting ? 'Creating account...' : 'Create Account & Join'}
            </button>
            <div
              style={{
                textAlign: 'center',
                marginTop: 16,
                fontSize: 13,
                color: tokens.muted,
              }}
            >
              <span
                onClick={() => navigate('/login')}
                style={{
                  cursor: 'pointer',
                  color: tokens.cta,
                  textDecoration: 'underline',
                }}
              >
                Already have an account? Sign In
              </span>
            </div>
          </form>
        )}
      </div>
    </div>
  );
}