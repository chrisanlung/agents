//go:build dev || local

package route

import (
	"github.com/chrisanlung/lustia-auth/internal/controller"
	"github.com/gin-gonic/gin"
)

// registerDummyTrigger wires the dev-only dummy payment trigger endpoint.
// Build tag: only compiled with -tags dev|local (C-2, SECURITY.md).
func registerDummyTrigger(rg *gin.RouterGroup, ctrl *controller.PaymentController) {
	ctrl.RegisterDummyTrigger(rg)
}
