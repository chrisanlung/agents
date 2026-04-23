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
// Fakes for branch service tests
// ---------------------------------------------------------------------------

type stubBranchRepo struct {
	branches     map[string]*model.Branch
	saved        []*model.Branch
	statusUpdate map[string]string
}

func newStubBranchRepo() *stubBranchRepo {
	return &stubBranchRepo{
		branches:     make(map[string]*model.Branch),
		statusUpdate: make(map[string]string),
	}
}

func (r *stubBranchRepo) FindByID(_ context.Context, id string) (*model.Branch, error) {
	b, ok := r.branches[id]
	if !ok {
		return nil, constants.ErrBranchNotFound
	}
	return b, nil
}
func (r *stubBranchRepo) FindByTenant(_ context.Context, _ string, _ service.BranchFilter) ([]*model.Branch, string, error) {
	return nil, "", nil
}
func (r *stubBranchRepo) Save(_ context.Context, b *model.Branch) error {
	r.saved = append(r.saved, b)
	r.branches[b.ID] = b
	return nil
}
func (r *stubBranchRepo) Update(_ context.Context, b *model.Branch) error {
	r.branches[b.ID] = b
	return nil
}
func (r *stubBranchRepo) UpdateStatus(_ context.Context, id, newStatus string, _ *time.Time) error {
	b, ok := r.branches[id]
	if !ok {
		return constants.ErrBranchNotFound
	}
	b.Status = newStatus
	r.statusUpdate[id] = newStatus
	return nil
}
func (r *stubBranchRepo) SoftDelete(_ context.Context, id string) error {
	delete(r.branches, id)
	return nil
}

type stubTenantRepoForBranch struct {
	tenant *model.Tenant
	count  int
}

func (r *stubTenantRepoForBranch) FindBySlug(_ context.Context, _ string) (*model.Tenant, error) {
	return r.tenant, nil
}
func (r *stubTenantRepoForBranch) FindByID(_ context.Context, _ string) (*model.Tenant, error) {
	if r.tenant == nil {
		return nil, constants.ErrTenantNotFound
	}
	return r.tenant, nil
}
func (r *stubTenantRepoForBranch) Save(_ context.Context, _ *model.Tenant) error { return nil }
func (r *stubTenantRepoForBranch) List(_ context.Context, _ service.TenantFilter) ([]*service.TenantWithCounts, string, error) {
	return nil, "", nil
}
func (r *stubTenantRepoForBranch) UpdateStatus(_ context.Context, _, _, _ string, _ *string) error {
	return nil
}
func (r *stubTenantRepoForBranch) CountActiveBranches(_ context.Context, _ string) (int, error) {
	return r.count, nil
}

type stubUserRepoForBranch struct{}

func (r *stubUserRepoForBranch) FindByEmail(_ context.Context, _ string) (*model.User, error) {
	return nil, constants.ErrUserNotFound
}
func (r *stubUserRepoForBranch) FindByID(_ context.Context, _ string) (*model.User, error) {
	return &model.User{MustChangePassword: false}, nil
}
func (r *stubUserRepoForBranch) FindByTenant(_ context.Context, _ string, _ service.UserFilter) ([]*model.User, string, error) {
	return nil, "", nil
}
func (r *stubUserRepoForBranch) Save(_ context.Context, _ *model.User) error         { return nil }
func (r *stubUserRepoForBranch) Update(_ context.Context, _ *model.User) error       { return nil }
func (r *stubUserRepoForBranch) UpdatePassword(_ context.Context, _ string, _ string) error {
	return nil
}
func (r *stubUserRepoForBranch) IncrementFailedLogin(_ context.Context, _ string, _ *time.Time) error {
	return nil
}
func (r *stubUserRepoForBranch) ResetFailedLogin(_ context.Context, _ string) error { return nil }
func (r *stubUserRepoForBranch) SoftDelete(_ context.Context, _ string) error       { return nil }

type stubClockForBranch struct{}

func (c *stubClockForBranch) Now() time.Time { return time.Date(2026, 4, 18, 12, 0, 0, 0, time.UTC) }

