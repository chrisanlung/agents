package integration_test

// T4 — full approval flow
//
// Prerequisites: SUPER_ADMIN_EMAIL + SUPER_ADMIN_PASSWORD env vars.
// If absent the test is skipped with an actionable message.
//
// Steps:
//  1. Submit a company registration.
//  2. Super admin lists pending registrations, finds the new one.
//  3. Super admin approves.
//  4. Response contains temporary_password.
//  5. Mailpit received a welcome email to the contact address (SMTP enabled).
//  6. DB assertions: tenant is active, user has must_change_password=true,
//     membership exists with status=active, user_role has tenant_admin.

import (
	"database/sql"
	"net/http"
	"os"
	"testing"

	"github.com/chrisanlung/lustia-integration-tests/harness"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func superAdminToken(t *testing.T) string {
	t.Helper()
	email := os.Getenv("SUPER_ADMIN_EMAIL")
	password := os.Getenv("SUPER_ADMIN_PASSWORD")
	if email == "" || password == "" {
		t.Skip("SUPER_ADMIN_EMAIL and SUPER_ADMIN_PASSWORD must be set to run this test")
	}
	c := harness.NewClient()
	login, resp, err := c.Login(email, password)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode,
		"super admin login must succeed — check SUPER_ADMIN_EMAIL / SUPER_ADMIN_PASSWORD; "+
			"if the account is locked, run: UPDATE \"user\" SET failed_login_count=0, locked_until=NULL "+
			"WHERE email='%s';", email)
	require.Equal(t, "platform", login.Scope, "super admin token must have scope=platform")
	return login.AccessToken
}

func TestApprovalFlow(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test — requires live service")
	}

	adminToken := superAdminToken(t)
	c := harness.NewClient()
	c.ForwardedFor = harness.UniqueIP()

	// --- Step 1: submit a registration ---
	contactEmail := harness.UniqueEmail("approval")
	reg, regResp, err := c.Register(harness.RegisterCompanyRequest{
		CompanyName:   "Approval Flow Spa",
		RequestedSlug: harness.UniqueSlug("approval"),
		Package:       "starter",
		ContactName:   "Approval Owner",
		ContactEmail:  contactEmail,
		ContactPhone:  "+628111222333",
	})
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, regResp.StatusCode, "registration must succeed")
	require.NotEmpty(t, reg.RegistrationID)

	// --- Step 2: super admin lists pending registrations ---
	list, listResp, err := c.ListRegistrations(adminToken, "pending")
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, listResp.StatusCode, "list registrations must succeed")

	found := false
	for _, item := range list.Data {
		if item.ID == reg.RegistrationID {
			found = true
			assert.Equal(t, "pending", item.Status)
			break
		}
	}
	assert.True(t, found, "newly submitted registration must appear in the pending list")

	// --- Step 3: approve ---
	approval, approvalResp, err := c.Approve(adminToken, reg.RegistrationID, nil)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, approvalResp.StatusCode, "approval must succeed")

	// --- Step 4: response assertions ---
	assert.Equal(t, "approved", approval.Registration.Status)
	assert.Equal(t, "active", approval.Tenant.Status)
	assert.Equal(t, "starter", approval.Tenant.Package)
	assert.Equal(t, 1, approval.Tenant.MaxBranches)
	assert.NotEmpty(t, approval.TenantAdmin.UserID)
	assert.Equal(t, contactEmail, approval.TenantAdmin.Email)
	require.NotEmpty(t, approval.TenantAdmin.TemporaryPassword,
		"temporary_password must be present in approval response (returned once)")
	assert.Len(t, approval.TenantAdmin.TemporaryPassword, 16,
		"temporary password must be 16 hex characters")

	// --- Step 5: Mailpit — welcome email delivered ---
	// Only check if SMTP is enabled in this environment. In test environments
	// without Mailpit this sub-check is skipped gracefully.
	smtpEnabled := os.Getenv("SMTP_ENABLED")
	if smtpEnabled == "" || smtpEnabled == "true" {
		msgs, mailErr := harness.Messages(contactEmail)
		if mailErr == nil {
			assert.Greater(t, len(msgs), 0,
				"welcome email must be delivered to %s via Mailpit", contactEmail)
			if len(msgs) > 0 {
				assert.Contains(t, msgs[0].Subject, "Lustia",
					"welcome email subject must mention Lustia")
			}
		} else {
			t.Logf("Mailpit check skipped: %v", mailErr)
		}
	}

	// --- Step 6: DB assertions ---
	db, err := harness.DB()
	require.NoError(t, err)

	// Tenant must be active.
	var tenantStatus string
	err = db.QueryRow(
		`SET LOCAL app.current_tenant = '__platform__'; `+
			`SELECT status FROM tenant WHERE id = $1`,
		approval.Tenant.ID,
	).Scan(&tenantStatus)
	// Note: SET LOCAL inside QueryRow doesn't work across statement boundaries.
	// Use a transaction instead.
	tx, err := db.Begin()
	require.NoError(t, err)
	defer tx.Rollback() //nolint:errcheck

	_, err = tx.Exec("SET LOCAL app.current_tenant = '__platform__'")
	require.NoError(t, err)

	err = tx.QueryRow(`SELECT status FROM tenant WHERE id = $1`, approval.Tenant.ID).Scan(&tenantStatus)
	require.NoError(t, err)
	assert.Equal(t, "active", tenantStatus, "tenant must be active after approval")

	// User must exist with must_change_password=true.
	var mustChange bool
	err = tx.QueryRow(
		`SELECT must_change_password FROM "user" WHERE id = $1`,
		approval.TenantAdmin.UserID,
	).Scan(&mustChange)
	require.NoError(t, err)
	assert.True(t, mustChange,
		"newly approved tenant_admin must have must_change_password=true")

	// Membership must exist with status=active.
	var membershipStatus string
	err = tx.QueryRow(
		`SELECT m.status FROM membership m WHERE m.user_id = $1 AND m.tenant_id = $2`,
		approval.TenantAdmin.UserID, approval.Tenant.ID,
	).Scan(&membershipStatus)
	require.NoError(t, err)
	assert.Equal(t, "active", membershipStatus, "membership must be active")

	// user_role must have tenant_admin role.
	var roleCount int
	err = tx.QueryRow(`
		SELECT COUNT(*) FROM user_role ur
		JOIN membership m ON m.id = ur.membership_id
		JOIN role r ON r.id = ur.role_id
		WHERE m.user_id = $1 AND r.name = 'tenant_admin'
	`, approval.TenantAdmin.UserID).Scan(&roleCount)
	require.NoError(t, err)
	assert.Equal(t, 1, roleCount, "tenant_admin role must be assigned")

	require.NoError(t, tx.Commit())
}

