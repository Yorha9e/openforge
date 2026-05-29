import { useEffect, useState, useRef } from 'react';
import { useParams, Link } from 'react-router-dom';
import { DockviewReact, DockviewReadyEvent, IDockviewPanelProps, DockviewApi } from 'dockview';
import { ChatPanel } from '../chat/ChatPanel';
import { DiffPanel } from './DiffPanel';
import { FileTreePanel, type ChangedFile } from './FileTreePanel';
import { GatePanel } from './GatePanel';
import { TerminalPanel } from './TerminalPanel';
import { TopologyPanel } from './TopologyPanel';
import { FlowchartPanel } from './FlowchartPanel';
import { TestReportPanel } from './TestReportPanel';
import { CommentPanel } from './CommentPanel';
import { ProModeProvider, useProMode } from './ProModeContext';
import { useLayoutPersistence } from './useLayoutPersistence';
import { api } from '../../shared/api';
import { tokens } from '../../shared/design-tokens';
import { 
  TerminalIcon, 
  TopologyIcon, 
  FlowchartIcon, 
  TestReportIcon, 
  CommentsIcon, 
  CodeReviewIcon,
  DebugIcon,
  PMReviewIcon,
  SaveIcon
} from '../../shared/icons';

interface PipelineData {
  id: string;
  title: string;
  status: string;
  changed_files?: ChangedFile[];
  current_stage?: string;
}

// Panel registry with default configurations
const PANEL_REGISTRY: Record<string, { component: string; title: string }> = {
  files: { component: 'filetree', title: 'Files' },
  chat: { component: 'chat', title: 'AI Chat' },
  diff: { component: 'diff', title: 'Diff View' },
  gate: { component: 'gate', title: 'Gate' },
  terminal: { component: 'terminal', title: 'Terminal' },
  topology: { component: 'topology', title: 'Topology' },
  flowchart: { component: 'flowchart', title: 'Flowchart' },
  testreport: { component: 'testreport', title: 'Test Report' },
  comments: { component: 'comments', title: 'Comments' },
};

// Preset layouts
const PRESETS: Record<string, { panels: string[]; description: string }> = {
  'code-review': {
    panels: ['files', 'chat', 'diff', 'gate', 'comments', 'flowchart'],
    description: '🔍 Code Review - 三列: Files(左) | Chat+Gate(中) | Diff(右,flex=2)',
  },
  'debug': {
    panels: ['files', 'chat', 'terminal', 'topology', 'testreport'],
    description: '🐛 Debug - 三列: Files(左) | Chat(中) | Terminal(右,flex=2)',
  },
  'pm-review': {
    panels: ['chat', 'flowchart', 'testreport', 'comments'],
    description: '📊 PM Review - 两行: Flowchart(上,全宽) | TestReport+Comments(左下) | Chat(右下)',
  },
};

export default function ProModePage() {
  const { pid } = useParams<{ pid: string }>();
  const [pipeline, setPipeline] = useState<PipelineData | null>(null);
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

  if (error) {
    return <div style={{ padding: 40, color: tokens.error, fontFamily: tokens.fontBody }}>Failed to load pipeline: {error}</div>;
  }

  return (
    <ProModeProvider pipelineId={pid || ''}>
      <div style={{ height: '100vh', background: tokens.bg }}>
        {pipeline && (
          <div style={{ padding: '8px 16px', background: tokens.surface, borderBottom: `1px solid ${tokens.border}`, display: 'flex', gap: 16, alignItems: 'center', fontSize: 13, color: tokens.muted }}>
            <Link to="/" style={{ color: tokens.muted, textDecoration: 'none', fontSize: 13 }}>&larr; Dashboard</Link>
            <span style={{ color: tokens.text, fontWeight: 600, fontFamily: tokens.fontHeading }}>{pipeline.title}</span>
            <span>#{pipeline.id}</span>
            <span style={{ padding: '2px 8px', borderRadius: 8, background: `${tokens.cta}20`, color: tokens.cta, fontSize: 11 }}>{pipeline.status}</span>
            <span>Stage: {pipeline.current_stage}</span>
            <div style={{ flex: 1 }} />
            <DirtyIndicator />
          </div>
        )}
        <div style={{ height: pipeline ? 'calc(100vh - 41px)' : '100vh' }}>
          <ProModeContent pipeline={pipeline} />
        </div>
      </div>
    </ProModeProvider>
  );
}

function DirtyIndicator() {
  const { isDirty } = useProMode();
  if (!isDirty) return null;
  return (
    <span style={{ 
      padding: '2px 8px', 
      borderRadius: 8, 
      background: `${tokens.warning}20`, 
      color: tokens.warning, 
      fontSize: 11,
      display: 'flex',
      alignItems: 'center',
      gap: 4,
    }}>
      ● Unsaved changes
    </span>
  );
}

