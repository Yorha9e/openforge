import { useEffect, useState } from 'react';
import { Link } from 'react-router-dom';
import { api } from '../../shared/api';
import { tokens } from '../../shared/design-tokens';
import { PageSkeleton } from '../../shared/skeleton';

export function ReviewInboxPage() {
  const [items, setItems] = useState<any[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    api.getReviewInbox()
      .then(setItems)
      .catch(err => setError(err instanceof Error ? err.message : 'Failed to load reviews'))
      .finally(() => setLoading(false));
  }, []);

  return (
    <div style={{ minHeight: '100vh', background: tokens.bg, color: tokens.text, fontFamily: tokens.fontBody }}>
      <header style={{ padding: '12px 24px', borderBottom: `1px solid ${tokens.border}`, display: 'flex', alignItems: 'center', gap: 16 }}>
        <Link to="/" style={{ color: tokens.muted, textDecoration: 'none', transition: tokens.transition }}
          onMouseEnter={e => (e.currentTarget.style.color = tokens.text)}
          onMouseLeave={e => (e.currentTarget.style.color = tokens.muted)}>&larr; Dashboard</Link>
        <h1 style={{ fontSize: 18, fontWeight: 700, fontFamily: tokens.fontHeading }}>Review Inbox</h1>
      </header>
      <main style={{ maxWidth: 720, margin: '0 auto', padding: 24 }}>
        {error && <p style={{ color: tokens.error, fontSize: 14, marginBottom: 12 }}>{error}</p>}
        {loading ? <PageSkeleton cards={2} />
        : items.length === 0 ? <p style={{ color: tokens.muted }}>No pending reviews.</p>
        : items.map(item => (
          <div key={item.pipeline_id + item.stage} style={{ background: tokens.surface, border: `1px solid ${tokens.border}`, borderRadius: 8, padding: 16, marginBottom: 12 }}>
            <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
              <div>
                <p style={{ fontWeight: 600 }}>Pipeline {item.pipeline_id} &mdash; {item.stage}</p>
                <p style={{ color: tokens.muted, fontSize: 13 }}>Awaiting since {new Date(item.created_at).toLocaleString()}</p>
              </div>
              <Link to={`/project/${item.pipeline_id}/pipeline/${item.pipeline_id}`}
                aria-label={`Review pipeline ${item.pipeline_id} stage ${item.stage}`}
                style={{ padding: '6px 12px', background: tokens.cta, color: tokens.ctaText, borderRadius: 4, textDecoration: 'none', fontSize: 13, transition: tokens.transition }}>
                Review
              </Link>
            </div>
          </div>
        ))}
      </main>
    </div>
  );
}
