package constants

// HTTP error codes returned in the error envelope Code field.
// These match the API contract defined in docs/API_CONTRACT.md §5.
const (
	CodeValidation             = "VALIDATION"
	CodeUnauthenticated        = "UNAUTHENTICATED"
	CodeInvalidCredentials     = "INVALID_CREDENTIALS"
	CodeTokenExpired           = "TOKEN_EXPIRED"
	CodeTokenInvalid           = "TOKEN_INVALID"
	CodeRefreshRevoked         = "REFRESH_REVOKED"
	CodeAccountLocked          = "ACCOUNT_LOCKED"
	CodeAccountInactive        = "ACCOUNT_INACTIVE"
	CodePasswordChangeRequired = "PASSWORD_CHANGE_REQUIRED"
	CodeForbidden              = "FORBIDDEN"
	CodeInsufficientPermission = "INSUFFICIENT_PERMISSION"
	CodeTenantInactive         = "TENANT_INACTIVE"
	CodeTenantNotFound         = "TENANT_NOT_FOUND"
	CodeTenantNotSelected      = "TENANT_NOT_SELECTED"
	CodeNotFound               = "NOT_FOUND"
	CodeConflict               = "CONFLICT"
	CodeDuplicateEmail         = "DUPLICATE_EMAIL"
	CodeUserAlreadyInTenant    = "CONFLICT_USER_ALREADY_IN_TENANT"
	CodeRateLimited            = "RATE_LIMITED"
	CodeInternal               = "INTERNAL"

	// Phase 3 — Tenant Onboarding & Branch Setup (ADR 0008 §2.5).
	CodeDuplicatePendingRegistration = "DUPLICATE_PENDING_REGISTRATION"
	CodeTenantSlugTaken              = "TENANT_SLUG_TAKEN"
	CodeRegistrationNotPending       = "REGISTRATION_NOT_PENDING"
	CodeInvalidStatusTransition      = "INVALID_STATUS_TRANSITION"
	CodeBranchLimitReached           = "BRANCH_LIMIT_REACHED"
)
