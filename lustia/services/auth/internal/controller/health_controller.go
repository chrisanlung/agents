package controller

import (
	"database/sql"
	"net/http"

	"github.com/gin-gonic/gin"
)

// HealthController exposes liveness and readiness probes.
type HealthController struct {
	db *sql.DB
}

// NewHealthController constructs a HealthController.
// The *sql.DB is used by the readiness probe to verify connectivity.
// Callers pass the underlying pool via `gormDB.DB()` in main.go — gorm stays out of this package.
func NewHealthController(db *sql.DB) *HealthController {
	return &HealthController{db: db}
}

// Register attaches health probe routes to the provided engine.
func (h *HealthController) Register(r interface {
	GET(string, ...gin.HandlerFunc) gin.IRoutes
}) {
	r.GET("/healthz", h.handleLiveness)
	r.GET("/readyz", h.handleReadiness)
}

// handleLiveness always returns 200. The process is alive if it can respond.
func (h *HealthController) handleLiveness(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// handleReadiness pings the database and returns 200 when healthy, 503 when not.
func (h *HealthController) handleReadiness(c *gin.Context) {
	if err := h.db.PingContext(c.Request.Context()); err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"status": "unavailable", "detail": "db ping failed"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}
