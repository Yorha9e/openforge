package domain

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"openforge/internal/agent/port"
)

type stubLLMClient struct {
	response string
	err      error
}

func (s *stubLLMClient) Chat(ctx context.Context, req port.ChatRequest) (*port.ChatResponse, error) {
	return &port.ChatResponse{Content: s.response}, s.err
}

func (s *stubLLMClient) ChatStream(ctx context.Context, req port.ChatRequest) (<-chan port.StreamChunk, error) {
	if s.err != nil {
		// Return nil channel so QueryEngine's nil-channel safety emits error event
		return nil, nil
	}
	ch := make(chan port.StreamChunk, 1)
	ch <- port.StreamChunk{Delta: s.response, FinishReason: "end_turn"}
	close(ch)
	return ch, nil
}

// capturingLLMClient captures the ChatRequest for assertion.
type capturingLLMClient struct {
	lastSystemPrompt string
	lastMessages     []port.Message
	response         string
}

func (c *capturingLLMClient) Chat(ctx context.Context, req port.ChatRequest) (*port.ChatResponse, error) {
	c.lastSystemPrompt = req.SystemPrompt
	c.lastMessages = req.Messages
	return &port.ChatResponse{Content: c.response}, nil
}

func (c *capturingLLMClient) ChatStream(ctx context.Context, req port.ChatRequest) (<-chan port.StreamChunk, error) {
	c.lastSystemPrompt = req.SystemPrompt
	c.lastMessages = req.Messages
	ch := make(chan port.StreamChunk, 1)
	ch <- port.StreamChunk{Delta: c.response, FinishReason: "end_turn"}
	close(ch)
	return ch, nil
}

func TestPromptAssembly_FullChain(t *testing.T) {
	// 1. Create a PromptBuilder with real static.xml
	pb, err := NewPromptBuilder("../../../config/prompts/static.xml", nil)
	if err != nil {
		t.Fatalf("NewPromptBuilder: %v", err)
	}

	// 2. Create SkillLoader with seed skills
	sl, err := NewSkillLoader([]string{"../../../config/skills/global"})
	if err != nil {
		t.Fatalf("NewSkillLoader: %v", err)
	}
	defer sl.Stop()

	// 3. Wire CapabilityInjector
	ci := NewCapabilityInjector(sl, &HardcodedToolRegistry{})
	pb.SetCapabilityInjector(ci)

	// 4. Create QueryEngine with capturing client
	client := &capturingLLMClient{response: "I'll help you add a login endpoint."}
	qe := NewQueryEngine(client, port.LLMConfig{
		Model: "deepseek", MaxTokens: 4096,
	}, pb, PipelineContext{
		PipelineID:     "pipe-test-1",
		ProjectID:      "proj-A",
		Stage:          "impl",
		StageLevel:     "L2",
		PermissionMode: "auto",
	})

	// 5. Submit a message
	_, err = qe.SubmitMessage(context.Background(), "add a login endpoint for the backend")
	if err != nil {
		t.Fatalf("SubmitMessage: %v", err)
	}

	sys := client.lastSystemPrompt

	// 6. Verify all 4 layers are present
	t.Run("L1_static_xml", func(t *testing.T) {
		if !containsStr(sys, "OpenForge") {
			t.Error("missing L1 identity: OpenForge")
		}
		if !containsStr(sys, "Never bypass the Gate") {
			t.Error("missing L1 security: Gate rule")
		}
		if !containsStr(sys, "NO COMMENTS unless asked") {
			t.Error("missing L1 code convention")
		}
	})

	t.Run("L2_project_stage", func(t *testing.T) {
		if !containsStr(sys, "pipeline_id") {
			t.Error("missing L2 metadata: pipeline_id")
		}
		if !containsStr(sys, "stage_instructions") && !containsStr(sys, "Execute according") {
			t.Error("missing L2 stage instruction")
		}
	})

	t.Run("Capability_skills", func(t *testing.T) {
		if !containsStr(sys, "conduit-backend") {
			t.Error("missing Capability layer: conduit-backend skill")
		}
	})

	t.Run("Capability_tools", func(t *testing.T) {
		if !containsStr(sys, "read_file") && !containsStr(sys, "read_file") {
			t.Error("missing Capability tools: read_file")
		}
		if !containsStr(sys, "bash") {
			t.Error("missing Capability tools: bash")
		}
	})

	t.Run("L4_conversation", func(t *testing.T) {
		if !containsStr(sys, "conversation_summary") {
			t.Error("missing L4 conversation summary")
			t.Logf("SystemPrompt length: %d chars\n\n%s", len(sys), sys)
		}
	})

	t.Run("prompt_not_empty", func(t *testing.T) {
		if len(sys) == 0 {
			t.Fatal("SystemPrompt is empty — LLM would receive no context!")
		}
	})

	t.Run("messages_count", func(t *testing.T) {
		if len(client.lastMessages) != 1 {
			t.Errorf("expected 1 user message, got %d", len(client.lastMessages))
		}
	})
}

