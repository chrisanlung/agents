package service

import (
	"context"
	"fmt"

	"github.com/chrisanlung/lustia-auth/internal/model"
)

// MeService handles the authenticated user's self-service profile operations.
type MeService struct {
	users       UserRepository
	memberships MembershipRepository
	tenants     TenantRepository
}

// NewMeService constructs a MeService.
func NewMeService(users UserRepository, memberships MembershipRepository, tenants TenantRepository) *MeService {
	return &MeService{users: users, memberships: memberships, tenants: tenants}
}

// GetMe loads the full user profile for the authenticated caller.
//
// callerMembershipID is the membership_id claim from the JWT, which is non-nil
// only when scope=tenant. When non-nil the matching membership's tenant and
// roles/branches are included in the response. For scope=user, all memberships
// are listed but none is marked active. For scope=platform the memberships
// slice is empty and tenant is nil (super-admin sees the platform view).
func (s *MeService) GetMe(ctx context.Context, callerID string, callerMembershipID *string, scope model.TokenScope) (GetMeOutput, error) {
	user, err := s.users.FindByID(ctx, callerID)
	if err != nil {
		return GetMeOutput{}, fmt.Errorf("get me: %w", err)
	}

	out := GetMeOutput{
		User:               toUserProfile(user),
		ActiveMembershipID: callerMembershipID,
		Memberships:        []MembershipSummary{},
	}

	switch scope {
	case model.ScopePlatform:
		// Super-admin: no memberships, no tenant info.
		return out, nil

	case model.ScopeTenant:
		// Tenant-scoped: populate active membership tenant + list all memberships.
		if callerMembershipID != nil {
			membership, err := s.memberships.FindByID(ctx, *callerMembershipID)
			if err != nil {
				return GetMeOutput{}, fmt.Errorf("get active membership for me: %w", err)
			}
			out.Tenant = &TenantInfo{
				ID:     membership.Tenant.ID,
				Name:   membership.Tenant.Name,
				Slug:   membership.Tenant.Slug,
				Status: membership.Tenant.Status,
			}
		}
		allMemberships, err := s.memberships.FindByUser(ctx, callerID)
		if err != nil {
			return GetMeOutput{}, fmt.Errorf("list memberships for me: %w", err)
		}
		out.Memberships = toMembershipSummaries(filterActive(allMemberships))
		return out, nil

	default:
		// scope=user: list active memberships, no active tenant.
		allMemberships, err := s.memberships.FindByUser(ctx, callerID)
		if err != nil {
			return GetMeOutput{}, fmt.Errorf("list memberships for me: %w", err)
		}
		out.Memberships = toMembershipSummaries(filterActive(allMemberships))
		return out, nil
	}
}

// UpdateMe applies the requested profile changes.
func (s *MeService) UpdateMe(ctx context.Context, in UpdateMeInput) (UserProfile, error) {
	user, err := s.users.FindByID(ctx, in.CallerUserID)
	if err != nil {
		return UserProfile{}, fmt.Errorf("find user for update: %w", err)
	}

	if in.FullName != nil {
		user.FullName = *in.FullName
	}
	if in.Phone != nil {
		if *in.Phone == "" {
			user.Phone = nil
		} else {
			user.Phone = in.Phone
		}
	}
	if in.AvatarURL != nil {
		if *in.AvatarURL == "" {
			user.AvatarURL = nil
		} else {
			user.AvatarURL = in.AvatarURL
		}
	}

	if err := s.users.Update(ctx, user); err != nil {
		return UserProfile{}, fmt.Errorf("update user: %w", err)
	}

	return toUserProfile(user), nil
}

// filterActive returns only the memberships with status=active.
func filterActive(ms []*model.Membership) []*model.Membership {
	out := make([]*model.Membership, 0, len(ms))
	for _, m := range ms {
		if m.IsActive() {
			out = append(out, m)
		}
	}
	return out
}
