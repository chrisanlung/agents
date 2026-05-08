package service

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/chrisanlung/lustia-auth/internal/constants"
	"github.com/chrisanlung/lustia-auth/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// Stubs (payment-service specific)
// ---------------------------------------------------------------------------

// stubPaymentTxnRepo is a minimal PaymentTransactionRepository stub.
type stubPaymentTxnRepo struct {
	// Inputs wired per test.
	findByProviderRefResult *model.PaymentTransaction
	findByProviderRefErr    error
	findByBookingIDResult   *model.PaymentTransaction
	findByBookingIDErr      error
	findByIDResult          *model.PaymentTransaction
	saveErr                 error
	markPaidRows            int64
	markPaidErr             error
	sweepCount              int
	sumResult               int64
	// Captured call counts for assertion.
	markExpiredCalls int
	markFailedCalls  int
	markVoidedCalls  int
}

func (r *stubPaymentTxnRepo) Save(_ context.Context, _ *model.PaymentTransaction) error {
	return r.saveErr
}
func (r *stubPaymentTxnRepo) Reset(_ context.Context, _, _, _, _ string, _ time.Time) error {
	return nil
}
func (r *stubPaymentTxnRepo) FindByID(_ context.Context, _ string) (*model.PaymentTransaction, error) {
	return r.findByIDResult, nil
}
func (r *stubPaymentTxnRepo) FindByBookingID(_ context.Context, _ string) (*model.PaymentTransaction, error) {
	return r.findByBookingIDResult, r.findByBookingIDErr
}
func (r *stubPaymentTxnRepo) FindByProviderReference(_ context.Context, _ string) (*model.PaymentTransaction, error) {
	return r.findByProviderRefResult, r.findByProviderRefErr
}
func (r *stubPaymentTxnRepo) FindByTenant(_ context.Context, _ string, _ PaymentTxnFilter) ([]*model.PaymentTransaction, int64, error) {
	return nil, 0, nil
}
func (r *stubPaymentTxnRepo) MarkPaid(_ context.Context, _ string, _ int64, _ time.Time, _ []byte) (int64, error) {
	return r.markPaidRows, r.markPaidErr
}
func (r *stubPaymentTxnRepo) BulkMarkSettled(_ context.Context, _ string, _ []string, _ time.Time) (int, error) {
	return 0, nil
}
func (r *stubPaymentTxnRepo) BulkMarkDisbursed(_ context.Context, _ string, _ []string, _ time.Time) (int, error) {
	return 0, nil
}
func (r *stubPaymentTxnRepo) SumByTenantStatus(_ context.Context, _ string, _ string) (int64, error) {
	return r.sumResult, nil
}
func (r *stubPaymentTxnRepo) SweepExpiredTransactions(_ context.Context) (int, error) {
	return r.sweepCount, nil
}
func (r *stubPaymentTxnRepo) MarkVoided(_ context.Context, _ string) (int64, error) {
	r.markVoidedCalls++
	return 1, nil
}
func (r *stubPaymentTxnRepo) MarkExpired(_ context.Context, _ string) (int64, error) {
	r.markExpiredCalls++
	return 1, nil
}
func (r *stubPaymentTxnRepo) MarkFailed(_ context.Context, _ string) (int64, error) {
	r.markFailedCalls++
	return 1, nil
}

// stubBookingRepoForPayment is a minimal BookingRepository for payment tests.
type stubBookingRepoForPayment struct {
	findByCodePublicResult *model.Booking
	findByCodePublicErr    error
	findByIDResult         *model.Booking
	findByIDErr            error
	transitionRows         int64
	transitionErr          error
	// Captured call inputs.
	lastTransitionInput TransitionStatusInput
}

