import { useState, useEffect } from 'react';
import { useProMode } from './ProModeContext';
import { tokens } from '../../shared/design-tokens';

interface TestCase {
  name: string;
  status: 'passed' | 'failed' | 'skipped';
  error?: string;
  duration?: number;
}

interface TestReport {
  total: number;
  passed: number;
  failed: number;
  skipped: number;
  duration: number;
  testCases: TestCase[];
}

export function TestReportPanel() {
  const { executionLogs } = useProMode();
  const [report, setReport] = useState<TestReport | null>(null);

  useEffect(() => {
    // Parse test.report events from logs
    const testLogs = executionLogs.filter(log => log.type === 'test.report');
    if (testLogs.length > 0) {
      const latest = testLogs[testLogs.length - 1];
      if (latest) {
        setReport({
          total: latest.total || 0,
          passed: latest.passed || 0,
          failed: latest.failed || 0,
          skipped: latest.skipped || 0,
          duration: latest.duration || 0,
          testCases: latest.testCases || [],
        });
      }
    }
  }, [executionLogs]);

  const formatDuration = (ms: number) => {
    if (ms < 1000) return `${ms}ms`;
    return `${(ms / 1000).toFixed(2)}s`;
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
        <span style={{ color: '#8b949e' }}>Test Report</span>
        {report && (
          <span style={{ color: '#484f58', fontSize: 11 }}>
            {formatDuration(report.duration)}
          </span>
        )}
      </div>
      <div style={{
        flex: 1,
        overflow: 'auto',
        padding: '12px',
      }}>
        {!report ? (
          <div style={{
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
            height: '100%',
            color: '#484f58',
            fontStyle: 'italic',
          }}>
            No test report available
          </div>
        ) : (
          <>
            <div style={{
              display: 'flex',
              gap: 16,
              marginBottom: 16,
            }}>
              <div style={{
                flex: 1,
                padding: '12px',
                background: '#4caf5020',
                borderRadius: 8,
                border: '1px solid #4caf5040',
                textAlign: 'center',
              }}>
                <div style={{ fontSize: 24, fontWeight: 600, color: '#4caf50' }}>
                  {report.passed}
                </div>
                <div style={{ fontSize: 11, color: '#4caf50' }}>Passed</div>
              </div>
              <div style={{
                flex: 1,
                padding: '12px',
                background: '#f4433620',
                borderRadius: 8,
                border: '1px solid #f4433640',
                textAlign: 'center',
              }}>
                <div style={{ fontSize: 24, fontWeight: 600, color: '#f44336' }}>
                  {report.failed}
                </div>
                <div style={{ fontSize: 11, color: '#f44336' }}>Failed</div>
              </div>
              <div style={{
                flex: 1,
                padding: '12px',
                background: '#ff980020',
                borderRadius: 8,
                border: '1px solid #ff980040',
                textAlign: 'center',
              }}>
                <div style={{ fontSize: 24, fontWeight: 600, color: '#ff9800' }}>
                  {report.skipped}
                </div>
                <div style={{ fontSize: 11, color: '#ff9800' }}>Skipped</div>
              </div>
            </div>
            {report.testCases.length > 0 && (
              <div>
                <div style={{ fontSize: 12, color: '#8b949e', marginBottom: 8 }}>
                  Test Cases
                </div>
                {report.testCases.map((testCase, index) => (
                  <div
                    key={index}
                    style={{
                      padding: '8px 12px',
                      marginBottom: 4,
                      background: testCase.status === 'failed' ? '#f4433620' : 'transparent',
                      borderRadius: 4,
                      borderLeft: `3px solid ${
                        testCase.status === 'passed' ? '#4caf50' :
                        testCase.status === 'failed' ? '#f44336' : '#ff9800'
                      }`,
                    }}
                  >
                    <div style={{
                      display: 'flex',
                      justifyContent: 'space-between',
                      alignItems: 'center',
                    }}>
                      <span style={{ fontSize: 12 }}>
                        <span style={{
                          color: testCase.status === 'passed' ? '#4caf50' :
                                 testCase.status === 'failed' ? '#f44336' : '#ff9800',
                          marginRight: 8,
                        }}>
                          {testCase.status === 'passed' ? '✓' :
                           testCase.status === 'failed' ? '✗' : '○'}
                        </span>
                        {testCase.name}
                      </span>
                      {testCase.duration && (
                        <span style={{ color: '#484f58', fontSize: 11 }}>
                          {formatDuration(testCase.duration)}
                        </span>
                      )}
                    </div>
                    {testCase.error && (
                      <div style={{
                        marginTop: 4,
                        padding: '4px 8px',
                        background: '#f4433610',
                        borderRadius: 4,
                        fontSize: 11,
                        color: '#f44336',
                        fontFamily: 'monospace',
                      }}>
                        {testCase.error}
                      </div>
                    )}
                  </div>
                ))}
              </div>
            )}
          </>
        )}
      </div>
    </div>
  );
}