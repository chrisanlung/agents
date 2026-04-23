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

// trackingMappingRepo records reconcile calls with the exact desired IDs.
type trackingMappingRepo struct {
	rows         map[string]*model.TherapistService // keyed therapistID+":"+serviceID
	reconcileCalls []reconcileCall
	deactivated    []string
}

type reconcileCall struct {
	therapistID string
	desiredIDs  []string
}

func newTrackingMappingRepo() *trackingMappingRepo {
	return &trackingMappingRepo{rows: make(map[string]*model.TherapistService)}
}

func (r *trackingMappingRepo) FindByTherapistID(_ context.Context, therapistID string) ([]*model.TherapistService, error) {
	var out []*model.TherapistService
	for _, row := range r.rows {
		if row.TherapistID == therapistID {
			out = append(out, row)
		}
	}
	return out, nil
}
func (r *trackingMappingRepo) FindByServiceID(_ context.Context, _ string) ([]*model.TherapistService, error) {
	return nil, nil
}
func (r *trackingMappingRepo) ReconcileForTherapist(_ context.Context, tenantID, therapistID, callerUserID string, desiredIDs []string) error {
	r.reconcileCalls = append(r.reconcileCalls, reconcileCall{therapistID: therapistID, desiredIDs: desiredIDs})
	// Apply the reconcile logic: insert new, re-activate existing, deactivate removed.
	now := time.Now()
	desired := make(map[string]struct{}, len(desiredIDs))
	for _, id := range desiredIDs {
		desired[id] = struct{}{}
	}
	for _, row := range r.rows {
		if row.TherapistID != therapistID {
			continue
		}
		if _, ok := desired[row.ServiceID]; ok {
			row.IsActive = true
		} else {
			row.IsActive = false
		}
	}
	for _, id := range desiredIDs {
		key := therapistID + ":" + id
		if _, exists := r.rows[key]; !exists {
			_ = tenantID
			r.rows[key] = &model.TherapistService{
				TherapistID: therapistID,
				ServiceID:   id,
				IsActive:    true,
				CreatedAt:   now,
				UpdatedAt:   now,
				CreatedBy:   &callerUserID,
				UpdatedBy:   &callerUserID,
			}
		}
	}
	return nil
}
func (r *trackingMappingRepo) DeactivateAllForTherapist(_ context.Context, therapistID string) error {
	r.deactivated = append(r.deactivated, therapistID)
	return nil
}

// stubServiceCatalogRepoForMapping provides FindByIDs for the mapping service.
type stubServiceCatalogRepoForMapping struct {
	rows map[string]*model.ServiceCatalog
}

func newStubServiceCatalogRepoForMapping(tenantID string, ids ...string) *stubServiceCatalogRepoForMapping {
	r := &stubServiceCatalogRepoForMapping{rows: make(map[string]*model.ServiceCatalog)}
	for _, id := range ids {
		r.rows[id] = &model.ServiceCatalog{ID: id, TenantID: tenantID, Name: "Svc " + id, IsActive: true, Currency: "IDR"}
	}
	return r
}

func (r *stubServiceCatalogRepoForMapping) Save(_ context.Context, _ *model.ServiceCatalog) error {
	return nil
}
func (r *stubServiceCatalogRepoForMapping) FindByID(_ context.Context, id string) (*model.ServiceCatalog, error) {
	s, ok := r.rows[id]
	if !ok {
		return nil, constants.ErrServiceNotFound
	}
	return s, nil
}
func (r *stubServiceCatalogRepoForMapping) FindByTenant(_ context.Context, _ string, _ service.ServiceFilter) ([]*model.ServiceCatalog, string, error) {
	return nil, "", nil
}
func (r *stubServiceCatalogRepoForMapping) Update(_ context.Context, _ *model.ServiceCatalog) error {
	return nil
}
func (r *stubServiceCatalogRepoForMapping) UpdateStatus(_ context.Context, _ string, _ bool) error {
	return nil
}
func (r *stubServiceCatalogRepoForMapping) SoftDelete(_ context.Context, _ string) error { return nil }
func (r *stubServiceCatalogRepoForMapping) FindByIDs(_ context.Context, tenantID string, ids []string) ([]*model.ServiceCatalog, error) {
	var out []*model.ServiceCatalog
	for _, id := range ids {
		if s, ok := r.rows[id]; ok && s.TenantID == tenantID {
			out = append(out, s)
		}
	}
	return out, nil
}

