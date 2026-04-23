package helper

import (
	"errors"
	"net/http"

	"github.com/chrisanlung/lustia-auth/internal/constants"
	"github.com/gin-gonic/gin"
)

// ErrorDetail carries the structured error fields in the response body.
type ErrorDetail struct {
	Code    string      `json:"code"`
	Message string      `json:"message"`
	Details interface{} `json:"details,omitempty"`
}

// ErrorResponse is the standard error envelope used by all error responses.
type ErrorResponse struct {
	Error ErrorDetail `json:"error"`
}

// RespondError writes a structured error response at the given HTTP status.
func RespondError(c *gin.Context, status int, code, message string) {
	c.JSON(status, ErrorResponse{
		Error: ErrorDetail{Code: code, Message: message},
	})
}

// RespondBindError writes a 400 VALIDATION error for request binding failures.
func RespondBindError(c *gin.Context, err error) {
	RespondError(c, http.StatusBadRequest, constants.CodeValidation, err.Error())
}

// RespondDomainError maps a sentinel error to the appropriate HTTP status code
// and error code string. Unknown errors map to 500 INTERNAL.
// This is the single place that translates service-layer errors into HTTP
// responses — no switch statements in controllers.
func RespondDomainError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, constants.ErrInvalidCredentials):
		RespondError(c, http.StatusUnauthorized, constants.CodeInvalidCredentials, "invalid credentials")
	case errors.Is(err, constants.ErrAccountLocked):
		RespondError(c, http.StatusUnauthorized, constants.CodeAccountLocked, "account is temporarily locked")
	case errors.Is(err, constants.ErrAccountInactive):
		RespondError(c, http.StatusUnauthorized, constants.CodeAccountInactive, "account is inactive")
	case errors.Is(err, constants.ErrRefreshTokenRevoked):
		RespondError(c, http.StatusUnauthorized, constants.CodeRefreshRevoked, "refresh token has been revoked")
	case errors.Is(err, constants.ErrRefreshTokenExpired):
		RespondError(c, http.StatusUnauthorized, constants.CodeTokenExpired, "refresh token has expired")
	case errors.Is(err, constants.ErrRefreshTokenNotFound):
		RespondError(c, http.StatusUnauthorized, constants.CodeTokenInvalid, "refresh token not found")
	case errors.Is(err, constants.ErrPasswordChangeRequired):
		RespondError(c, http.StatusForbidden, constants.CodePasswordChangeRequired, "password change required")
	case errors.Is(err, constants.ErrPermissionDenied):
		RespondError(c, http.StatusForbidden, constants.CodeInsufficientPermission, "insufficient permission")
	case errors.Is(err, constants.ErrTenantNotFound):
		RespondError(c, http.StatusUnauthorized, constants.CodeTenantNotFound, "tenant not found")
	case errors.Is(err, constants.ErrTenantInactive):
		RespondError(c, http.StatusUnauthorized, constants.CodeTenantInactive, "tenant is not active")
	case errors.Is(err, constants.ErrUserNotFound):
		RespondError(c, http.StatusNotFound, constants.CodeNotFound, "user not found")
	case errors.Is(err, constants.ErrRoleNotFound):
		RespondError(c, http.StatusNotFound, constants.CodeNotFound, "role not found")
	case errors.Is(err, constants.ErrBranchNotFound):
		RespondError(c, http.StatusNotFound, constants.CodeNotFound, "branch not found")
	case errors.Is(err, constants.ErrTenantNotSelected):
		RespondError(c, http.StatusForbidden, constants.CodeTenantNotSelected, "tenant not selected — call POST /auth/select-tenant first")
	case errors.Is(err, constants.ErrUserAlreadyInTenant):
		RespondError(c, http.StatusConflict, constants.CodeUserAlreadyInTenant, "user already has an active membership in this tenant")
	case errors.Is(err, constants.ErrMembershipNotFound):
		RespondError(c, http.StatusForbidden, constants.CodeForbidden, "membership not found or not active")
	case errors.Is(err, constants.ErrDuplicateEmail):
		RespondError(c, http.StatusConflict, constants.CodeDuplicateEmail, "email already exists")
	case errors.Is(err, constants.ErrConflict):
		RespondError(c, http.StatusConflict, constants.CodeConflict, "resource conflict")
	case errors.Is(err, constants.ErrRateLimited):
		RespondError(c, http.StatusTooManyRequests, constants.CodeRateLimited, "too many requests")
	case errors.Is(err, constants.ErrPasswordResetTokenNotFound),
		errors.Is(err, constants.ErrPasswordResetTokenExpired),
		errors.Is(err, constants.ErrPasswordResetTokenUsed):
		// Intentionally vague — anti-enumeration.
		RespondError(c, http.StatusUnprocessableEntity, constants.CodeTokenInvalid, "invalid or expired reset token")

	// Phase 3 — Tenant Onboarding & Branch Setup (ADR 0008 §2.5).
	case errors.Is(err, constants.ErrDuplicatePendingRegistration):
		RespondError(c, http.StatusConflict, constants.CodeDuplicatePendingRegistration, "a pending registration already exists for this email or slug")
	case errors.Is(err, constants.ErrTenantSlugTaken):
		RespondError(c, http.StatusConflict, constants.CodeTenantSlugTaken, "the requested slug is already taken by an existing tenant")
	case errors.Is(err, constants.ErrRegistrationNotPending):
		RespondError(c, http.StatusConflict, constants.CodeRegistrationNotPending, "registration is not in pending state")
	case errors.Is(err, constants.ErrInvalidStatusTransition):
		RespondError(c, http.StatusConflict, constants.CodeInvalidStatusTransition, "the requested status transition is not allowed")
	case errors.Is(err, constants.ErrBranchLimitReached):
		RespondError(c, http.StatusConflict, constants.CodeBranchLimitReached, "branch limit reached for this tenant's package")
	case errors.Is(err, constants.ErrRegistrationNotFound):
		RespondError(c, http.StatusNotFound, constants.CodeNotFound, "registration not found")

	// Phase 4 — Master Operational Data (ADR 0009 §11.1).
	case errors.Is(err, constants.ErrTherapistNotFound):
		RespondError(c, http.StatusNotFound, constants.CodeTherapistNotFound, "therapist not found")
	case errors.Is(err, constants.ErrServiceNotFound):
		RespondError(c, http.StatusNotFound, constants.CodeServiceNotFound, "service not found")
	case errors.Is(err, constants.ErrTherapistHasActiveBookings):
		RespondError(c, http.StatusConflict, constants.CodeTherapistHasActiveBookings, "therapist has active bookings and cannot be deleted")
	case errors.Is(err, constants.ErrAvailabilityOverlap):
		RespondError(c, http.StatusConflict, constants.CodeAvailabilityOverlap, "availability windows overlap on the same day")
	case errors.Is(err, constants.ErrCrossBranchForbidden):
		RespondError(c, http.StatusForbidden, constants.CodeCrossBranchForbidden, "cross-branch access is not permitted for branch_admin callers")

	// Generic service-layer validation error — 400 VALIDATION with the
	// service's message so the caller sees what went wrong.
	case errors.Is(err, constants.ErrInvalidInput):
		RespondError(c, http.StatusBadRequest, constants.CodeValidation, err.Error())

	default:
		RespondError(c, http.StatusInternalServerError, constants.CodeInternal, "an internal error occurred")
	}
}
