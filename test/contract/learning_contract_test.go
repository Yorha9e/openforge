package contract

import (
	"testing"

	agentv1 "openforge/gen/go/agent/v1"
	"github.com/stretchr/testify/require"
)

func TestContract_LearningQueryKnowledge_RequestShape(t *testing.T) {
	golden := loadGolden(t, "golden/learning_query_knowledge.req.json")
	req := &agentv1.QueryKnowledgeRequest{}
	assertRoundTrip(t, golden, req)

	require.Equal(t, "proj-1", req.GetProjectId())
	require.Equal(t, "how to validate auth tokens", req.GetQuery())
	require.Equal(t, int32(5), req.GetTopK())
	require.Equal(t, []string{"code_style", "architecture"}, req.GetCategories())
	require.True(t, req.GetExcludeUntrusted())
}

func TestContract_LearningQueryKnowledge_ResponseShape(t *testing.T) {
	golden := loadGolden(t, "golden/learning_query_knowledge.resp.json")
	resp := &agentv1.QueryKnowledgeResponse{}
	assertRoundTrip(t, golden, resp)

	require.Len(t, resp.GetMatches(), 1)
	m := resp.GetMatches()[0]
	require.NotNil(t, m.GetEntry())
	require.Equal(t, "preference", m.GetEntry().GetType())
	require.Equal(t, "code_style", m.GetEntry().GetCategory())
	require.Equal(t, "Use zod for runtime validation", m.GetEntry().GetDescription())
	require.InDelta(t, 0.87, m.GetEntry().GetConfidence(), 0.0001)
	require.Equal(t, "pipeline_review", m.GetEntry().GetSource())
	require.InDelta(t, 0.84, m.GetSimilarityScore(), 0.0001)
	require.Equal(t, "k-001", m.GetKnowledgeId())
	require.Equal(t, "trusted", m.GetTrustLevel())
}
