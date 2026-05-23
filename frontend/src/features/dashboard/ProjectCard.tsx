interface Project {
  id: string;
  name: string;
  git_url: string;
  created_at: string;
  pipeline_count?: number;
}

export function ProjectCard({ project }: { project: Project }) {
  return (
    <div style={{ background: '#1a1a1a', border: '1px solid #262626', borderRadius: 8, padding: 16 }}>
      <h3 style={{ fontSize: 18, fontWeight: 600, color: '#fff' }}>{project.name}</h3>
      <p style={{ color: '#a3a3a3', fontSize: 14, marginTop: 4 }}>{project.git_url}</p>
      <div style={{ display: 'flex', gap: 16, marginTop: 12, fontSize: 12, color: '#737373' }}>
        <span>{new Date(project.created_at).toLocaleDateString()}</span>
        {project.pipeline_count !== undefined && <span>{project.pipeline_count} pipelines</span>}
      </div>
    </div>
  );
}
