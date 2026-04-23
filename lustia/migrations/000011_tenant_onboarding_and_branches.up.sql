-- =============================================================================
-- Migration: 000011_tenant_onboarding_and_branches (UP)
-- Purpose  : Phase 3 — Tenant Onboarding & Branch Setup.
--            Implements ADR 0008 §2.1 exactly:
--              - Add package/limits/approval/rejection metadata to `tenant`.
--              - Add timezone/activated_at to `branch` (address/contact columns
--                already exist from migration 000001; IF NOT EXISTS guards are
--                present on all ALTER TABLE ADD COLUMN statements for safety).
--              - Create tenant_registration_status enum + tenant_registration table
--                with indexes, RLS, and grants.
--              - Guard-insert any missing permission codes and role_permission rows
--                (all were seeded by migration 000005; these are no-ops on a
--                standard chain but satisfy the CLEAN invariant for edge cases).
--
-- Re-run safety: every destructive or creative step is guarded via IF NOT EXISTS,
-- DO $$ ... END $$ exception blocks, or ON CONFLICT DO NOTHING. Safe to re-run
-- after a partial failure.
--
-- Reference: docs/DECISIONS/0008-tenant-onboarding-and-branch-setup.md
-- =============================================================================

-- ---------------------------------------------------------------------------
-- PART 1: tenant — add package, limits, approval, and rejection columns
-- ---------------------------------------------------------------------------

ALTER TABLE tenant
    ADD COLUMN IF NOT EXISTS package TEXT NOT NULL DEFAULT 'starter'
        CHECK (package IN ('starter', 'growth', 'enterprise')),
    ADD COLUMN IF NOT EXISTS max_branches INT NOT NULL DEFAULT 1
        CHECK (max_branches >= 1),
    ADD COLUMN IF NOT EXISTS approved_at  TIMESTAMPTZ NULL,
    ADD COLUMN IF NOT EXISTS approved_by  UUID        NULL
        CONSTRAINT fk_tenant_approved_by
        REFERENCES "user"(id)
        ON DELETE SET NULL,
    ADD COLUMN IF NOT EXISTS rejected_at  TIMESTAMPTZ NULL,
    ADD COLUMN IF NOT EXISTS rejected_by  UUID        NULL
        CONSTRAINT fk_tenant_rejected_by
        REFERENCES "user"(id)
        ON DELETE SET NULL,
    ADD COLUMN IF NOT EXISTS rejection_reason TEXT NULL
        CHECK (rejection_reason IS NULL OR char_length(rejection_reason) <= 1000);

COMMENT ON COLUMN tenant.package IS
    'Subscription package selected at registration. Drives max_branches default. '
    'Values: starter (1 branch), growth (5), enterprise (999 sentinel = unlimited). '
    'Application code applies the package → max_branches matrix on approval.';

COMMENT ON COLUMN tenant.max_branches IS
    'Maximum active (non-deleted) branches this tenant may create. '
    'Sentinel 999 = unlimited (enterprise). Enforced by branch_service, not the DB.';

COMMENT ON COLUMN tenant.approved_at IS 'Timestamp of platform admin approval. NULL = not yet approved.';
COMMENT ON COLUMN tenant.approved_by IS 'User ID of the platform admin who approved this tenant.';
COMMENT ON COLUMN tenant.rejected_at IS 'Timestamp of platform admin rejection. NULL = not rejected.';
COMMENT ON COLUMN tenant.rejected_by IS 'User ID of the platform admin who rejected this tenant.';
COMMENT ON COLUMN tenant.rejection_reason IS 'Freeform reason text supplied by the reviewer on rejection (max 1000 chars).';

