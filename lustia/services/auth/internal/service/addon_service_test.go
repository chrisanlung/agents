package service_test

import (
	"context"
	"errors"
	"sort"
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

type stubAddonRepo struct {
	rows          map[string]*model.Addon
	saved         []*model.Addon
	deleted       []string
	statusUpdates map[string]bool
	sortOrders    map[string]int
	// inject errors
	saveErr            error
	findByIDErr        error
	findByTenantErr    error
	bulkUpdateErr      error
}

func newStubAddonRepo() *stubAddonRepo {
	return &stubAddonRepo{
		rows:          make(map[string]*model.Addon),
		statusUpdates: make(map[string]bool),
		sortOrders:    make(map[string]int),
	}
}

func (r *stubAddonRepo) Save(_ context.Context, a *model.Addon) error {
	if r.saveErr != nil {
		return r.saveErr
	}
	// Enforce partial unique: (tenant_id, name) WHERE deleted_at IS NULL.
	for _, existing := range r.rows {
		if existing.TenantID == a.TenantID && existing.Name == a.Name && existing.DeletedAt == nil {
			return constants.ErrDuplicateAddonName
		}
	}
	cp := *a
	cp.CreatedAt = time.Now()
	cp.UpdatedAt = time.Now()
	r.rows[a.ID] = &cp
	r.saved = append(r.saved, &cp)
	return nil
}

func (r *stubAddonRepo) FindByID(_ context.Context, id string) (*model.Addon, error) {
	if r.findByIDErr != nil {
		return nil, r.findByIDErr
	}
	a, ok := r.rows[id]
	if !ok || a.DeletedAt != nil {
		return nil, constants.ErrAddonNotFound
	}
	cp := *a
	return &cp, nil
}

func (r *stubAddonRepo) FindByIDs(_ context.Context, ids []string) ([]*model.Addon, error) {
	var out []*model.Addon
	for _, id := range ids {
		if a, ok := r.rows[id]; ok && a.DeletedAt == nil {
			cp := *a
			out = append(out, &cp)
		}
	}
	return out, nil
}

func (r *stubAddonRepo) FindByTenant(_ context.Context, tenantID string, f service.AddonFilter) ([]*model.Addon, int64, error) {
	if r.findByTenantErr != nil {
		return nil, 0, r.findByTenantErr
	}

	limit := f.Limit
	if limit <= 0 || limit > 200 {
		limit = 10
	}
	page := f.Page
	if page < 1 {
		page = 1
	}

	// Collect matching rows into a stable-ordered slice (sort by sort_order, created_at, id).
	var matching []*model.Addon
	for _, a := range r.rows {
		if a.TenantID != tenantID {
			continue
		}
		if a.DeletedAt != nil {
			continue
		}
		if f.IsActive != nil && a.IsActive != *f.IsActive {
			continue
		}
		cp := *a
		matching = append(matching, &cp)
	}

	sort.Slice(matching, func(i, j int) bool {
		if matching[i].SortOrder != matching[j].SortOrder {
			return matching[i].SortOrder < matching[j].SortOrder
		}
		if !matching[i].CreatedAt.Equal(matching[j].CreatedAt) {
			return matching[i].CreatedAt.Before(matching[j].CreatedAt)
		}
		return matching[i].ID < matching[j].ID
	})

	total := int64(len(matching))

	// Apply offset pagination.
	start := (page - 1) * limit
	if start >= len(matching) {
		return []*model.Addon{}, total, nil
	}
	end := start + limit
	if end > len(matching) {
		end = len(matching)
	}
	return matching[start:end], total, nil
}

func (r *stubAddonRepo) Update(_ context.Context, a *model.Addon) error {
	existing, ok := r.rows[a.ID]
	if !ok || existing.DeletedAt != nil {
		return constants.ErrAddonNotFound
	}
	// Enforce partial unique on name change.
	for _, row := range r.rows {
		if row.ID != a.ID && row.TenantID == a.TenantID && row.Name == a.Name && row.DeletedAt == nil {
			return constants.ErrDuplicateAddonName
		}
	}
	cp := *a
	cp.UpdatedAt = time.Now()
	r.rows[a.ID] = &cp
	return nil
}

func (r *stubAddonRepo) UpdateStatus(_ context.Context, id string, isActive bool, _ string) error {
	a, ok := r.rows[id]
	if !ok || a.DeletedAt != nil {
		return constants.ErrAddonNotFound
	}
	a.IsActive = isActive
	r.statusUpdates[id] = isActive
	return nil
}

func (r *stubAddonRepo) SoftDelete(_ context.Context, id string, _ string) error {
	a, ok := r.rows[id]
	if !ok || a.DeletedAt != nil {
		return constants.ErrAddonNotFound
	}
	now := time.Now()
	a.DeletedAt = &now
	a.IsActive = false
	r.deleted = append(r.deleted, id)
	return nil
}

func (r *stubAddonRepo) BulkUpdateSortOrder(_ context.Context, items []service.AddonSortOrderItem) error {
	if r.bulkUpdateErr != nil {
		return r.bulkUpdateErr
	}
	for _, it := range items {
		if a, ok := r.rows[it.ID]; ok {
			a.SortOrder = it.SortOrder
			r.sortOrders[it.ID] = it.SortOrder
		}
	}
	return nil
}

// stubAuditRepoForAddon is a no-op AuditRepository for addon tests.
type stubAuditRepoForAddon struct{}

func (stubAuditRepoForAddon) Append(_ context.Context, _ service.AuditEntry) error { return nil }

// noopTxForAddon passes through without a real transaction.
type noopTxForAddon struct{}

func (noopTxForAddon) WithTx(ctx context.Context, fn func(context.Context) error) error {
	return fn(ctx)
}
func (noopTxForAddon) SetTenantContext(_ context.Context, _, _ string) error { return nil }

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func newAddonSvc(repo *stubAddonRepo) *service.AddonService {
	return service.NewAddonService(repo, stubAuditRepoForAddon{}, &stubClockForBranch{}, noopTxForAddon{})
}

func seedAddon(repo *stubAddonRepo, id, tenantID, name string, sortOrder int) *model.Addon {
	a := &model.Addon{
		ID:        id,
		TenantID:  tenantID,
		Name:      name,
		PriceIDR:  10000,
		IsActive:  true,
		SortOrder: sortOrder,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	repo.rows[id] = a
	return a
}

const (
	tenantA = "aaaa0000-0000-0000-0000-000000000001"
	tenantB = "bbbb0000-0000-0000-0000-000000000002"
	userID1 = "user0000-0000-0000-0000-000000000001"
)

// ---------------------------------------------------------------------------
// Create
// ---------------------------------------------------------------------------

func TestAddonService_Create_Success(t *testing.T) {
	t.Parallel()
	repo := newStubAddonRepo()
	svc := newAddonSvc(repo)

	out, err := svc.Create(context.Background(), service.CreateAddonInput{
		CallerUserID:   userID1,
		CallerTenantID: tenantA,
		Name:           "Aromaterapi",
		PriceIDR:       35000,
		SortOrder:      0,
	})

	require.NoError(t, err)
	assert.NotEmpty(t, out.ID)
	assert.Equal(t, "Aromaterapi", out.Name)
	assert.Equal(t, int64(35000), out.PriceIDR)
	assert.True(t, out.IsActive)
	assert.Len(t, repo.saved, 1)
	// Tenant isolation verified via repo.saved[0].TenantID (model level) rather
	// than out.TenantID (AddonDetail intentionally omits tenant_id).
	assert.Equal(t, tenantA, repo.saved[0].TenantID)
}

func TestAddonService_Create_DuplicateName_SameTenant(t *testing.T) {
	t.Parallel()
	repo := newStubAddonRepo()
	svc := newAddonSvc(repo)

	_, err := svc.Create(context.Background(), service.CreateAddonInput{
		CallerUserID: userID1, CallerTenantID: tenantA, Name: "Kerokan", PriceIDR: 25000,
	})
	require.NoError(t, err)

	_, err = svc.Create(context.Background(), service.CreateAddonInput{
		CallerUserID: userID1, CallerTenantID: tenantA, Name: "Kerokan", PriceIDR: 30000,
	})
	require.Error(t, err)
	assert.True(t, errors.Is(err, constants.ErrDuplicateAddonName), "expected ErrDuplicateAddonName, got %v", err)
}

func TestAddonService_Create_DuplicateName_DifferentTenant_Allowed(t *testing.T) {
	t.Parallel()
	repo := newStubAddonRepo()
	svc := newAddonSvc(repo)

	_, err := svc.Create(context.Background(), service.CreateAddonInput{
		CallerUserID: userID1, CallerTenantID: tenantA, Name: "Handuk Panas", PriceIDR: 15000,
	})
	require.NoError(t, err)

	// Same name but different tenant — must succeed.
	_, err = svc.Create(context.Background(), service.CreateAddonInput{
		CallerUserID: userID1, CallerTenantID: tenantB, Name: "Handuk Panas", PriceIDR: 15000,
	})
	require.NoError(t, err)
}

func TestAddonService_Create_NameTooLong_Rejected(t *testing.T) {
	t.Parallel()
	repo := newStubAddonRepo()
	svc := newAddonSvc(repo)

	longName := make([]rune, 121)
	for i := range longName {
		longName[i] = 'A'
	}
	_, err := svc.Create(context.Background(), service.CreateAddonInput{
		CallerUserID: userID1, CallerTenantID: tenantA, Name: string(longName),
	})
	require.Error(t, err)
	assert.True(t, errors.Is(err, constants.ErrInvalidInput))
}

func TestAddonService_Create_NameExactly120_Accepted(t *testing.T) {
	t.Parallel()
	repo := newStubAddonRepo()
	svc := newAddonSvc(repo)

	name120 := make([]rune, 120)
	for i := range name120 {
		name120[i] = 'X'
	}
	_, err := svc.Create(context.Background(), service.CreateAddonInput{
		CallerUserID: userID1, CallerTenantID: tenantA, Name: string(name120),
	})
	require.NoError(t, err)
}

func TestAddonService_Create_NegativePrice_Rejected(t *testing.T) {
	t.Parallel()
	repo := newStubAddonRepo()
	svc := newAddonSvc(repo)

	_, err := svc.Create(context.Background(), service.CreateAddonInput{
		CallerUserID: userID1, CallerTenantID: tenantA, Name: "Jus Segar", PriceIDR: -1,
	})
	require.Error(t, err)
	assert.True(t, errors.Is(err, constants.ErrInvalidInput))
}

func TestAddonService_Create_ZeroPrice_Accepted(t *testing.T) {
	t.Parallel()
	repo := newStubAddonRepo()
	svc := newAddonSvc(repo)

	_, err := svc.Create(context.Background(), service.CreateAddonInput{
		CallerUserID: userID1, CallerTenantID: tenantA, Name: "Gratis", PriceIDR: 0,
	})
	require.NoError(t, err)
}

func TestAddonService_Create_DescriptionTooLong_Rejected(t *testing.T) {
	t.Parallel()
	repo := newStubAddonRepo()
	svc := newAddonSvc(repo)

	longDesc := make([]rune, 501)
	for i := range longDesc {
		longDesc[i] = 'Z'
	}
	desc := string(longDesc)
	_, err := svc.Create(context.Background(), service.CreateAddonInput{
		CallerUserID: userID1, CallerTenantID: tenantA, Name: "Test", Description: &desc,
	})
	require.Error(t, err)
	assert.True(t, errors.Is(err, constants.ErrInvalidInput))
}

// ---------------------------------------------------------------------------
// List
// ---------------------------------------------------------------------------

func TestAddonService_List_ReturnsOnlyCallerTenant(t *testing.T) {
	t.Parallel()
	repo := newStubAddonRepo()
	seedAddon(repo, "id-a1", tenantA, "Addon A1", 0)
	seedAddon(repo, "id-a2", tenantA, "Addon A2", 1)
	seedAddon(repo, "id-b1", tenantB, "Addon B1", 0)

	svc := newAddonSvc(repo)
	out, err := svc.List(context.Background(), service.ListAddonsInput{
		CallerTenantID: tenantA,
		Limit:          10,
	})

	require.NoError(t, err)
	assert.Len(t, out.Addons, 2)
	// Tenant isolation is enforced by the repository WHERE clause; we verify it
	// at the model layer via repo state rather than on AddonDetail (which
	// deliberately omits tenant_id).
	ids := make(map[string]struct{}, len(out.Addons))
	for _, a := range out.Addons {
		ids[a.ID] = struct{}{}
		assert.Equal(t, tenantA, repo.rows[a.ID].TenantID)
	}
	assert.Contains(t, ids, "id-a1")
	assert.Contains(t, ids, "id-a2")
}

func TestAddonService_List_SoftDeletedExcluded(t *testing.T) {
	t.Parallel()
	repo := newStubAddonRepo()
	a := seedAddon(repo, "id-del", tenantA, "Dihapus", 0)
	now := time.Now()
	a.DeletedAt = &now

	svc := newAddonSvc(repo)
	out, err := svc.List(context.Background(), service.ListAddonsInput{
		CallerTenantID: tenantA, Limit: 10,
	})

	require.NoError(t, err)
	assert.Empty(t, out.Addons)
}

func TestAddonService_List_IsActiveFilter(t *testing.T) {
	t.Parallel()
	repo := newStubAddonRepo()
	seedAddon(repo, "id-active", tenantA, "Aktif", 0)
	inactive := seedAddon(repo, "id-inactive", tenantA, "Non-aktif", 1)
	inactive.IsActive = false

	svc := newAddonSvc(repo)
	trueVal := true
	out, err := svc.List(context.Background(), service.ListAddonsInput{
		CallerTenantID: tenantA, IsActive: &trueVal, Limit: 10,
	})

	require.NoError(t, err)
	require.Len(t, out.Addons, 1)
	assert.Equal(t, "id-active", out.Addons[0].ID)
}

func TestAddonService_List_DefaultLimit10(t *testing.T) {
	t.Parallel()
	repo := newStubAddonRepo()
	for i := 0; i < 15; i++ {
		seedAddon(repo, "id-"+string(rune('a'+i)), tenantA, "Addon"+string(rune('A'+i)), i)
	}

	svc := newAddonSvc(repo)
	out, err := svc.List(context.Background(), service.ListAddonsInput{
		CallerTenantID: tenantA,
		Limit:          0, // triggers default
	})

	require.NoError(t, err)
	assert.Len(t, out.Addons, 10)
	assert.EqualValues(t, 15, out.TotalCount)
	assert.EqualValues(t, 2, out.TotalPages)
}

func TestAddonService_List_OffsetPagination(t *testing.T) {
	t.Parallel()
	repo := newStubAddonRepo()
	// Seed 5 addons with explicit sort_order and IDs that sort cleanly.
	for i := 0; i < 5; i++ {
		id := "addon-" + string(rune('0'+i))
		seedAddon(repo, id, tenantA, "Item "+string(rune('A'+i)), i)
	}

	svc := newAddonSvc(repo)

	// Page 1: limit 3.
	page1, err := svc.List(context.Background(), service.ListAddonsInput{
		CallerTenantID: tenantA, Limit: 3, Page: 1,
	})
	require.NoError(t, err)
	assert.Len(t, page1.Addons, 3)
	assert.EqualValues(t, 5, page1.TotalCount)
	assert.EqualValues(t, 2, page1.TotalPages)

	// Page 2: use page number.
	page2, err := svc.List(context.Background(), service.ListAddonsInput{
		CallerTenantID: tenantA, Limit: 3, Page: 2,
	})
	require.NoError(t, err)
	assert.Len(t, page2.Addons, 2)
	assert.EqualValues(t, 5, page2.TotalCount)

	// No overlap between pages.
	p1IDs := make(map[string]struct{})
	for _, a := range page1.Addons {
		p1IDs[a.ID] = struct{}{}
	}
	for _, a := range page2.Addons {
		assert.NotContains(t, p1IDs, a.ID, "page 2 addon %s should not appear in page 1", a.ID)
	}
}

// ---------------------------------------------------------------------------
// Get
// ---------------------------------------------------------------------------

func TestAddonService_Get_CrossTenantIDOR(t *testing.T) {
	t.Parallel()
	repo := newStubAddonRepo()
	seedAddon(repo, "id-b", tenantB, "Addon B", 0)

	svc := newAddonSvc(repo)
	_, err := svc.Get(context.Background(), tenantA, "id-b")

	require.Error(t, err)
	assert.True(t, errors.Is(err, constants.ErrAddonNotFound), "cross-tenant get must return ErrAddonNotFound, got %v", err)
}

func TestAddonService_Get_NotFound(t *testing.T) {
	t.Parallel()
	repo := newStubAddonRepo()
	svc := newAddonSvc(repo)

	_, err := svc.Get(context.Background(), tenantA, "nonexistent-id")
	require.Error(t, err)
	assert.True(t, errors.Is(err, constants.ErrAddonNotFound))
}

func TestAddonService_Get_Success(t *testing.T) {
	t.Parallel()
	repo := newStubAddonRepo()
	seedAddon(repo, "id-a", tenantA, "Aromaterapi", 0)

	svc := newAddonSvc(repo)
	out, err := svc.Get(context.Background(), tenantA, "id-a")

	require.NoError(t, err)
	assert.Equal(t, "id-a", out.ID)
	assert.Equal(t, "Aromaterapi", out.Name)
}

// ---------------------------------------------------------------------------
// Update
// ---------------------------------------------------------------------------

func TestAddonService_Update_Success(t *testing.T) {
	t.Parallel()
	repo := newStubAddonRepo()
	seedAddon(repo, "id-a", tenantA, "Lama", 0)

	svc := newAddonSvc(repo)
	newName := "Baru"
	out, err := svc.Update(context.Background(), service.UpdateAddonInput{
		AddonID:        "id-a",
		CallerUserID:   userID1,
		CallerTenantID: tenantA,
		Name:           &newName,
	})

	require.NoError(t, err)
	assert.Equal(t, "Baru", out.Name)
}

func TestAddonService_Update_CrossTenantIDOR(t *testing.T) {
	t.Parallel()
	repo := newStubAddonRepo()
	seedAddon(repo, "id-b", tenantB, "Addon B", 0)

	svc := newAddonSvc(repo)
	newName := "Hacked"
	_, err := svc.Update(context.Background(), service.UpdateAddonInput{
		AddonID:        "id-b",
		CallerUserID:   userID1,
		CallerTenantID: tenantA, // wrong tenant
		Name:           &newName,
	})

	require.Error(t, err)
	assert.True(t, errors.Is(err, constants.ErrAddonNotFound))
}

// ---------------------------------------------------------------------------
// ChangeStatus (toggle)
// ---------------------------------------------------------------------------

func TestAddonService_ChangeStatus_Toggle(t *testing.T) {
	t.Parallel()
	repo := newStubAddonRepo()
	seedAddon(repo, "id-a", tenantA, "Test", 0)

	svc := newAddonSvc(repo)

	// Deactivate.
	out, err := svc.ChangeStatus(context.Background(), service.ChangeAddonStatusInput{
		AddonID: "id-a", CallerUserID: userID1, CallerTenantID: tenantA, IsActive: false,
	})
	require.NoError(t, err)
	assert.False(t, out.IsActive)
	assert.False(t, repo.statusUpdates["id-a"])

	// Re-activate.
	out, err = svc.ChangeStatus(context.Background(), service.ChangeAddonStatusInput{
		AddonID: "id-a", CallerUserID: userID1, CallerTenantID: tenantA, IsActive: true,
	})
	require.NoError(t, err)
	assert.True(t, out.IsActive)
	assert.True(t, repo.statusUpdates["id-a"])
}

func TestAddonService_ChangeStatus_CrossTenantIDOR(t *testing.T) {
	t.Parallel()
	repo := newStubAddonRepo()
	seedAddon(repo, "id-b", tenantB, "Addon B", 0)

	svc := newAddonSvc(repo)
	_, err := svc.ChangeStatus(context.Background(), service.ChangeAddonStatusInput{
		AddonID: "id-b", CallerUserID: userID1, CallerTenantID: tenantA, IsActive: false,
	})

	require.Error(t, err)
	assert.True(t, errors.Is(err, constants.ErrAddonNotFound))
}

// ---------------------------------------------------------------------------
// SoftDelete
// ---------------------------------------------------------------------------

func TestAddonService_SoftDelete_Success(t *testing.T) {
	t.Parallel()
	repo := newStubAddonRepo()
	seedAddon(repo, "id-a", tenantA, "Addon A", 0)

	svc := newAddonSvc(repo)
	err := svc.SoftDelete(context.Background(), tenantA, userID1, "id-a")

	require.NoError(t, err)
	assert.Contains(t, repo.deleted, "id-a")
	// Should no longer be findable.
	_, err = svc.Get(context.Background(), tenantA, "id-a")
	assert.True(t, errors.Is(err, constants.ErrAddonNotFound))
}

func TestAddonService_SoftDelete_CrossTenantIDOR(t *testing.T) {
	t.Parallel()
	repo := newStubAddonRepo()
	seedAddon(repo, "id-b", tenantB, "Addon B", 0)

	svc := newAddonSvc(repo)
	err := svc.SoftDelete(context.Background(), tenantA, userID1, "id-b")

	require.Error(t, err)
	assert.True(t, errors.Is(err, constants.ErrAddonNotFound))
	// Original row must be untouched.
	assert.Empty(t, repo.deleted)
}

func TestAddonService_SoftDelete_ExcludedFromList(t *testing.T) {
	t.Parallel()
	repo := newStubAddonRepo()
	seedAddon(repo, "id-a", tenantA, "Addon A", 0)
	seedAddon(repo, "id-b", tenantA, "Addon B", 1)

	svc := newAddonSvc(repo)
	require.NoError(t, svc.SoftDelete(context.Background(), tenantA, userID1, "id-a"))

	out, err := svc.List(context.Background(), service.ListAddonsInput{CallerTenantID: tenantA, Limit: 10})
	require.NoError(t, err)
	assert.Len(t, out.Addons, 1)
	assert.Equal(t, "id-b", out.Addons[0].ID)
}

// ---------------------------------------------------------------------------
// Reorder
// ---------------------------------------------------------------------------

func TestAddonService_Reorder_Success(t *testing.T) {
	t.Parallel()
	repo := newStubAddonRepo()
	seedAddon(repo, "id-1", tenantA, "First", 0)
	seedAddon(repo, "id-2", tenantA, "Second", 1)
	seedAddon(repo, "id-3", tenantA, "Third", 2)

	svc := newAddonSvc(repo)
	err := svc.Reorder(context.Background(), service.ReorderAddonsInput{
		CallerUserID:   userID1,
		CallerTenantID: tenantA,
		Items: []service.AddonSortOrderItem{
			{ID: "id-1", SortOrder: 2},
			{ID: "id-2", SortOrder: 0},
			{ID: "id-3", SortOrder: 1},
		},
	})

	require.NoError(t, err)
	assert.Equal(t, 2, repo.sortOrders["id-1"])
	assert.Equal(t, 0, repo.sortOrders["id-2"])
	assert.Equal(t, 1, repo.sortOrders["id-3"])
}

func TestAddonService_Reorder_MixedForeignIDs_Rejected(t *testing.T) {
	t.Parallel()
	repo := newStubAddonRepo()
	seedAddon(repo, "id-a", tenantA, "Mine", 0)
	seedAddon(repo, "id-b", tenantB, "Not Mine", 0)

	svc := newAddonSvc(repo)
	err := svc.Reorder(context.Background(), service.ReorderAddonsInput{
		CallerUserID:   userID1,
		CallerTenantID: tenantA,
		Items: []service.AddonSortOrderItem{
			{ID: "id-a", SortOrder: 0},
			{ID: "id-b", SortOrder: 1}, // belongs to tenantB
		},
	})

	require.Error(t, err)
	assert.True(t, errors.Is(err, constants.ErrAddonNotFound), "expected ErrAddonNotFound for foreign ID, got %v", err)
}

func TestAddonService_Reorder_NonexistentID_Rejected(t *testing.T) {
	t.Parallel()
	repo := newStubAddonRepo()
	seedAddon(repo, "id-a", tenantA, "Mine", 0)

	svc := newAddonSvc(repo)
	err := svc.Reorder(context.Background(), service.ReorderAddonsInput{
		CallerUserID:   userID1,
		CallerTenantID: tenantA,
		Items: []service.AddonSortOrderItem{
			{ID: "id-a", SortOrder: 0},
			{ID: "ghost-id", SortOrder: 1},
		},
	})

	require.Error(t, err)
	assert.True(t, errors.Is(err, constants.ErrAddonNotFound))
}

func TestAddonService_Reorder_EmptyItems_Rejected(t *testing.T) {
	t.Parallel()
	repo := newStubAddonRepo()
	svc := newAddonSvc(repo)

	err := svc.Reorder(context.Background(), service.ReorderAddonsInput{
		CallerUserID:   userID1,
		CallerTenantID: tenantA,
		Items:          []service.AddonSortOrderItem{},
	})

	require.Error(t, err)
	assert.True(t, errors.Is(err, constants.ErrInvalidInput))
}

func TestAddonService_Reorder_Atomicity_BulkFailRollsBack(t *testing.T) {
	t.Parallel()
	repo := newStubAddonRepo()
	repo.bulkUpdateErr = errors.New("db failure")
	seedAddon(repo, "id-a", tenantA, "A", 0)

	svc := newAddonSvc(repo)
	err := svc.Reorder(context.Background(), service.ReorderAddonsInput{
		CallerUserID:   userID1,
		CallerTenantID: tenantA,
		Items:          []service.AddonSortOrderItem{{ID: "id-a", SortOrder: 5}},
	})

	require.Error(t, err)
	// sortOrders map should be empty — BulkUpdateSortOrder was never applied.
	assert.Empty(t, repo.sortOrders)
}

// ---------------------------------------------------------------------------
// Full lifecycle
// ---------------------------------------------------------------------------

func TestAddonService_Lifecycle_CreateUpdateStatusDelete(t *testing.T) {
	t.Parallel()
	repo := newStubAddonRepo()
	svc := newAddonSvc(repo)

	// Create.
	created, err := svc.Create(context.Background(), service.CreateAddonInput{
		CallerUserID:   userID1,
		CallerTenantID: tenantA,
		Name:           "Jus Segar",
		PriceIDR:       20000,
		SortOrder:      0,
	})
	require.NoError(t, err)
	id := created.ID

	// Update name and price.
	newName := "Jus Segar Premium"
	newPrice := int64(25000)
	updated, err := svc.Update(context.Background(), service.UpdateAddonInput{
		AddonID:        id,
		CallerUserID:   userID1,
		CallerTenantID: tenantA,
		Name:           &newName,
		PriceIDR:       &newPrice,
	})
	require.NoError(t, err)
	assert.Equal(t, "Jus Segar Premium", updated.Name)
	assert.Equal(t, int64(25000), updated.PriceIDR)

	// Deactivate.
	toggled, err := svc.ChangeStatus(context.Background(), service.ChangeAddonStatusInput{
		AddonID: id, CallerUserID: userID1, CallerTenantID: tenantA, IsActive: false,
	})
	require.NoError(t, err)
	assert.False(t, toggled.IsActive)

	// Soft-delete.
	require.NoError(t, svc.SoftDelete(context.Background(), tenantA, userID1, id))

	// Should not appear in list.
	list, err := svc.List(context.Background(), service.ListAddonsInput{CallerTenantID: tenantA, Limit: 10})
	require.NoError(t, err)
	for _, a := range list.Addons {
		assert.NotEqual(t, id, a.ID, "deleted addon must not appear in list")
	}
}

// ---------------------------------------------------------------------------
// Additional boundary and gap tests (added by qa-expert 2026-04-24)
// ---------------------------------------------------------------------------

// TestAddonService_Create_EmptyName_Rejected covers the name=0 boundary.
func TestAddonService_Create_EmptyName_Rejected(t *testing.T) {
	t.Parallel()
	repo := newStubAddonRepo()
	svc := newAddonSvc(repo)

	_, err := svc.Create(context.Background(), service.CreateAddonInput{
		CallerUserID: userID1, CallerTenantID: tenantA, Name: "",
	})
	require.Error(t, err)
	assert.True(t, errors.Is(err, constants.ErrInvalidInput), "empty name must return ErrInvalidInput, got %v", err)
}

// TestAddonService_Create_NameAt1Char_Accepted covers the name=1 boundary (min valid).
func TestAddonService_Create_NameAt1Char_Accepted(t *testing.T) {
	t.Parallel()
	repo := newStubAddonRepo()
	svc := newAddonSvc(repo)

	_, err := svc.Create(context.Background(), service.CreateAddonInput{
		CallerUserID: userID1, CallerTenantID: tenantA, Name: "X",
	})
	require.NoError(t, err)
}

// TestAddonService_Create_Name120MultibyteRunes_Accepted verifies the name
// limit is rune-based (utf8.RuneCountInString), not byte-based.
// 120 CJK runes encode to 360 bytes — must still be accepted.
func TestAddonService_Create_Name120MultibyteRunes_Accepted(t *testing.T) {
	t.Parallel()
	repo := newStubAddonRepo()
	svc := newAddonSvc(repo)

	name := make([]rune, 120)
	for i := range name {
		name[i] = '漢' // 3 bytes each — 120 runes = 360 bytes
	}
	_, err := svc.Create(context.Background(), service.CreateAddonInput{
		CallerUserID: userID1, CallerTenantID: tenantA, Name: string(name),
	})
	require.NoError(t, err, "120 multi-byte runes must be accepted (rune count, not byte count)")
}

// TestAddonService_Create_DescriptionExactly500_Accepted covers the
// description=500 boundary (max valid).
func TestAddonService_Create_DescriptionExactly500_Accepted(t *testing.T) {
	t.Parallel()
	repo := newStubAddonRepo()
	svc := newAddonSvc(repo)

	desc := string(make([]rune, 500))
	for i := range []rune(desc) {
		_ = i
	}
	// Build a 500-rune string explicitly.
	runes := make([]rune, 500)
	for i := range runes {
		runes[i] = 'Z'
	}
	d := string(runes)
	_, err := svc.Create(context.Background(), service.CreateAddonInput{
		CallerUserID: userID1, CallerTenantID: tenantA, Name: "Test", Description: &d,
	})
	require.NoError(t, err, "description with exactly 500 runes must be accepted")
}

// TestAddonService_Create_LargePositivePrice_Accepted verifies there is no
// artificial upper bound on price.
func TestAddonService_Create_LargePositivePrice_Accepted(t *testing.T) {
	t.Parallel()
	repo := newStubAddonRepo()
	svc := newAddonSvc(repo)

	_, err := svc.Create(context.Background(), service.CreateAddonInput{
		CallerUserID: userID1, CallerTenantID: tenantA, Name: "Premium", PriceIDR: 999_999_999,
	})
	require.NoError(t, err)
}

// TestAddonService_Create_SortOrderAt9999_Accepted covers the max valid boundary.
func TestAddonService_Create_SortOrderAt9999_Accepted(t *testing.T) {
	t.Parallel()
	repo := newStubAddonRepo()
	svc := newAddonSvc(repo)

	_, err := svc.Create(context.Background(), service.CreateAddonInput{
		CallerUserID: userID1, CallerTenantID: tenantA, Name: "Last", SortOrder: 9999,
	})
	require.NoError(t, err)
}

// TestAddonService_Create_SortOrderAt10000_Rejected covers the first invalid
// value above max.
func TestAddonService_Create_SortOrderAt10000_Rejected(t *testing.T) {
	t.Parallel()
	repo := newStubAddonRepo()
	svc := newAddonSvc(repo)

	_, err := svc.Create(context.Background(), service.CreateAddonInput{
		CallerUserID: userID1, CallerTenantID: tenantA, Name: "Over", SortOrder: 10000,
	})
	require.Error(t, err)
	assert.True(t, errors.Is(err, constants.ErrInvalidInput))
}

// TestAddonService_Create_NegativeSortOrder_Rejected covers negative sort_order.
func TestAddonService_Create_NegativeSortOrder_Rejected(t *testing.T) {
	t.Parallel()
	repo := newStubAddonRepo()
	svc := newAddonSvc(repo)

	_, err := svc.Create(context.Background(), service.CreateAddonInput{
		CallerUserID: userID1, CallerTenantID: tenantA, Name: "Negative", SortOrder: -1,
	})
	require.Error(t, err)
	assert.True(t, errors.Is(err, constants.ErrInvalidInput))
}

// TestAddonService_Update_DuplicateName_Rejected verifies that changing an
// addon's name to a name already held by a different active addon in the same
// tenant returns ErrDuplicateAddonName.
func TestAddonService_Update_DuplicateName_Rejected(t *testing.T) {
	t.Parallel()
	repo := newStubAddonRepo()
	seedAddon(repo, "id-1", tenantA, "Aromaterapi", 0)
	seedAddon(repo, "id-2", tenantA, "Handuk Panas", 1)

	svc := newAddonSvc(repo)
	conflict := "Aromaterapi" // already taken by id-1
	_, err := svc.Update(context.Background(), service.UpdateAddonInput{
		AddonID:        "id-2",
		CallerUserID:   userID1,
		CallerTenantID: tenantA,
		Name:           &conflict,
	})

	require.Error(t, err)
	assert.True(t, errors.Is(err, constants.ErrDuplicateAddonName), "update with duplicate name must return ErrDuplicateAddonName, got %v", err)
}

// TestAddonService_ChangeStatus_OnSoftDeletedRow_ReturnsNotFound verifies that
// toggling an add-on that has already been soft-deleted returns ErrAddonNotFound.
func TestAddonService_ChangeStatus_OnSoftDeletedRow_ReturnsNotFound(t *testing.T) {
	t.Parallel()
	repo := newStubAddonRepo()
	seedAddon(repo, "id-a", tenantA, "Addon A", 0)

	svc := newAddonSvc(repo)
	// Soft-delete first.
	require.NoError(t, svc.SoftDelete(context.Background(), tenantA, userID1, "id-a"))

	// Now attempt a status toggle — must fail.
	_, err := svc.ChangeStatus(context.Background(), service.ChangeAddonStatusInput{
		AddonID: "id-a", CallerUserID: userID1, CallerTenantID: tenantA, IsActive: true,
	})
	require.Error(t, err)
	assert.True(t, errors.Is(err, constants.ErrAddonNotFound), "status toggle on soft-deleted row must return ErrAddonNotFound, got %v", err)
}

// TestAddonService_Reorder_SortOrderOutOfBounds_Rejected verifies that a
// reorder item with sort_order = 10000 is rejected.
func TestAddonService_Reorder_SortOrderOutOfBounds_Rejected(t *testing.T) {
	t.Parallel()
	repo := newStubAddonRepo()
	seedAddon(repo, "id-a", tenantA, "Addon A", 0)

	svc := newAddonSvc(repo)
	err := svc.Reorder(context.Background(), service.ReorderAddonsInput{
		CallerUserID:   userID1,
		CallerTenantID: tenantA,
		Items:          []service.AddonSortOrderItem{{ID: "id-a", SortOrder: 10000}},
	})

	require.Error(t, err)
	assert.True(t, errors.Is(err, constants.ErrInvalidInput), "sort_order 10000 in reorder must return ErrInvalidInput, got %v", err)
}
