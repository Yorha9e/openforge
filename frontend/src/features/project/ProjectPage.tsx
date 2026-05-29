import { useEffect, useState, useCallback } from 'react';
import { useParams, useNavigate, Link } from 'react-router-dom';
import { api } from '../../shared/api';
import { AppLayout } from '../../shared/AppLayout';
import { useToast } from '../../shared/toast';
import { tokens } from '../../shared/design-tokens';
import { PageSkeleton } from '../../shared/skeleton';

interface PipelineSummary {
  id: string;
  title: string;
  status: string;
  current_stage: string;
  level: string;
  created_at: string;
  created_by: string;
}

const STAGE_LABELS: Record<string, string> = {
  clarify: 'Clarify',
  decompose: 'Decompose',
  impl: 'Implementation',
  test: 'Testing',
  review: 'Review',
  deploy: 'Deploy',
  verify: 'Verify',
};

const STATUS_COLORS: Record<string, string> = {
  running: tokens.cta,
  pending: tokens.warning,
  paused: tokens.warning,
  completed: '#3b82f6',
  rejected: tokens.error,
  cancelled: tokens.muted,
};

export function ProjectPage() {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const { toast } = useToast();
  const [project, setProject] = useState<any>(null);
  const [pipelines, setPipelines] = useState<PipelineSummary[]>([]);
  const [loading, setLoading] = useState(true);
  const [title, setTitle] = useState('');
  const [creating, setCreating] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [pipeLineFilter, setPipelineFilter] = useState<'all' | 'active' | 'completed'>('all');
  const [showContent, setShowContent] = useState(false);

  useEffect(() => {
    if (!id) return;
    const minDelay = new Promise(r => setTimeout(r, 600));
    const fetch = Promise.all([
      api.getProject(id).then(setProject),
      api.listPipelines(id).then(p => setPipelines(Array.isArray(p) ? p : [])),
    ]).catch(err => setError(err instanceof Error ? err.message : 'Failed to load project'));
    Promise.all([fetch, minDelay]).finally(() => {
      setLoading(false);
      setShowContent(true);
    });
  }, [id]);

  const handleCreatePipeline = async () => {
    if (!id || !title.trim()) return;
    setCreating(true);
    try {
      const pipe = await api.createPipeline(id, title.trim());
      setPipelines(prev => [pipe, ...prev]);
      setTitle('');
      toast('Pipeline created');
      navigate(`/project/${id}/chat?pipeline=${pipe.id}`);
    } catch (err) {
      const msg = err instanceof Error ? err.message : 'Failed to create pipeline';
      setError(msg);
      toast(msg);
    } finally { setCreating(false); }
  };

  const handleContinueChat = (pipeline: PipelineSummary) => {
    navigate(`/project/${id}/chat?pipeline=${pipeline.id}`);
  };

  const handleProMode = (pipeline: PipelineSummary) => {
    navigate(`/project/${id}/pipeline/${pipeline.id}`);
  };

  const handleDeletePipeline = useCallback(async (pipeline: PipelineSummary) => {
    const confirmed = window.confirm(`Are you sure you want to delete the pipeline "${pipeline.title}"?\nThis action cannot be undone.`);
    if (!confirmed) return;
    try {
      await api.deletePipeline(pipeline.id);
      setPipelines(prev => prev.filter(p => p.id !== pipeline.id));
      toast('Pipeline deleted');
    } catch (err) {
      const msg = err instanceof Error ? err.message : 'Failed to delete pipeline';
      toast(msg);
    }
  }, [toast]);

  const [deletingProject, setDeletingProject] = useState(false);

  const handleDeleteProject = useCallback(async () => {
    if (!id || !project) return;
    const confirmed = window.confirm(
      `Are you sure you want to delete the project "${project.name}"?\n\n` +
      `This will soft-delete all pipelines in this project. Data is recoverable for 30 days.`
    );
    if (!confirmed) return;

    // Second confirmation: type project name
    const typed = window.prompt(`Type "${project.name}" to confirm permanent deletion:`)?.trim();
    if (typed !== project.name) {
      toast('Project name did not match. Deletion cancelled.');
      return;
    }

    setDeletingProject(true);
    try {
      await api.deleteProject(id);
      toast('Project deleted. Redirecting...');
      navigate('/');
    } catch (err) {
      const msg = err instanceof Error ? err.message : 'Failed to delete project';
      toast(msg);
    } finally {
      setDeletingProject(false);
    }
  }, [id, project, toast, navigate]);

  const filteredPipelines = pipelines.filter(p => {
    if (pipeLineFilter === 'active') return ['running', 'pending', 'paused'].includes(p.status);
    if (pipeLineFilter === 'completed') return ['completed', 'rejected', 'cancelled'].includes(p.status);
    return true;
  });

  const activeCount = pipelines.filter(p => ['running', 'pending', 'paused'].includes(p.status)).length;

  return (
    <AppLayout breadcrumbs={[
      { label: 'Dashboard', to: '/' },
      { label: project?.name || 'Project' },
    ]}>
      {!showContent ? <PageSkeleton cards={2} /> : (
        <>
          {/* Project header */}
          {project && (
            <div style={{
              display: 'flex', alignItems: 'center', justifyContent: 'space-between',
              marginBottom: 24, paddingBottom: 20,
              borderBottom: `1px solid ${tokens.border}`,
            }}>
              <div>
                <h1 style={{ fontSize: 22, fontWeight: 700, fontFamily: tokens.fontHeading, margin: '0 0 4px 0', color: tokens.text }}>
                  {project.name}
                </h1>
                <span style={{ fontSize: 13, color: tokens.muted }}>
                  {project.git_url}
                </span>
              </div>
              <div style={{ display: 'flex', gap: 8 }}>
                <Link to={`/project/${id}/costs`}
                  style={{ padding: '8px 14px', background: tokens.surface, color: tokens.muted, borderRadius: 6, textDecoration: 'none', fontSize: 13, border: `1px solid ${tokens.border}`, transition: tokens.transition }}
                  onMouseEnter={e => { e.currentTarget.style.color = tokens.text; e.currentTarget.style.borderColor = tokens.cta; }}
                  onMouseLeave={e => { e.currentTarget.style.color = tokens.muted; e.currentTarget.style.borderColor = tokens.border; }}>
                  Cost Dashboard
                </Link>
              </div>
            </div>
          )}

          {error && (
            <div style={{
              padding: '12px 16px', marginBottom: 24, borderRadius: 8,
              background: `${tokens.error}18`, border: `1px solid ${tokens.error}40`,
              color: tokens.error, fontSize: 14,
            }}>{error}</div>
          )}

          {/* New Pipeline form */}
          <div style={{
            background: tokens.surface, border: `1px solid ${tokens.border}`, borderRadius: 8,
            padding: 20, marginBottom: 24,
          }}>
            <h2 style={{ fontSize: 16, fontWeight: 700, fontFamily: tokens.fontHeading, margin: '0 0 12px 0', color: tokens.text }}>
              New Pipeline
            </h2>
            <div style={{ display: 'flex', gap: 8 }}>
              <input
                type="text"
                placeholder="What do you want to build?"
                value={title}
                onChange={e => setTitle(e.target.value)}
                onKeyDown={e => e.key === 'Enter' && handleCreatePipeline()}
                aria-label="Pipeline description"
                style={{
                  flex: 1, padding: '8px 12px', background: tokens.bg,
                  border: `1px solid ${tokens.border}`, borderRadius: 4, color: tokens.text,
                  fontSize: 14, outline: 'none', minHeight: 36,
                  transition: tokens.transition,
                }}
                onFocus={e => { e.currentTarget.style.borderColor = tokens.cta; }}
                onBlur={e => { e.currentTarget.style.borderColor = tokens.border; }}
                autoFocus
              />
              <button
                onClick={handleCreatePipeline}
                disabled={creating || !title.trim()}
                style={{
                  padding: '8px 20px', background: tokens.cta, color: tokens.ctaText,
                  border: 'none', borderRadius: 4, fontWeight: 500, fontSize: 13,
                  cursor: creating || !title.trim() ? 'default' : 'pointer',
                  opacity: creating || !title.trim() ? 0.5 : 1,
                  transition: tokens.transition, minHeight: 36,
                }}>
                {creating ? 'Creating...' : 'Start'}
              </button>
            </div>
          </div>

          {/* Pipeline list */}
          <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 16 }}>
            <h2 style={{ fontSize: 18, fontWeight: 700, fontFamily: tokens.fontHeading, margin: 0, color: tokens.text }}>
              Pipelines {activeCount > 0 && <span style={{ fontSize: 13, color: tokens.cta, fontWeight: 400, fontFamily: tokens.fontBody }}>({activeCount} active)</span>}
            </h2>
            <div style={{ display: 'flex', gap: 4 }}>
              {(['all', 'active', 'completed'] as const).map(f => (
                <button key={f} onClick={() => setPipelineFilter(f)}
                  style={{
                    padding: '4px 12px', background: pipeLineFilter === f ? tokens.cta : tokens.surface,
                    color: pipeLineFilter === f ? tokens.ctaText : tokens.muted,
                    border: `1px solid ${pipeLineFilter === f ? tokens.cta : tokens.border}`,
                    borderRadius: 4, cursor: 'pointer', fontSize: 12, fontWeight: 500,
                    transition: tokens.transition,
                  }}>
                  {f === 'all' ? 'All' : f === 'active' ? 'Active' : 'Completed'}
                </button>
              ))}
            </div>
          </div>

          {filteredPipelines.length === 0 ? (
            <div style={{
              textAlign: 'center', padding: '48px 0', color: tokens.muted,
              background: tokens.surface, borderRadius: 8, border: `1px solid ${tokens.border}`,
            }}>
              <p style={{ fontSize: 14, margin: 0 }}>
                {pipelines.length === 0 ? 'No pipelines yet. Create one above to get started.' : 'No pipelines match the selected filter.'}
              </p>
            </div>
          ) : (
            <div style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
              {filteredPipelines.map(p => {
                const isActive = ['running', 'pending', 'paused'].includes(p.status);
                return (
                  <div key={p.id} style={{
                    background: tokens.surface, border: `1px solid ${tokens.border}`, borderRadius: 8,
                    padding: '14px 16px', display: 'flex', alignItems: 'center', gap: 16,
                    transition: tokens.transition,
                  }}
                    onMouseEnter={e => { e.currentTarget.style.borderColor = tokens.cta; }}
                    onMouseLeave={e => { e.currentTarget.style.borderColor = tokens.border; }}
                  >
                    {/* Status indicator */}
                    <div style={{
                      width: 10, height: 10, borderRadius: '50%', flexShrink: 0,
                      background: STATUS_COLORS[p.status] || tokens.muted,
                    }} />

                    {/* Pipeline info */}
                    <div style={{ flex: 1, minWidth: 0 }}>
                      <div style={{
                        fontSize: 14, fontWeight: 600, color: tokens.text,
                        whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis',
                      }}>
                        {p.title}
                      </div>
                      <div style={{ fontSize: 12, color: tokens.muted, marginTop: 2 }}>
                        <span style={{ textTransform: 'capitalize' }}>{p.status}</span>
                        {p.current_stage && <span> · Stage: {STAGE_LABELS[p.current_stage] || p.current_stage}</span>}
                        <span> · {new Date(p.created_at).toLocaleDateString()}</span>
                      </div>
                    </div>

                    {/* Actions */}
                    <div style={{ display: 'flex', gap: 6, flexShrink: 0 }}>
                      <button onClick={() => handleContinueChat(p)}
                        style={{
                          padding: '6px 14px', background: isActive ? tokens.cta : tokens.surface,
                          color: isActive ? tokens.ctaText : tokens.muted,
                          border: `1px solid ${isActive ? tokens.cta : tokens.border}`,
                          borderRadius: 4, cursor: 'pointer', fontSize: 12, fontWeight: 500,
                          transition: tokens.transition,
                        }}
                        onMouseEnter={e => { e.currentTarget.style.background = isActive ? tokens.ctaHover : tokens.surface; e.currentTarget.style.borderColor = tokens.cta; }}
                        onMouseLeave={e => { e.currentTarget.style.background = isActive ? tokens.cta : tokens.surface; e.currentTarget.style.borderColor = isActive ? tokens.cta : tokens.border; }}>
                        {isActive ? 'Continue' : 'View Chat'}
                      </button>
                      <button onClick={() => handleProMode(p)}
                        style={{
                          padding: '6px 14px', background: tokens.surface, color: tokens.muted,
                          border: `1px solid ${tokens.border}`, borderRadius: 4, cursor: 'pointer', fontSize: 12,
                          transition: tokens.transition,
                        }}
                        onMouseEnter={e => { e.currentTarget.style.color = tokens.text; e.currentTarget.style.borderColor = tokens.cta; }}
                        onMouseLeave={e => { e.currentTarget.style.color = tokens.muted; e.currentTarget.style.borderColor = tokens.border; }}>
                        Pro Mode
                      </button>
                      <button onClick={() => handleDeletePipeline(p)}
                        style={{
                          padding: '6px 14px', background: 'transparent', color: tokens.error,
                          border: `1px solid ${tokens.error}40`, borderRadius: 4, cursor: 'pointer', fontSize: 12,
                          transition: tokens.transition,
                        }}
                        onMouseEnter={e => { e.currentTarget.style.background = `${tokens.error}15`; }}
                        onMouseLeave={e => { e.currentTarget.style.background = 'transparent'; }}>
                        Delete
                      </button>
                    </div>
                  </div>
                );
              })}
            </div>
          )}

          {/* Danger Zone */}
          {project && (
            <div style={{
              marginTop: 40, paddingTop: 24,
              borderTop: `1px solid ${tokens.border}`,
            }}>
              <h2 style={{
                fontSize: 14, fontWeight: 700, fontFamily: tokens.fontHeading,
                margin: '0 0 4px 0', color: tokens.error,
              }}>
                Danger Zone
              </h2>
              <p style={{ fontSize: 13, color: tokens.muted, margin: '0 0 16px 0' }}>
                Irreversible actions. Please proceed with caution.
              </p>
              <div style={{
                background: `${tokens.error}08`, border: `1px solid ${tokens.error}30`,
                borderRadius: 8, padding: '16px 20px',
                display: 'flex', alignItems: 'center', justifyContent: 'space-between',
              }}>
                <div>
                  <div style={{ fontSize: 14, fontWeight: 600, color: tokens.text }}>
                    Delete this project
                  </div>
                  <div style={{ fontSize: 12, color: tokens.muted, marginTop: 2 }}>
                    Soft-deletes the project and all pipelines. Recoverable for 30 days.
                  </div>
                </div>
                <button
                  onClick={handleDeleteProject}
                  disabled={deletingProject}
                  style={{
                    padding: '8px 18px', background: 'transparent',
                    color: tokens.error, border: `1px solid ${tokens.error}60`,
                    borderRadius: 6, cursor: deletingProject ? 'default' : 'pointer',
                    fontSize: 13, fontWeight: 600, transition: tokens.transition,
                    opacity: deletingProject ? 0.5 : 1,
                  }}
                  onMouseEnter={e => {
                    if (!deletingProject) {
                      e.currentTarget.style.background = `${tokens.error}15`;
                      e.currentTarget.style.borderColor = tokens.error;
                    }
                  }}
                  onMouseLeave={e => {
                    e.currentTarget.style.background = 'transparent';
                    e.currentTarget.style.borderColor = `${tokens.error}60`;
                  }}
                >
                  {deletingProject ? 'Deleting...' : 'Delete Project'}
                </button>
              </div>
            </div>
          )}
        </>
      )}
    </AppLayout>
  );
}
