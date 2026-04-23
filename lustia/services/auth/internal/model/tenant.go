package model

import "time"

// TenantStatus mirrors the Postgres tenant_status enum.
type TenantStatus = string

const (
	TenantStatusPendingApproval TenantStatus = "pending_approval"
	TenantStatusActive          TenantStatus = "active"
	TenantStatusSuspended       TenantStatus = "suspended"
	TenantStatusDeactivated     TenantStatus = "deactivated"
	// TenantStatusCancelled is kept for backward compatibility with existing rows.
	TenantStatusCancelled TenantStatus = "cancelled"
)

// TenantPackage mirrors the package CHECK constraint on the tenant table.
type TenantPackage = string

const (
	TenantPackageStarter    TenantPackage = "starter"
	TenantPackageGrowth     TenantPackage = "growth"
	TenantPackageEnterprise TenantPackage = "enterprise"

	// MaxBranchesUnlimited is the sentinel stored in max_branches for enterprise.
	MaxBranchesUnlimited = 999
)

// PackageMaxBranches is the canonical package → default max_branches matrix
// described in ADR 0008 §2.1.1. Using a function keeps it a pure lookup with
// no package-level mutable state.
func PackageMaxBranches(pkg TenantPackage) int {
	switch pkg {
	case TenantPackageGrowth:
		return 5
	case TenantPackageEnterprise:
		return MaxBranchesUnlimited
	default: // starter or unknown
		return 1
	}
}

// validTenantTransitions defines which target statuses each source status may
// reach. The approval transition (pending_approval → active) is handled by
// RegistrationService, but is included here so CanTransitionTo can be used
// for validation in both paths.
var validTenantTransitions = map[TenantStatus][]TenantStatus{
	TenantStatusPendingApproval: {TenantStatusActive, TenantStatusDeactivated},
	TenantStatusActive:          {TenantStatusSuspended, TenantStatusDeactivated},
	TenantStatusSuspended:       {TenantStatusActive, TenantStatusDeactivated},
	TenantStatusDeactivated:     {}, // terminal
	TenantStatusCancelled:       {TenantStatusDeactivated},
}

// Tenant maps to the tenant table (migration 000011 extends with package,
// max_branches, and approval / rejection metadata).
type Tenant struct {
	ID           string     `gorm:"column:id;primaryKey;type:uuid"`
	Name         string     `gorm:"column:name;not null"`
	Slug         string     `gorm:"column:slug;not null;uniqueIndex"`
	Status       string     `gorm:"column:status;not null"`
	Package      string     `gorm:"column:package;not null;default:starter"`
	MaxBranches  int        `gorm:"column:max_branches;not null;default:1"`
	ContactEmail *string    `gorm:"column:contact_email"`
	ContactName  *string    `gorm:"column:contact_name"`

	// Approval metadata (set on approval).
	ApprovedAt *time.Time `gorm:"column:approved_at"`
	ApprovedBy *string    `gorm:"column:approved_by;type:uuid"`

	// Rejection metadata (set on rejection — rare on the tenant row itself).
	RejectedAt      *time.Time `gorm:"column:rejected_at"`
	RejectedBy      *string    `gorm:"column:rejected_by;type:uuid"`
	RejectionReason *string    `gorm:"column:rejection_reason"`

	DeletedAt *time.Time `gorm:"column:deleted_at"`
	CreatedAt time.Time  `gorm:"column:created_at;not null;autoCreateTime"`
	UpdatedAt time.Time  `gorm:"column:updated_at;not null;autoUpdateTime"`
}

func (Tenant) TableName() string { return "tenant" }

// IsActive returns true when the tenant may authenticate users.
func (t Tenant) IsActive() bool { return t.Status == TenantStatusActive }

// IsApproved returns true when the tenant has an approval timestamp, meaning
// the onboarding approval step completed.
func (t Tenant) IsApproved() bool { return t.ApprovedAt != nil }

// CanTransitionTo returns true when moving from the tenant's current status to
// newStatus is permitted by the state machine in ADR 0008 §2.1.1.
func (t Tenant) CanTransitionTo(newStatus TenantStatus) bool {
	targets, ok := validTenantTransitions[t.Status]
	if !ok {
		return false
	}
	for _, s := range targets {
		if s == newStatus {
			return true
		}
	}
	return false
}
