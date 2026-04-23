package controller

import (
	"net/http"

	"github.com/chrisanlung/lustia-auth/internal/constants"
	"github.com/chrisanlung/lustia-auth/internal/helper"
	"github.com/chrisanlung/lustia-auth/internal/middleware"
	"github.com/chrisanlung/lustia-auth/internal/service"
	"github.com/gin-gonic/gin"
)

// TenantController handles platform-admin tenant listing and status management.
type TenantController struct {
	svc *service.TenantService
}

// NewTenantController constructs a TenantController.
func NewTenantController(svc *service.TenantService) *TenantController {
	return &TenantController{svc: svc}
}

// Register attaches admin tenant routes under the provided (already-secured) group.
func (h *TenantController) Register(rg *gin.RouterGroup, rbacMW func(code string) gin.HandlerFunc) {
	admin := rg.Group("/admin/tenants")
	admin.GET("", rbacMW(constants.PermTenantRead), h.handleList)
	admin.GET("/:id", rbacMW(constants.PermTenantRead), h.handleGet)
	admin.PATCH("/:id/status", rbacMW(constants.PermTenantUpdate), h.handleChangeStatus)
}

// handleList handles GET /admin/tenants.
func (h *TenantController) handleList(c *gin.Context) {
	var q ListTenantsQuery
	if err := c.ShouldBindQuery(&q); err != nil {
		helper.RespondBindError(c, err)
		return
	}

	out, err := h.svc.List(c.Request.Context(), service.ListTenantsInput{
		Status: q.Status,
		Cursor: q.Cursor,
		Limit:  q.Limit,
	})
	if err != nil {
		helper.RespondDomainError(c, err)
		return
	}

	items := make([]TenantSummaryResponse, len(out.Tenants))
	for i, t := range out.Tenants {
		items[i] = toTenantSummaryResponse(t)
	}
	c.JSON(http.StatusOK, ListTenantsResponse{
		Data:       items,
		NextCursor: out.NextCursor,
	})
}

// handleGet handles GET /admin/tenants/:id.
func (h *TenantController) handleGet(c *gin.Context) {
	tenantID := c.Param("id")
	t, err := h.svc.GetTenant(c.Request.Context(), tenantID)
	if err != nil {
		helper.RespondDomainError(c, err)
		return
	}
	c.JSON(http.StatusOK, toTenantSummaryResponse(t))
}

// handleChangeStatus handles PATCH /admin/tenants/:id/status.
func (h *TenantController) handleChangeStatus(c *gin.Context) {
	claims, ok := middleware.ClaimsFromContext(c)
	if !ok {
		helper.RespondError(c, http.StatusUnauthorized, constants.CodeUnauthenticated, "authentication required")
		return
	}

	tenantID := c.Param("id")

	var req ChangeTenantStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		helper.RespondBindError(c, err)
		return
	}

	t, err := h.svc.TransitionStatus(c.Request.Context(), service.TransitionTenantStatusInput{
		TenantID:     tenantID,
		CallerUserID: claims.Subject,
		NewStatus:    req.Status,
		Reason:       req.Reason,
	})
	if err != nil {
		helper.RespondDomainError(c, err)
		return
	}

	c.JSON(http.StatusOK, toTenantSummaryResponse(t))
}

// ---------------------------------------------------------------------------
// Mapping helpers
// ---------------------------------------------------------------------------

func toTenantSummaryResponse(t service.TenantSummary) TenantSummaryResponse {
	return TenantSummaryResponse{
		ID:              t.ID,
		Name:            t.Name,
		Slug:            t.Slug,
		Status:          t.Status,
		Package:         t.Package,
		MaxBranches:     t.MaxBranches,
		ContactEmail:    t.ContactEmail,
		ContactName:     t.ContactName,
		ApprovedAt:      t.ApprovedAt,
		ApprovedBy:      t.ApprovedBy,
		RejectedAt:      t.RejectedAt,
		RejectionReason: t.RejectionReason,
		CreatedAt:       t.CreatedAt,
		MembershipCount: t.MembershipCount,
		BranchCount:     t.BranchCount,
	}
}
