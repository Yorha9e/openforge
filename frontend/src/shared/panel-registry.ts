import { useSyncExternalStore } from 'react';
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

// Pre-register built-in panels (stubs — real components imported lazily)
panelRegistry.register({ id: 'chat',        component: () => null, title: 'AI Chat',       icon: 'chat',    defaultPosition: 'center' });
panelRegistry.register({ id: 'diff',        component: () => null, title: 'Diff View',     icon: 'diff',    defaultPosition: 'right' });
panelRegistry.register({ id: 'filetree',    component: () => null, title: 'File Tree',    icon: 'folder',  defaultPosition: 'left' });
panelRegistry.register({ id: 'gate',        component: () => null, title: 'Gate Approval', icon: 'shield',  defaultPosition: 'right' });
panelRegistry.register({ id: 'terminal',    component: () => null, title: 'Terminal',     icon: 'terminal',defaultPosition: 'bottom' });
panelRegistry.register({ id: 'topology',    component: () => null, title: 'Topology',     icon: 'graph',   defaultPosition: 'center' });
panelRegistry.register({ id: 'test-report', component: () => null, title: 'Test Report',  icon: 'test',    defaultPosition: 'right' });
panelRegistry.register({ id: 'cicd',        component: () => null, title: 'CI/CD',        icon: 'deploy',  defaultPosition: 'right' });
