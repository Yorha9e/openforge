import { DiffEditor } from '@monaco-editor/react';

interface DiffPanelProps {
  original?: string;
  modified?: string;
  language?: string;
  fileName?: string;
}

export function DiffPanel({ original, modified, language = 'typescript', fileName }: DiffPanelProps) {
  const fallbackOriginal = original ?? '// No original version\n// Select a file from the tree to view diff';
  const fallbackModified = modified ?? '// No changes yet\n// Agent will generate changes here';

  return (
    <div style={{ height: '100%', display: 'flex', flexDirection: 'column' }}>
      {fileName && (
        <div style={{ padding: '6px 12px', fontSize: 12, color: '#94a3b8', borderBottom: '1px solid #334155', fontFamily: "'Fira Code', monospace" }}>
          {fileName}
        </div>
      )}
      <div style={{ flex: 1 }}>
        <DiffEditor
          height="100%"
          original={fallbackOriginal}
          modified={fallbackModified}
          language={language}
          theme="vs-dark"
          options={{ readOnly: true, minimap: { enabled: false } }}
        />
      </div>
    </div>
  );
}
