package integration_test

// T14 — Seed data integrity (migration 14 sanity check)
//
// Queries the DB directly (bypassing the API) inside a tenant-scoped
// transaction to verify that migration 14 inserted the expected counts
// for the acme-spa tenant:
//
//   services              : 3 (Refleksi Kaki, Aromaterapi, Pijat Tradisional)
//   therapists at branch  : 2 (Budi Santoso, Siti Rahayu)
//   therapist_service rows: 5 (Budi→2, Siti→3 — see migration 14 comment)
//   availability windows  : 6 (Budi→3, Siti→3)
//
// Then asserts the same records are queryable via the API as Alice, confirming
// the RLS policy + service layer correctly project the seeded data.
//
// Fixed UUIDs come from migration 14:
//   acme-spa tenant  : d0000000-0000-0000-0001-000000000001
//   Cabang Utama     : f0000000-0000-0000-0001-000000000001
//   Budi Santoso     : f0000000-0000-0000-0003-000000000001
//   Siti Rahayu      : f0000000-0000-0000-0003-000000000002
//   svc Refleksi     : f0000000-0000-0000-0002-000000000001
//   svc Aromaterapi  : f0000000-0000-0000-0002-000000000002
//   svc Pijat Trad.  : f0000000-0000-0000-0002-000000000003

import (
	"database/sql"
	"net/http"
	"testing"

	"github.com/chrisanlung/lustia-integration-tests/harness"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	acmeSpaTenanID      = "d0000000-0000-0000-0001-000000000001"
	seededBudiID        = "f0000000-0000-0000-0003-000000000001"
	seededSitiID        = "f0000000-0000-0000-0003-000000000002"
	seededSvcRefleksiID = "f0000000-0000-0000-0002-000000000001"
	seededSvcAromID     = "f0000000-0000-0000-0002-000000000002"
	seededSvcPijatID    = "f0000000-0000-0000-0002-000000000003"
)

