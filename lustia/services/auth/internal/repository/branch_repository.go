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

// BranchRepository implements service.BranchRepository using GORM + PostgreSQL.
type BranchRepository struct {
	db *gorm.DB
}

// NewBranchRepository constructs a BranchRepository.
func NewBranchRepository(db *gorm.DB) *BranchRepository { return &BranchRepository{db: db} }

// FindByID returns a branch by primary key. Returns ErrBranchNotFound when no
// non-deleted row exists.
func (r *BranchRepository) FindByID(ctx context.Context, id string) (*model.Branch, error) {
	db := dbFromContext(ctx, r.db)
	var m model.Branch
	if err := db.Where("id = ? AND deleted_at IS NULL", id).First(&m).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, constants.ErrBranchNotFound
		}
		return nil, fmt.Errorf("find branch by id: %w", err)
	}
	return &m, nil
}

// FindByTenant returns an offset-paginated list of non-deleted branches for a
// given tenant, with an optional status filter, plus the total matching count.
func (r *BranchRepository) FindByTenant(ctx context.Context, tenantID string, filter service.BranchFilter) ([]*model.Branch, int64, error) {
	db := dbFromContext(ctx, r.db)

	limit := filter.Limit
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	page := filter.Page
	if page < 1 {
		page = 1
	}

	q := db.Model(&model.Branch{}).Where("tenant_id = ? AND deleted_at IS NULL", tenantID)
	if filter.Status != "" && filter.Status != "all" {
		q = q.Where("status = ?", filter.Status)
	}

	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count branches: %w", err)
	}

	var rows []model.Branch
	if err := q.Order("created_at DESC").
		Offset((page - 1) * limit).
		Limit(limit).
		Find(&rows).Error; err != nil {
		return nil, 0, fmt.Errorf("find branches by tenant: %w", err)
	}

	out := make([]*model.Branch, len(rows))
	for i := range rows {
		out[i] = &rows[i]
	}
	return out, total, nil
}

// Save inserts a new branch row.
func (r *BranchRepository) Save(ctx context.Context, b *model.Branch) error {
	db := dbFromContext(ctx, r.db)
	if err := db.Create(b).Error; err != nil {
		return fmt.Errorf("save branch: %w", translateDBError(err))
	}
	return nil
}

// Update writes mutable non-status columns on an existing branch row.
func (r *BranchRepository) Update(ctx context.Context, b *model.Branch) error {
	db := dbFromContext(ctx, r.db)
	updates := map[string]interface{}{
		"name":          b.Name,
		"address_line1": b.AddressLine1,
		"address_line2": b.AddressLine2,
		"city":          b.City,
		"province":      b.Province,
		"postal_code":   b.PostalCode,
		"country_code":  b.Country,
		"timezone":      b.Timezone,
		"contact_phone": b.ContactPhone,
		"contact_email": b.ContactEmail,
		"updated_by":    b.UpdatedBy,
	}
	if err := db.Model(&model.Branch{}).Where("id = ?", b.ID).Updates(updates).Error; err != nil {
		return fmt.Errorf("update branch: %w", translateDBError(err))
	}
	return nil
}

// UpdateStatus transitions a branch to a new status, setting activated_at when
// the branch is activated for the first time.
func (r *BranchRepository) UpdateStatus(ctx context.Context, id, newStatus string, activatedAt *time.Time) error {
	db := dbFromContext(ctx, r.db)
	updates := map[string]interface{}{
		"status": newStatus,
	}
	if activatedAt != nil {
		updates["activated_at"] = *activatedAt
	}
	result := db.Model(&model.Branch{}).Where("id = ? AND deleted_at IS NULL", id).Updates(updates)
	if result.Error != nil {
		return fmt.Errorf("update branch status: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return constants.ErrBranchNotFound
	}
	return nil
}

// SoftDelete marks a branch as deleted by setting deleted_at.
// Only inactive branches should be deleted (enforced at the service layer).
func (r *BranchRepository) SoftDelete(ctx context.Context, id string) error {
	db := dbFromContext(ctx, r.db)
	now := time.Now().UTC()
	result := db.Model(&model.Branch{}).
		Where("id = ? AND deleted_at IS NULL", id).
		Update("deleted_at", now)
	if result.Error != nil {
		return fmt.Errorf("soft delete branch: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return constants.ErrBranchNotFound
	}
	return nil
}
