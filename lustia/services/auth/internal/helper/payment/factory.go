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
	//
	// IPaymuVA is the Virtual Account number from the iPaymu dashboard.
	// It is used both as the `va` header on outbound requests and as the HMAC
	// key for inbound webhook signature verification.
	//
	// IPaymuAPIKey is the API key from the iPaymu dashboard.
	// It is the HMAC signing key for outbound request signatures.
	//
	// IPaymuBaseURL is the iPaymu API base URL.
	// Defaults to https://sandbox.ipaymu.com if empty.
	// Set to https://my.ipaymu.com for production (when prod credentials arrive).
	//
	// IPaymuNotifyURL is the publicly reachable webhook URL that iPaymu will
	// POST payment notifications to. In sandbox: your ngrok HTTPS URL.
	// No default — the factory returns an error if this is empty when
	// Provider="ipaymu", so misconfiguration fails fast at startup.
	IPaymuVA        string
	IPaymuAPIKey    string
	IPaymuBaseURL   string
	IPaymuNotifyURL string

	// IPaymuSkipSignatureVerify, when true, accepts inbound webhooks without
	// verifying the X-Signature header. Use ONLY for sandbox dev: empirical
	// evidence shows the iPaymu sandbox callback simulator emits a static
	// X-Signature placeholder that does not depend on body content
	// (identical signature across submits with different timestamps and
	// external IDs). The signature header is still parsed and logged for
	// audit. NEVER set this in production — production traffic carries a
	// real HMAC.
	IPaymuSkipSignatureVerify bool
}

// NewProviderFromEnv constructs a ProviderConfig from standard environment
// variables. Callers may override individual fields after this call.
func NewProviderFromEnv() ProviderConfig {
	return ProviderConfig{
		Provider:                  envOrDefault("PAYMENT_PROVIDER", "dummy"),
		AppEnv:                    envOrDefault("APP_ENV", "dev"),
		IPaymuVA:                  os.Getenv("IPAYMU_VA"),
		IPaymuAPIKey:              os.Getenv("IPAYMU_API_KEY"),
		IPaymuBaseURL:             os.Getenv("IPAYMU_BASE_URL"),
		IPaymuNotifyURL:           os.Getenv("IPAYMU_NOTIFY_URL"),
		IPaymuSkipSignatureVerify: os.Getenv("IPAYMU_SKIP_SIGNATURE_VERIFY") == "true",
	}
}

// NewProvider constructs the appropriate ProviderIface based on ProviderConfig.
//
// C-2 (SECURITY.md): If Provider="dummy" and AppEnv is not "dev" or "local",
// this function returns an error that the caller (main.go) must treat as
// log.Fatal. There is no fallback — an accidental dummy adapter in staging or
// production is a critical security hole.
//
// For Provider="ipaymu": fails fast if IPAYMU_VA, IPAYMU_API_KEY, or
// IPAYMU_NOTIFY_URL are empty. IPAYMU_BASE_URL defaults to sandbox.
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
			cfg.IPaymuBaseURL,
			cfg.IPaymuNotifyURL,
			cfg.IPaymuSkipSignatureVerify,
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
