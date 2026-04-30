package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/chrisanlung/lustia-auth/internal/constants"
	"github.com/chrisanlung/lustia-auth/internal/model"
	"github.com/google/uuid"
)

// DisbursementServiceIface is the consumer-owned interface for tenant payout operations.
type DisbursementServiceIface interface {
	// CalculatePayout computes gross/fee/net for a tenant in a period without
	// writing any DB rows. Returns the preview + contributing transactions.
	CalculatePayout(ctx context.Context, tenantID string, periodStart, periodEnd time.Time) (PayoutPreview, error)

	// Create inserts a new tenant_disbursement row (status=pending) with the
	// calculated values. Combined with CalculatePayout in the platform-admin
	// "create disbursement" flow.
	Create(ctx context.Context, in CreateDisbursementInput) (DisbursementDetail, error)

	// MarkProcessing transitions pending → processing.
	MarkProcessing(ctx context.Context, id, callerUserID string) error

	// MarkTransferred transitions processing → transferred. In the same DB
	// transaction, updates all linked payment_transaction rows to disbursed
	// (flag #4).
	MarkTransferred(ctx context.Context, id, callerUserID string, bankReference, notes *string) error

	// MarkFailed transitions processing → failed.
	MarkFailed(ctx context.Context, id, callerUserID string, reason *string) error

	// Cancel transitions pending → cancelled. Returns error if not pending.
	Cancel(ctx context.Context, id, callerUserID string) error

	// ListByTenant returns a paginated list of disbursements for a tenant.
	ListByTenant(ctx context.Context, tenantID string, filter DisbursementFilter) ([]*model.TenantDisbursement, int64, error)

	// ListAll returns a paginated list of all disbursements (platform admin).
	ListAll(ctx context.Context, filter DisbursementFilter) ([]*model.TenantDisbursement, int64, error)

	// GetDetail returns the full disbursement detail including contributing
	// transactions with booking code + customer name + amounts.
	// callerTenantID is nil for platform-admin callers.
	GetDetail(ctx context.Context, id string, callerTenantID *string) (DisbursementDetail, error)
}

type disbursementServiceImpl struct {
	disbursements TenantDisbursementRepository
	paymentTxns   PaymentTransactionRepository
	audit         AuditRepository
	clock         Clock
	tx            TxManager
}

// NewDisbursementService constructs a DisbursementService.
func NewDisbursementService(
	disbursements TenantDisbursementRepository,
	paymentTxns PaymentTransactionRepository,
	audit AuditRepository,
	clock Clock,
	tx TxManager,
) DisbursementServiceIface {
	return &disbursementServiceImpl{
		disbursements: disbursements,
		paymentTxns:   paymentTxns,
		audit:         audit,
		clock:         clock,
		tx:            tx,
	}
}

// ---------------------------------------------------------------------------
// CalculatePayout
// ---------------------------------------------------------------------------

// CalculatePayout computes the payout preview without writing any DB rows.
// ADR 0015 §2.5:
//
//	gross  = SUM(received_amount_idr) WHERE status='settled' AND disbursement_id IS NULL
//	fee    = SUM(platform_fee_idr)    (same WHERE)
//	net    = gross - fee
func (s *disbursementServiceImpl) CalculatePayout(ctx context.Context, tenantID string, periodStart, periodEnd time.Time) (PayoutPreview, error) {
	txns, _, err := s.paymentTxns.FindByTenant(ctx, tenantID, PaymentTxnFilter{
		Status: strPtr(model.PaymentTxnStatusSettled),
		Page:   1,
		Limit:  10000, // large enough for a weekly batch
	})
	if err != nil {
		return PayoutPreview{}, fmt.Errorf("calculate payout: find settled txns: %w", err)
	}

	var gross, fee int64
	var eligible []PaymentTxnSummary
	for _, t := range txns {
		// Exclude already-disbursed.
		if t.DisbursementID != nil {
			continue
		}
		// Filter by settled_at within the period.
		if t.SettledAt == nil {
			continue
		}
		if t.SettledAt.Before(periodStart) || t.SettledAt.After(periodEnd.Add(24*time.Hour)) {
			continue
		}
		if t.ReceivedAmountIDR != nil {
			gross += *t.ReceivedAmountIDR
		}
		if t.PlatformFeeIDR != nil {
			fee += *t.PlatformFeeIDR
		}
		eligible = append(eligible, toPaymentTxnSummary(t))
	}
	net := gross - fee

	return PayoutPreview{
		TenantID:         tenantID,
		PeriodStart:      periodStart,
		PeriodEnd:        periodEnd,
		GrossAmountIDR:   gross,
		PlatformFeeIDR:   fee,
		NetAmountIDR:     net,
		TransactionCount: len(eligible),
		Transactions:     eligible,
	}, nil
}

// ---------------------------------------------------------------------------
// Create
// ---------------------------------------------------------------------------

