package domain

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	agentport "openforge/internal/agent/port"
	pipelineport "openforge/internal/pipeline/port"
)

func TestIntegration_FileTreeBrowsing(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Track database operations
	var mu sync.Mutex
	totalInserts := 0
	batchInserts := 0

	mockRepo := &MockConversationRepository{
		batchSaveFunc: func(ctx context.Context, msgs []*pipelineport.DBMessage) error {
			mu.Lock()
			defer mu.Unlock()
			batchInserts++
			totalInserts += len(msgs)
			return nil
		},
	}

	qe := NewQueryEngine(nil, agentport.LLMConfig{}, nil, PipelineContext{
		PipelineID: "test-pipeline",
	})
	qe.SetConversationRepo(mockRepo)
	defer qe.StopFlushLoop()

	// Simulate browsing file tree - 50 files expanded
	// Each file expansion triggers multiple saveMessage calls
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(fileIndex int) {
			defer wg.Done()
			
			// Simulate tool call and result
			qe.saveMessage(fileIndex*3, "user", "text", fmt.Sprintf("list_dir /path/to/folder/%d", fileIndex))
			qe.saveMessage(fileIndex*3+1, "agent", "tool_use", fmt.Sprintf("tool_call_%d", fileIndex))
			qe.saveMessage(fileIndex*3+2, "agent", "tool_result", fmt.Sprintf("file1.txt\nfile2.txt\nfile3.txt"))
		}(i)
	}

	wg.Wait()

	// Wait for final flush
	time.Sleep(6 * time.Second)

	mu.Lock()
	t.Logf("Total messages saved: %d", totalInserts)
	t.Logf("Batch insert operations: %d", batchInserts)
	t.Logf("Average batch size: %.1f", float64(totalInserts)/float64(batchInserts))
	
	// Verify batching reduced database operations
	if batchInserts >= totalInserts {
		t.Error("Expected batching to reduce database operations")
	}
	
	// With 150 messages and buffer size 100, we should have ~2 batch operations
	if batchInserts > 5 {
		t.Errorf("Expected fewer than 5 batch operations, got %d", batchInserts)
	}
	mu.Unlock()
}

func TestIntegration_ConcurrentAccess(t *testing.T) {
	mockRepo := &MockConversationRepository{}
	
	qe := NewQueryEngine(nil, agentport.LLMConfig{}, nil, PipelineContext{
		PipelineID: "test-pipeline",
	})
	qe.SetConversationRepo(mockRepo)
	defer qe.StopFlushLoop()

	// Simulate concurrent message saves
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(seq int) {
			defer wg.Done()
			qe.saveMessage(seq, "user", "text", fmt.Sprintf("message %d", seq))
		}(i)
	}

	wg.Wait()

	// Wait for flush
	time.Sleep(6 * time.Second)

	mockRepo.mu.Lock()
	if len(mockRepo.savedMessages) != 100 {
		t.Errorf("Expected 100 messages, got %d", len(mockRepo.savedMessages))
	}
	mockRepo.mu.Unlock()
}