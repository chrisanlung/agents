package controller

import (
	"net/http"

	"github.com/chrisanlung/lustia-auth/internal/constants"
	"github.com/chrisanlung/lustia-auth/internal/helper"
	"github.com/chrisanlung/lustia-auth/internal/middleware"
	"github.com/chrisanlung/lustia-auth/internal/service"
	"github.com/gin-gonic/gin"
)

// TherapistController handles therapist CRUD endpoints under /tenant/therapists.
type TherapistController struct {
	svc *service.TherapistSvc
}

// NewTherapistController constructs a TherapistController.
func NewTherapistController(svc *service.TherapistSvc) *TherapistController {
	return &TherapistController{svc: svc}
}

// Register attaches all therapist routes under the provided secured group.
func (h *TherapistController) Register(rg *gin.RouterGroup, rbacMW func(code string) gin.HandlerFunc) {
	tenant := rg.Group("/tenant")
	tenant.POST("/therapists", rbacMW(constants.PermTherapistCreate), h.handleCreate)
	tenant.GET("/therapists", rbacMW(constants.PermTherapistRead), h.handleList)
	tenant.GET("/therapists/:id", rbacMW(constants.PermTherapistRead), h.handleGet)
	tenant.PATCH("/therapists/:id", rbacMW(constants.PermTherapistUpdate), h.handleUpdate)
	tenant.PATCH("/therapists/:id/status", rbacMW(constants.PermTherapistUpdate), h.handleChangeStatus)
	tenant.DELETE("/therapists/:id", rbacMW(constants.PermTherapistDelete), h.handleDelete)
}

func (h *TherapistController) handleCreate(c *gin.Context) {
	claims, ok := middleware.ClaimsFromContext(c)
	if !ok {
		helper.RespondError(c, http.StatusUnauthorized, constants.CodeUnauthenticated, "authentication required")
		return
	}

	var req CreateTherapistRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		helper.RespondBindError(c, err)
		return
	}

	out, err := h.svc.Create(c.Request.Context(), service.CreateTherapistInput{
		CallerUserID:   claims.Subject,
		CallerTenantID: claims.TenantID,
		CallerBranches: claims.Branches,
		IsAdmin:        service.HasTenantAdminRole(claims.Roles),
		BranchID:       req.BranchID,
		FullName:       req.FullName,
		Gender:         req.Gender,
		Phone:          req.Phone,
		Email:          req.Email,
		Bio:            req.Bio,
		PhotoURL:       req.PhotoURL,
		JoinedAt:       req.JoinedAt,
		UserID:         req.UserID,
	})
	if err != nil {
		helper.RespondDomainError(c, err)
		return
	}
	c.JSON(http.StatusCreated, toTherapistResponse(out))
}

func (h *TherapistController) handleList(c *gin.Context) {
	claims, ok := middleware.ClaimsFromContext(c)
	if !ok {
		helper.RespondError(c, http.StatusUnauthorized, constants.CodeUnauthenticated, "authentication required")
		return
	}

	var q ListTherapistsQuery
	if err := c.ShouldBindQuery(&q); err != nil {
		helper.RespondBindError(c, err)
		return
	}

	var branchID *string
	if q.BranchID != "" {
		branchID = &q.BranchID
	}

	out, err := h.svc.List(c.Request.Context(), service.ListTherapistsInput{
		CallerTenantID: claims.TenantID,
		CallerBranches: claims.Branches,
		IsAdmin:        service.HasTenantAdminRole(claims.Roles),
		BranchID:       branchID,
		IsActive:       q.IsActive,
		Cursor:         q.Cursor,
		Limit:          q.Limit,
	})
	if err != nil {
		helper.RespondDomainError(c, err)
		return
	}

	items := make([]TherapistResponse, len(out.Therapists))
	for i, t := range out.Therapists {
		items[i] = toTherapistResponse(t)
	}
	c.JSON(http.StatusOK, ListTherapistsResponse{Data: items, NextCursor: out.NextCursor})
}

func (h *TherapistController) handleGet(c *gin.Context) {
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
	c.JSON(http.StatusOK, toTherapistDetailResponse(out))
}

