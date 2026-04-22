package controller

import (
	"net/http"

	"github.com/chrisanlung/lustia-auth/internal/constants"
	"github.com/chrisanlung/lustia-auth/internal/helper"
	"github.com/chrisanlung/lustia-auth/internal/middleware"
	"github.com/chrisanlung/lustia-auth/internal/service"
	"github.com/gin-gonic/gin"
)

// AdminController groups the admin-only user and role management endpoints.
type AdminController struct {
	users *service.UserService
	roles *service.RoleService
}

// NewAdminController constructs an AdminController.
func NewAdminController(users *service.UserService, roles *service.RoleService) *AdminController {
	return &AdminController{users: users, roles: roles}
}

// Register attaches admin routes to the provided router group.
// All routes require authentication (jwtMW) and a permission check (rbacMW).
func (h *AdminController) Register(rg *gin.RouterGroup, jwtMW gin.HandlerFunc, rbacMW func(code string) gin.HandlerFunc) {
	admin := rg.Group("/admin", jwtMW)

	// User management.
	admin.POST("/users", rbacMW(constants.PermUserCreate), h.handleCreateUser)
	admin.GET("/users", rbacMW(constants.PermUserRead), h.handleListUsers)
	admin.GET("/users/:id", rbacMW(constants.PermUserRead), h.handleGetUser)
	admin.PATCH("/users/:id", rbacMW(constants.PermUserUpdate), h.handleUpdateUser)
	admin.POST("/users/:id/unlock", rbacMW(constants.PermUserUpdate), h.handleUnlockUser)

	// Role management (read-only).
	admin.GET("/roles", rbacMW(constants.PermRoleRead), h.handleListRoles)
}

func (h *AdminController) handleCreateUser(c *gin.Context) {
	claims, ok := middleware.ClaimsFromContext(c)
	if !ok {
		helper.RespondError(c, http.StatusUnauthorized, constants.CodeUnauthenticated, "authentication required")
		return
	}

	var req CreateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		helper.RespondBindError(c, err)
		return
	}

	out, err := h.users.CreateUser(c.Request.Context(), service.CreateUserInput{
		CallerUserID:   claims.Subject,
		CallerTenantID: claims.TenantID,
		Email:          req.Email,
		FullName:       req.FullName,
		Phone:          req.Phone,
		RoleIDs:        req.RoleIDs,
		BranchIDs:      req.BranchIDs,
	})
	if err != nil {
		helper.RespondDomainError(c, err)
		return
	}

	prof := toUserProfileResponse(out.User)
	c.JSON(http.StatusCreated, CreateUserResponse{
		User:            prof,
		InitialPassword: out.InitialPassword,
	})
}

func (h *AdminController) handleListUsers(c *gin.Context) {
	claims, ok := middleware.ClaimsFromContext(c)
	if !ok {
		helper.RespondError(c, http.StatusUnauthorized, constants.CodeUnauthenticated, "authentication required")
		return
	}

	var q ListUsersQuery
	if err := c.ShouldBindQuery(&q); err != nil {
		helper.RespondBindError(c, err)
		return
	}

	var roleID, branchID *string
	if q.RoleID != "" {
		roleID = &q.RoleID
	}
	if q.BranchID != "" {
		branchID = &q.BranchID
	}

	out, err := h.users.ListUsers(c.Request.Context(), service.ListUsersInput{
		CallerTenantID: claims.TenantID,
		RoleID:         roleID,
		BranchID:       branchID,
		IsActive:       q.IsActive,
		Cursor:         q.Cursor,
		Limit:          q.Limit,
	})
	if err != nil {
		helper.RespondDomainError(c, err)
		return
	}

	profiles := make([]UserProfileResponse, len(out.Users))
	for i, p := range out.Users {
		profiles[i] = toUserProfileResponse(p)
	}

	c.JSON(http.StatusOK, ListUsersResponse{
		Data:       profiles,
		NextCursor: out.NextCursor,
	})
}

func (h *AdminController) handleGetUser(c *gin.Context) {
	claims, ok := middleware.ClaimsFromContext(c)
	if !ok {
		helper.RespondError(c, http.StatusUnauthorized, constants.CodeUnauthenticated, "authentication required")
		return
	}

	userID := c.Param("id")

	prof, err := h.users.GetUser(c.Request.Context(), service.GetUserInput{
		CallerTenantID: claims.TenantID,
		UserID:         userID,
	})
	if err != nil {
		helper.RespondDomainError(c, err)
		return
	}

	c.JSON(http.StatusOK, toUserProfileResponse(prof))
}

func (h *AdminController) handleUpdateUser(c *gin.Context) {
	claims, ok := middleware.ClaimsFromContext(c)
	if !ok {
		helper.RespondError(c, http.StatusUnauthorized, constants.CodeUnauthenticated, "authentication required")
		return
	}

	userID := c.Param("id")

	var req UpdateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		helper.RespondBindError(c, err)
		return
	}

	prof, err := h.users.UpdateUser(c.Request.Context(), service.UpdateUserInput{
		CallerUserID:   claims.Subject,
		CallerTenantID: claims.TenantID,
		TargetUserID:   userID,
		FullName:       req.FullName,
		Phone:          req.Phone,
		AvatarURL:      req.AvatarURL,
		IsActive:       req.IsActive,
		RoleIDs:        req.RoleIDs,
		BranchIDs:      req.BranchIDs,
	})
	if err != nil {
		helper.RespondDomainError(c, err)
		return
	}

	c.JSON(http.StatusOK, toUserProfileResponse(prof))
}

func (h *AdminController) handleUnlockUser(c *gin.Context) {
	claims, ok := middleware.ClaimsFromContext(c)
	if !ok {
		helper.RespondError(c, http.StatusUnauthorized, constants.CodeUnauthenticated, "authentication required")
		return
	}

	userID := c.Param("id")

	if err := h.users.UnlockUser(c.Request.Context(), service.UnlockUserInput{
		CallerUserID:   claims.Subject,
		CallerTenantID: claims.TenantID,
		TargetUserID:   userID,
	}); err != nil {
		helper.RespondDomainError(c, err)
		return
	}

	c.Status(http.StatusNoContent)
}

func (h *AdminController) handleListRoles(c *gin.Context) {
	out, err := h.roles.ListRoles(c.Request.Context())
	if err != nil {
		helper.RespondDomainError(c, err)
		return
	}

	roles := make([]RoleResponse, len(out.Roles))
	for i, r := range out.Roles {
		perms := make([]PermissionResponse, len(r.Permissions))
		for j, p := range r.Permissions {
			perms[j] = PermissionResponse{
				ID:          p.ID,
				Code:        p.Code,
				Description: p.Description,
			}
		}
		roles[i] = RoleResponse{
			ID:          r.ID,
			Name:        r.Name,
			Description: r.Description,
			Permissions: perms,
		}
	}

	c.JSON(http.StatusOK, ListRolesResponse{Data: roles})
}
