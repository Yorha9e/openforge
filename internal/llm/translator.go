package llm

import (
	"encoding/json"
)

// OpenAIChatResponse is the OpenAI chat completion response shape.
type OpenAIChatResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
	} `json:"usage"`
}

// Translator converts between Anthropic Messages format and
// other provider formats (OpenAI, Google Gemini).
type Translator struct{}

func NewTranslator() *Translator { return &Translator{} }

// ToOpenAI converts an Anthropic-format ChatRequest to an OpenAI
// chat completion request body.
func (t *Translator) ToOpenAI(req ChatRequest) map[string]interface{} {
	messages := make([]map[string]string, 0, len(req.Messages)+1)
	if req.SystemPrompt != "" {
		messages = append(messages, map[string]string{
			"role": "system", "content": req.SystemPrompt,
		})
	}
	for _, m := range req.Messages {
		role := m.Role
		messages = append(messages, map[string]string{
			"role": role, "content": m.Content,
		})
	}
	return map[string]interface{}{
		"model":      req.Model,
		"messages":   messages,
		"max_tokens": req.MaxTokens,
	}
}

// FromOpenAI converts an OpenAI chat completion response to a ChatResponse.
func (t *Translator) FromOpenAI(resp OpenAIChatResponse) ChatResponse {
	content := ""
	stopReason := ""
	if len(resp.Choices) > 0 {
		content = resp.Choices[0].Message.Content
		switch resp.Choices[0].FinishReason {
		case "stop":
			stopReason = "end_turn"
		case "length":
			stopReason = "max_tokens"
		default:
			stopReason = resp.Choices[0].FinishReason
		}
	}
	return ChatResponse{
		Content:    content,
		StopReason: stopReason,
		Usage: Usage{
			PromptTokens:     resp.Usage.PromptTokens,
			CompletionTokens: resp.Usage.CompletionTokens,
		},
	}
}

// ToGemini converts to Google Gemini GenerateContentRequest format.
func (t *Translator) ToGemini(req ChatRequest) map[string]interface{} {
	contents := make([]map[string]interface{}, 0, len(req.Messages))
	for _, m := range req.Messages {
		role := "user"
		if m.Role == "assistant" {
			role = "model"
		}
		contents = append(contents, map[string]interface{}{
			"role":  role,
			"parts": []map[string]string{{"text": m.Content}},
		})
	}
	payload := map[string]interface{}{
		"contents": contents,
	}
	if req.SystemPrompt != "" {
		payload["system_instruction"] = map[string]interface{}{
			"parts": []map[string]string{{"text": req.SystemPrompt}},
		}
	}
	return payload
}

// FromGemini converts a Gemini response to ChatResponse.
func (t *Translator) FromGemini(body []byte) (ChatResponse, error) {
	var resp struct {
		Candidates []struct {
			Content struct {
				Parts []struct{ Text string }
				Role  string
			}
			FinishReason string `json:"finishReason"`
		}
		UsageMetadata struct {
			PromptTokenCount     int `json:"promptTokenCount"`
			CandidatesTokenCount int `json:"candidatesTokenCount"`
		} `json:"usageMetadata"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return ChatResponse{}, err
	}
	text := ""
	if len(resp.Candidates) > 0 && len(resp.Candidates[0].Content.Parts) > 0 {
		text = resp.Candidates[0].Content.Parts[0].Text
	}
	return ChatResponse{
		Content:    text,
		StopReason: resp.Candidates[0].FinishReason,
		Usage: Usage{
			PromptTokens:     resp.UsageMetadata.PromptTokenCount,
			CompletionTokens: resp.UsageMetadata.CandidatesTokenCount,
		},
	}, nil
}
