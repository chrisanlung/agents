package model

import "time"

// RefreshToken maps to the refresh_token table (append-only).
// The raw bearer value is never stored — only its SHA-256 hex hash.
type RefreshToken struct {
	ID         string     `gorm:"column:id;primaryKey;type:uuid"`
	UserID     string     `gorm:"column:user_id;not null;type:uuid"`
	TenantID   *string    `gorm:"column:tenant_id;type:uuid"` // denormalised from "user".tenant_id (migration 000008); NULL for super admin
	TokenHash  string     `gorm:"column:token_hash;not null;uniqueIndex"`
	IssuedAt   time.Time  `gorm:"column:issued_at;not null;autoCreateTime"`
	ExpiresAt  time.Time  `gorm:"column:expires_at;not null"`
	RevokedAt  *time.Time `gorm:"column:revoked_at"`
	ReplacedBy *string    `gorm:"column:replaced_by;type:uuid"`
	UserAgent  *string    `gorm:"column:user_agent"`
	IP         *string    `gorm:"column:ip"`
	CreatedAt  time.Time  `gorm:"column:created_at;not null;autoCreateTime"`
}

func (RefreshToken) TableName() string { return "refresh_token" }

// IsRevoked returns true if the token has been explicitly revoked.
func (rt RefreshToken) IsRevoked() bool { return rt.RevokedAt != nil }

// IsExpired returns true if the token is past its expiry time.
func (rt RefreshToken) IsExpired(now time.Time) bool { return rt.ExpiresAt.Before(now) }