// Create calculates payout for the given period and inserts a pending
// tenant_disbursement row.
func (s *disbursementServiceImpl) Create(ctx context.Context, in CreateDisbursementInput) (DisbursementDetail, error) {
	preview, err := s.CalculatePayout(ctx, in.TenantID, in.PeriodStart, in.PeriodEnd)
	if err != nil {
		return DisbursementDetail{}, err
	}

	now := s.clock.Now()
	d := &model.TenantDisbursement{
		ID:               uuid.New().String(),
		TenantID:         in.TenantID,
		PeriodStart:      in.PeriodStart,
		PeriodEnd:        in.PeriodEnd,
		GrossAmountIDR:   preview.GrossAmountIDR,
		PlatformFeeIDR:   preview.PlatformFeeIDR,
		NetAmountIDR:     preview.NetAmountIDR,
		TransactionCount: preview.TransactionCount,
		Status:           model.DisbursementStatusPending,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	if err := s.disbursements.Save(ctx, d); err != nil {
		return DisbursementDetail{}, fmt.Errorf("create disbursement: %w", err)
	}

	_ = s.audit.Append(ctx, AuditEntry{
		TenantID:     &in.TenantID,
		ActorUserID:  &in.CallerUserID,
		Action:       "disbursement.created",
		ResourceType: "tenant_disbursement",
		ResourceID:   d.ID,
		Meta: map[string]interface{}{
			"gross_idr":   d.GrossAmountIDR,
			"fee_idr":     d.PlatformFeeIDR,
			"net_idr":     d.NetAmountIDR,
			"txn_count":   d.TransactionCount,
			"period_start": in.PeriodStart.Format("2006-01-02"),
			"period_end":   in.PeriodEnd.Format("2006-01-02"),
		},
	})

	return s.GetDetail(ctx, d.ID, &in.TenantID)
}

// ---------------------------------------------------------------------------
// State transitions
// ---------------------------------------------------------------------------

// MarkProcessing transitions a disbursement from pending → processing.
func (s *disbursementServiceImpl) MarkProcessing(ctx context.Context, id, callerUserID string) error {
	d, err := s.disbursements.FindByID(ctx, id)
	if err != nil {
		return err
	}
	if d.Status != model.DisbursementStatusPending {
		return fmt.Errorf("%w: current status is %s, expected pending", constants.ErrDisbursementInvalidTransition, d.Status)
	}
	if err := s.disbursements.UpdateStatus(ctx, id, model.DisbursementStatusProcessing, nil, nil, nil); err != nil {
		return fmt.Errorf("mark processing: %w", err)
	}
	_ = s.audit.Append(ctx, AuditEntry{
		TenantID:     &d.TenantID,
		ActorUserID:  &callerUserID,
		Action:       "disbursement.processing",
		ResourceType: "tenant_disbursement",
		ResourceID:   id,
	})
	return nil
}

// MarkTransferred transitions processing → transferred.
// Per flag #4: updates all linked payment_transaction rows to status=disbursed
// in the same DB transaction as the disbursement status update.
func (s *disbursementServiceImpl) MarkTransferred(ctx context.Context, id, callerUserID string, bankReference, notes *string) error {
	d, err := s.disbursements.FindByID(ctx, id)
	if err != nil {
		return err
	}
	if d.Status != model.DisbursementStatusProcessing {
		return fmt.Errorf("%w: current status is %s, expected processing", constants.ErrDisbursementInvalidTransition, d.Status)
	}

	// Flag #4: Update disbursement status AND linked payment_transactions in
	// the same DB transaction. Both operate through dbFromContext so they share
	// the per-request transaction started by the tenant middleware.
	now := s.clock.Now()

	// Step 1: Update disbursement status.
	if err := s.disbursements.UpdateStatus(ctx, id, model.DisbursementStatusTransferred, &callerUserID, bankReference, notes); err != nil {
		return fmt.Errorf("mark transferred (disbursement status): %w", err)
	}

	// Step 2: Collect payment_transaction IDs linked to this disbursement.
	// We look up settled transactions for this tenant that have this disbursement_id.
	txns, _, err := s.paymentTxns.FindByTenant(ctx, d.TenantID, PaymentTxnFilter{
		Status: strPtr(model.PaymentTxnStatusSettled),
		Page:   1,
		Limit:  10000,
	})
	if err != nil {
		return fmt.Errorf("mark transferred: find settled txns: %w", err)
	}

	var txnIDs []string
	for _, t := range txns {
		if t.DisbursementID != nil && *t.DisbursementID == id {
			txnIDs = append(txnIDs, t.ID)
		}
	}

	// Step 3: Bulk-update payment_transactions to disbursed (same tx, flag #4).
	if len(txnIDs) > 0 {
		if _, err := s.paymentTxns.BulkMarkDisbursed(ctx, id, txnIDs, now); err != nil {
			return fmt.Errorf("mark transferred: bulk mark disbursed: %w", err)
		}
	}

	_ = s.audit.Append(ctx, AuditEntry{
		TenantID:     &d.TenantID,
		ActorUserID:  &callerUserID,
		Action:       "disbursement.transferred",
		ResourceType: "tenant_disbursement",
		ResourceID:   id,
		Meta: map[string]interface{}{
			"txn_disbursed_count": len(txnIDs),
		},
	})
	return nil
}

// MarkFailed transitions processing → failed.
func (s *disbursementServiceImpl) MarkFailed(ctx context.Context, id, callerUserID string, reason *string) error {
	d, err := s.disbursements.FindByID(ctx, id)
	if err != nil {
		return err
	}
	if d.Status != model.DisbursementStatusProcessing {
		return fmt.Errorf("%w: current status is %s, expected processing", constants.ErrDisbursementInvalidTransition, d.Status)
	}
	if err := s.disbursements.UpdateStatus(ctx, id, model.DisbursementStatusFailed, nil, nil, reason); err != nil {
		return fmt.Errorf("mark failed: %w", err)
	}
	_ = s.audit.Append(ctx, AuditEntry{
		TenantID:     &d.TenantID,
		ActorUserID:  &callerUserID,
		Action:       "disbursement.failed",
		ResourceType: "tenant_disbursement",
		ResourceID:   id,
		Meta:         map[string]interface{}{"reason": reason},
	})
	return nil
}

// Cancel transitions pending → cancelled.
func (s *disbursementServiceImpl) Cancel(ctx context.Context, id, callerUserID string) error {
	d, err := s.disbursements.FindByID(ctx, id)
	if err != nil {
		return err
	}
	if d.Status != model.DisbursementStatusPending {
		return constants.ErrDisbursementNotCancellable
	}
	if err := s.disbursements.UpdateStatus(ctx, id, model.DisbursementStatusCancelled, nil, nil, nil); err != nil {
		return fmt.Errorf("cancel disbursement: %w", err)
	}
	_ = s.audit.Append(ctx, AuditEntry{
		TenantID:     &d.TenantID,
		ActorUserID:  &callerUserID,
		Action:       "disbursement.cancelled",
		ResourceType: "tenant_disbursement",
		ResourceID:   id,
	})
	return nil
}

// ---------------------------------------------------------------------------
// List + GetDetail
// ---------------------------------------------------------------------------

// ListByTenant returns a paginated list of disbursements for a tenant.
func (s *disbursementServiceImpl) ListByTenant(ctx context.Context, tenantID string, filter DisbursementFilter) ([]*model.TenantDisbursement, int64, error) {
	return s.disbursements.FindByTenant(ctx, tenantID, filter)
}

// ListAll returns a paginated list of all disbursements (platform admin).
func (s *disbursementServiceImpl) ListAll(ctx context.Context, filter DisbursementFilter) ([]*model.TenantDisbursement, int64, error) {
	return s.disbursements.List(ctx, filter)
}

// GetDetail returns the full disbursement detail including contributing transactions.
func (s *disbursementServiceImpl) GetDetail(ctx context.Context, id string, callerTenantID *string) (DisbursementDetail, error) {
	d, err := s.disbursements.FindByID(ctx, id)
	if err != nil {
		return DisbursementDetail{}, err
	}

	// Tenant-scoped access control: tenant_admin may only see their own disbursements.
	if callerTenantID != nil && *callerTenantID != "" && d.TenantID != *callerTenantID {
		return DisbursementDetail{}, constants.ErrDisbursementNotFound
	}

	// Fetch contributing payment_transactions.
	txns, _, txnErr := s.paymentTxns.FindByTenant(ctx, d.TenantID, PaymentTxnFilter{
		Page:  1,
		Limit: 10000,
	})
	var summaries []PaymentTxnSummary
	if txnErr == nil {
		for _, t := range txns {
			if t.DisbursementID != nil && *t.DisbursementID == id {
				summaries = append(summaries, toPaymentTxnSummary(t))
			}
		}
	}

	detail := DisbursementDetail{
		ID:               d.ID,
		TenantID:         d.TenantID,
		PeriodStart:      d.PeriodStart,
		PeriodEnd:        d.PeriodEnd,
		GrossAmountIDR:   d.GrossAmountIDR,
		PlatformFeeIDR:   d.PlatformFeeIDR,
		NetAmountIDR:     d.NetAmountIDR,
		TransactionCount: d.TransactionCount,
		Status:           d.Status,
		BankReference:    d.BankReference,
		Notes:            d.Notes,
		TransferredAt:    d.TransferredAt,
		TransferredBy:    d.TransferredBy,
		CreatedAt:        d.CreatedAt,
		UpdatedAt:        d.UpdatedAt,
		Transactions:     summaries,
	}
	return detail, nil
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func strPtr(s string) *string { return &s }

// Ensure errors is used.
var _ = errors.Is
