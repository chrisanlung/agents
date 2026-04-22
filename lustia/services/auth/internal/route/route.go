// Package route wires all controllers and middleware onto a *gin.Engine.
// It is the only package allowed to know about all controllers simultaneously.
package route

import (
	"github.com/chrisanlung/lustia-auth/internal/controller"
	"github.com/chrisanlung/lustia-auth/internal/helper"
	"github.com/chrisanlung/lustia-auth/internal/middleware"
	"github.com/gin-gonic/gin"
)

// Deps carries every controller and cross-cutting dependency needed to build
// the full route tree. Constructed in main.go and passed here.
type Deps struct {
	Issuer             helper.TokenIssuer
	TxStarter          middleware.RequestTxStarter
	CORSAllowedOrigins []string // exact origins; empty = CORS off

	Auth   *controller.AuthController
	Admin  *controller.AdminController
	JWKS   *controller.JWKSController
	Health *controller.HealthController
}

// Register builds the full Gin route tree on r.
// Middleware ordering: Recover → RequestID → (per-group auth/rbac).
// The tenant-context middleware (SET LOCAL RLS) runs inside secured groups.
func Register(r *gin.Engine, deps Deps) {
	// --- Global middleware ---
	r.Use(middleware.Recover())
	r.Use(middleware.RequestID())
	if len(deps.CORSAllowedOrigins) > 0 {
		r.Use(middleware.CORS(deps.CORSAllowedOrigins))
	}

	// --- Rate limiter middleware ---
	limiter := helper.NewMemoryRateLimiter(60, 1) // 60 req/s burst, 1 req/s refill base
	rateLimitMW := middleware.RateLimit(limiter)

	// --- Shared middleware factories ---
	jwtMW := middleware.JWTVerify(deps.Issuer)
	tenantMW := middleware.Tenant(deps.TxStarter)
	rbacMW := middleware.RequirePermission
	pwdChangeMW := middleware.PasswordChangeRequired()
	scopeGateMW := middleware.ScopeGate()

	// --- Health probes (no auth, no rate limit) ---
	deps.Health.Register(r)

	// --- JWKS discovery (no auth, light rate limit) ---
	wellKnown := r.Group("", rateLimitMW)
	deps.JWKS.Register(wellKnown)

	// --- API v1 ---
	v1 := r.Group("/api/v1", rateLimitMW)

	// Auth endpoints: controller registers public + secured sub-groups with
	// middleware applied in the right order per subgroup.
	// Public:  tenantMW (opens tx + default sentinel) → handler
	// Secured: jwtMW → tenantMW (reads claims) → scopeGateMW → pwdChangeMW → handler
	deps.Auth.Register(v1, tenantMW, jwtMW, scopeGateMW, pwdChangeMW)

	// Admin endpoints: JWT → tenant (reads claims) → scope gate → password-change
	// gate → RBAC per route. scopeGateMW blocks scope=user callers on anything
	// that isn't in the allowlist (see middleware/scope_gate.go).
	adminGroup := v1.Group("", jwtMW, tenantMW, scopeGateMW, pwdChangeMW)
	deps.Admin.Register(adminGroup, jwtMW, rbacMW)
}
