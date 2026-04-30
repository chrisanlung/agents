package model

import "time"

// SettlementBatch maps to the settlement_batch table (migration 000029).
// Platform-level record of each provider (iPaymu) daily settlement report.
// No tenant_id — cross-tenant platform concern. See ADR 0015 §2.9.
type SettlementBatch struct {
	ID string `gorm:"column:id;primaryKey;type:uuid"`

	// Provider that originated this settlement.
	Provider string `gorm:"column:provider;not null"`

	// Timestamp from the provider's settlement report (not row created_at).
	SettledAt time.Time `gorm:"column:settled_at;not null"`

	// Aggregate totals.
	TotalAmountIDR   int64 `gorm:"column:total_amount_idr;not null"`
	TransactionCount int   `gorm:"column:transaction_count;not null"`

	// Raw provider settlement payload (audit + discrepancy investigation).
	RawPayload *[]byte `gorm:"column:raw_payload;type:jsonb"`

	// Platform audit.
	CreatedAt *time.Time `gorm:"column:created_at;autoCreateTime"`
	CreatedBy *string    `gorm:"column:created_by;type:uuid"` // platform admin; SET NULL on user delete
}

// TableName returns the Postgres table name.
func (SettlementBatch) TableName() string { return "settlement_batch" }
