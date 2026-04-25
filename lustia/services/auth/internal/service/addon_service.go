package service

import (
	"context"
	"fmt"
	"time"
	"unicode/utf8"

	"github.com/chrisanlung/lustia-auth/internal/constants"
	"github.com/chrisanlung/lustia-auth/internal/model"
	"github.com/google/uuid"
)

// AddonService handles tenant-wide add-on catalog CRUD (ADR 0010).
// tenant_id always comes from the JWT claims — never from the request body.
type AddonService struct {
	addons AddonRepository
	audit  AuditRepository
	clock  Clock
	tx     TxManager
}

// NewAddonService constructs an AddonService.
func NewAddonService(
	addons AddonRepository,
	audit AuditRepository,
	clock Clock,
	tx TxManager,
) *AddonService {
	return &AddonService{
		addons: addons,
		audit:  audit,
		clock:  clock,
		tx:     tx,
	}
}

// Create inserts a new add-on in the caller's tenant catalog.
func (s *AddonService) Create(ctx context.Context, in CreateAddonInput) (AddonDetail, error) {
	if err := validateAddonInput(in.Name, in.Description, in.PriceIDR, in.SortOrder); err != nil {
		return AddonDetail{}, err
	}

	id := uuid.New().String()
	a := &model.Addon{
		ID:          id,
		TenantID:    in.CallerTenantID,
		Name:        in.Name,
		Description: in.Description,
		PriceIDR:    in.PriceIDR,
		IsActive:    true,
		SortOrder:   in.SortOrder,
		CreatedBy:   &in.CallerUserID,
		UpdatedBy:   &in.CallerUserID,
	}

	if err := s.addons.Save(ctx, a); err != nil {
		return AddonDetail{}, fmt.Errorf("create addon: %w", err)
	}

	_ = s.audit.Append(context.Background(), AuditEntry{
		TenantID:     &in.CallerTenantID,
		ActorUserID:  &in.CallerUserID,
		Action:       "addon.created",
		ResourceType: "addon",
		ResourceID:   a.ID,
		Meta:         map[string]interface{}{"addon_id": a.ID},
	})

	return toAddonDetail(a), nil
}

// List returns an offset-paginated list of add-ons for the caller's tenant.
func (s *AddonService) List(ctx context.Context, in ListAddonsInput) (ListAddonsOutput, error) {
	limit := in.Limit
	if limit <= 0 || limit > 200 {
		limit = 10
	}
	page := in.Page
	if page < 1 {
		page = 1
	}

	rows, total, err := s.addons.FindByTenant(ctx, in.CallerTenantID, AddonFilter{
		IsActive: in.IsActive,
		Page:     page,
		Limit:    limit,
	})
	if err != nil {
		return ListAddonsOutput{}, fmt.Errorf("list addons: %w", err)
	}

	details := make([]AddonDetail, len(rows))
	for i, a := range rows {
		details[i] = toAddonDetail(a)
	}

	totalPages := 0
	if total > 0 {
		totalPages = int((total + int64(limit) - 1) / int64(limit))
	}
	return ListAddonsOutput{Addons: details, Page: page, TotalCount: total, TotalPages: totalPages}, nil
}

// Get returns a single add-on by ID, enforcing tenant isolation.
func (s *AddonService) Get(ctx context.Context, callerTenantID, addonID string) (AddonDetail, error) {
	a, err := s.addons.FindByID(ctx, addonID)
	if err != nil {
		return AddonDetail{}, err
	}
	if a.TenantID != callerTenantID {
		// Treat cross-tenant access as not found (IDOR guard).
		return AddonDetail{}, constants.ErrAddonNotFound
	}
	return toAddonDetail(a), nil
}

// Update applies partial-field updates to an existing add-on.
func (s *AddonService) Update(ctx context.Context, in UpdateAddonInput) (AddonDetail, error) {
	a, err := s.addons.FindByID(ctx, in.AddonID)
	if err != nil {
		return AddonDetail{}, err
	}
	if a.TenantID != in.CallerTenantID {
		return AddonDetail{}, constants.ErrAddonNotFound
	}

	// Apply partial updates.
	if in.Name != nil {
		a.Name = *in.Name
	}
	if in.Description != nil {
		a.Description = in.Description
	}
	if in.PriceIDR != nil {
		a.PriceIDR = *in.PriceIDR
	}
	if in.SortOrder != nil {
		a.SortOrder = *in.SortOrder
	}
	a.UpdatedBy = &in.CallerUserID

	// Re-validate the merged values.
	if err := validateAddonInput(a.Name, a.Description, a.PriceIDR, a.SortOrder); err != nil {
		return AddonDetail{}, err
	}

	if err := s.addons.Update(ctx, a); err != nil {
		return AddonDetail{}, fmt.Errorf("update addon: %w", err)
	}

	_ = s.audit.Append(context.Background(), AuditEntry{
		TenantID:     &in.CallerTenantID,
		ActorUserID:  &in.CallerUserID,
		Action:       "addon.updated",
		ResourceType: "addon",
		ResourceID:   a.ID,
	})

	return toAddonDetail(a), nil
}

