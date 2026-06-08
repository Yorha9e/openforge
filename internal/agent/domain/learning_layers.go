package domain

import (
	"context"
	"regexp"
	"strings"
)

// L1Insight captures a single function signature discovered in a pipeline
// diff (L1: explicit knowledge extraction from code changes).
type L1Insight struct {
	PipelineID string
	FuncName   string
	Params     string
	Source     string // raw matched line, for auditing
}

// L2Insight summarises the line-level diff statistics between an old
// and new version of a file/patch (L2: feedback signal).
type L2Insight struct {
	PipelineID  string
	OldLines    int
	NewLines    int
	CommonLines int
}

// L3Insight aggregates tool frequency and step counts derived from a
// pipeline's stored trajectory (L3: implicit learning from execution).
type L3Insight struct {
	PipelineID    string
	ToolFrequency map[string]int
	StepCount     int
}

// L4Insight wraps the top-K embedding hits produced for a query string
// (L4: semantic recall from the in-memory embedding index).
type L4Insight struct {
	Query   string
	TopHits []EmbeddingHit
}

// LearningLayers is the four-layer self-learning coordinator described in
// §3.12 / T9: explicit (L1), feedback (L2), trajectory (L3), and
// embedding-based recall (L4). It composes the existing stores and the
// InMemoryEmbeddingIndex (Round 1 T8) into a single entry point that
// LearningService calls after a pipeline completes.
type LearningLayers struct {
	trajStore      TrajectoryStore
	knowledgeStore KnowledgeSnapshotStore
	preferenceStore PreferenceStore
	embedding      *InMemoryEmbeddingIndex
}

// NewLearningLayers wires the four layer dependencies. Any nil argument
// is tolerated; the corresponding Extract method returns a zero-value
// insight when its backing store is unavailable.
func NewLearningLayers(
	trajStore TrajectoryStore,
	knowledgeStore KnowledgeSnapshotStore,
	preferenceStore PreferenceStore,
	embedding *InMemoryEmbeddingIndex,
) *LearningLayers {
	return &LearningLayers{
		trajStore:       trajStore,
		knowledgeStore:  knowledgeStore,
		preferenceStore: preferenceStore,
		embedding:       embedding,
	}
}

// l1FuncSignature matches Go-style function signatures like
// `func Hello() {}` or `func World(x int) string`.
var l1FuncSignature = regexp.MustCompile(`func\s+(\w+)\s*\(([^)]*)\)`)

// ExtractL1 scans a diff for function signatures and returns one
// L1Insight per match, in source order.
func (l *LearningLayers) ExtractL1(pipelineID, diff string) []L1Insight {
	matches := l1FuncSignature.FindAllStringSubmatch(diff, -1)
	insights := make([]L1Insight, 0, len(matches))
	for _, m := range matches {
		insights = append(insights, L1Insight{
			PipelineID: pipelineID,
			FuncName:   m[1],
			Params:     strings.TrimSpace(m[2]),
			Source:     m[0],
		})
	}
	return insights
}

// ExtractL2 produces a simple line-level diff statistic. The "common"
// count is the number of lines present in both inputs — a coarse
// signal of how stable the file is across the change.
func (l *LearningLayers) ExtractL2(pipelineID, oldDiff, newDiff string) L2Insight {
	oldLines := splitNonEmpty(oldDiff)
	newLines := splitNonEmpty(newDiff)
	oldSet := make(map[string]struct{}, len(oldLines))
	for _, ln := range oldLines {
		oldSet[ln] = struct{}{}
	}
	common := 0
	for _, ln := range newLines {
		if _, ok := oldSet[ln]; ok {
			common++
		}
	}
	return L2Insight{
		PipelineID:  pipelineID,
		OldLines:    len(oldLines),
		NewLines:    len(newLines),
		CommonLines: common,
	}
}

// ExtractL3 reads the trajectory store for the given pipeline and
// summarises tool usage and step count. Returns a zero-valued insight
// when no trajectory exists.
func (l *LearningLayers) ExtractL3(pipelineID string) L3Insight {
	if l.trajStore == nil {
		return L3Insight{PipelineID: pipelineID, ToolFrequency: map[string]int{}}
	}
	traj, err := l.trajStore.GetByPipeline(context.Background(), pipelineID)
	if err != nil || traj == nil {
		return L3Insight{PipelineID: pipelineID, ToolFrequency: map[string]int{}}
	}
	freq := make(map[string]int, len(traj.ToolsUsed))
	for _, tool := range traj.ToolsUsed {
		freq[tool]++
	}
	return L3Insight{
		PipelineID:    pipelineID,
		ToolFrequency: freq,
		StepCount:     len(traj.StageSequence),
	}
}

// ExtractL4 returns the top-3 embedding hits for the query string. When
// the embedding index is not configured, returns an empty hit list.
func (l *LearningLayers) ExtractL4(query string) L4Insight {
	if l.embedding == nil {
		return L4Insight{Query: query, TopHits: []EmbeddingHit{}}
	}
	return L4Insight{
		Query:   query,
		TopHits: l.embedding.Query(query, 3),
	}
}

func splitNonEmpty(s string) []string {
	parts := strings.Split(s, "\n")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p == "" {
			continue
		}
		out = append(out, p)
	}
	return out
}
