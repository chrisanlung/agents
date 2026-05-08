// Package main is the composition root for the auth service.
// All wiring happens here: config → DB → helpers → repositories → services →
// controllers → routes → server. No business logic lives in this file.
package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	commoncfg "github.com/chrisanlung/common-configs/config"
	"github.com/chrisanlung/common-configs/database"
	"github.com/chrisanlung/common-configs/log"
	"github.com/chrisanlung/lustia-auth/internal/controller"
	"github.com/chrisanlung/lustia-auth/internal/helper"
	helperPayment "github.com/chrisanlung/lustia-auth/internal/helper/payment"
	helperStorage "github.com/chrisanlung/lustia-auth/internal/helper/storage"
	"github.com/chrisanlung/lustia-auth/internal/repository"
	"github.com/chrisanlung/lustia-auth/internal/route"
	"github.com/chrisanlung/lustia-auth/internal/service"
	"github.com/gin-gonic/gin"
)

const (
	dbConnectionID  = "primary"
	shutdownTimeout = 10 * time.Second
	cleanupInterval = 1 * time.Hour
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// -------------------------------------------------------------------------
	// Config + Logger
	// -------------------------------------------------------------------------
	commoncfg.Init()
	cfg := commoncfg.Instance
	log.Init(ctx, cfg, "")

	// -------------------------------------------------------------------------
	// Database
	// -------------------------------------------------------------------------
	database.Init(ctx, cfg)
	gormDB, _, err := database.Manager.DB(ctx, dbConnectionID)
	if err != nil {
		log.Fatal(ctx, err, "failed to get DB connection")
	}

	// -------------------------------------------------------------------------
	// Helpers (infrastructure implementations)
	// -------------------------------------------------------------------------
	clock := helper.NewSystemClock()
	hasher := helper.NewArgon2idHasher()

	privateKeyPath := os.Getenv("JWT_PRIVATE_KEY_PATH")
	kp, err := helper.LoadOrGenerateKeyPair(ctx, privateKeyPath)
	if err != nil {
		log.Fatal(ctx, err, "failed to load JWT key pair")
	}
	issuer := helper.NewJWTIssuer(kp)

	// Rate limiter: burst=20, refill=2 tokens/sec per key (~120 req/min).
	// TODO(phase-10): replace with Redis-backed limiter.
	rateLimiter := helper.NewMemoryRateLimiter(20, 2)

	// Email sender: SMTP for local dev (Mailpit) and transactional providers
	// (SES, SendGrid, Postmark) in staging/prod. Disabled by default — the
	// password-forgot flow becomes a no-op when Enabled=false.
	smtpCfg := helper.SMTPConfig{
		Enabled:  envBool("SMTP_ENABLED", false),
		Host:     envStr("SMTP_HOST", ""),
		Port:     envInt("SMTP_PORT", 1025),
		Username: os.Getenv("SMTP_USERNAME"),
		Password: os.Getenv("SMTP_PASSWORD"),
		From:     envStr("SMTP_FROM", "Lustia <no-reply@lustia.local>"),
		StartTLS: envBool("SMTP_STARTTLS", false),
		Timeout:  envInt("SMTP_TIMEOUT_MS", 5000),
	}
	smtpSender := helper.NewSMTPSender(smtpCfg)
	mailer := emailAdapter{smtpSender}
	if smtpSender.Enabled() {
		log.Infof(ctx, "email sender: SMTP enabled host=%s port=%d from=%q", smtpCfg.Host, smtpCfg.Port, smtpCfg.From)
	} else {
		log.Info(ctx, "email sender: disabled (SMTP_ENABLED=false or SMTP_HOST empty) — password-reset emails will not be delivered")
	}
	resetURLBase := envStr("PASSWORD_RESET_URL_BASE", "http://localhost:3000/auth/reset")
	senderName := envStr("APP_DISPLAY_NAME", "Lustia")

	// -------------------------------------------------------------------------
	// Repositories
	// -------------------------------------------------------------------------
	userRepo := repository.NewUserRepository(gormDB)
	membershipRepo := repository.NewMembershipRepository(gormDB)
	tenantRepo := repository.NewTenantRepository(gormDB)
	roleRepo := repository.NewRoleRepository(gormDB)
	refreshTokenRepo := repository.NewRefreshTokenRepository(gormDB)
	passwordResetRepo := repository.NewPasswordResetRepository(gormDB)
	auditRepo := repository.NewAuditRepository(gormDB)
	txManager := repository.NewTxManager(gormDB)
	branchRepo := repository.NewBranchRepository(gormDB)
	registrationRepo := repository.NewRegistrationRepository(gormDB)

	// Phase 4 repositories.
	therapistRepo := repository.NewTherapistRepository(gormDB)
	serviceCatalogRepo := repository.NewServiceCatalogRepository(gormDB)
	therapistServiceRepo := repository.NewTherapistServiceRepository(gormDB)
	therapistAvailabilityRepo := repository.NewTherapistAvailabilityRepository(gormDB)
	// ADR 0010 — Tenant-wide add-on catalog.
	addonRepo := repository.NewAddonRepository(gormDB)
	// ADR 0012 — Room (Ruangan) catalog.
	roomRepo := repository.NewRoomRepository(gormDB)

	// -------------------------------------------------------------------------
	// Services (ADR 0007 wiring — all three services now take MembershipRepository)
	// -------------------------------------------------------------------------
	authSvc := service.NewAuthService(
		userRepo, membershipRepo, tenantRepo, refreshTokenRepo,
		hasher, issuer, clock, auditRepo, rateLimiter, txManager,
	)
	meSvc := service.NewMeService(userRepo, membershipRepo, tenantRepo)
	passwordSvc := service.NewPasswordService(
		userRepo, passwordResetRepo, refreshTokenRepo,
		hasher, clock, auditRepo,
		mailer, txManager, resetURLBase, senderName,
	)
	userSvc := service.NewUserService(
		userRepo, membershipRepo, roleRepo, hasher, clock, auditRepo, txManager,
	)
	roleSvc := service.NewRoleService(roleRepo)
	jwksSvc := service.NewJWKSService(issuer)

	// Phase 3 services.
	// Registration-specific rate limiter: 3 req/hour per key (IP or email).
	regRateLimiter := helper.NewMemoryRateLimiter(3, float64(3)/3600)
	// Platform-wide cap (SECURITY.md Phase 3 M-4): 50/hour across all callers.
	// Pre-CAPTCHA defense against distributed IP rotation. Migrate to Redis
	// before horizontal scaling (SECURITY.md §11.7).
	regGlobalLimiter := helper.NewMemoryRateLimiter(50, float64(50)/3600)

	// TENANT_ADMIN_LOGIN_URL is baked into the welcome email sent to new tenant
	// admins. A wrong value means approved tenants receive credentials pointing
	// at a URL that does not serve the portal — high-severity misconfiguration
	// (SECURITY.md Phase 3 H-1). Fail fast in release mode; fall back to a dev
	// URL only in debug mode for local setups.
	tenantAdminLoginURL := os.Getenv("TENANT_ADMIN_LOGIN_URL")
	if tenantAdminLoginURL == "" {
		if cfg.GetObject().App.Mode == "release" {
			log.Fatal(ctx, fmt.Errorf("TENANT_ADMIN_LOGIN_URL is required in release mode"), "misconfiguration")
		}
		tenantAdminLoginURL = "http://localhost:3002/login"
		log.Info(ctx, "TENANT_ADMIN_LOGIN_URL unset — falling back to http://localhost:3002/login (debug mode only)")
	}
	registrationSvc := service.NewRegistrationService(
		registrationRepo, tenantRepo, userRepo, membershipRepo, roleRepo,
		refreshTokenRepo, hasher, clock, auditRepo, mailer, regRateLimiter,
		regGlobalLimiter, txManager, tenantAdminLoginURL, senderName,
	)
	tenantSvc := service.NewTenantService(
		tenantRepo, membershipRepo, refreshTokenRepo, auditRepo, txManager, clock,
	)
	branchSvc := service.NewBranchService(
		branchRepo, tenantRepo, userRepo, auditRepo, clock,
	)

	// Phase 4 services (ADR 0009).
	therapistSvc := service.NewTherapistSvc(
		therapistRepo, therapistServiceRepo, serviceCatalogRepo,
		branchRepo, auditRepo, clock, txManager,
	)
	catalogSvc := service.NewCatalogService(
		serviceCatalogRepo, therapistServiceRepo, therapistRepo,
		branchRepo, auditRepo, clock,
	)

	availabilitySvc := service.NewAvailabilitySvc(
		therapistAvailabilityRepo, therapistRepo, auditRepo, clock,
	)
	mappingSvc := service.NewMappingService(
		therapistRepo, serviceCatalogRepo, therapistServiceRepo, auditRepo, txManager,
	)

	// ADR 0010 — Tenant-wide add-on catalog.
	addonSvc := service.NewAddonService(addonRepo, auditRepo, clock, txManager)
	// ADR 0012 — Room (Ruangan) catalog.
	roomSvc := service.NewRoomSvc(roomRepo, branchRepo, auditRepo, clock, txManager)

	// -------------------------------------------------------------------------
	// ADR 0011 — Storage adapter (fail-fast per §2.2)
	// Dipindah ke sini agar stor tersedia sebelum NewBookingService dipanggil.
	// -------------------------------------------------------------------------
	storageDriver := envStr("STORAGE_DRIVER", "")
	if storageDriver == "" {
		log.Fatal(ctx, fmt.Errorf("STORAGE_DRIVER is required"), "set STORAGE_DRIVER to one of: local, r2, supabase")
	}

	rawLocalPath := envStr("STORAGE_LOCAL_PATH", "./storage-data/uploads")
	absLocalPath, err := filepath.Abs(rawLocalPath)
	if err != nil {
		log.Fatal(ctx, err, "cannot resolve STORAGE_LOCAL_PATH")
	}
	if rawLocalPath != absLocalPath {
		log.Infof(ctx, "STORAGE_LOCAL_PATH resolved: %q → %q", rawLocalPath, absLocalPath)
	}

	storageAdapter, err := helperStorage.NewStorageAdapter(helperStorage.Config{
		Driver:        storageDriver,
		LocalPath:     absLocalPath,
		PublicBaseURL: envStr("STORAGE_PUBLIC_BASE_URL", "http://localhost:8080/uploads"),
	})
	if err != nil {
		log.Fatal(ctx, err, "storage adapter init failed")
	}

	// The concrete adapter satisfies service.Storage (identical method set).
	// We assert this at the type boundary here; main.go is the composition root.
	stor, ok := storageAdapter.(service.Storage)
	if !ok {
		log.Fatal(ctx, fmt.Errorf("storage adapter does not implement service.Storage"), "wiring error")
	}

	// Per-tenant hourly upload quota (ADR 0011 §2.4.6).
	uploadHourlyLimit := envInt("UPLOAD_TENANT_HOURLY_LIMIT", 30)
	uploadQuota := helperStorage.NewTenantQuota(time.Hour, uploadHourlyLimit)

	// Upload size cap in bytes (default 5 MiB).
	uploadMaxMB := envInt("UPLOAD_MAX_MB", 5)
	uploadMaxBytes := int64(uploadMaxMB) * 1024 * 1024

	// localStoragePath is only passed to route.Deps when driver=local so the
	// StaticFS route is conditionally registered (ADR 0011 §2.5).
	localStoragePath := ""
	if storageDriver == "local" {
		localStoragePath = absLocalPath
	}

	// ADR 0015 — Phase 6 Payment Provider.
	// C-2 (SECURITY.md): fail-fast on misconfiguration — dummy adapter only
	// in dev/local. PAYMENT_PROVIDER replaces the old PAYMENT_ADAPTER env var.
	appEnv := envStr("APP_ENV", "dev")
	paymentProvider := envStr("PAYMENT_PROVIDER", envStr("PAYMENT_ADAPTER", "dummy"))
	rawProvider, err := helperPayment.NewProvider(helperPayment.ProviderConfig{
		Provider:                  paymentProvider,
		AppEnv:                    appEnv,
		IPaymuVA:                  os.Getenv("IPAYMU_VA"),
		IPaymuAPIKey:              os.Getenv("IPAYMU_API_KEY"),
		IPaymuBaseURL:             os.Getenv("IPAYMU_BASE_URL"),
		IPaymuNotifyURL:           os.Getenv("IPAYMU_NOTIFY_URL"),
		IPaymuSkipSignatureVerify: os.Getenv("IPAYMU_SKIP_SIGNATURE_VERIFY") == "true",
	})
	if err != nil {
		log.Fatal(ctx, err, "payment provider misconfiguration (C-2)")
	}
	log.Infof(ctx, "payment provider: %s (APP_ENV=%s)", paymentProvider, appEnv)

	// paymentProviderBridge bridges helper/payment.ProviderIface → service.PaymentProvider
	// so the service package does not import the helper package (layering rule).
	paymentProviderBridge := &paymentProviderBridge{inner: rawProvider}

	// Phase 6 repositories.
	bookingRepo := repository.NewBookingRepository(gormDB)
	paymentTxnRepo := repository.NewPaymentTransactionRepository(gormDB)
	settlementBatchRepo := repository.NewSettlementBatchRepository(gormDB)
	disbursementRepo := repository.NewTenantDisbursementRepository(gormDB)

	// Phase 6 services.
	paymentSvc := service.NewPaymentService(
		paymentTxnRepo,
		bookingRepo,
		paymentProviderBridge,
		auditRepo,
		clock,
		txManager,
	)
	settlementSvc := service.NewSettlementService(
		settlementBatchRepo,
		paymentTxnRepo,
		paymentProviderBridge,
		auditRepo,
		clock,
		txManager,
	)
	disbursementSvc := service.NewDisbursementService(
		disbursementRepo,
		paymentTxnRepo,
		auditRepo,
		clock,
		txManager,
	)

	bookingSvc := service.NewBookingService(
		bookingRepo,
		branchRepo,
		serviceCatalogRepo,
		addonRepo,
		roomRepo,
		therapistRepo,
		therapistServiceRepo,
		therapistAvailabilityRepo,
		paymentSvc,  // Phase 6: PaymentServiceIface replaces MidtransClient
		auditRepo,
		mailer,
		clock,
		txManager,
		stor,       // ADR 0011: storage for therapist/room photo URLs on public endpoints
		tenantRepo, // used by GetPublicBranchDetail to fetch tenant name
	)

	// -------------------------------------------------------------------------
	// Controllers
	// -------------------------------------------------------------------------
	authCtrl := controller.NewAuthController(authSvc, meSvc, passwordSvc)
	adminCtrl := controller.NewAdminController(userSvc, roleSvc)
	jwksCtrl := controller.NewJWKSController(jwksSvc)
	sqlDB, err := gormDB.DB()
	if err != nil {
		log.Fatal(ctx, err, "failed to access underlying sql.DB")
	}
	healthCtrl := controller.NewHealthController(sqlDB, storageAdapter)

	// Phase 3 controllers.
	registrationCtrl := controller.NewRegistrationController(registrationSvc)
	tenantCtrl := controller.NewTenantController(tenantSvc)
	branchCtrl := controller.NewBranchController(branchSvc)

	// Phase 4 controllers (ADR 0009).
	therapistCtrl := controller.NewTherapistController(therapistSvc, stor, uploadQuota, uploadMaxBytes)
	serviceCtrl := controller.NewServiceController(catalogSvc)
	availabilityCtrl := controller.NewAvailabilityController(availabilitySvc)
	therapistMappingCtrl := controller.NewTherapistMappingController(mappingSvc, therapistSvc)
	// ADR 0010 — Tenant-wide add-on catalog.
	addonCtrl := controller.NewAddonController(addonSvc)
	// ADR 0012 — Room (Ruangan) catalog.
	roomCtrl := controller.NewRoomController(roomSvc, stor, uploadQuota, uploadMaxBytes)

	// ADR 0014 — Phase 5 Booking Engine.
	// ADR 0015 — sync-payment endpoint on BookingController needs paymentSvc.
	bookingCtrl := controller.NewBookingController(bookingSvc, paymentSvc)

	// ADR 0015 — Phase 6 Payment + Settlement + Payout.
	paymentCtrl := controller.NewPaymentController(paymentSvc)
	financeCtrl := controller.NewFinanceController(paymentSvc, paymentTxnRepo, disbursementSvc)
	payoutCtrl := controller.NewPayoutController(settlementSvc, disbursementSvc)

	// -------------------------------------------------------------------------
	// Gin engine + routes
	// -------------------------------------------------------------------------
	mode := cfg.GetObject().App.Mode
	gin.SetMode(mode)
	if err := helper.RegisterCustomValidators(); err != nil {
		log.Fatal(ctx, err, "failed to register custom validators")
	}
	r := gin.New()

	var corsOrigins []string
	if raw := envStr("CORS_ALLOWED_ORIGINS", ""); raw != "" {
		for _, o := range strings.Split(raw, ",") {
			o = strings.TrimSpace(o)
			if o != "" {
				corsOrigins = append(corsOrigins, o)
			}
		}
	}
	route.Register(r, route.Deps{
		Issuer:             issuer,
		TxStarter:          txManager,
		CORSAllowedOrigins: corsOrigins,
		Auth:               authCtrl,
		Admin:              adminCtrl,
		JWKS:               jwksCtrl,
		Health:             healthCtrl,
		Registration:       registrationCtrl,
		Tenant:             tenantCtrl,
		Branch:             branchCtrl,
		// Phase 4 — Master Operational Data (ADR 0009).
		Therapist:        therapistCtrl,
		Service:          serviceCtrl,
		Availability:     availabilityCtrl,
		TherapistMapping: therapistMappingCtrl,
		// ADR 0010 — Tenant-wide add-on catalog.
		Addon: addonCtrl,
		// ADR 0012 — Room (Ruangan) catalog.
		Room: roomCtrl,
		// ADR 0014 — Phase 5 Booking Engine.
		Booking: bookingCtrl,
		// ADR 0015 — Phase 6 Payment + Settlement + Payout.
		Payment: paymentCtrl,
		Finance: financeCtrl,
		Payout:  payoutCtrl,
		// ADR 0011 — Static file serving (driver=local only; empty = skip).
		LocalStoragePath: localStoragePath,
	})

	// -------------------------------------------------------------------------
	// HTTP server
	// -------------------------------------------------------------------------
	port := fmt.Sprintf(":%d", cfg.GetObject().Transport.Server.Rest.Port.Http)
	srv := &http.Server{
		Addr:    port,
		Handler: r,
	}

	go func() {
		log.Infof(ctx, "auth-service listening on %s", port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal(ctx, err, "server error")
		}
	}()

	// -------------------------------------------------------------------------
	// Refresh token cleanup goroutine
	// -------------------------------------------------------------------------
	go func() {
		ticker := time.NewTicker(cleanupInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				log.Info(ctx, "refresh token cleanup goroutine stopped")
				return
			case <-ticker.C:
				cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 30*time.Second)
				if err := refreshTokenRepo.DeleteExpiredAndRevoked(cleanupCtx); err != nil {
					log.Errorf(cleanupCtx, err, "refresh token cleanup failed")
				} else {
					log.Info(cleanupCtx, "refresh token cleanup completed")
				}
				cleanupCancel()
			}
		}
	}()

	// -------------------------------------------------------------------------
	// Graceful shutdown
	// -------------------------------------------------------------------------
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGTERM, syscall.SIGINT)
	<-quit

	log.Info(ctx, "shutting down auth-service...")
	cancel() // stop background goroutines

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Fatal(shutdownCtx, err, "graceful shutdown failed")
	}
	log.Info(shutdownCtx, "auth-service stopped")
}

