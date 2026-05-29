package domain

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unicode/utf8"

	agentport "openforge/internal/agent/port"
	pipelineport "openforge/internal/pipeline/port"
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
	convRepo        pipelineport.ConversationRepository
	activeBranchID  string
	mu              sync.Mutex
	
	// Message buffer for batch writes
	messageBuffer   *MessageBuffer
	flushTicker     *time.Ticker
	done            chan struct{}
	flushInterval   time.Duration
}

// NewQueryEngine creates a new QueryEngine with the given LLM client and config.
func NewQueryEngine(llmClient agentport.LLMRouterClient, config agentport.LLMConfig, promptBuilder *PromptBuilder, pipelineCtx PipelineContext) *QueryEngine {
	return &QueryEngine{
		llmClient:      llmClient,
		config:         config,
		state:          QueryStateIdle,
		promptBuilder:  promptBuilder,
		pipelineCtx:    pipelineCtx,
		toolRegistry:   make(ToolRegistry),
		activeBranchID: "main",
		// Initialize message buffer
		messageBuffer:  NewMessageBuffer(100),  // Buffer up to 100 messages
		done:           make(chan struct{}),
		flushInterval:  5 * time.Second,  // Flush every 5 seconds
	}
}

// SetToolRegistry sets the tool registry for multi-turn function calling.
func (qe *QueryEngine) SetToolRegistry(reg ToolRegistry) {
	qe.mu.Lock()
	defer qe.mu.Unlock()
	qe.toolRegistry = reg
}

func (qe *QueryEngine) buildToolDefs() []agentport.ToolDef {
	qe.mu.Lock()
	defer qe.mu.Unlock()
	defs := make([]agentport.ToolDef, 0, len(qe.toolRegistry))
	for _, meta := range qe.toolRegistry {
		defs = append(defs, agentport.ToolDef{
			Name:        meta.Name,
			Description: meta.Name + " tool",
			InputSchema: toolInputSchema(meta.Name),
		})
	}
	return defs
}