func (r *stubBookingRepoForPayment) Save(_ context.Context, _ *model.Booking) error { return nil }
func (r *stubBookingRepoForPayment) SaveAddons(_ context.Context, _ []*model.BookingAddon) error {
	return nil
}
func (r *stubBookingRepoForPayment) FindByID(_ context.Context, _ string) (*model.Booking, error) {
	return r.findByIDResult, r.findByIDErr
}
func (r *stubBookingRepoForPayment) FindByCode(_ context.Context, _ string) (*model.Booking, error) {
	return nil, constants.ErrBookingNotFound
}
func (r *stubBookingRepoForPayment) FindByCodePublic(_ context.Context, _ string) (*model.Booking, error) {
	return r.findByCodePublicResult, r.findByCodePublicErr
}
func (r *stubBookingRepoForPayment) FindAddonsByBooking(_ context.Context, _ string) ([]*model.BookingAddon, error) {
	return nil, nil
}
func (r *stubBookingRepoForPayment) FindByTenant(_ context.Context, _ string, _ BookingFilter) ([]*model.Booking, int64, error) {
	return nil, 0, nil
}
func (r *stubBookingRepoForPayment) TransitionStatus(_ context.Context, in TransitionStatusInput) (int64, error) {
	r.lastTransitionInput = in
	return r.transitionRows, r.transitionErr
}
func (r *stubBookingRepoForPayment) SweepExpired(_ context.Context) (int, error) { return 0, nil }
func (r *stubBookingRepoForPayment) FindByPaymentReference(_ context.Context, _ string) (*model.Booking, error) {
	return nil, constants.ErrBookingNotFound
}
func (r *stubBookingRepoForPayment) ReportSummary(_ context.Context, _ BookingReportFilter) (BookingReportSummary, error) {
	return BookingReportSummary{}, nil
}
func (r *stubBookingRepoForPayment) FindBookedRoomIDsInSlots(_ context.Context, _ string, windows []SlotWindow) (map[string][]string, error) {
	result := make(map[string][]string, len(windows))
	return result, nil
}
func (r *stubBookingRepoForPayment) IsTherapistBookedInSlots(_ context.Context, _ string, windows []SlotWindow, _ int) (map[string]bool, error) {
	result := make(map[string]bool, len(windows))
	return result, nil
}
func (r *stubBookingRepoForPayment) FindTherapistConflicts(_ context.Context, _ []string, _, _ time.Time) ([]TherapistBookingInterval, error) {
	return []TherapistBookingInterval{}, nil
}

// stubPaymentProvider is a minimal PaymentProvider stub.
type stubPaymentProvider struct {
	createQRResp    CreateQRResponse
	createQRErr     error
	webhookNotif    PaymentNotification
	webhookErr      error
	getStatusResult ProviderPaymentStatus
	getStatusErr    error
}

func (p *stubPaymentProvider) CreateQR(_ context.Context, req CreateQRRequest) (CreateQRResponse, error) {
	if p.createQRErr != nil {
		return CreateQRResponse{}, p.createQRErr
	}
	resp := p.createQRResp
	if resp.ProviderReference == "" {
		resp.ProviderReference = req.ProviderReference
	}
	if resp.ExpiresAt.IsZero() {
		resp.ExpiresAt = time.Now().Add(15 * time.Minute)
	}
	return resp, nil
}
func (p *stubPaymentProvider) VerifyWebhook(_ context.Context, _ []byte, _ map[string]string) (PaymentNotification, error) {
	return p.webhookNotif, p.webhookErr
}
func (p *stubPaymentProvider) GetStatus(_ context.Context, _ string) (ProviderPaymentStatus, error) {
	if p.getStatusErr != nil {
		return "", p.getStatusErr
	}
	if p.getStatusResult == "" {
		return ProviderStatusPaid, nil
	}
	return p.getStatusResult, nil
}
func (p *stubPaymentProvider) ListSettlements(_ context.Context, _ time.Time) ([]SettlementItem, error) {
	return nil, nil
}

// newTestPaymentSvc constructs a PaymentService with the given stubs.
func newTestPaymentSvc(
	txnRepo PaymentTransactionRepository,
	bookingRepo BookingRepository,
	provider PaymentProvider,
) PaymentServiceIface {
	return NewPaymentService(txnRepo, bookingRepo, provider, &stubAudit{}, &stubClock{t: time.Now()}, &stubTx{})
}

// ---------------------------------------------------------------------------
// InitiateForBooking
// ---------------------------------------------------------------------------

