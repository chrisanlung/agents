package service

import "time"

// ---------------------------------------------------------------------------
// Login
// ---------------------------------------------------------------------------

// LoginInput carries the credentials supplied by the caller.
// TenantSlug is removed in ADR 0007: login is now email+password only.
type LoginInput struct {
	Email     string
	Password  string
	UserAgent string
	IP        string
}

// MembershipSummary is a read-only projection of a membership with its tenant,
// roles, and branches. Used in login responses and /auth/me.
type MembershipSummary struct {
	MembershipID string
	TenantID     string
	TenantName   string
	TenantSlug   string
	Roles        []string
	Branches     []string
	Status       string
}

// LoginOutput carries the tokens returned after a successful login.
// Shape varies by Scope:
//   - scope=platform : super-admin, ActiveMembershipID=nil, Memberships=[]
//   - scope=tenant   : single active membership auto-selected, ActiveMembershipID set
//   - scope=user     : multiple memberships, no tenant selected yet, ActiveMembershipID=nil
type LoginOutput struct {
	AccessToken        string
	RefreshToken       string // opaque bearer; SHA-256 hash is stored in DB
	ExpiresAt          time.Time
	Scope              string
	User               UserProfile
	Memberships        []MembershipSummary
	ActiveMembershipID *string // nil for scope=platform and scope=user
}

// ---------------------------------------------------------------------------
// Refresh
// ---------------------------------------------------------------------------

// RefreshInput carries the opaque refresh token provided by the caller.
type RefreshInput struct {
	RefreshToken string
	UserAgent    string
	IP           string
}

// RefreshOutput carries the new token pair after successful rotation.
// Scope and ActiveMembershipID mirror the scope that was in effect when the
// refresh token was originally issued.
type RefreshOutput struct {
	AccessToken        string
	RefreshToken       string
	ExpiresAt          time.Time
	Scope              string
	ActiveMembershipID *string
}

// ---------------------------------------------------------------------------
// Logout
// ---------------------------------------------------------------------------

// LogoutInput carries the refresh token to revoke.
type LogoutInput struct {
	CallerUserID string
	RefreshToken string // opaque; may be empty (no-op)
}

// ---------------------------------------------------------------------------
// SwitchTenant
// ---------------------------------------------------------------------------

// SwitchTenantInput carries the data for POST /auth/select-tenant.
type SwitchTenantInput struct {
	CallerUserID string
	TenantID     string
	// OldRefreshToken is the refresh token to revoke during rotation.
	OldRefreshToken string
	UserAgent       string
	IP              string
}

// SwitchTenantOutput carries the fresh tokens issued after a tenant switch.
type SwitchTenantOutput struct {
	AccessToken        string
	RefreshToken       string
	ExpiresAt          time.Time
	Scope              string // always "tenant"
	ActiveMembershipID string
	Membership         MembershipSummary
}

// ---------------------------------------------------------------------------
// Me
// ---------------------------------------------------------------------------

// UserProfile is a read-only projection used in GetMe and LoginOutput.
// It carries only identity fields; roles and branches live on MembershipSummary.
type UserProfile struct {
	ID                 string
	Email              string
	FullName           string
	Phone              string
	AvatarURL          string
	IsActive           bool
	IsSuperAdmin       bool
	MustChangePassword bool
}

// GetMeOutput wraps the UserProfile with membership info.
// ActiveMembershipID is set when the caller's token has scope=tenant.
// Tenant is non-nil only when scope=tenant.
type GetMeOutput struct {
	User               UserProfile
	ActiveMembershipID *string
	Tenant             *TenantInfo // nil for scope=platform or scope=user
	Memberships        []MembershipSummary
}

// TenantInfo is a read-only projection of the tenant entity.
type TenantInfo struct {
	ID     string
	Name   string
	Slug   string
	Status string
}

// ---------------------------------------------------------------------------
// UpdateMe
// ---------------------------------------------------------------------------

// UpdateMeInput carries the fields the user may update on their own profile.
type UpdateMeInput struct {
	CallerUserID string
	FullName     *string
	Phone        *string
	AvatarURL    *string
}

// ---------------------------------------------------------------------------
// ChangePassword
// ---------------------------------------------------------------------------

// ChangePasswordInput carries old and new passwords for self-service change.
type ChangePasswordInput struct {
	CallerUserID string
	OldPassword  string
	NewPassword  string
}

// ---------------------------------------------------------------------------
// ForgotPassword / ResetPassword
// ---------------------------------------------------------------------------

// ForgotPasswordInput is the data needed to initiate a password reset.
// TenantSlug is removed in ADR 0007: lookup is now global by email.
// The response is always 204 — the service never signals whether the email
// was found (anti-enumeration).
type ForgotPasswordInput struct {
	Email string
	IP    string
}

// ResetPasswordInput consumes a reset token and sets a new password.
type ResetPasswordInput struct {
	Token       string // opaque bearer
	NewPassword string
}

// ---------------------------------------------------------------------------
// Admin — Users
// ---------------------------------------------------------------------------

// CreateUserInput carries the data for admin-initiated user creation.
type CreateUserInput struct {
	CallerUserID   string
	CallerTenantID string
	Email          string
	FullName       string
	Phone          string
	RoleIDs        []string
	BranchIDs      []string
	InitialPassword string // generated server-side; returned once in response
}

// CreateUserOutput carries the created/invited user and the generated password
// (if a new user row was created). CreatedUser and CreatedMembership signal
// which rows were actually inserted so the caller can return them accurately.
type CreateUserOutput struct {
	User              UserProfile
	InitialPassword   string // returned once; never stored in plaintext
	CreatedUser       bool   // true when a new "user" row was inserted
	CreatedMembership bool   // true when a new "membership" row was inserted
}

// ListUsersInput carries filters and pagination for user listing.
type ListUsersInput struct {
	CallerTenantID string
	RoleID         *string
	BranchID       *string
	IsActive       *bool
	Cursor         string
	Limit          int
}

// ListUsersOutput carries the page of users and the next cursor.
type ListUsersOutput struct {
	Users      []UserProfile
	NextCursor string
}

// GetUserInput identifies a single user to retrieve.
type GetUserInput struct {
	CallerTenantID string
	UserID         string
}

// UpdateUserInput carries fields for admin-initiated user update.
type UpdateUserInput struct {
	CallerUserID   string
	CallerTenantID string
	TargetUserID   string
	FullName       *string
	Phone          *string
	AvatarURL      *string
	IsActive       *bool
	RoleIDs        []string // nil = no change; empty = clear all
	BranchIDs      []string // nil = no change; empty = clear all
}

// UnlockUserInput identifies a user whose lockout should be cleared.
type UnlockUserInput struct {
	CallerUserID   string
	CallerTenantID string
	TargetUserID   string
}

// ---------------------------------------------------------------------------
// Admin — Roles
// ---------------------------------------------------------------------------

// ListRolesOutput carries all roles with their permissions.
type ListRolesOutput struct {
	Roles []RoleDetail
}

// RoleDetail is a read-only projection of a role with its permissions.
type RoleDetail struct {
	ID          string
	Name        string
	Description string
	Permissions []PermissionDetail
}

// PermissionDetail is a read-only projection of a single permission.
type PermissionDetail struct {
	ID          string
	Code        string
	Description string
}
