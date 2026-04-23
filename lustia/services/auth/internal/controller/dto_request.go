// Package controller contains the HTTP handlers (thin controllers) and
// request/response DTOs for the auth service. Controllers call services;
// they never call repositories directly.
package controller

// LoginRequest is the JSON body for POST /auth/login.
// tenant_slug is removed in ADR 0007: login is email+password only.
type LoginRequest struct {
	Email    string `json:"email"    binding:"required,email,max=320"`
	Password string `json:"password" binding:"required,min=8,max=128"`
}

// RefreshRequest is the JSON body for POST /auth/refresh.
type RefreshRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

// LogoutRequest is the JSON body for POST /auth/logout.
type LogoutRequest struct {
	RefreshToken string `json:"refresh_token"` // optional
}

// SelectTenantRequest is the JSON body for POST /auth/select-tenant.
type SelectTenantRequest struct {
	TenantID string `json:"tenant_id" binding:"required,uuid"`
}

// ForgotPasswordRequest is the JSON body for POST /auth/password/forgot.
// tenant_slug is removed in ADR 0007: lookup is global by email.
type ForgotPasswordRequest struct {
	Email string `json:"email" binding:"required,email,max=320"`
}

// ResetPasswordRequest is the JSON body for POST /auth/password/reset.
type ResetPasswordRequest struct {
	Token       string `json:"token"        binding:"required"`
	NewPassword string `json:"new_password" binding:"required,min=8,max=128"`
}

// UpdateMeRequest is the JSON body for PATCH /auth/me.
type UpdateMeRequest struct {
	FullName  *string `json:"full_name"  binding:"omitempty,min=1,max=200"`
	Phone     *string `json:"phone"      binding:"omitempty,max=30"`
	AvatarURL *string `json:"avatar_url" binding:"omitempty,url,max=2048"`
}

// ChangePasswordRequest is the JSON body for POST /auth/me/password.
type ChangePasswordRequest struct {
	OldPassword string `json:"old_password" binding:"required,min=8,max=128"`
	NewPassword string `json:"new_password" binding:"required,min=8,max=128"`
}

// CreateUserRequest is the JSON body for POST /admin/users.
type CreateUserRequest struct {
	Email     string   `json:"email"      binding:"required,email,max=320"`
	FullName  string   `json:"full_name"  binding:"required,min=1,max=200"`
	Phone     string   `json:"phone"      binding:"omitempty,max=30"`
	RoleIDs   []string `json:"role_ids"   binding:"omitempty,dive,uuid"`
	BranchIDs []string `json:"branch_ids" binding:"omitempty,dive,uuid"`
}

// ListUsersQuery are the query parameters for GET /admin/users.
type ListUsersQuery struct {
	RoleID   string `form:"role_id"   binding:"omitempty,uuid"`
	BranchID string `form:"branch_id" binding:"omitempty,uuid"`
	IsActive *bool  `form:"is_active"`
	Cursor   string `form:"cursor"`
	Limit    int    `form:"limit" binding:"omitempty,min=1,max=200"`
}

// UpdateUserRequest is the JSON body for PATCH /admin/users/:id.
type UpdateUserRequest struct {
	FullName  *string  `json:"full_name"  binding:"omitempty,min=1,max=200"`
	Phone     *string  `json:"phone"      binding:"omitempty,max=30"`
	AvatarURL *string  `json:"avatar_url" binding:"omitempty,url,max=2048"`
	IsActive  *bool    `json:"is_active"`
	RoleIDs   []string `json:"role_ids"   binding:"omitempty,dive,uuid"`
	BranchIDs []string `json:"branch_ids" binding:"omitempty,dive,uuid"`
}

// ---------------------------------------------------------------------------
// Phase 3 — Registration
// ---------------------------------------------------------------------------

// CompanyRegistrationRequest is the JSON body for POST /register/company.
// RequestedSlug is optional — when omitted, the service generates one from
// CompanyName and retries on collision. When supplied, it MUST match the
// slugify() output format ([a-z0-9] with single-hyphen separators, no leading
// or trailing hyphen) per SECURITY.md Phase 3 M-3.
type CompanyRegistrationRequest struct {
	CompanyName   string `json:"company_name"    binding:"required,min=2,max=200"`
	RequestedSlug string `json:"requested_slug"  binding:"omitempty,min=2,max=100,slug"`
	Package       string `json:"package"         binding:"omitempty,oneof=starter growth enterprise"`
	ContactName   string `json:"contact_name"    binding:"required,min=1,max=200"`
	ContactEmail  string `json:"contact_email"   binding:"required,email,max=320"`
	ContactPhone  string `json:"contact_phone"   binding:"omitempty,min=5,max=30"`
}

