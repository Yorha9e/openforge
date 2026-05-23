package service

import (
	"context"
	"fmt"
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
	// Third send goes to WAL (not error)
	err := ch.Send(ctx, Message{Body: []byte("wal-msg")})
	if err != nil {
		t.Errorf("expected WAL spill, got error: %v", err)
	}
	if ch.WALLen() != 1 {
		t.Errorf("WAL len = %d, want 1", ch.WALLen())
	}
}

func TestCSPChannel_WALBackpressure(t *testing.T) {
	ch := NewCSPChannel("test", 1)

	if err := ch.Send(context.Background(), Message{From: "a", To: "b", Body: []byte("1")}); err != nil {
		t.Fatalf("first send: %v", err)
	}

	for i := 2; i <= 5; i++ {
		body := []byte(fmt.Sprintf("%d", i))
		if err := ch.Send(context.Background(), Message{From: "a", To: "b", Body: body}); err != nil {
			t.Fatalf("wal send %d: %v", i, err)
		}
	}

	if ch.WALLen() != 4 {
		t.Fatalf("WAL len = %d, want 4", ch.WALLen())
	}

	<-ch.Receive()

	go ch.Drain(context.Background())

	count := 0
	for count < 4 {
		select {
		case <-ch.Receive():
			count++
		case <-time.After(1 * time.Second):
			t.Fatalf("expected 4 messages in channel after drain, got %d", count)
		}
	}

	if ch.WALLen() != 0 {
		t.Fatalf("WAL should be empty after drain, got %d", ch.WALLen())
	}
}

func TestCSPChannel_WALOverflow(t *testing.T) {
	ch := NewCSPChannel("test", 1)
	ch.Send(context.Background(), Message{From: "a", To: "b", Body: []byte("1")})
	for i := 0; i < maxWALSize; i++ {
		ch.Send(context.Background(), Message{From: "a", To: "b", Body: []byte(fmt.Sprintf("w%d", i))})
	}
	err := ch.Send(context.Background(), Message{From: "a", To: "b", Body: []byte("overflow")})
	if err == nil {
		t.Fatal("expected WAL overflow error, got nil")
	}
}
