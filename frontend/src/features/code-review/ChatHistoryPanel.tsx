import { useState, useEffect } from 'react';
import { useProMode } from './ProModeContext';
import { api } from '../../shared/api';
import { tokens } from '../../shared/design-tokens';

interface Branch {
  id: string;
  parent_branch: string | null;
  fork_msg_seq: number;
  status: 'active' | 'merged' | 'abandoned';
  created_by: string;
  created_at: string;
}

interface BranchNode extends Branch {
  children: BranchNode[];
  depth: number;
}

// Minimal shape required by canActivateBranch — keeps the helper unit-testable
// without dragging in the full Branch type.
export type BranchLike = Pick<Branch, 'id' | 'status'>;

export const MAX_ACTIVE_BRANCHES = 3;

export interface ActivationCheck {
  allowed: boolean;
  reason?: string;
}

/**
 * Decide whether `targetId` may be activated given the current set of
 * branches. Blocks activation when the user already has MAX_ACTIVE_BRANCHES
 * active branches; the user must deactivate one first.
 */
export function canActivateBranch(
  branches: BranchLike[],
  targetId: string,
): ActivationCheck {
  const activeCount = branches.filter((b) => b.status === 'active').length;
  const target = branches.find((b) => b.id === targetId);
  // If the target is unknown, do not block — let downstream logic handle it.
  if (!target) {
    return { allowed: true };
  }
  // Clicking an already-active branch is always a no-op, not a violation.
  if (target.status === 'active') {
    return { allowed: true };
  }
  if (activeCount >= MAX_ACTIVE_BRANCHES) {
    return {
      allowed: false,
      reason: `You already have ${MAX_ACTIVE_BRANCHES} active branches (max ${MAX_ACTIVE_BRANCHES}). Deactivate one before activating a new branch.`,
    };
  }
  return { allowed: true };
}

export function ChatHistoryPanel() {
  const { pipelineId, activeBranchId, setActiveBranchId } = useProMode();
  const [branches, setBranches] = useState<Branch[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [branchAlert, setBranchAlert] = useState<string | null>(null);

  useEffect(() => {
    if (!pipelineId) return;

    setLoading(true);
    api.listBranches(pipelineId)
      .then(data => {
        setBranches(data.branches || []);
        // Find active branch
        const active = data.branches?.find(b => b.status === 'active');
        if (active) {
          setActiveBranchId(active.id);
        }
      })
      .catch(err => {
        setError(err instanceof Error ? err.message : 'Failed to load branches');
      })
      .finally(() => {
        setLoading(false);
      });
  }, [pipelineId]);

  // Build tree structure
  const buildTree = (branches: Branch[]): BranchNode[] => {
    const map = new Map<string, BranchNode>();
    const roots: BranchNode[] = [];

    // Create nodes
    branches.forEach(branch => {
      map.set(branch.id, { ...branch, children: [], depth: 0 });
    });

    // Build tree
    branches.forEach(branch => {
      const node = map.get(branch.id)!;
      if (branch.parent_branch) {
        const parent = map.get(branch.parent_branch);
        if (parent) {
          parent.children.push(node);
          node.depth = parent.depth + 1;
        } else {
          roots.push(node);
        }
      } else {
        roots.push(node);
      }
    });

    return roots;
  };

  const tree = buildTree(branches);

  const handleBranchClick = (branchId: string) => {
    const check = canActivateBranch(branches, branchId);
    if (!check.allowed) {
      setBranchAlert(check.reason ?? 'Cannot activate branch.');
      // Auto-dismiss the alert after a short delay so it does not stick.
      window.setTimeout(() => setBranchAlert(null), 5000);
      return;
    }
    setBranchAlert(null);
    setActiveBranchId(branchId);
  };

  const getStatusColor = (status: string) => {
    switch (status) {
      case 'active': return '#4caf50';
      case 'merged': return '#2196f3';
      case 'abandoned': return '#9e9e9e';
      default: return '#8b949e';
    }
  };

  const getStatusIcon = (status: string) => {
    switch (status) {
      case 'active': return '●';
      case 'merged': return '✓';
      case 'abandoned': return '○';
      default: return '•';
    }
  };

  const renderBranchNode = (node: BranchNode) => (
    <div key={node.id} style={{ marginLeft: node.depth * 16 }}>
      <div
        onClick={() => handleBranchClick(node.id)}
        style={{
          padding: '6px 8px',
          marginBottom: 4,
          borderRadius: 4,
          cursor: 'pointer',
          background: activeBranchId === node.id ? '#21262d' : 'transparent',
          border: activeBranchId === node.id ? '1px solid #30363d' : '1px solid transparent',
          display: 'flex',
          alignItems: 'center',
          gap: 8,
          fontSize: 12,
          transition: 'all 0.2s',
        }}
      >
        <span style={{ color: getStatusColor(node.status) }}>
          {getStatusIcon(node.status)}
        </span>
        <span style={{
          flex: 1,
          overflow: 'hidden',
          textOverflow: 'ellipsis',
          whiteSpace: 'nowrap',
          color: activeBranchId === node.id ? tokens.text : tokens.muted,
        }}>
          {node.id.substring(0, 8)}
        </span>
        <span style={{
          padding: '1px 4px',
          background: `${getStatusColor(node.status)}20`,
          color: getStatusColor(node.status),
          borderRadius: 3,
          fontSize: 10,
        }}>
          {node.status}
        </span>
      </div>
      {node.children.length > 0 && (
        <div style={{ marginLeft: 8 }}>
          {node.children.map(child => renderBranchNode(child))}
        </div>
      )}
    </div>
  );

  return (
    <div style={{
      height: '100%',
      display: 'flex',
      flexDirection: 'column',
      background: '#0d1117',
      color: '#c9d1d9',
    }}>
      <div style={{
        padding: '8px 12px',
        borderBottom: '1px solid #21262d',
        display: 'flex',
        justifyContent: 'space-between',
        alignItems: 'center',
      }}>
        <span style={{ color: '#8b949e' }}>Chat History</span>
        <span style={{ color: '#484f58', fontSize: 11 }}>
          {branches.length} branches
        </span>
      </div>
      {branchAlert && (
        <div
          role="alert"
          style={{
            padding: '8px 12px',
            background: '#3d1f1f',
            color: '#ff7b72',
            borderBottom: '1px solid #5a2a2a',
            fontSize: 12,
            lineHeight: 1.4,
          }}
        >
          {branchAlert}
        </div>
      )}
      <div style={{
        flex: 1,
        overflow: 'auto',
        padding: '8px',
      }}>
        {loading ? (
          <div style={{
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
            height: '100%',
            color: '#484f58',
          }}>
            Loading branches...
          </div>
        ) : error ? (
          <div style={{
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
            height: '100%',
            color: '#f44336',
          }}>
            {error}
          </div>
        ) : branches.length === 0 ? (
          <div style={{
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
            height: '100%',
            color: '#484f58',
            fontStyle: 'italic',
          }}>
            No conversation branches yet
          </div>
        ) : (
          <div>
            {tree.map(node => renderBranchNode(node))}
          </div>
        )}
      </div>
    </div>
  );
}