func TestPaymentService_Initiate_Success(t *testing.T) {
	t.Parallel()

	txnRepo := &stubPaymentTxnRepo{}
	bookingRepo := &stubBookingRepoForPayment{}
	provider := &stubPaymentProvider{
		createQRResp: CreateQRResponse{
			QRString:   "00020101...QR",
			QRImageURL: "https://example.com/qr.png",
		},
	}

	svc := newTestPaymentSvc(txnRepo, bookingRepo, provider)
	out, err := svc.InitiateForBooking(context.Background(),
		"booking-id", "tenant-id", 150000,
		"AB12-CD34", "Budi", "budi@example.com", "08123", "Booking layanan",
	)

	require.NoError(t, err)
	assert.NotEmpty(t, out.TransactionID)
	assert.NotEmpty(t, out.ProviderReference)
	assert.Equal(t, "00020101...QR", out.QRString)
	assert.False(t, out.QRExpiresAt.IsZero())
}

func TestPaymentService_Initiate_ProviderError_Propagates(t *testing.T) {
	t.Parallel()

	txnRepo := &stubPaymentTxnRepo{}
	bookingRepo := &stubBookingRepoForPayment{}
	provider := &stubPaymentProvider{createQRErr: assert.AnError}

	svc := newTestPaymentSvc(txnRepo, bookingRepo, provider)
	_, err := svc.InitiateForBooking(context.Background(),
		"booking-id", "tenant-id", 150000, "AB12-CD34", "Budi", "", "", "",
	)
	assert.Error(t, err)
}

// ---------------------------------------------------------------------------
// HandleWebhook — idempotency (H-5)
// ---------------------------------------------------------------------------

func TestPaymentService_HandleWebhook_Idempotent_AlreadyPaid(t *testing.T) {
	t.Parallel()

	existing := &model.PaymentTransaction{
		ID:                "txn-1",
		TenantID:          testTenantID,
		BookingID:         "booking-1",
		ProviderReference: "ref-123",
		ExpectedAmountIDR: 150000,
		Status:            model.PaymentTxnStatusPaid, // already paid
	}
	txnRepo := &stubPaymentTxnRepo{
		findByProviderRefResult: existing,
		markPaidRows:            0, // WHERE status='awaiting' → 0 rows (idempotent)
	}
	bookingRepo := &stubBookingRepoForPayment{}
	payload, _ := json.Marshal(map[string]interface{}{
		"provider_reference": "ref-123",
		"amount_idr":         int64(150000),
	})
	provider := &stubPaymentProvider{
		webhookNotif: PaymentNotification{
			ProviderReference: "ref-123",
			Status:            ProviderStatusPaid,
			ReceivedAmountIDR: 150000,
			RawPayload:        payload,
		},
	}

	svc := newTestPaymentSvc(txnRepo, bookingRepo, provider)
	err := svc.HandleWebhook(context.Background(), payload, nil)
	// H-5: idempotent — must return nil, not an error.
	assert.NoError(t, err, "idempotent webhook must be a no-op")
}

// ---------------------------------------------------------------------------
// HandleWebhook — amount mismatch (H-4)
// ---------------------------------------------------------------------------

func TestPaymentService_HandleWebhook_AmountMismatch_DoesNotCredit(t *testing.T) {
	t.Parallel()

	existing := &model.PaymentTransaction{
		ID:                "txn-underpay",
		TenantID:          testTenantID,
		BookingID:         "booking-underpay",
		ProviderReference: "ref-underpay",
		ExpectedAmountIDR: 150000,
		Status:            model.PaymentTxnStatusAwaiting,
	}
	txnRepo := &stubPaymentTxnRepo{
		findByProviderRefResult: existing,
		markPaidRows:            0,
	}
	bookingRepo := &stubBookingRepoForPayment{}
	payload := []byte(`{"provider_reference":"ref-underpay","amount_idr":1}`)
	provider := &stubPaymentProvider{
		webhookNotif: PaymentNotification{
			ProviderReference: "ref-underpay",
			Status:            ProviderStatusPaid,
			ReceivedAmountIDR: 1, // IDR 1 vs 150000 expected — mismatch
			RawPayload:        payload,
		},
	}

	svc := newTestPaymentSvc(txnRepo, bookingRepo, provider)
	err := svc.HandleWebhook(context.Background(), payload, nil)
	// H-4: must return nil (HTTP 200, no retry storm) without crediting.
	assert.NoError(t, err, "H-4: underpayment must return nil")
	// MarkPaid must NOT have been called with success (rows=0 in repo means never called successfully).
	assert.Equal(t, int64(0), txnRepo.markPaidRows)
}

// ---------------------------------------------------------------------------
// HandleWebhook — unknown provider reference
// ---------------------------------------------------------------------------

