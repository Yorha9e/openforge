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

export function ComplianceReportPage() {
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    // Future: load actual report data from API
    const t = setTimeout(() => {
      setLoading(false);
    }, 800);
    return () => clearTimeout(t);
  }, []);

  if (error) return <ErrorBanner message={error} />;

  return (
    <div style={{ padding: 24 }}>
      <h1 style={{ color: '#F1F5F9', fontSize: 20 }}>Compliance Report</h1>
      
      {loading ? (
        <div style={{ marginTop: 16 }}>
          <SkeletonRow />
          <SkeletonRow />
          <SkeletonRow />
        </div>
      ) : (
        <div style={{ marginTop: 12 }}>
          <p style={{ color: '#94A3B8', marginBottom: 16 }}>
            Automated compliance report generation coming soon.
          </p>
          
          {/* Placeholder for future compliance report content */}
          <div style={{
            backgroundColor: '#1E293B',
            borderRadius: 8,
            padding: 24,
            border: '1px solid #334155',
          }}>
            <h2 style={{ color: '#E2E8F0', fontSize: 16, marginBottom: 12 }}>
              Report Features (Coming Soon)
            </h2>
            <ul style={{ color: '#94A3B8', listStyleType: 'disc', paddingLeft: 20 }}>
              <li>Security compliance status</li>
              <li>Audit trail analysis</li>
              <li>Data retention compliance</li>
              <li>Access control review</li>
              <li>Export to PDF/CSV</li>
            </ul>
          </div>
        </div>
      )}
    </div>
  );
}