// ApproveTenantRegistrationRequest is the JSON body for POST /admin/tenant-registrations/:id/approve.
// All fields are optional overrides.
type ApproveTenantRegistrationRequest struct {
	Package     *string `json:"package"      binding:"omitempty,oneof=starter growth enterprise"`
	MaxBranches *int    `json:"max_branches" binding:"omitempty,min=1"`
}

// RejectTenantRegistrationRequest is the JSON body for POST /admin/tenant-registrations/:id/reject.
type RejectTenantRegistrationRequest struct {
	Reason string `json:"reason" binding:"omitempty,max=1000"`
}

// ListRegistrationsQuery are query params for GET /admin/tenant-registrations.
type ListRegistrationsQuery struct {
	Status string `form:"status" binding:"omitempty,oneof=pending approved rejected all"`
	Cursor string `form:"cursor"`
	Limit  int    `form:"limit" binding:"omitempty,min=1,max=200"`
}

// ---------------------------------------------------------------------------
// Phase 3 — Tenant management
// ---------------------------------------------------------------------------

// ListTenantsQuery are query params for GET /admin/tenants.
type ListTenantsQuery struct {
	Status string `form:"status" binding:"omitempty,oneof=active suspended deactivated pending_approval all"`
	Cursor string `form:"cursor"`
	Limit  int    `form:"limit" binding:"omitempty,min=1,max=200"`
}

// ChangeTenantStatusRequest is the JSON body for PATCH /admin/tenants/:id/status.
// SECURITY.md Phase 3 M-2: DTO-layer allowlist matches valid tenant transitions.
type ChangeTenantStatusRequest struct {
	Status string `json:"status" binding:"required,oneof=active suspended deactivated"`
	Reason string `json:"reason" binding:"omitempty,max=1000"`
}

// ---------------------------------------------------------------------------
// Phase 3 — Branch management
// ---------------------------------------------------------------------------

// BranchRequest is the JSON body for POST /tenant/branches.
type BranchRequest struct {
	Name         string  `json:"name"          binding:"required,min=1,max=200"`
	Code         string  `json:"code"          binding:"required,min=1,max=50"`
	AddressLine1 *string `json:"address_line1" binding:"omitempty,max=500"`
	AddressLine2 *string `json:"address_line2" binding:"omitempty,max=500"`
	City         *string `json:"city"          binding:"omitempty,max=100"`
	Province     *string `json:"province"      binding:"omitempty,max=100"`
	PostalCode   *string `json:"postal_code"   binding:"omitempty,max=20"`
	Country      *string `json:"country"       binding:"omitempty,len=2"`
	Timezone     *string `json:"timezone"      binding:"omitempty,max=100"`
	ContactPhone *string `json:"contact_phone" binding:"omitempty,min=5,max=30"`
	ContactEmail *string `json:"contact_email" binding:"omitempty,email,max=320"`
}

// UpdateBranchRequest is the JSON body for PATCH /tenant/branches/:id.
// All fields are optional — only provided fields are updated.
type UpdateBranchRequest struct {
	Name         *string `json:"name"          binding:"omitempty,min=1,max=200"`
	AddressLine1 *string `json:"address_line1" binding:"omitempty,max=500"`
	AddressLine2 *string `json:"address_line2" binding:"omitempty,max=500"`
	City         *string `json:"city"          binding:"omitempty,max=100"`
	Province     *string `json:"province"      binding:"omitempty,max=100"`
	PostalCode   *string `json:"postal_code"   binding:"omitempty,max=20"`
	Country      *string `json:"country"       binding:"omitempty,len=2"`
	Timezone     *string `json:"timezone"      binding:"omitempty,max=100"`
	ContactPhone *string `json:"contact_phone" binding:"omitempty,min=5,max=30"`
	ContactEmail *string `json:"contact_email" binding:"omitempty,email,max=320"`
}

// ChangeBranchStatusRequest is the JSON body for PATCH /tenant/branches/:id/status.
type ChangeBranchStatusRequest struct {
	Status string `json:"status" binding:"required,oneof=active inactive"`
}

// ListBranchesQuery are query params for GET /tenant/branches.
type ListBranchesQuery struct {
	Status string `form:"status" binding:"omitempty,oneof=active inactive all"`
	Cursor string `form:"cursor"`
	Limit  int    `form:"limit" binding:"omitempty,min=1,max=200"`
}

// ---------------------------------------------------------------------------
// Phase 4 — Master Operational Data (ADR 0009).
// ---------------------------------------------------------------------------

