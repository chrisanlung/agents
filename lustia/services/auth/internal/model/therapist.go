package model

import "time"

// Therapist maps to the therapist table (migration 000013 added branch_id).
// A therapist is scoped to exactly one branch within a tenant.
type Therapist struct {
	ID          string     `gorm:"column:id;primaryKey;type:uuid"`
	TenantID    string     `gorm:"column:tenant_id;not null;type:uuid"`
	BranchID    string     `gorm:"column:branch_id;not null;type:uuid"`
	UserID      *string    `gorm:"column:user_id;type:uuid"`
	FullName    string     `gorm:"column:full_name;not null"`
	Gender      *string    `gorm:"column:gender"`
	Phone       *string    `gorm:"column:phone"`
	Email       *string    `gorm:"column:email"`
	Bio         *string    `gorm:"column:bio"`
	PhotoURL    *string    `gorm:"column:photo_url"`
	Specialties string     `gorm:"column:specialties;type:jsonb;not null;default:'[]'"`
	IsActive    bool       `gorm:"column:is_active;not null;default:true"`
	JoinedAt    *time.Time `gorm:"column:joined_at"`
	DeletedAt   *time.Time `gorm:"column:deleted_at"`
	CreatedAt   time.Time  `gorm:"column:created_at;not null;autoCreateTime"`
	UpdatedAt   time.Time  `gorm:"column:updated_at;not null;autoUpdateTime"`
	CreatedBy   *string    `gorm:"column:created_by;type:uuid"`
	UpdatedBy   *string    `gorm:"column:updated_by;type:uuid"`
}

// TableName returns the Postgres table name.
func (Therapist) TableName() string { return "therapist" }

// IsSoftDeleted returns true when the therapist row has been soft-deleted.
func (t Therapist) IsSoftDeleted() bool { return t.DeletedAt != nil }