func TestPromptAssembly_MultiTurnConversation(t *testing.T) {
	pb, err := NewPromptBuilder("../../../config/prompts/static.xml", nil)
	if err != nil {
		t.Fatalf("NewPromptBuilder: %v", err)
	}

	client := &capturingLLMClient{response: "ok"}
	qe := NewQueryEngine(client, port.LLMConfig{
		Model: "deepseek", MaxTokens: 4096,
	}, pb, PipelineContext{
		PipelineID:     "pipe-test-1",
		ProjectID:      "proj-A",
		Stage:          "impl",
		StageLevel:     "L2",
		PermissionMode: "auto",
	})

	// Turn 1: user asks a question
	ch1, err := qe.SubmitMessage(context.Background(), "add a login endpoint")
	if err != nil {
		t.Fatalf("turn 1: %v", err)
	}
	// Drain the response
	for range ch1 {
	}

	// Verify turn 1: 1 user message + 1 assistant reply = 2 messages total
	t.Logf("After turn 1: %d messages, SystemPrompt=%d chars", len(client.lastMessages), len(client.lastSystemPrompt))

	// Turn 2: user asks a follow-up
	ch2, err := qe.SubmitMessage(context.Background(), "also add rate limiting")
	if err != nil {
		t.Fatalf("turn 2: %v", err)
	}
	for range ch2 {
	}

	sys := client.lastSystemPrompt

	t.Run("turn2_has_3_messages", func(t *testing.T) {
		// user1, assistant1, user2 = 3 messages sent to LLM
		if len(client.lastMessages) != 3 {
			t.Errorf("expected 3 messages (user+assistant+user), got %d", len(client.lastMessages))
			for i, m := range client.lastMessages {
				t.Logf("  msg[%d]: role=%s content=%q", i, m.Role, truncateStr(m.Content, 60))
			}
		}
	})

	t.Run("turn2_messages_contain_history", func(t *testing.T) {
		if len(client.lastMessages) < 2 {
			t.Fatal("too few messages")
		}
		if client.lastMessages[0].Role != "user" || client.lastMessages[0].Content != "add a login endpoint" {
			t.Errorf("msg[0] should be first user message, got role=%s content=%q", client.lastMessages[0].Role, client.lastMessages[0].Content)
		}
		if client.lastMessages[1].Role != "assistant" {
			t.Errorf("msg[1] should be assistant reply, got role=%s", client.lastMessages[1].Role)
		}
		if client.lastMessages[2].Role != "user" || client.lastMessages[2].Content != "also add rate limiting" {
			t.Errorf("msg[2] should be second user message, got role=%s content=%q", client.lastMessages[2].Role, client.lastMessages[2].Content)
		}
	})

	t.Run("turn2_L4_summary_has_history", func(t *testing.T) {
		if !containsStr(sys, "add a login endpoint") {
			t.Error("L4 conversation_summary missing first user message")
		}
		if !containsStr(sys, "conversation_summary") {
			t.Error("L4 conversation_summary tag missing")
		}
	})
}

func TestPromptAssembly_LongConversationCompression(t *testing.T) {
	pb, err := NewPromptBuilder("../../../config/prompts/static.xml", nil)
	if err != nil {
		t.Fatalf("NewPromptBuilder: %v", err)
	}

	client := &capturingLLMClient{response: "ok"}
	qe := NewQueryEngine(client, port.LLMConfig{
		Model: "deepseek", MaxTokens: 4096,
	}, pb, PipelineContext{
		PipelineID:     "pipe-test-1",
		ProjectID:      "proj-A",
		Stage:          "impl",
		StageLevel:     "L2",
		PermissionMode: "auto",
	})

	// Simulate 15 rounds (30 messages) — exceeds 10 round keep window
	for i := 0; i < 15; i++ {
		ch, err := qe.SubmitMessage(context.Background(), fmt.Sprintf("turn %d request", i))
		if err != nil {
			t.Fatalf("round %d: %v", i, err)
		}
		for range ch {
		}
	}

	sys := client.lastSystemPrompt

	t.Run("compressed_section_present", func(t *testing.T) {
		if !containsStr(sys, "<compressed") {
			t.Error("missing <compressed> block for older rounds")
		}
		if !containsStr(sys, "rounds=") {
			t.Error("missing rounds count in compressed block")
		}
	})

	t.Run("recent_rounds_kept", func(t *testing.T) {
		// The last few messages should still be verbatim in L4
		if !containsStr(sys, "turn 14 request") {
			t.Error("missing recent message verbatim in L4 summary")
			t.Logf("SystemPrompt last 500 chars:\n%s", sys[len(sys)-500:])
		}
	})

	t.Run("messages_trimmed", func(t *testing.T) {
		// 15 rounds × 2 = 30 messages, but max 40 → should have all 30
		// Actually 15×2+1=31 (last submission adds 1 user msg, assistant not yet stored)
		if len(client.lastMessages) < 15 {
			t.Errorf("expected at least 15 messages in context, got %d", len(client.lastMessages))
		}
	})
}

