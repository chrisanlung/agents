package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// CORS returns a middleware that implements the minimum Cross-Origin
// resource-sharing behavior the dev web frontends need.
//
// allowedOrigins is the exact list of origins that may read responses
// (e.g. ["http://localhost:3001", "http://localhost:3002"]). An empty list
// disables CORS entirely. A single "*" allows any origin — acceptable for
// pure dev only; never use with AllowCredentials=true (browsers reject the
// combination).
//
// The middleware:
//   - echoes back the request Origin if it appears in allowedOrigins
//   - responds to preflight OPTIONS with 204
//   - sets Vary: Origin so caches don't mix allowed/denied responses
//   - exposes X-Request-ID on actual responses
func CORS(allowedOrigins []string) gin.HandlerFunc {
	wildcard := len(allowedOrigins) == 1 && allowedOrigins[0] == "*"
	allow := make(map[string]struct{}, len(allowedOrigins))
	for _, o := range allowedOrigins {
		allow[strings.TrimRight(o, "/")] = struct{}{}
	}

	const (
		allowedMethods = "GET,POST,PATCH,PUT,DELETE,OPTIONS"
		allowedHeaders = "Authorization,Content-Type,X-Request-ID,ngrok-skip-browser-warning"
		exposedHeaders = "X-Request-ID"
		maxAgeSeconds  = "300"
	)

	return func(c *gin.Context) {
		origin := strings.TrimRight(c.GetHeader("Origin"), "/")
		if origin == "" {
			// Not a browser-CORS request — let it through untouched.
			c.Next()
			return
		}

		h := c.Writer.Header()
		h.Add("Vary", "Origin")

		if wildcard {
			h.Set("Access-Control-Allow-Origin", "*")
		} else if _, ok := allow[origin]; ok {
			h.Set("Access-Control-Allow-Origin", origin)
			// AllowCredentials requires a concrete origin (never "*").
			h.Set("Access-Control-Allow-Credentials", "true")
		} else {
			// Origin not in the allowlist. Don't set the allow header; browser
			// will block, the handler still runs but the response is unreadable
			// cross-origin. Abort preflight with 403 for clarity.
			if c.Request.Method == http.MethodOptions {
				c.AbortWithStatus(http.StatusForbidden)
				return
			}
			c.Next()
			return
		}

		h.Set("Access-Control-Expose-Headers", exposedHeaders)

		if c.Request.Method == http.MethodOptions {
			h.Set("Access-Control-Allow-Methods", allowedMethods)
			h.Set("Access-Control-Allow-Headers", allowedHeaders)
			h.Set("Access-Control-Max-Age", maxAgeSeconds)
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}
