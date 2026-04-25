package controller

import (
	"net/http"

	"github.com/chrisanlung/lustia-auth/internal/constants"
	"github.com/chrisanlung/lustia-auth/internal/helper"
	"github.com/chrisanlung/lustia-auth/internal/middleware"
	"github.com/chrisanlung/lustia-auth/internal/service"
	"github.com/gin-gonic/gin"
)

// AddonController handles tenant-wide add-on catalog endpoints under
// /tenant/addons. All routes require scope=tenant and the relevant
// addon.* permission (ADR 0010 §4.2.2).
type AddonController struct {
	svc *service.AddonService
}

// NewAddonController constructs an AddonController.
func NewAddonController(svc *service.AddonService) *AddonController {
	return &AddonController{svc: svc}
}

// Register attaches all add-on routes to the provided secured group.
func (h *AddonController) Register(rg *gin.RouterGroup, rbacMW func(code string) gin.HandlerFunc) {
	g := rg.Group("/tenant/addons")
	g.GET("", rbacMW(constants.PermAddonRead), h.handleList)
	g.POST("", rbacMW(constants.PermAddonCreate), h.handleCreate)
	// PUT /reorder must be registered before /:id so Gin does not treat
	// "reorder" as an id parameter.
	g.PUT("/reorder", rbacMW(constants.PermAddonUpdate), h.handleReorder)
	g.GET("/:id", rbacMW(constants.PermAddonRead), h.handleGet)
	g.PATCH("/:id", rbacMW(constants.PermAddonUpdate), h.handleUpdate)
	g.PATCH("/:id/status", rbacMW(constants.PermAddonUpdate), h.handleChangeStatus)
	g.DELETE("/:id", rbacMW(constants.PermAddonDelete), h.handleDelete)
}

func (h *AddonController) handleList(c *gin.Context) {
	claims, ok := middleware.ClaimsFromContext(c)
	if !ok {
		helper.RespondError(c, http.StatusUnauthorized, constants.CodeUnauthenticated, "authentication required")
		return
	}

	var q ListAddonsQuery
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

	out, err := h.svc.List(c.Request.Context(), service.ListAddonsInput{
		CallerTenantID: claims.TenantID,
		IsActive:       q.IsActive,
		Page:           page,
		Limit:          limit,
	})
	if err != nil {
		helper.RespondDomainError(c, err)
		return
	}

	items := make([]AddonResponse, len(out.Addons))
	for i, a := range out.Addons {
		items[i] = toAddonResponse(a)
	}
	c.JSON(http.StatusOK, ListAddonsResponse{
		Data:       items,
		Page:       out.Page,
		Limit:      limit,
		TotalCount: out.TotalCount,
		TotalPages: out.TotalPages,
	})
}

func (h *AddonController) handleCreate(c *gin.Context) {
	claims, ok := middleware.ClaimsFromContext(c)
	if !ok {
		helper.RespondError(c, http.StatusUnauthorized, constants.CodeUnauthenticated, "authentication required")
		return
	}

	var req CreateAddonRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		helper.RespondBindError(c, err)
		return
	}

	out, err := h.svc.Create(c.Request.Context(), service.CreateAddonInput{
		CallerUserID:   claims.Subject,
		CallerTenantID: claims.TenantID,
		Name:           req.Name,
		Description:    req.Description,
		PriceIDR:       req.PriceIDR,
		SortOrder:      req.SortOrder,
	})
	if err != nil {
		helper.RespondDomainError(c, err)
		return
	}
	c.JSON(http.StatusCreated, toAddonResponse(out))
}

func (h *AddonController) handleGet(c *gin.Context) {
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
	c.JSON(http.StatusOK, toAddonResponse(out))
}

func (h *AddonController) handleUpdate(c *gin.Context) {
	claims, ok := middleware.ClaimsFromContext(c)
	if !ok {
		helper.RespondError(c, http.StatusUnauthorized, constants.CodeUnauthenticated, "authentication required")
		return
	}

	id := c.Param("id")

	var req UpdateAddonRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		helper.RespondBindError(c, err)
		return
	}

	out, err := h.svc.Update(c.Request.Context(), service.UpdateAddonInput{
		AddonID:        id,
		CallerUserID:   claims.Subject,
		CallerTenantID: claims.TenantID,
		Name:           req.Name,
		Description:    req.Description,
		PriceIDR:       req.PriceIDR,
		SortOrder:      req.SortOrder,
	})
	if err != nil {
		helper.RespondDomainError(c, err)
		return
	}
	c.JSON(http.StatusOK, toAddonResponse(out))
}

func (h *AddonController) handleChangeStatus(c *gin.Context) {
	claims, ok := middleware.ClaimsFromContext(c)
	if !ok {
		helper.RespondError(c, http.StatusUnauthorized, constants.CodeUnauthenticated, "authentication required")
		return
	}

	id := c.Param("id")

	var req ChangeAddonStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		helper.RespondBindError(c, err)
		return
	}

	out, err := h.svc.ChangeStatus(c.Request.Context(), service.ChangeAddonStatusInput{
		AddonID:        id,
		CallerUserID:   claims.Subject,
		CallerTenantID: claims.TenantID,
		IsActive:       *req.IsActive,
	})
	if err != nil {
		helper.RespondDomainError(c, err)
		return
	}
	c.JSON(http.StatusOK, toAddonResponse(out))
}

func (h *AddonController) handleDelete(c *gin.Context) {
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

func (h *AddonController) handleReorder(c *gin.Context) {
	claims, ok := middleware.ClaimsFromContext(c)
	if !ok {
		helper.RespondError(c, http.StatusUnauthorized, constants.CodeUnauthenticated, "authentication required")
		return
	}

	var req ReorderAddonsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		helper.RespondBindError(c, err)
		return
	}

	items := make([]service.AddonSortOrderItem, len(req.Items))
	for i, it := range req.Items {
		items[i] = service.AddonSortOrderItem{ID: it.ID, SortOrder: it.SortOrder}
	}

	if err := h.svc.Reorder(c.Request.Context(), service.ReorderAddonsInput{
		CallerUserID:   claims.Subject,
		CallerTenantID: claims.TenantID,
		Items:          items,
	}); err != nil {
		helper.RespondDomainError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// ---------------------------------------------------------------------------
// Mapping helper
// ---------------------------------------------------------------------------

func toAddonResponse(d service.AddonDetail) AddonResponse {
	return AddonResponse{
		ID:          d.ID,
		Name:        d.Name,
		Description: d.Description,
		PriceIDR:    d.PriceIDR,
		IsActive:    d.IsActive,
		SortOrder:   d.SortOrder,
		CreatedAt:   d.CreatedAt,
		UpdatedAt:   d.UpdatedAt,
	}
}
