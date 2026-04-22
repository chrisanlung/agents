package middleware

import (
	"context"
	"net/http"

	"github.com/chrisanlung/lustia-auth/internal/constants"
	"github.com/chrisanlung/lustia-auth/internal/helper"
	"github.com/gin-gonic/gin"
)

// rateLimiterIface is the minimum interface needed for rate limiting middleware.
// This avoids importing helper directly from middleware — it accepts any value
// that implements Allow.
type rateLimiterIface interface {
	Allow(ctx context.Context, key string) bool
}

// RateLimit applies a per-(IP+path) rate limit using the injected limiter.
// Requests that exceed the limit receive 429 RATE_LIMITED.
func RateLimit(limiter rateLimiterIface) gin.HandlerFunc {
	return func(c *gin.Context) {
		key := c.ClientIP() + ":" + c.FullPath()
		if !limiter.Allow(c.Request.Context(), key) {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, helper.ErrorResponse{
				Error: helper.ErrorDetail{
					Code:    constants.CodeRateLimited,
					Message: "too many requests, please try again later",
				},
			})
			return
		}
		c.Next()
	}
}
