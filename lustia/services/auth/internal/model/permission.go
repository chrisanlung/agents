package model

import "time"

// Permission maps to the permission table.
type Permission struct {
	ID          string    `gorm:"column:id;primaryKey;type:uuid"`
	Code        string    `gorm:"column:code;not null;uniqueIndex"`
	Description *string   `gorm:"column:description"`
	CreatedAt   time.Time `gorm:"column:created_at;not null;autoCreateTime"`
	UpdatedAt   time.Time `gorm:"column:updated_at;not null;autoUpdateTime"`
}

func (Permission) TableName() string { return "permission" }

// RolePermission maps to the role_permission junction table.
type RolePermission struct {
	RoleID       string     `gorm:"column:role_id;primaryKey;type:uuid"`
	PermissionID string     `gorm:"column:permission_id;primaryKey;type:uuid"`
	CreatedAt    time.Time  `gorm:"column:created_at;not null;autoCreateTime"`
	Permission   Permission `gorm:"foreignKey:PermissionID;references:ID"`
}

func (RolePermission) TableName() string { return "role_permission" }
