package controller

import (
	"context"
	"database/sql"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
)

// storageProber is optionally implemented by storage adapters.
// If an adapter satisfies this interface, the readiness probe calls it.
// The Storage interface (ADR 0011 §2.1) intentionally does not include a
// HealthCheck method — keeping it focused on Upload/Delete/URL. Adapters that
// want probe support implement this separate interface.
type storageProber interface {
	HealthCheck(ctx context.Context) error
}

// localBasePather is satisfied by *storage.LocalStorage to expose its path
// for a lightweight os.Stat probe without importing the storage package.
type localBasePather interface {
	BasePath() string
}

// HealthController exposes liveness and readiness probes.
type HealthController struct {
	db      *sql.DB
	storage interface{} // *storage.LocalStorage or R2/Supabase stub — nil if not wired
}

// NewHealthController constructs a HealthController.
// The *sql.DB is used by the readiness probe to verify connectivity.
// storage may be nil; when non-nil it is probed on /readyz.
func NewHealthController(db *sql.DB, stor interface{}) *HealthController {
	return &HealthController{db: db, storage: stor}
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

// handleReadiness pings the database and optionally probes the storage adapter.
// Returns 200 when all checks pass, 503 otherwise.
func (h *HealthController) handleReadiness(c *gin.Context) {
	checks := gin.H{}

	// DB check.
	if err := h.db.PingContext(c.Request.Context()); err != nil {
		checks["db"] = "unavailable"
		c.JSON(http.StatusServiceUnavailable, gin.H{"status": "unavailable", "checks": checks})
		return
	}
	checks["db"] = "ok"

	// Storage check — optional; degrade gracefully if not wired.
	if h.storage != nil {
		storStatus := h.probeStorage(c.Request.Context())
		checks["storage"] = storStatus
	}

	c.JSON(http.StatusOK, gin.H{"status": "ok", "checks": checks})
}

// probeStorage returns "ok", "not_implemented", or an error string.
func (h *HealthController) probeStorage(ctx context.Context) string {
	// Try the optional HealthCheck method first (R2/Supabase stubs implement this).
	if p, ok := h.storage.(storageProber); ok {
		if err := p.HealthCheck(ctx); err != nil {
			return err.Error()
		}
		return "ok"
	}

	// LocalStorage exposes BasePath() — use os.Stat as a lightweight probe.
	if lp, ok := h.storage.(localBasePather); ok {
		if _, err := os.Stat(lp.BasePath()); err != nil {
			return "local path unavailable: " + err.Error()
		}
		return "ok"
	}

	return "probe not available"
}
