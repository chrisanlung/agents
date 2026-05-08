package service

import (
	"context"
	"crypto/rand"
	"encoding/base32"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/chrisanlung/lustia-auth/internal/constants"
	"github.com/chrisanlung/lustia-auth/internal/model"
	"github.com/google/uuid"
)

// math function aliases to keep haversine self-contained.
var (
	mathSin  = math.Sin
	mathCos  = math.Cos
	mathSqrt = math.Sqrt
	mathAsin = math.Asin
)

// jakartaLocation is shared with settlement_service.go (declared there in
// init()). Therapist availability windows are stored as TIME (clock-on-the-
// wall) without a timezone, but operators set them in local Jakarta time.
// The slot generator must parse "HH:MM:SS" + the requested date in Jakarta
// local so the resulting time.Time has the correct +07:00 offset; otherwise
// customers in Jakarta see slots shifted by 7 hours (09:00 stored → 16:00
// perceived).

// crockfordAlphabet is the Crockford Base32 character set minus 0/O/1/I/L for
// legibility. Uses A-Z and 2-7. ADR 0014 §3.4.
// encoding/base32.StdEncoding uses A-Z and 2-7 which happens to match
// Crockford's recommendation for the same reason.
var crockfordEncoding = base32.StdEncoding.WithPadding(base32.NoPadding)

// bookingCodeMaxRetries is the maximum number of attempts to generate a
// unique booking code before giving up.
const bookingCodeMaxRetries = 5

// BookingService handles the booking lifecycle per ADR 0014.
type BookingService struct {
	bookings     BookingRepository
	branches     BranchRepository
	services     ServiceCatalogRepository
	addons       AddonRepository
	rooms        RoomRepository
	therapists   TherapistRepository
	therapistSvc TherapistServiceRepository
	availability TherapistAvailabilityRepository
	// payment is the Phase 6 PaymentProvider (ADR 0015). Replaces Phase 5
	// MidtransClient. BookingService only calls InitiateForBooking — the
	// webhook path is now handled by PaymentService.
	paymentSvc PaymentServiceIface
	audit       AuditRepository
	email       EmailSender
	clock       Clock
	tx          TxManager
	// storage digunakan untuk menghasilkan signed URL foto terapis/ruangan
	// pada endpoint publik (ADR 0011).
	storage Storage
	// tenants digunakan untuk mengambil nama tenant pada GetPublicBranchDetail.
	tenants TenantRepository
}

// NewBookingService constructs a BookingService.
func NewBookingService(
	bookings BookingRepository,
	branches BranchRepository,
	services ServiceCatalogRepository,
	addons AddonRepository,
	rooms RoomRepository,
	therapists TherapistRepository,
	therapistSvc TherapistServiceRepository,
	availability TherapistAvailabilityRepository,
	paymentSvc PaymentServiceIface,
	audit AuditRepository,
	email EmailSender,
	clock Clock,
	tx TxManager,
	storage Storage,
	tenants TenantRepository,
) *BookingService {
	return &BookingService{
		bookings:     bookings,
		branches:     branches,
		services:     services,
		addons:       addons,
		rooms:        rooms,
		therapists:   therapists,
		therapistSvc: therapistSvc,
		availability: availability,
		paymentSvc:   paymentSvc,
		audit:        audit,
		email:        email,
		clock:        clock,
		tx:           tx,
		storage:      storage,
		tenants:      tenants,
	}
}

// ---------------------------------------------------------------------------
// Public customer flow
// ---------------------------------------------------------------------------

