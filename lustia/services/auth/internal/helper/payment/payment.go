// Package payment provides the shared types used by payment provider adapters
// (iPaymu, dummy). The PaymentProvider interface is declared in
// service/interfaces.go (consumer-owned per SOLID-I). This file holds only the
// transfer types that are exchanged between the adapter package and the service
// package via the bridge in main.go, so neither side imports the other.
package payment

import (
	"context"
	"time"
)

// ---------------------------------------------------------------------------
// Shared transfer types
// ---------------------------------------------------------------------------

// CreateQRRequest carries data to create a payment transaction.
// Despite the legacy name, this request now supports both QRIS and VA channels
// via the Channel field (migration 000037).
type CreateQRRequest struct {
	// ProviderReference is a caller-assigned unique ID (typically UUID).
	// iPaymu uses this as the external_id / reference.
	ProviderReference string

	// OrderID is booking.code (e.g. "AB12-CD34") — human-readable order ref.
	OrderID string

	AmountIDR     int64
	CustomerName  string
	CustomerEmail string
	CustomerPhone string
	Description   string

	// ExpiryMinutes is the payment validity window; ADR 0015 §2.7 hard caps at 15.
	ExpiryMinutes int

	// Channel selects payment method: "qris" (default) or "va_bca", "va_mandiri",
	// "va_bni", "va_bri", "va_permata", "va_cimb". The adapter maps the channel
	// to the right provider paymentMethod/paymentChannel. Empty string is
	// treated as "qris" for backwards compatibility with pre-037 callers.
	Channel string
}

// CreateQRResponse is returned by CreateQR.
// For QRIS, QRString/QRImageURL are populated; for VA, VANumber/VABank.
type CreateQRResponse struct {
	// ProviderReference is the provider-assigned transaction identifier.
	ProviderReference string

	// Channel echoes the request channel (or defaults to "qris" if unspecified).
	Channel string

	// QRString is the raw QRIS string rendered by qr_flutter on the client.
	// Set when Channel = "qris"; empty for VA.
	QRString string

	// QRImageURL is an optional provider-hosted QR image URL. May be empty.
	QRImageURL string

	// VANumber is the Virtual Account number returned by the provider.
	// Set when Channel starts with "va_"; empty for QRIS.
	VANumber string

	// VABank is the bank code (bca / mandiri / bni / bri / permata / cimb).
	// Set when Channel starts with "va_"; empty for QRIS.
	VABank string

	// ExpiresAt is the absolute expiry time for this payment.
	ExpiresAt time.Time
}

// PaymentNotification is the normalised webhook payload. Each adapter's
// VerifyWebhook maps raw bytes + headers → this struct.
type PaymentNotification struct {
	// ProviderReference is the provider's transaction ID.
	ProviderReference string

	// Status is the normalised payment state.
	Status PaymentStatus

	// ReceivedAmountIDR is the amount the provider confirmed received.
	ReceivedAmountIDR int64

	// RawPayload is the original webhook body stored verbatim for audit.
	RawPayload []byte
}

// SettlementItem represents one transaction in a provider daily settlement report.
type SettlementItem struct {
	// ProviderReference is the provider's transaction ID.
	ProviderReference string

	// SettledAmountIDR is the amount settled into Lustia's bank account.
	SettledAmountIDR int64

	// SettledAt is the provider-reported settlement timestamp.
	SettledAt time.Time
}

// PaymentStatus is the normalised payment state returned by adapters.
type PaymentStatus string

const (
	// StatusPending means the transaction exists but is not yet confirmed.
	StatusPending PaymentStatus = "pending"

	// StatusPaid means the customer has paid (confirmed by provider webhook).
	StatusPaid PaymentStatus = "paid"

	// StatusFailed means the transaction failed (declined, fraud, etc.).
	StatusFailed PaymentStatus = "failed"

	// StatusExpired means the transaction expired before payment.
	StatusExpired PaymentStatus = "expired"
)

// ---------------------------------------------------------------------------
// Package-internal interface used by factory.go
// (The canonical PaymentProvider lives in service/interfaces.go)
// ---------------------------------------------------------------------------

// ProviderIface is the internal interface all adapters must satisfy.
// It mirrors service.PaymentProvider exactly so that the bridge in main.go
// can assert compatibility at compile time.
type ProviderIface interface {
	CreateQR(ctx context.Context, req CreateQRRequest) (CreateQRResponse, error)
	VerifyWebhook(ctx context.Context, payload []byte, headers map[string]string) (PaymentNotification, error)
	GetStatus(ctx context.Context, providerReference string) (PaymentStatus, error)
	ListSettlements(ctx context.Context, date time.Time) ([]SettlementItem, error)
}
