package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/chrisanlung/lustia-auth/internal/constants"
	"github.com/chrisanlung/lustia-auth/internal/model"
	"github.com/google/uuid"
)

// TherapistSvc handles therapist CRUD and the cross-branch access rule defined
// in ADR 0009 §11.2 and §11.3 flag #1.
type TherapistSvc struct {
	therapists   TherapistRepository
	mappings     TherapistServiceRepository
	services     ServiceCatalogRepository
	branches     BranchRepository
	audit        AuditRepository
	clock        Clock
	tx           TxManager
}

// NewTherapistSvc constructs a TherapistSvc.
func NewTherapistSvc(
	therapists TherapistRepository,
	mappings TherapistServiceRepository,
	services ServiceCatalogRepository,
	branches BranchRepository,
	audit AuditRepository,
	clock Clock,
	tx TxManager,
) *TherapistSvc {
	return &TherapistSvc{
		therapists: therapists,
		mappings:   mappings,
		services:   services,
		branches:   branches,
		audit:      audit,
		clock:      clock,
		tx:         tx,
	}
}

// Create creates a new therapist profile. Cross-branch check applied for
// branch_admin callers per §11.3 flag #1.
func (s *TherapistSvc) Create(ctx context.Context, in CreateTherapistInput) (TherapistDetail, error) {
	if !in.IsAdmin {
		if !containsBranch(in.CallerBranches, in.BranchID) {
			return TherapistDetail{}, constants.ErrCrossBranchForbidden
		}
	}

	// Verify the branch exists and belongs to the tenant.
	b, err := s.branches.FindByID(ctx, in.BranchID)
	if err != nil {
		return TherapistDetail{}, err // ErrBranchNotFound propagates as 404
	}
	if b.TenantID != in.CallerTenantID {
		return TherapistDetail{}, constants.ErrBranchNotFound
	}

	var joinedAt *time.Time
	if in.JoinedAt != nil && *in.JoinedAt != "" {
		t, err := time.Parse("2006-01-02", *in.JoinedAt)
		if err != nil {
			return TherapistDetail{}, fmt.Errorf("%w: joined_at must be YYYY-MM-DD", constants.ErrInvalidInput)
		}
		joinedAt = &t
	}

	t := &model.Therapist{
		ID:          uuid.New().String(),
		TenantID:    in.CallerTenantID,
		BranchID:    in.BranchID,
		UserID:      in.UserID,
		FullName:    in.FullName,
		Gender:      in.Gender,
		Phone:       in.Phone,
		Email:       in.Email,
		Bio:         in.Bio,
		PhotoURL:    in.PhotoURL,
		Specialties: "[]", // JSONB NOT NULL — explicit default (Phase 4 BUG-P4-A)
		IsActive:    true,
		JoinedAt:    joinedAt,
		CreatedBy:   &in.CallerUserID,
		UpdatedBy:   &in.CallerUserID,
	}

	if err := s.therapists.Save(ctx, t); err != nil {
		return TherapistDetail{}, fmt.Errorf("create therapist: %w", err)
	}

	bgCtx := context.Background()
	_ = s.audit.Append(bgCtx, AuditEntry{
		TenantID:     &in.CallerTenantID,
		ActorUserID:  &in.CallerUserID,
		Action:       "therapist.created",
		ResourceType: "therapist",
		ResourceID:   t.ID,
		Meta:         map[string]interface{}{"branch_id": t.BranchID},
	})

	return toTherapistDetail(t), nil
}

// List returns a paginated list of therapists for the caller's tenant.
func (s *TherapistSvc) List(ctx context.Context, in ListTherapistsInput) (ListTherapistsOutput, error) {
	filter := TherapistFilter{
		BranchID: in.BranchID,
		IsActive: in.IsActive,
		Cursor:   in.Cursor,
		Limit:    in.Limit,
	}

	// branch_admin sees only their assigned branches.
	if !in.IsAdmin && len(in.CallerBranches) > 0 {
		filter.BranchIDs = in.CallerBranches
	}

	rows, cursor, err := s.therapists.FindByTenant(ctx, in.CallerTenantID, filter)
	if err != nil {
		return ListTherapistsOutput{}, fmt.Errorf("list therapists: %w", err)
	}

	details := make([]TherapistDetail, len(rows))
	for i, t := range rows {
		details[i] = toTherapistDetail(t)
	}
	return ListTherapistsOutput{Therapists: details, NextCursor: cursor}, nil
}

