package domain

import (
	"container/heap"
	"fmt"
	"sync"
	"time"
)

// LLMRequest represents a queued LLM call request with WFQ priority (§4.6).
type LLMRequest struct {
	ID          string
	PipelineID  string
	Priority    int       // 0 (P0, highest) to 3 (P3, lowest)
	Payload     any
	SubmittedAt time.Time
}

// pqItem is an internal heap item for container/heap.
type pqItem struct {
	request *LLMRequest
	index   int // index in the heap (maintained by heap.Interface)
	seq     uint64
}

// priorityQueueHeap implements container/heap.Interface.
type priorityQueueHeap []*pqItem

func (h priorityQueueHeap) Len() int { return len(h) }

func (h priorityQueueHeap) Less(i, j int) bool {
	if h[i].request.Priority != h[j].request.Priority {
		// Lower priority number = higher urgency, so pop sooner.
		return h[i].request.Priority < h[j].request.Priority
	}
	// Same priority: FIFO — earlier seq pops first.
	return h[i].seq < h[j].seq
}

func (h priorityQueueHeap) Swap(i, j int) {
	h[i], h[j] = h[j], h[i]
	h[i].index = i
	h[j].index = j
}

func (h *priorityQueueHeap) Push(x any) {
	n := len(*h)
	item := x.(*pqItem)
	item.index = n
	*h = append(*h, item)
}

func (h *priorityQueueHeap) Pop() any {
	old := *h
	n := len(old)
	item := old[n-1]
	old[n-1] = nil // avoid memory leak
	item.index = -1
	*h = old[0 : n-1]
	return item
}

// LLMPriorityQueue is a weighted fair queuing priority queue for LLM requests.
// P0 (priority 0) is highest urgency; P3 (priority 3) is lowest.
// Within the same priority the queue is FIFO.
type LLMPriorityQueue struct {
	heap priorityQueueHeap
	seq  uint64
	mu   sync.Mutex
}

// NewLLMPriorityQueue creates a new LLMPriorityQueue.
func NewLLMPriorityQueue() *LLMPriorityQueue {
	h := make(priorityQueueHeap, 0)
	return &LLMPriorityQueue{heap: h}
}

// Enqueue adds a request to the queue with the given priority.
func (q *LLMPriorityQueue) Enqueue(pipelineID string, priority int, payload any) {
	q.mu.Lock()
	defer q.mu.Unlock()

	q.seq++
	item := &pqItem{
		request: &LLMRequest{
			ID:          fmt.Sprintf("llm-%d", q.seq),
			PipelineID:  pipelineID,
			Priority:    priority,
			Payload:     payload,
			SubmittedAt: time.Now(),
		},
		seq: q.seq,
	}
	heap.Push(&q.heap, item)
}

// Dequeue removes and returns the highest-priority request. Returns nil if empty.
func (q *LLMPriorityQueue) Dequeue() *LLMRequest {
	q.mu.Lock()
	defer q.mu.Unlock()

	if q.heap.Len() == 0 {
		return nil
	}
	return heap.Pop(&q.heap).(*pqItem).request
}

// Len returns the number of items currently in the queue.
func (q *LLMPriorityQueue) Len() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.heap.Len()
}
