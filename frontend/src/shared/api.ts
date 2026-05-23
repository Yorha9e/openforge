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
    throw new Error('Unauthorized');
  }
  if (!res.ok) {
    const err = await res.json().catch(() => ({ error: res.statusText }));
    throw new Error(err.error || 'Request failed');
  }
  return res.json();
}

export const api = {
  login: (username: string, password: string) =>
    request<{ access_token: string; refresh_token: string; expires_in: number }>('/auth/login', {
      method: 'POST',
      body: JSON.stringify({ username, password }),
    }),

  refreshToken: (refreshToken: string) =>
    request<{ access_token: string; refresh_token: string; expires_in: number }>('/auth/refresh', {
      method: 'POST',
      body: JSON.stringify({ refresh_token: refreshToken }),
    }),

  listProjects: () => request<any[]>('/projects'),

  getProject: (id: string) => request<any>(`/projects/${id}`),

  createPipeline: (projectId: string, title: string) =>
    request<any>(`/projects/${projectId}/pipelines`, {
      method: 'POST',
      body: JSON.stringify({ title }),
    }),

  getPipeline: (id: string) => request<any>(`/pipelines/${id}`),

  getMessages: (pipelineId: string) => request<any>(`/pipelines/${pipelineId}/messages`),

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
};

export function wsURL(): string {
  const proto = location.protocol === 'https:' ? 'wss:' : 'ws:';
  return `${proto}//${location.host}/ws/chat`;
}
