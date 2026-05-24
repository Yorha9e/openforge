import { tokens } from '../../shared/design-tokens';

interface SkillBadgeProps {
  name: string;
  version: string;
  source?: string;
  triggerScore?: number;
  currentPriority?: number;
  tokenCost?: number;
}

export function SkillBadge({
  name,
  version,
  source = 'global',
  triggerScore,
  currentPriority,
  tokenCost,
}: SkillBadgeProps) {
  const sourceColors: Record<string, string> = {
    project: '#3b82f6',
    team: '#8b5cf6',
    global: tokens.muted || '#6b7280',
  };

  const badgeColor = sourceColors[source] || sourceColors.global;

  return (
    <span
      title={`Skill: ${name} v${version}\nSource: ${source}\nTrigger Score: ${triggerScore?.toFixed(1)}\nPriority: ${currentPriority?.toFixed(1)}\nTokens: ~${tokenCost}`}
      style={{
        display: 'inline-flex',
        alignItems: 'center',
        gap: 4,
        padding: '2px 8px',
        borderRadius: 4,
        fontSize: 11,
        fontWeight: 500,
        background: `${badgeColor}18`,
        color: badgeColor,
        border: `1px solid ${badgeColor}40`,
        whiteSpace: 'nowrap',
        marginRight: 4,
      }}
    >
      <span style={{ fontSize: 12 }}>{'\u{1F9E9}'}</span>
      {name} v{version}
    </span>
  );
}
