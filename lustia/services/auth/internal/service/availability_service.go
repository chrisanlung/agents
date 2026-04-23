package service

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/chrisanlung/lustia-auth/internal/constants"
	"github.com/chrisanlung/lustia-auth/internal/model"
	"github.com/google/uuid"
)

// AvailabilitySvc handles per-therapist weekly availability reads and full-replace
// writes per ADR 0009 §11.7.
type AvailabilitySvc struct {
	availability TherapistAvailabilityRepository
	therapists   TherapistRepository
	audit        AuditRepository
	clock        Clock
}

// NewAvailabilitySvc constructs an AvailabilitySvc.
func NewAvailabilitySvc(
	availability TherapistAvailabilityRepository,
	therapists TherapistRepository,
	audit AuditRepository,
	clock Clock,
) *AvailabilitySvc {
	return &AvailabilitySvc{
		availability: availability,
		therapists:   therapists,
		audit:        audit,
		clock:        clock,
	}
}

// Get returns the therapist's current weekly availability pattern.
func (s *AvailabilitySvc) Get(ctx context.Context, callerTenantID, therapistID string) (AvailabilityOutput, error) {
	t, err := s.therapists.FindByID(ctx, therapistID)
	if err != nil {
		return AvailabilityOutput{}, err
	}
	if t.TenantID != callerTenantID {
		return AvailabilityOutput{}, constants.ErrTherapistNotFound
	}

	rows, err := s.availability.FindByTherapistID(ctx, therapistID)
	if err != nil {
		return AvailabilityOutput{}, fmt.Errorf("get availability: %w", err)
	}

	return AvailabilityOutput{
		TherapistID: therapistID,
		Windows:     toAvailabilityWindows(rows),
	}, nil
}

// Replace performs a full-replace of the therapist's weekly availability
// pattern per §11.7.2. Validates all windows before writing.
func (s *AvailabilitySvc) Replace(ctx context.Context, in ReplaceAvailabilityInput) (AvailabilityOutput, error) {
	t, err := s.therapists.FindByID(ctx, in.TherapistID)
	if err != nil {
		return AvailabilityOutput{}, err
	}
	if t.TenantID != in.CallerTenantID {
		return AvailabilityOutput{}, constants.ErrTherapistNotFound
	}
	if !in.IsAdmin {
		if !containsBranch(in.CallerBranches, t.BranchID) {
			return AvailabilityOutput{}, constants.ErrCrossBranchForbidden
		}
	}

	// Validate windows before any write.
	if err := validateAvailabilityWindows(in.Windows); err != nil {
		return AvailabilityOutput{}, err
	}

	rows := make([]*model.TherapistAvailability, 0, len(in.Windows))
	now := s.clock.Now().UTC()

	for _, w := range in.Windows {
		startT, endT, err := parseWindowTimes(w.Start, w.End)
		if err != nil {
			return AvailabilityOutput{}, err
		}
		// EffectiveFrom is NOT NULL in the DB with DEFAULT CURRENT_DATE; GORM
		// sends explicit NULL when the field is a nil pointer, overriding the
		// default. Set it to today so the insert satisfies the constraint.
		today := now
		_, _ = startT, endT // parsed for validation only; the model stores the
		// raw "HH:MM:SS" string because Postgres TIME returns a string driver value.
		rows = append(rows, &model.TherapistAvailability{
			ID:            uuid.New().String(),
			TenantID:      in.CallerTenantID,
			TherapistID:   in.TherapistID,
			BranchID:      t.BranchID,
			DayOfWeek:     w.DOW,
			StartTime:     w.Start + ":00", // "HH:MM" -> "HH:MM:SS"
			EndTime:       w.End + ":00",
			EffectiveFrom: &today,
			CreatedAt:     now,
			UpdatedAt:     now,
			CreatedBy:     &in.CallerUserID,
			UpdatedBy:     &in.CallerUserID,
		})
	}

	if err := s.availability.ReplaceAllForTherapist(ctx, in.TherapistID, rows); err != nil {
		return AvailabilityOutput{}, fmt.Errorf("replace availability: %w", err)
	}

	bgCtx := context.Background()
	_ = s.audit.Append(bgCtx, AuditEntry{
		TenantID:     &in.CallerTenantID,
		ActorUserID:  &in.CallerUserID,
		Action:       "therapist_availability.replaced",
		ResourceType: "therapist",
		ResourceID:   in.TherapistID,
		Meta:         map[string]interface{}{"window_count": len(rows)},
	})

	// Re-read from the new rows we just wrote (no DB round-trip needed).
	return AvailabilityOutput{
		TherapistID: in.TherapistID,
		Windows:     toAvailabilityWindows(rows),
	}, nil
}

