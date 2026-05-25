package domain

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	agentport "openforge/internal/agent/port"
)

// QueryState represents the state of the query engine.
type QueryState string

const (
	QueryStateIdle          QueryState = "IDLE"
	QueryStateAwaitingLLM   QueryState = "AWAITING_LLM"
	QueryStateAwaitingTools QueryState = "AWAITING_TOOLS"
	QueryStateAwaitingUser  QueryState = "AWAITING_USER"
)

// StreamEvent represents a single event emitted during agent execution.
type StreamEvent struct {
	Type       string // "delta" | "tool_start" | "tool_done" | "tool_error" | "context_compress" | "done" | "error"
	Content    string
	ToolName   string
	ToolStatus string
	Error      error
}

// PipelineContext holds pipeline metadata needed for prompt building.
type PipelineContext struct {
	PipelineID     string
	ProjectID      string
	Stage          string
	StageLevel     string
	PermissionMode string
	UserRole       string
}

// CheckpointRepository saves checkpoints for recovery.
type CheckpointRepository interface {
	Save(ctx context.Context, cp *Checkpoint) error
}

// CheckpointData holds serialized state for recovery.
type CheckpointData struct {
	Messages   []agentport.Message `json:"messages"`
	TokenUsed  int64               `json:"token_used"`
	RoundCount int32               `json:"round_count"`
	Reason     string              `json:"reason"`
	Timestamp  time.Time           `json:"timestamp"`
}

// QueryEngine manages a conversational interaction with an LLM.
type QueryEngine struct {
	llmClient       agentport.LLMRouterClient
	config          agentport.LLMConfig
	messages        []agentport.Message
	tokenCount      int64
	state           QueryState
	promptBuilder   *PromptBuilder
	pipelineCtx     PipelineContext
	forceSkill      string
	toolRegistry    ToolRegistry
	compressor      *Compressor
	roundCount      int32
	checkpointSeq   int32
	checkpointCache []*Checkpoint
	checkpointRepo  CheckpointRepository
	mu              sync.Mutex
}

// NewQueryEngine creates a new QueryEngine with the given LLM client and config.
func NewQueryEngine(llmClient agentport.LLMRouterClient, config agentport.LLMConfig, promptBuilder *PromptBuilder, pipelineCtx PipelineContext) *QueryEngine {
	return &QueryEngine{
		llmClient:     llmClient,
		config:        config,
		state:         QueryStateIdle,
		promptBuilder: promptBuilder,
		pipelineCtx:   pipelineCtx,
		toolRegistry:  make(ToolRegistry),
	}
}

// SetToolRegistry sets the tool registry for multi-turn function calling.
func (qe *QueryEngine) SetToolRegistry(reg ToolRegistry) {
	qe.mu.Lock()
	defer qe.mu.Unlock()
	qe.toolRegistry = reg
}

// SetCompressor sets the context compressor for long conversations.
func (qe *QueryEngine) SetCompressor(comp *Compressor) {
	qe.mu.Lock()
	defer qe.mu.Unlock()
	qe.compressor = comp
}

// SetCheckpointRepo sets the checkpoint repository for recovery.
func (qe *QueryEngine) SetCheckpointRepo(repo CheckpointRepository) {
	qe.mu.Lock()
	defer qe.mu.Unlock()
	qe.checkpointRepo = repo
}

// State returns the current query engine state.
func (qe *QueryEngine) State() QueryState {
	qe.mu.Lock()
	defer qe.mu.Unlock()
	return qe.state
}

