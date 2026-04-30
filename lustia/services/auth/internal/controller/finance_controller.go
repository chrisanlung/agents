package controller

import (
	"context"
	"net/http"
	"time"

	"github.com/chrisanlung/lustia-auth/internal/constants"
	"github.com/chrisanlung/lustia-auth/internal/helper"
	"github.com/chrisanlung/lustia-auth/internal/middleware"
	"github.com/chrisanlung/lustia-auth/internal/model"
	"github.com/chrisanlung/lustia-auth/internal/service"
	"github.com/gin-gonic/gin"
)

// FinancePaymentServiceIface is the consumer-owned projection of PaymentServiceIface
// that FinanceController needs (SOLID-I: minimal interface).
type FinancePaymentServiceIface interface {
	GetTenantBalance(ctx context.Context, tenantID string) (service.BalanceSummary, error)
}

// FinancePaymentTxnServiceIface is the consumer-owned projection for transaction lists.
type FinancePaymentTxnServiceIface interface {
	FindByTenant(ctx context.Context, tenantID string, filter service.PaymentTxnFilter) ([]*model.PaymentTransaction, int64, error)
}

// FinanceDisbursementServiceIface is the consumer-owned projection for tenant disbursements.
type FinanceDisbursementServiceIface interface {
	ListByTenant(ctx context.Context, tenantID string, filter service.DisbursementFilter) ([]*model.TenantDisbursement, int64, error)
	GetDetail(ctx context.Context, id string, callerTenantID *string) (service.DisbursementDetail, error)
}

// FinanceController handles tenant-facing finance endpoints.
// All routes require the finance.read permission (ADR 0015 §2.11).
type FinanceController struct {
	paymentSvc      service.PaymentServiceIface
	paymentTxnRepo  service.PaymentTransactionRepository
	disbursementSvc service.DisbursementServiceIface
}

// NewFinanceController constructs a FinanceController.
func NewFinanceController(
	paymentSvc service.PaymentServiceIface,
	paymentTxnRepo service.PaymentTransactionRepository,
	disbursementSvc service.DisbursementServiceIface,
) *FinanceController {
	return &FinanceController{
		paymentSvc:      paymentSvc,
		paymentTxnRepo:  paymentTxnRepo,
		disbursementSvc: disbursementSvc,
	}
}

// Register wires all tenant finance routes onto the given group.
func (c *FinanceController) Register(rg *gin.RouterGroup, rbac func(string) gin.HandlerFunc) {
	rg.GET("/tenant/finance/balance", rbac(constants.PermFinanceRead), c.GetBalance)
	rg.GET("/tenant/finance/transactions", rbac(constants.PermFinanceRead), c.ListTransactions)
	rg.GET("/tenant/finance/disbursements", rbac(constants.PermFinanceRead), c.ListDisbursements)
	rg.GET("/tenant/finance/disbursements/:id", rbac(constants.PermFinanceRead), c.GetDisbursement)
}

// ---------------------------------------------------------------------------
// Handlers
// ---------------------------------------------------------------------------

// GetBalance handles GET /api/v1/tenant/finance/balance.
// Returns the three-tier balance summary card for the current tenant.
func (c *FinanceController) GetBalance(ctx *gin.Context) {
	claims, ok := middleware.ClaimsFromContext(ctx)
	if !ok {
		helper.RespondError(ctx, http.StatusUnauthorized, "UNAUTHORIZED", "authentication required")
		return
	}

	summary, err := c.paymentSvc.GetTenantBalance(ctx.Request.Context(), claims.TenantID)
	if err != nil {
		helper.RespondDomainError(ctx, err)
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"in_process_idr":        summary.InProcessIDR,
		"ready_to_disburse_idr": summary.ReadyToDisburseIDR,
		"disbursed_idr":         summary.DisbursedIDR,
	})
}

