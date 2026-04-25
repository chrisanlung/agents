-- =============================================================================
-- Migration: 000022_seed_dev_rooms (UP)
-- Purpose  : Insert sample rooms for the acme-spa dev tenant (Cabang Utama).
--
-- !! DEV-ONLY — DO NOT APPLY IN STAGING OR PRODUCTION !!
--
-- Depends on:
--   migration 000010  — acme-spa tenant  (d0000000-0000-0000-0001-000000000001)
--   migration 000014  — Cabang Utama     (f0000000-0000-0000-0001-000000000001)
--   migration 000021  — room table
--
-- In staging/prod stop the migration runner at step 21:
--
--   migrate -database "$DATABASE_URL" -path ./migrations up 21
--
-- Or omit this file from the production image.
--
-- IDEMPOTENCY:
--   Every INSERT uses ON CONFLICT (id) DO NOTHING.
--   Re-running produces no error. Fixed UUIDs guarantee stable rows on every
--   fresh dev DB.
--
-- WHY SEED ROOMS NOW:
--   The Phase 5 booking engine will need at least one room per type to develop
--   and test the booking-time allocation logic. Three rooms covering the most
--   common types (vip, couple, single) are sufficient for Phase 5 development
--   without over-seeding.
--
-- FIXED UUIDs (prefix e0000000-0000-0000-0021-* = Phase 4 room seed rows):
--
--   Rooms (branch-scoped, Cabang Utama):
--     VIP 1      (vip,    cap=2) : e0000000-0000-0000-0021-000000000001
--     Couple A   (couple, cap=2) : e0000000-0000-0000-0021-000000000002
--     Single 1   (single, cap=1) : e0000000-0000-0000-0021-000000000003
--
-- NOTE: This migration runs as lustia_migrator (BYPASSRLS). The SET LOCAL
-- for app.current_tenant is included as a best-practice guard — consistent
-- with migrations 000014 and 000019.
-- =============================================================================

SET LOCAL app.current_tenant = 'd0000000-0000-0000-0001-000000000001';

INSERT INTO room (
    id,
    tenant_id,
    branch_id,
    name,
    description,
    room_type,
    capacity,
    amenities,
    photo_key,
    is_active,
    sort_order,
    created_at,
    updated_at
)
VALUES
    (
        'e0000000-0000-0000-0021-000000000001',
        'd0000000-0000-0000-0001-000000000001',
        'f0000000-0000-0000-0001-000000000001',
        'VIP 1',
        'Ruangan VIP dengan fasilitas lengkap. Dilengkapi tempat tidur premium, aromaterapi, dan musik relaksasi.',
        'vip',
        2,
        ARRAY['shower', 'aromaterapi', 'tv', 'locker'],
        NULL,
        true,
        0,
        now(), now()
    ),
    (
        'e0000000-0000-0000-0021-000000000002',
        'd0000000-0000-0000-0001-000000000001',
        'f0000000-0000-0000-0001-000000000001',
        'Couple A',
        'Ruangan couple dengan dua bed berdampingan. Cocok untuk pasangan yang ingin sesi bersamaan.',
        'couple',
        2,
        ARRAY['aromaterapi', 'musik relaksasi'],
        NULL,
        true,
        1,
        now(), now()
    ),
    (
        'e0000000-0000-0000-0021-000000000003',
        'd0000000-0000-0000-0001-000000000001',
        'f0000000-0000-0000-0001-000000000001',
        'Single 1',
        'Ruangan standar untuk sesi individual.',
        'single',
        1,
        ARRAY['locker'],
        NULL,
        true,
        2,
        now(), now()
    )
ON CONFLICT (id) DO NOTHING;
