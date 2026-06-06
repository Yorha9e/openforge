package profile

import (
	"fmt"
	"testing"

	"openforge/internal/adapter"
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

func TestNewObjectStore_MinIOReturnsRealAdapter(t *testing.T) {
	cfg := &Config{
		ObjectStore: "minio",
		Minio: MinioConfig{
			Endpoint:        "localhost:9000",
			AccessKeyID:     "minio",
			SecretAccessKey: "minio123",
			Bucket:          "of-test",
			UseSSL:          false,
			Region:          "us-east-1",
		},
	}
	s := newObjectStore(cfg)
	_, ok := s.(*adapter.MinioObjectStore)
	if !ok {
		t.Fatalf("expected *adapter.MinioObjectStore, got %T", s)
	}
}

func TestNewObjectStore_DefaultNoop(t *testing.T) {
	s := newObjectStore(&Config{})
	if fmt.Sprintf("%T", s) != "*profile.noopObjectStore" {
		t.Fatalf("expected noopObjectStore, got %T", s)
	}
}
