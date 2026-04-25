package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/chrisanlung/lustia-auth/internal/constants"
	"github.com/chrisanlung/lustia-auth/internal/model"
	"github.com/chrisanlung/lustia-auth/internal/service"
	"gorm.io/gorm"
)

// AddonRepository implements service.AddonRepository using GORM + PostgreSQL.
// The addon table uses RLS (tenant_id = app.current_tenant), so all queries
// are already tenant-scoped at the DB layer; we keep explicit tenant_id
// predicates as a defence-in-depth measure.
type AddonRepository struct {
	db *gorm.DB
}

// NewAddonRepository constructs an AddonRepository.
func NewAddonRepository(db *gorm.DB) *AddonRepository {
	return &AddonRepository{db: db}
}

// Save inserts a new addon row.
func (r *AddonRepository) Save(ctx context.Context, a *model.Addon) error {
	db := dbFromContext(ctx, r.db)
	if err := db.Create(a).Error; err != nil {
		translated := translateAddonDBError(err)
		return fmt.Errorf("save addon: %w", translated)
	}
	return nil
}

// FindByID returns a non-deleted add-on by primary key.
// Returns ErrAddonNotFound when no matching row exists.
func (r *AddonRepository) FindByID(ctx context.Context, id string) (*model.Addon, error) {
	db := dbFromContext(ctx, r.db)
	var a model.Addon
	if err := db.Where("id = ? AND deleted_at IS NULL", id).First(&a).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, constants.ErrAddonNotFound
		}
		return nil, fmt.Errorf("find addon by id: %w", err)
	}
	return &a, nil
}

// FindByIDs returns non-deleted add-ons for the given IDs in a single query.
// Used by the reorder flow to batch-validate tenant ownership inside the tx.
func (r *AddonRepository) FindByIDs(ctx context.Context, ids []string) ([]*model.Addon, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	db := dbFromContext(ctx, r.db)
	var rows []model.Addon
	if err := db.Where("id IN ? AND deleted_at IS NULL", ids).Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("find addons by ids: %w", err)
	}
	out := make([]*model.Addon, len(rows))
	for i := range rows {
		out[i] = &rows[i]
	}
	return out, nil
}

// FindByTenant returns an offset-paginated list of non-deleted add-ons for the
// given tenant, plus the total matching count.
// Results are ordered by sort_order ASC, created_at ASC, id ASC.
func (r *AddonRepository) FindByTenant(ctx context.Context, tenantID string, filter service.AddonFilter) ([]*model.Addon, int64, error) {
	db := dbFromContext(ctx, r.db)

	limit := filter.Limit
	if limit <= 0 || limit > 200 {
		limit = 10
	}
	page := filter.Page
	if page < 1 {
		page = 1
	}

	// Explicit tenant_id scope as defence-in-depth — RLS already enforces it.
	q := db.Model(&model.Addon{}).Where("tenant_id = ? AND deleted_at IS NULL", tenantID)

	if filter.IsActive != nil {
		q = q.Where("is_active = ?", *filter.IsActive)
	}

	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count addons by tenant: %w", err)
	}

	var rows []model.Addon
	if err := q.Order("sort_order ASC, created_at ASC, id ASC").
		Offset((page - 1) * limit).
		Limit(limit).
		Find(&rows).Error; err != nil {
		return nil, 0, fmt.Errorf("find addons by tenant: %w", err)
	}

	out := make([]*model.Addon, len(rows))
	for i := range rows {
		out[i] = &rows[i]
	}
	return out, total, nil
}

// Update writes the mutable columns of an existing add-on row.
func (r *AddonRepository) Update(ctx context.Context, a *model.Addon) error {
	db := dbFromContext(ctx, r.db)
	updates := map[string]interface{}{
		"name":        a.Name,
		"description": a.Description,
		"price_idr":   a.PriceIDR,
		"sort_order":  a.SortOrder,
		"updated_by":  a.UpdatedBy,
	}
	if err := db.Model(&model.Addon{}).
		Where("id = ? AND deleted_at IS NULL", a.ID).
		Updates(updates).Error; err != nil {
		return fmt.Errorf("update addon: %w", translateAddonDBError(err))
	}
	return nil
}

// UpdateStatus sets is_active for an add-on row.
func (r *AddonRepository) UpdateStatus(ctx context.Context, id string, isActive bool, updatedBy string) error {
	db := dbFromContext(ctx, r.db)
	result := db.Model(&model.Addon{}).
		Where("id = ? AND deleted_at IS NULL", id).
		Updates(map[string]interface{}{
			"is_active":  isActive,
			"updated_by": updatedBy,
		})
	if result.Error != nil {
		return fmt.Errorf("update addon status: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return constants.ErrAddonNotFound
	}
	return nil
}

// SoftDelete sets deleted_at and is_active=false on the add-on row.
// No hard DELETE is issued — lustia_app has no DELETE grant on this table.
func (r *AddonRepository) SoftDelete(ctx context.Context, id string, updatedBy string) error {
	db := dbFromContext(ctx, r.db)
	now := time.Now().UTC()
	result := db.Model(&model.Addon{}).
		Where("id = ? AND deleted_at IS NULL", id).
		Updates(map[string]interface{}{
			"deleted_at": now,
			"is_active":  false,
			"updated_by": updatedBy,
		})
	if result.Error != nil {
		return fmt.Errorf("soft delete addon: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return constants.ErrAddonNotFound
	}
	return nil
}

// BulkUpdateSortOrder atomically updates sort_order for multiple add-ons using
// a single CASE expression UPDATE. All IDs must have been validated by the
// service layer (tenant check) before this call.
func (r *AddonRepository) BulkUpdateSortOrder(ctx context.Context, items []service.AddonSortOrderItem) error {
	if len(items) == 0 {
		return nil
	}

	db := dbFromContext(ctx, r.db)

	// Build:
	//   UPDATE addon
	//   SET sort_order = CASE id
	//     WHEN '<id1>' THEN <order1>
	//     WHEN '<id2>' THEN <order2>
	//     ...
	//   END
	//   WHERE id IN ('<id1>', '<id2>', ...)
	//   AND deleted_at IS NULL
	//
	// We use ? placeholders to avoid SQL injection. GORM's Raw accepts a
	// variadic args slice so we build the args list alongside the SQL.

	var sb strings.Builder
	args := make([]interface{}, 0, len(items)*3)

	sb.WriteString("UPDATE addon SET sort_order = CASE id ")
	for _, it := range items {
		sb.WriteString("WHEN ? THEN ? ")
		args = append(args, it.ID, it.SortOrder)
	}
	sb.WriteString("END WHERE id IN ? AND deleted_at IS NULL")

	ids := make([]string, len(items))
	for i, it := range items {
		ids[i] = it.ID
	}
	args = append(args, ids)

	if err := db.Exec(sb.String(), args...).Error; err != nil {
		return fmt.Errorf("bulk update sort order: %w", err)
	}
	return nil
}

// translateAddonDBError maps PostgreSQL unique-violation errors on the addon
// table to the appropriate sentinel. Falls back to the generic translator for
// other constraint violations.
func translateAddonDBError(err error) error {
	if err == nil {
		return nil
	}
	translated := translateDBError(err)
	// The partial unique index is addon_tenant_name_uidx (tenant_id, name).
	// translateDBError maps 23505 generically to ErrConflict; we narrow it to
	// ErrDuplicateAddonName when the constraint name matches.
	if errors.Is(translated, constants.ErrConflict) {
		if strings.Contains(err.Error(), "addon_tenant_name_uidx") {
			return constants.ErrDuplicateAddonName
		}
	}
	return translated
}
