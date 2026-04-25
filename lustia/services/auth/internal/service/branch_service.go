package service

import (
	"context"
	"fmt"
	"time"

	"github.com/chrisanlung/lustia-auth/internal/constants"
	"github.com/chrisanlung/lustia-auth/internal/model"
	"github.com/google/uuid"
)

// BranchService handles tenant-scoped branch CRUD and the branch-limit check
// defined in ADR 0008 §2.2.7.
type BranchService struct {
	branches BranchRepository
	tenants  TenantRepository
	users    UserRepository
	audit    AuditRepository
	clock    Clock
}

// NewBranchService constructs a BranchService.
func NewBranchService(
	branches BranchRepository,
	tenants TenantRepository,
	users UserRepository,
	audit AuditRepository,
	clock Clock,
) *BranchService {
	return &BranchService{
		branches: branches,
		tenants:  tenants,
		users:    users,
		audit:    audit,
		clock:    clock,
	}
}

// Create creates a new branch for the caller's tenant. The branch count check
// uses the tenant's max_branches; 999 is the unlimited sentinel.
func (s *BranchService) Create(ctx context.Context, in CreateBranchInput) (BranchDetail, error) {
	tenant, err := s.tenants.FindByID(ctx, in.CallerTenantID)
	if err != nil {
		return BranchDetail{}, err
	}

	// Enforce branch limit (sentinel 999 = unlimited per ADR 0008 §2.2.7).
	if tenant.MaxBranches < model.MaxBranchesUnlimited {
		count, err := s.tenants.CountActiveBranches(ctx, in.CallerTenantID)
		if err != nil {
			return BranchDetail{}, fmt.Errorf("count branches: %w", err)
		}
		if count >= tenant.MaxBranches {
			return BranchDetail{}, constants.ErrBranchLimitReached
		}
	}

	country := in.Country
	if country == "" {
		country = "ID"
	}
	tz := in.Timezone
	if tz == "" {
		tz = "Asia/Jakarta"
	}

	b := &model.Branch{
		ID:               uuid.New().String(),
		TenantID:         in.CallerTenantID,
		Name:             in.Name,
		Code:             in.Code,
		Status:           model.BranchStatusInactive,
		OperationalHours: []byte("{}"), // column is NOT NULL jsonb — default via migration fires on omitted columns, not on zero-value bytes
		AddressLine1:     in.AddressLine1,
		AddressLine2:     in.AddressLine2,
		City:             in.City,
		Province:         in.Province,
		PostalCode:       in.PostalCode,
		Country:          country,
		Timezone:         tz,
		ContactPhone:     in.ContactPhone,
		ContactEmail:     in.ContactEmail,
		CreatedBy:        &in.CallerUserID,
		UpdatedBy:        &in.CallerUserID,
	}

	if err := s.branches.Save(ctx, b); err != nil {
		return BranchDetail{}, fmt.Errorf("save branch: %w", err)
	}

	_ = s.audit.Append(ctx, AuditEntry{
		TenantID:     &in.CallerTenantID,
		ActorUserID:  &in.CallerUserID,
		Action:       "branch.created",
		ResourceType: "branch",
		ResourceID:   b.ID,
		Meta:         map[string]interface{}{"name": b.Name, "code": b.Code},
	})

	return toBranchDetail(b), nil
}

// ListByTenant returns a paginated list of branches for the caller's tenant.
func (s *BranchService) ListByTenant(ctx context.Context, in ListBranchesInput) (ListBranchesOutput, error) {
	limit := in.Limit
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	page := in.Page
	if page < 1 {
		page = 1
	}

	rows, total, err := s.branches.FindByTenant(ctx, in.CallerTenantID, BranchFilter{
		Status: in.Status,
		Page:   page,
		Limit:  limit,
	})
	if err != nil {
		return ListBranchesOutput{}, fmt.Errorf("list branches: %w", err)
	}
	details := make([]BranchDetail, len(rows))
	for i, b := range rows {
		details[i] = toBranchDetail(b)
	}

	totalPages := 0
	if total > 0 {
		totalPages = int((total + int64(limit) - 1) / int64(limit))
	}
	return ListBranchesOutput{Branches: details, Page: page, TotalCount: total, TotalPages: totalPages}, nil
}

// GetBranch returns a single branch by ID, scoped to the caller's tenant.
func (s *BranchService) GetBranch(ctx context.Context, callerTenantID, branchID string) (BranchDetail, error) {
	b, err := s.branches.FindByID(ctx, branchID)
	if err != nil {
		return BranchDetail{}, err
	}
	// RLS already enforces tenant scoping at the DB level; the check below is
	// a defence-in-depth guard for cases where RLS is bypassed in tests.
	if b.TenantID != callerTenantID {
		return BranchDetail{}, constants.ErrBranchNotFound
	}
	return toBranchDetail(b), nil
}

