package model

import "time"

// TokenScope enumerates the three scopes an access token may carry.
// The scope controls what the caller is authorised to do without selecting
// a tenant first.
type TokenScope string

const (
	// ScopePlatform is issued for super-admin users. The token carries
	// tenant_id = "__platform__" and roles = ["super_admin"].
	ScopePlatform TokenScope = "platform"

	// ScopeTenant is issued when a single active membership exists at login
	// time, or after a successful POST /auth/select-tenant call. The token
	// carries tenant_id, membership_id, roles, permissions, and branches
	// derived from that specific membership.
	ScopeTenant TokenScope = "tenant"

	// ScopeUser is issued when a user has more than one active membership and
	// has not yet selected a tenant. tenant_id, membership_id, roles,
	// permissions, and branches are all null/empty. Protected endpoints except
	// GET /auth/me, POST /auth/select-tenant, POST /auth/logout, and
	// POST /auth/me/password return 403 TENANT_NOT_SELECTED until the user
	// calls select-tenant.
	ScopeUser TokenScope = "user"
)

// AccessClaims represents the payload embedded in a signed JWT access token.
// Every claim maps directly to a field in the signed JWT. Downstream services
// verify the token using the JWKS public key and read these claims
// directly — they never call the auth service on each request.
type AccessClaims struct {
	// Standard claims.
	Issuer    string    // iss
	Subject   string    // sub — UserID as string
	Audience  []string  // aud
	IssuedAt  time.Time // iat
	ExpiresAt time.Time // exp
	JWTID     string    // jti — unique token ID for revocation/audit

	// Lustia-specific claims.
	Scope        TokenScope // "platform" | "tenant" | "user"
	TenantID     string     // tenant_id; "__platform__" for super_admin; "" for scope=user
	MembershipID *string    // membership_id; nil for scope=platform and scope=user
	Roles        []string   // role names; empty for scope=user
	Permissions  []string   // permission codes; empty for scope=user
	Branches     []string   // branch IDs the user is assigned to; empty for scope=user
	Email        string     // user email (convenience)
	FullName     string     // user full name (convenience)

	// MustChangePassword signals the caller must rotate their password before
	// making any protected request other than GET /auth/me, POST /auth/me/password,
	// or POST /auth/logout. Enforced by middleware.PasswordChangeRequired.
	MustChangePassword bool
}
