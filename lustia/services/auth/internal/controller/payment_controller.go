package controller

import (
	"context"
	"net/http"

	"github.com/chrisanlung/lustia-auth/internal/constants"
	"github.com/chrisanlung/lustia-auth/internal/helper"
	"github.com/chrisanlung/lustia-auth/internal/service"
	"github.com/gin-gonic/gin"
)

// PaymentServiceForController is the consumer-owned interface used by
// PaymentController. Declared here (consumer-owned, SOLID-I).
type PaymentServiceForController interface {
	HandleWebhook(ctx context.Context, rawPayload []byte, headers map[string]string) error
	GetStatus(ctx context.Context, bookingCode string) (service.PaymentStatusView, error)
}

// PaymentController handles payment-specific HTTP endpoints.
// Webhook + polling live here; booking creation is in BookingController.
type PaymentController struct {
	svc service.PaymentServiceIface
}

// NewPaymentController constructs a PaymentController.
func NewPaymentController(svc service.PaymentServiceIface) *PaymentController {
	return &PaymentController{svc: svc}
}

// RegisterPublic registers unauthenticated payment endpoints.
// Caller (route.go) applies per-IP rate limiters at the route level.
func (c *PaymentController) RegisterPublic(rg *gin.RouterGroup, paymentStatusLimiter gin.HandlerFunc) {
	rg.POST("/public/payments/webhook", c.HandleWebhook)
	rg.GET("/public/bookings/:code/payment-status", paymentStatusLimiter, c.GetPaymentStatus)
}

// ---------------------------------------------------------------------------
// Handlers
// ---------------------------------------------------------------------------

// HandleWebhook handles POST /api/v1/public/payments/webhook.
// Phase 6: reads raw bytes so the adapter can verify the provider HMAC
// signature before any JSON decoding (H-3 equivalent for iPaymu).
// Always returns HTTP 200 — providers must not retry on non-200.
func (c *PaymentController) HandleWebhook(ctx *gin.Context) {
	rawBody, err := ctx.GetRawData()
	if err != nil || len(rawBody) == 0 {
		ctx.JSON(http.StatusOK, gin.H{"status": "ok"})
		return
	}

	headers := make(map[string]string, 8)
	for key, vals := range ctx.Request.Header {
		if len(vals) > 0 {
			headers[key] = vals[0]
		}
	}

	if err := c.svc.HandleWebhook(ctx.Request.Context(), rawBody, headers); err != nil {
		// Log but still return 200 to avoid provider retry storms.
		ctx.JSON(http.StatusOK, gin.H{"status": "error", "message": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// GetPaymentStatus handles GET /api/v1/public/bookings/:code/payment-status.
// Rate-limited to 12/min per IP (ADR 0015 §2.7).
// Switches RLS to __public__ via the per-request tx middleware; service
// layer further narrows to a single booking row by code.
func (c *PaymentController) GetPaymentStatus(ctx *gin.Context) {
	code := ctx.Param("code")
	if code == "" {
		helper.RespondError(ctx, http.StatusBadRequest, constants.CodeBookingCodeInvalid,
			"booking code is required")
		return
	}

	view, err := c.svc.GetStatus(ctx.Request.Context(), code)
	if err != nil {
		helper.RespondDomainError(ctx, err)
		return
	}

	resp := PaymentStatusResponse{
		Status:      view.Status,
		QRExpiresAt: view.QRExpiresAt.Format("2006-01-02T15:04:05Z07:00"),
	}
	if view.PaidAt != nil {
		t := view.PaidAt.Format("2006-01-02T15:04:05Z07:00")
		resp.PaidAt = &t
	}
	ctx.JSON(http.StatusOK, resp)
}

// ---------------------------------------------------------------------------
// Response DTOs
// ---------------------------------------------------------------------------

// PaymentStatusResponse is the response for GET /public/bookings/:code/payment-status.
type PaymentStatusResponse struct {
	Status      string  `json:"status"`
	PaidAt      *string `json:"paid_at,omitempty"`
	QRExpiresAt string  `json:"qr_expires_at"`
}
