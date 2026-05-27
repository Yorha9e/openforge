package adapter

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"openforge/internal/shared/kernel"
)

func TestFeishuNotifier_Send_Success(t *testing.T) {
	received := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		if body["msg_type"] != nil {
			received = true
		}
		w.WriteHeader(200)
		w.Write([]byte(`{"code":0,"msg":"ok"}`))
	}))
	defer srv.Close()

	n := NewFeishuNotifier(srv.URL)
	if !n.enabled {
		t.Fatal("should be enabled")
	}

	ctx := context.Background()
	target := kernel.Target{Webhook: srv.URL}
	msg := kernel.Notification{
		Level: "info",
		Title: "Test",
		Body:  "Test message",
	}

	err := n.Send(ctx, target, msg)
	if err != nil {
		t.Fatalf("send failed: %v", err)
	}
	if !received {
		t.Error("webhook not called")
	}
}

func TestFeishuNotifier_Send_RetryOnFailure(t *testing.T) {
	attempts := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts < 3 {
			w.WriteHeader(500)
			return
		}
		w.WriteHeader(200)
		w.Write([]byte(`{"code":0}`))
	}))
	defer srv.Close()

	n := NewFeishuNotifier(srv.URL)
	if !n.enabled {
		t.Fatal("should be enabled")
	}

	ctx := context.Background()
	target := kernel.Target{Webhook: srv.URL}
	msg := kernel.Notification{
		Level: "error",
		Title: "Retry Test",
		Body:  "This should retry",
	}

	err := n.SendWithRetry(ctx, target, msg, 3)
	if err != nil {
		t.Fatalf("retry failed: %v", err)
	}
	if attempts != 3 {
		t.Errorf("expected 3 attempts, got %d", attempts)
	}
}

func TestFeishuNotifier_Disabled_EmptyWebhook(t *testing.T) {
	n := NewFeishuNotifier("")
	if n.enabled {
		t.Error("should be disabled when webhook is empty")
	}

	ctx := context.Background()
	target := kernel.Target{}
	msg := kernel.Notification{Level: "info", Title: "Test", Body: "Test"}

	err := n.Send(ctx, target, msg)
	if err == nil {
		t.Error("Send should return error when disabled")
	}
}

func TestMultiChannelNotifier_Send_Success(t *testing.T) {
	channel1Called := false
	channel2Called := false

	channel1 := &mockNotifier{
		sendFunc: func(ctx context.Context, target kernel.Target, msg kernel.Notification) error {
			channel1Called = true
			return nil
		},
	}

	channel2 := &mockNotifier{
		sendFunc: func(ctx context.Context, target kernel.Target, msg kernel.Notification) error {
			channel2Called = true
			return nil
		},
	}

	multi := NewMultiChannelNotifier([]kernel.Notifier{channel1, channel2})
	if len(multi.channels) != 2 {
		t.Fatal("should have 2 channels")
	}

	ctx := context.Background()
	target := kernel.Target{}
	msg := kernel.Notification{Level: "info", Title: "Test", Body: "Test"}

	err := multi.Send(ctx, target, msg)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	if !channel1Called {
		t.Error("channel1 not called")
	}
	if !channel2Called {
		t.Error("channel2 not called")
	}
}

func TestMultiChannelNotifier_Send_PartialFailure(t *testing.T) {
	channel1 := &mockNotifier{
		sendFunc: func(ctx context.Context, target kernel.Target, msg kernel.Notification) error {
			return nil
		},
	}

	channel2 := &mockNotifier{
		sendFunc: func(ctx context.Context, target kernel.Target, msg kernel.Notification) error {
			return fmt.Errorf("channel2 failed")
		},
	}

	multi := NewMultiChannelNotifier([]kernel.Notifier{channel1, channel2})

	ctx := context.Background()
	target := kernel.Target{}
	msg := kernel.Notification{Level: "info", Title: "Test", Body: "Test"}

	err := multi.Send(ctx, target, msg)
	if err == nil {
		t.Error("should return error when any channel fails")
	}
}

func TestMultiChannelNotifier_Disabled_NoChannels(t *testing.T) {
	multi := NewMultiChannelNotifier([]kernel.Notifier{})
	if len(multi.channels) != 0 {
		t.Error("should have no channels")
	}

	ctx := context.Background()
	target := kernel.Target{}
	msg := kernel.Notification{Level: "info", Title: "Test", Body: "Test"}

	err := multi.Send(ctx, target, msg)
	if err == nil {
		t.Error("Send should return error when no channels")
	}
}

// mockNotifier is a mock implementation of kernel.Notifier for testing.
type mockNotifier struct {
	sendFunc        func(ctx context.Context, target kernel.Target, msg kernel.Notification) error
	sendWithRetryFunc func(ctx context.Context, target kernel.Target, msg kernel.Notification, maxRetries int) error
}

func (m *mockNotifier) Send(ctx context.Context, target kernel.Target, msg kernel.Notification) error {
	if m.sendFunc != nil {
		return m.sendFunc(ctx, target, msg)
	}
	return nil
}

func (m *mockNotifier) SendWithRetry(ctx context.Context, target kernel.Target, msg kernel.Notification, maxRetries int) error {
	if m.sendWithRetryFunc != nil {
		return m.sendWithRetryFunc(ctx, target, msg, maxRetries)
	}
	return m.Send(ctx, target, msg)
}