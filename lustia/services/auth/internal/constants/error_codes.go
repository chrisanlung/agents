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
	CodeUsernameInvalid        = "USERNAME_INVALID"
	CodeUsernameAlreadyTaken   = "USERNAME_ALREADY_TAKEN"
	CodeUserAlreadyInTenant    = "CONFLICT_USER_ALREADY_IN_TENANT"
	CodeRateLimited            = "RATE_LIMITED"
	CodeInternal               = "INTERNAL"

	// Phase 3 — Tenant Onboarding & Branch Setup (ADR 0008 §2.5).
	CodeDuplicatePendingRegistration = "DUPLICATE_PENDING_REGISTRATION"
	CodeTenantSlugTaken              = "TENANT_SLUG_TAKEN"
	CodeRegistrationNotPending       = "REGISTRATION_NOT_PENDING"
	CodeInvalidStatusTransition      = "INVALID_STATUS_TRANSITION"
	CodeBranchLimitReached           = "BRANCH_LIMIT_REACHED"

	// Phase 4 — Master Operational Data (ADR 0009 §11.1).
	CodeTherapistNotFound          = "THERAPIST_NOT_FOUND"
	CodeServiceNotFound            = "SERVICE_NOT_FOUND"
	CodeTherapistHasActiveBookings = "THERAPIST_HAS_ACTIVE_BOOKINGS"
	CodeAvailabilityOverlap        = "AVAILABILITY_OVERLAP"
	CodeCrossBranchForbidden       = "CROSS_BRANCH_FORBIDDEN"

	// ADR 0010 — Tenant-wide add-on catalog (rewritten 2026-04-24).
	CodeAddonNotFound      = "ADDON_NOT_FOUND"
	CodeDuplicateAddonName = "DUPLICATE_ADDON_NAME"

	// ADR 0012 — Room (Ruangan) catalog.
	CodeRoomNotFound      = "ROOM_NOT_FOUND"
	CodeDuplicateRoomName = "DUPLICATE_ROOM_NAME"
	CodeRoomBranchImmutable = "ROOM_BRANCH_IMMUTABLE"

	// ADR 0014 — Phase 5 Booking Engine.
	CodeBookingNotFound               = "BOOKING_NOT_FOUND"
	CodeBookingCodeInvalid            = "BOOKING_CODE_INVALID"
	CodeBookingSlotConflict           = "BOOKING_SLOT_CONFLICT"
	CodeBookingExpired                = "BOOKING_EXPIRED"
	CodeBookingInvalidStatusTransition = "BOOKING_INVALID_STATUS_TRANSITION"
	CodeNoTherapistAvailable          = "NO_THERAPIST_AVAILABLE"
	CodeNoRoomAvailable               = "NO_ROOM_AVAILABLE"
	CodeTherapistNotForService        = "THERAPIST_NOT_FOR_SERVICE"
	CodeBookingTherapistConflict      = "BOOKING_THERAPIST_CONFLICT"

	// ADR 0011 — Storage abstraction + therapist extended profile.
	CodeUploadQuotaExceeded    = "UPLOAD_QUOTA_EXCEEDED"
	CodeInvalidImageFormat     = "INVALID_IMAGE_FORMAT"
	CodeImageTooLarge          = "IMAGE_TOO_LARGE"
	CodeImageDimensionsTooLarge = "IMAGE_DIMENSIONS_TOO_LARGE"
)
