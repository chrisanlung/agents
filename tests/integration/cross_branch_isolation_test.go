package integration_test

// T13 — Cross-branch isolation (critical flow per ADR 0009 §2.7)
//
// This test exercises three isolation boundaries:
//
//  Boundary A — Cross-tenant isolation:
//    Create a second tenant; create a therapist there; verify Alice (different
//    tenant) cannot mutate that therapist → 404 THERAPIST_NOT_FOUND.
//
//  Boundary B — Cross-branch isolation for branch_admin:
//    Create a third tenant with max_branches=2; create two branches there.
//    Create a branch_admin user at branch A and a therapist at branch B.
//    Login as branch_admin of branch A; attempt to UPDATE/PUT-availability/
//    PUT-services on the branch-B therapist → 403 CROSS_BRANCH_FORBIDDEN.
//
//  Observed behaviour — branch_admin LIST scope:
//    The contract says a branch_admin "always sees only their assigned branches"
//    for the therapist list (API_CONTRACT.md §11.4.2). We assert whatever the
//    service currently returns and lock it in as the regression baseline.
//
// Note: this test depends on super admin credentials (SUPER_ADMIN_EMAIL /
// SUPER_ADMIN_PASSWORD env vars). It is skipped when those are absent.

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/chrisanlung/lustia-integration-tests/harness"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCrossBranchIsolation(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test — requires live service")
	}

	adminToken := superAdminToken(t)
	aliceToken := aliceTokenP4(t)
	c := harness.NewClient()
	c.ForwardedFor = harness.UniqueIP()

	// =========================================================================
	// Boundary A — Cross-tenant isolation
	// =========================================================================
	// Create a second tenant, approve it, log in as its admin, create a branch
	// and a therapist. Then verify Alice (acme-spa) cannot see or mutate it.

	t.Run("cross_tenant_isolation", func(t *testing.T) {
		// Create + approve second tenant.
		secondEmail := harness.UniqueEmail("iso-tenant2")
		secondReg, regResp, err := c.Register(harness.RegisterCompanyRequest{
			CompanyName:   "Isolation Tenant 2",
			RequestedSlug: harness.UniqueSlug("iso2"),
			Package:       "starter",
			ContactName:   "Iso Owner 2",
			ContactEmail:  secondEmail,
		})
		require.NoError(t, err)
		require.Equal(t, http.StatusCreated, regResp.StatusCode)

		approval, approvalResp, err := c.Approve(adminToken, secondReg.RegistrationID, nil)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, approvalResp.StatusCode)

		tempPass := approval.TenantAdmin.TemporaryPassword
		secondSlug := approval.Tenant.Slug

		// Rotate password (must_change_password gate).
		secondLogin, slResp, err := c.LoginWithSlug(secondEmail, tempPass, secondSlug)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, slResp.StatusCode)
		secondToken := secondLogin.AccessToken

		newPass := "IsoTenant2Pass2026!"
		cpResp, _, err := c.ChangePassword(secondToken, tempPass, newPass)
		require.NoError(t, err)
		require.Equal(t, http.StatusNoContent, cpResp.StatusCode)

		// Get fresh token after password change.
		secondLogin2, sl2Resp, err := c.LoginWithSlug(secondEmail, newPass, secondSlug)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, sl2Resp.StatusCode)
		secondToken2 := secondLogin2.AccessToken

		// Create a branch for this tenant.
		branch2, brResp, err := c.CreateBranch(secondToken2, harness.CreateBranchRequest{
			Name:     "ISO Branch 2",
			Code:     "ISO2",
			City:     "Bandung",
			Country:  "ID",
			Timezone: "Asia/Jakarta",
		})
		require.NoError(t, err)
		require.Equal(t, http.StatusCreated, brResp.StatusCode)

		// Create a therapist at tenant-2's branch.
		tName := fmt.Sprintf("ISO Therapist %s", harness.UniqueSlug("iso"))
		th, thResp, err := c.CreateTherapist(secondToken2, harness.CreateTherapistRequest{
			BranchID: branch2.ID,
			FullName: tName,
		})
		require.NoError(t, err)
		require.Equal(t, http.StatusCreated, thResp.StatusCode)
		crossTenantTherapistID := th.ID

		// Alice (acme-spa) tries to mutate that therapist → must get 404.
		_, patchResp, rawPatch, err := patchTherapistRaw(c, aliceToken, crossTenantTherapistID,
			harness.UpdateTherapistRequest{})
		require.NoError(t, err)
		assert.Equal(t, http.StatusNotFound, patchResp.StatusCode,
			"cross-tenant PATCH must return 404 (tenant isolation)")
		assert.Equal(t, "THERAPIST_NOT_FOUND", harness.ErrorCode(rawPatch))
	})

	// =========================================================================
	// Boundary B — Cross-branch isolation for branch_admin
	// =========================================================================
	t.Run("cross_branch_admin_isolation", func(t *testing.T) {
		// Create + approve a third tenant with max_branches=2.
		thirdEmail := harness.UniqueEmail("iso-tenant3")
		thirdReg, regResp, err := c.Register(harness.RegisterCompanyRequest{
			CompanyName:   "Isolation Tenant 3",
			RequestedSlug: harness.UniqueSlug("iso3"),
			Package:       "starter",
			ContactName:   "Iso Owner 3",
			ContactEmail:  thirdEmail,
		})
		require.NoError(t, err)
		require.Equal(t, http.StatusCreated, regResp.StatusCode)

		maxBranches := 2
		approval3, approvalResp, err := c.Approve(adminToken, thirdReg.RegistrationID,
			&harness.ApproveRegistrationRequest{MaxBranches: &maxBranches})
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, approvalResp.StatusCode)

		tempPass3 := approval3.TenantAdmin.TemporaryPassword
		thirdSlug := approval3.Tenant.Slug

		// Login + rotate password for third tenant admin.
		thirdLogin, _, err := c.LoginWithSlug(thirdEmail, tempPass3, thirdSlug)
		require.NoError(t, err)
		newPass3 := "IsoTenant3Pass2026!"
		_, _, err = c.ChangePassword(thirdLogin.AccessToken, tempPass3, newPass3)
		require.NoError(t, err)
		thirdLogin2, _, err := c.LoginWithSlug(thirdEmail, newPass3, thirdSlug)
		require.NoError(t, err)
		thirdToken := thirdLogin2.AccessToken

		// Create branch A and branch B for the third tenant.
		branchA, brAResp, err := c.CreateBranch(thirdToken, harness.CreateBranchRequest{
			Name: "ISO Branch A", Code: "ISOA", City: "Jakarta", Country: "ID", Timezone: "Asia/Jakarta",
		})
		require.NoError(t, err)
		require.Equal(t, http.StatusCreated, brAResp.StatusCode)

		branchB, brBResp, err := c.CreateBranch(thirdToken, harness.CreateBranchRequest{
			Name: "ISO Branch B", Code: "ISOB", City: "Surabaya", Country: "ID", Timezone: "Asia/Jakarta",
		})
		require.NoError(t, err)
		require.Equal(t, http.StatusCreated, brBResp.StatusCode)

		// Create a branch_admin user at branch A via POST /admin/users.
		// (The user is created with must_change_password=true; we'll change it.)
		baEmail := harness.UniqueEmail("branch-admin")
		baUserResp, createUserResp, rawCreateUser, err := createAdminUser(c, thirdToken, baEmail, branchA.ID, thirdSlug)
		require.NoError(t, err)
		if createUserResp.StatusCode != http.StatusCreated {
			t.Fatalf("create branch_admin user failed: status=%d body=%s",
				createUserResp.StatusCode, string(rawCreateUser))
		}

		baInitialPassword := baUserResp.InitialPassword
		require.NotEmpty(t, baInitialPassword, "initial_password must be present in user create response")

		// Login as branch_admin and rotate password.
		baLogin, baLoginResp, err := c.LoginWithSlug(baEmail, baInitialPassword, thirdSlug)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, baLoginResp.StatusCode,
			"branch_admin must be able to login")
		baNewPass := "BranchAdminPass2026!"
		_, _, err = c.ChangePassword(baLogin.AccessToken, baInitialPassword, baNewPass)
		require.NoError(t, err)
		baLogin2, _, err := c.LoginWithSlug(baEmail, baNewPass, thirdSlug)
		require.NoError(t, err)
		baToken := baLogin2.AccessToken

		// Create a therapist at branch B using the tenant admin token.
		thB, thBResp, err := c.CreateTherapist(thirdToken, harness.CreateTherapistRequest{
			BranchID: branchB.ID,
			FullName: fmt.Sprintf("BranchB Therapist %s", harness.UniqueSlug("thb")),
		})
		require.NoError(t, err)
		require.Equal(t, http.StatusCreated, thBResp.StatusCode)
		therapistBranchBID := thB.ID

		// ----- branch_admin at A tries to UPDATE therapist at B → 403 -----
		_, patchResp, rawPatch, err := patchTherapistRaw(c, baToken, therapistBranchBID,
			harness.UpdateTherapistRequest{})
		require.NoError(t, err)
		assert.Equal(t, http.StatusForbidden, patchResp.StatusCode,
			"branch_admin must not update a therapist at another branch")
		assert.Equal(t, "CROSS_BRANCH_FORBIDDEN", harness.ErrorCode(rawPatch),
			"error code must be CROSS_BRANCH_FORBIDDEN")

		// ----- branch_admin at A tries to PUT availability for B's therapist → 403 -----
		_, availResp, rawAvail, err := putAvailRaw(c, baToken, therapistBranchBID,
			[]harness.AvailabilityWindow{{DOW: 1, Start: "09:00", End: "17:00"}})
		require.NoError(t, err)
		assert.Equal(t, http.StatusForbidden, availResp.StatusCode,
			"branch_admin must not PUT availability for a therapist at another branch")
		assert.Equal(t, "CROSS_BRANCH_FORBIDDEN", harness.ErrorCode(rawAvail))

		// ----- branch_admin at A tries to PUT services for B's therapist → 403 -----
		_, svcMappingResp, rawSvcMapping, err := putServicesRaw(c, baToken, therapistBranchBID, []string{})
		require.NoError(t, err)
		assert.Equal(t, http.StatusForbidden, svcMappingResp.StatusCode,
			"branch_admin must not PUT service mappings for a therapist at another branch")
		assert.Equal(t, "CROSS_BRANCH_FORBIDDEN", harness.ErrorCode(rawSvcMapping))

		// ----- LIST therapists as branch_admin and document the scope -----
		//
		// API_CONTRACT.md §11.4.2: "A branch_admin always sees only their
		// assigned branches regardless of this filter."
		// We assert that therapist B (at branchB, not branchA) is NOT visible
		// to branch_admin of branch A.
		listResult, listResp, err := c.ListTherapists(baToken, "")
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, listResp.StatusCode,
			"branch_admin must be able to list therapists")

		therapistBVisible := false
		for _, th := range listResult.Data {
			if th.ID == therapistBranchBID {
				therapistBVisible = true
				break
			}
		}
		// Per API contract, branch_admin sees only their assigned branches.
		assert.False(t, therapistBVisible,
			"branch_admin at branch A must NOT see therapists at branch B "+
				"(ADR 0009 §11.4.2: branch_admin always scoped to assigned branches)")

		t.Logf("branch_admin LIST scope: %d therapists visible; therapist at other branch visible=%v",
			len(listResult.Data), therapistBVisible)
	})
}

