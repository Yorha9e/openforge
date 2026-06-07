package contract

import (
	"testing"

	agentv1 "openforge/gen/go/agent/v1"
	"github.com/stretchr/testify/require"
)

func TestContract_CoordinatorCreateAgent_RequestShape(t *testing.T) {
	golden := loadGolden(t, "golden/coordinator_create_agent.req.json")
	req := &agentv1.CreateAgentRequest{}
	assertRoundTrip(t, golden, req)

	require.Equal(t, "agent-001", req.GetAgentId())
	require.Equal(t, "p-001", req.GetPipelineId())
	require.Equal(t, "proj-1", req.GetProjectId())
	require.Equal(t, agentv1.AgentRole_AGENT_ROLE_WORKER, req.GetRole())
	require.Equal(t, "dev", req.GetMetadata()["env"])
	require.Equal(t, "team-a", req.GetMetadata()["owner"])
}

func TestContract_CoordinatorCreateAgent_ResponseShape(t *testing.T) {
	golden := loadGolden(t, "golden/coordinator_create_agent.resp.json")
	resp := &agentv1.CreateAgentResponse{}
	assertRoundTrip(t, golden, resp)

	require.Equal(t, "agent-001", resp.GetAgentId())
	require.Equal(t, "RUNNING", resp.GetStatus())
}
