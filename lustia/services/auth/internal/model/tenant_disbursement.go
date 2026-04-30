package model

import "time"

// DisbursementStatus mirrors the status CHECK on tenant_disbursement.
type DisbursementStatus = string

const (
	DisbursementStatusPending     DisbursementStatus = "pending"
	DisbursementStatusProcessing  DisbursementStatus = "processing"
	DisbursementStatusTransferred DisbursementStatus = "transferred"
	DisbursementStatusFailed      DisbursementStatus = "failed"
	DisbursementStatusCancelled   DisbursementStatus = "cancelled"
)

// TenantDisbursement maps to the tenant_disbursement table (migration 000029).
// Per-tenant weekly payout record. Phase 6: manual bank transfer. See ADR 0015 §2.5.
//
// Status machine:
//
//	pending → processing → transferred
//	pending → cancelled
//	processing → failed → processing (retry)
type TenantDisbursement struct {
	ID       string `gorm:"column:id;primaryKey;type:uuid"`
	TenantID string `gorm:"column:tenant_id;not null;type:uuid"`

	// Disbursement period (inclusive on both ends).
	PeriodStart time.Time `gorm:"column:period_start;not null;type:date"`
	PeriodEnd   time.Time `gorm:"column:period_end;not null;type:date"`

	// Aggregate amounts.
	GrossAmountIDR   int64 `gorm:"column:gross_amount_idr;not null"`
	PlatformFeeIDR   int64 `gorm:"column:platform_fee_idr;not null"`
	NetAmountIDR     int64 `gorm:"column:net_amount_idr;not null"` // gross - fee
	TransactionCount int   `gorm:"column:transaction_count;not null"`

	// Lifecycle.
	Status DisbursementStatus `gorm:"column:status;not null;default:pending"`

	// Transfer metadata.
	BankReference *string    `gorm:"column:bank_reference"`
	Notes         *string    `gorm:"column:notes"`
	TransferredAt *time.Time `gorm:"column:transferred_at"`
	TransferredBy *string    `gorm:"column:transferred_by;type:uuid"` // SET NULL on user delete

	CreatedAt time.Time `gorm:"column:created_at;not null;autoCreateTime"`
	UpdatedAt time.Time `gorm:"column:updated_at;not null;autoUpdateTime"`
}

// TableName returns the Postgres table name.
func (TenantDisbursement) TableName() string { return "tenant_disbursement" }
