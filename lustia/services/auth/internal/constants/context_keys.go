package constants

// Context keys used to stash values set by middleware into gin.Context.
// Using typed constants avoids string collision across middleware.
const (
	// CtxKeyAccessClaims is the key for the parsed model.AccessClaims in the
	// gin context. Set by middleware/jwt.go.
	CtxKeyAccessClaims = "access_claims"

	// CtxKeyRequestID is the key for the X-Request-ID value.
	CtxKeyRequestID = "request_id"

	// CtxKeyTenantID is the key for the resolved tenant ID stashed by
	// middleware/tenant.go.
	CtxKeyTenantID = "tenant_id"

	// CtxKeyUserID is the key for the resolved user ID stashed by
	// middleware/tenant.go.
	CtxKeyUserID = "user_id"
)
