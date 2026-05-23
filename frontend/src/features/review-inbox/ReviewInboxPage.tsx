import { useEffect, useState } from 'react';
import { Link } from 'react-router-dom';
import { api } from '../../shared/api';

export function ReviewInboxPage() {
  const [items, setItems] = useState<any[]>([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    api.getReviewInbox().then(setItems).catch(console.error).finally(() => setLoading(false));
  }, []);

  return (
    <div style={{ minHeight: '100vh', background: '#0f0f0f', color: '#fff' }}>
      <header style={{ padding: '12px 24px', borderBottom: '1px solid #262626', display: 'flex', alignItems: 'center', gap: 16 }}>
        <Link to="/" style={{ color: '#a3a3a3', textDecoration: 'none' }}>&larr; Dashboard</Link>
        <h1 style={{ fontSize: 18, fontWeight: 700 }}>Review Inbox</h1>
      </header>
      <main style={{ maxWidth: 720, margin: '0 auto', padding: 24 }}>
        {loading ? <p style={{ color: '#a3a3a3' }}>Loading...</p>
        : items.length === 0 ? <p style={{ color: '#a3a3a3' }}>No pending reviews.</p>
        : items.map(item => (
          <div key={item.pipeline_id + item.stage} style={{ background: '#1a1a1a', border: '1px solid #262626', borderRadius: 8, padding: 16, marginBottom: 12 }}>
            <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
              <div>
                <p style={{ fontWeight: 600 }}>Pipeline {item.pipeline_id} — {item.stage}</p>
                <p style={{ color: '#a3a3a3', fontSize: 13 }}>Awaiting since {new Date(item.created_at).toLocaleString()}</p>
              </div>
              <Link to={`/project/${item.pipeline_id}/pipeline/${item.pipeline_id}`}
                style={{ padding: '6px 12px', background: '#2563eb', color: '#fff', borderRadius: 4, textDecoration: 'none', fontSize: 13 }}>
                Review
              </Link>
            </div>
          </div>
        ))}
      </main>
    </div>
  );
}
