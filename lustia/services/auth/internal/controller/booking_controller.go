package controller

import (
	"context"
	"net/http"
	"slices"

	"github.com/chrisanlung/lustia-auth/internal/constants"
	"github.com/chrisanlung/lustia-auth/internal/helper"
	"github.com/chrisanlung/lustia-auth/internal/middleware"
	"github.com/chrisanlung/lustia-auth/internal/model"
	"github.com/chrisanlung/lustia-auth/internal/service"
	"github.com/gin-gonic/gin"
)

// BookingServiceIface is the consumer-owned interface for booking business logic.
// Declared here so the controller depends only on this interface, not the
// concrete *service.BookingService.
type BookingServiceIface interface {
	CreatePublic(ctx context.Context, in service.PublicCreateBookingInput) (service.CreateBookingOutput, error)
	GetPublicByCode(ctx context.Context, code string) (service.PublicBookingView, error)
	CreateConcierge(ctx context.Context, in service.ConciergeCreateBookingInput) (service.CreateBookingOutput, error)
	Get(ctx context.Context, in service.GetBookingInput) (service.BookingDetail, error)
	GetByCode(ctx context.Context, in service.GetBookingInput) (service.BookingDetail, error)
	List(ctx context.Context, in service.ListBookingsInput) (service.ListBookingsOutput, error)
	CheckIn(ctx context.Context, in service.CheckInInput) (service.BookingDetail, error)
	Complete(ctx context.Context, in service.CompleteInput) (service.BookingDetail, error)
	MarkNoShow(ctx context.Context, in service.NoShowInput) (service.BookingDetail, error)
	Cancel(ctx context.Context, in service.CancelInput) (service.BookingDetail, error)
	HandlePaymentWebhook(ctx context.Context, n service.MidtransWebhookNotification) error
	GetReportSummary(ctx context.Context, in service.GetReportInput) (service.BookingReportSummary, error)
	ListAvailableSlots(ctx context.Context, in service.AvailableSlotsInput) ([]service.Slot, error)
	ListPublicBranches(ctx context.Context, filter service.PublicBranchFilter) ([]service.PublicBranchSummary, int64, error)
	GetPublicBranchDetail(ctx context.Context, branchID string) (service.PublicBranchDetail, error)
}

// BookingController handles all booking-related HTTP endpoints.
type BookingController struct {
	svc BookingServiceIface
}

// NewBookingController constructs a BookingController.
func NewBookingController(svc BookingServiceIface) *BookingController {
	return &BookingController{svc: svc}
}

// ---------------------------------------------------------------------------
// Public routes (no JWT)
// ---------------------------------------------------------------------------

// RegisterPublic registers the unauthenticated public booking endpoints.
// Rate limiting is applied externally by the route package (H-2).
func (c *BookingController) RegisterPublic(rg *gin.RouterGroup) {
	rg.POST("/public/bookings", c.CreatePublic)
	rg.GET("/public/bookings/:code", c.GetPublicByCode)
	rg.POST("/public/payments/webhook", c.HandleWebhook)
	rg.GET("/public/branches", c.ListPublicBranches)
	rg.GET("/public/branches/:id/availability", c.GetAvailability)
}

// RegisterOperator registers the JWT-protected operator booking endpoints.
func (c *BookingController) RegisterOperator(rg *gin.RouterGroup, rbac func(string) gin.HandlerFunc) {
	rg.GET("/tenant/bookings", rbac(constants.PermBookingRead), c.ListBookings)
	rg.GET("/tenant/bookings/by-code/:code", rbac(constants.PermBookingRead), c.GetByCode)
	rg.GET("/tenant/bookings/:id", rbac(constants.PermBookingRead), c.GetBooking)
	rg.POST("/tenant/bookings", rbac(constants.PermBookingCreate), c.CreateConcierge)
	rg.POST("/tenant/bookings/:id/checkin", rbac(constants.PermBookingCheckin), c.CheckIn)
	rg.POST("/tenant/bookings/:id/complete", rbac(constants.PermBookingComplete), c.Complete)
	rg.POST("/tenant/bookings/:id/no-show", rbac(constants.PermBookingNoShow), c.MarkNoShow)
	rg.POST("/tenant/bookings/:id/cancel", rbac(constants.PermBookingCancel), c.Cancel)
	rg.GET("/tenant/reports/bookings/summary", rbac(constants.PermBookingRead), c.ReportSummary)
}

