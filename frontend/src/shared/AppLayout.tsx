import { useState, type ReactNode } from 'react';
import { Link, useLocation, useNavigate } from 'react-router-dom';
import { useAuth } from './auth';
import { tokens } from './design-tokens';

interface NavItem {
  path: string;
  label: string;
  icon: ReactNode;
  adminOnly?: boolean;
}

const NAV_ITEMS: NavItem[] = [
  {
    path: '/',
    label: 'Dashboard',
    icon: (
      <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
        <rect x="3" y="3" width="7" height="7" /><rect x="14" y="3" width="7" height="7" /><rect x="14" y="14" width="7" height="7" /><rect x="3" y="14" width="7" height="7" />
      </svg>
    ),
  },
  {
    path: '/review-inbox',
    label: 'Review Inbox',
    icon: (
      <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
        <path d="M22 12h-6l-2 3H10l-2-3H2"/><path d="M5.45 5.11L2 12v6a2 2 0 002 2h16a2 2 0 002-2v-6l-3.45-6.89A2 2 0 0016.76 4H7.24a2 2 0 00-1.79 1.11z"/>
      </svg>
    ),
  },
  {
    path: '/admin/skills',
    label: 'Skills',
    icon: (
      <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
        <polygon points="12 2 15.09 8.26 22 9.27 17 14.14 18.18 21.02 12 17.77 5.82 21.02 7 14.14 2 9.27 8.91 8.26 12 2"/>
      </svg>
    ),
  },
  {
    path: '/admin',
    label: 'Admin',
    adminOnly: true,
    icon: (
      <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
        <path d="M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10z"/>
      </svg>
    ),
  },
  {
    path: '/settings',
    label: 'Settings',
    icon: (
      <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
        <circle cx="12" cy="12" r="3"/><path d="M19.4 15a1.65 1.65 0 00.33 1.82l.06.06a2 2 0 010 2.83 2 2 0 01-2.83 0l-.06-.06a1.65 1.65 0 00-1.82-.33 1.65 1.65 0 00-1 1.51V21a2 2 0 01-4 0v-.09A1.65 1.65 0 009 19.4a1.65 1.65 0 00-1.82.33l-.06.06a2 2 0 01-2.83-2.83l.06-.06A1.65 1.65 0 004.68 15a1.65 1.65 0 00-1.51-1H3a2 2 0 010-4h.09A1.65 1.65 0 004.6 9a1.65 1.65 0 00-.33-1.82l-.06-.06a2 2 0 012.83-2.83l.06.06A1.65 1.65 0 009 4.68a1.65 1.65 0 001-1.51V3a2 2 0 014 0v.09a1.65 1.65 0 001 1.51 1.65 1.65 0 001.82-.33l.06-.06a2 2 0 012.83 2.83l-.06.06A1.65 1.65 0 0019.4 9a1.65 1.65 0 001.51 1H21a2 2 0 010 4h-.09a1.65 1.65 0 00-1.51 1z"/>
      </svg>
    ),
  },
];

