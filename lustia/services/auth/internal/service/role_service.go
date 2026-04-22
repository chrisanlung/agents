package service

import (
	"context"
	"fmt"

	"github.com/chrisanlung/lustia-auth/internal/helper"
)

// RoleService handles read-only role and permission queries.
// Role creation/editing is out of scope for Phase 2.
type RoleService struct {
	roles RoleRepository
}

// NewRoleService constructs a RoleService.
func NewRoleService(roles RoleRepository) *RoleService { return &RoleService{roles: roles} }

// ListRoles returns all platform roles with their permission codes.
func (s *RoleService) ListRoles(ctx context.Context) (ListRolesOutput, error) {
	models, err := s.roles.FindAll(ctx)
	if err != nil {
		return ListRolesOutput{}, fmt.Errorf("list roles: %w", err)
	}

	details := make([]RoleDetail, len(models))
	for i, r := range models {
		perms := make([]PermissionDetail, len(r.Permissions))
		for j, rp := range r.Permissions {
			perms[j] = PermissionDetail{
				ID:          rp.Permission.ID,
				Code:        rp.Permission.Code,
				Description: helper.DerefString(rp.Permission.Description),
			}
		}
		details[i] = RoleDetail{
			ID:          r.ID,
			Name:        r.Name,
			Description: helper.DerefString(r.Description),
			Permissions: perms,
		}
	}

	return ListRolesOutput{Roles: details}, nil
}