func toolInputSchema(name string) map[string]interface{} {
	base := map[string]interface{}{
		"type":                 "object",
		"additionalProperties": true,
	}

	switch name {
	case "bash":
		base["properties"] = map[string]interface{}{
			"command": map[string]interface{}{"type": "string"},
		}
		base["required"] = []string{"command"}
	case "read_file", "file_exists", "delete_file", "mkdir", "ls", "glob", "file_info":
		base["properties"] = map[string]interface{}{
			"path": map[string]interface{}{"type": "string"},
		}
		if name != "ls" && name != "glob" {
			base["required"] = []string{"path"}
		}
	case "write_file", "append_file":
		base["properties"] = map[string]interface{}{
			"path":    map[string]interface{}{"type": "string"},
			"content": map[string]interface{}{"type": "string"},
		}
		base["required"] = []string{"path", "content"}
	case "edit_file":
		base["properties"] = map[string]interface{}{
			"path":    map[string]interface{}{"type": "string"},
			"old_str": map[string]interface{}{"type": "string"},
			"new_str": map[string]interface{}{"type": "string"},
		}
		base["required"] = []string{"path", "old_str", "new_str"}
	case "copy_file", "move_file":
		base["properties"] = map[string]interface{}{
			"src": map[string]interface{}{"type": "string"},
			"dst": map[string]interface{}{"type": "string"},
		}
		base["required"] = []string{"src", "dst"}
	case "read_lines":
		base["properties"] = map[string]interface{}{
			"path":  map[string]interface{}{"type": "string"},
			"start": map[string]interface{}{"type": "number"},
			"end":   map[string]interface{}{"type": "number"},
		}
		base["required"] = []string{"path"}
	case "insert_at_line":
		base["properties"] = map[string]interface{}{
			"path":    map[string]interface{}{"type": "string"},
			"line":    map[string]interface{}{"type": "number"},
			"content": map[string]interface{}{"type": "string"},
		}
		base["required"] = []string{"path", "line", "content"}
	case "grep":
		base["properties"] = map[string]interface{}{
			"pattern": map[string]interface{}{"type": "string"},
			"path":    map[string]interface{}{"type": "string"},
		}
		base["required"] = []string{"pattern"}
	}

	return base
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

// startFlushLoop starts the background goroutine for periodic flushing.
func (qe *QueryEngine) startFlushLoop() {
	qe.flushTicker = time.NewTicker(qe.flushInterval)
	go qe.flushLoop()
}

// flushLoop runs in a background goroutine and flushes messages periodically.
func (qe *QueryEngine) flushLoop() {
	for {
		select {
		case <-qe.flushTicker.C:
			qe.flushMessages()
		case <-qe.done:
			// Final flush before shutdown
			qe.flushMessages()
			return
		}
	}
}

// flushMessages flushes all buffered messages to the database.
func (qe *QueryEngine) flushMessages() {
	if qe.convRepo == nil {
		return
	}

	messages := qe.messageBuffer.Flush()
	if len(messages) == 0 {
		return
	}

	// Batch save to database
	if err := qe.convRepo.BatchSaveMessages(context.Background(), messages); err != nil {
		// Log error but don't crash - messages are best-effort
		slog.Error("failed to batch save messages",
			"error", err,
			"count", len(messages),
			"pipeline_id", qe.pipelineCtx.PipelineID,
		)
	}
}

// StopFlushLoop stops the background flush goroutine and flushes remaining messages.
// Safe to call multiple times.
func (qe *QueryEngine) StopFlushLoop() {
	qe.mu.Lock()
	defer qe.mu.Unlock()
	
	// Prevent double-close panic
	if qe.done == nil {
		return
	}
	
	if qe.flushTicker != nil {
		qe.flushTicker.Stop()
	}
	close(qe.done)
	qe.done = nil // Mark as stopped
	
	// Final flush of remaining messages
	if qe.convRepo != nil {
		messages := qe.messageBuffer.Flush()
		if len(messages) > 0 {
			if err := qe.convRepo.BatchSaveMessages(context.Background(), messages); err != nil {
				slog.Error("failed to flush remaining messages on shutdown",
					"error", err,
					"count", len(messages),
					"pipeline_id", qe.pipelineCtx.PipelineID,
				)
			}
		}
	}
}

// SetConversationRepo sets the conversation repository for chat persistence.
func (qe *QueryEngine) SetConversationRepo(repo pipelineport.ConversationRepository) {
	qe.mu.Lock()
	defer qe.mu.Unlock()
	qe.convRepo = repo
	
	// Start flush loop when repository is set
	if repo != nil {
		qe.startFlushLoop()
	}
}

// saveMessage persists a message to the conversation store (best-effort, non-blocking).
// msgSeq must be computed by the caller while holding qe.mu to avoid data races.
func (qe *QueryEngine) saveMessage(msgSeq int, role, msgType, content string) {
	if qe.convRepo == nil {
		return
	}
	
	msg := &pipelineport.DBMessage{
		PipelineID: qe.pipelineCtx.PipelineID,
		BranchID:   qe.activeBranchID,
		MsgSeq:     msgSeq,
		Role:       role,
		MsgType:    msgType,
		Content:    content,
	}
	
	// Try to add to buffer
	if !qe.messageBuffer.Add(msg) {
		// Buffer full, flush immediately and retry
		qe.flushMessages()
		qe.messageBuffer.Add(msg)
	}
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
	userMsgSeq := len(qe.messages)
	qe.messages = append(qe.messages, agentport.Message{Role: "user", Content: msg})
	qe.tokenCount += int64(utf8.RuneCountInString(msg) / 4)
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
		Tools:        qe.buildToolDefs(),
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
		atomic.AddInt64(&qe.tokenCount, int64(utf8.RuneCountInString(resp.Content)/4))
	}

	// Persist user message now that LLM call succeeded
	qe.saveMessage(userMsgSeq, "user", "text", msg)

	// Single-turn: LLM done, no tool calls
	if resp.StopReason == "end_turn" || resp.StopReason == "stop_sequence" {
		qe.mu.Lock()
		msgSeq := len(qe.messages)
		qe.messages = append(qe.messages, agentport.Message{Role: "assistant", Content: resp.Content})
		qe.state = QueryStateAwaitingUser
		qe.mu.Unlock()
		qe.saveMessage(msgSeq, "agent", "text", resp.Content)
		return singleTurnOut(resp.Content), nil
	}

	toolCalls := parseToolCalls(resp.Content)
	if len(toolCalls) == 0 {
		qe.mu.Lock()
		msgSeq := len(qe.messages)
		qe.messages = append(qe.messages, agentport.Message{Role: "assistant", Content: resp.Content})
		qe.state = QueryStateAwaitingUser
		qe.mu.Unlock()
		qe.saveMessage(msgSeq, "agent", "text", resp.Content)
		return singleTurnOut(resp.Content), nil
	}

	// Multi-turn: tool_use detected, enter tool loop
	qe.mu.Lock()
	msgSeq := len(qe.messages)
	qe.messages = append(qe.messages, agentport.Message{Role: "assistant", Content: resp.Content})
	qe.mu.Unlock()
	qe.saveMessage(msgSeq, "agent", "text", resp.Content)

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

	out <- StreamEvent{Type: "delta", Content: extractTextOnly(firstResp.Content)}

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
			msgSeq := len(qe.messages)
			qe.messages = append(qe.messages, agentport.Message{Role: "tool", Content: formatToolResultXML(r)})
			qe.mu.Unlock()
			qe.saveMessage(msgSeq, "agent", "text", formatToolResultXML(r))
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
			Tools:        qe.buildToolDefs(),
		})
		if err != nil {
			out <- StreamEvent{Type: "error", Error: fmt.Errorf("LLM call failed: %w", err)}
			return
		}

		if resp.Usage != nil {
			atomic.AddInt64(&qe.tokenCount, resp.Usage.InputTokens+resp.Usage.OutputTokens)
		} else {
			atomic.AddInt64(&qe.tokenCount, int64(utf8.RuneCountInString(resp.Content)/4))
		}

		out <- StreamEvent{Type: "delta", Content: extractTextOnly(resp.Content)}

		if resp.StopReason == "end_turn" || resp.StopReason == "stop_sequence" {
			qe.mu.Lock()
			msgSeq := len(qe.messages)
			qe.messages = append(qe.messages, agentport.Message{Role: "assistant", Content: resp.Content})
			qe.state = QueryStateAwaitingUser
			qe.mu.Unlock()
			qe.saveMessage(msgSeq, "agent", "text", resp.Content)
			out <- StreamEvent{Type: "done", Content: extractTextOnly(resp.Content)}
			return
		}

		toolCalls = parseToolCalls(resp.Content)
		if len(toolCalls) == 0 {
			qe.mu.Lock()
			msgSeq := len(qe.messages)
			qe.messages = append(qe.messages, agentport.Message{Role: "assistant", Content: resp.Content})
			qe.state = QueryStateAwaitingUser
			qe.mu.Unlock()
			qe.saveMessage(msgSeq, "agent", "text", resp.Content)
			out <- StreamEvent{Type: "done", Content: extractTextOnly(resp.Content)}
			return
		}

		qe.mu.Lock()
		msgSeq := len(qe.messages)
		qe.messages = append(qe.messages, agentport.Message{Role: "assistant", Content: resp.Content})
		qe.mu.Unlock()
		qe.saveMessage(msgSeq, "agent", "text", resp.Content)
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
	system := prompt.System
	// Append tool descriptions so the LLM knows what tools are available
	if len(qe.toolRegistry) > 0 {
		system += "\n\n" + FormatToolsForPrompt(qe.toolRegistry)
	}
	return system
}

