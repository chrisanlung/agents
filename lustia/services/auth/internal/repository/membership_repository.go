package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/chrisanlung/lustia-auth/internal/constants"
	"github.com/chrisanlung/lustia-auth/internal/model"
	"gorm.io/gorm"
)

// MembershipRepository implements service.MembershipRepository using GORM + PostgreSQL.
type MembershipRepository struct {
	db *gorm.DB
}

// NewMembershipRepository constructs a MembershipRepository.
func NewMembershipRepository(db *gorm.DB) *MembershipRepository {
	return &MembershipRepository{db: db}
}

// membershipPreloads applies the standard set of eager-loads needed for
// membership queries: roles (with their permissions) and branches and tenant.
func membershipPreloads(q *gorm.DB) *gorm.DB {
	return q.
		Preload("Roles.Role.Permissions.Permission").
		Preload("Branches").
		Preload("Tenant")
}

// FindByUser returns all memberships for the given user, with roles, branches,
// and tenant preloaded. The RLS policy on membership allows a user to always
// see their own rows, so no explicit tenant context switch is needed here.
func (r *MembershipRepository) FindByUser(ctx context.Context, userID string) ([]*model.Membership, error) {
	db := dbFromContext(ctx, r.db)
	var ms []model.Membership
	q := membershipPreloads(db.Where("user_id = ?", userID))
	if err := q.Find(&ms).Error; err != nil {
		return nil, fmt.Errorf("find memberships by user: %w", err)
	}
	result := make([]*model.Membership, len(ms))
	for i := range ms {
		result[i] = &ms[i]
	}
	return result, nil
}

// FindByUserAndTenant returns the membership for a specific (user, tenant) pair.
// Returns constants.ErrMembershipNotFound when no row exists.
func (r *MembershipRepository) FindByUserAndTenant(ctx context.Context, userID, tenantID string) (*model.Membership, error) {
	db := dbFromContext(ctx, r.db)
	var m model.Membership
	q := membershipPreloads(db.Where("user_id = ? AND tenant_id = ?", userID, tenantID))
	if err := q.First(&m).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, constants.ErrMembershipNotFound
		}
		return nil, fmt.Errorf("find membership by user+tenant: %w", err)
	}
	return &m, nil
}

// FindByID returns a single membership by its primary key.
// Returns constants.ErrMembershipNotFound when no row exists.
func (r *MembershipRepository) FindByID(ctx context.Context, id string) (*model.Membership, error) {
	db := dbFromContext(ctx, r.db)
	var m model.Membership
	q := membershipPreloads(db.Where("id = ?", id))
	if err := q.First(&m).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, constants.ErrMembershipNotFound
		}
		return nil, fmt.Errorf("find membership by id: %w", err)
	}
	return &m, nil
}

// Save inserts a new membership row.
func (r *MembershipRepository) Save(ctx context.Context, m *model.Membership) error {
	db := dbFromContext(ctx, r.db)
	if err := db.Create(m).Error; err != nil {
		return fmt.Errorf("save membership: %w", translateDBError(err))
	}
	return nil
}

// Update writes mutable columns on an existing membership row.
func (r *MembershipRepository) Update(ctx context.Context, m *model.Membership) error {
	db := dbFromContext(ctx, r.db)
	updates := map[string]interface{}{
		"status":     m.Status,
		"invited_at": m.InvitedAt,
		"joined_at":  m.JoinedAt,
		"left_at":    m.LeftAt,
		"metadata":   m.Metadata,
		"updated_by": m.UpdatedBy,
	}
	if err := db.Model(&model.Membership{}).Where("id = ?", m.ID).Updates(updates).Error; err != nil {
		return fmt.Errorf("update membership: %w", translateDBError(err))
	}
	return nil
}

// SetStatus transitions a membership to a new status.
func (r *MembershipRepository) SetStatus(ctx context.Context, id string, status model.MembershipStatus) error {
	db := dbFromContext(ctx, r.db)
	if err := db.Model(&model.Membership{}).Where("id = ?", id).
		Update("status", string(status)).Error; err != nil {
		return fmt.Errorf("set membership status: %w", err)
	}
	return nil
}

// AssignRoles replaces all role assignments for the given membership atomically.
// Passing an empty slice clears all roles.
func (r *MembershipRepository) AssignRoles(ctx context.Context, membershipID string, roleIDs []string, assignedBy string) error {
	db := dbFromContext(ctx, r.db)
	if err := db.Where("membership_id = ?", membershipID).Delete(&model.UserRole{}).Error; err != nil {
		return fmt.Errorf("clear existing roles: %w", err)
	}
	if len(roleIDs) == 0 {
		return nil
	}
	rows := make([]model.UserRole, len(roleIDs))
	for i, rid := range roleIDs {
		rows[i] = model.UserRole{
			MembershipID: membershipID,
			RoleID:       rid,
			AssignedBy:   &assignedBy,
		}
	}
	if err := db.Create(&rows).Error; err != nil {
		return fmt.Errorf("assign roles: %w", translateDBError(err))
	}
	return nil
}

// AssignBranches replaces all branch assignments for the given membership atomically.
// Passing an empty slice clears all branches.
func (r *MembershipRepository) AssignBranches(ctx context.Context, membershipID string, branchIDs []string, assignedBy string) error {
	db := dbFromContext(ctx, r.db)
	if err := db.Where("membership_id = ?", membershipID).Delete(&model.UserBranch{}).Error; err != nil {
		return fmt.Errorf("clear existing branches: %w", err)
	}
	if len(branchIDs) == 0 {
		return nil
	}
	rows := make([]model.UserBranch, len(branchIDs))
	for i, bid := range branchIDs {
		rows[i] = model.UserBranch{
			MembershipID: membershipID,
			BranchID:     bid,
			AssignedBy:   &assignedBy,
		}
	}
	if err := db.Create(&rows).Error; err != nil {
		return fmt.Errorf("assign branches: %w", translateDBError(err))
	}
	return nil
}
