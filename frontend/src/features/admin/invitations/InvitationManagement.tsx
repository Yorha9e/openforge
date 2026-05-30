import { useState, useEffect, type FormEvent } from 'react';
import { tokens } from '../../../shared/design-tokens';
import { api } from '../../../shared/api';
import { useToast } from '../../../shared/toast';
import { InvitationCard } from './InvitationCard';

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

const roles = [
  { value: 'admin', label: 'Admin' },
  { value: 'pm', label: 'Product Manager' },
  { value: 'dev_lead', label: 'Dev Lead' },
  { value: 'dev', label: 'Developer' },
  { value: 'observer', label: 'Observer' },
];

export default function InvitationManagement() {
  const { toast } = useToast();
  const [invitations, setInvitations] = useState<Invitation[]>([]);
  const [loading, setLoading] = useState(true);
  const [showCreateForm, setShowCreateForm] = useState(false);
  const [creating, setCreating] = useState(false);
  const [role, setRole] = useState('dev');
  const [projectId, setProjectId] = useState('');
  const [expiresInDays, setExpiresInDays] = useState(7);

  const fetchInvitations = async () => {
    try {
      setLoading(true);
      const result = await api.listInvitations();
      setInvitations(result.data || []);
    } catch (err) {
      toast(err instanceof Error ? err.message : 'Failed to load invitations', 'error');
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchInvitations();
  }, []);

  const handleCreate = async (e: FormEvent) => {
    e.preventDefault();
    setCreating(true);
    try {
      await api.createInvitation(role, projectId, expiresInDays);
      toast('Invitation created successfully', 'success');
      setShowCreateForm(false);
      setRole('dev');
      setProjectId('');
      setExpiresInDays(7);
      await fetchInvitations();
    } catch (err) {
      toast(err instanceof Error ? err.message : 'Failed to create invitation', 'error');
    } finally {
      setCreating(false);
    }
  };

  const handleDelete = async (token: string) => {
    try {
      await api.deleteInvitation(token);
      toast('Invitation deleted', 'success');
      setInvitations(prev => prev.filter(inv => inv.token !== token));
    } catch (err) {
      toast(err instanceof Error ? err.message : 'Failed to delete invitation', 'error');
    }
  };

  return (
    <div style={{ padding: 24, maxWidth: 800, margin: '0 auto' }}>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 24 }}>
        <h1
          style={{
            fontSize: 24,
            fontWeight: 700,
            color: tokens.text,
            fontFamily: tokens.fontHeading,
            margin: 0,
          }}
        >
          Invitation Management
        </h1>
        <button
          onClick={() => setShowCreateForm(!showCreateForm)}
          style={{
            padding: '8px 16px',
            background: tokens.cta,
            color: tokens.ctaText,
            border: 'none',
            borderRadius: 4,
            fontWeight: 500,
            cursor: 'pointer',
            fontSize: 14,
          }}
        >
          {showCreateForm ? 'Cancel' : 'Create Invitation'}
        </button>
      </div>

      {showCreateForm && (
        <form
          onSubmit={handleCreate}
          style={{
            background: tokens.surface,
            padding: 24,
            borderRadius: 8,
            marginBottom: 24,
            border: `1px solid ${tokens.border}`,
          }}
        >
          <h2
            style={{
              fontSize: 18,
              fontWeight: 600,
              color: tokens.text,
              marginTop: 0,
              marginBottom: 16,
            }}
          >
            New Invitation
          </h2>
          <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr 1fr', gap: 16 }}>
            <div>
              <label
                style={{
                  display: 'block',
                  fontSize: 13,
                  color: tokens.muted,
                  marginBottom: 4,
                  fontWeight: 500,
                }}
              >
                Role
              </label>
              <select
                value={role}
                onChange={e => setRole(e.target.value)}
                style={{
                  width: '100%',
                  padding: '8px 12px',
                  background: tokens.bg,
                  border: `1px solid ${tokens.border}`,
                  borderRadius: 4,
                  color: tokens.text,
                  fontSize: 14,
                  boxSizing: 'border-box',
                }}
              >
                {roles.map(r => (
                  <option key={r.value} value={r.value}>{r.label}</option>
                ))}
              </select>
            </div>
            <div>
              <label
                style={{
                  display: 'block',
                  fontSize: 13,
                  color: tokens.muted,
                  marginBottom: 4,
                  fontWeight: 500,
                }}
              >
                Project ID (optional)
              </label>
              <input
                type="text"
                value={projectId}
                onChange={e => setProjectId(e.target.value)}
                placeholder="Leave empty for any"
                style={{
                  width: '100%',
                  padding: '8px 12px',
                  background: tokens.bg,
                  border: `1px solid ${tokens.border}`,
                  borderRadius: 4,
                  color: tokens.text,
                  fontSize: 14,
                  boxSizing: 'border-box',
                }}
              />
            </div>
            <div>
              <label
                style={{
                  display: 'block',
                  fontSize: 13,
                  color: tokens.muted,
                  marginBottom: 4,
                  fontWeight: 500,
                }}
              >
                Expires in (days)
              </label>
              <input
                type="number"
                value={expiresInDays}
                onChange={e => setExpiresInDays(parseInt(e.target.value) || 7)}
                min={1}
                max={90}
                style={{
                  width: '100%',
                  padding: '8px 12px',
                  background: tokens.bg,
                  border: `1px solid ${tokens.border}`,
                  borderRadius: 4,
                  color: tokens.text,
                  fontSize: 14,
                  boxSizing: 'border-box',
                }}
              />
            </div>
          </div>
          <button
            type="submit"
            disabled={creating}
            style={{
              marginTop: 16,
              padding: '8px 24px',
              background: creating ? tokens.muted : tokens.cta,
              color: tokens.ctaText,
              border: 'none',
              borderRadius: 4,
              fontWeight: 500,
              cursor: creating ? 'default' : 'pointer',
              fontSize: 14,
            }}
          >
            {creating ? 'Creating...' : 'Generate Invitation'}
          </button>
        </form>
      )}

      {loading ? (
        <div style={{ color: tokens.muted, textAlign: 'center', padding: 40 }}>Loading invitations...</div>
      ) : invitations.length === 0 ? (
        <div
          style={{
            background: tokens.surface,
            padding: 40,
            borderRadius: 8,
            textAlign: 'center',
            color: tokens.muted,
          }}
        >
          No invitations yet. Create one to invite team members.
        </div>
      ) : (
        <div>
          <div style={{ fontSize: 13, color: tokens.muted, marginBottom: 12 }}>
            {invitations.length} invitation{invitations.length !== 1 ? 's' : ''}
          </div>
          {invitations.map(inv => (
            <InvitationCard
              key={inv.id}
              invitation={inv}
              onDelete={handleDelete}
            />
          ))}
        </div>
      )}
    </div>
  );
}
