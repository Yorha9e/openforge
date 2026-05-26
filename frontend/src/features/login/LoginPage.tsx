import { useState, type FormEvent } from 'react';
import { useNavigate, Navigate } from 'react-router-dom';
import { useAuth } from '../../shared/auth';
import { useToast } from '../../shared/toast';
import { tokens } from '../../shared/design-tokens';

export function LoginPage() {
  const { login, token } = useAuth();
  const navigate = useNavigate();
  const { toast } = useToast();
  const [username, setUsername] = useState('');
  const [password, setPassword] = useState('');
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(false);
  const [usernameFocused, setUsernameFocused] = useState(false);
  const [passwordFocused, setPasswordFocused] = useState(false);
  const [btnHovered, setBtnHovered] = useState(false);

  if (token) return <Navigate to="/" replace />;

  const handleSubmit = async (e: FormEvent) => {
    e.preventDefault();
    setError(''); setLoading(true);
    try {
      await login(username, password);
      navigate('/', { replace: true });
    } catch (err) {
      const msg = err instanceof Error ? err.message : 'Login failed';
      setError(msg);
      toast(msg);
    } finally {
      setLoading(false);
    }
  };

  return (
    <div style={{ minHeight: '100vh', display: 'flex', alignItems: 'center', justifyContent: 'center', background: tokens.bg, fontFamily: tokens.fontBody }}>
      <form onSubmit={handleSubmit} style={{ background: tokens.surface, padding: 32, borderRadius: 8, width: '100%', maxWidth: 360 }}>
        <h1 style={{ fontSize: 24, fontWeight: 700, color: tokens.text, textAlign: 'center', marginBottom: 24, fontFamily: tokens.fontHeading }}>OpenForge</h1>
        {error && <p role="alert" style={{ color: tokens.error, fontSize: 14, marginBottom: 12 }}>{error}</p>}
        <div style={{ marginBottom: 12 }}>
          <label htmlFor="login-username" style={{ display: 'block', fontSize: 13, color: tokens.muted, marginBottom: 4, fontWeight: 500 }}>Username</label>
          <input
            id="login-username"
            type="text"
            value={username}
            onChange={e => setUsername(e.target.value)}
            aria-required="true"
            onFocus={() => { setUsernameFocused(true); }}
            onBlur={() => { setUsernameFocused(false); }}
            style={{
              width: '100%', padding: '10px 12px', background: tokens.bg, border: `1px solid ${tokens.border}`, borderRadius: 4, color: tokens.text, boxSizing: 'border-box',
              outline: usernameFocused ? '2px solid' : 'none', outlineColor: tokens.cta, outlineOffset: 2,
              transition: tokens.transition, fontSize: 14,
            }}
            autoFocus />
        </div>
        <div style={{ marginBottom: 16 }}>
          <label htmlFor="login-password" style={{ display: 'block', fontSize: 13, color: tokens.muted, marginBottom: 4, fontWeight: 500 }}>Password</label>
          <input
            id="login-password"
            type="password"
            value={password}
            onChange={e => setPassword(e.target.value)}
            aria-required="true"
            onFocus={() => { setPasswordFocused(true); }}
            onBlur={() => { setPasswordFocused(false); }}
            style={{
              width: '100%', padding: '10px 12px', background: tokens.bg, border: `1px solid ${tokens.border}`, borderRadius: 4, color: tokens.text, boxSizing: 'border-box',
              outline: passwordFocused ? '2px solid' : 'none', outlineColor: tokens.cta, outlineOffset: 2,
              transition: tokens.transition, fontSize: 14,
            }} />
        </div>
        <button
          type="submit"
          disabled={loading}
          aria-label="Sign In"
          onMouseEnter={() => setBtnHovered(true)}
          onMouseLeave={() => setBtnHovered(false)}
          style={{
            width: '100%', padding: '10px 0', background: btnHovered && !loading ? tokens.ctaHover : tokens.cta, color: tokens.ctaText, border: 'none', borderRadius: 4, fontWeight: 500,
            cursor: loading ? 'default' : 'pointer', opacity: loading ? 0.5 : 1,
            transition: tokens.transition,
          }}>
          {loading ? 'Signing in...' : 'Sign In'}
        </button>
      </form>
    </div>
  );
}
