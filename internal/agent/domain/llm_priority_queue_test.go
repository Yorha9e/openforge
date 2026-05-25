package domain

import (
	"testing"
)

func requireReq(t *testing.T, req *LLMRequest) {
	t.Helper()
	if req == nil {
		t.Fatal("Dequeue returned nil, expected non-nil")
	}
}

func TestLLMPriorityQueue_EnqueueDequeue_Empty(t *testing.T) {
	q := NewLLMPriorityQueue()
	if q.Len() != 0 {
		t.Fatalf("expected Len=0, got %d", q.Len())
	}
	got := q.Dequeue()
	if got != nil {
		t.Fatal("expected nil from empty queue")
	}
}

func TestLLMPriorityQueue_PriorityOrdering(t *testing.T) {
	q := NewLLMPriorityQueue()

	q.Enqueue("p1", 3, "P3-lowest")
	q.Enqueue("p2", 0, "P0-highest")
	q.Enqueue("p3", 1, "P1-medium")

	// Expected order: P0, P1, P3
	r1 := q.Dequeue()
	requireReq(t, r1)
	if r1.Payload != "P0-highest" {
		t.Errorf("expected P0 first, got %v", r1.Payload)
	}

	r2 := q.Dequeue()
	requireReq(t, r2)
	if r2.Payload != "P1-medium" {
		t.Errorf("expected P1 second, got %v", r2.Payload)
	}

	r3 := q.Dequeue()
	requireReq(t, r3)
	if r3.Payload != "P3-lowest" {
		t.Errorf("expected P3 third, got %v", r3.Payload)
	}

	if q.Len() != 0 {
		t.Errorf("expected queue empty, got Len=%d", q.Len())
	}
}

func TestLLMPriorityQueue_SamePriorityFIFO(t *testing.T) {
	q := NewLLMPriorityQueue()

	q.Enqueue("p1", 1, "first")
	q.Enqueue("p2", 1, "second")
	q.Enqueue("p3", 1, "third")

	r1 := q.Dequeue()
	requireReq(t, r1)
	if r1.Payload != "first" {
		t.Errorf("expected 'first', got %v", r1.Payload)
	}

	r2 := q.Dequeue()
	requireReq(t, r2)
	if r2.Payload != "second" {
		t.Errorf("expected 'second', got %v", r2.Payload)
	}

	r3 := q.Dequeue()
	requireReq(t, r3)
	if r3.Payload != "third" {
		t.Errorf("expected 'third', got %v", r3.Payload)
	}
}

func TestLLMPriorityQueue_P0BeforeP3(t *testing.T) {
	q := NewLLMPriorityQueue()

	// Enqueue several P3 items, then a P0 item.
	q.Enqueue("p1", 3, "P3-a")
	q.Enqueue("p2", 3, "P3-b")
	q.Enqueue("p3", 0, "P0-urgent")
	q.Enqueue("p4", 3, "P3-c")

	// P0 must come out before any remaining P3.
	r1 := q.Dequeue()
	requireReq(t, r1)
	if r1.Payload != "P0-urgent" {
		t.Errorf("expected P0 before P3, got %v", r1.Payload)
	}

	// Remaining P3 should be FIFO.
	r2 := q.Dequeue()
	requireReq(t, r2)
	if r2.Payload != "P3-a" {
		t.Errorf("expected P3-a, got %v", r2.Payload)
	}

	r3 := q.Dequeue()
	requireReq(t, r3)
	if r3.Payload != "P3-b" {
		t.Errorf("expected P3-b, got %v", r3.Payload)
	}

	r4 := q.Dequeue()
	requireReq(t, r4)
	if r4.Payload != "P3-c" {
		t.Errorf("expected P3-c, got %v", r4.Payload)
	}
}