// patchTherapistRaw returns raw response bytes for PATCH /therapists/:id.
func patchTherapistRaw(c *harness.APIClient, token, id string, body harness.UpdateTherapistRequest) (harness.TherapistResponse, *http.Response, []byte, error) {
	resp, raw, err := c.RawDo("PATCH", "/api/v1/tenant/therapists/"+id, body, token)
	if err != nil {
		return harness.TherapistResponse{}, nil, nil, err
	}
	var out harness.TherapistResponse
	_ = harness.DecodeJSONPublic(raw, &out)
	return out, resp, raw, nil
}

// putServicesRaw returns raw response bytes for PUT /therapists/:id/services.
func putServicesRaw(c *harness.APIClient, token, therapistID string, serviceIDs []string) (harness.ServiceMappingResponse, *http.Response, []byte, error) {
	resp, raw, err := c.RawDo("PUT",
		"/api/v1/tenant/therapists/"+therapistID+"/services",
		harness.PutServicesRequest{ServiceIDs: serviceIDs},
		token)
	if err != nil {
		return harness.ServiceMappingResponse{}, nil, nil, err
	}
	var out harness.ServiceMappingResponse
	_ = harness.DecodeJSONPublic(raw, &out)
	return out, resp, raw, nil
}

// createUserResult holds the minimal fields we need from POST /admin/users.
type createUserResult struct {
	User            struct{ ID string `json:"id"` } `json:"user"`
	InitialPassword string                          `json:"initial_password"`
}

