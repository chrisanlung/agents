//go:build !dev && !local

package route

import (
	"github.com/chrisanlung/lustia-auth/internal/controller"
	"github.com/gin-gonic/gin"
)

// registerDummyTrigger is a no-op in non-dev/local builds.
// The dummy trigger endpoint is excluded from production binaries (C-2, SECURITY.md).
func registerDummyTrigger(_ *gin.RouterGroup, _ *controller.PaymentController) {}
