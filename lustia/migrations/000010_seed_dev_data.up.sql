-- =============================================================================
-- Migration: 000010_seed_dev_data (UP)
-- Purpose  : Insert dev-only reference data — the acme-spa tenant and a
--            sample tenant_admin user (alice@acme-spa.example).
--
-- ⚠  DEV-ONLY — DO NOT APPLY IN STAGING OR PRODUCTION ⚠
--
-- This migration is intentionally separated from the main chain. In dev,
-- apply the full chain (migrations 1–10). In staging/prod, stop at migration
-- 9:
--
--   migrate -database "$DATABASE_URL" -path ./migrations up 9
--
-- Or simply do not ship this file to production images. The migration runner
-- (golang-migrate) applies files in numeric order; omitting file 000010 from
-- the deployment artifact is sufficient.
--
-- The main chain (1–9) produces a working system on any environment; only
-- the sample tenant + sample user are dev-only.
--
-- IDEMPOTENCY:
--   Every INSERT uses ON CONFLICT DO NOTHING. Re-running this migration is
--   safe and produces no error. Fixed UUIDs are used throughout so the same
--   rows are produced on every fresh DB.
--
-- FIXED UUIDs:
--   Tenant acme-spa    : d0000000-0000-0000-0001-000000000001
--   User alice         : d0000000-0000-0000-0002-000000000001
--   Membership (alice) : d0000000-0000-0000-0003-000000000001
--
-- PASSWORD HASH:
--   alice's password is  Staff2026!
--   Argon2id hash (generated externally):
--   $argon2id$v=19$m=65536,t=3,p=4$c29tZXNhbHR2YWx1ZQ$eFObR2rHflYL+mAl8wLlHIJpofxNMtSxXIv8HxmPb2A
--
--   To regenerate: go run ./cmd/argon2hash 'Staff2026!'
--   (or any Argon2id tool with m=65536, t=3, p=4)
--
--   This hash is stored as a CONSTANT in the migration — we do NOT generate
--   hashes inside SQL (no pg_crypto bcrypt, no functions). The Go CLI is the
--   canonical tool for Argon2id hash generation in this project.
-- =============================================================================

-- ---------------------------------------------------------------------------
-- 1. Tenant: acme-spa
-- ---------------------------------------------------------------------------
INSERT INTO tenant (
    id,
    name,
    slug,
    status,
    plan,
    contact_name,
    contact_email,
    metadata,
    created_at,
    updated_at
)
VALUES (
    'd0000000-0000-0000-0001-000000000001',
    'Acme Spa',
    'acme-spa',
    'active',
    'starter',
    'Alice Smith',
    'contact@acme-spa.example',
    '{}'::jsonb,
    now(),
    now()
)
ON CONFLICT (id) DO NOTHING;

-- ---------------------------------------------------------------------------
-- 2. User: alice@acme-spa.example
--
--    is_super_admin = false (a tenant admin, not a platform admin).
--    must_change_password = false (dev convenience — she can log in directly).
--    password = Staff2026!
-- ---------------------------------------------------------------------------
INSERT INTO "user" (
    id,
    email,
    password_hash,
    full_name,
    is_active,
    is_super_admin,
    must_change_password,
    metadata,
    created_at,
    updated_at
)
VALUES (
    'd0000000-0000-0000-0002-000000000001',
    'alice@acme-spa.example',
    -- Argon2id hash of 'Staff2026!' produced by cmd/hashpw (SECURITY.md §2.1
    -- params: m=65536, t=3, p=2). Regenerate with:
    --   SUPER_ADMIN_INITIAL_PASSWORD='Staff2026!' go run ./cmd/hashpw
    '$argon2id$v=19$m=65536,t=3,p=2$3doOnTlDFuP+XpHW/oZwkQ$U7BW1jdRf684fEEJN3m8NxbMrBoKSMjU8HyMpfeRF+4',
    'Alice Smith',
    true,
    false,
    false,
    '{}'::jsonb,
    now(),
    now()
)
ON CONFLICT (id) DO NOTHING;

-- ---------------------------------------------------------------------------
-- 3. Membership: alice ↔ acme-spa (active)
-- ---------------------------------------------------------------------------
INSERT INTO membership (
    id,
    user_id,
    tenant_id,
    status,
    joined_at,
    metadata,
    created_at,
    updated_at
)
VALUES (
    'd0000000-0000-0000-0003-000000000001',
    'd0000000-0000-0000-0002-000000000001',  -- alice
    'd0000000-0000-0000-0001-000000000001',  -- acme-spa
    'active',
    now(),
    '{}'::jsonb,
    now(),
    now()
)
ON CONFLICT (id) DO NOTHING;

-- ---------------------------------------------------------------------------
-- 4. user_role: assign tenant_admin role to alice's membership
--
--    Role UUID b0000000-0000-0000-0000-000000000002 = 'tenant_admin'
--    (seeded by migration 000005, fixed UUID).
-- ---------------------------------------------------------------------------
INSERT INTO user_role (
    membership_id,
    role_id,
    assigned_at
)
VALUES (
    'd0000000-0000-0000-0003-000000000001',  -- alice's membership
    'b0000000-0000-0000-0000-000000000002',  -- tenant_admin role
    now()
)
ON CONFLICT DO NOTHING;
