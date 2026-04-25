package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/chrisanlung/lustia-auth/internal/constants"
	"github.com/chrisanlung/lustia-auth/internal/model"
	"github.com/chrisanlung/lustia-auth/internal/service"
	"gorm.io/gorm"
)

// RegistrationRepository implements service.RegistrationRepository.
type RegistrationRepository struct {
	db *gorm.DB
}

// NewRegistrationRepository constructs a RegistrationRepository.
func NewRegistrationRepository(db *gorm.DB) *RegistrationRepository {
	return &RegistrationRepository{db: db}
}

// Save inserts a new tenant_registration row.
func (r *RegistrationRepository) Save(ctx context.Context, reg *model.TenantRegistration) error {
	db := dbFromContext(ctx, r.db)
	if err := db.Create(reg).Error; err != nil {
		return fmt.Errorf("save registration: %w", translateDBError(err))
	}
	return nil
}

// FindByID returns a registration by primary key.
func (r *RegistrationRepository) FindByID(ctx context.Context, id string) (*model.TenantRegistration, error) {
	db := dbFromContext(ctx, r.db)
	var m model.TenantRegistration
	if err := db.Where("id = ?", id).First(&m).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, constants.ErrRegistrationNotFound
		}
		return nil, fmt.Errorf("find registration by id: %w", err)
	}
	return &m, nil
}

// FindPendingByEmail returns the pending registration for the given email, or
// ErrRegistrationNotFound when none exists.
func (r *RegistrationRepository) FindPendingByEmail(ctx context.Context, email string) (*model.TenantRegistration, error) {
	db := dbFromContext(ctx, r.db)
	var m model.TenantRegistration
	if err := db.Where("contact_email = ? AND status = ?", email, model.TenantRegistrationStatusPending).
		First(&m).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, constants.ErrRegistrationNotFound
		}
		return nil, fmt.Errorf("find pending registration by email: %w", err)
	}
	return &m, nil
}

// FindPendingBySlug returns the pending registration for the given slug, or
// ErrRegistrationNotFound when none exists.
func (r *RegistrationRepository) FindPendingBySlug(ctx context.Context, slug string) (*model.TenantRegistration, error) {
	db := dbFromContext(ctx, r.db)
	var m model.TenantRegistration
	if err := db.Where("requested_slug = ? AND status = ?", slug, model.TenantRegistrationStatusPending).
		First(&m).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, constants.ErrRegistrationNotFound
		}
		return nil, fmt.Errorf("find pending registration by slug: %w", err)
	}
	return &m, nil
}

// List returns an offset-paginated list of registrations with an optional status
// filter, plus the total matching count. When filter.Status is empty it defaults
// to "pending".
func (r *RegistrationRepository) List(ctx context.Context, filter service.RegistrationFilter) ([]*model.TenantRegistration, int64, error) {
	db := dbFromContext(ctx, r.db)

	limit := filter.Limit
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	page := filter.Page
	if page < 1 {
		page = 1
	}

	statusFilter := filter.Status
	if statusFilter == "" {
		statusFilter = string(model.TenantRegistrationStatusPending)
	}

	q := db.Model(&model.TenantRegistration{})
	if statusFilter != "all" {
		q = q.Where("status = ?", statusFilter)
	}

	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count registrations: %w", err)
	}

	var rows []model.TenantRegistration
	if err := q.Order("created_at DESC").
		Offset((page - 1) * limit).
		Limit(limit).
		Find(&rows).Error; err != nil {
		return nil, 0, fmt.Errorf("list registrations: %w", err)
	}

	out := make([]*model.TenantRegistration, len(rows))
	for i := range rows {
		out[i] = &rows[i]
	}
	return out, total, nil
}

// Update writes mutable columns on an existing registration row (used for
// approval and rejection transitions).
func (r *RegistrationRepository) Update(ctx context.Context, reg *model.TenantRegistration) error {
	db := dbFromContext(ctx, r.db)
	updates := map[string]interface{}{
		"status":              string(reg.Status),
		"approved_tenant_id":  reg.ApprovedTenantID,
		"approved_user_id":    reg.ApprovedUserID,
		"approved_at":         reg.ApprovedAt,
		"approved_by":         reg.ApprovedBy,
		"rejected_at":         reg.RejectedAt,
		"rejected_by":         reg.RejectedBy,
		"rejection_reason":    reg.RejectionReason,
	}
	result := db.Model(&model.TenantRegistration{}).Where("id = ?", reg.ID).Updates(updates)
	if result.Error != nil {
		return fmt.Errorf("update registration: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return constants.ErrRegistrationNotFound
	}
	return nil
}
