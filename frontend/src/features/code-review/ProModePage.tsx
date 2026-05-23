import { DockviewReact, DockviewReadyEvent, IDockviewPanelProps } from 'dockview';
import { ChatPanel } from '../chat/ChatPanel';
import { DiffPanel } from './DiffPanel';
import { FileTreePanel } from './FileTreePanel';

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
    <div style={{ height: '100%', background: '#1a1a1a', color: '#fff', padding: 12 }}>
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
    <div style={{ height: '100vh', background: '#0f0f0f' }}>
      <DockviewReact components={components} onReady={onReady} className="dockview-theme-dark" />
    </div>
  );
}
