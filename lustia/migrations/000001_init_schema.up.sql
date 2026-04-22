-- =============================================================================
-- Migration: 000001_init_schema
-- Purpose  : Extensions, enum types, platform-level tables (tenant, branch,
--            role, permission, role_permission), shared trigger function,
--            and DB roles.
-- =============================================================================

-- ---------------------------------------------------------------------------
-- 0. DB roles
--    lustia_app      : normal application connection; RLS enforced.
--    lustia_migrator : runs migrations; BYPASSRLS so it can manage schema.
-- ---------------------------------------------------------------------------
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'lustia_app') THEN
        CREATE ROLE lustia_app WITH LOGIN NOINHERIT;
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'lustia_migrator') THEN
        CREATE ROLE lustia_migrator WITH LOGIN BYPASSRLS;
    END IF;
END
$$;

-- ---------------------------------------------------------------------------
-- 1. Extensions
-- ---------------------------------------------------------------------------
CREATE EXTENSION IF NOT EXISTS pgcrypto;    -- gen_random_uuid(), crypt()
CREATE EXTENSION IF NOT EXISTS citext;      -- case-insensitive text type
CREATE EXTENSION IF NOT EXISTS btree_gist;  -- needed for EXCLUDE USING gist with non-range types (UUID, int)

-- ---------------------------------------------------------------------------
-- 2. Enum types
-- ---------------------------------------------------------------------------
DO $$ BEGIN
    CREATE TYPE tenant_status AS ENUM (
        'pending_approval',
        'active',
        'suspended',
        'deactivated'
    );
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

DO $$ BEGIN
    CREATE TYPE branch_status AS ENUM (
        'active',
        'inactive'
    );
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

DO $$ BEGIN
    CREATE TYPE booking_status AS ENUM (
        'draft',
        'pending',
        'confirmed',
        'checked_in',
        'in_progress',
        'completed',
        'cancelled',
        'no_show'
    );
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

DO $$ BEGIN
    CREATE TYPE booking_source AS ENUM (
        'walk_in',
        'online',
        'phone'
    );
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

DO $$ BEGIN
    CREATE TYPE invoice_status AS ENUM (
        'draft',
        'issued',
        'partially_paid',
        'paid',
        'void'
    );
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

DO $$ BEGIN
    CREATE TYPE payment_method AS ENUM (
        'cash',
        'card',
        'transfer',
        'qris',
        'gateway'
    );
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

DO $$ BEGIN
    CREATE TYPE payment_status AS ENUM (
        'pending',
        'captured',
        'failed',
        'refunded'
    );
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

-- ---------------------------------------------------------------------------
-- 3. Shared trigger function: keeps updated_at current on every mutation
-- ---------------------------------------------------------------------------
CREATE OR REPLACE FUNCTION set_updated_at()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
BEGIN
    NEW.updated_at = now();
    RETURN NEW;
END;
$$;

-- ---------------------------------------------------------------------------
-- 4. tenant
--    Platform-level entity. One row per company that uses Lustia.
--    No tenant_id on this table — it IS the root of tenancy.
--    Not subject to tenant-scoped RLS (platform-admin table).
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS tenant (
    id              UUID            PRIMARY KEY DEFAULT gen_random_uuid(),
    name            TEXT            NOT NULL CHECK (char_length(name) BETWEEN 1 AND 200),
    slug            TEXT            NOT NULL CHECK (char_length(slug) BETWEEN 1 AND 100),
    status          tenant_status   NOT NULL DEFAULT 'pending_approval',
    plan            TEXT            NULL     CHECK (plan IS NULL OR char_length(plan) <= 50),
    -- Contact info
    contact_name    TEXT            NULL     CHECK (contact_name IS NULL OR char_length(contact_name) <= 200),
    contact_email   CITEXT          NULL     CHECK (contact_email IS NULL OR char_length(contact_email) <= 320),
    contact_phone   TEXT            NULL     CHECK (contact_phone IS NULL OR char_length(contact_phone) <= 30),
    -- Extensible metadata (e.g. logo_url, industry, billing address)
    metadata        JSONB           NOT NULL DEFAULT '{}'::jsonb,
    -- Soft delete
    deleted_at      TIMESTAMPTZ     NULL,
    -- Audit
    created_at      TIMESTAMPTZ     NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ     NOT NULL DEFAULT now(),
    created_by      UUID            NULL,   -- NULL on bootstrap insert
    updated_by      UUID            NULL
);

CREATE UNIQUE INDEX IF NOT EXISTS tenant_slug_active_uidx
    ON tenant (slug) WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS tenant_status_idx    ON tenant (status);
CREATE INDEX IF NOT EXISTS tenant_metadata_gin  ON tenant USING GIN (metadata);

