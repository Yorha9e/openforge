import { tokens } from '../../shared/design-tokens';
import { useState, useCallback, useMemo } from 'react';

interface FileEntry {
  name: string;
  isDir: boolean;
  size?: number;
  permissions?: string;
  owner?: string;
  group?: string;
  modTime?: string;
  children?: FileEntry[];
}

interface FileTreeRendererProps {
  content: string;
  maxHeight?: number;
}

/**
 * Parses ls -la output into a tree structure.
 * Example input:
 * ```
 * total 12
 * drwxr-xr-x  3 user group 4096 Jan  1 00:00 .
 * -rw-r--r--  1 user group  123 Jan  1 00:00 file.txt
 * ```
 */
function parseLsOutput(content: string): FileEntry[] {
  const lines = content.split('\n').filter(line => line.trim());
  const entries: FileEntry[] = [];

  for (const line of lines) {
    // Skip "total" line
    if (line.startsWith('total ')) continue;

    // Parse ls -la format: permissions links owner group size date name
    const match = line.match(/^([d\-l][rwxsSt\-]{9})\s+\d+\s+(\S+)\s+(\S+)\s+(\d+)\s+(\w+\s+\d+\s+[\d:]+)\s+(.+)$/);
    if (match) {
      const [, permissions, owner, group, sizeStr, modTime, name] = match;
      // Skip . and .. entries
      if (name === '.' || name === '..') continue;

      entries.push({
        name,
        isDir: permissions[0] === 'd',
        size: parseInt(sizeStr, 10),
        permissions,
        owner,
        group,
        modTime,
      });
    }
  }

  return entries;
}

/**
 * Parses JSON format from list_dir tool.
 * Expected format: { entries: [{ name, is_dir, size, ... }] }
 */
function parseJsonOutput(content: string): FileEntry[] {
  try {
    const parsed = JSON.parse(content);
    if (Array.isArray(parsed)) {
      return parsed.map(item => ({
        name: item.name || item.filename || '',
        isDir: item.is_dir ?? item.isDir ?? item.type === 'directory',
        size: item.size,
      }));
    }
    if (parsed.entries && Array.isArray(parsed.entries)) {
      return parsed.entries.map((item: any) => ({
        name: item.name || item.filename || '',
        isDir: item.is_dir ?? item.isDir ?? item.type === 'directory',
        size: item.size,
      }));
    }
  } catch {
    // Not JSON, return empty
  }
  return [];
}

/**
 * Formats file size to human-readable string.
 */
function formatSize(bytes?: number): string {
  if (bytes === undefined || bytes === null) return '';
  if (bytes === 0) return '0 B';

  const units = ['B', 'KB', 'MB', 'GB'];
  const k = 1024;
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  const size = bytes / Math.pow(k, i);

  return `${size.toFixed(i === 0 ? 0 : 1)} ${units[i]}`;
}

/**
 * SVG icons for file tree
 */
const Icons = {
  folder: (
    <svg width="16" height="16" viewBox="0 0 16 16" fill="none" aria-hidden="true">
      <path d="M2 4C2 3.44772 2.44772 3 3 3H6L7.5 5H13C13.5523 5 14 5.44772 14 6V12C14 12.5523 13.5523 13 13 13H3C2.44772 13 2 12.5523 2 12V4Z" fill={tokens.cta} opacity={0.8} />
    </svg>
  ),
  folderOpen: (
    <svg width="16" height="16" viewBox="0 0 16 16" fill="none" aria-hidden="true">
      <path d="M2 4C2 3.44772 2.44772 3 3 3H6L7.5 5H13C13.5523 5 14 5.44772 14 6V7H7L5 13H3C2.44772 13 2 12.5523 2 12V4Z" fill={tokens.cta} />
    </svg>
  ),
  file: (
    <svg width="16" height="16" viewBox="0 0 16 16" fill="none" aria-hidden="true">
      <path d="M4 2C4 1.44772 4.44772 1 5 1H9L12 4V14C12 14.5523 11.5523 15 11 15H5C4.44772 15 4 14.5523 4 14V2Z" stroke={tokens.muted} strokeWidth="1.2" fill="none" />
      <path d="M9 1L12 4H10C9.44772 4 9 3.55228 9 3V1Z" fill={tokens.muted} />
    </svg>
  ),
  chevron: (expanded: boolean) => (
    <svg width="12" height="12" viewBox="0 0 12 12" fill="none" aria-hidden="true"
      style={{ transform: expanded ? 'rotate(90deg)' : undefined, transition: 'transform 150ms' }}>
      <path d="M4.5 2.5L8 6L4.5 9.5" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round" />
    </svg>
  ),
};

