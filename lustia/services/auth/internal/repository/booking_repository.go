package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/chrisanlung/lustia-auth/internal/constants"
	"github.com/chrisanlung/lustia-auth/internal/model"
	"github.com/chrisanlung/lustia-auth/internal/service"
	"gorm.io/gorm"
)

// BookingRepository implements service.BookingRepository using GORM.
// ADR 0002: exclusion constraint violations on booking.booking_*_no_overlap
// are mapped to constants.ErrBookingSlotConflict at this boundary.
type BookingRepository struct {
	db *gorm.DB
}

// NewBookingRepository constructs a BookingRepository.
func NewBookingRepository(db *gorm.DB) *BookingRepository {
	return &BookingRepository{db: db}
}

// Save inserts a new booking row. The DB GiST exclusion constraint fires here
// for double-booking; translateDBError maps it to ErrBookingSlotConflict.
func (r *BookingRepository) Save(ctx context.Context, b *model.Booking) error {
	db := dbFromContext(ctx, r.db)
	if err := db.Create(b).Error; err != nil {
		return fmt.Errorf("save booking: %w", translateBookingDBError(err))
	}
	return nil
}

// SaveAddons bulk-inserts booking_addon rows for a booking.
func (r *BookingRepository) SaveAddons(ctx context.Context, addons []*model.BookingAddon) error {
	if len(addons) == 0 {
		return nil
	}
	db := dbFromContext(ctx, r.db)
	if err := db.Create(&addons).Error; err != nil {
		return fmt.Errorf("save booking addons: %w", err)
	}
	return nil
}

// FindByID returns a booking by primary key.
func (r *BookingRepository) FindByID(ctx context.Context, id string) (*model.Booking, error) {
	db := dbFromContext(ctx, r.db)
	var b model.Booking
	if err := db.Where("id = ?", id).First(&b).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, constants.ErrBookingNotFound
		}
		return nil, fmt.Errorf("find booking by id: %w", err)
	}
	return &b, nil
}

// FindByCode returns a booking by its human-readable code (tenant-scoped path).
func (r *BookingRepository) FindByCode(ctx context.Context, code string) (*model.Booking, error) {
	db := dbFromContext(ctx, r.db)
	var b model.Booking
	if err := db.Where("code = ?", code).First(&b).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, constants.ErrBookingNotFound
		}
		return nil, fmt.Errorf("find booking by code: %w", err)
	}
	return &b, nil
}

// FindByCodePublic returns a booking by code under the __public__ RLS sentinel.
//
// H-7 (SECURITY.md): WHERE code = $1 is MANDATORY here. The booking_public_select
// RLS policy is additive and does NOT restrict to a specific code by itself.
// Without this predicate it would expose all booking rows under __public__.
// DO NOT add a parameterless variant of this method.
func (r *BookingRepository) FindByCodePublic(ctx context.Context, code string) (*model.Booking, error) {
	if code == "" {
		// H-7: empty code would result in an unbounded scan under __public__ —
		// refuse to execute.
		return nil, constants.ErrBookingNotFound
	}
	db := dbFromContext(ctx, r.db)
	var b model.Booking
	// WHERE code = $1 enforced; do NOT add a parameterless variant.
	if err := db.Where("code = ?", code).First(&b).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, constants.ErrBookingNotFound
		}
		return nil, fmt.Errorf("find booking by code (public): %w", err)
	}
	return &b, nil
}

// FindAddonsByBooking returns all booking_addon rows for the given booking ID.
func (r *BookingRepository) FindAddonsByBooking(ctx context.Context, bookingID string) ([]*model.BookingAddon, error) {
	db := dbFromContext(ctx, r.db)
	var rows []model.BookingAddon
	if err := db.Where("booking_id = ?", bookingID).Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("find booking addons: %w", err)
	}
	out := make([]*model.BookingAddon, len(rows))
	for i := range rows {
		out[i] = &rows[i]
	}
	return out, nil
}

