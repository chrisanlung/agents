package storage

import (
	"fmt"
	"sync"
	"time"

	"github.com/chrisanlung/lustia-auth/internal/constants"
)

// TenantQuota implements an in-memory sliding-window rate limit for per-tenant
// upload events. It is not persisted across service restarts — acceptable
// per ADR 0011 §2.4.6.
//
// Expired timestamps are pruned on each Check call to prevent unbounded growth.
type TenantQuota struct {
	mu     sync.Mutex
	window time.Duration
	limit  int
	events map[string][]time.Time // tenantID → sorted upload timestamps
}

// NewTenantQuota constructs a TenantQuota.
// window is the rolling window duration (e.g. 1 * time.Hour).
// limit is the max number of uploads allowed within that window per tenant.
func NewTenantQuota(window time.Duration, limit int) *TenantQuota {
	return &TenantQuota{
		window: window,
		limit:  limit,
		events: make(map[string][]time.Time),
	}
}

// Check returns ErrUploadQuotaExceeded when the tenant has already consumed
// all available slots in the current window. It prunes expired timestamps
// before evaluating.
func (q *TenantQuota) Check(tenantID string) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	q.prune(tenantID)
	if len(q.events[tenantID]) >= q.limit {
		return fmt.Errorf("%w: tenant %q", constants.ErrUploadQuotaExceeded, tenantID)
	}
	return nil
}

// Record registers a successful upload event for the tenant.
// Callers must call Check before Record.
func (q *TenantQuota) Record(tenantID string) {
	q.mu.Lock()
	defer q.mu.Unlock()

	q.events[tenantID] = append(q.events[tenantID], time.Now())
}

// prune removes timestamps older than the window. Must be called with q.mu held.
func (q *TenantQuota) prune(tenantID string) {
	cutoff := time.Now().Add(-q.window)
	ts := q.events[tenantID]

	// Find first index still within the window.
	i := 0
	for i < len(ts) && ts[i].Before(cutoff) {
		i++
	}
	if i == 0 {
		return
	}
	q.events[tenantID] = ts[i:]
	if len(q.events[tenantID]) == 0 {
		delete(q.events, tenantID)
	}
}