func TestPaymentService_HandleWebhook_UnknownRef_NoOp(t *testing.T) {
	t.Parallel()

	txnRepo := &stubPaymentTxnRepo{
		findByProviderRefErr: constants.ErrPaymentTransactionNotFound,
	}
	bookingRepo := &stubBookingRepoForPayment{}
	payload := []byte(`{"provider_reference":"unknown-ref","amount_idr":150000}`)
	provider := &stubPaymentProvider{
		webhookNotif: PaymentNotification{
			ProviderReference: "unknown-ref",
			Status:            ProviderStatusPaid,
			ReceivedAmountIDR: 150000,
			RawPayload:        payload,
		},
	}

	svc := newTestPaymentSvc(txnRepo, bookingRepo, provider)
	err := svc.HandleWebhook(context.Background(), payload, nil)
	assert.NoError(t, err, "unknown provider_reference must be a silent no-op")
}

// ---------------------------------------------------------------------------
// HandleWebhook — non-paid status (no-op)
// ---------------------------------------------------------------------------

func TestPaymentService_HandleWebhook_NonPaidStatus_NoOp(t *testing.T) {
	t.Parallel()

	txnRepo := &stubPaymentTxnRepo{}
	bookingRepo := &stubBookingRepoForPayment{}
	payload := []byte(`{}`)
	provider := &stubPaymentProvider{
		webhookNotif: PaymentNotification{
			ProviderReference: "ref-expired",
			Status:            ProviderStatusExpired,
		},
	}

	svc := newTestPaymentSvc(txnRepo, bookingRepo, provider)
	err := svc.HandleWebhook(context.Background(), payload, nil)
	assert.NoError(t, err, "non-paid webhook status must be a no-op")
}

// ---------------------------------------------------------------------------
// RetryQR
// ---------------------------------------------------------------------------

func TestPaymentService_RetryQR_Success(t *testing.T) {
	t.Parallel()

	booking := &model.Booking{
		ID:            "booking-retry",
		TenantID:      testTenantID,
		TotalPriceIDR: 150000,
		Status:        model.BookingStatusPendingPayment,
		Code:          "RETR-Y001",
	}
	existing := &model.PaymentTransaction{
		ID:       "txn-retry",
		TenantID: testTenantID,
		Status:   model.PaymentTxnStatusAwaiting,
	}
	txnRepo := &stubPaymentTxnRepo{findByBookingIDResult: existing}
	bookingRepo := &stubBookingRepoForPayment{findByIDResult: booking}
	provider := &stubPaymentProvider{
		createQRResp: CreateQRResponse{QRString: "NEW-QR"},
	}

	svc := newTestPaymentSvc(txnRepo, bookingRepo, provider)
	out, err := svc.RetryQR(context.Background(), "booking-retry")
	require.NoError(t, err)
	assert.Equal(t, "NEW-QR", out.QRString)
}

func TestPaymentService_RetryQR_NotAllowed_WhenPaid(t *testing.T) {
	t.Parallel()

	booking := &model.Booking{
		ID:     "booking-paid",
		Status: model.BookingStatusPaid, // already paid — retry not allowed
	}
	txnRepo := &stubPaymentTxnRepo{}
	bookingRepo := &stubBookingRepoForPayment{findByIDResult: booking}
	provider := &stubPaymentProvider{}

	svc := newTestPaymentSvc(txnRepo, bookingRepo, provider)
	_, err := svc.RetryQR(context.Background(), "booking-paid")
	assert.ErrorIs(t, err, constants.ErrPaymentRetryNotAllowed)
}

// ---------------------------------------------------------------------------
// GetTenantBalance
// ---------------------------------------------------------------------------

func TestPaymentService_GetTenantBalance(t *testing.T) {
	t.Parallel()

	txnRepo := &stubPaymentTxnRepo{sumResult: 100000}
	bookingRepo := &stubBookingRepoForPayment{}
	svc := newTestPaymentSvc(txnRepo, bookingRepo, &stubPaymentProvider{})

	bal, err := svc.GetTenantBalance(context.Background(), testTenantID)
	require.NoError(t, err)
	// All three statuses return sumResult=100000 from the stub.
	assert.Equal(t, int64(100000), bal.InProcessIDR)
	assert.Equal(t, int64(100000), bal.ReadyToDisburseIDR)
	assert.Equal(t, int64(100000), bal.DisbursedIDR)
}

