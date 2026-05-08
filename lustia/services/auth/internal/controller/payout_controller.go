package controller

import (
	"net/http"
	"time"

	"github.com/chrisanlung/lustia-auth/internal/constants"
	"github.com/chrisanlung/lustia-auth/internal/helper"
	"github.com/chrisanlung/lustia-auth/internal/middleware"
	"github.com/chrisanlung/lustia-auth/internal/model"
	"github.com/chrisanlung/lustia-auth/internal/service"
	"github.com/gin-gonic/gin"
)

// PayoutController handles platform-admin settlement + payout endpoints.
// All routes require super_admin permissions per ADR 0015 §2.11.
type PayoutController struct {
	settlementSvc   service.SettlementServiceIface
	disbursementSvc service.DisbursementServiceIface
}

// NewPayoutController constructs a PayoutController.
func NewPayoutController(
	settlementSvc service.SettlementServiceIface,
	disbursementSvc service.DisbursementServiceIface,
) *PayoutController {
	return &PayoutController{
		settlementSvc:   settlementSvc,
		disbursementSvc: disbursementSvc,
	}
}

// Register wires all platform-admin payout + settlement routes.
func (c *PayoutController) Register(rg *gin.RouterGroup, rbac func(string) gin.HandlerFunc) {
	// Settlement reconciliation.
	rg.POST("/admin/settlement/reconcile", rbac(constants.PermSettlementReconcile), c.Reconcile)
	rg.GET("/admin/settlement-batches", rbac(constants.PermFinanceReadAll), c.ListSettlementBatches)
	// /summary MUST be registered before /:id so Gin does not match the literal
	// string "summary" as a path parameter value.
	rg.GET("/admin/settlement-batches/summary", rbac(constants.PermFinanceReadAll), c.SettlementSummary)
	rg.GET("/admin/settlement-batches/:id", rbac(constants.PermFinanceReadAll), c.GetSettlementBatch)

	// Disbursement management.
	rg.GET("/admin/payout/tenant-summary", rbac(constants.PermDisbursementCreate), c.TenantPayoutSummary)
	rg.POST("/admin/disbursements", rbac(constants.PermDisbursementCreate), c.CreateDisbursement)
	rg.GET("/admin/disbursements", rbac(constants.PermFinanceReadAll), c.ListDisbursements)
	rg.GET("/admin/disbursements/:id", rbac(constants.PermFinanceReadAll), c.GetDisbursement)
	rg.POST("/admin/disbursements/:id/processing", rbac(constants.PermDisbursementCreate), c.MarkProcessing)
	rg.POST("/admin/disbursements/:id/transferred", rbac(constants.PermDisbursementTransfer), c.MarkTransferred)
	rg.POST("/admin/disbursements/:id/failed", rbac(constants.PermDisbursementTransfer), c.MarkFailed)
	rg.POST("/admin/disbursements/:id/cancel", rbac(constants.PermDisbursementCreate), c.CancelDisbursement)
}

// ---------------------------------------------------------------------------
// Settlement handlers
// ---------------------------------------------------------------------------

// reconcileRequest is the body for POST /admin/settlement/reconcile.
type reconcileRequest struct {
	// Date is the settlement date in YYYY-MM-DD format.
	Date string `json:"date" binding:"required"`
}

// Reconcile handles POST /api/v1/admin/settlement/reconcile.
func (c *PayoutController) Reconcile(ctx *gin.Context) {
	var req reconcileRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		helper.RespondBindError(ctx, err)
		return
	}

	date, err := time.Parse("2006-01-02", req.Date)
	if err != nil {
		helper.RespondError(ctx, http.StatusBadRequest, "INVALID_DATE",
			"date must be in YYYY-MM-DD format")
		return
	}

	claims, _ := middleware.ClaimsFromContext(ctx)
	summary, err := c.settlementSvc.Reconcile(ctx.Request.Context(), date, claims.Subject)
	if err != nil {
		helper.RespondDomainError(ctx, err)
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"batch_id":          summary.BatchID,
		"settled_at":        summary.SettledAt.Format("2006-01-02"),
		"transaction_count": summary.TransactionCount,
		"total_amount_idr":  summary.TotalAmountIDR,
		"mismatch_count":    summary.MismatchCount,
	})
}

