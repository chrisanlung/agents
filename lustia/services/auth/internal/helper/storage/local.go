// Package storage provides file-storage adapters for the auth service.
// The Storage interface is declared in service/interfaces.go (consumer-owned).
// All adapters live here and are wired through the factory.
package storage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// LocalStorage writes files to an absolute directory on the local filesystem
// and serves them through a public base URL.
// Dev-only: unsafe for multi-replica deployments (ADR 0011 §2.2 condition 3).
type LocalStorage struct {
	basePath      string // absolute directory path
	publicBaseURL string // e.g. "http://localhost:8080/uploads"
}

// NewLocalStorage constructs a LocalStorage.
// basePath must be absolute; publicBaseURL is the URL prefix for URL().
func NewLocalStorage(basePath, publicBaseURL string) (*LocalStorage, error) {
	if !filepath.IsAbs(basePath) {
		return nil, fmt.Errorf("local storage: basePath must be absolute, got %q", basePath)
	}
	if err := os.MkdirAll(basePath, 0o755); err != nil {
		return nil, fmt.Errorf("local storage: cannot create basePath %q: %w", basePath, err)
	}
	// Trim trailing slash for consistent URL joins.
	publicBaseURL = strings.TrimRight(publicBaseURL, "/")
	return &LocalStorage{basePath: basePath, publicBaseURL: publicBaseURL}, nil
}

// Upload writes r to basePath/key using a temp-file-then-rename strategy for
// near-atomic writes (atomic on Linux; best-effort on Windows per ADR 0011 §4.1).
func (s *LocalStorage) Upload(_ context.Context, key string, r io.Reader, _ string) error {
	dest := filepath.Join(s.basePath, filepath.FromSlash(key))
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return fmt.Errorf("local upload: mkdir %q: %w", filepath.Dir(dest), err)
	}

	tmp, err := os.CreateTemp(filepath.Dir(dest), ".tmp-upload-*")
	if err != nil {
		return fmt.Errorf("local upload: create temp file: %w", err)
	}
	tmpName := tmp.Name()

	// Ensure temp file is always cleaned up if we return early with an error.
	committed := false
	defer func() {
		if !committed {
			_ = tmp.Close()
			_ = os.Remove(tmpName)
		}
	}()

	if _, err := io.Copy(tmp, r); err != nil {
		return fmt.Errorf("local upload: write %q: %w", key, err)
	}
	if err := tmp.Sync(); err != nil {
		return fmt.Errorf("local upload: sync %q: %w", key, err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("local upload: close temp %q: %w", key, err)
	}

	if err := os.Rename(tmpName, dest); err != nil {
		return fmt.Errorf("local upload: rename to %q: %w", dest, err)
	}
	committed = true
	return nil
}

// Delete removes the file at basePath/key.
// A missing file is not an error (idempotent deletes — ADR 0011 §2.1).
func (s *LocalStorage) Delete(_ context.Context, key string) error {
	if key == "" {
		return nil
	}
	dest := filepath.Join(s.basePath, filepath.FromSlash(key))
	if err := os.Remove(dest); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("local delete %q: %w", key, err)
	}
	return nil
}

// URL returns the public URL for key by joining publicBaseURL and key.
// For local storage the key is always readable (no signing needed).
func (s *LocalStorage) URL(_ context.Context, key string) (string, error) {
	if key == "" {
		return "", fmt.Errorf("URL: empty key")
	}
	return s.publicBaseURL + "/" + key, nil
}

// BasePath returns the resolved absolute storage directory (used by route.go
// for StaticFS registration).
func (s *LocalStorage) BasePath() string { return s.basePath }
