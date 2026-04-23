package integration_test

// T9 — Therapist CRUD
//
// Login as Alice (tenant_admin, acme-spa). Exercises the full therapist
// lifecycle through the Phase 4 endpoints:
//
//   POST   /api/v1/tenant/therapists         → 201
//   GET    /api/v1/tenant/therapists/:id      → 200, shape verified
//   PATCH  /api/v1/tenant/therapists/:id      → 200, profile updated
//   PATCH  /api/v1/tenant/therapists/:id/status → deactivate then reactivate
//   DELETE /api/v1/tenant/therapists/:id      → 204
//   GET    (after delete)                     → 404 THERAPIST_NOT_FOUND
//   POST   with branch_id from another tenant → 403 or 404

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/chrisanlung/lustia-integration-tests/harness"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// acmeSpaSlug is the tenant slug for Alice's tenant.
const acmeSpaSlug = "acme-spa"

// seededBranchID is the fixed UUID of "Cabang Utama" created by migration 14.
const seededBranchID = "f0000000-0000-0000-0001-000000000001"

// aliceTokenP4 returns a fresh access token for Alice, handling the login
// with explicit tenant_slug. It reuses the aliceEmail / alicePassword
// constants declared in branch_crud_limit_test.go.
func aliceTokenP4(t *testing.T) string {
	t.Helper()
	c := harness.NewClient()
	login, resp, err := c.LoginWithSlug(aliceEmail, alicePassword, acmeSpaSlug)
	if err != nil || resp.StatusCode != http.StatusOK {
		// Fall back to the slug-less login in case the service accepts it
		// (existing Phase 3 tests do not pass a slug).
		login2, resp2, err2 := c.Login(aliceEmail, alicePassword)
		require.NoError(t, err2)
		require.Equal(t, http.StatusOK, resp2.StatusCode, "alice login must succeed")
		return login2.AccessToken
	}
	return login.AccessToken
}

