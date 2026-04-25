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