// SettlementSummary handles GET /api/v1/admin/settlement-batches/summary.
// Returns aggregate KPIs for the platform-admin "Volume Disetel Minggu Ini"
// dashboard card.
func (c *PayoutController) SettlementSummary(ctx *gin.Context) {
	from := ctx.Query("from")
	to := ctx.Query("to")

	if from == "" || to == "" {
		helper.RespondError(ctx, http.StatusBadRequest, "VALIDATION", "from and to query parameters are required")
		return
	}

	// Validate format — time.Parse in the service will also check, but a
	// quick format guard here keeps error messages crisp at the HTTP boundary.
	if _, err := time.Parse("2006-01-02", from); err != nil {
		helper.RespondError(ctx, http.StatusBadRequest, "INVALID_DATE", "from must be in YYYY-MM-DD format")
		return
	}
	if _, err := time.Parse("2006-01-02", to); err != nil {
		helper.RespondError(ctx, http.StatusBadRequest, "INVALID_DATE", "to must be in YYYY-MM-DD format")
		return
	}

	out, err := c.settlementSvc.Summary(ctx.Request.Context(), service.SettlementSummaryInput{
		From: from,
		To:   to,
	})
	if err != nil {
		helper.RespondDomainError(ctx, err)
		return
	}

	ctx.JSON(http.StatusOK, out)
}

// ListSettlementBatches handles GET /api/v1/admin/settlement-batches.
func (c *PayoutController) ListSettlementBatches(ctx *gin.Context) {
	var q settlementBatchListQuery
	if err := ctx.ShouldBindQuery(&q); err != nil {
		helper.RespondBindError(ctx, err)
		return
	}
	if q.Page <= 0 {
		q.Page = 1
	}
	if q.Limit <= 0 {
		q.Limit = 10
	}

	batches, total, err := c.settlementSvc.ListBatches(ctx.Request.Context(), service.SettlementBatchFilter{
		Provider: q.Provider,
		Page:     q.Page,
		Limit:    q.Limit,
	})
	if err != nil {
		helper.RespondDomainError(ctx, err)
		return
	}

	data := make([]settlementBatchResponse, 0, len(batches))
	for _, b := range batches {
		data = append(data, toSettlementBatchResponse(b))
	}
	ctx.JSON(http.StatusOK, gin.H{
		"data":        data,
		"total":       total,
		"page":        q.Page,
		"total_pages": totalPages(total, q.Limit),
	})
}

// GetSettlementBatch handles GET /api/v1/admin/settlement-batches/:id.
func (c *PayoutController) GetSettlementBatch(ctx *gin.Context) {
	id := ctx.Param("id")
	detail, err := c.settlementSvc.GetBatchDetail(ctx.Request.Context(), id)
	if err != nil {
		helper.RespondDomainError(ctx, err)
		return
	}

	txns := make([]paymentTxnSummaryR, 0, len(detail.Transactions))
	for _, t := range detail.Transactions {
		r := paymentTxnSummaryR{
			ID:                t.ID,
			BookingID:         t.BookingID,
			ProviderReference: t.ProviderReference,
			Status:            t.Status,
			ExpectedAmountIDR: t.ExpectedAmountIDR,
			ReceivedAmountIDR: t.ReceivedAmountIDR,
			PlatformFeeIDR:    t.PlatformFeeIDR,
			TenantNetIDR:      t.TenantNetIDR,
		}
		if t.PaidAt != nil {
			s := t.PaidAt.Format(time.RFC3339)
			r.PaidAt = &s
		}
		txns = append(txns, r)
	}

	mismatches := make([]gin.H, 0, len(detail.Mismatches))
	for _, m := range detail.Mismatches {
		mismatches = append(mismatches, gin.H{
			"provider_reference": m.ProviderReference,
			"issue":              m.Issue,
			"amount_idr":         m.AmountIDR,
		})
	}

	ctx.JSON(http.StatusOK, gin.H{
		"batch_id":          detail.BatchID,
		"settled_at":        detail.SettledAt.Format("2006-01-02"),
		"transaction_count": detail.TransactionCount,
		"total_amount_idr":  detail.TotalAmountIDR,
		"mismatch_count":    detail.MismatchCount,
		"transactions":      txns,
		"mismatches":        mismatches,
	})
}

