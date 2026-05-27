import { useEffect, useState } from 'react';
import { Link } from 'react-router-dom';
import { api } from '../../shared/api';
import { AppLayout } from '../../shared/AppLayout';
import { tokens } from '../../shared/design-tokens';
import { PageSkeleton } from '../../shared/skeleton';

export default function ReviewInboxPage() {
  const [items, setItems] = useState<any[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [showContent, setShowContent] = useState(false);

  useEffect(() => {
    const minDelay = new Promise(r => setTimeout(r, 600));
    const fetch = api.getReviewInbox()
      .then(setItems)
      .catch(err => setError(err instanceof Error ? err.message : 'Failed to load reviews'));
    Promise.all([fetch, minDelay]).finally(() => {
      setLoading(false);
      setShowContent(true);
    });
  }, []);

  if (loading) return <PageSkeleton cards={1} />;

  return (
    <AppLayout>
      {error && <p style={{ color: tokens.error, fontSize: 14, marginBottom: 12 }}>{error}</p>}
      {!showContent ? <PageSkeleton cards={2} />
      : items.length === 0 ? (
        <div style={{
          textAlign: 'center', padding: '60px 0', color: tokens.muted,
          background: tokens.surface, borderRadius: 8, border: `1px solid ${tokens.border}`,
        }}>
          <p style={{ fontSize: 16, fontWeight: 500, margin: 0 }}>No pending reviews.</p>
        </div>
      ) : (
        <div style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
          {items.map(item => (
            <div key={item.pipeline_id + item.stage} style={{
              background: tokens.surface, border: `1px solid ${tokens.border}`, borderRadius: 8,
              padding: 16, display: 'flex', justifyContent: 'space-between', alignItems: 'center',
              transition: tokens.transition,
            }}>
              <div>
                <p style={{ fontWeight: 600, margin: '0 0 4px 0', fontSize: 14 }}>Pipeline {item.pipeline_id} &mdash; {item.stage}</p>
                <p style={{ color: tokens.muted, fontSize: 13, margin: 0 }}>Awaiting since {new Date(item.created_at).toLocaleString()}</p>
              </div>
              <Link to={`/project/${item.pipeline_id}/pipeline/${item.pipeline_id}`}
                style={{
                  padding: '6px 14px', background: tokens.cta, color: tokens.ctaText,
                  borderRadius: 4, textDecoration: 'none', fontSize: 13, fontWeight: 500,
                  transition: tokens.transition,
                }}>
                Review
              </Link>
            </div>
          ))}
        </div>
      )}
    </AppLayout>
  );
}
