import { useState } from 'react';
import { tokens } from '../../shared/design-tokens';

interface FileNode {
  path: string;
  status: 'added' | 'modified' | 'deleted';
}

const DEMO_FILES: FileNode[] = [
  { path: 'src/components/Header.tsx', status: 'modified' },
  { path: 'src/utils/api.ts', status: 'modified' },
  { path: 'src/pages/Home.tsx', status: 'added' },
];

const statusColors: Record<string, string> = { added: tokens.cta, modified: tokens.warning, deleted: tokens.error };

export function FileTreePanel() {
  const [selected, setSelected] = useState<string | null>(null);
  const [hovered, setHovered] = useState<string | null>(null);

  return (
    <div role="tree" aria-label="Changed files">
      <h3 style={{ fontSize: 14, fontWeight: 600, marginBottom: 8, color: tokens.muted, fontFamily: tokens.fontHeading }}>Changed Files</h3>
      {DEMO_FILES.map(f => (
        <div
          key={f.path}
          role="treeitem"
          aria-selected={selected === f.path}
          onClick={() => setSelected(f.path)}
          onMouseEnter={() => setHovered(f.path)}
          onMouseLeave={() => setHovered(null)}
          style={{
            padding: '4px 8px', cursor: 'pointer', borderRadius: 4,
            fontSize: 13, color: tokens.text,
            background: selected === f.path ? tokens.surface : hovered === f.path ? tokens.border : 'transparent',
            display: 'flex', alignItems: 'center', gap: 6,
            transition: tokens.transition,
          }}
        >
          <span style={{ color: statusColors[f.status], fontSize: 10 }} aria-hidden="true">&#9679;</span>
          <span>{f.path}</span>
        </div>
      ))}
    </div>
  );
}
