package service

import (
	"context"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/chrisanlung/lustia-auth/internal/constants"
	"github.com/chrisanlung/lustia-auth/internal/model"
	"github.com/lib/pq"

	"github.com/google/uuid"
)

// RoomSvc handles room (Ruangan) catalog CRUD and the branch-scope access rule
// defined in ADR 0012 §2.2 and §2.2.
type RoomSvc struct {
	rooms    RoomRepository
	branches BranchRepository
	audit    AuditRepository
	clock    Clock
	tx       TxManager
}

// NewRoomSvc constructs a RoomSvc.
func NewRoomSvc(
	rooms RoomRepository,
	branches BranchRepository,
	audit AuditRepository,
	clock Clock,
	tx TxManager,
) *RoomSvc {
	return &RoomSvc{
		rooms:    rooms,
		branches: branches,
		audit:    audit,
		clock:    clock,
		tx:       tx,
	}
}

// Create inserts a new room in the caller's tenant. Cross-branch check applied
// for branch_admin callers per ADR 0012 §2.2.
func (s *RoomSvc) Create(ctx context.Context, in CreateRoomInput) (RoomDetail, error) {
	// Branch-scope check: branch_admin may only create rooms in their branches.
	if !in.IsAdmin {
		if !containsBranch(in.CallerBranches, in.BranchID) {
			return RoomDetail{}, constants.ErrCrossBranchForbidden
		}
	}

	// Verify the branch exists and belongs to the caller's tenant.
	b, err := s.branches.FindByID(ctx, in.BranchID)
	if err != nil {
		return RoomDetail{}, err
	}
	if b.TenantID != in.CallerTenantID {
		return RoomDetail{}, constants.ErrBranchNotFound
	}

	// Validate fields.
	if err := validateRoomInput(in.Name, in.Description, string(in.RoomType), int(in.Capacity), in.Amenities, in.SortOrder); err != nil {
		return RoomDetail{}, err
	}

	normalizedAmenities := normalizeAmenities(in.Amenities)

	id := uuid.New().String()
	rm := &model.Room{
		ID:          id,
		TenantID:    in.CallerTenantID,
		BranchID:    in.BranchID,
		Name:        in.Name,
		Description: in.Description,
		RoomType:    in.RoomType,
		Capacity:    in.Capacity,
		Amenities:   sliceToAmenities(normalizedAmenities),
		IsActive:    true,
		SortOrder:   in.SortOrder,
		CreatedBy:   &in.CallerUserID,
		UpdatedBy:   &in.CallerUserID,
	}

	if err := s.rooms.Save(ctx, rm); err != nil {
		return RoomDetail{}, fmt.Errorf("create room: %w", err)
	}

	_ = s.audit.Append(context.Background(), AuditEntry{
		TenantID:     &in.CallerTenantID,
		ActorUserID:  &in.CallerUserID,
		Action:       "room.created",
		ResourceType: "room",
		ResourceID:   rm.ID,
		Meta:         map[string]interface{}{"branch_id": rm.BranchID},
	})

	return toRoomDetail(rm), nil
}

// List returns an offset-paginated list of rooms for the caller's tenant.
func (s *RoomSvc) List(ctx context.Context, in ListRoomsInput) (ListRoomsOutput, error) {
	limit := in.Limit
	if limit <= 0 || limit > 200 {
		limit = 10
	}
	page := in.Page
	if page < 1 {
		page = 1
	}

	filter := RoomFilter{
		BranchID: in.BranchID,
		IsActive: in.IsActive,
		RoomType: in.RoomType,
		Page:     page,
		Limit:    limit,
	}

	rows, total, err := s.rooms.FindByTenant(ctx, in.CallerTenantID, filter)
	if err != nil {
		return ListRoomsOutput{}, fmt.Errorf("list rooms: %w", err)
	}

	details := make([]RoomDetail, len(rows))
	for i, rm := range rows {
		details[i] = toRoomDetail(rm)
	}

	totalPages := 0
	if total > 0 {
		totalPages = int((total + int64(limit) - 1) / int64(limit))
	}
	return ListRoomsOutput{Rooms: details, Page: page, TotalCount: total, TotalPages: totalPages}, nil
}

// Get returns a single room by ID, enforcing tenant isolation.
func (s *RoomSvc) Get(ctx context.Context, callerTenantID, roomID string) (RoomDetail, error) {
	rm, err := s.rooms.FindByID(ctx, roomID)
	if err != nil {
		return RoomDetail{}, err
	}
	if rm.TenantID != callerTenantID {
		return RoomDetail{}, constants.ErrRoomNotFound
	}
	return toRoomDetail(rm), nil
}

