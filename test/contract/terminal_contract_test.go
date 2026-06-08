package contract

import (
	"testing"

	agentv1 "openforge/gen/go/agent/v1"
	"github.com/stretchr/testify/require"
)

func TestContract_TerminalOpen_RequestShape(t *testing.T) {
	golden := loadGolden(t, "golden/terminal_open.req.json")
	req := &agentv1.OpenTerminalRequest{}
	assertRoundTrip(t, golden, req)

	require.Equal(t, "p-001", req.GetPipelineId())
	require.Equal(t, "ctr-abc", req.GetContainerId())
	require.Equal(t, agentv1.TerminalMode_TERMINAL_MODE_READ_ONLY, req.GetMode())
	require.Equal(t, "user@example.com", req.GetActor())
}

func TestContract_TerminalOpen_ResponseShape(t *testing.T) {
	golden := loadGolden(t, "golden/terminal_open.resp.json")
	resp := &agentv1.TerminalOutput{}
	assertRoundTrip(t, golden, resp)

	require.Equal(t, "stdout", resp.GetStream())
	require.Equal(t, []byte("hello\n"), resp.GetData())
	require.False(t, resp.GetIsDone())
}
