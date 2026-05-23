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

export function DashboardPage() {
  const { user, logout } = useAuth();
  const [projects, setProjects] = useState<Project[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [logoutHovered, setLogoutHovered] = useState(false);
  const [navHovered, setNavHovered] = useState<string | null>(null);
  const [showContent, setShowContent] = useState(false);

  useEffect(() => {
    const minDelay = new Promise(r => setTimeout(r, 600));
    const fetch = api.listProjects()
      .then(p => setProjects(Array.isArray(p) ? p : []))
      .catch(err => setError(err instanceof Error ? err.message : 'Failed to load projects'));
    Promise.all([fetch, minDelay]).finally(() => {
      setLoading(false);
      setShowContent(true);
    });
  }, []);

  const navLink = (to: string, label: string) => (
    <Link
      key={to}
      to={to}
      onMouseEnter={() => setNavHovered(to)}
      onMouseLeave={() => setNavHovered(null)}
      style={{
        color: navHovered === to ? tokens.text : tokens.muted,
        textDecoration: 'none', fontSize: 14, transition: tokens.transition,
      }}
    >{label}</Link>
  );

  return (
    <div style={{ minHeight: '100vh', background: tokens.bg, color: tokens.text, fontFamily: tokens.fontBody }}>
      <header style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', padding: '12px 24px', borderBottom: `1px solid ${tokens.border}` }}>
        <div style={{ display: 'flex', alignItems: 'center', gap: 24 }}>
          <h1 style={{ fontSize: 18, fontWeight: 700, fontFamily: tokens.fontHeading, margin: 0 }}>OpenForge</h1>
          <nav style={{ display: 'flex', gap: 16 }}>
            {navLink('/review-inbox', 'Review Inbox')}
            {navLink('/settings', 'Settings')}
          </nav>
        </div>
        <div style={{ display: 'flex', alignItems: 'center', gap: 16 }}>
          <span style={{ color: tokens.muted, fontSize: 14 }}>{user?.id}</span>
          <button
            onClick={logout}
            onMouseEnter={() => setLogoutHovered(true)}
            onMouseLeave={() => setLogoutHovered(false)}
            aria-label="Sign Out"
            style={{ background: 'none', border: 'none', color: logoutHovered ? tokens.text : tokens.muted, fontSize: 14, cursor: 'pointer', transition: tokens.transition }}>Sign Out</button>
        </div>
      </header>
      <main style={{ maxWidth: 960, margin: '0 auto', padding: 24 }}>
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 24 }}>
          <h2 style={{ fontSize: 28, fontWeight: 700, fontFamily: tokens.fontHeading, margin: 0 }}>Projects</h2>
        </div>
        {error && <p style={{ color: tokens.error, fontSize: 14, marginBottom: 12 }}>{error}</p>}
        {!showContent ? <PageSkeleton />
        : projects.length === 0 ? (
          <div style={{ textAlign: 'center', padding: '60px 0' }}>
            <p style={{ color: tokens.muted, fontSize: 16, marginBottom: 8 }}>No projects yet</p>
            <p style={{ color: tokens.muted, fontSize: 14 }}>Create a project to get started.</p>
          </div>
        ) : (
          <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fill, minmax(320px, 1fr))', gap: 16 }}>
            {projects.map(p => (
              <Link key={p.id} to={`/project/${p.id}`} style={{ textDecoration: 'none' }}>
                <ProjectCard project={p} />
              </Link>
            ))}
          </div>
        )}
      </main>
    </div>
  );
}
