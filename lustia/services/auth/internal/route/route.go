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

	// ADR 0015 — Phase 6 Payment + Settlement.
	Payment *controller.PaymentController
	Finance *controller.FinanceController
	Payout  *controller.PayoutController

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
	// ADR 0015 — Phase 6 Payment endpoints (webhook moved to PaymentController).
	// H-2 (SECURITY.md): Public booking endpoints have per-route rate limits
	// applied BEFORE the handler. The per-route limiters enforce the concrete
	// thresholds from the security review:
	//   POST /public/bookings                       : 5 req/min per IP
	//   GET  /public/bookings/:code                 : 20 req/min per IP
	//   GET  /public/bookings/:code/payment-status  : 12 req/min per IP (ADR 0015 §2.7)
	//   GET  /public/branches/:id/availability      : 30 req/min per IP
	//   GET  /public/branches                       : 60 req/min per IP
	//   POST /public/payments/webhook               : not rate-limited (provider retries)
	if deps.Booking != nil {
		// POST /public/bookings: 5 req/min per IP.
		createBookingLimiter := middleware.RateLimit(helper.NewMemoryRateLimiter(5, float64(5)/60))

		// GET /public/bookings/:code: 20 req/min per IP.
		codeBookingLimiter := middleware.RateLimit(helper.NewMemoryRateLimiter(20, float64(20)/60))

		// GET /public/branches/:id/availability: 30 req/min per IP.
		availabilityLimiter := middleware.RateLimit(helper.NewMemoryRateLimiter(30, float64(30)/60))

		// GET /public/branches: 60 req/min per IP (read-only, cacheable).
		branchListLimiter := middleware.RateLimit(helper.NewMemoryRateLimiter(60, float64(60)/60))

		// Public group — no JWT, tenant middleware sets __platform__ default.
		publicBookingGroup := v1.Group("", tenantMW)
		publicBookingGroup.POST("/public/bookings", createBookingLimiter, deps.Booking.CreatePublic)
		publicBookingGroup.GET("/public/bookings/:code", codeBookingLimiter, deps.Booking.GetPublicByCode)
		publicBookingGroup.GET("/public/branches", branchListLimiter, deps.Booking.ListPublicBranches)
		publicBookingGroup.GET("/public/branches/:id", branchListLimiter, deps.Booking.GetPublicBranchDetail)
		publicBookingGroup.GET("/public/branches/:id/availability", availabilityLimiter, deps.Booking.GetAvailability)

		// Operator group — JWT required.
		deps.Booking.RegisterOperator(tenantGroup, rbacMW)
	}

	// ADR 0015 — Phase 6 Payment controller (webhook + polling + dev trigger).
	if deps.Payment != nil {
		// GET /public/bookings/:code/payment-status: 12 req/min per IP (ADR 0015 §2.7).
		paymentStatusLimiter := middleware.RateLimit(helper.NewMemoryRateLimiter(12, float64(12)/60))

		publicPaymentGroup := v1.Group("", tenantMW)
		// Webhook: not rate-limited (provider retries on non-200).
		publicPaymentGroup.POST("/public/payments/webhook", deps.Payment.HandleWebhook)
		// Polling: 12/min per IP.
		publicPaymentGroup.GET("/public/bookings/:code/payment-status", paymentStatusLimiter, deps.Payment.GetPaymentStatus)

		// Dev-only dummy trigger (build-tag gated; no-op in prod binary).
		registerDummyTrigger(publicPaymentGroup, deps.Payment)
	}

	// ADR 0015 — Finance (tenant-facing) + Payout (platform-admin).
	if deps.Finance != nil {
		deps.Finance.Register(tenantGroup, rbacMW)
	}
	if deps.Payout != nil {
		deps.Payout.Register(tenantGroup, rbacMW)
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
