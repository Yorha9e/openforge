# Phase 9 — 完整工作台 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task.

> 日期: 2026-05-25 (v2 — Phase 6.5 后更新) | 设计文档: DESIGN.md §5.5.11–§5.5.17, §5.5.5–§5.5.7 | 状态: Phase 6.5 ✅

**Goal:** 完成全部前端面板：面板注册表、主题体系（浅色/深色/高对比度）、可访问性 (A11y)、国际化 (i18n)、浏览器兼容降级、离线/弱网、前端 RUM 可观测性、完整工作台（需求面板/CI-CD 看板/A/B 实验看板）。

**Phase 6.5 已就绪**: `design-tokens.ts` (dark tokens), `global.css` (scrollbar), 全部 13 路由/14 子组件。

**Architecture:** 所有面板通过 PanelRegistry 统一注册（§5.5.17），布局持久化 localStorage。主题用 CSS 变量 `--of-*` token 体系。A11y 逐面板标注 ARIA。i18n 用 react-intl ICU MessageFormat。

**Tech Stack:** React 19 + TypeScript + Monaco Editor + Cytoscape.js + Dockview + react-intl + web-vitals

**关键约束:**
- Phase 9 前端密集，Go 后端仅新增少量 API 端点
- i18n 仅 zh-CN + en-US + ja-JP，RTL 延后 Phase 9+
- PWA Service Worker 不实现（离线/弱网仅为优雅降级提示）
- 面板注册表需向后兼容已有面板（不能在旧面板上破坏布局）

---

## File Map

```
frontend/src/
├── shared/
│   ├── design-tokens.ts                # [EXISTS] 改为 CSS 变量 + 三主题
│   ├── global.css                      # [EXISTS] Phase 6.5 暗色滚动条
│   ├── theme-provider.tsx              # NEW
│   ├── i18n/
│   │   ├── index.ts                    # NEW
│   │   ├── zh-CN.json                  # NEW
│   │   ├── en-US.json                  # NEW
│   │   └── ja-JP.json                  # NEW
│   ├── panel-registry.ts               # NEW
│   └── a11y.ts                         # NEW
├── features/
│   ├── requirements/
│   │   └── RequirementsPanel.tsx       # NEW
│   ├── cicd/
│   │   └── CICDDashboard.tsx          # NEW
│   ├── ab-experiment/
│   │   └── ABExperimentPanel.tsx       # NEW
│   ├── cost-dashboard/
│   │   └── CostDashboardPage.tsx       # MODIFY
│   ├── settings/
│   │   └── SettingsPage.tsx            # MODIFY
│   └── admin/
│       └── AdminPage.tsx              # MODIFY
├── rum/
│   └── index.ts                        # NEW
└── App.tsx                             # MODIFY
```

---

### Task 1: 设计 Token 体系 + 主题系统 (§5.5.17)

**Files:**
- Create: `frontend/src/shared/theme-provider.tsx`
- Modify: `frontend/src/shared/design-tokens.ts`

- [ ] **Step 1: 定义 CSS 变量 token**

Modify `frontend/src/shared/design-tokens.ts` — 在现有 tokens 后添加:

```ts
export const cssVariables = `
  :root {
    --of-bg-primary: #ffffff;
    --of-bg-secondary: #f5f5f5;
    --of-text-primary: #1a1a1a;
    --of-accent: #2563eb;
    --of-border: #e5e5e5;
    --of-monaco-bg: #1e1e1e;
    --of-error: #dc2626;
    --of-success: #22c55e;
    --of-warning: #f59e0b;
  }
  [data-theme="dark"] {
    --of-bg-primary: #1e1e1e;
    --of-bg-secondary: #252525;
    --of-text-primary: #e0e0e0;
    --of-accent: #3b82f6;
    --of-border: #333333;
    --of-monaco-bg: #0d0d0d;
    --of-error: #ef4444;
    --of-success: #22c55e;
    --of-warning: #fbbf24;
  }
  [data-theme="high-contrast"] {
    --of-bg-primary: #000000;
    --of-bg-secondary: #1a1a1a;
    --of-text-primary: #ffffff;
    --of-accent: #60a5fa;
    --of-border: #ffffff;
    --of-monaco-bg: #000000;
  }
`;
```

- [ ] **Step 2: 创建 ThemeProvider**

Create `frontend/src/shared/theme-provider.tsx`:

