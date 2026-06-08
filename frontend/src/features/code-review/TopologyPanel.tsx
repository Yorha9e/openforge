import { useEffect, useMemo, useRef, useState } from 'react';
import cytoscape from 'cytoscape';
import { tokens } from '../../shared/design-tokens';

type TopologyLevel = 1 | 2 | 3;

interface TopologyNode {
  id: string;
  label: string;
  type: 'frontend' | 'backend' | 'shared' | 'test';
  /**
   * Monorepo layer. The topology API assigns:
   *   1 = business / hooks (e.g. useFoo)
   *   2 = default / components
   *   3 = data layer
   * Optional for backward compatibility with older callers that
   * pre-date the L1/L2/L3 view.
   */
  level?: TopologyLevel;
}

interface TopologyEdge {
  source: string;
  target: string;
}

interface TopologyPanelProps {
  nodes?: TopologyNode[];
  edges?: TopologyEdge[];
  /**
   * Initial level filter. Defaults to 2 (the "everything" view).
   */
  defaultLevel?: TopologyLevel;
}

const ALL_LEVELS: TopologyLevel[] = [1, 2, 3];

export function TopologyPanel({ nodes = [], edges = [], defaultLevel = 2 }: TopologyPanelProps) {
  const containerRef = useRef<HTMLDivElement>(null);
  const cyRef = useRef<cytoscape.Core | null>(null);
  const [level, setLevel] = useState<TopologyLevel>(defaultLevel);

  // Filter nodes to the active level. Nodes without an explicit level
  // are treated as L2 (default) so the legacy "everything" view still
  // renders the same graph it always did when level=2.
  const visibleNodes = useMemo(() => {
    return nodes.filter((n) => (n.level ?? 2) === level);
  }, [nodes, level]);

  // Drop edges whose endpoints are not visible in the current level.
  const visibleEdges = useMemo(() => {
    const ids = new Set(visibleNodes.map((n) => n.id));
    return edges
      .map((e, idx) => ({ edge: e, idx }))
      .filter(({ edge }) => ids.has(edge.source) && ids.has(edge.target))
      .map(({ edge, idx }) => ({ edge, idx }));
  }, [edges, visibleNodes]);

  useEffect(() => {
    if (!containerRef.current) return;

    if (visibleNodes.length === 0) {
      if (cyRef.current) {
        cyRef.current.destroy();
        cyRef.current = null;
      }
      return;
    }

    const getTypeColor = (type: string) => {
      switch (type) {
        case 'frontend': return '#00bcd4';
        case 'backend': return '#9c27b0';
        case 'shared': return '#ff9800';
        case 'test': return '#4caf50';
        default: return '#8b949e';
      }
    };

    const elements: cytoscape.ElementDefinition[] = [
      ...visibleNodes.map(node => ({
        data: {
          id: node.id,
          label: node.label,
          type: node.type,
        },
      })),
      ...visibleEdges.map(({ edge, idx }) => ({
        data: {
          id: `e${idx}`,
          source: edge.source,
          target: edge.target,
        },
      })),
    ];

    if (cyRef.current) {
      cyRef.current.destroy();
    }

    cyRef.current = cytoscape({
      container: containerRef.current,
      elements,
      style: [
        {
          selector: 'node',
          style: {
            'background-color': (ele: cytoscape.NodeSingular) => getTypeColor(ele.data('type')),
            'label': 'data(label)',
            'color': '#c9d1d9',
            'text-valign': 'center',
            'text-halign': 'center',
            'font-size': '12px',
            'width': '60px',
            'height': '60px',
            'border-width': '2px',
            'border-color': '#21262d',
          },
        },
        {
          selector: 'edge',
          style: {
            'width': 2,
            'line-color': '#30363d',
            'target-arrow-color': '#30363d',
            'target-arrow-shape': 'triangle',
            'curve-style': 'bezier',
          },
        },
      ],
      layout: {
        name: 'cose',
        animate: true,
        animationDuration: 500,
        nodeRepulsion: () => 8000,
        idealEdgeLength: () => 100,
        edgeElasticity: () => 100,
        nestingFactor: 1.2,
        gravity: 1,
        numIter: 1000,
        padding: 50,
      },
    });

    return () => {
      if (cyRef.current) {
        cyRef.current.destroy();
        cyRef.current = null;
      }
    };
  }, [visibleNodes, visibleEdges]);

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
        <span style={{ color: '#8b949e' }}>Topology</span>
        <div style={{ display: 'flex', gap: 8, fontSize: 11, alignItems: 'center' }}>
          <div role="tablist" aria-label="Topology level" style={{ display: 'flex', gap: 2 }}>
            {ALL_LEVELS.map((lvl) => {
              const active = level === lvl;
              const label = lvl === 1 ? 'L1 Business' : lvl === 2 ? 'L2 Component' : 'L3 Data';
              return (
                <button
                  key={lvl}
                  type="button"
                  role="tab"
                  aria-selected={active}
                  onClick={() => setLevel(lvl)}
                  style={{
                    background: active ? '#1f6feb' : 'transparent',
                    color: active ? '#ffffff' : '#8b949e',
                    border: '1px solid ' + (active ? '#1f6feb' : '#21262d'),
                    borderRadius: 4,
                    padding: '2px 8px',
                    fontSize: 11,
                    cursor: 'pointer',
                    fontWeight: active ? 600 : 400,
                  }}
                >
                  {label}
                </button>
              );
            })}
          </div>
          <div style={{ display: 'flex', gap: 8, fontSize: 11, marginLeft: 8 }}>
            <span style={{ color: '#00bcd4' }}>● Frontend</span>
            <span style={{ color: '#9c27b0' }}>● Backend</span>
            <span style={{ color: '#ff9800' }}>● Shared</span>
            <span style={{ color: '#4caf50' }}>● Test</span>
          </div>
        </div>
      </div>
      <div
        ref={containerRef}
        style={{
          flex: 1,
          background: '#0d1117',
        }}
      >
        {visibleNodes.length === 0 && (
          <div style={{
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
            height: '100%',
            color: '#484f58',
            fontStyle: 'italic',
          }}>
            {nodes.length === 0
              ? 'No topology data available'
              : `No nodes at level L${level}`}
          </div>
        )}
      </div>
    </div>
  );
}
