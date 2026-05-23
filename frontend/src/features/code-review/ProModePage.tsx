import { DockviewReact, DockviewReadyEvent, IDockviewPanelProps } from 'dockview';
import { ChatPanel } from '../chat/ChatPanel';
import { DiffPanel } from './DiffPanel';
import { FileTreePanel } from './FileTreePanel';
import { tokens } from '../../shared/design-tokens';

const components = {
  chat: (props: IDockviewPanelProps) => (
    <div style={{ height: '100%', overflow: 'hidden' }}>
      <ChatPanel />
    </div>
  ),
  diff: (props: IDockviewPanelProps) => (
    <div style={{ height: '100%' }}>
      <DiffPanel />
    </div>
  ),
  filetree: (props: IDockviewPanelProps) => (
    <div style={{ height: '100%', background: tokens.surface, color: tokens.text, padding: 12 }}>
      <FileTreePanel />
    </div>
  ),
};

export function ProModePage() {
  const onReady = (event: DockviewReadyEvent) => {
    const api = event.api;
    api.addPanel({ id: 'chat', component: 'chat', title: 'AI Chat' });
    api.addPanel({ id: 'diff', component: 'diff', title: 'Diff View', position: { direction: 'right' } });
    api.addPanel({ id: 'files', component: 'filetree', title: 'Files', position: { direction: 'left' } });
  };

  return (
    <div style={{ height: '100vh', background: tokens.bg }}>
      <DockviewReact components={components} onReady={onReady} className="dockview-theme-dark" />
    </div>
  );
}
