package service

import (
	"context"
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

type stubDisbursementRepo struct {
	saved         *model.TenantDisbursement
	findResult    *model.TenantDisbursement
	findErr       error
	updatedStatus string
	updatedID     string
}

func (r *stubDisbursementRepo) Save(_ context.Context, d *model.TenantDisbursement) error {
	r.saved = d
	return nil
}
func (r *stubDisbursementRepo) FindByID(_ context.Context, _ string) (*model.TenantDisbursement, error) {
	return r.findResult, r.findErr
}
func (r *stubDisbursementRepo) FindByTenant(_ context.Context, _ string, _ DisbursementFilter) ([]*model.TenantDisbursement, int64, error) {
	if r.findResult != nil {
		return []*model.TenantDisbursement{r.findResult}, 1, nil
	}
	return nil, 0, nil
}
func (r *stubDisbursementRepo) List(_ context.Context, _ DisbursementFilter) ([]*model.TenantDisbursement, int64, error) {
	return nil, 0, nil
}
func (r *stubDisbursementRepo) UpdateStatus(_ context.Context, id, newStatus string, _ *string, _, _ *string) error {
	r.updatedID = id
	r.updatedStatus = newStatus
	return nil
}

// stubPaymentTxnRepoForDisb tracks BulkMarkDisbursed calls.
type stubPaymentTxnRepoForDisb struct {
	stubPaymentTxnRepo
	disbursedCount int
	settledTxns    []*model.PaymentTransaction
}

func (r *stubPaymentTxnRepoForDisb) FindByTenant(_ context.Context, _ string, _ PaymentTxnFilter) ([]*model.PaymentTransaction, int64, error) {
	return r.settledTxns, int64(len(r.settledTxns)), nil
}

func (r *stubPaymentTxnRepoForDisb) BulkMarkDisbursed(_ context.Context, _ string, ids []string, _ time.Time) (int, error) {
	r.disbursedCount += len(ids)
	return len(ids), nil
}

func newTestDisbursementSvc(disbRepo TenantDisbursementRepository, txnRepo PaymentTransactionRepository) DisbursementServiceIface {
	return NewDisbursementService(disbRepo, txnRepo, &stubAudit{}, &stubClock{t: time.Now()}, &stubTx{})
}

// ---------------------------------------------------------------------------
// State transitions
// ---------------------------------------------------------------------------

func TestDisbursementService_MarkProcessing_Success(t *testing.T) {
	t.Parallel()

	d := &model.TenantDisbursement{
		ID:       "disb-1",
		TenantID: testTenantID,
		Status:   model.DisbursementStatusPending,
	}
	disbRepo := &stubDisbursementRepo{findResult: d}
	svc := newTestDisbursementSvc(disbRepo, &stubPaymentTxnRepoForDisb{})

	err := svc.MarkProcessing(context.Background(), "disb-1", "admin-user")
	require.NoError(t, err)
	assert.Equal(t, model.DisbursementStatusProcessing, disbRepo.updatedStatus)
}

func TestDisbursementService_MarkProcessing_WrongStatus_Fails(t *testing.T) {
	t.Parallel()

	d := &model.TenantDisbursement{
		ID:     "disb-2",
		Status: model.DisbursementStatusTransferred, // not pending
	}
	disbRepo := &stubDisbursementRepo{findResult: d}
	svc := newTestDisbursementSvc(disbRepo, &stubPaymentTxnRepoForDisb{})

	err := svc.MarkProcessing(context.Background(), "disb-2", "admin-user")
	assert.ErrorIs(t, err, constants.ErrDisbursementInvalidTransition)
}

// TestDisbursementService_MarkTransferred_UpdatesLinkedTxns verifies flag #4:
// MarkTransferred must update the disbursement status AND bulk-mark linked
// payment_transactions as disbursed in the same logical operation.
func TestDisbursementService_MarkTransferred_UpdatesLinkedTxns(t *testing.T) {
	t.Parallel()

	disbID := "disb-transfer"
	d := &model.TenantDisbursement{
		ID:       disbID,
		TenantID: testTenantID,
		Status:   model.DisbursementStatusProcessing,
	}

	// Two settled transactions linked to this disbursement.
	linkedTxns := []*model.PaymentTransaction{
		{ID: "txn-1", TenantID: testTenantID, Status: model.PaymentTxnStatusSettled, DisbursementID: &disbID},
		{ID: "txn-2", TenantID: testTenantID, Status: model.PaymentTxnStatusSettled, DisbursementID: &disbID},
	}
	txnRepo := &stubPaymentTxnRepoForDisb{settledTxns: linkedTxns}
	disbRepo := &stubDisbursementRepo{findResult: d}

	svc := newTestDisbursementSvc(disbRepo, txnRepo)
	bankRef := "BANK-REF-001"
	err := svc.MarkTransferred(context.Background(), disbID, "admin-user", &bankRef, nil)

	require.NoError(t, err, "MarkTransferred must succeed")
	assert.Equal(t, model.DisbursementStatusTransferred, disbRepo.updatedStatus, "disbursement must be transferred")
	// Flag #4: both linked txns must have been bulk-disbursed.
	assert.Equal(t, 2, txnRepo.disbursedCount, "both linked transactions must be marked disbursed")
}

func TestDisbursementService_MarkTransferred_WrongStatus_Fails(t *testing.T) {
	t.Parallel()

	d := &model.TenantDisbursement{
		ID:     "disb-3",
		Status: model.DisbursementStatusPending, // must be processing first
	}
	disbRepo := &stubDisbursementRepo{findResult: d}
	svc := newTestDisbursementSvc(disbRepo, &stubPaymentTxnRepoForDisb{})

	err := svc.MarkTransferred(context.Background(), "disb-3", "admin-user", nil, nil)
	assert.ErrorIs(t, err, constants.ErrDisbursementInvalidTransition)
}

func TestDisbursementService_Cancel_OnlyPending(t *testing.T) {
	t.Parallel()

	pending := &model.TenantDisbursement{ID: "d-pending", Status: model.DisbursementStatusPending}
	processing := &model.TenantDisbursement{ID: "d-proc", Status: model.DisbursementStatusProcessing}

	t.Run("pending can be cancelled", func(t *testing.T) {
		t.Parallel()
		disbRepo := &stubDisbursementRepo{findResult: pending}
		svc := newTestDisbursementSvc(disbRepo, &stubPaymentTxnRepoForDisb{})
		err := svc.Cancel(context.Background(), "d-pending", "admin")
		require.NoError(t, err)
		assert.Equal(t, model.DisbursementStatusCancelled, disbRepo.updatedStatus)
	})

	t.Run("processing cannot be cancelled", func(t *testing.T) {
		t.Parallel()
		disbRepo := &stubDisbursementRepo{findResult: processing}
		svc := newTestDisbursementSvc(disbRepo, &stubPaymentTxnRepoForDisb{})
		err := svc.Cancel(context.Background(), "d-proc", "admin")
		assert.ErrorIs(t, err, constants.ErrDisbursementNotCancellable)
	})
}

// ---------------------------------------------------------------------------
// MarkFailed — processing → failed
// ---------------------------------------------------------------------------

func TestDisbursementService_MarkFailed_Success(t *testing.T) {
	t.Parallel()

	d := &model.TenantDisbursement{ID: "disb-fail", TenantID: testTenantID, Status: model.DisbursementStatusProcessing}
	disbRepo := &stubDisbursementRepo{findResult: d}
	svc := newTestDisbursementSvc(disbRepo, &stubPaymentTxnRepoForDisb{})

	reason := "bank rejected: invalid account"
	err := svc.MarkFailed(context.Background(), "disb-fail", "admin", &reason)
	require.NoError(t, err)
	assert.Equal(t, model.DisbursementStatusFailed, disbRepo.updatedStatus)
}

// ---------------------------------------------------------------------------
// GetDetail — tenant-scoped access control
// ---------------------------------------------------------------------------

func TestDisbursementService_GetDetail_TenantIsolation(t *testing.T) {
	t.Parallel()

	otherTenant := "other-tenant-id"
	d := &model.TenantDisbursement{
		ID:       "disb-other",
		TenantID: otherTenant,
		Status:   model.DisbursementStatusPending,
	}
	disbRepo := &stubDisbursementRepo{findResult: d}
	svc := newTestDisbursementSvc(disbRepo, &stubPaymentTxnRepoForDisb{})

	callerTenant := testTenantID // different tenant — should be denied
	_, err := svc.GetDetail(context.Background(), "disb-other", &callerTenant)
	assert.ErrorIs(t, err, constants.ErrDisbursementNotFound,
		"tenant isolation: different tenant must receive not-found")
}

func TestDisbursementService_GetDetail_PlatformAdmin_NoScope(t *testing.T) {
	t.Parallel()

	d := &model.TenantDisbursement{
		ID:       "disb-any",
		TenantID: testTenantID,
		Status:   model.DisbursementStatusPending,
	}
	disbRepo := &stubDisbursementRepo{findResult: d}
	svc := newTestDisbursementSvc(disbRepo, &stubPaymentTxnRepoForDisb{})

	// nil callerTenantID = platform admin
	detail, err := svc.GetDetail(context.Background(), "disb-any", nil)
	require.NoError(t, err)
	assert.Equal(t, "disb-any", detail.ID)
}
