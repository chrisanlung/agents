package controller

import (
	"net/http"

	"github.com/chrisanlung/lustia-auth/internal/helper"
	"github.com/chrisanlung/lustia-auth/internal/service"
	"github.com/gin-gonic/gin"
)

// JWKSController exposes the public JSON Web Key Set document so downstream
// services can verify RS256 access tokens without contacting auth on every
// request.
type JWKSController struct {
	jwks *service.JWKSService
}

// NewJWKSController constructs a JWKSController.
func NewJWKSController(jwks *service.JWKSService) *JWKSController {
	return &JWKSController{jwks: jwks}
}

// Register attaches the JWKS route to the provided router group.
func (h *JWKSController) Register(rg *gin.RouterGroup) {
	rg.GET("/.well-known/jwks.json", h.handleGetJWKS)
}

func (h *JWKSController) handleGetJWKS(c *gin.Context) {
	doc, err := h.jwks.GetJWKS(c.Request.Context())
	if err != nil {
		helper.RespondDomainError(c, err)
		return
	}

	c.Data(http.StatusOK, "application/json; charset=utf-8", doc)
}
