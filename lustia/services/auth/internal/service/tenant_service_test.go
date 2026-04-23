package service_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/chrisanlung/lustia-auth/internal/constants"
	"github.com/chrisanlung/lustia-auth/internal/model"
	"github.com/chrisanlung/lustia-auth/internal/service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// Fakes for tenant service tests
// ---------------------------------------------------------------------------

type stubTenantRepoForSvc struct {
	tenant       *model.Tenant
	updateStatus func(ctx context.Context, id, newStatus, actor string, reason *string) error
}

func (r *stubTenantRepoForSvc) FindBySlug(_ context.Context, _ string) (*model.Tenant, error) {
	return r.tenant, nil
}
func (r *stubTenantRepoForSvc) FindByID(_ context.Context, _ string) (*model.Tenant, error) {
	if r.tenant == nil {
		return nil, constants.ErrTenantNotFound
	}
	return r.tenant, nil
}
func (r *stubTenantRepoForSvc) Save(_ context.Context, _ *model.Tenant) error { return nil }
func (r *stubTenantRepoForSvc) List(_ context.Context, _ service.TenantFilter) ([]*service.TenantWithCounts, string, error) {
	return nil, "", nil
}
func (r *stubTenantRepoForSvc) UpdateStatus(ctx context.Context, id, newStatus, actor string, reason *string) error {
	if r.updateStatus != nil {
		return r.updateStatus(ctx, id, newStatus, actor, reason)
	}
	// Default: update the in-memory tenant status.
	if r.tenant != nil {
		r.tenant.Status = newStatus
	}
	return nil
}
func (r *stubTenantRepoForSvc) CountActiveBranches(_ context.Context, _ string) (int, error) {
	return 0, nil
}

type stubMembershipRepoForSvc struct {
	suspendCalled bool
	suspended     []string
}

func (r *stubMembershipRepoForSvc) FindByUser(_ context.Context, _ string) ([]*model.Membership, error) {
	return nil, nil
}
func (r *stubMembershipRepoForSvc) FindByUserAndTenant(_ context.Context, _, _ string) (*model.Membership, error) {
	return nil, constants.ErrMembershipNotFound
}
func (r *stubMembershipRepoForSvc) FindByID(_ context.Context, _ string) (*model.Membership, error) {
	return nil, constants.ErrMembershipNotFound
}
func (r *stubMembershipRepoForSvc) Save(_ context.Context, _ *model.Membership) error { return nil }
func (r *stubMembershipRepoForSvc) Update(_ context.Context, _ *model.Membership) error {
	return nil
}
func (r *stubMembershipRepoForSvc) SetStatus(_ context.Context, _ string, _ model.MembershipStatus) error {
	return nil
}
func (r *stubMembershipRepoForSvc) AssignRoles(_ context.Context, _ string, _ []string, _ string) error {
	return nil
}
func (r *stubMembershipRepoForSvc) AssignBranches(_ context.Context, _ string, _ []string, _ string) error {
	return nil
}
func (r *stubMembershipRepoForSvc) SuspendAllForTenant(_ context.Context, _ string) ([]string, error) {
	r.suspendCalled = true
	r.suspended = []string{"m-1", "m-2"}
	return r.suspended, nil
}
func (r *stubMembershipRepoForSvc) FindActiveByTenant(_ context.Context, _ string) ([]*model.Membership, error) {
	return nil, nil
}

type stubTokenRepoForSvc struct {
	revokeCalled bool
}

func (r *stubTokenRepoForSvc) FindByHash(_ context.Context, _ string) (*model.RefreshToken, error) {
	return nil, constants.ErrRefreshTokenNotFound
}
func (r *stubTokenRepoForSvc) Save(_ context.Context, _ *model.RefreshToken) error { return nil }
func (r *stubTokenRepoForSvc) Revoke(_ context.Context, _ string, _ *string) error { return nil }
func (r *stubTokenRepoForSvc) RevokeAllForUser(_ context.Context, _ string) error  { return nil }
func (r *stubTokenRepoForSvc) RevokeAllForTenantUsers(_ context.Context, _ string) error {
	r.revokeCalled = true
	return nil
}
func (r *stubTokenRepoForSvc) DeleteExpiredAndRevoked(_ context.Context) error { return nil }

type stubClockForSvc struct{}

func (c *stubClockForSvc) Now() time.Time { return time.Date(2026, 4, 18, 12, 0, 0, 0, time.UTC) }

