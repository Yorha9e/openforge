package port

import (
	"context"
	"time"

	"openforge/internal/pipeline/domain"
)

type PipelineRepository interface {
	Create(ctx context.Context, p *domain.Pipeline) error
	GetByID(ctx context.Context, id string) (*domain.Pipeline, error)
	ListByProject(ctx context.Context, projectID string) ([]*domain.Pipeline, error)
	UpdateStatus(ctx context.Context, id string, status string, version int) error
	IncrementBacktrack(ctx context.Context, id string) error
	Delete(ctx context.Context, id string) error
}

type GateRepository interface {
	CreateEvent(ctx context.Context, ev *domain.GateEvent) error
	GetLatestHash(ctx context.Context, pipelineID string) (string, error)
	ListByPipeline(ctx context.Context, pipelineID string) ([]*domain.GateEvent, error)
	ListPending(ctx context.Context, actor string) ([]*domain.GateEvent, error)
	Claim(ctx context.Context, pipelineID, stage, actor string, ttl time.Duration) error
	ReleaseClaim(ctx context.Context, pipelineID, stage, actor string) error
}

// TokenCostRow holds one aggregated data point for cost reporting.
type TokenCostRow struct {
	Date             string
	ProjectID        string
	Provider         string
	Model            string
	PromptTokens     int64
	CompletionTokens int64
	EstimatedCost    float64
}

// ProjectBudget holds monthly budget config for a project.
type ProjectBudget struct {
	ProjectID      string
	MonthlyLimit   int64
	CurrentUsage   int64
	CostLimit      float64
	CurrentCost    float64
	ResetAt        time.Time
}

type TokenCostRepository interface {
	AggregateByDay(ctx context.Context, projectID string, days int) ([]TokenCostRow, error)
	AggregateByModel(ctx context.Context, projectID string, days int) ([]TokenCostRow, error)
	GetProjectBudget(ctx context.Context, projectID string) (*ProjectBudget, error)
	GetCurrentMonthUsage(ctx context.Context, projectID string) (int64, float64, error)

	// RecordTokenUsage 写入单条 token_usage 记录。供 Node.js TokenMeter.flush() 通过
	// gRPC RecordTokenUsage 调用。token_usage.id 是 BIGSERIAL + 分区键(created_at)的
	// 复合主键，幂等由调用方保证（同 ID 不会重复 flush）。
	RecordTokenUsage(ctx context.Context, rec TokenUsageRecord) error

	// BatchRecordTokenUsage 批量写入，封装在单个事务内。
	BatchRecordTokenUsage(ctx context.Context, recs []TokenUsageRecord) error

	// GetBudget 读 cost_quota 中给定 project 的月度预算（美元）。0 表示无限制。
	GetBudget(ctx context.Context, projectID string) (monthlyUSD float64, err error)

	// SetBudget 覆盖写 cost_quota 行（PK = project_id）。monthlyUSD=0 表示清空。
	SetBudget(ctx context.Context, projectID string, monthlyUSD float64) error
}

// TokenUsageRecord 与 token_usage 表列一一对应；pipeline_id/project_id 来自上游 QueryEngine。
// 注：DB 中 id 列为 BIGSERIAL（与 created_at 组成复合主键），调用方传入的 ID 仅用于
// gRPC 层回执关联，不参与 INSERT；落库 id 由 DB 序列生成。
type TokenUsageRecord struct {
	ID               string
	PipelineID       string
	ProjectID        string
	Provider         string
	Model            string
	PromptTokens     int64
	CompletionTokens int64
	EstimatedCost    float64
	CreatedAt        time.Time
}

// DBMessage mirrors the conversation_message table.
type DBMessage struct {
	ID         string    `json:"id"`
	PipelineID string    `json:"pipeline_id"`
	BranchID   string    `json:"branch_id"`
	MsgSeq     int       `json:"msg_seq"`
	Role       string    `json:"role"`
	MsgType    string    `json:"msg_type"`
	Content    string    `json:"content"`
	TokenCount int       `json:"token_count"`
	CreatedAt  time.Time `json:"created_at"`
}

// DBBranch mirrors the conversation_branch table.
type DBBranch struct {
	ID           string    `json:"id"`
	PipelineID   string    `json:"pipeline_id"`
	ParentBranch string    `json:"parent_branch"`
	ForkMsgSeq   int       `json:"fork_msg_seq"`
	Status       string    `json:"status"`
	CreatedBy    string    `json:"created_by"`
	CreatedAt    time.Time `json:"created_at"`
}

// ConversationRepository persists chat messages and branch history.
type ConversationRepository interface {
	SaveMessage(ctx context.Context, msg *DBMessage) error
	BatchSaveMessages(ctx context.Context, msgs []*DBMessage) error
	GetMessages(ctx context.Context, pipelineID string, branchID string) ([]*DBMessage, error)
	CreateBranch(ctx context.Context, branch *DBBranch) error
	GetBranch(ctx context.Context, branchID string) (*DBBranch, error)
	GetActiveBranch(ctx context.Context, pipelineID string) (*DBBranch, error)
	ListBranches(ctx context.Context, pipelineID string) ([]*DBBranch, error)
}

// DBGateRequest mirrors the gate_request table for pending approval tracking.
type DBGateRequest struct {
	ID          string     `json:"id"`
	PipelineID  string     `json:"pipeline_id"`
	Stage       string     `json:"stage"`
	Status      string     `json:"status"` // pending, approved, rejected, timeout, cancelled
	RequestedBy string     `json:"requested_by"`
	ApprovedBy  *string    `json:"approved_by,omitempty"`
	ApprovedAt  *time.Time `json:"approved_at,omitempty"`
	Result      string     `json:"result"`
	TimeoutAt   time.Time  `json:"timeout_at"`
	CreatedAt   time.Time  `json:"created_at"`
}

// GateRequestRepository manages the lifecycle of pending gate approval requests.
type GateRequestRepository interface {
	CreateRequest(ctx context.Context, req *DBGateRequest) error
	UpdateRequestStatus(ctx context.Context, id string, status string, approvedBy string, result string) error
	GetPendingRequests(ctx context.Context) ([]*DBGateRequest, error)
	GetActiveRequest(ctx context.Context, pipelineID, stage string) (*DBGateRequest, error)
	HandleTimeouts(ctx context.Context) ([]string, error)
}

// StageRepository manages pipeline stage records.
type StageRepository interface {
	Create(ctx context.Context, s *domain.Stage) error
	GetByID(ctx context.Context, id string) (*domain.Stage, error)
	ListByPipeline(ctx context.Context, pipelineID string) ([]*domain.Stage, error)
	UpdateStatus(ctx context.Context, id string, status string, summary string) error
}