// SubmitMessage appends a user message and starts the multi-turn agent loop.
// First call is synchronous (Chat) for tool_use detection; subsequent calls
// loop through tool execution and LLM feedback (Phase 7.5).
func (qe *QueryEngine) SubmitMessage(ctx context.Context, msg string) (<-chan StreamEvent, error) {
	if strings.HasPrefix(msg, "/skill ") {
		skillName := strings.TrimSpace(strings.TrimPrefix(msg, "/skill "))
		if skillName != "" {
			qe.mu.Lock()
			qe.forceSkill = skillName
			qe.mu.Unlock()
			msg = fmt.Sprintf("Activate skill: %s", skillName)
		}
	}

	qe.mu.Lock()
	qe.messages = append(qe.messages, agentport.Message{Role: "user", Content: msg})
	qe.tokenCount += int64(len(msg) / 4)
	history := make([]agentport.Message, len(qe.messages))
	copy(history, qe.messages)
	qe.mu.Unlock()

	// Build system prompt via PromptBuilder (L1→L2→Capability→L4)
	systemPrompt := qe.buildPrompt(msg, history)

	trimmed := history
	if len(trimmed) > 40 {
		trimmed = trimmed[len(trimmed)-40:]
	}

	// First call: synchronous Chat for tool_use detection
	qe.mu.Lock()
	qe.state = QueryStateAwaitingLLM
	qe.mu.Unlock()

	resp, err := qe.llmClient.Chat(ctx, agentport.ChatRequest{
		Messages:     trimmed,
		SystemPrompt: systemPrompt,
		Config:       qe.config,
	})
	if err != nil {
		qe.mu.Lock()
		qe.messages = qe.messages[:len(qe.messages)-1]
		qe.mu.Unlock()
		out := make(chan StreamEvent, 1)
		out <- StreamEvent{Type: "error", Error: err}
		close(out)
		return out, nil
	}

	// Track tokens
	if resp.Usage != nil {
		atomic.AddInt64(&qe.tokenCount, resp.Usage.InputTokens+resp.Usage.OutputTokens)
	} else {
		atomic.AddInt64(&qe.tokenCount, int64(len(resp.Content)/4))
	}

	// Single-turn: LLM done, no tool calls
	if resp.StopReason == "end_turn" || resp.StopReason == "stop_sequence" {
		qe.mu.Lock()
		qe.messages = append(qe.messages, agentport.Message{Role: "assistant", Content: resp.Content})
		qe.state = QueryStateAwaitingUser
		qe.mu.Unlock()
		return singleTurnOut(resp.Content), nil
	}

	toolCalls := parseToolCalls(resp.Content)
	if len(toolCalls) == 0 {
		qe.mu.Lock()
		qe.messages = append(qe.messages, agentport.Message{Role: "assistant", Content: resp.Content})
		qe.state = QueryStateAwaitingUser
		qe.mu.Unlock()
		return singleTurnOut(resp.Content), nil
	}

	// Multi-turn: tool_use detected, enter tool loop
	qe.mu.Lock()
	qe.messages = append(qe.messages, agentport.Message{Role: "assistant", Content: resp.Content})
	qe.mu.Unlock()

	out := make(chan StreamEvent, 64)
	go qe.runToolLoop(ctx, out, resp, toolCalls)
	return out, nil
}

func singleTurnOut(content string) <-chan StreamEvent {
	out := make(chan StreamEvent, 2)
	out <- StreamEvent{Type: "delta", Content: content}
	out <- StreamEvent{Type: "done", Content: content}
	close(out)
	return out
}

