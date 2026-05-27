const BASE = '/api';

let authToken: string | null = null;

export function setToken(token: string | null) {
  authToken = token;
}

async function request<T>(path: string, options: RequestInit = {}): Promise<T> {
  const headers: Record<string, string> = {
    'Content-Type': 'application/json',
    ...((options.headers as Record<string, string>) || {}),
  };
  if (authToken) {
    headers['Authorization'] = `Bearer ${authToken}`;
  }
  const res = await fetch(`${BASE}${path}`, { ...options, headers });
  if (res.status === 401) {
    authToken = null;
    localStorage.removeItem('of_token');
    localStorage.removeItem('of_refresh');
    localStorage.removeItem('of_user');
    // Not on login page already — redirect silently
    if (!window.location.pathname.startsWith('/login')) {
      window.location.href = '/login';
    }
    throw new Error('Unauthorized');
  }
  if (!res.ok) {
    const err = await res.json().catch(() => ({ error: res.statusText }));
    throw new Error(err.error || 'Request failed');
  }
  return res.json();
}

export type AdminStatus = {
  phase: string;
  profile: string;
  tier: string;
  skills: number;
  rbac: string;
  oidc: string;
  auth_provider: string;
  models: number;
  circuit_breakers?: Record<string, string>;
  slo?: { total: number; success_rate: number; p95_ms?: number };
  ha?: { task_queue: string; hash_ring_nodes: number; load_shedding: string };
};

export type FeatureFlags = {
  enterprise_platform: boolean;
  compliance_suite: boolean;
  production_ops: boolean;
  distribution_artifacts: boolean;
};

export const api = {
  login: (username: string, password: string) =>
    request<{ access_token: string; refresh_token: string; expires_in: number; display_name?: string; role?: string }>('/auth/login', {
      method: 'POST',
      body: JSON.stringify({ username, password }),
    }),

  register: (username: string, password: string, displayName: string, avatarUrl?: string) =>
    request<{ access_token: string; refresh_token: string; expires_in: number; display_name: string; role: string }>('/auth/register', {
      method: 'POST',
      body: JSON.stringify({ username, password, display_name: displayName, avatar_url: avatarUrl || '' }),
    }),

  refreshToken: (refreshToken: string) =>
    request<{ access_token: string; refresh_token: string; expires_in: number }>('/auth/refresh', {
      method: 'POST',
      body: JSON.stringify({ refresh_token: refreshToken }),
    }),

  listProjects: () => request<any[]>('/projects'),

  getProject: (id: string) => request<any>(`/projects/${id}`),

  createProject: (name: string, gitUrl: string) =>
    request<any>('/projects', {
      method: 'POST',
      body: JSON.stringify({ name, git_url: gitUrl }),
    }),

  createPipeline: (projectId: string, title: string) =>
    request<any>(`/projects/${projectId}/pipelines`, {
      method: 'POST',
      body: JSON.stringify({ title }),
    }),

  getPipeline: (id: string) => request<any>(`/pipelines/${id}`),

  listPipelines: (projectId: string) => request<any[]>(`/projects/${projectId}/pipelines`),

  activePipelines: () => request<any[]>('/pipelines/active'),

  getMessages: (pipelineId: string) => request<any>(`/pipelines/${pipelineId}/messages`),

  deletePipeline: (id: string) => request<any>(`/pipelines/${id}`, { method: 'DELETE' }),

  // Gate
  getReviewInbox: () => request<any[]>('/review-inbox'),

  approveGate: (pipelineId: string, stage: string, checklist: any, summary: string) =>
    request<any>(`/pipelines/${pipelineId}/gate/${stage}`, {
      method: 'POST',
      body: JSON.stringify({ checklist, summary_feedback: summary }),
    }),

  rejectGate: (pipelineId: string, stage: string, comments: any[], summary: string) =>
    request<any>(`/pipelines/${pipelineId}/gate/${stage}/reject`, {
      method: 'POST',
      body: JSON.stringify({ line_comments: comments, summary_feedback: summary }),
    }),

  // Token / Cost Dashboard
  getTokenUsage: (projectId: string, days?: number) =>
    request<any[]>(`/projects/${projectId}/token-usage${days ? `?days=${days}` : ''}`),

  getTokenBudget: (projectId: string) =>
    request<any>(`/projects/${projectId}/token-budget`),

  // Models
  listModels: () => request<any[]>('/models'),

  // Settings
  getSettings: () => request<any>('/settings'),
  updateSettings: (settings: any) =>
    request<any>('/settings', { method: 'PUT', body: JSON.stringify(settings) }),

  // Skills
  listSkills: () => request<any[]>('/admin/skills'),
  updateSkillDeprecated: (name: string, deprecated: boolean) =>
    request<any>(`/admin/skills/${encodeURIComponent(name)}`, {
      method: 'PATCH',
      body: JSON.stringify({ deprecated }),
    }),
  updateSkillPriorities: (priorities: Record<string, number>) =>
    request<any>('/admin/skills/priorities', {
      method: 'PUT',
      body: JSON.stringify({ priorities }),
    }),

  // Admin
  getAdminStatus: () => request<AdminStatus>('/admin/status'),
  listExperiments: () => request<any[]>('/admin/experiments'),

  // Feature Flags
  getFeatureFlags: () => request<FeatureFlags>('/admin/feature-flags'),
  updateFeatureFlags: (flags: FeatureFlags) =>
    request<FeatureFlags>('/admin/feature-flags', {
      method: 'PUT',
      body: JSON.stringify(flags),
    }),

  // Health (public, but use request helper for consistency)
  getHealth: () => request<any>('/health'),
};

export function wsURL(): string {
  const proto = location.protocol === 'https:' ? 'wss:' : 'ws:';
  return `${proto}//${location.host}/ws/chat`;
}
