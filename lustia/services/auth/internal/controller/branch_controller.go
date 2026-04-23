package controller

import (
	"net/http"

	"github.com/chrisanlung/lustia-auth/internal/constants"
	"github.com/chrisanlung/lustia-auth/internal/helper"
	"github.com/chrisanlung/lustia-auth/internal/middleware"
	"github.com/chrisanlung/lustia-auth/internal/service"
	"github.com/gin-gonic/gin"
)

// BranchController handles tenant-scoped branch CRUD and onboarding state.
type BranchController struct {
	svc *service.BranchService
}

// NewBranchController constructs a BranchController.
func NewBranchController(svc *service.BranchService) *BranchController {
	return &BranchController{svc: svc}
}

// Register attaches branch routes under the provided (already-secured) group.
// All routes require RBAC enforced at route level via rbacMW.
func (h *BranchController) Register(rg *gin.RouterGroup, rbacMW func(code string) gin.HandlerFunc) {
	tenant := rg.Group("/tenant")
	tenant.POST("/branches", rbacMW(constants.PermBranchCreate), h.handleCreate)
	tenant.GET("/branches", rbacMW(constants.PermBranchRead), h.handleList)
	tenant.GET("/branches/:id", rbacMW(constants.PermBranchRead), h.handleGet)
	tenant.PATCH("/branches/:id", rbacMW(constants.PermBranchUpdate), h.handleUpdate)
	tenant.PATCH("/branches/:id/status", rbacMW(constants.PermBranchUpdate), h.handleChangeStatus)
	tenant.DELETE("/branches/:id", rbacMW(constants.PermBranchDelete), h.handleDelete)
	tenant.GET("/onboarding-state", rbacMW(constants.PermBranchRead), h.handleOnboardingState)
}

func (h *BranchController) handleCreate(c *gin.Context) {
	claims, ok := middleware.ClaimsFromContext(c)
	if !ok {
		helper.RespondError(c, http.StatusUnauthorized, constants.CodeUnauthenticated, "authentication required")
		return
	}

	var req BranchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		helper.RespondBindError(c, err)
		return
	}

	out, err := h.svc.Create(c.Request.Context(), service.CreateBranchInput{
		CallerUserID:   claims.Subject,
		CallerTenantID: claims.TenantID,
		Name:           req.Name,
		Code:           req.Code,
		AddressLine1:   req.AddressLine1,
		AddressLine2:   req.AddressLine2,
		City:           req.City,
		Province:       req.Province,
		PostalCode:     req.PostalCode,
		Country:        helper.DerefString(req.Country),
		Timezone:       helper.DerefString(req.Timezone),
		ContactPhone:   req.ContactPhone,
		ContactEmail:   req.ContactEmail,
	})
	if err != nil {
		helper.RespondDomainError(c, err)
		return
	}
	c.JSON(http.StatusCreated, toBranchResponse(out))
}

func (h *BranchController) handleList(c *gin.Context) {
	claims, ok := middleware.ClaimsFromContext(c)
	if !ok {
		helper.RespondError(c, http.StatusUnauthorized, constants.CodeUnauthenticated, "authentication required")
		return
	}

	var q ListBranchesQuery
	if err := c.ShouldBindQuery(&q); err != nil {
		helper.RespondBindError(c, err)
		return
	}

	out, err := h.svc.ListByTenant(c.Request.Context(), service.ListBranchesInput{
		CallerTenantID: claims.TenantID,
		Status:         q.Status,
		Cursor:         q.Cursor,
		Limit:          q.Limit,
	})
	if err != nil {
		helper.RespondDomainError(c, err)
		return
	}

	items := make([]BranchResponse, len(out.Branches))
	for i, b := range out.Branches {
		items[i] = toBranchResponse(b)
	}
	c.JSON(http.StatusOK, ListBranchesResponse{
		Data:       items,
		NextCursor: out.NextCursor,
	})
}

func (h *BranchController) handleGet(c *gin.Context) {
	claims, ok := middleware.ClaimsFromContext(c)
	if !ok {
		helper.RespondError(c, http.StatusUnauthorized, constants.CodeUnauthenticated, "authentication required")
		return
	}

	branchID := c.Param("id")
	out, err := h.svc.GetBranch(c.Request.Context(), claims.TenantID, branchID)
	if err != nil {
		helper.RespondDomainError(c, err)
		return
	}
	c.JSON(http.StatusOK, toBranchResponse(out))
}

