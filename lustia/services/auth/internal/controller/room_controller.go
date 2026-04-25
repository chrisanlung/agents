package controller

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"

	commoncfglog "github.com/chrisanlung/common-configs/log"
	"github.com/chrisanlung/lustia-auth/internal/constants"
	"github.com/chrisanlung/lustia-auth/internal/helper"
	"github.com/chrisanlung/lustia-auth/internal/middleware"
	"github.com/chrisanlung/lustia-auth/internal/service"
	"github.com/gin-gonic/gin"
)

// RoomController handles room (Ruangan) catalog endpoints under /tenant/rooms.
// All routes require scope=tenant and the relevant room.* permission (ADR 0012).
type RoomController struct {
	svc      *service.RoomSvc
	storage  service.Storage
	quota    uploadQuotaChecker // reuses the interface from therapist_controller.go
	maxBytes int64
}

// NewRoomController constructs a RoomController.
func NewRoomController(svc *service.RoomSvc, stor service.Storage, quota uploadQuotaChecker, maxBytes int64) *RoomController {
	return &RoomController{svc: svc, storage: stor, quota: quota, maxBytes: maxBytes}
}

// Register attaches all room routes to the provided secured group.
// PUT /reorder is registered BEFORE /:id to prevent Gin treating "reorder" as
// an id param (ADR 0012 §2.3, same lesson as addons).
func (h *RoomController) Register(rg *gin.RouterGroup, rbacMW func(code string) gin.HandlerFunc) {
	g := rg.Group("/tenant/rooms")
	g.GET("", rbacMW(constants.PermRoomRead), h.handleList)
	g.POST("", rbacMW(constants.PermRoomCreate), h.handleCreate)
	// PUT /reorder must be registered before /:id so Gin does not treat
	// "reorder" as an id parameter (mirrors addon_controller.go ADR 0012).
	g.PUT("/reorder", rbacMW(constants.PermRoomUpdate), h.handleReorder)
	g.GET("/:id", rbacMW(constants.PermRoomRead), h.handleGet)
	g.PATCH("/:id", rbacMW(constants.PermRoomUpdate), h.handleUpdate)
	g.PATCH("/:id/status", rbacMW(constants.PermRoomUpdate), h.handleChangeStatus)
	g.DELETE("/:id", rbacMW(constants.PermRoomDelete), h.handleDelete)
	// Photo upload/remove (ADR 0012 §2.5 — reuses ADR 0011 pipeline).
	g.POST("/:id/photo", rbacMW(constants.PermRoomUpdate), h.handleUploadPhoto)
	g.DELETE("/:id/photo", rbacMW(constants.PermRoomUpdate), h.handleRemovePhoto)
}

func (h *RoomController) handleList(c *gin.Context) {
	claims, ok := middleware.ClaimsFromContext(c)
	if !ok {
		helper.RespondError(c, http.StatusUnauthorized, constants.CodeUnauthenticated, "authentication required")
		return
	}

	var q ListRoomsQuery
	if err := c.ShouldBindQuery(&q); err != nil {
		helper.RespondBindError(c, err)
		return
	}

	page := q.Page
	if page < 1 {
		page = 1
	}
	limit := q.Limit
	if limit < 1 {
		limit = 10
	}

	out, err := h.svc.List(c.Request.Context(), service.ListRoomsInput{
		CallerTenantID: claims.TenantID,
		CallerBranches: claims.Branches,
		IsAdmin:        service.HasTenantAdminRole(claims.Roles),
		BranchID:       q.BranchID,
		IsActive:       q.IsActive,
		RoomType:       q.RoomType,
		Page:           page,
		Limit:          limit,
	})
	if err != nil {
		helper.RespondDomainError(c, err)
		return
	}

	ctx := c.Request.Context()
	items := make([]RoomResponse, len(out.Rooms))
	for i, rm := range out.Rooms {
		items[i] = h.toRoomResponse(ctx, rm)
	}
	c.JSON(http.StatusOK, ListRoomsResponse{
		Data:       items,
		Page:       out.Page,
		Limit:      limit,
		TotalCount: out.TotalCount,
		TotalPages: out.TotalPages,
	})
}

func (h *RoomController) handleCreate(c *gin.Context) {
	claims, ok := middleware.ClaimsFromContext(c)
	if !ok {
		helper.RespondError(c, http.StatusUnauthorized, constants.CodeUnauthenticated, "authentication required")
		return
	}

	var req CreateRoomRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		helper.RespondBindError(c, err)
		return
	}

	amenities := req.Amenities
	if amenities == nil {
		amenities = []string{}
	}

	out, err := h.svc.Create(c.Request.Context(), service.CreateRoomInput{
		CallerUserID:   claims.Subject,
		CallerTenantID: claims.TenantID,
		CallerBranches: claims.Branches,
		IsAdmin:        service.HasTenantAdminRole(claims.Roles),
		BranchID:       req.BranchID,
		Name:           req.Name,
		Description:    req.Description,
		RoomType:       req.RoomType,
		Capacity:       req.Capacity,
		Amenities:      amenities,
		SortOrder:      req.SortOrder,
	})
	if err != nil {
		helper.RespondDomainError(c, err)
		return
	}
	c.JSON(http.StatusCreated, h.toRoomResponse(c.Request.Context(), out))
}

