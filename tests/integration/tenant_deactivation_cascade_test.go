package integration_test

// T7 — tenant deactivation cascade
//
// PATCH /admin/tenants/:id/status → deactivated
// Assertions:
//  1. HTTP 200, tenant status=deactivated.
//  2. All membership rows for that tenant have status='suspended' (DB).
//  3. All refresh_token rows for that tenant's users have revoked_at IS NOT NULL (DB).
//     BUG S2: As of Phase 3 delivery, the service does NOT revoke refresh tokens on
//     tenant deactivation. The membership cascade (step 2) works correctly but the
//     refresh_token revocation is missing from TransitionStatus in tenant_service.go.
//  4. Login with the tenant_admin of that tenant returns 401.
//     BUG S2: Login currently returns 200 scope=user (user is authenticated but has
//     no active memberships). The expected behavior is 401 TENANT_INACTIVE or that
//     the login service rejects users whose only membership is suspended/deactivated.
//     This is a security gap: a deactivated tenant's admin can still obtain access
//     tokens (with scope=user, no tenant context), which may allow access to
//     user-scoped endpoints.

import (
	"net/http"
	"testing"
	"time"

	"github.com/chrisanlung/lustia-integration-tests/harness"
	"github.com/stretchr/testify/require"
)

func TestTenantDeactivationCascade(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test — requires live service")
	}

	// 1. Create a fresh tenant via the approval flow.
	adminToken := superAdminToken(t)
	c := harness.NewClient()
	c.ForwardedFor = harness.UniqueIP()

	email := harness.UniqueEmail("deactivate")
	reg, regResp, err := c.Register(harness.RegisterCompanyRequest{
		CompanyName:   "Deactivate Me Spa",
		RequestedSlug: harness.UniqueSlug("deactivate"),
		Package:       "starter",
		ContactName:   "Deactivate Owner",
		ContactEmail:  email,
		ContactPhone:  "+628111222333",
	})
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, regResp.StatusCode)

	approval, approvalResp, err := c.Approve(adminToken, reg.RegistrationID, nil)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, approvalResp.StatusCode)

	tenantID := approval.Tenant.ID
	tenantAdminUserID := approval.TenantAdmin.UserID
	tempPassword := approval.TenantAdmin.TemporaryPassword

	// 2. Login as the tenant admin to create a refresh token row.
	loginOut, loginResp, err := c.Login(email, tempPassword)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, loginResp.StatusCode, "tenant admin login must succeed")
	tenantAdminToken := loginOut.AccessToken
	_ = tenantAdminToken // used below

	// 3. Deactivate the tenant.
	deactivated, deactivateResp, err := c.ChangeTenantStatus(adminToken, tenantID, "deactivated", "integration test teardown")
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, deactivateResp.StatusCode,
		"PATCH tenant status must succeed")
	require.Equal(t, "deactivated", deactivated.Status, "tenant must be deactivated")

	// 4. DB: all memberships for this tenant must be suspended.
	db, err := harness.DB()
	require.NoError(t, err)

	tx, err := db.Begin()
	require.NoError(t, err)
	defer tx.Rollback() //nolint:errcheck

	_, err = tx.Exec("SET LOCAL app.current_tenant = '__platform__'")
	require.NoError(t, err)

	var activeMembershipCount int
	err = tx.QueryRow(`
		SELECT COUNT(*) FROM membership
		WHERE tenant_id = $1 AND status != 'suspended'
	`, tenantID).Scan(&activeMembershipCount)
	require.NoError(t, err)
	if activeMembershipCount != 0 {
		t.Errorf("expected 0 non-suspended memberships after deactivation, got %d",
			activeMembershipCount)
	}

	// 5. DB: all refresh tokens for this tenant's users must be revoked.
	// BUG S2 — this assertion is expected to FAIL until the backend is fixed.
	// refresh_token revocation is not performed by TransitionStatus in tenant_service.go.
	var unrevokedTokenCount int
	err = tx.QueryRow(`
		SELECT COUNT(*) FROM refresh_token
		WHERE user_id = $1 AND revoked_at IS NULL
	`, tenantAdminUserID).Scan(&unrevokedTokenCount)
	require.NoError(t, err)
	if unrevokedTokenCount != 0 {
		t.Errorf("BUG S2 (tenant_service.go TransitionStatus): %d unrevoked refresh token(s) remain "+
			"after tenant deactivation — expected 0. The deactivation cascade must also call "+
			"RefreshTokenRepository.RevokeAllForUser for each affected user.",
			unrevokedTokenCount)
	}

	require.NoError(t, tx.Commit())

	// 6. Login must fail (401) after tenant is deactivated.
	// BUG S2 — this assertion is expected to FAIL until the backend is fixed.
	// Currently returns 200 with scope=user because the login flow does not
	// prevent authentication when all memberships are suspended. The user still
	// has a valid identity but no active tenant context.
	time.Sleep(100 * time.Millisecond)

	loginOut2, loginAfterDeactivateResp, err := c.Login(email, tempPassword)
	require.NoError(t, err)
	if loginAfterDeactivateResp.StatusCode != http.StatusUnauthorized {
		t.Errorf("BUG S2 (auth_service.go Login): login returned HTTP %d (body scope=%q) "+
			"after tenant deactivation — expected 401. Users whose only membership is "+
			"suspended/deactivated must not be able to obtain new tokens.",
			loginAfterDeactivateResp.StatusCode, loginOut2.Scope)
	}
}
