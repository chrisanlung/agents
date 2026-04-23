package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/chrisanlung/lustia-auth/internal/constants"
	"github.com/chrisanlung/lustia-auth/internal/model"
	"github.com/google/uuid"
)

// serviceCode derives the NOT NULL `code` column from the service name. It is
// an internal identifier (not a user-editable slug); the trailing short UUID
// fragment guarantees uniqueness even when two services share a name. Length
// capped at 64 to leave headroom under the column's practical limit. Phase 4
// BUG-P4-B: DB column has no default, caller must supply a value.
func serviceCode(name, id string) string {
	s := strings.ToLower(strings.TrimSpace(name))
	var b strings.Builder
	b.Grow(len(s))
	prevDash := true
	for _, r := range s {
		switch {
		case (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9'):
			b.WriteRune(r)
			prevDash = false
		default:
			if !prevDash {
				b.WriteByte('-')
				prevDash = true
			}
		}
	}
	base := strings.Trim(b.String(), "-")
	if len(base) > 48 {
		base = strings.TrimRight(base[:48], "-")
	}
	if base == "" {
		base = "svc"
	}
	suffix := strings.ReplaceAll(id, "-", "")
	if len(suffix) > 8 {
		suffix = suffix[:8]
	}
	return base + "-" + suffix
}

// CatalogService handles service-catalog CRUD (tenant-scoped).
// Named CatalogService to avoid the "service_service" stutter mentioned in ADR 0009.
type CatalogService struct {
	services  ServiceCatalogRepository
	mappings  TherapistServiceRepository
	therapists TherapistRepository
	branches   BranchRepository
	audit     AuditRepository
	clock     Clock
}

// NewCatalogService constructs a CatalogService.
func NewCatalogService(
	services ServiceCatalogRepository,
	mappings TherapistServiceRepository,
	therapists TherapistRepository,
	branches BranchRepository,
	audit AuditRepository,
	clock Clock,
) *CatalogService {
	return &CatalogService{
		services:   services,
		mappings:   mappings,
		therapists: therapists,
		branches:   branches,
		audit:      audit,
		clock:      clock,
	}
}

// Create creates a new service in the caller's tenant.
func (s *CatalogService) Create(ctx context.Context, in CreateServiceInput) (ServiceDetail, error) {
	id := uuid.New().String()
	sv := &model.ServiceCatalog{
		ID:              id,
		TenantID:        in.CallerTenantID,
		Name:            in.Name,
		Code:            serviceCode(in.Name, id),
		Description:     in.Description,
		Category:        in.Category,
		DurationMinutes: in.DurationMinutes,
		PriceIDR:        in.PriceIDR,
		Currency:        "IDR",
		IsActive:        true,
		CreatedBy:       &in.CallerUserID,
		UpdatedBy:       &in.CallerUserID,
	}

	if err := s.services.Save(ctx, sv); err != nil {
		return ServiceDetail{}, fmt.Errorf("create service: %w", err)
	}

	bgCtx := context.Background()
	_ = s.audit.Append(bgCtx, AuditEntry{
		TenantID:     &in.CallerTenantID,
		ActorUserID:  &in.CallerUserID,
		Action:       "service.created",
		ResourceType: "service",
		ResourceID:   sv.ID,
		// SECURITY.md Phase 4 L-1: log service_id only, not free-form name.
		Meta: map[string]interface{}{"service_id": sv.ID},
	})

	return toServiceDetail(sv), nil
}

// List returns a paginated list of services for the caller's tenant.
func (s *CatalogService) List(ctx context.Context, in ListServicesInput) (ListServicesOutput, error) {
	rows, cursor, err := s.services.FindByTenant(ctx, in.CallerTenantID, ServiceFilter{
		IsActive: in.IsActive,
		Category: in.Category,
		Cursor:   in.Cursor,
		Limit:    in.Limit,
	})
	if err != nil {
		return ListServicesOutput{}, fmt.Errorf("list services: %w", err)
	}

	details := make([]ServiceDetail, len(rows))
	for i, sv := range rows {
		details[i] = toServiceDetail(sv)
	}
	return ListServicesOutput{Services: details, NextCursor: cursor}, nil
}

// Get returns a single service with active therapist mappings per §11.3 flag #6.
func (s *CatalogService) Get(ctx context.Context, callerTenantID, serviceID string) (ServiceDetailWithTherapists, error) {
	sv, err := s.services.FindByID(ctx, serviceID)
	if err != nil {
		return ServiceDetailWithTherapists{}, err
	}
	if sv.TenantID != callerTenantID {
		return ServiceDetailWithTherapists{}, constants.ErrServiceNotFound
	}

	// Active mappings only — per §11.3 flag #6.
	mappingRows, err := s.mappings.FindByServiceID(ctx, serviceID)
	if err != nil {
		return ServiceDetailWithTherapists{}, fmt.Errorf("load therapist mappings: %w", err)
	}

	items, err := s.enrichTherapistMappings(ctx, mappingRows)
	if err != nil {
		return ServiceDetailWithTherapists{}, err
	}

	return ServiceDetailWithTherapists{
		ServiceDetail: toServiceDetail(sv),
		Therapists:    items,
	}, nil
}

// Update applies partial-field updates to an existing service.
func (s *CatalogService) Update(ctx context.Context, in UpdateServiceInput) (ServiceDetail, error) {
	sv, err := s.services.FindByID(ctx, in.ServiceID)
	if err != nil {
		return ServiceDetail{}, err
	}
	if sv.TenantID != in.CallerTenantID {
		return ServiceDetail{}, constants.ErrServiceNotFound
	}

	if in.Name != nil {
		sv.Name = *in.Name
	}
	if in.Description != nil {
		sv.Description = in.Description
	}
	if in.Category != nil {
		sv.Category = in.Category
	}
	if in.DurationMinutes != nil {
		sv.DurationMinutes = *in.DurationMinutes
	}
	if in.PriceIDR != nil {
		sv.PriceIDR = *in.PriceIDR
	}
	sv.UpdatedBy = &in.CallerUserID

	if err := s.services.Update(ctx, sv); err != nil {
		return ServiceDetail{}, fmt.Errorf("update service: %w", err)
	}

	bgCtx := context.Background()
	_ = s.audit.Append(bgCtx, AuditEntry{
		TenantID:     &in.CallerTenantID,
		ActorUserID:  &in.CallerUserID,
		Action:       "service.updated",
		ResourceType: "service",
		ResourceID:   sv.ID,
	})

	return toServiceDetail(sv), nil
}

// ChangeStatus activates or deactivates a service.
func (s *CatalogService) ChangeStatus(ctx context.Context, in ChangeServiceStatusInput) (ServiceDetail, error) {
	sv, err := s.services.FindByID(ctx, in.ServiceID)
	if err != nil {
		return ServiceDetail{}, err
	}
	if sv.TenantID != in.CallerTenantID {
		return ServiceDetail{}, constants.ErrServiceNotFound
	}

	if err := s.services.UpdateStatus(ctx, in.ServiceID, in.IsActive); err != nil {
		return ServiceDetail{}, fmt.Errorf("change service status: %w", err)
	}
	sv.IsActive = in.IsActive

	action := "service.activated"
	if !in.IsActive {
		action = "service.deactivated"
	}

	bgCtx := context.Background()
	_ = s.audit.Append(bgCtx, AuditEntry{
		TenantID:     &in.CallerTenantID,
		ActorUserID:  &in.CallerUserID,
		Action:       action,
		ResourceType: "service",
		ResourceID:   sv.ID,
		Meta:         map[string]interface{}{"is_active": in.IsActive},
	})

	return toServiceDetail(sv), nil
}

// SoftDelete soft-deletes a service per §11.5.6.
func (s *CatalogService) SoftDelete(ctx context.Context, callerTenantID, callerUserID, serviceID string) error {
	sv, err := s.services.FindByID(ctx, serviceID)
	if err != nil {
		return err
	}
	if sv.TenantID != callerTenantID {
		return constants.ErrServiceNotFound
	}

	if err := s.services.SoftDelete(ctx, serviceID); err != nil {
		return fmt.Errorf("soft delete service: %w", err)
	}

	bgCtx := context.Background()
	_ = s.audit.Append(bgCtx, AuditEntry{
		TenantID:     &callerTenantID,
		ActorUserID:  &callerUserID,
		Action:       "service.deleted",
		ResourceType: "service",
		ResourceID:   serviceID,
	})
	return nil
}

// ---------------------------------------------------------------------------
// Private helpers
// ---------------------------------------------------------------------------

// enrichTherapistMappings fetches therapist + branch details for active mapping rows.
func (s *CatalogService) enrichTherapistMappings(ctx context.Context, rows []*model.TherapistService) ([]ServiceTherapistItem, error) {
	if len(rows) == 0 {
		return []ServiceTherapistItem{}, nil
	}

	items := make([]ServiceTherapistItem, 0, len(rows))
	for _, r := range rows {
		t, err := s.therapists.FindByID(ctx, r.TherapistID)
		if err != nil {
			continue // therapist was soft-deleted since mapping was written
		}

		b, err := s.branches.FindByID(ctx, t.BranchID)
		branchName := ""
		if err == nil {
			branchName = b.Name
		}

		items = append(items, ServiceTherapistItem{
			TherapistID: t.ID,
			FullName:    t.FullName,
			BranchID:    t.BranchID,
			BranchName:  branchName,
			IsActive:    t.IsActive,
		})
	}
	return items, nil
}

// toServiceDetail converts a model.ServiceCatalog to ServiceDetail.
func toServiceDetail(sv *model.ServiceCatalog) ServiceDetail {
	currency := sv.Currency
	if currency == "" {
		currency = "IDR"
	}
	return ServiceDetail{
		ID:              sv.ID,
		TenantID:        sv.TenantID,
		Name:            sv.Name,
		Description:     sv.Description,
		Category:        sv.Category,
		DurationMinutes: sv.DurationMinutes,
		PriceIDR:        sv.PriceIDR,
		Currency:        currency,
		IsActive:        sv.IsActive,
		CreatedAt:       sv.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:       sv.UpdatedAt.UTC().Format(time.RFC3339),
	}
}