func newTestTenantService(tenantRepo *stubTenantRepoForSvc, membershipRepo *stubMembershipRepoForSvc, tokenRepo *stubTokenRepoForSvc) *service.TenantService {
	return service.NewTenantService(
		tenantRepo,
		membershipRepo,
		tokenRepo,
		&fakeAuditRepo{},
		&stubTxManagerForReg{},
		&stubClockForSvc{},
	)
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

func TestTransitionStatus_ActiveToSuspended(t *testing.T) {
	t.Parallel()

	tenant := &model.Tenant{ID: "t1", Status: model.TenantStatusActive}
	tenantRepo := &stubTenantRepoForSvc{tenant: tenant}
	membershipRepo := &stubMembershipRepoForSvc{}
	tokenRepo := &stubTokenRepoForSvc{}

	svc := newTestTenantService(tenantRepo, membershipRepo, tokenRepo)

	out, err := svc.TransitionStatus(context.Background(), service.TransitionTenantStatusInput{
		TenantID:     "t1",
		CallerUserID: "admin-1",
		NewStatus:    model.TenantStatusSuspended,
	})

	require.NoError(t, err)
	assert.Equal(t, model.TenantStatusSuspended, out.Status)
	assert.False(t, membershipRepo.suspendCalled, "memberships should not be suspended for non-deactivation")
	assert.False(t, tokenRepo.revokeCalled)
}

func TestTransitionStatus_InvalidTransition(t *testing.T) {
	t.Parallel()

	// deactivated → active is not allowed.
	tenant := &model.Tenant{ID: "t1", Status: model.TenantStatusDeactivated}
	tenantRepo := &stubTenantRepoForSvc{tenant: tenant}

	svc := newTestTenantService(tenantRepo, &stubMembershipRepoForSvc{}, &stubTokenRepoForSvc{})

	_, err := svc.TransitionStatus(context.Background(), service.TransitionTenantStatusInput{
		TenantID:     "t1",
		CallerUserID: "admin-1",
		NewStatus:    model.TenantStatusActive,
	})

	require.Error(t, err)
	assert.True(t, errors.Is(err, constants.ErrInvalidStatusTransition))
}

func TestTransitionStatus_SuspendedToActive(t *testing.T) {
	t.Parallel()

	tenant := &model.Tenant{ID: "t1", Status: model.TenantStatusSuspended}
	tenantRepo := &stubTenantRepoForSvc{tenant: tenant}

	svc := newTestTenantService(tenantRepo, &stubMembershipRepoForSvc{}, &stubTokenRepoForSvc{})

	out, err := svc.TransitionStatus(context.Background(), service.TransitionTenantStatusInput{
		TenantID:     "t1",
		CallerUserID: "admin-1",
		NewStatus:    model.TenantStatusActive,
	})

	require.NoError(t, err)
	assert.Equal(t, model.TenantStatusActive, out.Status)
}

func TestTransitionStatus_DeactivationCascade(t *testing.T) {
	t.Parallel()

	tenant := &model.Tenant{ID: "t1", Status: model.TenantStatusActive}
	tenantRepo := &stubTenantRepoForSvc{tenant: tenant}
	membershipRepo := &stubMembershipRepoForSvc{}
	tokenRepo := &stubTokenRepoForSvc{}

	svc := newTestTenantService(tenantRepo, membershipRepo, tokenRepo)

	out, err := svc.TransitionStatus(context.Background(), service.TransitionTenantStatusInput{
		TenantID:     "t1",
		CallerUserID: "admin-1",
		NewStatus:    model.TenantStatusDeactivated,
		Reason:       "subscription expired",
	})

	require.NoError(t, err)
	assert.Equal(t, model.TenantStatusDeactivated, out.Status)

	// ADR 0008 §5 answer #4: memberships must be suspended.
	assert.True(t, membershipRepo.suspendCalled, "SuspendAllForTenant must be called on deactivation")

	// Refresh tokens scoped to this tenant must be revoked.
	assert.True(t, tokenRepo.revokeCalled, "RevokeAllForTenantUsers must be called on deactivation")
}

func TestTransitionStatus_PendingToDeactivated(t *testing.T) {
	t.Parallel()

	// pending_approval → deactivated is allowed (admin rejects but keeps row for audit).
	tenant := &model.Tenant{ID: "t1", Status: model.TenantStatusPendingApproval}
	tenantRepo := &stubTenantRepoForSvc{tenant: tenant}

	svc := newTestTenantService(tenantRepo, &stubMembershipRepoForSvc{}, &stubTokenRepoForSvc{})

	out, err := svc.TransitionStatus(context.Background(), service.TransitionTenantStatusInput{
		TenantID:     "t1",
		CallerUserID: "admin-1",
		NewStatus:    model.TenantStatusDeactivated,
	})

	require.NoError(t, err)
	assert.Equal(t, model.TenantStatusDeactivated, out.Status)
}