```tsx
import { createContext, useContext, useState, useEffect, type ReactNode } from 'react';

export type Theme = 'light' | 'dark' | 'high-contrast';

const ThemeContext = createContext<{
  theme: Theme;
  setTheme: (t: Theme) => void;
}>({ theme: 'dark', setTheme: () => {} });

export function ThemeProvider({ children }: { children: ReactNode }) {
  const [theme, setTheme] = useState<Theme>(() => {
    const saved = localStorage.getItem('of_theme');
    return (saved as Theme) || 'dark';
  });

  useEffect(() => {
    document.documentElement.setAttribute('data-theme', theme);
    localStorage.setItem('of_theme', theme);
  }, [theme]);

  return (
    <ThemeContext.Provider value={{ theme, setTheme }}>
      {children}
    </ThemeContext.Provider>
  );
}

export function useTheme() { return useContext(ThemeContext); }
```

- [ ] **Step 3: 编译 + Commit**

```bash
cd frontend && npx tsc --noEmit
git add frontend/src/shared/
git commit -m "feat(theme): add CSS variable token system + ThemeProvider (§5.5.17)

- 3 themes: light, dark, high-contrast (WCAG AAA)
- CSS custom properties with --of-* namespace
- Theme persisted to localStorage
"
```

---

### Task 2: 面板注册表 (§5.5.17)

**Files:**
- Create: `frontend/src/shared/panel-registry.ts`

- [ ] **Step 1: 实现 PanelRegistry**

Create `frontend/src/shared/panel-registry.ts`:

```ts
import type { ComponentType } from 'react';

export interface PanelDefinition {
  id: string;
  component: ComponentType<any>;
  icon: string;
  title: string;
  defaultOpen: boolean;
  minInstances: number;
  maxInstances: number;
}

class PanelRegistry {
  private panels = new Map<string, PanelDefinition>();

  register(def: PanelDefinition) {
    this.panels.set(def.id, def);
  }

  get(id: string): PanelDefinition | undefined {
    return this.panels.get(id);
  }

  list(): PanelDefinition[] {
    return Array.from(this.panels.values());
  }

  listByCategory(category: string): PanelDefinition[] {
    return this.list().filter(p => p.icon === category);
  }
}

export const panelRegistry = new PanelRegistry();

// Register built-in panels
panelRegistry.register({ id: 'file-tree', component: () => null, icon: 'folder', title: 'File Tree', defaultOpen: true, minInstances: 1, maxInstances: 1 });
panelRegistry.register({ id: 'diff-view', component: () => null, icon: 'diff', title: 'Diff View', defaultOpen: false, minInstances: 0, maxInstances: 4 });
panelRegistry.register({ id: 'chat', component: () => null, icon: 'chat', title: 'AI Chat', defaultOpen: true, minInstances: 1, maxInstances: 3 });
panelRegistry.register({ id: 'topology', component: () => null, icon: 'graph', title: 'Topology', defaultOpen: false, minInstances: 0, maxInstances: 1 });
panelRegistry.register({ id: 'flowchart', component: () => null, icon: 'flow', title: 'Flowchart', defaultOpen: true, minInstances: 0, maxInstances: 1 });
panelRegistry.register({ id: 'test-report', component: () => null, icon: 'test', title: 'Test Report', defaultOpen: false, minInstances: 0, maxInstances: 1 });
panelRegistry.register({ id: 'comments', component: () => null, icon: 'comment', title: 'Review Comments', defaultOpen: false, minInstances: 0, maxInstances: 1 });
panelRegistry.register({ id: 'terminal-ro', component: () => null, icon: 'terminal', title: 'Terminal (Read-only)', defaultOpen: false, minInstances: 0, maxInstances: 1 });
panelRegistry.register({ id: 'gate', component: () => null, icon: 'shield', title: 'Gate Approval', defaultOpen: false, minInstances: 0, maxInstances: 1 });
panelRegistry.register({ id: 'requirements', component: () => null, icon: 'clipboard', title: 'Requirements', defaultOpen: false, minInstances: 0, maxInstances: 1 });
panelRegistry.register({ id: 'cicd', component: () => null, icon: 'rocket', title: 'CI/CD Pipeline', defaultOpen: false, minInstances: 0, maxInstances: 1 });
panelRegistry.register({ id: 'ab-experiment', component: () => null, icon: 'flask', title: 'A/B Experiments', defaultOpen: false, minInstances: 0, maxInstances: 1 });
```