func (h *RoomController) handleGet(c *gin.Context) {
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
	c.JSON(http.StatusOK, h.toRoomResponse(c.Request.Context(), out))
}

func (h *RoomController) handleUpdate(c *gin.Context) {
	claims, ok := middleware.ClaimsFromContext(c)
	if !ok {
		helper.RespondError(c, http.StatusUnauthorized, constants.CodeUnauthenticated, "authentication required")
		return
	}

	id := c.Param("id")

	var req UpdateRoomRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		helper.RespondBindError(c, err)
		return
	}

	// Detect whether Amenities was explicitly provided in the JSON body.
	// We use a raw JSON decode to distinguish "absent" from "empty array".
	// The binding above already validated the amenities values; we set
	// AmenitiesSet=true when the slice pointer is non-nil in the request.
	// Since ShouldBindJSON sets []string nil when absent vs []string{} when
	// provided, we check the raw request to distinguish "not provided" from "set to empty".
	// The simplest heuristic: if req.Amenities was bound (even as empty), treat as set.
	// Because gin binds nil for absent JSON arrays, we check:
	amenitiesSet := req.Amenities != nil

	out, err := h.svc.Update(c.Request.Context(), service.UpdateRoomInput{
		RoomID:          id,
		CallerUserID:    claims.Subject,
		CallerTenantID:  claims.TenantID,
		CallerBranches:  claims.Branches,
		IsAdmin:         service.HasTenantAdminRole(claims.Roles),
		BranchIDAttempt: req.BranchID,
		Name:            req.Name,
		Description:     req.Description,
		RoomType:        req.RoomType,
		Capacity:        req.Capacity,
		Amenities:       req.Amenities,
		AmenitiesSet:    amenitiesSet,
		SortOrder:       req.SortOrder,
	})
	if err != nil {
		helper.RespondDomainError(c, err)
		return
	}
	c.JSON(http.StatusOK, h.toRoomResponse(c.Request.Context(), out))
}

