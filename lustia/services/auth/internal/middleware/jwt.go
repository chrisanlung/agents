package middleware

import (
	"net/http"
	"strings"

	"github.com/chrisanlung/lustia-auth/internal/constants"
	"github.com/chrisanlung/lustia-auth/internal/helper"
	"github.com/chrisanlung/lustia-auth/internal/model"
	"github.com/gin-gonic/gin"
)

// JWTVerify parses and validates the Bearer token from the Authorization header.
// On success it stores the model.AccessClaims in the gin context under
// CtxKeyAccessClaims. On failure it aborts with 401.
//
// Sensitive data (the raw token) is never logged — only structural errors.
func JWTVerify(issuer helper.TokenIssuer) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, helper.ErrorResponse{
				Error: helper.ErrorDetail{Code: constants.CodeUnauthenticated, Message: "authorization header required"},
			})
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, helper.ErrorResponse{
				Error: helper.ErrorDetail{Code: constants.CodeUnauthenticated, Message: "authorization header format must be 'Bearer <token>'"},
			})
			return
		}

		claims, err := issuer.VerifyAccessToken(c.Request.Context(), parts[1])
		if err != nil {
			code := constants.CodeTokenInvalid
			msg := "invalid token"
			if strings.Contains(err.Error(), "expired") {
				code = constants.CodeTokenExpired
				msg = "token has expired"
			}
			c.AbortWithStatusJSON(http.StatusUnauthorized, helper.ErrorResponse{
				Error: helper.ErrorDetail{Code: code, Message: msg},
			})
			return
		}

		c.Set(constants.CtxKeyAccessClaims, claims)
		c.Next()
	}
}

// ClaimsFromContext extracts model.AccessClaims from the gin context.
// Returns (zero, false) if the claims key is absent.
func ClaimsFromContext(c *gin.Context) (model.AccessClaims, bool) {
	v, exists := c.Get(constants.CtxKeyAccessClaims)
	if !exists {
		return model.AccessClaims{}, false
	}
	claims, ok := v.(model.AccessClaims)
	return claims, ok
}
