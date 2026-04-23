package model

import "time"

// TenantRegistrationStatus mirrors the Postgres tenant_registration_status enum
// introduced in migration 000011.
type TenantRegistrationStatus string

const (
	TenantRegistrationStatusPending  TenantRegistrationStatus = "pending"
	TenantRegistrationStatusApproved TenantRegistrationStatus = "approved"
	TenantRegistrationStatusRejected TenantRegistrationStatus = "rejected"
)

// TenantRegistration maps to the tenant_registration table (migration 000011).
// It represents a public company-registration submission pending platform review.
type TenantRegistration struct {
	ID            string                   `gorm:"column:id;primaryKey;type:uuid"`
	CompanyName   string                   `gorm:"column:company_name;not null"`
	RequestedSlug string                   `gorm:"column:requested_slug;not null"`
	Package       string                   `gorm:"column:package;not null;default:starter"`
	ContactName   string                   `gorm:"column:contact_name;not null"`
	ContactEmail  string                   `gorm:"column:contact_email;not null"`
	ContactPhone  *string                  `gorm:"column:contact_phone"`
	Status        TenantRegistrationStatus `gorm:"column:status;not null;default:pending"`
	Metadata      []byte                   `gorm:"column:metadata;type:jsonb"`

	// Set on approval.
	ApprovedTenantID *string    `gorm:"column:approved_tenant_id;type:uuid"`
	ApprovedUserID   *string    `gorm:"column:approved_user_id;type:uuid"`
	ApprovedAt       *time.Time `gorm:"column:approved_at"`
	ApprovedBy       *string    `gorm:"column:approved_by;type:uuid"`

	// Set on rejection.
	RejectedAt      *time.Time `gorm:"column:rejected_at"`
	RejectedBy      *string    `gorm:"column:rejected_by;type:uuid"`
	RejectionReason *string    `gorm:"column:rejection_reason"`

	CreatedAt time.Time `gorm:"column:created_at;not null;autoCreateTime"`
	UpdatedAt time.Time `gorm:"column:updated_at;not null;autoUpdateTime"`
}

func (TenantRegistration) TableName() string { return "tenant_registration" }

// IsPending returns true when the registration has not yet been reviewed.
func (r TenantRegistration) IsPending() bool {
	return r.Status == TenantRegistrationStatusPending
}

// IsApproved returns true when the registration was accepted.
func (r TenantRegistration) IsApproved() bool {
	return r.Status == TenantRegistrationStatusApproved
}

// IsRejected returns true when the registration was declined.
func (r TenantRegistration) IsRejected() bool {
	return r.Status == TenantRegistrationStatusRejected
}
