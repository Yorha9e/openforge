import { useState } from 'react';
import { api } from '../../shared/api';
import { tokens } from '../../shared/design-tokens';

interface Props {
  pipelineId: string;
  stage: string;
}

export function GatePanel({ pipelineId, stage }: Props) {
  const [summary, setSummary] = useState('');
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [textareaFocused, setTextareaFocused] = useState(false);
  const [approveHovered, setApproveHovered] = useState(false);
  const [rejectHovered, setRejectHovered] = useState(false);

  const handleApprove = async () => {
    setLoading(true);
    setError(null);
    try {
      await api.approveGate(pipelineId, stage,
        { code_reviewed: true, security_checked: true, license_cleared: true, coding_standard_met: true },
        summary,
      );
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Approval failed');
    }
    finally { setLoading(false); }
  };

  const handleReject = async () => {
    setLoading(true);
    setError(null);
    try {
      await api.rejectGate(pipelineId, stage, [], summary);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Rejection failed');
    }
    finally { setLoading(false); }
  };

  return (
    <div style={{ padding: 16, color: tokens.text, fontFamily: tokens.fontBody }}>
      <h3 style={{ fontSize: 16, fontWeight: 600, marginBottom: 12, fontFamily: tokens.fontHeading }}>Gate Review &mdash; {stage}</h3>
      {error && <p style={{ color: tokens.error, fontSize: 14, marginBottom: 12 }}>{error}</p>}
      <textarea
        placeholder="Review summary..."
        value={summary}
        onChange={e => setSummary(e.target.value)}
        rows={3}
        onFocus={() => setTextareaFocused(true)}
        onBlur={() => setTextareaFocused(false)}
        aria-label="Review summary"
        style={{
          width: '100%', padding: 8, background: tokens.bg, border: `1px solid ${tokens.border}`, borderRadius: 4, color: tokens.text,
          resize: 'vertical', marginBottom: 12, boxSizing: 'border-box', fontFamily: tokens.fontBody,
          outline: textareaFocused ? '2px solid' : 'none', outlineColor: tokens.cta, outlineOffset: 2,
          transition: tokens.transition,
        }}
      />
      <div style={{ display: 'flex', gap: 8 }}>
        <button
          onClick={handleApprove}
          disabled={loading}
          onMouseEnter={() => setApproveHovered(true)}
          onMouseLeave={() => setApproveHovered(false)}
          aria-label="Approve gate"
          style={{
            flex: 1, padding: '8px 0', background: approveHovered && !loading ? tokens.ctaHover : tokens.cta,
            color: tokens.ctaText, border: 'none', borderRadius: 4, fontWeight: 600, cursor: 'pointer',
            transition: tokens.transition,
          }}>
          Approve
        </button>
        <button
          onClick={handleReject}
          disabled={loading}
          onMouseEnter={() => setRejectHovered(true)}
          onMouseLeave={() => setRejectHovered(false)}
          aria-label="Reject gate"
          style={{
            flex: 1, padding: '8px 0', background: rejectHovered && !loading ? '#B91C1C' : tokens.error,
            color: '#fff', border: 'none', borderRadius: 4, fontWeight: 600, cursor: 'pointer',
            transition: tokens.transition,
          }}>
          Reject
        </button>
      </div>
    </div>
  );
}
