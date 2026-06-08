// Minimal pipeline status card used in the SimpleMode right-column grid.
// Kept self-contained so it has no dependencies on the ProMode surface.
export function PipelineStatusCard({ pipeline }: { pipeline: any }) {
  const status = pipeline?.status ?? 'unknown';
  const stage = pipeline?.stage ?? pipeline?.current_stage ?? '';
  return (
    <div
      data-testid={`pipeline-status-${pipeline?.id ?? 'unknown'}`}
      className="rounded border p-3 bg-white shadow-sm"
    >
      <div className="font-semibold">{pipeline?.id ?? '(no id)'}</div>
      {stage && <div className="text-sm text-gray-500">{stage}</div>}
      <span className="inline-block mt-1 px-2 py-0.5 rounded text-xs bg-blue-100 text-blue-800">
        {status}
      </span>
    </div>
  );
}
