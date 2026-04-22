package middleware

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/chrisanlung/lustia-auth/internal/constants"
	"github.com/chrisanlung/lustia-auth/internal/helper"
	"github.com/chrisanlung/lustia-auth/internal/model"
	"github.com/chrisanlung/lustia-auth/internal/repository"
	"github.com/gin-gonic/gin"
)

// RequestTxStarter is the minimum surface Tenant middleware needs from the
// persistence layer. repository.TxManager satisfies it.
//
// BeginForRequest opens a per-request transaction, injects it into the
// returned context, and returns a finisher used at end-of-request.
//
// SetTenantContext sets app.current_tenant + app.current_user as
// transaction-local GUC variables inside the tx that BeginForRequest opened.
type RequestTxStarter interface {
	BeginForRequest(ctx context.Context) (context.Context, repository.RequestTxFinisher, error)
	SetTenantContext(ctx context.Context, tenantID, userID string) error
}

// Tenant opens a transaction for every request, sets the PostgreSQL-side
// tenant context so RLS policies take effect, and finalises the tx on
// response.
//
// Tenant-context derivation rules (in priority order):
//
//  1. No claims present (public endpoint) → '__platform__' sentinel.
//     Public flows (login / refresh / forgot-password) switch the context
//     mid-flight via TxManager.SetTenantContext after resolving the user.
//
//  2. Claims present, scope=platform → '__platform__' (already in claim).
//
//  3. Claims present, scope=tenant → claim.TenantID (real UUID).
//
//  4. Claims present, scope=user → '__platform__'.
//     The user has authenticated but has not selected a tenant yet.
//     We set '__platform__' so that the membership table (which uses a policy
//     allowing users to always see their own rows) is still accessible for
//     the /auth/select-tenant and /auth/me endpoints.
//     NOTE: this is NOT the same as scope=platform — the ScopeGate middleware
//     blocks tenant-protected endpoints for scope=user callers.
//
// Commit happens on any response with status < 500. On 5xx the transaction
// is rolled back so half-applied writes do not persist.
func Tenant(tm RequestTxStarter) gin.HandlerFunc {
	return func(c *gin.Context) {
		var tenantID, userID string

		if claims, ok := ClaimsFromContext(c); ok {
			userID = claims.Subject
			switch claims.Scope {
			case model.ScopeUser:
				// scope=user: authenticated but no tenant selected yet.
				// Use __platform__ so self-referential RLS policies (membership
				// SELECT for the caller's own rows) still fire correctly.
				tenantID = constants.PlatformTenantSentinel
			case model.ScopePlatform:
				tenantID = constants.PlatformTenantSentinel
			default:
				// scope=tenant: claim carries the real tenant UUID.
				tenantID = claims.TenantID
			}
		} else {
			// Public endpoint — default sentinel; service code switches mid-flight.
			tenantID = constants.PlatformTenantSentinel
			userID = ""
		}

		baseCtx := c.Request.Context()
		txCtx, finisher, err := tm.BeginForRequest(baseCtx)
		if err != nil {
			slog.ErrorContext(baseCtx, "failed to begin request tx", "error", err)
			c.AbortWithStatusJSON(http.StatusInternalServerError, helper.ErrorResponse{
				Error: helper.ErrorDetail{
					Code:    constants.CodeInternal,
					Message: "failed to begin request transaction",
				},
			})
			return
		}

		if err := tm.SetTenantContext(txCtx, tenantID, userID); err != nil {
			_ = finisher.Rollback()
			slog.ErrorContext(txCtx, "failed to set tenant context", "error", err)
			c.AbortWithStatusJSON(http.StatusInternalServerError, helper.ErrorResponse{
				Error: helper.ErrorDetail{
					Code:    constants.CodeInternal,
					Message: "failed to establish tenant context",
				},
			})
			return
		}

		// Replace the request context so handlers + repositories pick up the tx.
		c.Request = c.Request.WithContext(txCtx)
		c.Set(constants.CtxKeyTenantID, tenantID)
		c.Set(constants.CtxKeyUserID, userID)

		c.Next()

		// Commit on 2xx/3xx/4xx (client-errors still reflect committed state
		// changes like failed_login_count). Rollback only on server-side faults.
		if c.Writer.Status() >= http.StatusInternalServerError {
			if err := finisher.Rollback(); err != nil {
				slog.ErrorContext(txCtx, "tx rollback failed", "error", err)
			}
			return
		}
		if err := finisher.Commit(); err != nil {
			slog.ErrorContext(txCtx, "tx commit failed", "error", err)
		}
	}
}