-- ---------------------------------------------------------------------------
-- PART 2: branch — add timezone and activated_at
--
-- NOTE: address_line1/2, city, province, postal_code, country_code,
-- contact_phone, contact_email were already added in migration 000001.
-- The IF NOT EXISTS guards on every statement below make it safe to re-run
-- regardless. The ADR §2.1.2 column `country` (CHAR(2)) maps to the existing
-- `country_code` column — no rename is performed to avoid breaking migration 001;
-- DATA_MODEL.md documents `country_code` as the canonical column name.
--
-- The set_updated_at trigger on `branch` already exists from migration 000001.
-- ---------------------------------------------------------------------------

ALTER TABLE branch
    ADD COLUMN IF NOT EXISTS address_line1  TEXT        NULL,
    ADD COLUMN IF NOT EXISTS address_line2  TEXT        NULL,
    ADD COLUMN IF NOT EXISTS city           TEXT        NULL
        CHECK (city IS NULL OR char_length(city) <= 100),
    ADD COLUMN IF NOT EXISTS province       TEXT        NULL
        CHECK (province IS NULL OR char_length(province) <= 100),
    ADD COLUMN IF NOT EXISTS postal_code    TEXT        NULL
        CHECK (postal_code IS NULL OR char_length(postal_code) <= 20),
    ADD COLUMN IF NOT EXISTS country_code   CHAR(2)     NULL,
    ADD COLUMN IF NOT EXISTS timezone       TEXT        NOT NULL DEFAULT 'Asia/Jakarta',
    ADD COLUMN IF NOT EXISTS contact_phone  TEXT        NULL
        CHECK (contact_phone IS NULL OR char_length(contact_phone) <= 30),
    ADD COLUMN IF NOT EXISTS contact_email  CITEXT      NULL
        CHECK (contact_email IS NULL OR char_length(contact_email::text) BETWEEN 3 AND 320),
    ADD COLUMN IF NOT EXISTS activated_at   TIMESTAMPTZ NULL;

COMMENT ON COLUMN branch.timezone IS
    'IANA timezone identifier for this branch (e.g. Asia/Jakarta). '
    'Used to convert availability windows to absolute UTC slots for booking. '
    'Resolves DATA_MODEL.md open question #3.';

COMMENT ON COLUMN branch.activated_at IS
    'Timestamp when the branch first transitioned to active status. '
    'NULL for branches that have never been activated (newly created inactive branches).';

-- ---------------------------------------------------------------------------
-- PART 3: tenant_registration_status enum
-- ---------------------------------------------------------------------------

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_type WHERE typname = 'tenant_registration_status'
    ) THEN
        CREATE TYPE tenant_registration_status AS ENUM (
            'pending',
            'approved',
            'rejected'
        );
    END IF;
END
$$;

-- ---------------------------------------------------------------------------
-- PART 4: tenant_registration table
--
-- Design notes:
--   • No tenant_id column — this is a platform-level queue, not tenant-scoped.
--   • approved_tenant_id and approved_user_id are set on approval and link
--     back to the created tenant/user rows for auditability.
--   • RLS: platform-only (app.current_tenant = '__platform__').
--   • set_updated_at trigger reuses the function from migration 000001.
-- ---------------------------------------------------------------------------