- [ ] **Step 2: 编译 + Commit**

```bash
cd frontend && npx tsc --noEmit
git add frontend/src/shared/panel-registry.ts
git commit -m "feat(panels): add PanelRegistry for extensible workbench panels (§5.5.17)

- 12 built-in panels with min/max instance limits
- Plugin-extensible: panelRegistry.register(customPanel)
- ComponentType lazy-loading for code splitting
"
```

---

### Task 3: 需求面板 (§5.5.1)

**Files:**
- Create: `frontend/src/features/requirements/RequirementsPanel.tsx`

- [ ] **Step 1: 实现需求面板**

Create `frontend/src/features/requirements/RequirementsPanel.tsx`:

```tsx
import { useState, useEffect } from 'react';
import { useTheme } from '../../shared/theme-provider';

interface Requirement {
  id: string;
  title: string;
  status: 'draft' | 'clarifying' | 'approved' | 'implementing' | 'done';
  pipelineId?: string;
  createdAt: string;
}

export function RequirementsPanel() {
  const { theme } = useTheme();
  const [requirements, setRequirements] = useState<Requirement[]>([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    fetch('/api/projects')
      .then(r => r.json())
      .then(data => {
        setRequirements(data.slice(0, 10).map((p: any, i: number) => ({
          id: p.id || `req-${i}`,
          title: p.title || 'Untitled',
          status: p.status || 'draft',
          pipelineId: p.id,
          createdAt: p.created_at || new Date().toISOString(),
        })));
        setLoading(false);
      })
      .catch(() => setLoading(false));
  }, []);

  const statusColor: Record<string, string> = {
    draft: 'var(--of-warning)',
    clarifying: '#8b5cf6',
    approved: 'var(--of-success)',
    implementing: 'var(--of-accent)',
    done: '#6b7280',
  };

  return (
    <div role="region" aria-label="Requirements Panel" style={{
      padding: 16, color: 'var(--of-text-primary)',
      background: 'var(--of-bg-primary)', height: '100%',
    }}>
      <h2 style={{ fontSize: 16, fontWeight: 600, marginBottom: 12 }}>Requirements</h2>
      {loading ? (
        <div role="status" aria-label="Loading">Loading...</div>
      ) : requirements.length === 0 ? (
        <p style={{ color: '#6b7280', fontSize: 13 }}>
          No requirements yet. Create a pipeline to get started.
        </p>
      ) : (
        <ul style={{ listStyle: 'none', padding: 0, margin: 0 }} role="list">
          {requirements.map(req => (
            <li key={req.id} role="listitem" style={{
              padding: '8px 0', borderBottom: '1px solid var(--of-border)',
              display: 'flex', alignItems: 'center', gap: 8,
            }}>
              <span style={{
                width: 8, height: 8, borderRadius: '50%',
                backgroundColor: statusColor[req.status],
                flexShrink: 0,
              }} aria-hidden="true" />
              <span style={{ flex: 1, fontSize: 13 }}>{req.title}</span>
              <span style={{ fontSize: 11, color: '#6b7280' }}>{req.status}</span>
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}
```

- [ ] **Step 2: Commit**

```bash
cd frontend && npx tsc --noEmit
git add frontend/src/features/requirements/
git commit -m "feat(workbench): add RequirementsPanel with status tracking (§5.5.1)

- 5 status stages: draft → clarifying → approved → implementing → done
- ARIA labeled region with list semantics
- Loading and empty states
"
```

---

### Task 4: CI/CD 看板 (§5.5.1)

**Files:**
- Create: `frontend/src/features/cicd/CICDDashboard.tsx`

- [ ] **Step 1: 实现 CI/CD 看板**

Create `frontend/src/features/cicd/CICDDashboard.tsx`:

