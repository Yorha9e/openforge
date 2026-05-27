package profile

import (
	"fmt"
	"testing"
)

func TestNewTaskQueue_RedisStreams(t *testing.T) {
	cfg := &Config{TaskQueue: "redis-streams"}
	q := newTaskQueue(cfg)
	if fmt.Sprintf("%T", q) != "*adapter.RedisTaskQueue" {
		t.Fatalf("expected *adapter.RedisTaskQueue, got %T", q)
	}
}

func TestNewTaskQueue_DefaultNoop(t *testing.T) {
	q := newTaskQueue(&Config{})
	if fmt.Sprintf("%T", q) != "*profile.noopTaskQueue" {
		t.Fatalf("expected noopTaskQueue, got %T", q)
	}
}
