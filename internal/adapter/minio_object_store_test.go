package adapter

import (
	"bytes"
	"context"
	"os"
	"strings"
	"testing"

	"openforge/internal/shared/kernel"
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

// newTestMinio returns an enabled MinioObjectStore backed by MINIO_ENDPOINT
// (or skips when unset). The returned cleanup closes nothing — the
// MinioObjectStore has no open handles beyond the underlying *minio.Client.
func newTestMinio(t *testing.T) (*MinioObjectStore, func()) {
	t.Helper()
	minioAvailable(t)

	store := NewMinioObjectStore(MinioConfig{
		Endpoint:        os.Getenv("MINIO_ENDPOINT"),
		AccessKeyID:     os.Getenv("MINIO_ACCESS_KEY"),
		SecretAccessKey: os.Getenv("MINIO_SECRET_KEY"),
		Bucket:          "test-openforge",
		UseSSL:          false,
	})
	if !store.IsEnabled() {
		t.Fatal("store should be enabled when MINIO_ENDPOINT is set")
	}
	return store, func() {}
}

func TestMinioObjectStore_SetBucketObjectLock_AppliesGovernance(t *testing.T) {
	if testing.Short() {
		t.Skip("requires minio")
	}
	store, cleanup := newTestMinio(t)
	defer cleanup()

	ctx := context.Background()
	err := store.SetBucketObjectLock(ctx, kernel.ObjectLockConfig{
		Mode: kernel.ObjectLockModeGovernance,
		Days: 365,
	})
	if err != nil {
		t.Fatalf("SetBucketObjectLock failed: %v", err)
	}

	got, err := store.GetBucketObjectLock(ctx)
	if err != nil {
		t.Fatalf("GetBucketObjectLock failed: %v", err)
	}
	if got.Mode != kernel.ObjectLockModeGovernance {
		t.Errorf("Mode = %q, want GOVERNANCE", got.Mode)
	}
	if got.Days != 365 {
		t.Errorf("Days = %d, want 365", got.Days)
	}
}

func TestMinioObjectStore_SetObjectRetention_PreventsDelete(t *testing.T) {
	if testing.Short() {
		t.Skip("requires minio")
	}
	store, cleanup := newTestMinio(t)
	defer cleanup()

	ctx := context.Background()
	// Enable bucket-level object lock first; the bucket must already
	// have object lock support enabled for per-object retention to apply.
	if err := store.SetBucketObjectLock(ctx, kernel.ObjectLockConfig{
		Mode: kernel.ObjectLockModeGovernance,
		Days: 30,
	}); err != nil {
		t.Fatalf("SetBucketObjectLock failed: %v", err)
	}

	key := "audit/log-test.json"
	if err := store.Put(ctx, key, strings.NewReader(`{"a":1}`)); err != nil {
		t.Fatalf("Put failed: %v", err)
	}
	defer func() {
		// Best-effort cleanup; if retention is still active the delete
		// will fail and the next test run will see the existing object.
		_ = store.Delete(ctx, key)
	}()

	if err := store.SetObjectRetention(ctx, key, kernel.RetentionConfig{
		Mode: kernel.ObjectLockModeGovernance,
		Days: 30,
	}); err != nil {
		t.Fatalf("SetObjectRetention failed: %v", err)
	}

	if err := store.Delete(ctx, key); err == nil {
		t.Error("expected Delete to fail under governance lock, got nil")
	}
}
