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
	"strconv"
	"strings"
	"syscall"
	"time"

	commoncfg "github.com/chrisanlung/common-configs/config"
	"github.com/chrisanlung/common-configs/database"
	"github.com/chrisanlung/common-configs/log"
	"github.com/chrisanlung/lustia-auth/internal/controller"
	"github.com/chrisanlung/lustia-auth/internal/helper"
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
	healthCtrl := controller.NewHealthController(sqlDB)

	// -------------------------------------------------------------------------
	// Gin engine + routes
	// -------------------------------------------------------------------------
	mode := cfg.GetObject().App.Mode
	gin.SetMode(mode)
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
