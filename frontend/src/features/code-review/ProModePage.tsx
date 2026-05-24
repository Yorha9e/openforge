import { useEffect, useState } from 'react';
import { useParams } from 'react-router-dom';
import { DockviewReact, DockviewReadyEvent, IDockviewPanelProps } from 'dockview';
import { ChatPanel } from '../chat/ChatPanel';
import { DiffPanel } from './DiffPanel';
import { FileTreePanel, type ChangedFile } from './FileTreePanel';
import { api } from '../../shared/api';
import { tokens } from '../../shared/design-tokens';

interface PipelineData {
  id: string;
  title: string;
  status: string;
  changed_files?: ChangedFile[];
  current_stage?: string;
}

export function ProModePage() {
  const { pid } = useParams<{ pid: string }>();
  const [pipeline, setPipeline] = useState<PipelineData | null>(null);
  const [selectedFile, setSelectedFile] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (!pid) return;
    api.getPipeline(pid)
      .then((data: any) => setPipeline({
        id: data.id, title: data.title, status: data.status,
        changed_files: data.changed_files || [],
        current_stage: data.current_stage || 'clarify',
      }))
      .catch(err => setError(err instanceof Error ? err.message : 'Failed'));
  }, [pid]);

  const onReady = (event: DockviewReadyEvent) => {
    const dapi = event.api;
    dapi.addPanel({ id: 'chat', component: 'chat', title: 'AI Chat' });
    dapi.addPanel({ id: 'diff', component: 'diff', title: 'Diff View', position: { direction: 'right' } });
    dapi.addPanel({ id: 'files', component: 'filetree', title: 'Files', position: { direction: 'left' } });
  };

  const components = {
    chat: (_props: IDockviewPanelProps) => (
      <div style={{ height: '100%', overflow: 'hidden' }}>
        <ChatPanel />
      </div>
    ),
    diff: (_props: IDockviewPanelProps) => (
      <div style={{ height: '100%' }}>
        <DiffPanel fileName={selectedFile ?? undefined} />
      </div>
    ),
    filetree: (_props: IDockviewPanelProps) => (
      <div style={{ height: '100%', background: tokens.surface, color: tokens.text, padding: 12 }}>
        <FileTreePanel
          files={pipeline?.changed_files}
          onSelectFile={f => setSelectedFile(f.path)}
          selectedFile={selectedFile ?? undefined}
        />
      </div>
    ),
  };

  if (error) {
    return <div style={{ padding: 40, color: tokens.error, fontFamily: tokens.fontBody }}>Failed to load pipeline: {error}</div>;
  }

  return (
    <div style={{ height: '100vh', background: tokens.bg }}>
      {pipeline && (
        <div style={{ padding: '8px 16px', background: tokens.surface, borderBottom: `1px solid ${tokens.border}`, display: 'flex', gap: 16, alignItems: 'center', fontSize: 13, color: tokens.muted }}>
          <span style={{ color: tokens.text, fontWeight: 600, fontFamily: tokens.fontHeading }}>{pipeline.title}</span>
          <span>#{pipeline.id}</span>
          <span style={{ padding: '2px 8px', borderRadius: 8, background: `${tokens.cta}20`, color: tokens.cta, fontSize: 11 }}>{pipeline.status}</span>
          <span>Stage: {pipeline.current_stage}</span>
        </div>
      )}
      <div style={{ height: pipeline ? 'calc(100vh - 41px)' : '100vh' }}>
        <DockviewReact components={components} onReady={onReady} className="dockview-theme-dark" />
      </div>
    </div>
  );
}