// runToolLoop continues after tool_use is detected (Phase 7.5).
func (qe *QueryEngine) runToolLoop(ctx context.Context, out chan<- StreamEvent, firstResp *agentport.ChatResponse, firstToolCalls []ToolCallParsed) {
	defer close(out)

	out <- StreamEvent{Type: "delta", Content: firstResp.Content}

	toolCalls := firstToolCalls
	const maxRounds = 50

	for round := int32(0); round < maxRounds; round++ {
		select {
		case <-ctx.Done():
			out <- StreamEvent{Type: "error", Error: ctx.Err()}
			return
		default:
		}

		atomic.StoreInt32(&qe.roundCount, round)

		// Execute current round's tool calls
		qe.mu.Lock()
		qe.state = QueryStateAwaitingTools
		qe.mu.Unlock()

		for _, tc := range toolCalls {
			out <- StreamEvent{Type: "tool_start", ToolName: tc.Name, Content: tc.Name}
		}

		results := qe.toolRegistry.executeTools(ctx, toolCalls, DefaultToolErrorPolicy())

		for _, r := range results {
			eventType := "tool_done"
			if r.Status != "success" {
				eventType = "tool_error"
			}
			out <- StreamEvent{Type: eventType, ToolName: r.Tool, ToolStatus: r.Status, Content: r.Output}

			qe.mu.Lock()
			qe.messages = append(qe.messages, agentport.Message{Role: "tool", Content: formatToolResultXML(r)})
			qe.mu.Unlock()
		}

		// Token check after tool execution
		currentTokens := atomic.LoadInt64(&qe.tokenCount)
		if qe.compressor != nil && qe.compressor.NeedsCompression(currentTokens, 131072) {
			out <- StreamEvent{Type: "context_compress", Content: fmt.Sprintf("Compressing (%d tokens)", currentTokens)}
			qe.mu.Lock()
			compressed, err := qe.compressor.Compress(ctx, qe.messages, currentTokens, 131072, false)
			if err == nil {
				qe.messages = compressed
				qe.tokenCount = EstimateTokens(compressed)
				if qe.compressor.NeedsCompression(qe.tokenCount, 131072) {
					compressed2, err2 := qe.compressor.Compress(ctx, qe.messages, qe.tokenCount, 131072, true)
					if err2 == nil {
						qe.messages = compressed2
						qe.tokenCount = EstimateTokens(compressed2)
						if qe.compressor.NeedsCompression(qe.tokenCount, 131072) {
							qe.saveCheckpoint("context_overflow")
							qe.mu.Unlock()
							out <- StreamEvent{Type: "error", Error: fmt.Errorf("context_overflow")}
							return
						}
					}
				}
			}
			qe.mu.Unlock()
		}

		// Build system prompt for next LLM call
		qe.mu.Lock()
		msgsCopy := make([]agentport.Message, len(qe.messages))
		copy(msgsCopy, qe.messages)
		qe.mu.Unlock()

		systemPrompt := qe.buildPrompt("", msgsCopy)
		trimmed := msgsCopy
		if len(trimmed) > 40 {
			trimmed = trimmed[len(trimmed)-40:]
		}

		qe.mu.Lock()
		qe.state = QueryStateAwaitingLLM
		qe.mu.Unlock()

		resp, err := qe.llmClient.Chat(ctx, agentport.ChatRequest{
			Messages:     trimmed,
			SystemPrompt: systemPrompt,
			Config:       qe.config,
		})
		if err != nil {
			out <- StreamEvent{Type: "error", Error: fmt.Errorf("LLM call failed: %w", err)}
			return
		}

		if resp.Usage != nil {
			atomic.AddInt64(&qe.tokenCount, resp.Usage.InputTokens+resp.Usage.OutputTokens)
		} else {
			atomic.AddInt64(&qe.tokenCount, int64(len(resp.Content)/4))
		}

		out <- StreamEvent{Type: "delta", Content: resp.Content}

		if resp.StopReason == "end_turn" || resp.StopReason == "stop_sequence" {
			qe.mu.Lock()
			qe.messages = append(qe.messages, agentport.Message{Role: "assistant", Content: resp.Content})
			qe.state = QueryStateAwaitingUser
			qe.mu.Unlock()
			out <- StreamEvent{Type: "done", Content: resp.Content}
			return
		}

		toolCalls = parseToolCalls(resp.Content)
		if len(toolCalls) == 0 {
			qe.mu.Lock()
			qe.messages = append(qe.messages, agentport.Message{Role: "assistant", Content: resp.Content})
			qe.state = QueryStateAwaitingUser
			qe.mu.Unlock()
			out <- StreamEvent{Type: "done", Content: resp.Content}
			return
		}

		qe.mu.Lock()
		qe.messages = append(qe.messages, agentport.Message{Role: "assistant", Content: resp.Content})
		qe.mu.Unlock()
	}

	out <- StreamEvent{Type: "done", Content: "Maximum tool rounds reached."}
	qe.saveCheckpoint("max_rounds")
}

func (qe *QueryEngine) buildPrompt(userMsg string, messages []agentport.Message) string {
	if qe.promptBuilder == nil {
		return ""
	}
	buildReq := &BuildRequest{
		PipelineID:          qe.pipelineCtx.PipelineID,
		ProjectID:           qe.pipelineCtx.ProjectID,
		Stage:               qe.pipelineCtx.Stage,
		StageLevel:          qe.pipelineCtx.StageLevel,
		PermissionMode:      qe.pipelineCtx.PermissionMode,
		UserRole:            qe.pipelineCtx.UserRole,
		UserMessage:         userMsg,
		ConversationHistory: messages,
	}
	prompt, err := qe.promptBuilder.Build(context.Background(), buildReq)
	if err != nil || prompt == nil {
		return ""
	}
	return prompt.System
}

