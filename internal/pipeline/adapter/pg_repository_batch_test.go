package adapter

import (
	"context"
	"testing"

	"openforge/internal/pipeline/port"
)

func TestPGRepository_BatchSaveMessages_Empty(t *testing.T) {
	// Test empty input with nil repository (should not panic)
	var repo *PGRepository
	
	// This should not panic even with nil receiver
	err := repo.BatchSaveMessages(context.Background(), []*port.DBMessage{})
	if err != nil {
		t.Errorf("BatchSaveMessages with empty input should not error, got: %v", err)
	}
}

func TestPGRepository_BatchSaveMessages_NilDB(t *testing.T) {
	// Test with nil database connection
	repo := &PGRepository{db: nil}
	
	messages := []*port.DBMessage{
		{
			PipelineID: "test-pipeline-1",
			BranchID:   "main",
			MsgSeq:     1,
			Role:       "user",
			MsgType:    "text",
			Content:    "Hello",
		},
	}
	
	// This should return an error when db is nil
	err := repo.BatchSaveMessages(context.Background(), messages)
	if err == nil {
		t.Error("Expected error when db is nil")
	}
}
