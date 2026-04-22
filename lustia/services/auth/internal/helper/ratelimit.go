package helper

import (
	"context"
	"sync"
	"time"
)

// tokenBucket is a simple token-bucket rate limiter for one key.
type tokenBucket struct {
	mu         sync.Mutex
	tokens     float64
	maxTokens  float64
	refillRate float64 // tokens per second
	lastRefill time.Time
}

func newTokenBucket(maxTokens, refillRate float64) *tokenBucket {
	return &tokenBucket{
		tokens:     maxTokens,
		maxTokens:  maxTokens,
		refillRate: refillRate,
		lastRefill: time.Now(),
	}
}

func (b *tokenBucket) allow() bool {
	b.mu.Lock()
	defer b.mu.Unlock()

	now := time.Now()
	elapsed := now.Sub(b.lastRefill).Seconds()
	b.tokens = clampTokens(b.maxTokens, b.tokens+elapsed*b.refillRate)
	b.lastRefill = now

	if b.tokens >= 1 {
		b.tokens--
		return true
	}
	return false
}

// MemoryRateLimiter implements RateLimiter using in-memory token buckets.
// Each unique key gets its own bucket.
//
// TODO(phase-10): migrate to Redis via common-configs/redis for distributed limiting.
type MemoryRateLimiter struct {
	mu         sync.Mutex
	buckets    map[string]*tokenBucket
	maxTokens  float64
	refillRate float64
}

// NewMemoryRateLimiter constructs a MemoryRateLimiter.
// maxTokens is the burst capacity; refillRate is tokens-per-second.
func NewMemoryRateLimiter(maxTokens, refillRate float64) *MemoryRateLimiter {
	return &MemoryRateLimiter{
		buckets:    make(map[string]*tokenBucket),
		maxTokens:  maxTokens,
		refillRate: refillRate,
	}
}

// Allow returns true if the key's bucket has capacity, false otherwise.
func (rl *MemoryRateLimiter) Allow(_ context.Context, key string) bool {
	rl.mu.Lock()
	b, ok := rl.buckets[key]
	if !ok {
		b = newTokenBucket(rl.maxTokens, rl.refillRate)
		rl.buckets[key] = b
	}
	rl.mu.Unlock()
	return b.allow()
}

func clampTokens(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}
