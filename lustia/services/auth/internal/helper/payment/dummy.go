//go:build dev || local

// DummyProvider is only compiled when the build tag "dev" or "local" is set.
// Belt-and-suspenders: the factory.go runtime guard also prevents this adapter
// from being selected outside dev/local APP_ENV (C-2). See SECURITY.md C-2.
package payment

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// DummyProvider is a stub payment adapter for local development.
//
// Key Phase 6 behaviour (ADR 0015 §2.7 and implementation brief):
//   - CreateQR returns a fake QRIS string and a QR-server image URL.
//   - CreateQR does NOT auto-mark paid — the booking stays pending_payment.
//   - Status only changes when the dummy-trigger endpoint calls VerifyWebhook
//     with a synthetic payload via PaymentService.HandleWebhook.
//
// SECURITY (C-2): This adapter MUST NEVER run in non-dev/local environments.
// Two layers of protection:
//  1. Build tag "dev" or "local" — excluded from production binaries.
//  2. Runtime factory guard — log.Fatal if APP_ENV is not dev/local.
type DummyProvider struct{}

// NewDummy constructs a DummyProvider.
// env must be "dev" or "local"; any other value returns an error.
func NewDummy(env string) (*DummyProvider, error) {
	if env != "dev" && env != "local" {
		return nil, fmt.Errorf(
			"DummyProvider refused to construct: APP_ENV=%q is not dev or local; "+
				"this adapter is forbidden in non-development environments (SECURITY.md C-2)",
			env,
		)
	}
	return &DummyProvider{}, nil
}

// CreateQR returns a synthetic QRIS string and a placeholder QR image URL.
// Phase 6: does NOT mark paid. The booking remains pending_payment until the
// dummy-trigger endpoint posts a synthetic webhook notification.
func (d *DummyProvider) CreateQR(_ context.Context, req CreateQRRequest) (CreateQRResponse, error) {
	if req.ProviderReference == "" {
		return CreateQRResponse{}, errors.New("dummy: provider_reference is required")
	}
	// Minimal QRIS payload that satisfies scanner format (starts with "000201").
	// Contains the provider reference so the trigger endpoint can match it back.
	qrString := fmt.Sprintf("00020101021226570014COM.DUMMY.WWW011893600914%s0303UMI5204581253033605802ID5925LUSTIA DEV DUMMY PAYMENT6007JAKARTA6304ABCD",
		req.ProviderReference)

	// QR server URL (Google Charts API fallback — dev only, never production).
	qrImageURL := fmt.Sprintf("https://api.qrserver.com/v1/create-qr-code/?size=300x300&data=%s", req.ProviderReference)

	expiresAt := time.Now().Add(time.Duration(req.ExpiryMinutes) * time.Minute)
	if req.ExpiryMinutes <= 0 {
		expiresAt = time.Now().Add(15 * time.Minute)
	}

	return CreateQRResponse{
		ProviderReference: req.ProviderReference,
		QRString:          qrString,
		QRImageURL:        qrImageURL,
		ExpiresAt:         expiresAt,
	}, nil
}

// VerifyWebhook accepts any payload without signature verification.
// The dummy-trigger endpoint constructs the synthetic payload; there is no
// HMAC check needed in dev/local.
//
// Expected payload: JSON containing "provider_reference" and "amount_idr".
// The service layer handles actual JSON parsing; here we just pass through.
func (d *DummyProvider) VerifyWebhook(_ context.Context, payload []byte, _ map[string]string) (PaymentNotification, error) {
	if len(payload) == 0 {
		return PaymentNotification{}, errors.New("dummy: empty webhook payload")
	}
	// Minimal JSON parse: extract provider_reference from the raw bytes.
	// We use a simple scan rather than json.Unmarshal to avoid import complexity.
	ref, amount, err := parseDummyWebhookPayload(payload)
	if err != nil {
		return PaymentNotification{}, fmt.Errorf("dummy: parse payload: %w", err)
	}
	if ref == "" {
		return PaymentNotification{}, errors.New("dummy: missing provider_reference in payload")
	}
	return PaymentNotification{
		ProviderReference: ref,
		Status:            StatusPaid,
		ReceivedAmountIDR: amount,
		RawPayload:        payload,
	}, nil
}

// GetStatus always returns StatusPaid for dev convenience.
// The real adapter polls iPaymu's status API; the dummy just confirms paid.
func (d *DummyProvider) GetStatus(_ context.Context, providerReference string) (PaymentStatus, error) {
	if providerReference == "" {
		return StatusFailed, errors.New("dummy: provider_reference is required")
	}
	return StatusPaid, nil
}

// ListSettlements returns a single synthetic settlement item covering all
// requested-date transactions. Used for dev reconciliation smoke tests.
func (d *DummyProvider) ListSettlements(_ context.Context, date time.Time) ([]SettlementItem, error) {
	// Return a single synthetic item to allow settlement_service tests to run.
	return []SettlementItem{
		{
			ProviderReference: fmt.Sprintf("dummy-settlement-%s", date.Format("2006-01-02")),
			SettledAmountIDR:  0, // zero — dev ops must trigger real txns via dummy-trigger
			SettledAt:         date,
		},
	}, nil
}

// parseDummyWebhookPayload performs minimal JSON field extraction from the
// dummy trigger payload {"provider_reference":"...","amount_idr":N}.
// Uses encoding/json to be correct rather than fragile string scanning.
func parseDummyWebhookPayload(payload []byte) (ref string, amount int64, err error) {
	// We deliberately use encoding/json here for correctness.
	// Import is fine inside the function to avoid pulling json into package-level.
	type dummyWebhook struct {
		ProviderReference string `json:"provider_reference"`
		AmountIDR         int64  `json:"amount_idr"`
	}
	// Inline import trick is not needed — this file already imports "encoding/json"
	// via the function below; we use a small struct decode.
	var dw dummyWebhook
	if err = unmarshalJSON(payload, &dw); err != nil {
		return "", 0, err
	}
	return dw.ProviderReference, dw.AmountIDR, nil
}
