package integration_test

// T11 — Therapist ↔ Service mapping (critical flow per ADR 0009 §2.7)
//
// Verifies the soft-reconcile semantics of PUT /therapists/:id/services:
//
//  1. Seed 2 services via Create.
//  2. PUT both → is_active=true for both.
//  3. PUT with only 1 → the removed one becomes is_active=false in the DB
//     (row NOT deleted — audit trail preserved).
//  4. PUT with original 2 again → the previously-deactivated one is
//     re-activated (no new row inserted; count stays the same).
//  5. GET /services/:id → therapists array contains ONLY active-mapping
//     therapists; the one currently inactive is absent.
//  6. GET /therapists/:id → services array contains ALL mappings with correct
//     is_active flags.

import (
	"database/sql"
	"fmt"
	"net/http"
	"testing"

	"github.com/chrisanlung/lustia-integration-tests/harness"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTherapistServiceMapping(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test — requires live service")
	}

	token := aliceTokenP4(t)
	c := harness.NewClient()

	// -------------------------------------------------------------------------
	// Setup: create a fresh therapist and two services for this test run.
	// -------------------------------------------------------------------------
	therapistName := fmt.Sprintf("Mapper T11 %s", harness.UniqueSlug("t11"))
	therapist, tResp, err := c.CreateTherapist(token, harness.CreateTherapistRequest{
		BranchID: seededBranchID,
		FullName: therapistName,
	})
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, tResp.StatusCode)
	therapistID := therapist.ID

	svc1, s1Resp, err := c.CreateService(token, harness.CreateServiceRequest{
		Name:            fmt.Sprintf("MapSvc1 %s", harness.UniqueSlug("ms1")),
		DurationMinutes: 60, PriceIDR: 100000,
	})
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, s1Resp.StatusCode)
	svc1ID := svc1.ID

	svc2, s2Resp, err := c.CreateService(token, harness.CreateServiceRequest{
		Name:            fmt.Sprintf("MapSvc2 %s", harness.UniqueSlug("ms2")),
		DurationMinutes: 90, PriceIDR: 200000,
	})
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, s2Resp.StatusCode)
	svc2ID := svc2.ID

	// Cleanup: soft-delete therapist and services at end of test.
	t.Cleanup(func() {
		_, _ = c.DeleteTherapist(token, therapistID)
		_, _ = c.DeleteService(token, svc1ID)
		_, _ = c.DeleteService(token, svc2ID)
	})

	// -------------------------------------------------------------------------
	// Step 1: PUT both services → response shows is_active=true for both.
	// -------------------------------------------------------------------------
	mapping, putResp, err := c.PutTherapistServices(token, therapistID, []string{svc1ID, svc2ID})
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, putResp.StatusCode, "PUT /services must return 200")
	assert.Equal(t, therapistID, mapping.TherapistID)
	assert.Len(t, mapping.Services, 2, "both services must be in the mapping response")

	for _, m := range mapping.Services {
		assert.True(t, m.IsActive,
			"service %s must be active after first PUT", m.ServiceID)
	}

	// -------------------------------------------------------------------------
	// Step 2: PUT with only svc1 → svc2 becomes is_active=false.
	// -------------------------------------------------------------------------
	mapping2, put2Resp, err := c.PutTherapistServices(token, therapistID, []string{svc1ID})
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, put2Resp.StatusCode)

	// Response must show both mappings (active and inactive).
	assert.Len(t, mapping2.Services, 2,
		"response after partial PUT must still include all mapping rows (active + inactive)")

	for _, m := range mapping2.Services {
		if m.ServiceID == svc1ID {
			assert.True(t, m.IsActive, "svc1 must remain active")
		} else if m.ServiceID == svc2ID {
			assert.False(t, m.IsActive, "svc2 must be deactivated (soft, not deleted)")
		}
	}

	// DB assertion: the row must still exist with is_active=false (not deleted).
	// Run inside a tenant-scoped transaction so the RLS policy allows the read.
	var svc2IsActive bool
	require.NoError(t, harness.TxWithTenant(acmeSpaTenanID, func(tx *sql.Tx) error {
		return tx.QueryRow(
			`SELECT is_active FROM therapist_service WHERE therapist_id = $1 AND service_id = $2`,
			therapistID, svc2ID,
		).Scan(&svc2IsActive)
	}), "therapist_service row for svc2 must still exist after partial PUT")
	assert.False(t, svc2IsActive, "DB row for svc2 must have is_active=false")

	// -------------------------------------------------------------------------
	// Step 3: GET /therapists/:id/services — returns both rows with correct flags.
	// -------------------------------------------------------------------------
	getMapping, getMappingResp, err := c.GetTherapistServices(token, therapistID)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, getMappingResp.StatusCode)
	assert.Len(t, getMapping.Services, 2,
		"GET /therapists/:id/services must return all mapping rows (active + inactive)")

	// -------------------------------------------------------------------------
	// Step 4: PUT with original 2 again → svc2 re-activates; no new DB row.
	// -------------------------------------------------------------------------
	// Capture row count before re-activation to confirm no INSERT happens.
	var rowCountBefore int
	require.NoError(t, harness.TxWithTenant(acmeSpaTenanID, func(tx *sql.Tx) error {
		return tx.QueryRow(
			`SELECT COUNT(*) FROM therapist_service WHERE therapist_id = $1`,
			therapistID,
		).Scan(&rowCountBefore)
	}), "row count query before re-activation must succeed")

	mapping3, put3Resp, err := c.PutTherapistServices(token, therapistID, []string{svc1ID, svc2ID})
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, put3Resp.StatusCode)

	for _, m := range mapping3.Services {
		assert.True(t, m.IsActive,
			"both services must be active after re-activating PUT, got is_active=%v for %s",
			m.IsActive, m.ServiceID)
	}

	var rowCountAfter int
	require.NoError(t, harness.TxWithTenant(acmeSpaTenanID, func(tx *sql.Tx) error {
		return tx.QueryRow(
			`SELECT COUNT(*) FROM therapist_service WHERE therapist_id = $1`,
			therapistID,
		).Scan(&rowCountAfter)
	}), "row count query after re-activation must succeed")
	assert.Equal(t, rowCountBefore, rowCountAfter,
		"re-activating a mapping must UPDATE the existing row, not INSERT a new one")

	// -------------------------------------------------------------------------
	// Step 5: Deactivate svc2 again, then verify GET /services/:id (the svc2
	// detail endpoint) returns therapists array with only active-mapping entries.
	// -------------------------------------------------------------------------
	_, _, err = c.PutTherapistServices(token, therapistID, []string{svc1ID}) // svc2 inactive again
	require.NoError(t, err)

	// GET /services/:id for svc2 — therapists array must NOT include our therapist
	// because the mapping is currently is_active=false.
	svc2Detail, svc2DetailResp, err := c.GetService(token, svc2ID)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, svc2DetailResp.StatusCode)

	for _, th := range svc2Detail.Therapists {
		assert.NotEqual(t, therapistID, th.TherapistID,
			"GET /services/:id must not list therapists with inactive mappings (flag #6)")
	}

	// GET /services/:id for svc1 — therapists array MUST include our therapist.
	svc1Detail, _, err := c.GetService(token, svc1ID)
	require.NoError(t, err)
	foundInSvc1 := false
	for _, th := range svc1Detail.Therapists {
		if th.TherapistID == therapistID {
			foundInSvc1 = true
			assert.True(t, th.IsActive)
			break
		}
	}
	assert.True(t, foundInSvc1,
		"therapist with active mapping must appear in GET /services/:id therapists array")

	// -------------------------------------------------------------------------
	// Step 6: GET /therapists/:id — services array includes ALL mappings
	// (active + inactive) with their respective flags.
	// -------------------------------------------------------------------------
	therapistDetail, therapistDetailResp, err := c.GetTherapist(token, therapistID)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, therapistDetailResp.StatusCode)
	assert.Len(t, therapistDetail.Services, 2,
		"GET /therapists/:id must include all service mappings regardless of is_active")

	for _, s := range therapistDetail.Services {
		if s.ServiceID == svc1ID {
			assert.True(t, s.IsActive, "svc1 mapping must be active")
		} else if s.ServiceID == svc2ID {
			assert.False(t, s.IsActive, "svc2 mapping must be inactive")
		}
	}

	// -------------------------------------------------------------------------
	// Step 7: PUT with empty list → all mappings deactivated.
	// -------------------------------------------------------------------------
	emptyMapping, emptyPutResp, err := c.PutTherapistServices(token, therapistID, []string{})
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, emptyPutResp.StatusCode, "PUT with empty list must return 200")

	// All returned mappings must be inactive.
	for _, m := range emptyMapping.Services {
		assert.False(t, m.IsActive,
			"after PUT([]), service %s must be inactive", m.ServiceID)
	}

	// DB sanity: no rows were hard-deleted.
	var rowCountFinal int
	require.NoError(t, harness.TxWithTenant(acmeSpaTenanID, func(tx *sql.Tx) error {
		return tx.QueryRow(
			`SELECT COUNT(*) FROM therapist_service WHERE therapist_id = $1`,
			therapistID,
		).Scan(&rowCountFinal)
	}), "final row count query must succeed")
	assert.Equal(t, rowCountBefore, rowCountFinal,
		"PUT([]) must soft-deactivate, not hard-delete, mapping rows")
}
