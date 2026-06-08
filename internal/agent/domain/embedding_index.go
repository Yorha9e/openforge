package domain

import (
	"context"
	"sort"
	"strings"
	"sync"
)

// InMemoryEmbeddingIndex is a thread-safe in-memory embedding index used by
// the KnowledgeQuerier for L4 embedding-level retrieval. It is intentionally
// simple: term-frequency scoring with a corpus-wide frequency weight (id)
// and a tokenization step that lowercases and splits on whitespace.
//
// This is the Phase 7 in-process implementation; a vector-DB backed
// implementation can be added later behind the same `EmbeddingIndex`
// interface without touching the call sites.
type InMemoryEmbeddingIndex struct {
	mu   sync.RWMutex
	docs map[string][]string // id → token list
	idf  map[string]float64  // token → corpus frequency (used as weight)
}

// EmbeddingHit is a single hit returned by Query.
type EmbeddingHit struct {
	ID    string
	Score float64
}

// NewInMemoryEmbeddingIndex creates a new empty in-memory index.
func NewInMemoryEmbeddingIndex() *InMemoryEmbeddingIndex {
	return &InMemoryEmbeddingIndex{
		docs: make(map[string][]string),
		idf:  make(map[string]float64),
	}
}

// Add inserts or replaces a document with the given id and text.
// Document text is tokenized once and the per-token corpus frequency is
// incremented so subsequent queries can weight rare terms higher.
func (i *InMemoryEmbeddingIndex) Add(id, text string) {
	tokens := tokenize(text)
	i.mu.Lock()
	defer i.mu.Unlock()
	i.docs[id] = tokens
	for _, t := range tokens {
		i.idf[t]++
	}
}

// Query returns up to topK hits for the given query, ordered by descending
// score. When the index is empty, Query returns an empty slice.
func (i *InMemoryEmbeddingIndex) Query(query string, topK int) []EmbeddingHit {
	i.mu.RLock()
	defer i.mu.RUnlock()

	if len(i.docs) == 0 {
		return []EmbeddingHit{}
	}

	qTokens := tokenize(query)
	if len(qTokens) == 0 {
		return []EmbeddingHit{}
	}

	scores := make(map[string]float64)
	for id, docTokens := range i.docs {
		var score float64
		for _, qt := range qTokens {
			w := i.idf[qt]
			if w == 0 {
				continue
			}
			for _, dt := range docTokens {
				if qt == dt {
					score += w
				}
			}
		}
		if score > 0 {
			scores[id] = score
		}
	}

	type kv struct {
		id    string
		score float64
	}
	kvs := make([]kv, 0, len(scores))
	for id, s := range scores {
		kvs = append(kvs, kv{id: id, score: s})
	}
	sort.Slice(kvs, func(a, b int) bool { return kvs[a].score > kvs[b].score })

	if topK > 0 && len(kvs) > topK {
		kvs = kvs[:topK]
	}

	hits := make([]EmbeddingHit, len(kvs))
	for j, item := range kvs {
		hits[j] = EmbeddingHit{ID: item.id, Score: item.score}
	}
	return hits
}

// tokenize lowercases input and splits on ASCII whitespace. Empty tokens
// (e.g. from consecutive separators) are dropped.
func tokenize(s string) []string {
	s = strings.ToLower(s)
	raw := strings.Fields(s)
	out := make([]string, 0, len(raw))
	for _, t := range raw {
		if t != "" {
			out = append(out, t)
		}
	}
	return out
}

// compile-time interface check that InMemoryEmbeddingIndex satisfies the
// EmbeddingIndex interface used by KnowledgeQuerier.
var _ EmbeddingIndex = (*inMemoryEmbeddingIndexAdapter)(nil)

// inMemoryEmbeddingIndexAdapter adapts InMemoryEmbeddingIndex to the
// EmbeddingIndex interface declared in knowledge_querier.go (which uses
// Search(ctx, query, topK) ([]SearchResult, error)). The adapter is
// implemented as a thin wrapper so the in-memory type does not have to
// change its public API.
type inMemoryEmbeddingIndexAdapter struct {
	idx *InMemoryEmbeddingIndex
}

// Search implements EmbeddingIndex by delegating to the underlying index
// and converting EmbeddingHit → SearchResult.
func (a *inMemoryEmbeddingIndexAdapter) Search(_ context.Context, query string, topK int) ([]SearchResult, error) {
	hits := a.idx.Query(query, topK)
	out := make([]SearchResult, len(hits))
	for i, h := range hits {
		out[i] = SearchResult{ID: h.ID, Score: h.Score}
	}
	return out, nil
}

// AsEmbeddingIndex wraps the in-memory index so it can be injected into
// KnowledgeQuerier.SetEmbeddingIndex. Callers that want the richer
// Add/Query API use the *InMemoryEmbeddingIndex directly.
func (i *InMemoryEmbeddingIndex) AsEmbeddingIndex() EmbeddingIndex {
	return &inMemoryEmbeddingIndexAdapter{idx: i}
}
