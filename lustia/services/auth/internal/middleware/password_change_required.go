package middleware

import (
	"net/http"
	"strings"

	"github.com/chrisanlung/lustia-auth/internal/constants"
	"github.com/chrisanlung/lustia-auth/internal/helper"
	"github.com/gin-gonic/gin"
)

// allowedPathsDuringPasswordChange lists the endpoints a caller may still hit
// while their access token carries must_change_password=true. Any other
// protected route is blocked with 403 PASSWORD_CHANGE_REQUIRED.
//
// Matched as prefix (method + full path) so that per-version prefixes are
// tolerated (e.g. /api/v1/auth/me/password).
var allowedPathsDuringPasswordChange = []struct {
	method string
	suffix string
}{
	{"GET", "/auth/me"},
	{"POST", "/auth/me/password"},
	{"POST", "/auth/logout"},
}

// PasswordChangeRequired must run after JWTVerify. When the authenticated
// caller's claim has MustChangePassword=true, only the allow-listed routes
// above are permitted; everything else is blocked.
func PasswordChangeRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		claims, ok := ClaimsFromContext(c)
		if !ok {
			// JWTVerify should have set the claims. If it didn't, this middleware
			// has nothing to do — let the next layer handle the missing context.
			c.Next()
			return
		}
		if !claims.MustChangePassword {
			c.Next()
			return
		}

		if isAllowedDuringPasswordChange(c.Request.Method, c.FullPath()) {
			c.Next()
			return
		}

		c.AbortWithStatusJSON(http.StatusForbidden, helper.ErrorResponse{
			Error: helper.ErrorDetail{
				Code:    constants.CodePasswordChangeRequired,
				Message: "password change is required before any other action",
			},
		})
	}
}

func isAllowedDuringPasswordChange(method, fullPath string) bool {
	for _, a := range allowedPathsDuringPasswordChange {
		if !strings.EqualFold(a.method, method) {
			continue
		}
		if strings.HasSuffix(fullPath, a.suffix) {
			return true
		}
	}
	return false
}
