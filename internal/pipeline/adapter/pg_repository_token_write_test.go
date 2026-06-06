package adapter

import (
	"context"
	"testing"
	"time"

	"openforge/internal/pipeline/port"
)

// TestPGRepository_RecordTokenUsage_NilReceiver 验证 nil repo / nil db 路径的
// 错误返回。该用例不需要真实 DB，与同包内 pg_repository_batch_test.go 风格一致。
func TestPGRepository_RecordTokenUsage_NilReceiver(t *testing.T) {
	var repo *PGRepository
	err := repo.RecordTokenUsage(context.Background(), port.TokenUsageRecord{
		PipelineID: "p-1", ProjectID: "pr-1",
		Provider: "anthropic", Model: "claude-sonnet-4-6",
		PromptTokens: 100, CompletionTokens: 200, EstimatedCost: 0.003,
		CreatedAt: time.Now().UTC(),
	})
	if err == nil {
		t.Error("expected error when receiver is nil, got nil")
	}
}

// TestPGRepository_RecordTokenUsage_NilDB 验证 db=nil 时返回明确错误。
func TestPGRepository_RecordTokenUsage_NilDB(t *testing.T) {
	repo := &PGRepository{db: nil}
	err := repo.RecordTokenUsage(context.Background(), port.TokenUsageRecord{
		PipelineID: "p-1", ProjectID: "pr-1",
		Provider: "anthropic", Model: "claude-sonnet-4-6",
		PromptTokens: 100, CompletionTokens: 200, EstimatedCost: 0.003,
		CreatedAt: time.Now().UTC(),
	})
	if err == nil {
		t.Error("expected error when db is nil, got nil")
	}
}

// TestPGRepository_BatchRecordTokenUsage_Empty 空切片是 no-op。
func TestPGRepository_BatchRecordTokenUsage_Empty(t *testing.T) {
	repo := &PGRepository{db: nil}
	if err := repo.BatchRecordTokenUsage(context.Background(), nil); err != nil {
		t.Errorf("BatchRecordTokenUsage(nil) should be no-op, got: %v", err)
	}
	if err := repo.BatchRecordTokenUsage(context.Background(), []port.TokenUsageRecord{}); err != nil {
		t.Errorf("BatchRecordTokenUsage([]) should be no-op, got: %v", err)
	}
}

// TestPGRepository_BatchRecordTokenUsage_NilDB 非空切片 + nil db 应返回错误。
func TestPGRepository_BatchRecordTokenUsage_NilDB(t *testing.T) {
	repo := &PGRepository{db: nil}
	err := repo.BatchRecordTokenUsage(context.Background(), []port.TokenUsageRecord{
		{PipelineID: "p-1", ProjectID: "pr-1", Provider: "anthropic",
			Model: "claude-sonnet-4-6", PromptTokens: 10, CompletionTokens: 20,
			EstimatedCost: 0.0001, CreatedAt: time.Now().UTC()},
	})
	if err == nil {
		t.Error("expected error when db is nil, got nil")
	}
}

// TestTokenUsageRecordStruct 校验 TokenUsageRecord 字段类型。
// 这是一个静默断言：编译时类型不匹配会直接报红，运行时则校验 CreatedAt 可正常往返。
func TestTokenUsageRecordStruct(t *testing.T) {
	now := time.Now().UTC()
	rec := port.TokenUsageRecord{
		ID:               "abc-123",
		PipelineID:       "p-1",
		ProjectID:        "pr-1",
		Provider:         "anthropic",
		Model:            "claude-sonnet-4-6",
		PromptTokens:     100,
		CompletionTokens: 200,
		EstimatedCost:    0.003,
		CreatedAt:        now,
	}
	if rec.ID != "abc-123" {
		t.Errorf("ID field not preserved: got %q", rec.ID)
	}
	if !rec.CreatedAt.Equal(now) {
		t.Errorf("CreatedAt not preserved: got %v want %v", rec.CreatedAt, now)
	}
}
