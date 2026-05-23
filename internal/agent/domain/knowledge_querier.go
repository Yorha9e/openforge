// Package domain provides knowledge querying for OpenForge prompt building.
// This file implements the KnowledgeQuerier which integrates with the Learning Engine
// to query relevant knowledge, trajectories, and preferences for prompts.
package domain

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"
)

// KnowledgeQuerier queries relevant knowledge from the Learning Engine.
// It is an L2 sub-component, called by L2Builder, not an independent injector.
type KnowledgeQuerier struct {
	learningEngine LearningEngine
	embeddingIndex EmbeddingIndex
	cache          *KnowledgeCache
	mu             sync.RWMutex
}

// LearningEngine interface for interacting with the learning system
type LearningEngine interface {
	QueryKnowledge(ctx context.Context, req *QueryKnowledgeRequest) (*QueryKnowledgeResponse, error)
	MatchTrajectory(ctx context.Context, req *MatchTrajectoryRequest) (*MatchTrajectoryResponse, error)
	WriteKnowledge(ctx context.Context, req *WriteKnowledgeRequest) error
}

// EmbeddingIndex interface for embedding-based search
type EmbeddingIndex interface {
	Search(ctx context.Context, query string, topK int) ([]SearchResult, error)
}

// QueryKnowledgeRequest request for querying knowledge
type QueryKnowledgeRequest struct {
	ProjectID string
	Query     string
	TopK      int
	Level     string // "L1_static", "L2_feedback", "L3_trajectory", "L4_embedding", "all"
}

// QueryKnowledgeResponse response from knowledge query
type QueryKnowledgeResponse struct {
	Items []KnowledgeItem
}

// KnowledgeItem represents a piece of knowledge
type KnowledgeItem struct {
	ID        string
	Key       string
	Value     string
	Source    string
	Score     float64
	Level     string
	Timestamp int64
}

// MatchTrajectoryRequest request for matching trajectories
type MatchTrajectoryRequest struct {
	ProjectID string
	Query     string
	TopK      int
	Level     string // "L3_trajectory", "L4_embedding"
}

// MatchTrajectoryResponse response from trajectory matching
type MatchTrajectoryResponse struct {
	Trajectories []Trajectory
}

// Trajectory represents a successful/failed trajectory
type Trajectory struct {
	ID        string
	Summary   string
	Steps     []TrajectoryStep
	Outcome   string // "success" or "failure"
	Score     float64
	Timestamp int64
}

// TrajectoryStep represents a step in a trajectory
type TrajectoryStep struct {
	Action   string
	Target   string
	Result   string
	Duration int64
}

// WriteKnowledgeRequest request for writing knowledge
type WriteKnowledgeRequest struct {
	ProjectID   string
	PipelineID  string
	Preferences []Preference
	Trajectory  *Trajectory
}

// Preference represents a learned preference
type Preference struct {
	Key    string
	Value  string
	Source string
}

// SearchResult represents a search result from embedding index
type SearchResult struct {
	ID      string
	Content string
	Score   float64
}

// KnowledgeCache caches knowledge queries
type KnowledgeCache struct {
	cache map[string]*queryCacheEntry
	mu    sync.RWMutex
}

type queryCacheEntry struct {
	content   string
	expiresAt time.Time
}

// NewKnowledgeQuerier creates a new KnowledgeQuerier
func NewKnowledgeQuerier() *KnowledgeQuerier {
	return &KnowledgeQuerier{
		cache: &KnowledgeCache{
			cache: make(map[string]*queryCacheEntry),
		},
	}
}

// SetLearningEngine sets the learning engine dependency
func (kq *KnowledgeQuerier) SetLearningEngine(engine LearningEngine) {
	kq.mu.Lock()
	defer kq.mu.Unlock()
	kq.learningEngine = engine
}

// SetEmbeddingIndex sets the embedding index dependency
func (kq *KnowledgeQuerier) SetEmbeddingIndex(index EmbeddingIndex) {
	kq.mu.Lock()
	defer kq.mu.Unlock()
	kq.embeddingIndex = index
}

