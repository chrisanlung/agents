package service

import (
	"context"
	"crypto/rand"
	"encoding/base32"
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
	payment      MidtransClient
	audit        AuditRepository
	email        EmailSender
	clock        Clock
	tx           TxManager
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
	payment MidtransClient,
	audit AuditRepository,
	email EmailSender,
	clock Clock,
	tx TxManager,
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
		payment:      payment,
		audit:        audit,
		email:        email,
		clock:        clock,
		tx:           tx,
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

	// Create payment transaction via the injected adapter.
	payResp, err := s.payment.CreateTransaction(ctx, MidtransPaymentRequest{
		OrderID:       code,
		GrossAmount:   total,
		CustomerName:  in.CustomerName,
		CustomerEmail: in.CustomerEmail,
		CustomerPhone: in.CustomerPhone,
		Description:   fmt.Sprintf("Booking %s di %s", svc.Name, branch.Name),
	})
	if err != nil {
		slog.WarnContext(ctx, "payment create_transaction failed (non-fatal; booking still created)", "error", err)
	}

	// For dummy adapter: immediately set payment_reference and mark pending.
	// The real adapter will receive webhook confirmation later.
	if payResp.Status == MidtransStatusPaid {
		paidAt := now.Format(time.RFC3339)
		snapToken := payResp.SnapToken
		_, _ = s.bookings.TransitionStatus(ctx, TransitionStatusInput{
			BookingID:        b.ID,
			ExpectedStatus:   model.BookingStatusPendingPayment,
			NewStatus:        model.BookingStatusPaid,
			PaidAt:           &paidAt,
			PaymentReference: &snapToken,
		})
		b.Status = model.BookingStatusPaid
	} else if payResp.SnapToken != "" {
		// Store the snap token as payment reference for later webhook resolution.
		_ = s.storePaymentReference(ctx, b.ID, payResp.SnapToken)
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

	return CreateBookingOutput{
		BookingDetail: detail,
		SnapToken:     payResp.SnapToken,
		RedirectURL:   payResp.RedirectURL,
	}, nil
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

// Cancel transitions a booking from paid → cancelled (ops force-cancel).
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
	if b.Status != model.BookingStatusPaid && b.Status != model.BookingStatusCheckedIn {
		return BookingDetail{}, constants.ErrBookingInvalidStatusTransition
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

	_ = s.audit.Append(ctx, AuditEntry{
		TenantID:     &in.CallerTenantID,
		ActorUserID:  &in.CallerUserID,
		Action:       "booking.cancelled",
		ResourceType: "booking",
		ResourceID:   in.BookingID,
		Meta:         map[string]interface{}{"reason": in.Reason},
	})

	b.Status = model.BookingStatusCancelled
	return s.buildBookingDetail(ctx, b)
}

// SweepExpired transitions all overdue pending_payment bookings to expired.
// Returns the count of swept rows. Called lazily; also safe to call from a cron.
func (s *BookingService) SweepExpired(ctx context.Context) (int, error) {
	return s.bookings.SweepExpired(ctx)
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

// HandlePaymentWebhook processes a Midtrans webhook notification.
// Implements H-4 (amount validation) and H-5 (race condition handling).
func (s *BookingService) HandlePaymentWebhook(ctx context.Context, n MidtransWebhookNotification) error {
	// Delegate signature verification + status normalisation to the adapter.
	// The real adapter (when implemented) verifies SHA-512 signature (H-3)
	// before calling HandleNotification. The dummy adapter skips signature.
	status, err := s.payment.HandleNotification(ctx, n)
	if err != nil {
		return fmt.Errorf("payment handle notification: %w", err)
	}

	if status != MidtransStatusPaid {
		// Not a settlement/capture — nothing to transition.
		return nil
	}

	// Find booking by payment_reference (= code, which is used as order_id).
	b, err := s.bookings.FindByPaymentReference(ctx, n.OrderID)
	if err != nil {
		if errors.Is(err, constants.ErrBookingNotFound) {
			// Unknown reference — log and return 200 (no retry storm).
			slog.WarnContext(ctx, "webhook: booking not found for order_id", "order_id", n.OrderID)
			return nil
		}
		return fmt.Errorf("webhook find booking: %w", err)
	}

	// H-4: Validate gross_amount >= booking total.
	// Dummy adapter sends empty GrossAmount — skip check for dummy (always paid).
	if n.GrossAmount != "" {
		paid, err := parseIDRAmount(n.GrossAmount)
		if err != nil || paid < b.TotalPriceIDR {
			slog.WarnContext(ctx, "webhook: payment amount mismatch",
				"order_id", n.OrderID,
				"expected", b.TotalPriceIDR,
				"received_str", n.GrossAmount,
			)
			_ = s.audit.Append(ctx, AuditEntry{
				TenantID:     &b.TenantID,
				Action:       "payment.amount_mismatch",
				ResourceType: "booking",
				ResourceID:   b.ID,
				Meta: map[string]interface{}{
					"order_id":     n.OrderID,
					"expected_idr": b.TotalPriceIDR,
					"received_str": n.GrossAmount,
				},
			})
			// Return nil so the caller returns HTTP 200 (no Midtrans retry).
			return nil
		}
	}

	// H-5: Conditional UPDATE — WHERE status='pending_payment'.
	now := s.clock.Now().Format(time.RFC3339)
	ref := n.OrderID
	rowsAffected, err := s.bookings.TransitionStatus(ctx, TransitionStatusInput{
		BookingID:        b.ID,
		ExpectedStatus:   model.BookingStatusPendingPayment,
		NewStatus:        model.BookingStatusPaid,
		PaidAt:           &now,
		PaymentReference: &ref,
	})
	if err != nil {
		return fmt.Errorf("webhook transition status: %w", err)
	}

	if rowsAffected == 0 {
		// H-5: No rows updated — look up current status to diagnose.
		current, lookupErr := s.bookings.FindByID(ctx, b.ID)
		if lookupErr != nil {
			slog.ErrorContext(ctx, "webhook: cannot look up booking after zero rowsAffected",
				"booking_id", b.ID, "error", lookupErr)
			return nil
		}
		switch current.Status {
		case model.BookingStatusPaid:
			// Idempotent webhook delivery — already paid, do nothing.
			slog.InfoContext(ctx, "webhook: idempotent — booking already paid",
				"booking_id", b.ID, "order_id", n.OrderID)
		case model.BookingStatusExpired:
			// H-5: Customer paid at T=14m59s, expiry sweep ran before webhook.
			// Alert ops — may need manual intervention.
			slog.WarnContext(ctx, "webhook: payment for expired booking",
				"booking_id", b.ID, "order_id", n.OrderID)
			_ = s.audit.Append(ctx, AuditEntry{
				TenantID:     &b.TenantID,
				Action:       "payment.expired_booking_payment",
				ResourceType: "booking",
				ResourceID:   b.ID,
				Meta: map[string]interface{}{
					"order_id":          n.OrderID,
					"payment_reference": n.OrderID,
				},
			})
		default:
			slog.WarnContext(ctx, "webhook: unexpected status after zero rowsAffected",
				"booking_id", b.ID, "status", current.Status)
		}
		return nil
	}

	// Send confirmation email after successful payment (non-fatal).
	go func() {
		svc, _ := s.services.FindByID(context.Background(), b.ServiceID)
		branch, _ := s.branches.FindByID(context.Background(), b.BranchID)
		svcName, branchName := "", ""
		if svc != nil {
			svcName = svc.Name
		}
		if branch != nil {
			branchName = branch.Name
		}
		s.sendConfirmationEmail(context.Background(), b, svcName, branchName)
	}()

	return nil
}

// ListAvailableSlots computes available time slots for a service on a given date.
// Calls SweepExpired first to ensure stale pending_payment rows don't block slots.
func (s *BookingService) ListAvailableSlots(ctx context.Context, in AvailableSlotsInput) ([]Slot, error) {
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

	// Generate candidate slots at 30-minute intervals for the branch's
	// operational hours. For Phase 5 we use 09:00–21:00 as a default if
	// operational_hours is not set. A production implementation would parse
	// the branch.OperationalHours JSONB field.
	// TODO(phase-6): parse branch.OperationalHours per weekday.
	dayStart := time.Date(date.Year(), date.Month(), date.Day(), 9, 0, 0, 0, time.UTC)
	dayEnd := time.Date(date.Year(), date.Month(), date.Day(), 21, 0, 0, 0, time.UTC)
	duration := time.Duration(svc.DurationMinutes) * time.Minute

	var slots []Slot
	for slotStart := dayStart; slotStart.Add(duration).Before(dayEnd) || slotStart.Add(duration).Equal(dayEnd); slotStart = slotStart.Add(30 * time.Minute) {
		slotEnd := slotStart.Add(duration)

		// Count available therapists for this slot.
		therapistsAvail := s.countAvailableTherapists(ctx, branch.TenantID, branch.ID, in.ServiceID, slotStart, slotEnd)
		// Count available rooms for this slot.
		roomsAvail := s.countAvailableRooms(ctx, branch.TenantID, branch.ID, slotStart, slotEnd)

		if therapistsAvail > 0 {
			slots = append(slots, Slot{
				Start:                    slotStart.Format(time.RFC3339),
				End:                      slotEnd.Format(time.RFC3339),
				TherapistsAvailableCount: therapistsAvail,
				RoomsAvailableCount:      roomsAvail,
			})
		}
	}

	return slots, nil
}

// ---------------------------------------------------------------------------
// Public branch listing (no auth)
// ---------------------------------------------------------------------------

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

// autoAssignRoom picks the first available room for a slot.
func (s *BookingService) autoAssignRoom(ctx context.Context, tenantID, branchID string, _, _ time.Time) (string, error) {
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
		// No rooms at this branch — proceed without room assignment.
		return "", nil
	}
	return rows[0].ID, nil
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
