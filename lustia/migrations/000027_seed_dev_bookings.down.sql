-- =============================================================================
-- Migration: 000027_seed_dev_bookings (DOWN)
-- Purpose  : Remove dev seed booking rows.
--
-- booking_addon rows cascade-delete automatically when booking rows are deleted
-- (ON DELETE CASCADE on fk_booking_addon_booking). Delete booking rows directly.
-- =============================================================================

SET LOCAL app.current_tenant = 'd0000000-0000-0000-0001-000000000001';

DELETE FROM booking
WHERE id IN (
    'b0000000-0000-0000-0027-000000000001',
    'b0000000-0000-0000-0027-000000000002',
    'b0000000-0000-0000-0027-000000000003',
    'b0000000-0000-0000-0027-000000000004',
    'b0000000-0000-0000-0027-000000000005'
);