// Update applies partial-field updates to an existing room. Branch is immutable.
func (s *RoomSvc) Update(ctx context.Context, in UpdateRoomInput) (RoomDetail, error) {
	rm, err := s.rooms.FindByID(ctx, in.RoomID)
	if err != nil {
		return RoomDetail{}, err
	}
	if rm.TenantID != in.CallerTenantID {
		return RoomDetail{}, constants.ErrRoomNotFound
	}

	// branch_id is immutable — reject any attempt to change it.
	if in.BranchIDAttempt != nil && *in.BranchIDAttempt != rm.BranchID {
		return RoomDetail{}, fmt.Errorf("%w", constants.ErrRoomBranchImmutable)
	}

	// Branch-scope check on the existing room's branch.
	if !in.IsAdmin {
		if !containsBranch(in.CallerBranches, rm.BranchID) {
			return RoomDetail{}, constants.ErrCrossBranchForbidden
		}
	}

	// Apply partial updates.
	if in.Name != nil {
		rm.Name = *in.Name
	}
	if in.Description != nil {
		rm.Description = in.Description
	}
	if in.RoomType != nil {
		rm.RoomType = *in.RoomType
	}
	if in.Capacity != nil {
		rm.Capacity = *in.Capacity
	}
	if in.AmenitiesSet {
		normalized := normalizeAmenities(in.Amenities)
		rm.Amenities = sliceToAmenities(normalized)
	}
	if in.SortOrder != nil {
		rm.SortOrder = *in.SortOrder
	}
	rm.UpdatedBy = &in.CallerUserID

	// Re-validate the merged values.
	if err := validateRoomInput(rm.Name, rm.Description, rm.RoomType, int(rm.Capacity), rm.AmenitiesSlice(), rm.SortOrder); err != nil {
		return RoomDetail{}, err
	}

	if err := s.rooms.Update(ctx, rm); err != nil {
		return RoomDetail{}, fmt.Errorf("update room: %w", err)
	}

	_ = s.audit.Append(context.Background(), AuditEntry{
		TenantID:     &in.CallerTenantID,
		ActorUserID:  &in.CallerUserID,
		Action:       "room.updated",
		ResourceType: "room",
		ResourceID:   rm.ID,
	})

	return toRoomDetail(rm), nil
}

// ChangeStatus activates or deactivates a room.
func (s *RoomSvc) ChangeStatus(ctx context.Context, in ChangeRoomStatusInput) (RoomDetail, error) {
	rm, err := s.rooms.FindByID(ctx, in.RoomID)
	if err != nil {
		return RoomDetail{}, err
	}
	if rm.TenantID != in.CallerTenantID {
		return RoomDetail{}, constants.ErrRoomNotFound
	}
	if !in.IsAdmin {
		if !containsBranch(in.CallerBranches, rm.BranchID) {
			return RoomDetail{}, constants.ErrCrossBranchForbidden
		}
	}

	if err := s.rooms.UpdateStatus(ctx, in.RoomID, in.IsActive, in.CallerUserID); err != nil {
		return RoomDetail{}, fmt.Errorf("change room status: %w", err)
	}
	rm.IsActive = in.IsActive

	action := "room.activated"
	if !in.IsActive {
		action = "room.deactivated"
	}
	_ = s.audit.Append(context.Background(), AuditEntry{
		TenantID:     &in.CallerTenantID,
		ActorUserID:  &in.CallerUserID,
		Action:       action,
		ResourceType: "room",
		ResourceID:   rm.ID,
		Meta:         map[string]interface{}{"is_active": in.IsActive},
	})

	return toRoomDetail(rm), nil
}

// SoftDelete soft-deletes a room (sets deleted_at; no hard DELETE).
func (s *RoomSvc) SoftDelete(ctx context.Context, callerTenantID, callerUserID string, callerBranches []string, isAdmin bool, roomID string) error {
	rm, err := s.rooms.FindByID(ctx, roomID)
	if err != nil {
		return err
	}
	if rm.TenantID != callerTenantID {
		return constants.ErrRoomNotFound
	}
	if !isAdmin {
		if !containsBranch(callerBranches, rm.BranchID) {
			return constants.ErrCrossBranchForbidden
		}
	}

	if err := s.rooms.SoftDelete(ctx, roomID, callerUserID); err != nil {
		return fmt.Errorf("soft delete room: %w", err)
	}

	_ = s.audit.Append(context.Background(), AuditEntry{
		TenantID:     &callerTenantID,
		ActorUserID:  &callerUserID,
		Action:       "room.deleted",
		ResourceType: "room",
		ResourceID:   roomID,
	})
	return nil
}

