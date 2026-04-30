package payment

import (
	"context"
	"crypto/sha512"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// RealMidtransClient is a STUB for the real Midtrans Snap adapter.
// All methods return "not yet implemented" until Midtrans API keys are
// provisioned (ADR 0014 §3.3). The interface is satisfied so the factory
// compiles and type-checks against service.MidtransClient.
//
// When keys arrive, implement CreateTransaction by calling the Midtrans
// Snap API, and implement HandleNotification with the signature verification
// documented below (H-3 requirement from SECURITY.md).
type RealMidtransClient struct {
	serverKey   string
	clientKey   string
	environment string // "sandbox" | "production"
}

// NewRealMidtrans constructs the real Midtrans adapter.
// serverKey and clientKey are loaded from the secret manager (never from .env
// in production — see OPERATIONS.md §8 and SECURITY.md Phase 5 checklist).
func NewRealMidtrans(serverKey, clientKey, environment string) (*RealMidtransClient, error) {
	if serverKey == "" {
		return nil, errors.New("midtrans: server key is required")
	}
	if clientKey == "" {
		return nil, errors.New("midtrans: client key is required")
	}
	return &RealMidtransClient{
		serverKey:   serverKey,
		clientKey:   clientKey,
		environment: environment,
	}, nil
}

// CreateQR is a stub — RealMidtransClient is superseded by IPaymuProvider in
// Phase 6 (ADR 0015). Kept in the codebase for historical reference only.
// Returns an error on any call.
func (r *RealMidtransClient) CreateQR(_ context.Context, _ CreateQRRequest) (CreateQRResponse, error) {
	return CreateQRResponse{}, errors.New("midtrans adapter superseded by ipaymu in Phase 6 (ADR 0015)")
}

// VerifyWebhook is a stub — see CreateQR comment.
func (r *RealMidtransClient) VerifyWebhook(_ context.Context, _ []byte, _ map[string]string) (PaymentNotification, error) {
	return PaymentNotification{}, errors.New("midtrans adapter superseded by ipaymu in Phase 6 (ADR 0015)")
}

// GetStatus is a stub — see CreateQR comment.
func (r *RealMidtransClient) GetStatus(_ context.Context, _ string) (PaymentStatus, error) {
	return StatusFailed, errors.New("midtrans adapter superseded by ipaymu in Phase 6 (ADR 0015)")
}

// ListSettlements is a stub — see CreateQR comment.
func (r *RealMidtransClient) ListSettlements(_ context.Context, _ time.Time) ([]SettlementItem, error) {
	return nil, errors.New("midtrans adapter superseded by ipaymu in Phase 6 (ADR 0015)")
}

// VerifySignature verifies the Midtrans SHA-512 webhook signature.
// Kept for reference; not used in Phase 6 (iPaymu uses HMAC-SHA256).
//
// Signature formula (Midtrans docs):
//
//	SHA-512(order_id + status_code + gross_amount + server_key)
//
// Uses subtle.ConstantTimeCompare to prevent timing-oracle attacks (H-3).
func (r *RealMidtransClient) VerifySignature(orderID, statusCode, grossAmount, signatureKey string) bool {
	raw := orderID + statusCode + grossAmount + r.serverKey
	computed := sha512.Sum512([]byte(raw))
	expected := hex.EncodeToString(computed[:])
	return subtle.ConstantTimeCompare(
		[]byte(strings.ToLower(expected)),
		[]byte(strings.ToLower(signatureKey)),
	) == 1
}

// ParseGrossAmount parses the Midtrans gross_amount string to int64 IDR.
// Midtrans sends gross_amount as a string with possible decimal places
// (e.g. "150000.00") — we truncate to int64 (whole Rupiah).
func ParseGrossAmount(s string) (int64, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, errors.New("gross_amount is empty")
	}
	// Handle decimal format: "150000.00" → parse as float64 then convert.
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, fmt.Errorf("parse gross_amount %q: %w", s, err)
	}
	return int64(f), nil
}

// ErrWebhookSignatureInvalid is returned by HandleNotification when the
// Midtrans SHA-512 signature does not match the expected value.
var ErrWebhookSignatureInvalid = errors.New("midtrans webhook signature invalid")
