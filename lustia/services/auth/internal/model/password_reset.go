package model

import "time"

// PasswordReset maps to the password_reset table.
// Short-lived tokens for the forgot-password flow.
// Note: no RLS on this table — tokens are fetched by hash before tenant
// context is established. See SECURITY.md §2.6 for accepted rationale.
type PasswordReset struct {
	ID        string     `gorm:"column:id;primaryKey;type:uuid"`
	UserID    string     `gorm:"column:user_id;not null;type:uuid"`
	TokenHash string     `gorm:"column:token_hash;not null;uniqueIndex"`
	ExpiresAt time.Time  `gorm:"column:expires_at;not null"`
	UsedAt    *time.Time `gorm:"column:used_at"`
	CreatedAt time.Time  `gorm:"column:created_at;not null;autoCreateTime"`
}

func (PasswordReset) TableName() string { return "password_reset" }

// IsExpired returns true when the token has passed its expiry time.
func (p PasswordReset) IsExpired(now time.Time) bool { return p.ExpiresAt.Before(now) }

// IsUsed returns true when the token has already been consumed.
func (p PasswordReset) IsUsed() bool { return p.UsedAt != nil }
