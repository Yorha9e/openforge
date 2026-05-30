const BASE = '/api';

// Detect Electron environment and cache API base URL
let electronApiBase: string | null = null;
let electronApiBasePromise: Promise<string> | null = null;

function getApiBase(): string | Promise<string> {
  if (electronApiBase) return electronApiBase;
  if (!window.electronAPI?.isElectron) return BASE;
  
  // Electron: fetch the full API base URL from main process
  if (!electronApiBasePromise) {
    electronApiBasePromise = window.electronAPI.getApiBaseUrl().then((base: string) => {
      electronApiBase = `${base}/api`;
      return electronApiBase;
    });
  }
  return electronApiBasePromise;
}

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

  // Resolve API base (may be async in Electron)
  const base = await getApiBase();

  // Add timeout via AbortController
  const controller = new AbortController();
  const timeout = setTimeout(() => controller.abort(), 30000); // 30s timeout

  try {
    const res = await fetch(`${base}${path}`, { ...options, headers, signal: controller.signal });
    if (res.status === 401) {
      authToken = null;
      localStorage.removeItem('of_token');
      localStorage.removeItem('of_refresh');
      localStorage.removeItem('of_user');
      // Use hash-compatible navigation for Electron
      if (window.electronAPI?.isElectron) {
        window.location.hash = '#/login';
      } else if (!window.location.pathname.startsWith('/login')) {
        window.location.href = '/login';
      }
      throw new Error('Unauthorized');
    }
    if (!res.ok) {
      const err = await res.json().catch(() => ({ error: res.statusText }));
      throw new Error(err.error || 'Request failed');
    }
    return res.json();
  } catch (err: any) {
    if (err.name === 'AbortError') {
      throw new Error('Request timeout (30s)');
    }
    throw err;
  } finally {
    clearTimeout(timeout);
  }
}

export type InfraStatus = 'connected' | 'degraded' | 'unavailable' | 'unused';

export type InfraComponent = {
  key: string;
  name: string;
  status: InfraStatus;
  uptime_seconds?: number;
  latency_ms?: number;
  message?: string;
  version?: string;
  circuit_breaker_state?: 'closed' | 'open' | 'half_open';
};

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
  infrastructure?: InfraComponent[];
};

/**
 * 当后端不提供 infrastructure 字段时，前端根据 profile + circuit_breakers 推导。
 */
export function deriveInfraHealth(status: AdminStatus): InfraComponent[] {
  const { profile, circuit_breakers, ha } = status;
  const isStandard = profile === 'standard' || profile === 'enterprise';
  const isEnterprise = profile === 'enterprise';

  const cb = (key: string): InfraStatus | undefined => {
    if (!circuit_breakers) return undefined;
    const state = circuit_breakers[key];
    if (state === 'open') return 'unavailable';
    if (state === 'half_open') return 'degraded';
    if (state === 'closed') return 'connected';
    return undefined;
  };

  return [
    { key: 'postgres', name: 'PostgreSQL', status: cb('postgres') ?? 'connected' },
    { key: 'docker', name: 'Docker', status: cb('docker') ?? 'connected' },
    { key: 'grpc_io', name: 'gRPC IO', status: 'connected' },
    { key: 'dr_backup', name: 'DR Backup', status: 'connected' },
    { key: 'redis', name: 'Redis', status: isStandard ? (cb('redis') ?? 'connected') : 'unused' },
    { key: 'minio', name: 'MinIO', status: isStandard ? (cb('minio') ?? 'connected') : 'unused' },
    { key: 'vault', name: 'Vault', status: isStandard ? 'connected' : 'unused' },
    { key: 'nginx', name: 'Nginx', status: isStandard ? 'connected' : 'unused' },
    { key: 'sandbox', name: 'Sandbox', status: isStandard ? 'connected' : 'unused' },
    { key: 'feishu', name: 'Feishu', status: isStandard ? 'connected' : 'unused' },
    { key: 'telemetry', name: 'Telemetry', status: isStandard ? 'connected' : 'unused' },
    { key: 'k8s', name: 'K8s', status: isEnterprise ? 'connected' : 'unused' },
  ];
}

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

  deleteProject: (id: string) => request<any>(`/projects/${id}`, { method: 'DELETE' }),

  createPipeline: (projectId: string, title: string) =>
    request<any>(`/projects/${projectId}/pipelines`, {
      method: 'POST',
      body: JSON.stringify({ title }),
    }),

  getPipeline: (id: string) => request<any>(`/pipelines/${id}`),
  getPipelineDiff: (pipelineId: string, filePath?: string) =>
    request<any>(`/pipelines/${pipelineId}/diff${filePath ? `?file=${encodeURIComponent(filePath)}` : ''}`),

  listPipelines: (projectId: string) => request<any[]>(`/projects/${projectId}/pipelines`),

  activePipelines: () => request<any[]>('/pipelines/active'),

  getMessages: (pipelineId: string) => request<any>(`/pipelines/${pipelineId}/messages`),

  listBranches: (pipelineId: string) => request<{ branches: any[] }>(`/pipelines/${pipelineId}/branches`),

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

  // File system browsing
  listFiles: (path: string) =>
    request<{ files: Array<{ name: string; is_dir: boolean; size: number; path: string }>; count: number; path: string }>(
      `/files?path=${encodeURIComponent(path)}`
    ),

  getFileContent: (path: string) =>
    request<{ content: string; language: string; path: string; size: number }>(
      `/files/content?path=${encodeURIComponent(path)}`
    ),
};

export function wsURL(): string | Promise<string> {
  // In Electron, use the server URL from main process
  if (window.electronAPI?.isElectron) {
    return window.electronAPI.getServerUrl();
  }
  const proto = location.protocol === 'https:' ? 'wss:' : 'ws:';
  return `${proto}//${location.host}/ws/chat`;
}
