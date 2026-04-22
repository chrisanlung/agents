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

// PasswordResetRepository implements service.PasswordResetRepository.
// Note: this table has no RLS — tokens are looked up by hash before the
// tenant context is established. See SECURITY.md §2.6 for accepted rationale.
type PasswordResetRepository struct {
	db *gorm.DB
}

// NewPasswordResetRepository constructs a PasswordResetRepository.
func NewPasswordResetRepository(db *gorm.DB) *PasswordResetRepository {
	return &PasswordResetRepository{db: db}
}

func (r *PasswordResetRepository) FindByHash(ctx context.Context, hash string) (*model.PasswordReset, error) {
	db := dbFromContext(ctx, r.db)
	var m model.PasswordReset
	if err := db.Where("token_hash = ?", hash).First(&m).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, constants.ErrPasswordResetTokenNotFound
		}
		return nil, fmt.Errorf("find password reset token: %w", err)
	}
	return &m, nil
}

func (r *PasswordResetRepository) Save(ctx context.Context, prt *model.PasswordReset) error {
	db := dbFromContext(ctx, r.db)
	if err := db.Create(prt).Error; err != nil {
		return fmt.Errorf("save password reset token: %w", translateDBError(err))
	}
	return nil
}

func (r *PasswordResetRepository) MarkUsed(ctx context.Context, id string) error {
	db := dbFromContext(ctx, r.db)
	now := time.Now().UTC()
	if err := db.Model(&model.PasswordReset{}).Where("id = ?", id).
		Update("used_at", now).Error; err != nil {
		return fmt.Errorf("mark reset token used: %w", err)
	}
	return nil
}

func (r *PasswordResetRepository) CountRecentByUser(ctx context.Context, userID string, since time.Time) (int, error) {
	db := dbFromContext(ctx, r.db)
	var count int64
	if err := db.Model(&model.PasswordReset{}).
		Where("user_id = ? AND created_at >= ?", userID, since).
		Count(&count).Error; err != nil {
		return 0, fmt.Errorf("count recent password resets: %w", err)
	}
	return int(count), nil
}
