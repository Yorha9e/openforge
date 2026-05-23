import { useState } from 'react';
import { useParams, useSearchParams } from 'react-router-dom';
import { ChatProvider } from './ChatProvider';
import { MessageList } from './MessageList';
import { MessageInput } from './MessageInput';
import { ModelSelector } from './ModelSelector';
import { AgentPanel } from './AgentPanel';
import { tokens } from '../../shared/design-tokens';

export function ChatPanel() {
  const { id } = useParams<{ id: string }>();
  const [params] = useSearchParams();
  const pipelineId = params.get('pipeline') || 'default';
  const [model, setModel] = useState('deepseek');
  if (!id) return null;

  return (
    <ChatProvider pipelineId={pipelineId}>
      <div style={{ height: '100%', background: tokens.bg, color: tokens.text, display: 'flex', flexDirection: 'column', fontFamily: tokens.fontBody }} role="region" aria-label="Chat panel">
        <header style={{ padding: '8px 24px', borderBottom: `1px solid ${tokens.border}`, fontSize: 14, color: tokens.muted, display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
          <span>Pipeline: {pipelineId}</span>
          <ModelSelector current={model} onSelect={setModel} />
        </header>
        <AgentPanel agents={[]} />
        <MessageList />
        <MessageInput />
      </div>
    </ChatProvider>
  );
}
