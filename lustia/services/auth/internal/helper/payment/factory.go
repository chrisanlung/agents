package payment

import (
	"context"
	"fmt"
	"os"
	"time"
)

// ProviderConfig carries all configuration read from environment variables in
// main.go. The factory selects the right adapter and fails fast on
// misconfiguration so problems surface at startup, not at runtime.
type ProviderConfig struct {
	// Provider selects the implementation: "dummy" (dev/local only) or "ipaymu".
	Provider string

	// AppEnv is the value of APP_ENV (e.g. "dev", "local", "staging", "prod").
	// C-2: The factory refuses to construct the dummy adapter unless AppEnv is
	// "dev" or "local". See SECURITY.md C-2.
	AppEnv string

	// iPaymu credentials — only required when Provider="ipaymu".
	IPaymuVA            string
	IPaymuAPIKey        string
	IPaymuSecret        string
	IPaymuWebhookSecret string
}

// NewProviderFromEnv constructs a ProviderConfig from standard environment
// variables. Callers may override individual fields after this call.
func NewProviderFromEnv() ProviderConfig {
	return ProviderConfig{
		Provider:            envOrDefault("PAYMENT_PROVIDER", "dummy"),
		AppEnv:              envOrDefault("APP_ENV", "dev"),
		IPaymuVA:            os.Getenv("IPAYMU_VA"),
		IPaymuAPIKey:        os.Getenv("IPAYMU_API_KEY"),
		IPaymuSecret:        os.Getenv("IPAYMU_SECRET"),
		IPaymuWebhookSecret: os.Getenv("IPAYMU_WEBHOOK_SECRET"),
	}
}

// NewProvider constructs the appropriate ProviderIface based on ProviderConfig.
//
// C-2 (SECURITY.md): If Provider="dummy" and AppEnv is not "dev" or "local",
// this function returns an error that the caller (main.go) must treat as
// log.Fatal. There is no fallback — an accidental dummy adapter in staging or
// production is a critical security hole.
//
// The caller in main.go MUST do:
//
//	provider, err := payment.NewProvider(cfg)
//	if err != nil {
//	    log.Fatal(ctx, err, "payment provider misconfiguration")
//	}
func NewProvider(cfg ProviderConfig) (ProviderIface, error) {
	switch cfg.Provider {
	case "dummy":
		return newDummyOrError(cfg.AppEnv)

	case "ipaymu":
		return NewIPaymu(
			cfg.IPaymuVA,
			cfg.IPaymuAPIKey,
			cfg.IPaymuSecret,
			cfg.IPaymuWebhookSecret,
		)

	default:
		return nil, fmt.Errorf(
			"unknown PAYMENT_PROVIDER=%q; valid values: dummy (dev/local only), ipaymu",
			cfg.Provider,
		)
	}
}

// ---------------------------------------------------------------------------
// Legacy bridge kept for backward-compatibility with Phase 5 main.go code
// that used NewClient + MidtransClientIface. Will be removed when main.go is
// fully migrated to the Phase 6 bridge.
// ---------------------------------------------------------------------------

// MidtransClientIface is kept for the Phase 5→6 transition period.
// Deprecated: use ProviderIface + NewProvider instead.
type MidtransClientIface interface {
	CreateQR(ctx context.Context, req CreateQRRequest) (CreateQRResponse, error)
	VerifyWebhook(ctx context.Context, payload []byte, headers map[string]string) (PaymentNotification, error)
	GetStatus(ctx context.Context, providerReference string) (PaymentStatus, error)
	ListSettlements(ctx context.Context, date time.Time) ([]SettlementItem, error)
}

// Config is the legacy config type kept for backward-compatibility.
// Deprecated: use ProviderConfig instead.
type Config = ProviderConfig

// NewClient is the legacy factory kept for backward-compatibility.
// Deprecated: use NewProvider instead.
func NewClient(cfg Config) (ProviderIface, error) {
	return NewProvider(cfg)
}

func envOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
