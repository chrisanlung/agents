package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/chrisanlung/lustia-auth/internal/constants"
	"github.com/chrisanlung/lustia-auth/internal/model"
	"github.com/google/uuid"
)

// paymentServiceImpl implements the payment orchestration for Phase 6.
// It is not exported — callers use the PaymentServiceIface interface.
type paymentServiceImpl struct {
	paymentTxns PaymentTransactionRepository
	bookings    BookingRepository
	provider    PaymentProvider
	audit       AuditRepository
	clock       Clock
	tx          TxManager
}

// PaymentServiceIface is the consumer-owned interface for the payment service.
// Declared here (consumer-owned) so the controller can depend on the
// interface rather than the concrete type.
type PaymentServiceIface interface {
	// InitiateForBooking creates a payment_transaction row and calls the
	// payment provider to generate a QRIS payment. Called by BookingService
	// after the booking row is saved.
	InitiateForBooking(ctx context.Context, bookingID string, tenantID string, expectedAmountIDR int64, orderID string, customerName, customerEmail, customerPhone, description string) (InitiatePaymentOutput, error)

	// HandleWebhook processes an inbound provider webhook notification.
	// Signature verification is delegated to provider.VerifyWebhook before
	// this method is called (the controller does the raw-body verification).
	// This method is idempotent: same provider_reference arriving twice = no-op.
	// H-4: amount-mismatch check — logs + audit, returns 200 (no retry storm).
	HandleWebhook(ctx context.Context, rawPayload []byte, headers map[string]string) error

	// GetStatus returns the current payment status for a booking code.
	// Used by the polling endpoint (ADR 0015 §2.7).
	GetStatus(ctx context.Context, bookingCode string) (PaymentStatusView, error)

	// RetryQR generates a new QR code for a booking whose QR has expired but
	// the booking is still in pending_payment status. Only allowed within the
	// pending_payment window.
	RetryQR(ctx context.Context, bookingID string) (InitiatePaymentOutput, error)

	// GetTenantBalance returns the three-tier balance summary for a tenant.
	GetTenantBalance(ctx context.Context, tenantID string) (BalanceSummary, error)

	// SweepExpiredTransactions transitions awaiting payment_transaction rows
	// past their qr_expires_at to status=expired. Called from BookingService.SweepExpired
	// so both tables are swept together (deliverable §5).
	SweepExpiredTransactions(ctx context.Context) (int, error)
}

// NewPaymentService constructs a paymentServiceImpl and returns it as PaymentServiceIface.
func NewPaymentService(
	paymentTxns PaymentTransactionRepository,
	bookings BookingRepository,
	provider PaymentProvider,
	audit AuditRepository,
	clock Clock,
	tx TxManager,
) PaymentServiceIface {
	return &paymentServiceImpl{
		paymentTxns: paymentTxns,
		bookings:    bookings,
		provider:    provider,
		audit:       audit,
		clock:       clock,
		tx:          tx,
	}
}

// ---------------------------------------------------------------------------
// InitiateForBooking
// ---------------------------------------------------------------------------

