import { useEffect, useState } from 'react';
import { Link } from 'react-router-dom';
import { api } from '../../shared/api';
import { useAuth } from '../../shared/auth';
import { ProjectCard } from './ProjectCard';
import { tokens } from '../../shared/design-tokens';
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

function NavIcon({ children, to, label, external }: { children: React.ReactNode; to: string; label: string; external?: boolean }) {
  const [hovered, setHovered] = useState(false);
  const linkProps = external ? { href: to, target: '_blank', rel: 'noopener noreferrer' } : { to };
  const Component = external ? 'a' : Link;

  return (
    <Component
      {...linkProps as any}
      onMouseEnter={() => setHovered(true)}
      onMouseLeave={() => setHovered(false)}
      aria-label={label}
      style={{
        display: 'flex', flexDirection: 'column', alignItems: 'center', gap: 6,
        padding: '12px 16px', borderRadius: 8, textDecoration: 'none',
        background: hovered ? tokens.surface : 'transparent',
        border: `1px solid ${hovered ? tokens.border : 'transparent'}`,
        transition: tokens.transition, cursor: 'pointer',
        minWidth: 80,
      }}
    >
      <div style={{ color: hovered ? tokens.cta : tokens.muted, transition: tokens.transition }}>
        {children}
      </div>
      <span style={{ fontSize: 11, color: hovered ? tokens.text : tokens.muted, transition: tokens.transition, whiteSpace: 'nowrap' }}>
        {label}
      </span>
    </Component>
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

export function DashboardPage() {
  const { user, logout } = useAuth();
  const [projects, setProjects] = useState<Project[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [logoutHovered, setLogoutHovered] = useState(false);
  const [showContent, setShowContent] = useState(false);
  const [showCreate, setShowCreate] = useState(false);
  const [newName, setNewName] = useState('');
  const [newGitUrl, setNewGitUrl] = useState('');
  const [creating, setCreating] = useState(false);
  const [stats, setStats] = useState<DashboardStats>({ projectCount: 0, skillCount: 0, pendingReviews: 0 });
  const [systemStatus, setSystemStatus] = useState<{ phase: string; profile: string; health: string; models: number; skills: number } | null>(null);

  useEffect(() => {
    const minDelay = new Promise(r => setTimeout(r, 600));
    const fetchData = Promise.all([
      api.listProjects().then(p => setProjects(Array.isArray(p) ? p : [])),
      fetchSkillCount().then(c => setStats(s => ({ ...s, skillCount: c }))),
      fetchReviewCount().then(c => setStats(s => ({ ...s, pendingReviews: c }))),
      api.getHealth().then((d: any) => d.status).catch(() => 'unknown'),
      api.getAdminStatus().then((d: any) => setSystemStatus({
        phase: d.phase, profile: d.profile, health: 'ok', models: d.models, skills: d.skills,
      })).catch(() => {}),
    ]).catch(err => setError(err instanceof Error ? err.message : 'Failed to load'));
    Promise.all([fetchData, minDelay]).finally(() => {
      setLoading(false);
      setShowContent(true);
    });
  }, []);

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

  useEffect(() => {
    setStats(s => ({ ...s, projectCount: projects.length }));
  }, [projects]);

  const navLinks = [
    { to: '/review-inbox', label: 'Review Inbox', icon: (
      <svg width="22" height="22" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
        <path d="M22 12h-6l-2 3H10l-2-3H2"/><path d="M5.45 5.11L2 12v6a2 2 0 002 2h16a2 2 0 002-2v-6l-3.45-6.89A2 2 0 0016.76 4H7.24a2 2 0 00-1.79 1.11z"/>
      </svg>
    )},
    { to: '/admin/skills', label: 'Skills', icon: (
      <svg width="22" height="22" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
        <polygon points="12 2 15.09 8.26 22 9.27 17 14.14 18.18 21.02 12 17.77 5.82 21.02 7 14.14 2 9.27 8.91 8.26 12 2"/>
      </svg>
    )},
    { to: '/settings', label: 'Settings', icon: (
      <svg width="22" height="22" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
        <circle cx="12" cy="12" r="3"/><path d="M19.4 15a1.65 1.65 0 00.33 1.82l.06.06a2 2 0 010 2.83 2 2 0 01-2.83 0l-.06-.06a1.65 1.65 0 00-1.82-.33 1.65 1.65 0 00-1 1.51V21a2 2 0 01-4 0v-.09A1.65 1.65 0 009 19.4a1.65 1.65 0 00-1.82.33l-.06.06a2 2 0 01-2.83-2.83l.06-.06A1.65 1.65 0 004.68 15a1.65 1.65 0 00-1.51-1H3a2 2 0 010-4h.09A1.65 1.65 0 004.6 9a1.65 1.65 0 00-.33-1.82l-.06-.06a2 2 0 012.83-2.83l.06.06A1.65 1.65 0 009 4.68a1.65 1.65 0 001-1.51V3a2 2 0 014 0v.09a1.65 1.65 0 001 1.51 1.65 1.65 0 001.82-.33l.06-.06a2 2 0 012.83 2.83l-.06.06A1.65 1.65 0 0019.4 9a1.65 1.65 0 001.51 1H21a2 2 0 010 4h-.09a1.65 1.65 0 00-1.51 1z"/>
      </svg>
    )},
    { to: '/admin', label: 'Admin', icon: (
      <svg width="22" height="22" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
        <path d="M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10z"/>
      </svg>
    )},
  ];

  return (
    <div style={{ minHeight: '100vh', background: tokens.bg, color: tokens.text, fontFamily: tokens.fontBody }}>
      <header style={{
        display: 'flex', justifyContent: 'space-between', alignItems: 'center',
        padding: '12px 32px', borderBottom: `1px solid ${tokens.border}`,
        background: tokens.surface,
      }}>
        <div style={{ display: 'flex', alignItems: 'center', gap: 16 }}>
          <h1 style={{
            fontSize: 20, fontWeight: 700, fontFamily: tokens.fontHeading, margin: 0,
            color: tokens.cta, letterSpacing: '-0.5px',
          }}>OpenForge</h1>
        </div>
        <div style={{ display: 'flex', alignItems: 'center', gap: 12 }}>
          <span style={{ color: tokens.muted, fontSize: 13 }}>{user?.id}</span>
          <button
            onClick={logout}
            onMouseEnter={() => setLogoutHovered(true)}
            onMouseLeave={() => setLogoutHovered(false)}
            aria-label="Sign Out"
            style={{
              background: 'none', border: `1px solid ${logoutHovered ? tokens.error : tokens.border}`,
              color: logoutHovered ? tokens.error : tokens.muted,
              fontSize: 13, cursor: 'pointer', borderRadius: 6,
              padding: '4px 12px', transition: tokens.transition,
            }}>Sign Out</button>
        </div>
      </header>

      {!showContent ? <PageSkeleton /> : (
        <main style={{ maxWidth: 1080, margin: '0 auto', padding: '32px 24px' }}>
          {error && (
            <div style={{
              padding: '12px 16px', marginBottom: 24, borderRadius: 8,
              background: `${tokens.error}18`, border: `1px solid ${tokens.error}40`,
              color: tokens.error, fontSize: 14,
            }}>{error}</div>
          )}

          {/* Stats Row */}
          <div style={{
            display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(200px, 1fr))',
            gap: 12, marginBottom: 32,
          }}>
            <StatCard label="Projects" value={stats.projectCount} to="/" accent={tokens.cta} />
            <StatCard label="Available Skills" value={stats.skillCount} to="/admin/skills" accent="#8b5cf6" />
            <StatCard label="Pending Reviews" value={stats.pendingReviews} to="/review-inbox" accent={tokens.warning} />
            <StatCard label="Pipelines Active" value={0} to="/" accent="#3b82f6" />
          </div>

          {/* System Status Bar */}
          {systemStatus && (
            <div style={{
              display: 'flex', flexWrap: 'wrap', gap: 16, marginBottom: 24, padding: '10px 16px',
              background: tokens.surface, borderRadius: 8, border: `1px solid ${tokens.border}`,
              fontSize: 12, color: tokens.muted, alignItems: 'center',
            }}>
              <span style={{ display: 'flex', alignItems: 'center', gap: 6 }}>
                <span style={{ width: 8, height: 8, borderRadius: '50%', background: tokens.cta, display: 'inline-block' }} />
                {systemStatus.phase} · {systemStatus.profile}
              </span>
              <span style={{ color: tokens.border }}>|</span>
              <span>{systemStatus.skills} skills</span>
              <span style={{ color: tokens.border }}>|</span>
              <span>{systemStatus.models} models</span>
            </div>
          )}

          {/* Quick Nav */}
          <div style={{
            display: 'flex', flexWrap: 'wrap', gap: 8, marginBottom: 32,
            justifyContent: 'center',
          }}>
            {navLinks.map(link => (
              <NavIcon key={link.to} to={link.to} label={link.label}>
                {link.icon}
              </NavIcon>
            ))}
          </div>

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

          {projects.length === 0 ? (
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
        </main>
      )}
    </div>
  );
}