function DiffPanelWrapper() {
  const { selectedFile, selectedFileStatus, fileContent, fileContentLoading } = useProMode();
  return (
    <DiffPanel 
      fileName={selectedFile ?? undefined} 
      fileStatus={selectedFileStatus}
      modified={fileContent?.content}
      language={fileContent?.language}
      viewMode={fileContent ? 'single' : 'diff'}
      loading={fileContentLoading}
    />
  );
}

function ProModeContent({ pipeline }: { pipeline: PipelineData | null }) {
  const { pipelineId, selectedFile, selectFile, files, updateFiles, setDirty, isDirty } = useProMode();
  const [activePanels, setActivePanels] = useState<Set<string>>(new Set(['chat', 'diff', 'files', 'gate']));
  const [activePreset, setActivePreset] = useState<string>('code-review');
  const dockviewApiRef = useRef<DockviewApi | null>(null);
  const { saveLayout, loadLayout, markDirty, resetLayout, isDirty: layoutIsDirty } = useLayoutPersistence(pipeline?.id || 'default');

  // Sync dirty state from layout persistence to context
  useEffect(() => {
    setDirty(layoutIsDirty);
  }, [layoutIsDirty, setDirty]);

  useEffect(() => {
    if (pipeline?.changed_files) {
      updateFiles(pipeline.changed_files);
    }
  }, [pipeline?.changed_files, updateFiles]);

  const onReady = (event: DockviewReadyEvent) => {
    const dapi = event.api;
    dockviewApiRef.current = dapi;

    // Try to load saved layout
    const savedLayout = loadLayout();
    if (savedLayout) {
      try {
        dapi.fromJSON(savedLayout);
        return;
      } catch (error) {
        console.error('Failed to restore layout:', error);
        resetLayout();
      }
    }

    // Default layout: Code Review preset
    applyPreset('code-review', dapi);
  };

  const applyPreset = (presetName: string, dapi?: DockviewApi) => {
    const api = dapi || dockviewApiRef.current;
    if (!api) return;

    const preset = PRESETS[presetName];
    if (!preset) return;

    // Check if current layout is dirty
    if (layoutIsDirty) {
      const ok = window.confirm('布局已修改，是否保存？');
      if (ok) {
        saveLayout(api.toJSON());
      }
    }

    // Clear all panels
    api.clear();

    // Add panels based on preset
    preset.panels.forEach((panelId, index) => {
      const panelConfig = PANEL_REGISTRY[panelId];
      if (!panelConfig) return;

      const position = index === 0 ? undefined : { direction: 'right' as const };
      api.addPanel({
        id: panelId,
        component: panelConfig.component,
        title: panelConfig.title,
        position,
      });
    });

    // Update active panels
    setActivePanels(new Set(preset.panels));
    setActivePreset(presetName);
  };

  const handleTogglePanel = (panelId: string) => {
    const api = dockviewApiRef.current;
    if (!api) return;

    setActivePanels(prev => {
      const next = new Set(prev);
      if (next.has(panelId)) {
        next.delete(panelId);
        // Remove panel
        try {
          const panel = api.getPanel(panelId);
          if (panel) {
            api.removePanel(panel);
          }
        } catch (error) {
          console.error('Failed to remove panel:', error);
        }
      } else {
        next.add(panelId);
        // Add panel
        const panelConfig = PANEL_REGISTRY[panelId];
        if (panelConfig) {
          api.addPanel({
            id: panelId,
            component: panelConfig.component,
            title: panelConfig.title,
            position: { direction: 'right' },
          });
        }
      }
      markDirty();
      return next;
    });
  };

  const handlePresetChange = (presetName: string) => {
    applyPreset(presetName);
  };

  const handleSaveLayout = () => {
    const api = dockviewApiRef.current;
    if (api) {
      saveLayout(api.toJSON());
    }
  };

  const components = {
    chat: (_props: IDockviewPanelProps) => (
      <div style={{ height: '100%', overflow: 'hidden' }}>
        <ChatPanel embedded pipelineId={pipelineId} />
      </div>
    ),
    diff: (_props: IDockviewPanelProps) => (
      <div style={{ height: '100%' }}>
        <DiffPanelWrapper />
      </div>
    ),
    filetree: (_props: IDockviewPanelProps) => (
      <div style={{ height: '100%', background: tokens.surface, color: tokens.text, padding: 12 }}>
        <FileTreePanel
          files={files}
          workDir={localStorage.getItem('openforge_work_dir') || undefined}
          onSelectFile={f => selectFile(f.path)}
          selectedFile={selectedFile ?? undefined}
        />
      </div>
    ),
    gate: (_props: IDockviewPanelProps) => (
      <div style={{ height: '100%', background: tokens.surface, color: tokens.text }}>
        <GatePanel pipelineId={pipeline?.id || ''} stage={pipeline?.current_stage || 'clarify'} />
      </div>
    ),
    terminal: (_props: IDockviewPanelProps) => (
      <div style={{ height: '100%' }}>
        <TerminalPanel />
      </div>
    ),
    topology: (_props: IDockviewPanelProps) => (
      <div style={{ height: '100%' }}>
        <TopologyPanel />
      </div>
    ),
    flowchart: (_props: IDockviewPanelProps) => (
      <div style={{ height: '100%' }}>
        <FlowchartPanel />
      </div>
    ),
    testreport: (_props: IDockviewPanelProps) => (
      <div style={{ height: '100%' }}>
        <TestReportPanel />
      </div>
    ),
    comments: (_props: IDockviewPanelProps) => (
      <div style={{ height: '100%' }}>
        <CommentPanel />
      </div>
    ),
  };

  return (
    <div style={{ height: '100%', display: 'flex', flexDirection: 'column' }}>
      <div style={{ flex: 1, overflow: 'hidden' }}>
        <DockviewReact 
          components={components} 
          onReady={onReady} 
          className="dockview-theme-dark"
        />
      </div>
      <PanelMenu
        activePanels={activePanels}
        onTogglePanel={handleTogglePanel}
        activePreset={activePreset}
        onPresetChange={handlePresetChange}
        onSave={handleSaveLayout}
        isDirty={isDirty}
      />
    </div>
  );
}

