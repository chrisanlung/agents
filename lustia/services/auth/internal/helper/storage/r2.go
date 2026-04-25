package storage

import (
	"context"
	"errors"
	"io"
	"os"
)

// CloudflareR2Storage is a stub adapter for Cloudflare R2 (S3-compatible).
// TODO: implement in ADR 0011 §2.7 — Phase 5.
// All methods return errors.New("r2 adapter not yet implemented").
type CloudflareR2Storage struct {
	accountID       string
	accessKeyID     string
	secretAccessKey string
	bucket          string
}

// NewCloudflareR2Storage constructs a CloudflareR2Storage from environment variables.
// Required env vars: STORAGE_R2_ACCOUNT_ID, STORAGE_R2_ACCESS_KEY_ID,
// STORAGE_R2_SECRET_ACCESS_KEY, STORAGE_R2_BUCKET.
func NewCloudflareR2Storage() (*CloudflareR2Storage, error) {
	return &CloudflareR2Storage{
		accountID:       os.Getenv("STORAGE_R2_ACCOUNT_ID"),
		accessKeyID:     os.Getenv("STORAGE_R2_ACCESS_KEY_ID"),
		secretAccessKey: os.Getenv("STORAGE_R2_SECRET_ACCESS_KEY"),
		bucket:          os.Getenv("STORAGE_R2_BUCKET"),
	}, nil
}

// Upload is not yet implemented. See ADR 0011 §2.7.
func (s *CloudflareR2Storage) Upload(_ context.Context, _ string, _ io.Reader, _ string) error {
	return errors.New("r2 adapter not yet implemented")
}

// Delete is not yet implemented. See ADR 0011 §2.7.
func (s *CloudflareR2Storage) Delete(_ context.Context, _ string) error {
	return errors.New("r2 adapter not yet implemented")
}

// URL is not yet implemented. See ADR 0011 §2.7.
func (s *CloudflareR2Storage) URL(_ context.Context, _ string) (string, error) {
	return "", errors.New("r2 adapter not yet implemented")
}

// HealthCheck returns an error indicating the adapter is not yet implemented.
func (s *CloudflareR2Storage) HealthCheck(_ context.Context) error {
	return errors.New("r2 adapter not yet implemented")
}
