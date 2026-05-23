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
        background: tokens.surface, border: `1px solid ${hovered ? tokens.cta : tokens.border}`, borderRadius: 8, padding: 16,
        transition: tokens.transition,
      }}>
      <h3 style={{ fontSize: 18, fontWeight: 600, color: tokens.text, fontFamily: tokens.fontHeading }}>{project.name}</h3>
      <p style={{ color: tokens.muted, fontSize: 14, marginTop: 4 }}>{project.git_url}</p>
      <div style={{ display: 'flex', gap: 16, marginTop: 12, fontSize: 12, color: tokens.muted }}>
        <span>{new Date(project.created_at).toLocaleDateString()}</span>
        {project.pipeline_count !== undefined && <span>{project.pipeline_count} pipelines</span>}
      </div>
    </div>
  );
}
