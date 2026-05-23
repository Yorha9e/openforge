import { createContext, useContext, useState, useCallback, useEffect, type ReactNode } from 'react';
import { api, setToken } from './api';

interface AuthState {
  token: string | null;
  refreshToken: string | null;
  user: { id: string; role: string } | null;
  login: (username: string, password: string) => Promise<void>;
  logout: () => void;
}

const AuthContext = createContext<AuthState>({
  token: null, refreshToken: null, user: null,
  login: async () => {}, logout: () => {},
});

export function AuthProvider({ children }: { children: ReactNode }) {
  const [token, setAccessToken] = useState<string | null>(() => localStorage.getItem('of_token'));
  const [refreshToken, setRefreshToken] = useState<string | null>(() => localStorage.getItem('of_refresh'));
  const [user, setUser] = useState<{ id: string; role: string } | null>(() => {
    const u = localStorage.getItem('of_user');
    return u ? JSON.parse(u) : null;
  });

  useEffect(() => { setToken(token); }, [token]);

  const login = useCallback(async (username: string, password: string) => {
    const result = await api.login(username, password);
    setAccessToken(result.access_token);
    setRefreshToken(result.refresh_token);
    const u = { id: username, role: 'pm' };
    setUser(u);
    localStorage.setItem('of_token', result.access_token);
    localStorage.setItem('of_refresh', result.refresh_token);
    localStorage.setItem('of_user', JSON.stringify(u));
    setToken(result.access_token);
  }, []);

  const logout = useCallback(() => {
    setAccessToken(null); setRefreshToken(null); setUser(null);
    localStorage.removeItem('of_token');
    localStorage.removeItem('of_refresh');
    localStorage.removeItem('of_user');
    setToken(null);
  }, []);

  return (
    <AuthContext.Provider value={{ token, refreshToken, user, login, logout }}>
      {children}
    </AuthContext.Provider>
  );
}

export function useAuth() { return useContext(AuthContext); }