// ChangeStatus activates or deactivates an add-on.
func (s *AddonService) ChangeStatus(ctx context.Context, in ChangeAddonStatusInput) (AddonDetail, error) {
	a, err := s.addons.FindByID(ctx, in.AddonID)
	if err != nil {
		return AddonDetail{}, err
	}
	if a.TenantID != in.CallerTenantID {
		return AddonDetail{}, constants.ErrAddonNotFound
	}

	if err := s.addons.UpdateStatus(ctx, in.AddonID, in.IsActive, in.CallerUserID); err != nil {
		return AddonDetail{}, fmt.Errorf("change addon status: %w", err)
	}
	a.IsActive = in.IsActive

	action := "addon.activated"
	if !in.IsActive {
		action = "addon.deactivated"
	}
	_ = s.audit.Append(context.Background(), AuditEntry{
		TenantID:     &in.CallerTenantID,
		ActorUserID:  &in.CallerUserID,
		Action:       action,
		ResourceType: "addon",
		ResourceID:   a.ID,
		Meta:         map[string]interface{}{"is_active": in.IsActive},
	})

	return toAddonDetail(a), nil
}

// SoftDelete soft-deletes an add-on (sets deleted_at; no hard DELETE).
func (s *AddonService) SoftDelete(ctx context.Context, callerTenantID, callerUserID, addonID string) error {
	a, err := s.addons.FindByID(ctx, addonID)
	if err != nil {
		return err
	}
	if a.TenantID != callerTenantID {
		return constants.ErrAddonNotFound
	}

	if err := s.addons.SoftDelete(ctx, addonID, callerUserID); err != nil {
		return fmt.Errorf("soft delete addon: %w", err)
	}

	_ = s.audit.Append(context.Background(), AuditEntry{
		TenantID:     &callerTenantID,
		ActorUserID:  &callerUserID,
		Action:       "addon.deleted",
		ResourceType: "addon",
		ResourceID:   addonID,
	})
	return nil
}

// Reorder atomically updates sort_order for a batch of add-ons.
// All items must belong to the caller's tenant; any foreign ID aborts the
// whole operation (IDOR guard — validated inside the transaction).
func (s *AddonService) Reorder(ctx context.Context, in ReorderAddonsInput) error {
	if len(in.Items) == 0 {
		return fmt.Errorf("%w: reorder items must not be empty", constants.ErrInvalidInput)
	}
	if len(in.Items) > 200 {
		return fmt.Errorf("%w: reorder items must not exceed 200", constants.ErrInvalidInput)
	}

	return s.tx.WithTx(ctx, func(txCtx context.Context) error {
		// Collect IDs and build lookup map for O(1) membership test.
		ids := make([]string, len(in.Items))
		for i, it := range in.Items {
			ids[i] = it.ID
		}

		// Batch-fetch all IDs in a single query inside the tx (IDOR guard).
		rows, err := s.addons.FindByIDs(txCtx, ids)
		if err != nil {
			return fmt.Errorf("reorder: fetch addons: %w", err)
		}

		// Build set of found IDs for O(1) lookup.
		found := make(map[string]string, len(rows)) // id → tenantID
		for _, r := range rows {
			found[r.ID] = r.TenantID
		}

		// Validate: every requested ID must exist and belong to the caller's tenant.
		for _, it := range in.Items {
			tenantID, ok := found[it.ID]
			if !ok {
				return fmt.Errorf("%w: add-on %s not found", constants.ErrAddonNotFound, it.ID)
			}
			if tenantID != in.CallerTenantID {
				// Treat cross-tenant as not found (IDOR guard).
				return fmt.Errorf("%w: add-on %s not found", constants.ErrAddonNotFound, it.ID)
			}
			if it.SortOrder < 0 || it.SortOrder > 9999 {
				return fmt.Errorf("%w: sort_order must be 0–9999", constants.ErrInvalidInput)
			}
		}

		if err := s.addons.BulkUpdateSortOrder(txCtx, in.Items); err != nil {
			return fmt.Errorf("reorder: bulk update: %w", err)
		}

		_ = s.audit.Append(context.Background(), AuditEntry{
			TenantID:     &in.CallerTenantID,
			ActorUserID:  &in.CallerUserID,
			Action:       "addon.reordered",
			ResourceType: "addon",
			ResourceID:   "",
			Meta:         map[string]interface{}{"count": len(in.Items)},
		})
		return nil
	})
}

// ---------------------------------------------------------------------------
// Private helpers
// ---------------------------------------------------------------------------

// validateAddonInput enforces service-layer invariants common to create and update.
func validateAddonInput(name string, description *string, priceIDR int64, sortOrder int) error {
	nameLen := utf8.RuneCountInString(name)
	if nameLen < 1 || nameLen > 120 {
		return fmt.Errorf("%w: nama add-on harus 1–120 karakter", constants.ErrInvalidInput)
	}
	if description != nil {
		descLen := utf8.RuneCountInString(*description)
		if descLen > 500 {
			return fmt.Errorf("%w: deskripsi add-on maksimal 500 karakter", constants.ErrInvalidInput)
		}
	}
	if priceIDR < 0 {
		return fmt.Errorf("%w: harga tidak boleh negatif", constants.ErrInvalidInput)
	}
	if sortOrder < 0 || sortOrder > 9999 {
		return fmt.Errorf("%w: sort_order harus 0–9999", constants.ErrInvalidInput)
	}
	return nil
}

// toAddonDetail converts a model.Addon to an AddonDetail DTO.
func toAddonDetail(a *model.Addon) AddonDetail {
	return AddonDetail{
		ID:          a.ID,
		Name:        a.Name,
		Description: a.Description,
		PriceIDR:    a.PriceIDR,
		IsActive:    a.IsActive,
		SortOrder:   a.SortOrder,
		CreatedAt:   a.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:   a.UpdatedAt.UTC().Format(time.RFC3339),
	}
}
