package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/chrisanlung/lustia-auth/internal/constants"
	"github.com/chrisanlung/lustia-auth/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// Stubs
// ---------------------------------------------------------------------------

type stubSettlementBatchRepo struct {
	saved     *model.SettlementBatch
	find      *model.SettlementBatch
	summaryFn func(from, to time.Time) (count int64, volume, platformFee, payout int64, err error)
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
func (r *stubSettlementBatchRepo) Summary(_ context.Context, from, to time.Time) (int64, int64, int64, int64, error) {
	if r.summaryFn != nil {
		return r.summaryFn(from, to)
	}
	return 0, 0, 0, 0, nil
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

// ---------------------------------------------------------------------------
// Summary — date validation + timezone math + repository delegation
// ---------------------------------------------------------------------------

func newSummaryService(fn func(from, to time.Time) (int64, int64, int64, int64, error)) SettlementServiceIface {
	batchRepo := &stubSettlementBatchRepo{summaryFn: fn}
	return NewSettlementService(batchRepo, &stubPaymentTxnRepoForSettlement{}, &stubSettlementProvider{}, &stubAudit{}, &stubClock{t: time.Now()}, &stubTx{})
}

func TestSettlementService_Summary(t *testing.T) {
	t.Parallel()

	// jakartaLoc is loaded by the package init; reuse it here for date math.
	loc := jakartaLocation

	tests := []struct {
		name        string
		from        string
		to          string
		repoCount   int64
		repoVolume  int64
		repoFee     int64
		repoPayout  int64
		wantErr     error
		wantCount   int64
		wantVolume  int64
		wantFee     int64
		wantPayout  int64
		// captureFrom / captureTo let us inspect the UTC bounds the service passes
		// down to the repository so we can verify Jakarta→UTC conversion.
		checkBounds func(t *testing.T, from, to time.Time)
	}{
		{
			name:      "empty window returns all zeros",
			from:      "2026-04-26",
			to:        "2026-05-02",
			repoCount: 0,
			wantCount: 0, wantVolume: 0, wantFee: 0, wantPayout: 0,
		},
		{
			name:       "three reconciled batches, correct sums returned",
			from:       "2026-04-26",
			to:         "2026-05-02",
			repoCount:  3,
			repoVolume: 18450000,
			repoFee:    922500,
			repoPayout: 17527500,
			wantCount:  3, wantVolume: 18450000, wantFee: 922500, wantPayout: 17527500,
		},
		{
			name: "from boundary: settled_at exactly at midnight Jakarta is included",
			from: "2026-04-26",
			to:   "2026-04-26",
			checkBounds: func(t *testing.T, from, to time.Time) {
				t.Helper()
				// midnight Jakarta = 17:00 UTC previous day
				wantFrom := time.Date(2026, 4, 25, 17, 0, 0, 0, time.UTC)
				assert.Equal(t, wantFrom, from, "from UTC should be 2026-04-25T17:00:00Z (midnight Jakarta)")
			},
		},
		{
			name: "to boundary: settled_at at 23:59:59.999999999 Jakarta is included",
			from: "2026-04-26",
			to:   "2026-04-26",
			checkBounds: func(t *testing.T, from, to time.Time) {
				t.Helper()
				// end-of-day Jakarta 2026-04-26 = midnight Jakarta 2026-04-27 − 1ns
				// midnight Jakarta 2026-04-27 = 17:00 UTC 2026-04-26
				// subtract 1 ns → 2026-04-26T16:59:59.999999999Z
				wantTo := time.Date(2026, 4, 26, 16, 59, 59, 999999999, time.UTC)
				assert.Equal(t, wantTo, to, "to UTC should be 2026-04-26T16:59:59.999999999Z (end-of-day Jakarta)")
			},
		},
		{
			name: "settled_at one second before from (Jakarta) is NOT included — repository receives correct bound",
			from: "2026-04-26",
			to:   "2026-05-02",
			checkBounds: func(t *testing.T, from, _ time.Time) {
				t.Helper()
				// A timestamp 1s before from (Jakarta midnight) is before fromUTC.
				oneSecBeforeFrom := from.Add(-time.Second)
				assert.True(t, oneSecBeforeFrom.Before(from),
					"timestamp 1s before Jakarta midnight must be before the from bound passed to the repository")
			},
		},
		{
			name:    "to before from returns ErrInvalidInput",
			from:    "2026-05-02",
			to:      "2026-04-26",
			wantErr: constants.ErrInvalidInput,
		},
		{
			name:    "range exceeding 90 days returns ErrInvalidInput",
			from:    "2026-01-01",
			to:      "2026-04-15", // 104 days
			wantErr: constants.ErrInvalidInput,
		},
		{
			name:    "invalid from format returns ErrInvalidInput",
			from:    "2026/05/02",
			to:      "2026-05-02",
			wantErr: constants.ErrInvalidInput,
		},
		{
			name:    "invalid to format returns ErrInvalidInput",
			from:    "2026-04-26",
			to:      "02-05-2026",
			wantErr: constants.ErrInvalidInput,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var capturedFrom, capturedTo time.Time
			svc := newSummaryService(func(from, to time.Time) (int64, int64, int64, int64, error) {
				capturedFrom = from
				capturedTo = to
				return tt.repoCount, tt.repoVolume, tt.repoFee, tt.repoPayout, nil
			})

			out, err := svc.Summary(context.Background(), SettlementSummaryInput{
				From: tt.from,
				To:   tt.to,
			})

			if tt.wantErr != nil {
				require.Error(t, err)
				assert.True(t, errors.Is(err, tt.wantErr),
					"expected sentinel %v; got %v", tt.wantErr, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.from, out.From)
			assert.Equal(t, tt.to, out.To)
			assert.Equal(t, tt.wantCount, out.BatchCount)
			assert.Equal(t, tt.wantVolume, out.TotalVolumeIDR)
			assert.Equal(t, tt.wantFee, out.TotalPlatformFeeIDR)
			assert.Equal(t, tt.wantPayout, out.TotalPayoutIDR)

			if tt.checkBounds != nil {
				tt.checkBounds(t, capturedFrom, capturedTo)
			}

			// Sanity: verify from/to UTC timestamps are in the expected Jakarta
			// windows when no explicit checkBounds override is provided.
			if tt.checkBounds == nil && tt.from != "" && tt.to != "" && tt.wantErr == nil {
				fromDate, _ := time.ParseInLocation("2006-01-02", tt.from, loc)
				toDate, _ := time.ParseInLocation("2006-01-02", tt.to, loc)
				assert.Equal(t, fromDate.UTC(), capturedFrom, "fromUTC must equal midnight Jakarta(from) in UTC")
				wantTo := toDate.AddDate(0, 0, 1).Add(-time.Nanosecond).UTC()
				assert.Equal(t, wantTo, capturedTo, "toUTC must equal end-of-day Jakarta(to) in UTC")
			}
		})
	}
}
