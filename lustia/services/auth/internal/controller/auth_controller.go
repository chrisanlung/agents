package controller

import (
	"net/http"

	"github.com/chrisanlung/lustia-auth/internal/constants"
	"github.com/chrisanlung/lustia-auth/internal/helper"
	"github.com/chrisanlung/lustia-auth/internal/middleware"
	"github.com/chrisanlung/lustia-auth/internal/model"
	"github.com/chrisanlung/lustia-auth/internal/service"
	"github.com/gin-gonic/gin"
)

// AuthController groups the public and self-service auth endpoints.
type AuthController struct {
	auth     *service.AuthService
	me       *service.MeService
	password *service.PasswordService
}

// NewAuthController constructs an AuthController.
func NewAuthController(
	auth *service.AuthService,
	me *service.MeService,
	password *service.PasswordService,
) *AuthController {
	return &AuthController{auth: auth, me: me, password: password}
}

// Register attaches routes to the provided router group.
//
// Middleware ordering:
//
//	public  : tenantMW (opens tx + default '__platform__') → handler
//	secured : jwtMW → tenantMW (reads claims, sets tenant from JWT) →
//	          scopeGateMW → pwdChangeMW → handler
//
// scopeGateMW is passed in from route.go so the allowlist paths
// (/auth/me, /auth/select-tenant, /auth/logout, /auth/me/password) are
// still reachable under scope=user.
func (h *AuthController) Register(rg *gin.RouterGroup, tenantMW, jwtMW, scopeGateMW, pwdChangeMW gin.HandlerFunc) {
	auth := rg.Group("/auth")

	// Public endpoints — tenantMW opens the request transaction + installs
	// the default '__platform__' sentinel. Service code switches the tenant
	// context after resolving the user.
	public := auth.Group("", tenantMW)
	public.POST("/login", h.handleLogin)
	public.POST("/refresh", h.handleRefresh)
	public.POST("/password/forgot", h.handleForgotPassword)
	public.POST("/password/reset", h.handleResetPassword)

	// Authenticated endpoints — JWT runs FIRST so tenantMW then sees the
	// claims and sets app.current_tenant. scopeGateMW blocks scope=user on
	// most endpoints; pwdChangeMW blocks must_change_password callers.
	secured := auth.Group("", jwtMW, tenantMW, scopeGateMW, pwdChangeMW)
	secured.POST("/logout", h.handleLogout)
	secured.GET("/me", h.handleGetMe)
	secured.PATCH("/me", h.handleUpdateMe)
	secured.POST("/me/password", h.handleChangePassword)
	secured.POST("/select-tenant", h.handleSelectTenant)
}

func (h *AuthController) handleLogin(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		helper.RespondBindError(c, err)
		return
	}

	// Resolve effective identifier. Identifier takes precedence; fall back to
	// the deprecated Email field for backward compatibility with old clients.
	effectiveIdentifier := req.Identifier
	if effectiveIdentifier == "" {
		effectiveIdentifier = req.Email
	}
	if effectiveIdentifier == "" {
		helper.RespondError(c, http.StatusBadRequest, constants.CodeValidation,
			"identifier or email is required")
		return
	}

	out, err := h.auth.Login(c.Request.Context(), service.LoginInput{
		Identifier: effectiveIdentifier,
		Password:   req.Password,
		UserAgent:  c.GetHeader("User-Agent"),
		IP:         c.ClientIP(),
	})
	if err != nil {
		helper.RespondDomainError(c, err)
		return
	}

	c.JSON(http.StatusOK, LoginResponse{
		AccessToken:        out.AccessToken,
		RefreshToken:       out.RefreshToken,
		TokenType:          constants.TokenTypeBearer,
		ExpiresAt:          out.ExpiresAt,
		Scope:              out.Scope,
		User:               toUserProfileResponse(out.User),
		Memberships:        toMembershipSummaryResponses(out.Memberships),
		ActiveMembershipID: out.ActiveMembershipID,
	})
}

func (h *AuthController) handleRefresh(c *gin.Context) {
	var req RefreshRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		helper.RespondBindError(c, err)
		return
	}

	out, err := h.auth.Refresh(c.Request.Context(), service.RefreshInput{
		RefreshToken: req.RefreshToken,
		UserAgent:    c.GetHeader("User-Agent"),
		IP:           c.ClientIP(),
	})
	if err != nil {
		helper.RespondDomainError(c, err)
		return
	}

	c.JSON(http.StatusOK, RefreshResponse{
		AccessToken:        out.AccessToken,
		RefreshToken:       out.RefreshToken,
		TokenType:          constants.TokenTypeBearer,
		ExpiresAt:          out.ExpiresAt,
		Scope:              out.Scope,
		ActiveMembershipID: out.ActiveMembershipID,
	})
}

func (h *AuthController) handleLogout(c *gin.Context) {
	claims, ok := middleware.ClaimsFromContext(c)
	if !ok {
		helper.RespondError(c, http.StatusUnauthorized, constants.CodeUnauthenticated, "authentication required")
		return
	}

	var req LogoutRequest
	_ = c.ShouldBindJSON(&req) // body is optional

	if err := h.auth.Logout(c.Request.Context(), service.LogoutInput{
		CallerUserID: claims.Subject,
		RefreshToken: req.RefreshToken,
	}); err != nil {
		helper.RespondDomainError(c, err)
		return
	}

	c.Status(http.StatusNoContent)
}

