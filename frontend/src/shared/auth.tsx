import { createContext, useContext, useState, useCallback, type ReactNode } from 'react';
import { api, setToken } from './api';

function parseRoleFromJWT(token: string): string {
  try {
    const parts = token.split('.');
    const payload = JSON.parse(atob(parts[1] || ''));
    return payload.role || 'pm';
  } catch {
    return 'pm';
  }
}

interface AuthState {
  token: string | null;
  refreshToken: string | null;
  user: { id: string; role: string; project_id?: string; project_name?: string } | null;
  login: (username: string, password: string) => Promise<void>;
  register: (username: string, password: string, displayName: string, role?: string, email?: string) => Promise<void>;
  registerWithInvitation: (token: string, username: string, password: string, displayName: string, email: string) => Promise<void>;
  logout: () => void;
}

const AuthContext = createContext<AuthState>({
  token: null, refreshToken: null, user: null,
  login: async () => {}, register: async () => {}, registerWithInvitation: async () => {}, logout: () => {},
});

export function AuthProvider({ children }: { children: ReactNode }) {
  const [token, setAccessToken] = useState<string | null>(() => {
    const t = localStorage.getItem('of_token');
    if (t) setToken(t);
    return t;
  });
  const [refreshToken, setRefreshToken] = useState<string | null>(() => localStorage.getItem('of_refresh'));
  const [user, setUser] = useState<{ id: string; role: string; project_id?: string; project_name?: string } | null>(() => {
    const u = localStorage.getItem('of_user');
    return u ? JSON.parse(u) : null;
  });

  const applyAuthResult = useCallback((result: { access_token: string; refresh_token: string; expires_in: number; display_name?: string; role?: string; project_id?: string; project_name?: string }, username: string) => {
    setAccessToken(result.access_token);
    setRefreshToken(result.refresh_token);
    const u = { 
      id: username, 
      role: result.role || parseRoleFromJWT(result.access_token),
      project_id: result.project_id,
      project_name: result.project_name
    };
    setUser(u);
    localStorage.setItem('of_token', result.access_token);
    localStorage.setItem('of_refresh', result.refresh_token);
    localStorage.setItem('of_user', JSON.stringify(u));
    setToken(result.access_token);
  }, []);

  const login = useCallback(async (username: string, password: string) => {
    const result = await api.login(username, password);
    applyAuthResult(result, username);
  }, [applyAuthResult]);

  const register = useCallback(async (username: string, password: string, displayName: string, role?: string, email?: string) => {
    const result = await api.register(username, password, displayName, role, email);
    applyAuthResult(result, username);
  }, [applyAuthResult]);

  const registerWithInvitation = useCallback(async (token: string, username: string, password: string, displayName: string, email: string) => {
    const result = await api.registerWithInvitation(token, username, password, displayName, email);
    applyAuthResult(result, username);
    return result.project_id;
  }, [applyAuthResult]);

  const logout = useCallback(() => {
    setAccessToken(null); setRefreshToken(null); setUser(null);
    localStorage.removeItem('of_token');
    localStorage.removeItem('of_refresh');
    localStorage.removeItem('of_user');
    setToken(null);
  }, []);

  return (
    <AuthContext.Provider value={{ token, refreshToken, user, login, register, registerWithInvitation, logout }}>
      {children}
    </AuthContext.Provider>
  );
}

export function useAuth() { return useContext(AuthContext); }

export function useRole(): string {
  const { user } = useAuth();
  return user?.role || '';
}

const roleHierarchy: Record<string, string[]> = {
  admin:    ['admin', 'pm', 'dev_lead', 'dev', 'observer'],
  pm:       ['pm', 'dev', 'observer'],
  dev_lead: ['dev_lead', 'dev', 'observer'],
  dev:      ['dev', 'observer'],
  observer: ['observer'],
};

export function useCanAccess(requiredRole: string): boolean {
  const role = useRole();
  if (!role) return false;
  const allowed = roleHierarchy[role];
  if (!allowed) return false;
  return allowed.includes(requiredRole);
}
