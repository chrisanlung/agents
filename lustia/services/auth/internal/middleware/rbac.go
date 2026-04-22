package middleware

import (
	"net/http"
	"slices"

	"github.com/chrisanlung/lustia-auth/internal/constants"
	"github.com/chrisanlung/lustia-auth/internal/helper"
	"github.com/gin-gonic/gin"
)

// RequirePermission returns a Gin middleware that aborts with 403 if the
// authenticated caller does not hold the named permission code. It reads
// the permissions slice from the JWT claims already loaded by JWTVerify.
func RequirePermission(code string) gin.HandlerFunc {
	return func(c *gin.Context) {
		claims, ok := ClaimsFromContext(c)
		if !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, helper.ErrorResponse{
				Error: helper.ErrorDetail{Code: constants.CodeUnauthenticated, Message: "authentication required"},
			})
			return
		}

		if !slices.Contains(claims.Permissions, code) {
			c.AbortWithStatusJSON(http.StatusForbidden, helper.ErrorResponse{
				Error: helper.ErrorDetail{
					Code:    constants.CodeInsufficientPermission,
					Message: "you do not have the required permission: " + code,
				},
			})
			return
		}

		c.Next()
	}
}
