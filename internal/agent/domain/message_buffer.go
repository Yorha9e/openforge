package domain

import (
	"sync"

	pipelineport "openforge/internal/pipeline/port"
)

// MessageBuffer provides thread-safe buffering for database messages.
// Messages are buffered in memory and flushed in batches to reduce database load.
type MessageBuffer struct {
	mu       sync.Mutex
	messages []*pipelineport.DBMessage
	maxSize  int
}

// NewMessageBuffer creates a new MessageBuffer with the specified maximum size.
func NewMessageBuffer(maxSize int) *MessageBuffer {
	if maxSize <= 0 {
		maxSize = 1 // Prevent panic from negative capacity
	}
	return &MessageBuffer{
		messages: make([]*pipelineport.DBMessage, 0, maxSize),
		maxSize:  maxSize,
	}
}

// Add adds a message to the buffer. Returns false if buffer is full or msg is nil.
func (mb *MessageBuffer) Add(msg *pipelineport.DBMessage) bool {
	if msg == nil {
		return false
	}

	mb.mu.Lock()
	defer mb.mu.Unlock()

	if len(mb.messages) >= mb.maxSize {
		return false
	}

	mb.messages = append(mb.messages, msg)
	return true
}

// Flush returns all messages in the buffer and resets it.
func (mb *MessageBuffer) Flush() []*pipelineport.DBMessage {
	mb.mu.Lock()
	defer mb.mu.Unlock()

	if len(mb.messages) == 0 {
		return nil
	}

	messages := mb.messages
	mb.messages = make([]*pipelineport.DBMessage, 0, mb.maxSize)
	return messages
}

// Size returns the current number of messages in the buffer.
func (mb *MessageBuffer) Size() int {
	mb.mu.Lock()
	defer mb.mu.Unlock()

	return len(mb.messages)
}