func TestPromptAssembly_EmptyHistory(t *testing.T) {
	pb, err := NewPromptBuilder("../../../config/prompts/static.xml", nil)
	if err != nil {
		t.Fatalf("NewPromptBuilder: %v", err)
	}

	client := &capturingLLMClient{response: "ok"}
	qe := NewQueryEngine(client, port.LLMConfig{
		Model: "deepseek", MaxTokens: 4096,
	}, pb, PipelineContext{
		PipelineID:     "pipe-test-1",
		ProjectID:      "proj-A",
		Stage:          "clarify",
		StageLevel:     "L3",
		PermissionMode: "plan",
	})

	// Very first message — no prior history
	ch, err := qe.SubmitMessage(context.Background(), "analyze the project structure")
	if err != nil {
		t.Fatalf("SubmitMessage: %v", err)
	}
	for range ch {
	}

	sys := client.lastSystemPrompt

	t.Run("first_msg_L4_has_one_msg", func(t *testing.T) {
		if !containsStr(sys, "analyze the project structure") {
			t.Error("L4 conversation_summary missing the only user message")
			t.Logf("SystemPrompt:\n%s", sys)
		}
	})

	t.Run("first_msg_sent_to_llm", func(t *testing.T) {
		if len(client.lastMessages) != 1 {
			t.Errorf("expected 1 message, got %d", len(client.lastMessages))
		}
		if client.lastMessages[0].Content != "analyze the project structure" {
			t.Errorf("wrong content: %q", client.lastMessages[0].Content)
		}
	})
}

func truncateStr(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

func containsStr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestQueryEngine_SubmitMessage(t *testing.T) {
	client := &stubLLMClient{response: "hello"}
	qe := NewQueryEngine(client, port.LLMConfig{MaxTokens: 4096}, nil, PipelineContext{})

	if qe.State() != QueryStateIdle {
		t.Fatalf("initial state = %s, want IDLE", qe.State())
	}

	ch, err := qe.SubmitMessage(context.Background(), "hi")
	if err != nil {
		t.Fatalf("SubmitMessage: %v", err)
	}

	var gotDone bool
	for ev := range ch {
		if ev.Type == "done" {
			gotDone = true
			if ev.Content != "hello" {
				t.Errorf("content = %q, want %q", ev.Content, "hello")
			}
		}
	}
	if !gotDone {
		t.Error("never received 'done' event")
	}

	if qe.State() != QueryStateAwaitingUser {
		t.Errorf("final state = %s, want AWAITING_USER", qe.State())
	}

	msgs := qe.Messages()
	if len(msgs) != 2 {
		t.Fatalf("message count = %d, want 2", len(msgs))
	}
	if msgs[0].Role != "user" || msgs[0].Content != "hi" {
		t.Errorf("message[0] = {role:%s content:%s}", msgs[0].Role, msgs[0].Content)
	}
	if msgs[1].Role != "assistant" || msgs[1].Content != "hello" {
		t.Errorf("message[1] = {role:%s content:%s}", msgs[1].Role, msgs[1].Content)
	}
}

func TestQueryEngine_ErrorTransitionsToErrorState(t *testing.T) {
	client := &stubLLMClient{err: errors.New("api down")}
	qe := NewQueryEngine(client, port.LLMConfig{MaxTokens: 4096}, nil, PipelineContext{})

	ch, err := qe.SubmitMessage(context.Background(), "hi")
	if err != nil {
		t.Fatalf("SubmitMessage should not error on submit: %v", err)
	}
	for ev := range ch {
		if ev.Type == "error" {
			return
		}
	}
	t.Error("expected error event, got none")
}

func TestQueryEngine_Clear(t *testing.T) {
	client := &stubLLMClient{response: "ok"}
	qe := NewQueryEngine(client, port.LLMConfig{MaxTokens: 4096}, nil, PipelineContext{})
	// Use a context that can be waited on; SubmitMessage runs the stub synchronously
	ch, err := qe.SubmitMessage(context.Background(), "hi")
	if err != nil {
		t.Fatalf("SubmitMessage: %v", err)
	}
	// drain the channel to ensure goroutine finishes
	for range ch {
	}

	qe.Clear()
	if qe.State() != QueryStateIdle {
		t.Errorf("after clear state = %s, want IDLE", qe.State())
	}
	if len(qe.Messages()) != 0 {
		t.Errorf("after clear messages = %d, want 0", len(qe.Messages()))
	}
}
