-- =============================================================================
-- Migration: 000035_therapist_prep_minutes (UP)
-- Purpose  : Add therapist.prep_minutes — the cleanup/reset buffer (in minutes)
--            the therapist needs after each completed booking before the next
--            booking can begin.
--
--            Semantics: virtual only — the booking row's scheduled_end is NOT
--            extended. The booking engine expands the booked slot's end by
--            prep_minutes when checking whether a candidate slot is free (both
--            ListAvailableSlots and CreatePublic/CreateConcierge conflict paths).
--
--            Range 0–60. DEFAULT 10 satisfies the CHECK constraint immediately,
--            so no backfill UPDATE is needed; all existing rows inherit 10 via
--            the column DEFAULT.
--
--            Constraint name: therapist_prep_minutes_check (explicit, per
--            project convention — avoids reliance on Postgres auto-naming).
--
-- Cross-agent:
--   go-expert  — (1) Add `PrepMinutes int` to model.Therapist with GORM tag
--                    `gorm:"column:prep_minutes;not null;default:10"`.
--                (2) Update ListAvailableSlots conflict check: expand each
--                    booked slot's effective end by therapist.PrepMinutes when
--                    testing whether a candidate window overlaps.
--                (3) Same expansion in CreatePublic / CreateConcierge conflict
--                    guard (the GiST exclusion constraint covers exact
--                    scheduled_end; the prep buffer is a service-layer concern
--                    only).
--                (4) Include prep_minutes in GET /admin/therapists/:id response
--                    and accept it on PATCH /admin/therapists/:id.
--   nextjs-expert — Add a numeric input (0–60, step 1, label "Prep time
--                   (minutes)") to the tenant-admin therapist editor screen.
--
-- Reference: docs/DATA_MODEL.md §therapist table
-- =============================================================================

ALTER TABLE therapist
    ADD COLUMN IF NOT EXISTS prep_minutes INT NOT NULL DEFAULT 10;

ALTER TABLE therapist
    ADD CONSTRAINT therapist_prep_minutes_check
        CHECK (prep_minutes BETWEEN 0 AND 60);

COMMENT ON COLUMN therapist.prep_minutes IS
    'Cleanup / reset buffer in minutes the therapist needs after a completed '
    'booking before the next booking may start. Range 0–60, default 10. '
    'Virtual only: the booking row''s scheduled_end is never extended — the '
    'booking engine applies this buffer only when checking slot availability.';
