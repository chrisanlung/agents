package model

import "time"

// Role maps to the role table.
type Role struct {
	ID          string               `gorm:"column:id;primaryKey;type:uuid"`
	Name        string               `gorm:"column:name;not null;uniqueIndex"`
	Description *string              `gorm:"column:description"`
	CreatedAt   time.Time            `gorm:"column:created_at;not null;autoCreateTime"`
	UpdatedAt   time.Time            `gorm:"column:updated_at;not null;autoUpdateTime"`
	Permissions []RolePermission     `gorm:"foreignKey:RoleID"`
}

func (Role) TableName() string { return "role" }
