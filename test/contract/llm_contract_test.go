package contract

import (
	"testing"

	agentv1 "openforge/gen/go/agent/v1"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

// assertRoundTrip unmarshals golden JSON into msg, then re-marshals it
// back to JSON. The two byte streams must be JSON-equal.
//
// We use MarshalOptions{UseProtoNames: true, EmitUnpopulated: true} so
// the output uses snake_case field names (matching golden) AND keeps
// zero-value fields (false, "", 0) in the output, which protojson
// would otherwise drop.
func assertRoundTrip(t *testing.T, golden []byte, msg proto.Message) {
	t.Helper()
	require.NoError(t, protojson.Unmarshal(golden, msg), "unmarshal golden")
	opts := protojson.MarshalOptions{
		UseProtoNames:   true,
		EmitUnpopulated: true,
	}
	out, err := opts.Marshal(msg)
	require.NoError(t, err, "marshal back to json")
	require.JSONEq(t, string(golden), string(out), "Go message must round-trip via protojson")
}

// TestContract_LLMChat_RequestShape covers coordinator.ChatRequest —
// the LLM Chat entry point that the LLM router consumes.
func TestContract_LLMChat_RequestShape(t *testing.T) {
	golden := loadGolden(t, "golden/llm_chat.req.json")
	req := &agentv1.ChatRequest{}
	assertRoundTrip(t, golden, req)

	require.Equal(t, "p-001", req.GetPipelineId())
	require.Equal(t, "b-main", req.GetBranchId())
	require.Len(t, req.GetMessages(), 1)
	require.Equal(t, agentv1.ChatRole_CHAT_ROLE_USER, req.GetMessages()[0].GetRole())
	require.Equal(t, "Hello", req.GetMessages()[0].GetContent())
	require.Equal(t, int32(1), req.GetMessages()[0].GetMsgSeq())
}

// TestContract_LLMChat_ResponseShape covers coordinator.ChatEvent —
// the streaming response shape the BFF uses to bridge gRPC→WS.
func TestContract_LLMChat_ResponseShape(t *testing.T) {
	golden := loadGolden(t, "golden/llm_chat.resp.json")
	ev := &agentv1.ChatEvent{}
	assertRoundTrip(t, golden, ev)

	require.Equal(t, int32(1), ev.GetMsgSeq())
	require.Equal(t, agentv1.ChatEventType_CHAT_EVENT_TYPE_DELTA, ev.GetEventType())
	require.Equal(t, "Hi", ev.GetDeltaText())
	require.False(t, ev.GetIsDone())
}