// Get returns a single therapist with all service mappings (active + inactive).
func (s *TherapistSvc) Get(ctx context.Context, callerTenantID, therapistID string) (TherapistDetailWithServices, error) {
	t, err := s.therapists.FindByID(ctx, therapistID)
	if err != nil {
		return TherapistDetailWithServices{}, err
	}
	if t.TenantID != callerTenantID {
		return TherapistDetailWithServices{}, constants.ErrTherapistNotFound
	}

	mappingRows, err := s.mappings.FindByTherapistID(ctx, therapistID)
	if err != nil {
		return TherapistDetailWithServices{}, fmt.Errorf("load service mappings: %w", err)
	}

	serviceItems, err := s.enrichMappings(ctx, callerTenantID, mappingRows)
	if err != nil {
		return TherapistDetailWithServices{}, err
	}

	return TherapistDetailWithServices{
		TherapistDetail: toTherapistDetail(t),
		Services:        serviceItems,
	}, nil
}

// Update applies partial-field updates to an existing therapist profile.
func (s *TherapistSvc) Update(ctx context.Context, in UpdateTherapistInput) (TherapistDetail, error) {
	t, err := s.therapists.FindByID(ctx, in.TherapistID)
	if err != nil {
		return TherapistDetail{}, err
	}
	if t.TenantID != in.CallerTenantID {
		return TherapistDetail{}, constants.ErrTherapistNotFound
	}

	if !in.IsAdmin {
		if !containsBranch(in.CallerBranches, t.BranchID) {
			return TherapistDetail{}, constants.ErrCrossBranchForbidden
		}
	}

	if in.FullName != nil {
		t.FullName = *in.FullName
	}
	if in.Gender != nil {
		t.Gender = in.Gender
	}
	if in.Phone != nil {
		t.Phone = in.Phone
	}
	if in.Email != nil {
		t.Email = in.Email
	}
	if in.Bio != nil {
		t.Bio = in.Bio
	}
	if in.PhotoURL != nil {
		t.PhotoURL = in.PhotoURL
	}
	if in.UserID != nil {
		t.UserID = in.UserID
	}
	if in.JoinedAt != nil && *in.JoinedAt != "" {
		parsed, err := time.Parse("2006-01-02", *in.JoinedAt)
		if err != nil {
			return TherapistDetail{}, fmt.Errorf("%w: joined_at must be YYYY-MM-DD", constants.ErrInvalidInput)
		}
		t.JoinedAt = &parsed
	}
	t.UpdatedBy = &in.CallerUserID

	if err := s.therapists.Update(ctx, t); err != nil {
		return TherapistDetail{}, fmt.Errorf("update therapist: %w", err)
	}

	bgCtx := context.Background()
	_ = s.audit.Append(bgCtx, AuditEntry{
		TenantID:     &in.CallerTenantID,
		ActorUserID:  &in.CallerUserID,
		Action:       "therapist.updated",
		ResourceType: "therapist",
		ResourceID:   t.ID,
	})

	return toTherapistDetail(t), nil
}

// ChangeStatus activates or deactivates a therapist.
func (s *TherapistSvc) ChangeStatus(ctx context.Context, in ChangeTherapistStatusInput) (TherapistDetail, error) {
	t, err := s.therapists.FindByID(ctx, in.TherapistID)
	if err != nil {
		return TherapistDetail{}, err
	}
	if t.TenantID != in.CallerTenantID {
		return TherapistDetail{}, constants.ErrTherapistNotFound
	}
	if !in.IsAdmin {
		if !containsBranch(in.CallerBranches, t.BranchID) {
			return TherapistDetail{}, constants.ErrCrossBranchForbidden
		}
	}

	if err := s.therapists.UpdateStatus(ctx, in.TherapistID, in.IsActive); err != nil {
		return TherapistDetail{}, fmt.Errorf("change therapist status: %w", err)
	}
	t.IsActive = in.IsActive

	action := "therapist.activated"
	if !in.IsActive {
		action = "therapist.deactivated"
	}

	bgCtx := context.Background()
	_ = s.audit.Append(bgCtx, AuditEntry{
		TenantID:     &in.CallerTenantID,
		ActorUserID:  &in.CallerUserID,
		Action:       action,
		ResourceType: "therapist",
		ResourceID:   t.ID,
		Meta:         map[string]interface{}{"is_active": in.IsActive},
	})

	return toTherapistDetail(t), nil
}

// SoftDelete soft-deletes a therapist and cascades is_active=false to all
// therapist_service mappings per §11.3 flag #7.
func (s *TherapistSvc) SoftDelete(ctx context.Context, callerTenantID, callerUserID string, callerBranches []string, isAdmin bool, therapistID string) error {
	t, err := s.therapists.FindByID(ctx, therapistID)
	if err != nil {
		return err
	}
	if t.TenantID != callerTenantID {
		return constants.ErrTherapistNotFound
	}
	if !isAdmin {
		if !containsBranch(callerBranches, t.BranchID) {
			return constants.ErrCrossBranchForbidden
		}
	}

	// Phase 5 will check for active bookings here; for now, no booking table exists.

	err = s.tx.WithTx(ctx, func(txCtx context.Context) error {
		if err := s.therapists.SoftDelete(txCtx, therapistID); err != nil {
			return err
		}
		return s.mappings.DeactivateAllForTherapist(txCtx, therapistID)
	})
	if err != nil {
		return fmt.Errorf("soft delete therapist: %w", err)
	}

	bgCtx := context.Background()
	_ = s.audit.Append(bgCtx, AuditEntry{
		TenantID:     &callerTenantID,
		ActorUserID:  &callerUserID,
		Action:       "therapist.deleted",
		ResourceType: "therapist",
		ResourceID:   therapistID,
		Meta:         map[string]interface{}{"cascade_mappings_deactivated": true},
	})
	return nil
}

