import { useSyncExternalStore } from 'react';
import { lazy } from 'react';
import type { ComponentType } from 'react';

export interface PanelDefinition {
  id: string;
  component: ComponentType<any>;
  title: string;
  icon: string;
  defaultPosition?: 'left' | 'right' | 'bottom' | 'center';
}

class PanelRegistry {
  private panels = new Map<string, PanelDefinition>();
  private listeners = new Set<() => void>();

  register(def: PanelDefinition): void {
    this.panels.set(def.id, def);
    this.notify();
  }

  unregister(id: string): void {
    this.panels.delete(id);
    this.notify();
  }

  get(id: string): PanelDefinition | undefined {
    return this.panels.get(id);
  }

  list(): PanelDefinition[] {
    return Array.from(this.panels.values());
  }

  subscribe(listener: () => void): () => void {
    this.listeners.add(listener);
    return () => this.listeners.delete(listener);
  }

  private notify(): void {
    this.listeners.forEach((fn) => fn());
  }
}

export function usePanel(id: string): PanelDefinition | undefined {
  return useSyncExternalStore(
    (cb) => panelRegistry.subscribe(cb),
    () => panelRegistry.get(id),
  );
}

export const panelRegistry = new PanelRegistry();

// Lazy-loaded panel components
const ChatPanel = lazy(() => import('../features/chat/ChatPanel').then(m => ({ default: m.ChatPanel })));
const DiffPanel = lazy(() => import('../features/code-review/DiffPanel').then(m => ({ default: m.DiffPanel })));
const FileTreePanel = lazy(() => import('../features/code-review/FileTreePanel').then(m => ({ default: m.FileTreePanel })));
const GatePanel = lazy(() => import('../features/code-review/GatePanel').then(m => ({ default: m.GatePanel })));
const TerminalPanel = lazy(() => import('../features/code-review/TerminalPanel').then(m => ({ default: m.TerminalPanel })));
const TopologyPanel = lazy(() => import('../features/code-review/TopologyPanel').then(m => ({ default: m.TopologyPanel })));
const TestReportPanel = lazy(() => import('../features/code-review/TestReportPanel').then(m => ({ default: m.TestReportPanel })));
const CICDDashboard = lazy(() => import('../features/cicd/CICDDashboard').then(m => ({ default: m.CICDDashboard })));
const FlowchartPanel = lazy(() => import('../features/code-review/FlowchartPanel').then(m => ({ default: m.FlowchartPanel })));
const CommentPanel = lazy(() => import('../features/code-review/CommentPanel').then(m => ({ default: m.CommentPanel })));
const ChatHistoryPanel = lazy(() => import('../features/code-review/ChatHistoryPanel').then(m => ({ default: m.ChatHistoryPanel })));

// Pre-register built-in panels with real lazy components
panelRegistry.register({ id: 'chat',         component: ChatPanel,        title: 'AI Chat',       icon: 'chat',     defaultPosition: 'center' });
panelRegistry.register({ id: 'diff',         component: DiffPanel,        title: 'Diff View',     icon: 'diff',     defaultPosition: 'right' });
panelRegistry.register({ id: 'filetree',     component: FileTreePanel,    title: 'File Tree',     icon: 'folder',   defaultPosition: 'left' });
panelRegistry.register({ id: 'gate',         component: GatePanel,        title: 'Gate Approval', icon: 'shield',   defaultPosition: 'right' });
panelRegistry.register({ id: 'terminal',     component: TerminalPanel,    title: 'Terminal',      icon: 'terminal', defaultPosition: 'bottom' });
panelRegistry.register({ id: 'topology',     component: TopologyPanel,    title: 'Topology',      icon: 'graph',    defaultPosition: 'center' });
panelRegistry.register({ id: 'test-report',  component: TestReportPanel,  title: 'Test Report',   icon: 'test',     defaultPosition: 'right' });
panelRegistry.register({ id: 'cicd',         component: CICDDashboard,    title: 'CI/CD',         icon: 'deploy',   defaultPosition: 'right' });
panelRegistry.register({ id: 'flowchart',    component: FlowchartPanel,   title: 'Flowchart',     icon: 'flow',     defaultPosition: 'center' });
panelRegistry.register({ id: 'comments',     component: CommentPanel,     title: 'Comments',      icon: 'comment',  defaultPosition: 'right' });
panelRegistry.register({ id: 'chat-history', component: ChatHistoryPanel, title: 'Chat History',  icon: 'history',  defaultPosition: 'left' });
