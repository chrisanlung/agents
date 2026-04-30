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

// PaymentTransactionRepository implements service.PaymentTransactionRepository
// using GORM. All methods use dbFromContext so they participate in the
// per-request transaction opened by the tenant middleware.
type PaymentTransactionRepository struct {
	db *gorm.DB
}

// NewPaymentTransactionRepository constructs a PaymentTransactionRepository.
func NewPaymentTransactionRepository(db *gorm.DB) *PaymentTransactionRepository {
	return &PaymentTransactionRepository{db: db}
}

// Save inserts a new payment_transaction row. The UNIQUE (booking_id) and
// UNIQUE (provider_reference) constraints fire here; errors are translated
// to constants.ErrConflict at the boundary.
func (r *PaymentTransactionRepository) Save(ctx context.Context, txn *model.PaymentTransaction) error {
	db := dbFromContext(ctx, r.db)
	if err := db.Create(txn).Error; err != nil {
		return fmt.Errorf("save payment_transaction: %w", translatePaymentTxnDBError(err))
	}
	return nil
}

// Reset performs an UPDATE on the existing row for bookingID — used when the
// QR code expires and the customer requests a new one (flag #2).
// The UNIQUE(booking_id) invariant means we can never INSERT a second row for
// the same booking; this method updates the existing row in place.
func (r *PaymentTransactionRepository) Reset(
	ctx context.Context,
	bookingID, newReference, newQRString, newQRImageURL string,
	newExpiresAt time.Time,
) error {
	db := dbFromContext(ctx, r.db)
	result := db.Model(&model.PaymentTransaction{}).
		Where("booking_id = ? AND status = 'awaiting'", bookingID).
		Updates(map[string]interface{}{
			"provider_reference": newReference,
			"qr_string":          newQRString,
			"qr_image_url":       newQRImageURL,
			"qr_expires_at":      newExpiresAt,
			"updated_at":         time.Now(),
		})
	if result.Error != nil {
		return fmt.Errorf("reset payment_transaction: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return constants.ErrPaymentTransactionNotFound
	}
	return nil
}

// FindByID returns a payment_transaction by primary key.
func (r *PaymentTransactionRepository) FindByID(ctx context.Context, id string) (*model.PaymentTransaction, error) {
	db := dbFromContext(ctx, r.db)
	var txn model.PaymentTransaction
	if err := db.Where("id = ?", id).First(&txn).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, constants.ErrPaymentTransactionNotFound
		}
		return nil, fmt.Errorf("find payment_transaction by id: %w", err)
	}
	return &txn, nil
}

// FindByBookingID returns the payment_transaction for the given booking.
func (r *PaymentTransactionRepository) FindByBookingID(ctx context.Context, bookingID string) (*model.PaymentTransaction, error) {
	db := dbFromContext(ctx, r.db)
	var txn model.PaymentTransaction
	if err := db.Where("booking_id = ?", bookingID).First(&txn).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, constants.ErrPaymentTransactionNotFound
		}
		return nil, fmt.Errorf("find payment_transaction by booking_id: %w", err)
	}
	return &txn, nil
}

// FindByProviderReference returns the payment_transaction for the given
// provider_reference (idempotency key for webhooks).
func (r *PaymentTransactionRepository) FindByProviderReference(ctx context.Context, providerRef string) (*model.PaymentTransaction, error) {
	db := dbFromContext(ctx, r.db)
	var txn model.PaymentTransaction
	if err := db.Where("provider_reference = ?", providerRef).First(&txn).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, constants.ErrPaymentTransactionNotFound
		}
		return nil, fmt.Errorf("find payment_transaction by provider_reference: %w", err)
	}
	return &txn, nil
}

// FindByTenant returns a paginated list of payment_transactions for a tenant.
func (r *PaymentTransactionRepository) FindByTenant(
	ctx context.Context,
	tenantID string,
	filter service.PaymentTxnFilter,
) ([]*model.PaymentTransaction, int64, error) {
	db := dbFromContext(ctx, r.db)
	q := db.Model(&model.PaymentTransaction{}).Where("tenant_id = ?", tenantID)

	if filter.Status != nil && *filter.Status != "" {
		q = q.Where("status = ?", *filter.Status)
	}
	if filter.FromDate != nil && *filter.FromDate != "" {
		q = q.Where("created_at >= ?", *filter.FromDate)
	}
	if filter.ToDate != nil && *filter.ToDate != "" {
		q = q.Where("created_at <= ?", *filter.ToDate)
	}

	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count payment_transactions: %w", err)
	}

	page, limit := normalisePage(filter.Page, filter.Limit)
	var rows []*model.PaymentTransaction
	if err := q.Order("created_at DESC").
		Offset((page - 1) * limit).Limit(limit).
		Find(&rows).Error; err != nil {
		return nil, 0, fmt.Errorf("find payment_transactions: %w", err)
	}
	return rows, total, nil
}