CREATE TABLE IF NOT EXISTS tenant_registration (
    id                  UUID                        PRIMARY KEY DEFAULT gen_random_uuid(),

    -- Submission fields
    company_name        TEXT                        NOT NULL
        CHECK (char_length(company_name) BETWEEN 2 AND 200),
    requested_slug      TEXT                        NOT NULL
        CHECK (
            char_length(requested_slug) BETWEEN 2 AND 100
            AND requested_slug ~ '^[a-z0-9][a-z0-9-]*[a-z0-9]$'
        ),
    package             TEXT                        NOT NULL DEFAULT 'starter'
        CHECK (package IN ('starter', 'growth', 'enterprise')),

    -- Contact info from the submitter
    contact_name        TEXT                        NOT NULL
        CHECK (char_length(contact_name) BETWEEN 1 AND 200),
    contact_email       CITEXT                      NOT NULL
        CHECK (char_length(contact_email::text) BETWEEN 3 AND 320),
    contact_phone       TEXT                        NULL
        CHECK (contact_phone IS NULL OR char_length(contact_phone) BETWEEN 5 AND 30),

    -- Lifecycle
    status              tenant_registration_status  NOT NULL DEFAULT 'pending',

    -- Set on approval (all four are populated atomically in the approval transaction)
    approved_tenant_id  UUID                        NULL
        CONSTRAINT fk_tenant_reg_approved_tenant
        REFERENCES tenant(id)
        ON DELETE SET NULL,
    approved_user_id    UUID                        NULL
        CONSTRAINT fk_tenant_reg_approved_user
        REFERENCES "user"(id)
        ON DELETE SET NULL,
    approved_at         TIMESTAMPTZ                 NULL,
    approved_by         UUID                        NULL
        CONSTRAINT fk_tenant_reg_approved_by
        REFERENCES "user"(id)
        ON DELETE SET NULL,

    -- Set on rejection
    rejected_at         TIMESTAMPTZ                 NULL,
    rejected_by         UUID                        NULL
        CONSTRAINT fk_tenant_reg_rejected_by
        REFERENCES "user"(id)
        ON DELETE SET NULL,
    rejection_reason    TEXT                        NULL
        CHECK (rejection_reason IS NULL OR char_length(rejection_reason) <= 1000),

    -- Extensible metadata (e.g. IP, user-agent, referral source)
    metadata            JSONB                       NOT NULL DEFAULT '{}',

    -- Audit
    created_at          TIMESTAMPTZ                 NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ                 NOT NULL DEFAULT now()
);

COMMENT ON TABLE tenant_registration IS
    'Public registration queue for company onboarding (ADR 0008). '
    'Each row is a submitted company registration awaiting platform admin review. '
    'Platform-level table — no tenant_id, RLS enforces __platform__ sentinel only.';

-- Composite index: the primary read pattern is "list registrations by status, ordered by submission time".
CREATE INDEX IF NOT EXISTS tenant_registration_status_idx
    ON tenant_registration (status, created_at);

-- Prevent duplicate pending submissions from the same email.
-- Multiple non-pending rows per email are allowed (approved + re-submitted later is valid).
CREATE UNIQUE INDEX IF NOT EXISTS tenant_registration_pending_email_uidx
    ON tenant_registration (contact_email)
    WHERE status = 'pending';

-- Prevent a slug from appearing in two simultaneous pending registrations.
CREATE UNIQUE INDEX IF NOT EXISTS tenant_registration_pending_slug_uidx
    ON tenant_registration (requested_slug)
    WHERE status = 'pending';

-- FK index: approved_tenant_id (FK columns not automatically indexed in Postgres).
CREATE INDEX IF NOT EXISTS tenant_registration_approved_tenant_id_idx
    ON tenant_registration (approved_tenant_id)
    WHERE approved_tenant_id IS NOT NULL;

-- FK index: approved_user_id.
CREATE INDEX IF NOT EXISTS tenant_registration_approved_user_id_idx
    ON tenant_registration (approved_user_id)
    WHERE approved_user_id IS NOT NULL;

-- set_updated_at trigger (function defined in migration 000001).
CREATE TRIGGER trg_tenant_registration_updated_at
    BEFORE UPDATE ON tenant_registration
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- ---------------------------------------------------------------------------
-- PART 5: RLS on tenant_registration — platform-only
--
-- The registration endpoint runs under the __platform__ sentinel.
-- Tenant-scoped app sessions must not read or write this queue.
-- ---------------------------------------------------------------------------

ALTER TABLE tenant_registration ENABLE ROW LEVEL SECURITY;
ALTER TABLE tenant_registration FORCE ROW LEVEL SECURITY;

-- Drop in case of re-run after partial failure.
DROP POLICY IF EXISTS tenant_registration_platform_only ON tenant_registration;

