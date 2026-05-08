package controller

import (
	"bytes"
	"context"
	"encoding/hex"
	"fmt"
	"net/http"
	"time"

	"github.com/chrisanlung/lustia-auth/internal/constants"
	"github.com/chrisanlung/lustia-auth/internal/helper"
	"github.com/chrisanlung/lustia-auth/internal/middleware"
	"github.com/chrisanlung/lustia-auth/internal/service"
	"github.com/gin-gonic/gin"

	commoncfglog "github.com/chrisanlung/common-configs/log"
	"crypto/rand"
)

// TherapistController handles therapist CRUD endpoints under /tenant/therapists.
type TherapistController struct {
	svc     *service.TherapistSvc
	storage service.Storage
	quota   uploadQuotaChecker
	maxBytes int64
}

// uploadQuotaChecker is the minimal interface the controller needs for quota
// enforcement. Satisfied by *storage.TenantQuota.
type uploadQuotaChecker interface {
	Check(tenantID string) error
	Record(tenantID string)
}

// NewTherapistController constructs a TherapistController.
func NewTherapistController(svc *service.TherapistSvc, stor service.Storage, quota uploadQuotaChecker, maxBytes int64) *TherapistController {
	return &TherapistController{svc: svc, storage: stor, quota: quota, maxBytes: maxBytes}
}

// Register attaches all therapist routes under the provided secured group.
func (h *TherapistController) Register(rg *gin.RouterGroup, rbacMW func(code string) gin.HandlerFunc) {
	tenant := rg.Group("/tenant")
	tenant.POST("/therapists", rbacMW(constants.PermTherapistCreate), h.handleCreate)
	tenant.GET("/therapists", rbacMW(constants.PermTherapistRead), h.handleList)
	tenant.GET("/therapists/:id", rbacMW(constants.PermTherapistRead), h.handleGet)
	tenant.PATCH("/therapists/:id", rbacMW(constants.PermTherapistUpdate), h.handleUpdate)
	tenant.PATCH("/therapists/:id/status", rbacMW(constants.PermTherapistUpdate), h.handleChangeStatus)
	tenant.DELETE("/therapists/:id", rbacMW(constants.PermTherapistDelete), h.handleDelete)

	// ADR 0011 — photo upload/remove endpoints.
	tenant.POST("/therapists/:id/photo", rbacMW(constants.PermTherapistUpdate), h.handleUploadPhoto)
	tenant.DELETE("/therapists/:id/photo", rbacMW(constants.PermTherapistUpdate), h.handleRemovePhoto)
}

func (h *TherapistController) handleCreate(c *gin.Context) {
	claims, ok := middleware.ClaimsFromContext(c)
	if !ok {
		helper.RespondError(c, http.StatusUnauthorized, constants.CodeUnauthenticated, "authentication required")
		return
	}

	var req CreateTherapistRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		helper.RespondBindError(c, err)
		return
	}

	out, err := h.svc.Create(c.Request.Context(), service.CreateTherapistInput{
		CallerUserID:   claims.Subject,
		CallerTenantID: claims.TenantID,
		CallerBranches: claims.Branches,
		IsAdmin:        service.HasTenantAdminRole(claims.Roles),
		BranchID:       req.BranchID,
		FullName:       req.FullName,
		Gender:         req.Gender,
		Phone:          req.Phone,
		Email:          req.Email,
		Bio:            req.Bio,
		HeightCm:       req.HeightCm,
		WeightKg:       req.WeightKg,
		Build:          req.Build,
		JoinedAt:       req.JoinedAt,
		UserID:         req.UserID,
	})
	if err != nil {
		helper.RespondDomainError(c, err)
		return
	}
	c.JSON(http.StatusCreated, h.toTherapistResponse(c.Request.Context(), out))
}

