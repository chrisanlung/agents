package integration_test

// T3 — registration rate limit
//
// Submit 4 registrations from the same IP within the hour → 4th returns 429
// with error code RATE_LIMITED.
//
// Implementation note: the auth-service uses an in-memory per-IP rate limiter
// (3 req/hour per IP). The service reads X-Forwarded-For when Gin trusts proxy
// headers. We verified empirically that the service DOES honour
// X-Forwarded-For: sending the same header value on every request triggers
// the rate limit correctly. Each test run uses a unique IP address so parallel
// runs and re-runs don't interfere.
//
// Flaky-test risk: if the rate-limit window started on a prior test run and
// hasn't expired yet, this test may hit 429 sooner than expected or not at all
// if the counter was reset between runs. The unique synthetic IP per test run
// mitigates the "sooner" case. The test is marked potentially flaky in
// TEST_PLAN.md §Flaky-test log.

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"testing"

	"github.com/chrisanlung/lustia-integration-tests/harness"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRegistrationRateLimit(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test — requires live service")
	}

	// Generate a unique synthetic IP per test run so the in-memory bucket is fresh.
	b := make([]byte, 3)
	_, _ = rand.Read(b)
	syntheticIP := fmt.Sprintf("10.%d.%d.%d", b[0], b[1], b[2])

	c := harness.NewClient()
	c.ForwardedFor = syntheticIP

	// Send 3 requests that should succeed.
	for i := 1; i <= 3; i++ {
		req := harness.RegisterCompanyRequest{
			CompanyName:   fmt.Sprintf("RL Corp %s %d", hex.EncodeToString(b), i),
			RequestedSlug: harness.UniqueSlug(fmt.Sprintf("rl%s%d", hex.EncodeToString(b[:1]), i)),
			Package:       "starter",
			ContactName:   "RL Owner",
			ContactEmail:  harness.UniqueEmail(fmt.Sprintf("rl%d", i)),
			ContactPhone:  "+628123456789",
		}
		_, resp, err := c.Register(req)
		require.NoError(t, err)
		assert.Equal(t, http.StatusCreated, resp.StatusCode,
			"request %d should succeed (under rate limit)", i)
	}

	// 4th request from same IP must be rate-limited.
	req4 := harness.RegisterCompanyRequest{
		CompanyName:   fmt.Sprintf("RL Corp %s 4", hex.EncodeToString(b)),
		RequestedSlug: harness.UniqueSlug(fmt.Sprintf("rl%s4", hex.EncodeToString(b[:1]))),
		Package:       "starter",
		ContactName:   "RL Owner",
		ContactEmail:  harness.UniqueEmail("rl4"),
		ContactPhone:  "+628123456789",
	}
	resp4, rawBody, err := c.RawDo("POST", "/api/v1/register/company", req4, "")
	require.NoError(t, err)

	assert.Equal(t, http.StatusTooManyRequests, resp4.StatusCode, "4th request must return 429")
	code := harness.ErrorCode(rawBody)
	assert.Equal(t, "RATE_LIMITED", code, "error code on 429 must be RATE_LIMITED")
}