// emailAdapter bridges service.EmailMessage → helper.EmailMessage so the
// service package does not need to import helper (layering) and the helper
// does not need to import service (avoid cycle). The two structs have
// identical fields, so conversion is free.
type emailAdapter struct {
	inner *helper.SMTPSender
}

func (a emailAdapter) Send(ctx context.Context, msg service.EmailMessage) error {
	return a.inner.Send(ctx, helper.EmailMessage{
		To:       msg.To,
		Subject:  msg.Subject,
		TextBody: msg.TextBody,
		HTMLBody: msg.HTMLBody,
	})
}

// paymentProviderBridge bridges helper/payment.ProviderIface →
// service.PaymentProvider so the service package does not import the helper
// package (layering rule). Method signatures are identical; this is a
// transparent translation layer.
type paymentProviderBridge struct {
	inner helperPayment.ProviderIface
}

func (b *paymentProviderBridge) CreateQR(ctx context.Context, req service.CreateQRRequest) (service.CreateQRResponse, error) {
	resp, err := b.inner.CreateQR(ctx, helperPayment.CreateQRRequest{
		ProviderReference: req.ProviderReference,
		OrderID:           req.OrderID,
		AmountIDR:         req.AmountIDR,
		CustomerName:      req.CustomerName,
		CustomerEmail:     req.CustomerEmail,
		CustomerPhone:     req.CustomerPhone,
		Description:       req.Description,
		ExpiryMinutes:     req.ExpiryMinutes,
	})
	if err != nil {
		return service.CreateQRResponse{}, err
	}
	return service.CreateQRResponse{
		ProviderReference: resp.ProviderReference,
		QRString:          resp.QRString,
		QRImageURL:        resp.QRImageURL,
		ExpiresAt:         resp.ExpiresAt,
	}, nil
}

