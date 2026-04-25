-- =============================================================================
-- Migration: 000022_seed_dev_rooms (DOWN)
-- Purpose  : Remove the dev-only room seed rows inserted by migration 000022.
--
-- Deletes by fixed UUID — safe because these rows are only ever present on
-- dev environments (see migration 000022 header). No FK children exist for
-- room rows in Phase 4 (booking.room_id is a Phase 5 addition).
-- =============================================================================

DELETE FROM room
WHERE id IN (
    'e0000000-0000-0000-0021-000000000001',
    'e0000000-0000-0000-0021-000000000002',
    'e0000000-0000-0000-0021-000000000003'
);
