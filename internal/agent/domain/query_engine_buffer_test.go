package domain

import (
	"context"
	"sync"
	"testing"
	"time"

	agentport "openforge/internal/agent/port"
	pipelineport "openforge/internal/pipeline/port"
)

// MockConversationRepository for testing
type MockConversationRepository struct {
	mu               sync.Mutex
	savedMessages    []*pipelineport.DBMessage
	batchSaveCalls   int
	batchSaveFunc    func(ctx context.Context, msgs []*pipelineport.DBMessage) error
}

func (m *MockConversationRepository) SaveMessage(ctx context.Context, msg *pipelineport.DBMessage) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.savedMessages = append(m.savedMessages, msg)
	return nil
}

func (m *MockConversationRepository) BatchSaveMessages(ctx context.Context, msgs []*pipelineport.DBMessage) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.batchSaveCalls++
	m.savedMessages = append(m.savedMessages, msgs...)
	if m.batchSaveFunc != nil {
		return m.batchSaveFunc(ctx, msgs)
	}
	return nil
}

func (m *MockConversationRepository) GetMessages(ctx context.Context, pipelineID string, branchID string) ([]*pipelineport.DBMessage, error) {
	return nil, nil
}

func (m *MockConversationRepository) CreateBranch(ctx context.Context, branch *pipelineport.DBBranch) error {
	return nil
}

func (m *MockConversationRepository) GetBranch(ctx context.Context, branchID string) (*pipelineport.DBBranch, error) {
	return nil, nil
}

func (m *MockConversationRepository) GetActiveBranch(ctx context.Context, pipelineID string) (*pipelineport.DBBranch, error) {
	return nil, nil
}

func (m *MockConversationRepository) ListBranches(ctx context.Context, pipelineID string) ([]*pipelineport.DBBranch, error) {
	return nil, nil
}

func TestQueryEngine_MessageBuffer(t *testing.T) {
	mockRepo := &MockConversationRepository{}
	
	qe := NewQueryEngine(nil, agentport.LLMConfig{}, nil, PipelineContext{
		PipelineID: "test-pipeline",
	})
	qe.SetConversationRepo(mockRepo)
	defer qe.StopFlushLoop()

	// Simulate multiple saveMessage calls
	for i := 0; i < 10; i++ {
		qe.saveMessage(i, "user", "text", "message")
	}

	// Wait for flush
	time.Sleep(6 * time.Second)

	// Verify messages were batched
	mockRepo.mu.Lock()
	if len(mockRepo.savedMessages) != 10 {
		t.Errorf("Expected 10 messages, got %d", len(mockRepo.savedMessages))
	}
	if mockRepo.batchSaveCalls != 1 {
		t.Errorf("Expected 1 batch save call, got %d", mockRepo.batchSaveCalls)
	}
	mockRepo.mu.Unlock()
}

func TestQueryEngine_BufferFlushOnFull(t *testing.T) {
	mockRepo := &MockConversationRepository{}
	
	qe := NewQueryEngine(nil, agentport.LLMConfig{}, nil, PipelineContext{
		PipelineID: "test-pipeline",
	})
	// Set small buffer for testing
	qe.messageBuffer = NewMessageBuffer(5)
	qe.SetConversationRepo(mockRepo)
	defer qe.StopFlushLoop()

	// Add more messages than buffer size
	for i := 0; i < 12; i++ {
		qe.saveMessage(i, "user", "text", "message")
	}

	// Wait a bit for async operations
	time.Sleep(100 * time.Millisecond)

	// Verify all messages were saved
	mockRepo.mu.Lock()
	if len(mockRepo.savedMessages) != 12 {
		t.Errorf("Expected 12 messages, got %d", len(mockRepo.savedMessages))
	}
	mockRepo.mu.Unlock()
}