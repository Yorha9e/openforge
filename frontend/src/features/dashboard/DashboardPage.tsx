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
  const [sidebarOpen, setSidebarOpen] = useState(true);
  const [sidebarVisible, setSidebarVisible] = useState(false);
  const [showContent, setShowContent] = useState(false);
  const [activeNav, setActiveNav] = useState('/');

  useEffect(() => {
    const minDelay = new Promise(r => setTimeout(r, 600));
    const fetch = api.listProjects()
      .then(p => setProjects(Array.isArray(p) ? p : []))
      .catch(err => setError(err instanceof Error ? err.message : 'Failed to load projects'));
    Promise.all([fetch, minDelay]).finally(() => {
      setLoading(false);
      setShowContent(true);
    });
    // Trigger sidebar animation after mount
    requestAnimationFrame(() => setSidebarVisible(true));
  }, []);

  const sidebarWidth = 220;

  const sidebarItems = [
    { to: '/', label: 'Projects' },
    { to: '/review-inbox', label: 'Review Inbox' },
    { to: '/settings', label: 'Settings' },
  ];

  return (
    <div style={{ minHeight: '100vh', background: tokens.bg, color: tokens.text, fontFamily: tokens.fontBody, display: 'flex' }}>
      {/* Sidebar */}
      <aside style={{
        width: sidebarOpen ? sidebarWidth : 0,
        minWidth: sidebarOpen ? sidebarWidth : 0,
        background: '#0D1520',
        borderRight: sidebarOpen ? `1px solid ${tokens.border}` : 'none',
        display: 'flex', flexDirection: 'column',
        transition: 'width 250ms ease, min-width 250ms ease, border-color 250ms ease',
        overflow: 'hidden',
        opacity: sidebarVisible ? 1 : 0,
        transform: sidebarVisible ? 'translateX(0)' : 'translateX(-16px)',
        ...(sidebarVisible ? {} : { transition: 'none' }),
      }}>
        <div style={{ padding: '16px 20px', borderBottom: `1px solid ${tokens.border}`, display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
          <span style={{ fontFamily: tokens.fontHeading, fontSize: 16, fontWeight: 700, whiteSpace: 'nowrap' }}>OpenForge</span>
          <button
            onClick={() => setSidebarOpen(!sidebarOpen)}
            aria-label="Toggle sidebar"
            style={{ background: 'none', border: 'none', color: tokens.muted, cursor: 'pointer', fontSize: 16, padding: 0, lineHeight: 1 }}
          >◀</button>
        </div>
        <nav style={{ flex: 1, padding: '12px 0' }}>
          {sidebarItems.map(item => (
            <Link
              key={item.to}
              to={item.to}
              onClick={() => setActiveNav(item.to)}
              style={{
                display: 'block', padding: '10px 20px',
                color: activeNav === item.to ? tokens.text : tokens.muted,
                background: activeNav === item.to ? tokens.surface : 'transparent',
                borderLeft: activeNav === item.to ? `3px solid ${tokens.cta}` : '3px solid transparent',
                textDecoration: 'none', fontSize: 14, fontWeight: activeNav === item.to ? 600 : 400,
                transition: tokens.transition, whiteSpace: 'nowrap',
              }}
            >{item.label}</Link>
          ))}
        </nav>
        <div style={{ padding: '12px 20px', borderTop: `1px solid ${tokens.border}` }}>
          <span style={{ color: tokens.muted, fontSize: 12, whiteSpace: 'nowrap' }}>{user?.id}</span>
          <button
            onClick={logout}
            onMouseEnter={() => setLogoutHovered(true)}
            onMouseLeave={() => setLogoutHovered(false)}
            aria-label="Sign Out"
            style={{ display: 'block', marginTop: 4, background: 'none', border: 'none', color: logoutHovered ? tokens.text : tokens.muted, fontSize: 12, cursor: 'pointer', padding: 0, transition: tokens.transition }}
          >Sign Out</button>
        </div>
      </aside>

      {/* Toggle button when sidebar collapsed */}
      {!sidebarOpen && (
        <button
          onClick={() => setSidebarOpen(true)}
          aria-label="Open sidebar"
          style={{
            position: 'fixed', top: 12, left: 12, zIndex: 100,
            background: tokens.surface, border: `1px solid ${tokens.border}`, borderRadius: 6,
            color: tokens.muted, cursor: 'pointer', fontSize: 16, width: 36, height: 36,
            display: 'flex', alignItems: 'center', justifyContent: 'center',
            animation: 'of-fade-in 200ms ease-out both',
          }}
        >▶</button>
      )}

      {/* Main content */}
      <div className={showContent ? 'page-enter' : ''} style={{ flex: 1, display: 'flex', flexDirection: 'column', minWidth: 0 }}>
        <header style={{ padding: '12px 24px', borderBottom: `1px solid ${tokens.border}` }}>
          <h2 style={{ fontSize: 18, fontWeight: 700, fontFamily: tokens.fontHeading, margin: 0 }}>Projects</h2>
        </header>
        <main style={{ flex: 1, maxWidth: 960, margin: '0 auto', padding: 24, width: '100%' }}>
          {error && <p style={{ color: tokens.error, fontSize: 14, marginBottom: 12 }}>{error}</p>}
          {!showContent ? <PageSkeleton />
          : projects.length === 0 ? (
            <div style={{ textAlign: 'center', padding: '60px 0' }}>
              <p style={{ color: tokens.muted, fontSize: 16, marginBottom: 8 }}>No projects yet</p>
              <p style={{ color: tokens.muted, fontSize: 14 }}>
                Create a project from the sidebar or visit <Link to="/settings" style={{ color: tokens.cta }}>Settings</Link>.
              </p>
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
    </div>
  );
}
