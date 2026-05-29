import { useState, useMemo, useEffect, useCallback } from 'react';
import { tokens } from '../../shared/design-tokens';
import { api } from '../../shared/api';

export interface ChangedFile {
  path: string;
  status: 'added' | 'modified' | 'deleted';
}

interface WorkDirFile {
  name: string;
  is_dir: boolean;
  size: number;
  path: string;
}

interface FileTreePanelProps {
  files?: ChangedFile[];
  workDir?: string;
  onSelectFile?: (file: ChangedFile | WorkDirFile) => void;
  selectedFile?: string;
}

const statusColors: Record<string, string> = { 
  added: tokens.cta, 
  modified: tokens.warning, 
  deleted: tokens.error 
};

const statusIcons: Record<string, string> = {
  added: 'A',
  modified: 'M',
  deleted: 'D',
};

interface TreeNode {
  name: string;
  path: string;
  type: 'file' | 'directory';
  status?: 'added' | 'modified' | 'deleted';
  size?: number;
  children: TreeNode[];
  file?: ChangedFile | WorkDirFile;
}

function buildTreeFromChangedFiles(files: ChangedFile[]): TreeNode {
  const root: TreeNode = {
    name: '',
    path: '',
    type: 'directory',
    children: [],
  };

  for (const file of files) {
    const parts = file.path.split('/').filter(Boolean);
    let current = root;

    for (let i = 0; i < parts.length; i++) {
      const part = parts[i] ?? '';
      const isFile = i === parts.length - 1;
      const currentPath = parts.slice(0, i + 1).join('/');

      if (isFile) {
        current.children.push({
          name: part,
          path: file.path,
          type: 'file',
          status: file.status,
          children: [],
          file,
        });
      } else {
        const existingDir = current.children.find(c => c.name === part && c.type === 'directory');
        if (existingDir) {
          current = existingDir;
        } else {
          const newDir: TreeNode = {
            name: part,
            path: currentPath,
            type: 'directory',
            children: [],
          };
          current.children.push(newDir);
          current = newDir;
        }
      }
    }
  }

  sortNodes(root.children);
  return root;
}

