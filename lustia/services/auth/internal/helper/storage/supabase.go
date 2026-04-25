package storage

import (
	"context"
	"errors"
	"io"
	"os"
)

// SupabaseStorage is a stub adapter for Supabase Storage.
// TODO: implement in ADR 0011 §2.7 — Phase 5.
// All methods return errors.New("supabase adapter not yet implemented").
type SupabaseStorage struct {
	supabaseURL string
	serviceKey  string
	bucket      string
}

// NewSupabaseStorage constructs a SupabaseStorage from environment variables.
// Required env vars: STORAGE_SUPABASE_URL, STORAGE_SUPABASE_SERVICE_KEY,
// STORAGE_SUPABASE_BUCKET.
func NewSupabaseStorage() (*SupabaseStorage, error) {
	return &SupabaseStorage{
		supabaseURL: os.Getenv("STORAGE_SUPABASE_URL"),
		serviceKey:  os.Getenv("STORAGE_SUPABASE_SERVICE_KEY"),
		bucket:      os.Getenv("STORAGE_SUPABASE_BUCKET"),
	}, nil
}

// Upload is not yet implemented. See ADR 0011 §2.7.
func (s *SupabaseStorage) Upload(_ context.Context, _ string, _ io.Reader, _ string) error {
	return errors.New("supabase adapter not yet implemented")
}

// Delete is not yet implemented. See ADR 0011 §2.7.
func (s *SupabaseStorage) Delete(_ context.Context, _ string) error {
	return errors.New("supabase adapter not yet implemented")
}

// URL is not yet implemented. See ADR 0011 §2.7.
func (s *SupabaseStorage) URL(_ context.Context, _ string) (string, error) {
	return "", errors.New("supabase adapter not yet implemented")
}

// HealthCheck returns an error indicating the adapter is not yet implemented.
func (s *SupabaseStorage) HealthCheck(_ context.Context) error {
	return errors.New("supabase adapter not yet implemented")
}
