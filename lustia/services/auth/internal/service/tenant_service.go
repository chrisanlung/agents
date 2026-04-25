package service

import (
	"context"
	"fmt"
	"time"

	"github.com/chrisanlung/lustia-auth/internal/constants"
	"github.com/chrisanlung/lustia-auth/internal/model"
)

// TenantService handles platform-admin tenant management: listing, status
// transitions, and the deactivation cascade required by ADR 0008 §5 answer #4.
type TenantService struct {
	tenants     TenantRepository
	memberships MembershipRepository
	tokens      RefreshTokenRepository
	audit       AuditRepository
	tx          TxManager
	clock       Clock
}

// NewTenantService constructs a TenantService.
func NewTenantService(
	tenants TenantRepository,
	memberships MembershipRepository,
	tokens RefreshTokenRepository,
	audit AuditRepository,
	tx TxManager,
	clock Clock,
) *TenantService {
	return &TenantService{
		tenants:     tenants,
		memberships: memberships,
		tokens:      tokens,
		audit:       audit,
		tx:          tx,
		clock:       clock,
	}
}

// List returns a paginated list of tenants visible to a platform admin.
func (s *TenantService) List(ctx context.Context, in ListTenantsInput) (ListTenantsOutput, error) {
	limit := in.Limit
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	page := in.Page
	if page < 1 {
		page = 1
	}

	rows, total, err := s.tenants.List(ctx, TenantFilter{
		Status:  in.Status,
		Q:       in.Q,
		Package: in.Package,
		Page:    page,
		Limit:   limit,
	})
	if err != nil {
		return ListTenantsOutput{}, fmt.Errorf("list tenants: %w", err)
	}

	summaries := make([]TenantSummary, len(rows))
	for i, r := range rows {
		summaries[i] = toTenantSummary(r)
	}

	totalPages := 0
	if total > 0 {
		totalPages = int((total + int64(limit) - 1) / int64(limit))
	}
	return ListTenantsOutput{
		Tenants:    summaries,
		Page:       page,
		TotalCount: total,
		TotalPages: totalPages,
	}, nil
}

// GetTenant returns a single tenant by ID.
func (s *TenantService) GetTenant(ctx context.Context, tenantID string) (TenantSummary, error) {
	t, err := s.tenants.FindByID(ctx, tenantID)
	if err != nil {
		return TenantSummary{}, err
	}
	// Counts are set to zero for the single-item fetch path to avoid a
	// subquery — the admin list view carries the counts, the detail view
	// focuses on the full tenant object.
	return toTenantSummary(&TenantWithCounts{Tenant: *t}), nil
}

// TransitionStatus enforces the tenant state machine from ADR 0008 §2.1.1.
//
// On transition to deactivated, all active memberships for the tenant are
// suspended and all scoped refresh tokens are revoked, within a single
// transaction. Each cascaded membership suspension is audit-logged individually.
func (s *TenantService) TransitionStatus(ctx context.Context, in TransitionTenantStatusInput) (TenantSummary, error) {
	var out TenantSummary

	err := s.tx.WithTx(ctx, func(ctx context.Context) error {
		tenant, err := s.tenants.FindByID(ctx, in.TenantID)
		if err != nil {
			return err
		}

		if !tenant.CanTransitionTo(in.NewStatus) {
			return constants.ErrInvalidStatusTransition
		}

		var reason *string
		if in.Reason != "" {
			reason = &in.Reason
		}

		if err := s.tenants.UpdateStatus(ctx, in.TenantID, in.NewStatus, in.CallerUserID, reason); err != nil {
			return fmt.Errorf("update tenant status: %w", err)
		}

		// Deactivation cascade — ADR 0008 §5 answer #4.
		if in.NewStatus == model.TenantStatusDeactivated {
			affectedIDs, err := s.memberships.SuspendAllForTenant(ctx, in.TenantID)
			if err != nil {
				return fmt.Errorf("suspend memberships on deactivation: %w", err)
			}

			// Audit each suspended membership individually.
			for _, mid := range affectedIDs {
				_ = s.audit.Append(ctx, AuditEntry{
					TenantID:     &in.TenantID,
					ActorUserID:  &in.CallerUserID,
					Action:       "membership.suspended_by_tenant_deactivation",
					ResourceType: "membership",
					ResourceID:   mid,
					Meta:         map[string]interface{}{"tenant_id": in.TenantID},
				})
			}

			// Revoke all refresh tokens scoped to this tenant so in-flight
			// sessions cannot continue. The caller's RLS context is __platform__
			// (super admin), but refresh_token's UPDATE policy requires
			// app.current_tenant = target tenant OR NULL+platform (ADR 0005 does
			// not grant a cross-tenant bypass on this table). Temporarily switch
			// context to the target tenant for the revoke, then restore platform
			// so the surrounding transaction finishes under super-admin scope.
			// Covers SECURITY.md Phase 3 bug BUG-T7-A.
			if err := s.tx.SetTenantContext(ctx, in.TenantID, in.CallerUserID); err != nil {
				return fmt.Errorf("switch RLS context for tenant deactivation: %w", err)
			}
			if err := s.tokens.RevokeAllForTenantUsers(ctx, in.TenantID); err != nil {
				return fmt.Errorf("revoke tenant tokens on deactivation: %w", err)
			}
			if err := s.tx.SetTenantContext(ctx, constants.PlatformTenantSentinel, in.CallerUserID); err != nil {
				return fmt.Errorf("restore platform RLS context after revoke: %w", err)
			}
		}

		// Re-fetch the updated tenant for the response.
		updated, err := s.tenants.FindByID(ctx, in.TenantID)
		if err != nil {
			return fmt.Errorf("re-fetch tenant after status update: %w", err)
		}

		_ = s.audit.Append(ctx, AuditEntry{
			TenantID:     &in.TenantID,
			ActorUserID:  &in.CallerUserID,
			Action:       "tenant.status_changed",
			ResourceType: "tenant",
			ResourceID:   in.TenantID,
			Meta: map[string]interface{}{
				"new_status": in.NewStatus,
				"reason":     in.Reason,
			},
		})

		out = toTenantSummary(&TenantWithCounts{Tenant: *updated})
		return nil
	})
	if err != nil {
		return TenantSummary{}, err
	}
	return out, nil
}

// ---------------------------------------------------------------------------
// Mapping helpers (tenant → DTO)
// ---------------------------------------------------------------------------

func toTenantSummary(t *TenantWithCounts) TenantSummary {
	s := TenantSummary{
		ID:              t.ID,
		Name:            t.Name,
		Slug:            t.Slug,
		Status:          t.Status,
		Package:         t.Package,
		MaxBranches:     t.MaxBranches,
		CreatedAt:       t.CreatedAt.Format(time.RFC3339),
		MembershipCount: t.MembershipCount,
		BranchCount:     t.BranchCount,
	}
	if t.ContactEmail != nil {
		s.ContactEmail = *t.ContactEmail
	}
	if t.ContactName != nil {
		s.ContactName = *t.ContactName
	}
	if t.ApprovedAt != nil {
		v := t.ApprovedAt.Format(time.RFC3339)
		s.ApprovedAt = &v
	}
	s.ApprovedBy = t.ApprovedBy
	if t.RejectedAt != nil {
		v := t.RejectedAt.Format(time.RFC3339)
		s.RejectedAt = &v
	}
	s.RejectionReason = t.RejectionReason
	return s
}
