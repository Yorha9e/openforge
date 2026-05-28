import { tokens } from '../../shared/design-tokens';

interface RoleNode {
  role: string;
  color: string;
  description: string;
  peers?: RoleNode[];
}

interface RolePyramidProps {
  nodes: RoleNode[];
  currentUserRole?: string;
}

function RoleNodeItem({ node, isCurrentUser }: { node: RoleNode; isCurrentUser: boolean }) {
  return (
    <div
      tabIndex={0}
      aria-label={`Role: ${node.role}${isCurrentUser ? ' (you)' : ''} — ${node.description}`}
      style={{
        display: 'flex',
        alignItems: 'center',
        gap: 10,
        padding: '8px 16px',
        borderRadius: 8,
        background: isCurrentUser ? 'rgba(34, 197, 94, 0.04)' : '#111B2A',
        border: `1px solid ${isCurrentUser ? 'rgba(34, 197, 94, 0.3)' : tokens.border}`,
        minWidth: 130,
        cursor: 'default',
        transition: tokens.transition,
      }}
    >
      <div style={{ width: 10, height: 10, borderRadius: '50%', background: node.color, flexShrink: 0 }} />
      <div>
        <div style={{ display: 'flex', alignItems: 'center', gap: 6 }}>
          <span style={{ fontSize: 13, fontWeight: 700, color: node.color }}>{node.role}</span>
          {isCurrentUser && (
            <span style={{
              fontSize: 9, fontWeight: 700, padding: '1px 6px', borderRadius: 4,
              background: 'rgba(34, 197, 94, 0.12)', color: tokens.cta,
            }}>
              YOU
            </span>
          )}
        </div>
        <div style={{ fontSize: 10, color: tokens.muted }}>{node.description}</div>
      </div>
    </div>
  );
}

/** Vertical line segment between nodes */
function VLine({ height = 14 }: { height?: number }) {
  return <div style={{ width: 1, height, background: tokens.border, margin: '0 auto' }} />;
}

/** Small horizontal connector for T-shape branching */
function HBranch({ left, right }: { left: number; right: number }) {
  return (
    <div style={{ width: left + right, height: 1, background: tokens.border, margin: '0 auto' }} />
  );
}

export function RolePyramid({ nodes, currentUserRole }: RolePyramidProps) {
  return (
    <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'center', gap: 0 }}>
      {nodes.map((node, idx) => {
        const isCurrent = node.role === currentUserRole;
        const hasPeers = node.peers && node.peers.length > 0;

        if (hasPeers) {
          const peers = node.peers!;
          // Peer card width ≈ 158px each, gap = 8px → total peer width ~324px
          // Center lines for each peer at ~79px and ~245px from left edge of peer group
          // T-junction top: single line down → horizontal bar → two drops into peers
          // T-junction bottom: two drops from peers → horizontal bar merge → single line down
          return (
            <div key={`group-${idx}`} style={{ display: 'flex', flexDirection: 'column', alignItems: 'center' }}>
              {idx > 0 && <VLine height={16} />}
              {/* Top T-branch */}
              <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'center' }}>
                <VLine height={8} />
                <div style={{ display: 'flex', alignItems: 'flex-start', justifyContent: 'center', position: 'relative' }}>
                  {/* Left drop line */}
                  <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'center' }}>
                    <VLine height={8} />
                  </div>
                  {/* Spacer with horizontal bar */}
                  <div style={{ width: 80, display: 'flex', flexDirection: 'column', alignItems: 'center' }}>
                    <div style={{ height: 8 }} />
                    <div style={{ width: 80, height: 1, background: tokens.border }} />
                    <div style={{ height: 8 }} />
                  </div>
                  {/* Right drop line */}
                  <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'center' }}>
                    <VLine height={8} />
                  </div>
                </div>
              </div>
              {/* Peer nodes */}
              <div style={{ display: 'flex', gap: 8 }}>
                {peers.map(peer => (
                  <RoleNodeItem key={peer.role} node={peer} isCurrentUser={peer.role === currentUserRole} />
                ))}
              </div>
              {/* Bottom merge T-branch */}
              <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'center' }}>
                <div style={{ display: 'flex', alignItems: 'flex-end', justifyContent: 'center', position: 'relative' }}>
                  <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'center' }}>
                    <VLine height={8} />
                  </div>
                  <div style={{ width: 80, display: 'flex', flexDirection: 'column', alignItems: 'center' }}>
                    <div style={{ height: 8 }} />
                    <div style={{ width: 80, height: 1, background: tokens.border }} />
                    <div style={{ height: 8 }} />
                  </div>
                  <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'center' }}>
                    <VLine height={8} />
                  </div>
                </div>
                <VLine height={8} />
              </div>
            </div>
          );
        }

        // Regular single node
        return (
          <div key={node.role} style={{ display: 'flex', flexDirection: 'column', alignItems: 'center' }}>
            {idx > 0 && <VLine height={16} />}
            <RoleNodeItem node={node} isCurrentUser={isCurrent} />
          </div>
        );
      })}
    </div>
  );
}
