package integration_test

// T12 — Availability PUT (critical flow per ADR 0009 §2.7)
//
// Verifies the full-replace semantics of PUT /therapists/:id/availability
// and all service-layer validation rules:
//
//  Happy paths:
//   - PUT empty windows → 200, GET returns empty
//   - PUT 3 windows (Mon 09:00-12:00, Mon 13:00-17:00, Tue 09:00-17:00) → 200
//   - GET → returns the 3 windows sorted by dow ASC, start ASC
//
//  Validation error paths (all must return 400 or 409):
//   - Overlapping same-day windows → 409 AVAILABILITY_OVERLAP
//   - end <= start → 400 VALIDATION
//   - dow=7 (out of range) → 400 VALIDATION
//   - Non-5-minute boundary (09:03-12:00) → 400 VALIDATION

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/chrisanlung/lustia-integration-tests/harness"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAvailabilityPut(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test — requires live service")
	}

	token := aliceTokenP4(t)
	c := harness.NewClient()

	// Create a fresh therapist for availability editing so we don't disturb
	// the seeded Budi / Siti availability windows.
	therapistName := fmt.Sprintf("Avail T12 %s", harness.UniqueSlug("t12"))
	therapist, tResp, err := c.CreateTherapist(token, harness.CreateTherapistRequest{
		BranchID: seededBranchID,
		FullName: therapistName,
	})
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, tResp.StatusCode)
	therapistID := therapist.ID

	t.Cleanup(func() {
		c.DeleteTherapist(token, therapistID) //nolint:errcheck
	})

	// -------------------------------------------------------------------------
	// Step 1: PUT empty windows → 200, GET returns empty list.
	// -------------------------------------------------------------------------
	emptyPut, emptyPutResp, err := c.PutAvailability(token, therapistID, []harness.AvailabilityWindow{})
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, emptyPutResp.StatusCode, "PUT empty windows must return 200")
	assert.Equal(t, therapistID, emptyPut.TherapistID)
	assert.Empty(t, emptyPut.Windows, "empty PUT must result in empty windows")

	gotEmpty, getEmptyResp, err := c.GetAvailability(token, therapistID)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, getEmptyResp.StatusCode)
	assert.Empty(t, gotEmpty.Windows, "GET after empty PUT must return empty windows array")

	// -------------------------------------------------------------------------
	// Step 2: PUT 3 valid windows → 200.
	// dow: 1=Monday, 2=Tuesday
	// -------------------------------------------------------------------------
	windows3 := []harness.AvailabilityWindow{
		{DOW: 1, Start: "09:00", End: "12:00"}, // Monday morning
		{DOW: 1, Start: "13:00", End: "17:00"}, // Monday afternoon
		{DOW: 2, Start: "09:00", End: "17:00"}, // Tuesday full day
	}
	put3, put3Resp, err := c.PutAvailability(token, therapistID, windows3)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, put3Resp.StatusCode, "PUT 3 valid windows must return 200")
	assert.Len(t, put3.Windows, 3, "response must contain 3 windows")

	// -------------------------------------------------------------------------
	// Step 3: GET → returns the 3 windows.
	// The contract says: sorted by dow ASC, then start ASC.
	// -------------------------------------------------------------------------
	got3, get3Resp, err := c.GetAvailability(token, therapistID)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, get3Resp.StatusCode)
	require.Len(t, got3.Windows, 3, "GET must return 3 windows after PUT 3")

	// Verify sort order: [Mon 09:00, Mon 13:00, Tue 09:00]
	assert.Equal(t, 1, got3.Windows[0].DOW)
	assert.Equal(t, "09:00", got3.Windows[0].Start)
	assert.Equal(t, 1, got3.Windows[1].DOW)
	assert.Equal(t, "13:00", got3.Windows[1].Start)
	assert.Equal(t, 2, got3.Windows[2].DOW)
	assert.Equal(t, "09:00", got3.Windows[2].Start)

	// All windows must have non-empty IDs (needed by Phase 5 booking engine).
	for i, w := range got3.Windows {
		assert.NotEmpty(t, w.ID, "window %d must have a non-empty ID in GET response", i)
	}

	// -------------------------------------------------------------------------
	// Validation tests — all must fail cleanly without mutating the DB state.
	// After each failure, GET must still return the 3 windows from Step 2.
	// -------------------------------------------------------------------------

	// T12-V1: Overlapping same-day windows → 409 AVAILABILITY_OVERLAP
	//   Mon 09:00–12:00 and Mon 11:00–13:00 overlap from 11:00 to 12:00.
	_, overlapResp, rawOverlap, err := putAvailRaw(c, token, therapistID,
		[]harness.AvailabilityWindow{
			{DOW: 1, Start: "09:00", End: "12:00"},
			{DOW: 1, Start: "11:00", End: "13:00"},
		})
	require.NoError(t, err)
	assert.Equal(t, http.StatusConflict, overlapResp.StatusCode,
		"overlapping windows must return 409")
	assert.Equal(t, "AVAILABILITY_OVERLAP", harness.ErrorCode(rawOverlap),
		"error code must be AVAILABILITY_OVERLAP for overlapping windows")

	// T12-V2: end <= start → 400 VALIDATION
	_, endBeforeStartResp, rawEndBefore, err := putAvailRaw(c, token, therapistID,
		[]harness.AvailabilityWindow{
			{DOW: 1, Start: "17:00", End: "09:00"},
		})
	require.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, endBeforeStartResp.StatusCode,
		"end <= start must return 400")
	assert.Equal(t, "VALIDATION", harness.ErrorCode(rawEndBefore))

	// T12-V3: dow=7 out of range → 400 VALIDATION
	_, invalidDOWResp, rawInvalidDOW, err := putAvailRaw(c, token, therapistID,
		[]harness.AvailabilityWindow{
			{DOW: 7, Start: "09:00", End: "17:00"},
		})
	require.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, invalidDOWResp.StatusCode,
		"dow=7 must return 400 VALIDATION")
	assert.Equal(t, "VALIDATION", harness.ErrorCode(rawInvalidDOW))

	// T12-V4: Non-5-minute boundary (09:03) → 400 VALIDATION
	_, nonFiveResp, rawNonFive, err := putAvailRaw(c, token, therapistID,
		[]harness.AvailabilityWindow{
			{DOW: 1, Start: "09:03", End: "12:00"},
		})
	require.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, nonFiveResp.StatusCode,
		"non-5-minute boundary must return 400 VALIDATION")
	assert.Equal(t, "VALIDATION", harness.ErrorCode(rawNonFive))

	// -------------------------------------------------------------------------
	// Confirm DB state unchanged after all validation failures:
	// GET must still return the 3 windows from Step 2.
	// -------------------------------------------------------------------------
	gotAfterErrors, _, err := c.GetAvailability(token, therapistID)
	require.NoError(t, err)
	assert.Len(t, gotAfterErrors.Windows, 3,
		"availability must be unchanged after all validation-failure PUTs")
}

// putAvailRaw calls PUT availability and returns raw response bytes.
func putAvailRaw(c *harness.APIClient, token, therapistID string, windows []harness.AvailabilityWindow) (harness.AvailabilityResponse, *http.Response, []byte, error) {
	resp, raw, err := c.RawDo("PUT",
		"/api/v1/tenant/therapists/"+therapistID+"/availability",
		harness.PutAvailabilityRequest{Windows: windows},
		token)
	if err != nil {
		return harness.AvailabilityResponse{}, nil, nil, err
	}
	var out harness.AvailabilityResponse
	_ = harness.DecodeJSONPublic(raw, &out)
	return out, resp, raw, nil
}