// InitiateForBooking creates a payment_transaction row and generates a QR.
// Called from BookingService.CreatePublic after the booking row is saved.
func (s *paymentServiceImpl) InitiateForBooking(
	ctx context.Context,
	bookingID, tenantID string,
	expectedAmountIDR int64,
	orderID, customerName, customerEmail, customerPhone, description string,
) (InitiatePaymentOutput, error) {
	providerRef := uuid.New().String()

	qrResp, err := s.provider.CreateQR(ctx, CreateQRRequest{
		ProviderReference: providerRef,
		OrderID:           orderID,
		AmountIDR:         expectedAmountIDR,
		CustomerName:      customerName,
		CustomerEmail:     customerEmail,
		CustomerPhone:     customerPhone,
		Description:       description,
		ExpiryMinutes:     15, // ADR 0015 §2.7 hard cap
	})
	if err != nil {
		return InitiatePaymentOutput{}, fmt.Errorf("create qr for booking %s: %w", bookingID, err)
	}

	now := s.clock.Now()
	txn := &model.PaymentTransaction{
		ID:                uuid.New().String(),
		TenantID:          tenantID,
		BookingID:         bookingID,
		Provider:          model.PaymentProviderIPaymu, // default; factory selects concrete impl
		ProviderReference: qrResp.ProviderReference,
		QRString:          &qrResp.QRString,
		QRImageURL:        &qrResp.QRImageURL,
		QRExpiresAt:       qrResp.ExpiresAt,
		ExpectedAmountIDR: expectedAmountIDR,
		Status:            model.PaymentTxnStatusAwaiting,
		CreatedAt:         now,
		UpdatedAt:         now,
	}

	if err := s.paymentTxns.Save(ctx, txn); err != nil {
		return InitiatePaymentOutput{}, fmt.Errorf("save payment_transaction: %w", err)
	}

	_ = s.audit.Append(ctx, AuditEntry{
		TenantID:     &tenantID,
		Action:       "payment.initiated",
		ResourceType: "payment_transaction",
		ResourceID:   txn.ID,
		Meta: map[string]interface{}{
			"booking_id":         bookingID,
			"provider_reference": txn.ProviderReference,
			"expected_amount":    expectedAmountIDR,
		},
	})

	return InitiatePaymentOutput{
		TransactionID:     txn.ID,
		ProviderReference: txn.ProviderReference,
		QRString:          qrResp.QRString,
		QRImageURL:        qrResp.QRImageURL,
		QRExpiresAt:       qrResp.ExpiresAt,
	}, nil
}

// ---------------------------------------------------------------------------
// HandleWebhook
// ---------------------------------------------------------------------------

