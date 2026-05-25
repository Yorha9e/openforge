package adapter

import (
	"context"
	"testing"
	"time"

	"openforge/internal/shared/kernel"
)

func TestRedisTaskQueue_EnqueueDequeue(t *testing.T) {
	q := NewRedisTaskQueue("", "")
	ctx := context.Background()

	msg := kernel.Message{ID: "test-1", Payload: []byte("hello")}
	if err := q.Enqueue(ctx, "t", msg, 1); err != nil {
		t.Fatalf("Enqueue failed: %v", err)
	}

	got, err := q.Dequeue(ctx, "t")
	if err != nil {
		t.Fatalf("Dequeue failed: %v", err)
	}
	if got.ID != "test-1" {
		t.Errorf("got ID = %q, want %q", got.ID, "test-1")
	}
	if string(got.Payload) != "hello" {
		t.Errorf("got Payload = %q, want %q", string(got.Payload), "hello")
	}
}

func TestRedisTaskQueue_DequeueBlocking_ContextCancel(t *testing.T) {
	q := NewRedisTaskQueue("", "")
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, err := q.Dequeue(ctx, "empty-topic")
	if err == nil {
		t.Fatal("expected context deadline error, got nil")
	}
}

func TestRedisTaskQueue_Ack(t *testing.T) {
	q := NewRedisTaskQueue("", "")
	if err := q.Ack(context.Background(), "t", "id-1"); err != nil {
		t.Fatalf("Ack failed: %v", err)
	}
}

func TestRedisTaskQueue_MultipleTopics(t *testing.T) {
	q := NewRedisTaskQueue("", "")
	ctx := context.Background()

	if err := q.Enqueue(ctx, "topic-a", kernel.Message{ID: "a1"}, 1); err != nil {
		t.Fatal(err)
	}
	if err := q.Enqueue(ctx, "topic-b", kernel.Message{ID: "b1"}, 1); err != nil {
		t.Fatal(err)
	}

	m1, _ := q.Dequeue(ctx, "topic-a")
	m2, _ := q.Dequeue(ctx, "topic-b")

	if m1.ID != "a1" || m2.ID != "b1" {
		t.Errorf("got %q and %q, want a1 and b1", m1.ID, m2.ID)
	}
}

func TestRedisTaskQueue_PriorityPropagation(t *testing.T) {
	q := NewRedisTaskQueue("", "")
	ctx := context.Background()

	msg := kernel.Message{ID: "p1"}
	if err := q.Enqueue(ctx, "prio", msg, 42); err != nil {
		t.Fatal(err)
	}

	got, err := q.Dequeue(ctx, "prio")
	if err != nil {
		t.Fatal(err)
	}
	if got.Priority != 42 {
		t.Errorf("got Priority = %d, want 42", got.Priority)
	}
}

func TestRedisTaskQueue_FIFO_Order(t *testing.T) {
	q := NewRedisTaskQueue("", "")
	ctx := context.Background()

	for i := 0; i < 5; i++ {
		id := rune('a' + i)
		if err := q.Enqueue(ctx, "fifo", kernel.Message{ID: string(id)}, 0); err != nil {
			t.Fatal(err)
		}
	}

	for i := 0; i < 5; i++ {
		got, err := q.Dequeue(ctx, "fifo")
		if err != nil {
			t.Fatal(err)
		}
		want := string(rune('a' + i))
		if got.ID != want {
			t.Errorf("position %d: got %q, want %q", i, got.ID, want)
		}
	}
}

func TestRedisTaskQueue_Enqueue_FullChannel(t *testing.T) {
	t.Run("returns error on context cancel", func(t *testing.T) {
		q := NewRedisTaskQueue("", "")
		// Fill the channel (capacity = queueCap = 100)
		ctx := context.Background()
		for i := 0; i < queueCap; i++ {
			if err := q.Enqueue(ctx, "fill", kernel.Message{ID: "x"}, 0); err != nil {
				t.Fatalf("unexpected error on item %d: %v", i, err)
			}
		}

		// Next Enqueue should block; use a cancelled context to trigger error
		cancelled, cancel := context.WithCancel(ctx)
		cancel()
		if err := q.Enqueue(cancelled, "fill", kernel.Message{ID: "overflow"}, 0); err == nil {
			t.Fatal("expected error on full channel with cancelled context")
		}
	})
}