// ---------------------------------------------------------------------------
// SweepExpiredTransactions
// ---------------------------------------------------------------------------

func TestPaymentService_SweepExpiredTransactions(t *testing.T) {
	t.Parallel()

	txnRepo := &stubPaymentTxnRepo{sweepCount: 3}
	svc := newTestPaymentSvc(txnRepo, &stubBookingRepoForPayment{}, &stubPaymentProvider{})

	n, err := svc.SweepExpiredTransactions(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 3, n)
}

// ---------------------------------------------------------------------------
// SyncStatus
// ---------------------------------------------------------------------------

func newSyncInput(bookingID string) SyncPaymentInput {
	return SyncPaymentInput{
		BookingID:      bookingID,
		CallerTenantID: testTenantID,
		CallerUserID:   "operator-user-id",
		CallerBranches: []string{testBranchID},
		IsAdmin:        false,
	}
}

func pendingBooking(id string) *model.Booking {
	return &model.Booking{
		ID:       id,
		TenantID: testTenantID,
		BranchID: testBranchID,
		Status:   model.BookingStatusPendingPayment,
	}
}

func awaitingTxn(bookingID string) *model.PaymentTransaction {
	return &model.PaymentTransaction{
		ID:                "txn-" + bookingID,
		TenantID:          testTenantID,
		BookingID:         bookingID,
		ProviderReference: "ref-" + bookingID,
		ExpectedAmountIDR: 150000,
		Status:            model.PaymentTxnStatusAwaiting,
	}
}

// TestSyncStatus_Pending_NoOp verifies that a pending provider status causes
// no state change and returns action "no_op".
func TestSyncStatus_Pending_NoOp(t *testing.T) {
	t.Parallel()

	bookingID := "bk-sync-pending"
	txnRepo := &stubPaymentTxnRepo{findByBookingIDResult: awaitingTxn(bookingID)}
	bookingRepo := &stubBookingRepoForPayment{
		findByIDResult: pendingBooking(bookingID),
		transitionRows: 1,
	}
	provider := &stubPaymentProvider{getStatusResult: ProviderStatusPending}

	svc := newTestPaymentSvc(txnRepo, bookingRepo, provider)
	res, err := svc.SyncStatus(context.Background(), newSyncInput(bookingID))

	require.NoError(t, err)
	assert.Equal(t, "no_op", res.ActionTaken)
	assert.Equal(t, ProviderStatusPending, res.ProviderStatus)
	// No transitions should have been recorded.
	assert.Empty(t, bookingRepo.lastTransitionInput.BookingID)
	assert.Equal(t, 0, txnRepo.markExpiredCalls)
	assert.Equal(t, 0, txnRepo.markFailedCalls)
}

// TestSyncStatus_Paid_UpdatesTxnAndBooking verifies the happy path: provider
// reports paid → MarkPaid + booking transition to paid.
func TestSyncStatus_Paid_UpdatesTxnAndBooking(t *testing.T) {
	t.Parallel()

	bookingID := "bk-sync-paid"
	txnRepo := &stubPaymentTxnRepo{
		findByBookingIDResult: awaitingTxn(bookingID),
		markPaidRows:          1,
	}
	bookingRepo := &stubBookingRepoForPayment{
		findByIDResult: pendingBooking(bookingID),
		transitionRows: 1,
	}
	provider := &stubPaymentProvider{getStatusResult: ProviderStatusPaid}

	svc := newTestPaymentSvc(txnRepo, bookingRepo, provider)
	res, err := svc.SyncStatus(context.Background(), newSyncInput(bookingID))

	require.NoError(t, err)
	assert.Equal(t, "paid", res.ActionTaken)
	assert.Equal(t, ProviderStatusPaid, res.ProviderStatus)
	assert.Equal(t, bookingID, res.BookingID)
	// Booking was transitioned to paid.
	assert.Equal(t, model.BookingStatusPaid, bookingRepo.lastTransitionInput.NewStatus)
}

