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

// settlementSummaryRow is the scan target for the Summary aggregate query.
type settlementSummaryRow struct {
	BatchCount      int64
	VolumeIDR       int64
	PlatformFeeIDR  int64
	PayoutIDR       int64
}

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

// Summary returns aggregate KPIs by scanning payment_transaction rows whose
// settled_at falls within [from, to] (UTC) and whose status is 'settled' or
// 'disbursed'. The batch_count is the number of distinct settlement_batch_id
// values in that set (not a row count on settlement_batch itself).
//
// Uses dbFromContext so it participates in any active transaction, consistent
// with the rest of the repository pattern — even though no transaction is
// expected on this read path.
func (r *SettlementBatchRepository) Summary(ctx context.Context, from, to time.Time) (count int64, volume, platformFee, payout int64, err error) {
	db := dbFromContext(ctx, r.db)

	var row settlementSummaryRow
	if err = db.Table("payment_transaction").
		Select(
			"COUNT(DISTINCT settlement_batch_id) AS batch_count, "+
				"COALESCE(SUM(received_amount_idr), 0) AS volume_idr, "+
				"COALESCE(SUM(platform_fee_idr), 0) AS platform_fee_idr, "+
				"COALESCE(SUM(tenant_net_idr), 0) AS payout_idr",
		).
		Where(
			"status IN (?, ?) AND settled_at >= ? AND settled_at <= ?",
			"settled", "disbursed", from, to,
		).
		Scan(&row).Error; err != nil {
		return 0, 0, 0, 0, fmt.Errorf("settlement summary query: %w", err)
	}
	return row.BatchCount, row.VolumeIDR, row.PlatformFeeIDR, row.PayoutIDR, nil
}