// HandleWebhook processes a provider webhook notification.
// Security controls:
//   - H-4: amount-mismatch guard — does not credit if received < expected.
//   - H-5: WHERE status='awaiting' conditional UPDATE prevents double-credit.
//   - Idempotency: same provider_reference arriving twice returns nil (no-op).
//
// Both booking.status and payment_transaction.status are updated in the same
// DB transaction via the per-request tx started by the tenant middleware.
func (s *paymentServiceImpl) HandleWebhook(ctx context.Context, rawPayload []byte, headers map[string]string) error {
	// Adapter verifies signature + parses the notification.
	notif, err := s.provider.VerifyWebhook(ctx, rawPayload, headers)
	if err != nil {
		return fmt.Errorf("verify webhook: %w", err)
	}

	if notif.Status != ProviderStatusPaid {
		// Non-payment event (e.g. expiry, failure). Log and ack.
		slog.InfoContext(ctx, "webhook: non-paid event", "provider_reference", notif.ProviderReference, "status", notif.Status)
		return nil
	}

	// Look up the payment_transaction.
	ptxn, err := s.paymentTxns.FindByProviderReference(ctx, notif.ProviderReference)
	if err != nil {
		if errors.Is(err, constants.ErrPaymentTransactionNotFound) {
			slog.WarnContext(ctx, "webhook: unknown provider_reference (no-op)",
				"provider_reference", notif.ProviderReference)
			return nil
		}
		return fmt.Errorf("webhook find payment_transaction: %w", err)
	}

	// H-4: Amount-mismatch guard. If received < expected, audit log + no-op.
	if notif.ReceivedAmountIDR < ptxn.ExpectedAmountIDR {
		slog.WarnContext(ctx, "webhook: payment amount mismatch — will not credit",
			"provider_reference", notif.ProviderReference,
			"expected", ptxn.ExpectedAmountIDR,
			"received", notif.ReceivedAmountIDR,
		)
		_ = s.audit.Append(ctx, AuditEntry{
			TenantID:     &ptxn.TenantID,
			Action:       "payment.amount_mismatch",
			ResourceType: "payment_transaction",
			ResourceID:   ptxn.ID,
			Meta: map[string]interface{}{
				"provider_reference": notif.ProviderReference,
				"expected_idr":       ptxn.ExpectedAmountIDR,
				"received_idr":       notif.ReceivedAmountIDR,
			},
		})
		// Return nil — HTTP 200 to suppress provider retries.
		return nil
	}

	now := s.clock.Now()

	// H-5: Conditional UPDATE WHERE status='awaiting'.
	rowsAffected, err := s.paymentTxns.MarkPaid(
		ctx,
		notif.ProviderReference,
		notif.ReceivedAmountIDR,
		now,
		notif.RawPayload,
	)
	if err != nil {
		return fmt.Errorf("webhook mark paid: %w", err)
	}

	if rowsAffected == 0 {
		// Idempotent delivery — payment_transaction already past 'awaiting'.
		current, lookupErr := s.paymentTxns.FindByProviderReference(ctx, notif.ProviderReference)
		if lookupErr == nil && current.Status == model.PaymentTxnStatusPaid {
			slog.InfoContext(ctx, "webhook: idempotent — already paid",
				"provider_reference", notif.ProviderReference)
		} else if lookupErr == nil {
			slog.WarnContext(ctx, "webhook: unexpected status after zero rows",
				"provider_reference", notif.ProviderReference,
				"current_status", current.Status)
		}
		return nil
	}

	// Also transition booking.status → paid.
	paidAtStr := now.Format(time.RFC3339)
	ref := ptxn.ProviderReference
	_, bookingErr := s.bookings.TransitionStatus(ctx, TransitionStatusInput{
		BookingID:        ptxn.BookingID,
		ExpectedStatus:   model.BookingStatusPendingPayment,
		NewStatus:        model.BookingStatusPaid,
		PaidAt:           &paidAtStr,
		PaymentReference: &ref,
	})
	if bookingErr != nil {
		// Log but do not fail: the payment_transaction is already paid; the
		// booking status mismatch can be reconciled manually. Better to ack
		// the webhook than to trigger a retry storm.
		slog.ErrorContext(ctx, "webhook: failed to transition booking status (payment_transaction already marked paid)",
			"booking_id", ptxn.BookingID,
			"error", bookingErr,
		)
	}

	_ = s.audit.Append(ctx, AuditEntry{
		TenantID:     &ptxn.TenantID,
		Action:       "payment.paid",
		ResourceType: "payment_transaction",
		ResourceID:   ptxn.ID,
		Meta: map[string]interface{}{
			"provider_reference": notif.ProviderReference,
			"received_idr":       notif.ReceivedAmountIDR,
		},
	})
	return nil
}

// ---------------------------------------------------------------------------
// GetStatus
// ---------------------------------------------------------------------------

// GetStatus returns the payment status for a booking identified by its code.
// Used by the polling endpoint (ADR 0015 §2.7).
// RLS context must be set to __public__ by the caller before this returns.
func (s *paymentServiceImpl) GetStatus(ctx context.Context, bookingCode string) (PaymentStatusView, error) {
	b, err := s.bookings.FindByCodePublic(ctx, bookingCode)
	if err != nil {
		return PaymentStatusView{}, constants.ErrBookingNotFound
	}

	ptxn, err := s.paymentTxns.FindByBookingID(ctx, b.ID)
	if err != nil {
		// No payment_transaction yet (e.g. paid_at_venue booking).
		return PaymentStatusView{
			Status:      b.Status,
			PaidAt:      b.PaidAt,
			QRExpiresAt: time.Time{},
		}, nil
	}

	return PaymentStatusView{
		Status:      ptxn.Status,
		PaidAt:      ptxn.PaidAt,
		QRExpiresAt: ptxn.QRExpiresAt,
	}, nil
}

// ---------------------------------------------------------------------------
// RetryQR
// ---------------------------------------------------------------------------