// FindByTenant returns an offset-paginated list of bookings for the given tenant.
func (r *BookingRepository) FindByTenant(ctx context.Context, tenantID string, filter service.BookingFilter) ([]*model.Booking, int64, error) {
	db := dbFromContext(ctx, r.db)

	limit := filter.Limit
	if limit <= 0 || limit > 200 {
		limit = 10
	}
	page := filter.Page
	if page < 1 {
		page = 1
	}

	q := db.Model(&model.Booking{}).Where("tenant_id = ?", tenantID)

	if filter.BranchID != nil {
		q = q.Where("branch_id = ?", *filter.BranchID)
	}
	if filter.Status != nil {
		q = q.Where("status = ?", *filter.Status)
	}
	if filter.ServiceID != nil {
		q = q.Where("service_id = ?", *filter.ServiceID)
	}
	if filter.FromDate != nil {
		q = q.Where("scheduled_start >= ?", *filter.FromDate)
	}
	if filter.ToDate != nil {
		q = q.Where("scheduled_start < ?", *filter.ToDate+" 23:59:59+00")
	}

	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count bookings: %w", err)
	}

	var rows []model.Booking
	if err := q.Order("scheduled_start DESC, created_at DESC").
		Offset((page - 1) * limit).
		Limit(limit).
		Find(&rows).Error; err != nil {
		return nil, 0, fmt.Errorf("find bookings by tenant: %w", err)
	}

	out := make([]*model.Booking, len(rows))
	for i := range rows {
		out[i] = &rows[i]
	}
	return out, total, nil
}

// TransitionStatus performs a conditional UPDATE with WHERE status=$expected.
//
// H-5 (SECURITY.md): The WHERE status=expected clause makes the transition
// atomic. rowsAffected==0 means either the booking was already expired/paid/
// cancelled — the caller distinguishes these cases and handles accordingly.
func (r *BookingRepository) TransitionStatus(ctx context.Context, in service.TransitionStatusInput) (int64, error) {
	db := dbFromContext(ctx, r.db)

	updates := map[string]interface{}{
		"status":     in.NewStatus,
		"updated_at": time.Now().UTC(),
	}

	switch in.NewStatus {
	case model.BookingStatusPaid:
		if in.PaidAt != nil {
			updates["paid_at"] = *in.PaidAt
		}
		if in.PaymentReference != nil {
			updates["payment_reference"] = *in.PaymentReference
		}
	case model.BookingStatusCancelled:
		if in.CancelledAt != nil {
			updates["cancelled_at"] = *in.CancelledAt
		}
		if in.CancelledBy != nil {
			updates["cancelled_by"] = *in.CancelledBy
		}
		if in.CancelReason != nil {
			updates["cancel_reason"] = *in.CancelReason
		}
	case model.BookingStatusCheckedIn:
		if in.CheckedInAt != nil {
			updates["checked_in_at"] = *in.CheckedInAt
		}
		if in.CheckedInBy != nil {
			updates["checked_in_by"] = *in.CheckedInBy
		}
	case model.BookingStatusCompleted:
		if in.CompletedAt != nil {
			updates["completed_at"] = *in.CompletedAt
		}
		if in.CompletedBy != nil {
			updates["completed_by"] = *in.CompletedBy
		}
	}

	result := db.Model(&model.Booking{}).
		Where("id = ? AND status = ?", in.BookingID, in.ExpectedStatus).
		Updates(updates)
	if result.Error != nil {
		return 0, fmt.Errorf("transition booking status: %w", result.Error)
	}
	return result.RowsAffected, nil
}

// SweepExpired transitions all pending_payment bookings older than 15 minutes
// to 'expired'. Returns the count of rows swept.
// This is the lazy sweep invoked before booking-create and availability-list
// (ADR 0014 §4.1, SECURITY.md M-3).
func (r *BookingRepository) SweepExpired(ctx context.Context) (int, error) {
	db := dbFromContext(ctx, r.db)
	cutoff := time.Now().UTC().Add(-15 * time.Minute)

	result := db.Model(&model.Booking{}).
		Where("status = ? AND created_at < ?", model.BookingStatusPendingPayment, cutoff).
		Updates(map[string]interface{}{
			"status":     model.BookingStatusExpired,
			"updated_at": time.Now().UTC(),
		})
	if result.Error != nil {
		return 0, fmt.Errorf("sweep expired bookings: %w", result.Error)
	}
	return int(result.RowsAffected), nil
}

// FindByPaymentReference looks up a booking by its payment_reference field.
// Used by the webhook handler to resolve a Midtrans order_id to a booking row.
func (r *BookingRepository) FindByPaymentReference(ctx context.Context, reference string) (*model.Booking, error) {
	db := dbFromContext(ctx, r.db)
	var b model.Booking
	if err := db.Where("payment_reference = ?", reference).First(&b).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, constants.ErrBookingNotFound
		}
		return nil, fmt.Errorf("find booking by payment_reference: %w", err)
	}
	return &b, nil
}

