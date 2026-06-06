package adapter

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"

	"openforge/internal/shared/kernel"
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

// SetBucketObjectLock enables object lock on the bucket with the given
// default retention. The bucket must have been created with object lock
// support; on MinIO this is enabled per-bucket via SetBucketObjectLockConfig.
// If the bucket does not support object lock, the underlying call returns
// an error which we propagate.
//
// A disabled store returns a clear error rather than a panic so callers
// (notably bootstrap) can log and continue.
func (m *MinioObjectStore) SetBucketObjectLock(ctx context.Context, cfg kernel.ObjectLockConfig) error {
	if !m.enabled {
		return fmt.Errorf("minio object store is disabled")
	}

	mode := minio.RetentionMode(cfg.Mode)
	if !mode.IsValid() {
		return fmt.Errorf("SetBucketObjectLock: invalid mode %q (want GOVERNANCE or COMPLIANCE)", cfg.Mode)
	}

	// minio-go: SetBucketObjectLockConfig(ctx, bucket, mode, validity, unit)
	// mode, validity and unit must be all set or all nil. Passing nil
	// disables object lock on the bucket.
	if cfg.Days <= 0 {
		return fmt.Errorf("SetBucketObjectLock: Days must be > 0")
	}

	ctx, cancel := context.WithTimeout(ctx, m.timeout)
	defer cancel()

	days := uint(cfg.Days)
	unit := minio.Days
	if err := m.client.SetBucketObjectLockConfig(ctx, m.bucket, &mode, &days, &unit); err != nil {
		return fmt.Errorf("SetBucketObjectLock %s: %w", m.bucket, err)
	}
	return nil
}

// GetBucketObjectLock reports the bucket's current object lock configuration.
// If object lock is not enabled the underlying call returns nil values;
// we surface that as a zero ObjectLockConfig with no error.
func (m *MinioObjectStore) GetBucketObjectLock(ctx context.Context) (kernel.ObjectLockConfig, error) {
	if !m.enabled {
		return kernel.ObjectLockConfig{}, fmt.Errorf("minio object store is disabled")
	}

	ctx, cancel := context.WithTimeout(ctx, m.timeout)
	defer cancel()

	mode, validity, _, err := m.client.GetBucketObjectLockConfig(ctx, m.bucket)
	if err != nil {
		return kernel.ObjectLockConfig{}, fmt.Errorf("GetBucketObjectLock %s: %w", m.bucket, err)
	}
	if mode == nil {
		// Object lock not enabled.
		return kernel.ObjectLockConfig{}, nil
	}
	cfg := kernel.ObjectLockConfig{Mode: kernel.ObjectLockMode(*mode)}
	if validity != nil {
		cfg.Days = int(*validity)
	}
	return cfg, nil
}

// SetObjectRetention applies per-object retention. Once set, the object
// cannot be deleted (the Delete call returns an error) until the
// retain-until date has passed. A privileged caller can override with
// GOVERNANCE bypass.
//
// We use the bucket-level helper rather than the per-object one when the
// retain-until date is the only thing we set; minio-go's PutObjectRetention
// takes a RetainUntilDate pointer. Computing the date from Days keeps the
// kernel surface in terms of "days from now" without exposing time.Time.
func (m *MinioObjectStore) SetObjectRetention(ctx context.Context, key string, ret kernel.RetentionConfig) error {
	if !m.enabled {
		return fmt.Errorf("minio object store is disabled")
	}

	mode := minio.RetentionMode(ret.Mode)
	if !mode.IsValid() {
		return fmt.Errorf("SetObjectRetention: invalid mode %q (want GOVERNANCE or COMPLIANCE)", ret.Mode)
	}
	if ret.Days <= 0 {
		return fmt.Errorf("SetObjectRetention: Days must be > 0")
	}

	ctx, cancel := context.WithTimeout(ctx, m.timeout)
	defer cancel()

	retainUntil := time.Now().Add(time.Duration(ret.Days) * 24 * time.Hour).UTC()
	opts := minio.PutObjectRetentionOptions{
		Mode:            &mode,
		RetainUntilDate: &retainUntil,
	}
	if err := m.client.PutObjectRetention(ctx, m.bucket, key, opts); err != nil {
		return fmt.Errorf("SetObjectRetention %s/%s: %w", m.bucket, key, err)
	}
	return nil
}
