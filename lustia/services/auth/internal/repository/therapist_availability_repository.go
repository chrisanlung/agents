package repository

import (
	"context"
	"fmt"

	"github.com/chrisanlung/lustia-auth/internal/model"
	"gorm.io/gorm"
)

// TherapistAvailabilityRepository implements service.TherapistAvailabilityRepository.
type TherapistAvailabilityRepository struct {
	db *gorm.DB
}

// NewTherapistAvailabilityRepository constructs a TherapistAvailabilityRepository.
func NewTherapistAvailabilityRepository(db *gorm.DB) *TherapistAvailabilityRepository {
	return &TherapistAvailabilityRepository{db: db}
}

// FindByTherapistID returns all availability rows for a therapist, ordered by
// day_of_week ASC, start_time ASC.
func (r *TherapistAvailabilityRepository) FindByTherapistID(ctx context.Context, therapistID string) ([]*model.TherapistAvailability, error) {
	db := dbFromContext(ctx, r.db)
	var rows []model.TherapistAvailability
	if err := db.
		Where("therapist_id = ?", therapistID).
		Order("day_of_week ASC, start_time ASC").
		Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("find availability by therapist: %w", err)
	}
	out := make([]*model.TherapistAvailability, len(rows))
	for i := range rows {
		out[i] = &rows[i]
	}
	return out, nil
}

// TherapistCoversSlot returns true when the therapist has at least one
// availability window that covers dayOfWeek and whose time range contains
// [slotStart, slotEnd] (both as "HH:MM:SS" strings).
// Returns (false, nil) — not an error — when no row matches (e.g. the therapist
// has no schedule, or the ID is unknown).
func (r *TherapistAvailabilityRepository) TherapistCoversSlot(ctx context.Context, therapistID string, dayOfWeek int, slotStart, slotEnd string) (bool, error) {
	db := dbFromContext(ctx, r.db)
	var count int64
	// start_time ≤ slotStart AND end_time ≥ slotEnd covers the slot fully.
	if err := db.Model(&model.TherapistAvailability{}).
		Where("therapist_id = ? AND day_of_week = ? AND start_time <= ? AND end_time >= ?",
			therapistID, dayOfWeek, slotStart, slotEnd).
		Count(&count).Error; err != nil {
		return false, fmt.Errorf("therapist covers slot: %w", err)
	}
	return count > 0, nil
}

// FindByTherapistsAndDOW returns all availability rows for the given therapist
// IDs on the given day-of-week in a single query. Returns an empty slice (never
// nil) when therapistIDs is empty or no matching rows exist.
func (r *TherapistAvailabilityRepository) FindByTherapistsAndDOW(ctx context.Context, therapistIDs []string, dayOfWeek int) ([]*model.TherapistAvailability, error) {
	if len(therapistIDs) == 0 {
		return []*model.TherapistAvailability{}, nil
	}
	db := dbFromContext(ctx, r.db)
	var rows []model.TherapistAvailability
	if err := db.
		Where("therapist_id IN ? AND day_of_week = ?", therapistIDs, dayOfWeek).
		Order("therapist_id ASC, start_time ASC").
		Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("find availability by therapists and dow: %w", err)
	}
	out := make([]*model.TherapistAvailability, len(rows))
	for i := range rows {
		out[i] = &rows[i]
	}
	return out, nil
}

// ReplaceAllForTherapist atomically deletes all existing rows for the therapist
// and inserts the new set in a single transaction. An empty slice clears all
// availability.
func (r *TherapistAvailabilityRepository) ReplaceAllForTherapist(ctx context.Context, therapistID string, rows []*model.TherapistAvailability) error {
	db := dbFromContext(ctx, r.db)

	// Execute within a transaction (the caller may already be inside one;
	// dbFromContext will return that tx, so this nested call is safe).
	return db.Transaction(func(tx *gorm.DB) error {
		// Delete all existing rows.
		if err := tx.Where("therapist_id = ?", therapistID).
			Delete(&model.TherapistAvailability{}).Error; err != nil {
			return fmt.Errorf("delete existing availability: %w", err)
		}

		if len(rows) == 0 {
			return nil
		}

		if err := tx.Create(&rows).Error; err != nil {
			return fmt.Errorf("insert availability rows: %w", translateDBError(err))
		}
		return nil
	})
}
