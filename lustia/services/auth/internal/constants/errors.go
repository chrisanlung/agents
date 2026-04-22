// Package constants defines sentinel errors, string codes, and other
// project-wide constants for the auth service. Zero external dependencies.
package constants

import "errors"

// Sentinel errors. Repositories translate infrastructure errors (e.g.
// gorm.ErrRecordNotFound, pgconn.PgError) into these at the boundary so that
// services never see framework-specific types.
var (
	// ErrInvalidCredentials is returned when email/password do not match.
	ErrInvalidCredentials = errors.New("invalid credentials")

	// ErrAccountLocked is returned when the user account is temporarily locked.
	ErrAccountLocked = errors.New("account locked")

	// ErrAccountInactive is returned when the user's is_active flag is false.
	ErrAccountInactive = errors.New("account inactive")

	// ErrTenantNotFound is returned when no tenant matches the given slug.
	ErrTenantNotFound = errors.New("tenant not found")

	// ErrTenantInactive is returned when the tenant's status is not 'active'.
	ErrTenantInactive = errors.New("tenant inactive")

	// ErrTenantNotSelected is returned when a caller with scope=user tries to
	// access a protected endpoint before calling POST /auth/select-tenant.
	ErrTenantNotSelected = errors.New("tenant not selected")

	// ErrUserNotFound is returned when a user lookup yields no result.
	ErrUserNotFound = errors.New("user not found")

	// ErrMembershipNotFound is returned when a membership lookup yields no result.
	ErrMembershipNotFound = errors.New("membership not found")

	// ErrUserAlreadyInTenant is returned when an admin tries to create or invite
	// a user who already has an active membership in the caller's tenant.
	ErrUserAlreadyInTenant = errors.New("user already has an active membership in this tenant")

	// ErrRefreshTokenNotFound is returned when the provided refresh token hash
	// does not exist in the store.
	ErrRefreshTokenNotFound = errors.New("refresh token not found")

	// ErrRefreshTokenRevoked is returned when a valid token row exists but
	// revoked_at is set.
	ErrRefreshTokenRevoked = errors.New("refresh token revoked")

	// ErrRefreshTokenExpired is returned when a valid token row exists but
	// expires_at is in the past.
	ErrRefreshTokenExpired = errors.New("refresh token expired")

	// ErrPasswordResetTokenNotFound is returned when the reset token hash has
	// no matching row.
	ErrPasswordResetTokenNotFound = errors.New("password reset token not found")

	// ErrPasswordResetTokenExpired is returned when the reset token has expired.
	ErrPasswordResetTokenExpired = errors.New("password reset token expired")

	// ErrPasswordResetTokenUsed is returned when the reset token has already
	// been consumed.
	ErrPasswordResetTokenUsed = errors.New("password reset token already used")

	// ErrPermissionDenied is returned when the caller lacks a required
	// permission code.
	ErrPermissionDenied = errors.New("permission denied")

	// ErrConflict is returned when a unique-constraint or exclusion-constraint
	// violation occurs (SQLSTATE 23505 / 23P01).
	ErrConflict = errors.New("conflict")

	// ErrDuplicateEmail is a specialisation of ErrConflict for the global
	// email unique index.
	ErrDuplicateEmail = errors.New("duplicate email")

	// ErrPasswordChangeRequired is returned when the user must change their
	// password before proceeding.
	ErrPasswordChangeRequired = errors.New("password change required")

	// ErrRateLimited is returned when a rate-limit bucket is exhausted.
	ErrRateLimited = errors.New("rate limited")

	// ErrInvalidInput is a generic validation error for service-layer checks.
	ErrInvalidInput = errors.New("invalid input")

	// ErrRoleNotFound is returned when a role ID does not resolve.
	ErrRoleNotFound = errors.New("role not found")

	// ErrBranchNotFound is returned when a branch ID does not resolve.
	ErrBranchNotFound = errors.New("branch not found")
)
