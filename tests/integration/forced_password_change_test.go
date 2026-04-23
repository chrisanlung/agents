package integration_test

// T5 — forced password change gate
//
// Verify the must_change_password enforcement:
//  1. Login as a freshly approved tenant_admin → JWT must have must_change_password=true.
//  2. Any protected endpoint (except /auth/me, /auth/me/password, /auth/logout)
//     returns 403 PASSWORD_CHANGE_REQUIRED.
//  3. POST /auth/me/password succeeds.
//  4. Old password no longer works for login.
//  5. New password works and new JWT has must_change_password=false.

import (
	"net/http"
	"testing"

	"github.com/chrisanlung/lustia-integration-tests/harness"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestForcedPasswordChange(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test — requires live service")
	}

	c := harness.NewClient()

	// Step 1: get a freshly approved tenant admin (must_change_password=true).
	token, tempPassword, email := ApproveAndGetToken(t)

	// Step 1a: decode JWT and confirm must_change_password=true.
	claims, err := harness.DecodeJWT(token)
	require.NoError(t, err, "JWT must be decodable")
	assert.True(t, claims.MustChangePassword,
		"freshly approved user JWT must have must_change_password=true")

	// Step 2: protected endpoint (list branches) must return 403 PASSWORD_CHANGE_REQUIRED.
	_, rawBody, err := c.RawDo("GET", "/api/v1/tenant/branches", nil, token)
	require.NoError(t, err)
	assert.Equal(t, "PASSWORD_CHANGE_REQUIRED", harness.ErrorCode(rawBody),
		"protected endpoint must return PASSWORD_CHANGE_REQUIRED before password change")

	// Step 2a: GET /auth/me must still work (whitelisted endpoint).
	_, meResp, err := c.GetMe(token)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, meResp.StatusCode,
		"GET /auth/me must be allowed even when must_change_password=true")

	// Step 3: change password.
	newPassword := "NewSecurePass2026!"
	cpResp, _, err := c.ChangePassword(token, tempPassword, newPassword)
	require.NoError(t, err)
	assert.Equal(t, http.StatusNoContent, cpResp.StatusCode,
		"POST /auth/me/password must succeed with correct old password")

	// Step 4: old password no longer works.
	_, oldLoginResp, err := c.Login(email, tempPassword)
	require.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, oldLoginResp.StatusCode,
		"old (temporary) password must be rejected after password change")

	// Step 5: new password works and new JWT lacks must_change_password.
	newLogin, newLoginResp, err := c.Login(email, newPassword)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, newLoginResp.StatusCode,
		"new password must allow login")

	newClaims, err := harness.DecodeJWT(newLogin.AccessToken)
	require.NoError(t, err)
	assert.False(t, newClaims.MustChangePassword,
		"new JWT must NOT have must_change_password=true after password change")

	// Step 5a: protected endpoint now works.
	_, branchListResp, err := c.ListBranches(newLogin.AccessToken)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, branchListResp.StatusCode,
		"branch list must work after password change with new token")
}
