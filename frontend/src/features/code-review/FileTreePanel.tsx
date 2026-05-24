import { useState } from 'react';
import { tokens } from '../../shared/design-tokens';

export interface ChangedFile {
  path: string;
  status: 'added' | 'modified' | 'deleted';
}

interface FileTreePanelProps {
  files?: ChangedFile[];
  onSelectFile?: (file: ChangedFile) => void;
  selectedFile?: string;
}

const statusColors: Record<string, string> = { added: tokens.cta, modified: tokens.warning, deleted: tokens.error };

export function FileTreePanel({ files, onSelectFile, selectedFile }: FileTreePanelProps) {
  const [hovered, setHovered] = useState<string | null>(null);
  const displayFiles = files && files.length > 0 ? files : [{ path: 'No changed files yet', status: 'modified' as const }];

  return (
    <div role="tree" aria-label="Changed files">
      <h3 style={{ fontSize: 14, fontWeight: 600, marginBottom: 8, color: tokens.muted, fontFamily: tokens.fontHeading }}>
        Changed Files ({files?.length ?? 0})
      </h3>
      {displayFiles.map(f => (
        <div
          key={f.path}
          role="treeitem"
          aria-selected={selectedFile === f.path}
          onClick={() => onSelectFile?.(f)}
          onMouseEnter={() => setHovered(f.path)}
          onMouseLeave={() => setHovered(null)}
          style={{
            padding: '4px 8px', cursor: 'pointer', borderRadius: 4,
            fontSize: 13, color: tokens.text,
            background: selectedFile === f.path ? tokens.surface : hovered === f.path ? tokens.border : 'transparent',
            display: 'flex', alignItems: 'center', gap: 6,
            transition: tokens.transition,
          }}
        >
          <span style={{ color: statusColors[f.status], fontSize: 10 }} aria-hidden="true">{'●'}</span>
          <span style={{ fontFamily: "'Fira Code', monospace", overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>{f.path}</span>
        </div>
      ))}
    </div>
  );
}
