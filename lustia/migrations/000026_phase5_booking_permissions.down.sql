-- =============================================================================
-- Migration: 000026_phase5_booking_permissions (DOWN)
-- Purpose  : Remove booking.* permissions and their role assignments.
--
-- role_permission rows are deleted first (FK references permission.id).
-- =============================================================================

DELETE FROM role_permission
WHERE permission_id IN (
    SELECT id FROM permission
    WHERE code IN (
        'booking.read', 'booking.create', 'booking.cancel',
        'booking.checkin', 'booking.complete', 'booking.no_show'
    )
);

DELETE FROM permission
WHERE code IN (
    'booking.read', 'booking.create', 'booking.cancel',
    'booking.checkin', 'booking.complete', 'booking.no_show'
);
