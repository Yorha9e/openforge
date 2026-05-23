import { useState } from 'react';
import { useParams, useSearchParams } from 'react-router-dom';
import { ChatProvider } from './ChatProvider';
import { MessageList } from './MessageList';
import { MessageInput } from './MessageInput';
import { ModelSelector } from './ModelSelector';
import { AgentPanel, AgentInfo } from './AgentPanel';
import { tokens } from '../../shared/design-tokens';

function useAgents(_pipelineId: string): AgentInfo[] {
  // Phase 6+: fetch from GET /api/pipelines/{id}/agents
  // For now, return empty — agents API not yet exposed
  return [];
}

export function ChatPanel() {
  const { id } = useParams<{ id: string }>();
  const [params] = useSearchParams();
  const pipelineId = params.get('pipeline') || 'default';
  const [model, setModel] = useState('deepseek');
  const agents = useAgents(pipelineId);
  if (!id) return null;

  return (
    <ChatProvider pipelineId={pipelineId}>
      <div style={{ height: '100%', background: tokens.bg, color: tokens.text, display: 'flex', flexDirection: 'column', fontFamily: tokens.fontBody }} role="region" aria-label="Chat panel">
        <header style={{ padding: '8px 24px', borderBottom: `1px solid ${tokens.border}`, fontSize: 14, color: tokens.muted, display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
          <span>Pipeline: {pipelineId}</span>
          <ModelSelector current={model} onSelect={setModel} />
        </header>
        <AgentPanel agents={agents} />
        <MessageList />
        <MessageInput />
      </div>
    </ChatProvider>
  );
}
