import { useState } from 'react';
import { tokens } from '../../../shared/design-tokens';

interface Invitation {
  id: string;
  token: string;
  role: string;
  project_id: string;
  created_by: string;
  expires_at: string;
  used_at: string | null;
  used_by: string;
  created_at: string;
}

interface InvitationCardProps {
  invitation: Invitation;
  onDelete: (token: string) => void;
  baseUrl?: string;
}

export function InvitationCard({ invitation, onDelete, baseUrl = '' }: InvitationCardProps) {
  const [copied, setCopied] = useState(false);
  const [confirmDelete, setConfirmDelete] = useState(false);

  const isExpired = new Date(invitation.expires_at) < new Date();
  const isUsed = invitation.used_at !== null;
  const isActive = !isExpired && !isUsed;

  const inviteUrl = `${baseUrl}/invite?token=${invitation.token}`;

  const handleCopy = async () => {
    try {
      await navigator.clipboard.writeText(inviteUrl);
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    } catch {
      // Fallback for older browsers
      const textArea = document.createElement('textarea');
      textArea.value = inviteUrl;
      document.body.appendChild(textArea);
      textArea.select();
      document.execCommand('copy');
      document.body.removeChild(textArea);
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    }
  };

  const handleDelete = () => {
    if (confirmDelete) {
      onDelete(invitation.token);
      setConfirmDelete(false);
    } else {
      setConfirmDelete(true);
      setTimeout(() => setConfirmDelete(false), 3000);
    }
  };

  const statusColor = isActive ? tokens.cta : isUsed ? tokens.muted : tokens.error;
  const statusLabel = isActive ? 'Active' : isUsed ? 'Used' : 'Expired';

  return (
    <div
      style={{
        background: tokens.bg,
        border: `1px solid ${tokens.border}`,
        borderRadius: 8,
        padding: 16,
        marginBottom: 12,
      }}
    >
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start', marginBottom: 12 }}>
        <div>
          <span
            style={{
              display: 'inline-block',
              padding: '2px 8px',
              borderRadius: 4,
              fontSize: 12,
              fontWeight: 600,
              background: statusColor + '20',
              color: statusColor,
              border: `1px solid ${statusColor}40`,
            }}
          >
            {statusLabel}
          </span>
        </div>
        <div style={{ display: 'flex', gap: 8 }}>
          {isActive && (
            <button
              onClick={handleCopy}
              style={{
                padding: '4px 12px',
                background: copied ? tokens.cta : 'transparent',
                color: copied ? tokens.ctaText : tokens.cta,
                border: `1px solid ${tokens.cta}`,
                borderRadius: 4,
                fontSize: 12,
                cursor: 'pointer',
                transition: tokens.transition,
              }}
            >
              {copied ? 'Copied!' : 'Copy Link'}
            </button>
          )}
          <button
            onClick={handleDelete}
            style={{
              padding: '4px 12px',
              background: confirmDelete ? tokens.error : 'transparent',
              color: confirmDelete ? '#fff' : tokens.error,
              border: `1px solid ${tokens.error}`,
              borderRadius: 4,
              fontSize: 12,
              cursor: 'pointer',
              transition: tokens.transition,
            }}
          >
            {confirmDelete ? 'Confirm Delete' : 'Delete'}
          </button>
        </div>
      </div>

      <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 8, fontSize: 13 }}>
        <div>
          <span style={{ color: tokens.muted }}>Role: </span>
          <span style={{ color: tokens.text, fontWeight: 500 }}>{invitation.role}</span>
        </div>
        <div>
          <span style={{ color: tokens.muted }}>Project: </span>
          <span style={{ color: tokens.text, fontWeight: 500 }}>{invitation.project_id || 'Any'}</span>
        </div>
        <div>
          <span style={{ color: tokens.muted }}>Expires: </span>
          <span style={{ color: isExpired ? tokens.error : tokens.text }}>
            {new Date(invitation.expires_at).toLocaleDateString()}
          </span>
        </div>
        <div>
          <span style={{ color: tokens.muted }}>Created: </span>
          <span style={{ color: tokens.text }}>{new Date(invitation.created_at).toLocaleDateString()}</span>
        </div>
        {isUsed && invitation.used_by && (
          <div style={{ gridColumn: 'span 2' }}>
            <span style={{ color: tokens.muted }}>Used by: </span>
            <span style={{ color: tokens.text }}>{invitation.used_by}</span>
          </div>
        )}
      </div>

      {isActive && (
        <div
          style={{
            marginTop: 12,
            padding: 8,
            background: tokens.surface,
            borderRadius: 4,
            fontSize: 12,
            color: tokens.muted,
            wordBreak: 'break-all',
            fontFamily: 'monospace',
          }}
        >
          {inviteUrl}
        </div>
      )}
    </div>
  );
}
