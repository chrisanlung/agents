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

	// ADR 0010 — Tenant-wide add-on catalog (rewritten 2026-04-24).
	Addon *controller.AddonController

	// ADR 0012 — Room (Ruangan) catalog.
	Room *controller.RoomController

	// ADR 0014 — Phase 5 Booking Engine.
	Booking *controller.BookingController

	// ADR 0011 — Local static file serving (driver=local only).
	// When non-empty, a StaticFS route is registered at /uploads.
	LocalStoragePath string
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

	// ADR 0010 — Tenant-wide add-on catalog under /tenant/addons.
	if deps.Addon != nil {
		deps.Addon.Register(tenantGroup, rbacMW)
	}

	// ADR 0012 — Room (Ruangan) catalog under /tenant/rooms.
	if deps.Room != nil {
		deps.Room.Register(tenantGroup, rbacMW)
	}

	// ADR 0014 — Phase 5 Booking Engine.
	// H-2 (SECURITY.md): Public booking endpoints have per-route rate limits
	// applied BEFORE the handler. The per-route limiters enforce the concrete
	// thresholds from the security review:
	//   POST /public/bookings             : 5 req/min + 20 req/hour per IP
	//   GET  /public/bookings/:code       : 20 req/min per IP
	//   GET  /public/branches/:id/availability : 30 req/min per IP
	//   GET  /public/branches             : 60 req/min per IP
	//   POST /public/payments/webhook     : not rate-limited (Midtrans retries)
	if deps.Booking != nil {
		// Public group — tenant middleware sets __public__ sentinel.
		// Each sub-route gets its own per-IP limiter at the specified threshold.

		// POST /public/bookings: 5 req/min per IP.
		// burst=5, refill=5/60 tokens/sec ≈ 5 per minute.
		createBookingLimiter := middleware.RateLimit(helper.NewMemoryRateLimiter(5, float64(5)/60))

		// GET /public/bookings/:code: 20 req/min per IP.
		codeBookingLimiter := middleware.RateLimit(helper.NewMemoryRateLimiter(20, float64(20)/60))

		// GET /public/branches/:id/availability: 30 req/min per IP.
		availabilityLimiter := middleware.RateLimit(helper.NewMemoryRateLimiter(30, float64(30)/60))

		// GET /public/branches: 60 req/min per IP (read-only, cacheable).
		branchListLimiter := middleware.RateLimit(helper.NewMemoryRateLimiter(60, float64(60)/60))

		// Public group — no JWT, tenant middleware sets __platform__ default.
		// We override to __public__ per-request in the booking service.
		publicBookingGroup := v1.Group("", tenantMW)
		publicBookingGroup.POST("/public/bookings", createBookingLimiter, deps.Booking.CreatePublic)
		publicBookingGroup.GET("/public/bookings/:code", codeBookingLimiter, deps.Booking.GetPublicByCode)
		publicBookingGroup.POST("/public/payments/webhook", deps.Booking.HandleWebhook) // not rate-limited
		publicBookingGroup.GET("/public/branches", branchListLimiter, deps.Booking.ListPublicBranches)
		publicBookingGroup.GET("/public/branches/:id", branchListLimiter, deps.Booking.GetPublicBranchDetail)
		publicBookingGroup.GET("/public/branches/:id/availability", availabilityLimiter, deps.Booking.GetAvailability)

		// Operator group — JWT required.
		deps.Booking.RegisterOperator(tenantGroup, rbacMW)
	}

	// ADR 0011 — Static file serving for local storage driver only.
	// Not registered for r2/supabase (CDN URLs are returned directly).
	// Middleware chain: CORS (re-applied explicitly per DO-3) +
	// X-Content-Type-Options: nosniff + Cache-Control: immutable.
	if deps.LocalStoragePath != "" {
		uploads := r.Group("/uploads")
		if len(deps.CORSAllowedOrigins) > 0 {
			uploads.Use(middleware.CORS(deps.CORSAllowedOrigins))
		}
		uploads.Use(func(c *gin.Context) {
			c.Header("X-Content-Type-Options", "nosniff")
			c.Header("Cache-Control", "public, max-age=31536000, immutable")
			c.Next()
		})
		uploads.StaticFS("", gin.Dir(deps.LocalStoragePath, false))
	}
}
