package model

import "time"

// TherapistService maps to the therapist_service join table.
// Physical schema (per migration 000003 + 000015): composite PK on
// (therapist_id, service_id), is_active soft-flag, created_at/_by. Migration
// 000017 adds updated_at/_by so reconcile can track deactivations.
// tenant_id is NOT a column here — tenant scoping comes from therapist.tenant_id
// via join (see RLS policies in migration 000004).
type TherapistService struct {
	TherapistID string    `gorm:"column:therapist_id;not null;type:uuid;primaryKey"`
	ServiceID   string    `gorm:"column:service_id;not null;type:uuid;primaryKey"`
	IsActive    bool      `gorm:"column:is_active;not null;default:true"`
	CreatedAt   time.Time `gorm:"column:created_at;not null;autoCreateTime"`
	UpdatedAt   time.Time `gorm:"column:updated_at;not null;autoUpdateTime"`
	CreatedBy   *string   `gorm:"column:created_by;type:uuid"`
	UpdatedBy   *string   `gorm:"column:updated_by;type:uuid"`
}

// TableName returns the Postgres table name.
func (TherapistService) TableName() string { return "therapist_service" }