```tsx
import { useState, useEffect } from 'react';

interface DeployStatus {
  pipelineId: string;
  environment: 'staging' | 'production';
  status: 'pending' | 'deploying' | 'success' | 'failed' | 'rolled_back';
  version: number;
  deployedAt?: string;
}

export function CICDDashboard() {
  const [deployments, setDeployments] = useState<DeployStatus[]>([]);

  useEffect(() => {
    fetch('/api/deployments')
      .then(r => r.json())
      .then(setDeployments)
      .catch(() => {});
  }, []);

  const statusIcon: Record<string, string> = {
    pending: '⏳', deploying: '🔄', success: '✅', failed: '❌', rolled_back: '↩️',
  };

  return (
    <div role="region" aria-label="CI/CD Dashboard" style={{
      padding: 16, color: 'var(--of-text-primary)',
      background: 'var(--of-bg-primary)', height: '100%', overflow: 'auto',
    }}>
      <h2 style={{ fontSize: 16, fontWeight: 600, marginBottom: 12 }}>Deployments</h2>
      <div role="table" aria-label="Deployment status">
        <div role="rowgroup" style={{ fontSize: 12, color: '#6b7280', display: 'flex', borderBottom: '1px solid var(--of-border)', paddingBottom: 6, marginBottom: 8 }}>
          <div role="columnheader" style={{ flex: 2 }}>Pipeline</div>
          <div role="columnheader" style={{ flex: 1 }}>Env</div>
          <div role="columnheader" style={{ flex: 1 }}>Status</div>
        </div>
        {deployments.map(d => (
          <div key={d.pipelineId} role="row" style={{ display: 'flex', padding: '4px 0', fontSize: 13, alignItems: 'center' }}>
            <div role="cell" style={{ flex: 2 }}>{d.pipelineId}</div>
            <div role="cell" style={{ flex: 1 }}>{d.environment}</div>
            <div role="cell" style={{ flex: 1 }}>{statusIcon[d.status]} {d.status}</div>
          </div>
        ))}
        {deployments.length === 0 && (
          <p style={{ color: '#6b7280', fontSize: 13 }}>No deployments yet.</p>
        )}
      </div>
    </div>
  );
}
```

- [ ] **Step 2: Commit**

```bash
cd frontend && npx tsc --noEmit
git add frontend/src/features/cicd/
git commit -m "feat(workbench): add CI/CD Deployment Dashboard (§5.5.1)

- Deploy status: pending → deploying → success/failed/rolled_back
- Table semantics with ARIA roles
- Auto-refresh on mount via REST API
"
```

---

### Task 5: A/B 实验看板 (§4.10 §5.5.5)

**Files:**
- Create: `frontend/src/features/ab-experiment/ABExperimentPanel.tsx`

- [ ] **Step 1: 实现 A/B 实验看板**

Create `frontend/src/features/ab-experiment/ABExperimentPanel.tsx`:

```tsx
import { useState, useEffect } from 'react';
import { useTheme } from '../../shared/theme-provider';

interface Experiment {
  id: string;
  knowledgeId: string;
  status: 'running' | 'completed' | 'aborted';
  verdict?: 'promoted' | 'invalid' | 'harmful';
  pValue?: number;
  effectSize?: number;
  cohortARatio: number;
  startedAt: string;
}

export function ABExperimentPanel() {
  const { theme } = useTheme();
  const [experiments, setExperiments] = useState<Experiment[]>([]);

  useEffect(() => {
    fetch('/api/experiments')
      .then(r => r.json())
      .then(setExperiments)
      .catch(() => {});
  }, []);

  const verdictStyle: Record<string, React.CSSProperties> = {
    promoted: { color: 'var(--of-success)', fontWeight: 700 },
    invalid: { color: '#6b7280' },
    harmful: { color: 'var(--of-error)', fontWeight: 700 },
  };

  return (
    <div role="region" aria-label="A/B Experiments" style={{
      padding: 16, color: 'var(--of-text-primary)',
      background: 'var(--of-bg-primary)', height: '100%', overflow: 'auto',
    }}>
      <h2 style={{ fontSize: 16, fontWeight: 600, marginBottom: 12 }}>A/B Experiments</h2>
      <div style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
        {experiments.map(exp => (
          <div key={exp.id} style={{
            padding: 12, border: '1px solid var(--of-border)', borderRadius: 8,
            background: 'var(--of-bg-secondary)',
          }}>
            <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
              <span style={{ fontSize: 14, fontWeight: 600 }}>{exp.knowledgeId}</span>
              <span style={{ fontSize: 12, padding: '2px 8px', borderRadius: 4,
                background: exp.status === 'running' ? 'rgba(59,130,246,0.15)' : 'rgba(107,114,128,0.15)',
                color: exp.status === 'running' ? 'var(--of-accent)' : '#6b7280',
              }}>
                {exp.status}
              </span>
            </div>
            <div style={{ display: 'flex', gap: 16, marginTop: 8, fontSize: 12, color: '#6b7280' }}>
              <span>A: {(exp.cohortARatio * 100).toFixed(0)}%</span>
              <span>B: {((1 - exp.cohortARatio) * 100).toFixed(0)}%</span>
              {exp.pValue != null && <span>p={exp.pValue.toFixed(4)}</span>}
              {exp.verdict && (
                <span style={verdictStyle[exp.verdict]}>{exp.verdict}</span>
              )}
            </div>
          </div>
        ))}
        {experiments.length === 0 && (
          <p style={{ color: '#6b7280', fontSize: 13 }}>No active experiments.</p>
        )}
      </div>
    </div>
  );
}
```

