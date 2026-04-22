// Package middleware contains Gin HTTP middleware for the auth service.
package middleware

import (
	"log/slog"
	"net/http"

	"github.com/chrisanlung/lustia-auth/internal/constants"
	"github.com/chrisanlung/lustia-auth/internal/helper"
	"github.com/gin-gonic/gin"
)

// Recover catches panics in handlers, logs them with a structured entry,
// and responds with 500 INTERNAL. No panic propagates past this layer.
func Recover() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if rec := recover(); rec != nil {
				ctx := c.Request.Context()
				slog.ErrorContext(ctx, "panic recovered", "recover", rec)
				c.AbortWithStatusJSON(http.StatusInternalServerError, helper.ErrorResponse{
					Error: helper.ErrorDetail{
						Code:    constants.CodeInternal,
						Message: "an internal error occurred",
					},
				})
			}
		}()
		c.Next()
	}
}