func (b *paymentProviderBridge) VerifyWebhook(ctx context.Context, payload []byte, headers map[string]string) (service.PaymentNotification, error) {
	notif, err := b.inner.VerifyWebhook(ctx, payload, headers)
	if err != nil {
		return service.PaymentNotification{}, err
	}
	return service.PaymentNotification{
		ProviderReference: notif.ProviderReference,
		Status:            service.ProviderPaymentStatus(notif.Status),
		ReceivedAmountIDR: notif.ReceivedAmountIDR,
		RawPayload:        notif.RawPayload,
	}, nil
}

func (b *paymentProviderBridge) GetStatus(ctx context.Context, providerReference string) (service.ProviderPaymentStatus, error) {
	status, err := b.inner.GetStatus(ctx, providerReference)
	return service.ProviderPaymentStatus(status), err
}

func (b *paymentProviderBridge) ListSettlements(ctx context.Context, date time.Time) ([]service.SettlementItem, error) {
	items, err := b.inner.ListSettlements(ctx, date)
	if err != nil {
		return nil, err
	}
	out := make([]service.SettlementItem, len(items))
	for i, item := range items {
		out[i] = service.SettlementItem{
			ProviderReference: item.ProviderReference,
			SettledAmountIDR:  item.SettledAmountIDR,
			SettledAt:         item.SettledAt,
		}
	}
	return out, nil
}

// ---------------------------------------------------------------------------
// tiny env helpers — only used in this composition root, not elsewhere.
// ---------------------------------------------------------------------------

func envStr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func envInt(key string, def int) int {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return n
}

func envBool(key string, def bool) bool {
	v := strings.ToLower(os.Getenv(key))
	switch v {
	case "":
		return def
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}
