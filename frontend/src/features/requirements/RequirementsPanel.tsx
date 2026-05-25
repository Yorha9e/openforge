import { useState, useEffect } from 'react';
import { tokens } from '../../shared/design-tokens';
import { api } from '../../shared/api';

interface RequirementItem {
  id: string;
  title: string;
  description: string;
  affectedModules: string[];
  status: 'draft' | 'clarifying' | 'approved' | 'implementing' | 'done';
}

interface Props {
  projectId: string;
}

const STATUS_COLORS: Record<string, string> = {
  draft: tokens.warning,
  clarifying: '#8b5cf6',
  approved: tokens.cta,
  implementing: '#f97316',
  done: tokens.muted,
};

export function RequirementsPanel({ projectId }: Props) {
  const [requirements, setRequirements] = useState<RequirementItem[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    setLoading(true);
    setError(null);
    api
      .getProject(projectId)
      .then((project: any) => {
        if (Array.isArray(project.requirements)) {
          setRequirements(project.requirements);
        } else {
          setRequirements([
            {
              id: projectId,
              title: project.title || project.name || 'Untitled',
              description: project.description || '',
              affectedModules: project.affected_modules || [],
              status: project.status || 'draft',
            },
          ]);
        }
        setLoading(false);
      })
      .catch((err: Error) => {
        setError(err.message);
        setLoading(false);
      });
  }, [projectId]);

  if (loading) {
    return (
      <div style={{ padding: 16 }}>
        <div role="status" aria-label="Loading" style={{
          height: 12, width: '60%', borderRadius: 4,
          background: tokens.border, opacity: 0.5,
        }} />
      </div>
    );
  }

  if (error) {
    return (
      <div style={{ padding: 16, color: tokens.error, fontSize: 13 }}>
        Failed to load requirements: {error}
      </div>
    );
  }

  if (requirements.length === 0) {
    return (
      <p style={{ padding: 16, color: tokens.muted, fontSize: 13, margin: 0 }}>
        No requirements defined yet.
      </p>
    );
  }

  return (
    <div role="region" aria-label="Requirements" style={{ padding: 16 }}>
      <h2 style={{
        fontSize: 16, fontWeight: 600, margin: '0 0 12px', padding: 0,
        fontFamily: tokens.fontHeading, color: tokens.text,
      }}>
        Requirements
      </h2>
      <div style={{ display: 'flex', flexDirection: 'column', gap: 10 }}>
        {requirements.map((req) => (
          <div
            key={req.id}
            role="listitem"
            style={{
              background: tokens.surface,
              border: `1px solid ${tokens.border}`,
              borderRadius: 8,
              padding: 12,
            }}
          >
            <div style={{ display: 'flex', alignItems: 'center', gap: 8, marginBottom: 6 }}>
              <span
                aria-hidden="true"
                style={{
                  width: 10, height: 10, borderRadius: '50%',
                  backgroundColor: STATUS_COLORS[req.status] || tokens.muted,
                  flexShrink: 0,
                }}
              />
              <span style={{ fontWeight: 600, fontSize: 14, color: tokens.text }}>
                {req.title}
              </span>
              <span style={{ fontSize: 11, color: tokens.muted, marginLeft: 'auto', textTransform: 'capitalize' }}>
                {req.status}
              </span>
            </div>
            {req.description && (
              <p style={{ margin: '0 0 8px', fontSize: 13, color: tokens.muted, lineHeight: 1.5 }}>
                {req.description}
              </p>
            )}
            {req.affectedModules.length > 0 && (
              <div style={{ display: 'flex', flexWrap: 'wrap', gap: 4 }}>
                {req.affectedModules.map((mod) => (
                  <span
                    key={mod}
                    style={{
                      fontSize: 11, padding: '2px 6px', borderRadius: 4,
                      background: tokens.bg, color: tokens.muted,
                      border: `1px solid ${tokens.border}`,
                    }}
                  >
                    {mod}
                  </span>
                ))}
              </div>
            )}
          </div>
        ))}
      </div>
    </div>
  );
}
