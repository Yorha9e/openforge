import { useState, useEffect, useMemo, useRef, useCallback } from 'react';
import { tokens } from '../../shared/design-tokens';
import { api } from '../../shared/api';
import { AppLayout } from '../../shared/AppLayout';

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

export default function SkillPanel() {
  const [skills, setSkills] = useState<SkillEntry[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [search, setSearch] = useState('');
  const [debouncedSearch, setDebouncedSearch] = useState('');
  const [sourceFilter, setSourceFilter] = useState('all');
  const [selectedSkill, setSelectedSkill] = useState<SkillEntry | null>(null);
  const [deprecateConfirm, setDeprecateConfirm] = useState<string | null>(null);
  const [priorityEdits, setPriorityEdits] = useState<Record<string, number>>({});
  const [apiLoading, setApiLoading] = useState(false);
  const [apiError, setApiError] = useState<string | null>(null);
  const searchTimer = useRef<ReturnType<typeof setTimeout>>(null);

  // Search debounce: 200ms
  useEffect(() => {
    if (searchTimer.current) clearTimeout(searchTimer.current);
    searchTimer.current = setTimeout(() => {
      setDebouncedSearch(search);
    }, 200);
    return () => {
      if (searchTimer.current) clearTimeout(searchTimer.current);
    };
  }, [search]);

  const loadSkills = useCallback(() => {
    setLoading(true);
    setError(null);
    api.listSkills()
      .then((data: SkillEntry[]) => { setSkills(Array.isArray(data) ? data : []); })
      .catch(err => setError(err instanceof Error ? err.message : 'Failed'))
      .finally(() => setLoading(false));
  }, []);

  useEffect(() => {
    loadSkills();
  }, [loadSkills]);

  const filtered = useMemo(() => {
    return skills.filter(s => {
      if (sourceFilter !== 'all' && s.source !== sourceFilter) return false;
      if (debouncedSearch) {
        const q = debouncedSearch.toLowerCase();
        return (
          s.name.toLowerCase().includes(q) ||
          s.keywords?.some(k => k.toLowerCase().includes(q)) ||
          s.stages?.some(st => st.toLowerCase().includes(q))
        );
      }
      return true;
    });
  }, [skills, debouncedSearch, sourceFilter]);

  const sorted = useMemo(() => {
    return [...filtered].sort((a, b) => b.current_priority - a.current_priority);
  }, [filtered]);

  const handleDeprecateToggle = async (skillName: string, deprecated: boolean) => {
    setApiLoading(true);
    setApiError(null);
    try {
      await api.updateSkillDeprecated(skillName, deprecated);
      setDeprecateConfirm(null);
      setSelectedSkill(null);
      loadSkills();
    } catch (err) {
      setApiError(err instanceof Error ? err.message : 'Failed to update skill');
    } finally {
      setApiLoading(false);
    }
  };

  const handleSavePriorities = async () => {
    if (Object.keys(priorityEdits).length === 0) return;
    setApiLoading(true);
    setApiError(null);
    try {
      await api.updateSkillPriorities(priorityEdits);
      setPriorityEdits({});
      loadSkills();
    } catch (err) {
      setApiError(err instanceof Error ? err.message : 'Failed to save priorities');
    } finally {
      setApiLoading(false);
    }
  };

  if (loading) {
    return <AppLayout><p style={{ color: tokens.muted }}>Loading skills...</p></AppLayout>;
  }

  if (error) {
    return <AppLayout><p style={{ color: tokens.error }}>Error: {error}</p></AppLayout>;
  }

  return (
    <AppLayout>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 24 }}>
        <h1 style={{ fontSize: 22, fontWeight: 700, fontFamily: tokens.fontHeading, margin: 0, color: tokens.text }}>Skill Management</h1>
        <span style={{ fontSize: 13, color: tokens.muted }}>{skills.length} skills loaded</span>
      </div>

      {apiError && (
        <div style={{ padding: '8px 12px', background: '#7F1D1D', color: '#FCA5A5', borderRadius: 4, marginBottom: 16, fontSize: 13 }}>
          {apiError}
        </div>
      )}

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

            {/* Save Priorities button */}
            {Object.keys(priorityEdits).length > 0 && (
              <div style={{ marginTop: 16, display: 'flex', gap: 8, alignItems: 'center' }}>
                <button onClick={handleSavePriorities} disabled={apiLoading}
                  style={{ padding: '8px 20px', background: tokens.cta, color: tokens.ctaText, border: 'none', borderRadius: 4, cursor: apiLoading ? 'not-allowed' : 'pointer', fontWeight: 500, opacity: apiLoading ? 0.6 : 1 }}>
                  {apiLoading ? 'Saving...' : `Save Priorities (${Object.keys(priorityEdits).length} changed)`}
                </button>
                <button onClick={() => setPriorityEdits({})} disabled={apiLoading}
                  style={{ padding: '8px 16px', background: 'transparent', border: `1px solid ${tokens.border}`, borderRadius: 4, cursor: 'pointer', color: tokens.text, fontSize: 13 }}>
                  Cancel
                </button>
              </div>
            )}
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
                  <button onClick={() => handleDeprecateToggle(selectedSkill.name, false)} disabled={apiLoading}
                    style={{ padding: '6px 16px', background: tokens.cta, color: tokens.ctaText, border: 'none', borderRadius: 4, cursor: apiLoading ? 'not-allowed' : 'pointer', fontWeight: 500, opacity: apiLoading ? 0.6 : 1 }}>
                    Restore
                  </button>
                ) : deprecateConfirm === selectedSkill.name ? (
                  <div style={{ display: 'flex', gap: 8, alignItems: 'center' }}>
                    <span style={{ fontSize: 12, color: tokens.warning }}>Confirm deprecation?</span>
                    <button onClick={() => handleDeprecateToggle(selectedSkill.name, true)} disabled={apiLoading}
                      style={{ padding: '4px 12px', background: tokens.error, color: 'white', border: 'none', borderRadius: 4, cursor: apiLoading ? 'not-allowed' : 'pointer', fontSize: 12, opacity: apiLoading ? 0.6 : 1 }}>Confirm</button>
                    <button onClick={() => setDeprecateConfirm(null)} disabled={apiLoading}
                      style={{ padding: '4px 12px', background: tokens.surface, border: `1px solid ${tokens.border}`, borderRadius: 4, cursor: 'pointer', color: tokens.text, fontSize: 12 }}>Cancel</button>
                  </div>
                ) : (
                  <button onClick={() => setDeprecateConfirm(selectedSkill.name)} disabled={apiLoading}
                    style={{ padding: '6px 16px', background: 'transparent', color: tokens.error, border: `1px solid ${tokens.error}`, borderRadius: 4, cursor: apiLoading ? 'not-allowed' : 'pointer', fontSize: 12, fontWeight: 500, opacity: apiLoading ? 0.6 : 1 }}>
                    Deprecate
                  </button>
                )}
              </div>
            </div>
          )}
        </div>
      )}
    </AppLayout>
  );
}