// ---------------------------------------------------------------------------
// Public handlers
// ---------------------------------------------------------------------------

// CreatePublic handles POST /api/v1/public/bookings.
// C-1: CreateBookingRequest has no total_price_idr field.
func (c *BookingController) CreatePublic(ctx *gin.Context) {
	var req CreateBookingRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		helper.RespondBindError(ctx, err)
		return
	}

	out, err := c.svc.CreatePublic(ctx.Request.Context(), service.PublicCreateBookingInput{
		BranchID:       req.BranchID,
		ServiceID:      req.ServiceID,
		AddonIDs:       req.AddonIDs,
		RoomID:         req.RoomID,
		TherapistID:    req.TherapistID,
		ScheduledStart: req.ScheduledStart,
		CustomerName:   req.CustomerName,
		CustomerPhone:  req.CustomerPhone,
		CustomerEmail:  req.CustomerEmail,
		ClientIP:       ctx.ClientIP(),
	})
	if err != nil {
		helper.RespondDomainError(ctx, err)
		return
	}

	resp := CreateBookingResponse{
		BookingResponse: toBookingResponse(out.BookingDetail),
		SnapToken:       out.SnapToken,
		RedirectURL:     out.RedirectURL,
	}
	ctx.JSON(http.StatusCreated, resp)
}

// GetPublicByCode handles GET /api/v1/public/bookings/:code.
// H-7: code is required; empty code is rejected at the service layer.
func (c *BookingController) GetPublicByCode(ctx *gin.Context) {
	code := ctx.Param("code")
	if code == "" {
		helper.RespondError(ctx, http.StatusBadRequest, constants.CodeBookingCodeInvalid, "booking code is required")
		return
	}

	view, err := c.svc.GetPublicByCode(ctx.Request.Context(), code)
	if err != nil {
		helper.RespondDomainError(ctx, err)
		return
	}
	ctx.JSON(http.StatusOK, toPublicBookingResponse(view))
}

