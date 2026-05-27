package adapter

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// MinioConfig holds MinIO connection parameters (decoupled from profile package).
type MinioConfig struct {
	Endpoint        string
	AccessKeyID     string
	SecretAccessKey string
	Bucket          string
	UseSSL          bool
	Region          string
	Timeout         time.Duration
}

// MinioObjectStore implements kernel.ObjectStore using MinIO.
type MinioObjectStore struct {
	client  *minio.Client
	bucket  string
	timeout time.Duration
	enabled bool
}

// NewMinioObjectStore creates a new MinIO-backed object store.
// If the endpoint is empty or connection fails, enabled=false and system continues with noop.
func NewMinioObjectStore(cfg MinioConfig) *MinioObjectStore {
	if cfg.Endpoint == "" {
		slog.Warn("minio object store disabled: empty endpoint")
		return &MinioObjectStore{enabled: false}
	}

	bucket := cfg.Bucket
	if bucket == "" {
		bucket = "openforge"
	}

	region := cfg.Region
	if region == "" {
		region = "us-east-1"
	}

	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = 5 * time.Second
	}

	client, err := minio.New(cfg.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKeyID, cfg.SecretAccessKey, ""),
		Secure: cfg.UseSSL,
		Region: region,
	})
	if err != nil {
		slog.Warn("minio client creation failed, falling back to noop", "error", err)
		return &MinioObjectStore{enabled: false}
	}

	// Try to create bucket (idempotent, ignore AlreadyExists)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	err = client.MakeBucket(ctx, bucket, minio.MakeBucketOptions{Region: region})
	if err != nil {
		// Check if bucket already exists
		exists, err := client.BucketExists(ctx, bucket)
		if err != nil || !exists {
			slog.Warn("minio bucket creation failed, falling back to noop", "error", err, "bucket", bucket)
			return &MinioObjectStore{enabled: false}
		}
	}

	slog.Info("minio object store enabled",
		"endpoint", cfg.Endpoint,
		"bucket", bucket,
		"region", region,
	)

	return &MinioObjectStore{
		client:  client,
		bucket:  bucket,
		timeout: timeout,
		enabled: true,
	}
}

// Put uploads an object to MinIO.
func (m *MinioObjectStore) Put(ctx context.Context, key string, reader io.Reader) error {
	if !m.enabled {
		return fmt.Errorf("minio object store is disabled")
	}

	ctx, cancel := context.WithTimeout(ctx, m.timeout)
	defer cancel()

	_, err := m.client.PutObject(ctx, m.bucket, key, reader, -1, minio.PutObjectOptions{})
	if err != nil {
		return fmt.Errorf("minio put %s: %w", key, err)
	}
	return nil
}

// Get downloads an object from MinIO.
func (m *MinioObjectStore) Get(ctx context.Context, key string) (io.ReadCloser, error) {
	if !m.enabled {
		return nil, fmt.Errorf("minio object store is disabled")
	}

	ctx, cancel := context.WithTimeout(ctx, m.timeout)
	defer cancel()

	object, err := m.client.GetObject(ctx, m.bucket, key, minio.GetObjectOptions{})
	if err != nil {
		return nil, fmt.Errorf("minio get %s: %w", key, err)
	}
	return object, nil
}

// Delete removes an object from MinIO.
func (m *MinioObjectStore) Delete(ctx context.Context, key string) error {
	if !m.enabled {
		return fmt.Errorf("minio object store is disabled")
	}

	ctx, cancel := context.WithTimeout(ctx, m.timeout)
	defer cancel()

	err := m.client.RemoveObject(ctx, m.bucket, key, minio.RemoveObjectOptions{})
	if err != nil {
		return fmt.Errorf("minio delete %s: %w", key, err)
	}
	return nil
}

// List returns objects with the given prefix.
func (m *MinioObjectStore) List(ctx context.Context, prefix string) ([]string, error) {
	if !m.enabled {
		return nil, fmt.Errorf("minio object store is disabled")
	}

	ctx, cancel := context.WithTimeout(ctx, m.timeout)
	defer cancel()

	objectCh := m.client.ListObjects(ctx, m.bucket, minio.ListObjectsOptions{
		Prefix:    prefix,
		Recursive: true,
	})

	var keys []string
	for object := range objectCh {
		if object.Err != nil {
			return nil, fmt.Errorf("minio list: %w", object.Err)
		}
		keys = append(keys, object.Key)
	}
	return keys, nil
}

// IsEnabled returns whether the store is operational.
func (m *MinioObjectStore) IsEnabled() bool {
	return m.enabled
}