// Query queries relevant knowledge and trajectories.
// Returns empty string when learningEngine is nil (Phase 7: real implementation).
func (kq *KnowledgeQuerier) Query(ctx context.Context, projectID, query string) (string, error) {
	if kq.learningEngine == nil {
		return "", nil
	}

	key := fmt.Sprintf("%s:%s", projectID, query)
	if entry, ok := kq.cache.Load(key); ok {
		e := entry.(*queryCacheEntry)
		if time.Now().Before(e.expiresAt) {
			return e.content, nil
		}
	}

	prefs, _ := kq.learningEngine.QueryKnowledge(ctx, &QueryKnowledgeRequest{
		ProjectID: projectID, Query: query, TopK: 5, Level: "all",
	})
	trajs, _ := kq.learningEngine.MatchTrajectory(ctx, &MatchTrajectoryRequest{
		ProjectID: projectID, Query: query, TopK: 3, Level: "L3_trajectory",
	})

	content := kq.format(prefs, trajs)
	kq.cache.Store(key, &queryCacheEntry{content: content, expiresAt: time.Now().Add(5 * time.Minute)})
	return content, nil
}

func (kq *KnowledgeQuerier) format(prefs *QueryKnowledgeResponse, trajs *MatchTrajectoryResponse) string {
	var parts []string

	if prefs != nil && len(prefs.Items) > 0 {
		var b strings.Builder
		b.WriteString("<learned_preferences>\n")
		for _, item := range prefs.Items {
			b.WriteString(fmt.Sprintf("<preference key=\"%s\" source=\"%s\">%s</preference>\n",
				item.Key, item.Source, item.Value))
		}
		b.WriteString("</learned_preferences>\n")
		parts = append(parts, b.String())
	}

	if trajs != nil && len(trajs.Trajectories) > 0 {
		var b strings.Builder
		b.WriteString("<relevant_trajectories>\n")
		for _, traj := range trajs.Trajectories {
			b.WriteString(fmt.Sprintf("<trajectory outcome=\"%s\" score=\"%.2f\">\n", traj.Outcome, traj.Score))
			b.WriteString(fmt.Sprintf("<summary>%s</summary>\n", traj.Summary))
			b.WriteString("</trajectory>\n")
		}
		b.WriteString("</relevant_trajectories>\n")
		parts = append(parts, b.String())
	}

	if len(parts) == 0 {
		return ""
	}
	return "<knowledge_context>\n" + strings.Join(parts, "\n") + "\n</knowledge_context>\n"
}

// Cache methods

func (kc *KnowledgeCache) Load(key string) (interface{}, bool) {
	kc.mu.RLock()
	defer kc.mu.RUnlock()
	if entry, ok := kc.cache[key]; ok {
		if time.Now().Before(entry.expiresAt) {
			return entry, true
		}
		delete(kc.cache, key)
	}
	return nil, false
}

func (kc *KnowledgeCache) Store(key string, entry *queryCacheEntry) {
	kc.mu.Lock()
	defer kc.mu.Unlock()
	kc.cache[key] = entry
}

func (kc *KnowledgeCache) Delete(key string) {
	kc.mu.Lock()
	defer kc.mu.Unlock()
	delete(kc.cache, key)
}

func (kc *KnowledgeCache) Clear() {
	kc.mu.Lock()
	defer kc.mu.Unlock()
	kc.cache = make(map[string]*queryCacheEntry)
}

// Helpers retained from original knowledge_injector.go

func truncateStringWithEllipsis(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

func isReadOnlyTool(name string) bool {
	readOnlyTools := map[string]bool{
		"read_file":             true,
		"search_content":        true,
		"search_files":          true,
		"analyze_topology":      true,
		"lsp_hover":             true,
		"lsp_definition":        true,
		"lsp_references":        true,
		"lsp_symbols":           true,
		"list_models":           true,
		"check_token_budget":    true,
		"query_module_ownership": true,
		"validate_artifact_hash": true,
		"generate_artifact_url": true,
	}
	return readOnlyTools[name]
}
