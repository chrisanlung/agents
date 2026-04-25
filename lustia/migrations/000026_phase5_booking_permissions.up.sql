-- =============================================================================
-- Migration: 000026_phase5_booking_permissions (UP)
-- Purpose  : Phase 5 — Insert booking.* permission codes and wire them to roles.
--            Implements ADR 0014 §3.15 (permissions matrix).
--
-- UUID namespace: c0000000-0000-0000-0026-*
-- Verified ZERO matches in lustia/migrations/ before writing this file.
-- The 0026 suffix matches the migration number for traceability.
--
-- Permissions added:
--   booking.read     — list and view bookings
--   booking.create   — create a booking (concierge / ops flow)
--   booking.cancel   — force-cancel a booking with reason
--   booking.checkin  — scan QR / enter code to check in a customer
--   booking.complete — mark a booking as completed
--   booking.no_show  — flag a booking as no-show
--
-- Role wiring:
--   super_admin  : all six permissions
--   tenant_admin : all six permissions
--   branch_admin : all six permissions (branch-scope restriction is service-layer)
--   therapist    : booking.read only (sees own-branch schedule in ops portal)
--
-- NOTE: The customer role is not applicable — customer-facing booking creation
-- uses the public endpoint (no JWT). The permission table entries are for
-- internal operator access only (ADR 0014 §3.15).
--
-- CLEAN invariant: migrations 1 → 26 produce a working schema.
--
-- Re-run safety:
--   INSERT uses ON CONFLICT (id) DO NOTHING on permission rows.
--   INSERT uses ON CONFLICT DO NOTHING on role_permission rows
--   (composite unique key: role_id, permission_id).
-- =============================================================================

INSERT INTO permission (id, code, description, created_at, updated_at)
VALUES
    ('c0000000-0000-0000-0026-000000000001', 'booking.read',     'List and view bookings',                      now(), now()),
    ('c0000000-0000-0000-0026-000000000002', 'booking.create',   'Create a booking (concierge / ops flow)',      now(), now()),
    ('c0000000-0000-0000-0026-000000000003', 'booking.cancel',   'Force-cancel a booking with reason',           now(), now()),
    ('c0000000-0000-0000-0026-000000000004', 'booking.checkin',  'Check in a customer by QR scan or code input', now(), now()),
    ('c0000000-0000-0000-0026-000000000005', 'booking.complete', 'Mark a booking as completed',                  now(), now()),
    ('c0000000-0000-0000-0026-000000000006', 'booking.no_show',  'Flag a booking as no-show',                    now(), now())
-- Phase 1 migration 000003 already seeded booking.read / create / update /
-- cancel / checkin / complete with IDs in the c0000000-0000-0000-0010-* prefix.
-- Their UUIDs differ from this migration's 0026-* prefix, but the `code` column
-- has a UNIQUE index, so we conflict-resolve on `code`. New rows that don't yet
-- exist (booking.no_show) still get inserted; the role_permission inserts below
-- pick up the existing 0010-* IDs via the WHERE p.code IN (...) lookup.
ON CONFLICT (code) DO NOTHING;

-- super_admin: all six permissions
INSERT INTO role_permission (role_id, permission_id)
SELECT r.id, p.id
FROM role r
CROSS JOIN permission p
WHERE r.name = 'super_admin'
  AND p.code IN (
      'booking.read', 'booking.create', 'booking.cancel',
      'booking.checkin', 'booking.complete', 'booking.no_show'
  )
ON CONFLICT DO NOTHING;

-- tenant_admin: all six permissions (cross-branch visibility)
INSERT INTO role_permission (role_id, permission_id)
SELECT r.id, p.id
FROM role r
CROSS JOIN permission p
WHERE r.name = 'tenant_admin'
  AND p.code IN (
      'booking.read', 'booking.create', 'booking.cancel',
      'booking.checkin', 'booking.complete', 'booking.no_show'
  )
ON CONFLICT DO NOTHING;

-- branch_admin: all six permissions; branch-scope restriction enforced by
-- BookingService at the application layer (mirrors TherapistService / RoomService
-- pattern from Phase 4).
INSERT INTO role_permission (role_id, permission_id)
SELECT r.id, p.id
FROM role r
CROSS JOIN permission p
WHERE r.name = 'branch_admin'
  AND p.code IN (
      'booking.read', 'booking.create', 'booking.cancel',
      'booking.checkin', 'booking.complete', 'booking.no_show'
  )
ON CONFLICT DO NOTHING;

-- therapist: booking.read only — can view own-branch schedule in the ops portal.
-- The application layer filters to bookings where therapist_id = caller's therapist.
INSERT INTO role_permission (role_id, permission_id)
SELECT r.id, p.id
FROM role r
CROSS JOIN permission p
WHERE r.name = 'therapist'
  AND p.code IN ('booking.read')
ON CONFLICT DO NOTHING;