CREATE TRIGGER trg_tenant_updated_at
    BEFORE UPDATE ON tenant
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- ---------------------------------------------------------------------------
-- 5. branch
--    A physical or logical location within a tenant.
--    Not subject to tenant-scoped RLS (managed by tenant-service, not app-user
--    tenant_id RLS — it carries tenant_id but super_admin/tenant_admin can
--    see across branches; RLS is applied in migration 004).
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS branch (
    id                  UUID            PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id           UUID            NOT NULL
                            CONSTRAINT fk_branch_tenant
                            REFERENCES tenant(id)
                            ON DELETE RESTRICT,
    name                TEXT            NOT NULL CHECK (char_length(name) BETWEEN 1 AND 200),
    code                TEXT            NULL     CHECK (code IS NULL OR char_length(code) <= 50),
    status              branch_status   NOT NULL DEFAULT 'active',
    -- Address
    address_line1       TEXT            NULL,
    address_line2       TEXT            NULL,
    city                TEXT            NULL     CHECK (city IS NULL OR char_length(city) <= 100),
    province            TEXT            NULL     CHECK (province IS NULL OR char_length(province) <= 100),
    postal_code         TEXT            NULL     CHECK (postal_code IS NULL OR char_length(postal_code) <= 20),
    country_code        CHAR(2)         NULL,
    -- Contact
    contact_phone       TEXT            NULL     CHECK (contact_phone IS NULL OR char_length(contact_phone) <= 30),
    contact_email       CITEXT          NULL     CHECK (contact_email IS NULL OR char_length(contact_email) <= 320),
    -- Operational hours as JSONB array: [{day:0, open:"09:00", close:"21:00"}, ...]
    operational_hours   JSONB           NOT NULL DEFAULT '[]'::jsonb,
    -- Extensible metadata
    metadata            JSONB           NOT NULL DEFAULT '{}'::jsonb,
    -- Soft delete
    deleted_at          TIMESTAMPTZ     NULL,
    -- Audit
    created_at          TIMESTAMPTZ     NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ     NOT NULL DEFAULT now(),
    created_by          UUID            NULL,
    updated_by          UUID            NULL
);

CREATE UNIQUE INDEX IF NOT EXISTS branch_tenant_code_active_uidx
    ON branch (tenant_id, code) WHERE deleted_at IS NULL AND code IS NOT NULL;

CREATE INDEX IF NOT EXISTS branch_tenant_id_idx     ON branch (tenant_id);
CREATE INDEX IF NOT EXISTS branch_status_idx        ON branch (status);
CREATE INDEX IF NOT EXISTS branch_metadata_gin      ON branch USING GIN (metadata);

CREATE TRIGGER trg_branch_updated_at
    BEFORE UPDATE ON branch
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- ---------------------------------------------------------------------------
-- 6. role
--    Platform-wide role definitions. Seeded; not tenant-scoped.
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS role (
    id          UUID    PRIMARY KEY DEFAULT gen_random_uuid(),
    name        TEXT    NOT NULL CHECK (char_length(name) BETWEEN 1 AND 100),
    description TEXT    NULL,
    -- Audit
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_by  UUID        NULL,
    updated_by  UUID        NULL
);

CREATE UNIQUE INDEX IF NOT EXISTS role_name_uidx ON role (name);

CREATE TRIGGER trg_role_updated_at
    BEFORE UPDATE ON role
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- ---------------------------------------------------------------------------
-- 7. permission
--    Fine-grained permission codes (e.g. 'booking.create').
--    Platform-wide; seeded.
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS permission (
    id          UUID    PRIMARY KEY DEFAULT gen_random_uuid(),
    code        TEXT    NOT NULL CHECK (char_length(code) BETWEEN 1 AND 100),
    description TEXT    NULL,
    -- Audit
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_by  UUID        NULL,
    updated_by  UUID        NULL
);

CREATE UNIQUE INDEX IF NOT EXISTS permission_code_uidx ON permission (code);

CREATE TRIGGER trg_permission_updated_at
    BEFORE UPDATE ON permission
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- ---------------------------------------------------------------------------
-- 8. role_permission
--    Junction: which permissions belong to which role.
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS role_permission (
    role_id         UUID    NOT NULL
                        CONSTRAINT fk_role_permission_role
                        REFERENCES role(id)
                        ON DELETE CASCADE,
    permission_id   UUID    NOT NULL
                        CONSTRAINT fk_role_permission_permission
                        REFERENCES permission(id)
                        ON DELETE CASCADE,
    -- Audit
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_by      UUID        NULL,

    CONSTRAINT pk_role_permission PRIMARY KEY (role_id, permission_id)
);

CREATE INDEX IF NOT EXISTS role_permission_permission_id_idx ON role_permission (permission_id);
