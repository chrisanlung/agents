package service

import (
	"context"
	"testing"
	"time"

	"github.com/chrisanlung/lustia-auth/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// Stubs
// ---------------------------------------------------------------------------

type stubSettlementBatchRepo struct {
	saved *model.SettlementBatch
	find  *model.SettlementBatch
}

func (r *stubSettlementBatchRepo) Save(_ context.Context, b *model.SettlementBatch) error {
	r.saved = b
	return nil
}
func (r *stubSettlementBatchRepo) FindByID(_ context.Context, _ string) (*model.SettlementBatch, error) {
	if r.find != nil {
		return r.find, nil
	}
	return nil, nil
}
func (r *stubSettlementBatchRepo) FindByDate(_ context.Context, _ time.Time) ([]*model.SettlementBatch, error) {
	return nil, nil
}
func (r *stubSettlementBatchRepo) List(_ context.Context, _ SettlementBatchFilter) ([]*model.SettlementBatch, int64, error) {
	return nil, 0, nil
}

// stubPaymentTxnRepoForSettlement extends stubPaymentTxnRepo to track BulkMarkSettled calls.
type stubPaymentTxnRepoForSettlement struct {
	stubPaymentTxnRepo
	bulkSettledCount    int
	bulkSettledTenantID string
	findByRefResults    map[string]*model.PaymentTransaction
}

func (r *stubPaymentTxnRepoForSettlement) FindByProviderReference(_ context.Context, ref string) (*model.PaymentTransaction, error) {
	if r.findByRefResults != nil {
		if t, ok := r.findByRefResults[ref]; ok {
			return t, nil
		}
	}
	return r.stubPaymentTxnRepo.FindByProviderReference(nil, ref)
}

func (r *stubPaymentTxnRepoForSettlement) BulkMarkSettled(_ context.Context, _ string, refs []string, _ time.Time) (int, error) {
	r.bulkSettledCount += len(refs)
	return len(refs), nil
}

// stubSettlementProvider returns a configurable list of settlement items.
type stubSettlementProvider struct {
	stubPaymentProvider
	items []SettlementItem
}

func (p *stubSettlementProvider) ListSettlements(_ context.Context, _ time.Time) ([]SettlementItem, error) {
	return p.items, nil
}

// ---------------------------------------------------------------------------
// Reconcile — fee computation + tenant scope (flag #3)
// ---------------------------------------------------------------------------

func TestSettlementService_Reconcile_FeeComputation(t *testing.T) {
	t.Parallel()

	// Provider reports two transactions belonging to the same tenant.
	txnA := &model.PaymentTransaction{
		ID:                "txn-a",
		TenantID:          testTenantID,
		BookingID:         "booking-a",
		ProviderReference: "ref-a",
		ExpectedAmountIDR: 100000,
		Status:            model.PaymentTxnStatusPaid,
	}
	txnB := &model.PaymentTransaction{
		ID:                "txn-b",
		TenantID:          testTenantID,
		BookingID:         "booking-b",
		ProviderReference: "ref-b",
		ExpectedAmountIDR: 200000,
		Status:            model.PaymentTxnStatusPaid,
	}

	txnRepo := &stubPaymentTxnRepoForSettlement{
		findByRefResults: map[string]*model.PaymentTransaction{
			"ref-a": txnA,
			"ref-b": txnB,
		},
	}
	batchRepo := &stubSettlementBatchRepo{}
	provider := &stubSettlementProvider{
		items: []SettlementItem{
			{ProviderReference: "ref-a", SettledAmountIDR: 100000, SettledAt: time.Now()},
			{ProviderReference: "ref-b", SettledAmountIDR: 200000, SettledAt: time.Now()},
		},
	}

	svc := NewSettlementService(batchRepo, txnRepo, provider, &stubAudit{}, &stubClock{t: time.Now()}, &stubTx{})
	summary, err := svc.Reconcile(context.Background(), time.Now(), "admin-user")

	require.NoError(t, err)
	assert.Equal(t, 2, summary.TransactionCount, "two transactions marked settled")
	assert.Equal(t, int64(300000), summary.TotalAmountIDR)
	assert.Equal(t, 0, summary.MismatchCount)
	require.NotNil(t, batchRepo.saved, "settlement_batch row must be saved")
	assert.Equal(t, int64(300000), batchRepo.saved.TotalAmountIDR)
}

// TestSettlementService_Reconcile_MismatchDetection verifies that provider
// references not found in our DB are counted as mismatches (flag #3 audit).
func TestSettlementService_Reconcile_MismatchDetection(t *testing.T) {
	t.Parallel()

	// Provider reports ref-unknown which is not in our DB.
	txnRepo := &stubPaymentTxnRepoForSettlement{
		stubPaymentTxnRepo: stubPaymentTxnRepo{
			findByProviderRefErr: nil, // override below via findByRefResults
		},
		findByRefResults: map[string]*model.PaymentTransaction{
			// ref-known is in our DB; ref-unknown is NOT.
			"ref-known": {
				ID:                "txn-known",
				TenantID:          testTenantID,
				ProviderReference: "ref-known",
				Status:            model.PaymentTxnStatusPaid,
			},
		},
	}
	batchRepo := &stubSettlementBatchRepo{}
	provider := &stubSettlementProvider{
		items: []SettlementItem{
			{ProviderReference: "ref-known", SettledAmountIDR: 100000},
			{ProviderReference: "ref-unknown", SettledAmountIDR: 50000}, // mismatch
		},
	}

	svc := NewSettlementService(batchRepo, txnRepo, provider, &stubAudit{}, &stubClock{t: time.Now()}, &stubTx{})
	summary, err := svc.Reconcile(context.Background(), time.Now(), "admin-user")

	require.NoError(t, err)
	assert.Equal(t, 1, summary.MismatchCount, "one mismatch for unknown provider ref")
	assert.Equal(t, 1, summary.TransactionCount, "only ref-known is settled")
}
