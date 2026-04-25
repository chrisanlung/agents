package controller

import (
	"net/http"

	"github.com/chrisanlung/lustia-auth/internal/constants"
	"github.com/chrisanlung/lustia-auth/internal/helper"
	"github.com/chrisanlung/lustia-auth/internal/middleware"
	"github.com/chrisanlung/lustia-auth/internal/service"
	"github.com/gin-gonic/gin"
)

// RegistrationController handles the public company-registration submission and
// the platform-admin approval/rejection workflow.
type RegistrationController struct {
	svc *service.RegistrationService
}

// NewRegistrationController constructs a RegistrationController.
func NewRegistrationController(svc *service.RegistrationService) *RegistrationController {
	return &RegistrationController{svc: svc}
}

// RegisterPublic attaches the public registration route (no JWT).
// rateLimitMW is applied at the group level to enforce 3/hour per IP.
func (h *RegistrationController) RegisterPublic(rg *gin.RouterGroup, rateLimitMW gin.HandlerFunc) {
	pub := rg.Group("/register", rateLimitMW)
	pub.POST("/company", h.handleSubmitRegistration)
}

// RegisterAdmin attaches admin routes under the provided (already-secured) group.
func (h *RegistrationController) RegisterAdmin(rg *gin.RouterGroup, rbacMW func(code string) gin.HandlerFunc) {
	admin := rg.Group("/admin/tenant-registrations")
	admin.GET("", rbacMW(constants.PermTenantApprove), h.handleListRegistrations)
	admin.GET("/:id", rbacMW(constants.PermTenantApprove), h.handleGetRegistration)
	admin.POST("/:id/approve", rbacMW(constants.PermTenantApprove), h.handleApprove)
	admin.POST("/:id/reject", rbacMW(constants.PermTenantApprove), h.handleReject)
}

// handleGetRegistration handles GET /admin/tenant-registrations/:id.
func (h *RegistrationController) handleGetRegistration(c *gin.Context) {
	id := c.Param("id")
	out, err := h.svc.GetByID(c.Request.Context(), id)
	if err != nil {
		helper.RespondDomainError(c, err)
		return
	}
	c.JSON(http.StatusOK, toRegistrationSummaryResponse(out))
}

// handleSubmitRegistration handles POST /register/company.
func (h *RegistrationController) handleSubmitRegistration(c *gin.Context) {
	var req CompanyRegistrationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		helper.RespondBindError(c, err)
		return
	}

	ip := c.ClientIP()
	out, err := h.svc.SubmitRegistration(c.Request.Context(), service.RegistrationInput{
		CompanyName:   req.CompanyName,
		RequestedSlug: req.RequestedSlug,
		Package:       req.Package,
		ContactName:   req.ContactName,
		ContactEmail:  req.ContactEmail,
		ContactPhone:  req.ContactPhone,
		IP:            ip,
	})
	if err != nil {
		helper.RespondDomainError(c, err)
		return
	}

	c.JSON(http.StatusCreated, CompanyRegistrationResponse{
		RegistrationID: out.RegistrationID,
		Status:         out.Status,
	})
}

// handleListRegistrations handles GET /admin/tenant-registrations.
func (h *RegistrationController) handleListRegistrations(c *gin.Context) {
	var q ListRegistrationsQuery
	if err := c.ShouldBindQuery(&q); err != nil {
		helper.RespondBindError(c, err)
		return
	}

	page := q.Page
	if page < 1 {
		page = 1
	}
	limit := q.Limit
	if limit < 1 {
		limit = 10
	}

	out, err := h.svc.ListPending(c.Request.Context(), service.ListRegistrationsInput{
		Status: q.Status,
		Page:   page,
		Limit:  limit,
	})
	if err != nil {
		helper.RespondDomainError(c, err)
		return
	}

	items := make([]RegistrationSummaryResponse, len(out.Registrations))
	for i, r := range out.Registrations {
		items[i] = toRegistrationSummaryResponse(r)
	}
	c.JSON(http.StatusOK, ListRegistrationsResponse{
		Data:       items,
		Page:       out.Page,
		Limit:      limit,
		TotalCount: out.TotalCount,
		TotalPages: out.TotalPages,
	})
}

// handleApprove handles POST /admin/tenant-registrations/:id/approve.
func (h *RegistrationController) handleApprove(c *gin.Context) {
	claims, ok := middleware.ClaimsFromContext(c)
	if !ok {
		helper.RespondError(c, http.StatusUnauthorized, constants.CodeUnauthenticated, "authentication required")
		return
	}

	registrationID := c.Param("id")

	var req ApproveTenantRegistrationRequest
	// Body is optional — all fields override.
	_ = c.ShouldBindJSON(&req)

	out, err := h.svc.Approve(c.Request.Context(), service.ApproveRegistrationInput{
		RegistrationID: registrationID,
		CallerUserID:   claims.Subject,
		Package:        req.Package,
		MaxBranches:    req.MaxBranches,
	})
	if err != nil {
		helper.RespondDomainError(c, err)
		return
	}

	c.JSON(http.StatusOK, ApproveTenantRegistrationResponse{
		Tenant: TenantDetailResponse{
			ID:          out.Tenant.ID,
			Name:        out.Tenant.Name,
			Slug:        out.Tenant.Slug,
			Status:      out.Tenant.Status,
			Package:     out.Tenant.Package,
			MaxBranches: out.Tenant.MaxBranches,
			ContactEmail: out.Tenant.ContactEmail,
			ContactName:  out.Tenant.ContactName,
			ApprovedAt:  out.Tenant.ApprovedAt,
			ApprovedBy:  out.Tenant.ApprovedBy,
			CreatedAt:   out.Tenant.CreatedAt,
		},
		TenantAdmin: TenantAdminResponse{
			UserID:            out.TenantAdmin.UserID,
			Email:             out.TenantAdmin.Email,
			TemporaryPassword: out.TenantAdmin.TemporaryPassword,
		},
		Registration: toRegistrationSummaryResponse(out.Registration),
	})
}

// handleReject handles POST /admin/tenant-registrations/:id/reject.
func (h *RegistrationController) handleReject(c *gin.Context) {
	claims, ok := middleware.ClaimsFromContext(c)
	if !ok {
		helper.RespondError(c, http.StatusUnauthorized, constants.CodeUnauthenticated, "authentication required")
		return
	}

	registrationID := c.Param("id")

	var req RejectTenantRegistrationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		helper.RespondBindError(c, err)
		return
	}

	out, err := h.svc.Reject(c.Request.Context(), service.RejectRegistrationInput{
		RegistrationID: registrationID,
		CallerUserID:   claims.Subject,
		Reason:         req.Reason,
	})
	if err != nil {
		helper.RespondDomainError(c, err)
		return
	}

	c.JSON(http.StatusOK, toRegistrationSummaryResponse(out.Registration))
}

// ---------------------------------------------------------------------------
// Mapping helpers
// ---------------------------------------------------------------------------

func toRegistrationSummaryResponse(r service.RegistrationDetail) RegistrationSummaryResponse {
	return RegistrationSummaryResponse{
		ID:              r.ID,
		CompanyName:     r.CompanyName,
		RequestedSlug:   r.RequestedSlug,
		Package:         r.Package,
		ContactName:     r.ContactName,
		ContactEmail:    r.ContactEmail,
		ContactPhone:    r.ContactPhone,
		Status:          r.Status,
		ApprovedAt:      r.ApprovedAt,
		ApprovedBy:      r.ApprovedBy,
		RejectedAt:      r.RejectedAt,
		RejectionReason: r.RejectionReason,
		CreatedAt:       r.CreatedAt,
	}
}
