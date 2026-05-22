import { useEffect, useState } from 'react';
import { useParams, Link, useNavigate } from 'react-router-dom';
import { api } from '../../shared/api';

export function ProjectPage() {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const [project, setProject] = useState<any>(null);
  const [title, setTitle] = useState('');
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    if (!id) return;
    api.getProject(id).then(setProject).catch(console.error).finally(() => setLoading(false));
  }, [id]);

  const handleCreate = async () => {
    if (!id || !title.trim()) return;
    try {
      const pipe = await api.createPipeline(id, title.trim());
      navigate(`/project/${id}/chat?pipeline=${pipe.id}`);
    } catch (err) { console.error(err); }
  };

  if (loading) return <div style={{ minHeight: '100vh', background: '#0f0f0f', color: '#fff', padding: 24 }}>Loading...</div>;
  if (!project) return <div style={{ minHeight: '100vh', background: '#0f0f0f', color: '#fff', padding: 24 }}>Project not found</div>;

  return (
    <div style={{ minHeight: '100vh', background: '#0f0f0f', color: '#fff' }}>
      <header style={{ display: 'flex', alignItems: 'center', gap: 16, padding: '12px 24px', borderBottom: '1px solid #262626' }}>
        <Link to="/" style={{ color: '#a3a3a3', textDecoration: 'none' }}>&larr; Back</Link>
        <h1 style={{ fontSize: 18, fontWeight: 700 }}>{project.name}</h1>
      </header>
      <main style={{ maxWidth: 640, margin: '0 auto', padding: 24 }}>
        <div style={{ background: '#1a1a1a', border: '1px solid #262626', borderRadius: 8, padding: 24 }}>
          <h2 style={{ fontSize: 20, fontWeight: 700, marginBottom: 16 }}>New Pipeline</h2>
          <div style={{ display: 'flex', gap: 8 }}>
            <input type="text" placeholder="What do you want to build?" value={title}
              onChange={e => setTitle(e.target.value)}
              onKeyDown={e => e.key === 'Enter' && handleCreate()}
              style={{ flex: 1, padding: '8px 12px', background: '#262626', border: '1px solid #404040', borderRadius: 4, color: '#fff' }} />
            <button onClick={handleCreate}
              style={{ padding: '8px 16px', background: '#2563eb', color: '#fff', border: 'none', borderRadius: 4, fontWeight: 500, cursor: 'pointer' }}>
              Start
            </button>
          </div>
        </div>
      </main>
    </div>
  );
}
