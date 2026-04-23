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

// TherapistRepository implements service.TherapistRepository using GORM.
type TherapistRepository struct {
	db *gorm.DB
}

// NewTherapistRepository constructs a TherapistRepository.
func NewTherapistRepository(db *gorm.DB) *TherapistRepository {
	return &TherapistRepository{db: db}
}

// Save inserts a new therapist row.
func (r *TherapistRepository) Save(ctx context.Context, t *model.Therapist) error {
	db := dbFromContext(ctx, r.db)
	if err := db.Create(t).Error; err != nil {
		return fmt.Errorf("save therapist: %w", translateDBError(err))
	}
	return nil
}

// FindByID returns a non-deleted therapist by primary key.
// Returns ErrTherapistNotFound when no matching row exists.
func (r *TherapistRepository) FindByID(ctx context.Context, id string) (*model.Therapist, error) {
	db := dbFromContext(ctx, r.db)
	var m model.Therapist
	if err := db.Where("id = ? AND deleted_at IS NULL", id).First(&m).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, constants.ErrTherapistNotFound
		}
		return nil, fmt.Errorf("find therapist by id: %w", err)
	}
	return &m, nil
}

// FindByTenant returns a cursor-paginated list of non-deleted therapists for
// the given tenant, applying optional filters.
func (r *TherapistRepository) FindByTenant(ctx context.Context, tenantID string, filter service.TherapistFilter) ([]*model.Therapist, string, error) {
	db := dbFromContext(ctx, r.db)

	limit := filter.Limit
	if limit <= 0 || limit > 200 {
		limit = 50
	}

	q := db.Where("tenant_id = ? AND deleted_at IS NULL", tenantID)

	// is_active filter: nil → active-only (default), otherwise apply the flag.
	if filter.IsActive != nil {
		q = q.Where("is_active = ?", *filter.IsActive)
	} else {
		q = q.Where("is_active = true")
	}

	// Branch restriction: applies for branch_admin callers (non-empty BranchIDs)
	// or when a specific branch_id filter is requested.
	if len(filter.BranchIDs) > 0 {
		q = q.Where("branch_id IN ?", filter.BranchIDs)
	} else if filter.BranchID != nil {
		q = q.Where("branch_id = ?", *filter.BranchID)
	}

	if filter.Cursor != "" {
		q = q.Where(
			"(full_name, created_at) > (SELECT full_name, created_at FROM therapist WHERE id = ?)",
			filter.Cursor,
		)
	}

	var rows []model.Therapist
	if err := q.Order("full_name ASC, created_at ASC").Limit(limit + 1).Find(&rows).Error; err != nil {
		return nil, "", fmt.Errorf("find therapists by tenant: %w", err)
	}

	var nextCursor string
	if len(rows) > limit {
		rows = rows[:limit]
		nextCursor = rows[len(rows)-1].ID
	}

	out := make([]*model.Therapist, len(rows))
	for i := range rows {
		out[i] = &rows[i]
	}
	return out, nextCursor, nil
}

// Update writes the mutable profile columns of an existing therapist row.
func (r *TherapistRepository) Update(ctx context.Context, t *model.Therapist) error {
	db := dbFromContext(ctx, r.db)
	updates := map[string]interface{}{
		"full_name":   t.FullName,
		"gender":      t.Gender,
		"phone":       t.Phone,
		"email":       t.Email,
		"bio":         t.Bio,
		"photo_url":   t.PhotoURL,
		"joined_at":   t.JoinedAt,
		"user_id":     t.UserID,
		"updated_by":  t.UpdatedBy,
	}
	if err := db.Model(&model.Therapist{}).Where("id = ? AND deleted_at IS NULL", t.ID).Updates(updates).Error; err != nil {
		return fmt.Errorf("update therapist: %w", translateDBError(err))
	}
	return nil
}

// UpdateStatus sets is_active for a therapist row.
func (r *TherapistRepository) UpdateStatus(ctx context.Context, id string, isActive bool) error {
	db := dbFromContext(ctx, r.db)
	result := db.Model(&model.Therapist{}).
		Where("id = ? AND deleted_at IS NULL", id).
		Update("is_active", isActive)
	if result.Error != nil {
		return fmt.Errorf("update therapist status: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return constants.ErrTherapistNotFound
	}
	return nil
}

// SoftDelete sets deleted_at and is_active=false on the therapist row.
func (r *TherapistRepository) SoftDelete(ctx context.Context, id string) error {
	db := dbFromContext(ctx, r.db)
	now := time.Now().UTC()
	result := db.Model(&model.Therapist{}).
		Where("id = ? AND deleted_at IS NULL", id).
		Updates(map[string]interface{}{
			"deleted_at": now,
			"is_active":  false,
		})
	if result.Error != nil {
		return fmt.Errorf("soft delete therapist: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return constants.ErrTherapistNotFound
	}
	return nil
}
