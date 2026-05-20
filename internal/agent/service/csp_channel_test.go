package service

import (
	"context"
	"testing"
	"time"
)

func TestCSPChannelSendReceive(t *testing.T) {
	ch := NewCSPChannel("test", 16)
	ctx := context.Background()

	msg := Message{From: "agent-A", To: "agent-B", Body: []byte("hello")}
	if err := ch.Send(ctx, msg); err != nil {
		t.Fatalf("Send() error = %v", err)
	}

	select {
	case received := <-ch.Receive():
		if string(received.Body) != "hello" {
			t.Errorf("Body = %q, want %q", received.Body, "hello")
		}
	case <-time.After(1 * time.Second):
		t.Fatal("timeout waiting for message")
	}
}

func TestCSPChannelBackpressure(t *testing.T) {
	ch := NewCSPChannel("small", 2)
	ctx := context.Background()

	for i := 0; i < 2; i++ {
		if err := ch.Send(ctx, Message{Body: []byte("msg")}); err != nil {
			t.Fatalf("Send %d error = %v", i, err)
		}
	}
	err := ch.Send(ctx, Message{Body: []byte("overflow")})
	if err == nil {
		t.Error("expected backpressure error on full channel")
	}
}
