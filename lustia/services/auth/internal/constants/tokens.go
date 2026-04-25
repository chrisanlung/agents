package constants

// JWT claim names and token type strings.
const (
	JWTIssuer   = "lustia-auth"
	JWTAudience = "lustia"

	// PlatformTenantSentinel is the special tenant scope used by super_admin users.
	// It is passed as SET LOCAL app.current_tenant = '__platform__' to satisfy
	// the RLS super-admin bypass policy on the user table.
	PlatformTenantSentinel = "__platform__"

	// PublicTenantSentinel is the special tenant scope used by unauthenticated
	// customer endpoints. It triggers the booking_public_select additive RLS
	// policy (migration 000025) that allows reading active-tenant booking rows
	// by code only. The service layer MUST scope all public queries to a specific
	// code — an unbounded query under this sentinel would expose all booking rows.
	// See ADR 0014 §3.17 and SECURITY.md H-7.
	PublicTenantSentinel = "__public__"

	TokenTypeBearer = "Bearer"
)