// ReportSummary returns aggregate booking metrics for a tenant in a date range.
func (r *BookingRepository) ReportSummary(ctx context.Context, in service.BookingReportFilter) (service.BookingReportSummary, error) {
	db := dbFromContext(ctx, r.db)

	type row struct {
		Status string
		Count  int64
		Total  int64
	}

	q := db.Model(&model.Booking{}).
		Select("status, COUNT(*) as count, COALESCE(SUM(total_price_idr), 0) as total").
		Where("tenant_id = ? AND scheduled_start >= ? AND scheduled_start <= ?",
			in.TenantID, in.FromDate, in.ToDate).
		Group("status")

	if in.BranchID != nil {
		q = q.Where("branch_id = ?", *in.BranchID)
	}

	var rows []row
	if err := q.Scan(&rows).Error; err != nil {
		return service.BookingReportSummary{}, fmt.Errorf("report summary: %w", err)
	}

	var summary service.BookingReportSummary
	for _, r := range rows {
		summary.TotalBookings += r.Count
		switch r.Status {
		case model.BookingStatusPaid, model.BookingStatusCheckedIn, model.BookingStatusCompleted:
			summary.TotalPaidIDR += r.Total
		}
		switch r.Status {
		case model.BookingStatusCompleted:
			summary.CompletedCount = r.Count
		case model.BookingStatusCancelled:
			summary.CancelledCount = r.Count
		case model.BookingStatusNoShow:
			summary.NoShowCount = r.Count
		case model.BookingStatusExpired:
			summary.ExpiredCount = r.Count
		}
	}

	denom := summary.CompletedCount + summary.NoShowCount
	if denom > 0 {
		summary.NoShowRate = float64(summary.NoShowCount) / float64(denom)
	}
	return summary, nil
}

// FindBookedRoomIDsInSlots returns, for each slot window, the set of room UUIDs
// that have at least one non-cancelled/non-expired booking whose scheduled
// window overlaps the slot. Results are keyed by the slot's RFC3339 start time.
//
// Two-pass approach: one aggregate query using CASE/UNNEST avoids N+1 per-slot
// queries. The query uses overlapping interval logic:
//   overlap = scheduled_start < slotEnd AND scheduled_end > slotStart
func (r *BookingRepository) FindBookedRoomIDsInSlots(ctx context.Context, branchID string, windows []service.SlotWindow) (map[string][]string, error) {
	if len(windows) == 0 {
		return map[string][]string{}, nil
	}
	db := dbFromContext(ctx, r.db)

	type resultRow struct {
		SlotStart string
		RoomID    string
	}

	// Build VALUES list for the slot windows so we can do a single JOIN.
	// We use a raw SQL query with Scan for maximum compatibility with the
	// existing GORM setup (avoids needing a lateral join or CTE via GORM DSL).
	//
	// Strategy: for each booking that overlaps ANY of the windows, return the
	// (window_start, room_id) pairs. We then fan out to all overlapping windows.
	// For typical daily slots (24 windows × 60-min service) this is fast.
	activeStatuses := []string{
		model.BookingStatusPendingPayment,
		model.BookingStatusPaid,
		model.BookingStatusCheckedIn,
	}

	// We iterate once per slot window. For the typical slot count (≤ 48 per day)
	// and typical booking volume this is acceptable. A single CTE-based query
	// would be more efficient but requires raw SQL construction with a variable
	// number of params — we keep this simple and correct.
	result := make(map[string][]string, len(windows))
	for _, w := range windows {
		type roomRow struct {
			RoomID string `gorm:"column:room_id"`
		}
		var rows []roomRow
		if err := db.Model(&model.Booking{}).
			Select("room_id").
			Where("branch_id = ? AND room_id IS NOT NULL AND status IN ? AND scheduled_start < ? AND scheduled_end > ?",
				branchID, activeStatuses, w.End, w.Start).
			Scan(&rows).Error; err != nil {
			return nil, fmt.Errorf("find booked room ids in slot %s: %w", w.Start, err)
		}
		ids := make([]string, 0, len(rows))
		for _, row := range rows {
			if row.RoomID != "" {
				ids = append(ids, row.RoomID)
			}
		}
		result[w.Start] = ids
	}
	return result, nil
}