// UpdateBranch modifies the non-status fields of an existing branch.
func (s *BranchService) UpdateBranch(ctx context.Context, in UpdateBranchInput) (BranchDetail, error) {
	b, err := s.branches.FindByID(ctx, in.BranchID)
	if err != nil {
		return BranchDetail{}, err
	}
	if b.TenantID != in.CallerTenantID {
		return BranchDetail{}, constants.ErrBranchNotFound
	}

	// Apply partial updates (only provided fields).
	if in.Name != nil {
		b.Name = *in.Name
	}
	if in.AddressLine1 != nil {
		b.AddressLine1 = in.AddressLine1
	}
	if in.AddressLine2 != nil {
		b.AddressLine2 = in.AddressLine2
	}
	if in.City != nil {
		b.City = in.City
	}
	if in.Province != nil {
		b.Province = in.Province
	}
	if in.PostalCode != nil {
		b.PostalCode = in.PostalCode
	}
	if in.Country != nil {
		b.Country = *in.Country
	}
	if in.Timezone != nil {
		b.Timezone = *in.Timezone
	}
	if in.ContactPhone != nil {
		b.ContactPhone = in.ContactPhone
	}
	if in.ContactEmail != nil {
		b.ContactEmail = in.ContactEmail
	}
	b.UpdatedBy = &in.CallerUserID

	if err := s.branches.Update(ctx, b); err != nil {
		return BranchDetail{}, fmt.Errorf("update branch: %w", err)
	}

	_ = s.audit.Append(ctx, AuditEntry{
		TenantID:     &in.CallerTenantID,
		ActorUserID:  &in.CallerUserID,
		Action:       "branch.updated",
		ResourceType: "branch",
		ResourceID:   b.ID,
	})

	return toBranchDetail(b), nil
}

// ChangeStatus transitions a branch to a new status, enforcing the state
// machine defined in model.Branch.CanTransitionTo.
func (s *BranchService) ChangeStatus(ctx context.Context, in ChangeBranchStatusInput) (BranchDetail, error) {
	b, err := s.branches.FindByID(ctx, in.BranchID)
	if err != nil {
		return BranchDetail{}, err
	}
	if b.TenantID != in.CallerTenantID {
		return BranchDetail{}, constants.ErrBranchNotFound
	}
	if !b.CanTransitionTo(in.NewStatus) {
		return BranchDetail{}, constants.ErrInvalidStatusTransition
	}

	var activatedAt *time.Time
	if in.NewStatus == model.BranchStatusActive && b.ActivatedAt == nil {
		now := s.clock.Now()
		activatedAt = &now
	}

	if err := s.branches.UpdateStatus(ctx, in.BranchID, in.NewStatus, activatedAt); err != nil {
		return BranchDetail{}, fmt.Errorf("update branch status: %w", err)
	}

	b.Status = in.NewStatus
	if activatedAt != nil {
		b.ActivatedAt = activatedAt
	}

	_ = s.audit.Append(ctx, AuditEntry{
		TenantID:     &in.CallerTenantID,
		ActorUserID:  &in.CallerUserID,
		Action:       "branch.status_changed",
		ResourceType: "branch",
		ResourceID:   b.ID,
		Meta:         map[string]interface{}{"new_status": in.NewStatus},
	})

	return toBranchDetail(b), nil
}

// DeleteBranch soft-deletes a branch. Only inactive branches may be deleted.
func (s *BranchService) DeleteBranch(ctx context.Context, callerTenantID, callerUserID, branchID string) error {
	b, err := s.branches.FindByID(ctx, branchID)
	if err != nil {
		return err
	}
	if b.TenantID != callerTenantID {
		return constants.ErrBranchNotFound
	}
	// Per ADR 0008 §2.1.2: only inactive branches may be deleted.
	if b.Status != model.BranchStatusInactive {
		return constants.ErrInvalidStatusTransition
	}

	if err := s.branches.SoftDelete(ctx, branchID); err != nil {
		return fmt.Errorf("soft delete branch: %w", err)
	}

	_ = s.audit.Append(ctx, AuditEntry{
		TenantID:     &callerTenantID,
		ActorUserID:  &callerUserID,
		Action:       "branch.deleted",
		ResourceType: "branch",
		ResourceID:   branchID,
	})
	return nil
}

// GetOnboardingState returns the data required by the tenant-admin onboarding
// redirect logic described in ADR 0008 §2.3.3.
func (s *BranchService) GetOnboardingState(ctx context.Context, callerTenantID, callerUserID string) (OnboardingStateOutput, error) {
	tenant, err := s.tenants.FindByID(ctx, callerTenantID)
	if err != nil {
		return OnboardingStateOutput{}, fmt.Errorf("find tenant for onboarding: %w", err)
	}

	count, err := s.tenants.CountActiveBranches(ctx, callerTenantID)
	if err != nil {
		return OnboardingStateOutput{}, fmt.Errorf("count branches for onboarding: %w", err)
	}

	user, err := s.users.FindByID(ctx, callerUserID)
	if err != nil {
		return OnboardingStateOutput{}, fmt.Errorf("find user for onboarding: %w", err)
	}

	return OnboardingStateOutput{
		HasBranches:        count > 0,
		ActiveBranchCount:  count,
		MaxBranches:        tenant.MaxBranches,
		MustChangePassword: user.MustChangePassword,
	}, nil
}

// ---------------------------------------------------------------------------
// Mapping helpers (branch → DTO)
// ---------------------------------------------------------------------------

func toBranchDetail(b *model.Branch) BranchDetail {
	d := BranchDetail{
		ID:           b.ID,
		TenantID:     b.TenantID,
		Name:         b.Name,
		Code:         b.Code,
		Status:       b.Status,
		AddressLine1: b.AddressLine1,
		AddressLine2: b.AddressLine2,
		City:         b.City,
		Province:     b.Province,
		PostalCode:   b.PostalCode,
		Country:      b.Country,
		Timezone:     b.Timezone,
		ContactPhone: b.ContactPhone,
		ContactEmail: b.ContactEmail,
		CreatedAt:    b.CreatedAt.Format(time.RFC3339),
		UpdatedAt:    b.UpdatedAt.Format(time.RFC3339),
	}
	if b.ActivatedAt != nil {
		s := b.ActivatedAt.Format(time.RFC3339)
		d.ActivatedAt = &s
	}
	return d
}
