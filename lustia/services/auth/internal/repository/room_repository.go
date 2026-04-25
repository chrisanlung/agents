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

// RoomRepository implements service.RoomRepository using GORM + PostgreSQL.
// The room table uses RLS (tenant_id = app.current_tenant), so all queries are
// already tenant-scoped at the DB layer; we keep explicit tenant_id predicates
// as a defence-in-depth measure (security M-1 from addon review).
type RoomRepository struct {
	db *gorm.DB
}

// NewRoomRepository constructs a RoomRepository.
func NewRoomRepository(db *gorm.DB) *RoomRepository {
	return &RoomRepository{db: db}
}

// Save inserts a new room row.
func (r *RoomRepository) Save(ctx context.Context, rm *model.Room) error {
	db := dbFromContext(ctx, r.db)
	if err := db.Create(rm).Error; err != nil {
		return fmt.Errorf("save room: %w", translateRoomDBError(err))
	}
	return nil
}

// FindByID returns a non-deleted room by primary key.
// Returns ErrRoomNotFound when no matching row exists.
func (r *RoomRepository) FindByID(ctx context.Context, id string) (*model.Room, error) {
	db := dbFromContext(ctx, r.db)
	var rm model.Room
	if err := db.Where("id = ? AND deleted_at IS NULL", id).First(&rm).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, constants.ErrRoomNotFound
		}
		return nil, fmt.Errorf("find room by id: %w", err)
	}
	return &rm, nil
}

// FindByIDs returns non-deleted rooms for the given IDs in a single query.
// Used by the reorder flow to batch-validate branch + tenant ownership inside the tx.
func (r *RoomRepository) FindByIDs(ctx context.Context, ids []string) ([]*model.Room, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	db := dbFromContext(ctx, r.db)
	var rows []model.Room
	if err := db.Where("id IN ? AND deleted_at IS NULL", ids).Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("find rooms by ids: %w", err)
	}
	out := make([]*model.Room, len(rows))
	for i := range rows {
		out[i] = &rows[i]
	}
	return out, nil
}

// FindByTenant returns an offset-paginated list of non-deleted rooms for the
// given tenant, applying optional filters, plus the total matching count.
// Results are ordered by sort_order ASC, created_at ASC, id ASC.
// Explicit tenant_id scope is kept as defence-in-depth on top of RLS (security M-1).
func (r *RoomRepository) FindByTenant(ctx context.Context, tenantID string, filter service.RoomFilter) ([]*model.Room, int64, error) {
	db := dbFromContext(ctx, r.db)

	limit := filter.Limit
	if limit <= 0 || limit > 200 {
		limit = 10
	}
	page := filter.Page
	if page < 1 {
		page = 1
	}

	q := db.Model(&model.Room{}).Where("tenant_id = ? AND deleted_at IS NULL", tenantID)

	if filter.BranchID != nil {
		q = q.Where("branch_id = ?", *filter.BranchID)
	}

	if filter.IsActive != nil {
		q = q.Where("is_active = ?", *filter.IsActive)
	}

	if filter.RoomType != nil {
		q = q.Where("room_type = ?", *filter.RoomType)
	}

	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count rooms by tenant: %w", err)
	}

	var rows []model.Room
	if err := q.Order("sort_order ASC, created_at ASC, id ASC").
		Offset((page - 1) * limit).
		Limit(limit).
		Find(&rows).Error; err != nil {
		return nil, 0, fmt.Errorf("find rooms by tenant: %w", err)
	}

	out := make([]*model.Room, len(rows))
	for i := range rows {
		out[i] = &rows[i]
	}
	return out, total, nil
}

