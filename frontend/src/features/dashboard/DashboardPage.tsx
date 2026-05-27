import { useEffect, useState } from 'react';
import { Link } from 'react-router-dom';
import { api } from '../../shared/api';
import { useAuth } from '../../shared/auth';
import { AppLayout } from '../../shared/AppLayout';
import { tokens } from '../../shared/design-tokens';
import { ProjectCard } from './ProjectCard';
import { PageSkeleton } from '../../shared/skeleton';

interface Project {
  id: string;
  name: string;
  git_url: string;
  created_at: string;
  pipeline_count?: number;
}

interface DashboardStats {
  projectCount: number;
  skillCount: number;
  pendingReviews: number;
  pipelinesActive: number;
}

function StatCard({ label, value, to, accent }: { label: string; value: number; to: string; accent: string }) {
  const [hovered, setHovered] = useState(false);
  return (
    <Link
      to={to}
      onMouseEnter={() => setHovered(true)}
      onMouseLeave={() => setHovered(false)}
      style={{
        display: 'block', padding: '16px 20px', borderRadius: 8,
        background: tokens.surface,
        border: `1px solid ${hovered ? accent : tokens.border}`,
        textDecoration: 'none',
        transition: tokens.transition,
      }}
      aria-label={`${label}: ${value}`}
    >
      <div style={{ fontSize: 13, color: tokens.muted, marginBottom: 4 }}>{label}</div>
      <div style={{ fontSize: 28, fontWeight: 700, fontFamily: tokens.fontHeading, color: accent }}>{value}</div>
    </Link>
  );
}

function fetchSkillCount(): Promise<number> {
  return api.listSkills()
    .then((data: any[]) => (Array.isArray(data) ? data.length : 0))
    .catch(() => 0);
}

function fetchReviewCount(): Promise<number> {
  return api.getReviewInbox()
    .then((data: any) => (Array.isArray(data) ? data.length : 0))
    .catch(() => 0);
}

interface ActivePipeline {
  id: string;
  project_id: string;
  project_name: string;
  title: string;
  status: string;
  current_stage: string;
  updated_at: string;
}

const ALL_STAGES = ['clarify', 'decompose', 'impl', 'test', 'deploy', 'verify'];
const STAGE_NAMES: Record<string, string> = {
  clarify: 'Clarify', decompose: 'Decompose', impl: 'Impl', test: 'Test', deploy: 'Deploy', verify: 'Verify',
};

