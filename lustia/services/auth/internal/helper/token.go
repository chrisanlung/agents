package helper

import (
	"crypto/sha256"
	"encoding/hex"

	"github.com/google/uuid"
)

// GenerateOpaqueToken generates a cryptographically random opaque token string
// suitable for use as a refresh token or password reset token.
// Format: two UUIDs concatenated (72 chars of entropy from crypto/rand-backed uuid).
func GenerateOpaqueToken() string {
	return uuid.New().String() + uuid.New().String()
}

// HashToken returns the lowercase hex-encoded SHA-256 digest of a raw opaque
// token string. Only the hash is stored in the DB; the raw value lives only
// in memory and on the wire.
func HashToken(raw string) string {
	h := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(h[:])
}

// DerefString returns the dereferenced string or "" when nil.
func DerefString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// StrPtr returns a pointer to s, or nil when s is empty.
func StrPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