func newTestBranchService(branchRepo *stubBranchRepo, tenantRepo *stubTenantRepoForBranch) *service.BranchService {
	return service.NewBranchService(
		branchRepo,
		tenantRepo,
		&stubUserRepoForBranch{},
		&fakeAuditRepo{},
		&stubClockForBranch{},
	)
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

func TestCreateBranch_UnderLimit(t *testing.T) {
	t.Parallel()

	branchRepo := newStubBranchRepo()
	// Tenant allows 3 branches; 0 currently exist.
	tenantRepo := &stubTenantRepoForBranch{
		tenant: &model.Tenant{ID: "t1", Status: model.TenantStatusActive, MaxBranches: 3},
		count:  0,
	}
	svc := newTestBranchService(branchRepo, tenantRepo)

	out, err := svc.Create(context.Background(), service.CreateBranchInput{
		CallerUserID:   "user-1",
		CallerTenantID: "t1",
		Name:           "Main Branch",
		Code:           "MAIN",
		Country:        "ID",
		Timezone:       "Asia/Jakarta",
	})

	require.NoError(t, err)
	assert.Equal(t, "Main Branch", out.Name)
	assert.Equal(t, model.BranchStatusInactive, out.Status)
	require.Len(t, branchRepo.saved, 1)
}

func TestCreateBranch_AtLimit(t *testing.T) {
	t.Parallel()

	branchRepo := newStubBranchRepo()
	// Tenant has max_branches=1 and 1 already exists.
	tenantRepo := &stubTenantRepoForBranch{
		tenant: &model.Tenant{ID: "t1", Status: model.TenantStatusActive, MaxBranches: 1},
		count:  1,
	}
	svc := newTestBranchService(branchRepo, tenantRepo)

	_, err := svc.Create(context.Background(), service.CreateBranchInput{
		CallerUserID:   "user-1",
		CallerTenantID: "t1",
		Name:           "Second Branch",
		Code:           "SEC",
	})

	require.Error(t, err)
	assert.True(t, errors.Is(err, constants.ErrBranchLimitReached))
	assert.Empty(t, branchRepo.saved)
}

func TestCreateBranch_EnterpriseSentinelUnlimited(t *testing.T) {
	t.Parallel()

	branchRepo := newStubBranchRepo()
	// Enterprise: max_branches=999 — the 999 sentinel means unlimited.
	tenantRepo := &stubTenantRepoForBranch{
		tenant: &model.Tenant{ID: "t1", MaxBranches: model.MaxBranchesUnlimited},
		count:  500, // far above any other limit
	}
	svc := newTestBranchService(branchRepo, tenantRepo)

	out, err := svc.Create(context.Background(), service.CreateBranchInput{
		CallerUserID:   "user-1",
		CallerTenantID: "t1",
		Name:           "Branch 501",
		Code:           "B501",
	})

	require.NoError(t, err)
	assert.NotEmpty(t, out.ID)
}

func TestChangeBranchStatus_ValidTransition(t *testing.T) {
	t.Parallel()

	branchRepo := newStubBranchRepo()
	branch := &model.Branch{ID: "b1", TenantID: "t1", Status: model.BranchStatusInactive}
	branchRepo.branches["b1"] = branch

	tenantRepo := &stubTenantRepoForBranch{
		tenant: &model.Tenant{ID: "t1", MaxBranches: 5},
	}
	svc := newTestBranchService(branchRepo, tenantRepo)

	out, err := svc.ChangeStatus(context.Background(), service.ChangeBranchStatusInput{
		BranchID:       "b1",
		CallerUserID:   "user-1",
		CallerTenantID: "t1",
		NewStatus:      model.BranchStatusActive,
	})

	require.NoError(t, err)
	assert.Equal(t, model.BranchStatusActive, out.Status)
	assert.Equal(t, model.BranchStatusActive, branchRepo.statusUpdate["b1"])
}

func TestChangeBranchStatus_InvalidTransition(t *testing.T) {
	t.Parallel()

	branchRepo := newStubBranchRepo()
	// active → active is not a valid transition.
	branch := &model.Branch{ID: "b1", TenantID: "t1", Status: model.BranchStatusActive}
	branchRepo.branches["b1"] = branch

	tenantRepo := &stubTenantRepoForBranch{
		tenant: &model.Tenant{ID: "t1", MaxBranches: 5},
	}
	svc := newTestBranchService(branchRepo, tenantRepo)

	_, err := svc.ChangeStatus(context.Background(), service.ChangeBranchStatusInput{
		BranchID:       "b1",
		CallerUserID:   "user-1",
		CallerTenantID: "t1",
		NewStatus:      model.BranchStatusActive, // same state — invalid
	})

	require.Error(t, err)
	assert.True(t, errors.Is(err, constants.ErrInvalidStatusTransition))
}

func TestDeleteBranch_ActiveNotAllowed(t *testing.T) {
	t.Parallel()

	branchRepo := newStubBranchRepo()
	branch := &model.Branch{ID: "b1", TenantID: "t1", Status: model.BranchStatusActive}
	branchRepo.branches["b1"] = branch

	tenantRepo := &stubTenantRepoForBranch{
		tenant: &model.Tenant{ID: "t1", MaxBranches: 5},
	}
	svc := newTestBranchService(branchRepo, tenantRepo)

	err := svc.DeleteBranch(context.Background(), "t1", "user-1", "b1")

	require.Error(t, err)
	assert.True(t, errors.Is(err, constants.ErrInvalidStatusTransition),
		"deleting an active branch must return ErrInvalidStatusTransition")
}

func TestDeleteBranch_InactiveSucceeds(t *testing.T) {
	t.Parallel()

	branchRepo := newStubBranchRepo()
	branch := &model.Branch{ID: "b1", TenantID: "t1", Status: model.BranchStatusInactive}
	branchRepo.branches["b1"] = branch

	tenantRepo := &stubTenantRepoForBranch{
		tenant: &model.Tenant{ID: "t1", MaxBranches: 5},
	}
	svc := newTestBranchService(branchRepo, tenantRepo)

	err := svc.DeleteBranch(context.Background(), "t1", "user-1", "b1")

	require.NoError(t, err)
	_, exists := branchRepo.branches["b1"]
	assert.False(t, exists, "branch should be removed after soft-delete")
}
