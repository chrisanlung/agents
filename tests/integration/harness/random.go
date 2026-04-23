package harness

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"
)

// UniqueEmail returns an email address that is unique across test runs.
// Format: <prefix>-<timestamp>-<random>@integration-test.invalid
// The `.invalid` TLD (RFC 2606) ensures these addresses can never receive real mail.
func UniqueEmail(prefix string) string {
	b := make([]byte, 4)
	_, _ = rand.Read(b)
	return fmt.Sprintf("%s-%d-%s@integration-test.invalid", prefix, time.Now().UnixNano(), hex.EncodeToString(b))
}

// UniqueSlug returns a slug that is unique across test runs.
// Format: <prefix>-<timestamp>-<random>
// Slug regex: ^[a-z0-9][a-z0-9-]*[a-z0-9]$  (min 2 chars)
func UniqueSlug(prefix string) string {
	b := make([]byte, 3)
	_, _ = rand.Read(b)
	ts := time.Now().UnixNano() % 1_000_000_000 // keep slug length manageable
	return fmt.Sprintf("%s-%d-%s", prefix, ts, hex.EncodeToString(b))
}

// UniqueIP returns a synthetic 10.x.x.x IP address unique per call.
// Used to give each test its own fresh rate-limit bucket so tests that submit
// registrations do not interfere with T3 (the rate-limit test itself).
func UniqueIP() string {
	b := make([]byte, 3)
	_, _ = rand.Read(b)
	return fmt.Sprintf("10.%d.%d.%d", b[0], b[1], b[2])
}