// Update writes the mutable columns of an existing room row.
func (r *RoomRepository) Update(ctx context.Context, rm *model.Room) error {
	db := dbFromContext(ctx, r.db)
	updates := map[string]interface{}{
		"name":        rm.Name,
		"description": rm.Description,
		"room_type":   rm.RoomType,
		"capacity":    rm.Capacity,
		"amenities":   rm.Amenities,
		"is_active":   rm.IsActive,
		"sort_order":  rm.SortOrder,
		"updated_by":  rm.UpdatedBy,
	}
	if err := db.Model(&model.Room{}).
		Where("id = ? AND deleted_at IS NULL", rm.ID).
		Updates(updates).Error; err != nil {
		return fmt.Errorf("update room: %w", translateRoomDBError(err))
	}
	return nil
}

// UpdateStatus sets is_active for a room row.
func (r *RoomRepository) UpdateStatus(ctx context.Context, id string, isActive bool, updatedBy string) error {
	db := dbFromContext(ctx, r.db)
	result := db.Model(&model.Room{}).
		Where("id = ? AND deleted_at IS NULL", id).
		Updates(map[string]interface{}{
			"is_active":  isActive,
			"updated_by": updatedBy,
		})
	if result.Error != nil {
		return fmt.Errorf("update room status: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return constants.ErrRoomNotFound
	}
	return nil
}

// SoftDelete sets deleted_at and is_active=false on the room row.
// No hard DELETE is issued — lustia_app has no DELETE grant on this table.
func (r *RoomRepository) SoftDelete(ctx context.Context, id string, updatedBy string) error {
	db := dbFromContext(ctx, r.db)
	now := time.Now().UTC()
	result := db.Model(&model.Room{}).
		Where("id = ? AND deleted_at IS NULL", id).
		Updates(map[string]interface{}{
			"deleted_at": now,
			"is_active":  false,
			"updated_by": updatedBy,
		})
	if result.Error != nil {
		return fmt.Errorf("soft delete room: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return constants.ErrRoomNotFound
	}
	return nil
}

// BulkUpdateSortOrder atomically updates sort_order for multiple rooms using a
// single CASE expression UPDATE. All IDs must have been validated by the service
// layer (tenant + branch check) before this call.
func (r *RoomRepository) BulkUpdateSortOrder(ctx context.Context, items []service.RoomSortOrderItem) error {
	if len(items) == 0 {
		return nil
	}

	db := dbFromContext(ctx, r.db)

	// Build:
	//   UPDATE room
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

	sb.WriteString("UPDATE room SET sort_order = CASE id ")
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
		return fmt.Errorf("bulk update room sort order: %w", err)
	}
	return nil
}

// UpdatePhotoKey atomically swaps the photo_key for a room row within the
// caller's transaction context. Returns the old key so the caller can schedule
// a background delete after the transaction commits. A nil newKey clears the photo.
func (r *RoomRepository) UpdatePhotoKey(ctx context.Context, id string, newKey *string, updatedBy string) (oldKey *string, err error) {
	db := dbFromContext(ctx, r.db)

	var rm model.Room
	if err := db.Where("id = ? AND deleted_at IS NULL", id).First(&rm).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, constants.ErrRoomNotFound
		}
		return nil, fmt.Errorf("find room for photo update: %w", err)
	}

	prev := rm.PhotoKey

	if err := db.Model(&model.Room{}).
		Where("id = ? AND deleted_at IS NULL", id).
		Updates(map[string]interface{}{
			"photo_key":  newKey,
			"updated_by": updatedBy,
		}).Error; err != nil {
		return nil, fmt.Errorf("update room photo key: %w", err)
	}

	return prev, nil
}

// translateRoomDBError maps PostgreSQL unique-violation errors on the room
// table to the appropriate sentinel. Falls back to the generic translator for
// other constraint violations.
func translateRoomDBError(err error) error {
	if err == nil {
		return nil
	}
	translated := translateDBError(err)
	// The partial unique index is room_branch_name_uidx (branch_id, name).
	// translateDBError maps 23505 generically to ErrConflict; we narrow it to
	// ErrDuplicateRoomName when the constraint name matches.
	if errors.Is(translated, constants.ErrConflict) {
		if strings.Contains(err.Error(), "room_branch_name_uidx") {
			return constants.ErrDuplicateRoomName
		}
	}
	return translated
}