/**
 * A single file/directory entry in the tree.
 */
function TreeEntry({ entry, depth = 0 }: { entry: FileEntry; depth?: number }) {
  const [expanded, setExpanded] = useState(false);

  const handleClick = useCallback(() => {
    if (entry.isDir) {
      setExpanded(prev => !prev);
    }
  }, [entry.isDir]);

  const handleKeyDown = useCallback((e: React.KeyboardEvent) => {
    if (e.key === 'Enter' || e.key === ' ') {
      e.preventDefault();
      handleClick();
    }
  }, [handleClick]);

  return (
    <div role={entry.isDir ? 'treeitem' : 'none'} aria-expanded={entry.isDir ? expanded : undefined}>
      <div
        onClick={handleClick}
        onKeyDown={handleKeyDown}
        tabIndex={0}
        role={entry.isDir ? 'button' : undefined}
        aria-label={entry.isDir ? `${expanded ? 'Collapse' : 'Expand'} ${entry.name}` : entry.name}
        style={{
          display: 'flex',
          alignItems: 'center',
          gap: 6,
          padding: '4px 8px',
          paddingLeft: `${depth * 20 + 8}px`,
          cursor: entry.isDir ? 'pointer' : 'default',
          borderRadius: 4,
          minHeight: 32,
          transition: 'background 150ms',
          background: 'transparent',
          border: 'none',
          width: '100%',
          textAlign: 'left',
          color: 'inherit',
          font: 'inherit',
        }}
        onMouseEnter={e => {
          (e.currentTarget as HTMLElement).style.background = `${tokens.border}40`;
        }}
        onMouseLeave={e => {
          (e.currentTarget as HTMLElement).style.background = 'transparent';
        }}
      >
        {entry.isDir && (
          <span style={{ width: 12, display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
            {Icons.chevron(expanded)}
          </span>
        )}
        {!entry.isDir && <span style={{ width: 12 }} />}

        <span style={{ width: 16, display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
          {entry.isDir ? (expanded ? Icons.folderOpen : Icons.folder) : Icons.file}
        </span>

        <span style={{
          flex: 1,
          overflow: 'hidden',
          textOverflow: 'ellipsis',
          whiteSpace: 'nowrap',
          color: entry.isDir ? tokens.cta : tokens.text,
          fontWeight: entry.isDir ? 500 : 400,
        }}>
          {entry.name}
        </span>

        {entry.size !== undefined && (
          <span style={{ color: tokens.muted, fontSize: 11, marginLeft: 8, flexShrink: 0 }}>
            {formatSize(entry.size)}
          </span>
        )}
      </div>

      {entry.isDir && expanded && entry.children && entry.children.length > 0 && (
        <div role="group">
          {entry.children.map((child, i) => (
            <TreeEntry key={`${child.name}-${i}`} entry={child} depth={depth + 1} />
          ))}
        </div>
      )}
    </div>
  );
}

/**
 * FileTreeRenderer - Renders ls output as an interactive file tree.
 *
 * Supports two input formats:
 * 1. Plain text ls -la output
 * 2. JSON format from list_dir tool
 */
export function FileTreeRenderer({ content, maxHeight = 300 }: FileTreeRendererProps) {
  const entries = useMemo(() => {
    // Try JSON first
    const jsonEntries = parseJsonOutput(content);
    if (jsonEntries.length > 0) return jsonEntries;

    // Fall back to plain text parsing
    return parseLsOutput(content);
  }, [content]);

  if (entries.length === 0) {
    return (
      <pre style={{
        background: tokens.bg,
        padding: 8,
        borderRadius: 4,
        color: tokens.muted,
        fontSize: 12,
        margin: 0,
        maxHeight,
        overflow: 'auto',
      }}>
        {content}
      </pre>
    );
  }

  // Sort: directories first, then files alphabetically
  const sorted = [...entries].sort((a, b) => {
    if (a.isDir && !b.isDir) return -1;
    if (!a.isDir && b.isDir) return 1;
    return a.name.localeCompare(b.name);
  });

  return (
    <div
      role="tree"
      aria-label="File tree"
      style={{
        background: tokens.bg,
        borderRadius: 6,
        maxHeight,
        overflow: 'auto',
        padding: '4px 0',
        border: `1px solid ${tokens.border}`,
      }}
    >
      {sorted.map((entry, i) => (
        <TreeEntry key={`${entry.name}-${i}`} entry={entry} />
      ))}
    </div>
  );
}