// ---------------------------------------------------------------------------
// Disbursement handlers
// ---------------------------------------------------------------------------

// TenantPayoutSummary handles GET /api/v1/admin/payout/tenant-summary.
// Returns per-tenant settled-but-not-disbursed balances for the given period.
func (c *PayoutController) TenantPayoutSummary(ctx *gin.Context) {
	var q tenantSummaryQuery
	if err := ctx.ShouldBindQuery(&q); err != nil {
		helper.RespondBindError(ctx, err)
		return
	}
	// This endpoint is informational — for Phase 6 it returns whatever
	// CalculatePayout would return per tenant. Full multi-tenant listing
	// is a Phase 7 feature (requires tenantRepo scan). Return placeholder.
	ctx.JSON(http.StatusOK, gin.H{
		"message": "tenant-summary endpoint available in Phase 7; use POST /admin/disbursements to create per-tenant disbursements",
		"period_start": q.PeriodStart,
		"period_end":   q.PeriodEnd,
	})
}

// CreateDisbursement handles POST /api/v1/admin/disbursements.
// Calculates payout and creates a pending disbursement row in one call.
type createDisbursementRequest struct {
	TenantID    string `json:"tenant_id"    binding:"required"`
	PeriodStart string `json:"period_start" binding:"required"` // YYYY-MM-DD
	PeriodEnd   string `json:"period_end"   binding:"required"` // YYYY-MM-DD
}

func (c *PayoutController) CreateDisbursement(ctx *gin.Context) {
	var req createDisbursementRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		helper.RespondBindError(ctx, err)
		return
	}

	start, err := time.Parse("2006-01-02", req.PeriodStart)
	if err != nil {
		helper.RespondError(ctx, http.StatusBadRequest, "INVALID_DATE", "period_start must be YYYY-MM-DD")
		return
	}
	end, err := time.Parse("2006-01-02", req.PeriodEnd)
	if err != nil {
		helper.RespondError(ctx, http.StatusBadRequest, "INVALID_DATE", "period_end must be YYYY-MM-DD")
		return
	}

	claims, _ := middleware.ClaimsFromContext(ctx)
	detail, err := c.disbursementSvc.Create(ctx.Request.Context(), service.CreateDisbursementInput{
		CallerUserID: claims.Subject,
		TenantID:     req.TenantID,
		PeriodStart:  start,
		PeriodEnd:    end,
	})
	if err != nil {
		helper.RespondDomainError(ctx, err)
		return
	}
	ctx.JSON(http.StatusCreated, toDisbursementDetailResponse(detail))
}

// ListDisbursements handles GET /api/v1/admin/disbursements.
func (c *PayoutController) ListDisbursements(ctx *gin.Context) {
	var q disbursementListQuery
	if err := ctx.ShouldBindQuery(&q); err != nil {
		helper.RespondBindError(ctx, err)
		return
	}
	if q.Page <= 0 {
		q.Page = 1
	}
	if q.Limit <= 0 {
		q.Limit = 10
	}

	rows, total, err := c.disbursementSvc.ListAll(ctx.Request.Context(), service.DisbursementFilter{
		Status: q.Status,
		Page:   q.Page,
		Limit:  q.Limit,
	})
	if err != nil {
		helper.RespondDomainError(ctx, err)
		return
	}

	data := make([]disbursementSummaryResponse, 0, len(rows))
	for _, d := range rows {
		data = append(data, toDisbursementSummaryResponse(d))
	}
	ctx.JSON(http.StatusOK, gin.H{
		"data":        data,
		"total":       total,
		"page":        q.Page,
		"total_pages": totalPages(total, q.Limit),
	})
}

