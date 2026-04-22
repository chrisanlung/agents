package middleware

import (
	"github.com/chrisanlung/lustia-auth/internal/constants"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const headerRequestID = "X-Request-ID"

// RequestID ensures every request carries a unique X-Request-ID.
// If the client sends one it is reused; otherwise a new UUID is generated.
// The ID is written to the response header and stored in the gin context.
func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		rid := c.GetHeader(headerRequestID)
		if rid == "" {
			rid = uuid.New().String()
		}
		c.Set(constants.CtxKeyRequestID, rid)
		c.Header(headerRequestID, rid)
		c.Next()
	}
}
