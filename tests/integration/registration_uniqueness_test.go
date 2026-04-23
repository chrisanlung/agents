package integration_test

// T2 — registration uniqueness / security M-1 fix
//
// Same email submitted twice → second must return 409 with error code CONFLICT
// (NOT DUPLICATE_PENDING_REGISTRATION — that was the M-1 finding; merging the
// two codes into CONFLICT removes the email-existence enumeration attack).

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/chrisanlung/lustia-integration-tests/harness"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRegistrationDuplicateEmail_Returns409Conflict(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test — requires live service")
	}

	c := harness.NewClient()
	c.ForwardedFor = harness.UniqueIP()
	email := harness.UniqueEmail("dup")
	slug1 := harness.UniqueSlug("dup1")
	slug2 := harness.UniqueSlug("dup2")

	// First submission must succeed.
	first := harness.RegisterCompanyRequest{
		CompanyName:   "Dup Email Corp First",
		RequestedSlug: slug1,
		Package:       "starter",
		ContactName:   "Dup Owner",
		ContactEmail:  email,
		ContactPhone:  "+628123456789",
	}
	_, resp1, err := c.Register(first)
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, resp1.StatusCode, "first submission must succeed")

	// Second submission with the same email must return 409.
	second := harness.RegisterCompanyRequest{
		CompanyName:   "Dup Email Corp Second",
		RequestedSlug: slug2,
		Package:       "starter",
		ContactName:   "Dup Owner 2",
		ContactEmail:  email, // same email!
		ContactPhone:  "+628123456789",
	}
	resp2, raw, rawErr := c.RawDo("POST", "/api/v1/register/company", second, "")
	require.NoError(t, rawErr)

	assert.Equal(t, http.StatusConflict, resp2.StatusCode, "duplicate email must return 409")

	// Security M-1: the code must be the generic CONFLICT, not the more
	// specific DUPLICATE_PENDING_REGISTRATION (which leaks email existence).
	var errBody struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	require.NoError(t, json.Unmarshal(raw, &errBody))
	assert.Equal(t, "CONFLICT", errBody.Error.Code,
		"M-1 fix: duplicate must use generic CONFLICT, not DUPLICATE_PENDING_REGISTRATION")
	assert.NotEqual(t, "DUPLICATE_PENDING_REGISTRATION", errBody.Error.Code,
		"M-1 regression: DUPLICATE_PENDING_REGISTRATION must no longer be returned")
}

func TestRegistrationDuplicateSlug_Returns409Conflict(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test — requires live service")
	}

	c := harness.NewClient()
	c.ForwardedFor = harness.UniqueIP()
	slug := harness.UniqueSlug("dupslug")

	// First submission.
	first := harness.RegisterCompanyRequest{
		CompanyName:   "Dup Slug Corp First",
		RequestedSlug: slug,
		Package:       "starter",
		ContactName:   "Slug Owner",
		ContactEmail:  harness.UniqueEmail("slugowner1"),
		ContactPhone:  "+628123456789",
	}
	_, resp1, err := c.Register(first)
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, resp1.StatusCode, "first slug submission must succeed")

	// Second submission with same slug but different email.
	second := harness.RegisterCompanyRequest{
		CompanyName:   "Dup Slug Corp Second",
		RequestedSlug: slug, // same slug!
		Package:       "starter",
		ContactName:   "Other Owner",
		ContactEmail:  harness.UniqueEmail("slugowner2"),
		ContactPhone:  "+628123456789",
	}
	_, raw, err := c.RawDo("POST", "/api/v1/register/company", second, "")
	require.NoError(t, err)

	var errBody struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	require.NoError(t, json.Unmarshal(raw, &errBody))

	// Both slug conflicts should also be CONFLICT (generic), not the specific
	// DUPLICATE_PENDING_REGISTRATION or TENANT_SLUG_TAKEN codes.
	assert.Equal(t, "CONFLICT", errBody.Error.Code,
		"M-1 fix: slug conflict must use generic CONFLICT")
}
