package model

import "time"

// Addon maps to the addon table (migration 000018).
// Tenant-wide optional paid extras that a customer can select with any service
// during a booking. Scoped to tenant — no per-service mapping. See ADR 0010.
type Addon struct {
	ID          string     `gorm:"column:id;primaryKey;type:uuid"`
	TenantID    string     `gorm:"column:tenant_id;not null;type:uuid"`
	Name        string     `gorm:"column:name;not null"`
	Description *string    `gorm:"column:description"`
	PriceIDR    int64      `gorm:"column:price_idr;not null;default:0"`
	IsActive    bool       `gorm:"column:is_active;not null;default:true"`
	SortOrder   int        `gorm:"column:sort_order;not null;default:0"`
	CreatedAt   time.Time  `gorm:"column:created_at;not null;autoCreateTime"`
	UpdatedAt   time.Time  `gorm:"column:updated_at;not null;autoUpdateTime"`
	CreatedBy   *string    `gorm:"column:created_by;type:uuid"`
	UpdatedBy   *string    `gorm:"column:updated_by;type:uuid"`
	DeletedAt   *time.Time `gorm:"column:deleted_at"`
}

// TableName returns the Postgres table name.
func (Addon) TableName() string { return "addon" }

// IsSoftDeleted returns true when the add-on has been soft-deleted.
func (a Addon) IsSoftDeleted() bool { return a.DeletedAt != nil }
