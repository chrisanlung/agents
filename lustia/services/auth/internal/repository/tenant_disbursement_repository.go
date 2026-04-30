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

// TenantDisbursementRepository implements service.TenantDisbursementRepository
// using GORM. RLS is standard tenant isolation; platform-admin bypasses via
// super_admin context set by middleware.
type TenantDisbursementRepository struct {
	db *gorm.DB
}

// NewTenantDisbursementRepository constructs a TenantDisbursementRepository.
func NewTenantDisbursementRepository(db *gorm.DB) *TenantDisbursementRepository {
	return &TenantDisbursementRepository{db: db}
}

// Save inserts a new tenant_disbursement row (status=pending).
func (r *TenantDisbursementRepository) Save(ctx context.Context, d *model.TenantDisbursement) error {
	db := dbFromContext(ctx, r.db)
	if err := db.Create(d).Error; err != nil {
		return fmt.Errorf("save tenant_disbursement: %w", translateDBError(err))
	}
	return nil
}

// FindByID returns a disbursement by primary key.
func (r *TenantDisbursementRepository) FindByID(ctx context.Context, id string) (*model.TenantDisbursement, error) {
	db := dbFromContext(ctx, r.db)
	var d model.TenantDisbursement
	if err := db.Where("id = ?", id).First(&d).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, constants.ErrDisbursementNotFound
		}
		return nil, fmt.Errorf("find tenant_disbursement by id: %w", err)
	}
	return &d, nil
}

// FindByTenant returns a paginated list of disbursements for a tenant.
func (r *TenantDisbursementRepository) FindByTenant(
	ctx context.Context,
	tenantID string,
	filter service.DisbursementFilter,
) ([]*model.TenantDisbursement, int64, error) {
	db := dbFromContext(ctx, r.db)
	q := db.Model(&model.TenantDisbursement{}).Where("tenant_id = ?", tenantID)

	if filter.Status != nil && *filter.Status != "" {
		q = q.Where("status = ?", *filter.Status)
	}

	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count tenant_disbursements: %w", err)
	}

	page, limit := normalisePage(filter.Page, filter.Limit)
	var rows []*model.TenantDisbursement
	if err := q.Order("created_at DESC").
		Offset((page - 1) * limit).Limit(limit).
		Find(&rows).Error; err != nil {
		return nil, 0, fmt.Errorf("find tenant_disbursements by tenant: %w", err)
	}
	return rows, total, nil
}

// List returns a paginated list of all disbursements (platform admin view).
func (r *TenantDisbursementRepository) List(
	ctx context.Context,
	filter service.DisbursementFilter,
) ([]*model.TenantDisbursement, int64, error) {
	db := dbFromContext(ctx, r.db)
	q := db.Model(&model.TenantDisbursement{})

	if filter.TenantID != nil && *filter.TenantID != "" {
		q = q.Where("tenant_id = ?", *filter.TenantID)
	}
	if filter.Status != nil && *filter.Status != "" {
		q = q.Where("status = ?", *filter.Status)
	}

	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count all tenant_disbursements: %w", err)
	}

	page, limit := normalisePage(filter.Page, filter.Limit)
	var rows []*model.TenantDisbursement
	if err := q.Order("created_at DESC").
		Offset((page - 1) * limit).Limit(limit).
		Find(&rows).Error; err != nil {
		return nil, 0, fmt.Errorf("list tenant_disbursements: %w", err)
	}
	return rows, total, nil
}

// UpdateStatus transitions a disbursement to a new status.
// For the transferred transition the bank_reference, notes, transferred_at,
// and transferred_by fields are updated in the same call.
func (r *TenantDisbursementRepository) UpdateStatus(
	ctx context.Context,
	id, newStatus string,
	transferredByUserID *string,
	bankReference, notes *string,
) error {
	db := dbFromContext(ctx, r.db)

	updates := map[string]interface{}{
		"status":     newStatus,
		"updated_at": time.Now(),
	}
	if bankReference != nil {
		updates["bank_reference"] = *bankReference
	}
	if notes != nil {
		updates["notes"] = *notes
	}
	if newStatus == model.DisbursementStatusTransferred {
		now := time.Now()
		updates["transferred_at"] = now
		updates["transferred_by"] = transferredByUserID
	}

	result := db.Model(&model.TenantDisbursement{}).
		Where("id = ?", id).
		Updates(updates)
	if result.Error != nil {
		return fmt.Errorf("update disbursement status: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return constants.ErrDisbursementNotFound
	}
	return nil
}