// CreatePublic handles the guest booking creation endpoint.
//
// Security constraints enforced here:
//   - C-1: total price is NEVER taken from the request; computed server-side.
//   - H-1: all FK IDs are validated against the resolved tenant_id.
//   - H-2: rate limiting is applied at the middleware layer (route registration).
func (s *BookingService) CreatePublic(ctx context.Context, in PublicCreateBookingInput) (CreateBookingOutput, error) {
	// Pivot RLS to __public__ for the initial branch + service + addon
	// lookups. Without this, the default __platform__ tenant context blocks
	// every SELECT against tenant-scoped tables. After we resolve the branch
	// and know the tenant, we switch to that tenant's context for the INSERT.
	if err := s.tx.SetTenantContext(ctx, constants.PublicTenantSentinel, ""); err != nil {
		return CreateBookingOutput{}, fmt.Errorf("set public tenant context: %w", err)
	}

	// Lazy expiry sweep before attempting INSERT — prevents stale pending_payment
	// rows from blocking the exclusion constraint check (M-3, SECURITY.md).
	if _, err := s.bookings.SweepExpired(ctx); err != nil {
		slog.WarnContext(ctx, "expiry sweep error (non-fatal)", "error", err)
	}

	// --- H-1: Resolve tenant from branch ---
	branch, err := s.branches.FindByID(ctx, in.BranchID)
	if err != nil {
		return CreateBookingOutput{}, constants.ErrBranchNotFound
	}
	if branch.DeletedAt != nil || branch.Status != model.BranchStatusActive {
		return CreateBookingOutput{}, constants.ErrBranchNotFound
	}
	tenantID := branch.TenantID

	// Switch RLS context to the resolved tenant so the INSERT is permitted.
	if err := s.tx.SetTenantContext(ctx, tenantID, ""); err != nil {
		return CreateBookingOutput{}, fmt.Errorf("set tenant context: %w", err)
	}

	// --- H-1: Validate service belongs to this tenant ---
	svc, err := s.services.FindByID(ctx, in.ServiceID)
	if err != nil || svc.TenantID != tenantID || !svc.IsActive {
		return CreateBookingOutput{}, constants.ErrServiceNotFound
	}

	// --- H-1: Validate addons belong to this tenant ---
	// C-1: collect addon prices from DB, NOT from the request.
	addonRows := make([]*model.Addon, 0, len(in.AddonIDs))
	for _, addonID := range in.AddonIDs {
		a, err := s.addons.FindByID(ctx, addonID)
		if err != nil || a.TenantID != tenantID || !a.IsActive {
			return CreateBookingOutput{}, constants.ErrAddonNotFound
		}
		addonRows = append(addonRows, a)
	}

	// --- Parse scheduled_start ---
	start, err := time.Parse(time.RFC3339, in.ScheduledStart)
	if err != nil {
		return CreateBookingOutput{}, fmt.Errorf("%w: scheduled_start must be RFC3339", constants.ErrInvalidInput)
	}
	end := start.Add(time.Duration(svc.DurationMinutes) * time.Minute)

	// --- H-1: Validate optional room belongs to this branch ---
	if in.RoomID != nil {
		rm, err := s.rooms.FindByID(ctx, *in.RoomID)
		if err != nil || rm.TenantID != tenantID || rm.BranchID != branch.ID || !rm.IsActive {
			return CreateBookingOutput{}, constants.ErrRoomNotFound
		}
	}

	// --- H-1: Validate optional therapist belongs to this branch and can perform service ---
	if in.TherapistID != nil {
		th, err := s.therapists.FindByID(ctx, *in.TherapistID)
		if err != nil || th.TenantID != tenantID || th.BranchID != branch.ID || !th.IsActive {
			return CreateBookingOutput{}, constants.ErrTherapistNotFound
		}
		// Verify therapist is mapped to this service.
		if ok, err := s.therapistCanPerformService(ctx, *in.TherapistID, in.ServiceID); err != nil || !ok {
			return CreateBookingOutput{}, constants.ErrTherapistNotForService
		}
	}

	// Auto-assign therapist if not specified.
	therapistID := in.TherapistID
	if therapistID == nil {
		id, err := s.autoAssignTherapist(ctx, tenantID, branch.ID, in.ServiceID, start, end)
		if err != nil {
			return CreateBookingOutput{}, err
		}
		therapistID = &id
	}

	// Pre-flight prep-buffer conflict check (migration 000035).
	// The GiST exclusion constraint enforces [scheduled_start, scheduled_end)
	// overlap at DB level; this check additionally enforces the therapist's
	// prep window so a booking starting inside prep time is rejected early with
	// a clear error rather than a silent DB constraint bypass.
	if therapistID != nil {
		busy, err := s.isTherapistBusyWithPrep(ctx, *therapistID, start, end)
		if err != nil {
			return CreateBookingOutput{}, fmt.Errorf("pre-flight conflict check: %w", err)
		}
		if busy {
			return CreateBookingOutput{}, constants.ErrBookingTherapistConflict
		}
	}

	// Auto-assign room if not specified.
	roomID := in.RoomID
	if roomID == nil {
		id, err := s.autoAssignRoom(ctx, tenantID, branch.ID, start, end)
		if err != nil {
			return CreateBookingOutput{}, err
		}
		if id != "" {
			roomID = &id
		}
	}

	// C-1: Compute total price server-side from DB-fetched values only.
	total := svc.PriceIDR
	for _, a := range addonRows {
		total += a.PriceIDR
	}

	// Generate unique booking code with retry on collision.
	code, err := s.generateUniqueCode(ctx)
	if err != nil {
		return CreateBookingOutput{}, err
	}

	now := s.clock.Now()
	paymentMethod := model.BookingPaymentMidtrans
	b := &model.Booking{
		ID:             uuid.New().String(),
		TenantID:       tenantID,
		BranchID:       branch.ID,
		ServiceID:      svc.ID,
		RoomID:         roomID,
		TherapistID:    therapistID,
		CustomerName:   in.CustomerName,
		CustomerPhone:  in.CustomerPhone,
		CustomerEmail:  in.CustomerEmail,
		Code:           code,
		ScheduledStart: start,
		ScheduledEnd:   end,
		TotalPriceIDR:  total,
		PaymentMethod:  &paymentMethod,
		Status:         model.BookingStatusPendingPayment,
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	if err := s.bookings.Save(ctx, b); err != nil {
		return CreateBookingOutput{}, fmt.Errorf("save booking: %w", err)
	}

	// Save addon snapshots (price_idr from DB — C-1).
	bookingAddons := make([]*model.BookingAddon, len(addonRows))
	for i, a := range addonRows {
		bookingAddons[i] = &model.BookingAddon{
			BookingID: b.ID,
			AddonID:   a.ID,
			PriceIDR:  a.PriceIDR,
		}
	}
	if err := s.bookings.SaveAddons(ctx, bookingAddons); err != nil {
		return CreateBookingOutput{}, fmt.Errorf("save booking addons: %w", err)
	}

	// Phase 6 (ADR 0015 §2.8): Initiate QRIS payment transaction.
	// PaymentService creates the payment_transaction row + calls provider.CreateQR.
	// BookingService stays HTTP-agnostic — no provider details leak here.
	payOut, err := s.paymentSvc.InitiateForBooking(
		ctx,
		b.ID,
		tenantID,
		total,
		code, // orderID = booking.code
		in.CustomerName,
		in.CustomerEmail,
		in.CustomerPhone,
		fmt.Sprintf("Booking %s di %s", svc.Name, branch.Name),
	)
	if err != nil {
		// Non-fatal: booking is created; QR generation failed.
		// Customer can retry via RetryQR endpoint. Log + continue.
		slog.WarnContext(ctx, "payment initiate failed (non-fatal; booking still created)",
			"booking_id", b.ID, "error", err)
	}

	// Send confirmation email (non-fatal on error).
	go s.sendConfirmationEmail(context.Background(), b, svc.Name, branch.Name)

	_ = s.audit.Append(ctx, AuditEntry{
		TenantID:     &tenantID,
		Action:       "booking.created_public",
		ResourceType: "booking",
		ResourceID:   b.ID,
		Meta:         map[string]interface{}{"code": b.Code, "status": b.Status},
	})

	detail, err := s.buildBookingDetail(ctx, b)
	if err != nil {
		return CreateBookingOutput{}, err
	}

	out := CreateBookingOutput{BookingDetail: detail}
	if payOut.QRString != "" {
		out.QRString = payOut.QRString
		out.QRImageURL = payOut.QRImageURL
		out.QRExpiresAt = payOut.QRExpiresAt.Format(time.RFC3339)
		out.PaymentReference = payOut.ProviderReference
	}
	return out, nil
}

// GetPublicByCode returns a masked booking view for the public code-lookup endpoint.
//
// H-7 (SECURITY.md): This is the only public read path. It calls
// FindByCodePublic which enforces WHERE code = $1 — no unbounded public query.
func (s *BookingService) GetPublicByCode(ctx context.Context, code string) (PublicBookingView, error) {
	if code == "" {
		return PublicBookingView{}, constants.ErrBookingCodeInvalid
	}

	// H-7: SetTenantContext to __public__ so the booking_public_select RLS policy fires.
	if err := s.tx.SetTenantContext(ctx, constants.PublicTenantSentinel, ""); err != nil {
		return PublicBookingView{}, fmt.Errorf("set public tenant context: %w", err)
	}

	// H-7: FindByCodePublic enforces WHERE code = $1 at the repository level.
	b, err := s.bookings.FindByCodePublic(ctx, code)
	if err != nil {
		return PublicBookingView{}, constants.ErrBookingNotFound
	}

	addons, _ := s.bookings.FindAddonsByBooking(ctx, b.ID)

	// Restore RLS to tenant context for subsequent reads.
	if err := s.tx.SetTenantContext(ctx, b.TenantID, ""); err != nil {
		return PublicBookingView{}, fmt.Errorf("restore tenant context: %w", err)
	}

	svc, _ := s.services.FindByID(ctx, b.ServiceID)
	svcName := ""
	if svc != nil {
		svcName = svc.Name
	}

	branch, _ := s.branches.FindByID(ctx, b.BranchID)
	branchName := ""
	if branch != nil {
		branchName = branch.Name
	}

	addonSnapshots := buildAddonSnapshots(addons, nil)

	return PublicBookingView{
		Code:           b.Code,
		BranchName:     branchName,
		ServiceName:    svcName,
		ScheduledStart: b.ScheduledStart.Format(time.RFC3339),
		ScheduledEnd:   b.ScheduledEnd.Format(time.RFC3339),
		Status:         b.Status,
		TotalPriceIDR:  b.TotalPriceIDR,
		CustomerName:   b.CustomerName,
		CustomerPhone:  maskPhone(b.CustomerPhone),   // M-2: partial masking
		CustomerEmail:  maskEmail(b.CustomerEmail),   // M-2: partial masking
		Addons:         addonSnapshots,
	}, nil
}

// ---------------------------------------------------------------------------
// Operator (concierge + management) flow
// ---------------------------------------------------------------------------

// CreateConcierge handles the ops-staff booking creation endpoint.
// Payment is flagged paid_at_venue. No Midtrans transaction is created.
//
// H-6 (SECURITY.md): validates caller's branch scope and all FK resources.
func (s *BookingService) CreateConcierge(ctx context.Context, in ConciergeCreateBookingInput) (CreateBookingOutput, error) {
	// Lazy sweep.
	if _, err := s.bookings.SweepExpired(ctx); err != nil {
		slog.WarnContext(ctx, "expiry sweep error (non-fatal)", "error", err)
	}

	// H-6: Validate caller's branch scope.
	if !in.IsAdmin && !containsBranch(in.CallerBranches, in.BranchID) {
		return CreateBookingOutput{}, constants.ErrCrossBranchForbidden
	}

	branch, err := s.branches.FindByID(ctx, in.BranchID)
	if err != nil {
		return CreateBookingOutput{}, constants.ErrBranchNotFound
	}
	if branch.TenantID != in.CallerTenantID || branch.DeletedAt != nil {
		return CreateBookingOutput{}, constants.ErrBranchNotFound
	}

	// H-6: Service must belong to caller's tenant.
	svc, err := s.services.FindByID(ctx, in.ServiceID)
	if err != nil || svc.TenantID != in.CallerTenantID || !svc.IsActive {
		return CreateBookingOutput{}, constants.ErrServiceNotFound
	}

	// H-6: Addons must belong to caller's tenant.
	addonRows := make([]*model.Addon, 0, len(in.AddonIDs))
	for _, addonID := range in.AddonIDs {
		a, err := s.addons.FindByID(ctx, addonID)
		if err != nil || a.TenantID != in.CallerTenantID || !a.IsActive {
			return CreateBookingOutput{}, constants.ErrAddonNotFound
		}
		addonRows = append(addonRows, a)
	}

	start, err := time.Parse(time.RFC3339, in.ScheduledStart)
	if err != nil {
		return CreateBookingOutput{}, fmt.Errorf("%w: scheduled_start must be RFC3339", constants.ErrInvalidInput)
	}
	end := start.Add(time.Duration(svc.DurationMinutes) * time.Minute)

	// H-6: Room must belong to the specific branch.
	if in.RoomID != nil {
		rm, err := s.rooms.FindByID(ctx, *in.RoomID)
		if err != nil || rm.TenantID != in.CallerTenantID || rm.BranchID != branch.ID || !rm.IsActive {
			return CreateBookingOutput{}, constants.ErrRoomNotFound
		}
	}

	// H-6: Therapist must belong to the specific branch.
	if in.TherapistID != nil {
		th, err := s.therapists.FindByID(ctx, *in.TherapistID)
		if err != nil || th.TenantID != in.CallerTenantID || th.BranchID != branch.ID || !th.IsActive {
			return CreateBookingOutput{}, constants.ErrTherapistNotFound
		}
		if ok, err := s.therapistCanPerformService(ctx, *in.TherapistID, in.ServiceID); err != nil || !ok {
			return CreateBookingOutput{}, constants.ErrTherapistNotForService
		}
	}

	// Auto-assign if not specified.
	therapistID := in.TherapistID
	if therapistID == nil {
		id, err := s.autoAssignTherapist(ctx, in.CallerTenantID, branch.ID, in.ServiceID, start, end)
		if err != nil {
			return CreateBookingOutput{}, err
		}
		therapistID = &id
	}

	// Pre-flight prep-buffer conflict check (migration 000035).
	if therapistID != nil {
		busy, err := s.isTherapistBusyWithPrep(ctx, *therapistID, start, end)
		if err != nil {
			return CreateBookingOutput{}, fmt.Errorf("pre-flight conflict check: %w", err)
		}
		if busy {
			return CreateBookingOutput{}, constants.ErrBookingTherapistConflict
		}
	}

	roomID := in.RoomID
	if roomID == nil {
		id, err := s.autoAssignRoom(ctx, in.CallerTenantID, branch.ID, start, end)
		if err != nil {
			return CreateBookingOutput{}, err
		}
		if id != "" {
			roomID = &id
		}
	}

	// C-1: Server-side price computation.
	total := svc.PriceIDR
	for _, a := range addonRows {
		total += a.PriceIDR
	}

	code, err := s.generateUniqueCode(ctx)
	if err != nil {
		return CreateBookingOutput{}, err
	}

	now := s.clock.Now()
	paidAt := now.Format(time.RFC3339)
	paymentMethod := model.BookingPaymentAtVenue
	b := &model.Booking{
		ID:            uuid.New().String(),
		TenantID:      in.CallerTenantID,
		BranchID:      branch.ID,
		ServiceID:     svc.ID,
		RoomID:        roomID,
		TherapistID:   therapistID,
		CustomerName:  in.CustomerName,
		CustomerPhone: in.CustomerPhone,
		CustomerEmail: in.CustomerEmail,
		Code:          code,
		ScheduledStart: start,
		ScheduledEnd:   end,
		TotalPriceIDR: total,
		PaymentMethod: &paymentMethod,
		Status:        model.BookingStatusPaid, // concierge = immediate paid
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	parsedPaidAt := now
	b.PaidAt = &parsedPaidAt
	_ = paidAt // used in audit below

	if err := s.bookings.Save(ctx, b); err != nil {
		return CreateBookingOutput{}, fmt.Errorf("save concierge booking: %w", err)
	}

	bookingAddons := make([]*model.BookingAddon, len(addonRows))
	for i, a := range addonRows {
		bookingAddons[i] = &model.BookingAddon{
			BookingID: b.ID,
			AddonID:   a.ID,
			PriceIDR:  a.PriceIDR,
		}
	}
	if err := s.bookings.SaveAddons(ctx, bookingAddons); err != nil {
		return CreateBookingOutput{}, fmt.Errorf("save concierge booking addons: %w", err)
	}

	// Send confirmation email (non-fatal).
	go s.sendConfirmationEmail(context.Background(), b, svc.Name, branch.Name)

	_ = s.audit.Append(ctx, AuditEntry{
		TenantID:     &in.CallerTenantID,
		ActorUserID:  &in.CallerUserID,
		Action:       "booking.created_concierge",
		ResourceType: "booking",
		ResourceID:   b.ID,
		Meta:         map[string]interface{}{"code": b.Code},
	})

	detail, err := s.buildBookingDetail(ctx, b)
	if err != nil {
		return CreateBookingOutput{}, err
	}
	return CreateBookingOutput{BookingDetail: detail}, nil
}

// Get returns the full booking detail for an operator.
func (s *BookingService) Get(ctx context.Context, in GetBookingInput) (BookingDetail, error) {
	b, err := s.bookings.FindByID(ctx, in.BookingID)
	if err != nil {
		return BookingDetail{}, err
	}
	if b.TenantID != in.CallerTenantID {
		return BookingDetail{}, constants.ErrBookingNotFound
	}
	if !in.IsAdmin && !containsBranch(in.CallerBranches, b.BranchID) {
		return BookingDetail{}, constants.ErrBookingNotFound
	}
	return s.buildBookingDetail(ctx, b)
}

// GetByCode returns the full booking detail for an operator (code lookup path).
func (s *BookingService) GetByCode(ctx context.Context, in GetBookingInput) (BookingDetail, error) {
	b, err := s.bookings.FindByCode(ctx, in.BookingID) // BookingID carries the code in this variant
	if err != nil {
		return BookingDetail{}, err
	}
	if b.TenantID != in.CallerTenantID {
		return BookingDetail{}, constants.ErrBookingNotFound
	}
	if !in.IsAdmin && !containsBranch(in.CallerBranches, b.BranchID) {
		return BookingDetail{}, constants.ErrBookingNotFound
	}
	return s.buildBookingDetail(ctx, b)
}

// List returns a paginated list of bookings for an operator.
func (s *BookingService) List(ctx context.Context, in ListBookingsInput) (ListBookingsOutput, error) {
	filter := BookingFilter{
		BranchID:  in.BranchID,
		Status:    in.Status,
		ServiceID: in.ServiceID,
		FromDate:  in.FromDate,
		ToDate:    in.ToDate,
		Page:      in.Page,
		Limit:     in.Limit,
	}

	// Branch-scope restriction for branch_admin callers.
	if !in.IsAdmin && len(in.CallerBranches) > 0 {
		if filter.BranchID == nil {
			// No specific branch requested — restrict to caller's branches.
			// Use the first branch as a representative filter; a more complete
			// implementation would use a BranchIDs []string filter. For Phase 5
			// we restrict to the first branch to keep the query simple.
			filter.BranchID = &in.CallerBranches[0]
		} else if !containsBranch(in.CallerBranches, *filter.BranchID) {
			return ListBookingsOutput{}, constants.ErrCrossBranchForbidden
		}
	}

	rows, total, err := s.bookings.FindByTenant(ctx, in.CallerTenantID, filter)
	if err != nil {
		return ListBookingsOutput{}, fmt.Errorf("list bookings: %w", err)
	}

	limit := in.Limit
	if limit <= 0 {
		limit = 10
	}
	totalPages := int((total + int64(limit) - 1) / int64(limit))
	details := make([]BookingDetail, 0, len(rows))
	for _, b := range rows {
		d, err := s.buildBookingDetail(ctx, b)
		if err != nil {
			return ListBookingsOutput{}, err
		}
		details = append(details, d)
	}
	return ListBookingsOutput{
		Bookings:   details,
		Page:       in.Page,
		TotalCount: total,
		TotalPages: totalPages,
	}, nil
}

// CheckIn transitions a booking from paid → checked_in.
func (s *BookingService) CheckIn(ctx context.Context, in CheckInInput) (BookingDetail, error) {
	b, err := s.bookings.FindByID(ctx, in.BookingID)
	if err != nil {
		return BookingDetail{}, err
	}
	if b.TenantID != in.CallerTenantID {
		return BookingDetail{}, constants.ErrBookingNotFound
	}
	if !in.IsAdmin && !containsBranch(in.CallerBranches, b.BranchID) {
		return BookingDetail{}, constants.ErrCrossBranchForbidden
	}
	if b.Status != model.BookingStatusPaid {
		return BookingDetail{}, constants.ErrBookingInvalidStatusTransition
	}
	// Verify code matches.
	if in.Code != "" && in.Code != b.Code {
		return BookingDetail{}, constants.ErrBookingCodeInvalid
	}

	now := s.clock.Now().Format(time.RFC3339)
	n, err := s.bookings.TransitionStatus(ctx, TransitionStatusInput{
		BookingID:      in.BookingID,
		ExpectedStatus: model.BookingStatusPaid,
		NewStatus:      model.BookingStatusCheckedIn,
		CheckedInAt:    &now,
		CheckedInBy:    &in.CallerUserID,
	})
	if err != nil {
		return BookingDetail{}, err
	}
	if n == 0 {
		return BookingDetail{}, constants.ErrBookingInvalidStatusTransition
	}

	_ = s.audit.Append(ctx, AuditEntry{
		TenantID:     &in.CallerTenantID,
		ActorUserID:  &in.CallerUserID,
		Action:       "booking.checked_in",
		ResourceType: "booking",
		ResourceID:   in.BookingID,
	})

	b.Status = model.BookingStatusCheckedIn
	return s.buildBookingDetail(ctx, b)
}

// Complete transitions a booking from checked_in → completed.
func (s *BookingService) Complete(ctx context.Context, in CompleteInput) (BookingDetail, error) {
	b, err := s.bookings.FindByID(ctx, in.BookingID)
	if err != nil {
		return BookingDetail{}, err
	}
	if b.TenantID != in.CallerTenantID {
		return BookingDetail{}, constants.ErrBookingNotFound
	}
	if !in.IsAdmin && !containsBranch(in.CallerBranches, b.BranchID) {
		return BookingDetail{}, constants.ErrCrossBranchForbidden
	}
	if b.Status != model.BookingStatusCheckedIn {
		return BookingDetail{}, constants.ErrBookingInvalidStatusTransition
	}

	now := s.clock.Now().Format(time.RFC3339)
	n, err := s.bookings.TransitionStatus(ctx, TransitionStatusInput{
		BookingID:      in.BookingID,
		ExpectedStatus: model.BookingStatusCheckedIn,
		NewStatus:      model.BookingStatusCompleted,
		CompletedAt:    &now,
		CompletedBy:    &in.CallerUserID,
	})
	if err != nil {
		return BookingDetail{}, err
	}
	if n == 0 {
		return BookingDetail{}, constants.ErrBookingInvalidStatusTransition
	}

	_ = s.audit.Append(ctx, AuditEntry{
		TenantID:     &in.CallerTenantID,
		ActorUserID:  &in.CallerUserID,
		Action:       "booking.completed",
		ResourceType: "booking",
		ResourceID:   in.BookingID,
	})

	b.Status = model.BookingStatusCompleted
	return s.buildBookingDetail(ctx, b)
}

// MarkNoShow transitions a booking from paid or checked_in → no_show.
func (s *BookingService) MarkNoShow(ctx context.Context, in NoShowInput) (BookingDetail, error) {
	b, err := s.bookings.FindByID(ctx, in.BookingID)
	if err != nil {
		return BookingDetail{}, err
	}
	if b.TenantID != in.CallerTenantID {
		return BookingDetail{}, constants.ErrBookingNotFound
	}
	if !in.IsAdmin && !containsBranch(in.CallerBranches, b.BranchID) {
		return BookingDetail{}, constants.ErrCrossBranchForbidden
	}
	if b.Status != model.BookingStatusPaid && b.Status != model.BookingStatusCheckedIn {
		return BookingDetail{}, constants.ErrBookingInvalidStatusTransition
	}

	n, err := s.bookings.TransitionStatus(ctx, TransitionStatusInput{
		BookingID:      in.BookingID,
		ExpectedStatus: b.Status,
		NewStatus:      model.BookingStatusNoShow,
	})
	if err != nil {
		return BookingDetail{}, err
	}
	if n == 0 {
		return BookingDetail{}, constants.ErrBookingInvalidStatusTransition
	}

	_ = s.audit.Append(ctx, AuditEntry{
		TenantID:     &in.CallerTenantID,
		ActorUserID:  &in.CallerUserID,
		Action:       "booking.no_show",
		ResourceType: "booking",
		ResourceID:   in.BookingID,
	})

	b.Status = model.BookingStatusNoShow
	return s.buildBookingDetail(ctx, b)
}

// Cancel transitions a booking to cancelled (ops force-cancel).
// Allowed from: paid, checked_in, pending_payment.
// When cancelled from pending_payment the associated awaiting payment_transaction
// is marked voided so the customer's outstanding QR becomes inert.
func (s *BookingService) Cancel(ctx context.Context, in CancelInput) (BookingDetail, error) {
	if in.Reason == "" {
		return BookingDetail{}, fmt.Errorf("%w: cancel reason is required", constants.ErrInvalidInput)
	}

	b, err := s.bookings.FindByID(ctx, in.BookingID)
	if err != nil {
		return BookingDetail{}, err
	}
	if b.TenantID != in.CallerTenantID {
		return BookingDetail{}, constants.ErrBookingNotFound
	}
	if !in.IsAdmin && !containsBranch(in.CallerBranches, b.BranchID) {
		return BookingDetail{}, constants.ErrCrossBranchForbidden
	}

	cancelFromPending := b.Status == model.BookingStatusPendingPayment
	allowed := cancelFromPending ||
		b.Status == model.BookingStatusPaid ||
		b.Status == model.BookingStatusCheckedIn
	if !allowed {
		return BookingDetail{}, constants.ErrBookingInvalidStatusTransition
	}

	// When cancelling a pending_payment booking, void the awaiting
	// payment_transaction so the customer's QR code becomes inert.
	// Non-fatal if no transaction exists (paid_at_venue concierge booking).
	if cancelFromPending {
		if voidErr := s.paymentSvc.VoidTransactionForBooking(ctx, in.BookingID); voidErr != nil {
			slog.WarnContext(ctx, "cancel: void payment_transaction (non-fatal)",
				"booking_id", in.BookingID, "error", voidErr)
		}
	}

	now := s.clock.Now().Format(time.RFC3339)
	n, err := s.bookings.TransitionStatus(ctx, TransitionStatusInput{
		BookingID:      in.BookingID,
		ExpectedStatus: b.Status,
		NewStatus:      model.BookingStatusCancelled,
		CancelledAt:    &now,
		CancelledBy:    &in.CallerUserID,
		CancelReason:   &in.Reason,
	})
	if err != nil {
		return BookingDetail{}, err
	}
	if n == 0 {
		return BookingDetail{}, constants.ErrBookingInvalidStatusTransition
	}

	// Differentiate audit action so ops can distinguish cancels from each source status.
	auditAction := "booking.cancelled"
	if cancelFromPending {
		auditAction = "booking.cancelled_pending"
	}
	_ = s.audit.Append(ctx, AuditEntry{
		TenantID:     &in.CallerTenantID,
		ActorUserID:  &in.CallerUserID,
		Action:       auditAction,
		ResourceType: "booking",
		ResourceID:   in.BookingID,
		Meta: map[string]interface{}{
			"reason":          in.Reason,
			"previous_status": string(b.Status),
		},
	})

	b.Status = model.BookingStatusCancelled
	return s.buildBookingDetail(ctx, b)
}

// SweepExpired transitions all overdue pending_payment bookings AND their
// associated payment_transaction rows to expired in the same call.
// Returns the count of booking rows swept.
// ADR 0015 §5: payment_transaction rows WHERE status='awaiting' AND
// qr_expires_at < now() are also expired here.
func (s *BookingService) SweepExpired(ctx context.Context) (int, error) {
	swept, err := s.bookings.SweepExpired(ctx)
	if err != nil {
		return swept, err
	}
	// Sweep expired payment_transaction rows alongside bookings (non-fatal).
	if _, ptxnErr := s.paymentSvc.SweepExpiredTransactions(ctx); ptxnErr != nil {
		slog.WarnContext(ctx, "payment_transaction expiry sweep error (non-fatal)", "error", ptxnErr)
	}
	return swept, nil
}

// GetReportSummary returns aggregate booking metrics for the reports page.
func (s *BookingService) GetReportSummary(ctx context.Context, in GetReportInput) (BookingReportSummary, error) {
	return s.bookings.ReportSummary(ctx, BookingReportFilter{
		TenantID: in.CallerTenantID,
		BranchID: in.BranchID,
		FromDate: in.FromDate,
		ToDate:   in.ToDate,
	})
}

// HandlePaymentWebhook is kept for backward-compat on the BookingServiceIface.
// Phase 6: this method is a thin pass-through to PaymentService.HandleWebhook.
// The raw-body + headers path is handled by the payment controller directly;
// this shim exists so the controller interface does not break during transition.
//
// Deprecated: call PaymentServiceIface.HandleWebhook directly from the controller.
func (s *BookingService) HandlePaymentWebhook(ctx context.Context, rawPayload []byte, headers map[string]string) error {
	return s.paymentSvc.HandleWebhook(ctx, rawPayload, headers)
}

// ListAvailableSlots computes available time slots for a service on a given date.
// Calls SweepExpired first to ensure stale pending_payment rows don't block slots.
//
// Two-pass approach (avoids N+1):
//   Pass 1 — build candidate slot windows + counts (existing therapist/room count logic).
//   Pass 2 — single bulk query for booked room IDs per slot window; optional
//             single bulk query for therapist booking conflicts when TherapistID given.
//
// TherapistAvailable semantics (when TherapistID supplied):
//   true  = therapist has no overlapping booking AND their weekly schedule covers the slot.
//   false = either they are booked OR their schedule does not cover the slot.
// The UI should show a "Terapis sudah dibooking di jam ini" reason when
// TherapistAvailable=false but TherapistsAvailableCount > 0 (i.e. other therapists
// are still free — the slot itself is not blocked, only this specific therapist).
func (s *BookingService) ListAvailableSlots(ctx context.Context, in AvailableSlotsInput) ([]Slot, error) {
	// Pivot RLS to __public__ sentinel — this endpoint is hit by the customer
	// mobile app without a JWT, so the default __platform__ tenant context
	// would block every SELECT against branch/service/booking. Same pattern
	// as ListPublicBranches + GetPublicBranchDetail.
	if err := s.tx.SetTenantContext(ctx, constants.PublicTenantSentinel, ""); err != nil {
		return nil, fmt.Errorf("set public tenant context: %w", err)
	}

	// Lazy expiry sweep before computing availability.
	if _, err := s.bookings.SweepExpired(ctx); err != nil {
		slog.WarnContext(ctx, "expiry sweep error (non-fatal)", "error", err)
	}

	branch, err := s.branches.FindByID(ctx, in.BranchID)
	if err != nil {
		return nil, constants.ErrBranchNotFound
	}

	svc, err := s.services.FindByID(ctx, in.ServiceID)
	if err != nil || svc.TenantID != branch.TenantID || !svc.IsActive {
		return nil, constants.ErrServiceNotFound
	}

	// Parse date.
	date, err := time.Parse("2006-01-02", in.Date)
	if err != nil {
		return nil, fmt.Errorf("%w: date must be YYYY-MM-DD", constants.ErrInvalidInput)
	}

	// Branch operational hours bound the overall slot grid; therapist windows
	// are intersected with this range during chain generation. Defaults to
	// 09:00–22:00 when branch.OperationalHours is empty/unset; otherwise the
	// per-DOW range from the JSONB blob is used. A null/missing entry on a day
	// means the branch is closed → return zero slots.
	//
	// JSONB shape (set by tenant-admin's OperationalHoursField):
	//   {"mon":"09:00-22:00","tue":"09:00-22:00",...,"sun":null}
	dayStart := time.Date(date.Year(), date.Month(), date.Day(), 9, 0, 0, 0, jakartaLocation)
	dayEnd := time.Date(date.Year(), date.Month(), date.Day(), 22, 0, 0, 0, jakartaLocation)
	if raw := strings.TrimSpace(string(branch.OperationalHours)); raw != "" && raw != "{}" {
		dowKeys := []string{"sun", "mon", "tue", "wed", "thu", "fri", "sat"}
		dowKey := dowKeys[int(date.Weekday())]
		var hours map[string]*string
		if jerr := json.Unmarshal(branch.OperationalHours, &hours); jerr == nil && len(hours) > 0 {
			v, ok := hours[dowKey]
			if ok && v == nil {
				return []Slot{}, nil
			}
			if ok && v != nil {
				if dash := strings.Index(*v, "-"); dash > 0 {
					fromStr := strings.TrimSpace((*v)[:dash])
					toStr := strings.TrimSpace((*v)[dash+1:])
					if t, perr := time.ParseInLocation("15:04", fromStr, jakartaLocation); perr == nil {
						dayStart = time.Date(date.Year(), date.Month(), date.Day(), t.Hour(), t.Minute(), 0, 0, jakartaLocation)
					}
					if t, perr := time.ParseInLocation("15:04", toStr, jakartaLocation); perr == nil {
						dayEnd = time.Date(date.Year(), date.Month(), date.Day(), t.Hour(), t.Minute(), 0, 0, jakartaLocation)
					}
				}
			}
		}
	}
	duration := time.Duration(svc.DurationMinutes) * time.Minute

	// --- Pass A: list eligible therapists for the service (branch-wide) ---

	// Fetch all active rooms at the branch once (used for both count and ID set).
	allRooms, _, _ := s.rooms.FindByTenant(ctx, branch.TenantID, RoomFilter{
		BranchID: &branch.ID,
		IsActive: boolPtr(true),
		Page:     1,
		Limit:    200,
	})
	allRoomIDs := make([]string, len(allRooms))
	for i, rm := range allRooms {
		allRoomIDs[i] = rm.ID
	}

	// Fetch all active therapists at the branch mapped to this service. This
	// replaces the single countAvailableTherapists call with a list we can
	// use for per-slot counting in Pass D.
	allTherapists, _, _ := s.therapists.FindByTenant(ctx, branch.TenantID, TherapistFilter{
		BranchID: &branch.ID,
		IsActive: boolPtr(true),
		Page:     1,
		Limit:    200,
	})
	// Build prep-minutes lookup for chained-slot generation.
	therapistPrep := make(map[string]int, len(allTherapists))
	var eligibleTherapistIDs []string
	for _, th := range allTherapists {
		ok, _ := s.therapistCanPerformService(ctx, th.ID, in.ServiceID)
		if ok {
			eligibleTherapistIDs = append(eligibleTherapistIDs, th.ID)
			therapistPrep[th.ID] = th.PrepMinutes
		}
	}

	// day-of-week for Pass B (all slots on the same date share the same DOW).
	dow := int(dayStart.Weekday()) // time.Sunday=0 … time.Saturday=6

	// --- Pass B: bulk-fetch availability windows for eligible therapists ---
	// Single query for therapist_availability WHERE therapist_id IN (?) AND day_of_week = ?
	availRows, availErr := s.availability.FindByTherapistsAndDOW(ctx, eligibleTherapistIDs, dow)
	if availErr != nil {
		slog.WarnContext(ctx, "FindByTherapistsAndDOW error (non-fatal)", "error", availErr)
		availRows = nil
	}
	// Build map[therapistID][]window for O(1) lookup in Pass D.
	type availWindow struct {
		startTime string // "HH:MM:SS"
		endTime   string // "HH:MM:SS"
	}
	therapistWindows := make(map[string][]availWindow, len(eligibleTherapistIDs))
	for _, row := range availRows {
		therapistWindows[row.TherapistID] = append(therapistWindows[row.TherapistID], availWindow{
			startTime: row.StartTime,
			endTime:   row.EndTime,
		})
	}

	// --- Chained candidate-slot generation ---
	//
	// For each therapist's availability window, generate slots stepping by
	// (duration + prep_minutes). This ensures the grid shown to customers
	// reflects the actual bookable sequence rather than a fixed 30-minute
	// cadence that doesn't align with how therapist capacity is consumed.
	//
	// Algorithm per window:
	//   windowStart = max(branch_dayStart, HH:MM parse of row.StartTime)
	//   windowEnd   = min(branch_dayEnd,   HH:MM parse of row.EndTime)
	//   step        = duration + therapistPrep[therapistID]
	//   for slotStart = windowStart; slotStart + duration <= windowEnd; slotStart += step
	//       emit candidateSlot{slotStart, slotStart + duration}
	//
	// When TherapistID is supplied (specific-therapist mode) only that therapist's
	// windows drive the chain. In Otomatis mode the union of all eligible
	// therapists' chains is emitted (deduplicated by start time).

	type candidateSlot struct {
		start time.Time
		end   time.Time
	}

	// parseWindowBound converts "HH:MM:SS" (or "HH:MM") into a full time.Time
	// on the requested date (UTC). Returns fallback when the string is empty or
	// cannot be parsed.
	parseWindowBound := func(hhmmss string, fallback time.Time) time.Time {
		if hhmmss == "" {
			return fallback
		}
		h, m, sec := 0, 0, 0
		if len(hhmmss) >= 8 {
			if _, scanErr := fmt.Sscanf(hhmmss, "%d:%d:%d", &h, &m, &sec); scanErr != nil {
				return fallback
			}
		} else if len(hhmmss) >= 5 {
			if _, scanErr := fmt.Sscanf(hhmmss, "%d:%d", &h, &m); scanErr != nil {
				return fallback
			}
		} else {
			return fallback
		}
		return time.Date(date.Year(), date.Month(), date.Day(), h, m, sec, 0, jakartaLocation)
	}

	// generateChain emits all candidate slots for a single availability window,
	// clipped to [dayStart, dayEnd].
	generateChain := func(therapistID string, win availWindow) []candidateSlot {
		wStart := parseWindowBound(win.startTime, dayStart)
		wEnd := parseWindowBound(win.endTime, dayEnd)

		// Clip window to branch operational hours.
		if wStart.Before(dayStart) {
			wStart = dayStart
		}
		if wEnd.After(dayEnd) {
			wEnd = dayEnd
		}

		prep := time.Duration(therapistPrep[therapistID]) * time.Minute
		step := duration + prep
		if step <= 0 {
			return nil
		}

		var chain []candidateSlot
		for slotStart := wStart; ; slotStart = slotStart.Add(step) {
			slotEnd := slotStart.Add(duration)
			if slotEnd.After(wEnd) {
				break
			}
			chain = append(chain, candidateSlot{start: slotStart, end: slotEnd})
		}
		return chain
	}

	// Collect candidates: single-therapist or Otomatis union.
	seen := make(map[string]struct{}) // dedup by RFC3339 start
	var candidates []candidateSlot

	if in.TherapistID != nil {
		// Single-therapist mode: only this therapist's windows.
		for _, win := range therapistWindows[*in.TherapistID] {
			for _, slot := range generateChain(*in.TherapistID, win) {
				key := slot.start.Format(time.RFC3339)
				if _, dup := seen[key]; !dup {
					seen[key] = struct{}{}
					candidates = append(candidates, slot)
				}
			}
		}
	} else {
		// Otomatis mode: union of all eligible therapists' chains.
		for _, tid := range eligibleTherapistIDs {
			for _, win := range therapistWindows[tid] {
				for _, slot := range generateChain(tid, win) {
					key := slot.start.Format(time.RFC3339)
					if _, dup := seen[key]; !dup {
						seen[key] = struct{}{}
						candidates = append(candidates, slot)
					}
				}
			}
		}
	}

	// Sort candidates by start time so the response is ordered.
	slices.SortFunc(candidates, func(a, b candidateSlot) int {
		return a.start.Compare(b.start)
	})

	if len(candidates) == 0 {
		return []Slot{}, nil
	}

	// --- Pass C: bulk-fetch conflicting bookings for eligible therapists ---
	// Single query: booking JOIN therapist on therapist_id, with effective_end
	// already widened by prep_minutes. Covers the whole day.
	conflicts, conflictErr := s.bookings.FindTherapistConflicts(ctx, eligibleTherapistIDs, dayStart, dayEnd)
	if conflictErr != nil {
		slog.WarnContext(ctx, "FindTherapistConflicts error (non-fatal)", "error", conflictErr)
		conflicts = nil
	}
	// Build map[therapistID][]TherapistBookingInterval for Pass D.
	therapistIntervals := make(map[string][]TherapistBookingInterval, len(eligibleTherapistIDs))
	for _, iv := range conflicts {
		therapistIntervals[iv.TherapistID] = append(therapistIntervals[iv.TherapistID], iv)
	}

	// --- Pass 2: bulk-fetch booked room IDs per slot window ---
	windows := make([]SlotWindow, len(candidates))
	for i, c := range candidates {
		windows[i] = SlotWindow{
			Start: c.start.Format(time.RFC3339),
			End:   c.end.Format(time.RFC3339),
		}
	}

	bookedRoomsBySlot, err := s.bookings.FindBookedRoomIDsInSlots(ctx, branch.ID, windows)
	if err != nil {
		// Non-fatal: log and fall back to empty (all rooms available).
		slog.WarnContext(ctx, "FindBookedRoomIDsInSlots error (non-fatal)", "error", err)
		bookedRoomsBySlot = map[string][]string{}
	}

	// --- Pass 3 (optional): bulk-fetch single-therapist booking conflicts ---
	// When TherapistID is supplied we still use the single-therapist path for
	// the TherapistAvailable boolean field (pre-existing semantics preserved).
	// prep_minutes is read from the therapistPrep map built in Pass A; if the
	// therapist is not in the map (e.g. not eligible), fall back to FindByID.
	var therapistBookedBySlot map[string]bool
	if in.TherapistID != nil {
		prepMinutes := therapistPrep[*in.TherapistID]
		if prepMinutes == 0 {
			// Fallback: therapist not in eligible list — fetch directly.
			if th, thErr := s.therapists.FindByID(ctx, *in.TherapistID); thErr == nil {
				prepMinutes = th.PrepMinutes
			} else {
				slog.WarnContext(ctx, "FindByID therapist for prep_minutes (non-fatal)", "error", thErr)
			}
		}
		therapistBookedBySlot, err = s.bookings.IsTherapistBookedInSlots(ctx, *in.TherapistID, windows, prepMinutes)
		if err != nil {
			slog.WarnContext(ctx, "IsTherapistBookedInSlots error (non-fatal)", "error", err)
			therapistBookedBySlot = map[string]bool{}
		}
	}

	// --- Pass D: per-slot count using bulk-fetched data ---
	// For each candidate slot, count how many eligible therapists:
	//   (1) have an availability window covering (dow, slotStart, slotEnd), AND
	//   (2) have no effective booking interval overlapping (slot.start, slot.end).
	//
	// therapistCoveredByWindow returns true when at least one window for the
	// therapist covers [slotStartHHMMSS, slotEndHHMMSS] on the current DOW.
	therapistCoveredByWindow := func(therapistID, slotStartHHMMSS, slotEndHHMMSS string) bool {
		for _, w := range therapistWindows[therapistID] {
			// start_time ≤ slotStart AND end_time ≥ slotEnd
			if w.startTime <= slotStartHHMMSS && w.endTime >= slotEndHHMMSS {
				return true
			}
		}
		return false
	}

	// therapistHasConflict returns true when at least one effective booking
	// interval for the therapist overlaps the candidate slot [slotStart, slotEnd).
	therapistHasConflict := func(therapistID string, slotStart, slotEnd time.Time) bool {
		for _, iv := range therapistIntervals[therapistID] {
			// Standard half-open interval overlap: start < other_end && end > other_start
			if slotStart.Before(iv.EffectiveEnd) && slotEnd.After(iv.EffectiveStart) {
				return true
			}
		}
		return false
	}

	// --- Assemble slots ---
	var slots []Slot
	for _, c := range candidates {
		startKey := c.start.Format(time.RFC3339)
		slotStartHHMMSS := c.start.Format("15:04:05")
		slotEndHHMMSS := c.end.Format("15:04:05")

		// Compute available room IDs for this slot (all rooms minus booked ones).
		bookedSet := make(map[string]bool, len(bookedRoomsBySlot[startKey]))
		for _, rid := range bookedRoomsBySlot[startKey] {
			bookedSet[rid] = true
		}
		availRoomIDs := make([]string, 0, len(allRoomIDs))
		for _, rid := range allRoomIDs {
			if !bookedSet[rid] {
				availRoomIDs = append(availRoomIDs, rid)
			}
		}
		roomsAvail := len(availRoomIDs)

		// --- Pass D: per-slot therapist count ---
		// Count eligible therapists who cover this slot AND have no conflict.
		therapistCount := 0
		for _, tid := range eligibleTherapistIDs {
			if !therapistCoveredByWindow(tid, slotStartHHMMSS, slotEndHHMMSS) {
				continue
			}
			if therapistHasConflict(tid, c.start, c.end) {
				continue
			}
			therapistCount++
		}

		// Emit every slot, including those with therapistCount==0, so the UI can
		// render them as disabled with reason "Tidak ada therapist tersedia".
		// (Previously count==0 caused a continue; the Flutter side already handles
		// count==0 as a disabled slot — see acceptance criteria.)
		slot := Slot{
			Start:                    c.start.Format(time.RFC3339),
			End:                      c.end.Format(time.RFC3339),
			TherapistsAvailableCount: therapistCount,
			RoomsAvailableCount:      roomsAvail,
			AvailableRoomIDs:         availRoomIDs,
		}

		// Per-therapist availability check (only when TherapistID supplied).
		// Preserves pre-existing TherapistAvailable field semantics.
		if in.TherapistID != nil {
			isBooked := therapistBookedBySlot[startKey]
			if isBooked {
				// Therapist has a conflicting booking — mark unavailable.
				f := false
				slot.TherapistAvailable = &f
			} else {
				// Check whether the therapist's weekly schedule covers this slot.
				covers, covErr := s.availability.TherapistCoversSlot(ctx, *in.TherapistID, dow, slotStartHHMMSS, slotEndHHMMSS)
				if covErr != nil {
					slog.WarnContext(ctx, "TherapistCoversSlot error (non-fatal)", "error", covErr)
					covers = false
				}
				slot.TherapistAvailable = &covers
			}
		}

		slots = append(slots, slot)
	}

	if slots == nil {
		slots = []Slot{}
	}
	return slots, nil
}

// ---------------------------------------------------------------------------
// Public branch listing (no auth)
// ---------------------------------------------------------------------------

// GetPublicBranchDetail mengembalikan detail lengkap satu cabang beserta
// layanan, terapis, dan ruangan aktif untuk layar booking pelanggan.
// RLS sentinel __public__ memastikan cabang/tenant tidak aktif tidak terlihat.
func (s *BookingService) GetPublicBranchDetail(ctx context.Context, branchID string) (PublicBranchDetail, error) {
	if err := s.tx.SetTenantContext(ctx, constants.PublicTenantSentinel, ""); err != nil {
		return PublicBranchDetail{}, fmt.Errorf("set public tenant context: %w", err)
	}

	b, err := s.branches.FindByID(ctx, branchID)
	if err != nil {
		return PublicBranchDetail{}, err
	}

	out := PublicBranchDetail{
		PublicBranchSummary: PublicBranchSummary{
			ID:               b.ID,
			TenantID:         b.TenantID,
			Name:             b.Name,
			City:             b.City,
			Province:         b.Province,
			AddressLine1:     b.AddressLine1,
			ContactPhone:     b.ContactPhone,
			ContactEmail:     b.ContactEmail,
			Latitude:         b.Latitude,
			Longitude:        b.Longitude,
			OperationalHours: b.OperationalHours,
		},
		Services:   []ServiceDetail{},
		Therapists: []TherapistDetail{},
		Rooms:      []RoomDetail{},
		Addons:     []AddonDetail{},
	}

	// Ambil nama tenant untuk ditampilkan di picker UI.
	// Kesalahan non-fatal: field dikosongkan jika tenant tidak ditemukan.
	if t, tErr := s.tenants.FindByID(ctx, b.TenantID); tErr == nil {
		out.TenantName = t.Name
	} else {
		slog.WarnContext(ctx, "tenant lookup failed for public branch detail (non-fatal)",
			"branch_id", branchID, "tenant_id", b.TenantID, "error", tErr)
	}

	// Pengambilan catalog best-effort: error diabaikan agar layar detail
	// tetap merender dengan data yang berhasil dimuat.
	trueVal := true
	srvRows, _, _ := s.services.FindByTenant(ctx, b.TenantID, ServiceFilter{
		IsActive: &trueVal,
		Page:     1,
		Limit:    200,
	})
	for _, sv := range srvRows {
		out.Services = append(out.Services, toServiceDetail(sv))
	}

	thRows, _, _ := s.therapists.FindByTenant(ctx, b.TenantID, TherapistFilter{
		BranchID: &b.ID,
		IsActive: &trueVal,
		Page:     1,
		Limit:    200,
	})
	for _, t := range thRows {
		d := toTherapistDetail(t)
		// Resolusi URL foto terapis (ADR 0011): PhotoKey tidak pernah
		// dikirim ke wire — hanya resolved URL yang diteruskan.
		if t.PhotoKey != nil && s.storage != nil {
			if u, uErr := s.storage.URL(ctx, *t.PhotoKey); uErr == nil {
				d.PhotoKey = &u // sementara pakai field PhotoKey sebagai carrier
			} else {
				slog.WarnContext(ctx, "therapist photo URL resolution failed (non-fatal)",
					"therapist_id", t.ID, "error", uErr)
				d.PhotoKey = nil
			}
		}
		// Populate active service mappings so the customer mobile can hide
		// therapists that don't perform the selected service.
		if mappings, mErr := s.therapistSvc.FindByTherapistID(ctx, t.ID); mErr == nil {
			ids := make([]string, 0, len(mappings))
			for _, m := range mappings {
				if m.IsActive {
					ids = append(ids, m.ServiceID)
				}
			}
			d.ServiceIDs = ids
		} else {
			slog.WarnContext(ctx, "therapist service mapping lookup failed (non-fatal)",
				"therapist_id", t.ID, "error", mErr)
		}
		out.Therapists = append(out.Therapists, d)
	}

	roomRows, _, _ := s.rooms.FindByTenant(ctx, b.TenantID, RoomFilter{
		BranchID: &b.ID,
		IsActive: &trueVal,
		Page:     1,
		Limit:    200,
	})
	for _, rm := range roomRows {
		d := toRoomDetail(rm)
		// Resolusi URL foto ruangan (ADR 0011).
		if rm.PhotoKey != nil && s.storage != nil {
			if u, uErr := s.storage.URL(ctx, *rm.PhotoKey); uErr == nil {
				d.PhotoKey = &u
			} else {
				slog.WarnContext(ctx, "room photo URL resolution failed (non-fatal)",
					"room_id", rm.ID, "error", uErr)
				d.PhotoKey = nil
			}
		}
		out.Rooms = append(out.Rooms, d)
	}

	// Ambil add-on aktif tenant-wide (ADR 0010): add-on tidak terikat cabang,
	// cukup satu query per tenant. Best-effort: error tidak memblokir response.
	addonRows, _, _ := s.addons.FindByTenant(ctx, b.TenantID, AddonFilter{
		IsActive: &trueVal,
		Page:     1,
		Limit:    200,
	})
	for _, a := range addonRows {
		out.Addons = append(out.Addons, toAddonDetail(a))
	}

	return out, nil
}

// ListPublicBranches returns active branches visible to the public.
func (s *BookingService) ListPublicBranches(ctx context.Context, filter PublicBranchFilter) ([]PublicBranchSummary, int64, error) {
	branchFilter := BranchFilter{
		Status: model.BranchStatusActive,
		Page:   filter.Page,
		Limit:  filter.Limit,
	}
	// Pivot the session-level tenant context to the __public__ sentinel so
	// the additive `branch_public_select` RLS policy (migration 000028) fires.
	// Without this, the default `__platform__` sentinel from tenant middleware
	// produces zero rows because branch.tenant_id never equals '__platform__'.
	if err := s.tx.SetTenantContext(ctx, constants.PublicTenantSentinel, ""); err != nil {
		return nil, 0, fmt.Errorf("set public tenant context: %w", err)
	}
	rows, total, err := s.branches.FindByTenant(ctx, "", branchFilter)
	if err != nil {
		return nil, 0, fmt.Errorf("list public branches: %w", err)
	}

	summaries := make([]PublicBranchSummary, 0, len(rows))
	for _, b := range rows {
		sum := PublicBranchSummary{
			ID:               b.ID,
			TenantID:         b.TenantID,
			Name:             b.Name,
			City:             b.City,
			Province:         b.Province,
			AddressLine1:     b.AddressLine1,
			ContactPhone:     b.ContactPhone,
			ContactEmail:     b.ContactEmail,
			Latitude:         b.Latitude,
			Longitude:        b.Longitude,
			OperationalHours: b.OperationalHours,
		}
		if filter.Lat != nil && filter.Lng != nil && b.Latitude != nil && b.Longitude != nil {
			dist := haversineMeters(*filter.Lat, *filter.Lng, *b.Latitude, *b.Longitude)
			sum.DistanceMeters = &dist
		}
		summaries = append(summaries, sum)
	}

	// Sort by distance if lat/lng provided.
	if filter.Lat != nil && filter.Lng != nil {
		sortByDistance(summaries)
	}

	return summaries, total, nil
}

// ---------------------------------------------------------------------------
// Internal helpers
// ---------------------------------------------------------------------------

// generateUniqueCode generates a Crockford Base32 booking code "XXXX-XXXX".
// Uses crypto/rand. Retries on UNIQUE collision (bookingCodeMaxRetries times).
func (s *BookingService) generateUniqueCode(ctx context.Context) (string, error) {
	for i := 0; i < bookingCodeMaxRetries; i++ {
		b := make([]byte, 5)
		if _, err := rand.Read(b); err != nil {
			return "", fmt.Errorf("generate booking code: %w", err)
		}
		encoded := crockfordEncoding.EncodeToString(b) // 8 chars
		if len(encoded) < 8 {
			continue
		}
		code := strings.ToUpper(encoded[:4]) + "-" + strings.ToUpper(encoded[4:8])

		// Check for collision (extremely unlikely at < 10k bookings, but defensive).
		_, err := s.bookings.FindByCode(ctx, code)
		if errors.Is(err, constants.ErrBookingNotFound) {
			// No collision — use this code.
			return code, nil
		}
		// Collision — retry.
	}
	return "", fmt.Errorf("failed to generate unique booking code after %d attempts", bookingCodeMaxRetries)
}

// isTherapistBusyWithPrep returns true when the therapist has an active booking
// whose effective interval [scheduled_start, scheduled_end + prep_minutes)
// overlaps the candidate window [start, end). This is the service-layer
// pre-flight check that enforces the prep buffer (migration 000035) before
// the INSERT reaches the DB-level GiST exclusion constraint.
//
// The GiST constraint covers exact scheduled_end; this method covers the extra
// prep window that the constraint intentionally does NOT enforce at the DB level.
func (s *BookingService) isTherapistBusyWithPrep(ctx context.Context, therapistID string, start, end time.Time) (bool, error) {
	th, err := s.therapists.FindByID(ctx, therapistID)
	if err != nil {
		return false, err
	}
	window := SlotWindow{
		Start: start.Format(time.RFC3339),
		End:   end.Format(time.RFC3339),
	}
	result, err := s.bookings.IsTherapistBookedInSlots(ctx, therapistID, []SlotWindow{window}, th.PrepMinutes)
	if err != nil {
		return false, fmt.Errorf("therapist conflict check: %w", err)
	}
	return result[window.Start], nil
}

// therapistCanPerformService returns true when the therapist has an active
// mapping to the given service via the therapist_service table.
func (s *BookingService) therapistCanPerformService(ctx context.Context, therapistID, serviceID string) (bool, error) {
	mappings, err := s.therapistSvc.FindByTherapistID(ctx, therapistID)
	if err != nil {
		return false, err
	}
	for _, m := range mappings {
		if m.ServiceID == serviceID && m.IsActive {
			return true, nil
		}
	}
	return false, nil
}

// autoAssignTherapist picks the first available therapist for a slot.
// Sort order: sort_order ASC, id ASC (deterministic per ADR 0014 §3.6).
func (s *BookingService) autoAssignTherapist(ctx context.Context, tenantID, branchID, serviceID string, start, end time.Time) (string, error) {
	// Find all active therapists at this branch.
	rows, _, err := s.therapists.FindByTenant(ctx, tenantID, TherapistFilter{
		BranchID: &branchID,
		IsActive: boolPtr(true),
		Page:     1,
		Limit:    200,
	})
	if err != nil {
		return "", fmt.Errorf("auto assign therapist: %w", err)
	}

	// Filter to those who can perform the service.
	var candidates []string
	for _, th := range rows {
		ok, err := s.therapistCanPerformService(ctx, th.ID, serviceID)
		if err == nil && ok {
			candidates = append(candidates, th.ID)
		}
	}

	if len(candidates) == 0 {
		return "", constants.ErrNoTherapistAvailable
	}

	// Return the first candidate (list is already sorted by full_name/id from repo).
	// A more precise implementation would check the exclusion constraint directly;
	// for Phase 5 we return the first and let the DB constraint catch conflicts.
	return candidates[0], nil
}

// autoAssignRoom picks the first active room not already booked at [start, end).
// Returns "" (let caller leave room_id NULL) when every active room is booked
// or the branch has no rooms — operators reassign manually using a backup room
// outside the system. The booking_room exclusion constraint ignores NULL rows
// (WHERE room_id IS NOT NULL), so NULL bookings never conflict at the DB.
func (s *BookingService) autoAssignRoom(ctx context.Context, tenantID, branchID string, start, end time.Time) (string, error) {
	rows, _, err := s.rooms.FindByTenant(ctx, tenantID, RoomFilter{
		BranchID: &branchID,
		IsActive: boolPtr(true),
		Page:     1,
		Limit:    200,
	})
	if err != nil {
		return "", fmt.Errorf("auto assign room: %w", err)
	}
	if len(rows) == 0 {
		return "", nil
	}

	booked, err := s.bookings.FindBookedRoomIDsInSlots(ctx, branchID, []SlotWindow{{
		Start: start.Format(time.RFC3339),
		End:   end.Format(time.RFC3339),
	}})
	if err != nil {
		return "", fmt.Errorf("auto assign room (overlap check): %w", err)
	}
	bookedSet := make(map[string]struct{})
	for _, ids := range booked {
		for _, id := range ids {
			bookedSet[id] = struct{}{}
		}
	}
	for _, rm := range rows {
		if _, taken := bookedSet[rm.ID]; !taken {
			return rm.ID, nil
		}
	}
	return "", nil
}

// countAvailableTherapists counts therapists at a branch available for a slot.
func (s *BookingService) countAvailableTherapists(ctx context.Context, tenantID, branchID, serviceID string, _, _ time.Time) int {
	rows, _, err := s.therapists.FindByTenant(ctx, tenantID, TherapistFilter{
		BranchID: &branchID,
		IsActive: boolPtr(true),
		Page:     1,
		Limit:    200,
	})
	if err != nil {
		return 0
	}
	count := 0
	for _, th := range rows {
		ok, _ := s.therapistCanPerformService(ctx, th.ID, serviceID)
		if ok {
			count++
		}
	}
	return count
}

// countAvailableRooms counts rooms at a branch available for a slot.
func (s *BookingService) countAvailableRooms(ctx context.Context, tenantID, branchID string, _, _ time.Time) int {
	rows, _, err := s.rooms.FindByTenant(ctx, tenantID, RoomFilter{
		BranchID: &branchID,
		IsActive: boolPtr(true),
		Page:     1,
		Limit:    200,
	})
	if err != nil {
		return 0
	}
	return len(rows)
}

// buildBookingDetail assembles a BookingDetail from a Booking model row by
// loading related entities (service name, branch name, etc.).
func (s *BookingService) buildBookingDetail(ctx context.Context, b *model.Booking) (BookingDetail, error) {
	addons, _ := s.bookings.FindAddonsByBooking(ctx, b.ID)
	addonSnapshots := buildAddonSnapshots(addons, nil)

	svcName, branchName, roomName, therapistName := "", "", (*string)(nil), (*string)(nil)
	if svc, err := s.services.FindByID(ctx, b.ServiceID); err == nil {
		svcName = svc.Name
	}
	if branch, err := s.branches.FindByID(ctx, b.BranchID); err == nil {
		branchName = branch.Name
	}
	if b.RoomID != nil {
		if room, err := s.rooms.FindByID(ctx, *b.RoomID); err == nil {
			roomName = &room.Name
		}
	}
	if b.TherapistID != nil {
		if th, err := s.therapists.FindByID(ctx, *b.TherapistID); err == nil {
			therapistName = &th.FullName
		}
	}

	d := BookingDetail{
		ID:              b.ID,
		TenantID:        b.TenantID,
		BranchID:        b.BranchID,
		BranchName:      branchName,
		ServiceID:       b.ServiceID,
		ServiceName:     svcName,
		RoomID:          b.RoomID,
		RoomName:        roomName,
		TherapistID:     b.TherapistID,
		TherapistName:   therapistName,
		CustomerName:    b.CustomerName,
		CustomerPhone:   b.CustomerPhone,
		CustomerEmail:   b.CustomerEmail,
		Code:            b.Code,
		ScheduledStart:  b.ScheduledStart.Format(time.RFC3339),
		ScheduledEnd:    b.ScheduledEnd.Format(time.RFC3339),
		TotalPriceIDR:   b.TotalPriceIDR,
		PaymentMethod:   b.PaymentMethod,
		PaymentReference: b.PaymentReference,
		Status:          b.Status,
		Addons:          addonSnapshots,
		CreatedAt:       b.CreatedAt.Format(time.RFC3339),
		UpdatedAt:       b.UpdatedAt.Format(time.RFC3339),
	}
	if b.PaidAt != nil {
		t := b.PaidAt.Format(time.RFC3339)
		d.PaidAt = &t
	}
	if b.CancelledAt != nil {
		t := b.CancelledAt.Format(time.RFC3339)
		d.CancelledAt = &t
	}
	d.CancelledBy = b.CancelledBy
	d.CancelReason = b.CancelReason
	if b.CheckedInAt != nil {
		t := b.CheckedInAt.Format(time.RFC3339)
		d.CheckedInAt = &t
	}
	d.CheckedInBy = b.CheckedInBy
	if b.CompletedAt != nil {
		t := b.CompletedAt.Format(time.RFC3339)
		d.CompletedAt = &t
	}
	d.CompletedBy = b.CompletedBy

	return d, nil
}

// storePaymentReference writes the snap token to payment_reference on the booking.
func (s *BookingService) storePaymentReference(ctx context.Context, bookingID, ref string) error {
	_, err := s.bookings.TransitionStatus(ctx, TransitionStatusInput{
		BookingID:        bookingID,
		ExpectedStatus:   model.BookingStatusPendingPayment,
		NewStatus:        model.BookingStatusPendingPayment, // no status change
		PaymentReference: &ref,
	})
	return err
}

// sendConfirmationEmail sends the booking confirmation email.
// Non-fatal: caller goroutine ignores errors.
func (s *BookingService) sendConfirmationEmail(ctx context.Context, b *model.Booking, svcName, branchName string) {
	if s.email == nil {
		return
	}
	body := fmt.Sprintf(
		"Halo %s,\n\nBooking Anda telah dikonfirmasi.\n\nKode Booking: %s\nLayanan: %s\nCabang: %s\nWaktu: %s\nTotal: Rp %s\n\nTunjukkan kode ini kepada staff di cabang.\n\nTerima kasih,\nLustia",
		b.CustomerName,
		b.Code,
		svcName,
		branchName,
		b.ScheduledStart.Format("02 Jan 2006 15:04"),
		formatIDR(b.TotalPriceIDR),
	)
	_ = s.email.Send(ctx, EmailMessage{
		To:       b.CustomerEmail,
		Subject:  "Booking dikonfirmasi — " + b.Code,
		TextBody: body,
	})
}

// buildAddonSnapshots converts booking_addon rows to AddonSnapshot list.
// nameMap is optional; when nil names are omitted.
func buildAddonSnapshots(addons []*model.BookingAddon, nameMap map[string]string) []AddonSnapshot {
	out := make([]AddonSnapshot, 0, len(addons))
	for _, a := range addons {
		name := ""
		if nameMap != nil {
			name = nameMap[a.AddonID]
		}
		out = append(out, AddonSnapshot{
			AddonID:  a.AddonID,
			Name:     name,
			PriceIDR: a.PriceIDR,
		})
	}
	return out
}

// maskPhone masks a phone number, exposing only the last 4 characters.
// e.g. "+62812345678" → "****5678"
func maskPhone(phone string) string {
	if len(phone) <= 4 {
		return "****"
	}
	return "****" + phone[len(phone)-4:]
}

// maskEmail masks an email address, exposing first 2 chars + domain.
// e.g. "firstname@example.com" → "fi**@example.com"
func maskEmail(email string) string {
	parts := strings.SplitN(email, "@", 2)
	if len(parts) != 2 {
		return "****@****"
	}
	local := parts[0]
	if len(local) <= 2 {
		return local + "**@" + parts[1]
	}
	return local[:2] + "**@" + parts[1]
}

// parseIDRAmount parses a Midtrans gross_amount string to int64.
func parseIDRAmount(s string) (int64, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, errors.New("empty gross_amount")
	}
	// Handle decimal: "150000.00"
	dotIdx := strings.IndexByte(s, '.')
	if dotIdx >= 0 {
		s = s[:dotIdx]
	}
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("parse idr amount %q: %w", s, err)
	}
	return n, nil
}

