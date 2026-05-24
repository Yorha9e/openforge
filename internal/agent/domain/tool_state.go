package domain

type ToolState int

const (
	ToolStateQueued    ToolState = iota
	ToolStateExecuting
	ToolStateCompleted
	ToolStateYielded
	ToolStateFailed
)

func (s ToolState) String() string {
	switch s {
	case ToolStateQueued:
		return "QUEUED"
	case ToolStateExecuting:
		return "EXECUTING"
	case ToolStateCompleted:
		return "COMPLETED"
	case ToolStateYielded:
		return "YIELDED"
	case ToolStateFailed:
		return "FAILED"
	default:
		return "UNKNOWN"
	}
}

func (s ToolState) IsTerminal() bool {
	return s == ToolStateCompleted || s == ToolStateFailed
}
