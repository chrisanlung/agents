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

// SettlementBatchRepository implements service.SettlementBatchRepository using
// GORM. settlement_batch is a platform-only table; callers must SET LOCAL
// app.current_tenant = '__platform__' before calling Save (flag #3).
type SettlementBatchRepository struct {
	db *gorm.DB
}

// NewSettlementBatchRepository constructs a SettlementBatchRepository.
func NewSettlementBatchRepository(db *gorm.DB) *SettlementBatchRepository {
	return &SettlementBatchRepository{db: db}
}

// Save inserts a new settlement_batch row.
// The caller (SettlementService.Reconcile) is responsible for switching
// app.current_tenant to '__platform__' before this call per flag #3.
func (r *SettlementBatchRepository) Save(ctx context.Context, batch *model.SettlementBatch) error {
	db := dbFromContext(ctx, r.db)
	if err := db.Create(batch).Error; err != nil {
		return fmt.Errorf("save settlement_batch: %w", translateDBError(err))
	}
	return nil
}

// FindByID returns a settlement_batch by primary key.
func (r *SettlementBatchRepository) FindByID(ctx context.Context, id string) (*model.SettlementBatch, error) {
	db := dbFromContext(ctx, r.db)
	var batch model.SettlementBatch
	if err := db.Where("id = ?", id).First(&batch).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, constants.ErrSettlementBatchNotFound
		}
		return nil, fmt.Errorf("find settlement_batch by id: %w", err)
	}
	return &batch, nil
}

// FindByDate returns all batches whose settled_at falls on the given calendar date.
func (r *SettlementBatchRepository) FindByDate(ctx context.Context, date time.Time) ([]*model.SettlementBatch, error) {
	db := dbFromContext(ctx, r.db)
	dateStr := date.Format("2006-01-02")
	var batches []*model.SettlementBatch
	if err := db.Where("DATE(settled_at) = ?", dateStr).
		Order("settled_at DESC").
		Find(&batches).Error; err != nil {
		return nil, fmt.Errorf("find settlement_batches by date: %w", err)
	}
	return batches, nil
}

// List returns a paginated list of settlement batches (newest first).
func (r *SettlementBatchRepository) List(ctx context.Context, filter service.SettlementBatchFilter) ([]*model.SettlementBatch, int64, error) {
	db := dbFromContext(ctx, r.db)
	q := db.Model(&model.SettlementBatch{})

	if filter.Provider != nil && *filter.Provider != "" {
		q = q.Where("provider = ?", *filter.Provider)
	}

	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count settlement_batches: %w", err)
	}

	page, limit := normalisePage(filter.Page, filter.Limit)
	var rows []*model.SettlementBatch
	if err := q.Order("settled_at DESC").
		Offset((page - 1) * limit).Limit(limit).
		Find(&rows).Error; err != nil {
		return nil, 0, fmt.Errorf("list settlement_batches: %w", err)
	}
	return rows, total, nil
}
