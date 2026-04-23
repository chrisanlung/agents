package integration_test

// T1 — registration happy path
// POST /api/v1/register/company with valid unique data → 201 with registration_id.

import (
	"net/http"
	"testing"

	"github.com/chrisanlung/lustia-integration-tests/harness"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRegistrationHappyPath(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test — requires live service")
	}

	c := harness.NewClient()
	c.ForwardedFor = harness.UniqueIP() // avoid rate-limit collisions between test runs

	req := harness.RegisterCompanyRequest{
		CompanyName:   "Happy Path Spa",
		RequestedSlug: harness.UniqueSlug("happy"),
		Package:       "starter",
		ContactName:   "Happy Owner",
		ContactEmail:  harness.UniqueEmail("happy"),
		ContactPhone:  "+628123456789",
	}

	out, resp, err := c.Register(req)
	require.NoError(t, err, "HTTP call must succeed")
	assert.Equal(t, http.StatusCreated, resp.StatusCode, "expected 201 Created")
	assert.NotEmpty(t, out.RegistrationID, "registration_id must be a non-empty string")
	assert.Equal(t, "pending", out.Status, "newly submitted registration must have status=pending")
}

func TestRegistrationValidation_MissingContactEmail(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test — requires live service")
	}

	c := harness.NewClient()
	c.ForwardedFor = harness.UniqueIP()
	// omit required contact_email
	req := harness.RegisterCompanyRequest{
		CompanyName:   "No Email Corp",
		RequestedSlug: harness.UniqueSlug("noemail"),
		ContactName:   "Some Person",
	}

	_, resp, err := c.Register(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode, "missing email must return 400")
}

func TestRegistrationValidation_InvalidSlugChars(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test — requires live service")
	}

	c := harness.NewClient()
	c.ForwardedFor = harness.UniqueIP()
	req := harness.RegisterCompanyRequest{
		CompanyName:   "Bad Slug Corp",
		RequestedSlug: "BAD_SLUG_WITH_CAPS", // uppercase not allowed
		ContactName:   "Some Person",
		ContactEmail:  harness.UniqueEmail("badslug"),
	}

	_, resp, err := c.Register(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode, "invalid slug chars must return 400")
}