// ListTransactions handles GET /api/v1/tenant/finance/transactions.
// Returns a paginated list of payment_transaction rows for the caller's tenant.
func (c *FinanceController) ListTransactions(ctx *gin.Context) {
	claims, ok := middleware.ClaimsFromContext(ctx)
	if !ok {
		helper.RespondError(ctx, http.StatusUnauthorized, "UNAUTHORIZED", "authentication required")
		return
	}

	var q financeTransactionQuery
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

	filter := service.PaymentTxnFilter{
		Status:   q.Status,
		FromDate: q.FromDate,
		ToDate:   q.ToDate,
		Page:     q.Page,
		Limit:    q.Limit,
	}

	rows, total, err := c.paymentTxnRepo.FindByTenant(ctx.Request.Context(), claims.TenantID, filter)
	if err != nil {
		helper.RespondDomainError(ctx, err)
		return
	}

	data := make([]paymentTxnResponse, 0, len(rows))
	for _, t := range rows {
		data = append(data, toPaymentTxnResponse(t))
	}
	ctx.JSON(http.StatusOK, gin.H{
		"data":        data,
		"total":       total,
		"page":        q.Page,
		"total_pages": totalPages(total, q.Limit),
	})
}

// ListDisbursements handles GET /api/v1/tenant/finance/disbursements.
func (c *FinanceController) ListDisbursements(ctx *gin.Context) {
	claims, ok := middleware.ClaimsFromContext(ctx)
	if !ok {
		helper.RespondError(ctx, http.StatusUnauthorized, "UNAUTHORIZED", "authentication required")
		return
	}

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

	rows, total, err := c.disbursementSvc.ListByTenant(ctx.Request.Context(), claims.TenantID, service.DisbursementFilter{
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

// GetDisbursement handles GET /api/v1/tenant/finance/disbursements/:id.
func (c *FinanceController) GetDisbursement(ctx *gin.Context) {
	claims, ok := middleware.ClaimsFromContext(ctx)
	if !ok {
		helper.RespondError(ctx, http.StatusUnauthorized, "UNAUTHORIZED", "authentication required")
		return
	}

	id := ctx.Param("id")
	detail, err := c.disbursementSvc.GetDetail(ctx.Request.Context(), id, &claims.TenantID)
	if err != nil {
		helper.RespondDomainError(ctx, err)
		return
	}
	ctx.JSON(http.StatusOK, toDisbursementDetailResponse(detail))
}

// ---------------------------------------------------------------------------
// Request DTOs
// ---------------------------------------------------------------------------

type financeTransactionQuery struct {
	Status   *string `form:"status"`
	FromDate *string `form:"from_date"`
	ToDate   *string `form:"to_date"`
	Page     int     `form:"page"  binding:"omitempty,min=1"`
	Limit    int     `form:"limit" binding:"omitempty,min=1,max=100"`
}

type disbursementListQuery struct {
	Status *string `form:"status"`
	Page   int     `form:"page"  binding:"omitempty,min=1"`
	Limit  int     `form:"limit" binding:"omitempty,min=1,max=100"`
}

// ---------------------------------------------------------------------------
// Response DTOs + mappers
// ---------------------------------------------------------------------------

type paymentTxnResponse struct {
	ID                string  `json:"id"`
	BookingID         string  `json:"booking_id"`
	ProviderReference string  `json:"provider_reference"`
	Provider          string  `json:"provider"`
	Status            string  `json:"status"`
	ExpectedAmountIDR int64   `json:"expected_amount_idr"`
	ReceivedAmountIDR *int64  `json:"received_amount_idr,omitempty"`
	PlatformFeeIDR    *int64  `json:"platform_fee_idr,omitempty"`
	TenantNetIDR      *int64  `json:"tenant_net_idr,omitempty"`
	PaidAt            *string `json:"paid_at,omitempty"`
	SettledAt         *string `json:"settled_at,omitempty"`
	DisbursedAt       *string `json:"disbursed_at,omitempty"`
	CreatedAt         string  `json:"created_at"`
}

func toPaymentTxnResponse(t *model.PaymentTransaction) paymentTxnResponse {
	r := paymentTxnResponse{
		ID:                t.ID,
		BookingID:         t.BookingID,
		ProviderReference: t.ProviderReference,
		Provider:          t.Provider,
		Status:            t.Status,
		ExpectedAmountIDR: t.ExpectedAmountIDR,
		ReceivedAmountIDR: t.ReceivedAmountIDR,
		PlatformFeeIDR:    t.PlatformFeeIDR,
		TenantNetIDR:      t.TenantNetIDR,
		CreatedAt:         t.CreatedAt.Format(time.RFC3339),
	}
	if t.PaidAt != nil {
		s := t.PaidAt.Format(time.RFC3339)
		r.PaidAt = &s
	}
	if t.SettledAt != nil {
		s := t.SettledAt.Format(time.RFC3339)
		r.SettledAt = &s
	}
	if t.DisbursedAt != nil {
		s := t.DisbursedAt.Format(time.RFC3339)
		r.DisbursedAt = &s
	}
	return r
}

type disbursementSummaryResponse struct {
	ID               string  `json:"id"`
	TenantID         string  `json:"tenant_id"`
	PeriodStart      string  `json:"period_start"`
	PeriodEnd        string  `json:"period_end"`
	GrossAmountIDR   int64   `json:"gross_amount_idr"`
	PlatformFeeIDR   int64   `json:"platform_fee_idr"`
	NetAmountIDR     int64   `json:"net_amount_idr"`
	TransactionCount int     `json:"transaction_count"`
	Status           string  `json:"status"`
	BankReference    *string `json:"bank_reference,omitempty"`
	TransferredAt    *string `json:"transferred_at,omitempty"`
	CreatedAt        string  `json:"created_at"`
}

func toDisbursementSummaryResponse(d *model.TenantDisbursement) disbursementSummaryResponse {
	r := disbursementSummaryResponse{
		ID:               d.ID,
		TenantID:         d.TenantID,
		PeriodStart:      d.PeriodStart.Format("2006-01-02"),
		PeriodEnd:        d.PeriodEnd.Format("2006-01-02"),
		GrossAmountIDR:   d.GrossAmountIDR,
		PlatformFeeIDR:   d.PlatformFeeIDR,
		NetAmountIDR:     d.NetAmountIDR,
		TransactionCount: d.TransactionCount,
		Status:           d.Status,
		BankReference:    d.BankReference,
		CreatedAt:        d.CreatedAt.Format(time.RFC3339),
	}
	if d.TransferredAt != nil {
		s := d.TransferredAt.Format(time.RFC3339)
		r.TransferredAt = &s
	}
	return r
}

type disbursementDetailResponse struct {
	disbursementSummaryResponse
	Notes         *string              `json:"notes,omitempty"`
	TransferredBy *string              `json:"transferred_by,omitempty"`
	UpdatedAt     string               `json:"updated_at"`
	Transactions  []paymentTxnSummaryR `json:"transactions"`
}

type paymentTxnSummaryR struct {
	ID                string  `json:"id"`
	BookingID         string  `json:"booking_id"`
	ProviderReference string  `json:"provider_reference"`
	Status            string  `json:"status"`
	ExpectedAmountIDR int64   `json:"expected_amount_idr"`
	ReceivedAmountIDR *int64  `json:"received_amount_idr,omitempty"`
	PlatformFeeIDR    *int64  `json:"platform_fee_idr,omitempty"`
	TenantNetIDR      *int64  `json:"tenant_net_idr,omitempty"`
	PaidAt            *string `json:"paid_at,omitempty"`
}

func toDisbursementDetailResponse(d service.DisbursementDetail) disbursementDetailResponse {
	txns := make([]paymentTxnSummaryR, 0, len(d.Transactions))
	for _, t := range d.Transactions {
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

	summary := disbursementSummaryResponse{
		ID:               d.ID,
		TenantID:         d.TenantID,
		PeriodStart:      d.PeriodStart.Format("2006-01-02"),
		PeriodEnd:        d.PeriodEnd.Format("2006-01-02"),
		GrossAmountIDR:   d.GrossAmountIDR,
		PlatformFeeIDR:   d.PlatformFeeIDR,
		NetAmountIDR:     d.NetAmountIDR,
		TransactionCount: d.TransactionCount,
		Status:           d.Status,
		BankReference:    d.BankReference,
		CreatedAt:        d.CreatedAt.Format(time.RFC3339),
	}
	if d.TransferredAt != nil {
		s := d.TransferredAt.Format(time.RFC3339)
		summary.TransferredAt = &s
	}

	return disbursementDetailResponse{
		disbursementSummaryResponse: summary,
		Notes:                       d.Notes,
		TransferredBy:               d.TransferredBy,
		UpdatedAt:                   d.UpdatedAt.Format(time.RFC3339),
		Transactions:                txns,
	}
}

// totalPages computes the page count for pagination responses.
func totalPages(total int64, limit int) int64 {
	if limit <= 0 {
		return 0
	}
	return (total + int64(limit) - 1) / int64(limit)
}
