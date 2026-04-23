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

	// Phase 3 — Tenant Onboarding & Branch Setup (ADR 0008 §2.5).

	// ErrDuplicatePendingRegistration is returned when the same email or slug
	// already has a pending registration.
	ErrDuplicatePendingRegistration = errors.New("duplicate pending registration")

	// ErrTenantSlugTaken is returned when the requested slug belongs to an
	// already-approved tenant.
	ErrTenantSlugTaken = errors.New("tenant slug already taken")

	// ErrRegistrationNotPending is returned when an approve/reject action is
	// attempted on a registration that is not in the 'pending' state.
	ErrRegistrationNotPending = errors.New("registration is not in pending state")

	// ErrInvalidStatusTransition is returned when a requested status change does
	// not match the allowed state machine transitions.
	ErrInvalidStatusTransition = errors.New("invalid status transition")

	// ErrBranchLimitReached is returned when creating a new branch would exceed
	// the tenant's max_branches limit.
	ErrBranchLimitReached = errors.New("branch limit reached for this tenant")

	// ErrRegistrationNotFound is returned when a registration ID does not resolve.
	ErrRegistrationNotFound = errors.New("registration not found")

	// Phase 4 — Master Operational Data (ADR 0009).

	// ErrTherapistNotFound is returned when a therapist ID does not resolve, is
	// soft-deleted, or belongs to a different tenant.
	ErrTherapistNotFound = errors.New("therapist not found")

	// ErrServiceNotFound is returned when a service ID does not resolve, is
	// soft-deleted, or belongs to a different tenant.
	ErrServiceNotFound = errors.New("service not found")

	// ErrTherapistHasActiveBookings is returned when a soft-delete is attempted
	// on a therapist with future non-terminal bookings (enforced in Phase 5).
	ErrTherapistHasActiveBookings = errors.New("therapist has active bookings")

	// ErrAvailabilityOverlap is returned when two availability windows on the
	// same day overlap for the same therapist.
	ErrAvailabilityOverlap = errors.New("availability windows overlap")

	// ErrCrossBranchForbidden is returned when a branch_admin attempts to
	// create or mutate a therapist or availability window that belongs to a
	// branch not present in their JWT branches claim.
	ErrCrossBranchForbidden = errors.New("cross-branch access forbidden")
)