// Reorder atomically updates sort_order for a batch of rooms.
// All items must belong to the same branch (caller-supplied) and the caller's
// tenant. Branch-scope check applies. Validated inside transaction.
func (s *RoomSvc) Reorder(ctx context.Context, in ReorderRoomsInput) error {
	if len(in.Items) == 0 {
		return fmt.Errorf("%w: reorder items must not be empty", constants.ErrInvalidInput)
	}
	if len(in.Items) > 200 {
		return fmt.Errorf("%w: reorder items must not exceed 200", constants.ErrInvalidInput)
	}

	// Branch-scope check: branch_admin may only reorder rooms in their branches.
	if !in.IsAdmin {
		if !containsBranch(in.CallerBranches, in.BranchID) {
			return constants.ErrCrossBranchForbidden
		}
	}

	return s.tx.WithTx(ctx, func(txCtx context.Context) error {
		ids := make([]string, len(in.Items))
		for i, it := range in.Items {
			ids[i] = it.ID
		}

		// Batch-fetch all IDs in a single query inside the tx (IDOR guard).
		rows, err := s.rooms.FindByIDs(txCtx, ids)
		if err != nil {
			return fmt.Errorf("reorder: fetch rooms: %w", err)
		}

		// Build lookup map: id → room.
		found := make(map[string]*model.Room, len(rows))
		for _, rm := range rows {
			found[rm.ID] = rm
		}

		// Validate: every requested ID must exist, belong to the caller's tenant,
		// and belong to the requested branch. Mixed-branch reorder is rejected.
		for _, it := range in.Items {
			rm, ok := found[it.ID]
			if !ok {
				return fmt.Errorf("%w: room %s not found", constants.ErrRoomNotFound, it.ID)
			}
			if rm.TenantID != in.CallerTenantID {
				return fmt.Errorf("%w: room %s not found", constants.ErrRoomNotFound, it.ID)
			}
			if rm.BranchID != in.BranchID {
				return fmt.Errorf("%w: semua ruangan harus dari cabang yang sama", constants.ErrInvalidInput)
			}
			if it.SortOrder < 0 || it.SortOrder > 9999 {
				return fmt.Errorf("%w: sort_order harus 0–9999", constants.ErrInvalidInput)
			}
		}

		if err := s.rooms.BulkUpdateSortOrder(txCtx, in.Items); err != nil {
			return fmt.Errorf("reorder: bulk update: %w", err)
		}

		_ = s.audit.Append(context.Background(), AuditEntry{
			TenantID:     &in.CallerTenantID,
			ActorUserID:  &in.CallerUserID,
			Action:       "room.reordered",
			ResourceType: "room",
			ResourceID:   "",
			Meta:         map[string]interface{}{"count": len(in.Items), "branch_id": in.BranchID},
		})
		return nil
	})
}

// UpdatePhotoKey atomically stores the new photo key and returns the old key
// (so the caller can schedule a background delete). Enforces tenant ownership
// and cross-branch rules.
func (s *RoomSvc) UpdatePhotoKey(ctx context.Context, in UploadRoomPhotoInput) (oldKey *string, err error) {
	rm, err := s.rooms.FindByID(ctx, in.RoomID)
	if err != nil {
		return nil, err
	}
	if rm.TenantID != in.CallerTenantID {
		return nil, constants.ErrRoomNotFound
	}
	if !in.IsAdmin {
		if !containsBranch(in.CallerBranches, rm.BranchID) {
			return nil, constants.ErrCrossBranchForbidden
		}
	}

	newKey := in.NewKey
	oldKey, err = s.rooms.UpdatePhotoKey(ctx, in.RoomID, &newKey, in.CallerUserID)
	if err != nil {
		return nil, fmt.Errorf("update room photo key: %w", err)
	}

	_ = s.audit.Append(context.Background(), AuditEntry{
		TenantID:     &in.CallerTenantID,
		ActorUserID:  &in.CallerUserID,
		Action:       "room.photo_uploaded",
		ResourceType: "room",
		ResourceID:   in.RoomID,
		Meta:         map[string]interface{}{"new_key": newKey},
	})
	return oldKey, nil
}

