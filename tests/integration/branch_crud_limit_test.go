package integration_test

// T6 — branch CRUD + limit enforcement
//
// Sub-test A (Alice / existing tenant, starter = 1 branch, 1 already exists):
//   POST another branch → 409 BRANCH_LIMIT_REACHED.
//
// Sub-test B (fresh starter tenant, 0 branches):
//   1st branch → 201.
//   2nd branch → 409 BRANCH_LIMIT_REACHED.
//   PATCH updates the branch name.
//   Deactivate branch (PATCH /status inactive), then DELETE → 204.

import (
	"net/http"
	"os"
	"testing"

	"github.com/chrisanlung/lustia-integration-tests/harness"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	// Alice is the seeded tenant_admin for acme-spa (starter plan, 1 branch already exists).
	aliceEmail    = "alice@acme-spa.example"
	alicePassword = "Staff2026!"
)

func aliceToken(t *testing.T) string {
	t.Helper()
	c := harness.NewClient()
	login, resp, err := c.Login(aliceEmail, alicePassword)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode, "alice login must succeed")
	return login.AccessToken
}

// TestBranchLimitAlice verifies that Alice (starter, already has 1 branch)
// cannot create a second branch.
func TestBranchLimitAlice(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test — requires live service")
	}

	// Skip if Alice's seeded data has been modified (e.g. branch was deleted).
	// The test will still pass — BRANCH_LIMIT_REACHED is what we're after.
	token := aliceToken(t)
	c := harness.NewClient()

	secondBranch := harness.CreateBranchRequest{
		Name:     "Second Alice Branch",
		Code:     "ALICE2",
		City:     "Jakarta",
		Country:  "ID",
		Timezone: "Asia/Jakarta",
	}
	resp1, rawBody, err := c.RawDo("POST", "/api/v1/tenant/branches", secondBranch, token)
	require.NoError(t, err)
	assert.Equal(t, http.StatusConflict, resp1.StatusCode,
		"alice (starter, 1 branch) must not be able to create a second branch")
	assert.Equal(t, "BRANCH_LIMIT_REACHED", harness.ErrorCode(rawBody),
		"error code must be BRANCH_LIMIT_REACHED")
}

// TestBranchCRUDAndLimit tests the full branch lifecycle for a fresh starter tenant.
func TestBranchCRUDAndLimit(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test — requires live service")
	}

	smtpEnabled := os.Getenv("SMTP_ENABLED")
	_ = smtpEnabled // SMTP state noted; approval will succeed regardless

	// Get a fresh tenant (no branches yet).
	tenantToken, tempPassword, email := ApproveAndGetToken(t)

	// Change password so the gate is lifted.
	c := harness.NewClient()
	newPassword := "BranchTest2026!"
	cpResp, _, err := c.ChangePassword(tenantToken, tempPassword, newPassword)
	require.NoError(t, err)
	require.Equal(t, http.StatusNoContent, cpResp.StatusCode)

	login, loginResp, err := c.Login(email, newPassword)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, loginResp.StatusCode)
	token := login.AccessToken

	// --- Create first branch (must succeed) ---
	branch1, b1Resp, err := c.CreateBranch(token, harness.CreateBranchRequest{
		Name:     "Main Branch",
		Code:     "MAIN",
		City:     "Jakarta",
		Province: "DKI Jakarta",
		Country:  "ID",
		Timezone: "Asia/Jakarta",
	})
	require.NoError(t, err)
	assert.Equal(t, http.StatusCreated, b1Resp.StatusCode, "first branch must be created")
	assert.NotEmpty(t, branch1.ID)
	assert.Equal(t, "inactive", branch1.Status, "new branch starts as inactive")

	// --- Create second branch (must fail — starter = max 1) ---
	b2Resp, rawB2, err := c.RawDo("POST", "/api/v1/tenant/branches", harness.CreateBranchRequest{
		Name:     "Second Branch",
		Code:     "SECOND",
		City:     "Surabaya",
		Country:  "ID",
		Timezone: "Asia/Jakarta",
	}, token)
	require.NoError(t, err)
	assert.Equal(t, http.StatusConflict, b2Resp.StatusCode, "second branch must be rejected")
	assert.Equal(t, "BRANCH_LIMIT_REACHED", harness.ErrorCode(rawB2))

	// --- PATCH the branch (update name) ---
	updated, patchResp, err := c.UpdateBranch(token, branch1.ID, map[string]any{
		"name": "Updated Main Branch",
		"city": "Bandung",
	})
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, patchResp.StatusCode)
	assert.Equal(t, "Updated Main Branch", updated.Name)

	// --- Activate the branch ---
	activated, activateResp, err := c.ChangeBranchStatus(token, branch1.ID, "active")
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, activateResp.StatusCode)
	assert.Equal(t, "active", activated.Status)

	// --- Cannot delete an active branch ---
	deleteActiveResp, err := c.DeleteBranch(token, branch1.ID)
	require.NoError(t, err)
	assert.Equal(t, http.StatusConflict, deleteActiveResp.StatusCode,
		"deleting an active branch must return 409 INVALID_STATUS_TRANSITION")

	// --- Deactivate first ---
	deactivated, deactivateResp, err := c.ChangeBranchStatus(token, branch1.ID, "inactive")
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, deactivateResp.StatusCode)
	assert.Equal(t, "inactive", deactivated.Status)

	// --- Delete (soft-delete) ---
	deleteResp, err := c.DeleteBranch(token, branch1.ID)
	require.NoError(t, err)
	assert.Equal(t, http.StatusNoContent, deleteResp.StatusCode, "soft-delete must return 204")

	// --- Confirm the branch no longer appears in the list ---
	list, listResp, err := c.ListBranches(token)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, listResp.StatusCode)
	for _, b := range list.Data {
		assert.NotEqual(t, branch1.ID, b.ID,
			"soft-deleted branch must not appear in the default branch list")
	}
}
