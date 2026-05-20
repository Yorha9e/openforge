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
