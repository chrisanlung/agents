package storage_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/chrisanlung/lustia-auth/internal/helper/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Compile-time interface satisfaction checks.
var _ storage.Adapter = (*storage.LocalStorage)(nil)
var _ storage.Adapter = (*storage.CloudflareR2Storage)(nil)
var _ storage.Adapter = (*storage.SupabaseStorage)(nil)

func TestLocalStorage_UploadDeleteURL_RoundTrip(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	s, err := storage.NewLocalStorage(dir, "http://example.com/files")
	require.NoError(t, err)

	ctx := context.Background()
	key := "therapists/tid/abc123.jpg"
	content := []byte("fake-jpeg-bytes")

	// Upload.
	err = s.Upload(ctx, key, bytes.NewReader(content), "image/jpeg")
	require.NoError(t, err)

	// File must exist.
	dest := filepath.Join(dir, filepath.FromSlash(key))
	got, err := os.ReadFile(dest)
	require.NoError(t, err)
	assert.Equal(t, content, got)

	// URL resolution.
	url, err := s.URL(ctx, key)
	require.NoError(t, err)
	assert.Equal(t, "http://example.com/files/"+key, url)

	// Delete.
	err = s.Delete(ctx, key)
	require.NoError(t, err)

	// File must be gone.
	_, err = os.Stat(dest)
	assert.True(t, os.IsNotExist(err))

	// Deleting again is a no-op (idempotent).
	err = s.Delete(ctx, key)
	require.NoError(t, err)
}

func TestLocalStorage_Upload_CreatesSubdirectories(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	s, err := storage.NewLocalStorage(dir, "http://example.com")
	require.NoError(t, err)

	ctx := context.Background()
	key := "deep/nested/path/file.png"
	err = s.Upload(ctx, key, bytes.NewReader([]byte("data")), "image/png")
	require.NoError(t, err)

	dest := filepath.Join(dir, filepath.FromSlash(key))
	content, err := os.ReadFile(dest)
	require.NoError(t, err)
	assert.Equal(t, []byte("data"), content)
}

func TestLocalStorage_URL_EmptyKey(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	s, err := storage.NewLocalStorage(dir, "http://example.com")
	require.NoError(t, err)

	_, err = s.URL(context.Background(), "")
	assert.Error(t, err)
}

func TestNewLocalStorage_RelativePathReturnsError(t *testing.T) {
	t.Parallel()

	_, err := storage.NewLocalStorage("relative/path", "http://example.com")
	assert.Error(t, err)
}

func TestNewStorageAdapter_UnknownDriver(t *testing.T) {
	t.Parallel()

	_, err := storage.NewStorageAdapter(storage.Config{Driver: "unknown"})
	assert.Error(t, err)
}

func TestNewStorageAdapter_EmptyDriver(t *testing.T) {
	t.Parallel()

	_, err := storage.NewStorageAdapter(storage.Config{Driver: ""})
	assert.Error(t, err)
}

func TestNewStorageAdapter_LocalDriver(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	adapter, err := storage.NewStorageAdapter(storage.Config{
		Driver:        "local",
		LocalPath:     dir,
		PublicBaseURL: "http://localhost/uploads",
	})
	require.NoError(t, err)
	assert.NotNil(t, adapter)
}
