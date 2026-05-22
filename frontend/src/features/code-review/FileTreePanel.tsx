import { useState } from 'react';

interface FileNode {
  path: string;
  status: 'added' | 'modified' | 'deleted';
}

const DEMO_FILES: FileNode[] = [
  { path: 'src/components/Header.tsx', status: 'modified' },
  { path: 'src/utils/api.ts', status: 'modified' },
  { path: 'src/pages/Home.tsx', status: 'added' },
];

const statusColors: Record<string, string> = { added: '#4ade80', modified: '#facc15', deleted: '#f87171' };

export function FileTreePanel() {
  const [selected, setSelected] = useState<string | null>(null);

  return (
    <div>
      <h3 style={{ fontSize: 14, fontWeight: 600, marginBottom: 8, color: '#a3a3a3' }}>Changed Files</h3>
      {DEMO_FILES.map(f => (
        <div
          key={f.path}
          onClick={() => setSelected(f.path)}
          style={{
            padding: '4px 8px', cursor: 'pointer', borderRadius: 4,
            fontSize: 13, color: '#e5e5e5',
            background: selected === f.path ? '#262626' : 'transparent',
            display: 'flex', alignItems: 'center', gap: 6,
          }}
        >
          <span style={{ color: statusColors[f.status], fontSize: 10 }}>●</span>
          <span>{f.path}</span>
        </div>
      ))}
    </div>
  );
}