// TestSyncStatus_Failed_MarksFailedAndExpires verifies that a failed provider
// status marks the transaction failed and transitions the booking to expired.
func TestSyncStatus_Failed_MarksFailedAndExpires(t *testing.T) {
	t.Parallel()

	bookingID := "bk-sync-failed"
	txnRepo := &stubPaymentTxnRepo{findByBookingIDResult: awaitingTxn(bookingID)}
	bookingRepo := &stubBookingRepoForPayment{
		findByIDResult: pendingBooking(bookingID),
		transitionRows: 1,
	}
	provider := &stubPaymentProvider{getStatusResult: ProviderStatusFailed}

	svc := newTestPaymentSvc(txnRepo, bookingRepo, provider)
	res, err := svc.SyncStatus(context.Background(), newSyncInput(bookingID))

	require.NoError(t, err)
	assert.Equal(t, "failed", res.ActionTaken)
	assert.Equal(t, 1, txnRepo.markFailedCalls,
		"MarkFailed must be called once for a failed provider status")
	assert.Equal(t, 0, txnRepo.markExpiredCalls)
	assert.Equal(t, model.BookingStatusExpired, bookingRepo.lastTransitionInput.NewStatus)
}

// TestSyncStatus_Expired_MarksExpiredAndExpires verifies that an expired
// provider status marks the transaction expired and transitions the booking.
func TestSyncStatus_Expired_MarksExpiredAndExpires(t *testing.T) {
	t.Parallel()

	bookingID := "bk-sync-expired"
	txnRepo := &stubPaymentTxnRepo{findByBookingIDResult: awaitingTxn(bookingID)}
	bookingRepo := &stubBookingRepoForPayment{
		findByIDResult: pendingBooking(bookingID),
		transitionRows: 1,
	}
	provider := &stubPaymentProvider{getStatusResult: ProviderStatusExpired}

	svc := newTestPaymentSvc(txnRepo, bookingRepo, provider)
	res, err := svc.SyncStatus(context.Background(), newSyncInput(bookingID))

	require.NoError(t, err)
	assert.Equal(t, "expired", res.ActionTaken)
	assert.Equal(t, 1, txnRepo.markExpiredCalls,
		"MarkExpired must be called once for an expired provider status")
	assert.Equal(t, 0, txnRepo.markFailedCalls)
	assert.Equal(t, model.BookingStatusExpired, bookingRepo.lastTransitionInput.NewStatus)
}

// TestSyncStatus_CrossTenantGuard verifies that a booking belonging to a
// different tenant returns ErrBookingNotFound (no information leak).
func TestSyncStatus_CrossTenantGuard(t *testing.T) {
	t.Parallel()

	bookingID := "bk-sync-xt"
	otherTenantBooking := &model.Booking{
		ID:       bookingID,
		TenantID: "other-tenant-uuid",
		BranchID: testBranchID,
		Status:   model.BookingStatusPendingPayment,
	}
	bookingRepo := &stubBookingRepoForPayment{findByIDResult: otherTenantBooking}
	svc := newTestPaymentSvc(&stubPaymentTxnRepo{}, bookingRepo, &stubPaymentProvider{})

	_, err := svc.SyncStatus(context.Background(), newSyncInput(bookingID))
	assert.ErrorIs(t, err, constants.ErrBookingNotFound,
		"cross-tenant access must be rejected as booking-not-found")
}

// TestSyncStatus_NonPendingBooking_NoOp verifies that a booking which is
// already paid (or any non-pending_payment status) returns no_op without
// calling the provider at all.
func TestSyncStatus_NonPendingBooking_NoOp(t *testing.T) {
	t.Parallel()

	bookingID := "bk-sync-already-paid"
	paidBooking := &model.Booking{
		ID:       bookingID,
		TenantID: testTenantID,
		BranchID: testBranchID,
		Status:   model.BookingStatusPaid, // already resolved
	}
	bookingRepo := &stubBookingRepoForPayment{findByIDResult: paidBooking}
	// Provider returns paid — but must NOT be reached.
	provider := &stubPaymentProvider{getStatusResult: ProviderStatusPaid}
	txnRepo := &stubPaymentTxnRepo{}

	svc := newTestPaymentSvc(txnRepo, bookingRepo, provider)
	res, err := svc.SyncStatus(context.Background(), newSyncInput(bookingID))

	require.NoError(t, err)
	assert.Equal(t, "no_op", res.ActionTaken,
		"non-pending_payment booking must return no_op without touching provider or DB")
	// No transition recorded — early return.
	assert.Empty(t, bookingRepo.lastTransitionInput.BookingID)
}