// GetMappings returns the service mappings for a therapist (all, active + inactive).
func (s *TherapistSvc) GetMappings(ctx context.Context, callerTenantID, therapistID string) (TherapistMappingOutput, error) {
	t, err := s.therapists.FindByID(ctx, therapistID)
	if err != nil {
		return TherapistMappingOutput{}, err
	}
	if t.TenantID != callerTenantID {
		return TherapistMappingOutput{}, constants.ErrTherapistNotFound
	}

	mappingRows, err := s.mappings.FindByTherapistID(ctx, therapistID)
	if err != nil {
		return TherapistMappingOutput{}, fmt.Errorf("load service mappings: %w", err)
	}
	items, err := s.enrichMappings(ctx, callerTenantID, mappingRows)
	if err != nil {
		return TherapistMappingOutput{}, err
	}
	return TherapistMappingOutput{TherapistID: therapistID, Services: items}, nil
}

// ---------------------------------------------------------------------------
// Private helpers
// ---------------------------------------------------------------------------

// containsBranch returns true if branchID appears in the branches slice.
func containsBranch(branches []string, branchID string) bool {
	for _, b := range branches {
		if b == branchID {
			return true
		}
	}
	return false
}

// enrichMappings fetches service details for a set of mapping rows and merges
// them into TherapistServiceItem values. tenantID must be the caller's active
// tenant — therapist_service.tenant_id is not a column (derived via therapist
// join), so the caller supplies it.
func (s *TherapistSvc) enrichMappings(ctx context.Context, tenantID string, rows []*model.TherapistService) ([]TherapistServiceItem, error) {
	if len(rows) == 0 {
		return []TherapistServiceItem{}, nil
	}
	ids := make([]string, len(rows))
	for i, r := range rows {
		ids[i] = r.ServiceID
	}

	svcs, err := s.services.FindByIDs(ctx, tenantID, ids)
	if err != nil {
		return nil, fmt.Errorf("enrich service mappings: %w", err)
	}

	svcMap := make(map[string]*model.ServiceCatalog, len(svcs))
	for _, sv := range svcs {
		svcMap[sv.ID] = sv
	}

	items := make([]TherapistServiceItem, 0, len(rows))
	for _, r := range rows {
		sv, ok := svcMap[r.ServiceID]
		if !ok {
			continue // service was soft-deleted — skip
		}
		items = append(items, TherapistServiceItem{
			ServiceID:       sv.ID,
			Name:            sv.Name,
			Category:        sv.Category,
			DurationMinutes: sv.DurationMinutes,
			PriceIDR:        sv.PriceIDR,
			IsActive:        r.IsActive,
			AssignedAt:      r.CreatedAt.UTC().Format(time.RFC3339),
		})
	}
	return items, nil
}

// toTherapistDetail converts a model.Therapist to TherapistDetail.
func toTherapistDetail(t *model.Therapist) TherapistDetail {
	d := TherapistDetail{
		ID:       t.ID,
		TenantID: t.TenantID,
		BranchID: t.BranchID,
		UserID:   t.UserID,
		FullName: t.FullName,
		Gender:   t.Gender,
		Phone:    t.Phone,
		Email:    t.Email,
		Bio:      t.Bio,
		PhotoURL: t.PhotoURL,
		IsActive: t.IsActive,
		CreatedAt: t.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt: t.UpdatedAt.UTC().Format(time.RFC3339),
	}

	// Specialties are stored as a JSONB array string. Empty/invalid parse falls
	// back to an empty slice so the response always matches the API shape.
	if t.Specialties != "" {
		var specs []string
		if err := json.Unmarshal([]byte(t.Specialties), &specs); err == nil {
			d.Specialties = specs
		}
	}
	if d.Specialties == nil {
		d.Specialties = []string{}
	}

	if t.JoinedAt != nil {
		s := t.JoinedAt.UTC().Format(time.RFC3339)
		d.JoinedAt = &s
	}
	return d
}

// hasTenantAdminRole checks if the roles slice contains "tenant_admin".
// Used by controllers to populate the IsAdmin field in input structs.
func HasTenantAdminRole(roles []string) bool {
	for _, r := range roles {
		if strings.EqualFold(r, "tenant_admin") || strings.EqualFold(r, "super_admin") {
			return true
		}
	}
	return false
}
