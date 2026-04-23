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

	Auth         *controller.AuthController
	Admin        *controller.AdminController
	JWKS         *controller.JWKSController
	Health       *controller.HealthController
	Registration *controller.RegistrationController
	Tenant       *controller.TenantController
	Branch       *controller.BranchController

	// Phase 4 — Master Operational Data (ADR 0009).
	Therapist        *controller.TherapistController
	Service          *controller.ServiceController
	Availability     *controller.AvailabilityController
	TherapistMapping *controller.TherapistMappingController
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
	// Global limiter: 60 req/s burst, 1 req/s refill base (shared across all routes).
	limiter := helper.NewMemoryRateLimiter(60, 1)
	rateLimitMW := middleware.RateLimit(limiter)

	// Registration-specific limiter: 3 requests per hour per key (IP or email).
	// Burst=3, refill= 3/3600 ≈ 0.00083 tokens/sec (effectively 3/hour).
	regLimiter := helper.NewMemoryRateLimiter(3, float64(3)/3600)
	regRateLimitMW := middleware.RateLimit(regLimiter)

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

	// Public registration (no JWT). The registration-specific rate limiter is
	// applied at the group level inside the controller.
	// tenantMW opens a transaction and sets the __platform__ RLS context so
	// the INSERT into tenant_registration is permitted.
	publicReg := v1.Group("", tenantMW)
	deps.Registration.RegisterPublic(publicReg, regRateLimitMW)

	// Admin endpoints: JWT → tenant (reads claims) → scope gate → password-change
	// gate → RBAC per route. scopeGateMW blocks scope=user callers on anything
	// that isn't in the allowlist (see middleware/scope_gate.go).
	adminGroup := v1.Group("", jwtMW, tenantMW, scopeGateMW, pwdChangeMW)
	deps.Admin.Register(adminGroup, jwtMW, rbacMW)
	deps.Registration.RegisterAdmin(adminGroup, rbacMW)
	deps.Tenant.Register(adminGroup, rbacMW)

	// Tenant branch routes: JWT → tenant context → scope gate → password-change
	// gate → RBAC per route.
	// scope=tenant is required (enforced implicitly: RBAC checks branch.* which
	// only tenant-scoped tokens carry; scope=platform or scope=user won't have
	// those permissions).
	tenantGroup := v1.Group("", jwtMW, tenantMW, scopeGateMW, pwdChangeMW)
	deps.Branch.Register(tenantGroup, rbacMW)

	// Phase 4 — Master Operational Data (ADR 0009).
	// All Phase 4 endpoints share the same tenant-scoped group with RBAC per route.
	deps.Therapist.Register(tenantGroup, rbacMW)
	deps.Service.Register(tenantGroup, rbacMW)
	deps.Availability.Register(tenantGroup, rbacMW)
	deps.TherapistMapping.Register(tenantGroup, rbacMW)
}