// RetryQR generates a fresh QR code for an expired-QR booking.
// Allowed only when the booking is still in pending_payment status.
// Uses Reset (UPDATE) rather than INSERT per flag #2 (UNIQUE booking_id).
func (s *paymentServiceImpl) RetryQR(ctx context.Context, bookingID string) (InitiatePaymentOutput, error) {
	b, err := s.bookings.FindByID(ctx, bookingID)
	if err != nil {
		return InitiatePaymentOutput{}, err
	}

	if b.Status != model.BookingStatusPendingPayment {
		return InitiatePaymentOutput{}, constants.ErrPaymentRetryNotAllowed
	}

	// Look up existing transaction.
	ptxn, err := s.paymentTxns.FindByBookingID(ctx, bookingID)
	if err != nil {
		return InitiatePaymentOutput{}, err
	}

	// Generate new QR.
	newRef := uuid.New().String()
	qrResp, err := s.provider.CreateQR(ctx, CreateQRRequest{
		ProviderReference: newRef,
		OrderID:           b.Code,
		AmountIDR:         b.TotalPriceIDR,
		ExpiryMinutes:     15,
	})
	if err != nil {
		return InitiatePaymentOutput{}, fmt.Errorf("retry qr create: %w", err)
	}

	if err := s.paymentTxns.Reset(ctx, bookingID, newRef, qrResp.QRString, qrResp.QRImageURL, qrResp.ExpiresAt); err != nil {
		return InitiatePaymentOutput{}, fmt.Errorf("reset payment_transaction: %w", err)
	}

	_ = s.audit.Append(ctx, AuditEntry{
		TenantID:     &ptxn.TenantID,
		Action:       "payment.qr_retried",
		ResourceType: "payment_transaction",
		ResourceID:   ptxn.ID,
		Meta: map[string]interface{}{
			"booking_id":             bookingID,
			"new_provider_reference": newRef,
		},
	})

	return InitiatePaymentOutput{
		TransactionID:     ptxn.ID,
		ProviderReference: newRef,
		QRString:          qrResp.QRString,
		QRImageURL:        qrResp.QRImageURL,
		QRExpiresAt:       qrResp.ExpiresAt,
	}, nil
}

// ---------------------------------------------------------------------------
// SweepExpiredTransactions
// ---------------------------------------------------------------------------

// SweepExpiredTransactions transitions awaiting payment_transaction rows past
// their qr_expires_at to status=expired. Called from BookingService.SweepExpired.
func (s *paymentServiceImpl) SweepExpiredTransactions(ctx context.Context) (int, error) {
	return s.paymentTxns.SweepExpiredTransactions(ctx)
}

// ---------------------------------------------------------------------------
// GetTenantBalance
// ---------------------------------------------------------------------------

// GetTenantBalance returns the three-tier balance breakdown for a tenant's
// finance dashboard card (ADR 0015 §2.12).
func (s *paymentServiceImpl) GetTenantBalance(ctx context.Context, tenantID string) (BalanceSummary, error) {
	inProcess, err := s.paymentTxns.SumByTenantStatus(ctx, tenantID, model.PaymentTxnStatusPaid)
	if err != nil {
		return BalanceSummary{}, fmt.Errorf("balance in-process: %w", err)
	}
	readyToDisburse, err := s.paymentTxns.SumByTenantStatus(ctx, tenantID, model.PaymentTxnStatusSettled)
	if err != nil {
		return BalanceSummary{}, fmt.Errorf("balance ready-to-disburse: %w", err)
	}
	disbursed, err := s.paymentTxns.SumByTenantStatus(ctx, tenantID, model.PaymentTxnStatusDisbursed)
	if err != nil {
		return BalanceSummary{}, fmt.Errorf("balance disbursed: %w", err)
	}
	return BalanceSummary{
		InProcessIDR:       inProcess,
		ReadyToDisburseIDR: readyToDisburse,
		DisbursedIDR:       disbursed,
	}, nil
}