// parseToolCalls extracts tool calls from LLM content.
// Handles Anthropic tool_use blocks, including JSON snippets embedded in mixed text.
func parseToolCalls(content string) []ToolCallParsed {
	var calls []ToolCallParsed

	normalizeArgs := func(toolName string, args map[string]interface{}) map[string]interface{} {
		if rawInput, ok := args["input"]; ok && len(args) == 1 {
			switch v := rawInput.(type) {
			case map[string]interface{}:
				args = v
			case string:
				parsed := make(map[string]interface{})
				if json.Unmarshal([]byte(v), &parsed) == nil && len(parsed) > 0 {
					args = parsed
				} else if toolName == "bash" {
					args = map[string]interface{}{"command": v}
				}
			}
		}
		return args
	}

	decodeInput := func(raw json.RawMessage, toolName string) map[string]interface{} {
		args := make(map[string]interface{})
		if err := json.Unmarshal(raw, &args); err != nil {
			var inputStr string
			if err := json.Unmarshal(raw, &inputStr); err == nil {
				_ = json.Unmarshal([]byte(inputStr), &args)
			}
		}
		return normalizeArgs(toolName, args)
	}

	decodeBlocks := func(raw string) bool {
		startCount := len(calls)

		var blocks []struct {
			Type  string          `json:"type"`
			Name  string          `json:"name"`
			Input json.RawMessage `json:"input"`
		}
		if err := json.Unmarshal([]byte(raw), &blocks); err == nil {
			for _, b := range blocks {
				if b.Type != "tool_use" || b.Name == "" {
					continue
				}
				calls = append(calls, ToolCallParsed{
					ID:   fmt.Sprintf("toolu-%d", len(calls)),
					Name: b.Name,
					Args: decodeInput(b.Input, b.Name),
				})
			}
		}

		var single struct {
			Type  string          `json:"type"`
			Name  string          `json:"name"`
			Input json.RawMessage `json:"input"`
		}
		if err := json.Unmarshal([]byte(raw), &single); err == nil {
			if single.Type == "tool_use" && single.Name != "" {
				calls = append(calls, ToolCallParsed{
					ID:   fmt.Sprintf("toolu-%d", len(calls)),
					Name: single.Name,
					Args: decodeInput(single.Input, single.Name),
				})
			}
		}

		return len(calls) > startCount
	}

	if decodeBlocks(content) {
		return calls
	}

	fencedJSONRe := regexp.MustCompile("(?s)```(?:json)?\\s*([\\[{].*?[\\]}])\\s*```")
	for _, m := range fencedJSONRe.FindAllStringSubmatch(content, -1) {
		if len(m) > 1 && decodeBlocks(strings.TrimSpace(m[1])) {
			return calls
		}
	}

	if tuIdx := strings.Index(content, `"tool_use"`); tuIdx >= 0 {
		if arrStart := strings.LastIndex(content[:tuIdx], "["); arrStart >= 0 {
			if arrEndRel := strings.Index(content[tuIdx:], "]"); arrEndRel >= 0 {
				candidate := content[arrStart : tuIdx+arrEndRel+1]
				if decodeBlocks(candidate) {
					return calls
				}
			}
		}
		if objStart := strings.LastIndex(content[:tuIdx], "{"); objStart >= 0 {
			if objEndRel := strings.Index(content[tuIdx:], "}"); objEndRel >= 0 {
				candidate := content[objStart : tuIdx+objEndRel+1]
				if decodeBlocks(candidate) {
					return calls
				}
			}
		}
	}

	return calls
}

