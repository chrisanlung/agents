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

// CreateTransaction creates a Midtrans Snap transaction.
// STUB — returns ErrNotImplemented until real integration ships.
func (r *RealMidtransClient) CreateTransaction(_ context.Context, _ PaymentRequest) (PaymentResponse, error) {
	return PaymentResponse{}, errors.New("midtrans adapter not yet implemented")
}

// HandleNotification processes a Midtrans webhook notification.
//
// H-3 (SECURITY.md): When implemented, this method MUST:
//  1. Verify the SHA-512 signature using subtle.ConstantTimeCompare before
//     any DB read or state transition:
//
//     expected := sha512.Sum512([]byte(
//         notification.OrderID + notification.StatusCode +
//         notification.GrossAmount + r.serverKey,
//     ))
//     if !subtle.ConstantTimeCompare(
//         []byte(hex.EncodeToString(expected[:])),
//         []byte(notification.SignatureKey),
//     ) {
//         return StatusFailed, ErrWebhookSignatureInvalid
//     }
//
//  2. Only proceed to status mapping after signature passes.
//  3. Return StatusPaid only for "settlement" or "capture" transaction status.
//
// STUB — returns ErrNotImplemented until real integration ships.
func (r *RealMidtransClient) HandleNotification(_ context.Context, _ WebhookNotification) (PaymentStatus, error) {
	return StatusFailed, errors.New("midtrans adapter not yet implemented")
}

// VerifySignature verifies the Midtrans SHA-512 webhook signature.
// Exposed for testing and for the real HandleNotification implementation.
//
// Signature formula (Midtrans docs):
//
//	SHA-512(order_id + status_code + gross_amount + server_key)
//
// Uses subtle.ConstantTimeCompare to prevent timing-oracle attacks (H-3).
func (r *RealMidtransClient) VerifySignature(n WebhookNotification) bool {
	raw := n.OrderID + n.StatusCode + n.GrossAmount + r.serverKey
	computed := sha512.Sum512([]byte(raw))
	expected := hex.EncodeToString(computed[:])
	return subtle.ConstantTimeCompare(
		[]byte(strings.ToLower(expected)),
		[]byte(strings.ToLower(n.SignatureKey)),
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