// CreateTherapistRequest is the JSON body for POST /tenant/therapists.
type CreateTherapistRequest struct {
	BranchID  string  `json:"branch_id"  binding:"required,uuid"`
	FullName  string  `json:"full_name"  binding:"required,min=1,max=200"`
	Gender    *string `json:"gender"     binding:"omitempty,oneof=male female other"`
	Phone     *string `json:"phone"      binding:"omitempty,max=30"`
	Email     *string `json:"email"      binding:"omitempty,email,max=320"`
	Bio       *string `json:"bio"        binding:"omitempty,max=500"`
	PhotoURL  *string `json:"photo_url"  binding:"omitempty,url,max=2048"`
	JoinedAt  *string `json:"joined_at"  binding:"omitempty"`
	UserID    *string `json:"user_id"    binding:"omitempty,uuid"`
}

// UpdateTherapistRequest is the JSON body for PATCH /tenant/therapists/:id.
// All fields are optional — only provided fields are updated.
type UpdateTherapistRequest struct {
	FullName  *string `json:"full_name"  binding:"omitempty,min=1,max=200"`
	Gender    *string `json:"gender"     binding:"omitempty,oneof=male female other"`
	Phone     *string `json:"phone"      binding:"omitempty,max=30"`
	Email     *string `json:"email"      binding:"omitempty,email,max=320"`
	Bio       *string `json:"bio"        binding:"omitempty,max=500"`
	PhotoURL  *string `json:"photo_url"  binding:"omitempty,url,max=2048"`
	JoinedAt  *string `json:"joined_at"  binding:"omitempty"`
	UserID    *string `json:"user_id"    binding:"omitempty,uuid"`
}

// ChangeTherapistStatusRequest is the JSON body for PATCH /tenant/therapists/:id/status.
type ChangeTherapistStatusRequest struct {
	IsActive *bool `json:"is_active" binding:"required"`
}

// ListTherapistsQuery are query params for GET /tenant/therapists.
type ListTherapistsQuery struct {
	BranchID string `form:"branch_id" binding:"omitempty,uuid"`
	IsActive *bool  `form:"is_active"`
	Cursor   string `form:"cursor"`
	Limit    int    `form:"limit" binding:"omitempty,min=1,max=200"`
}

// CreateServiceRequest is the JSON body for POST /tenant/services.
type CreateServiceRequest struct {
	Name            string  `json:"name"             binding:"required,min=1,max=200"`
	Description     *string `json:"description"      binding:"omitempty,max=1000"`
	Category        *string `json:"category"         binding:"omitempty,max=100"`
	DurationMinutes int     `json:"duration_minutes" binding:"required,min=1,max=1440"`
	PriceIDR        int64   `json:"price_idr"        binding:"min=0"`
}

// UpdateServiceRequest is the JSON body for PATCH /tenant/services/:id.
// All fields are optional — only provided fields are updated.
type UpdateServiceRequest struct {
	Name            *string `json:"name"             binding:"omitempty,min=1,max=200"`
	Description     *string `json:"description"      binding:"omitempty,max=1000"`
	Category        *string `json:"category"         binding:"omitempty,max=100"`
	DurationMinutes *int    `json:"duration_minutes" binding:"omitempty,min=1,max=1440"`
	PriceIDR        *int64  `json:"price_idr"        binding:"omitempty,min=0"`
}

// ChangeServiceStatusRequest is the JSON body for PATCH /tenant/services/:id/status.
type ChangeServiceStatusRequest struct {
	IsActive *bool `json:"is_active" binding:"required"`
}

// ListServicesQuery are query params for GET /tenant/services.
type ListServicesQuery struct {
	IsActive *bool   `form:"is_active"`
	Category *string `form:"category"`
	Cursor   string  `form:"cursor"`
	Limit    int     `form:"limit" binding:"omitempty,min=1,max=200"`
}

// PutTherapistServicesRequest is the JSON body for PUT /tenant/therapists/:id/services.
type PutTherapistServicesRequest struct {
	ServiceIDs []string `json:"service_ids" binding:"required,max=200,dive,uuid"`
}

// AvailabilityWindowRequest is a single window in the PUT availability payload.
type AvailabilityWindowRequest struct {
	DOW   int    `json:"dow"   binding:"min=0,max=6"`
	Start string `json:"start" binding:"required"`
	End   string `json:"end"   binding:"required"`
}

// PutAvailabilityRequest is the JSON body for PUT /tenant/therapists/:id/availability.
type PutAvailabilityRequest struct {
	Windows []AvailabilityWindowRequest `json:"windows" binding:"required"`
}
