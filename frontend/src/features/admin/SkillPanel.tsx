import { useState, useEffect, useMemo, useRef, useCallback } from 'react';
import { tokens } from '../../shared/design-tokens';
import { api } from '../../shared/api';
import { AppLayout } from '../../shared/AppLayout';
import { Skeleton } from '../../shared/skeleton';
import { SkillTable, type SkillEntry } from './SkillTable';
import { PrioritySlider } from './PrioritySlider';

export default function SkillPanel() {
  const [skills, setSkills] = useState<SkillEntry[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [search, setSearch] = useState('');
  const [debouncedSearch, setDebouncedSearch] = useState('');
  const [sourceFilter, setSourceFilter] = useState('all');
  const [sortBy, setSortBy] = useState<string>('current_priority');
  const [sortDir, setSortDir] = useState<'asc' | 'desc'>('desc');
  const [page, setPage] = useState(1);
  const pageSize = 20;
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
      setPage(1);
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

  // Filter + Sort
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
    const list = [...filtered];
    if (!sortBy) return list;
    list.sort((a, b) => {
      let va: any, vb: any;
      if (sortBy === 'name') { va = a.name; vb = b.name; }
      else if (sortBy === 'source') { va = a.source; vb = b.source; }
      else { va = a.current_priority; vb = b.current_priority; }
      if (va < vb) return sortDir === 'asc' ? -1 : 1;
      if (va > vb) return sortDir === 'asc' ? 1 : -1;
      return 0;
    });
    return list;
  }, [filtered, sortBy, sortDir]);

  // Pagination
  const totalPages = Math.max(1, Math.ceil(sorted.length / pageSize));
  const paged = useMemo(() => {
    const start = (page - 1) * pageSize;
    return sorted.slice(start, start + pageSize);
  }, [sorted, page, pageSize]);

  // When filter changes, reset to page 1
  useEffect(() => { setPage(1); }, [sourceFilter, debouncedSearch]);

  const handleSort = (col: string) => {
    if (col === sortBy) {
      setSortDir(d => d === 'asc' ? 'desc' : 'asc');
    } else {
      setSortBy(col);
      setSortDir(col === 'name' || col === 'source' ? 'asc' : 'desc');
    }
  };

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

  const handlePriorityChange = (skillName: string, value: number) => {
    const skill = skills.find(s => s.name === skillName);
    if (!skill) return;
    if (value === skill.current_priority) {
      const next = { ...priorityEdits };
      delete next[skillName];
      setPriorityEdits(next);
    } else {
      setPriorityEdits(prev => ({ ...prev, [skillName]: value }));
    }
  };

  // Loading state with skeleton
  if (loading) {
    return (
      <AppLayout>
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 24 }}>
          <h1 style={{ fontSize: 22, fontWeight: 700, fontFamily: tokens.fontHeading, margin: 0, color: tokens.text }}>Skill Management</h1>
        </div>
        <div style={{ display: 'grid', gridTemplateColumns: '3fr 2fr', gap: 16 }}>
          <div>
            <Skeleton variant="text" lines={1} height={16} />
            <div style={{ height: 16 }} />
            <Skeleton variant="rect" height={300} />
          </div>
          <div>
            <Skeleton variant="card" height={250} />
          </div>
        </div>
      </AppLayout>
    );
  }

  if (error) {
    return (
      <AppLayout>
        <div role="alert" style={{ textAlign: 'center', padding: 40, color: tokens.error }}>
          ⚠️ {error}
        </div>
      </AppLayout>
    );
  }

  return (
    <AppLayout>
      {/* Header */}
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 20 }}>
        <h1 style={{ fontSize: 22, fontWeight: 700, fontFamily: tokens.fontHeading, margin: 0, color: tokens.text }}>Skill Management</h1>
        <span style={{ fontSize: 12, color: tokens.muted }}>
          {skills.length} skills loaded{sourceFilter !== 'all' || debouncedSearch ? ` · ${sorted.length} filtered` : ''}
        </span>
      </div>

      {/* API Error bar */}
      {apiError && (
        <div style={{ padding: '8px 12px', background: '#7F1D1D', color: '#FCA5A5', borderRadius: 4, marginBottom: 16, fontSize: 13 }}>
          {apiError}
        </div>
      )}

      {/* Toolbar */}
      <div style={{ display: 'flex', gap: 8, marginBottom: 14 }}>
        <input
          type="text"
          placeholder="Search skills by name, keyword, or stage..."
          value={search}
          onChange={e => setSearch(e.target.value)}
          style={{
            flex: 1,
            padding: '9px 12px',
            background: '#111B2A',
            border: `1px solid ${tokens.border}`,
            borderRadius: 4,
            color: tokens.text,
            fontSize: 13,
            outline: 'none',
          }}
          onFocus={e => { e.currentTarget.style.borderColor = tokens.cta; }}
          onBlur={e => { e.currentTarget.style.borderColor = tokens.border; }}
        />
        <select
          value={sourceFilter}
          onChange={e => setSourceFilter(e.target.value)}
          style={{ padding: '9px 12px', background: '#111B2A', border: `1px solid ${tokens.border}`, borderRadius: 4, color: tokens.text, fontSize: 13 }}
        >
          <option value="all">All Sources</option>
          <option value="global">Global</option>
          <option value="team">Team</option>
          <option value="project">Project</option>
        </select>
        <select
          value={`${sortBy}:${sortDir}`}
          onChange={e => { const parts = e.target.value.split(':'); setSortBy(parts[0] ?? 'current_priority'); setSortDir((parts[1] ?? 'desc') as 'asc' | 'desc'); }}
          style={{ padding: '9px 12px', background: '#111B2A', border: `1px solid ${tokens.border}`, borderRadius: 4, color: tokens.text, fontSize: 13 }}
        >
          <option value="current_priority:desc">Sort: Priority ↓</option>
          <option value="current_priority:asc">Sort: Priority ↑</option>
          <option value="name:asc">Sort: Name A-Z</option>
          <option value="name:desc">Sort: Name Z-A</option>
          <option value="source:asc">Sort: Source</option>
        </select>
      </div>

      {/* Empty state */}
      {sorted.length === 0 ? (
        <div style={{ textAlign: 'center', padding: 40, color: tokens.muted }}>
          <div style={{ fontSize: 32, marginBottom: 8 }}>{'\u{1F50D}'}</div>
          <div style={{ fontSize: 14, fontWeight: 600, marginBottom: 4 }}>No skills match "{debouncedSearch}"</div>
          <div style={{ fontSize: 12, color: tokens.muted, marginBottom: 12 }}>Try adjusting your filters</div>
          {(sourceFilter !== 'all' || debouncedSearch) && (
            <button onClick={() => { setSearch(''); setSourceFilter('all'); }}
              style={{ padding: '6px 14px', border: `1px solid ${tokens.border}`, borderRadius: 4, background: 'transparent', color: tokens.text, cursor: 'pointer', fontSize: 12 }}>
              Clear Filters
            </button>
          )}
        </div>
      ) : (
        <div style={{ display: 'grid', gridTemplateColumns: selectedSkill ? '3fr 2fr' : '1fr', gap: 16 }}>
          {/* Table */}
          <div>
            <SkillTable
              skills={paged}
              onSelect={setSelectedSkill}
              selectedName={selectedSkill?.name}
              sortBy={sortBy}
              sortDir={sortDir}
              onSort={handleSort}
              page={page}
              pageSize={pageSize}
              total={sorted.length}
              onPageChange={setPage}
              priorityEdits={priorityEdits}
            />

            {/* Save Priorities */}
            {Object.keys(priorityEdits).length > 0 && (
              <div style={{ marginTop: 12, display: 'flex', gap: 8, alignItems: 'center' }}>
                <button onClick={handleSavePriorities} disabled={apiLoading}
                  style={{
                    padding: '7px 18px', background: tokens.cta, color: tokens.ctaText,
                    border: 'none', borderRadius: 4, cursor: apiLoading ? 'not-allowed' : 'pointer',
                    fontWeight: 500, fontSize: 12, opacity: apiLoading ? 0.6 : 1,
                  }}>
                  {apiLoading ? 'Saving...' : `Save Priorities (${Object.keys(priorityEdits).length} changed)`}
                </button>
                <button onClick={() => setPriorityEdits({})} disabled={apiLoading}
                  style={{ padding: '7px 14px', background: 'transparent', border: `1px solid ${tokens.border}`, borderRadius: 4, cursor: 'pointer', color: tokens.text, fontSize: 12 }}>
                  Cancel
                </button>
              </div>
            )}
          </div>

          {/* Detail Panel */}
          {selectedSkill && (
            <div style={{ background: '#111B2A', border: `1px solid ${tokens.border}`, borderRadius: 8, padding: 18 }}>
              <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start', marginBottom: 14 }}>
                <h4 style={{ fontSize: 16, fontWeight: 700, margin: 0 }}>
                  {selectedSkill.name}
                  <span style={{ fontSize: 12, color: tokens.muted, fontWeight: 400, marginLeft: 6 }}>v{selectedSkill.version}</span>
                </h4>
                <button
                  onClick={() => setSelectedSkill(null)}
                  aria-label="Close detail panel"
                  style={{ background: 'none', border: 'none', color: tokens.muted, cursor: 'pointer', fontSize: 18, lineHeight: 1, padding: '0 4px' }}
                >
                  ×
                </button>
              </div>

              {/* Keywords */}
              <div style={{ marginBottom: 14 }}>
                <h5 style={{ fontSize: 10, fontWeight: 600, color: tokens.muted, textTransform: 'uppercase', letterSpacing: '0.05em', marginBottom: 6 }}>Keywords</h5>
                <div style={{ display: 'flex', gap: 4, flexWrap: 'wrap' }}>
                  {selectedSkill.keywords?.map(k => (
                    <span key={k} style={{ fontSize: 10, padding: '2px 7px', borderRadius: 3, background: `${tokens.cta}14`, color: tokens.cta, fontWeight: 500 }}>{k}</span>
                  ))}
                </div>
              </div>

              {/* Priority Slider */}
              <div style={{ marginBottom: 14 }}>
                <h5 style={{ fontSize: 10, fontWeight: 600, color: tokens.muted, textTransform: 'uppercase', letterSpacing: '0.05em', marginBottom: 6 }}>Priority</h5>
                <PrioritySlider
                  value={priorityEdits[selectedSkill.name] ?? selectedSkill.current_priority}
                  max={100}
                  baseValue={selectedSkill.base_priority}
                  onChange={(v) => handlePriorityChange(selectedSkill.name, v)}
                  showLabels
                />
                <div style={{ fontSize: 10, color: tokens.muted, marginTop: 4 }}>Workflow steps: {selectedSkill.workflow_steps}</div>
              </div>

              {/* Prompt Preview */}
              <div style={{ marginBottom: 14 }}>
                <h5 style={{ fontSize: 10, fontWeight: 600, color: tokens.muted, textTransform: 'uppercase', letterSpacing: '0.05em', marginBottom: 6 }}>Prompt Preview</h5>
                <div style={{
                  padding: 10,
                  background: '#0D1624',
                  border: `1px solid ${tokens.border}`,
                  borderRadius: 4,
                  fontSize: 11,
                  lineHeight: 1.5,
                  maxHeight: 120,
                  overflow: 'auto',
                  color: '#8C9AB3',
                  fontFamily: tokens.fontHeading,
                }}>
                  {selectedSkill.prompt_preview}
                </div>
              </div>

              {/* Deprecate / Restore */}
              <div>
                {selectedSkill.deprecated ? (
                  <button onClick={() => handleDeprecateToggle(selectedSkill.name, false)} disabled={apiLoading}
                    style={{ padding: '6px 14px', background: tokens.cta, color: tokens.ctaText, border: 'none', borderRadius: 4, cursor: apiLoading ? 'not-allowed' : 'pointer', fontSize: 12, fontWeight: 500, opacity: apiLoading ? 0.6 : 1 }}>
                    Restore Skill
                  </button>
                ) : deprecateConfirm === selectedSkill.name ? (
                  <div style={{ display: 'flex', gap: 8, alignItems: 'center' }}>
                    <span style={{ fontSize: 12, color: tokens.warning }}>Confirm deprecation?</span>
                    <button onClick={() => handleDeprecateToggle(selectedSkill.name, true)} disabled={apiLoading}
                      style={{ padding: '5px 12px', background: tokens.error, color: 'white', border: 'none', borderRadius: 4, cursor: apiLoading ? 'not-allowed' : 'pointer', fontSize: 11, opacity: apiLoading ? 0.6 : 1 }}>Confirm</button>
                    <button onClick={() => setDeprecateConfirm(null)} disabled={apiLoading}
                      style={{ padding: '5px 12px', background: 'transparent', border: `1px solid ${tokens.border}`, borderRadius: 4, cursor: 'pointer', color: tokens.text, fontSize: 11 }}>Cancel</button>
                  </div>
                ) : (
                  <button onClick={() => setDeprecateConfirm(selectedSkill.name)} disabled={apiLoading}
                    style={{
                      padding: '6px 14px',
                      background: 'transparent',
                      color: tokens.error,
                      border: `1px solid rgba(239, 68, 68, 0.3)`,
                      borderRadius: 4,
                      cursor: apiLoading ? 'not-allowed' : 'pointer',
                      fontSize: 12,
                      fontWeight: 500,
                      opacity: apiLoading ? 0.6 : 1,
                    }}>
                    Deprecate Skill
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
