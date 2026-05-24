import { useState } from 'react';
import { tokens } from '../../shared/design-tokens';

interface Project {
  id: string;
  name: string;
  git_url: string;
  created_at: string;
  pipeline_count?: number;
}

export function ProjectCard({ project }: { project: Project }) {
  const [hovered, setHovered] = useState(false);

  return (
    <div
      onMouseEnter={() => setHovered(true)}
      onMouseLeave={() => setHovered(false)}
      style={{
        background: tokens.surface,
        border: `1px solid ${hovered ? tokens.cta : tokens.border}`,
        borderRadius: 8, padding: 20,
        transition: tokens.transition,
        position: 'relative',
        cursor: 'pointer',
      }}
    >
      {/* Arrow indicator on hover */}
      <div style={{
        position: 'absolute', top: 16, right: 16,
        color: tokens.cta, opacity: hovered ? 1 : 0,
        transform: hovered ? 'translateX(0)' : 'translateX(-8px)',
        transition: tokens.transition,
      }}>
        <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
          <path d="M5 12h14M12 5l7 7-7 7"/>
        </svg>
      </div>

      <div style={{ display: 'flex', alignItems: 'flex-start', gap: 12 }}>
        {/* Folder icon */}
        <div style={{
          width: 40, height: 40, borderRadius: 8, flexShrink: 0,
          background: hovered ? `${tokens.cta}20` : `${tokens.border}40`,
          display: 'flex', alignItems: 'center', justifyContent: 'center',
          color: hovered ? tokens.cta : tokens.muted,
          transition: tokens.transition,
        }}>
          <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round">
            <path d="M22 19a2 2 0 01-2 2H4a2 2 0 01-2-2V5a2 2 0 012-2h5l2 3h9a2 2 0 012 2z"/>
          </svg>
        </div>

        <div style={{ flex: 1, minWidth: 0 }}>
          <h3 style={{
            fontSize: 16, fontWeight: 600, color: tokens.text,
            fontFamily: tokens.fontHeading, margin: '0 0 4px 0',
            whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis',
          }}>{project.name}</h3>
          <p style={{
            color: tokens.muted, fontSize: 12, margin: 0,
            whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis',
          }}>{project.git_url}</p>
        </div>
      </div>

      <div style={{
        display: 'flex', gap: 12, marginTop: 16, paddingTop: 12,
        borderTop: `1px solid ${tokens.border}`,
        fontSize: 12, color: tokens.muted, alignItems: 'center',
      }}>
        <span style={{ display: 'flex', alignItems: 'center', gap: 4 }}>
          <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round"><rect x="3" y="4" width="18" height="18" rx="2" ry="2"/><line x1="16" y1="2" x2="16" y2="6"/><line x1="8" y1="2" x2="8" y2="6"/><line x1="3" y1="10" x2="21" y2="10"/></svg>
          {new Date(project.created_at).toLocaleDateString()}
        </span>
        {project.pipeline_count !== undefined && (
          <span style={{
            display: 'flex', alignItems: 'center', gap: 4,
            padding: '2px 8px', borderRadius: 12,
            background: project.pipeline_count > 0 ? `${tokens.cta}18` : `${tokens.muted}18`,
            color: project.pipeline_count > 0 ? tokens.cta : tokens.muted,
            fontSize: 11, fontWeight: 500,
          }}>
            {project.pipeline_count} pipeline{project.pipeline_count !== 1 ? 's' : ''}
          </span>
        )}
      </div>
    </div>
  );
}
