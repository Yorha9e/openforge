package adapter

import (
	"bytes"
	"context"
	"os"
	"testing"
)

func minioAvailable(t *testing.T) {
	t.Helper()
	if os.Getenv("MINIO_ENDPOINT") == "" {
		t.Skip("MINIO_ENDPOINT not set, skipping minio integration test")
	}
}

func TestMinioObjectStore_Disabled_EmptyEndpoint(t *testing.T) {
	store := NewMinioObjectStore(MinioConfig{})
	if store.IsEnabled() {
		t.Error("should be disabled when endpoint is empty")
	}

	ctx := context.Background()
	_, err := store.Get(ctx, "test")
	if err == nil {
		t.Error("Get should return error when disabled")
	}

	err = store.Put(ctx, "test", bytes.NewReader([]byte("data")))
	if err == nil {
		t.Error("Put should return error when disabled")
	}

	err = store.Delete(ctx, "test")
	if err == nil {
		t.Error("Delete should return error when disabled")
	}

	_, err = store.List(ctx, "test")
	if err == nil {
		t.Error("List should return error when disabled")
	}
}

func TestMinioObjectStore_PutGet_CRUD(t *testing.T) {
	minioAvailable(t)

	endpoint := os.Getenv("MINIO_ENDPOINT")
	accessKey := os.Getenv("MINIO_ACCESS_KEY")
	secretKey := os.Getenv("MINIO_SECRET_KEY")

	store := NewMinioObjectStore(MinioConfig{
		Endpoint:        endpoint,
		AccessKeyID:     accessKey,
		SecretAccessKey: secretKey,
		Bucket:          "test-openforge",
		UseSSL:          false,
	})
	if !store.IsEnabled() {
		t.Fatal("store should be enabled")
	}

	ctx := context.Background()
	key := "test-object-123"
	data := []byte("hello world")

	// Put
	err := store.Put(ctx, key, bytes.NewReader(data))
	if err != nil {
		t.Fatalf("Put failed: %v", err)
	}

	// Get
	reader, err := store.Get(ctx, key)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	defer reader.Close()

	buf := new(bytes.Buffer)
	buf.ReadFrom(reader)
	if !bytes.Equal(buf.Bytes(), data) {
		t.Errorf("Get returned wrong data: got %s, want %s", buf.String(), string(data))
	}

	// List
	keys, err := store.List(ctx, "test-object")
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	found := false
	for _, k := range keys {
		if k == key {
			found = true
			break
		}
	}
	if !found {
		t.Error("List did not return the put object")
	}

	// Delete
	err = store.Delete(ctx, key)
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	// Verify deleted
	_, err = store.Get(ctx, key)
	if err == nil {
		t.Error("Get should fail after Delete")
	}
}

func TestMinioObjectStore_Get_NotFound(t *testing.T) {
	minioAvailable(t)

	endpoint := os.Getenv("MINIO_ENDPOINT")
	accessKey := os.Getenv("MINIO_ACCESS_KEY")
	secretKey := os.Getenv("MINIO_SECRET_KEY")

	store := NewMinioObjectStore(MinioConfig{
		Endpoint:        endpoint,
		AccessKeyID:     accessKey,
		SecretAccessKey: secretKey,
		Bucket:          "test-openforge",
		UseSSL:          false,
	})
	if !store.IsEnabled() {
		t.Fatal("store should be enabled")
	}

	ctx := context.Background()
	_, err := store.Get(ctx, "nonexistent-key-12345")
	if err == nil {
		t.Error("expected error for nonexistent key")
	}
}
