package port

import "context"

type LearningEngineClient interface {
	WriteKnowledge(ctx context.Context, entry KnowledgeEntry) error
}

type KnowledgeEntry struct {
	Type       string
	Category   string
	Data       []byte
	Confidence float64
	Source     string
}
