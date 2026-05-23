import { useEffect, useState } from 'react';
import { Link } from 'react-router-dom';
import { api } from '../../shared/api';
import { useAuth } from '../../shared/auth';
import { ProjectCard } from './ProjectCard';

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

  useEffect(() => {
    api.listProjects().then(setProjects).catch(console.error).finally(() => setLoading(false));
  }, []);

  return (
    <div style={{ minHeight: '100vh', background: '#0f0f0f', color: '#fff' }}>
      <header style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', padding: '12px 24px', borderBottom: '1px solid #262626' }}>
        <h1 style={{ fontSize: 18, fontWeight: 700 }}>OpenForge</h1>
        <div style={{ display: 'flex', alignItems: 'center', gap: 16 }}>
          <span style={{ color: '#a3a3a3', fontSize: 14 }}>{user?.id}</span>
          <button onClick={logout} style={{ background: 'none', border: 'none', color: '#a3a3a3', fontSize: 14, cursor: 'pointer' }}>Sign Out</button>
        </div>
      </header>
      <main style={{ maxWidth: 960, margin: '0 auto', padding: 24 }}>
        <h2 style={{ fontSize: 28, fontWeight: 700, marginBottom: 24 }}>Projects</h2>
        {loading ? <p style={{ color: '#a3a3a3' }}>Loading...</p>
        : projects.length === 0 ? <p style={{ color: '#a3a3a3' }}>No projects yet.</p>
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