// formatIDR formats an IDR amount with thousand separators.
func formatIDR(n int64) string {
	s := strconv.FormatInt(n, 10)
	// Insert dots every 3 digits from the right.
	var buf strings.Builder
	for i, ch := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			buf.WriteByte('.')
		}
		buf.WriteRune(ch)
	}
	return buf.String()
}

// haversineMeters returns the great-circle distance in metres between two
// (lat, lng) coordinate pairs. Used for branch distance sorting.
func haversineMeters(lat1, lng1, lat2, lng2 float64) float64 {
	const R = 6371000.0 // Earth radius in metres
	const deg2rad = 3.14159265358979323846 / 180.0
	dLat := (lat2 - lat1) * deg2rad
	dLng := (lng2 - lng1) * deg2rad
	a := sinHalf(dLat)*sinHalf(dLat) +
		cosRad(lat1)*cosRad(lat2)*sinHalf(dLng)*sinHalf(dLng)
	c := 2.0 * asin(sqrtF(a))
	return R * c
}

// sortByDistance sorts PublicBranchSummary slices by DistanceMeters ascending.
func sortByDistance(summaries []PublicBranchSummary) {
	slices.SortStableFunc(summaries, func(a, b PublicBranchSummary) int {
		da, db := float64(0), float64(0)
		if a.DistanceMeters != nil {
			da = *a.DistanceMeters
		}
		if b.DistanceMeters != nil {
			db = *b.DistanceMeters
		}
		if da < db {
			return -1
		}
		if da > db {
			return 1
		}
		return 0
	})
}

// boolPtr returns a pointer to a bool literal.
func boolPtr(b bool) *bool { return &b }

// Note: containsBranch is declared in therapist_svc.go (same package).

// ---------------------------------------------------------------------------
// Minimal math helpers to avoid importing math (avoid heavy init for simple ops).
// ---------------------------------------------------------------------------

func sinHalf(x float64) float64 {
	// sin(x/2) approximated via Taylor for small x is fine; use stdlib-compatible
	// implementation.
	return sinF(x / 2)
}

func sinF(x float64) float64 {
	// Use the standard math.Sin via wrapping.
	// We import math in the init block to keep the file self-contained.
	return mathSin(x)
}

func cosRad(latDeg float64) float64 {
	return mathCos(latDeg * (3.14159265358979323846 / 180.0))
}

func sqrtF(x float64) float64 { return mathSqrt(x) }
func asin(x float64) float64  { return mathAsin(x) }
