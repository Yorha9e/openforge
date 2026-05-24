package port

import "context"

// Tool is the generic interface all tools must implement.
type Tool[Input any, Output any] interface {
	Name() string
	Description() string
	InputSchema() []byte
	IsConcurrencySafe() bool
	IsReadOnly() bool
	Execute(ctx context.Context, input Input) (Output, error)
}

// StreamingTool adds streaming execution capability.
type StreamingTool[Input any, Output any] interface {
	Tool[Input, Output]
	ExecuteStream(ctx context.Context, input Input) (<-chan StreamChunk[Output], error)
}

type StreamChunk[T any] struct {
	Value T
	Err   error
	Done  bool
}