// parseToolCalls extracts tool calls from LLM content.
// Phase 7.5: heuristic from Anthropic tool_use patterns. Phase 8: structured content blocks.
func parseToolCalls(content string) []ToolCallParsed {
	var calls []ToolCallParsed
	idx := 0
	for {
		start := strings.Index(content[idx:], `"name"`)
		if start < 0 {
			break
		}
		start += idx + len(`"name"`)
		rest := strings.TrimLeft(content[start:], `": `)
		nameEnd := strings.IndexAny(rest, `",}`)
		if nameEnd < 0 {
			break
		}
		name := strings.TrimSpace(rest[:nameEnd])
		if name != "" {
			calls = append(calls, ToolCallParsed{ID: fmt.Sprintf("toolu-%d", len(calls)), Name: name, Args: make(map[string]interface{})})
		}
		idx = start + nameEnd
		if len(calls) >= 20 {
			break
		}
	}
	return calls
}

func (qe *QueryEngine) saveCheckpoint(reason string) {
	if qe.checkpointRepo == nil {
		return
	}
	seq := int(atomic.AddInt32(&qe.checkpointSeq, 1))
	qe.mu.Lock()
	msgsCopy := make([]agentport.Message, len(qe.messages))
	copy(msgsCopy, qe.messages)
	qe.mu.Unlock()

	data, _ := json.Marshal(CheckpointData{Messages: msgsCopy, TokenUsed: qe.tokenCount, RoundCount: atomic.LoadInt32(&qe.roundCount), Reason: reason, Timestamp: time.Now()})

	qe.mu.Lock()
	cp := &Checkpoint{PipelineID: qe.pipelineCtx.PipelineID, Stage: qe.pipelineCtx.Stage, Seq: seq, Trigger: "auto", Data: data, CreatedAt: time.Now()}
	qe.checkpointCache = append(qe.checkpointCache, cp)
	if len(qe.checkpointCache) > 3 {
		qe.checkpointCache = qe.checkpointCache[1:]
	}
	qe.mu.Unlock()

	go func() { _ = qe.checkpointRepo.Save(context.Background(), cp) }()
}

// Resume restores the engine and re-enters the loop.
func (qe *QueryEngine) Resume(ctx context.Context) (<-chan StreamEvent, error) {
	return qe.SubmitMessage(ctx, "")
}

// SnapshotMessages returns a deep copy of messages for checkpointing.
func (qe *QueryEngine) SnapshotMessages() []agentport.Message {
	qe.mu.Lock()
	defer qe.mu.Unlock()
	cp := make([]agentport.Message, len(qe.messages))
	copy(cp, qe.messages)
	return cp
}

// TokenUsed returns the total number of tokens consumed.
func (qe *QueryEngine) TokenUsed() int {
	return int(atomic.LoadInt64(&qe.tokenCount))
}

// Messages returns a copy of the full conversation history.
func (qe *QueryEngine) Messages() []agentport.Message {
	qe.mu.Lock()
	defer qe.mu.Unlock()
	out := make([]agentport.Message, len(qe.messages))
	copy(out, qe.messages)
	return out
}

// SetForceSkill sets the force skill name for the next prompt build.
func (qe *QueryEngine) SetForceSkill(name string) {
	qe.mu.Lock()
	defer qe.mu.Unlock()
	qe.forceSkill = name
}

// ForceSkill returns the current force skill name.
func (qe *QueryEngine) ForceSkill() string {
	qe.mu.Lock()
	defer qe.mu.Unlock()
	return qe.forceSkill
}

// ClearForceSkill clears the force skill after prompt build consumes it.
func (qe *QueryEngine) ClearForceSkill() {
	qe.mu.Lock()
	defer qe.mu.Unlock()
	qe.forceSkill = ""
}

// Clear resets the conversation history and token count.
func (qe *QueryEngine) Clear() {
	qe.mu.Lock()
	defer qe.mu.Unlock()
	qe.messages = nil
	qe.tokenCount = 0
	qe.state = QueryStateIdle
}
