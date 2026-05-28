import { useMemo } from 'react';
import { tokens } from '../../shared/design-tokens';

export interface SkillEntry {
  name: string;
  version: string;
  source: string;
  stages: string[];
  keywords: string[];
  base_priority: number;
  current_priority: number;
  enabled: boolean;
  deprecated: boolean;
  is_latest: boolean;
  prompt_preview: string;
  workflow_steps: number;
}

interface SkillTableProps {
  skills: SkillEntry[];
  onSelect: (skill: SkillEntry) => void;
  selectedName?: string;
  sortBy?: string;
  sortDir?: 'asc' | 'desc';
  onSort?: (col: string) => void;
  page?: number;
  pageSize?: number;
  total?: number;
  onPageChange?: (page: number) => void;
  priorityEdits: Record<string, number>;
}

const sourceIcons: Record<string, string> = {
  global: '\u{1F310}',
  team: '\u{1F465}',
  project: '\u{1F4C1}',
};

const sortArrow = (col: string, sortBy?: string, sortDir?: 'asc' | 'desc') => {
  if (col !== sortBy) return '';
  return sortDir === 'asc' ? ' \u25B2' : ' \u25BC';
};

function statusStyle(skill: SkillEntry) {
  if (skill.deprecated) return { color: tokens.error, label: 'Deprecated' };
  if (!skill.enabled) return { color: tokens.warning, label: 'Disabled' };
  return { color: tokens.cta, label: 'Active' };
}

export function SkillTable({
  skills,
  onSelect,
  selectedName,
  sortBy,
  sortDir,
  onSort,
  page = 1,
  pageSize = 20,
  total,
  onPageChange,
  priorityEdits,
}: SkillTableProps) {
  const totalPages = Math.max(1, Math.ceil((total ?? skills.length) / pageSize));

  const cols = [
    { key: 'name', label: 'Skill' },
    { key: 'source', label: 'Source' },
    { key: 'current_priority', label: 'Priority' },
    { key: 'stages', label: 'Stages' },
    { key: 'deprecated', label: 'Status' },
  ];

  return (
    <div>
      <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: 12 }}>
        <thead>
          <tr>
            {cols.map(col => (
              <th
                key={col.key}
                scope="col"
                aria-sort={sortBy === col.key ? (sortDir === 'asc' ? 'ascending' : 'descending') : undefined}
                onClick={() => onSort?.(col.key)}
                style={{
                  textAlign: 'left',
                  padding: '8px 10px',
                  fontSize: 10,
                  fontWeight: 600,
                  color: tokens.muted,
                  textTransform: 'uppercase',
                  letterSpacing: '0.04em',
                  borderBottom: `2px solid ${tokens.border}`,
                  cursor: 'pointer',
                  userSelect: 'none',
                  whiteSpace: 'nowrap',
                }}
              >
                {col.label}{sortArrow(col.key, sortBy, sortDir)}
              </th>
            ))}
          </tr>
        </thead>
        <tbody>
          {skills.map(skill => {
            const status = statusStyle(skill);
            const isSelected = selectedName === skill.name;
            return (
              <tr
                key={`${skill.name}@${skill.version}`}
                onClick={() => onSelect(skill)}
                tabIndex={0}
                onKeyDown={e => { if (e.key === 'Enter') onSelect(skill); }}
                aria-selected={isSelected}
                style={{
                  borderBottom: `1px solid rgba(45, 58, 79, 0.4)`,
                  cursor: 'pointer',
                  background: isSelected ? 'rgba(34, 197, 94, 0.06)' : undefined,
                  borderLeft: isSelected ? `3px solid ${tokens.cta}` : '3px solid transparent',
                  outline: 'none',
                  transition: 'background 100ms',
                }}
                onMouseEnter={e => { if (!isSelected) e.currentTarget.style.background = '#1F2B3D'; }}
                onMouseLeave={e => { if (!isSelected) e.currentTarget.style.background = ''; }}
              >
                <td style={{ padding: '8px 10px' }}>
                  <div style={{ fontWeight: 600 }}>{skill.name}</div>
                  <div style={{ fontSize: 10, color: tokens.muted }}>v{skill.version}</div>
                </td>
                <td style={{ padding: '8px 10px' }}>
                  <span style={{
                    display: 'inline-flex',
                    alignItems: 'center',
                    gap: 3,
                    fontSize: 10,
                    padding: '1px 6px',
                    borderRadius: 3,
                    background: tokens.border,
                    color: '#8C9AB3',
                  }}>
                    {sourceIcons[skill.source] || '\u{1F4C1}'} {skill.source}
                  </span>
                </td>
                <td style={{ padding: '8px 10px' }}>
                  <strong style={{ fontSize: 13 }}>{priorityEdits[skill.name] ?? skill.current_priority}</strong>
                  <div style={{ height: 4, width: 50, background: tokens.border, borderRadius: 2, marginTop: 3, overflow: 'hidden' }}>
                    <div style={{
                      height: '100%',
                      width: `${Math.min((priorityEdits[skill.name] ?? skill.current_priority), 100)}%`,
                      background: `linear-gradient(90deg, ${tokens.cta}, #10B981)`,
                      borderRadius: 2,
                    }} />
                  </div>
                </td>
                <td style={{ padding: '8px 10px' }}>
                  {skill.stages?.slice(0, 3).map(s => (
                    <span key={s} style={{
                      display: 'inline-block',
                      padding: '1px 5px',
                      margin: '1px 2px',
                      borderRadius: 3,
                      fontSize: 10,
                      background: tokens.border,
                      color: '#8C9AB3',
                    }}>{s}</span>
                  ))}
                  {(skill.stages?.length || 0) > 3 && (
                    <span style={{ fontSize: 10, color: tokens.muted }}>+{skill.stages!.length - 3}</span>
                  )}
                </td>
                <td style={{ padding: '8px 10px', color: status.color, fontWeight: 500, fontSize: 11 }}>
                  <span style={{
                    display: 'inline-flex',
                    alignItems: 'center',
                    gap: 4,
                    padding: '2px 8px',
                    borderRadius: 10,
                    background: `${status.color}14`,
                  }}>
                    <span style={{ width: 5, height: 5, borderRadius: '50%', background: status.color }} />
                    {status.label}
                  </span>
                </td>
              </tr>
            );
          })}
        </tbody>
      </table>

      {/* Pagination */}
      {totalPages > 1 && (
        <div style={{
          display: 'flex',
          justifyContent: 'space-between',
          alignItems: 'center',
          padding: '10px 0',
          fontSize: 11,
          color: tokens.muted,
        }}>
          <span>Page {page} of {totalPages} ({total ?? skills.length} total)</span>
          <div style={{ display: 'flex', gap: 3 }}>
            {Array.from({ length: totalPages }).map((_, i) => (
              <button
                key={i}
                onClick={() => onPageChange?.(i + 1)}
                style={{
                  width: 28,
                  height: 28,
                  border: `1px solid ${page === i + 1 ? tokens.cta : tokens.border}`,
                  borderRadius: 4,
                  background: page === i + 1 ? tokens.cta : 'transparent',
                  color: page === i + 1 ? tokens.ctaText : tokens.text,
                  cursor: 'pointer',
                  fontSize: 11,
                  fontWeight: page === i + 1 ? 600 : 400,
                  textAlign: 'center',
                }}
              >
                {i + 1}
              </button>
            ))}
          </div>
        </div>
      )}
    </div>
  );
}
