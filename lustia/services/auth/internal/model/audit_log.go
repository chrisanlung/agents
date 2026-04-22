package model

import "time"

// AuditLog maps to the audit_log table (append-only, no updated_at).
type AuditLog struct {
	ID           string                 `gorm:"column:id;primaryKey;type:uuid"`
	TenantID     *string                `gorm:"column:tenant_id;type:uuid"`
	ActorUserID  *string                `gorm:"column:actor_user_id;type:uuid"`
	Action       string                 `gorm:"column:action;not null"`
	ResourceType string                 `gorm:"column:resource_type;not null"`
	ResourceID   *string                `gorm:"column:resource_id"`
	Meta         map[string]interface{} `gorm:"column:meta;serializer:json"`
	CreatedAt    time.Time              `gorm:"column:created_at;not null;autoCreateTime"`
}

func (AuditLog) TableName() string { return "audit_log" }