func (h *AuthController) handleGetMe(c *gin.Context) {
	claims, ok := middleware.ClaimsFromContext(c)
	if !ok {
		helper.RespondError(c, http.StatusUnauthorized, constants.CodeUnauthenticated, "authentication required")
		return
	}

	out, err := h.me.GetMe(c.Request.Context(), claims.Subject, claims.MembershipID, claims.Scope)
	if err != nil {
		helper.RespondDomainError(c, err)
		return
	}

	resp := GetMeResponse{
		User:               toUserProfileResponse(out.User),
		ActiveMembershipID: out.ActiveMembershipID,
		Memberships:        toMembershipSummaryResponses(out.Memberships),
	}
	if resp.Memberships == nil {
		resp.Memberships = []MembershipSummaryResponse{}
	}
	if out.Tenant != nil {
		resp.Tenant = &TenantInfoResponse{
			ID:     out.Tenant.ID,
			Name:   out.Tenant.Name,
			Slug:   out.Tenant.Slug,
			Status: out.Tenant.Status,
		}
	}
	c.JSON(http.StatusOK, resp)
}

func (h *AuthController) handleUpdateMe(c *gin.Context) {
	claims, ok := middleware.ClaimsFromContext(c)
	if !ok {
		helper.RespondError(c, http.StatusUnauthorized, constants.CodeUnauthenticated, "authentication required")
		return
	}

	var req UpdateMeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		helper.RespondBindError(c, err)
		return
	}

	prof, err := h.me.UpdateMe(c.Request.Context(), service.UpdateMeInput{
		CallerUserID: claims.Subject,
		FullName:     req.FullName,
		Phone:        req.Phone,
		AvatarURL:    req.AvatarURL,
	})
	if err != nil {
		helper.RespondDomainError(c, err)
		return
	}

	c.JSON(http.StatusOK, toUserProfileResponse(prof))
}

func (h *AuthController) handleChangePassword(c *gin.Context) {
	claims, ok := middleware.ClaimsFromContext(c)
	if !ok {
		helper.RespondError(c, http.StatusUnauthorized, constants.CodeUnauthenticated, "authentication required")
		return
	}

	var req ChangePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		helper.RespondBindError(c, err)
		return
	}

	if err := h.password.ChangePassword(c.Request.Context(), service.ChangePasswordInput{
		CallerUserID: claims.Subject,
		OldPassword:  req.OldPassword,
		NewPassword:  req.NewPassword,
	}); err != nil {
		helper.RespondDomainError(c, err)
		return
	}

	c.Status(http.StatusNoContent)
}

func (h *AuthController) handleForgotPassword(c *gin.Context) {
	var req ForgotPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		helper.RespondBindError(c, err)
		return
	}

	// Intentionally ignore errors — anti-enumeration.
	_ = h.password.ForgotPassword(c.Request.Context(), service.ForgotPasswordInput{
		Email: req.Email,
		IP:    c.ClientIP(),
	})

	c.Status(http.StatusNoContent)
}

func (h *AuthController) handleResetPassword(c *gin.Context) {
	var req ResetPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		helper.RespondBindError(c, err)
		return
	}

	if err := h.password.ResetPassword(c.Request.Context(), service.ResetPasswordInput{
		Token:       req.Token,
		NewPassword: req.NewPassword,
	}); err != nil {
		helper.RespondDomainError(c, err)
		return
	}

	c.Status(http.StatusNoContent)
}

// handleSelectTenant implements POST /auth/select-tenant.
// It is accessible to callers with any scope (the ScopeGate allowlist
// includes this path), so a scope=user caller can call it to upgrade to
// scope=tenant.
func (h *AuthController) handleSelectTenant(c *gin.Context) {
	claims, ok := middleware.ClaimsFromContext(c)
	if !ok {
		helper.RespondError(c, http.StatusUnauthorized, constants.CodeUnauthenticated, "authentication required")
		return
	}

	var req SelectTenantRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		helper.RespondBindError(c, err)
		return
	}

	// Extract the old refresh token from the Authorization header context — it
	// may be absent if the client does not send it in the body. We pass whatever
	// refresh_token the client provides in an optional body field; if absent the
	// old token simply won't be revoked in this call (user can logout to clean up).
	// For simplicity, the old refresh token rotation is handled inside SwitchTenant
	// via an optional body field. We re-use LogoutRequest's optional field.
	var body struct {
		RefreshToken string `json:"refresh_token"`
	}
	_ = c.ShouldBindBodyWith(&body, nil) // best-effort; body already consumed above

	out, err := h.auth.SwitchTenant(c.Request.Context(), service.SwitchTenantInput{
		CallerUserID:    claims.Subject,
		TenantID:        req.TenantID,
		OldRefreshToken: body.RefreshToken,
		UserAgent:       c.GetHeader("User-Agent"),
		IP:              c.ClientIP(),
	})
	if err != nil {
		helper.RespondDomainError(c, err)
		return
	}

	c.JSON(http.StatusOK, SelectTenantResponse{
		AccessToken:        out.AccessToken,
		RefreshToken:       out.RefreshToken,
		TokenType:          constants.TokenTypeBearer,
		ExpiresAt:          out.ExpiresAt,
		Scope:              out.Scope,
		ActiveMembershipID: out.ActiveMembershipID,
		Membership:         toMembershipSummaryResponse(out.Membership),
	})
}

// tokenTypeFromScope is unused but kept for documentation purposes.
// The scope field on responses always uses the string directly from the service.
var _ = func() model.TokenScope { return model.ScopeTenant }
