package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/chrisanlung/lustia-auth/internal/constants"
	"github.com/chrisanlung/lustia-auth/internal/model"
	"gorm.io/gorm"
)

// RoleRepository implements service.RoleRepository.
type RoleRepository struct {
	db *gorm.DB
}

// NewRoleRepository constructs a RoleRepository.
func NewRoleRepository(db *gorm.DB) *RoleRepository { return &RoleRepository{db: db} }

func (r *RoleRepository) FindAll(ctx context.Context) ([]*model.Role, error) {
	db := dbFromContext(ctx, r.db)
	var models []model.Role
	if err := db.
		Preload("Permissions.Permission").
		Find(&models).Error; err != nil {
		return nil, fmt.Errorf("find all roles: %w", err)
	}
	roles := make([]*model.Role, len(models))
	for i := range models {
		roles[i] = &models[i]
	}
	return roles, nil
}

func (r *RoleRepository) FindByIDs(ctx context.Context, ids []string) ([]*model.Role, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	db := dbFromContext(ctx, r.db)
	var models []model.Role
	if err := db.
		Where("id IN ?", ids).
		Preload("Permissions.Permission").
		Find(&models).Error; err != nil {
		return nil, fmt.Errorf("find roles by ids: %w", err)
	}
	if len(models) != len(ids) {
		return nil, errors.Join(constants.ErrRoleNotFound, fmt.Errorf("expected %d roles, got %d", len(ids), len(models)))
	}
	roles := make([]*model.Role, len(models))
	for i := range models {
		roles[i] = &models[i]
	}
	return roles, nil
}