func (h *TherapistController) handleList(c *gin.Context) {
	claims, ok := middleware.ClaimsFromContext(c)
	if !ok {
		helper.RespondError(c, http.StatusUnauthorized, constants.CodeUnauthenticated, "authentication required")
		return
	}

	var q ListTherapistsQuery
	if err := c.ShouldBindQuery(&q); err != nil {
		helper.RespondBindError(c, err)
		return
	}

	var branchID *string
	if q.BranchID != "" {
		branchID = &q.BranchID
	}

	page := q.Page
	if page < 1 {
		page = 1
	}
	limit := q.Limit
	if limit < 1 {
		limit = 10
	}

	out, err := h.svc.List(c.Request.Context(), service.ListTherapistsInput{
		CallerTenantID: claims.TenantID,
		CallerBranches: claims.Branches,
		IsAdmin:        service.HasTenantAdminRole(claims.Roles),
		BranchID:       branchID,
		IsActive:       q.IsActive,
		Page:           page,
		Limit:          limit,
	})
	if err != nil {
		helper.RespondDomainError(c, err)
		return
	}

	items := make([]TherapistResponse, len(out.Therapists))
	for i, t := range out.Therapists {
		items[i] = h.toTherapistResponse(c.Request.Context(), t)
	}
	c.JSON(http.StatusOK, ListTherapistsResponse{
		Data:       items,
		Page:       out.Page,
		Limit:      limit,
		TotalCount: out.TotalCount,
		TotalPages: out.TotalPages,
	})
}

func (h *TherapistController) handleGet(c *gin.Context) {
	claims, ok := middleware.ClaimsFromContext(c)
	if !ok {
		helper.RespondError(c, http.StatusUnauthorized, constants.CodeUnauthenticated, "authentication required")
		return
	}

	id := c.Param("id")
	out, err := h.svc.Get(c.Request.Context(), claims.TenantID, id)
	if err != nil {
		helper.RespondDomainError(c, err)
		return
	}
	c.JSON(http.StatusOK, h.toTherapistDetailResponse(c.Request.Context(), out))
}

func (h *TherapistController) handleUpdate(c *gin.Context) {
	claims, ok := middleware.ClaimsFromContext(c)
	if !ok {
		helper.RespondError(c, http.StatusUnauthorized, constants.CodeUnauthenticated, "authentication required")
		return
	}

	id := c.Param("id")

	var req UpdateTherapistRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		helper.RespondBindError(c, err)
		return
	}

	out, err := h.svc.Update(c.Request.Context(), service.UpdateTherapistInput{
		TherapistID:    id,
		CallerUserID:   claims.Subject,
		CallerTenantID: claims.TenantID,
		CallerBranches: claims.Branches,
		IsAdmin:        service.HasTenantAdminRole(claims.Roles),
		FullName:       req.FullName,
		Gender:         req.Gender,
		Phone:          req.Phone,
		Email:          req.Email,
		Bio:            req.Bio,
		HeightCm:       req.HeightCm,
		WeightKg:       req.WeightKg,
		Build:          req.Build,
		JoinedAt:       req.JoinedAt,
		UserID:         req.UserID,
		PrepMinutes:    req.PrepMinutes,
	})
	if err != nil {
		helper.RespondDomainError(c, err)
		return
	}
	c.JSON(http.StatusOK, h.toTherapistResponse(c.Request.Context(), out))
}

func (h *TherapistController) handleChangeStatus(c *gin.Context) {
	claims, ok := middleware.ClaimsFromContext(c)
	if !ok {
		helper.RespondError(c, http.StatusUnauthorized, constants.CodeUnauthenticated, "authentication required")
		return
	}

	id := c.Param("id")

	var req ChangeTherapistStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		helper.RespondBindError(c, err)
		return
	}

	out, err := h.svc.ChangeStatus(c.Request.Context(), service.ChangeTherapistStatusInput{
		TherapistID:    id,
		CallerUserID:   claims.Subject,
		CallerTenantID: claims.TenantID,
		CallerBranches: claims.Branches,
		IsAdmin:        service.HasTenantAdminRole(claims.Roles),
		IsActive:       *req.IsActive,
	})
	if err != nil {
		helper.RespondDomainError(c, err)
		return
	}
	c.JSON(http.StatusOK, h.toTherapistResponse(c.Request.Context(), out))
}

