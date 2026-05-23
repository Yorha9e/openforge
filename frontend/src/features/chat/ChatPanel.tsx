import { useParams, useSearchParams } from 'react-router-dom';
import { ChatProvider } from './ChatProvider';
import { MessageList } from './MessageList';
import { MessageInput } from './MessageInput';

export function ChatPanel() {
  const { id } = useParams<{ id: string }>();
  const [params] = useSearchParams();
  const pipelineId = params.get('pipeline') || 'default';
  if (!id) return null;

  return (
    <ChatProvider pipelineId={pipelineId}>
      <div style={{ height: '100vh', background: '#0f0f0f', color: '#fff', display: 'flex', flexDirection: 'column' }}>
        <header style={{ padding: '8px 24px', borderBottom: '1px solid #262626', fontSize: 14, color: '#a3a3a3' }}>
          Pipeline: {pipelineId}
        </header>
        <MessageList />
        <MessageInput />
      </div>
    </ChatProvider>
  );
}
