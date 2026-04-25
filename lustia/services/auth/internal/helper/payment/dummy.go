//go:build dev || local

// DummyMidtransClient is only compiled when the build tag "dev" or "local" is set.
// Belt-and-suspenders: the factory.go runtime guard also prevents this adapter
// from being selected outside dev/local APP_ENV (C-2). See SECURITY.md C-2.
package payment

import (
	"context"
	"errors"
	"fmt"
)

// DummyMidtransClient is a stub payment adapter that always returns "paid".
// It accepts any payload without signature verification.
//
// SECURITY (C-2): This adapter must NEVER run in non-dev/local environments.
// Two layers of protection:
//  1. Build tag "dev" or "local" — this file is excluded from production binaries.
//  2. Runtime factory guard in factory.go — log.Fatal if APP_ENV is not dev/local.
type DummyMidtransClient struct{}

// NewDummy constructs a DummyMidtransClient.
// env must be "dev" or "local"; any other value returns an error.
func NewDummy(env string) (*DummyMidtransClient, error) {
	if env != "dev" && env != "local" {
		return nil, fmt.Errorf(
			"DummyMidtransClient refused to construct: APP_ENV=%q is not dev or local; "+
				"this adapter is forbidden in non-development environments (SECURITY.md C-2)",
			env,
		)
	}
	return &DummyMidtransClient{}, nil
}

// CreateTransaction returns a synthetic paid response immediately.
// No real HTTP call is made. This is the Phase 5 dummy flow — the mobile
// app shows a "Bayar (DUMMY)" button and the response signals instant payment.
func (d *DummyMidtransClient) CreateTransaction(_ context.Context, req PaymentRequest) (PaymentResponse, error) {
	return PaymentResponse{
		SnapToken:   "dummy-snap-token-" + req.OrderID,
		RedirectURL: "http://localhost:8080/dummy-payment?order=" + req.OrderID,
		Status:      StatusPaid,
	}, nil
}

// HandleNotification accepts any payload and returns StatusPaid.
// Signature verification is deliberately skipped for the dummy adapter —
// the build + runtime guards (C-2) prevent this from reaching production.
func (d *DummyMidtransClient) HandleNotification(_ context.Context, n WebhookNotification) (PaymentStatus, error) {
	if n.OrderID == "" {
		return StatusFailed, errors.New("dummy: missing order_id in notification")
	}
	return StatusPaid, nil
}
