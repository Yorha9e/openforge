import { useEffect, useState } from 'react';
import { useParams, Link, useNavigate } from 'react-router-dom';
import { api } from '../../shared/api';
import { useToast } from '../../shared/toast';
import { tokens } from '../../shared/design-tokens';
import { PageSkeleton } from '../../shared/skeleton';

export function ProjectPage() {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const { toast } = useToast();
  const [project, setProject] = useState<any>(null);
  const [title, setTitle] = useState('');
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [createError, setCreateError] = useState<string | null>(null);
  const [inputFocused, setInputFocused] = useState(false);
  const [btnHovered, setBtnHovered] = useState(false);
  const [showContent, setShowContent] = useState(false);

  useEffect(() => {
    if (!id) return;
    const minDelay = new Promise(r => setTimeout(r, 600));
    const fetch = api.getProject(id)
      .then(setProject)
      .catch(err => setError(err instanceof Error ? err.message : 'Failed to load project'));
    Promise.all([fetch, minDelay]).finally(() => {
      setLoading(false);
      setShowContent(true);
    });
  }, [id]);

  const handleCreate = async () => {
    if (!id || !title.trim()) return;
    setCreateError(null);
    try {
      const pipe = await api.createPipeline(id, title.trim());
      navigate(`/project/${id}/chat?pipeline=${pipe.id}`);
    } catch (err) {
      const msg = err instanceof Error ? err.message : 'Failed to create pipeline';
      setCreateError(msg);
      toast(msg);
    }
  };

  if (!showContent) return <PageSkeleton cards={1} />;
  if (!project && !loading) return <div style={{ minHeight: '100vh', background: tokens.bg, color: tokens.text, padding: 24, fontFamily: tokens.fontBody }}>Project not found</div>;

  return (
    <div style={{ minHeight: '100vh', background: tokens.bg, color: tokens.text, fontFamily: tokens.fontBody }}>
      <header style={{ display: 'flex', alignItems: 'center', gap: 16, padding: '12px 24px', borderBottom: `1px solid ${tokens.border}` }}>
        <Link to="/" style={{ color: tokens.muted, textDecoration: 'none', transition: tokens.transition }}
          onMouseEnter={e => (e.currentTarget.style.color = tokens.text)}
          onMouseLeave={e => (e.currentTarget.style.color = tokens.muted)}>&larr; Back</Link>
        <h1 style={{ fontSize: 18, fontWeight: 700, fontFamily: tokens.fontHeading }}>{project.name}</h1>
      </header>
      <main style={{ maxWidth: 640, margin: '0 auto', padding: 24 }}>
        {error && <p style={{ color: tokens.error, fontSize: 14, marginBottom: 12 }}>{error}</p>}
        <div style={{ background: tokens.surface, border: `1px solid ${tokens.border}`, borderRadius: 8, padding: 24 }}>
          <h2 style={{ fontSize: 20, fontWeight: 700, marginBottom: 16, fontFamily: tokens.fontHeading }}>New Pipeline</h2>
          {createError && <p style={{ color: tokens.error, fontSize: 14, marginBottom: 12 }}>{createError}</p>}
          <div style={{ display: 'flex', gap: 8 }}>
            <input
              type="text"
              placeholder="What do you want to build?"
              value={title}
              onChange={e => setTitle(e.target.value)}
              onKeyDown={e => e.key === 'Enter' && handleCreate()}
              onFocus={() => setInputFocused(true)}
              onBlur={() => setInputFocused(false)}
              aria-label="Pipeline description"
              style={{
                flex: 1, padding: '8px 12px', background: tokens.bg, border: `1px solid ${tokens.border}`, borderRadius: 4, color: tokens.text,
                outline: inputFocused ? '2px solid' : 'none', outlineColor: tokens.cta, outlineOffset: 2,
                transition: tokens.transition,
              }} />
            <button
              onClick={handleCreate}
              onMouseEnter={() => setBtnHovered(true)}
              onMouseLeave={() => setBtnHovered(false)}
              aria-label="Start pipeline"
              style={{
                padding: '8px 16px', background: btnHovered ? tokens.ctaHover : tokens.cta, color: tokens.ctaText, border: 'none', borderRadius: 4, fontWeight: 500, cursor: 'pointer',
                transition: tokens.transition,
              }}>
              Start
            </button>
          </div>
        </div>
      </main>
    </div>
  );
}
