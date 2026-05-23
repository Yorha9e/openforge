package domain

import (
	"context"
	"errors"
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

func (s *stubLLMClient) ChatStream(ctx context.Context, req port.ChatRequest) (<-chan string, error) {
	if s.err != nil {
		// Return nil channel so QueryEngine's nil-channel safety emits error event
		return nil, nil
	}
	ch := make(chan string, 1)
	ch <- s.response
	close(ch)
	return ch, nil
}

func TestQueryEngine_SubmitMessage(t *testing.T) {
	client := &stubLLMClient{response: "hello"}
	qe := NewQueryEngine(client, port.LLMConfig{MaxTokens: 4096})

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
	qe := NewQueryEngine(client, port.LLMConfig{MaxTokens: 4096})

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
	qe := NewQueryEngine(client, port.LLMConfig{MaxTokens: 4096})
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
