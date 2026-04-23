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
