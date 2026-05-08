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

// FindByTenant returns branches that match the filter, mirroring the real
// repository behaviour used to test the branch-scope enforcement logic.
func (r *stubBranchRepo) FindByTenant(_ context.Context, tenantID string, filter service.BranchFilter) ([]*model.Branch, int64, error) {
	// Build an allow-set from filter.IDs (nil/empty means no ID restriction).
	allowIDs := make(map[string]bool, len(filter.IDs))
	restrictByID := len(filter.IDs) > 0
	for _, id := range filter.IDs {
		allowIDs[id] = true
	}

	var out []*model.Branch
	for _, b := range r.branches {
		if tenantID != "" && b.TenantID != tenantID {
			continue
		}
		if filter.Status != "" && filter.Status != "all" && b.Status != filter.Status {
			continue
		}
		if restrictByID && !allowIDs[b.ID] {
			continue
		}
		out = append(out, b)
	}
	return out, int64(len(out)), nil
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
func (r *stubTenantRepoForBranch) List(_ context.Context, _ service.TenantFilter) ([]*service.TenantWithCounts, int64, error) {
	return nil, 0, nil
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
func (r *stubUserRepoForBranch) FindByUsername(_ context.Context, _ string) (*model.User, error) {
	return nil, constants.ErrUserNotFound
}
func (r *stubUserRepoForBranch) FindByID(_ context.Context, _ string) (*model.User, error) {
	return &model.User{MustChangePassword: false}, nil
}
func (r *stubUserRepoForBranch) FindByTenant(_ context.Context, _ string, _ service.UserFilter) ([]*model.User, int64, error) {
	return nil, 0, nil
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

// ---------------------------------------------------------------------------
// Branch-scope filter tests (security fix — see branch_service.go ListByTenant)
// ---------------------------------------------------------------------------

func TestListByTenant_BranchScopeFilter(t *testing.T) {
	t.Parallel()

	// Seed three branches belonging to tenant t1.
	branchA := &model.Branch{ID: "branch-a", TenantID: "t1", Status: model.BranchStatusActive}
	branchB := &model.Branch{ID: "branch-b", TenantID: "t1", Status: model.BranchStatusActive}
	branchC := &model.Branch{ID: "branch-c", TenantID: "t1", Status: model.BranchStatusInactive}

	tenantRepo := &stubTenantRepoForBranch{
		tenant: &model.Tenant{ID: "t1", MaxBranches: 5},
	}

	tests := []struct {
		name           string
		isAdmin        bool
		callerBranches []string
		scopeToMine    bool
		statusFilter   string
		wantIDs        []string // expected branch IDs in result (any order)
		wantTotal      int64
	}{
		{
			name:           "admin with empty CallerBranches sees all branches",
			isAdmin:        true,
			callerBranches: []string{},
			wantIDs:        []string{"branch-a", "branch-b", "branch-c"},
			wantTotal:      3,
		},
		{
			name:           "admin with non-empty CallerBranches still sees all branches (admin override)",
			isAdmin:        true,
			callerBranches: []string{"branch-a"},
			scopeToMine:    true, // even when scope=mine is set, admin always sees all
			wantIDs:        []string{"branch-a", "branch-b", "branch-c"},
			wantTotal:      3,
		},
		{
			name:           "non-admin WITHOUT scope=mine sees all branches (default tenant-admin path)",
			isAdmin:        false,
			callerBranches: []string{"branch-a"},
			wantIDs:        []string{"branch-a", "branch-b", "branch-c"},
			wantTotal:      3,
		},
		{
			name:           "non-admin with scope=mine and two assigned branches sees only those two",
			isAdmin:        false,
			callerBranches: []string{"branch-a", "branch-c"},
			scopeToMine:    true,
			wantIDs:        []string{"branch-a", "branch-c"},
			wantTotal:      2,
		},
		{
			name:           "non-admin with scope=mine and empty CallerBranches sees nothing (defensive zero)",
			isAdmin:        false,
			callerBranches: []string{},
			scopeToMine:    true,
			wantIDs:        []string{},
			wantTotal:      0,
		},
		{
			name:           "non-admin with scope=mine + status filter: only active assigned branches returned",
			isAdmin:        false,
			callerBranches: []string{"branch-a", "branch-c"},
			scopeToMine:    true,
			statusFilter:   model.BranchStatusActive,
			wantIDs:        []string{"branch-a"},
			wantTotal:      1,
		},
		{
			name:           "non-admin with scope=mine and one assigned branch sees only that branch",
			isAdmin:        false,
			callerBranches: []string{"branch-b"},
			scopeToMine:    true,
			wantIDs:        []string{"branch-b"},
			wantTotal:      1,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			branchRepo := newStubBranchRepo()
			branchRepo.branches["branch-a"] = branchA
			branchRepo.branches["branch-b"] = branchB
			branchRepo.branches["branch-c"] = branchC

			svc := newTestBranchService(branchRepo, tenantRepo)

			out, err := svc.ListByTenant(context.Background(), service.ListBranchesInput{
				CallerTenantID:        "t1",
				CallerBranches:        tt.callerBranches,
				IsAdmin:               tt.isAdmin,
				ScopeToCallerBranches: tt.scopeToMine,
				Status:                tt.statusFilter,
				Page:                  1,
				Limit:                 10,
			})

			require.NoError(t, err)
			assert.Equal(t, tt.wantTotal, out.TotalCount)

			gotIDs := make([]string, len(out.Branches))
			for i, b := range out.Branches {
				gotIDs[i] = b.ID

			}
			assert.ElementsMatch(t, tt.wantIDs, gotIDs)
		})
	}
}
