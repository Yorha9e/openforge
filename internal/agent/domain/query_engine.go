package domain

import (
	"context"
	"fmt"
	"strings"
	"sync"

	agentport "openforge/internal/agent/port"
)

// QueryState represents the state of the query engine.
type QueryState string

const (
	QueryStateIdle         QueryState = "IDLE"
	QueryStateAwaitingUser QueryState = "AWAITING_USER"
)

// StreamEvent represents a single event emitted during a streaming LLM response.
type StreamEvent struct {
	Type    string // "delta", "done", or "error"
	Content string
	Error   error
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

// QueryEngine manages a conversational interaction with an LLM.
// It holds message history, a reference to the LLM router client, and the
// configuration used for each request.
type QueryEngine struct {
	llmClient     agentport.LLMRouterClient
	config        agentport.LLMConfig
	messages      []agentport.Message
	tokenCount    int
	state         QueryState
	promptBuilder *PromptBuilder
	pipelineCtx   PipelineContext
	forceSkill    string
	mu            sync.Mutex
}

// NewQueryEngine creates a new QueryEngine with the given LLM client and config.
// promptBuilder and pipelineCtx may be zero-valued; system prompt injection is skipped in that case.
func NewQueryEngine(llmClient agentport.LLMRouterClient, config agentport.LLMConfig, promptBuilder *PromptBuilder, pipelineCtx PipelineContext) *QueryEngine {
	return &QueryEngine{
		llmClient:    llmClient,
		config:       config,
		state:        QueryStateIdle,
		promptBuilder: promptBuilder,
		pipelineCtx:  pipelineCtx,
	}
}

// State returns the current query engine state.
func (qe *QueryEngine) State() QueryState {
	qe.mu.Lock()
	defer qe.mu.Unlock()
	return qe.state
}

// SubmitMessage appends a user message to the conversation history and starts a
// streaming LLM call. It returns a channel of StreamEvent values.
//
// The channel emits zero or more "delta" events (one per chunk), followed by a
// single "done" event carrying the full assistant response. If the streaming
// call itself fails immediately, SubmitMessage returns the error and the user
// message is rolled back from history.
//
// Callers must read the channel until it is closed.
func (qe *QueryEngine) SubmitMessage(ctx context.Context, msg string) (<-chan StreamEvent, error) {
	// Parse /skill <name> command
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
	qe.tokenCount += len(msg) / 4
	history := make([]agentport.Message, len(qe.messages))
	copy(history, qe.messages)
	qe.mu.Unlock()

	// Build system prompt via PromptBuilder (L1→L2→Capability→L4)
	var systemPrompt string
	if qe.promptBuilder != nil {
		buildReq := &BuildRequest{
			PipelineID:         qe.pipelineCtx.PipelineID,
			ProjectID:          qe.pipelineCtx.ProjectID,
			Stage:              qe.pipelineCtx.Stage,
			StageLevel:         qe.pipelineCtx.StageLevel,
			PermissionMode:     qe.pipelineCtx.PermissionMode,
			UserRole:           qe.pipelineCtx.UserRole,
			UserMessage:        msg,
			ConversationHistory: history,
		}
		if prompt, err := qe.promptBuilder.Build(context.Background(), buildReq); err == nil {
			systemPrompt = prompt.System
		}
	}

	// Limit messages to recent rounds to avoid context overflow.
	// The full conversation context is available in the L4 summary.
	const maxMessages = 40 // ~20 rounds
	trimmedHistory := history
	if len(trimmedHistory) > maxMessages {
		trimmedHistory = trimmedHistory[len(trimmedHistory)-maxMessages:]
	}

	req := agentport.ChatRequest{
		Messages:     trimmedHistory,
		SystemPrompt: systemPrompt,
		Config:       qe.config,
	}

	stream, err := qe.llmClient.ChatStream(ctx, req)
	if err != nil {
		// Roll back the user message on initialisation failure.
		qe.mu.Lock()
		qe.messages = qe.messages[:len(qe.messages)-1]
		qe.mu.Unlock()
		return nil, err
	}

	out := make(chan StreamEvent)
	go func() {
		defer close(out)

		// Safety: a nil channel would block range forever.
		if stream == nil {
			out <- StreamEvent{Type: "error", Error: errStreamNil}
			return
		}

		var full strings.Builder
		for chunk := range stream {
			full.WriteString(chunk)
			out <- StreamEvent{Type: "delta", Content: chunk}
		}

		responseText := full.String()

		qe.mu.Lock()
		qe.messages = append(qe.messages, agentport.Message{Role: "assistant", Content: responseText})
		qe.mu.Unlock()

		qe.mu.Lock()
		qe.state = QueryStateAwaitingUser
		qe.mu.Unlock()

		out <- StreamEvent{Type: "done", Content: responseText}
	}()

	return out, nil
}

// errStreamNil is returned as an error event when the LLM client returns a nil
// channel without signalling an error.
var errStreamNil = &streamNilError{}

type streamNilError struct{}

func (e *streamNilError) Error() string { return "llm client returned nil stream channel" }

// TokenUsed returns the total number of tokens consumed across all messages
// submitted through this engine. Accurate counting depends on the underlying
// LLM client reporting usage data.
func (qe *QueryEngine) TokenUsed() int {
	qe.mu.Lock()
	defer qe.mu.Unlock()
	return qe.tokenCount
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

// ForceSkill returns the current force skill name (empty if none).
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