// GetDisbursement handles GET /api/v1/admin/disbursements/:id.
func (c *PayoutController) GetDisbursement(ctx *gin.Context) {
	id := ctx.Param("id")
	detail, err := c.disbursementSvc.GetDetail(ctx.Request.Context(), id, nil) // nil = platform admin
	if err != nil {
		helper.RespondDomainError(ctx, err)
		return
	}
	ctx.JSON(http.StatusOK, toDisbursementDetailResponse(detail))
}

// MarkProcessing handles POST /api/v1/admin/disbursements/:id/processing.
func (c *PayoutController) MarkProcessing(ctx *gin.Context) {
	claims, _ := middleware.ClaimsFromContext(ctx)
	id := ctx.Param("id")
	if err := c.disbursementSvc.MarkProcessing(ctx.Request.Context(), id, claims.Subject); err != nil {
		helper.RespondDomainError(ctx, err)
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"status": "processing"})
}

// markTransferredRequest is the body for POST /admin/disbursements/:id/transferred.
type markTransferredRequest struct {
	BankReference *string `json:"bank_reference"`
	Notes         *string `json:"notes"`
}

// MarkTransferred handles POST /api/v1/admin/disbursements/:id/transferred.
func (c *PayoutController) MarkTransferred(ctx *gin.Context) {
	var req markTransferredRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		helper.RespondBindError(ctx, err)
		return
	}
	claims, _ := middleware.ClaimsFromContext(ctx)
	id := ctx.Param("id")
	if err := c.disbursementSvc.MarkTransferred(ctx.Request.Context(), id, claims.Subject, req.BankReference, req.Notes); err != nil {
		helper.RespondDomainError(ctx, err)
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"status": "transferred"})
}

// markFailedRequest is the body for POST /admin/disbursements/:id/failed.
type markFailedRequest struct {
	Reason *string `json:"reason"`
}

// MarkFailed handles POST /api/v1/admin/disbursements/:id/failed.
func (c *PayoutController) MarkFailed(ctx *gin.Context) {
	var req markFailedRequest
	_ = ctx.ShouldBindJSON(&req) // optional body
	claims, _ := middleware.ClaimsFromContext(ctx)
	id := ctx.Param("id")
	if err := c.disbursementSvc.MarkFailed(ctx.Request.Context(), id, claims.Subject, req.Reason); err != nil {
		helper.RespondDomainError(ctx, err)
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"status": "failed"})
}

// CancelDisbursement handles POST /api/v1/admin/disbursements/:id/cancel.
func (c *PayoutController) CancelDisbursement(ctx *gin.Context) {
	claims, _ := middleware.ClaimsFromContext(ctx)
	id := ctx.Param("id")
	if err := c.disbursementSvc.Cancel(ctx.Request.Context(), id, claims.Subject); err != nil {
		helper.RespondDomainError(ctx, err)
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"status": "cancelled"})
}

// ---------------------------------------------------------------------------
// Request / Response DTOs
// ---------------------------------------------------------------------------

type settlementBatchListQuery struct {
	Provider *string `form:"provider"`
	Page     int     `form:"page"  binding:"omitempty,min=1"`
	Limit    int     `form:"limit" binding:"omitempty,min=1,max=100"`
}

type tenantSummaryQuery struct {
	PeriodStart string `form:"period_start"`
	PeriodEnd   string `form:"period_end"`
}

type settlementBatchResponse struct {
	ID               string  `json:"id"`
	Provider         string  `json:"provider"`
	SettledAt        string  `json:"settled_at"`
	TotalAmountIDR   int64   `json:"total_amount_idr"`
	TransactionCount int     `json:"transaction_count"`
	CreatedAt        *string `json:"created_at,omitempty"`
}

func toSettlementBatchResponse(b *model.SettlementBatch) settlementBatchResponse {
	r := settlementBatchResponse{
		ID:               b.ID,
		Provider:         b.Provider,
		SettledAt:        b.SettledAt.Format("2006-01-02"),
		TotalAmountIDR:   b.TotalAmountIDR,
		TransactionCount: b.TransactionCount,
	}
	if b.CreatedAt != nil {
		s := b.CreatedAt.Format(time.RFC3339)
		r.CreatedAt = &s
	}
	return r
}
