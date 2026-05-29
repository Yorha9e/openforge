import { useProMode } from './ProModeContext';
import { tokens } from '../../shared/design-tokens';

interface StageInfo {
  type: string;
  status: 'pending' | 'running' | 'passed' | 'failed';
}

export function FlowchartPanel() {
  const { executionLogs } = useProMode();

  const stages: StageInfo[] = [
    { type: 'clarify', status: 'pending' },
    { type: 'decompose', status: 'pending' },
    { type: 'impl', status: 'pending' },
    { type: 'test', status: 'pending' },
    { type: 'deploy', status: 'pending' },
    { type: 'verify', status: 'pending' },
  ];

  // Parse stage_change events from logs
  executionLogs.forEach(log => {
    if (log.type === 'stage_change') {
      const stage = stages.find(s => s.type === log.stage);
      if (stage) {
        stage.status = log.status === 'running' ? 'running' : 
                      log.status === 'passed' ? 'passed' : 
                      log.status === 'failed' ? 'failed' : 'pending';
      }
    }
  });

  const getStatusColor = (status: string) => {
    switch (status) {
      case 'running': return '#ff9800';
      case 'passed': return '#4caf50';
      case 'failed': return '#f44336';
      default: return '#484f58';
    }
  };

  const getStatusIcon = (status: string) => {
    switch (status) {
      case 'running': return '⟳';
      case 'passed': return '✓';
      case 'failed': return '✗';
      default: return '○';
    }
  };

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
        <span style={{ color: '#8b949e' }}>Pipeline Flow</span>
        <div style={{ display: 'flex', gap: 8, fontSize: 11 }}>
          <span style={{ color: '#484f58' }}>○ Pending</span>
          <span style={{ color: '#ff9800' }}>⟳ Running</span>
          <span style={{ color: '#4caf50' }}>✓ Passed</span>
          <span style={{ color: '#f44336' }}>✗ Failed</span>
        </div>
      </div>
      <div style={{
        flex: 1,
        display: 'flex',
        alignItems: 'center',
        padding: '20px 40px',
        overflowX: 'auto',
      }}>
        {stages.map((stage, index) => (
          <div key={stage.type} style={{
            display: 'flex',
            alignItems: 'center',
            minWidth: 120,
          }}>
            <div style={{
              display: 'flex',
              flexDirection: 'column',
              alignItems: 'center',
              gap: 8,
              minWidth: 100,
            }}>
              <div style={{
                width: 48,
                height: 48,
                borderRadius: '50%',
                background: stage.status === 'running' ? `${getStatusColor(stage.status)}20` : 'transparent',
                border: `2px solid ${getStatusColor(stage.status)}`,
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'center',
                fontSize: 20,
                color: getStatusColor(stage.status),
                animation: stage.status === 'running' ? 'pulse 2s infinite' : 'none',
              }}>
                {getStatusIcon(stage.status)}
              </div>
              <div style={{
                fontSize: 12,
                color: getStatusColor(stage.status),
                textAlign: 'center',
                fontWeight: stage.status === 'running' ? 600 : 400,
              }}>
                {stage.type}
              </div>
              <div style={{
                fontSize: 10,
                color: '#484f58',
                textTransform: 'capitalize',
              }}>
                {stage.status}
              </div>
            </div>
            {index < stages.length - 1 && (
              <div style={{
                flex: 1,
                height: 2,
                background: '#21262d',
                minWidth: 40,
                margin: '0 8px',
                position: 'relative',
              }}>
                <div style={{
                  position: 'absolute',
                  left: 0,
                  top: 0,
                  height: '100%',
                  width: stage.status === 'passed' ? '100%' : '0%',
                  background: '#4caf50',
                  transition: 'width 0.3s ease',
                }} />
              </div>
            )}
          </div>
        ))}
      </div>
      <style>
        {`
          @keyframes pulse {
            0% { opacity: 1; }
            50% { opacity: 0.5; }
            100% { opacity: 1; }
          }
        `}
      </style>
    </div>
  );
}