// MarkPaid atomically updates status awaiting→paid.
// Returns rowsAffected so callers can detect duplicate webhooks (idempotency).
// H-5: WHERE status='awaiting' prevents double-credit.
func (r *PaymentTransactionRepository) MarkPaid(
	ctx context.Context,
	providerRef string,
	receivedAmount int64,
	paidAt time.Time,
	rawWebhook []byte,
) (int64, error) {
	db := dbFromContext(ctx, r.db)
	result := db.Model(&model.PaymentTransaction{}).
		Where("provider_reference = ? AND status = 'awaiting'", providerRef).
		Updates(map[string]interface{}{
			"status":               model.PaymentTxnStatusPaid,
			"received_amount_idr":  receivedAmount,
			"paid_at":              paidAt,
			"raw_webhook":          rawWebhook,
			"updated_at":           time.Now(),
		})
	if result.Error != nil {
		return 0, fmt.Errorf("mark payment_transaction paid: %w", result.Error)
	}
	return result.RowsAffected, nil
}

// BulkMarkSettled updates matching rows to status=settled within the given
// settlement batch. Returns the count of rows updated.
func (r *PaymentTransactionRepository) BulkMarkSettled(
	ctx context.Context,
	batchID string,
	providerRefs []string,
	settledAt time.Time,
) (int, error) {
	if len(providerRefs) == 0 {
		return 0, nil
	}
	db := dbFromContext(ctx, r.db)
	result := db.Model(&model.PaymentTransaction{}).
		Where("provider_reference IN ? AND status = 'paid'", providerRefs).
		Updates(map[string]interface{}{
			"status":              model.PaymentTxnStatusSettled,
			"settlement_batch_id": batchID,
			"settled_at":          settledAt,
			"updated_at":          time.Now(),
		})
	if result.Error != nil {
		return 0, fmt.Errorf("bulk mark payment_transactions settled: %w", result.Error)
	}
	return int(result.RowsAffected), nil
}

// BulkMarkDisbursed updates matching rows to status=disbursed within the
// given disbursement. Must run inside the same transaction as the disbursement
// status update per flag #4.
func (r *PaymentTransactionRepository) BulkMarkDisbursed(
	ctx context.Context,
	disbursementID string,
	txnIDs []string,
	disbursedAt time.Time,
) (int, error) {
	if len(txnIDs) == 0 {
		return 0, nil
	}
	db := dbFromContext(ctx, r.db)
	result := db.Model(&model.PaymentTransaction{}).
		Where("id IN ? AND status = 'settled'", txnIDs).
		Updates(map[string]interface{}{
			"status":          model.PaymentTxnStatusDisbursed,
			"disbursement_id": disbursementID,
			"disbursed_at":    disbursedAt,
			"updated_at":      time.Now(),
		})
	if result.Error != nil {
		return 0, fmt.Errorf("bulk mark payment_transactions disbursed: %w", result.Error)
	}
	return int(result.RowsAffected), nil
}

// SumByTenantStatus returns the sum of received_amount_idr for a given
// (tenantID, status) pair. Used for the balance card.
func (r *PaymentTransactionRepository) SumByTenantStatus(ctx context.Context, tenantID string, status string) (int64, error) {
	db := dbFromContext(ctx, r.db)
	var sum int64
	err := db.Model(&model.PaymentTransaction{}).
		Select("COALESCE(SUM(received_amount_idr), 0)").
		Where("tenant_id = ? AND status = ?", tenantID, status).
		Scan(&sum).Error
	if err != nil {
		return 0, fmt.Errorf("sum payment_transactions by tenant+status: %w", err)
	}
	return sum, nil
}

// SweepExpiredTransactions transitions awaiting rows past their qr_expires_at
// to status=expired. Called alongside booking expiry sweep.
func (r *PaymentTransactionRepository) SweepExpiredTransactions(ctx context.Context) (int, error) {
	db := dbFromContext(ctx, r.db)
	result := db.Model(&model.PaymentTransaction{}).
		Where("status = 'awaiting' AND qr_expires_at < NOW()").
		Updates(map[string]interface{}{
			"status":     model.PaymentTxnStatusExpired,
			"updated_at": time.Now(),
		})
	if result.Error != nil {
		return 0, fmt.Errorf("sweep expired payment_transactions: %w", result.Error)
	}
	return int(result.RowsAffected), nil
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// translatePaymentTxnDBError translates GORM/Postgres errors to sentinels.
func translatePaymentTxnDBError(err error) error {
	return translateDBError(err)
}

// normalisePage ensures page >= 1 and limit is between 1 and 100.
func normalisePage(page, limit int) (int, int) {
	if page <= 0 {
		page = 1
	}
	if limit <= 0 {
		limit = 10
	}
	if limit > 100 {
		limit = 100
	}
	return page, limit
}
