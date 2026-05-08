package model

import "time"

// MembershipStatus mirrors the membership_status PostgreSQL enum from
// migration 000009.
type MembershipStatus string

const (
	MembershipStatusActive    MembershipStatus = "active"
	MembershipStatusSuspended MembershipStatus = "suspended"
	MembershipStatusInvited   MembershipStatus = "invited"
	MembershipStatusLeft      MembershipStatus = "left"
)

// Membership maps to the "membership" table introduced in migration 000009.
// It represents one user's relationship to one tenant, together with the
// role and branch assignments that belong to that relationship.
type Membership struct {
	ID         string           `gorm:"column:id;primaryKey;type:uuid"`
	UserID     string           `gorm:"column:user_id;not null;type:uuid"`
	TenantID   string           `gorm:"column:tenant_id;not null;type:uuid"`
	Status     MembershipStatus `gorm:"column:status;not null;default:active"`
	InvitedAt  *time.Time       `gorm:"column:invited_at"`
	JoinedAt   time.Time        `gorm:"column:joined_at;not null"`
	LeftAt     *time.Time       `gorm:"column:left_at"`
	Metadata   []byte           `gorm:"column:metadata;type:jsonb"`
	CreatedAt  time.Time        `gorm:"column:created_at;not null;autoCreateTime"`
	UpdatedAt  time.Time        `gorm:"column:updated_at;not null;autoUpdateTime"`
	CreatedBy  *string          `gorm:"column:created_by;type:uuid"`
	UpdatedBy  *string          `gorm:"column:updated_by;type:uuid"`

	// Associations loaded via Preload in repository methods.
	Roles    []UserRole  `gorm:"foreignKey:MembershipID"`
	Branches []UserBranch `gorm:"foreignKey:MembershipID"`
	Tenant   Tenant      `gorm:"foreignKey:TenantID;references:ID"`
}

func (Membership) TableName() string { return "membership" }

// MembershipAssignments carries the role and branch IDs/names for one membership.
// Declared in model/ so it can be imported by both repository/ and service/
// without creating an import cycle.
type MembershipAssignments struct {
	RoleIDs     []string
	RoleNames   []string
	BranchIDs   []string
	BranchNames []string
}

// IsActive returns true when the membership status is active.
func (m Membership) IsActive() bool { return m.Status == MembershipStatusActive }

// RoleNames returns the list of role name strings from pre-loaded Roles.
func (m Membership) RoleNames() []string {
	names := make([]string, 0, len(m.Roles))
	for _, ur := range m.Roles {
		names = append(names, ur.Role.Name)
	}
	return names
}

// PermissionCodes returns a deduplicated slice of all permission code strings
// the membership holds across all its roles.
func (m Membership) PermissionCodes() []string {
	seen := make(map[string]struct{})
	var codes []string
	for _, ur := range m.Roles {
		for _, rp := range ur.Role.Permissions {
			if _, ok := seen[rp.Permission.Code]; !ok {
				seen[rp.Permission.Code] = struct{}{}
				codes = append(codes, rp.Permission.Code)
			}
		}
	}
	return codes
}

// BranchIDs returns the list of branch ID strings from pre-loaded Branches.
func (m Membership) BranchIDs() []string {
	ids := make([]string, 0, len(m.Branches))
	for _, ub := range m.Branches {
		ids = append(ids, ub.BranchID)
	}
	return ids
}
