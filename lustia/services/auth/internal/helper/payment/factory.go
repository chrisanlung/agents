package payment

import (
	"context"
	"fmt"
)

// MidtransClientIface is a local interface alias so factory.go can return a
// typed value. The canonical interface lives in service/interfaces.go
// (consumer-owned per SOLID-I); this alias avoids an import cycle.
// Both DummyMidtransClient and RealMidtransClient satisfy it.
type MidtransClientIface interface {
	CreateTransaction(ctx context.Context, req PaymentRequest) (PaymentResponse, error)
	HandleNotification(ctx context.Context, n WebhookNotification) (PaymentStatus, error)
}

// Config carries the payment adapter configuration read from environment
// variables in main.go.
type Config struct {
	// Adapter selects the implementation: "dummy" or "midtrans".
	Adapter string

	// AppEnv is the value of APP_ENV (e.g. "dev", "local", "staging", "prod").
	// C-2: The factory refuses to construct the dummy adapter unless AppEnv is
	// "dev" or "local". See SECURITY.md C-2.
	AppEnv string

	// Midtrans credentials — only required when Adapter="midtrans".
	MidtransServerKey   string
	MidtransClientKey   string
	MidtransEnvironment string // "sandbox" | "production"
}

// NewClient constructs the appropriate MidtransClientIface based on Config.
//
// C-2 (SECURITY.md): If Adapter="dummy" and AppEnv is not "dev" or "local",
// this function returns an error that the caller (main.go) must treat as
// log.Fatal. There is no fallback — an accidental dummy adapter in staging
// or production is a critical security hole (free paid bookings for attackers).
//
// The caller in main.go MUST do:
//
//	client, err := payment.NewClient(cfg)
//	if err != nil {
//	    log.Fatal(ctx, err, "payment adapter misconfiguration")
//	}
func NewClient(cfg Config) (MidtransClientIface, error) {
	switch cfg.Adapter {
	case "dummy":
		return newDummyOrError(cfg.AppEnv)

	case "midtrans":
		return NewRealMidtrans(
			cfg.MidtransServerKey,
			cfg.MidtransClientKey,
			cfg.MidtransEnvironment,
		)

	default:
		return nil, fmt.Errorf(
			"unknown PAYMENT_ADAPTER=%q; valid values: dummy (dev/local only), midtrans",
			cfg.Adapter,
		)
	}
}
