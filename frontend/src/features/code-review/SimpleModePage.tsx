import { useEffect, useState } from 'react';
import { useParams } from 'react-router-dom';
import { api } from '../../shared/api';
import { ChatPanel } from '../chat/ChatPanel';
import { PipelineStatusCard } from './PipelineStatusCard';

export type ViewMode = 'simple' | 'pro';

export interface SimpleModeSettings {
  layout?: { defaultViewMode?: string };
}

/**
 * Tailwind arbitrary value producing a 60/40 (3fr / 2fr) two-column grid.
 * Exported so the unit test can assert the class string without rendering.
 */
export function simpleModeGridClass(): string {
  return 'grid grid-cols-[3fr_2fr] gap-4 h-full p-4';
}

/**
 * Resolves the default view mode from the user settings payload.
 * Defensive: any unexpected / missing value falls back to 'simple' so
 * users land on the simpler, more guided surface.
 */
export function viewModeFromSettings(settings: SimpleModeSettings | null | undefined): ViewMode {
  const raw = settings?.layout?.defaultViewMode;
  return raw === 'pro' ? 'pro' : 'simple';
}

export function SimpleModePage({ projectId }: { projectId?: string }) {
  const params = useParams<{ id: string }>();
  const pid = projectId ?? params.id ?? '';
  const [pipelines, setPipelines] = useState<any[]>([]);

  useEffect(() => {
    let cancelled = false;
    if (!pid) return () => { cancelled = true; };
    api
      .listPipelines(pid)
      .then((data) => {
        if (!cancelled) setPipelines(Array.isArray(data) ? data : []);
      })
      .catch(() => {
        if (!cancelled) setPipelines([]);
      });
    return () => {
      cancelled = true;
    };
  }, [pid]);

  return (
    <div data-testid="simple-mode-grid" className={simpleModeGridClass()}>
      <div data-testid="chat-panel" className="h-full overflow-auto">
        {/* ChatPanel reads projectId from useParams() internally; only mount it
            when we actually have a project id from the route or prop. */}
        {pid ? <ChatPanel /> : null}
      </div>
      <div data-testid="info-card-grid" className="grid grid-cols-2 gap-3 auto-rows-min">
        {pipelines.map((p) => (
          <PipelineStatusCard key={p.id} pipeline={p} />
        ))}
      </div>
    </div>
  );
}
