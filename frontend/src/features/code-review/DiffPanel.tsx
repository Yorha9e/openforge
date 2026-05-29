import { DiffEditor } from '@monaco-editor/react';
import { tokens } from '../../shared/design-tokens';

interface DiffPanelProps {
  original?: string;
  modified?: string;
  language?: string;
  fileName?: string;
  fileStatus?: 'added' | 'modified' | 'deleted';
  viewMode?: 'diff' | 'single'; // 'single' for viewing single file content
  loading?: boolean;
}

export function DiffPanel({ original, modified, language = 'typescript', fileName, fileStatus, viewMode = 'diff', loading = false }: DiffPanelProps) {
  // Determine language from file extension
  const getLanguage = (path: string): string => {
    const ext = path.split('.').pop()?.toLowerCase();
    const langMap: Record<string, string> = {
      ts: 'typescript', tsx: 'typescript', js: 'javascript', jsx: 'javascript',
      go: 'go', py: 'python', rs: 'rust', java: 'java', json: 'json',
      yaml: 'yaml', yml: 'yaml', md: 'markdown', sql: 'sql',
      html: 'html', css: 'css', scss: 'scss',
    };
    return langMap[ext || ''] || 'typescript';
  };

  const detectedLanguage = fileName ? getLanguage(fileName) : language;

  // Determine if we should use single file mode
  const isSingleMode = viewMode === 'single' || (!original && modified);

  // Generate appropriate content based on file status
  const getOriginalContent = (): string => {
    if (loading) return '// Loading...';
    if (isSingleMode) return ''; // In single mode, original is empty
    if (original) return original;
    if (!fileName) return '// Select a file from the tree to view diff';
    if (fileStatus === 'added') return '// New file - no previous version';
    return `// Original version of ${fileName}\n// Loading...`;
  };

  const getModifiedContent = (): string => {
    if (loading) return '// Loading...';
    if (isSingleMode) return modified || '// No content loaded';
    if (modified) return modified;
    if (!fileName) return '// No changes yet\n// Agent will generate changes here';
    if (fileStatus === 'deleted') return '// File deleted';
    return `// Modified version of ${fileName}\n// Loading...`;
  };

  return (
    <div style={{ height: '100%', display: 'flex', flexDirection: 'column' }}>
      {fileName && (
        <div style={{ 
          padding: '6px 12px', 
          fontSize: 12, 
          color: tokens.muted, 
          borderBottom: `1px solid ${tokens.border}`, 
          fontFamily: "'Fira Code', monospace",
          display: 'flex',
          alignItems: 'center',
          gap: 8
        }}>
          {!isSingleMode && (
            <span style={{ 
              color: fileStatus === 'added' ? tokens.cta : 
                     fileStatus === 'deleted' ? tokens.error : 
                     tokens.warning,
              fontSize: 10
            }}>
              {'●'}
            </span>
          )}
          <span>{fileName}</span>
          {!isSingleMode && fileStatus && (
            <span style={{ 
              fontSize: 10, 
              padding: '1px 6px', 
              borderRadius: 4,
              background: `${fileStatus === 'added' ? tokens.cta : 
                           fileStatus === 'deleted' ? tokens.error : 
                           tokens.warning}20`,
              color: fileStatus === 'added' ? tokens.cta : 
                     fileStatus === 'deleted' ? tokens.error : 
                     tokens.warning
            }}>
              {fileStatus}
            </span>
          )}
        </div>
      )}
      <div style={{ flex: 1 }}>
        <DiffEditor
          key={`${fileName || 'empty'}-${detectedLanguage}-${isSingleMode ? 'single' : 'diff'}`}
          height="100%"
          original={getOriginalContent()}
          modified={getModifiedContent()}
          language={detectedLanguage}
          theme="vs-dark"
          options={{ 
            readOnly: true, 
            minimap: { enabled: false },
            renderSideBySide: !isSingleMode, // Inline mode for single file
            enableSplitViewResizing: !isSingleMode,
          }}
        />
      </div>
    </div>
  );
}
