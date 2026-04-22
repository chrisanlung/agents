package constants

// JWT claim names and token type strings.
const (
	JWTIssuer   = "lustia-auth"
	JWTAudience = "lustia"

	// PlatformTenantSentinel is the special tenant scope used by super_admin users.
	// It is passed as SET LOCAL app.current_tenant = '__platform__' to satisfy
	// the RLS super-admin bypass policy on the user table.
	PlatformTenantSentinel = "__platform__"

	TokenTypeBearer = "Bearer"
)
