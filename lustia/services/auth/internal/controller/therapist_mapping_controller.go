package controller

import (
	"net/http"

	"github.com/chrisanlung/lustia-auth/internal/constants"
	"github.com/chrisanlung/lustia-auth/internal/helper"
	"github.com/chrisanlung/lustia-auth/internal/middleware"
	"github.com/chrisanlung/lustia-auth/internal/service"
	"github.com/gin-gonic/gin"
)

// TherapistMappingController handles therapist ↔ service mapping endpoints.
type TherapistMappingController struct {
	svc         *service.MappingService
	therapistSvc *service.TherapistSvc
}

// NewTherapistMappingController constructs a TherapistMappingController.
func NewTherapistMappingController(
	svc *service.MappingService,
	therapistSvc *service.TherapistSvc,
) *TherapistMappingController {
	return &TherapistMappingController{svc: svc, therapistSvc: therapistSvc}
}

// Register attaches mapping routes under the provided secured group.
func (h *TherapistMappingController) Register(rg *gin.RouterGroup, rbacMW func(code string) gin.HandlerFunc) {
	tenant := rg.Group("/tenant")
	tenant.GET("/therapists/:id/services", rbacMW(constants.PermTherapistRead), h.handleGetMappings)
	tenant.PUT("/therapists/:id/services", rbacMW(constants.PermTherapistUpdate), h.handleReconcile)
}

func (h *TherapistMappingController) handleGetMappings(c *gin.Context) {
	claims, ok := middleware.ClaimsFromContext(c)
	if !ok {
		helper.RespondError(c, http.StatusUnauthorized, constants.CodeUnauthenticated, "authentication required")
		return
	}

	id := c.Param("id")
	out, err := h.therapistSvc.GetMappings(c.Request.Context(), claims.TenantID, id)
	if err != nil {
		helper.RespondDomainError(c, err)
		return
	}

	items := make([]TherapistServiceItemResponse, len(out.Services))
	for i, sv := range out.Services {
		items[i] = toTherapistServiceItemResponse(sv)
	}
	c.JSON(http.StatusOK, TherapistMappingResponse{
		TherapistID: out.TherapistID,
		Services:    items,
	})
}

func (h *TherapistMappingController) handleReconcile(c *gin.Context) {
	claims, ok := middleware.ClaimsFromContext(c)
	if !ok {
		helper.RespondError(c, http.StatusUnauthorized, constants.CodeUnauthenticated, "authentication required")
		return
	}

	id := c.Param("id")

	var req PutTherapistServicesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		helper.RespondBindError(c, err)
		return
	}

	out, err := h.svc.Reconcile(c.Request.Context(), service.ReconcileMappingInput{
		TherapistID:    id,
		CallerUserID:   claims.Subject,
		CallerTenantID: claims.TenantID,
		CallerBranches: claims.Branches,
		IsAdmin:        service.HasTenantAdminRole(claims.Roles),
		ServiceIDs:     req.ServiceIDs,
	})
	if err != nil {
		helper.RespondDomainError(c, err)
		return
	}

	items := make([]TherapistServiceItemResponse, len(out.Services))
	for i, sv := range out.Services {
		items[i] = toTherapistServiceItemResponse(sv)
	}
	c.JSON(http.StatusOK, TherapistMappingResponse{
		TherapistID: out.TherapistID,
		Services:    items,
	})
}