function buildTreeFromWorkDir(files: WorkDirFile[], basePath: string): TreeNode {
  const root: TreeNode = {
    name: basePath.split('/').pop() || basePath,
    path: basePath,
    type: 'directory',
    children: [],
  };

  // Normalize path separators for comparison
  const normalizePath = (p: string) => p.replace(/\\/g, '/').replace(/\/$/, '');
  const normalizedBasePath = normalizePath(basePath);

  // Create a map to track directory nodes by path
  const dirMap = new Map<string, TreeNode>();
  dirMap.set(normalizedBasePath, root);

  for (const file of files) {
    const normalizedFilePath = normalizePath(file.path);
    const relativePath = normalizedFilePath.replace(normalizedBasePath, '').replace(/^\//, '');
    
    if (!relativePath) continue; // Skip if it's the base path itself
    
    const parts = relativePath.split('/');
    let currentDir = root;
    let currentPath = normalizedBasePath;

    // Navigate/create directory hierarchy
    for (let i = 0; i < parts.length - 1; i++) {
      const part = parts[i];
      if (!part) continue;
      
      currentPath = `${currentPath}/${part}`;
      let childDir = currentDir.children.find(c => c.name === part && c.type === 'directory');
      
      if (!childDir) {
        childDir = {
          name: part,
          path: currentPath,
          type: 'directory',
          children: [],
        };
        currentDir.children.push(childDir);
        dirMap.set(currentPath, childDir);
      }
      currentDir = childDir;
    }

    // Add the file/directory to the current level
    const fileName = parts[parts.length - 1];
    if (fileName) {
      // Check if this file already exists (avoid duplicates)
      const existingIndex = currentDir.children.findIndex(c => c.name === fileName);
      if (existingIndex === -1) {
        currentDir.children.push({
          name: fileName,
          path: file.path,
          type: file.is_dir ? 'directory' : 'file',
          size: file.size,
          children: [],
          file,
        });
      }
    }
  }

  sortNodes(root.children);
  return root;
}

function sortNodes(nodes: TreeNode[]) {
  nodes.sort((a, b) => {
    if (a.type !== b.type) return a.type === 'directory' ? -1 : 1;
    return a.name.localeCompare(b.name);
  });
  nodes.forEach(n => sortNodes(n.children));
}

function TreeNodeComponent({ 
  node, 
  depth, 
  selectedFile, 
  onSelectFile, 
  expandedDirs, 
  onToggleDir,
  onLoadChildren,
  loadingDirs,
}: { 
  node: TreeNode; 
  depth: number; 
  selectedFile?: string;
  onSelectFile?: (file: ChangedFile | WorkDirFile) => void;
  expandedDirs: Set<string>;
  onToggleDir: (path: string) => void;
  onLoadChildren?: (path: string) => Promise<void>;
  loadingDirs: Set<string>;
}) {
  const isExpanded = expandedDirs.has(node.path);
  const isLoading = loadingDirs.has(node.path);
  const isSelected = node.type === 'file' && selectedFile === node.path;

  const handleClick = () => {
    console.log('[FileTree] Click:', { type: node.type, path: node.path, hasFile: !!node.file });
    if (node.type === 'directory') {
      onToggleDir(node.path);
      if (!isExpanded && node.children.length === 0 && onLoadChildren) {
        onLoadChildren(node.path);
      }
    } else if (node.file) {
      console.log('[FileTree] Selecting file:', node.file.path);
      onSelectFile?.(node.file);
    } else {
      console.warn('[FileTree] File node without file data:', node);
    }
  };

  if (node.type === 'directory') {
    return (
      <div>
        <div
          onClick={handleClick}
          style={{
            padding: '3px 8px',
            paddingLeft: `${8 + depth * 16}px`,
            cursor: 'pointer',
            fontSize: 13,
            color: tokens.muted,
            display: 'flex',
            alignItems: 'center',
            gap: 4,
            userSelect: 'none',
          }}
        >
          <span style={{ 
            fontSize: 10, 
            width: 12, 
            textAlign: 'center',
            transition: 'transform 150ms',
            transform: isExpanded ? 'rotate(90deg)' : 'rotate(0deg)',
          }}>
            {isLoading ? '...' : '▶'}
          </span>
          <span style={{ fontWeight: 500 }}>{node.name}</span>
        </div>
        {isExpanded && node.children.map(child => (
          <TreeNodeComponent
            key={child.path}
            node={child}
            depth={depth + 1}
            selectedFile={selectedFile}
            onSelectFile={onSelectFile}
            expandedDirs={expandedDirs}
            onToggleDir={onToggleDir}
            onLoadChildren={onLoadChildren}
            loadingDirs={loadingDirs}
          />
        ))}
      </div>
    );
  }

  return (
    <div
      onClick={handleClick}
      style={{
        padding: '3px 8px',
        paddingLeft: `${8 + depth * 16 + 16}px`,
        cursor: 'pointer',
        borderRadius: 4,
        fontSize: 13,
        color: tokens.text,
        background: isSelected ? tokens.surface : 'transparent',
        display: 'flex',
        alignItems: 'center',
        gap: 6,
        transition: tokens.transition,
      }}
      onMouseEnter={e => {
        if (!isSelected) e.currentTarget.style.background = `${tokens.border}40`;
      }}
      onMouseLeave={e => {
        if (!isSelected) e.currentTarget.style.background = 'transparent';
      }}
    >
      {node.status && (
        <span style={{ 
          fontSize: 9,
          fontWeight: 700,
          width: 14,
          height: 14,
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'center',
          borderRadius: 3,
          background: `${statusColors[node.status]}20`,
          color: statusColors[node.status],
        }}>
          {statusIcons[node.status]}
        </span>
      )}
      <span style={{ 
        fontFamily: "'Fira Code', monospace", 
        overflow: 'hidden', 
        textOverflow: 'ellipsis', 
        whiteSpace: 'nowrap',
        color: node.status === 'deleted' ? tokens.muted : tokens.text,
        textDecoration: node.status === 'deleted' ? 'line-through' : 'none',
        flex: 1,
      }}>
        {node.name}
      </span>
      {node.size !== undefined && (
        <span style={{ color: tokens.muted, fontSize: 11, marginLeft: 8, flexShrink: 0 }}>
          {formatSize(node.size)}
        </span>
      )}
    </div>
  );
}

function formatSize(bytes: number): string {
  if (bytes === 0) return '0 B';
  const units = ['B', 'KB', 'MB', 'GB'];
  const k = 1024;
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  const size = bytes / Math.pow(k, i);
  return `${size.toFixed(i === 0 ? 0 : 1)} ${units[i]}`;
}

export function FileTreePanel({ files, workDir, onSelectFile, selectedFile }: FileTreePanelProps) {
  const [expandedDirs, setExpandedDirs] = useState<Set<string>>(new Set());
  const [workDirFiles, setWorkDirFiles] = useState<WorkDirFile[]>([]);
  const [loading, setLoading] = useState(false);
  const [loadingDirs, setLoadingDirs] = useState<Set<string>>(new Set());
  const [error, setError] = useState<string | null>(null);

  // Load files from working directory
  useEffect(() => {
    if (!workDir) return;
    
    setLoading(true);
    setError(null);
    api.listFiles(workDir)
      .then(data => {
        setWorkDirFiles(data.files);
        // Auto-expand root
        setExpandedDirs(new Set([workDir]));
      })
      .catch(err => {
        setError(err instanceof Error ? err.message : 'Failed to load files');
      })
      .finally(() => {
        setLoading(false);
      });
  }, [workDir]);

  // Load children for a directory (lazy loading)
  const loadChildren = useCallback(async (dirPath: string) => {
    setLoadingDirs(prev => new Set(prev).add(dirPath));
    try {
      const data = await api.listFiles(dirPath);
      // Update the workDirFiles by adding new files
      setWorkDirFiles(prev => {
        // Create a set of existing paths for deduplication
        const existingPaths = new Set(prev.map(f => f.path));
        const newFiles = data.files.filter(f => !existingPaths.has(f.path));
        return [...prev, ...newFiles];
      });
    } catch (err) {
      console.error('Failed to load directory:', err);
    } finally {
      setLoadingDirs(prev => {
        const next = new Set(prev);
        next.delete(dirPath);
        return next;
      });
    }
  }, []);

  const tree = useMemo(() => {
    if (workDir && workDirFiles.length > 0) {
      const t = buildTreeFromWorkDir(workDirFiles, workDir);
      console.log('[FileTree] Built workDir tree:', { workDir, fileCount: workDirFiles.length, rootChildren: t.children.length });
      return t;
    }
    if (files && files.length > 0) {
      const t = buildTreeFromChangedFiles(files);
      console.log('[FileTree] Built changed files tree:', { fileCount: files.length, rootChildren: t.children.length });
      return t;
    }
    console.log('[FileTree] No data to build tree', { workDir, workDirFiles: workDirFiles.length, files: files?.length });
    return null;
  }, [files, workDir, workDirFiles]);

  // Auto-expand all directories on first render (for changed files mode)
  useMemo(() => {
    if (workDir || !tree) return;
    const allDirs = new Set<string>();
    const collectDirs = (node: TreeNode) => {
      if (node.type === 'directory') {
        allDirs.add(node.path);
        node.children.forEach(collectDirs);
      }
    };
    tree.children.forEach(collectDirs);
    setExpandedDirs(allDirs);
  }, [tree, workDir]);

  const handleToggleDir = (path: string) => {
    setExpandedDirs(prev => {
      const next = new Set(prev);
      if (next.has(path)) {
        next.delete(path);
      } else {
        next.add(path);
      }
      return next;
    });
  };

  // Loading state
  if (loading) {
    return (
      <div role="tree" aria-label="File tree">
        <h3 style={{ fontSize: 14, fontWeight: 600, marginBottom: 8, color: tokens.muted, fontFamily: tokens.fontHeading }}>
          {workDir ? 'Working Directory' : 'Changed Files'}
        </h3>
        <div style={{ padding: '12px 8px', fontSize: 13, color: tokens.muted, fontStyle: 'italic' }}>
          Loading files...
        </div>
      </div>
    );
  }

  // Error state
  if (error) {
    return (
      <div role="tree" aria-label="File tree">
        <h3 style={{ fontSize: 14, fontWeight: 600, marginBottom: 8, color: tokens.muted, fontFamily: tokens.fontHeading }}>
          {workDir ? 'Working Directory' : 'Changed Files'}
        </h3>
        <div style={{ padding: '12px 8px', fontSize: 13, color: tokens.error }}>
          {error}
        </div>
      </div>
    );
  }

  // Empty state
  if (!tree || tree.children.length === 0) {
    return (
      <div role="tree" aria-label="File tree">
        <h3 style={{ fontSize: 14, fontWeight: 600, marginBottom: 8, color: tokens.muted, fontFamily: tokens.fontHeading }}>
          {workDir ? 'Working Directory' : 'Changed Files'} (0)
        </h3>
        <div style={{ padding: '12px 8px', fontSize: 13, color: tokens.muted, fontStyle: 'italic' }}>
          {workDir ? 'No files in working directory' : 'No changed files yet'}
        </div>
      </div>
    );
  }

  const totalFiles = workDir ? workDirFiles.length : (files?.length || 0);

  return (
    <div role="tree" aria-label="File tree">
      <h3 style={{ 
        fontSize: 14, 
        fontWeight: 600, 
        marginBottom: 8, 
        color: tokens.muted, 
        fontFamily: tokens.fontHeading,
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'space-between',
      }}>
        <span>{workDir ? 'Working Directory' : 'Changed Files'}</span>
        <span style={{ 
          fontSize: 11, 
          fontWeight: 400, 
          padding: '1px 6px', 
          borderRadius: 10,
          background: `${tokens.cta}20`,
          color: tokens.cta,
        }}>
          {totalFiles}
        </span>
      </h3>
      <div style={{ maxHeight: 'calc(100% - 40px)', overflowY: 'auto' }}>
        {workDir ? (
          // Working directory mode: show root with children
          tree.children.map(node => (
            <TreeNodeComponent
              key={node.path}
              node={node}
              depth={0}
              selectedFile={selectedFile}
              onSelectFile={onSelectFile}
              expandedDirs={expandedDirs}
              onToggleDir={handleToggleDir}
              onLoadChildren={loadChildren}
              loadingDirs={loadingDirs}
            />
          ))
        ) : (
          // Changed files mode: show flat list
          tree.children.map(node => (
            <TreeNodeComponent
              key={node.path}
              node={node}
              depth={0}
              selectedFile={selectedFile}
              onSelectFile={onSelectFile}
              expandedDirs={expandedDirs}
              onToggleDir={handleToggleDir}
              loadingDirs={loadingDirs}
            />
          ))
        )}
      </div>
    </div>
  );
}