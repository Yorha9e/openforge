import { useEffect, useRef } from 'react';
import cytoscape from 'cytoscape';
import { tokens } from '../../shared/design-tokens';

interface TopologyNode {
  id: string;
  label: string;
  type: 'frontend' | 'backend' | 'shared' | 'test';
}

interface TopologyEdge {
  source: string;
  target: string;
}

export function TopologyPanel({ nodes = [], edges = [] }: { nodes?: TopologyNode[]; edges?: TopologyEdge[] }) {
  const containerRef = useRef<HTMLDivElement>(null);
  const cyRef = useRef<cytoscape.Core | null>(null);

  useEffect(() => {
    if (!containerRef.current) return;

    if (nodes.length === 0) {
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
      ...nodes.map(node => ({
        data: {
          id: node.id,
          label: node.label,
          type: node.type,
        },
      })),
      ...edges.map((edge, index) => ({
        data: {
          id: `e${index}`,
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
  }, [nodes, edges]);

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
        <div style={{ display: 'flex', gap: 8, fontSize: 11 }}>
          <span style={{ color: '#00bcd4' }}>● Frontend</span>
          <span style={{ color: '#9c27b0' }}>● Backend</span>
          <span style={{ color: '#ff9800' }}>● Shared</span>
          <span style={{ color: '#4caf50' }}>● Test</span>
        </div>
      </div>
      <div
        ref={containerRef}
        style={{
          flex: 1,
          background: '#0d1117',
        }}
      >
        {nodes.length === 0 && (
          <div style={{
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
            height: '100%',
            color: '#484f58',
            fontStyle: 'italic',
          }}>
            No topology data available
          </div>
        )}
      </div>
    </div>
  );
}