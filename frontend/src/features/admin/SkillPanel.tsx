import { useState, useEffect, useMemo } from 'react';
import { tokens } from '../../shared/design-tokens';

interface SkillEntry {
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

export function SkillPanel() {
  const [skills, setSkills] = useState<SkillEntry[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [search, setSearch] = useState('');
  const [sourceFilter, setSourceFilter] = useState('all');
  const [selectedSkill, setSelectedSkill] = useState<SkillEntry | null>(null);
  const [deprecateConfirm, setDeprecateConfirm] = useState<string | null>(null);
  const [priorityEdits, setPriorityEdits] = useState<Record<string, number>>({});

  useEffect(() => {
    fetch('/api/admin/skills')
      .then(r => { if (!r.ok) throw new Error(r.statusText); return r.json(); })
      .then((data: SkillEntry[]) => { setSkills(Array.isArray(data) ? data : []); })
      .catch(err => setError(err instanceof Error ? err.message : 'Failed'))
      .finally(() => setLoading(false));
  }, []);

  const filtered = useMemo(() => {
    return skills.filter(s => {
      if (sourceFilter !== 'all' && s.source !== sourceFilter) return false;
      if (search) {
        const q = search.toLowerCase();
        return (
          s.name.toLowerCase().includes(q) ||
          s.keywords?.some(k => k.toLowerCase().includes(q)) ||
          s.stages?.some(st => st.toLowerCase().includes(q))
        );
      }
      return true;
    });
  }, [skills, search, sourceFilter]);

  const sorted = useMemo(() => {
    return [...filtered].sort((a, b) => b.current_priority - a.current_priority);
  }, [filtered]);

  if (loading) {
    return <div style={{ padding: 24, color: tokens.muted, fontFamily: tokens.fontBody }}>Loading skills...</div>;
  }

  if (error) {
    return <div style={{ padding: 24, color: tokens.error, fontFamily: tokens.fontBody }}>Error: {error}</div>;
  }

  return (
    <div style={{ padding: 24, color: tokens.text, fontFamily: tokens.fontBody }}>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 24 }}>
        <h1 style={{ fontSize: 24, fontWeight: 600, margin: 0 }}>Skill Management</h1>
        <span style={{ fontSize: 13, color: tokens.muted }}>{skills.length} skills loaded</span>
      </div>

      <div style={{ display: 'flex', gap: 12, marginBottom: 20 }}>
        <input
          type="text" placeholder="Search skills..." value={search}
          onChange={e => setSearch(e.target.value)}
          style={{ flex: 1, padding: '8px 12px', background: tokens.surface, border: `1px solid ${tokens.border}`, borderRadius: 4, color: tokens.text, fontSize: 14, outline: 'none' }}
        />
        <select value={sourceFilter} onChange={e => setSourceFilter(e.target.value)}
          style={{ padding: '8px 12px', background: tokens.surface, border: `1px solid ${tokens.border}`, borderRadius: 4, color: tokens.text, fontSize: 14 }}>
          <option value="all">All Sources</option>
          <option value="global">Global</option>
          <option value="team">Team</option>
          <option value="project">Project</option>
        </select>
      </div>

      {sorted.length === 0 ? (
        <div style={{ textAlign: 'center', padding: 40, color: tokens.muted }}>No skills match your filters.</div>
      ) : (
        <div style={{ display: 'grid', gridTemplateColumns: selectedSkill ? '1fr 1fr' : '1fr', gap: 16 }}>
          <div>
            <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: 13 }}>
              <thead>
                <tr style={{ borderBottom: `2px solid ${tokens.border}`, textAlign: 'left' }}>
                  <th style={{ padding: '8px 12px' }}>Skill</th>
                  <th style={{ padding: '8px 12px' }}>Source</th>
                  <th style={{ padding: '8px 12px' }}>Priority</th>
                  <th style={{ padding: '8px 12px' }}>Stages</th>
                  <th style={{ padding: '8px 12px' }}>Status</th>
                </tr>
              </thead>
              <tbody>
                {sorted.map(skill => (
                  <tr key={`${skill.name}@${skill.version}`}
                    onClick={() => setSelectedSkill(skill)}
                    style={{ borderBottom: `1px solid ${tokens.border}`, cursor: 'pointer', background: selectedSkill?.name === skill.name ? `${tokens.cta}18` : 'transparent' }}>
                    <td style={{ padding: '8px 12px' }}>
                      <div style={{ fontWeight: 500 }}>{skill.name}</div>
                      <div style={{ fontSize: 11, color: tokens.muted }}>v{skill.version}</div>
                    </td>
                    <td style={{ padding: '8px 12px', color: tokens.muted }}>{skill.source}</td>
                    <td style={{ padding: '8px 12px' }}>
                      <span style={{ fontWeight: 600 }}>{priorityEdits[skill.name] ?? skill.current_priority}</span>
                      <span style={{ fontSize: 11, color: tokens.muted }}> / 100</span>
                    </td>
                    <td style={{ padding: '8px 12px' }}>
                      {skill.stages?.map(s => (
                        <span key={s} style={{ display: 'inline-block', padding: '1px 6px', margin: '1px 2px', borderRadius: 3, fontSize: 10, background: tokens.surface, border: `1px solid ${tokens.border}` }}>{s}</span>
                      ))}
                    </td>
                    <td style={{ padding: '8px 12px' }}>
                      {skill.deprecated ? <span style={{ color: tokens.error, fontWeight: 500 }}>Deprecated</span>
                        : skill.enabled ? <span style={{ color: tokens.cta }}>Active</span>
                        : <span style={{ color: tokens.warning }}>Disabled</span>}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>

          {selectedSkill && (
            <div style={{ background: tokens.surface, borderRadius: 8, padding: 20, border: `1px solid ${tokens.border}` }}>
              <h2 style={{ fontSize: 18, fontWeight: 600, marginBottom: 16 }}>{selectedSkill.name} <span style={{ fontSize: 13, color: tokens.muted, fontWeight: 400 }}>v{selectedSkill.version}</span></h2>

              <div style={{ marginBottom: 16 }}>
                <h3 style={{ fontSize: 13, color: tokens.muted, marginBottom: 8 }}>Keywords</h3>
                <div>{selectedSkill.keywords?.map(k => (
                  <span key={k} style={{ display: 'inline-block', padding: '2px 8px', margin: '2px 4px 2px 0', borderRadius: 4, fontSize: 11, background: `${tokens.cta}18`, color: tokens.cta }}>{k}</span>
                ))}</div>
              </div>

              <div style={{ marginBottom: 16 }}>
                <h3 style={{ fontSize: 13, color: tokens.muted, marginBottom: 8 }}>Priority Breakdown</h3>
                <div style={{ fontSize: 13 }}>
                  <div>Base: <strong>{selectedSkill.base_priority}</strong></div>
                  <div>Current: <strong>{priorityEdits[selectedSkill.name] ?? selectedSkill.current_priority}</strong></div>
                  <div style={{ fontSize: 11, color: tokens.muted }}>Workflow steps: {selectedSkill.workflow_steps}</div>
                </div>
              </div>

              <div style={{ marginBottom: 16 }}>
                <h3 style={{ fontSize: 13, color: tokens.muted, marginBottom: 8 }}>Prompt Preview</h3>
                <div style={{ padding: 12, background: tokens.bg, borderRadius: 4, fontSize: 12, lineHeight: 1.5, maxHeight: 200, overflow: 'auto' }}>
                  {selectedSkill.prompt_preview}
                </div>
              </div>

              <div>
                {selectedSkill.deprecated ? (
                  <button onClick={() => setDeprecateConfirm(null)} style={{ padding: '6px 16px', background: tokens.cta, color: tokens.ctaText, border: 'none', borderRadius: 4, cursor: 'pointer', fontWeight: 500 }}>Restore</button>
                ) : deprecateConfirm === selectedSkill.name ? (
                  <div style={{ display: 'flex', gap: 8, alignItems: 'center' }}>
                    <span style={{ fontSize: 12, color: tokens.warning }}>Confirm deprecation?</span>
                    <button onClick={() => setDeprecateConfirm(null)} style={{ padding: '4px 12px', background: tokens.error, color: 'white', border: 'none', borderRadius: 4, cursor: 'pointer', fontSize: 12 }}>Confirm</button>
                    <button onClick={() => setDeprecateConfirm(null)} style={{ padding: '4px 12px', background: tokens.surface, border: `1px solid ${tokens.border}`, borderRadius: 4, cursor: 'pointer', color: tokens.text, fontSize: 12 }}>Cancel</button>
                  </div>
                ) : (
                  <button onClick={() => setDeprecateConfirm(selectedSkill.name)} style={{ padding: '6px 16px', background: 'transparent', color: tokens.error, border: `1px solid ${tokens.error}`, borderRadius: 4, cursor: 'pointer', fontSize: 12, fontWeight: 500 }}>Deprecate</button>
                )}
              </div>
            </div>
          )}
        </div>
      )}
    </div>
  );
}
