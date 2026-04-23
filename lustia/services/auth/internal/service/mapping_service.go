package service

import (
	"context"
	"fmt"

	"github.com/chrisanlung/lustia-auth/internal/constants"
)

// MappingService handles the PUT /therapists/:id/services full-replace endpoint.
// Kept separate from TherapistSvc per the SRP guidance in the task description.
type MappingService struct {
	therapists TherapistRepository
	services   ServiceCatalogRepository
	mappings   TherapistServiceRepository
	audit      AuditRepository
	tx         TxManager
}

// NewMappingService constructs a MappingService.
func NewMappingService(
	therapists TherapistRepository,
	services ServiceCatalogRepository,
	mappings TherapistServiceRepository,
	audit AuditRepository,
	tx TxManager,
) *MappingService {
	return &MappingService{
		therapists: therapists,
		services:   services,
		mappings:   mappings,
		audit:      audit,
		tx:         tx,
	}
}

// Reconcile performs the full-replace of a therapist's service assignments per
// ADR 0009 §11.6.2 and §11.3 flag #2.
func (s *MappingService) Reconcile(ctx context.Context, in ReconcileMappingInput) (TherapistMappingOutput, error) {
	// Resolve therapist and enforce cross-branch rule.
	t, err := s.therapists.FindByID(ctx, in.TherapistID)
	if err != nil {
		return TherapistMappingOutput{}, err
	}
	if t.TenantID != in.CallerTenantID {
		return TherapistMappingOutput{}, constants.ErrTherapistNotFound
	}
	if !in.IsAdmin {
		if !containsBranch(in.CallerBranches, t.BranchID) {
			return TherapistMappingOutput{}, constants.ErrCrossBranchForbidden
		}
	}

	// Validate that all requested service IDs exist in this tenant.
	if len(in.ServiceIDs) > 0 {
		found, err := s.services.FindByIDs(ctx, in.CallerTenantID, in.ServiceIDs)
		if err != nil {
			return TherapistMappingOutput{}, fmt.Errorf("validate service ids: %w", err)
		}
		foundSet := make(map[string]struct{}, len(found))
		for _, sv := range found {
			foundSet[sv.ID] = struct{}{}
		}
		for _, id := range in.ServiceIDs {
			if _, ok := foundSet[id]; !ok {
				return TherapistMappingOutput{}, fmt.Errorf("%w: service_id %s not found or soft-deleted in this tenant", constants.ErrServiceNotFound, id)
			}
		}
	}

	err = s.tx.WithTx(ctx, func(txCtx context.Context) error {
		return s.mappings.ReconcileForTherapist(txCtx, in.CallerTenantID, in.TherapistID, in.CallerUserID, in.ServiceIDs)
	})
	if err != nil {
		return TherapistMappingOutput{}, fmt.Errorf("reconcile therapist services: %w", err)
	}

	bgCtx := context.Background()
	_ = s.audit.Append(bgCtx, AuditEntry{
		TenantID:     &in.CallerTenantID,
		ActorUserID:  &in.CallerUserID,
		Action:       "therapist_service.updated",
		ResourceType: "therapist",
		ResourceID:   in.TherapistID,
		Meta:         map[string]interface{}{"service_ids": in.ServiceIDs},
	})

	// Return full mapping state after reconciliation.
	rows, err := s.mappings.FindByTherapistID(ctx, in.TherapistID)
	if err != nil {
		return TherapistMappingOutput{}, fmt.Errorf("reload mappings after reconcile: %w", err)
	}

	// Enrich with service details.
	svIDs := make([]string, 0, len(rows))
	for _, r := range rows {
		svIDs = append(svIDs, r.ServiceID)
	}
	svcs, err := s.services.FindByIDs(ctx, in.CallerTenantID, svIDs)
	if err != nil {
		return TherapistMappingOutput{}, fmt.Errorf("enrich mapping response: %w", err)
	}
	svcMap := make(map[string]*ServiceDetail, len(svcs))
	for _, sv := range svcs {
		d := toServiceDetail(sv)
		svcMap[sv.ID] = &d
	}

	items := make([]TherapistServiceItem, 0, len(rows))
	for _, r := range rows {
		sv, ok := svcMap[r.ServiceID]
		if !ok {
			continue
		}
		items = append(items, TherapistServiceItem{
			ServiceID:       sv.ID,
			Name:            sv.Name,
			Category:        sv.Category,
			DurationMinutes: sv.DurationMinutes,
			PriceIDR:        sv.PriceIDR,
			IsActive:        r.IsActive,
			AssignedAt:      r.CreatedAt.UTC().Format("2006-01-02T15:04:05Z"),
		})
	}

	return TherapistMappingOutput{TherapistID: in.TherapistID, Services: items}, nil
}
