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

// ServiceCatalogRepository implements service.ServiceCatalogRepository using GORM.
type ServiceCatalogRepository struct {
	db *gorm.DB
}

// NewServiceCatalogRepository constructs a ServiceCatalogRepository.
func NewServiceCatalogRepository(db *gorm.DB) *ServiceCatalogRepository {
	return &ServiceCatalogRepository{db: db}
}

// Save inserts a new service row.
func (r *ServiceCatalogRepository) Save(ctx context.Context, s *model.ServiceCatalog) error {
	db := dbFromContext(ctx, r.db)
	if err := db.Create(s).Error; err != nil {
		return fmt.Errorf("save service: %w", translateDBError(err))
	}
	return nil
}

// FindByID returns a non-deleted service by primary key.
// Returns ErrServiceNotFound when no matching row exists.
func (r *ServiceCatalogRepository) FindByID(ctx context.Context, id string) (*model.ServiceCatalog, error) {
	db := dbFromContext(ctx, r.db)
	var m model.ServiceCatalog
	if err := db.Where("id = ? AND deleted_at IS NULL", id).First(&m).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, constants.ErrServiceNotFound
		}
		return nil, fmt.Errorf("find service by id: %w", err)
	}
	return &m, nil
}

// FindByTenant returns an offset-paginated list of non-deleted services for the
// given tenant, applying optional filters, plus the total matching count.
func (r *ServiceCatalogRepository) FindByTenant(ctx context.Context, tenantID string, filter service.ServiceFilter) ([]*model.ServiceCatalog, int64, error) {
	db := dbFromContext(ctx, r.db)

	limit := filter.Limit
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	page := filter.Page
	if page < 1 {
		page = 1
	}

	q := db.Model(&model.ServiceCatalog{}).Where("tenant_id = ? AND deleted_at IS NULL", tenantID)

	if filter.IsActive != nil {
		q = q.Where("is_active = ?", *filter.IsActive)
	} else {
		q = q.Where("is_active = true")
	}

	if filter.Category != nil {
		q = q.Where("category = ?", *filter.Category)
	}

	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count services by tenant: %w", err)
	}

	var rows []model.ServiceCatalog
	if err := q.Order("name ASC, created_at ASC").
		Offset((page - 1) * limit).
		Limit(limit).
		Find(&rows).Error; err != nil {
		return nil, 0, fmt.Errorf("find services by tenant: %w", err)
	}

	out := make([]*model.ServiceCatalog, len(rows))
	for i := range rows {
		out[i] = &rows[i]
	}
	return out, total, nil
}

// Update writes the mutable columns of an existing service row.
func (r *ServiceCatalogRepository) Update(ctx context.Context, s *model.ServiceCatalog) error {
	db := dbFromContext(ctx, r.db)
	// GORM's Updates(map) uses literal column names, bypassing struct tags.
	// Column is `price`, not `price_idr` (same trap as branch_repository.go
	// during Phase 3).
	updates := map[string]interface{}{
		"name":             s.Name,
		"description":      s.Description,
		"category":         s.Category,
		"duration_minutes": s.DurationMinutes,
		"price":            s.PriceIDR,
		"updated_by":       s.UpdatedBy,
	}
	if err := db.Model(&model.ServiceCatalog{}).Where("id = ? AND deleted_at IS NULL", s.ID).Updates(updates).Error; err != nil {
		return fmt.Errorf("update service: %w", translateDBError(err))
	}
	return nil
}

// UpdateStatus sets is_active for a service row.
func (r *ServiceCatalogRepository) UpdateStatus(ctx context.Context, id string, isActive bool) error {
	db := dbFromContext(ctx, r.db)
	result := db.Model(&model.ServiceCatalog{}).
		Where("id = ? AND deleted_at IS NULL", id).
		Update("is_active", isActive)
	if result.Error != nil {
		return fmt.Errorf("update service status: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return constants.ErrServiceNotFound
	}
	return nil
}

// SoftDelete sets deleted_at and is_active=false on the service row.
func (r *ServiceCatalogRepository) SoftDelete(ctx context.Context, id string) error {
	db := dbFromContext(ctx, r.db)
	now := time.Now().UTC()
	result := db.Model(&model.ServiceCatalog{}).
		Where("id = ? AND deleted_at IS NULL", id).
		Updates(map[string]interface{}{
			"deleted_at": now,
			"is_active":  false,
		})
	if result.Error != nil {
		return fmt.Errorf("soft delete service: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return constants.ErrServiceNotFound
	}
	return nil
}

// FindByIDs returns all non-deleted services whose IDs appear in ids, scoped to
// the given tenant.
func (r *ServiceCatalogRepository) FindByIDs(ctx context.Context, tenantID string, ids []string) ([]*model.ServiceCatalog, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	db := dbFromContext(ctx, r.db)
	var rows []model.ServiceCatalog
	if err := db.Where("tenant_id = ? AND id IN ? AND deleted_at IS NULL", tenantID, ids).Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("find services by ids: %w", err)
	}
	out := make([]*model.ServiceCatalog, len(rows))
	for i := range rows {
		out[i] = &rows[i]
	}
	return out, nil
}
