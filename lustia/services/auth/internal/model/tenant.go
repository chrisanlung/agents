package model

import "time"

// TenantStatus mirrors the Postgres tenant_status enum.
type TenantStatus = string

const (
	TenantStatusPendingApproval TenantStatus = "pending_approval"
	TenantStatusActive          TenantStatus = "active"
	TenantStatusSuspended       TenantStatus = "suspended"
	TenantStatusCancelled       TenantStatus = "cancelled"
)

// Tenant maps to the tenant table.
type Tenant struct {
	ID           string     `gorm:"column:id;primaryKey;type:uuid"`
	Name         string     `gorm:"column:name;not null"`
	Slug         string     `gorm:"column:slug;not null;uniqueIndex"`
	Status       string     `gorm:"column:status;not null"`
	ContactEmail *string    `gorm:"column:contact_email"`
	DeletedAt    *time.Time `gorm:"column:deleted_at"`
	CreatedAt    time.Time  `gorm:"column:created_at;not null;autoCreateTime"`
	UpdatedAt    time.Time  `gorm:"column:updated_at;not null;autoUpdateTime"`
}

func (Tenant) TableName() string { return "tenant" }

// IsActive returns true when the tenant may authenticate users.
func (t Tenant) IsActive() bool { return t.Status == TenantStatusActive }
