package service_test

import (
	"context"
	"errors"
	"testing"

	"github.com/chrisanlung/lustia-auth/internal/constants"
	"github.com/chrisanlung/lustia-auth/internal/model"
	"github.com/chrisanlung/lustia-auth/internal/service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// Fakes
// ---------------------------------------------------------------------------

type stubAvailabilityRepo struct {
	rows     []*model.TherapistAvailability
	replaced []([]*model.TherapistAvailability)
}

func (r *stubAvailabilityRepo) FindByTherapistID(_ context.Context, _ string) ([]*model.TherapistAvailability, error) {
	return r.rows, nil
}
func (r *stubAvailabilityRepo) ReplaceAllForTherapist(_ context.Context, _ string, rows []*model.TherapistAvailability) error {
	r.replaced = append(r.replaced, rows)
	r.rows = rows
	return nil
}

func newTestAvailabilitySvc(therapistRepo *stubTherapistRepo, availRepo *stubAvailabilityRepo) *service.AvailabilitySvc {
	return service.NewAvailabilitySvc(availRepo, therapistRepo, &fakeAuditRepo{}, &stubClockForBranch{})
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

func TestAvailabilityReplace_Success(t *testing.T) {
	t.Parallel()

	therapistRepo := newStubTherapistRepo()
	therapistRepo.rows["th1"] = &model.Therapist{ID: "th1", TenantID: "t1", BranchID: "b1", IsActive: true}
	availRepo := &stubAvailabilityRepo{}

	svc := newTestAvailabilitySvc(therapistRepo, availRepo)

	out, err := svc.Replace(context.Background(), service.ReplaceAvailabilityInput{
		TherapistID:    "th1",
		CallerUserID:   "u1",
		CallerTenantID: "t1",
		IsAdmin:        true,
		Windows: []service.ReplaceAvailabilityWindow{
			{DOW: 1, Start: "09:00", End: "12:00"},
			{DOW: 1, Start: "14:00", End: "17:00"},
			{DOW: 3, Start: "09:00", End: "17:00"},
		},
	})

	require.NoError(t, err)
	assert.Equal(t, "th1", out.TherapistID)
	assert.Len(t, out.Windows, 3)
	require.Len(t, availRepo.replaced, 1)
	assert.Len(t, availRepo.replaced[0], 3)
}

func TestAvailabilityReplace_OverlapSameDay(t *testing.T) {
	t.Parallel()

	therapistRepo := newStubTherapistRepo()
	therapistRepo.rows["th1"] = &model.Therapist{ID: "th1", TenantID: "t1", BranchID: "b1", IsActive: true}
	availRepo := &stubAvailabilityRepo{}

	svc := newTestAvailabilitySvc(therapistRepo, availRepo)

	// 09:00–12:00 and 11:00–14:00 overlap.
	_, err := svc.Replace(context.Background(), service.ReplaceAvailabilityInput{
		TherapistID:    "th1",
		CallerUserID:   "u1",
		CallerTenantID: "t1",
		IsAdmin:        true,
		Windows: []service.ReplaceAvailabilityWindow{
			{DOW: 1, Start: "09:00", End: "12:00"},
			{DOW: 1, Start: "11:00", End: "14:00"},
		},
	})

	require.Error(t, err)
	assert.True(t, errors.Is(err, constants.ErrAvailabilityOverlap))
	assert.Empty(t, availRepo.replaced)
}

func TestAvailabilityReplace_EndBeforeStart(t *testing.T) {
	t.Parallel()

	therapistRepo := newStubTherapistRepo()
	therapistRepo.rows["th1"] = &model.Therapist{ID: "th1", TenantID: "t1", BranchID: "b1", IsActive: true}
	availRepo := &stubAvailabilityRepo{}

	svc := newTestAvailabilitySvc(therapistRepo, availRepo)

	_, err := svc.Replace(context.Background(), service.ReplaceAvailabilityInput{
		TherapistID:    "th1",
		CallerUserID:   "u1",
		CallerTenantID: "t1",
		IsAdmin:        true,
		Windows: []service.ReplaceAvailabilityWindow{
			{DOW: 2, Start: "17:00", End: "09:00"}, // end before start
		},
	})

	require.Error(t, err)
	assert.True(t, errors.Is(err, constants.ErrInvalidInput))
}

func TestAvailabilityReplace_InvalidDayOfWeek(t *testing.T) {
	t.Parallel()

	therapistRepo := newStubTherapistRepo()
	therapistRepo.rows["th1"] = &model.Therapist{ID: "th1", TenantID: "t1", BranchID: "b1", IsActive: true}
	availRepo := &stubAvailabilityRepo{}

	svc := newTestAvailabilitySvc(therapistRepo, availRepo)

	_, err := svc.Replace(context.Background(), service.ReplaceAvailabilityInput{
		TherapistID:    "th1",
		CallerUserID:   "u1",
		CallerTenantID: "t1",
		IsAdmin:        true,
		Windows: []service.ReplaceAvailabilityWindow{
			{DOW: 7, Start: "09:00", End: "17:00"}, // 7 is out of range 0–6
		},
	})

	require.Error(t, err)
	assert.True(t, errors.Is(err, constants.ErrInvalidInput))
}

func TestAvailabilityReplace_NonFiveMinuteBoundary(t *testing.T) {
	t.Parallel()

	therapistRepo := newStubTherapistRepo()
	therapistRepo.rows["th1"] = &model.Therapist{ID: "th1", TenantID: "t1", BranchID: "b1", IsActive: true}
	availRepo := &stubAvailabilityRepo{}

	svc := newTestAvailabilitySvc(therapistRepo, availRepo)

	_, err := svc.Replace(context.Background(), service.ReplaceAvailabilityInput{
		TherapistID:    "th1",
		CallerUserID:   "u1",
		CallerTenantID: "t1",
		IsAdmin:        true,
		Windows: []service.ReplaceAvailabilityWindow{
			{DOW: 1, Start: "09:03", End: "12:00"}, // 03 is not a 5-minute boundary
		},
	})

	require.Error(t, err)
	assert.True(t, errors.Is(err, constants.ErrInvalidInput))
}

func TestAvailabilityReplace_EmptyWindowsClearsAll(t *testing.T) {
	t.Parallel()

	therapistRepo := newStubTherapistRepo()
	therapistRepo.rows["th1"] = &model.Therapist{ID: "th1", TenantID: "t1", BranchID: "b1", IsActive: true}
	availRepo := &stubAvailabilityRepo{}

	svc := newTestAvailabilitySvc(therapistRepo, availRepo)

	out, err := svc.Replace(context.Background(), service.ReplaceAvailabilityInput{
		TherapistID:    "th1",
		CallerUserID:   "u1",
		CallerTenantID: "t1",
		IsAdmin:        true,
		Windows:        []service.ReplaceAvailabilityWindow{},
	})

	require.NoError(t, err)
	assert.Empty(t, out.Windows)
}

func TestAvailabilityReplace_CrossBranchForbidden(t *testing.T) {
	t.Parallel()

	therapistRepo := newStubTherapistRepo()
	therapistRepo.rows["th1"] = &model.Therapist{ID: "th1", TenantID: "t1", BranchID: "b2", IsActive: true}
	availRepo := &stubAvailabilityRepo{}

	svc := newTestAvailabilitySvc(therapistRepo, availRepo)

	_, err := svc.Replace(context.Background(), service.ReplaceAvailabilityInput{
		TherapistID:    "th1",
		CallerUserID:   "u1",
		CallerTenantID: "t1",
		CallerBranches: []string{"b1"}, // b1 only; therapist is at b2
		IsAdmin:        false,
		Windows: []service.ReplaceAvailabilityWindow{
			{DOW: 1, Start: "09:00", End: "17:00"},
		},
	})

	require.Error(t, err)
	assert.True(t, errors.Is(err, constants.ErrCrossBranchForbidden))
}