// RemovePhotoKey clears the photo_key for a room row and returns the old key
// for the caller to schedule a background storage delete.
func (s *RoomSvc) RemovePhotoKey(ctx context.Context, in RemoveRoomPhotoInput) (oldKey *string, err error) {
	rm, err := s.rooms.FindByID(ctx, in.RoomID)
	if err != nil {
		return nil, err
	}
	if rm.TenantID != in.CallerTenantID {
		return nil, constants.ErrRoomNotFound
	}
	if !in.IsAdmin {
		if !containsBranch(in.CallerBranches, rm.BranchID) {
			return nil, constants.ErrCrossBranchForbidden
		}
	}

	oldKey, err = s.rooms.UpdatePhotoKey(ctx, in.RoomID, nil, in.CallerUserID)
	if err != nil {
		return nil, fmt.Errorf("remove room photo key: %w", err)
	}

	_ = s.audit.Append(context.Background(), AuditEntry{
		TenantID:     &in.CallerTenantID,
		ActorUserID:  &in.CallerUserID,
		Action:       "room.photo_removed",
		ResourceType: "room",
		ResourceID:   in.RoomID,
	})
	return oldKey, nil
}

// ---------------------------------------------------------------------------
// Private helpers
// ---------------------------------------------------------------------------

// validateRoomInput enforces service-layer invariants common to create and update.
func validateRoomInput(name string, description *string, roomType string, capacity int, amenities []string, sortOrder int) error {
	nameLen := utf8.RuneCountInString(name)
	if nameLen < 1 || nameLen > 120 {
		return fmt.Errorf("%w: nama ruangan harus 1–120 karakter", constants.ErrInvalidInput)
	}

	if description != nil {
		descLen := utf8.RuneCountInString(*description)
		if descLen > 500 {
			return fmt.Errorf("%w: deskripsi ruangan maksimal 500 karakter", constants.ErrInvalidInput)
		}
	}

	if !validRoomType(roomType) {
		return fmt.Errorf("%w: room_type harus salah satu dari single, couple, group, vip", constants.ErrInvalidInput)
	}

	if capacity < 1 || capacity > 20 {
		return fmt.Errorf("%w: kapasitas harus 1–20", constants.ErrInvalidInput)
	}

	if len(amenities) > 20 {
		return fmt.Errorf("%w: maksimal 20 amenitas", constants.ErrInvalidInput)
	}
	for _, a := range amenities {
		if utf8.RuneCountInString(a) > 80 {
			return fmt.Errorf("%w: setiap amenitas maksimal 80 karakter", constants.ErrInvalidInput)
		}
	}

	if sortOrder < 0 || sortOrder > 9999 {
		return fmt.Errorf("%w: sort_order harus 0–9999", constants.ErrInvalidInput)
	}

	return nil
}

// validRoomType returns true when v is one of the four allowed room type values
// matching the CHECK constraint in migration 000021.
func validRoomType(v string) bool {
	switch v {
	case "single", "couple", "group", "vip":
		return true
	default:
		return false
	}
}

// normalizeAmenities trims whitespace, lowercases, deduplicates, and drops empty
// strings from the input slice. Returns an empty slice (never nil) when all
// entries are blank.
func normalizeAmenities(in []string) []string {
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, a := range in {
		a = strings.TrimSpace(a)
		a = strings.ToLower(a)
		if a == "" {
			continue
		}
		if _, dup := seen[a]; dup {
			continue
		}
		seen[a] = struct{}{}
		out = append(out, a)
	}
	return out
}

// sliceToAmenities converts a []string to pq.StringArray for the TEXT[] column.
// Always returns a non-nil empty slice (never nil) so the NOT NULL DB constraint
// is satisfied even when the caller passes an empty list.
func sliceToAmenities(in []string) pq.StringArray {
	if len(in) == 0 {
		return pq.StringArray{}
	}
	return pq.StringArray(in)
}

// toRoomDetail converts a model.Room to a RoomDetail DTO.
func toRoomDetail(rm *model.Room) RoomDetail {
	amenities := rm.AmenitiesSlice()
	if amenities == nil {
		amenities = []string{}
	}
	return RoomDetail{
		ID:          rm.ID,
		BranchID:    rm.BranchID,
		Name:        rm.Name,
		Description: rm.Description,
		RoomType:    rm.RoomType,
		Capacity:    rm.Capacity,
		Amenities:   amenities,
		PhotoKey:    rm.PhotoKey,
		IsActive:    rm.IsActive,
		SortOrder:   rm.SortOrder,
		CreatedAt:   rm.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:   rm.UpdatedAt.UTC().Format(time.RFC3339),
	}
}
