//go:build dev || local

package controller

import (
	"encoding/json"
	"net/http"

	"github.com/gin-gonic/gin"
)

// RegisterDummyTrigger registers the dev-only dummy payment trigger endpoint.
// Called from route.go only when the binary is built with -tags dev|local.
// ADR 0015 §2.7 — POST /api/v1/public/payments/dummy-trigger.
func (c *PaymentController) RegisterDummyTrigger(rg *gin.RouterGroup) {
	rg.POST("/public/payments/dummy-trigger", c.DummyTrigger)
}

// dummyTriggerRequest is the body for the dev-only trigger endpoint.
type dummyTriggerRequest struct {
	// Code is the booking code (e.g. "AB12-CD34"). The service looks up the
	// associated payment_transaction.provider_reference by booking code.
	Code string `json:"code" binding:"required"`
	// AmountIDR is the amount to simulate (defaults to booking.total_price_idr
	// in the service layer if zero).
	AmountIDR int64 `json:"amount_idr"`
}

// DummyTrigger handles POST /api/v1/public/payments/dummy-trigger.
// Synthesises a PaymentNotification payload and forwards it to HandleWebhook.
// Build tag: only compiled with -tags dev|local (C-2, SECURITY.md).
func (c *PaymentController) DummyTrigger(ctx *gin.Context) {
	var req dummyTriggerRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Look up the real provider_reference + expected amount. HandleWebhook does
	// FindByProviderReference (NOT by booking code) and rejects amount
	// mismatches, so the synthesised payload must carry both the reference
	// produced at QR-creation time and the booking's expected amount.
	lookup, err := c.svc.GetProviderRefByCode(ctx.Request.Context(), req.Code)
	if err != nil {
		ctx.JSON(http.StatusNotFound, gin.H{"error": "booking or payment_transaction not found"})
		return
	}

	amount := req.AmountIDR
	if amount == 0 {
		amount = lookup.ExpectedAmountIDR
	}

	// Synthesise the dummy webhook payload (same shape DummyProvider.VerifyWebhook expects).
	payload := map[string]interface{}{
		"provider_reference": lookup.ProviderReference,
		"amount_idr":         amount,
	}
	rawPayload, _ := json.Marshal(payload)

	headers := map[string]string{
		"X-Dummy-Trigger": "true",
	}

	if err := c.svc.HandleWebhook(ctx.Request.Context(), rawPayload, headers); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"status": "ok", "triggered_for": req.Code})
}