func newTestMappingSvc(therapistRepo *stubTherapistRepo, mappingRepo *trackingMappingRepo, svcRepo *stubServiceCatalogRepoForMapping) *service.MappingService {
	return service.NewMappingService(
		therapistRepo,
		svcRepo,
		mappingRepo,
		&fakeAuditRepo{},
		&stubTxManager{},
	)
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

func TestMappingReconcile_AddedIDsInserted(t *testing.T) {
	t.Parallel()

	therapistRepo := newStubTherapistRepo()
	therapistRepo.rows["th1"] = &model.Therapist{ID: "th1", BranchID: "b1", IsActive: true}
	mappingRepo := newTrackingMappingRepo()
	svcRepo := newStubServiceCatalogRepoForMapping("t1", "s1", "s2")

	svc := newTestMappingSvc(therapistRepo, mappingRepo, svcRepo)

	out, err := svc.Reconcile(context.Background(), service.ReconcileMappingInput{
		TherapistID:    "th1",
		CallerUserID:   "u1",
		CallerTenantID: "t1",
		IsAdmin:        true,
		ServiceIDs:     []string{"s1", "s2"},
	})

	require.NoError(t, err)
	require.Len(t, mappingRepo.reconcileCalls, 1)
	assert.ElementsMatch(t, []string{"s1", "s2"}, mappingRepo.reconcileCalls[0].desiredIDs)
	assert.Equal(t, "th1", out.TherapistID)
	// Both s1 and s2 should appear in result.
	serviceIDs := make([]string, len(out.Services))
	for i, s := range out.Services {
		serviceIDs[i] = s.ServiceID
	}
	assert.ElementsMatch(t, []string{"s1", "s2"}, serviceIDs)
}

func TestMappingReconcile_RemovedIDsSetInactive(t *testing.T) {
	t.Parallel()

	therapistRepo := newStubTherapistRepo()
	therapistRepo.rows["th1"] = &model.Therapist{ID: "th1", BranchID: "b1", IsActive: true}
	mappingRepo := newTrackingMappingRepo()
	// Pre-populate: th1 currently has s1 and s2.
	mappingRepo.rows["th1:s1"] = &model.TherapistService{TherapistID: "th1", ServiceID: "s1", IsActive: true}
	mappingRepo.rows["th1:s2"] = &model.TherapistService{TherapistID: "th1", ServiceID: "s2", IsActive: true}

	svcRepo := newStubServiceCatalogRepoForMapping("t1", "s1", "s2")
	svc := newTestMappingSvc(therapistRepo, mappingRepo, svcRepo)

	// Reconcile to only s1 — s2 should become is_active=false.
	_, err := svc.Reconcile(context.Background(), service.ReconcileMappingInput{
		TherapistID:    "th1",
		CallerUserID:   "u1",
		CallerTenantID: "t1",
		IsAdmin:        true,
		ServiceIDs:     []string{"s1"},
	})

	require.NoError(t, err)
	assert.True(t, mappingRepo.rows["th1:s1"].IsActive)
	assert.False(t, mappingRepo.rows["th1:s2"].IsActive, "s2 not in desired set should be deactivated")
}

func TestMappingReconcile_ReactivatesPreviouslyDeactivated(t *testing.T) {
	t.Parallel()

	therapistRepo := newStubTherapistRepo()
	therapistRepo.rows["th1"] = &model.Therapist{ID: "th1", BranchID: "b1", IsActive: true}
	mappingRepo := newTrackingMappingRepo()
	// s1 was previously deactivated.
	mappingRepo.rows["th1:s1"] = &model.TherapistService{TherapistID: "th1", ServiceID: "s1", IsActive: false}

	svcRepo := newStubServiceCatalogRepoForMapping("t1", "s1")
	svc := newTestMappingSvc(therapistRepo, mappingRepo, svcRepo)

	_, err := svc.Reconcile(context.Background(), service.ReconcileMappingInput{
		TherapistID:    "th1",
		CallerUserID:   "u1",
		CallerTenantID: "t1",
		IsAdmin:        true,
		ServiceIDs:     []string{"s1"}, // re-add previously removed s1
	})

	require.NoError(t, err)
	assert.True(t, mappingRepo.rows["th1:s1"].IsActive, "previously deactivated s1 should be re-activated")
}

func TestMappingReconcile_NoHardDeletes(t *testing.T) {
	t.Parallel()

	therapistRepo := newStubTherapistRepo()
	therapistRepo.rows["th1"] = &model.Therapist{ID: "th1", BranchID: "b1", IsActive: true}
	mappingRepo := newTrackingMappingRepo()
	mappingRepo.rows["th1:s1"] = &model.TherapistService{TherapistID: "th1", ServiceID: "s1", IsActive: true}

	svcRepo := newStubServiceCatalogRepoForMapping("t1", "s1")
	svc := newTestMappingSvc(therapistRepo, mappingRepo, svcRepo)

	// Reconcile to empty — s1 must remain in the rows map (just inactive).
	_, err := svc.Reconcile(context.Background(), service.ReconcileMappingInput{
		TherapistID:    "th1",
		CallerUserID:   "u1",
		CallerTenantID: "t1",
		IsAdmin:        true,
		ServiceIDs:     []string{},
	})

	require.NoError(t, err)
	// Row must still exist (no hard delete).
	row, exists := mappingRepo.rows["th1:s1"]
	require.True(t, exists, "mapping row must not be hard-deleted")
	assert.False(t, row.IsActive, "deactivated but still present")
}

func TestMappingReconcile_ServiceNotFound(t *testing.T) {
	t.Parallel()

	therapistRepo := newStubTherapistRepo()
	therapistRepo.rows["th1"] = &model.Therapist{ID: "th1", BranchID: "b1", IsActive: true}
	mappingRepo := newTrackingMappingRepo()
	// Only s1 exists; s99 does not.
	svcRepo := newStubServiceCatalogRepoForMapping("t1", "s1")
	svc := newTestMappingSvc(therapistRepo, mappingRepo, svcRepo)

	_, err := svc.Reconcile(context.Background(), service.ReconcileMappingInput{
		TherapistID:    "th1",
		CallerUserID:   "u1",
		CallerTenantID: "t1",
		IsAdmin:        true,
		ServiceIDs:     []string{"s1", "s99"},
	})

	require.Error(t, err)
	assert.True(t, errors.Is(err, constants.ErrServiceNotFound))
}
