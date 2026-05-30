import { useState, type FormEvent } from 'react';
import { useNavigate, useSearchParams, Navigate } from 'react-router-dom';
import { useAuth } from '../../shared/auth';
import { useToast } from '../../shared/toast';
import { tokens } from '../../shared/design-tokens';
import { RoleSelector } from '../../shared/RoleSelector';

export function RegisterPage() {
  const { register, registerWithInvitation, token } = useAuth();
  const navigate = useNavigate();
  const [searchParams] = useSearchParams();
  const { toast } = useToast();
  const [username, setUsername] = useState('');
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [displayName, setDisplayName] = useState('');
  const [role, setRole] = useState('');
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(false);
  const [usernameFocused, setUsernameFocused] = useState(false);
  const [emailFocused, setEmailFocused] = useState(false);
  const [passwordFocused, setPasswordFocused] = useState(false);
  const [displayNameFocused, setDisplayNameFocused] = useState(false);
  const [btnHovered, setBtnHovered] = useState(false);

  const inviteToken = searchParams.get('token');
  const isInviteRegistration = !!inviteToken;

  if (token) return <Navigate to="/" replace />;

  const handleSubmit = async (e: FormEvent) => {
    e.preventDefault();
    setError('');
    setLoading(true);

    // Validation
    if (!username) {
      setError('Username is required');
      setLoading(false);
      return;
    }
    if (!email) {
      setError('Email is required');
      setLoading(false);
      return;
    }
    if (!password) {
      setError('Password is required');
      setLoading(false);
      return;
    }
    // Role is not required for invite registration (role comes from invitation)
    if (!isInviteRegistration && !role) {
      setError('Role is required');
      setLoading(false);
      return;
    }

    try {
      if (isInviteRegistration && inviteToken) {
        // 邀请注册：使用邀请token注册并加入项目
        const projectId = await registerWithInvitation(inviteToken, username, password, displayName || username, email);
        if (projectId) {
          navigate(`/project/${projectId}`, { replace: true });
        } else {
          navigate('/', { replace: true });
        }
      } else {
        // 普通注册
        await register(username, password, displayName || username, role, email);
        navigate('/', { replace: true });
      }
    } catch (err) {
      const msg = err instanceof Error ? err.message : 'Registration failed';
      setError(msg);
      toast(msg);
    } finally {
      setLoading(false);
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
      <form
        onSubmit={handleSubmit}
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
            marginBottom: 24,
            fontFamily: tokens.fontHeading,
          }}
        >
          {isInviteRegistration ? 'Create Account & Join Project' : 'Create Account'}
        </h1>
        {error && (
          <p
            role="alert"
            style={{ color: tokens.error, fontSize: 14, marginBottom: 12 }}
          >
            {error}
          </p>
        )}
        <div style={{ marginBottom: 12 }}>
          <label
            htmlFor="register-username"
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
            id="register-username"
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
            htmlFor="register-email"
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
            id="register-email"
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
            htmlFor="register-displayname"
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
            id="register-displayname"
            type="text"
            value={displayName}
            onChange={e => setDisplayName(e.target.value)}
            onFocus={() => setDisplayNameFocused(true)}
            onBlur={() => setDisplayNameFocused(false)}
            style={inputStyle(displayNameFocused)}
          />
        </div>
        <div style={{ marginBottom: 12 }}>
          <label
            htmlFor="register-password"
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
            id="register-password"
            type="password"
            value={password}
            onChange={e => setPassword(e.target.value)}
            onFocus={() => setPasswordFocused(true)}
            onBlur={() => setPasswordFocused(false)}
            style={inputStyle(passwordFocused)}
          />
        </div>
        {/* 角色选择器：仅普通注册显示 */}
        {!isInviteRegistration && (
          <div style={{ marginBottom: 16 }}>
            <RoleSelector
              value={role}
              onChange={setRole}
              disabled={loading}
            />
          </div>
        )}
        <button
          type="submit"
          disabled={loading}
          aria-label="Sign Up"
          onMouseEnter={() => setBtnHovered(true)}
          onMouseLeave={() => setBtnHovered(false)}
          style={{
            width: '100%',
            padding: '10px 0',
            background: btnHovered && !loading ? tokens.ctaHover : tokens.cta,
            color: tokens.ctaText,
            border: 'none',
            borderRadius: 4,
            fontWeight: 500,
            cursor: loading ? 'default' : 'pointer',
            opacity: loading ? 0.5 : 1,
            transition: tokens.transition,
            fontSize: 14,
          }}
        >
          {loading ? 'Creating account...' : (isInviteRegistration ? 'Create Account & Join' : 'Create Account')}
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
    </div>
  );
}
