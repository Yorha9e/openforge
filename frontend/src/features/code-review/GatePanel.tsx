import { useState } from 'react';
import { api } from '../../shared/api';

interface Props {
  pipelineId: string;
  stage: string;
}

export function GatePanel({ pipelineId, stage }: Props) {
  const [summary, setSummary] = useState('');
  const [loading, setLoading] = useState(false);

  const handleApprove = async () => {
    setLoading(true);
    try {
      await api.approveGate(pipelineId, stage,
        { code_reviewed: true, security_checked: true, license_cleared: true, coding_standard_met: true },
        summary,
      );
    } catch (err) { console.error(err); }
    finally { setLoading(false); }
  };

  const handleReject = async () => {
    setLoading(true);
    try {
      await api.rejectGate(pipelineId, stage, [], summary);
    } catch (err) { console.error(err); }
    finally { setLoading(false); }
  };

  return (
    <div style={{ padding: 16, color: '#fff' }}>
      <h3 style={{ fontSize: 16, fontWeight: 600, marginBottom: 12 }}>Gate Review — {stage}</h3>
      <textarea
        placeholder="Review summary..."
        value={summary}
        onChange={e => setSummary(e.target.value)}
        rows={3}
        style={{ width: '100%', padding: 8, background: '#262626', border: '1px solid #404040', borderRadius: 4, color: '#fff', resize: 'vertical', marginBottom: 12, boxSizing: 'border-box' }}
      />
      <div style={{ display: 'flex', gap: 8 }}>
        <button onClick={handleApprove} disabled={loading}
          style={{ flex: 1, padding: '8px 0', background: '#16a34a', color: '#fff', border: 'none', borderRadius: 4, fontWeight: 600, cursor: 'pointer' }}>
          Approve
        </button>
        <button onClick={handleReject} disabled={loading}
          style={{ flex: 1, padding: '8px 0', background: '#dc2626', color: '#fff', border: 'none', borderRadius: 4, fontWeight: 600, cursor: 'pointer' }}>
          Reject
        </button>
      </div>
    </div>
  );
}