function PanelMenu({ activePanels, onTogglePanel, activePreset, onPresetChange, onSave, isDirty }: {
  activePanels: Set<string>;
  onTogglePanel: (id: string) => void;
  activePreset: string;
  onPresetChange: (preset: string) => void;
  onSave: () => void;
  isDirty: boolean;
}) {
  const panels = [
    { id: 'terminal', icon: <TerminalIcon size={14} />, label: 'Terminal' },
    { id: 'topology', icon: <TopologyIcon size={14} />, label: 'Topology' },
    { id: 'flowchart', icon: <FlowchartIcon size={14} />, label: 'Flowchart' },
    { id: 'testreport', icon: <TestReportIcon size={14} />, label: 'Test Report' },
    { id: 'comments', icon: <CommentsIcon size={14} />, label: 'Comments' },
  ];

  const presets = [
    { id: 'code-review', icon: <CodeReviewIcon size={14} />, label: 'Code Review' },
    { id: 'debug', icon: <DebugIcon size={14} />, label: 'Debug' },
    { id: 'pm-review', icon: <PMReviewIcon size={14} />, label: 'PM Review' },
  ];

  return (
    <div style={{
      height: 48,
      minHeight: 48,
      background: tokens.surface,
      borderTop: `1px solid ${tokens.border}`,
      display: 'flex',
      alignItems: 'center',
      padding: '0 16px',
      gap: 8,
      flexShrink: 0,
    }}>
      <div style={{ display: 'flex', gap: 8 }}>
        {panels.map(panel => (
          <button
            key={panel.id}
            onClick={() => onTogglePanel(panel.id)}
            style={{
              background: activePanels.has(panel.id) ? `${tokens.cta}20` : 'transparent',
              border: activePanels.has(panel.id) ? `1px solid ${tokens.cta}` : '1px solid transparent',
              borderRadius: 6,
              padding: '8px',
              cursor: 'pointer',
              opacity: activePanels.has(panel.id) ? 1 : 0.3,
              fontSize: 12,
              color: tokens.text,
              minWidth: 44,
              minHeight: 44,
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
            }}
            title={panel.label}
            aria-label={panel.label}
          >
            {panel.icon}
          </button>
        ))}
      </div>
      <div style={{ flex: 1 }} />
      <div style={{ display: 'flex', gap: 8, alignItems: 'center' }}>
        {presets.map(preset => (
          <button
            key={preset.id}
            onClick={() => onPresetChange(preset.id)}
            style={{
              background: activePreset === preset.id ? `${tokens.cta}20` : 'transparent',
              border: activePreset === preset.id ? `1px solid ${tokens.cta}` : '1px solid transparent',
              borderRadius: 6,
              padding: '8px 12px',
              cursor: 'pointer',
              fontSize: 12,
              color: tokens.text,
              minHeight: 44,
              display: 'flex',
              alignItems: 'center',
              gap: 6,
            }}
            aria-label={preset.label}
          >
            {preset.icon}
            <span>{preset.label}</span>
          </button>
        ))}
        {isDirty && (
          <button
            onClick={onSave}
            style={{
              background: `${tokens.warning}20`,
              border: `1px solid ${tokens.warning}`,
              borderRadius: 6,
              padding: '4px 8px',
              cursor: 'pointer',
              fontSize: 12,
              color: tokens.warning,
              marginLeft: 8,
              display: 'flex',
              alignItems: 'center',
              gap: 4,
            }}
          >
            <SaveIcon size={14} />
            Save
          </button>
        )}
      </div>
    </div>
  );
}