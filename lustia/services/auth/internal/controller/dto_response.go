package controller

import (
	"time"

	"github.com/chrisanlung/lustia-auth/internal/service"
)

// MembershipSummaryResponse is the public projection of a single membership.
type MembershipSummaryResponse struct {
	MembershipID string   `json:"membership_id"`
	TenantID     string   `json:"tenant_id"`
	TenantName   string   `json:"tenant_name"`
	TenantSlug   string   `json:"tenant_slug"`
	Roles        []string `json:"roles"`
	Branches     []string `json:"branches"`
	Status       string   `json:"status"`
}

// LoginResponse is the unified login response. The Scope field tells the caller
// which branch was taken:
//   - "platform"  → super-admin, ActiveMembershipID omitted
//   - "tenant"    → single membership auto-selected, ActiveMembershipID set
//   - "user"      → multiple memberships, caller must call /auth/select-tenant
type LoginResponse struct {
	AccessToken        string                      `json:"access_token"`
	RefreshToken       string                      `json:"refresh_token"`
	TokenType          string                      `json:"token_type"` // always "Bearer"
	ExpiresAt          time.Time                   `json:"expires_at"` // RFC3339 UTC
	Scope              string                      `json:"scope"`
	User               UserProfileResponse         `json:"user"`
	Memberships        []MembershipSummaryResponse `json:"memberships"`
	ActiveMembershipID *string                     `json:"active_membership_id"` // null for scope=platform and scope=user
}

// SelectTenantResponse is the response for POST /auth/select-tenant.
type SelectTenantResponse struct {
	AccessToken        string                    `json:"access_token"`
	RefreshToken       string                    `json:"refresh_token"`
	TokenType          string                    `json:"token_type"`
	ExpiresAt          time.Time                 `json:"expires_at"`
	Scope              string                    `json:"scope"` // always "tenant"
	ActiveMembershipID string                    `json:"active_membership_id"`
	Membership         MembershipSummaryResponse `json:"membership"`
}

// RefreshResponse carries a rotated token pair. Scope and ActiveMembershipID
// mirror what was in effect when the original refresh token was issued.
type RefreshResponse struct {
	AccessToken        string    `json:"access_token"`
	RefreshToken       string    `json:"refresh_token"`
	TokenType          string    `json:"token_type"`
	ExpiresAt          time.Time `json:"expires_at"`
	Scope              string    `json:"scope"`
	ActiveMembershipID *string   `json:"active_membership_id"` // null for scope=platform and scope=user
}

// UserProfileResponse is the public projection of a user identity.
// Roles and branches are no longer part of the user profile — they belong to
// the membership. IsSuperAdmin and MustChangePassword are added per ADR 0007.
type UserProfileResponse struct {
	ID                 string `json:"id"`
	Email              string `json:"email"`
	FullName           string `json:"full_name"`
	Phone              string `json:"phone,omitempty"`
	AvatarURL          string `json:"avatar_url,omitempty"`
	IsActive           bool   `json:"is_active"`
	IsSuperAdmin       bool   `json:"is_super_admin"`
	MustChangePassword bool   `json:"must_change_password,omitempty"`
}

// GetMeResponse wraps the user profile with optional tenant info and memberships.
type GetMeResponse struct {
	User               UserProfileResponse         `json:"user"`
	ActiveMembershipID *string                     `json:"active_membership_id"`
	Tenant             *TenantInfoResponse         `json:"tenant,omitempty"`
	Memberships        []MembershipSummaryResponse `json:"memberships"`
}

// TenantInfoResponse is the public projection of a tenant.
type TenantInfoResponse struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Slug   string `json:"slug"`
	Status string `json:"status"`
}

// CreateUserResponse carries the created/invited user + optional initial password.
// CreatedUser and CreatedMembership tell the caller exactly what was inserted.
type CreateUserResponse struct {
	User              UserProfileResponse `json:"user"`
	InitialPassword   string              `json:"initial_password,omitempty"` // set only when CreatedUser=true
	CreatedUser       bool                `json:"created_user"`
	CreatedMembership bool                `json:"created_membership"`
}

// ListUsersResponse carries a page of users and the next cursor.
type ListUsersResponse struct {
	Data       []UserProfileResponse `json:"data"`
	NextCursor string                `json:"next_cursor,omitempty"`
}

// RoleResponse carries a role with its permissions.
type RoleResponse struct {
	ID          string               `json:"id"`
	Name        string               `json:"name"`
	Description string               `json:"description,omitempty"`
	Permissions []PermissionResponse `json:"permissions"`
}

// PermissionResponse is the public projection of a permission.
type PermissionResponse struct {
	ID          string `json:"id"`
	Code        string `json:"code"`
	Description string `json:"description,omitempty"`
}

// ListRolesResponse carries all roles.
type ListRolesResponse struct {
	Data []RoleResponse `json:"data"`
}

// ---------------------------------------------------------------------------
// Mapping helpers
// ---------------------------------------------------------------------------

// toUserProfileResponse converts a service.UserProfile to a UserProfileResponse.
func toUserProfileResponse(p service.UserProfile) UserProfileResponse {
	return UserProfileResponse{
		ID:                 p.ID,
		Email:              p.Email,
		FullName:           p.FullName,
		Phone:              p.Phone,
		AvatarURL:          p.AvatarURL,
		IsActive:           p.IsActive,
		IsSuperAdmin:       p.IsSuperAdmin,
		MustChangePassword: p.MustChangePassword,
	}
}

// toMembershipSummaryResponse converts a service.MembershipSummary to its
// response DTO, ensuring slices are never nil.
func toMembershipSummaryResponse(s service.MembershipSummary) MembershipSummaryResponse {
	roles := s.Roles
	if roles == nil {
		roles = []string{}
	}
	branches := s.Branches
	if branches == nil {
		branches = []string{}
	}
	return MembershipSummaryResponse{
		MembershipID: s.MembershipID,
		TenantID:     s.TenantID,
		TenantName:   s.TenantName,
		TenantSlug:   s.TenantSlug,
		Roles:        roles,
		Branches:     branches,
		Status:       s.Status,
	}
}

// toMembershipSummaryResponses maps a slice of summaries.
func toMembershipSummaryResponses(summaries []service.MembershipSummary) []MembershipSummaryResponse {
	out := make([]MembershipSummaryResponse, len(summaries))
	for i, s := range summaries {
		out[i] = toMembershipSummaryResponse(s)
	}
	return out
}