CREATE POLICY tenant_registration_platform_only ON tenant_registration
    AS PERMISSIVE FOR ALL
    USING (
        current_setting('app.current_tenant', true) = '__platform__'
    )
    WITH CHECK (
        current_setting('app.current_tenant', true) = '__platform__'
    );

-- ---------------------------------------------------------------------------
-- PART 6: Grants on tenant_registration
-- ---------------------------------------------------------------------------

GRANT SELECT, INSERT, UPDATE ON tenant_registration TO lustia_app;

-- ---------------------------------------------------------------------------
-- PART 7: Permission and role_permission guard-inserts
--
-- All of the codes below were already seeded by migration 000005.
-- These inserts are ON CONFLICT DO NOTHING — they are a CLEAN invariant
-- safety net, not functional additions. On a standard migration chain (1→11)
-- every INSERT below will match 0 rows actually inserted.
--
-- Codes confirmed present in migration 000005:
--   tenant.read      c0000000-0000-0000-0001-000000000001
--   tenant.create    c0000000-0000-0000-0001-000000000002
--   tenant.update    c0000000-0000-0000-0001-000000000003
--   tenant.delete    c0000000-0000-0000-0001-000000000004
--   tenant.approve   c0000000-0000-0000-0001-000000000005
--   branch.read      c0000000-0000-0000-0002-000000000001
--   branch.create    c0000000-0000-0000-0002-000000000002
--   branch.update    c0000000-0000-0000-0002-000000000003
--   branch.delete    c0000000-0000-0000-0002-000000000004
-- ---------------------------------------------------------------------------

INSERT INTO permission (id, code, description, created_at, updated_at)
VALUES
    ('c0000000-0000-0000-0001-000000000001', 'tenant.read',    'Read tenant details',                        now(), now()),
    ('c0000000-0000-0000-0001-000000000002', 'tenant.create',  'Create a new tenant',                        now(), now()),
    ('c0000000-0000-0000-0001-000000000003', 'tenant.update',  'Update tenant details',                      now(), now()),
    ('c0000000-0000-0000-0001-000000000004', 'tenant.delete',  'Soft-delete a tenant',                       now(), now()),
    ('c0000000-0000-0000-0001-000000000005', 'tenant.approve', 'Approve or reject a tenant registration',    now(), now()),
    ('c0000000-0000-0000-0002-000000000001', 'branch.read',    'Read branch details',                        now(), now()),
    ('c0000000-0000-0000-0002-000000000002', 'branch.create',  'Create a new branch',                        now(), now()),
    ('c0000000-0000-0000-0002-000000000003', 'branch.update',  'Update branch details',                      now(), now()),
    ('c0000000-0000-0000-0002-000000000004', 'branch.delete',  'Soft-delete a branch',                       now(), now())
ON CONFLICT (id) DO NOTHING;

-- super_admin: all permissions (already granted via SELECT * FROM permission in migration 005).
INSERT INTO role_permission (role_id, permission_id)
SELECT
    'b0000000-0000-0000-0000-000000000001'::uuid,
    p.id
FROM permission p
WHERE p.code IN (
    'tenant.read', 'tenant.create', 'tenant.update', 'tenant.delete', 'tenant.approve',
    'branch.read', 'branch.create', 'branch.update', 'branch.delete'
)
ON CONFLICT DO NOTHING;

-- tenant_admin: branch.* (already granted in migration 005; tenant.* partial also already there).
-- Confirmed: migration 005 excludes only tenant.approve, role.*, permission.manage for tenant_admin.
-- So tenant_admin already has tenant.read/create/update/delete and branch.read/create/update/delete.
INSERT INTO role_permission (role_id, permission_id)
SELECT
    'b0000000-0000-0000-0000-000000000002'::uuid,
    p.id
FROM permission p
WHERE p.code IN (
    'tenant.read', 'tenant.create', 'tenant.update', 'tenant.delete',
    'branch.read', 'branch.create', 'branch.update', 'branch.delete'
)
ON CONFLICT DO NOTHING;
