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

type stubServiceCatalogRepo struct {
	rows    map[string]*model.ServiceCatalog
	saved   []*model.ServiceCatalog
	deleted []string
}

func newStubServiceCatalogRepo() *stubServiceCatalogRepo {
	return &stubServiceCatalogRepo{rows: make(map[string]*model.ServiceCatalog)}
}

func (r *stubServiceCatalogRepo) Save(_ context.Context, s *model.ServiceCatalog) error {
	r.saved = append(r.saved, s)
	r.rows[s.ID] = s
	return nil
}
func (r *stubServiceCatalogRepo) FindByID(_ context.Context, id string) (*model.ServiceCatalog, error) {
	s, ok := r.rows[id]
	if !ok {
		return nil, constants.ErrServiceNotFound
	}
	return s, nil
}
func (r *stubServiceCatalogRepo) FindByTenant(_ context.Context, tenantID string, f service.ServiceFilter) ([]*model.ServiceCatalog, int64, error) {
	var out []*model.ServiceCatalog
	for _, s := range r.rows {
		if s.TenantID != tenantID {
			continue
		}
		if s.DeletedAt != nil {
			continue
		}
		if f.IsActive != nil && s.IsActive != *f.IsActive {
			continue
		}
		if f.IsActive == nil && !s.IsActive {
			continue
		}
		if f.Category != nil && (s.Category == nil || *s.Category != *f.Category) {
			continue
		}
		out = append(out, s)
	}
	return out, int64(len(out)), nil
}
func (r *stubServiceCatalogRepo) Update(_ context.Context, s *model.ServiceCatalog) error {
	r.rows[s.ID] = s
	return nil
}
func (r *stubServiceCatalogRepo) UpdateStatus(_ context.Context, id string, isActive bool) error {
	s, ok := r.rows[id]
	if !ok {
		return constants.ErrServiceNotFound
	}
	s.IsActive = isActive
	return nil
}
func (r *stubServiceCatalogRepo) SoftDelete(_ context.Context, id string) error {
	_, ok := r.rows[id]
	if !ok {
		return constants.ErrServiceNotFound
	}
	now := time.Now()
	r.rows[id].DeletedAt = &now
	r.deleted = append(r.deleted, id)
	return nil
}
func (r *stubServiceCatalogRepo) FindByIDs(_ context.Context, tenantID string, ids []string) ([]*model.ServiceCatalog, error) {
	var out []*model.ServiceCatalog
	for _, id := range ids {
		s, ok := r.rows[id]
		if ok && s.TenantID == tenantID && s.DeletedAt == nil {
			out = append(out, s)
		}
	}
	return out, nil
}

type stubMappingRepoForCatalog struct{}

func (r *stubMappingRepoForCatalog) FindByTherapistID(_ context.Context, _ string) ([]*model.TherapistService, error) {
	return nil, nil
}
func (r *stubMappingRepoForCatalog) FindByServiceID(_ context.Context, _ string) ([]*model.TherapistService, error) {
	return nil, nil
}
func (r *stubMappingRepoForCatalog) ReconcileForTherapist(_ context.Context, _, _, _ string, _ []string) error {
	return nil
}
func (r *stubMappingRepoForCatalog) DeactivateAllForTherapist(_ context.Context, _ string) error {
	return nil
}

type stubTherapistRepoForCatalog struct{}

func (r *stubTherapistRepoForCatalog) Save(_ context.Context, _ *model.Therapist) error { return nil }
func (r *stubTherapistRepoForCatalog) FindByID(_ context.Context, _ string) (*model.Therapist, error) {
	return nil, constants.ErrTherapistNotFound
}
func (r *stubTherapistRepoForCatalog) FindByTenant(_ context.Context, _ string, _ service.TherapistFilter) ([]*model.Therapist, int64, error) {
	return nil, 0, nil
}
func (r *stubTherapistRepoForCatalog) Update(_ context.Context, _ *model.Therapist) error { return nil }
func (r *stubTherapistRepoForCatalog) UpdateStatus(_ context.Context, _ string, _ bool) error {
	return nil
}
func (r *stubTherapistRepoForCatalog) SoftDelete(_ context.Context, _ string) error { return nil }
func (r *stubTherapistRepoForCatalog) UpdatePhotoKey(_ context.Context, _ string, _ *string, _ string) (*string, error) {
	return nil, nil
}

