package model

import "time"

// Therapist maps to the therapist table.
// Migration 000020 (ADR 0011) renamed photo_url → photo_key and added
// height_cm, weight_kg, build.
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
	PhotoKey    *string    `gorm:"column:photo_key"`
	HeightCm    int16      `gorm:"column:height_cm;not null"`
	WeightKg    int16      `gorm:"column:weight_kg;not null"`
	Build       string     `gorm:"column:build;not null"`
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
