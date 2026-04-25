package storage_test

import (
	"testing"
	"time"

	"github.com/chrisanlung/lustia-auth/internal/constants"
	"github.com/chrisanlung/lustia-auth/internal/helper/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTenantQuota_AllowsUpToLimit(t *testing.T) {
	t.Parallel()

	q := storage.NewTenantQuota(time.Hour, 3)

	for i := range 3 {
		err := q.Check("tenant-1")
		require.NoError(t, err, "call %d should be allowed", i+1)
		q.Record("tenant-1")
	}

	// Fourth call must be rejected.
	err := q.Check("tenant-1")
	require.Error(t, err)
	assert.ErrorIs(t, err, constants.ErrUploadQuotaExceeded)
}

func TestTenantQuota_IsolatedPerTenant(t *testing.T) {
	t.Parallel()

	q := storage.NewTenantQuota(time.Hour, 2)

	// Fill tenant-A.
	q.Record("tenant-A")
	q.Record("tenant-A")

	// tenant-B should still have capacity.
	err := q.Check("tenant-B")
	require.NoError(t, err)
}

func TestTenantQuota_ExpiryAllowsNewUploads(t *testing.T) {
	t.Parallel()

	// Use a very short window so we can test expiry without sleeping long.
	q := storage.NewTenantQuota(50*time.Millisecond, 1)

	q.Record("tenant-1")

	// Immediately at limit.
	err := q.Check("tenant-1")
	require.Error(t, err)

	// Wait for the window to expire.
	time.Sleep(60 * time.Millisecond)

	// Should be allowed again.
	err = q.Check("tenant-1")
	require.NoError(t, err)
}

func TestTenantQuota_ZeroLimit(t *testing.T) {
	t.Parallel()

	q := storage.NewTenantQuota(time.Hour, 0)

	// Any upload should be denied immediately.
	err := q.Check("tenant-1")
	require.Error(t, err)
	assert.ErrorIs(t, err, constants.ErrUploadQuotaExceeded)
}
