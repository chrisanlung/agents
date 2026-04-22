// Package helper contains pure utility functions for the auth service.
// Helpers depend only on stdlib, constants, model, and specific third-party
// libraries relevant to the helper (argon2id, jwt). Never gin or gorm.
package helper

import (
	"context"
	"fmt"

	"github.com/alexedwards/argon2id"
)

// argon2Params are the Argon2id parameters used for all password hashing.
// These values follow OWASP/RFC 9106 minimum recommendations.
// SECURITY: do NOT lower these values. See docs/SECURITY.md §2.1.
var argon2Params = &argon2id.Params{
	Memory:      64 * 1024, // 64 MiB
	Iterations:  3,
	Parallelism: 2,
	SaltLength:  16,
	KeyLength:   32,
}

// PasswordHasher is the interface the service layer depends on for password
// hashing. Declared here (in the consumer package) so service tests can swap
// in a fake without importing the concrete implementation.
type PasswordHasher interface {
	Hash(ctx context.Context, password string) (string, error)
	Verify(ctx context.Context, password, hash string) (bool, error)
}

// Argon2idHasher implements PasswordHasher using the Argon2id algorithm.
type Argon2idHasher struct{}

// NewArgon2idHasher constructs an Argon2idHasher.
func NewArgon2idHasher() *Argon2idHasher { return &Argon2idHasher{} }

// Hash returns an Argon2id encoded string for the given plaintext password.
func (h *Argon2idHasher) Hash(_ context.Context, password string) (string, error) {
	hash, err := argon2id.CreateHash(password, argon2Params)
	if err != nil {
		return "", fmt.Errorf("argon2id hash: %w", err)
	}
	return hash, nil
}

// Verify checks whether plaintext matches the encoded Argon2id hash.
// Returns (false, nil) when the password does not match.
// Returns (false, err) only for internal errors (malformed hash, etc.).
func (h *Argon2idHasher) Verify(_ context.Context, password, hash string) (bool, error) {
	match, err := argon2id.ComparePasswordAndHash(password, hash)
	if err != nil {
		return false, fmt.Errorf("argon2id verify: %w", err)
	}
	return match, nil
}
