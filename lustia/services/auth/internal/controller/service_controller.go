package controller

import (
	"net/http"

	"github.com/chrisanlung/lustia-auth/internal/constants"
	"github.com/chrisanlung/lustia-auth/internal/helper"
	"github.com/chrisanlung/lustia-auth/internal/middleware"
	"github.com/chrisanlung/lustia-auth/internal/service"
	"github.com/gin-gonic/gin"
)

// ServiceController handles service-catalog endpoints under /tenant/services.
type ServiceController struct {
	svc *service.CatalogService
}

// NewServiceController constructs a ServiceController.
func NewServiceController(svc *service.CatalogService) *ServiceController {
	return &ServiceController{svc: svc}
}

// Register attaches all service routes under the provided secured group.
func (h *ServiceController) Register(rg *gin.RouterGroup, rbacMW func(code string) gin.HandlerFunc) {
	tenant := rg.Group("/tenant")
	tenant.POST("/services", rbacMW(constants.PermServiceCreate), h.handleCreate)
	tenant.GET("/services", rbacMW(constants.PermServiceRead), h.handleList)
	tenant.GET("/services/:id", rbacMW(constants.PermServiceRead), h.handleGet)
	tenant.PATCH("/services/:id", rbacMW(constants.PermServiceUpdate), h.handleUpdate)
	tenant.PATCH("/services/:id/status", rbacMW(constants.PermServiceUpdate), h.handleChangeStatus)
	tenant.DELETE("/services/:id", rbacMW(constants.PermServiceDelete), h.handleDelete)
}

func (h *ServiceController) handleCreate(c *gin.Context) {
	claims, ok := middleware.ClaimsFromContext(c)
	if !ok {
		helper.RespondError(c, http.StatusUnauthorized, constants.CodeUnauthenticated, "authentication required")
		return
	}

	var req CreateServiceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		helper.RespondBindError(c, err)
		return
	}

	out, err := h.svc.Create(c.Request.Context(), service.CreateServiceInput{
		CallerUserID:    claims.Subject,
		CallerTenantID:  claims.TenantID,
		Name:            req.Name,
		Description:     req.Description,
		Category:        req.Category,
		DurationMinutes: req.DurationMinutes,
		PriceIDR:        req.PriceIDR,
	})
	if err != nil {
		helper.RespondDomainError(c, err)
		return
	}
	c.JSON(http.StatusCreated, toServiceResponse(out))
}

func (h *ServiceController) handleList(c *gin.Context) {
	claims, ok := middleware.ClaimsFromContext(c)
	if !ok {
		helper.RespondError(c, http.StatusUnauthorized, constants.CodeUnauthenticated, "authentication required")
		return
	}

	var q ListServicesQuery
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

	out, err := h.svc.List(c.Request.Context(), service.ListServicesInput{
		CallerTenantID: claims.TenantID,
		IsActive:       q.IsActive,
		Category:       q.Category,
		Page:           page,
		Limit:          limit,
	})
	if err != nil {
		helper.RespondDomainError(c, err)
		return
	}

	items := make([]ServiceResponse, len(out.Services))
	for i, sv := range out.Services {
		items[i] = toServiceResponse(sv)
	}
	c.JSON(http.StatusOK, ListServicesResponse{
		Data:       items,
		Page:       out.Page,
		Limit:      limit,
		TotalCount: out.TotalCount,
		TotalPages: out.TotalPages,
	})
}

func (h *ServiceController) handleGet(c *gin.Context) {
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
	c.JSON(http.StatusOK, toServiceDetailResponse(out))
}

func (h *ServiceController) handleUpdate(c *gin.Context) {
	claims, ok := middleware.ClaimsFromContext(c)
	if !ok {
		helper.RespondError(c, http.StatusUnauthorized, constants.CodeUnauthenticated, "authentication required")
		return
	}

	id := c.Param("id")

	var req UpdateServiceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		helper.RespondBindError(c, err)
		return
	}

	out, err := h.svc.Update(c.Request.Context(), service.UpdateServiceInput{
		ServiceID:       id,
		CallerUserID:    claims.Subject,
		CallerTenantID:  claims.TenantID,
		Name:            req.Name,
		Description:     req.Description,
		Category:        req.Category,
		DurationMinutes: req.DurationMinutes,
		PriceIDR:        req.PriceIDR,
	})
	if err != nil {
		helper.RespondDomainError(c, err)
		return
	}
	c.JSON(http.StatusOK, toServiceResponse(out))
}

func (h *ServiceController) handleChangeStatus(c *gin.Context) {
	claims, ok := middleware.ClaimsFromContext(c)
	if !ok {
		helper.RespondError(c, http.StatusUnauthorized, constants.CodeUnauthenticated, "authentication required")
		return
	}

	id := c.Param("id")

	var req ChangeServiceStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		helper.RespondBindError(c, err)
		return
	}

	out, err := h.svc.ChangeStatus(c.Request.Context(), service.ChangeServiceStatusInput{
		ServiceID:      id,
		CallerUserID:   claims.Subject,
		CallerTenantID: claims.TenantID,
		IsActive:       *req.IsActive,
	})
	if err != nil {
		helper.RespondDomainError(c, err)
		return
	}
	c.JSON(http.StatusOK, toServiceResponse(out))
}

func (h *ServiceController) handleDelete(c *gin.Context) {
	claims, ok := middleware.ClaimsFromContext(c)
	if !ok {
		helper.RespondError(c, http.StatusUnauthorized, constants.CodeUnauthenticated, "authentication required")
		return
	}

	id := c.Param("id")
	if err := h.svc.SoftDelete(c.Request.Context(), claims.TenantID, claims.Subject, id); err != nil {
		helper.RespondDomainError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// ---------------------------------------------------------------------------
// Mapping helpers
// ---------------------------------------------------------------------------

func toServiceResponse(d service.ServiceDetail) ServiceResponse {
	return ServiceResponse{
		ID:              d.ID,
		TenantID:        d.TenantID,
		Name:            d.Name,
		Description:     d.Description,
		Category:        d.Category,
		DurationMinutes: d.DurationMinutes,
		PriceIDR:        d.PriceIDR,
		Currency:        d.Currency,
		IsActive:        d.IsActive,
		CreatedAt:       d.CreatedAt,
		UpdatedAt:       d.UpdatedAt,
	}
}

func toServiceDetailResponse(d service.ServiceDetailWithTherapists) ServiceDetailResponse {
	therapists := make([]ServiceTherapistItemResponse, len(d.Therapists))
	for i, t := range d.Therapists {
		therapists[i] = ServiceTherapistItemResponse{
			TherapistID: t.TherapistID,
			FullName:    t.FullName,
			BranchID:    t.BranchID,
			BranchName:  t.BranchName,
			IsActive:    t.IsActive,
		}
	}
	return ServiceDetailResponse{
		ServiceResponse: toServiceResponse(d.ServiceDetail),
		Therapists:      therapists,
	}
}
