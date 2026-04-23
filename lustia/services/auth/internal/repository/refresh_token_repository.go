package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/chrisanlung/lustia-auth/internal/constants"
	"github.com/chrisanlung/lustia-auth/internal/model"
	"gorm.io/gorm"
)

// RefreshTokenRepository implements service.RefreshTokenRepository.
type RefreshTokenRepository struct {
	db *gorm.DB
}

// NewRefreshTokenRepository constructs a RefreshTokenRepository.
func NewRefreshTokenRepository(db *gorm.DB) *RefreshTokenRepository {
	return &RefreshTokenRepository{db: db}
}

func (r *RefreshTokenRepository) FindByHash(ctx context.Context, hash string) (*model.RefreshToken, error) {
	db := dbFromContext(ctx, r.db)
	var m model.RefreshToken
	if err := db.Where("token_hash = ?", hash).First(&m).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, constants.ErrRefreshTokenNotFound
		}
		return nil, fmt.Errorf("find refresh token: %w", err)
	}
	return &m, nil
}

func (r *RefreshTokenRepository) Save(ctx context.Context, rt *model.RefreshToken) error {
	db := dbFromContext(ctx, r.db)
	if err := db.Create(rt).Error; err != nil {
		return fmt.Errorf("save refresh token: %w", translateDBError(err))
	}
	return nil
}

func (r *RefreshTokenRepository) Revoke(ctx context.Context, id string, replacedBy *string) error {
	db := dbFromContext(ctx, r.db)
	now := time.Now().UTC()
	updates := map[string]interface{}{
		"revoked_at":  now,
		"replaced_by": replacedBy,
	}
	if err := db.Model(&model.RefreshToken{}).Where("id = ?", id).Updates(updates).Error; err != nil {
		return fmt.Errorf("revoke refresh token: %w", err)
	}
	return nil
}

func (r *RefreshTokenRepository) RevokeAllForUser(ctx context.Context, userID string) error {
	db := dbFromContext(ctx, r.db)
	now := time.Now().UTC()
	if err := db.Model(&model.RefreshToken{}).
		Where("user_id = ? AND revoked_at IS NULL", userID).
		Update("revoked_at", now).Error; err != nil {
		return fmt.Errorf("revoke all refresh tokens for user: %w", err)
	}
	return nil
}

// RevokeAllForTenantUsers revokes all active refresh tokens that were scoped to
// the given tenant. Used during tenant deactivation to invalidate in-flight
// sessions for that tenant.
func (r *RefreshTokenRepository) RevokeAllForTenantUsers(ctx context.Context, tenantID string) error {
	db := dbFromContext(ctx, r.db)
	now := time.Now().UTC()
	if err := db.Model(&model.RefreshToken{}).
		Where("tenant_id = ? AND revoked_at IS NULL", tenantID).
		Update("revoked_at", now).Error; err != nil {
		return fmt.Errorf("revoke refresh tokens for tenant users: %w", err)
	}
	return nil
}

func (r *RefreshTokenRepository) DeleteExpiredAndRevoked(ctx context.Context) error {
	db := dbFromContext(ctx, r.db)
	cutoff := time.Now().UTC().Add(-7 * 24 * time.Hour)
	if err := db.Where(
		"expires_at < NOW() OR (revoked_at IS NOT NULL AND revoked_at < ?)", cutoff,
	).Delete(&model.RefreshToken{}).Error; err != nil {
		return fmt.Errorf("cleanup expired/revoked tokens: %w", err)
	}
	return nil
}
