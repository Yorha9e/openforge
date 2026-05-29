import { useEffect, useState } from 'react';

// G17: Loading skeleton component
function SkeletonRow() {
  return (
    <div style={{
      height: 16,
      backgroundColor: '#334155',
      borderRadius: 4,
      marginBottom: 12,
      animation: 'pulse 1.5s ease-in-out infinite',
    }} />
  );
}

// Error banner component
function ErrorBanner({ message }: { message: string }) {
  return (
    <div style={{
      padding: 12,
      backgroundColor: '#7F1D1D',
      border: '1px solid #DC2626',
      borderRadius: 6,
      color: '#FCA5A5',
      marginBottom: 16,
    }}>
      {message}
    </div>
  );
}

export default function ADRPage() {
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    // Future: load ADR documents from API or markdown files
    const t = setTimeout(() => {
      setLoading(false);
    }, 800);
    return () => clearTimeout(t);
  }, []);

  if (error) return <ErrorBanner message={error} />;

  return (
    <div style={{ padding: 24 }}>
      <h1 style={{ color: '#F1F5F9', fontSize: 20 }}>Architecture Decision Records</h1>
      
      {loading ? (
        <div style={{ marginTop: 16 }}>
          <SkeletonRow />
          <SkeletonRow />
          <SkeletonRow />
        </div>
      ) : (
        <div style={{ marginTop: 12 }}>
          <p style={{ color: '#94A3B8', marginBottom: 16 }}>
            Architecture Decision Records (ADRs) documenting key technical decisions.
          </p>
          
          {/* Placeholder for future ADR list */}
          <div style={{
            backgroundColor: '#1E293B',
            borderRadius: 8,
            padding: 24,
            border: '1px solid #334155',
          }}>
            <h2 style={{ color: '#E2E8F0', fontSize: 16, marginBottom: 12 }}>
              ADR Documents (Coming Soon)
            </h2>
            <p style={{ color: '#94A3B8', marginBottom: 16 }}>
              This page will render architecture decision records from the docs/adr/ directory.
            </p>
            
            <div style={{ marginTop: 16 }}>
              <h3 style={{ color: '#E2E8F0', fontSize: 14, marginBottom: 12 }}>
                Planned ADR Topics
              </h3>
              <ul style={{ color: '#94A3B8', listStyleType: 'disc', paddingLeft: 20 }}>
                <li>Database schema design decisions</li>
                <li>API versioning strategy</li>
                <li>Authentication and authorization patterns</li>
                <li>Deployment and infrastructure choices</li>
                <li>Technology stack selections</li>
              </ul>
            </div>
          </div>
          
          <div style={{ marginTop: 24 }}>
            <h3 style={{ color: '#E2E8F0', fontSize: 14, marginBottom: 12 }}>
              ADR Format
            </h3>
            <div style={{
              backgroundColor: '#1E293B',
              borderRadius: 8,
              padding: 24,
              border: '1px solid #334155',
              fontFamily: 'monospace',
              fontSize: 14,
              color: '#94A3B8',
            }}>
              <p># ADR-001: [Title]</p>
              <p><br /></p>
              <p>## Status</p>
              <p>Proposed | Accepted | Deprecated | Superseded</p>
              <p><br /></p>
              <p>## Context</p>
              <p>What is the issue that we're seeing that motivates this decision?</p>
              <p><br /></p>
              <p>## Decision</p>
              <p>What is the change that we're proposing and/or doing?</p>
              <p><br /></p>
              <p>## Consequences</p>
              <p>What becomes easier or more difficult because of this change?</p>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}