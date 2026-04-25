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

// TenantRepository implements service.TenantRepository.
type TenantRepository struct {
	db *gorm.DB
}

// NewTenantRepository constructs a TenantRepository.
func NewTenantRepository(db *gorm.DB) *TenantRepository { return &TenantRepository{db: db} }

func (r *TenantRepository) FindBySlug(ctx context.Context, slug string) (*model.Tenant, error) {
	db := dbFromContext(ctx, r.db)
	var m model.Tenant
	if err := db.Where("slug = ? AND deleted_at IS NULL", slug).First(&m).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, constants.ErrTenantNotFound
		}
		return nil, fmt.Errorf("find tenant by slug: %w", err)
	}
	return &m, nil
}

func (r *TenantRepository) FindByID(ctx context.Context, id string) (*model.Tenant, error) {
	db := dbFromContext(ctx, r.db)
	var m model.Tenant
	if err := db.Where("id = ? AND deleted_at IS NULL", id).First(&m).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, constants.ErrTenantNotFound
		}
		return nil, fmt.Errorf("find tenant by id: %w", err)
	}
	return &m, nil
}

// Save inserts a new tenant row.
func (r *TenantRepository) Save(ctx context.Context, t *model.Tenant) error {
	db := dbFromContext(ctx, r.db)
	if err := db.Create(t).Error; err != nil {
		return fmt.Errorf("save tenant: %w", translateDBError(err))
	}
	return nil
}

// tenantListRow is a read model used for the list query with aggregate counts.
type tenantListRow struct {
	model.Tenant
	MembershipCount int `gorm:"column:membership_count"`
	BranchCount     int `gorm:"column:branch_count"`
}

// List returns an offset-paginated list of tenants with optional status filter,
// plus the total matching count.
// Counts are computed via SQL subqueries to avoid N+1 queries.
func (r *TenantRepository) List(ctx context.Context, filter service.TenantFilter) ([]*service.TenantWithCounts, int64, error) {
	db := dbFromContext(ctx, r.db)

	limit := filter.Limit
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	page := filter.Page
	if page < 1 {
		page = 1
	}

	// COUNT query uses a plain table scan with the same WHERE predicates.
	countQ := db.Table("tenant").Where("deleted_at IS NULL")
	if filter.Status != "" && filter.Status != "all" {
		countQ = countQ.Where("status = ?", filter.Status)
	}
	if filter.Package != "" {
		countQ = countQ.Where("package = ?", filter.Package)
	}
	if filter.Q != "" {
		// Trigram-indexed substring match on lower(name) OR lower(slug).
		// pg_trgm GIN indexes (migration 000023) make this O(log n).
		needle := "%" + strings.ToLower(filter.Q) + "%"
		countQ = countQ.Where("lower(name) LIKE ? OR lower(slug) LIKE ?", needle, needle)
	}
	var total int64
	if err := countQ.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count tenants: %w", err)
	}

	q := db.Table("tenant AS t").
		Select(`t.*,
			(SELECT COUNT(*) FROM membership m WHERE m.tenant_id = t.id) AS membership_count,
			(SELECT COUNT(*) FROM branch b WHERE b.tenant_id = t.id AND b.deleted_at IS NULL) AS branch_count`).
		Where("t.deleted_at IS NULL")

	if filter.Status != "" && filter.Status != "all" {
		q = q.Where("t.status = ?", filter.Status)
	}
	if filter.Package != "" {
		q = q.Where("t.package = ?", filter.Package)
	}
	if filter.Q != "" {
		needle := "%" + strings.ToLower(filter.Q) + "%"
		q = q.Where("lower(t.name) LIKE ? OR lower(t.slug) LIKE ?", needle, needle)
	}

	var rows []tenantListRow
	if err := q.Order("t.created_at DESC").
		Offset((page - 1) * limit).
		Limit(limit).
		Scan(&rows).Error; err != nil {
		return nil, 0, fmt.Errorf("list tenants: %w", err)
	}

	out := make([]*service.TenantWithCounts, len(rows))
	for i, row := range rows {
		tc := &service.TenantWithCounts{
			Tenant:          row.Tenant,
			MembershipCount: row.MembershipCount,
			BranchCount:     row.BranchCount,
		}
		out[i] = tc
	}
	return out, total, nil
}

// UpdateStatus writes a status transition, setting the appropriate timestamp
// and actor columns depending on the new status.
func (r *TenantRepository) UpdateStatus(ctx context.Context, id, newStatus, actorUserID string, reason *string) error {
	db := dbFromContext(ctx, r.db)
	now := time.Now().UTC()

	updates := map[string]interface{}{
		"status":     newStatus,
		"updated_at": now,
	}

	switch newStatus {
	case model.TenantStatusActive:
		updates["approved_at"] = now
		updates["approved_by"] = actorUserID
	case model.TenantStatusDeactivated:
		updates["rejected_at"] = now
		updates["rejected_by"] = actorUserID
		if reason != nil {
			updates["rejection_reason"] = *reason
		}
	}

	result := db.Model(&model.Tenant{}).Where("id = ?", id).Updates(updates)
	if result.Error != nil {
		return fmt.Errorf("update tenant status: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return constants.ErrTenantNotFound
	}
	return nil
}

// CountActiveBranches returns the number of non-deleted branches for a tenant.
func (r *TenantRepository) CountActiveBranches(ctx context.Context, tenantID string) (int, error) {
	db := dbFromContext(ctx, r.db)
	var count int64
	if err := db.Model(&model.Branch{}).
		Where("tenant_id = ? AND deleted_at IS NULL", tenantID).
		Count(&count).Error; err != nil {
		return 0, fmt.Errorf("count active branches: %w", err)
	}
	return int(count), nil
}