// IsTherapistBookedInSlots returns, for each slot window, whether the given
// therapist has a non-cancelled/non-expired booking overlapping that window.
// Results are keyed by the slot's RFC3339 start time. (false, nil) when the
// therapistID is unknown or has no bookings.
//
// prepMinutes (migration 000035) widens each existing booking's effective end
// by that many minutes when testing overlap: the candidate window [start, end)
// conflicts if it overlaps [booked_start, booked_end + prepMinutes minutes).
// Use prepMinutes=0 to preserve pre-migration behaviour.
func (r *BookingRepository) IsTherapistBookedInSlots(ctx context.Context, therapistID string, windows []service.SlotWindow, prepMinutes int) (map[string]bool, error) {
	if len(windows) == 0 || therapistID == "" {
		return map[string]bool{}, nil
	}
	db := dbFromContext(ctx, r.db)

	activeStatuses := []string{
		model.BookingStatusPendingPayment,
		model.BookingStatusPaid,
		model.BookingStatusCheckedIn,
	}

	result := make(map[string]bool, len(windows))
	for _, w := range windows {
		var count int64
		if prepMinutes > 0 {
			// Widen existing booking's effective end by prepMinutes:
			//   overlap = candidate_start < booked_end + interval
			//          AND candidate_end   > booked_start
			// Uses Postgres interval arithmetic; safe against SQL injection
			// because prepMinutes is an int validated at 0–60 by the service.
			if err := db.Model(&model.Booking{}).
				Where(
					"therapist_id = ? AND status IN ? AND scheduled_start < ? AND scheduled_end + (? * interval '1 minute') > ?",
					therapistID, activeStatuses, w.End, prepMinutes, w.Start,
				).
				Count(&count).Error; err != nil {
				return nil, fmt.Errorf("is therapist booked in slot %s (prep=%d): %w", w.Start, prepMinutes, err)
			}
		} else {
			if err := db.Model(&model.Booking{}).
				Where("therapist_id = ? AND status IN ? AND scheduled_start < ? AND scheduled_end > ?",
					therapistID, activeStatuses, w.End, w.Start).
				Count(&count).Error; err != nil {
				return nil, fmt.Errorf("is therapist booked in slot %s: %w", w.Start, err)
			}
		}
		result[w.Start] = count > 0
	}
	return result, nil
}

// FindTherapistConflicts returns all non-cancelled/non-expired bookings for
// the given therapist IDs that overlap [dayStart, dayEnd), with each row's
// effective end already widened by the therapist's prep_minutes via a JOIN.
// A single query covers all therapists (Pass C of ListAvailableSlots bulk logic).
//
// SQL approach: JOIN booking b ON therapist t via b.therapist_id = t.id so that
// t.prep_minutes is available inline. The effective_end is computed with Postgres
// interval arithmetic. Returns an empty slice when therapistIDs is empty.
func (r *BookingRepository) FindTherapistConflicts(ctx context.Context, therapistIDs []string, dayStart, dayEnd time.Time) ([]service.TherapistBookingInterval, error) {
	if len(therapistIDs) == 0 {
		return []service.TherapistBookingInterval{}, nil
	}
	db := dbFromContext(ctx, r.db)

	activeStatuses := []string{
		model.BookingStatusPendingPayment,
		model.BookingStatusPaid,
		model.BookingStatusCheckedIn,
	}

	type row struct {
		TherapistID    string    `gorm:"column:therapist_id"`
		EffectiveStart time.Time `gorm:"column:effective_start"`
		EffectiveEnd   time.Time `gorm:"column:effective_end"`
	}

	var rows []row
	// The SELECT computes effective_end = scheduled_end + prep_minutes * interval
	// directly in SQL so we never drag full booking rows across the wire. The
	// WHERE clause uses the same widened effective_end to filter out bookings
	// that can't possibly overlap any slot in the day.
	if err := db.Raw(`
		SELECT
			b.therapist_id,
			b.scheduled_start  AS effective_start,
			b.scheduled_end + (t.prep_minutes * interval '1 minute') AS effective_end
		FROM booking b
		INNER JOIN therapist t ON t.id = b.therapist_id
		WHERE b.therapist_id IN ?
		  AND b.status IN ?
		  AND b.scheduled_start < ?
		  AND b.scheduled_end + (t.prep_minutes * interval '1 minute') > ?
	`, therapistIDs, activeStatuses, dayEnd, dayStart).Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("find therapist conflicts: %w", err)
	}

	out := make([]service.TherapistBookingInterval, len(rows))
	for i, r := range rows {
		out[i] = service.TherapistBookingInterval{
			TherapistID:    r.TherapistID,
			EffectiveStart: r.EffectiveStart,
			EffectiveEnd:   r.EffectiveEnd,
		}
	}
	return out, nil
}

// translateBookingDBError extends the generic DB error mapper with
// booking-specific exclusion constraint translation.
// ADR 0002: the GiST exclusion constraint (excl_booking_therapist_no_overlap /
// excl_booking_room_no_overlap) fires with SQLSTATE 23P01 — map it to
// ErrBookingSlotConflict so the service layer returns a clear 409.
func translateBookingDBError(err error) error {
	if err == nil {
		return nil
	}
	// First apply generic translation (unique violations, etc.)
	translated := translateDBError(err)
	// If the generic translator returned ErrConflict, check whether it was
	// an exclusion constraint for the booking table specifically.
	if errors.Is(translated, constants.ErrConflict) {
		return constants.ErrBookingSlotConflict
	}
	return translated
}
