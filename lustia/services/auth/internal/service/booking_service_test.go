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
	initiateErr      error
	webhookErr       error
	voidCalledCount  int
	voidErr          error
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

func (p *stubPaymentSvc) VoidTransactionForBooking(_ context.Context, _ string) error {
	p.voidCalledCount++
	return p.voidErr
}

func (p *stubPaymentSvc) SyncStatus(_ context.Context, _ SyncPaymentInput) (SyncPaymentResult, error) {
	return SyncPaymentResult{ActionTaken: "no_op"}, nil
}

func (p *stubPaymentSvc) GetProviderRefByCode(_ context.Context, _ string) (DummyTriggerLookup, error) {
	return DummyTriggerLookup{}, nil
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

type stubAvailabilityRepo struct {
	// coversSlot, when non-nil, is called for TherapistCoversSlot. When nil,
	// the default behaviour is to return true (therapist covers every slot).
	coversSlot func(therapistID string, dayOfWeek int, slotStart string, slotEnd string) bool
	// windows, when non-nil, is the explicit slice returned by
	// FindByTherapistsAndDOW. Set this on tests that need precise per-slot
	// availability control.
	//
	// When nil (default), FindByTherapistsAndDOW synthesises a full-day window
	// (00:00:00–23:59:59) for every therapist ID in the request, which means
	// every slot is covered by every therapist. This matches the pre-fix
	// behaviour where therapistsAvailTotal was non-zero for any mapped therapist.
	windows []*model.TherapistAvailability
}

func (r *stubAvailabilityRepo) FindByTherapistID(_ context.Context, _ string) ([]*model.TherapistAvailability, error) {
	return nil, nil
}
func (r *stubAvailabilityRepo) ReplaceAllForTherapist(_ context.Context, _ string, _ []*model.TherapistAvailability) error {
	return nil
}
func (r *stubAvailabilityRepo) TherapistCoversSlot(_ context.Context, therapistID string, dayOfWeek int, slotStart, slotEnd string) (bool, error) {
	if r.coversSlot != nil {
		return r.coversSlot(therapistID, dayOfWeek, slotStart, slotEnd), nil
	}
	return true, nil
}
func (r *stubAvailabilityRepo) FindByTherapistsAndDOW(_ context.Context, therapistIDs []string, dayOfWeek int) ([]*model.TherapistAvailability, error) {
	if r.windows != nil {
		return r.windows, nil
	}
	// Default: synthesise a full-day window for every requested therapist so
	// that therapistCoveredByWindow returns true for all slots, preserving the
	// pre-fix default where all therapists were treated as always available.
	out := make([]*model.TherapistAvailability, 0, len(therapistIDs))
	for _, id := range therapistIDs {
		out = append(out, &model.TherapistAvailability{
			TherapistID: id,
			DayOfWeek:   dayOfWeek,
			StartTime:   "00:00:00",
			EndTime:     "23:59:59",
		})
	}
	return out, nil
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
	// bookedRoomsBySlot maps RFC3339 slot-start → room UUIDs that are booked.
	// nil means no rooms booked at any slot (default).
	bookedRoomsBySlot map[string][]string
	// therapistBookedAtSlot maps RFC3339 slot-start → true when the therapist
	// has a conflicting booking at that slot.
	therapistBookedAtSlot map[string]bool
	// therapistConflicts is the slice returned by FindTherapistConflicts.
	// nil means no conflicts (default — all therapists are free for the day).
	therapistConflicts []TherapistBookingInterval
	sweepCount         int
	addons             []*model.BookingAddon
	reportResult       BookingReportSummary
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
func (r *stubBookingRepo) FindBookedRoomIDsInSlots(_ context.Context, _ string, windows []SlotWindow) (map[string][]string, error) {
	result := make(map[string][]string, len(windows))
	for _, w := range windows {
		// If the test configured bookedRoomsBySlot, return it; otherwise empty.
		if r.bookedRoomsBySlot != nil {
			result[w.Start] = r.bookedRoomsBySlot[w.Start]
		} else {
			result[w.Start] = nil
		}
	}
	return result, nil
}
func (r *stubBookingRepo) IsTherapistBookedInSlots(_ context.Context, _ string, windows []SlotWindow, _ int) (map[string]bool, error) {
	result := make(map[string]bool, len(windows))
	for _, w := range windows {
		if r.therapistBookedAtSlot != nil {
			result[w.Start] = r.therapistBookedAtSlot[w.Start]
		}
	}
	return result, nil
}
func (r *stubBookingRepo) FindTherapistConflicts(_ context.Context, _ []string, _, _ time.Time) ([]TherapistBookingInterval, error) {
	if r.therapistConflicts != nil {
		return r.therapistConflicts, nil
	}
	return []TherapistBookingInterval{}, nil
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
	return newTestServiceWithAvailability(bookingRepo, branchRepo, svcRepo, addonRepo, roomRepo, therapistRepo, therapistSvcRepo, paymentSvc, &stubAvailabilityRepo{})
}

// newTestServiceWithAvailability is like newTestService but accepts a custom
// TherapistAvailabilityRepository stub for availability-related tests.
func newTestServiceWithAvailability(
	bookingRepo *stubBookingRepo,
	branchRepo BranchRepository,
	svcRepo ServiceCatalogRepository,
	addonRepo AddonRepository,
	roomRepo RoomRepository,
	therapistRepo TherapistRepository,
	therapistSvcRepo TherapistServiceRepository,
	paymentSvc PaymentServiceIface,
	availRepo TherapistAvailabilityRepository,
) *BookingService {
	return NewBookingService(
		bookingRepo,
		branchRepo,
		svcRepo,
		addonRepo,
		roomRepo,
		therapistRepo,
		therapistSvcRepo,
		availRepo,
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

// ---------------------------------------------------------------------------
// capturingAudit records every appended AuditEntry for later assertion.
// ---------------------------------------------------------------------------

type capturingAudit struct {
	entries []AuditEntry
}

func (a *capturingAudit) Append(_ context.Context, e AuditEntry) error {
	a.entries = append(a.entries, e)
	return nil
}

// newTestServiceWithAudit is like newTestService but injects a capturingAudit.
func newTestServiceWithAudit(
	bookingRepo *stubBookingRepo,
	paymentSvc PaymentServiceIface,
	audit *capturingAudit,
) *BookingService {
	return NewBookingService(
		bookingRepo,
		&stubBranchRepo{branch: testBranch()},
		&stubServiceRepo{svc: testService()},
		&stubAddonRepo{addons: map[string]*model.Addon{}},
		&stubRoomRepo{},
		&stubTherapistRepo{},
		&stubTherapistSvcRepo{},
		&stubAvailabilityRepo{},
		paymentSvc,
		audit,
		&stubEmail{},
		&stubClock{t: time.Date(2026, 4, 25, 10, 0, 0, 0, time.UTC)},
		&stubTx{},
		&stubStorage{},
		&stubTenantRepo{},
	)
}

// ---------------------------------------------------------------------------
// Cancel — pending_payment relaxation
// ---------------------------------------------------------------------------

// TestCancel_FromPendingPayment_HappyPath verifies that a pending_payment
// booking can be cancelled, the payment_transaction is voided, the audit action
// is "booking.cancelled_pending", and previous_status is recorded in Meta.
func TestCancel_FromPendingPayment_HappyPath(t *testing.T) {
	t.Parallel()

	booking := &model.Booking{
		ID:       "bk-cancel-pending",
		TenantID: testTenantID,
		BranchID: testBranchID,
		Status:   model.BookingStatusPendingPayment,
	}
	bookingRepo := &stubBookingRepo{
		findByIDResult:         booking,
		transitionRowsAffected: 1,
	}
	paymentSvc := &stubPaymentSvc{}
	audit := &capturingAudit{}
	svc := newTestServiceWithAudit(bookingRepo, paymentSvc, audit)

	d, err := svc.Cancel(context.Background(), CancelInput{
		CallerUserID:   "op-user",
		CallerTenantID: testTenantID,
		CallerBranches: []string{testBranchID},
		IsAdmin:        false,
		BookingID:      "bk-cancel-pending",
		Reason:         "pelanggan membatalkan sebelum bayar",
	})

	require.NoError(t, err)
	assert.Equal(t, model.BookingStatusCancelled, d.Status)

	// VoidTransactionForBooking must have been called once.
	assert.Equal(t, 1, paymentSvc.voidCalledCount,
		"awaiting payment_transaction must be voided when cancelling pending_payment")

	// Audit action must be the differentiated label.
	require.Len(t, audit.entries, 1)
	entry := audit.entries[0]
	assert.Equal(t, "booking.cancelled_pending", entry.Action)
	assert.Equal(t, string(model.BookingStatusPendingPayment), entry.Meta["previous_status"],
		"previous_status must be recorded in audit Meta")
}

// TestCancel_FromPaid_DoesNotVoidTransaction verifies that cancelling a paid
// booking does NOT call VoidTransactionForBooking and uses action "booking.cancelled".
func TestCancel_FromPaid_DoesNotVoidTransaction(t *testing.T) {
	t.Parallel()

	booking := &model.Booking{
		ID:       "bk-cancel-paid",
		TenantID: testTenantID,
		BranchID: testBranchID,
		Status:   model.BookingStatusPaid,
	}
	bookingRepo := &stubBookingRepo{
		findByIDResult:         booking,
		transitionRowsAffected: 1,
	}
	paymentSvc := &stubPaymentSvc{}
	audit := &capturingAudit{}
	svc := newTestServiceWithAudit(bookingRepo, paymentSvc, audit)

	_, err := svc.Cancel(context.Background(), CancelInput{
		CallerUserID:   "op-user",
		CallerTenantID: testTenantID,
		CallerBranches: []string{testBranchID},
		BookingID:      "bk-cancel-paid",
		Reason:         "no-show setelah bayar",
	})
	require.NoError(t, err)

	assert.Equal(t, 0, paymentSvc.voidCalledCount,
		"VoidTransactionForBooking must NOT be called when cancelling a paid booking")
	require.Len(t, audit.entries, 1)
	assert.Equal(t, "booking.cancelled", audit.entries[0].Action)
}

// TestCancel_FromExpired_Rejected verifies that cancelling an expired booking
// is still rejected (status guard unchanged for non-allowed statuses).
func TestCancel_FromExpired_Rejected(t *testing.T) {
	t.Parallel()

	booking := &model.Booking{
		ID:       "bk-cancel-expired",
		TenantID: testTenantID,
		BranchID: testBranchID,
		Status:   model.BookingStatusExpired,
	}
	bookingRepo := &stubBookingRepo{findByIDResult: booking}
	svc := newTestServiceWithAudit(bookingRepo, &stubPaymentSvc{}, &capturingAudit{})

	_, err := svc.Cancel(context.Background(), CancelInput{
		CallerUserID:   "op-user",
		CallerTenantID: testTenantID,
		CallerBranches: []string{testBranchID},
		BookingID:      "bk-cancel-expired",
		Reason:         "test",
	})
	assert.ErrorIs(t, err, constants.ErrBookingInvalidStatusTransition,
		"cancelling an expired booking must be rejected")
}

// TestCancel_PendingPayment_BranchScopeEnforced verifies that a branch_admin
// cannot cancel a pending_payment booking from a branch outside their scope.
func TestCancel_PendingPayment_BranchScopeEnforced(t *testing.T) {
	t.Parallel()

	booking := &model.Booking{
		ID:       "bk-cancel-scope",
		TenantID: testTenantID,
		BranchID: "other-branch-uuid",
		Status:   model.BookingStatusPendingPayment,
	}
	bookingRepo := &stubBookingRepo{findByIDResult: booking}
	svc := newTestServiceWithAudit(bookingRepo, &stubPaymentSvc{}, &capturingAudit{})

	_, err := svc.Cancel(context.Background(), CancelInput{
		CallerUserID:   "branch-admin-user",
		CallerTenantID: testTenantID,
		CallerBranches: []string{testBranchID}, // different from booking.BranchID
		IsAdmin:        false,
		BookingID:      "bk-cancel-scope",
		Reason:         "test",
	})
	assert.ErrorIs(t, err, constants.ErrCrossBranchForbidden,
		"branch-scope guard must still apply for pending_payment cancel")
}

// TestCancel_PendingPayment_AuditIncludesPreviousStatus verifies that the
// Meta map contains previous_status for paid bookings as well (regression guard).
func TestCancel_AuditMeta_IncludesPreviousStatus(t *testing.T) {
	t.Parallel()

	for _, status := range []string{
		model.BookingStatusPaid,
		model.BookingStatusCheckedIn,
		model.BookingStatusPendingPayment,
	} {
		status := status
		t.Run(status, func(t *testing.T) {
			t.Parallel()
			booking := &model.Booking{
				ID:       "bk-meta-" + status,
				TenantID: testTenantID,
				BranchID: testBranchID,
				Status:   status,
			}
			bookingRepo := &stubBookingRepo{
				findByIDResult:         booking,
				transitionRowsAffected: 1,
			}
			audit := &capturingAudit{}
			svc := newTestServiceWithAudit(bookingRepo, &stubPaymentSvc{}, audit)

			_, err := svc.Cancel(context.Background(), CancelInput{
				CallerUserID:   "op",
				CallerTenantID: testTenantID,
				CallerBranches: []string{testBranchID},
				BookingID:      "bk-meta-" + status,
				Reason:         "test",
			})
			require.NoError(t, err)
			require.Len(t, audit.entries, 1)
			assert.Equal(t, status, audit.entries[0].Meta["previous_status"],
				"previous_status must equal the booking status before transition")
		})
	}
}

// ---------------------------------------------------------------------------
// ListAvailableSlots — availability + room-ID projection + therapist filter
// ---------------------------------------------------------------------------

// slot0Start is the RFC3339 key for the first slot of the test date (09:00 UTC).
// 2026-04-25 is a Saturday (DOW = 6 in Go's time.Weekday).
const testAvailDate = "2026-04-25"

// slotKey returns the RFC3339 start for slot index i (30-min steps from 09:00 Jakarta).
// This helper is kept for legacy tests that rely on 30-min increment addressing.
func slotKey(i int) string {
	t := time.Date(2026, 4, 25, 9, 0, 0, 0, jakartaLocation).Add(time.Duration(i) * 30 * time.Minute)
	return t.Format(time.RFC3339)
}

// chainKey returns the RFC3339 start for a specific HH:MM on the test date (Jakarta).
// Use this for chained-slot tests where start times are not multiples of 30 minutes.
func chainKey(hour, minute int) string {
	return time.Date(2026, 4, 25, hour, minute, 0, 0, jakartaLocation).Format(time.RFC3339)
}

const (
	testRoomID2 = "eeeeeeee-0000-0000-0000-000000000007"
)

// newSlotsTestService builds a BookingService wired for ListAvailableSlots tests.
func newSlotsTestService(
	bookingRepo *stubBookingRepo,
	availRepo *stubAvailabilityRepo,
	rooms []*model.Room,
) *BookingService {
	return newTestServiceWithAvailability(
		bookingRepo,
		&stubBranchRepo{branch: testBranch()},
		&stubServiceRepo{svc: testService()},
		&stubAddonRepo{addons: map[string]*model.Addon{}},
		&stubRoomRepo{rooms: rooms},
		&stubTherapistRepo{therapists: []*model.Therapist{testTherapistModel()}},
		&stubTherapistSvcRepo{mappings: []*model.TherapistService{testTherapistMapping()}},
		&stubPaymentSvc{},
		availRepo,
	)
}

func TestListAvailableSlots_NoTherapistID_AllRoomsAvailable(t *testing.T) {
	t.Parallel()

	// Two rooms, nothing booked → both appear in available_room_ids on every slot.
	rooms := []*model.Room{
		{ID: testRoomID, TenantID: testTenantID, BranchID: testBranchID, IsActive: true},
		{ID: testRoomID2, TenantID: testTenantID, BranchID: testBranchID, IsActive: true},
	}
	svc := newSlotsTestService(&stubBookingRepo{}, &stubAvailabilityRepo{}, rooms)

	slots, err := svc.ListAvailableSlots(context.Background(), AvailableSlotsInput{
		BranchID:  testBranchID,
		ServiceID: testServiceID,
		Date:      testAvailDate,
	})

	require.NoError(t, err)
	require.NotEmpty(t, slots)
	for _, s := range slots {
		// therapist_available must be absent (nil) when no TherapistID given.
		assert.Nil(t, s.TherapistAvailable, "TherapistAvailable must be nil when no therapist_id")
		assert.ElementsMatch(t, []string{testRoomID, testRoomID2}, s.AvailableRoomIDs,
			"both rooms should be in available_room_ids when nothing is booked")
		assert.Equal(t, 2, s.RoomsAvailableCount)
	}
}

func TestListAvailableSlots_NoTherapistID_OneRoomBooked(t *testing.T) {
	t.Parallel()

	// Room 1 is booked at slot 0, not at slot 1.
	rooms := []*model.Room{
		{ID: testRoomID, TenantID: testTenantID, BranchID: testBranchID, IsActive: true},
		{ID: testRoomID2, TenantID: testTenantID, BranchID: testBranchID, IsActive: true},
	}
	bookingRepo := &stubBookingRepo{
		bookedRoomsBySlot: map[string][]string{
			slotKey(0): {testRoomID}, // only slot 0
		},
	}
	svc := newSlotsTestService(bookingRepo, &stubAvailabilityRepo{}, rooms)

	slots, err := svc.ListAvailableSlots(context.Background(), AvailableSlotsInput{
		BranchID:  testBranchID,
		ServiceID: testServiceID,
		Date:      testAvailDate,
	})

	require.NoError(t, err)
	require.NotEmpty(t, slots)

	slot0 := slots[0]
	assert.NotContains(t, slot0.AvailableRoomIDs, testRoomID,
		"booked room must NOT appear in available_room_ids at slot 0")
	assert.Contains(t, slot0.AvailableRoomIDs, testRoomID2,
		"unbooked room must appear in available_room_ids at slot 0")
	assert.Equal(t, 1, slot0.RoomsAvailableCount)

	// Slot 1 has no bookings → both rooms available.
	slot1 := slots[1]
	assert.ElementsMatch(t, []string{testRoomID, testRoomID2}, slot1.AvailableRoomIDs)
}

func TestListAvailableSlots_TherapistID_AllSlotsFree(t *testing.T) {
	t.Parallel()

	// Therapist covers every slot (coversSlot always true, no bookings).
	rooms := []*model.Room{
		{ID: testRoomID, TenantID: testTenantID, BranchID: testBranchID, IsActive: true},
	}
	availRepo := &stubAvailabilityRepo{
		coversSlot: func(_ string, _ int, _, _ string) bool { return true },
	}
	svc := newSlotsTestService(&stubBookingRepo{}, availRepo, rooms)

	tid := testTherapist
	slots, err := svc.ListAvailableSlots(context.Background(), AvailableSlotsInput{
		BranchID:    testBranchID,
		ServiceID:   testServiceID,
		Date:        testAvailDate,
		TherapistID: &tid,
	})

	require.NoError(t, err)
	require.NotEmpty(t, slots)
	for _, s := range slots {
		require.NotNil(t, s.TherapistAvailable)
		assert.True(t, *s.TherapistAvailable,
			"TherapistAvailable must be true when therapist is free and schedule covers slot")
	}
}

func TestListAvailableSlots_TherapistID_BookedAtSlot0(t *testing.T) {
	t.Parallel()

	// Therapist is booked at slot 0, free at slot 1.
	rooms := []*model.Room{
		{ID: testRoomID, TenantID: testTenantID, BranchID: testBranchID, IsActive: true},
	}
	bookingRepo := &stubBookingRepo{
		therapistBookedAtSlot: map[string]bool{
			slotKey(0): true, // booked at slot 0
		},
	}
	availRepo := &stubAvailabilityRepo{
		coversSlot: func(_ string, _ int, _, _ string) bool { return true },
	}
	svc := newSlotsTestService(bookingRepo, availRepo, rooms)

	tid := testTherapist
	slots, err := svc.ListAvailableSlots(context.Background(), AvailableSlotsInput{
		BranchID:    testBranchID,
		ServiceID:   testServiceID,
		Date:        testAvailDate,
		TherapistID: &tid,
	})

	require.NoError(t, err)
	require.NotEmpty(t, slots)

	require.NotNil(t, slots[0].TherapistAvailable)
	assert.False(t, *slots[0].TherapistAvailable,
		"slot 0: therapist is booked → TherapistAvailable must be false")

	require.NotNil(t, slots[1].TherapistAvailable)
	assert.True(t, *slots[1].TherapistAvailable,
		"slot 1: therapist is free → TherapistAvailable must be true")
}

func TestListAvailableSlots_TherapistID_ScheduleDoesNotCoverDay(t *testing.T) {
	t.Parallel()

	// Therapist's schedule never covers any slot (coversSlot always false).
	rooms := []*model.Room{
		{ID: testRoomID, TenantID: testTenantID, BranchID: testBranchID, IsActive: true},
	}
	availRepo := &stubAvailabilityRepo{
		coversSlot: func(_ string, _ int, _, _ string) bool { return false },
	}
	svc := newSlotsTestService(&stubBookingRepo{}, availRepo, rooms)

	tid := testTherapist
	slots, err := svc.ListAvailableSlots(context.Background(), AvailableSlotsInput{
		BranchID:    testBranchID,
		ServiceID:   testServiceID,
		Date:        testAvailDate,
		TherapistID: &tid,
	})

	require.NoError(t, err)
	require.NotEmpty(t, slots)
	for _, s := range slots {
		require.NotNil(t, s.TherapistAvailable)
		assert.False(t, *s.TherapistAvailable,
			"all slots must be TherapistAvailable=false when schedule never covers the day")
	}
}

func TestListAvailableSlots_AggregateCounts_ReflectsPerSlotAvailability(t *testing.T) {
	t.Parallel()

	// Two rooms, one therapist mapped. The stub FindTherapistConflicts returns no
	// conflicts (default) so therapistsAvailableCount reflects availability windows
	// only. The default stubAvailabilityRepo gives a full-day window → count=1.
	// The therapistBookedAtSlot map feeds only the per-therapist TherapistAvailable
	// boolean (Pass 3), not the aggregate Pass D count.
	rooms := []*model.Room{
		{ID: testRoomID, TenantID: testTenantID, BranchID: testBranchID, IsActive: true},
		{ID: testRoomID2, TenantID: testTenantID, BranchID: testBranchID, IsActive: true},
	}
	bookingRepo := &stubBookingRepo{
		therapistBookedAtSlot: map[string]bool{slotKey(0): true},
		bookedRoomsBySlot:     map[string][]string{slotKey(0): {testRoomID}},
		// therapistConflicts nil → FindTherapistConflicts returns [] → no conflicts
		// for aggregate count; therapist counted as available in Pass D.
	}
	availRepo := &stubAvailabilityRepo{
		coversSlot: func(_ string, _ int, _, _ string) bool { return true },
		// windows nil → default full-day window synthesised in FindByTherapistsAndDOW.
	}
	svc := newSlotsTestService(bookingRepo, availRepo, rooms)

	tid := testTherapist
	slots, err := svc.ListAvailableSlots(context.Background(), AvailableSlotsInput{
		BranchID:    testBranchID,
		ServiceID:   testServiceID,
		Date:        testAvailDate,
		TherapistID: &tid,
	})

	require.NoError(t, err)
	require.NotEmpty(t, slots)

	slot0 := slots[0]
	// Aggregate: one therapist mapped, no conflicts in FindTherapistConflicts stub →
	// therapistsAvailableCount = 1 even though IsTherapistBookedInSlots says booked
	// (the two paths are independent: Pass D uses FindTherapistConflicts, Pass 3
	// uses IsTherapistBookedInSlots for the per-therapist boolean only).
	assert.Equal(t, 1, slot0.TherapistsAvailableCount,
		"TherapistsAvailableCount reflects Pass D (FindTherapistConflicts); 1 therapist free")
	// One room is booked → rooms_available_count reflects branch-minus-booked.
	assert.Equal(t, 1, slot0.RoomsAvailableCount)
	assert.NotContains(t, slot0.AvailableRoomIDs, testRoomID)
	assert.Contains(t, slot0.AvailableRoomIDs, testRoomID2)
}

// ---------------------------------------------------------------------------
// prep_minutes — migration 000035
// ---------------------------------------------------------------------------

// therapistWithPrep returns a Therapist model with PrepMinutes set.
func therapistWithPrep(prepMinutes int) *model.Therapist {
	t := testTherapistModel()
	t.PrepMinutes = prepMinutes
	return t
}

// newSlotsTestServiceWithTherapists is like newSlotsTestService but accepts an
// explicit therapist list so tests can set PrepMinutes on the therapist row.
func newSlotsTestServiceWithTherapists(
	bookingRepo *stubBookingRepo,
	availRepo *stubAvailabilityRepo,
	rooms []*model.Room,
	therapists []*model.Therapist,
) *BookingService {
	return newTestServiceWithAvailability(
		bookingRepo,
		&stubBranchRepo{branch: testBranch()},
		&stubServiceRepo{svc: testService()},
		&stubAddonRepo{addons: map[string]*model.Addon{}},
		&stubRoomRepo{rooms: rooms},
		&stubTherapistRepo{therapists: therapists},
		&stubTherapistSvcRepo{mappings: []*model.TherapistService{testTherapistMapping()}},
		&stubPaymentSvc{},
		availRepo,
	)
}

// TestListAvailableSlots_PrepMinutes10_ChainStep verifies that with prep_minutes=10
// and a 60-min service the chain step is 70 minutes, so the slot grid is
// 09:00, 10:10, 11:20, … rather than a 30-minute cadence.
//
// The 09:00 slot is marked as booked via IsTherapistBookedInSlots (TherapistAvailable=false).
// The next chain slot at 10:10 is free (TherapistAvailable=true).
// There is NO slot at 10:00 or 10:30 in the chain.
func TestListAvailableSlots_PrepMinutes10_ChainStep(t *testing.T) {
	t.Parallel()

	rooms := []*model.Room{
		{ID: testRoomID, TenantID: testTenantID, BranchID: testBranchID, IsActive: true},
	}

	// Mark 09:00 as booked (the booking body). 10:10 is intentionally absent →
	// IsTherapistBookedInSlots returns false → TherapistAvailable=true.
	bookingRepo := &stubBookingRepo{
		therapistBookedAtSlot: map[string]bool{
			chainKey(9, 0): true, // 09:00 booked
		},
	}
	availRepo := &stubAvailabilityRepo{
		coversSlot: func(_ string, _ int, _, _ string) bool { return true },
	}
	svc := newSlotsTestServiceWithTherapists(bookingRepo, availRepo, rooms, []*model.Therapist{therapistWithPrep(10)})

	tid := testTherapist
	slots, err := svc.ListAvailableSlots(context.Background(), AvailableSlotsInput{
		BranchID:    testBranchID,
		ServiceID:   testServiceID,
		Date:        testAvailDate,
		TherapistID: &tid,
	})

	require.NoError(t, err)
	require.NotEmpty(t, slots)

	// slots[0] must be at 09:00, booked → TherapistAvailable=false.
	require.NotNil(t, slots[0].TherapistAvailable)
	assert.Equal(t, chainKey(9, 0), slots[0].Start,
		"first chain slot must start at 09:00")
	assert.False(t, *slots[0].TherapistAvailable,
		"09:00 slot is booked → TherapistAvailable must be false")

	// slots[1] must be at 10:10 (09:00 + 70min step), free.
	require.True(t, len(slots) >= 2, "chain must have at least 2 slots")
	assert.Equal(t, chainKey(10, 10), slots[1].Start,
		"second chain slot must start at 10:10 (step = 60 + 10 = 70 min)")
	require.NotNil(t, slots[1].TherapistAvailable)
	assert.True(t, *slots[1].TherapistAvailable,
		"10:10 slot is free → TherapistAvailable must be true")

	// Verify that 10:00 and 10:30 are NOT in the chain (old 30-min grid positions).
	for _, s := range slots {
		assert.NotEqual(t, chainKey(10, 0), s.Start,
			"10:00 must NOT be a chain slot when prep_minutes=10 (step=70min)")
		assert.NotEqual(t, chainKey(10, 30), s.Start,
			"10:30 must NOT be a chain slot when prep_minutes=10 (step=70min)")
	}
}

// TestListAvailableSlots_PrepMinutes0_BackToBackSlots verifies that prep_minutes=0
// produces back-to-back slots (step = duration = 60min) with no gap.
// A slot immediately after a booked slot is free (TherapistAvailable=true).
func TestListAvailableSlots_PrepMinutes0_BackToBackSlots(t *testing.T) {
	t.Parallel()

	rooms := []*model.Room{
		{ID: testRoomID, TenantID: testTenantID, BranchID: testBranchID, IsActive: true},
	}

	// Mark 09:00 as booked; with prep=0 step=60min the next slot is at 10:00 (free).
	bookingRepo := &stubBookingRepo{
		therapistBookedAtSlot: map[string]bool{
			chainKey(9, 0): true, // 09:00 booked
			// 10:00 intentionally absent — prep=0 means no buffer after 09:00–10:00
		},
	}
	availRepo := &stubAvailabilityRepo{
		coversSlot: func(_ string, _ int, _, _ string) bool { return true },
	}
	svc := newSlotsTestServiceWithTherapists(bookingRepo, availRepo, rooms, []*model.Therapist{therapistWithPrep(0)})

	tid := testTherapist
	slots, err := svc.ListAvailableSlots(context.Background(), AvailableSlotsInput{
		BranchID:    testBranchID,
		ServiceID:   testServiceID,
		Date:        testAvailDate,
		TherapistID: &tid,
	})

	require.NoError(t, err)
	require.NotEmpty(t, slots)

	// slots[0] = 09:00, booked.
	assert.Equal(t, chainKey(9, 0), slots[0].Start)
	require.NotNil(t, slots[0].TherapistAvailable)
	assert.False(t, *slots[0].TherapistAvailable,
		"09:00 slot is booked → TherapistAvailable must be false")

	// slots[1] = 10:00 (step=60min, prep=0). Immediately after booked slot → free.
	require.True(t, len(slots) >= 2)
	assert.Equal(t, chainKey(10, 0), slots[1].Start,
		"second slot must start at 10:00 (step = 60 + 0 = 60 min)")
	require.NotNil(t, slots[1].TherapistAvailable)
	assert.True(t, *slots[1].TherapistAvailable,
		"prep=0: slot at exact booked_end (10:00) must be free")
}

// TestListAvailableSlots_PrepMinutes60_ChainStep verifies that prep_minutes=60
// produces a step of 120 minutes (60-min service + 60-min prep), so the chain
// is 09:00, 11:00, 13:00, … with no intermediate slots at 10:00 or 10:30.
//
// The 09:00 slot is booked; the next chain slot at 11:00 must be free.
func TestListAvailableSlots_PrepMinutes60_ChainStep(t *testing.T) {
	t.Parallel()

	rooms := []*model.Room{
		{ID: testRoomID, TenantID: testTenantID, BranchID: testBranchID, IsActive: true},
	}

	// Mark 09:00 as booked; 11:00 is absent → TherapistAvailable=true.
	bookingRepo := &stubBookingRepo{
		therapistBookedAtSlot: map[string]bool{
			chainKey(9, 0): true, // 09:00 booked
		},
	}
	availRepo := &stubAvailabilityRepo{
		coversSlot: func(_ string, _ int, _, _ string) bool { return true },
	}
	svc := newSlotsTestServiceWithTherapists(bookingRepo, availRepo, rooms, []*model.Therapist{therapistWithPrep(60)})

	tid := testTherapist
	slots, err := svc.ListAvailableSlots(context.Background(), AvailableSlotsInput{
		BranchID:    testBranchID,
		ServiceID:   testServiceID,
		Date:        testAvailDate,
		TherapistID: &tid,
	})

	require.NoError(t, err)
	require.NotEmpty(t, slots)

	slotMap := make(map[string]*Slot, len(slots))
	for i := range slots {
		slotMap[slots[i].Start] = &slots[i]
	}

	// slots[0] = 09:00, booked.
	s09_00 := slotMap[chainKey(9, 0)]
	require.NotNil(t, s09_00, "slot at 09:00 must exist")
	require.NotNil(t, s09_00.TherapistAvailable)
	assert.False(t, *s09_00.TherapistAvailable, "09:00 slot is booked → false")

	// slots[1] = 11:00 (step = 60 + 60 = 120 min), free.
	s11_00 := slotMap[chainKey(11, 0)]
	require.NotNil(t, s11_00, "slot at 11:00 must exist (step=120min from 09:00)")
	require.NotNil(t, s11_00.TherapistAvailable)
	assert.True(t, *s11_00.TherapistAvailable,
		"11:00 free — exactly at effective end of 60-min prep buffer")
}

// TestCreateConcierge_PrepBuffer_Conflict verifies that CreateConcierge returns
// ErrBookingTherapistConflict when the candidate slot falls within an existing
// booking's prep window (therapist booked 09:00–10:00 with prep=10, new booking
// requested at 10:00–11:00).
func TestCreateConcierge_PrepBuffer_Conflict(t *testing.T) {
	t.Parallel()

	// The candidate start is 10:00 UTC. isTherapistBusyWithPrep calls
	// IsTherapistBookedInSlots with the candidate window; the stub returns true
	// for that start key to simulate the prep-buffer conflict.
	candidateStart := "2026-04-26T10:00:00Z"
	bookingRepo := &stubBookingRepo{
		findByCodeErr: constants.ErrBookingNotFound,
		therapistBookedAtSlot: map[string]bool{
			candidateStart: true, // prep buffer in effect at 10:00
		},
	}
	svc := newTestService(
		bookingRepo,
		&stubBranchRepo{branch: testBranch()},
		&stubServiceRepo{svc: testService()},
		&stubAddonRepo{addons: map[string]*model.Addon{}},
		&stubRoomRepo{rooms: []*model.Room{testRoom()}},
		// Therapist has PrepMinutes=10; FindByID returns this row.
		&stubTherapistRepo{therapists: []*model.Therapist{therapistWithPrep(10)}},
		&stubTherapistSvcRepo{mappings: []*model.TherapistService{testTherapistMapping()}},
		&stubPaymentSvc{},
	)

	tid := testTherapist
	_, err := svc.CreateConcierge(context.Background(), ConciergeCreateBookingInput{
		CallerUserID:   "op-user",
		CallerTenantID: testTenantID,
		CallerBranches: []string{testBranchID},
		IsAdmin:        false,
		BranchID:       testBranchID,
		ServiceID:      testServiceID,
		TherapistID:    &tid,
		ScheduledStart: candidateStart,
		CustomerName:   "Budi",
		CustomerPhone:  "081234567890",
		CustomerEmail:  "budi@example.com",
	})

	assert.ErrorIs(t, err, constants.ErrBookingTherapistConflict,
		"CreateConcierge must return ErrBookingTherapistConflict when slot is inside prep buffer")
}

// ---------------------------------------------------------------------------
// Per-slot therapistAvailableCount — bulk availability + conflict logic (new)
// ---------------------------------------------------------------------------

// testAvailDate = "2026-04-25", Saturday → DOW = 6 (time.Saturday)
const testAvailDOW = 6 // time.Saturday

// makeAvailWindow constructs a TherapistAvailability row for unit tests.
// startTime and endTime must be "HH:MM:SS" strings.
func makeAvailWindow(therapistID, startTime, endTime string) *model.TherapistAvailability {
	return &model.TherapistAvailability{
		TherapistID: therapistID,
		DayOfWeek:   testAvailDOW,
		StartTime:   startTime,
		EndTime:     endTime,
	}
}

// makeInterval builds a TherapistBookingInterval with RFC3339 string inputs.
func makeInterval(therapistID, effectiveStart, effectiveEnd string) TherapistBookingInterval {
	start, _ := time.Parse(time.RFC3339, effectiveStart)
	end, _ := time.Parse(time.RFC3339, effectiveEnd)
	return TherapistBookingInterval{
		TherapistID:    therapistID,
		EffectiveStart: start,
		EffectiveEnd:   end,
	}
}

// TestListAvailableSlots_Bulk_OneTherapist_WorkingHoursRespected verifies that
// only slots within the therapist's working window (09:00–12:00) are generated,
// and count==1 for each. No slots are generated outside the window.
//
// Setup: 1 therapist mapped, working 09:00–12:00, 60-min service, prep=0 (step=60min).
// Chain: 09:00–10:00, 10:00–11:00, 11:00–12:00. (11:00+60=12:00 ≤ 12:00 ✓)
// No slot at 12:00 (12:00+60=13:00 > 12:00 ✗) or later.
func TestListAvailableSlots_Bulk_OneTherapist_WorkingHoursRespected(t *testing.T) {
	t.Parallel()

	rooms := []*model.Room{
		{ID: testRoomID, TenantID: testTenantID, BranchID: testBranchID, IsActive: true},
	}
	availRepo := &stubAvailabilityRepo{
		coversSlot: func(_ string, _ int, _, _ string) bool { return true },
		windows: []*model.TherapistAvailability{
			makeAvailWindow(testTherapist, "09:00:00", "12:00:00"),
		},
	}
	svc := newSlotsTestService(&stubBookingRepo{}, availRepo, rooms)

	slots, err := svc.ListAvailableSlots(context.Background(), AvailableSlotsInput{
		BranchID:  testBranchID,
		ServiceID: testServiceID,
		Date:      testAvailDate,
	})

	require.NoError(t, err)
	// Exactly 3 slots (09:00, 10:00, 11:00) for the 09:00–12:00 window with
	// 60-min service and 0 prep.
	assert.Len(t, slots, 3, "chained 60-min slots in 09:00–12:00 window → exactly 3 slots")

	slotMap := make(map[string]*Slot, len(slots))
	for i := range slots {
		slotMap[slots[i].Start] = &slots[i]
	}

	// 09:00–10:00 is within the window → count=1.
	s09_00 := slotMap[chainKey(9, 0)]
	require.NotNil(t, s09_00, "slot 09:00 must exist")
	assert.Equal(t, 1, s09_00.TherapistsAvailableCount,
		"09:00 slot is within 09:00–12:00 window → therapist available (count=1)")

	// 11:00 is the last valid slot (end=12:00 = window end).
	s11_00 := slotMap[chainKey(11, 0)]
	require.NotNil(t, s11_00, "slot 11:00 must exist (last in chain: end=12:00)")
	assert.Equal(t, 1, s11_00.TherapistsAvailableCount)

	// 12:00 must NOT be generated (12:00+60=13:00 > window end 12:00).
	_, has12_00 := slotMap[chainKey(12, 0)]
	assert.False(t, has12_00, "slot at 12:00 must NOT exist — it would end at 13:00 > window end")

	// 17:00 must NOT be generated (far outside the window).
	_, has17_00 := slotMap[chainKey(17, 0)]
	assert.False(t, has17_00, "slot at 17:00 must NOT exist — outside working window")
}

// TestListAvailableSlots_Bulk_TwoTherapists_DisjointWindows verifies that the
// Otomatis union produces slots from both therapist chains with no slots in the
// gap between their windows.
//
// Therapist A: 09:00–12:00, step=60min → 09:00, 10:00, 11:00 (3 slots).
// Therapist B: 13:00–17:00, step=60min → 13:00, 14:00, 15:00, 16:00 (4 slots).
// Union: 7 distinct slots. No slot exists in the 12:00–13:00 gap.
func TestListAvailableSlots_Bulk_TwoTherapists_DisjointWindows(t *testing.T) {
	t.Parallel()

	const therapistB = "ffffffff-0000-0000-0000-000000000009"

	rooms := []*model.Room{
		{ID: testRoomID, TenantID: testTenantID, BranchID: testBranchID, IsActive: true},
	}

	// Two therapists both mapped to the service (both have PrepMinutes=0).
	therapistBModel := &model.Therapist{
		ID: therapistB, TenantID: testTenantID, BranchID: testBranchID,
		FullName: "Dewi Lestari", IsActive: true,
	}
	mappingB := &model.TherapistService{TherapistID: therapistB, ServiceID: testServiceID, IsActive: true}

	availRepo := &stubAvailabilityRepo{
		coversSlot: func(_ string, _ int, _, _ string) bool { return true },
		windows: []*model.TherapistAvailability{
			makeAvailWindow(testTherapist, "09:00:00", "12:00:00"),
			makeAvailWindow(therapistB, "13:00:00", "17:00:00"),
		},
	}

	svc := newTestServiceWithAvailability(
		&stubBookingRepo{},
		&stubBranchRepo{branch: testBranch()},
		&stubServiceRepo{svc: testService()},
		&stubAddonRepo{addons: map[string]*model.Addon{}},
		&stubRoomRepo{rooms: rooms},
		&stubTherapistRepo{therapists: []*model.Therapist{testTherapistModel(), therapistBModel}},
		&stubTherapistSvcRepo{mappings: []*model.TherapistService{testTherapistMapping(), mappingB}},
		&stubPaymentSvc{},
		availRepo,
	)

	slots, err := svc.ListAvailableSlots(context.Background(), AvailableSlotsInput{
		BranchID:  testBranchID,
		ServiceID: testServiceID,
		Date:      testAvailDate,
	})

	require.NoError(t, err)
	// A: 3 slots, B: 4 slots, no overlap → exactly 7.
	assert.Len(t, slots, 7, "union of disjoint therapist chains must produce exactly 7 slots")

	slotMap := make(map[string]*Slot, len(slots))
	for i := range slots {
		slotMap[slots[i].Start] = &slots[i]
	}

	// 09:00 slot → only therapist A covers it → count=1.
	s09_00 := slotMap[chainKey(9, 0)]
	require.NotNil(t, s09_00, "slot 09:00 must exist (therapist A chain)")
	assert.Equal(t, 1, s09_00.TherapistsAvailableCount, "09:00 covered by therapist A only")

	// 14:00 slot → only therapist B covers it → count=1.
	s14_00 := slotMap[chainKey(14, 0)]
	require.NotNil(t, s14_00, "slot 14:00 must exist (therapist B chain)")
	assert.Equal(t, 1, s14_00.TherapistsAvailableCount, "14:00 covered by therapist B only")

	// 16:00 slot is the last in B's chain (16:00+60=17:00 ≤ 17:00 ✓).
	s16_00 := slotMap[chainKey(16, 0)]
	require.NotNil(t, s16_00, "slot 16:00 must exist (last in therapist B chain)")
	assert.Equal(t, 1, s16_00.TherapistsAvailableCount)

	// No slot must exist in the 12:00–13:00 gap.
	_, has12_00 := slotMap[chainKey(12, 0)]
	assert.False(t, has12_00, "no slot at 12:00 — gap between therapist windows")
	_, has12_30 := slotMap[chainKey(12, 30)]
	assert.False(t, has12_30, "no slot at 12:30 — gap between therapist windows")
}

// TestListAvailableSlots_Bulk_PrepMinutes_ConflictBlocksSlot verifies that the
// per-slot aggregate count respects prep_minutes via FindTherapistConflicts.
//
// 1 therapist, working 09:00–17:00, prep=0 (step=60min → slots at 09:00, 10:00, 11:00, …).
// Booking 09:00–10:00 with effective_end=10:10 (DB widens by 10-min prep).
// The 10:00–11:00 slot overlaps [09:00, 10:10) → count=0.
// The 11:00–12:00 slot starts at 11:00 which is after 10:10 → count=1.
func TestListAvailableSlots_Bulk_PrepMinutes_ConflictBlocksSlot(t *testing.T) {
	t.Parallel()

	rooms := []*model.Room{
		{ID: testRoomID, TenantID: testTenantID, BranchID: testBranchID, IsActive: true},
	}

	// Simulate booking 09:00–10:00 with prep=10 → effective end 10:10.
	// The chain (prep=0 on testTherapistModel) has step=60min: 09:00, 10:00, 11:00, …
	// 10:00 overlaps the conflict interval → count=0.
	// 11:00 does not overlap → count=1.
	bookingRepo := &stubBookingRepo{
		therapistConflicts: []TherapistBookingInterval{
			makeInterval(testTherapist,
				"2026-04-25T09:00:00+07:00", // effective_start
				"2026-04-25T10:10:00+07:00", // effective_end = booked_end + 10min prep
			),
		},
	}
	availRepo := &stubAvailabilityRepo{
		coversSlot: func(_ string, _ int, _, _ string) bool { return true },
		windows: []*model.TherapistAvailability{
			makeAvailWindow(testTherapist, "09:00:00", "17:00:00"),
		},
	}
	svc := newSlotsTestService(bookingRepo, availRepo, rooms)

	slots, err := svc.ListAvailableSlots(context.Background(), AvailableSlotsInput{
		BranchID:  testBranchID,
		ServiceID: testServiceID,
		Date:      testAvailDate,
	})

	require.NoError(t, err)
	require.NotEmpty(t, slots)

	slotMap := make(map[string]*Slot, len(slots))
	for i := range slots {
		slotMap[slots[i].Start] = &slots[i]
	}

	// 10:00 slot overlaps the conflict [09:00, 10:10) → count=0.
	s10_00 := slotMap[chainKey(10, 0)]
	require.NotNil(t, s10_00, "slot at 10:00 must exist (in 60-min chain)")
	assert.Equal(t, 0, s10_00.TherapistsAvailableCount,
		"10:00–11:00 overlaps conflict [09:00, 10:10) → count=0")

	// 11:00 slot does not overlap (11:00 >= 10:10) → count=1.
	s11_00 := slotMap[chainKey(11, 0)]
	require.NotNil(t, s11_00, "slot at 11:00 must exist")
	assert.Equal(t, 1, s11_00.TherapistsAvailableCount,
		"11:00 slot starts after effective end 10:10 → count=1")

	// Verify 10:30 does NOT exist (no 30-min grid — chain step is 60min).
	_, has10_30 := slotMap[chainKey(10, 30)]
	assert.False(t, has10_30, "10:30 must NOT be a chain slot (step=60min)")
}

// TestListAvailableSlots_Bulk_PrepMinutes0_NoExtraBlock verifies that
// prep_minutes=0 does not block the slot immediately after the booking ends.
//
// 1 therapist, working 09:00–17:00, booking 09:00–10:00 with effective_end=10:00
// (prep=0). The 10:00–11:00 slot starts at 10:00 = effective_end → NOT blocked
// (half-open interval: slotStart < effective_end is false) → count=1.
func TestListAvailableSlots_Bulk_PrepMinutes0_NoExtraBlock(t *testing.T) {
	t.Parallel()

	rooms := []*model.Room{
		{ID: testRoomID, TenantID: testTenantID, BranchID: testBranchID, IsActive: true},
	}

	bookingRepo := &stubBookingRepo{
		therapistConflicts: []TherapistBookingInterval{
			makeInterval(testTherapist,
				"2026-04-25T09:00:00+07:00",
				"2026-04-25T10:00:00+07:00", // effective_end = booked_end (prep=0)
			),
		},
	}
	availRepo := &stubAvailabilityRepo{
		coversSlot: func(_ string, _ int, _, _ string) bool { return true },
		windows: []*model.TherapistAvailability{
			makeAvailWindow(testTherapist, "09:00:00", "17:00:00"),
		},
	}
	svc := newSlotsTestService(bookingRepo, availRepo, rooms)

	slots, err := svc.ListAvailableSlots(context.Background(), AvailableSlotsInput{
		BranchID:  testBranchID,
		ServiceID: testServiceID,
		Date:      testAvailDate,
	})

	require.NoError(t, err)
	require.NotEmpty(t, slots)

	slotMap := make(map[string]*Slot, len(slots))
	for i := range slots {
		slotMap[slots[i].Start] = &slots[i]
	}

	// 10:00 is in the 60-min chain and does NOT overlap [09:00, 10:00) (half-open).
	s10_00 := slotMap[chainKey(10, 0)]
	require.NotNil(t, s10_00, "slot at 10:00 must exist (step=60min chain)")
	assert.Equal(t, 1, s10_00.TherapistsAvailableCount,
		"prep=0: slot at exact booked_end is NOT blocked (half-open interval) → count=1")
}

// ---------------------------------------------------------------------------
// Chained slot grid — required tests (ADR slot-grid fix)
// ---------------------------------------------------------------------------

// TestListAvailableSlots_ChainedSlots_60min_15prep verifies the canonical chain:
// therapist 09:00–17:00, 60-min service, prep=15 → step=75min.
// Expected starts: 09:00, 10:15, 11:30, 12:45, 14:00, 15:15.
// Next would be 16:30 + 60 = 17:30 > 17:00 → dropped.
func TestListAvailableSlots_ChainedSlots_60min_15prep(t *testing.T) {
	t.Parallel()

	rooms := []*model.Room{
		{ID: testRoomID, TenantID: testTenantID, BranchID: testBranchID, IsActive: true},
	}
	availRepo := &stubAvailabilityRepo{
		coversSlot: func(_ string, _ int, _, _ string) bool { return true },
		windows: []*model.TherapistAvailability{
			makeAvailWindow(testTherapist, "09:00:00", "17:00:00"),
		},
	}
	svc := newSlotsTestServiceWithTherapists(
		&stubBookingRepo{},
		availRepo,
		rooms,
		[]*model.Therapist{therapistWithPrep(15)},
	)

	tid := testTherapist
	slots, err := svc.ListAvailableSlots(context.Background(), AvailableSlotsInput{
		BranchID:    testBranchID,
		ServiceID:   testServiceID,
		Date:        testAvailDate,
		TherapistID: &tid,
	})

	require.NoError(t, err)
	wantStarts := []string{
		chainKey(9, 0),   // 09:00
		chainKey(10, 15), // 10:15
		chainKey(11, 30), // 11:30
		chainKey(12, 45), // 12:45
		chainKey(14, 0),  // 14:00
		chainKey(15, 15), // 15:15
		// 16:30 + 60 = 17:30 > 17:00 → dropped
	}
	require.Len(t, slots, len(wantStarts),
		"expected exactly 6 chain slots for 09:00–17:00, 60min service, 15min prep")
	for i, s := range slots {
		assert.Equal(t, wantStarts[i], s.Start,
			"slot %d: expected start %s, got %s", i, wantStarts[i], s.Start)
	}
}

// TestListAvailableSlots_TwoWindows_LunchGap verifies that a therapist with two
// availability windows (09:00–12:00 and 13:00–17:00, i.e. a lunch break) produces
// independent chains from each window.
//
// 60-min service, prep=15 (step=75min):
//   Window 09:00–12:00: 09:00, 10:15. (10:15+60=11:15 ≤ 12:00 ✓; 11:30+60=12:30 > 12:00 ✗)
//   Window 13:00–17:00: 13:00, 14:15, 15:30. (15:30+60=16:30 ≤ 17:00 ✓; 16:45+60=17:45 > 17:00 ✗)
func TestListAvailableSlots_TwoWindows_LunchGap(t *testing.T) {
	t.Parallel()

	rooms := []*model.Room{
		{ID: testRoomID, TenantID: testTenantID, BranchID: testBranchID, IsActive: true},
	}
	availRepo := &stubAvailabilityRepo{
		coversSlot: func(_ string, _ int, _, _ string) bool { return true },
		windows: []*model.TherapistAvailability{
			makeAvailWindow(testTherapist, "09:00:00", "12:00:00"),
			makeAvailWindow(testTherapist, "13:00:00", "17:00:00"),
		},
	}
	svc := newSlotsTestServiceWithTherapists(
		&stubBookingRepo{},
		availRepo,
		rooms,
		[]*model.Therapist{therapistWithPrep(15)},
	)

	tid := testTherapist
	slots, err := svc.ListAvailableSlots(context.Background(), AvailableSlotsInput{
		BranchID:    testBranchID,
		ServiceID:   testServiceID,
		Date:        testAvailDate,
		TherapistID: &tid,
	})

	require.NoError(t, err)
	wantStarts := []string{
		chainKey(9, 0),   // window 1: 09:00
		chainKey(10, 15), // window 1: 10:15
		chainKey(13, 0),  // window 2: 13:00
		chainKey(14, 15), // window 2: 14:15
		chainKey(15, 30), // window 2: 15:30
	}
	require.Len(t, slots, len(wantStarts),
		"expected 5 slots across two windows with lunch gap")
	for i, s := range slots {
		assert.Equal(t, wantStarts[i], s.Start,
			"slot %d: expected start %s", i, wantStarts[i])
	}
}

// TestListAvailableSlots_PrepZero_BackToBack verifies that prep=0 produces
// back-to-back slots (step = service duration).
//
// 60-min service, prep=0, working 09:00–13:00 → slots: 09:00, 10:00, 11:00, 12:00.
// (12:00+60=13:00 ≤ 13:00 ✓; 13:00+60=14:00 > 13:00 ✗)
func TestListAvailableSlots_PrepZero_BackToBack(t *testing.T) {
	t.Parallel()

	rooms := []*model.Room{
		{ID: testRoomID, TenantID: testTenantID, BranchID: testBranchID, IsActive: true},
	}
	availRepo := &stubAvailabilityRepo{
		coversSlot: func(_ string, _ int, _, _ string) bool { return true },
		windows: []*model.TherapistAvailability{
			makeAvailWindow(testTherapist, "09:00:00", "13:00:00"),
		},
	}
	svc := newSlotsTestServiceWithTherapists(
		&stubBookingRepo{},
		availRepo,
		rooms,
		[]*model.Therapist{therapistWithPrep(0)},
	)

	tid := testTherapist
	slots, err := svc.ListAvailableSlots(context.Background(), AvailableSlotsInput{
		BranchID:    testBranchID,
		ServiceID:   testServiceID,
		Date:        testAvailDate,
		TherapistID: &tid,
	})

	require.NoError(t, err)
	wantStarts := []string{
		chainKey(9, 0),
		chainKey(10, 0),
		chainKey(11, 0),
		chainKey(12, 0),
	}
	require.Len(t, slots, 4,
		"prep=0, 09:00–13:00, 60-min service → exactly 4 back-to-back slots")
	for i, s := range slots {
		assert.Equal(t, wantStarts[i], s.Start,
			"slot %d start mismatch", i)
	}
}

// TestListAvailableSlots_Otomatis_TwoTherapists verifies that in Otomatis mode the
// union of two therapists' chained grids is returned, deduplicated and sorted.
//
// Therapist A: window 09:00–17:00, prep=15 → step=75min → 09:00, 10:15, 11:30, 12:45, 14:00, 15:15.
// Therapist B: window 09:30–17:30, prep=10 → step=70min → 09:30, 10:40, 11:50, 13:00, 14:10, 15:20, 16:30.
// Branch ops: 09:00–18:00 (set via dayStart/dayEnd defaults; B's 17:30 is within 18:00).
// B's last slot: 16:30+60=17:30 ≤ 17:30 ✓.
// Union of A(6) + B(7) = 13 distinct slots (no overlapping starts in this scenario).
func TestListAvailableSlots_Otomatis_TwoTherapists(t *testing.T) {
	t.Parallel()

	const therapistB = "ffffffff-0000-0000-0000-00000000000b"

	therapistBModel := &model.Therapist{
		ID:          therapistB,
		TenantID:    testTenantID,
		BranchID:    testBranchID,
		FullName:    "Budi Santoso",
		IsActive:    true,
		PrepMinutes: 10,
	}
	mappingB := &model.TherapistService{
		TherapistID: therapistB,
		ServiceID:   testServiceID,
		IsActive:    true,
	}

	rooms := []*model.Room{
		{ID: testRoomID, TenantID: testTenantID, BranchID: testBranchID, IsActive: true},
	}
	availRepo := &stubAvailabilityRepo{
		coversSlot: func(_ string, _ int, _, _ string) bool { return true },
		windows: []*model.TherapistAvailability{
			makeAvailWindow(testTherapist, "09:00:00", "17:00:00"),
			makeAvailWindow(therapistB, "09:30:00", "17:30:00"),
		},
	}

	// Branch defaults to 09:00–21:00; both therapist windows are within that range.
	svc := newTestServiceWithAvailability(
		&stubBookingRepo{},
		&stubBranchRepo{branch: testBranch()},
		&stubServiceRepo{svc: testService()},
		&stubAddonRepo{addons: map[string]*model.Addon{}},
		&stubRoomRepo{rooms: rooms},
		&stubTherapistRepo{therapists: []*model.Therapist{therapistWithPrep(15), therapistBModel}},
		&stubTherapistSvcRepo{mappings: []*model.TherapistService{testTherapistMapping(), mappingB}},
		&stubPaymentSvc{},
		availRepo,
	)

	slots, err := svc.ListAvailableSlots(context.Background(), AvailableSlotsInput{
		BranchID:  testBranchID,
		ServiceID: testServiceID,
		Date:      testAvailDate,
		// No TherapistID → Otomatis mode.
	})

	require.NoError(t, err)

	// Build expected union: A's 6 starts + B's 7 starts = 13 distinct starts.
	aStarts := []string{
		chainKey(9, 0), chainKey(10, 15), chainKey(11, 30),
		chainKey(12, 45), chainKey(14, 0), chainKey(15, 15),
	}
	bStarts := []string{
		chainKey(9, 30), chainKey(10, 40), chainKey(11, 50),
		chainKey(13, 0), chainKey(14, 10), chainKey(15, 20), chainKey(16, 30),
	}

	assert.Len(t, slots, len(aStarts)+len(bStarts),
		"union of A(6) + B(7) must produce 13 distinct slots")

	slotMap := make(map[string]bool, len(slots))
	for _, s := range slots {
		slotMap[s.Start] = true
	}
	for _, k := range aStarts {
		assert.True(t, slotMap[k], "therapist A slot %s must be in union", k)
	}
	for _, k := range bStarts {
		assert.True(t, slotMap[k], "therapist B slot %s must be in union", k)
	}

	// Verify slots are sorted by start time.
	for i := 1; i < len(slots); i++ {
		assert.True(t, slots[i].Start >= slots[i-1].Start,
			"slots must be in ascending order: %s >= %s", slots[i].Start, slots[i-1].Start)
	}
}

// TestListAvailableSlots_BranchOpsClipsTherapist verifies that the branch
// operational hours bound clips a therapist window that extends beyond them.
//
// The default dayStart=09:00, dayEnd=21:00. We simulate a narrower branch window
// by setting a therapist with availability 08:00–18:00 and expecting slots only
// from 09:00 (clipped lower bound) to a last slot ending ≤ 21:00.
// For a stricter clip test we use a window 07:00–23:00 — after clipping to
// [09:00, 21:00], 60-min/15-prep (step=75min) chain: 09:00, 10:15, 11:30, 12:45,
// 14:00, 15:15, 16:30, 17:45, 19:00, 20:15. (20:15+60=21:15 > 21:00 → stop).
//
// The test also asserts that no slot starts before 09:00 or ends after 21:00.
func TestListAvailableSlots_BranchOpsClipsTherapist(t *testing.T) {
	t.Parallel()

	rooms := []*model.Room{
		{ID: testRoomID, TenantID: testTenantID, BranchID: testBranchID, IsActive: true},
	}
	availRepo := &stubAvailabilityRepo{
		coversSlot: func(_ string, _ int, _, _ string) bool { return true },
		// Therapist claims 07:00–23:00 — branch ops (09:00–21:00) should clip both ends.
		windows: []*model.TherapistAvailability{
			makeAvailWindow(testTherapist, "07:00:00", "23:00:00"),
		},
	}
	svc := newSlotsTestServiceWithTherapists(
		&stubBookingRepo{},
		availRepo,
		rooms,
		[]*model.Therapist{therapistWithPrep(15)},
	)

	tid := testTherapist
	slots, err := svc.ListAvailableSlots(context.Background(), AvailableSlotsInput{
		BranchID:    testBranchID,
		ServiceID:   testServiceID,
		Date:        testAvailDate,
		TherapistID: &tid,
	})

	require.NoError(t, err)
	require.NotEmpty(t, slots)

	dayStartRFC := chainKey(9, 0)
	dayEnd := time.Date(2026, 4, 25, 21, 0, 0, 0, time.UTC)

	for _, s := range slots {
		assert.True(t, s.Start >= dayStartRFC,
			"slot %s must not start before branch dayStart 09:00", s.Start)
		end, _ := time.Parse(time.RFC3339, s.End)
		assert.False(t, end.After(dayEnd),
			"slot ending at %s must not exceed branch dayEnd 21:00", s.End)
	}

	// First slot must start exactly at dayStart after clip.
	assert.Equal(t, chainKey(9, 0), slots[0].Start,
		"first slot must start at branch dayStart 09:00 after clipping")
}

// TestListAvailableSlots_BookingConflictDropsSlot verifies that a booking conflict
// covering a chain slot drops the TherapistsAvailableCount to 0 for that slot
// while leaving adjacent slots unaffected.
//
// Single-therapist mode: TherapistAvailable=false for the conflicted slot.
// Otomatis mode: count drops from 1 to 0 for that slot.
func TestListAvailableSlots_BookingConflictDropsSlot(t *testing.T) {
	t.Parallel()

	rooms := []*model.Room{
		{ID: testRoomID, TenantID: testTenantID, BranchID: testBranchID, IsActive: true},
	}

	// Conflict covers 08:30–09:30 — overlaps the 09:00–10:00 chain slot.
	// Chain: 09:00–17:00, prep=0, step=60min.
	bookingRepo := &stubBookingRepo{
		therapistConflicts: []TherapistBookingInterval{
			makeInterval(testTherapist,
				"2026-04-25T08:30:00+07:00",
				"2026-04-25T09:30:00+07:00",
			),
		},
		therapistBookedAtSlot: map[string]bool{
			chainKey(9, 0): true, // same conflict expressed for the single-therapist path
		},
	}
	availRepo := &stubAvailabilityRepo{
		coversSlot: func(_ string, _ int, _, _ string) bool { return true },
		windows: []*model.TherapistAvailability{
			makeAvailWindow(testTherapist, "09:00:00", "17:00:00"),
		},
	}

	// --- Otomatis mode (no TherapistID): count drops for the conflicted slot ---
	svc := newSlotsTestService(bookingRepo, availRepo, rooms)

	otomatisSlots, err := svc.ListAvailableSlots(context.Background(), AvailableSlotsInput{
		BranchID:  testBranchID,
		ServiceID: testServiceID,
		Date:      testAvailDate,
	})
	require.NoError(t, err)

	otomatisMap := make(map[string]*Slot, len(otomatisSlots))
	for i := range otomatisSlots {
		otomatisMap[otomatisSlots[i].Start] = &otomatisSlots[i]
	}

	s09_00 := otomatisMap[chainKey(9, 0)]
	require.NotNil(t, s09_00, "slot 09:00 must exist in Otomatis result")
	assert.Equal(t, 0, s09_00.TherapistsAvailableCount,
		"Otomatis: 09:00 conflict → count must be 0")

	s10_00 := otomatisMap[chainKey(10, 0)]
	require.NotNil(t, s10_00, "slot 10:00 must exist in Otomatis result")
	assert.Equal(t, 1, s10_00.TherapistsAvailableCount,
		"Otomatis: 10:00 is unaffected → count must be 1")

	// --- Single-therapist mode: TherapistAvailable=false for the conflicted slot ---
	tid := testTherapist
	singleSlots, err := svc.ListAvailableSlots(context.Background(), AvailableSlotsInput{
		BranchID:    testBranchID,
		ServiceID:   testServiceID,
		Date:        testAvailDate,
		TherapistID: &tid,
	})
	require.NoError(t, err)

	singleMap := make(map[string]*Slot, len(singleSlots))
	for i := range singleSlots {
		singleMap[singleSlots[i].Start] = &singleSlots[i]
	}

	s09_single := singleMap[chainKey(9, 0)]
	require.NotNil(t, s09_single, "slot 09:00 must exist in single-therapist result")
	require.NotNil(t, s09_single.TherapistAvailable)
	assert.False(t, *s09_single.TherapistAvailable,
		"single-therapist: 09:00 booked → TherapistAvailable=false")

	s10_single := singleMap[chainKey(10, 0)]
	require.NotNil(t, s10_single, "slot 10:00 must exist in single-therapist result")
	require.NotNil(t, s10_single.TherapistAvailable)
	assert.True(t, *s10_single.TherapistAvailable,
		"single-therapist: 10:00 unaffected → TherapistAvailable=true")
}
