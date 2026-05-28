package adapter

import (
	"context"
	"testing"

	"openforge/internal/pipeline/port"
)

func TestPGRepository_BatchSaveMessages(t *testing.T) {
	// This test requires a real database connection
	// Skip if no database available
	if testing.Short() {
		t.Skip("Skipping database test in short mode")
	}

	// Create test messages
	messages := []*port.DBMessage{
		{
			PipelineID: "test-pipeline-1",
			BranchID:   "main",
			MsgSeq:     1,
			Role:       "user",
			MsgType:    "text",
			Content:    "Hello",
		},
		{
			PipelineID: "test-pipeline-1",
			BranchID:   "main",
			MsgSeq:     2,
			Role:       "agent",
			MsgType:    "text",
			Content:    "Hi there!",
		},
		{
			PipelineID: "test-pipeline-1",
			BranchID:   "main",
			MsgSeq:     3,
			Role:       "user",
			MsgType:    "text",
			Content:    "How are you?",
		},
	}

	// Test batch save
	err := repo.BatchSaveMessages(context.Background(), messages)
	if err != nil {
		t.Fatalf("BatchSaveMessages failed: %v", err)
	}

	// Verify messages were saved
	saved, err := repo.GetMessages(context.Background(), "test-pipeline-1", "main")
	if err != nil {
		t.Fatalf("GetMessages failed: %v", err)
	}

	if len(saved) != 3 {
		t.Errorf("Expected 3 messages, got %d", len(saved))
	}
}

func TestPGRepository_BatchSaveMessages_Empty(t *testing.T) {
	// Test empty input
	err := repo.BatchSaveMessages(context.Background(), []*port.DBMessage{})
	if err != nil {
		t.Errorf("BatchSaveMessages with empty input should not error, got: %v", err)
	}
}
