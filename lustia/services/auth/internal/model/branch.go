package model

import "time"

// BranchStatus mirrors the Postgres branch_status enum.
type BranchStatus = string

const (
	BranchStatusActive   BranchStatus = "active"
	BranchStatusInactive BranchStatus = "inactive"
)

// validBranchTransitions defines which target statuses each source status may
// reach, matching the state machine in ADR 0008 §2.1.2.
// Soft-delete is handled separately via SoftDelete(), not via status.
var validBranchTransitions = map[BranchStatus][]BranchStatus{
	BranchStatusInactive: {BranchStatusActive},
	BranchStatusActive:   {BranchStatusInactive},
}

// Branch maps to the branch table (migration 000011 extends with address,
// timezone, contact, and activated_at columns).
type Branch struct {
	ID               string     `gorm:"column:id;primaryKey;type:uuid"`
	TenantID         string     `gorm:"column:tenant_id;not null;type:uuid"`
	Name             string     `gorm:"column:name;not null"`
	Code             string     `gorm:"column:code;not null"`
	Status           string     `gorm:"column:status;not null;default:inactive"`
	OperationalHours []byte     `gorm:"column:operational_hours;type:jsonb;default:'{}'"`

	// Address fields (added in migration 000011).
	AddressLine1 *string `gorm:"column:address_line1"`
	AddressLine2 *string `gorm:"column:address_line2"`
	City         *string `gorm:"column:city"`
	Province     *string `gorm:"column:province"`
	PostalCode   *string `gorm:"column:postal_code"`
	Country      string  `gorm:"column:country_code;default:ID"`
	Timezone     string  `gorm:"column:timezone;not null;default:Asia/Jakarta"`

	// Contact fields (added in migration 000011).
	ContactPhone *string `gorm:"column:contact_phone"`
	ContactEmail *string `gorm:"column:contact_email"`

	// Lifecycle timestamp (added in migration 000011).
	ActivatedAt *time.Time `gorm:"column:activated_at"`

	DeletedAt *time.Time `gorm:"column:deleted_at"`
	CreatedAt time.Time  `gorm:"column:created_at;not null;autoCreateTime"`
	UpdatedAt time.Time  `gorm:"column:updated_at;not null;autoUpdateTime"`
	CreatedBy *string    `gorm:"column:created_by;type:uuid"`
	UpdatedBy *string    `gorm:"column:updated_by;type:uuid"`
}

func (Branch) TableName() string { return "branch" }

// IsActive returns true when the branch status is active.
func (b Branch) IsActive() bool { return b.Status == BranchStatusActive }

// CanTransitionTo returns true when moving from the branch's current status to
// newStatus is permitted by the state machine in ADR 0008 §2.1.2.
func (b Branch) CanTransitionTo(newStatus BranchStatus) bool {
	targets, ok := validBranchTransitions[b.Status]
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
