package storage

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// Config carries the storage configuration values read by the composition root
// (main.go). Values are passed in rather than read directly from os.Getenv
// here so this package stays testable.
type Config struct {
	Driver        string // "local" | "r2" | "supabase"
	LocalPath     string // absolute path when Driver="local"
	PublicBaseURL string // e.g. "http://localhost:8080/uploads" when Driver="local"
}

// Adapter is the minimal storage interface mirrored from service.Storage.
// Declared here so the factory can return a typed value without importing
// the service package (which would create an import cycle).
// The concrete adapters satisfy both this interface and service.Storage
// implicitly — they have identical method sets.
type Adapter interface {
	Upload(ctx context.Context, key string, r io.Reader, mimeType string) error
	Delete(ctx context.Context, key string) error
	URL(ctx context.Context, key string) (string, error)
}

// NewStorageAdapter constructs the concrete adapter matching cfg.Driver.
// The returned Adapter satisfies service.Storage (identical method set).
//
// Fail-fast conditions per ADR 0011 §2.2:
//  1. Driver unset or unknown — returns error.
//  2. driver=local + relative LocalPath — resolved with filepath.Abs (logged by caller).
//  3. driver=local + KUBERNETES_SERVICE_HOST set — multi-replica trap → returns error.
func NewStorageAdapter(cfg Config) (Adapter, error) {
	switch cfg.Driver {
	case "local":
		localPath := cfg.LocalPath
		if localPath == "" {
			localPath = "./storage-data/uploads"
		}
		if !filepath.IsAbs(localPath) {
			abs, err := filepath.Abs(localPath)
			if err != nil {
				return nil, fmt.Errorf("storage factory: cannot resolve local path %q: %w", localPath, err)
			}
			// Caller is responsible for logging the resolution.
			localPath = abs
		}
		if os.Getenv("KUBERNETES_SERVICE_HOST") != "" {
			return nil, fmt.Errorf("storage factory: local storage is unsafe in multi-replica deployments (KUBERNETES_SERVICE_HOST is set); use driver=r2 or driver=supabase")
		}
		publicBaseURL := cfg.PublicBaseURL
		if publicBaseURL == "" {
			publicBaseURL = "http://localhost:8080/uploads"
		}
		return NewLocalStorage(localPath, publicBaseURL)

	case "r2":
		return NewCloudflareR2Storage()

	case "supabase":
		return NewSupabaseStorage()

	case "":
		return nil, fmt.Errorf("storage factory: STORAGE_DRIVER is unset; set it to one of: local, r2, supabase")

	default:
		return nil, fmt.Errorf("storage factory: unknown driver %q; allowed values: local, r2, supabase", cfg.Driver)
	}
}
