package harness

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
)

// JWTClaims holds the subset of JWT claims the integration tests care about.
// Verification (signature/expiry) is intentionally omitted here — these are
// integration tests against the live service; we trust the service's JWT
// middleware handles verification on the receiving end. We decode only to
// inspect claim values.
type JWTClaims struct {
	Sub                string   `json:"sub"`
	TenantID           string   `json:"tenant_id"`
	MembershipID       *string  `json:"membership_id"`
	Scope              string   `json:"scope"`
	Roles              []string `json:"roles"`
	Permissions        []string `json:"permissions"`
	Branches           []string `json:"branches"`
	Email              string   `json:"email"`
	FullName           string   `json:"full_name"`
	MustChangePassword bool     `json:"must_change_password"`
}

// DecodeJWT decodes the payload of a JWT without verifying the signature.
// Returns an error if the token is structurally invalid.
func DecodeJWT(token string) (JWTClaims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return JWTClaims{}, fmt.Errorf("invalid JWT: expected 3 parts, got %d", len(parts))
	}
	// JWT base64url padding is omitted; add it back.
	payload := parts[1]
	switch len(payload) % 4 {
	case 2:
		payload += "=="
	case 3:
		payload += "="
	}
	raw, err := base64.URLEncoding.DecodeString(payload)
	if err != nil {
		return JWTClaims{}, fmt.Errorf("decode JWT payload: %w", err)
	}
	var claims JWTClaims
	if err := json.Unmarshal(raw, &claims); err != nil {
		return JWTClaims{}, fmt.Errorf("unmarshal JWT claims: %w", err)
	}
	return claims, nil
}
