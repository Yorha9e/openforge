import { useState, useEffect } from 'react';
import { useProMode } from './ProModeContext';
import { tokens } from '../../shared/design-tokens';

interface LineComment {
  file_path: string;
  line: number;
  comment: string;
  mark: 'approve' | 'reject' | 'comment';
}

interface FileComments {
  file: string;
  comments: LineComment[];
  expanded: boolean;
}

export function CommentPanel() {
  const { executionLogs } = useProMode();
  const [fileComments, setFileComments] = useState<FileComments[]>([]);

  useEffect(() => {
    // Parse gate events from logs to extract comments
    const gateLogs = executionLogs.filter(log => log.type === 'gate.event');
    const allComments: LineComment[] = [];

    gateLogs.forEach(log => {
      if (log.comments && Array.isArray(log.comments)) {
        allComments.push(...log.comments);
      }
    });

    // Group by file
    const grouped = new Map<string, LineComment[]>();
    allComments.forEach(comment => {
      const existing = grouped.get(comment.file_path) || [];
      existing.push(comment);
      grouped.set(comment.file_path, existing);
    });

    setFileComments(Array.from(grouped.entries()).map(([file, comments]) => ({
      file,
      comments,
      expanded: true,
    })));
  }, [executionLogs]);

  const toggleExpand = (file: string) => {
    setFileComments(prev => prev.map(fc => 
      fc.file === file ? { ...fc, expanded: !fc.expanded } : fc
    ));
  };

  const getMarkColor = (mark: string) => {
    switch (mark) {
      case 'approve': return '#4caf50';
      case 'reject': return '#f44336';
      case 'comment': return '#2196f3';
      default: return '#8b949e';
    }
  };

  const getMarkIcon = (mark: string) => {
    switch (mark) {
      case 'approve': return '✓';
      case 'reject': return '✗';
      case 'comment': return '💬';
      default: return '•';
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
        <span style={{ color: '#8b949e' }}>Comments</span>
        <span style={{ color: '#484f58', fontSize: 11 }}>
          {fileComments.reduce((acc, fc) => acc + fc.comments.length, 0)} comments
        </span>
      </div>
      <div style={{
        flex: 1,
        overflow: 'auto',
        padding: '8px',
      }}>
        {fileComments.length === 0 ? (
          <div style={{
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
            height: '100%',
            color: '#484f58',
            fontStyle: 'italic',
          }}>
            No review comments yet
          </div>
        ) : (
          fileComments.map(fileComment => (
            <div key={fileComment.file} style={{ marginBottom: 12 }}>
              <div
                onClick={() => toggleExpand(fileComment.file)}
                style={{
                  padding: '6px 8px',
                  background: '#21262d',
                  borderRadius: 4,
                  cursor: 'pointer',
                  display: 'flex',
                  alignItems: 'center',
                  gap: 8,
                  fontSize: 12,
                  fontFamily: "'Fira Code', monospace",
                }}
              >
                <span style={{
                  transform: fileComment.expanded ? 'rotate(90deg)' : 'rotate(0deg)',
                  transition: 'transform 0.2s',
                  fontSize: 10,
                }}>
                  ▶
                </span>
                <span style={{ flex: 1, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                  {fileComment.file}
                </span>
                <span style={{ color: '#484f58', fontSize: 11 }}>
                  {fileComment.comments.length}
                </span>
              </div>
              {fileComment.expanded && (
                <div style={{ marginLeft: 16, marginTop: 4 }}>
                  {fileComment.comments.map((comment, index) => (
                    <div
                      key={index}
                      style={{
                        padding: '6px 8px',
                        marginBottom: 4,
                        borderLeft: `3px solid ${getMarkColor(comment.mark)}`,
                        background: '#161b22',
                        borderRadius: '0 4px 4px 0',
                        fontSize: 12,
                      }}
                    >
                      <div style={{
                        display: 'flex',
                        alignItems: 'center',
                        gap: 6,
                        marginBottom: 4,
                        fontSize: 11,
                        color: '#8b949e',
                      }}>
                        <span style={{ color: getMarkColor(comment.mark) }}>
                          {getMarkIcon(comment.mark)}
                        </span>
                        <span>Line {comment.line}</span>
                        <span style={{
                          padding: '1px 4px',
                          background: `${getMarkColor(comment.mark)}20`,
                          color: getMarkColor(comment.mark),
                          borderRadius: 3,
                          fontSize: 10,
                        }}>
                          {comment.mark}
                        </span>
                      </div>
                      <div style={{ color: '#c9d1d9' }}>
                        {comment.comment}
                      </div>
                    </div>
                  ))}
                </div>
              )}
            </div>
          ))
        )}
      </div>
    </div>
  );
}