// ApproveAndGetToken is a reusable helper for tests that need a freshly
// approved tenant. Returns the token obtained by logging in with the
// temporary password. Exported so other test files in the package can use it.
func ApproveAndGetToken(t *testing.T) (accessToken, tempPassword, userEmail string) {
	t.Helper()
	adminToken := superAdminToken(t)
	c := harness.NewClient()
	c.ForwardedFor = harness.UniqueIP() // isolate rate-limit bucket from other tests

	email := harness.UniqueEmail("tenantadmin")
	reg, regResp, err := c.Register(harness.RegisterCompanyRequest{
		CompanyName:   "Helper Tenant Spa",
		RequestedSlug: harness.UniqueSlug("helper"),
		Package:       "starter",
		ContactName:   "Helper Owner",
		ContactEmail:  email,
		ContactPhone:  "+628111222333",
	})
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, regResp.StatusCode)

	approval, approvalResp, err := c.Approve(adminToken, reg.RegistrationID, nil)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, approvalResp.StatusCode)

	// Log in as the newly created tenant admin.
	login, loginResp, err := c.Login(email, approval.TenantAdmin.TemporaryPassword)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, loginResp.StatusCode,
		"login with temporary password must succeed")

	return login.AccessToken, approval.TenantAdmin.TemporaryPassword, email
}

// approvedUserID is a helper that queries the user ID for a given email.
func approvedUserID(t *testing.T, email string) string {
	t.Helper()
	db, err := harness.DB()
	require.NoError(t, err)

	tx, err := db.Begin()
	require.NoError(t, err)
	defer tx.Rollback() //nolint:errcheck

	_, err = tx.Exec("SET LOCAL app.current_tenant = '__platform__'")
	require.NoError(t, err)

	var id string
	err = tx.QueryRow(`SELECT id FROM "user" WHERE email = $1`, email).Scan(&id)
	if err == sql.ErrNoRows {
		t.Fatalf("user with email %s not found", email)
	}
	require.NoError(t, err)
	require.NoError(t, tx.Commit())
	return id
}