func (h *TherapistController) handleUpdate(c *gin.Context) {
	claims, ok := middleware.ClaimsFromContext(c)
	if !ok {
		helper.RespondError(c, http.StatusUnauthorized, constants.CodeUnauthenticated, "authentication required")
		return
	}

	id := c.Param("id")

	var req UpdateTherapistRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		helper.RespondBindError(c, err)
		return
	}

	out, err := h.svc.Update(c.Request.Context(), service.UpdateTherapistInput{
		TherapistID:    id,
		CallerUserID:   claims.Subject,
		CallerTenantID: claims.TenantID,
		CallerBranches: claims.Branches,
		IsAdmin:        service.HasTenantAdminRole(claims.Roles),
		FullName:       req.FullName,
		Gender:         req.Gender,
		Phone:          req.Phone,
		Email:          req.Email,
		Bio:            req.Bio,
		PhotoURL:       req.PhotoURL,
		JoinedAt:       req.JoinedAt,
		UserID:         req.UserID,
	})
	if err != nil {
		helper.RespondDomainError(c, err)
		return
	}
	c.JSON(http.StatusOK, toTherapistResponse(out))
}

func (h *TherapistController) handleChangeStatus(c *gin.Context) {
	claims, ok := middleware.ClaimsFromContext(c)
	if !ok {
		helper.RespondError(c, http.StatusUnauthorized, constants.CodeUnauthenticated, "authentication required")
		return
	}

	id := c.Param("id")

	var req ChangeTherapistStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		helper.RespondBindError(c, err)
		return
	}

	out, err := h.svc.ChangeStatus(c.Request.Context(), service.ChangeTherapistStatusInput{
		TherapistID:    id,
		CallerUserID:   claims.Subject,
		CallerTenantID: claims.TenantID,
		CallerBranches: claims.Branches,
		IsAdmin:        service.HasTenantAdminRole(claims.Roles),
		IsActive:       *req.IsActive,
	})
	if err != nil {
		helper.RespondDomainError(c, err)
		return
	}
	c.JSON(http.StatusOK, toTherapistResponse(out))
}

func (h *TherapistController) handleDelete(c *gin.Context) {
	claims, ok := middleware.ClaimsFromContext(c)
	if !ok {
		helper.RespondError(c, http.StatusUnauthorized, constants.CodeUnauthenticated, "authentication required")
		return
	}

	id := c.Param("id")
	if err := h.svc.SoftDelete(
		c.Request.Context(),
		claims.TenantID,
		claims.Subject,
		claims.Branches,
		service.HasTenantAdminRole(claims.Roles),
		id,
	); err != nil {
		helper.RespondDomainError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// ---------------------------------------------------------------------------
// Mapping helpers
// ---------------------------------------------------------------------------

func toTherapistResponse(d service.TherapistDetail) TherapistResponse {
	specialties := d.Specialties
	if specialties == nil {
		specialties = []string{}
	}
	return TherapistResponse{
		ID:          d.ID,
		TenantID:    d.TenantID,
		BranchID:    d.BranchID,
		UserID:      d.UserID,
		FullName:    d.FullName,
		Gender:      d.Gender,
		Bio:         d.Bio,
		PhotoURL:    d.PhotoURL,
		Specialties: specialties,
		IsActive:    d.IsActive,
		JoinedAt:    d.JoinedAt,
		CreatedAt:   d.CreatedAt,
		UpdatedAt:   d.UpdatedAt,
	}
}

func toTherapistDetailResponse(d service.TherapistDetailWithServices) TherapistDetailResponse {
	items := make([]TherapistServiceItemResponse, len(d.Services))
	for i, sv := range d.Services {
		items[i] = toTherapistServiceItemResponse(sv)
	}
	return TherapistDetailResponse{
		TherapistResponse: toTherapistResponse(d.TherapistDetail),
		Services:          items,
	}
}

func toTherapistServiceItemResponse(item service.TherapistServiceItem) TherapistServiceItemResponse {
	return TherapistServiceItemResponse{
		ServiceID:       item.ServiceID,
		Name:            item.Name,
		Category:        item.Category,
		DurationMinutes: item.DurationMinutes,
		PriceIDR:        item.PriceIDR,
		IsActive:        item.IsActive,
		AssignedAt:      item.AssignedAt,
	}
}
