package model

import "time"

// ServiceCatalog maps to the service table (migration 000013 added category).
// Named ServiceCatalog to avoid shadowing the stdlib "service" package name.
// A service is scoped to a tenant (branch_id is nullable; NULL = tenant-wide).
type ServiceCatalog struct {
	ID              string     `gorm:"column:id;primaryKey;type:uuid"`
	TenantID        string     `gorm:"column:tenant_id;not null;type:uuid"`
	BranchID        *string    `gorm:"column:branch_id;type:uuid"`
	Name            string     `gorm:"column:name;not null"`
	Description     *string    `gorm:"column:description"`
	Category        *string    `gorm:"column:category"`
	DurationMinutes int        `gorm:"column:duration_minutes;not null"`
	Code            string     `gorm:"column:code;not null"`
	PriceIDR        int64      `gorm:"column:price;not null;default:0"`
	Currency        string     `gorm:"column:currency;not null;default:IDR"`
	IsActive        bool       `gorm:"column:is_active;not null;default:true"`
	DeletedAt       *time.Time `gorm:"column:deleted_at"`
	CreatedAt       time.Time  `gorm:"column:created_at;not null;autoCreateTime"`
	UpdatedAt       time.Time  `gorm:"column:updated_at;not null;autoUpdateTime"`
	CreatedBy       *string    `gorm:"column:created_by;type:uuid"`
	UpdatedBy       *string    `gorm:"column:updated_by;type:uuid"`
}

// TableName returns the Postgres table name.
func (ServiceCatalog) TableName() string { return "service" }

// IsSoftDeleted returns true when the service has been soft-deleted.
func (s ServiceCatalog) IsSoftDeleted() bool { return s.DeletedAt != nil }
