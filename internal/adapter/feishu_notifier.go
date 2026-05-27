package adapter

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"openforge/internal/shared/kernel"
)

// FeishuNotifier implements kernel.Notifier for Feishu webhook notifications.
type FeishuNotifier struct {
	webhookURL string
	enabled    bool
	client     *http.Client
}

// NewFeishuNotifier creates a new Feishu notifier.
// If webhookURL is empty, the notifier is disabled.
func NewFeishuNotifier(webhookURL string) *FeishuNotifier {
	if webhookURL == "" {
		slog.Warn("feishu notifier disabled: empty webhook URL")
		return &FeishuNotifier{enabled: false}
	}

	slog.Info("feishu notifier enabled")
	return &FeishuNotifier{
		webhookURL: webhookURL,
		enabled:    true,
		client: &http.Client{
			Timeout: 3 * time.Second,
		},
	}
}

// feishuMessage represents a Feishu webhook message.
type feishuMessage struct {
	MsgType string `json:"msg_type"`
	Content any    `json:"content"`
}

// feishuCard represents a Feishu card message.
type feishuCard struct {
	Header   feishuCardHeader   `json:"header"`
	Elements []feishuCardElement `json:"elements"`
}

type feishuCardHeader struct {
	Title    feishuCardTitle `json:"title"`
	Template string          `json:"template"`
}

type feishuCardTitle struct {
	Content string `json:"content"`
	Tag     string `json:"tag"`
}

type feishuCardElement struct {
	Tag     string `json:"tag"`
	Content string `json:"content,omitempty"`
	Text    *feishuCardText `json:"text,omitempty"`
	Actions []feishuCardAction `json:"actions,omitempty"`
}

type feishuCardText struct {
	Content string `json:"content"`
	Tag     string `json:"tag"`
}

// Send sends a notification to Feishu webhook.
func (f *FeishuNotifier) Send(ctx context.Context, target kernel.Target, msg kernel.Notification) error {
	if !f.enabled {
		return fmt.Errorf("feishu notifier is disabled")
	}

	// Create card message
	card := feishuCard{
		Header: feishuCardHeader{
			Title: feishuCardTitle{
				Content: msg.Title,
				Tag:     "plain_text",
			},
			Template: getTemplateByLevel(msg.Level),
		},
		Elements: []feishuCardElement{
			{
				Tag: "markdown",
				Text: &feishuCardText{
					Content: msg.Body,
					Tag:     "plain_text",
				},
			},
		},
	}

	// Add action URL if provided
	if msg.ActionURL != "" {
		card.Elements = append(card.Elements, feishuCardElement{
			Tag: "action",
			Actions: []feishuCardAction{
				{
					Tag: "button",
					Text: feishuCardText{
						Content: "View Details",
						Tag:     "plain_text",
					},
					URL: msg.ActionURL,
					Type: "primary",
				},
			},
		})
	}

	feishuMsg := feishuMessage{
		MsgType: "interactive",
		Content: card,
	}

	return f.sendJSON(ctx, feishuMsg)
}

// SendWithRetry sends a notification with exponential backoff retry.
func (f *FeishuNotifier) SendWithRetry(ctx context.Context, target kernel.Target, msg kernel.Notification, maxRetries int) error {
	if !f.enabled {
		return fmt.Errorf("feishu notifier is disabled")
	}

	var lastErr error
	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			// Exponential backoff: 1s, 2s, 4s, etc.
			backoff := time.Duration(1<<uint(attempt-1)) * time.Second
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(backoff):
			}
		}

		err := f.Send(ctx, target, msg)
		if err == nil {
			return nil
		}
		lastErr = err
		slog.Warn("feishu notification failed, retrying", "attempt", attempt+1, "error", err)
	}

	return fmt.Errorf("feishu notification failed after %d attempts: %w", maxRetries+1, lastErr)
}

// sendJSON sends a JSON payload to the Feishu webhook.
func (f *FeishuNotifier) sendJSON(ctx context.Context, payload any) error {
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", f.webhookURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := f.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("feishu webhook returned status %d", resp.StatusCode)
	}

	return nil
}

// getTemplateByLevel returns a Feishu card template based on notification level.
func getTemplateByLevel(level string) string {
	switch level {
	case "error", "critical":
		return "red"
	case "warning":
		return "orange"
	case "success":
		return "green"
	default:
		return "blue"
	}
}

// feishuCardAction represents a button action in Feishu card.
type feishuCardAction struct {
	Tag    string         `json:"tag"`
	Text   feishuCardText `json:"text"`
	URL    string         `json:"url"`
	Type   string         `json:"type"`
}

// MultiChannelNotifier implements kernel.Notifier by broadcasting to multiple channels.
type MultiChannelNotifier struct {
	channels []kernel.Notifier
}

// NewMultiChannelNotifier creates a new multi-channel notifier.
func NewMultiChannelNotifier(channels []kernel.Notifier) *MultiChannelNotifier {
	if len(channels) == 0 {
		slog.Warn("multi-channel notifier disabled: no channels configured")
		return &MultiChannelNotifier{}
	}

	slog.Info("multi-channel notifier enabled", "channels", len(channels))
	return &MultiChannelNotifier{
		channels: channels,
	}
}

// Send broadcasts a notification to all channels.
func (m *MultiChannelNotifier) Send(ctx context.Context, target kernel.Target, msg kernel.Notification) error {
	if len(m.channels) == 0 {
		return fmt.Errorf("multi-channel notifier has no channels")
	}

	var lastErr error
	for _, channel := range m.channels {
		if err := channel.Send(ctx, target, msg); err != nil {
			slog.Error("multi-channel notification failed for channel", "error", err)
			lastErr = err
			// Continue with other channels
		}
	}

	return lastErr
}

// SendWithRetry broadcasts a notification to all channels with retry.
func (m *MultiChannelNotifier) SendWithRetry(ctx context.Context, target kernel.Target, msg kernel.Notification, maxRetries int) error {
	if len(m.channels) == 0 {
		return fmt.Errorf("multi-channel notifier has no channels")
	}

	var lastErr error
	for _, channel := range m.channels {
		if err := channel.SendWithRetry(ctx, target, msg, maxRetries); err != nil {
			slog.Error("multi-channel notification with retry failed for channel", "error", err)
			lastErr = err
			// Continue with other channels
		}
	}

	return lastErr
}

// Verify that both notifiers implement kernel.Notifier.
var _ kernel.Notifier = (*FeishuNotifier)(nil)
var _ kernel.Notifier = (*MultiChannelNotifier)(nil)