package payment

import (
	"context"
	"errors"
	"time"
)

// IPaymuProvider is the iPaymu payment adapter (STUB — Phase 6).
//
// iPaymu is a QRIS-based Indonesian payment aggregator chosen per ADR 0015.
// This stub satisfies the ProviderIface interface so the binary compiles and
// type-checks. All methods return a clear error directing developers to use
// the dummy adapter until real iPaymu credentials arrive.
//
// When credentials are provisioned:
//  1. Implement CreateQR: POST to https://my.ipaymu.com/payment/direct with
//     HMAC-SHA256 signature (method + path + body-sha256-hex + api_key).
//  2. Implement VerifyWebhook: verify HMAC-SHA256 of payload using
//     IPAYMU_WEBHOOK_SECRET before any DB access.
//  3. Implement GetStatus: GET to https://my.ipaymu.com/payment/status.
//  4. Implement ListSettlements: GET settlement report for the given date.
//
// Credentials read from environment (fail-fast in factory.go):
//   IPAYMU_VA              — Virtual Account number (from iPaymu dashboard)
//   IPAYMU_API_KEY         — API key (from iPaymu dashboard)
//   IPAYMU_SECRET          — API secret for HMAC-SHA256 signatures
//   IPAYMU_WEBHOOK_SECRET  — Separate secret for inbound webhook verification
type IPaymuProvider struct {
	va            string
	apiKey        string
	secret        string
	webhookSecret string
}

// errNotImplemented is the sentinel returned by all stub methods.
var errIPaymuNotImplemented = errors.New(
	"ipaymu adapter not yet implemented — set PAYMENT_PROVIDER=dummy for local development",
)

// NewIPaymu constructs an IPaymuProvider.
// All four credentials are required; an empty value is a configuration error.
func NewIPaymu(va, apiKey, secret, webhookSecret string) (*IPaymuProvider, error) {
	if va == "" {
		return nil, errors.New("ipaymu: IPAYMU_VA is required")
	}
	if apiKey == "" {
		return nil, errors.New("ipaymu: IPAYMU_API_KEY is required")
	}
	if secret == "" {
		return nil, errors.New("ipaymu: IPAYMU_SECRET is required")
	}
	if webhookSecret == "" {
		return nil, errors.New("ipaymu: IPAYMU_WEBHOOK_SECRET is required")
	}
	return &IPaymuProvider{
		va:            va,
		apiKey:        apiKey,
		secret:        secret,
		webhookSecret: webhookSecret,
	}, nil
}

// CreateQR initiates a QRIS transaction via iPaymu. STUB.
func (p *IPaymuProvider) CreateQR(_ context.Context, _ CreateQRRequest) (CreateQRResponse, error) {
	return CreateQRResponse{}, errIPaymuNotImplemented
}

// VerifyWebhook validates the iPaymu webhook signature + parses payload. STUB.
func (p *IPaymuProvider) VerifyWebhook(_ context.Context, _ []byte, _ map[string]string) (PaymentNotification, error) {
	return PaymentNotification{}, errIPaymuNotImplemented
}

// GetStatus polls iPaymu for the current transaction state. STUB.
func (p *IPaymuProvider) GetStatus(_ context.Context, _ string) (PaymentStatus, error) {
	return StatusFailed, errIPaymuNotImplemented
}

// ListSettlements fetches the iPaymu daily settlement report. STUB.
func (p *IPaymuProvider) ListSettlements(_ context.Context, _ time.Time) ([]SettlementItem, error) {
	return nil, errIPaymuNotImplemented
}