func newTestCatalogSvc(svcRepo *stubServiceCatalogRepo) *service.CatalogService {
	return service.NewCatalogService(
		svcRepo,
		&stubMappingRepoForCatalog{},
		&stubTherapistRepoForCatalog{},
		newStubBranchRepoForTherapist("t1"),
		&fakeAuditRepo{},
		&stubClockForBranch{},
	)
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

func TestCatalogCreate_Success(t *testing.T) {
	t.Parallel()

	repo := newStubServiceCatalogRepo()
	svc := newTestCatalogSvc(repo)

	cat := "Pijat"
	out, err := svc.Create(context.Background(), service.CreateServiceInput{
		CallerUserID:    "u1",
		CallerTenantID:  "t1",
		Name:            "Pijat Relaksasi",
		Category:        &cat,
		DurationMinutes: 60,
		PriceIDR:        150000,
	})

	require.NoError(t, err)
	assert.Equal(t, "Pijat Relaksasi", out.Name)
	assert.Equal(t, "t1", out.TenantID)
	assert.Equal(t, "IDR", out.Currency)
	assert.True(t, out.IsActive)
	require.Len(t, repo.saved, 1)
}

func TestCatalogList_ByCategory(t *testing.T) {
	t.Parallel()

	repo := newStubServiceCatalogRepo()
	cat1 := "Pijat"
	cat2 := "Refleksi"
	repo.rows["s1"] = &model.ServiceCatalog{ID: "s1", TenantID: "t1", Name: "A", Category: &cat1, IsActive: true, Currency: "IDR"}
	repo.rows["s2"] = &model.ServiceCatalog{ID: "s2", TenantID: "t1", Name: "B", Category: &cat2, IsActive: true, Currency: "IDR"}
	repo.rows["s3"] = &model.ServiceCatalog{ID: "s3", TenantID: "t1", Name: "C", Category: &cat1, IsActive: true, Currency: "IDR"}

	svc := newTestCatalogSvc(repo)

	out, err := svc.List(context.Background(), service.ListServicesInput{
		CallerTenantID: "t1",
		Category:       &cat1,
	})

	require.NoError(t, err)
	assert.Len(t, out.Services, 2)
	for _, sv := range out.Services {
		require.NotNil(t, sv.Category)
		assert.Equal(t, "Pijat", *sv.Category)
	}
}

func TestCatalogGet_DetailIncludesOnlyActiveMappings(t *testing.T) {
	t.Parallel()

	// stubMappingRepoForCatalog.FindByServiceID always returns empty —
	// the Therapists array must be empty (not nil) per §11.3 flag #6.
	repo := newStubServiceCatalogRepo()
	repo.rows["s1"] = &model.ServiceCatalog{ID: "s1", TenantID: "t1", Name: "Pijat", IsActive: true, Currency: "IDR"}

	svc := newTestCatalogSvc(repo)

	out, err := svc.Get(context.Background(), "t1", "s1")

	require.NoError(t, err)
	assert.Equal(t, "s1", out.ID)
	assert.NotNil(t, out.Therapists)
	assert.Empty(t, out.Therapists)
}

func TestCatalogGet_NotFound(t *testing.T) {
	t.Parallel()

	repo := newStubServiceCatalogRepo()
	svc := newTestCatalogSvc(repo)

	_, err := svc.Get(context.Background(), "t1", "non-existent")

	require.Error(t, err)
	assert.True(t, errors.Is(err, constants.ErrServiceNotFound))
}
