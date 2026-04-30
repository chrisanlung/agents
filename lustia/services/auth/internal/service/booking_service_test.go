package service

import (
	"context"
	"io"
	"testing"
	"time"

	"github.com/chrisanlung/lustia-auth/internal/constants"
	"github.com/chrisanlung/lustia-auth/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// Stubs / fakes
// ---------------------------------------------------------------------------

// stubClock returns a fixed time.
type stubClock struct{ t time.Time }

func (c *stubClock) Now() time.Time { return c.t }

// stubAudit swallows everything.
type stubAudit struct{}

func (a *stubAudit) Append(_ context.Context, _ AuditEntry) error { return nil }

// stubEmail swallows everything.
type stubEmail struct{}

func (e *stubEmail) Send(_ context.Context, _ EmailMessage) error { return nil }

// stubTx is a no-op TxManager that ignores SetTenantContext calls.
type stubTx struct{}

func (s *stubTx) WithTx(ctx context.Context, fn func(context.Context) error) error {
	return fn(ctx)
}
func (s *stubTx) SetTenantContext(_ context.Context, _, _ string) error { return nil }

// stubStorage adalah no-op Storage untuk unit test — tidak ada upload sungguhan.
type stubStorage struct{}

func (s *stubStorage) Upload(_ context.Context, _ string, _ io.Reader, _ string) error {
	return nil
}
func (s *stubStorage) Delete(_ context.Context, _ string) error { return nil }
func (s *stubStorage) URL(_ context.Context, key string) (string, error) {
	return "http://localhost/uploads/" + key, nil
}

// stubTenantRepo adalah no-op TenantRepository untuk unit test.
type stubTenantRepo struct {
	tenant *model.Tenant
	err    error
}

func (r *stubTenantRepo) FindBySlug(_ context.Context, _ string) (*model.Tenant, error) {
	return r.tenant, r.err
}
func (r *stubTenantRepo) FindByID(_ context.Context, _ string) (*model.Tenant, error) {
	return r.tenant, r.err
}
func (r *stubTenantRepo) Save(_ context.Context, _ *model.Tenant) error { return nil }
func (r *stubTenantRepo) List(_ context.Context, _ TenantFilter) ([]*TenantWithCounts, int64, error) {
	return nil, 0, nil
}
func (r *stubTenantRepo) UpdateStatus(_ context.Context, _, _, _ string, _ *string) error {
	return nil
}
func (r *stubTenantRepo) CountActiveBranches(_ context.Context, _ string) (int, error) {
	return 0, nil
}

// stubPaymentSvc is a minimal PaymentServiceIface stub for booking service tests.
// Phase 6: BookingService no longer calls CreateTransaction/HandleNotification
// directly — it delegates to PaymentServiceIface.
type stubPaymentSvc struct {
	initiateErr error
	webhookErr  error
}

func (p *stubPaymentSvc) InitiateForBooking(_ context.Context, _, _ string, _ int64, _, _, _, _, _ string) (InitiatePaymentOutput, error) {
	if p.initiateErr != nil {
		return InitiatePaymentOutput{}, p.initiateErr
	}
	return InitiatePaymentOutput{
		TransactionID:     "stub-txn-id",
		ProviderReference: "stub-ref",
		QRString:          "00020101...STUB",
		QRImageURL:        "http://localhost/qr.png",
		QRExpiresAt:       time.Now().Add(15 * time.Minute),
	}, nil
}

func (p *stubPaymentSvc) HandleWebhook(_ context.Context, _ []byte, _ map[string]string) error {
	return p.webhookErr
}

func (p *stubPaymentSvc) GetStatus(_ context.Context, _ string) (PaymentStatusView, error) {
	return PaymentStatusView{Status: "awaiting"}, nil
}

func (p *stubPaymentSvc) RetryQR(_ context.Context, _ string) (InitiatePaymentOutput, error) {
	return InitiatePaymentOutput{}, nil
}

func (p *stubPaymentSvc) GetTenantBalance(_ context.Context, _ string) (BalanceSummary, error) {
	return BalanceSummary{}, nil
}

func (p *stubPaymentSvc) SweepExpiredTransactions(_ context.Context) (int, error) {
	return 0, nil
}

// ---------------------------------------------------------------------------
// Stub repositories
// ---------------------------------------------------------------------------

type stubBranchRepo struct {
	branch *model.Branch
	err    error
}

func (r *stubBranchRepo) FindByID(_ context.Context, _ string) (*model.Branch, error) {
	return r.branch, r.err
}
func (r *stubBranchRepo) FindByTenant(_ context.Context, _ string, _ BranchFilter) ([]*model.Branch, int64, error) {
	if r.branch != nil {
		return []*model.Branch{r.branch}, 1, nil
	}
	return nil, 0, nil
}
func (r *stubBranchRepo) Save(_ context.Context, _ *model.Branch) error          { return nil }
func (r *stubBranchRepo) Update(_ context.Context, _ *model.Branch) error         { return nil }
func (r *stubBranchRepo) UpdateStatus(_ context.Context, _, _ string, _ *time.Time) error {
	return nil
}
func (r *stubBranchRepo) SoftDelete(_ context.Context, _ string) error { return nil }

type stubServiceRepo struct {
	svc *model.ServiceCatalog
	err error
}

func (r *stubServiceRepo) FindByID(_ context.Context, _ string) (*model.ServiceCatalog, error) {
	return r.svc, r.err
}
func (r *stubServiceRepo) Save(_ context.Context, _ *model.ServiceCatalog) error { return nil }
func (r *stubServiceRepo) FindByTenant(_ context.Context, _ string, _ ServiceFilter) ([]*model.ServiceCatalog, int64, error) {
	return nil, 0, nil
}
func (r *stubServiceRepo) Update(_ context.Context, _ *model.ServiceCatalog) error { return nil }
func (r *stubServiceRepo) UpdateStatus(_ context.Context, _ string, _ bool) error   { return nil }
func (r *stubServiceRepo) SoftDelete(_ context.Context, _ string) error              { return nil }
func (r *stubServiceRepo) FindByIDs(_ context.Context, _ string, _ []string) ([]*model.ServiceCatalog, error) {
	return nil, nil
}

type stubAddonRepo struct {
	addons map[string]*model.Addon
}

func (r *stubAddonRepo) FindByID(_ context.Context, id string) (*model.Addon, error) {
	if a, ok := r.addons[id]; ok {
		return a, nil
	}
	return nil, constants.ErrAddonNotFound
}
func (r *stubAddonRepo) FindByIDs(_ context.Context, ids []string) ([]*model.Addon, error) {
	var out []*model.Addon
	for _, id := range ids {
		if a, ok := r.addons[id]; ok {
			out = append(out, a)
		}
	}
	return out, nil
}
func (r *stubAddonRepo) Save(_ context.Context, _ *model.Addon) error { return nil }
func (r *stubAddonRepo) FindByTenant(_ context.Context, _ string, _ AddonFilter) ([]*model.Addon, int64, error) {
	return nil, 0, nil
}
func (r *stubAddonRepo) Update(_ context.Context, _ *model.Addon) error { return nil }
func (r *stubAddonRepo) UpdateStatus(_ context.Context, _ string, _ bool, _ string) error {
	return nil
}
func (r *stubAddonRepo) SoftDelete(_ context.Context, _ string, _ string) error { return nil }
func (r *stubAddonRepo) BulkUpdateSortOrder(_ context.Context, _ []AddonSortOrderItem) error {
	return nil
}

type stubRoomRepo struct {
	rooms []*model.Room
}

func (r *stubRoomRepo) FindByID(_ context.Context, id string) (*model.Room, error) {
	for _, rm := range r.rooms {
		if rm.ID == id {
			return rm, nil
		}
	}
	return nil, constants.ErrRoomNotFound
}
func (r *stubRoomRepo) FindByIDs(_ context.Context, _ []string) ([]*model.Room, error) {
	return nil, nil
}
func (r *stubRoomRepo) FindByTenant(_ context.Context, _ string, _ RoomFilter) ([]*model.Room, int64, error) {
	return r.rooms, int64(len(r.rooms)), nil
}
func (r *stubRoomRepo) Save(_ context.Context, _ *model.Room) error { return nil }
func (r *stubRoomRepo) Update(_ context.Context, _ *model.Room) error { return nil }
func (r *stubRoomRepo) UpdateStatus(_ context.Context, _ string, _ bool, _ string) error {
	return nil
}
func (r *stubRoomRepo) SoftDelete(_ context.Context, _ string, _ string) error { return nil }
func (r *stubRoomRepo) BulkUpdateSortOrder(_ context.Context, _ []RoomSortOrderItem) error {
	return nil
}
func (r *stubRoomRepo) UpdatePhotoKey(_ context.Context, _ string, _ *string, _ string) (*string, error) {
	return nil, nil
}

type stubTherapistRepo struct {
	therapists []*model.Therapist
}

func (r *stubTherapistRepo) FindByID(_ context.Context, id string) (*model.Therapist, error) {
	for _, t := range r.therapists {
		if t.ID == id {
			return t, nil
		}
	}
	return nil, constants.ErrTherapistNotFound
}
func (r *stubTherapistRepo) FindByTenant(_ context.Context, _ string, _ TherapistFilter) ([]*model.Therapist, int64, error) {
	return r.therapists, int64(len(r.therapists)), nil
}
func (r *stubTherapistRepo) Save(_ context.Context, _ *model.Therapist) error { return nil }
func (r *stubTherapistRepo) Update(_ context.Context, _ *model.Therapist) error { return nil }
func (r *stubTherapistRepo) UpdateStatus(_ context.Context, _ string, _ bool) error { return nil }
func (r *stubTherapistRepo) SoftDelete(_ context.Context, _ string) error            { return nil }
func (r *stubTherapistRepo) UpdatePhotoKey(_ context.Context, _ string, _ *string, _ string) (*string, error) {
	return nil, nil
}

type stubTherapistSvcRepo struct {
	mappings []*model.TherapistService
}

func (r *stubTherapistSvcRepo) FindByTherapistID(_ context.Context, _ string) ([]*model.TherapistService, error) {
	return r.mappings, nil
}
func (r *stubTherapistSvcRepo) FindByServiceID(_ context.Context, _ string) ([]*model.TherapistService, error) {
	return nil, nil
}
func (r *stubTherapistSvcRepo) ReconcileForTherapist(_ context.Context, _, _, _ string, _ []string) error {
	return nil
}
func (r *stubTherapistSvcRepo) DeactivateAllForTherapist(_ context.Context, _ string) error {
	return nil
}

type stubAvailabilityRepo struct{}

func (r *stubAvailabilityRepo) FindByTherapistID(_ context.Context, _ string) ([]*model.TherapistAvailability, error) {
	return nil, nil
}
func (r *stubAvailabilityRepo) ReplaceAllForTherapist(_ context.Context, _ string, _ []*model.TherapistAvailability) error {
	return nil
}

// stubBookingRepo is the central booking repository stub.
// It supports configurable behavior per method.
type stubBookingRepo struct {
	saveErr               error
	findByIDResult        *model.Booking
	findByIDErr           error
	findByCodeResult      *model.Booking
	findByCodeErr         error
	findByCodePublicErr   error
	findPaymentRefResult  *model.Booking
	transitionRowsAffected int64
	transitionErr         error
	sweepCount            int
	addons                []*model.BookingAddon
	reportResult          BookingReportSummary
}

func (r *stubBookingRepo) Save(_ context.Context, _ *model.Booking) error {
	return r.saveErr
}
func (r *stubBookingRepo) SaveAddons(_ context.Context, _ []*model.BookingAddon) error {
	return nil
}
func (r *stubBookingRepo) FindByID(_ context.Context, _ string) (*model.Booking, error) {
	return r.findByIDResult, r.findByIDErr
}
func (r *stubBookingRepo) FindByCode(_ context.Context, _ string) (*model.Booking, error) {
	return r.findByCodeResult, r.findByCodeErr
}
func (r *stubBookingRepo) FindByCodePublic(_ context.Context, code string) (*model.Booking, error) {
	if code == "" {
		return nil, constants.ErrBookingNotFound
	}
	return r.findByCodeResult, r.findByCodePublicErr
}
func (r *stubBookingRepo) FindAddonsByBooking(_ context.Context, _ string) ([]*model.BookingAddon, error) {
	return r.addons, nil
}
func (r *stubBookingRepo) FindByTenant(_ context.Context, _ string, _ BookingFilter) ([]*model.Booking, int64, error) {
	if r.findByIDResult != nil {
		return []*model.Booking{r.findByIDResult}, 1, nil
	}
	return nil, 0, nil
}
func (r *stubBookingRepo) TransitionStatus(_ context.Context, _ TransitionStatusInput) (int64, error) {
	return r.transitionRowsAffected, r.transitionErr
}
func (r *stubBookingRepo) SweepExpired(_ context.Context) (int, error) {
	return r.sweepCount, nil
}
func (r *stubBookingRepo) FindByPaymentReference(_ context.Context, _ string) (*model.Booking, error) {
	return r.findPaymentRefResult, nil
}
func (r *stubBookingRepo) ReportSummary(_ context.Context, _ BookingReportFilter) (BookingReportSummary, error) {
	return r.reportResult, nil
}

// ---------------------------------------------------------------------------
// Test helpers
// ---------------------------------------------------------------------------

func newTestService(
	bookingRepo *stubBookingRepo,
	branchRepo BranchRepository,
	svcRepo ServiceCatalogRepository,
	addonRepo AddonRepository,
	roomRepo RoomRepository,
	therapistRepo TherapistRepository,
	therapistSvcRepo TherapistServiceRepository,
	paymentSvc PaymentServiceIface,
) *BookingService {
	return NewBookingService(
		bookingRepo,
		branchRepo,
		svcRepo,
		addonRepo,
		roomRepo,
		therapistRepo,
		therapistSvcRepo,
		&stubAvailabilityRepo{},
		paymentSvc,
		&stubAudit{},
		&stubEmail{},
		&stubClock{t: time.Date(2026, 4, 25, 10, 0, 0, 0, time.UTC)},
		&stubTx{},
		&stubStorage{},
		&stubTenantRepo{},
	)
}

const (
	testTenantID  = "aaaaaaaa-0000-0000-0000-000000000001"
	testBranchID  = "bbbbbbbb-0000-0000-0000-000000000002"
	testServiceID = "cccccccc-0000-0000-0000-000000000003"
	testAddonID   = "dddddddd-0000-0000-0000-000000000004"
	testRoomID    = "eeeeeeee-0000-0000-0000-000000000005"
	testTherapist = "ffffffff-0000-0000-0000-000000000006"
)

func testBranch() *model.Branch {
	return &model.Branch{
		ID:       testBranchID,
		TenantID: testTenantID,
		Name:     "Lustia Utama",
		Status:   model.BranchStatusActive,
	}
}

func testService() *model.ServiceCatalog {
	return &model.ServiceCatalog{
		ID:              testServiceID,
		TenantID:        testTenantID,
		Name:            "Pijat Relaksasi",
		DurationMinutes: 60,
		PriceIDR:        150000,
		IsActive:        true,
	}
}

func testAddon() *model.Addon {
	return &model.Addon{
		ID:       testAddonID,
		TenantID: testTenantID,
		Name:     "Aromaterapi",
		PriceIDR: 30000,
		IsActive: true,
	}
}

func testRoom() *model.Room {
	return &model.Room{
		ID:       testRoomID,
		TenantID: testTenantID,
		BranchID: testBranchID,
		Name:     "Ruangan A",
		IsActive: true,
	}
}

func testTherapistModel() *model.Therapist {
	return &model.Therapist{
		ID:       testTherapist,
		TenantID: testTenantID,
		BranchID: testBranchID,
		FullName: "Sari Dewi",
		IsActive: true,
	}
}

func testTherapistMapping() *model.TherapistService {
	return &model.TherapistService{
		TherapistID: testTherapist,
		ServiceID:   testServiceID,
		IsActive:    true,
	}
}

// ---------------------------------------------------------------------------
// C-1: Price computed server-side — no client total accepted
// ---------------------------------------------------------------------------

func TestCreatePublic_C1_PriceComputedServerSide(t *testing.T) {
	t.Parallel()

	bookingRepo := &stubBookingRepo{
		// FindByCode used in generateUniqueCode — return not-found so code is accepted.
		findByCodeErr: constants.ErrBookingNotFound,
	}
	svc := newTestService(
		bookingRepo,
		&stubBranchRepo{branch: testBranch()},
		&stubServiceRepo{svc: testService()},
		&stubAddonRepo{addons: map[string]*model.Addon{testAddonID: testAddon()}},
		&stubRoomRepo{rooms: []*model.Room{testRoom()}},
		&stubTherapistRepo{therapists: []*model.Therapist{testTherapistModel()}},
		&stubTherapistSvcRepo{mappings: []*model.TherapistService{testTherapistMapping()}},
		&stubPaymentSvc{},
	)

	out, err := svc.CreatePublic(context.Background(), PublicCreateBookingInput{
		BranchID:       testBranchID,
		ServiceID:      testServiceID,
		AddonIDs:       []string{testAddonID},
		ScheduledStart: "2026-04-26T10:00:00Z",
		CustomerName:   "Budi Santoso",
		CustomerPhone:  "081234567890",
		CustomerEmail:  "budi@example.com",
		// NOTE: No TotalPriceIDR field — C-1: the DTO does not have this field.
		// Server must compute: 150000 (service) + 30000 (addon) = 180000.
	})

	require.NoError(t, err)
	// C-1: verify server-computed total, not any client value.
	assert.Equal(t, int64(180000), out.TotalPriceIDR,
		"C-1: total_price_idr must be server-computed (service + addons)")
}

// ---------------------------------------------------------------------------
// H-1: Cross-tenant FK validation
// ---------------------------------------------------------------------------

func TestCreatePublic_H1_CrossTenantServiceRejected(t *testing.T) {
	t.Parallel()

	otherTenantService := testService()
	otherTenantService.TenantID = "other-tenant-uuid"

	bookingRepo := &stubBookingRepo{findByCodeErr: constants.ErrBookingNotFound}
	svc := newTestService(
		bookingRepo,
		&stubBranchRepo{branch: testBranch()},
		&stubServiceRepo{svc: otherTenantService}, // service belongs to a different tenant
		&stubAddonRepo{addons: map[string]*model.Addon{}},
		&stubRoomRepo{},
		&stubTherapistRepo{},
		&stubTherapistSvcRepo{},
		&stubPaymentSvc{},
	)

	_, err := svc.CreatePublic(context.Background(), PublicCreateBookingInput{
		BranchID:       testBranchID,
		ServiceID:      testServiceID,
		ScheduledStart: "2026-04-26T10:00:00Z",
		CustomerName:   "Test",
		CustomerPhone:  "08123",
		CustomerEmail:  "test@example.com",
	})

	// H-1: cross-tenant → 404 (not 422 or 409) to avoid oracle.
	assert.ErrorIs(t, err, constants.ErrServiceNotFound,
		"H-1: cross-tenant service_id must return 404")
}

func TestCreatePublic_H1_CrossTenantAddonRejected(t *testing.T) {
	t.Parallel()

	otherTenantAddon := testAddon()
	otherTenantAddon.TenantID = "other-tenant-uuid"

	bookingRepo := &stubBookingRepo{findByCodeErr: constants.ErrBookingNotFound}
	svc := newTestService(
		bookingRepo,
		&stubBranchRepo{branch: testBranch()},
		&stubServiceRepo{svc: testService()},
		&stubAddonRepo{addons: map[string]*model.Addon{testAddonID: otherTenantAddon}},
		&stubRoomRepo{},
		&stubTherapistRepo{},
		&stubTherapistSvcRepo{},
		&stubPaymentSvc{},
	)

	_, err := svc.CreatePublic(context.Background(), PublicCreateBookingInput{
		BranchID:       testBranchID,
		ServiceID:      testServiceID,
		AddonIDs:       []string{testAddonID},
		ScheduledStart: "2026-04-26T10:00:00Z",
		CustomerName:   "Test",
		CustomerPhone:  "08123",
		CustomerEmail:  "test@example.com",
	})

	assert.ErrorIs(t, err, constants.ErrAddonNotFound,
		"H-1: cross-tenant addon_id must return 404")
}

// ---------------------------------------------------------------------------
// H-5: Expiry sweep race condition
// ---------------------------------------------------------------------------

// TestWebhook_BookingService_DelegatesTo_PaymentService verifies that the
// BookingService.HandlePaymentWebhook shim delegates to the injected
// PaymentServiceIface without error. Full H-4/H-5 coverage lives in
// payment_service_test.go.
func TestWebhook_BookingService_DelegatesTo_PaymentService(t *testing.T) {
	t.Parallel()

	bookingRepo := &stubBookingRepo{}
	svc := newTestService(
		bookingRepo,
		&stubBranchRepo{branch: testBranch()},
		&stubServiceRepo{svc: testService()},
		&stubAddonRepo{addons: map[string]*model.Addon{}},
		&stubRoomRepo{},
		&stubTherapistRepo{},
		&stubTherapistSvcRepo{},
		&stubPaymentSvc{},
	)

	err := svc.HandlePaymentWebhook(context.Background(),
		[]byte(`{"provider_reference":"ref","amount_idr":150000}`),
		map[string]string{},
	)
	assert.NoError(t, err, "booking service webhook shim must delegate cleanly")
}

// ---------------------------------------------------------------------------
// H-6: Concierge branch-scope check
// ---------------------------------------------------------------------------

func TestCreateConcierge_H6_BranchScopeRejected(t *testing.T) {
	t.Parallel()

	bookingRepo := &stubBookingRepo{findByCodeErr: constants.ErrBookingNotFound}
	svc := newTestService(
		bookingRepo,
		&stubBranchRepo{branch: testBranch()},
		&stubServiceRepo{svc: testService()},
		&stubAddonRepo{addons: map[string]*model.Addon{}},
		&stubRoomRepo{},
		&stubTherapistRepo{},
		&stubTherapistSvcRepo{},
		&stubPaymentSvc{},
	)

	_, err := svc.CreateConcierge(context.Background(), ConciergeCreateBookingInput{
		CallerUserID:   "user-1",
		CallerTenantID: testTenantID,
		CallerBranches: []string{"other-branch-id"}, // caller cannot access testBranchID
		IsAdmin:        false,
		BranchID:       testBranchID, // different from CallerBranches
		ServiceID:      testServiceID,
		ScheduledStart: "2026-04-26T10:00:00Z",
		CustomerName:   "Test",
		CustomerPhone:  "08123",
		CustomerEmail:  "test@example.com",
	})

	assert.ErrorIs(t, err, constants.ErrCrossBranchForbidden,
		"H-6: branch_admin cannot book at a branch not in their JWT branches claim")
}

func TestCreateConcierge_H6_AdminBypassesBranchScope(t *testing.T) {
	t.Parallel()

	bookingRepo := &stubBookingRepo{findByCodeErr: constants.ErrBookingNotFound}
	svc := newTestService(
		bookingRepo,
		&stubBranchRepo{branch: testBranch()},
		&stubServiceRepo{svc: testService()},
		&stubAddonRepo{addons: map[string]*model.Addon{}},
		&stubRoomRepo{rooms: []*model.Room{testRoom()}},
		&stubTherapistRepo{therapists: []*model.Therapist{testTherapistModel()}},
		&stubTherapistSvcRepo{mappings: []*model.TherapistService{testTherapistMapping()}},
		&stubPaymentSvc{},
	)

	_, err := svc.CreateConcierge(context.Background(), ConciergeCreateBookingInput{
		CallerUserID:   "admin-user",
		CallerTenantID: testTenantID,
		CallerBranches: []string{}, // empty — but IsAdmin=true
		IsAdmin:        true,
		BranchID:       testBranchID,
		ServiceID:      testServiceID,
		ScheduledStart: "2026-04-26T10:00:00Z",
		CustomerName:   "Test Admin",
		CustomerPhone:  "08123",
		CustomerEmail:  "test@example.com",
	})
	// tenant_admin should succeed regardless of CallerBranches.
	assert.NoError(t, err, "H-6: tenant_admin (IsAdmin=true) must bypass branch-scope check")
}

// ---------------------------------------------------------------------------
// H-7: FindByCodePublic must require non-empty code
// ---------------------------------------------------------------------------

func TestGetPublicByCode_H7_EmptyCodeRejected(t *testing.T) {
	t.Parallel()

	bookingRepo := &stubBookingRepo{}
	svc := newTestService(
		bookingRepo,
		&stubBranchRepo{branch: testBranch()},
		&stubServiceRepo{svc: testService()},
		&stubAddonRepo{addons: map[string]*model.Addon{}},
		&stubRoomRepo{},
		&stubTherapistRepo{},
		&stubTherapistSvcRepo{},
		&stubPaymentSvc{},
	)

	_, err := svc.GetPublicByCode(context.Background(), "")
	// H-7: empty code must be rejected — no unbounded scan.
	assert.Error(t, err, "H-7: empty code must be rejected")
}

func TestGetPublicByCode_H7_ValidCodeReturnsView(t *testing.T) {
	t.Parallel()

	booking := &model.Booking{
		ID:             "bk-public",
		TenantID:       testTenantID,
		BranchID:       testBranchID,
		ServiceID:      testServiceID,
		CustomerName:   "Budi",
		CustomerPhone:  "081234567890",
		CustomerEmail:  "budi@example.com",
		Code:           "ABCD-EFGH",
		ScheduledStart: time.Date(2026, 4, 26, 10, 0, 0, 0, time.UTC),
		ScheduledEnd:   time.Date(2026, 4, 26, 11, 0, 0, 0, time.UTC),
		TotalPriceIDR:  150000,
		Status:         model.BookingStatusPaid,
	}

	bookingRepo := &stubBookingRepo{
		findByCodeResult:    booking,
		findByCodePublicErr: nil,
	}
	svc := newTestService(
		bookingRepo,
		&stubBranchRepo{branch: testBranch()},
		&stubServiceRepo{svc: testService()},
		&stubAddonRepo{addons: map[string]*model.Addon{}},
		&stubRoomRepo{},
		&stubTherapistRepo{},
		&stubTherapistSvcRepo{},
		&stubPaymentSvc{},
	)

	view, err := svc.GetPublicByCode(context.Background(), "ABCD-EFGH")
	require.NoError(t, err)
	assert.Equal(t, "ABCD-EFGH", view.Code)

	// M-2: Verify PII masking.
	assert.Equal(t, "****7890", view.CustomerPhone, "M-2: phone must be masked")
	assert.Equal(t, "bu**@example.com", view.CustomerEmail, "M-2: email must be masked")
	// Full name is NOT masked.
	assert.Equal(t, "Budi", view.CustomerName)
}

// ---------------------------------------------------------------------------
// Code generation
// ---------------------------------------------------------------------------

func TestGenerateBookingCode_Format(t *testing.T) {
	t.Parallel()

	bookingRepo := &stubBookingRepo{findByCodeErr: constants.ErrBookingNotFound}
	svc := newTestService(
		bookingRepo,
		&stubBranchRepo{},
		&stubServiceRepo{},
		&stubAddonRepo{addons: map[string]*model.Addon{}},
		&stubRoomRepo{},
		&stubTherapistRepo{},
		&stubTherapistSvcRepo{},
		&stubPaymentSvc{},
	)

	code, err := svc.generateUniqueCode(context.Background())
	require.NoError(t, err)
	assert.Len(t, code, 9, "code must be 9 chars: XXXX-XXXX")
	assert.Equal(t, '-', rune(code[4]), "code must have hyphen at position 4")

	// Validate all characters are from [A-Z2-7] (Crockford Base32 compatible).
	for i, ch := range code {
		if i == 4 {
			continue // skip hyphen
		}
		assert.True(t,
			(ch >= 'A' && ch <= 'Z') || (ch >= '2' && ch <= '7'),
			"code char %q at position %d must be in [A-Z2-7]", ch, i,
		)
	}
}

// ---------------------------------------------------------------------------
// Price masking helpers
// ---------------------------------------------------------------------------

func TestMaskPhone(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in   string
		want string
	}{
		{"+62812345678", "****5678"},
		{"08123", "****8123"},
		{"1234", "****"},
		{"12", "****"},
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			assert.Equal(t, tc.want, maskPhone(tc.in))
		})
	}
}

func TestMaskEmail(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in   string
		want string
	}{
		{"firstname@example.com", "fi**@example.com"},
		{"ab@example.com", "ab**@example.com"},
		{"a@example.com", "a**@example.com"},
		{"notanemail", "****@****"},
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			assert.Equal(t, tc.want, maskEmail(tc.in))
		})
	}
}
