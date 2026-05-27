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

export function GrafanaPage() {
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    // Future: load Grafana dashboard URL from API
    const t = setTimeout(() => {
      setLoading(false);
    }, 800);
    return () => clearTimeout(t);
  }, []);

  if (error) return <ErrorBanner message={error} />;

  return (
    <div style={{ padding: 24 }}>
      <h1 style={{ color: '#F1F5F9', fontSize: 20 }}>Monitoring Dashboard</h1>
      
      {loading ? (
        <div style={{ marginTop: 16 }}>
          <SkeletonRow />
          <SkeletonRow />
          <SkeletonRow />
        </div>
      ) : (
        <div style={{ marginTop: 12 }}>
          <p style={{ color: '#94A3B8', marginBottom: 16 }}>
            Grafana dashboard integration coming soon.
          </p>
          
          {/* Placeholder for future Grafana iframe */}
          <div style={{
            backgroundColor: '#1E293B',
            borderRadius: 8,
            padding: 24,
            border: '1px solid #334155',
            minHeight: 400,
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
          }}>
            <div style={{ textAlign: 'center', color: '#64748B' }}>
              <svg style={{ width: 48, height: 48, marginBottom: 16 }} fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M9 19v-6a2 2 0 00-2-2H5a2 2 0 00-2 2v6a2 2 0 002 2h2a2 2 0 002-2zm0 0V9a2 2 0 012-2h2a2 2 0 012 2v10m-6 0a2 2 0 002 2h2a2 2 0 002-2m0 0V5a2 2 0 012-2h2a2 2 0 012 2v14a2 2 0 01-2 2h-2a2 2 0 01-2-2z" />
              </svg>
              <h2 style={{ color: '#E2E8F0', fontSize: 16, marginBottom: 8 }}>
                Grafana Dashboard
              </h2>
              <p style={{ color: '#94A3B8', fontSize: 14 }}>
                Real-time metrics and monitoring visualizations will be embedded here.
              </p>
            </div>
          </div>
          
          <div style={{ marginTop: 24 }}>
            <h3 style={{ color: '#E2E8F0', fontSize: 14, marginBottom: 12 }}>
              Available Metrics (Coming Soon)
            </h3>
            <ul style={{ color: '#94A3B8', listStyleType: 'disc', paddingLeft: 20 }}>
              <li>API response times and latency percentiles</li>
              <li>Request throughput and error rates</li>
              <li>Database connection pool utilization</li>
              <li>System resource usage (CPU, memory, disk)</li>
              <li>Business metrics and KPIs</li>
            </ul>
          </div>
        </div>
      )}
    </div>
  );
}