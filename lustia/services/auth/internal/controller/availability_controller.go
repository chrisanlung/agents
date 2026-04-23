package controller

import (
	"net/http"

	"github.com/chrisanlung/lustia-auth/internal/constants"
	"github.com/chrisanlung/lustia-auth/internal/helper"
	"github.com/chrisanlung/lustia-auth/internal/middleware"
	"github.com/chrisanlung/lustia-auth/internal/service"
	"github.com/gin-gonic/gin"
)

// AvailabilityController handles per-therapist availability endpoints.
type AvailabilityController struct {
	svc *service.AvailabilitySvc
}

// NewAvailabilityController constructs an AvailabilityController.
func NewAvailabilityController(svc *service.AvailabilitySvc) *AvailabilityController {
	return &AvailabilityController{svc: svc}
}

// Register attaches availability routes under the provided secured group.
// The PUT endpoint requires both availability.create AND availability.update per
// §11.3 flag #4. The RBAC middleware checks a single permission per route; for
// the compound check we register the route under availability.create and perform
// the availability.update check inline in the handler.
func (h *AvailabilityController) Register(rg *gin.RouterGroup, rbacMW func(code string) gin.HandlerFunc) {
	tenant := rg.Group("/tenant")
	tenant.GET("/therapists/:id/availability", rbacMW(constants.PermAvailabilityRead), h.handleGet)
	// PUT requires both create+update; register under create, inline-check update.
	tenant.PUT("/therapists/:id/availability", rbacMW(constants.PermAvailabilityCreate), h.handleReplace)
}

func (h *AvailabilityController) handleGet(c *gin.Context) {
	claims, ok := middleware.ClaimsFromContext(c)
	if !ok {
		helper.RespondError(c, http.StatusUnauthorized, constants.CodeUnauthenticated, "authentication required")
		return
	}

	id := c.Param("id")
	out, err := h.svc.Get(c.Request.Context(), claims.TenantID, id)
	if err != nil {
		helper.RespondDomainError(c, err)
		return
	}
	c.JSON(http.StatusOK, toAvailabilityResponse(out))
}

func (h *AvailabilityController) handleReplace(c *gin.Context) {
	claims, ok := middleware.ClaimsFromContext(c)
	if !ok {
		helper.RespondError(c, http.StatusUnauthorized, constants.CodeUnauthenticated, "authentication required")
		return
	}

	// Compound permission check: both availability.create AND availability.update
	// required per §11.3 flag #4.
	hasUpdate := false
	for _, p := range claims.Permissions {
		if p == constants.PermAvailabilityUpdate {
			hasUpdate = true
			break
		}
	}
	if !hasUpdate {
		helper.RespondError(c, http.StatusForbidden, constants.CodeInsufficientPermission, "insufficient permission: availability.update required")
		return
	}

	id := c.Param("id")

	var req PutAvailabilityRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		helper.RespondBindError(c, err)
		return
	}

	windows := make([]service.ReplaceAvailabilityWindow, len(req.Windows))
	for i, w := range req.Windows {
		windows[i] = service.ReplaceAvailabilityWindow{
			DOW:   w.DOW,
			Start: w.Start,
			End:   w.End,
		}
	}

	out, err := h.svc.Replace(c.Request.Context(), service.ReplaceAvailabilityInput{
		TherapistID:    id,
		CallerUserID:   claims.Subject,
		CallerTenantID: claims.TenantID,
		CallerBranches: claims.Branches,
		IsAdmin:        service.HasTenantAdminRole(claims.Roles),
		Windows:        windows,
	})
	if err != nil {
		helper.RespondDomainError(c, err)
		return
	}
	c.JSON(http.StatusOK, toAvailabilityResponse(out))
}

// ---------------------------------------------------------------------------
// Mapping helpers
// ---------------------------------------------------------------------------

func toAvailabilityResponse(out service.AvailabilityOutput) AvailabilityResponse {
	windows := make([]AvailabilityWindowResponse, len(out.Windows))
	for i, w := range out.Windows {
		windows[i] = AvailabilityWindowResponse{
			ID:    w.ID,
			DOW:   w.DOW,
			Start: w.Start,
			End:   w.End,
		}
	}
	return AvailabilityResponse{
		TherapistID: out.TherapistID,
		Windows:     windows,
	}
}