// createAdminUser creates a branch_admin user at the specified branch. It
// first resolves the branch_admin role ID via GET /admin/roles so the created
// user inherits the full branch_admin permission set (list/update therapists,
// availability, etc.) — without this the user has zero permissions and every
// subsequent LIST/UPDATE returns 403 by RBAC rather than by the cross-branch
// guard we actually want to test.
func createAdminUser(c *harness.APIClient, token, email, branchID, _ string) (createUserResult, *http.Response, []byte, error) {
	// 1) Resolve branch_admin role ID.
	rolesResp, rolesRaw, err := c.RawDo("GET", "/api/v1/admin/roles", nil, token)
	if err != nil {
		return createUserResult{}, nil, nil, err
	}
	if rolesResp.StatusCode != http.StatusOK {
		return createUserResult{}, rolesResp, rolesRaw, nil
	}
	var roles struct {
		Data []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"data"`
	}
	_ = harness.DecodeJSONPublic(rolesRaw, &roles)
	var branchAdminID string
	for _, r := range roles.Data {
		if r.Name == "branch_admin" {
			branchAdminID = r.ID
			break
		}
	}

	// 2) Create the user with the resolved role assigned.
	body := map[string]interface{}{
		"email":      email,
		"full_name":  "Branch Admin Test User",
		"branch_ids": []string{branchID},
	}
	if branchAdminID != "" {
		body["role_ids"] = []string{branchAdminID}
	}
	resp, raw, err := c.RawDo("POST", "/api/v1/admin/users", body, token)
	if err != nil {
		return createUserResult{}, nil, nil, err
	}
	var out createUserResult
	_ = harness.DecodeJSONPublic(raw, &out)
	return out, resp, raw, nil
}
