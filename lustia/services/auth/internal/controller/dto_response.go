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

// ListUsersResponse carries a page of users and pagination metadata.
type ListUsersResponse struct {
	Data       []UserProfileResponse `json:"data"`
	Page       int                   `json:"page"`
	Limit      int                   `json:"limit"`
	TotalCount int64                 `json:"total_count"`
	TotalPages int                   `json:"total_pages"`
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
// Phase 3 — Registration
// ---------------------------------------------------------------------------

// CompanyRegistrationResponse is the response for POST /register/company.
type CompanyRegistrationResponse struct {
	RegistrationID string `json:"registration_id"`
	Status         string `json:"status"`
}

// RegistrationSummaryResponse is the public projection of a tenant_registration row.
type RegistrationSummaryResponse struct {
	ID              string  `json:"id"`
	CompanyName     string  `json:"company_name"`
	RequestedSlug   string  `json:"requested_slug"`
	Package         string  `json:"package"`
	ContactName     string  `json:"contact_name"`
	ContactEmail    string  `json:"contact_email"`
	ContactPhone    *string `json:"contact_phone,omitempty"`
	Status          string  `json:"status"`
	ApprovedAt      *string `json:"approved_at,omitempty"`
	ApprovedBy      *string `json:"approved_by,omitempty"`
	RejectedAt      *string `json:"rejected_at,omitempty"`
	RejectionReason *string `json:"rejection_reason,omitempty"`
	CreatedAt       string  `json:"created_at"`
}

// ListRegistrationsResponse carries a page of registrations and pagination metadata.
type ListRegistrationsResponse struct {
	Data       []RegistrationSummaryResponse `json:"data"`
	Page       int                           `json:"page"`
	Limit      int                           `json:"limit"`
	TotalCount int64                         `json:"total_count"`
	TotalPages int                           `json:"total_pages"`
}

// TenantDetailResponse is the full tenant object returned in the approve response.
type TenantDetailResponse struct {
	ID           string  `json:"id"`
	Name         string  `json:"name"`
	Slug         string  `json:"slug"`
	Status       string  `json:"status"`
	Package      string  `json:"package"`
	MaxBranches  int     `json:"max_branches"`
	ContactEmail string  `json:"contact_email,omitempty"`
	ContactName  string  `json:"contact_name,omitempty"`
	ApprovedAt   *string `json:"approved_at,omitempty"`
	ApprovedBy   *string `json:"approved_by,omitempty"`
	CreatedAt    string  `json:"created_at"`
}

// TenantAdminResponse carries the newly-created tenant admin credentials.
// TemporaryPassword is returned once and also emailed to the contact.
type TenantAdminResponse struct {
	UserID            string `json:"user_id"`
	Email             string `json:"email"`
	TemporaryPassword string `json:"temporary_password"`
}

// ApproveTenantRegistrationResponse is the full approval response.
type ApproveTenantRegistrationResponse struct {
	Tenant       TenantDetailResponse        `json:"tenant"`
	TenantAdmin  TenantAdminResponse         `json:"tenant_admin"`
	Registration RegistrationSummaryResponse `json:"registration"`
}

// ---------------------------------------------------------------------------
// Phase 3 — Tenant management
// ---------------------------------------------------------------------------

// TenantSummaryResponse is the admin-facing projection of a tenant with counts.
type TenantSummaryResponse struct {
	ID              string  `json:"id"`
	Name            string  `json:"name"`
	Slug            string  `json:"slug"`
	Status          string  `json:"status"`
	Package         string  `json:"package"`
	MaxBranches     int     `json:"max_branches"`
	ContactEmail    string  `json:"contact_email,omitempty"`
	ContactName     string  `json:"contact_name,omitempty"`
	ApprovedAt      *string `json:"approved_at,omitempty"`
	ApprovedBy      *string `json:"approved_by,omitempty"`
	RejectedAt      *string `json:"rejected_at,omitempty"`
	RejectionReason *string `json:"rejection_reason,omitempty"`
	CreatedAt       string  `json:"created_at"`
	MembershipCount int     `json:"membership_count"`
	BranchCount     int     `json:"branch_count"`
}

// ListTenantsResponse carries a page of tenants and pagination metadata.
type ListTenantsResponse struct {
	Data       []TenantSummaryResponse `json:"data"`
	Page       int                     `json:"page"`
	Limit      int                     `json:"limit"`
	TotalCount int64                   `json:"total_count"`
	TotalPages int                     `json:"total_pages"`
}

// ---------------------------------------------------------------------------
// Phase 3 — Branch management
// ---------------------------------------------------------------------------

// BranchResponse is the public projection of a branch row.
type BranchResponse struct {
	ID           string  `json:"id"`
	TenantID     string  `json:"tenant_id"`
	Name         string  `json:"name"`
	Code         string  `json:"code"`
	Status       string  `json:"status"`
	AddressLine1 *string `json:"address_line1,omitempty"`
	AddressLine2 *string `json:"address_line2,omitempty"`
	City         *string `json:"city,omitempty"`
	Province     *string `json:"province,omitempty"`
	PostalCode   *string `json:"postal_code,omitempty"`
	Country      string  `json:"country"`
	Timezone     string  `json:"timezone"`
	ContactPhone *string `json:"contact_phone,omitempty"`
	ContactEmail *string `json:"contact_email,omitempty"`
	ActivatedAt  *string `json:"activated_at,omitempty"`
	CreatedAt    string  `json:"created_at"`
	UpdatedAt    string  `json:"updated_at"`
}

// ListBranchesResponse carries a page of branches and pagination metadata.
type ListBranchesResponse struct {
	Data       []BranchResponse `json:"data"`
	Page       int              `json:"page"`
	Limit      int              `json:"limit"`
	TotalCount int64            `json:"total_count"`
	TotalPages int              `json:"total_pages"`
}

// OnboardingStateResponse is the response for GET /tenant/onboarding-state.
type OnboardingStateResponse struct {
	HasBranches        bool `json:"has_branches"`
	ActiveBranchCount  int  `json:"active_branch_count"`
	MaxBranches        int  `json:"max_branches"`
	MustChangePassword bool `json:"must_change_password"`
}

// ---------------------------------------------------------------------------
// Phase 4 — Master Operational Data (ADR 0009).
// ---------------------------------------------------------------------------

// TherapistServiceItemResponse is a single entry in the services array on a
// therapist detail or mapping response.
type TherapistServiceItemResponse struct {
	ServiceID       string  `json:"service_id"`
	Name            string  `json:"name"`
	Category        *string `json:"category,omitempty"`
	DurationMinutes int     `json:"duration_minutes"`
	PriceIDR        int64   `json:"price_idr"`
	IsActive        bool    `json:"is_active"`
	AssignedAt      string  `json:"assigned_at"`
}

// TherapistResponse is the public projection of a therapist row (§11.4 shared shape).
// PhotoURL is resolved from the stored photo_key at the controller boundary
// (ADR 0011 §2.1). photo_key is never exposed directly over the wire.
type TherapistResponse struct {
	ID          string   `json:"id"`
	TenantID    string   `json:"tenant_id"`
	BranchID    string   `json:"branch_id"`
	UserID      *string  `json:"user_id"`
	FullName    string   `json:"full_name"`
	Gender      *string  `json:"gender"`
	Bio         *string  `json:"bio"`
	PhotoURL    *string  `json:"photo_url"`    // resolved URL; null when no photo
	HeightCm    int16    `json:"height_cm"`
	WeightKg    int16    `json:"weight_kg"`
	Build       string   `json:"build"`
	Specialties []string `json:"specialties"`
	IsActive    bool     `json:"is_active"`
	JoinedAt    *string  `json:"joined_at"`
	CreatedAt   string   `json:"created_at"`
	UpdatedAt   string   `json:"updated_at"`
}

// TherapistDetailResponse extends TherapistResponse with the services array
// for GET /tenant/therapists/:id.
type TherapistDetailResponse struct {
	TherapistResponse
	Services []TherapistServiceItemResponse `json:"services"`
}

// ListTherapistsResponse carries a page of therapists and pagination metadata.
type ListTherapistsResponse struct {
	Data       []TherapistResponse `json:"data"`
	Page       int                 `json:"page"`
	Limit      int                 `json:"limit"`
	TotalCount int64               `json:"total_count"`
	TotalPages int                 `json:"total_pages"`
}

// ServiceResponse is the public projection of a service row (§11.5 shared shape).
type ServiceResponse struct {
	ID              string  `json:"id"`
	TenantID        string  `json:"tenant_id"`
	Name            string  `json:"name"`
	Description     *string `json:"description"`
	Category        *string `json:"category"`
	DurationMinutes int     `json:"duration_minutes"`
	PriceIDR        int64   `json:"price_idr"`
	Currency        string  `json:"currency"`
	IsActive        bool    `json:"is_active"`
	CreatedAt       string  `json:"created_at"`
	UpdatedAt       string  `json:"updated_at"`
}

// ServiceTherapistItemResponse is a single entry in the therapists array on a
// service detail response.
type ServiceTherapistItemResponse struct {
	TherapistID string `json:"therapist_id"`
	FullName    string `json:"full_name"`
	BranchID    string `json:"branch_id"`
	BranchName  string `json:"branch_name"`
	IsActive    bool   `json:"is_active"`
}

// ServiceDetailResponse extends ServiceResponse with the therapists array
// for GET /tenant/services/:id. Add-ons are a separate tenant-wide resource
// accessed via GET /tenant/addons (ADR 0010 rewrite 2026-04-24).
type ServiceDetailResponse struct {
	ServiceResponse
	Therapists []ServiceTherapistItemResponse `json:"therapists"`
}

// AddonResponse is the public projection of an addon row (ADR 0010).
// tenant_id is intentionally omitted — it is implicit from the JWT.
type AddonResponse struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	Description *string `json:"description"`
	PriceIDR    int64   `json:"price_idr"`
	IsActive    bool    `json:"is_active"`
	SortOrder   int     `json:"sort_order"`
	CreatedAt   string  `json:"created_at"`
	UpdatedAt   string  `json:"updated_at"`
}