func TestTherapistCRUD(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test — requires live service")
	}

	token := aliceTokenP4(t)
	c := harness.NewClient()

	name := fmt.Sprintf("Therapist T9 %s", harness.UniqueSlug("t9"))
	email := harness.UniqueEmail("t9-therapist")

	// -------------------------------------------------------------------------
	// Step 1: Create therapist → 201
	// -------------------------------------------------------------------------
	created, createResp, err := c.CreateTherapist(token, harness.CreateTherapistRequest{
		BranchID: seededBranchID,
		FullName: name,
		Gender:   "female",
		Email:    email,
		Bio:      "Integration test therapist",
		JoinedAt: "2026-01-01",
	})
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, createResp.StatusCode,
		"POST /tenant/therapists must return 201")

	assert.NotEmpty(t, created.ID)
	assert.Equal(t, name, created.FullName)
	assert.Equal(t, seededBranchID, created.BranchID)
	assert.True(t, created.IsActive, "new therapist must be active by default")
	require.NotEmpty(t, created.ID, "therapist ID must be present for subsequent calls")

	therapistID := created.ID

	// -------------------------------------------------------------------------
	// Step 2: GET → verify shape
	// -------------------------------------------------------------------------
	got, getResp, err := c.GetTherapist(token, therapistID)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, getResp.StatusCode, "GET /tenant/therapists/:id must return 200")
	assert.Equal(t, therapistID, got.ID)
	assert.Equal(t, name, got.FullName)
	assert.NotNil(t, got.Specialties, "specialties must be an array (empty or populated), not nil")

	// -------------------------------------------------------------------------
	// Step 3: PATCH profile → 200
	// -------------------------------------------------------------------------
	updatedName := name + " Updated"
	updatedBio := "Updated bio via integration test"
	patched, patchResp, err := c.UpdateTherapist(token, therapistID, harness.UpdateTherapistRequest{
		FullName: &updatedName,
		Bio:      &updatedBio,
	})
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, patchResp.StatusCode, "PATCH therapist profile must return 200")
	assert.Equal(t, updatedName, patched.FullName)

	// -------------------------------------------------------------------------
	// Step 4: PATCH status deactivate → 200, is_active=false
	// -------------------------------------------------------------------------
	deactivated, deactResp, err := c.ChangeTherapistStatus(token, therapistID, false)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, deactResp.StatusCode, "deactivate must return 200")
	assert.False(t, deactivated.IsActive, "therapist must be inactive after deactivation")

	// -------------------------------------------------------------------------
	// Step 5: PATCH status activate → 200, is_active=true
	// -------------------------------------------------------------------------
	reactivated, reactResp, err := c.ChangeTherapistStatus(token, therapistID, true)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, reactResp.StatusCode, "reactivate must return 200")
	assert.True(t, reactivated.IsActive, "therapist must be active after reactivation")

	// -------------------------------------------------------------------------
	// Step 6: DELETE → 204; subsequent GET → 404 THERAPIST_NOT_FOUND
	// -------------------------------------------------------------------------
	delResp, err := c.DeleteTherapist(token, therapistID)
	require.NoError(t, err)
	assert.Equal(t, http.StatusNoContent, delResp.StatusCode, "DELETE must return 204")

	_, getAfterDel, rawAfterDel, err2 := getTherapistRaw(c, token, therapistID)
	require.NoError(t, err2)
	assert.Equal(t, http.StatusNotFound, getAfterDel.StatusCode,
		"GET after soft-delete must return 404")
	assert.Equal(t, "THERAPIST_NOT_FOUND", harness.ErrorCode(rawAfterDel),
		"error code must be THERAPIST_NOT_FOUND")

	// -------------------------------------------------------------------------
	// Step 7: Create with foreign branch_id (a random UUID not belonging to
	// Alice's tenant) → expect 403 CROSS_BRANCH_FORBIDDEN or 404 NOT_FOUND.
	//
	// The contract allows either:
	//  - 404 NOT_FOUND when the branch UUID is simply unknown in the DB.
	//  - 403 CROSS_BRANCH_FORBIDDEN when the service detects the branch belongs
	//    to another tenant.
	// Both are valid. We lock in whichever the service currently returns.
	// -------------------------------------------------------------------------
	fakeBranchID := "00000000-0000-0000-0000-000000000099"
	_, foreignResp, _, err3 := createTherapistRaw(c, token, harness.CreateTherapistRequest{
		BranchID: fakeBranchID,
		FullName: "Should Not Exist",
	})
	require.NoError(t, err3)
	assert.True(t,
		foreignResp.StatusCode == http.StatusForbidden ||
			foreignResp.StatusCode == http.StatusNotFound,
		"creating a therapist with an unknown/foreign branch_id must return 403 or 404, got %d",
		foreignResp.StatusCode)
}

// getTherapistRaw returns the raw response bytes alongside the typed result,
// needed for error-code assertions when the call is expected to fail.
func getTherapistRaw(c *harness.APIClient, token, id string) (harness.TherapistResponse, *http.Response, []byte, error) {
	resp, raw, err := c.RawDo("GET", "/api/v1/tenant/therapists/"+id, nil, token)
	if err != nil {
		return harness.TherapistResponse{}, nil, nil, err
	}
	var out harness.TherapistResponse
	_ = harness.DecodeJSONPublic(raw, &out)
	return out, resp, raw, nil
}

// createTherapistRaw returns raw response bytes for cases where we expect errors.
func createTherapistRaw(c *harness.APIClient, token string, body harness.CreateTherapistRequest) (harness.TherapistResponse, *http.Response, []byte, error) {
	resp, raw, err := c.RawDo("POST", "/api/v1/tenant/therapists", body, token)
	if err != nil {
		return harness.TherapistResponse{}, nil, nil, err
	}
	var out harness.TherapistResponse
	_ = harness.DecodeJSONPublic(raw, &out)
	return out, resp, raw, nil
}
