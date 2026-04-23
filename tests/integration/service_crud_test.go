package integration_test

// T10 — Service CRUD
//
// Login as Alice (tenant_admin, acme-spa). Exercises the full service
// lifecycle through the Phase 4 endpoints:
//
//   POST   /api/v1/tenant/services              → 201
//   GET    /api/v1/tenant/services?category=X   → seeded + new service visible
//   PATCH  /api/v1/tenant/services/:id/status   → deactivate
//   GET    /api/v1/tenant/services?is_active=false → visible under filter
//   PATCH  /api/v1/tenant/services/:id          → 200, fields updated
//   DELETE /api/v1/tenant/services/:id          → 204

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/chrisanlung/lustia-integration-tests/harness"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestServiceCRUD(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test — requires live service")
	}

	token := aliceTokenP4(t)
	c := harness.NewClient()

	// Unique category so the filter test is deterministic even if other tests
	// have seeded services in the same category.
	category := fmt.Sprintf("TestCat-%s", harness.UniqueSlug("svc"))
	svcName := fmt.Sprintf("Service T10 %s", harness.UniqueSlug("svc"))

	// -------------------------------------------------------------------------
	// Step 1: Create a service → 201
	// -------------------------------------------------------------------------
	svc, createResp, err := c.CreateService(token, harness.CreateServiceRequest{
		Name:            svcName,
		Description:     "Integration test service",
		Category:        category,
		DurationMinutes: 60,
		PriceIDR:        150000,
	})
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, createResp.StatusCode,
		"POST /tenant/services must return 201")

	assert.NotEmpty(t, svc.ID)
	assert.Equal(t, svcName, svc.Name)
	assert.True(t, svc.IsActive)
	assert.Equal(t, "IDR", svc.Currency, "currency must always be IDR in Phase 4")
	require.NotEmpty(t, svc.ID)

	serviceID := svc.ID

	// -------------------------------------------------------------------------
	// Step 2: List with ?category= filter → the new service is visible
	// -------------------------------------------------------------------------
	listed, listResp, err := c.ListServices(token, "category="+category)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, listResp.StatusCode)

	foundInList := false
	for _, s := range listed.Data {
		if s.ID == serviceID {
			foundInList = true
			break
		}
	}
	assert.True(t, foundInList,
		"newly created service must appear in ?category=%s list", category)

	// -------------------------------------------------------------------------
	// Step 3: List without filter — seeded services visible too.
	// Seeded categories: "Refleksi", "Aromaterapi", "Pijat" (migration 14).
	// -------------------------------------------------------------------------
	allActive, allResp, err := c.ListServices(token, "")
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, allResp.StatusCode)
	assert.GreaterOrEqual(t, len(allActive.Data), 1,
		"at least the newly created service must appear in the active list")

	// -------------------------------------------------------------------------
	// Step 4: Deactivate → is_active=false
	// -------------------------------------------------------------------------
	deactivated, deactResp, err := c.ChangeServiceStatus(token, serviceID, false)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, deactResp.StatusCode, "service deactivate must return 200")
	assert.False(t, deactivated.IsActive)

	// -------------------------------------------------------------------------
	// Step 5: List with ?is_active=false → deactivated service is visible
	// -------------------------------------------------------------------------
	inactive, inactiveResp, err := c.ListServices(token, "is_active=false")
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, inactiveResp.StatusCode)

	foundInactive := false
	for _, s := range inactive.Data {
		if s.ID == serviceID {
			foundInactive = true
			assert.False(t, s.IsActive, "listed deactivated service must have is_active=false")
			break
		}
	}
	assert.True(t, foundInactive,
		"deactivated service must appear in ?is_active=false list")

	// Confirm it does NOT appear in the default (active-only) list.
	activeOnly, _, err := c.ListServices(token, "")
	require.NoError(t, err)
	for _, s := range activeOnly.Data {
		assert.NotEqual(t, serviceID, s.ID,
			"deactivated service must not appear in active-only list")
	}

	// -------------------------------------------------------------------------
	// Step 6: PATCH fields → 200
	// -------------------------------------------------------------------------
	// Reactivate first so the service is visible on normal reads.
	_, reactResp, err := c.ChangeServiceStatus(token, serviceID, true)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, reactResp.StatusCode)

	newName := svcName + " Updated"
	newDuration := 90
	updated, patchResp, err := c.UpdateService(token, serviceID, harness.UpdateServiceRequest{
		Name:            &newName,
		DurationMinutes: &newDuration,
	})
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, patchResp.StatusCode, "PATCH service must return 200")
	assert.Equal(t, newName, updated.Name)
	assert.Equal(t, 90, updated.DurationMinutes)

	// -------------------------------------------------------------------------
	// Step 7: DELETE → 204
	// -------------------------------------------------------------------------
	delResp, err := c.DeleteService(token, serviceID)
	require.NoError(t, err)
	assert.Equal(t, http.StatusNoContent, delResp.StatusCode, "DELETE must return 204")

	// Verify it no longer appears in any list regardless of is_active filter.
	afterDel, _, err := c.ListServices(token, "is_active=false")
	require.NoError(t, err)
	for _, s := range afterDel.Data {
		assert.NotEqual(t, serviceID, s.ID,
			"soft-deleted service must not appear even in ?is_active=false list")
	}

	// GET detail must return 404 SERVICE_NOT_FOUND.
	_, getAfterDel, rawAfterDel, err2 := getServiceRaw(c, token, serviceID)
	require.NoError(t, err2)
	assert.Equal(t, http.StatusNotFound, getAfterDel.StatusCode)
	assert.Equal(t, "SERVICE_NOT_FOUND", harness.ErrorCode(rawAfterDel))
}

// getServiceRaw returns the raw response bytes for GET /services/:id.
func getServiceRaw(c *harness.APIClient, token, id string) (harness.ServiceResponse, *http.Response, []byte, error) {
	resp, raw, err := c.RawDo("GET", "/api/v1/tenant/services/"+id, nil, token)
	if err != nil {
		return harness.ServiceResponse{}, nil, nil, err
	}
	var out harness.ServiceResponse
	_ = harness.DecodeJSONPublic(raw, &out)
	return out, resp, raw, nil
}