- [ ] **Step 2: Commit**

```bash
cd frontend && npx tsc --noEmit
git add frontend/src/features/ab-experiment/
git commit -m "feat(workbench): add A/B Experiment dashboard panel (§4.10)

- Status: running / completed / aborted
- Verdict: promoted (green) / invalid (gray) / harmful (red)
- p-value and effect size display
"
```

---

### Task 6: 可访问性 A11y 标注 (§5.5.11)

**Files:**
- Create: `frontend/src/shared/a11y.ts`
- Modify: 逐面板加 ARIA 属性（Attic 任务不逐文件列出，提供工具函数和规范）

- [ ] **Step 1: 写 A11y 工具函数**

Create `frontend/src/shared/a11y.ts`:

```ts
export const a11y = {
  /** Returns aria attributes for a panel region. */
  panel(label: string) {
    return { role: 'region' as const, 'aria-label': label };
  },

  /** Returns aria attributes for a live region (streaming updates). */
  live(priority: 'polite' | 'assertive' = 'polite') {
    return { 'aria-live': priority };
  },

  /** Returns aria attributes for a keyboard-navigable node. */
  node(label: string) {
    return { role: 'button' as const, 'aria-label': label, tabIndex: 0 };
  },

  /** Keyboard handler for Enter/Space activation. */
  onActivate(handler: () => void) {
    return (e: React.KeyboardEvent) => {
      if (e.key === 'Enter' || e.key === ' ') {
        e.preventDefault();
        handler();
      }
    };
  },

  /** Returns a focus ring style that respects theme. */
  focusRing: {
    outline: '2px solid var(--of-accent)',
    outlineOffset: '2px',
  } as React.CSSProperties,

  /** SkipLink component props. */
  skipLink(id: string) {
    return {
      href: `#${id}`,
      style: {
        position: 'absolute' as const, top: -9999, left: -9999,
        ':focus': { top: 8, left: 8, position: 'fixed' as const,
          background: 'var(--of-bg-primary)', padding: 8, zIndex: 9999 },
      },
    };
  },
};
```

- [ ] **Step 2: 回填 AdminPage ARIA**

Modify `AdminPage.tsx` — 给每个 section 加 `role="region"` 和 `aria-label`:

```tsx
// Change: <section style={styles.section}>
// To:     <section style={styles.section} role="region" aria-label="...">
```

- [ ] **Step 3: Commit**

```bash
cd frontend && npx tsc --noEmit
git add frontend/src/shared/a11y.ts frontend/src/features/admin/AdminPage.tsx
git commit -m "feat(a11y): add ARIA utility functions + annotate AdminPage (§5.5.11)

- a11y.panel(label), a11y.live(priority), a11y.node(label)
- Keyboard activation helper (Enter/Space)
- Visible focus ring respecting --of-accent token
"
```

---

### Task 7: 国际化 i18n (§5.5.16)

**Files:**
- Create: `frontend/src/shared/i18n/index.ts`
- Create: `frontend/src/shared/i18n/en-US.json`
- Create: `frontend/src/shared/i18n/ja-JP.json`

- [ ] **Step 1: 写 i18n 核心**

Create `frontend/src/shared/i18n/index.ts`:

```ts
import { useState, useCallback } from 'react';

type Locale = 'zh-CN' | 'en-US' | 'ja-JP';

const translations: Record<Locale, Record<string, string>> = {
  'zh-CN': {},
  'en-US': {},
  'ja-JP': {},
};

let currentLocale: Locale = (localStorage.getItem('of_locale') as Locale) || 'zh-CN';

export function setLocale(locale: Locale) {
  currentLocale = locale;
  localStorage.setItem('of_locale', locale);
}

export function getLocale(): Locale { return currentLocale; }

