package middleware

import (
	"net/http"

	"github.com/chrisanlung/lustia-auth/internal/constants"
	"github.com/chrisanlung/lustia-auth/internal/helper"
	"github.com/chrisanlung/lustia-auth/internal/model"
	"github.com/gin-gonic/gin"
)

// ScopeGate returns a middleware that rejects requests whose JWT scope is
// scope=user (i.e. the caller has authenticated but has not yet selected a
// tenant via POST /auth/select-tenant).
//
// Endpoints allowed even with scope=user (the allowlist):
//   - GET  /api/v1/auth/me
//   - POST /api/v1/auth/select-tenant
//   - POST /api/v1/auth/logout
//   - GET  /api/v1/auth/.well-known/jwks.json
//   - POST /api/v1/auth/me/password
//
// All other authenticated endpoints return 403 TENANT_NOT_SELECTED until
// the caller upgrades their token by calling select-tenant.
//
// scope=platform and scope=tenant tokens are never blocked by this gate.
// Unauthenticated requests (no claims) are passed through — the JWTVerify
// middleware upstream already handles the unauthenticated case.
func ScopeGate() gin.HandlerFunc {
	return func(c *gin.Context) {
		claims, ok := ClaimsFromContext(c)
		if !ok {
			// No claims — let downstream middleware (e.g. JWTVerify) handle it.
			c.Next()
			return
		}

		if claims.Scope != model.ScopeUser {
			// scope=platform or scope=tenant — always allowed.
			c.Next()
			return
		}

		// scope=user — check against the allowlist.
		if isScopeUserAllowed(c.Request.Method, c.FullPath()) {
			c.Next()
			return
		}

		c.AbortWithStatusJSON(http.StatusForbidden, helper.ErrorResponse{
			Error: helper.ErrorDetail{
				Code:    constants.CodeTenantNotSelected,
				Message: "a tenant must be selected before accessing this endpoint; call POST /api/v1/auth/select-tenant first",
			},
		})
	}
}

// isScopeUserAllowed returns true when the given method + path is on the
// scope=user allowlist.
func isScopeUserAllowed(method, path string) bool {
	switch {
	case method == http.MethodGet && path == "/api/v1/auth/me":
		return true
	case method == http.MethodPost && path == "/api/v1/auth/select-tenant":
		return true
	case method == http.MethodPost && path == "/api/v1/auth/logout":
		return true
	case method == http.MethodGet && path == "/api/v1/auth/.well-known/jwks.json":
		return true
	case method == http.MethodPost && path == "/api/v1/auth/me/password":
		return true
	default:
		return false
	}
}
