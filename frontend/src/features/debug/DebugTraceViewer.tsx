import { useCallback, useEffect, useMemo, useState } from 'react';
import { useAuth } from '../../shared/auth';
import { Skeleton } from '../../shared/skeleton';

interface DebugTraceEvent {
  timestamp: string;
  name: string;
  payload?: Record<string, unknown>;
}

interface DebugTrace {
  pipeline_id: string;
  events: DebugTraceEvent[];
  duration_s: number;
}

interface DebugTraceViewerProps {
  pipelineId: string;
}

const API_BASE = '/api';

async function fetchDebugTrace(
  pipelineId: string,
  token: string | null,
): Promise<DebugTrace> {
  const headers: Record<string, string> = { 'Content-Type': 'application/json' };
  if (token) headers['Authorization'] = `Bearer ${token}`;
  const res = await fetch(`${API_BASE}/pipelines/${encodeURIComponent(pipelineId)}/replay`, {
    method: 'GET',
    headers,
  });
  if (!res.ok) {
    const text = await res.text().catch(() => '');
    throw new Error(`trace fetch failed: ${res.status} ${text}`);
  }
  return res.json() as Promise<DebugTrace>;
}

function formatTimestamp(iso: string): string {
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return iso;
  return d.toISOString().replace('T', ' ').replace('Z', ' UTC');
}

function payloadToString(payload?: Record<string, unknown>): string {
  if (!payload || Object.keys(payload).length === 0) return '{}';
  try {
    return JSON.stringify(payload, null, 2);
  } catch {
    return '[unserializable payload]';
  }
}

export default function DebugTraceViewer({ pipelineId }: DebugTraceViewerProps) {
  const { token } = useAuth();
  const [trace, setTrace] = useState<DebugTrace | null>(null);
  const [loading, setLoading] = useState<boolean>(true);
  const [error, setError] = useState<string | null>(null);
  const [filter, setFilter] = useState<string>('');

  const load = useCallback(async () => {
    if (!pipelineId) {
      setError('pipeline id missing');
      setLoading(false);
      return;
    }
    setLoading(true);
    setError(null);
    try {
      const data = await fetchDebugTrace(pipelineId, token);
      setTrace(data);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'failed to load trace');
      setTrace(null);
    } finally {
      setLoading(false);
    }
  }, [pipelineId, token]);

  useEffect(() => {
    load();
  }, [load]);

  const filteredEvents = useMemo(() => {
    if (!trace) return [];
    const needle = filter.trim().toLowerCase();
    if (!needle) return trace.events;
    return trace.events.filter((e) =>
      e.name.toLowerCase().includes(needle) ||
      (e.payload ? JSON.stringify(e.payload).toLowerCase().includes(needle) : false),
    );
  }, [trace, filter]);

  return (
    <div
      style={{
        padding: 24,
        background: '#0F172A',
        color: '#F8FAFC',
        minHeight: 320,
        fontFamily: "'Fira Sans', sans-serif",
      }}
    >
      <header
        style={{
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'space-between',
          marginBottom: 16,
          gap: 12,
          flexWrap: 'wrap',
        }}
      >
        <div>
          <h1
            style={{
              fontSize: 18,
              fontFamily: "'Fira Code', monospace",
              margin: 0,
            }}
          >
            Debug Trace
          </h1>
          <div style={{ color: '#94A3B8', fontSize: 12, marginTop: 4 }}>
            pipeline <code style={{ color: '#22C55E' }}>{pipelineId}</code>
            {trace && (
              <>
                {' · '}
                {trace.events.length} events · {trace.duration_s.toFixed(2)}s
              </>
            )}
          </div>
        </div>
        <div style={{ display: 'flex', gap: 8 }}>
          <input
            type="search"
            placeholder="filter events…"
            value={filter}
            onChange={(e) => setFilter(e.target.value)}
            style={{
              background: '#1E293B',
              border: '1px solid #334155',
              color: '#F8FAFC',
              borderRadius: 4,
              padding: '6px 10px',
              fontSize: 13,
              minWidth: 180,
            }}
            aria-label="filter events"
          />
          <button
            type="button"
            onClick={load}
            style={{
              background: '#22C55E',
              color: '#0F172A',
              border: 'none',
              borderRadius: 4,
              padding: '6px 12px',
              fontSize: 13,
              cursor: 'pointer',
              fontFamily: "'Fira Code', monospace",
            }}
          >
            reload
          </button>
        </div>
      </header>

      {error && (
        <div
          role="alert"
          style={{
            background: '#7F1D1D',
            border: '1px solid #DC2626',
            color: '#FCA5A5',
            padding: 12,
            borderRadius: 6,
            marginBottom: 16,
            fontSize: 13,
          }}
        >
          {error}
        </div>
      )}

      {loading ? (
        <div>
          <Skeleton variant="text" width="60%" />
          <Skeleton variant="text" width="80%" />
          <Skeleton variant="text" width="40%" />
        </div>
      ) : !trace || trace.events.length === 0 ? (
        <div
          style={{
            background: '#1E293B',
            border: '1px dashed #334155',
            borderRadius: 6,
            padding: 24,
            textAlign: 'center',
            color: '#64748B',
            fontSize: 13,
          }}
        >
          no trace events recorded for this pipeline in the last 90 days.
        </div>
      ) : (
        <ol
          style={{
            listStyle: 'none',
            margin: 0,
            padding: 0,
            display: 'flex',
            flexDirection: 'column',
            gap: 8,
          }}
        >
          {filteredEvents.map((evt, idx) => (
            <li
              key={`${evt.timestamp}-${idx}`}
              style={{
                background: '#1E293B',
                border: '1px solid #334155',
                borderRadius: 6,
                padding: 12,
              }}
            >
              <div
                style={{
                  display: 'flex',
                  justifyContent: 'space-between',
                  alignItems: 'baseline',
                  marginBottom: 6,
                  gap: 8,
                }}
              >
                <span
                  style={{
                    color: '#22C55E',
                    fontFamily: "'Fira Code', monospace",
                    fontSize: 13,
                    fontWeight: 600,
                  }}
                >
                  {evt.name}
                </span>
                <span style={{ color: '#94A3B8', fontSize: 11, fontFamily: "'Fira Code', monospace" }}>
                  {formatTimestamp(evt.timestamp)}
                </span>
              </div>
              <pre
                style={{
                  margin: 0,
                  padding: 8,
                  background: '#0F172A',
                  border: '1px solid #334155',
                  borderRadius: 4,
                  color: '#CBD5E1',
                  fontSize: 12,
                  fontFamily: "'Fira Code', monospace",
                  overflowX: 'auto',
                  whiteSpace: 'pre-wrap',
                  wordBreak: 'break-word',
                }}
              >
                {payloadToString(evt.payload)}
              </pre>
            </li>
          ))}
        </ol>
      )}

      {trace && filteredEvents.length !== trace.events.length && (
        <div style={{ color: '#94A3B8', fontSize: 12, marginTop: 12, textAlign: 'center' }}>
          showing {filteredEvents.length} of {trace.events.length} events
        </div>
      )}
    </div>
  );
}