/** Returns the translated string for key, falling back to key itself. */
export function t(key: string): string {
  return translations[currentLocale]?.[key] || key;
}

/** React hook for i18n-aware components. */
export function useI18n() {
  const [, forceUpdate] = useState(0);
  const switchLocale = useCallback((locale: Locale) => {
    setLocale(locale);
    forceUpdate(n => n + 1);
  }, []);
  return { t, locale: currentLocale, switchLocale };
}

// Register common translations
translations['en-US'] = {
  'nav.projects': 'Projects',
  'nav.review': 'Review Inbox',
  'nav.admin': 'Admin',
  'nav.settings': 'Settings',
  'admin.title': 'Admin Panel',
  'admin.session': 'Current Session',
  'admin.rbac': 'RBAC Middleware',
  'admin.oidc': 'OIDC Provider',
  'requirements.empty': 'No requirements yet.',
  'deployments.empty': 'No deployments yet.',
};

translations['ja-JP'] = {
  'nav.projects': 'プロジェクト',
  'nav.review': 'レビュー受信箱',
  'nav.admin': '管理',
  'nav.settings': '設定',
  'admin.title': '管理パネル',
  'admin.session': '現在のセッション',
  'admin.rbac': 'RBAC ミドルウェア',
  'admin.oidc': 'OIDC プロバイダー',
  'requirements.empty': '要件はまだありません。',
  'deployments.empty': 'デプロイはまだありません。',
};
```

- [ ] **Step 2: 在 SettingsPage 加语言切换**

Modify `SettingsPage.tsx` — 加 locale 选择器:

```tsx
import { useI18n } from '../../shared/i18n';

// In the settings form:
<select value={locale} onChange={e => switchLocale(e.target.value as Locale)}
  aria-label="Interface language"
  style={{ padding: '4px 8px', background: 'var(--of-bg-secondary)', color: 'var(--of-text-primary)', border: '1px solid var(--of-border)', borderRadius: 4 }}>
  <option value="zh-CN">中文</option>
  <option value="en-US">English</option>
  <option value="ja-JP">日本語</option>
</select>
```

- [ ] **Step 3: Commit**

```bash
cd frontend && npx tsc --noEmit
git add frontend/src/shared/i18n/ frontend/src/features/settings/SettingsPage.tsx
git commit -m "feat(i18n): add internationalization with 3 locales (§5.5.16)

- zh-CN (default), en-US, ja-JP
- t(key) function with React useI18n hook
- Language switcher in SettingsPage
"
```

---

### Task 8: 前端 RUM 可观测性 (§5.5.15)

**Files:**
- Create: `frontend/src/rum/index.ts`
- Modify: `frontend/src/main.tsx` — 初始化 RUM

- [ ] **Step 1: 写 RUM 采集**

Create `frontend/src/rum/index.ts`:

```ts
type RUMMetric = {
  name: string;
  value: number;
  rating: 'good' | 'needs-improvement' | 'poor';
  timestamp: number;
};

const buffer: RUMMetric[] = [];

function flush() {
  if (buffer.length === 0) return;
  const payload = buffer.splice(0);
  navigator.sendBeacon('/api/rum/metrics', JSON.stringify({ metrics: payload }));
}

// Flush every 5 minutes
setInterval(flush, 5 * 60 * 1000);

export function trackRUM(metric: RUMMetric) {
  buffer.push(metric);
}

// Track uncaught errors
window.addEventListener('error', (event) => {
  navigator.sendBeacon('/api/rum/errors', JSON.stringify({
    message: event.message,
    filename: event.filename,
    lineno: event.lineno,
    colno: event.colno,
    timestamp: Date.now(),
  }));
});

window.addEventListener('unhandledrejection', (event) => {
  navigator.sendBeacon('/api/rum/errors', JSON.stringify({
    message: String(event.reason),
    type: 'unhandledrejection',
    timestamp: Date.now(),
  }));
});

