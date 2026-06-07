package contract

import (
	"testing"

	agentv1 "openforge/gen/go/agent/v1"
	"github.com/stretchr/testify/require"
)

func TestContract_GateApprove_RequestShape(t *testing.T) {
	golden := loadGolden(t, "golden/gate_approve.req.json")
	req := &agentv1.GateApproveRequest{}
	assertRoundTrip(t, golden, req)

	require.Equal(t, "p-001", req.GetPipelineId())
	require.Equal(t, agentv1.StageType_STAGE_TYPE_IMPL, req.GetStage())
	require.Equal(t, "reviewer@example.com", req.GetActor())

	cl := req.GetChecklist()
	require.NotNil(t, cl)
	require.True(t, cl.GetCodeReviewed())
	require.True(t, cl.GetSecurityChecked())
	require.True(t, cl.GetLicenseCleared())
	require.True(t, cl.GetCodingStandardMet())
}

func TestContract_GateApprove_ResponseShape(t *testing.T) {
	golden := loadGolden(t, "golden/gate_approve.resp.json")
	resp := &agentv1.GateApproveResponse{}
	assertRoundTrip(t, golden, resp)

	require.True(t, resp.GetSuccess())
	require.Equal(t, agentv1.PipelineStatus_PIPELINE_STATUS_RUNNING, resp.GetNextStatus())
}
