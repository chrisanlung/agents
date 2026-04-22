package model

import "time"

// User maps to the "user" table (quoted because "user" is a PostgreSQL reserved
// word). After migration 000009, the user row is a global identity — tenant
// membership lives in the Membership table. TenantID is removed; IsSuperAdmin
// is added as an explicit column.
//
// Note: PasswordHash carries json:"-" to prevent it from appearing in any
// serialised representation. Tests should assert this — see SECURITY.md §7.3.
type User struct {
	ID               string     `gorm:"column:id;primaryKey;type:uuid"`
	Email            string     `gorm:"column:email;not null"`
	PasswordHash     string     `gorm:"column:password_hash;not null" json:"-"`
	FullName         string     `gorm:"column:full_name;not null"`
	Phone            *string    `gorm:"column:phone"`
	AvatarURL        *string    `gorm:"column:avatar_url"`
	IsActive           bool       `gorm:"column:is_active;not null;default:true"`
	IsSuperAdmin       bool       `gorm:"column:is_super_admin;not null;default:false"`
	LastLoginAt        *time.Time `gorm:"column:last_login_at"`
	FailedLoginCount   int        `gorm:"column:failed_login_count;not null;default:0"`
	LockedUntil        *time.Time `gorm:"column:locked_until"`
	MustChangePassword bool       `gorm:"column:must_change_password;not null;default:false"`
	DeletedAt        *time.Time `gorm:"column:deleted_at"`
	CreatedAt        time.Time  `gorm:"column:created_at;not null;autoCreateTime"`
	UpdatedAt        time.Time  `gorm:"column:updated_at;not null;autoUpdateTime"`

	// Memberships is loaded via Preload in repository methods.
	// Role and branch assignments live on the Membership, not the User.
	Memberships []Membership `gorm:"foreignKey:UserID"`
}

func (User) TableName() string { return `"user"` }

// IsLocked returns true when the account lockout expiry is in the future.
func (u User) IsLocked(now time.Time) bool {
	if u.LockedUntil == nil {
		return false
	}
	return u.LockedUntil.After(now)
}

// HasPermission checks whether the user holds the given permission code across
// all of their active memberships. Primarily used for admin checks where the
// membership has already been loaded.
func (u User) HasPermission(code string) bool {
	for _, m := range u.Memberships {
		for _, ur := range m.Roles {
			for _, rp := range ur.Role.Permissions {
				if rp.Permission.Code == code {
					return true
				}
			}
		}
	}
	return false
}

// UserRole maps to the user_role junction table. After migration 000009 the
// primary key is (membership_id, role_id) — user_id is gone.
type UserRole struct {
	MembershipID string    `gorm:"column:membership_id;primaryKey;type:uuid"`
	RoleID       string    `gorm:"column:role_id;primaryKey;type:uuid"`
	AssignedAt   time.Time `gorm:"column:assigned_at;not null;autoCreateTime"`
	AssignedBy   *string   `gorm:"column:assigned_by;type:uuid"`
	Role         Role      `gorm:"foreignKey:RoleID;references:ID"`
}

func (UserRole) TableName() string { return "user_role" }

// UserBranch maps to the user_branch junction table. After migration 000009 the
// primary key is (membership_id, branch_id) — user_id is gone.
type UserBranch struct {
	MembershipID string    `gorm:"column:membership_id;primaryKey;type:uuid"`
	BranchID     string    `gorm:"column:branch_id;primaryKey;type:uuid"`
	AssignedAt   time.Time `gorm:"column:assigned_at;not null;autoCreateTime"`
	AssignedBy   *string   `gorm:"column:assigned_by;type:uuid"`
}

func (UserBranch) TableName() string { return "user_branch" }
