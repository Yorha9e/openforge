package llm

// DeepSeekProvider reuses AnthropicProvider since DeepSeek API is
// Anthropic Messages API compatible (base URL differs).
type DeepSeekProvider struct {
	*AnthropicProvider
}

func NewDeepSeekProvider(baseURL, apiKey string) *DeepSeekProvider {
	return &DeepSeekProvider{AnthropicProvider: NewAnthropicProvider(baseURL, apiKey)}
}
