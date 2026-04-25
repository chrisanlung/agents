package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/chrisanlung/lustia-auth/internal/constants"
	"github.com/chrisanlung/lustia-auth/internal/model"
	"github.com/chrisanlung/lustia-auth/internal/service"
	"gorm.io/gorm"
)

// UserRepository implements service.UserRepository using GORM + PostgreSQL.
type UserRepository struct {
	db *gorm.DB
}

// NewUserRepository constructs a UserRepository.
func NewUserRepository(db *gorm.DB) *UserRepository { return &UserRepository{db: db} }

// FindByEmail looks up a user by email globally (no tenant filter).
// The RLS policy on "user" allows the row to be seen when the caller has a
// membership in any of the user's tenants OR app.current_tenant is __platform__.
// For the login flow the service switches the context to __platform__ before
// calling this, so the lookup is always visible.
func (r *UserRepository) FindByEmail(ctx context.Context, email string) (*model.User, error) {
	db := dbFromContext(ctx, r.db)
	var m model.User
	err := db.Where("email = ? AND deleted_at IS NULL", email).
		First(&m).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, constants.ErrUserNotFound
		}
		return nil, fmt.Errorf("find user by email: %w", err)
	}
	return &m, nil
}

// FindByID looks up a user by primary key. Memberships are not preloaded here;
// callers that need membership data should use FindByIDWithMemberships or call
// MembershipRepository separately.
func (r *UserRepository) FindByID(ctx context.Context, id string) (*model.User, error) {
	db := dbFromContext(ctx, r.db)
	var m model.User
	err := db.Where("id = ? AND deleted_at IS NULL", id).
		First(&m).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, constants.ErrUserNotFound
		}
		return nil, fmt.Errorf("find user by id: %w", err)
	}
	return &m, nil
}

// FindByTenant lists users who have a membership in the given tenant, with
// optional filters and offset pagination. Returns rows and total matching count.
func (r *UserRepository) FindByTenant(ctx context.Context, tenantID string, filter service.UserFilter) ([]*model.User, int64, error) {
	db := dbFromContext(ctx, r.db)

	limit := filter.Limit
	if limit <= 0 {
		limit = 50
	}
	page := filter.Page
	if page < 1 {
		page = 1
	}

	// Base: users that have a membership in this tenant.
	q := db.Model(&model.User{}).Where(`deleted_at IS NULL AND id IN (
		SELECT user_id FROM membership WHERE tenant_id = ? AND status = 'active'
	)`, tenantID)

	if filter.IsActive != nil {
		q = q.Where("is_active = ?", *filter.IsActive)
	}
	if filter.RoleID != nil {
		q = q.Where(`id IN (
			SELECT m.user_id FROM membership m
			JOIN user_role ur ON ur.membership_id = m.id
			WHERE m.tenant_id = ? AND ur.role_id = ?
		)`, tenantID, *filter.RoleID)
	}
	if filter.BranchID != nil {
		q = q.Where(`id IN (
			SELECT m.user_id FROM membership m
			JOIN user_branch ub ON ub.membership_id = m.id
			WHERE m.tenant_id = ? AND ub.branch_id = ?
		)`, tenantID, *filter.BranchID)
	}

	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count users: %w", err)
	}

	var models []model.User
	if err := q.
		Order("id ASC").
		Offset((page - 1) * limit).
		Limit(limit).
		Find(&models).Error; err != nil {
		return nil, 0, fmt.Errorf("list users: %w", err)
	}

	users := make([]*model.User, len(models))
	for i := range models {
		users[i] = &models[i]
	}
	return users, total, nil
}

// Save inserts a new user row.
func (r *UserRepository) Save(ctx context.Context, u *model.User) error {
	db := dbFromContext(ctx, r.db)
	if err := db.Create(u).Error; err != nil {
		return translateDBError(err)
	}
	return nil
}

// Update writes mutable profile columns on an existing user row.
func (r *UserRepository) Update(ctx context.Context, u *model.User) error {
	db := dbFromContext(ctx, r.db)
	updates := map[string]interface{}{
		"full_name":          u.FullName,
		"phone":              u.Phone,
		"avatar_url":         u.AvatarURL,
		"is_active":          u.IsActive,
		"last_login_at":      u.LastLoginAt,
		"failed_login_count": u.FailedLoginCount,
		"locked_until":       u.LockedUntil,
	}
	if err := db.Model(&model.User{}).Where("id = ?", u.ID).Updates(updates).Error; err != nil {
		return fmt.Errorf("update user: %w", translateDBError(err))
	}
	return nil
}

// UpdatePassword replaces the password hash and clears must_change_password.
func (r *UserRepository) UpdatePassword(ctx context.Context, id string, hash string) error {
	db := dbFromContext(ctx, r.db)
	// A successful password change always clears must_change_password — the user
	// has rotated, so the flag's purpose is satisfied.
	if err := db.Model(&model.User{}).Where("id = ?", id).
		Updates(map[string]interface{}{
			"password_hash":        hash,
			"must_change_password": false,
		}).Error; err != nil {
		return fmt.Errorf("update password: %w", err)
	}
	return nil
}

// IncrementFailedLogin bumps failed_login_count and optionally sets locked_until.
func (r *UserRepository) IncrementFailedLogin(ctx context.Context, id string, lockUntil *time.Time) error {
	db := dbFromContext(ctx, r.db)
	updates := map[string]interface{}{
		"failed_login_count": gorm.Expr("failed_login_count + 1"),
		"locked_until":       lockUntil,
	}
	if err := db.Model(&model.User{}).Where("id = ?", id).Updates(updates).Error; err != nil {
		return fmt.Errorf("increment failed login: %w", err)
	}
	return nil
}

// ResetFailedLogin clears failed_login_count and locked_until.
func (r *UserRepository) ResetFailedLogin(ctx context.Context, id string) error {
	db := dbFromContext(ctx, r.db)
	updates := map[string]interface{}{
		"failed_login_count": 0,
		"locked_until":       nil,
	}
	if err := db.Model(&model.User{}).Where("id = ?", id).Updates(updates).Error; err != nil {
		return fmt.Errorf("reset failed login: %w", err)
	}
	return nil
}

// SoftDelete sets deleted_at on a user row.
func (r *UserRepository) SoftDelete(ctx context.Context, id string) error {
	db := dbFromContext(ctx, r.db)
	now := time.Now().UTC()
	if err := db.Model(&model.User{}).Where("id = ?", id).
		Update("deleted_at", now).Error; err != nil {
		return fmt.Errorf("soft delete user: %w", err)
	}
	return nil
}