func TestSeedDataIntegrity(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test — requires live service and migration 14 applied")
	}

	// -------------------------------------------------------------------------
	// DB assertions — all queries run inside a tenant-scoped transaction so
	// the RLS policy on lustia_app lets us see acme-spa's rows.
	// -------------------------------------------------------------------------
	var (
		serviceCount   int
		therapistCount int
		mappingCount   int
		availCount     int
		budiName       string
		sitiName       string
		budiMappings   int
		sitiMappings   int
	)

	require.NoError(t, harness.TxWithTenant(acmeSpaTenanID, func(tx *sql.Tx) error {
		if err := tx.QueryRow(
			`SELECT COUNT(*) FROM service WHERE tenant_id = $1 AND deleted_at IS NULL`,
			acmeSpaTenanID,
		).Scan(&serviceCount); err != nil {
			return err
		}

		if err := tx.QueryRow(
			`SELECT COUNT(*) FROM therapist WHERE tenant_id = $1 AND branch_id = $2 AND deleted_at IS NULL`,
			acmeSpaTenanID, seededBranchID,
		).Scan(&therapistCount); err != nil {
			return err
		}

		if err := tx.QueryRow(
			`SELECT COUNT(*) FROM therapist_service ts
			 JOIN therapist t ON t.id = ts.therapist_id
			 WHERE t.tenant_id = $1`,
			acmeSpaTenanID,
		).Scan(&mappingCount); err != nil {
			return err
		}

		if err := tx.QueryRow(
			`SELECT COUNT(*) FROM therapist_availability WHERE tenant_id = $1`,
			acmeSpaTenanID,
		).Scan(&availCount); err != nil {
			return err
		}

		if err := tx.QueryRow(
			`SELECT full_name FROM therapist WHERE id = $1`, seededBudiID,
		).Scan(&budiName); err != nil {
			return err
		}

		if err := tx.QueryRow(
			`SELECT full_name FROM therapist WHERE id = $1`, seededSitiID,
		).Scan(&sitiName); err != nil {
			return err
		}

		if err := tx.QueryRow(
			`SELECT COUNT(*) FROM therapist_service WHERE therapist_id = $1 AND is_active = true`,
			seededBudiID,
		).Scan(&budiMappings); err != nil {
			return err
		}

		if err := tx.QueryRow(
			`SELECT COUNT(*) FROM therapist_service WHERE therapist_id = $1 AND is_active = true`,
			seededSitiID,
		).Scan(&sitiMappings); err != nil {
			return err
		}

		return nil
	}), "DB queries within tenant transaction must succeed")

	assert.GreaterOrEqual(t, serviceCount, 3,
		"acme-spa must have at least 3 services (seeded by migration 14)")
	assert.GreaterOrEqual(t, therapistCount, 2,
		"acme-spa Cabang Utama must have at least 2 therapists (seeded by migration 14)")
	assert.GreaterOrEqual(t, mappingCount, 5,
		"acme-spa must have at least 5 therapist_service rows (Budi×2 + Siti×3)")
	assert.GreaterOrEqual(t, availCount, 6,
		"acme-spa must have at least 6 availability windows (Budi×3 + Siti×3)")
	assert.Equal(t, "Budi Santoso", budiName)
	assert.Equal(t, "Siti Rahayu", sitiName)
	assert.GreaterOrEqual(t, budiMappings, 2, "Budi must have at least 2 active service mappings")
	assert.GreaterOrEqual(t, sitiMappings, 3, "Siti must have at least 3 active service mappings")

	// -------------------------------------------------------------------------
	// API assertions — same records must be visible via the API as Alice.
	// -------------------------------------------------------------------------
	token := aliceTokenP4(t)
	c := harness.NewClient()

	// GET Budi via API.
	budi, budiResp, err := c.GetTherapist(token, seededBudiID)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, budiResp.StatusCode,
		"GET /tenant/therapists/Budi must return 200 for Alice")
	assert.Equal(t, "Budi Santoso", budi.FullName)
	assert.Equal(t, acmeSpaTenanID, budi.TenantID)
	assert.Equal(t, seededBranchID, budi.BranchID)
	assert.True(t, budi.IsActive)

	// GET Siti via API.
	siti, sitiResp, err := c.GetTherapist(token, seededSitiID)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, sitiResp.StatusCode,
		"GET /tenant/therapists/Siti must return 200 for Alice")
	assert.Equal(t, "Siti Rahayu", siti.FullName)

	// List therapists — must contain both seeded therapists.
	list, listResp, err := c.ListTherapists(token, "")
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, listResp.StatusCode)

	foundBudi, foundSiti := false, false
	for _, th := range list.Data {
		if th.ID == seededBudiID {
			foundBudi = true
		}
		if th.ID == seededSitiID {
			foundSiti = true
		}
	}
	assert.True(t, foundBudi, "Budi Santoso must appear in the therapist list for Alice")
	assert.True(t, foundSiti, "Siti Rahayu must appear in the therapist list for Alice")

	// GET Refleksi Kaki service via API.
	refleksi, refleksiResp, err := c.GetService(token, seededSvcRefleksiID)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, refleksiResp.StatusCode)
	assert.Equal(t, "Refleksi Kaki 60 Menit", refleksi.Name)
	assert.NotNil(t, refleksi.Category)
	assert.Equal(t, "Refleksi", *refleksi.Category)
	assert.Equal(t, 60, refleksi.DurationMinutes)
	// Both Budi and Siti have active mappings to Refleksi → therapists array
	// must have at least 2 entries.
	assert.GreaterOrEqual(t, len(refleksi.Therapists), 2,
		"Refleksi Kaki must have at least 2 active therapists (Budi + Siti)")

	// Budi's availability via API — expect at least 3 windows.
	budiAvail, budiAvailResp, err := c.GetAvailability(token, seededBudiID)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, budiAvailResp.StatusCode)
	assert.GreaterOrEqual(t, len(budiAvail.Windows), 3,
		"Budi must have at least 3 availability windows (Mon/Wed/Fri)")

	// Siti's availability via API — expect at least 3 windows.
	sitiAvail, sitiAvailResp, err := c.GetAvailability(token, seededSitiID)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, sitiAvailResp.StatusCode)
	assert.GreaterOrEqual(t, len(sitiAvail.Windows), 3,
		"Siti must have at least 3 availability windows (Tue/Thu/Sat)")

	// List services — all 3 seeded services must appear.
	svcList, svcListResp, err := c.ListServices(token, "")
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, svcListResp.StatusCode)

	foundRefleksi, foundArom, foundPijat := false, false, false
	for _, s := range svcList.Data {
		switch s.ID {
		case seededSvcRefleksiID:
			foundRefleksi = true
		case seededSvcAromID:
			foundArom = true
		case seededSvcPijatID:
			foundPijat = true
		}
	}
	assert.True(t, foundRefleksi, "Refleksi Kaki must appear in the service list")
	assert.True(t, foundArom, "Aromaterapi must appear in the service list")
	assert.True(t, foundPijat, "Pijat Tradisional must appear in the service list")
}
