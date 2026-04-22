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