// extractTextOnly strips tool_use/tool_result blocks from Anthropic content JSON,
// returning only text portions. Also removes embedded tool JSON snippets in mixed output.
func extractTextOnly(content string) string {
	var blocks []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if err := json.Unmarshal([]byte(content), &blocks); err == nil {
		var texts []string
		for _, b := range blocks {
			if b.Type == "text" && b.Text != "" {
				texts = append(texts, b.Text)
			}
		}
		if len(texts) > 0 {
			return strings.Join(texts, "\n")
		}
	}

	fencedToolUseRe := regexp.MustCompile("(?s)```(?:json)?\\s*\\[.*?\"tool_use\".*?\\]\\s*```")
	cleaned := strings.TrimSpace(fencedToolUseRe.ReplaceAllString(content, ""))

	if tuIdx := strings.Index(cleaned, `"tool_use"`); tuIdx >= 0 {
		start := strings.LastIndex(cleaned[:tuIdx], "[")
		if start >= 0 {
			// Properly match closing bracket with depth counter (handles multi-tool arrays)
			depth := 0
			endIdx := -1
		bracketLoop:
			for i := start; i < len(cleaned); i++ {
				switch cleaned[i] {
				case '[':
					depth++
				case ']':
					depth--
					if depth == 0 {
						endIdx = i
						break bracketLoop
					}
				}
			}
			if endIdx >= 0 {
				cleaned = strings.TrimSpace(cleaned[:start] + cleaned[endIdx+1:])
			}
		}
	}

	if cleaned != "" {
		return cleaned
	}
	return content
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

// LoadMessages restores conversation history from persisted storage (e.g. after reconnect).
func (qe *QueryEngine) LoadMessages(msgs []agentport.Message) {
	qe.mu.Lock()
	defer qe.mu.Unlock()
	qe.messages = make([]agentport.Message, len(msgs))
	copy(qe.messages, msgs)
	tokens := int64(0)
	for _, m := range msgs {
		tokens += int64(utf8.RuneCountInString(m.Content) / 4)
	}
	qe.tokenCount = tokens
	qe.state = QueryStateAwaitingUser
}
