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

// stubBookingRepoForPayment is a minimal BookingRepository for payment tests.
type stubBookingRepoForPayment struct {
	findByCodePublicResult *model.Booking
	findByCodePublicErr    error
	findByIDResult         *model.Booking
	transitionRows         int64
	transitionErr          error
}

func (r *stubBookingRepoForPayment) Save(_ context.Context, _ *model.Booking) error { return nil }
func (r *stubBookingRepoForPayment) SaveAddons(_ context.Context, _ []*model.BookingAddon) error {
	return nil
}
func (r *stubBookingRepoForPayment) FindByID(_ context.Context, _ string) (*model.Booking, error) {
	return r.findByIDResult, nil
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
func (r *stubBookingRepoForPayment) TransitionStatus(_ context.Context, _ TransitionStatusInput) (int64, error) {
	return r.transitionRows, r.transitionErr
}
func (r *stubBookingRepoForPayment) SweepExpired(_ context.Context) (int, error) { return 0, nil }
func (r *stubBookingRepoForPayment) FindByPaymentReference(_ context.Context, _ string) (*model.Booking, error) {
	return nil, constants.ErrBookingNotFound
}
func (r *stubBookingRepoForPayment) ReportSummary(_ context.Context, _ BookingReportFilter) (BookingReportSummary, error) {
	return BookingReportSummary{}, nil
}

// stubPaymentProvider is a minimal PaymentProvider stub.
type stubPaymentProvider struct {
	createQRResp CreateQRResponse
	createQRErr  error
	webhookNotif PaymentNotification
	webhookErr   error
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
	return ProviderStatusPaid, nil
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
