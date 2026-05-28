package domain

import (
	"sync"
	"testing"

	pipelineport "openforge/internal/pipeline/port"
)

func TestMessageBuffer_Add(t *testing.T) {
	buffer := NewMessageBuffer(3)

	msg1 := &pipelineport.DBMessage{PipelineID: "p1", BranchID: "b1", MsgSeq: 1, Role: "user", Content: "hello"}
	msg2 := &pipelineport.DBMessage{PipelineID: "p1", BranchID: "b1", MsgSeq: 2, Role: "agent", Content: "hi"}
	msg3 := &pipelineport.DBMessage{PipelineID: "p1", BranchID: "b1", MsgSeq: 3, Role: "user", Content: "test"}

	if !buffer.Add(msg1) {
		t.Error("Expected Add to return true for first message")
	}
	if !buffer.Add(msg2) {
		t.Error("Expected Add to return true for second message")
	}
	if !buffer.Add(msg3) {
		t.Error("Expected Add to return true for third message")
	}

	if buffer.Size() != 3 {
		t.Errorf("Expected buffer size 3, got %d", buffer.Size())
	}
}

func TestMessageBuffer_Flush(t *testing.T) {
	buffer := NewMessageBuffer(3)

	msg1 := &pipelineport.DBMessage{PipelineID: "p1", BranchID: "b1", MsgSeq: 1, Role: "user", Content: "hello"}
	msg2 := &pipelineport.DBMessage{PipelineID: "p1", BranchID: "b1", MsgSeq: 2, Role: "agent", Content: "hi"}

	buffer.Add(msg1)
	buffer.Add(msg2)

	messages := buffer.Flush()
	if len(messages) != 2 {
		t.Errorf("Expected 2 messages after flush, got %d", len(messages))
	}

	if buffer.Size() != 0 {
		t.Errorf("Expected buffer size 0 after flush, got %d", buffer.Size())
	}
}

func TestMessageBuffer_ThreadSafety(t *testing.T) {
	buffer := NewMessageBuffer(100)
	var wg sync.WaitGroup

	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(seq int) {
			defer wg.Done()
			msg := &pipelineport.DBMessage{
				PipelineID: "p1",
				BranchID:   "b1",
				MsgSeq:     seq,
				Role:       "user",
				Content:    "test",
			}
			buffer.Add(msg)
		}(i)
	}

	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = buffer.Size()
			_ = buffer.Flush()
		}()
	}

	wg.Wait()
}
