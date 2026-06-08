package adapter

import (
	"context"
	"testing"
)

// TestPGRepository_SetBudget_NilDB 验证 db=nil 时 SetBudget 返回明确错误。
// 真实 cost_quota 表的 round-trip 由 T10 (Path A integration verification) 覆盖。
func TestPGRepository_SetBudget_NilDB(t *testing.T) {
	repo := &PGRepository{db: nil}
	err := repo.SetBudget(context.Background(), "pr-quota-1", 250.0)
	if err == nil {
		t.Error("expected error when db is nil, got nil")
	}
}

// TestPGRepository_SetBudget_NegativeAllowed 验证 SetBudget 接受 0 / 正数（覆盖清空语义）。
// 0 表示无限制（plan 文档约定），因此非负值都应通过 nil-DB 守卫后落入 SQL 阶段。
// 此用例仍属 nil-DB 路径；真实 SQL 验证留给 T10。
func TestPGRepository_SetBudget_NilReceiver(t *testing.T) {
	var repo *PGRepository
	if err := repo.SetBudget(context.Background(), "pr-quota-1", 0); err == nil {
		t.Error("expected error when receiver is nil, got nil")
	}
}

// TestPGRepository_GetBudget_NilDB 验证 db=nil 时 GetBudget 返回明确错误。
func TestPGRepository_GetBudget_NilDB(t *testing.T) {
	repo := &PGRepository{db: nil}
	got, err := repo.GetBudget(context.Background(), "pr-quota-1")
	if err == nil {
		t.Error("expected error when db is nil, got nil")
	}
	if got != 0 {
		t.Errorf("expected zero value on error, got %v", got)
	}
}

// TestPGRepository_GetBudget_NilReceiver 验证 nil receiver 时 GetBudget 返回错误。
func TestPGRepository_GetBudget_NilReceiver(t *testing.T) {
	var repo *PGRepository
	if _, err := repo.GetBudget(context.Background(), "pr-quota-1"); err == nil {
		t.Error("expected error when receiver is nil, got nil")
	}
}