export function AppLayout({ children, title, breadcrumbs }: { children: ReactNode; title?: string; breadcrumbs?: { label: string; to?: string }[] }) {
  const { user, logout } = useAuth();
  const location = useLocation();
  const navigate = useNavigate();
  const [sidebarOpen, setSidebarOpen] = useState(true);
  const [logoutHovered, setLogoutHovered] = useState(false);

  return (
    <div style={{ minHeight: '100vh', background: tokens.bg, color: tokens.text, fontFamily: tokens.fontBody, display: 'flex' }}>
      {/* Sidebar */}
      <aside style={{
        width: sidebarOpen ? 220 : 0,
        overflow: 'hidden',
        background: tokens.surface,
        borderRight: `1px solid ${tokens.border}`,
        display: 'flex', flexDirection: 'column',
        transition: 'width 200ms',
        flexShrink: 0,
      }}>
        {/* Brand */}
        <div style={{
          padding: '16px 20px', borderBottom: `1px solid ${tokens.border}`,
          display: 'flex', alignItems: 'center', gap: 10, minWidth: 220,
        }}>
          <span style={{ width: 8, height: 8, borderRadius: '50%', background: tokens.cta, display: 'inline-block', flexShrink: 0 }} />
          <span style={{ fontSize: 16, fontWeight: 700, fontFamily: tokens.fontHeading, color: tokens.cta, letterSpacing: '-0.5px', whiteSpace: 'nowrap' }}>
            OpenForge
          </span>
        </div>

        {/* Nav items */}
        <nav style={{ flex: 1, padding: '8px 0', minWidth: 220 }}>
          {NAV_ITEMS.filter(item => !item.adminOnly || user?.role === 'admin' || user?.role === 'superadmin').map(item => {
            const isActive = location.pathname === item.path ||
              (item.path !== '/' && location.pathname.startsWith(item.path));
            return (
              <Link
                key={item.path}
                to={item.path}
                style={{
                  display: 'flex', alignItems: 'center', gap: 10,
                  padding: '10px 20px', textDecoration: 'none',
                  color: isActive ? tokens.cta : tokens.muted,
                  background: isActive ? `${tokens.cta}10` : 'transparent',
                  borderLeft: `3px solid ${isActive ? tokens.cta : 'transparent'}`,
                  fontSize: 13, fontWeight: isActive ? 600 : 400,
                  transition: tokens.transition,
                  whiteSpace: 'nowrap',
                }}
                onMouseEnter={e => { if (!isActive) { e.currentTarget.style.color = tokens.text; e.currentTarget.style.background = `${tokens.border}30`; }}}
                onMouseLeave={e => { if (!isActive) { e.currentTarget.style.color = tokens.muted; e.currentTarget.style.background = 'transparent'; }}}
              >
                {item.icon}
                <span>{item.label}</span>
              </Link>
            );
          })}
        </nav>

        {/* Sidebar footer */}
        <div style={{ padding: '12px 20px', borderTop: `1px solid ${tokens.border}`, minWidth: 220 }}>
          <div style={{ fontSize: 11, color: tokens.muted }}>
            OpenForge v1.0
          </div>
        </div>
      </aside>

      {/* Main area */}
      <div style={{ flex: 1, display: 'flex', flexDirection: 'column', minWidth: 0 }}>
        {/* Top toolbar */}
        <header style={{
          display: 'flex', alignItems: 'center', gap: 12,
          padding: '0 24px', height: 52,
          borderBottom: `1px solid ${tokens.border}`,
          background: tokens.surface, flexShrink: 0,
        }}>
          {/* Toggle sidebar */}
          <button
            onClick={() => setSidebarOpen(!sidebarOpen)}
            aria-label={sidebarOpen ? 'Collapse sidebar' : 'Expand sidebar'}
            style={{
              background: 'none', border: 'none', color: tokens.muted, cursor: 'pointer',
              padding: 4, display: 'flex', alignItems: 'center',
              borderRadius: 4, transition: tokens.transition,
            }}
            onMouseEnter={e => { e.currentTarget.style.color = tokens.text; e.currentTarget.style.background = tokens.border; }}
            onMouseLeave={e => { e.currentTarget.style.color = tokens.muted; e.currentTarget.style.background = 'transparent'; }}
          >
            <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round">
              <line x1="3" y1="6" x2="21" y2="6" /><line x1="3" y1="12" x2="21" y2="12" /><line x1="3" y1="18" x2="21" y2="18" />
            </svg>
          </button>

          {/* Breadcrumbs */}
          <div style={{ display: 'flex', alignItems: 'center', gap: 6, flex: 1, minWidth: 0 }}>
            {breadcrumbs?.map((crumb, i) => (
              <span key={i} style={{ display: 'flex', alignItems: 'center', gap: 6, minWidth: 0 }}>
                {i > 0 && <span style={{ color: tokens.border, fontSize: 14 }}>/</span>}
                {crumb.to ? (
                  <Link to={crumb.to} style={{ color: tokens.muted, textDecoration: 'none', fontSize: 13, whiteSpace: 'nowrap', transition: tokens.transition }}
                    onMouseEnter={e => { e.currentTarget.style.color = tokens.text; }}
                    onMouseLeave={e => { e.currentTarget.style.color = tokens.muted; }}>
                    {crumb.label}
                  </Link>
                ) : (
                  <span style={{ color: tokens.text, fontSize: 13, fontWeight: 600, whiteSpace: 'nowrap' }}>{crumb.label}</span>
                )}
              </span>
            ))}
            {title && !breadcrumbs?.length && (
              <span style={{ color: tokens.text, fontSize: 13, fontWeight: 600 }}>{title}</span>
            )}
          </div>

          {/* User info */}
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
                fontSize: 12, cursor: 'pointer', borderRadius: 6,
                padding: '6px 14px', transition: tokens.transition, minHeight: 36,
              }}>Sign Out</button>
          </div>
        </header>

        {/* Content */}
        <main style={{ flex: 1, overflow: 'auto', padding: 24 }}>
          {children}
        </main>
      </div>
    </div>
  );
}
