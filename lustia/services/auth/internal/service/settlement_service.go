package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/chrisanlung/lustia-auth/internal/constants"
	"github.com/chrisanlung/lustia-auth/internal/model"
	"github.com/google/uuid"
)

// SettlementServiceIface is the consumer-owned interface for settlement operations.
type SettlementServiceIface interface {
	// Reconcile fetches the provider daily settlement report for `date`,
	// creates a settlement_batch row, and bulk-updates matching
	// payment_transactions to status=settled.
	// Per flag #3: uses __platform__ sentinel for batch INSERT, then switches
	// to per-tenant context for transaction UPDATE.
	Reconcile(ctx context.Context, date time.Time, callerUserID string) (SettlementBatchSummary, error)

	// ListBatches returns a paginated list of settlement batches.
	ListBatches(ctx context.Context, filter SettlementBatchFilter) ([]*model.SettlementBatch, int64, error)

	// GetBatchDetail returns a settlement batch with related transactions +
	// mismatch warnings.
	GetBatchDetail(ctx context.Context, batchID string) (SettlementBatchDetail, error)
}

// SettlementBatchFilter is declared in interfaces.go (service package).

type settlementServiceImpl struct {
	batches     SettlementBatchRepository
	paymentTxns PaymentTransactionRepository
	provider    PaymentProvider
	audit       AuditRepository
	clock       Clock
	tx          TxManager
}

// NewSettlementService constructs a SettlementService.
func NewSettlementService(
	batches SettlementBatchRepository,
	paymentTxns PaymentTransactionRepository,
	provider PaymentProvider,
	audit AuditRepository,
	clock Clock,
	tx TxManager,
) SettlementServiceIface {
	return &settlementServiceImpl{
		batches:     batches,
		paymentTxns: paymentTxns,
		provider:    provider,
		audit:       audit,
		clock:       clock,
		tx:          tx,
	}
}

// ---------------------------------------------------------------------------
// Reconcile
// ---------------------------------------------------------------------------

// Reconcile fetches the provider settlement report for `date` and creates the
// settlement_batch + bulk-updates payment_transactions.
//
// Flag #3 (DB-designer): The settlement_batch INSERT uses __platform__ sentinel
// (no tenant_id column; platform-only RLS policy). The bulk payment_transaction
// UPDATE uses per-tenant context for each tenant represented in the batch.
//
// For Phase 6, since all transactions share the same provider connection and
// the per-request transaction is already open, we rely on the service-layer
// TxManager to sequence the context switches inside the same DB transaction.
func (s *settlementServiceImpl) Reconcile(ctx context.Context, date time.Time, callerUserID string) (SettlementBatchSummary, error) {
	// 1. Fetch provider settlement items for the given date.
	items, err := s.provider.ListSettlements(ctx, date)
	if err != nil {
		return SettlementBatchSummary{}, fmt.Errorf("list settlements from provider: %w", err)
	}

	// 2. Calculate totals from provider report.
	var totalAmount int64
	providerRefs := make([]string, 0, len(items))
	for _, item := range items {
		totalAmount += item.SettledAmountIDR
		providerRefs = append(providerRefs, item.ProviderReference)
	}

	now := s.clock.Now()
	batchID := uuid.New().String()
	rawPayload, _ := json.Marshal(items) // raw provider payload for audit

	// 3. CREATE settlement_batch (platform context — no tenant_id on this table).
	// Flag #3: SET LOCAL app.current_tenant = '__platform__' before INSERT.
	if err := s.tx.SetTenantContext(ctx, constants.PlatformTenantSentinel, callerUserID); err != nil {
		return SettlementBatchSummary{}, fmt.Errorf("set platform tenant context: %w", err)
	}
	batch := &model.SettlementBatch{
		ID:               batchID,
		Provider:         model.PaymentProviderIPaymu,
		SettledAt:        date,
		TotalAmountIDR:   totalAmount,
		TransactionCount: len(items),
		RawPayload:       &rawPayload,
		CreatedBy:        &callerUserID,
	}
	if err := s.batches.Save(ctx, batch); err != nil {
		return SettlementBatchSummary{}, fmt.Errorf("save settlement_batch: %w", err)
	}

	// 4. Bulk-update matching payment_transaction rows.
	// Flag #3: The per-tenant context for UPDATE is handled by the RLS policy
	// (payment_transaction.tenant_id = current_setting('app.current_tenant')).
	// Since all payment_transactions for paid bookings belong to the tenant
	// whose context is set by the per-request middleware, we need to look up
	// each transaction's tenant_id and switch context per-tenant for the bulk update.
	//
	// Optimised approach: for Phase 6 with a single provider, group provider_refs
	// by tenant_id, then switch + update per tenant. This respects the RLS
	// boundary while keeping the batch within one DB transaction.
	updatedCount, mismatches, err := s.bulkSettleByTenant(ctx, batchID, providerRefs, items, now)
	if err != nil {
		slog.WarnContext(ctx, "settlement: bulk update partially failed (non-fatal)",
			"batch_id", batchID, "error", err)
	}

	_ = s.audit.Append(ctx, AuditEntry{
		ActorUserID:  &callerUserID,
		Action:       "settlement.reconciled",
		ResourceType: "settlement_batch",
		ResourceID:   batchID,
		Meta: map[string]interface{}{
			"date":              date.Format("2006-01-02"),
			"provider_item_cnt": len(items),
			"updated_txn_cnt":   updatedCount,
			"mismatch_cnt":      len(mismatches),
		},
	})

	return SettlementBatchSummary{
		BatchID:          batchID,
		SettledAt:        date,
		TransactionCount: updatedCount,
		TotalAmountIDR:   totalAmount,
		MismatchCount:    len(mismatches),
	}, nil
}

