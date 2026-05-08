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

// FindActiveByTenant returns all active memberships for a given tenant.
func (r *MembershipRepository) FindActiveByTenant(ctx context.Context, tenantID string) ([]*model.Membership, error) {
	db := dbFromContext(ctx, r.db)
	var ms []model.Membership
	if err := db.Where("tenant_id = ? AND status = ?", tenantID, model.MembershipStatusActive).
		Find(&ms).Error; err != nil {
		return nil, fmt.Errorf("find active memberships by tenant: %w", err)
	}
	result := make([]*model.Membership, len(ms))
	for i := range ms {
		result[i] = &ms[i]
	}
	return result, nil
}

// SuspendAllForTenant sets all active memberships for a tenant to 'suspended' in
// a single UPDATE. Returns the IDs of affected rows for audit logging.
func (r *MembershipRepository) SuspendAllForTenant(ctx context.Context, tenantID string) ([]string, error) {
	db := dbFromContext(ctx, r.db)

	// Collect IDs first so the audit layer can log each one.
	var ids []string
	if err := db.Model(&model.Membership{}).
		Where("tenant_id = ? AND status = ?", tenantID, model.MembershipStatusActive).
		Pluck("id", &ids).Error; err != nil {
		return nil, fmt.Errorf("collect active membership ids: %w", err)
	}
	if len(ids) == 0 {
		return ids, nil
	}

	if err := db.Model(&model.Membership{}).
		Where("id IN ?", ids).
		Update("status", string(model.MembershipStatusSuspended)).Error; err != nil {
		return nil, fmt.Errorf("suspend memberships: %w", err)
	}
	return ids, nil
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

// roleRow is the scan target for GetRolesAndBranches role queries.
type roleRow struct {
	RoleID   string
	RoleName string
}

// branchRow is the scan target for GetRolesAndBranches branch queries.
type branchRow struct {
	BranchID   string
	BranchName string
}

// membershipRoleRow is the scan target for the bulk role query.
type membershipRoleRow struct {
	MembershipID string
	RoleID       string
	RoleName     string
}

// membershipBranchRow is the scan target for the bulk branch query.
type membershipBranchRow struct {
	MembershipID string
	BranchID     string
	BranchName   string
}

// GetRolesAndBranches returns the role and branch IDs and names for a single
// membership. Two flat queries are issued (no N+1). Empty slices are returned
// when the membership has no assignments.
func (r *MembershipRepository) GetRolesAndBranches(ctx context.Context, membershipID string) (model.MembershipAssignments, error) {
	db := dbFromContext(ctx, r.db)

	var roles []roleRow
	if err := db.
		Table("user_role ur").
		Select("ur.role_id AS role_id, ro.name AS role_name").
		Joins("JOIN role ro ON ro.id = ur.role_id").
		Where("ur.membership_id = ?", membershipID).
		Scan(&roles).Error; err != nil {
		return model.MembershipAssignments{}, fmt.Errorf("get roles for membership: %w", err)
	}

	var branches []branchRow
	if err := db.
		Table("user_branch ub").
		Select("ub.branch_id AS branch_id, b.name AS branch_name").
		Joins("JOIN branch b ON b.id = ub.branch_id AND b.deleted_at IS NULL").
		Where("ub.membership_id = ?", membershipID).
		Scan(&branches).Error; err != nil {
		return model.MembershipAssignments{}, fmt.Errorf("get branches for membership: %w", err)
	}

	a := model.MembershipAssignments{
		RoleIDs:     make([]string, 0, len(roles)),
		RoleNames:   make([]string, 0, len(roles)),
		BranchIDs:   make([]string, 0, len(branches)),
		BranchNames: make([]string, 0, len(branches)),
	}
	for _, rr := range roles {
		a.RoleIDs = append(a.RoleIDs, rr.RoleID)
		a.RoleNames = append(a.RoleNames, rr.RoleName)
	}
	for _, br := range branches {
		a.BranchIDs = append(a.BranchIDs, br.BranchID)
		a.BranchNames = append(a.BranchNames, br.BranchName)
	}
	return a, nil
}

// GetRolesAndBranchesForMemberships returns MembershipAssignments keyed by
// membership_id for a batch of memberships. Two flat IN-queries are issued to
// avoid N+1. The returned map contains an entry only for memberships that have
// at least one assignment; callers treat absent keys as empty assignments.
func (r *MembershipRepository) GetRolesAndBranchesForMemberships(ctx context.Context, membershipIDs []string) (map[string]model.MembershipAssignments, error) {
	result := make(map[string]model.MembershipAssignments, len(membershipIDs))
	if len(membershipIDs) == 0 {
		return result, nil
	}

	db := dbFromContext(ctx, r.db)

	var roles []membershipRoleRow
	if err := db.
		Table("user_role ur").
		Select("ur.membership_id AS membership_id, ur.role_id AS role_id, ro.name AS role_name").
		Joins("JOIN role ro ON ro.id = ur.role_id").
		Where("ur.membership_id IN ?", membershipIDs).
		Scan(&roles).Error; err != nil {
		return nil, fmt.Errorf("bulk get roles for memberships: %w", err)
	}

	var branches []membershipBranchRow
	if err := db.
		Table("user_branch ub").
		Select("ub.membership_id AS membership_id, ub.branch_id AS branch_id, b.name AS branch_name").
		Joins("JOIN branch b ON b.id = ub.branch_id AND b.deleted_at IS NULL").
		Where("ub.membership_id IN ?", membershipIDs).
		Scan(&branches).Error; err != nil {
		return nil, fmt.Errorf("bulk get branches for memberships: %w", err)
	}

	// Group roles back per membership.
	for _, rr := range roles {
		a := result[rr.MembershipID]
		a.RoleIDs = append(a.RoleIDs, rr.RoleID)
		a.RoleNames = append(a.RoleNames, rr.RoleName)
		result[rr.MembershipID] = a
	}
	// Group branches back per membership.
	for _, br := range branches {
		a := result[br.MembershipID]
		a.BranchIDs = append(a.BranchIDs, br.BranchID)
		a.BranchNames = append(a.BranchNames, br.BranchName)
		result[br.MembershipID] = a
	}
	return result, nil
}
