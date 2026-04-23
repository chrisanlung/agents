-- =============================================================================
-- Migration: 000012_seed_dev_registrations (UP)
--
-- !! DEV-ONLY — DO NOT APPLY IN STAGING OR PRODUCTION !!
--
-- This migration inserts a sample pending company registration so that the
-- platform-admin UI approval queue can be exercised without posting to the
-- public registration endpoint on every fresh dev DB.
--
-- In staging/prod, stop the migration runner at step 11:
--
--   migrate -database "$DATABASE_URL" -path ./migrations up 11
--
-- Or simply omit this file from the production image. The main chain (1–11)
-- produces a working system on any environment; only the sample registration
-- row is dev-only.
--
-- IDEMPOTENCY:
--   INSERT uses ON CONFLICT (id) DO NOTHING. Re-running is safe and produces
--   no error. The fixed UUID ensures the same row on every fresh DB.
--
-- FIXED UUID:
--   Registration (zen-wellness) : e0000000-0000-0000-0001-000000000001
-- =============================================================================

-- ---------------------------------------------------------------------------
-- 1. Sample pending registration: Zen Wellness
-- ---------------------------------------------------------------------------
INSERT INTO tenant_registration (
    id,
    company_name,
    requested_slug,
    package,
    contact_name,
    contact_email,
    contact_phone,
    status,
    metadata,
    created_at,
    updated_at
)
VALUES (
    'e0000000-0000-0000-0001-000000000001',
    'Zen Wellness',
    'zen-wellness',
    'starter',
    'Zara Zhang',
    'founder@zen-wellness.example',
    NULL,
    'pending',
    '{"source": "dev_seed", "note": "Sample registration for UI testing"}'::jsonb,
    now(),
    now()
)
ON CONFLICT (id) DO NOTHING;
