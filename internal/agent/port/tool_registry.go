package port

import "context"

type ToolRegistryClient interface {
	SearchTools(ctx context.Context, query string, topK int) ([]ToolMatch, error)
}

type ToolMatch struct {
	Name        string
	Description string
	Score       float64
}
