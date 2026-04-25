-- =============================================================================
-- Migration: 000019_seed_dev_addons (UP)
-- Purpose  : Insert sample tenant-wide add-ons for the acme-spa dev tenant.
--
-- !! DEV-ONLY — DO NOT APPLY IN STAGING OR PRODUCTION !!
--
-- Depends on migration 000010 (acme-spa tenant) and 000018 (addon table).
-- In staging/prod stop the migration runner at step 18:
--
--   migrate -database "$DATABASE_URL" -path ./migrations up 18
--
-- Or omit this file from the production image.
--
-- IDEMPOTENCY:
--   Every INSERT uses ON CONFLICT (id) DO NOTHING.
--   Re-running produces no error. Fixed UUIDs guarantee stable rows on every
--   fresh dev DB.
--
-- FIXED UUIDs  (prefix f0000000-0000-0000-0005-* = Phase 4 add-on rows):
--
--   Acme-spa tenant  : d0000000-0000-0000-0001-000000000001  (migration 000010)
--
--   Addons:
--     Aromaterapi Premium : f0000000-0000-0000-0005-000000000001
--     Handuk Panas        : f0000000-0000-0000-0005-000000000002
--     Jus Segar           : f0000000-0000-0000-0005-000000000003
--     Kerokan             : f0000000-0000-0000-0005-000000000004
--
-- NOTE: app.current_tenant must be set before DML when RLS is enforced.
-- This migration runs as lustia_migrator (BYPASSRLS), so RLS is not in effect
-- and no SET LOCAL is required.
-- =============================================================================

-- We must set app.current_tenant so that the RLS trigger on INSERT passes
-- WITH CHECK (even though lustia_migrator has BYPASSRLS, being explicit here
-- mirrors the pattern used in migration 000014 and avoids surprises if the
-- BYPASSRLS assumption ever changes).
SET LOCAL app.current_tenant = 'd0000000-0000-0000-0001-000000000001';

INSERT INTO addon (id, tenant_id, name, description, price_idr, is_active, sort_order, created_at, updated_at)
VALUES
    (
        'f0000000-0000-0000-0005-000000000001',
        'd0000000-0000-0000-0001-000000000001',
        'Aromaterapi Premium',
        'Minyak esensial lavender atau eucalyptus pilihan. Meningkatkan relaksasi selama sesi.',
        35000,
        true,
        0,
        now(), now()
    ),
    (
        'f0000000-0000-0000-0005-000000000002',
        'd0000000-0000-0000-0001-000000000001',
        'Handuk Panas',
        'Handuk hangat untuk mengendurkan otot sebelum dan sesudah sesi.',
        15000,
        true,
        1,
        now(), now()
    ),
    (
        'f0000000-0000-0000-0005-000000000003',
        'd0000000-0000-0000-0001-000000000001',
        'Jus Segar',
        'Segelas jus buah segar pilihan (jeruk, semangka, atau jambu).',
        20000,
        true,
        2,
        now(), now()
    ),
    (
        'f0000000-0000-0000-0005-000000000004',
        'd0000000-0000-0000-0001-000000000001',
        'Kerokan',
        'Teknik kerokan tradisional dengan batu giok. Efektif untuk masuk angin.',
        25000,
        true,
        3,
        now(), now()
    )
ON CONFLICT (id) DO NOTHING;
