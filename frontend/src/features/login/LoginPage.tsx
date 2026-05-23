import { useState, type FormEvent } from 'react';
import { useNavigate } from 'react-router-dom';
import { useAuth } from '../../shared/auth';
import { tokens } from '../../shared/design-tokens';

export function LoginPage() {
  const { login, token } = useAuth();
  const navigate = useNavigate();
  const [username, setUsername] = useState('');
  const [password, setPassword] = useState('');
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(false);
  const [usernameFocused, setUsernameFocused] = useState(false);
  const [passwordFocused, setPasswordFocused] = useState(false);
  const [btnHovered, setBtnHovered] = useState(false);

  if (token) { navigate('/', { replace: true }); return null; }

  const handleSubmit = async (e: FormEvent) => {
    e.preventDefault();
    setError(''); setLoading(true);
    try {
      await login(username, password);
      navigate('/', { replace: true });
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Login failed');
    } finally {
      setLoading(false);
    }
  };

  return (
    <div style={{ minHeight: '100vh', display: 'flex', alignItems: 'center', justifyContent: 'center', background: tokens.bg, fontFamily: tokens.fontBody }}>
      <form onSubmit={handleSubmit} style={{ background: tokens.surface, padding: 32, borderRadius: 8, width: '100%', maxWidth: 360 }}>
        <h1 style={{ fontSize: 24, fontWeight: 700, color: tokens.text, textAlign: 'center', marginBottom: 24, fontFamily: tokens.fontHeading }}>OpenForge</h1>
        {error && <p style={{ color: tokens.error, fontSize: 14, marginBottom: 12 }}>{error}</p>}
        <input
          type="text"
          placeholder="Username"
          value={username}
          onChange={e => setUsername(e.target.value)}
          aria-label="Username"
          onFocus={() => { setUsernameFocused(true); }}
          onBlur={() => { setUsernameFocused(false); }}
          style={{
            width: '100%', padding: '8px 12px', background: tokens.bg, border: `1px solid ${tokens.border}`, borderRadius: 4, color: tokens.text, marginBottom: 12, boxSizing: 'border-box',
            outline: usernameFocused ? '2px solid' : 'none', outlineColor: tokens.cta, outlineOffset: 2,
            transition: tokens.transition,
          }}
          autoFocus />
        <input
          type="password"
          placeholder="Password"
          value={password}
          onChange={e => setPassword(e.target.value)}
          aria-label="Password"
          onFocus={() => { setPasswordFocused(true); }}
          onBlur={() => { setPasswordFocused(false); }}
          style={{
            width: '100%', padding: '8px 12px', background: tokens.bg, border: `1px solid ${tokens.border}`, borderRadius: 4, color: tokens.text, marginBottom: 16, boxSizing: 'border-box',
            outline: passwordFocused ? '2px solid' : 'none', outlineColor: tokens.cta, outlineOffset: 2,
            transition: tokens.transition,
          }} />
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