func (h *BranchController) handleUpdate(c *gin.Context) {
	claims, ok := middleware.ClaimsFromContext(c)
	if !ok {
		helper.RespondError(c, http.StatusUnauthorized, constants.CodeUnauthenticated, "authentication required")
		return
	}

	branchID := c.Param("id")

	var req UpdateBranchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		helper.RespondBindError(c, err)
		return
	}

	out, err := h.svc.UpdateBranch(c.Request.Context(), service.UpdateBranchInput{
		BranchID:       branchID,
		CallerUserID:   claims.Subject,
		CallerTenantID: claims.TenantID,
		Name:           req.Name,
		AddressLine1:   req.AddressLine1,
		AddressLine2:   req.AddressLine2,
		City:           req.City,
		Province:       req.Province,
		PostalCode:     req.PostalCode,
		Country:        req.Country,
		Timezone:       req.Timezone,
		ContactPhone:   req.ContactPhone,
		ContactEmail:   req.ContactEmail,
	})
	if err != nil {
		helper.RespondDomainError(c, err)
		return
	}
	c.JSON(http.StatusOK, toBranchResponse(out))
}

func (h *BranchController) handleChangeStatus(c *gin.Context) {
	claims, ok := middleware.ClaimsFromContext(c)
	if !ok {
		helper.RespondError(c, http.StatusUnauthorized, constants.CodeUnauthenticated, "authentication required")
		return
	}

	branchID := c.Param("id")

	var req ChangeBranchStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		helper.RespondBindError(c, err)
		return
	}

	out, err := h.svc.ChangeStatus(c.Request.Context(), service.ChangeBranchStatusInput{
		BranchID:       branchID,
		CallerUserID:   claims.Subject,
		CallerTenantID: claims.TenantID,
		NewStatus:      req.Status,
	})
	if err != nil {
		helper.RespondDomainError(c, err)
		return
	}
	c.JSON(http.StatusOK, toBranchResponse(out))
}

func (h *BranchController) handleDelete(c *gin.Context) {
	claims, ok := middleware.ClaimsFromContext(c)
	if !ok {
		helper.RespondError(c, http.StatusUnauthorized, constants.CodeUnauthenticated, "authentication required")
		return
	}

	branchID := c.Param("id")
	if err := h.svc.DeleteBranch(c.Request.Context(), claims.TenantID, claims.Subject, branchID); err != nil {
		helper.RespondDomainError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *BranchController) handleOnboardingState(c *gin.Context) {
	claims, ok := middleware.ClaimsFromContext(c)
	if !ok {
		helper.RespondError(c, http.StatusUnauthorized, constants.CodeUnauthenticated, "authentication required")
		return
	}

	out, err := h.svc.GetOnboardingState(c.Request.Context(), claims.TenantID, claims.Subject)
	if err != nil {
		helper.RespondDomainError(c, err)
		return
	}
	c.JSON(http.StatusOK, OnboardingStateResponse{
		HasBranches:        out.HasBranches,
		ActiveBranchCount:  out.ActiveBranchCount,
		MaxBranches:        out.MaxBranches,
		MustChangePassword: out.MustChangePassword,
	})
}

// ---------------------------------------------------------------------------
// Mapping helpers
// ---------------------------------------------------------------------------

func toBranchResponse(b service.BranchDetail) BranchResponse {
	return BranchResponse{
		ID:           b.ID,
		TenantID:     b.TenantID,
		Name:         b.Name,
		Code:         b.Code,
		Status:       b.Status,
		AddressLine1: b.AddressLine1,
		AddressLine2: b.AddressLine2,
		City:         b.City,
		Province:     b.Province,
		PostalCode:   b.PostalCode,
		Country:      b.Country,
		Timezone:     b.Timezone,
		ContactPhone: b.ContactPhone,
		ContactEmail: b.ContactEmail,
		ActivatedAt:  b.ActivatedAt,
		CreatedAt:    b.CreatedAt,
		UpdatedAt:    b.UpdatedAt,
	}
}