// ListAddonsResponse is the response for GET /tenant/addons.
type ListAddonsResponse struct {
	Data       []AddonResponse `json:"data"`
	Page       int             `json:"page"`
	Limit      int             `json:"limit"`
	TotalCount int64           `json:"total_count"`
	TotalPages int             `json:"total_pages"`
}

// ListServicesResponse carries a page of services and pagination metadata.
type ListServicesResponse struct {
	Data       []ServiceResponse `json:"data"`
	Page       int               `json:"page"`
	Limit      int               `json:"limit"`
	TotalCount int64             `json:"total_count"`
	TotalPages int               `json:"total_pages"`
}

// TherapistMappingResponse is the response for GET/PUT /therapists/:id/services.
type TherapistMappingResponse struct {
	TherapistID string                         `json:"therapist_id"`
	Services    []TherapistServiceItemResponse `json:"services"`
}

// AvailabilityWindowResponse is a single window in the availability response.
type AvailabilityWindowResponse struct {
	ID    string `json:"id"`
	DOW   int    `json:"dow"`
	Start string `json:"start"`
	End   string `json:"end"`
}

// AvailabilityResponse is the response for GET/PUT /therapists/:id/availability.
type AvailabilityResponse struct {
	TherapistID string                       `json:"therapist_id"`
	Windows     []AvailabilityWindowResponse `json:"windows"`
}

