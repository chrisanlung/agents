package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/chrisanlung/lustia-auth/internal/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// TherapistServiceRepository implements service.TherapistServiceRepository.
type TherapistServiceRepository struct {
	db *gorm.DB
}

// NewTherapistServiceRepository constructs a TherapistServiceRepository.
func NewTherapistServiceRepository(db *gorm.DB) *TherapistServiceRepository {
	return &TherapistServiceRepository{db: db}
}

// FindByTherapistID returns all mapping rows for a therapist (active + inactive),
// excluding mappings to soft-deleted services.
func (r *TherapistServiceRepository) FindByTherapistID(ctx context.Context, therapistID string) ([]*model.TherapistService, error) {
	db := dbFromContext(ctx, r.db)
	var rows []model.TherapistService
	if err := db.
		Joins("JOIN service s ON s.id = therapist_service.service_id AND s.deleted_at IS NULL").
		Where("therapist_service.therapist_id = ?", therapistID).
		Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("find therapist services: %w", err)
	}
	out := make([]*model.TherapistService, len(rows))
	for i := range rows {
		out[i] = &rows[i]
	}
	return out, nil
}

// FindByServiceID returns all active mapping rows for a service, excluding
// soft-deleted therapists.
func (r *TherapistServiceRepository) FindByServiceID(ctx context.Context, serviceID string) ([]*model.TherapistService, error) {
	db := dbFromContext(ctx, r.db)
	var rows []model.TherapistService
	if err := db.
		Joins("JOIN therapist t ON t.id = therapist_service.therapist_id AND t.deleted_at IS NULL").
		Where("therapist_service.service_id = ? AND therapist_service.is_active = true", serviceID).
		Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("find service therapists: %w", err)
	}
	out := make([]*model.TherapistService, len(rows))
	for i := range rows {
		out[i] = &rows[i]
	}
	return out, nil
}

// ReconcileForTherapist performs the diff-and-set operation per ADR 0009 §Flag #2.
// All three write operations run within the caller's transaction context.
func (r *TherapistServiceRepository) ReconcileForTherapist(ctx context.Context, tenantID, therapistID, callerUserID string, desiredIDs []string) error {
	db := dbFromContext(ctx, r.db)

	// Build a set of desired IDs for O(1) lookups.
	desired := make(map[string]struct{}, len(desiredIDs))
	for _, id := range desiredIDs {
		desired[id] = struct{}{}
	}

	// Fetch all existing mappings for this therapist.
	var existing []model.TherapistService
	if err := db.Where("therapist_id = ?", therapistID).Find(&existing).Error; err != nil {
		return fmt.Errorf("fetch existing mappings: %w", err)
	}

	existingMap := make(map[string]*model.TherapistService, len(existing))
	for i := range existing {
		existingMap[existing[i].ServiceID] = &existing[i]
	}

	now := time.Now().UTC()

	// Step 1 + 2: upsert desired IDs.
	for _, sid := range desiredIDs {
		if row, ok := existingMap[sid]; ok {
			// Existing row — re-activate if currently inactive.
			if !row.IsActive {
				if err := db.Model(&model.TherapistService{}).
					Where("therapist_id = ? AND service_id = ?", therapistID, sid).
					Updates(map[string]interface{}{
						"is_active":  true,
						"updated_by": callerUserID,
					}).Error; err != nil {
					return fmt.Errorf("reactivate mapping %s: %w", sid, err)
				}
			}
			// Already active — no change.
		} else {
			// New row — insert.
			// tenant_id is not a column on therapist_service — tenant scoping
			// is enforced via RLS joining therapist (migration 4 + 15 policies).
			_ = tenantID
			row := &model.TherapistService{
				TherapistID: therapistID,
				ServiceID:   sid,
				IsActive:    true,
				CreatedAt:   now,
				UpdatedAt:   now,
				CreatedBy:   &callerUserID,
				UpdatedBy:   &callerUserID,
			}
			if err := db.Clauses(clause.OnConflict{DoNothing: true}).Create(row).Error; err != nil {
				return fmt.Errorf("insert mapping %s: %w", sid, err)
			}
		}
	}

	// Step 4: deactivate rows not in desired set.
	for sid, row := range existingMap {
		if _, inDesired := desired[sid]; !inDesired && row.IsActive {
			if err := db.Model(&model.TherapistService{}).
				Where("therapist_id = ? AND service_id = ?", therapistID, sid).
				Updates(map[string]interface{}{
					"is_active":  false,
					"updated_by": callerUserID,
				}).Error; err != nil {
				return fmt.Errorf("deactivate mapping %s: %w", sid, err)
			}
		}
	}

	return nil
}

// DeactivateAllForTherapist sets is_active=false on every mapping row for the
// given therapist. Used during therapist soft-delete cascade.
func (r *TherapistServiceRepository) DeactivateAllForTherapist(ctx context.Context, therapistID string) error {
	db := dbFromContext(ctx, r.db)
	if err := db.Model(&model.TherapistService{}).
		Where("therapist_id = ? AND is_active = true", therapistID).
		Update("is_active", false).Error; err != nil {
		return fmt.Errorf("deactivate all mappings for therapist: %w", err)
	}
	return nil
}
