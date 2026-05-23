import { useState, type FormEvent } from 'react';
import { useNavigate } from 'react-router-dom';
import { useAuth } from '../../shared/auth';

export function LoginPage() {
  const { login, token } = useAuth();
  const navigate = useNavigate();
  const [username, setUsername] = useState('');
  const [password, setPassword] = useState('');
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(false);

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
    <div style={{ minHeight: '100vh', display: 'flex', alignItems: 'center', justifyContent: 'center', background: '#0f0f0f' }}>
      <form onSubmit={handleSubmit} style={{ background: '#1a1a1a', padding: 32, borderRadius: 8, width: '100%', maxWidth: 360 }}>
        <h1 style={{ fontSize: 24, fontWeight: 700, color: '#fff', textAlign: 'center', marginBottom: 24 }}>OpenForge</h1>
        {error && <p style={{ color: '#f87171', fontSize: 14, marginBottom: 12 }}>{error}</p>}
        <input type="text" placeholder="Username" value={username}
          onChange={e => setUsername(e.target.value)}
          style={{ width: '100%', padding: '8px 12px', background: '#262626', border: '1px solid #404040', borderRadius: 4, color: '#fff', marginBottom: 12, boxSizing: 'border-box' }}
          autoFocus />
        <input type="password" placeholder="Password" value={password}
          onChange={e => setPassword(e.target.value)}
          style={{ width: '100%', padding: '8px 12px', background: '#262626', border: '1px solid #404040', borderRadius: 4, color: '#fff', marginBottom: 16, boxSizing: 'border-box' }} />
        <button type="submit" disabled={loading}
          style={{ width: '100%', padding: '10px 0', background: '#2563eb', color: '#fff', border: 'none', borderRadius: 4, fontWeight: 500, cursor: loading ? 'default' : 'pointer', opacity: loading ? 0.5 : 1 }}>
          {loading ? 'Signing in...' : 'Sign In'}
        </button>
      </form>
    </div>
  );
}