func (h *TherapistController) handleDelete(c *gin.Context) {
	claims, ok := middleware.ClaimsFromContext(c)
	if !ok {
		helper.RespondError(c, http.StatusUnauthorized, constants.CodeUnauthenticated, "authentication required")
		return
	}

	id := c.Param("id")
	if err := h.svc.SoftDelete(
		c.Request.Context(),
		claims.TenantID,
		claims.Subject,
		claims.Branches,
		service.HasTenantAdminRole(claims.Roles),
		id,
	); err != nil {
		helper.RespondDomainError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// ---------------------------------------------------------------------------
// ADR 0011 — Photo upload/remove handlers
// ---------------------------------------------------------------------------

// handleUploadPhoto implements POST /api/v1/tenant/therapists/:id/photo.
// Pipeline order per ADR 0011 §2.4 (enforced in this exact sequence):
//
//  1. Body-size hard cap via MaxBytesReader.
//  2. Explicit multipart parse.
//  3. MIME sniff + structural validation + EXIF strip via helper.ProcessUpload.
//  4. Per-tenant quota check.
//  5. Key generation.
//  6. Storage.Upload.
//  7. DB UPDATE inside transaction (via service.UpdatePhotoKey).
//  8. Fire-and-forget old-key delete.
func (h *TherapistController) handleUploadPhoto(c *gin.Context) {
	claims, ok := middleware.ClaimsFromContext(c)
	if !ok {
		helper.RespondError(c, http.StatusUnauthorized, constants.CodeUnauthenticated, "authentication required")
		return
	}
	therapistID := c.Param("id")
	ctx := c.Request.Context()

	// Step 1: hard body-size cap (security-expert H-1).
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, h.maxBytes+1024)

	// Step 2: explicit multipart parse with 1 MiB in-memory spill.
	if err := c.Request.ParseMultipartForm(1 << 20); err != nil {
		helper.RespondError(c, http.StatusBadRequest, constants.CodeValidation,
			"gagal membaca form multipart: "+err.Error())
		return
	}

	file, _, err := c.Request.FormFile("photo")
	if err != nil {
		helper.RespondError(c, http.StatusBadRequest, constants.CodeValidation,
			"field 'photo' wajib disertakan")
		return
	}
	defer file.Close()

	// Steps 3+: MIME sniff, structural validation, dimension check, EXIF strip.
	stripped, mime, err := helper.ProcessUpload(file, h.maxBytes)
	if err != nil {
		helper.RespondDomainError(c, err)
		return
	}

	// Step 4: per-tenant quota check.
	if err := h.quota.Check(claims.TenantID); err != nil {
		helper.RespondDomainError(c, err)
		return
	}

	// Step 5: generate unique storage key.
	newKey, err := generatePhotoKey(therapistID, helper.ImageExt(mime))
	if err != nil {
		helper.RespondDomainError(c, fmt.Errorf("key generation failed: %w", err))
		return
	}

	// Step 6: upload to storage.
	if err := h.storage.Upload(ctx, newKey, bytes.NewReader(stripped), mime); err != nil {
		commoncfglog.Errorf(ctx, err, "storage upload failed",
			"tenant_id", claims.TenantID,
			"photo_key", newKey,
			"driver", "local", // TODO: thread driver name through storage
		)
		helper.RespondError(c, http.StatusInternalServerError, constants.CodeInternal,
			"gagal menyimpan foto")
		return
	}

	// Step 7: DB update inside transaction (service enforces tenant + branch rules).
	oldKey, err := h.svc.UpdatePhotoKey(ctx, service.UploadTherapistPhotoInput{
		TherapistID:    therapistID,
		CallerUserID:   claims.Subject,
		CallerTenantID: claims.TenantID,
		CallerBranches: claims.Branches,
		IsAdmin:        service.HasTenantAdminRole(claims.Roles),
		NewKey:         newKey,
	})
	if err != nil {
		// New file was written but DB update failed — attempt cleanup.
		go func() {
			bgCtx := context.Background()
			if delErr := h.storage.Delete(bgCtx, newKey); delErr != nil {
				commoncfglog.Errorf(bgCtx, delErr, "cleanup after failed db update",
					"photo_key", newKey)
			}
		}()
		helper.RespondDomainError(c, err)
		return
	}

	// Record quota event after confirmed success.
	h.quota.Record(claims.TenantID)

	// Step 8: fire-and-forget delete of old file — after tx committed.
	if oldKey != nil && *oldKey != "" {
		go func(key string) {
			bgCtx := context.Background()
			if err := h.storage.Delete(bgCtx, key); err != nil {
				commoncfglog.Errorf(bgCtx, err, "delete old photo key",
					"photo_key", key, "tenant_id", claims.TenantID)
			}
		}(*oldKey)
	}

	// Return updated therapist detail.
	out, err := h.svc.Get(ctx, claims.TenantID, therapistID)
	if err != nil {
		helper.RespondDomainError(c, err)
		return
	}
	c.JSON(http.StatusOK, h.toTherapistDetailResponse(ctx, out))
}

// handleRemovePhoto implements DELETE /api/v1/tenant/therapists/:id/photo.
func (h *TherapistController) handleRemovePhoto(c *gin.Context) {
	claims, ok := middleware.ClaimsFromContext(c)
	if !ok {
		helper.RespondError(c, http.StatusUnauthorized, constants.CodeUnauthenticated, "authentication required")
		return
	}
	therapistID := c.Param("id")
	ctx := c.Request.Context()

	oldKey, err := h.svc.RemovePhotoKey(ctx, service.RemoveTherapistPhotoInput{
		TherapistID:    therapistID,
		CallerUserID:   claims.Subject,
		CallerTenantID: claims.TenantID,
		CallerBranches: claims.Branches,
		IsAdmin:        service.HasTenantAdminRole(claims.Roles),
	})
	if err != nil {
		helper.RespondDomainError(c, err)
		return
	}

	if oldKey != nil && *oldKey != "" {
		go func(key string) {
			bgCtx := context.Background()
			if err := h.storage.Delete(bgCtx, key); err != nil {
				commoncfglog.Errorf(bgCtx, err, "delete removed photo key",
					"photo_key", key, "tenant_id", claims.TenantID)
			}
		}(*oldKey)
	}

	c.Status(http.StatusNoContent)
}

// ---------------------------------------------------------------------------
// Mapping helpers
// ---------------------------------------------------------------------------

// toTherapistResponse converts a TherapistDetail to its HTTP response DTO,
// resolving the photo key to a public URL at the controller boundary
// (ADR 0011 §2.1: URL resolution happens at the controller boundary).
func (h *TherapistController) toTherapistResponse(ctx context.Context, d service.TherapistDetail) TherapistResponse {
	specialties := d.Specialties
	if specialties == nil {
		specialties = []string{}
	}
	resp := TherapistResponse{
		ID:          d.ID,
		TenantID:    d.TenantID,
		BranchID:    d.BranchID,
		UserID:      d.UserID,
		FullName:    d.FullName,
		Gender:      d.Gender,
		Bio:         d.Bio,
		HeightCm:    d.HeightCm,
		WeightKg:    d.WeightKg,
		Build:       d.Build,
		Specialties: specialties,
		PrepMinutes: d.PrepMinutes,
		IsActive:    d.IsActive,
		JoinedAt:    d.JoinedAt,
		CreatedAt:   d.CreatedAt,
		UpdatedAt:   d.UpdatedAt,
	}
	if d.PhotoKey != nil && *d.PhotoKey != "" {
		url, err := h.storage.URL(ctx, *d.PhotoKey)
		if err != nil {
			// Log and degrade gracefully — do not fail the response.
			commoncfglog.Errorf(ctx, err, "resolve photo_url",
				"photo_key", *d.PhotoKey)
		} else {
			resp.PhotoURL = &url
		}
	}
	return resp
}

func (h *TherapistController) toTherapistDetailResponse(ctx context.Context, d service.TherapistDetailWithServices) TherapistDetailResponse {
	items := make([]TherapistServiceItemResponse, len(d.Services))
	for i, sv := range d.Services {
		items[i] = toTherapistServiceItemResponse(sv)
	}
	return TherapistDetailResponse{
		TherapistResponse: h.toTherapistResponse(ctx, d.TherapistDetail),
		Services:          items,
	}
}

func toTherapistServiceItemResponse(item service.TherapistServiceItem) TherapistServiceItemResponse {
	return TherapistServiceItemResponse{
		ServiceID:       item.ServiceID,
		Name:            item.Name,
		Category:        item.Category,
		DurationMinutes: item.DurationMinutes,
		PriceIDR:        item.PriceIDR,
		IsActive:        item.IsActive,
		AssignedAt:      item.AssignedAt,
	}
}

// ---------------------------------------------------------------------------
// Private helpers
// ---------------------------------------------------------------------------

// generatePhotoKey produces a storage key of the form
// "therapists/{therapistID}/{16-random-hex}.{ext}" per ADR 0011 §2.4 step 7.
func generatePhotoKey(therapistID, ext string) (string, error) {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate photo key: %w", err)
	}
	return fmt.Sprintf("therapists/%s/%s.%s", therapistID, hex.EncodeToString(b), ext), nil
}

// uploadStartTime is used to record upload duration for observability.
// TODO(ADR 0011 §2.6): wire to Prometheus counter/histogram when /metrics ships.
var _ = time.Now // silence unused import warning until metrics are wired