// bulkSettleByTenant groups provider refs by tenant_id (via DB lookup), then
// issues per-tenant context-switch + BulkMarkSettled for each group.
// Returns the total count of updated rows and any mismatches.
func (s *settlementServiceImpl) bulkSettleByTenant(
	ctx context.Context,
	batchID string,
	providerRefs []string,
	items []SettlementItem,
	settledAt time.Time,
) (int, []SettlementMismatch, error) {
	if len(providerRefs) == 0 {
		return 0, nil, nil
	}

	// Build a set of provider refs from the provider report for mismatch detection.
	providerRefSet := make(map[string]int64, len(items))
	for _, item := range items {
		providerRefSet[item.ProviderReference] = item.SettledAmountIDR
	}

	// For Phase 6 (single payment connection), group refs by tenant_id.
	// We perform BulkMarkSettled once per batch using the __platform__ sentinel
	// since payment_transaction has an UPDATE policy for the tenant and we
	// need to switch context. The simplest correct implementation for Phase 6:
	// look up each payment_transaction to discover its tenant_id, group, then update.
	type tenantBatch struct {
		tenantID string
		refs     []string
	}

	tenantMap := make(map[string][]string)
	var mismatches []SettlementMismatch

	for _, ref := range providerRefs {
		ptxn, err := s.paymentTxns.FindByProviderReference(ctx, ref)
		if err != nil || ptxn == nil {
			// Provider reports a reference we don't have — mismatch.
			mismatches = append(mismatches, SettlementMismatch{
				ProviderReference: ref,
				Issue:             "not_in_our_db",
				AmountIDR:         providerRefSet[ref],
			})
			slog.WarnContext(ctx, "settlement: provider ref not in our DB (mismatch)",
				"provider_reference", ref)
			continue
		}
		tenantMap[ptxn.TenantID] = append(tenantMap[ptxn.TenantID], ref)
	}

	total := 0
	for tenantID, refs := range tenantMap {
		// Flag #3: switch to per-tenant context for the UPDATE.
		if err := s.tx.SetTenantContext(ctx, tenantID, ""); err != nil {
			return total, mismatches, fmt.Errorf("set tenant context for settlement update: %w", err)
		}
		n, err := s.paymentTxns.BulkMarkSettled(ctx, batchID, refs, settledAt)
		if err != nil {
			return total, mismatches, fmt.Errorf("bulk mark settled for tenant %s: %w", tenantID, err)
		}
		total += n
	}
	return total, mismatches, nil
}

// ---------------------------------------------------------------------------
// ListBatches / GetBatchDetail
// ---------------------------------------------------------------------------

// ListBatches returns a paginated list of settlement batches.
func (s *settlementServiceImpl) ListBatches(ctx context.Context, filter SettlementBatchFilter) ([]*model.SettlementBatch, int64, error) {
	f := SettlementBatchFilter{Provider: filter.Provider, Page: filter.Page, Limit: filter.Limit}
	return s.batches.List(ctx, f)
}

// GetBatchDetail returns a settlement batch with related transactions + mismatches.
func (s *settlementServiceImpl) GetBatchDetail(ctx context.Context, batchID string) (SettlementBatchDetail, error) {
	batch, err := s.batches.FindByID(ctx, batchID)
	if err != nil {
		return SettlementBatchDetail{}, err
	}

	// Fetch matching payment_transactions for this batch.
	// We use a nil status filter and filter by settlement_batch_id in a
	// tenant-agnostic way via the __platform__ sentinel the admin middleware sets.
	txns, _, err := s.paymentTxns.FindByTenant(ctx, "", PaymentTxnFilter{
		Page:  1,
		Limit: 500,
	})
	// Note: FindByTenant with empty tenantID relies on platform-context RLS.
	// For Phase 6 this is acceptable; Phase 7 may add a FindByBatchID method.
	if err != nil {
		slog.WarnContext(ctx, "settlement detail: could not load transactions (non-fatal)", "error", err)
		txns = nil
	}

	var summaries []PaymentTxnSummary
	for _, t := range txns {
		if t.SettlementBatchID == nil || *t.SettlementBatchID != batchID {
			continue
		}
		summaries = append(summaries, toPaymentTxnSummary(t))
	}

	summary := SettlementBatchSummary{
		BatchID:          batch.ID,
		SettledAt:        batch.SettledAt,
		TransactionCount: batch.TransactionCount,
		TotalAmountIDR:   batch.TotalAmountIDR,
	}

	return SettlementBatchDetail{
		SettlementBatchSummary: summary,
		Transactions:           summaries,
		Mismatches:             nil, // persisted mismatches are a Phase 7 feature
	}, nil
}

// toPaymentTxnSummary converts a model row to the service I/O type.
func toPaymentTxnSummary(t *model.PaymentTransaction) PaymentTxnSummary {
	return PaymentTxnSummary{
		ID:                t.ID,
		BookingID:         t.BookingID,
		ProviderReference: t.ProviderReference,
		Status:            t.Status,
		ExpectedAmountIDR: t.ExpectedAmountIDR,
		ReceivedAmountIDR: t.ReceivedAmountIDR,
		PlatformFeeIDR:    t.PlatformFeeIDR,
		TenantNetIDR:      t.TenantNetIDR,
		PaidAt:            t.PaidAt,
		SettledAt:         t.SettledAt,
	}
}