export function DashboardPage() {
  const [projects, setProjects] = useState<Project[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [showCreate, setShowCreate] = useState(false);
  const [newName, setNewName] = useState('');
  const [newGitUrl, setNewGitUrl] = useState('');
  const [creating, setCreating] = useState(false);
  const [stats, setStats] = useState<DashboardStats>({ projectCount: 0, skillCount: 0, pendingReviews: 0, pipelinesActive: 0 });
  const [activePipelines, setActivePipelines] = useState<ActivePipeline[]>([]);
  const [showContent, setShowContent] = useState(false);

  useEffect(() => {
    const minDelay = new Promise(r => setTimeout(r, 600));
    const fetchData = Promise.all([
      api.listProjects().then(p => setProjects(Array.isArray(p) ? p : [])),
      fetchSkillCount().then(c => setStats(s => ({ ...s, skillCount: c }))),
      fetchReviewCount().then(c => setStats(s => ({ ...s, pendingReviews: c }))),
      api.activePipelines().then(d => {
        const arr = Array.isArray(d) ? d : [];
        setActivePipelines(arr);
        setStats(s => ({ ...s, pipelinesActive: arr.length }));
      }).catch(() => {}),
    ]).catch(err => setError(err instanceof Error ? err.message : 'Failed to load'));
    Promise.all([fetchData, minDelay]).finally(() => {
      setLoading(false);
      setShowContent(true);
    });
  }, []);

  useEffect(() => {
    setStats(s => ({ ...s, projectCount: projects.length }));
  }, [projects]);

  const handleCreateProject = async () => {
    if (!newName.trim()) return;
    setCreating(true);
    try {
      const gitUrl = newGitUrl.trim() || `https://github.com/user/${newName.trim().toLowerCase().replace(/\s+/g, '-')}`;
      const p = await api.createProject(newName.trim(), gitUrl);
      setProjects(prev => [p, ...prev]);
      setShowCreate(false); setNewName(''); setNewGitUrl('');
      setError(null);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to create project');
    } finally { setCreating(false); }
  };

  if (loading) return <PageSkeleton />;

  return (
    <AppLayout>
      {error && (
        <div style={{
          padding: '12px 16px', marginBottom: 24, borderRadius: 8,
          background: `${tokens.error}18`, border: `1px solid ${tokens.error}40`,
          color: tokens.error, fontSize: 14,
        }}>{error}</div>
      )}

      {/* Stats Row */}
      <div style={{
        display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(180px, 1fr))',
        gap: 12, marginBottom: 24,
      }}>
        <StatCard label="Projects" value={stats.projectCount} to="/" accent={tokens.cta} />
        <StatCard label="Available Skills" value={stats.skillCount} to="/admin/skills" accent="#8b5cf6" />
        <StatCard label="Pending Reviews" value={stats.pendingReviews} to="/review-inbox" accent={tokens.warning} />
        <StatCard label="Pipelines Active" value={stats.pipelinesActive} to="/" accent="#3b82f6" />
      </div>

      {/* Active Pipelines Board */}
      {activePipelines.length > 0 && (
        <>
          <h2 style={{ fontSize: 22, fontWeight: 700, fontFamily: tokens.fontHeading, margin: '0 0 16px 0', color: tokens.text }}>Active Pipelines</h2>
          <div style={{
            display: 'grid', gridTemplateColumns: 'repeat(auto-fill, minmax(320px, 1fr))',
            gap: 12, marginBottom: 32,
          }}>
            {activePipelines.map(ap => {
              const stageIdx = ALL_STAGES.indexOf(ap.current_stage || '');
              const isBlocked = ap.status === 'awaiting_review' || ap.status === 'paused';
              return (
                <Link key={ap.id} to={`/project/${ap.project_id}/pipeline/${ap.id}`} style={{ textDecoration: 'none' }}>
                  <div style={{
                    background: tokens.surface, borderRadius: 8, padding: '14px 16px',
                    border: `1px solid ${isBlocked ? tokens.warning : tokens.border}`,
                    boxShadow: isBlocked ? `0 0 12px ${tokens.warning}20` : 'none',
                    transition: tokens.transition,
                  }}
                    onMouseEnter={e => { (e.currentTarget as HTMLElement).style.borderColor = tokens.cta; }}
                    onMouseLeave={e => { (e.currentTarget as HTMLElement).style.borderColor = isBlocked ? tokens.warning : tokens.border; }}
                  >
                    <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start', marginBottom: 10 }}>
                      <div style={{ flex: 1, minWidth: 0 }}>
                        <div style={{ fontSize: 13, color: tokens.muted, marginBottom: 2, display: 'flex', alignItems: 'center', gap: 6 }}>
                          <span style={{
                            display: 'inline-block', width: 8, height: 8, borderRadius: '50%',
                            background: ap.status === 'running' ? tokens.cta : tokens.warning,
                            animation: ap.status === 'running' ? 'pulse 1.8s ease-in-out infinite' : 'none',
                          }} />
                          {ap.project_name}
                        </div>
                        <div style={{ fontSize: 14, fontWeight: 600, color: tokens.text, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                          {ap.title}
                        </div>
                      </div>
                      {isBlocked && (
                        <span style={{
                          padding: '2px 8px', borderRadius: 8, background: `${tokens.warning}20`,
                          color: tokens.warning, fontSize: 11, fontWeight: 600, flexShrink: 0, marginLeft: 8,
                        }}>
                          Review
                        </span>
                      )}
                    </div>
                    {/* Stage progress bar */}
                    <div style={{ display: 'flex', gap: 3, alignItems: 'center' }}>
                      {ALL_STAGES.map((s, i) => {
                        const isCurrent = i === stageIdx;
                        const isPassed = i < stageIdx;
                        return (
                          <div key={s} style={{ flex: 1, position: 'relative' }}>
                            <div style={{
                              height: 4, borderRadius: 2,
                              background: isPassed ? tokens.cta : isCurrent ? tokens.warning : tokens.border,
                              boxShadow: isCurrent ? `0 0 6px ${tokens.warning}` : 'none',
                              transition: tokens.transition,
                            }} />
                            {isCurrent && (
                              <div style={{
                                position: 'absolute', top: -2, left: '50%', transform: 'translateX(-50%)',
                                width: 8, height: 8, borderRadius: '50%', background: tokens.warning,
                                animation: 'pulse 1.2s ease-in-out infinite',
                              }} />
                            )}
                          </div>
                        );
                      })}
                    </div>
                    <div style={{ display: 'flex', justifyContent: 'space-between', marginTop: 6, fontSize: 11, color: tokens.muted }}>
                      <span>{ap.current_stage ? STAGE_NAMES[ap.current_stage] || ap.current_stage : '—'}</span>
                      <span>{new Date(ap.updated_at).toLocaleDateString()}</span>
                    </div>
                  </div>
                </Link>
              );
            })}
          </div>
        </>
      )}

      {/* Projects Section */}
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 20 }}>
        <h2 style={{ fontSize: 22, fontWeight: 700, fontFamily: tokens.fontHeading, margin: 0, color: tokens.text }}>Projects</h2>
        <button
          onClick={() => setShowCreate(!showCreate)}
          style={{
            padding: '8px 16px', background: tokens.cta, color: tokens.ctaText, border: 'none', borderRadius: 6,
            cursor: 'pointer', fontWeight: 500, fontSize: 13, transition: tokens.transition,
          }}>
          + New Project
        </button>
      </div>

      {showCreate && (
        <div style={{ background: tokens.surface, border: `1px solid ${tokens.border}`, borderRadius: 8, padding: 16, marginBottom: 20 }}>
          <div style={{ display: 'flex', gap: 8, flexWrap: 'wrap' }}>
            <input type="text" placeholder="Project name" value={newName}
              onChange={e => setNewName(e.target.value)}
              onKeyDown={e => e.key === 'Enter' && handleCreateProject()}
              style={{ flex: '1 1 200px', padding: '8px 12px', background: tokens.bg, border: `1px solid ${tokens.border}`, borderRadius: 4, color: tokens.text, fontSize: 14, outline: 'none' }} />
            <input type="text" placeholder="Git URL (optional)" value={newGitUrl}
              onChange={e => setNewGitUrl(e.target.value)}
              onKeyDown={e => e.key === 'Enter' && handleCreateProject()}
              style={{ flex: '1 1 300px', padding: '8px 12px', background: tokens.bg, border: `1px solid ${tokens.border}`, borderRadius: 4, color: tokens.text, fontSize: 14, outline: 'none' }} />
            <button
              onClick={handleCreateProject}
              disabled={creating || !newName.trim()}
              style={{ padding: '8px 20px', background: tokens.cta, color: tokens.ctaText, border: 'none', borderRadius: 4, cursor: creating ? 'default' : 'pointer', fontWeight: 500, opacity: creating || !newName.trim() ? 0.5 : 1 }}>
              {creating ? 'Creating...' : 'Create'}
            </button>
          </div>
        </div>
      )}

      {!showContent ? <PageSkeleton cards={1} />
      : projects.length === 0 ? (
        <div style={{
          textAlign: 'center', padding: '60px 0',
          background: tokens.surface, borderRadius: 8,
          border: `1px solid ${tokens.border}`,
        }}>
          <div style={{ marginBottom: 12, color: tokens.muted }}>
            <svg width="48" height="48" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1" strokeLinecap="round">
              <path d="M22 19a2 2 0 01-2 2H4a2 2 0 01-2-2V5a2 2 0 012-2h5l2 3h9a2 2 0 012 2z"/>
            </svg>
          </div>
          <p style={{ color: tokens.muted, fontSize: 16, marginBottom: 8, fontWeight: 500 }}>No projects yet</p>
          <p style={{ color: tokens.muted, fontSize: 14, marginBottom: 0 }}>Connect a Git repository to get started with OpenForge.</p>
        </div>
      ) : (
        <div style={{
          display: 'grid', gridTemplateColumns: 'repeat(auto-fill, minmax(320px, 1fr))',
          gap: 16,
        }}>
          {projects.map(p => (
            <Link key={p.id} to={`/project/${p.id}`} style={{ textDecoration: 'none' }}>
              <ProjectCard project={p} />
            </Link>
          ))}
        </div>
      )}
    </AppLayout>
  );
}
