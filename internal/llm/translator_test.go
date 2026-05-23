package llm

import (
	"encoding/json"
	"testing"
)

func TestTranslateToOpenAI(t *testing.T) {
	req := ChatRequest{
		Model:        "gpt-4o",
		SystemPrompt: "You are helpful.",
		Messages: []Message{
			{Role: "user", Content: "Hello"},
			{Role: "assistant", Content: "Hi there"},
		},
		MaxTokens: 4096,
	}

	tr := NewTranslator()
	openaiBody := tr.ToOpenAI(req)

	messages := openaiBody["messages"].([]map[string]string)
	if len(messages) != 3 {
		t.Fatalf("expected 3 messages (system + 2), got %d", len(messages))
	}
	if messages[0]["role"] != "system" {
		t.Errorf("first message role = %q, want system", messages[0]["role"])
	}
	if messages[0]["content"] != "You are helpful." {
		t.Errorf("system content = %q", messages[0]["content"])
	}
}

func TestTranslateOpenAIResponse(t *testing.T) {
	// Use JSON to construct the properly-tagged anonymous struct
	raw := `{
		"choices": [{"message": {"content": "Hello!"}, "finish_reason": "stop"}],
		"usage": {"prompt_tokens": 10, "completion_tokens": 5}
	}`
	var openaiResp OpenAIChatResponse
	if err := json.Unmarshal([]byte(raw), &openaiResp); err != nil {
		t.Fatal(err)
	}

	tr := NewTranslator()
	resp := tr.FromOpenAI(openaiResp)

	if resp.Content != "Hello!" {
		t.Errorf("content = %q, want Hello!", resp.Content)
	}
	if resp.StopReason != "end_turn" {
		t.Errorf("stop_reason = %q, want end_turn", resp.StopReason)
	}
	if resp.Usage.PromptTokens != 10 {
		t.Errorf("prompt_tokens = %d, want 10", resp.Usage.PromptTokens)
	}
}
