package contract

import (
	"testing"

	agentv1 "openforge/gen/go/agent/v1"
	"github.com/stretchr/testify/require"
)

func TestContract_ToolsSearch_RequestShape(t *testing.T) {
	golden := loadGolden(t, "golden/tools_search.req.json")
	req := &agentv1.SearchToolsRequest{}
	assertRoundTrip(t, golden, req)

	require.Equal(t, "search for files matching *.go", req.GetQuery())
	require.Equal(t, int32(3), req.GetTopK())
	require.Equal(t, []string{"file", "git"}, req.GetFilterCategories())
}

func TestContract_ToolsSearch_ResponseShape(t *testing.T) {
	golden := loadGolden(t, "golden/tools_search.resp.json")
	resp := &agentv1.SearchToolsResponse{}
	assertRoundTrip(t, golden, resp)

	require.Len(t, resp.GetMatches(), 1)
	m := resp.GetMatches()[0]
	require.NotNil(t, m.GetTool())
	require.Equal(t, "file_grep", m.GetTool().GetName())
	require.Equal(t, "file", m.GetTool().GetCategory())
	require.False(t, m.GetTool().GetIsDynamic())
	require.InDelta(t, 0.92, m.GetSimilarityScore(), 0.0001)
}
