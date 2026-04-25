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
// Fakes
// ---------------------------------------------------------------------------

type stubTherapistRepo struct {
	rows         map[string]*model.Therapist
	saved        []*model.Therapist
	statusChange map[string]bool
	deleted      []string
}

func newStubTherapistRepo() *stubTherapistRepo {
	return &stubTherapistRepo{
		rows:         make(map[string]*model.Therapist),
		statusChange: make(map[string]bool),
	}
}

func (r *stubTherapistRepo) Save(_ context.Context, t *model.Therapist) error {
	r.saved = append(r.saved, t)
	r.rows[t.ID] = t
	return nil
}
func (r *stubTherapistRepo) FindByID(_ context.Context, id string) (*model.Therapist, error) {
	t, ok := r.rows[id]
	if !ok {
		return nil, constants.ErrTherapistNotFound
	}
	return t, nil
}
func (r *stubTherapistRepo) FindByTenant(_ context.Context, tenantID string, f service.TherapistFilter) ([]*model.Therapist, int64, error) {
	var out []*model.Therapist
	for _, t := range r.rows {
		if t.TenantID != tenantID {
			continue
		}
		// BranchIDs takes precedence over BranchID when non-empty (branch_admin scope).
		if len(f.BranchIDs) > 0 {
			found := false
			for _, bid := range f.BranchIDs {
				if t.BranchID == bid {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		} else if f.BranchID != nil && t.BranchID != *f.BranchID {
			continue
		}
		if f.IsActive != nil && t.IsActive != *f.IsActive {
			continue
		}
		if f.IsActive == nil && !t.IsActive {
			continue // default: active only
		}
		out = append(out, t)
	}
	return out, int64(len(out)), nil
}
func (r *stubTherapistRepo) Update(_ context.Context, t *model.Therapist) error {
	r.rows[t.ID] = t
	return nil
}
func (r *stubTherapistRepo) UpdateStatus(_ context.Context, id string, isActive bool) error {
	t, ok := r.rows[id]
	if !ok {
		return constants.ErrTherapistNotFound
	}
	t.IsActive = isActive
	r.statusChange[id] = isActive
	return nil
}
func (r *stubTherapistRepo) SoftDelete(_ context.Context, id string) error {
	t, ok := r.rows[id]
	if !ok {
		return constants.ErrTherapistNotFound
	}
	now := time.Now()
	t.DeletedAt = &now
	t.IsActive = false
	r.deleted = append(r.deleted, id)
	delete(r.rows, id) // mimic FindByID returning not-found after delete
	return nil
}
func (r *stubTherapistRepo) UpdatePhotoKey(_ context.Context, id string, newKey *string, _ string) (*string, error) {
	t, ok := r.rows[id]
	if !ok {
		return nil, constants.ErrTherapistNotFound
	}
	old := t.PhotoKey
	t.PhotoKey = newKey
	return old, nil
}

type stubTherapistServiceRepo struct {
	deactivated []string
}

func (r *stubTherapistServiceRepo) FindByTherapistID(_ context.Context, _ string) ([]*model.TherapistService, error) {
	return nil, nil
}
func (r *stubTherapistServiceRepo) FindByServiceID(_ context.Context, _ string) ([]*model.TherapistService, error) {
	return nil, nil
}
func (r *stubTherapistServiceRepo) ReconcileForTherapist(_ context.Context, _, _, _ string, _ []string) error {
	return nil
}
func (r *stubTherapistServiceRepo) DeactivateAllForTherapist(_ context.Context, therapistID string) error {
	r.deactivated = append(r.deactivated, therapistID)
	return nil
}

type stubServiceCatalogRepoForTherapist struct{}

func (r *stubServiceCatalogRepoForTherapist) Save(_ context.Context, _ *model.ServiceCatalog) error {
	return nil
}
func (r *stubServiceCatalogRepoForTherapist) FindByID(_ context.Context, id string) (*model.ServiceCatalog, error) {
	return nil, constants.ErrServiceNotFound
}
func (r *stubServiceCatalogRepoForTherapist) FindByTenant(_ context.Context, _ string, _ service.ServiceFilter) ([]*model.ServiceCatalog, int64, error) {
	return nil, 0, nil
}
func (r *stubServiceCatalogRepoForTherapist) Update(_ context.Context, _ *model.ServiceCatalog) error {
	return nil
}
func (r *stubServiceCatalogRepoForTherapist) UpdateStatus(_ context.Context, _ string, _ bool) error {
	return nil
}
func (r *stubServiceCatalogRepoForTherapist) SoftDelete(_ context.Context, _ string) error {
	return nil
}
func (r *stubServiceCatalogRepoForTherapist) FindByIDs(_ context.Context, _ string, _ []string) ([]*model.ServiceCatalog, error) {
	return nil, nil
}

type stubBranchRepoForTherapist struct {
	branches map[string]*model.Branch
}

func newStubBranchRepoForTherapist(tenantID string, branchIDs ...string) *stubBranchRepoForTherapist {
	r := &stubBranchRepoForTherapist{branches: make(map[string]*model.Branch)}
	for _, id := range branchIDs {
		r.branches[id] = &model.Branch{ID: id, TenantID: tenantID, Status: model.BranchStatusActive}
	}
	return r
}

func (r *stubBranchRepoForTherapist) FindByID(_ context.Context, id string) (*model.Branch, error) {
	b, ok := r.branches[id]
	if !ok {
		return nil, constants.ErrBranchNotFound
	}
	return b, nil
}
func (r *stubBranchRepoForTherapist) FindByTenant(_ context.Context, _ string, _ service.BranchFilter) ([]*model.Branch, int64, error) {
	return nil, 0, nil
}
func (r *stubBranchRepoForTherapist) Save(_ context.Context, _ *model.Branch) error { return nil }
func (r *stubBranchRepoForTherapist) Update(_ context.Context, _ *model.Branch) error { return nil }
func (r *stubBranchRepoForTherapist) UpdateStatus(_ context.Context, _, _ string, _ *time.Time) error {
	return nil
}
func (r *stubBranchRepoForTherapist) SoftDelete(_ context.Context, _ string) error { return nil }

type stubTxManager struct{}

func (s *stubTxManager) WithTx(_ context.Context, fn func(context.Context) error) error {
	return fn(context.Background())
}
func (s *stubTxManager) SetTenantContext(_ context.Context, _, _ string) error { return nil }

func newTestTherapistSvc(therapistRepo *stubTherapistRepo, mappingRepo *stubTherapistServiceRepo, branchRepo *stubBranchRepoForTherapist) *service.TherapistSvc {
	return service.NewTherapistSvc(
		therapistRepo,
		mappingRepo,
		&stubServiceCatalogRepoForTherapist{},
		branchRepo,
		&fakeAuditRepo{},
		&stubClockForBranch{},
		&stubTxManager{},
	)
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

func TestTherapistCreate_Success(t *testing.T) {
	t.Parallel()

	therapistRepo := newStubTherapistRepo()
	branchRepo := newStubBranchRepoForTherapist("t1", "b1")
	svc := newTestTherapistSvc(therapistRepo, &stubTherapistServiceRepo{}, branchRepo)

	out, err := svc.Create(context.Background(), service.CreateTherapistInput{
		CallerUserID:   "u1",
		CallerTenantID: "t1",
		CallerBranches: []string{"b1"},
		IsAdmin:        true, // tenant_admin — no cross-branch check
		BranchID:       "b1",
		FullName:       "Siti Rahma",
		HeightCm:       165,
		WeightKg:       55,
		Build:          "sedang",
	})

	require.NoError(t, err)
	assert.Equal(t, "Siti Rahma", out.FullName)
	assert.Equal(t, "b1", out.BranchID)
	assert.True(t, out.IsActive)
	require.Len(t, therapistRepo.saved, 1)
}

func TestTherapistCreate_CrossBranchForbidden_BranchAdmin(t *testing.T) {
	t.Parallel()

	// branch_admin assigned to b1 tries to create a therapist at b2.
	therapistRepo := newStubTherapistRepo()
	branchRepo := newStubBranchRepoForTherapist("t1", "b1", "b2")
	svc := newTestTherapistSvc(therapistRepo, &stubTherapistServiceRepo{}, branchRepo)

	_, err := svc.Create(context.Background(), service.CreateTherapistInput{
		CallerUserID:   "u1",
		CallerTenantID: "t1",
		CallerBranches: []string{"b1"}, // assigned to b1 only
		IsAdmin:        false,
		BranchID:       "b2", // targeting b2 → forbidden
		FullName:       "Budi",
	})

	require.Error(t, err)
	assert.True(t, errors.Is(err, constants.ErrCrossBranchForbidden))
	assert.Empty(t, therapistRepo.saved)
}

func TestTherapistList_FiltersActive(t *testing.T) {
	t.Parallel()

	therapistRepo := newStubTherapistRepo()
	therapistRepo.rows["t-a"] = &model.Therapist{ID: "t-a", TenantID: "t1", BranchID: "b1", IsActive: true, FullName: "A"}
	therapistRepo.rows["t-b"] = &model.Therapist{ID: "t-b", TenantID: "t1", BranchID: "b1", IsActive: false, FullName: "B"}

	svc := newTestTherapistSvc(therapistRepo, &stubTherapistServiceRepo{}, newStubBranchRepoForTherapist("t1", "b1"))

	// Default: active only.
	out, err := svc.List(context.Background(), service.ListTherapistsInput{
		CallerTenantID: "t1",
		IsAdmin:        true,
	})
	require.NoError(t, err)
	require.Len(t, out.Therapists, 1)
	assert.Equal(t, "t-a", out.Therapists[0].ID)
}

func TestTherapistList_BranchAdminRestriction(t *testing.T) {
	t.Parallel()

	therapistRepo := newStubTherapistRepo()
	therapistRepo.rows["t-b1"] = &model.Therapist{ID: "t-b1", TenantID: "t1", BranchID: "b1", IsActive: true, FullName: "X"}
	therapistRepo.rows["t-b2"] = &model.Therapist{ID: "t-b2", TenantID: "t1", BranchID: "b2", IsActive: true, FullName: "Y"}

	svc := newTestTherapistSvc(therapistRepo, &stubTherapistServiceRepo{}, newStubBranchRepoForTherapist("t1", "b1", "b2"))

	// branch_admin at b1 should only see b1 therapists.
	out, err := svc.List(context.Background(), service.ListTherapistsInput{
		CallerTenantID: "t1",
		CallerBranches: []string{"b1"},
		IsAdmin:        false,
	})
	require.NoError(t, err)
	require.Len(t, out.Therapists, 1)
	assert.Equal(t, "t-b1", out.Therapists[0].ID)
}

func TestTherapistStatusToggle(t *testing.T) {
	t.Parallel()

	therapistRepo := newStubTherapistRepo()
	therapistRepo.rows["th1"] = &model.Therapist{ID: "th1", TenantID: "t1", BranchID: "b1", IsActive: true}

	svc := newTestTherapistSvc(therapistRepo, &stubTherapistServiceRepo{}, newStubBranchRepoForTherapist("t1", "b1"))

	out, err := svc.ChangeStatus(context.Background(), service.ChangeTherapistStatusInput{
		TherapistID:    "th1",
		CallerUserID:   "u1",
		CallerTenantID: "t1",
		IsAdmin:        true,
		IsActive:       false,
	})

	require.NoError(t, err)
	assert.False(t, out.IsActive)
	assert.False(t, therapistRepo.statusChange["th1"])
}

func TestTherapistSoftDelete_CascadesMappingDeactivation(t *testing.T) {
	t.Parallel()

	therapistRepo := newStubTherapistRepo()
	therapistRepo.rows["th1"] = &model.Therapist{ID: "th1", TenantID: "t1", BranchID: "b1", IsActive: true}
	mappingRepo := &stubTherapistServiceRepo{}

	svc := newTestTherapistSvc(therapistRepo, mappingRepo, newStubBranchRepoForTherapist("t1", "b1"))

	err := svc.SoftDelete(context.Background(), "t1", "u1", nil, true, "th1")

	require.NoError(t, err)
	assert.Contains(t, therapistRepo.deleted, "th1")
	// Cascade: mapping deactivation must have been called.
	assert.Contains(t, mappingRepo.deactivated, "th1")
}

func TestTherapistSoftDelete_CrossBranchForbidden(t *testing.T) {
	t.Parallel()

	therapistRepo := newStubTherapistRepo()
	therapistRepo.rows["th1"] = &model.Therapist{ID: "th1", TenantID: "t1", BranchID: "b2", IsActive: true}

	svc := newTestTherapistSvc(therapistRepo, &stubTherapistServiceRepo{}, newStubBranchRepoForTherapist("t1", "b1", "b2"))

	err := svc.SoftDelete(context.Background(), "t1", "u1", []string{"b1"}, false, "th1")

	require.Error(t, err)
	assert.True(t, errors.Is(err, constants.ErrCrossBranchForbidden))
	assert.Empty(t, therapistRepo.deleted)
}
