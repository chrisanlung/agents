// Package payment provides types used by MidtransClient adapters.
// The MidtransClient interface is declared in service/interfaces.go (consumer-owned
// per SOLID-I). This file contains only the shared helper types.
package payment

// PaymentStatus is the normalised payment state returned by adapters.
type PaymentStatus string

const (
	// StatusPending means the transaction exists but is not yet settled.
	StatusPending PaymentStatus = "pending"

	// StatusPaid means the transaction is fully settled.
	StatusPaid PaymentStatus = "paid"

	// StatusFailed means the transaction failed (declined, fraud, etc.).
	StatusFailed PaymentStatus = "failed"

	// StatusExpired means the transaction expired before settlement.
	StatusExpired PaymentStatus = "expired"
)

// PaymentRequest is the minimal data needed to create a payment transaction.
type PaymentRequest struct {
	// OrderID is the booking's unique code (e.g. "B7K3-M2QF") used as the
	// Midtrans order_id. It is also stored in booking.payment_reference.
	OrderID       string
	GrossAmount   int64  // IDR, whole rupiah
	CustomerName  string
	CustomerEmail string
	CustomerPhone string
	Description   string // "Booking [service name] at [branch name]"
}

// PaymentResponse is returned by CreateTransaction.
type PaymentResponse struct {
	SnapToken   string // Midtrans Snap token (empty for dummy)
	RedirectURL string // Midtrans Snap redirect URL (empty for dummy)
	Status      PaymentStatus
}

// WebhookNotification is the normalised representation of a Midtrans
// notification body. The real adapter maps the raw JSON; the dummy adapter
// constructs it synthetically.
type WebhookNotification struct {
	OrderID           string // booking.payment_reference
	TransactionStatus string // "settlement" | "capture" | "deny" | "cancel" | "expire"
	StatusCode        string // "200" | "201" | etc.
	GrossAmount       string // String in Midtrans API — parse to int64 before comparison
	SignatureKey      string // SHA-512(order_id + status_code + gross_amount + server_key)
	PaymentType       string // "credit_card" | "bank_transfer" | etc.
}
