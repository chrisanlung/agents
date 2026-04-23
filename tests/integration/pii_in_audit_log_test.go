package integration_test

// T8 — PII in audit log (security H-2 fix regression test)
//
// When a company registration is submitted, the service writes an
// audit_log entry with action='registration.submitted'.
//
// Security H-2 fix requires:
//   - meta->>'email_prefix' is present and is exactly 8 hex characters
//   - meta->>'email' IS NULL (raw email must NOT be stored)
//
// This test verifies the fix has not regressed for new registrations.

import (
	"regexp"
	"testing"

	"github.com/chrisanlung/lustia-integration-tests/harness"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var hexPrefixRE = regexp.MustCompile(`^[0-9a-f]{8}$`)

func TestPIINotInAuditLog(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test — requires live service")
	}

	c := harness.NewClient()
	c.ForwardedFor = harness.UniqueIP()

	email := harness.UniqueEmail("auditpii")
	reg, regResp, err := c.Register(harness.RegisterCompanyRequest{
		CompanyName:   "Audit PII Spa",
		RequestedSlug: harness.UniqueSlug("auditpii"),
		Package:       "starter",
		ContactName:   "Audit Owner",
		ContactEmail:  email,
		ContactPhone:  "+628111222333",
	})
	require.NoError(t, err)
	require.Equal(t, 201, regResp.StatusCode, "registration must succeed")
	require.NotEmpty(t, reg.RegistrationID)

	db, err := harness.DB()
	require.NoError(t, err)

	tx, err := db.Begin()
	require.NoError(t, err)
	defer tx.Rollback() //nolint:errcheck

	_, err = tx.Exec("SET LOCAL app.current_tenant = '__platform__'")
	require.NoError(t, err)

	// Query the audit_log entry for this specific registration.
	var emailPrefix, rawEmail *string
	err = tx.QueryRow(`
		SELECT
			meta->>'email_prefix',
			meta->>'email'
		FROM audit_log
		WHERE action = 'registration.submitted'
		  AND resource_id = $1
		ORDER BY created_at DESC
		LIMIT 1
	`, reg.RegistrationID).Scan(&emailPrefix, &rawEmail)
	require.NoError(t, err, "audit_log entry for registration.submitted must exist")
	require.NoError(t, tx.Commit())

	// H-2 assertion 1: email_prefix must be present and be exactly 8 hex chars.
	require.NotNil(t, emailPrefix,
		"audit log meta must have email_prefix (H-2 fix)")
	assert.Regexp(t, hexPrefixRE, *emailPrefix,
		"email_prefix must be exactly 8 lowercase hex characters")

	// H-2 assertion 2: raw email must NOT be in meta.
	assert.Nil(t, rawEmail,
		"audit log meta must NOT contain raw email field (H-2 fix regression)")
	if rawEmail != nil {
		t.Errorf("H-2 REGRESSION: raw email '%s' found in audit_log.meta — "+
			"see SECURITY.md §Phase 3 Security Review H-2", *rawEmail)
	}
}

// TestPIIAuditLog_OldEntriesAreHistorical documents that some pre-fix audit
// entries may still contain raw email. This is a known historical state, not
// a regression. The test explicitly documents the expectation.
func TestPIIAuditLog_OldEntriesAreHistorical(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test — requires live service")
	}

	db, err := harness.DB()
	require.NoError(t, err)

	tx, err := db.Begin()
	require.NoError(t, err)
	defer tx.Rollback() //nolint:errcheck

	_, err = tx.Exec("SET LOCAL app.current_tenant = '__platform__'")
	require.NoError(t, err)

	// Count entries that still have the raw 'email' key (pre-fix entries).
	var oldCount, totalCount int
	err = tx.QueryRow(`
		SELECT COUNT(*) FROM audit_log
		WHERE action = 'registration.submitted'
		  AND meta ? 'email'
	`).Scan(&oldCount)
	require.NoError(t, err)

	err = tx.QueryRow(`
		SELECT COUNT(*) FROM audit_log
		WHERE action = 'registration.submitted'
	`).Scan(&totalCount)
	require.NoError(t, err)
	require.NoError(t, tx.Commit())

	// Document the state — this is informational, not a failure.
	t.Logf("Audit log state: %d total registration.submitted entries, "+
		"%d still have raw email (pre-fix historical entries)", totalCount, oldCount)

	// Any entry created after the H-2 fix must NOT have raw email.
	// TestPIINotInAuditLog covers new entries; this test just documents history.
}