// HandleWebhook handles POST /api/v1/public/payments/webhook.
// Not rate-limited (Midtrans retries on non-200 responses).
func (c *BookingController) HandleWebhook(ctx *gin.Context) {
	var req WebhookNotificationRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		// Malformed body — return 200 anyway so Midtrans doesn't retry.
		ctx.JSON(http.StatusOK, gin.H{"status": "ok"})
		return
	}

	err := c.svc.HandlePaymentWebhook(ctx.Request.Context(), service.MidtransWebhookNotification{
		OrderID:           req.OrderID,
		TransactionStatus: req.TransactionStatus,
		StatusCode:        req.StatusCode,
		GrossAmount:       req.GrossAmount,
		SignatureKey:      req.SignatureKey,
		PaymentType:       req.PaymentType,
	})
	if err != nil {
		// Internal error — log but still return 200 to suppress Midtrans retries
		// for errors that are not recoverable (e.g. DB down is retried by ops).
		ctx.JSON(http.StatusOK, gin.H{"status": "error", "message": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// ListPublicBranches handles GET /api/v1/public/branches.
func (c *BookingController) ListPublicBranches(ctx *gin.Context) {
	var q PublicBranchListQuery
	if err := ctx.ShouldBindQuery(&q); err != nil {
		helper.RespondBindError(ctx, err)
		return
	}
	if q.Limit <= 0 {
		q.Limit = 10
	}
	if q.Page <= 0 {
		q.Page = 1
	}

	var category *string
	if q.Category != "" {
		category = &q.Category
	}

	summaries, total, err := c.svc.ListPublicBranches(ctx.Request.Context(), service.PublicBranchFilter{
		Q:        q.Q,
		Lat:      q.Lat,
		Lng:      q.Lng,
		Category: category,
		OpenNow:  q.OpenNow,
		Page:     q.Page,
		Limit:    q.Limit,
	})
	if err != nil {
		helper.RespondDomainError(ctx, err)
		return
	}

	data := make([]PublicBranchSummaryResponse, len(summaries))
	for i, s := range summaries {
		data[i] = PublicBranchSummaryResponse{
			ID:             s.ID,
			Name:           s.Name,
			City:           s.City,
			Province:       s.Province,
			AddressLine1:   s.AddressLine1,
			ContactPhone:   s.ContactPhone,
			ContactEmail:   s.ContactEmail,
			Latitude:       s.Latitude,
			Longitude:      s.Longitude,
			DistanceMeters: s.DistanceMeters,
			Categories:     s.Categories,
		}
	}

	ctx.JSON(http.StatusOK, ListPublicBranchesResponse{
		Data:       data,
		Page:       q.Page,
		Limit:      q.Limit,
		TotalCount: total,
	})
}

// GetPublicBranchDetail handles GET /api/v1/public/branches/:id.
// Response dipetakan ke PublicBranchDetailResponse agar Flutter menerima
// snake_case key dan operational_hours sebagai array JSON (bukan base64).
func (c *BookingController) GetPublicBranchDetail(ctx *gin.Context) {
	id := ctx.Param("id")
	detail, err := c.svc.GetPublicBranchDetail(ctx.Request.Context(), id)
	if err != nil {
		helper.RespondDomainError(ctx, err)
		return
	}
	ctx.JSON(http.StatusOK, toPublicBranchDetailResponse(detail))
}

// GetAvailability handles GET /api/v1/public/branches/:id/availability.
func (c *BookingController) GetAvailability(ctx *gin.Context) {
	branchID := ctx.Param("id")
	var q AvailabilityQuery
	if err := ctx.ShouldBindQuery(&q); err != nil {
		helper.RespondBindError(ctx, err)
		return
	}

	slots, err := c.svc.ListAvailableSlots(ctx.Request.Context(), service.AvailableSlotsInput{
		BranchID:  branchID,
		ServiceID: q.ServiceID,
		Date:      q.Date,
	})
	if err != nil {
		helper.RespondDomainError(ctx, err)
		return
	}

	out := make([]SlotResponse, len(slots))
	for i, s := range slots {
		out[i] = SlotResponse{
			Start:                    s.Start,
			End:                      s.End,
			TherapistsAvailableCount: s.TherapistsAvailableCount,
			RoomsAvailableCount:      s.RoomsAvailableCount,
		}
	}

	ctx.JSON(http.StatusOK, AvailableSlotsResponse{
		BranchID:  branchID,
		ServiceID: q.ServiceID,
		Date:      q.Date,
		Slots:     out,
	})
}

// ---------------------------------------------------------------------------
// Operator handlers
// ---------------------------------------------------------------------------

// ListBookings handles GET /api/v1/tenant/bookings.
func (c *BookingController) ListBookings(ctx *gin.Context) {
	claims, _ := middleware.ClaimsFromContext(ctx)

	// Guard: this endpoint requires a real tenant scope. Rejection with 403
	// (instead of letting the SQL UUID parser blow up further down) lets the
	// frontend handleApiError redirect the user back to /login or
	// /select-tenant cleanly. Triggers if claims missing, scope=platform,
	// scope=user without tenant selection, or anomalous '__platform__'
	// sentinel slipped through.
	if claims.Scope != "tenant" || claims.TenantID == "" || claims.TenantID == constants.PublicTenantSentinel || claims.TenantID == "__platform__" {
		helper.RespondError(ctx, http.StatusForbidden, constants.CodeInsufficientPermission, "konteks tenant tidak valid. Silakan masuk kembali.")
		return
	}

	var q ListBookingsQuery
	if err := ctx.ShouldBindQuery(&q); err != nil {
		helper.RespondBindError(ctx, err)
		return
	}
	if q.Limit <= 0 {
		q.Limit = 10
	}
	if q.Page <= 0 {
		q.Page = 1
	}

	var branchID, status, serviceID, fromDate, toDate *string
	if q.BranchID != "" {
		branchID = &q.BranchID
	}
	if q.Status != "" {
		status = &q.Status
	}
	if q.ServiceID != "" {
		serviceID = &q.ServiceID
	}
	if q.FromDate != "" {
		fromDate = &q.FromDate
	}
	if q.ToDate != "" {
		toDate = &q.ToDate
	}

	out, err := c.svc.List(ctx.Request.Context(), service.ListBookingsInput{
		CallerTenantID: claims.TenantID,
		CallerBranches: claims.Branches,
		IsAdmin:        isAdmin(claims.Roles),
		BranchID:       branchID,
		Status:         status,
		ServiceID:      serviceID,
		FromDate:       fromDate,
		ToDate:         toDate,
		Page:           q.Page,
		Limit:          q.Limit,
	})
	if err != nil {
		helper.RespondDomainError(ctx, err)
		return
	}

	data := make([]BookingResponse, len(out.Bookings))
	for i, b := range out.Bookings {
		data[i] = toBookingResponse(b)
	}
	ctx.JSON(http.StatusOK, ListBookingsResponse{
		Data:       data,
		Page:       out.Page,
		Limit:      q.Limit,
		TotalCount: out.TotalCount,
		TotalPages: out.TotalPages,
	})
}

// GetBooking handles GET /api/v1/tenant/bookings/:id.
func (c *BookingController) GetBooking(ctx *gin.Context) {
	claims, _ := middleware.ClaimsFromContext(ctx)
	id := ctx.Param("id")

	d, err := c.svc.Get(ctx.Request.Context(), service.GetBookingInput{
		CallerTenantID: claims.TenantID,
		CallerBranches: claims.Branches,
		IsAdmin:        isAdmin(claims.Roles),
		BookingID:      id,
	})
	if err != nil {
		helper.RespondDomainError(ctx, err)
		return
	}
	ctx.JSON(http.StatusOK, toBookingResponse(d))
}

// GetByCode handles GET /api/v1/tenant/bookings/by-code/:code.
func (c *BookingController) GetByCode(ctx *gin.Context) {
	claims, _ := middleware.ClaimsFromContext(ctx)
	code := ctx.Param("code")

	d, err := c.svc.GetByCode(ctx.Request.Context(), service.GetBookingInput{
		CallerTenantID: claims.TenantID,
		CallerBranches: claims.Branches,
		IsAdmin:        isAdmin(claims.Roles),
		BookingID:      code, // service.GetByCode uses BookingID field as code
	})
	if err != nil {
		helper.RespondDomainError(ctx, err)
		return
	}
	ctx.JSON(http.StatusOK, toBookingResponse(d))
}

// CreateConcierge handles POST /api/v1/tenant/bookings.
func (c *BookingController) CreateConcierge(ctx *gin.Context) {
	claims, _ := middleware.ClaimsFromContext(ctx)

	var req ConciergeCreateBookingRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		helper.RespondBindError(ctx, err)
		return
	}

	out, err := c.svc.CreateConcierge(ctx.Request.Context(), service.ConciergeCreateBookingInput{
		CallerUserID:   claims.Subject,
		CallerTenantID: claims.TenantID,
		CallerBranches: claims.Branches,
		IsAdmin:        isAdmin(claims.Roles),
		BranchID:       req.BranchID,
		ServiceID:      req.ServiceID,
		AddonIDs:       req.AddonIDs,
		RoomID:         req.RoomID,
		TherapistID:    req.TherapistID,
		ScheduledStart: req.ScheduledStart,
		CustomerName:   req.CustomerName,
		CustomerPhone:  req.CustomerPhone,
		CustomerEmail:  req.CustomerEmail,
	})
	if err != nil {
		helper.RespondDomainError(ctx, err)
		return
	}

	ctx.JSON(http.StatusCreated, CreateBookingResponse{
		BookingResponse: toBookingResponse(out.BookingDetail),
		SnapToken:       out.SnapToken,
		RedirectURL:     out.RedirectURL,
	})
}

// CheckIn handles POST /api/v1/tenant/bookings/:id/checkin.
func (c *BookingController) CheckIn(ctx *gin.Context) {
	claims, _ := middleware.ClaimsFromContext(ctx)
	id := ctx.Param("id")

	var req CheckInRequest
	_ = ctx.ShouldBindJSON(&req) // optional body

	d, err := c.svc.CheckIn(ctx.Request.Context(), service.CheckInInput{
		CallerUserID:   claims.Subject,
		CallerTenantID: claims.TenantID,
		CallerBranches: claims.Branches,
		IsAdmin:        isAdmin(claims.Roles),
		BookingID:      id,
		Code:           req.Code,
	})
	if err != nil {
		helper.RespondDomainError(ctx, err)
		return
	}
	ctx.JSON(http.StatusOK, toBookingResponse(d))
}

// Complete handles POST /api/v1/tenant/bookings/:id/complete.
func (c *BookingController) Complete(ctx *gin.Context) {
	claims, _ := middleware.ClaimsFromContext(ctx)
	id := ctx.Param("id")

	d, err := c.svc.Complete(ctx.Request.Context(), service.CompleteInput{
		CallerUserID:   claims.Subject,
		CallerTenantID: claims.TenantID,
		CallerBranches: claims.Branches,
		IsAdmin:        isAdmin(claims.Roles),
		BookingID:      id,
	})
	if err != nil {
		helper.RespondDomainError(ctx, err)
		return
	}
	ctx.JSON(http.StatusOK, toBookingResponse(d))
}

// MarkNoShow handles POST /api/v1/tenant/bookings/:id/no-show.
func (c *BookingController) MarkNoShow(ctx *gin.Context) {
	claims, _ := middleware.ClaimsFromContext(ctx)
	id := ctx.Param("id")

	d, err := c.svc.MarkNoShow(ctx.Request.Context(), service.NoShowInput{
		CallerUserID:   claims.Subject,
		CallerTenantID: claims.TenantID,
		CallerBranches: claims.Branches,
		IsAdmin:        isAdmin(claims.Roles),
		BookingID:      id,
	})
	if err != nil {
		helper.RespondDomainError(ctx, err)
		return
	}
	ctx.JSON(http.StatusOK, toBookingResponse(d))
}

// Cancel handles POST /api/v1/tenant/bookings/:id/cancel.
func (c *BookingController) Cancel(ctx *gin.Context) {
	claims, _ := middleware.ClaimsFromContext(ctx)
	id := ctx.Param("id")

	var req CancelBookingRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		helper.RespondBindError(ctx, err)
		return
	}

	d, err := c.svc.Cancel(ctx.Request.Context(), service.CancelInput{
		CallerUserID:   claims.Subject,
		CallerTenantID: claims.TenantID,
		CallerBranches: claims.Branches,
		IsAdmin:        isAdmin(claims.Roles),
		BookingID:      id,
		Reason:         req.Reason,
	})
	if err != nil {
		helper.RespondDomainError(ctx, err)
		return
	}
	ctx.JSON(http.StatusOK, toBookingResponse(d))
}

// ReportSummary handles GET /api/v1/tenant/reports/bookings/summary.
func (c *BookingController) ReportSummary(ctx *gin.Context) {
	claims, _ := middleware.ClaimsFromContext(ctx)

	var q ReportBookingQuery
	if err := ctx.ShouldBindQuery(&q); err != nil {
		helper.RespondBindError(ctx, err)
		return
	}

	var branchID *string
	if q.BranchID != "" {
		branchID = &q.BranchID
	}

	summary, err := c.svc.GetReportSummary(ctx.Request.Context(), service.GetReportInput{
		CallerTenantID: claims.TenantID,
		BranchID:       branchID,
		FromDate:       q.From,
		ToDate:         q.To,
	})
	if err != nil {
		helper.RespondDomainError(ctx, err)
		return
	}

	ctx.JSON(http.StatusOK, BookingReportSummaryResponse{
		TotalBookings:  summary.TotalBookings,
		TotalPaidIDR:   summary.TotalPaidIDR,
		CompletedCount: summary.CompletedCount,
		CancelledCount: summary.CancelledCount,
		NoShowCount:    summary.NoShowCount,
		ExpiredCount:   summary.ExpiredCount,
		NoShowRate:     summary.NoShowRate,
	})
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// isAdmin returns true when the roles slice contains tenant_admin or super_admin.
func isAdmin(roles []string) bool {
	return slices.Contains(roles, constants.RoleCodeTenantAdmin) ||
		slices.Contains(roles, constants.RoleCodeSuperAdmin)
}

// isAdminModel re-exports isAdmin from model constants for clarity.
var _ = model.BookingStatusPaid // compile-time reference to confirm model is imported
