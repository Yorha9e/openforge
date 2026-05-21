package kernel

// PipelineID is the unique identifier for a pipeline.
type PipelineID string

// ProjectID is the unique identifier for a project.
type ProjectID string

// AgentID is the unique identifier for an agent.
type AgentID string

// Level represents the capability level of a deployment profile.
type Level string

const (
	// LevelL1 is the minimal capability profile.
	LevelL1 Level = "L1"
	// LevelL2 is the standard capability profile.
	LevelL2 Level = "L2"
	// LevelL3 is the enterprise capability profile.
	LevelL3 Level = "L3"
	// LevelL4 is reserved for future expansion.
	LevelL4 Level = "L4"
)

// IsValid reports whether the level is one of the defined constants.
func (l Level) IsValid() bool {
	switch l {
	case LevelL1, LevelL2, LevelL3, LevelL4:
		return true
	}
	return false
}

// StageType represents a stage in the pipeline lifecycle.
type StageType string

const (
	StageClarify   StageType = "clarify"
	StageDecompose StageType = "decompose"
	StageImpl      StageType = "impl"
	StageTest      StageType = "test"
	StageDeploy    StageType = "deploy"
	StageVerify    StageType = "verify"
)

// PipelineStatus represents the current status of a pipeline run.
type PipelineStatus string

const (
	StatusPending        PipelineStatus = "pending"
	StatusRunning        PipelineStatus = "running"
	StatusPaused         PipelineStatus = "paused"
	StatusAwaitingReview PipelineStatus = "awaiting_review"
	StatusCompleted      PipelineStatus = "completed"
	StatusRejected       PipelineStatus = "rejected"
	StatusTokenExceeded  PipelineStatus = "token_exceeded"
	StatusCancelled      PipelineStatus = "cancelled"
	StatusDormant        PipelineStatus = "dormant"
)

// IsTerminal reports whether the status represents a terminal state.
func (s PipelineStatus) IsTerminal() bool {
	switch s {
	case StatusCompleted, StatusRejected, StatusTokenExceeded, StatusCancelled:
		return true
	}
	return false
}

// AgentRole represents the role of an agent in a pipeline run.
type AgentRole string

const (
	RolePlanner  AgentRole = "planner"
	RoleWorker   AgentRole = "worker"
	RoleReviewer AgentRole = "reviewer"
)

// ParseContextWindow extracts the context window size from a model name suffix.
// Recognised suffixes: [Nm] (million), [Nk] (thousand). Example:
//
//	"deepseek-v4-pro[1m]"   → 1_048_576
//	"claude-sonnet[200k]"   → 204_800
//	"mimo-v2.5-pro"         → 0, false (no suffix)
//
// The suffix is stripped from the returned model name.
func ParseContextWindow(model string) (cleanName string, contextWindow int) {
	n := len(model)
	// Suffix must end with ']' and contain '['.
	if n < 3 || model[n-1] != ']' {
		return model, 0
	}
	// Find the matching '[' — it must be the character before the suffix content.
	brace := -1
	for i := n - 2; i >= 0; i-- {
		if model[i] == '[' {
			brace = i
			break
		}
	}
	if brace == -1 {
		return model, 0
	}
	suffix := model[brace+1 : n-1]
	if len(suffix) < 2 {
		return model, 0
	}
	multiplier := 1
	switch suffix[len(suffix)-1] {
	case 'm', 'M':
		multiplier = 1_048_576
	case 'k', 'K':
		multiplier = 1_024
	default:
		return model, 0
	}
	numStr := suffix[:len(suffix)-1]
	val := 0
	for _, ch := range numStr {
		if ch < '0' || ch > '9' {
			return model, 0
		}
		val = val*10 + int(ch-'0')
	}
	if val <= 0 {
		return model, 0
	}
	return model[:brace], val * multiplier
}