func (h *RoomController) handleChangeStatus(c *gin.Context) {
	claims, ok := middleware.ClaimsFromContext(c)
	if !ok {
		helper.RespondError(c, http.StatusUnauthorized, constants.CodeUnauthenticated, "authentication required")
		return
	}

	id := c.Param("id")

	var req ChangeRoomStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		helper.RespondBindError(c, err)
		return
	}

	out, err := h.svc.ChangeStatus(c.Request.Context(), service.ChangeRoomStatusInput{
		RoomID:         id,
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
	c.JSON(http.StatusOK, h.toRoomResponse(c.Request.Context(), out))
}

func (h *RoomController) handleDelete(c *gin.Context) {
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

func (h *RoomController) handleReorder(c *gin.Context) {
	claims, ok := middleware.ClaimsFromContext(c)
	if !ok {
		helper.RespondError(c, http.StatusUnauthorized, constants.CodeUnauthenticated, "authentication required")
		return
	}

	var req ReorderRoomsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		helper.RespondBindError(c, err)
		return
	}

	items := make([]service.RoomSortOrderItem, len(req.Items))
	for i, it := range req.Items {
		items[i] = service.RoomSortOrderItem{ID: it.ID, SortOrder: it.SortOrder}
	}

	if err := h.svc.Reorder(c.Request.Context(), service.ReorderRoomsInput{
		CallerUserID:   claims.Subject,
		CallerTenantID: claims.TenantID,
		CallerBranches: claims.Branches,
		IsAdmin:        service.HasTenantAdminRole(claims.Roles),
		BranchID:       req.BranchID,
		Items:          items,
	}); err != nil {
		helper.RespondDomainError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// ---------------------------------------------------------------------------
// ADR 0012 §2.5 — Photo upload/remove (reuses ADR 0011 pipeline exactly)
// ---------------------------------------------------------------------------

// handleUploadPhoto implements POST /api/v1/tenant/rooms/:id/photo.
// Pipeline order per ADR 0012 §2.5 / ADR 0011 §2.4:
//
//  1. Body-size hard cap via MaxBytesReader.
//  2. Explicit multipart parse.
//  3. MIME sniff + structural validation + EXIF strip via helper.ProcessUpload.
//  4. Per-tenant quota check (shared TenantQuota instance with therapist photos).
//  5. Key generation.
//  6. Storage.Upload.
//  7. DB UPDATE inside transaction (via service.UpdatePhotoKey).
//  8. Fire-and-forget old-key delete.
func (h *RoomController) handleUploadPhoto(c *gin.Context) {
	claims, ok := middleware.ClaimsFromContext(c)
	if !ok {
		helper.RespondError(c, http.StatusUnauthorized, constants.CodeUnauthenticated, "authentication required")
		return
	}
	roomID := c.Param("id")
	ctx := c.Request.Context()

	// Step 1: hard body-size cap.
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

	// Step 4: per-tenant quota check (shared with therapist uploads).
	if err := h.quota.Check(claims.TenantID); err != nil {
		helper.RespondDomainError(c, err)
		return
	}

	// Step 5: generate unique storage key.
	newKey, err := generateRoomPhotoKey(roomID, helper.ImageExt(mime))
	if err != nil {
		helper.RespondDomainError(c, fmt.Errorf("key generation failed: %w", err))
		return
	}

	// Step 6: upload to storage.
	if err := h.storage.Upload(ctx, newKey, bytes.NewReader(stripped), mime); err != nil {
		commoncfglog.Errorf(ctx, err, "room storage upload failed",
			"tenant_id", claims.TenantID,
			"photo_key", newKey,
		)
		helper.RespondError(c, http.StatusInternalServerError, constants.CodeInternal,
			"gagal menyimpan foto")
		return
	}

	// Step 7: DB update inside transaction (service enforces tenant + branch rules).
	oldKey, err := h.svc.UpdatePhotoKey(ctx, service.UploadRoomPhotoInput{
		RoomID:         roomID,
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
				commoncfglog.Errorf(bgCtx, delErr, "cleanup after failed room db update",
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
				commoncfglog.Errorf(bgCtx, err, "delete old room photo key",
					"photo_key", key, "tenant_id", claims.TenantID)
			}
		}(*oldKey)
	}

	// Return updated room detail.
	out, err := h.svc.Get(ctx, claims.TenantID, roomID)
	if err != nil {
		helper.RespondDomainError(c, err)
		return
	}
	c.JSON(http.StatusOK, h.toRoomResponse(ctx, out))
}

// handleRemovePhoto implements DELETE /api/v1/tenant/rooms/:id/photo.
func (h *RoomController) handleRemovePhoto(c *gin.Context) {
	claims, ok := middleware.ClaimsFromContext(c)
	if !ok {
		helper.RespondError(c, http.StatusUnauthorized, constants.CodeUnauthenticated, "authentication required")
		return
	}
	roomID := c.Param("id")
	ctx := c.Request.Context()

	oldKey, err := h.svc.RemovePhotoKey(ctx, service.RemoveRoomPhotoInput{
		RoomID:         roomID,
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
				commoncfglog.Errorf(bgCtx, err, "delete removed room photo key",
					"photo_key", key, "tenant_id", claims.TenantID)
			}
		}(*oldKey)
	}

	c.Status(http.StatusNoContent)
}

// ---------------------------------------------------------------------------
// Mapping helpers
// ---------------------------------------------------------------------------

// toRoomResponse converts a RoomDetail to its HTTP response DTO, resolving the
// photo key to a public URL at the controller boundary (ADR 0011 §2.1 / ADR 0012 §2.3).
// photo_key is NEVER included in the response; tenant_id is also omitted.
func (h *RoomController) toRoomResponse(ctx context.Context, d service.RoomDetail) RoomResponse {
	amenities := d.Amenities
	if amenities == nil {
		amenities = []string{}
	}
	resp := RoomResponse{
		ID:          d.ID,
		BranchID:    d.BranchID,
		Name:        d.Name,
		Description: d.Description,
		RoomType:    d.RoomType,
		Capacity:    d.Capacity,
		Amenities:   amenities,
		IsActive:    d.IsActive,
		SortOrder:   d.SortOrder,
		CreatedAt:   d.CreatedAt,
		UpdatedAt:   d.UpdatedAt,
	}
	if d.PhotoKey != nil && *d.PhotoKey != "" {
		url, err := h.storage.URL(ctx, *d.PhotoKey)
		if err != nil {
			commoncfglog.Errorf(ctx, err, "resolve room photo_url",
				"photo_key", *d.PhotoKey)
		} else {
			resp.PhotoURL = &url
		}
	}
	return resp
}

// ---------------------------------------------------------------------------
// Private helpers
// ---------------------------------------------------------------------------

// generateRoomPhotoKey produces a storage key of the form
// "rooms/{roomID}/{16-random-hex}.{ext}" per ADR 0012 §2.5.
func generateRoomPhotoKey(roomID, ext string) (string, error) {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate room photo key: %w", err)
	}
	return fmt.Sprintf("rooms/%s/%s.%s", roomID, hex.EncodeToString(b), ext), nil
}