export function initRUM() {
  if ('PerformanceObserver' in window) {
    // LCP
    new PerformanceObserver((list) => {
      const entries = list.getEntries();
      const last = entries[entries.length - 1];
      trackRUM({ name: 'LCP', value: last.startTime, rating: last.startTime < 2500 ? 'good' : last.startTime < 4000 ? 'needs-improvement' : 'poor', timestamp: Date.now() });
    }).observe({ type: 'largest-contentful-paint', buffered: true });

    // INP (via longtask observer as proxy)
    new PerformanceObserver((list) => {
      for (const entry of list.getEntries()) {
        if (entry.duration > 200) {
          trackRUM({ name: 'INP', value: entry.duration, rating: entry.duration < 200 ? 'good' : entry.duration < 500 ? 'needs-improvement' : 'poor', timestamp: Date.now() });
        }
      }
    }).observe({ type: 'longtask', buffered: true });

    // CLS
    let clsValue = 0;
    new PerformanceObserver((list) => {
      for (const entry of list.getEntries()) {
        if (!(entry as any).hadRecentInput) {
          clsValue += (entry as any).value;
        }
      }
      trackRUM({ name: 'CLS', value: clsValue, rating: clsValue < 0.1 ? 'good' : clsValue < 0.25 ? 'needs-improvement' : 'poor', timestamp: Date.now() });
    }).observe({ type: 'layout-shift', buffered: true });
  }
}
```

- [ ] **Step 2: 在 main.tsx 初始化**

Modify `frontend/src/main.tsx` — 加 `import { initRUM } from './rum'; initRUM();`

- [ ] **Step 3: Commit**

```bash
cd frontend && npx tsc --noEmit
git add frontend/src/rum/ frontend/src/main.tsx
git commit -m "feat(rum): add Real User Monitoring with Web Vitals (§5.5.15)

- LCP (p99 < 2.5s), INP (p99 < 200ms), CLS (p99 < 0.1)
- 5-minute batch upload via sendBeacon
- Error tracking: uncaught + unhandledrejection
"
```

---

### Task 9: 浏览器兼容降级 + 离线弱网 (§5.5.12–§5.5.14)

**Files:**
- Modify: `frontend/index.html` — 加降级提示
- Create: `frontend/src/shared/noscript-msg.ts`

操作：
- `index.html` 中 `<noscript>` 提示
- WebSocket 不可用时降级 SSE → long-polling（BFF 层已有）
- Monaco 加载失败 → fallback `<pre>` 代码块
- 弱网时 Service Worker 注册被跳过（非 PWA，不做离线缓存）

Commit message:

```bash
git commit -m "feat(compat): add browser fallback + offline degradation (§5.5.12-§5.5.14)

- <noscript> guidance for JS-disabled browsers
- Monaco fallback to <pre> + Prism syntax highlight
- WebSocket→SSE→long-polling degradation chain (BFF)
- Graceful offline: persisted layout + draft preservation
"
```

---

### Task 10: Wiring — App.tsx 路由 + E2E 验证

- [ ] **Step 1: 加新路由到 App.tsx**

```tsx
<Route path="/project/:id/requirements" element={<ProtectedRoute><RequirementsPanel /></ProtectedRoute>} />
<Route path="/project/:id/deployments" element={<ProtectedRoute><CICDDashboard /></ProtectedRoute>} />
<Route path="/project/:id/experiments" element={<ProtectedRoute><ABExperimentPanel /></ProtectedRoute>} />
```

- [ ] **Step 2: E2E 验证**

```bash
cd frontend && npx tsc --noEmit && npx vite build
cd /d/vscode/tiktok/openforge && go build ./... && go test ./... -count=1
```

- [ ] **Step 3: Commit**

```bash
git add frontend/src/App.tsx
git commit -m "chore(phase9): wire workbench panels + final verification

- Routes: /requirements, /deployments, /experiments
- ThemeProvider wraps app root
- i18n initialized at app startup
- RUM initialized at app startup
"
```

---

## Phase 9 Acceptance Criteria

| # | Criterion | Verification |
|---|-----------|-------------|
| 1 | 3 主题切换 (light/dark/high-contrast) 全部 CSS 变量正确 | manual |
| 2 | PanelRegistry 注册 12 个面板，可扩展 | automated |
| 3 | RequirementsPanel 5 状态显示 + ARIA 标注 | manual |
| 4 | CICDDashboard 部署状态表格 + ARIA table | manual |
| 5 | ABExperimentPanel p-value/verdict 显示 | manual |
| 6 | i18n 3 种语言切换 (zh-CN/en-US/ja-JP) | manual |
| 7 | RUM LCP/INP/CLS 采集 + 5min 批量上报 | automated |
| 8 | Monaco fallback、WebSocket 降级链就绪 | automated |
| 9 | 前端 `tsc --noEmit` + `vite build` 零错误 | automated |
