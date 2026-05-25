package domain

import (
	"context"
	"fmt"
	"strings"

	agentport "openforge/internal/agent/port"
)

// CompressionConfig holds compression tuning parameters.
type CompressionConfig struct {
	ContextThreshold     float64 // 0.8 = 80%
	KeepRoundsNormal     int     // 10
	KeepRoundsAggressive int     // 5
	SummaryModel         string  // "haiku"
	MaxSummaryTokens     int
}

// DefaultCompressionConfig returns reasonable defaults.
func DefaultCompressionConfig() CompressionConfig {
	return CompressionConfig{
		ContextThreshold:     0.8,
		KeepRoundsNormal:     10,
		KeepRoundsAggressive: 5,
		SummaryModel:         "haiku",
		MaxSummaryTokens:     2000,
	}
}

// Compressor manages LLM-driven conversation compression.
type Compressor struct {
	llmClient agentport.LLMRouterClient
	config    CompressionConfig
}

// NewCompressor creates a new Compressor.
func NewCompressor(llmClient agentport.LLMRouterClient, config CompressionConfig) *Compressor {
	return &Compressor{llmClient: llmClient, config: config}
}

// NeedsCompression checks if messages exceed the context threshold.
func (c *Compressor) NeedsCompression(tokenUsed int64, contextWindow int) bool {
	threshold := float64(contextWindow) * c.config.ContextThreshold
	return float64(tokenUsed) > threshold
}

// Compress applies the compression algorithm. Returns compressed messages or error.
func (c *Compressor) Compress(ctx context.Context, messages []agentport.Message, tokenUsed int64, contextWindow int, aggressive bool) ([]agentport.Message, error) {
	keepRounds := c.config.KeepRoundsNormal
	if aggressive {
		keepRounds = c.config.KeepRoundsAggressive
	}

	keepCount := keepRounds * 2 // user + assistant per round (tool messages interleaved)
	if len(messages) <= keepCount {
		keepCount = len(messages) / 2
		if keepCount < 2 {
			keepCount = 2
		}
	}

	toCompress := messages[:len(messages)-keepCount]
	keepLast := messages[len(messages)-keepCount:]

	// Generate summary via Haiku
	summary, err := c.generateSummary(ctx, toCompress)
	if err != nil {
		// Degrade: text truncation
		summary = buildTextTruncationSummary(toCompress)
	}

	roundCount := (len(toCompress) + 1) / 2
	summaryMsg := agentport.Message{
		Role: "user",
		Content: fmt.Sprintf(`<conversation_summary compressed="true" rounds="%d" original_tokens="%d">
%s
</conversation_summary>`, roundCount, tokenUsed, summary),
	}

	// Prepend summary before keepLast
	result := append([]agentport.Message{summaryMsg}, keepLast...)
	return result, nil
}

// generateSummary calls Haiku to summarize messages.
func (c *Compressor) generateSummary(ctx context.Context, messages []agentport.Message) (string, error) {
	prompt := buildSummaryPrompt(messages)

	resp, err := c.llmClient.Chat(ctx, agentport.ChatRequest{
		Messages: []agentport.Message{{Role: "user", Content: prompt}},
		Config: agentport.LLMConfig{
			Model:     c.config.SummaryModel,
			MaxTokens: c.config.MaxSummaryTokens,
		},
	})
	if err != nil {
		return "", fmt.Errorf("summary generation failed: %w", err)
	}
	return resp.Content, nil
}

// buildSummaryPrompt constructs the compression prompt.
func buildSummaryPrompt(messages []agentport.Message) string {
	var b strings.Builder
	b.WriteString("Summarize the following conversation and tool interactions.\n\n")
	b.WriteString("Preserve:\n")
	b.WriteString("1. Key decisions and their rationale\n")
	b.WriteString("2. Error messages and their resolutions\n")
	b.WriteString("3. Code changes made (file paths, function names)\n")
	b.WriteString("4. Current task progress and next steps\n")
	b.WriteString("5. Any pending issues or blockers\n\n")
	b.WriteString("Format as structured bullet points.\n\n")
	b.WriteString("--- Conversation to summarize ---\n")

	for _, msg := range messages {
		content := msg.Content
		if len(content) > 300 {
			content = content[:300] + "..."
		}
		b.WriteString(fmt.Sprintf("[%s] %s\n", msg.Role, content))
	}
	return b.String()
}

// buildTextTruncationSummary is the degraded fallback when LLM is unavailable.
func buildTextTruncationSummary(messages []agentport.Message) string {
	if len(messages) == 0 {
		return ""
	}
	first := messages[0].Content
	if len(first) > 200 {
		first = first[:200] + "..."
	}
	last := messages[len(messages)-1].Content
	if len(last) > 200 {
		last = last[:200] + "..."
	}
	return fmt.Sprintf("(truncated) Earlier conversation: %d messages. First: %s ... Last: %s",
		len(messages), first, last)
}

// EstimateTokens gives a rough token count from messages for the aggressive check.
func EstimateTokens(messages []agentport.Message) int64 {
	total := 0
	for _, m := range messages {
		total += len(m.Content) / 4
	}
	return int64(total)
}
