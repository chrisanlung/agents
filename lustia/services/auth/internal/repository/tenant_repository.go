package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/chrisanlung/lustia-auth/internal/constants"
	"github.com/chrisanlung/lustia-auth/internal/model"
	"gorm.io/gorm"
)

// TenantRepository implements service.TenantRepository.
type TenantRepository struct {
	db *gorm.DB
}

// NewTenantRepository constructs a TenantRepository.
func NewTenantRepository(db *gorm.DB) *TenantRepository { return &TenantRepository{db: db} }

func (r *TenantRepository) FindBySlug(ctx context.Context, slug string) (*model.Tenant, error) {
	db := dbFromContext(ctx, r.db)
	var m model.Tenant
	if err := db.Where("slug = ? AND deleted_at IS NULL", slug).First(&m).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, constants.ErrTenantNotFound
		}
		return nil, fmt.Errorf("find tenant by slug: %w", err)
	}
	return &m, nil
}

func (r *TenantRepository) FindByID(ctx context.Context, id string) (*model.Tenant, error) {
	db := dbFromContext(ctx, r.db)
	var m model.Tenant
	if err := db.Where("id = ? AND deleted_at IS NULL", id).First(&m).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, constants.ErrTenantNotFound
		}
		return nil, fmt.Errorf("find tenant by id: %w", err)
	}
	return &m, nil
}
