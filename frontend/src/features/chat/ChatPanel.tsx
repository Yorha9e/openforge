import { useState, useEffect } from 'react';
import { useParams, useSearchParams } from 'react-router-dom';
import { ChatProvider } from './ChatProvider';
import { MessageList } from './MessageList';
import { MessageInput } from './MessageInput';
import { ModelSelector } from './ModelSelector';
import { AgentPanel, AgentInfo } from './AgentPanel';
import { SkillBadge } from './SkillBadge';
import { tokens } from '../../shared/design-tokens';

function useAgents(_pipelineId: string): AgentInfo[] {
  return [];
}

function useActiveSkills() {
  const [skills, setSkills] = useState<Array<{name: string; version: string; source: string}>>([]);

  useEffect(() => {
    fetch('/api/admin/skills')
      .then(r => r.json())
      .then((data: any[]) => {
        if (Array.isArray(data)) {
          setSkills(data.filter(s => s.enabled && !s.deprecated));
        }
      })
      .catch(() => setSkills([]));
  }, []);

  return skills;
}

export function ChatPanel({ embedded }: { embedded?: boolean }) {
  const { id } = useParams<{ id: string }>();
  const [params] = useSearchParams();
  const pipelineId = params.get('pipeline') || 'default';
  const [model, setModel] = useState('deepseek');
  const agents = useAgents(pipelineId);
  const activeSkills = useActiveSkills();
  if (!id) return null;

  const containerHeight = embedded ? '100%' : '100vh';

  return (
    <ChatProvider pipelineId={pipelineId}>
      <div style={{ height: containerHeight, maxHeight: containerHeight, background: tokens.bg, color: tokens.text, display: 'flex', flexDirection: 'column', fontFamily: tokens.fontBody }} role="region" aria-label="Chat panel">
        <header style={{ padding: '8px 24px', borderBottom: `1px solid ${tokens.border}`, fontSize: 14, color: tokens.muted, display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
          <div style={{ display: 'flex', alignItems: 'center', gap: 12 }}>
            <span>Pipeline: {pipelineId}</span>
            {activeSkills.length > 0 && (
              <span style={{ display: 'flex', alignItems: 'center', gap: 4 }}>
                {activeSkills.map(s => (
                  <SkillBadge key={s.name} name={s.name} version={s.version} source={s.source} />
                ))}
              </span>
            )}
          </div>
          <ModelSelector current={model} onSelect={setModel} />
        </header>
        <AgentPanel agents={agents} />
        <MessageList />
        <MessageInput />
      </div>
    </ChatProvider>
  );
}
