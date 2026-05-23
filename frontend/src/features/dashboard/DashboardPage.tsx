import { useEffect, useState } from 'react';
import { Link } from 'react-router-dom';
import { api } from '../../shared/api';
import { useAuth } from '../../shared/auth';
import { ProjectCard } from './ProjectCard';
import { tokens } from '../../shared/design-tokens';

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

  useEffect(() => {
    api.listProjects()
      .then(p => setProjects(Array.isArray(p) ? p : []))
      .catch(err => setError(err instanceof Error ? err.message : 'Failed to load projects'))
      .finally(() => setLoading(false));
  }, []);

  return (
    <div style={{ minHeight: '100vh', background: tokens.bg, color: tokens.text, fontFamily: tokens.fontBody }}>
      <header style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', padding: '12px 24px', borderBottom: `1px solid ${tokens.border}` }}>
        <h1 style={{ fontSize: 18, fontWeight: 700, fontFamily: tokens.fontHeading }}>OpenForge</h1>
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
        <h2 style={{ fontSize: 28, fontWeight: 700, marginBottom: 24, fontFamily: tokens.fontHeading }}>Projects</h2>
        {error && <p style={{ color: tokens.error, fontSize: 14, marginBottom: 12 }}>{error}</p>}
        {loading ? <p style={{ color: tokens.muted }}>Loading...</p>
        : projects.length === 0 ? <p style={{ color: tokens.muted }}>No projects yet.</p>
        : (
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