// ---------------------------------------------------------------------------
// ADR 0012 — Room (Ruangan) catalog.
// ---------------------------------------------------------------------------

// RoomResponse is the public projection of a room row (ADR 0012 §2.3).
// tenant_id and photo_key are intentionally omitted:
//   - tenant_id is implicit from the JWT (lesson from addon code-review).
//   - photo_key is an opaque internal storage key; controllers resolve it to
//     photo_url via Storage.URL before building the response.
type RoomResponse struct {
	ID          string   `json:"id"`
	BranchID    string   `json:"branch_id"`
	Name        string   `json:"name"`
	Description *string  `json:"description"`
	RoomType    string   `json:"room_type"`
	Capacity    int16    `json:"capacity"`
	Amenities   []string `json:"amenities"`
	PhotoURL    *string  `json:"photo_url"`  // resolved URL; null when no photo
	IsActive    bool     `json:"is_active"`
	SortOrder   int      `json:"sort_order"`
	CreatedAt   string   `json:"created_at"`
	UpdatedAt   string   `json:"updated_at"`
}

// ListRoomsResponse is the response for GET /tenant/rooms.
type ListRoomsResponse struct {
	Data       []RoomResponse `json:"data"`
	Page       int            `json:"page"`
	Limit      int            `json:"limit"`
	TotalCount int64          `json:"total_count"`
	TotalPages int            `json:"total_pages"`
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