// ---------------------------------------------------------------------------
// Validation
// ---------------------------------------------------------------------------

// validateAvailabilityWindows enforces the service-layer rules from §11.7.2.
func validateAvailabilityWindows(windows []ReplaceAvailabilityWindow) error {
	// Group by DOW for overlap and max-per-day checks.
	byDOW := make(map[int][]ReplaceAvailabilityWindow)
	for _, w := range windows {
		if w.DOW < 0 || w.DOW > 6 {
			return fmt.Errorf("%w: day_of_week must be 0–6", constants.ErrInvalidInput)
		}
		startT, endT, err := parseWindowTimes(w.Start, w.End)
		if err != nil {
			return err
		}
		if !endT.After(startT) {
			return fmt.Errorf("%w: end time must be after start time (got start=%s end=%s)", constants.ErrInvalidInput, w.Start, w.End)
		}
		// 5-minute boundary check.
		if startT.Minute()%5 != 0 || endT.Minute()%5 != 0 {
			return fmt.Errorf("%w: start and end times must be on 5-minute boundaries", constants.ErrInvalidInput)
		}
		byDOW[w.DOW] = append(byDOW[w.DOW], w)
	}

	for dow, ws := range byDOW {
		// Rule 3: max 3 windows per day.
		if len(ws) > 3 {
			return fmt.Errorf("%w: day %d has more than 3 availability windows", constants.ErrInvalidInput, dow)
		}

		// Rule 2: no overlapping windows on the same day (sort by start first).
		sort.Slice(ws, func(i, j int) bool {
			return ws[i].Start < ws[j].Start
		})
		for i := 1; i < len(ws); i++ {
			_, prevEnd, _ := parseWindowTimes(ws[i-1].Start, ws[i-1].End)
			currStart, _, _ := parseWindowTimes(ws[i].Start, ws[i].End)
			if currStart.Before(prevEnd) {
				return fmt.Errorf("%w: windows overlap on day %d (%s–%s and %s–%s)",
					constants.ErrAvailabilityOverlap, dow,
					ws[i-1].Start, ws[i-1].End, ws[i].Start, ws[i].End)
			}
		}
	}

	return nil
}

// parseWindowTimes parses "HH:MM" strings into time.Time values anchored to
// year 0001-01-01 in UTC so comparisons are valid regardless of wall-clock date.
func parseWindowTimes(start, end string) (startT, endT time.Time, err error) {
	startT, err = parseHHMM(start)
	if err != nil {
		return
	}
	endT, err = parseHHMM(end)
	return
}

// parseHHMM parses "HH:MM" into a time.Time anchored to 0001-01-01 UTC.
func parseHHMM(s string) (time.Time, error) {
	parts := strings.Split(s, ":")
	if len(parts) != 2 {
		return time.Time{}, fmt.Errorf("%w: invalid time format %q (expected HH:MM)", constants.ErrInvalidInput, s)
	}
	h, err := strconv.Atoi(parts[0])
	if err != nil || h < 0 || h > 23 {
		return time.Time{}, fmt.Errorf("%w: invalid hour in %q", constants.ErrInvalidInput, s)
	}
	m, err := strconv.Atoi(parts[1])
	if err != nil || m < 0 || m > 59 {
		return time.Time{}, fmt.Errorf("%w: invalid minute in %q", constants.ErrInvalidInput, s)
	}
	return time.Date(1, 1, 1, h, m, 0, 0, time.UTC), nil
}

// ---------------------------------------------------------------------------
// Mapping helpers
// ---------------------------------------------------------------------------

// toAvailabilityWindows converts model rows to AvailabilityWindow DTOs.
func toAvailabilityWindows(rows []*model.TherapistAvailability) []AvailabilityWindow {
	windows := make([]AvailabilityWindow, len(rows))
	for i, r := range rows {
		windows[i] = AvailabilityWindow{
			ID:    r.ID,
			DOW:   r.DayOfWeek,
			Start: r.StartHHMM(),
			End:   r.EndHHMM(),
		}
	}
	return windows
}
