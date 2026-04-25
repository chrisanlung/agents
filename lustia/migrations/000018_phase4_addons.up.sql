-- =============================================================================
-- Migration: 000018_phase4_addons (UP)
-- Purpose  : Phase 4 — Tenant-wide add-on catalog.
--            Implements ADR 0010 §4.1 (rewritten 2026-04-24).
--
-- The previous version of this migration (now deleted) shipped a per-service
-- `service_addon` table. That design was superseded in the same session before
-- being deployed anywhere. This migration replaces it with a tenant-scoped
-- `addon` table: one catalog per tenant, every add-on available across all
-- services and bookings. See docs/DECISIONS/0010-per-service-addons.md for
-- the full rationale.
--
-- CLEAN invariant: migrations 1 → 18 produce a working schema.
-- Migration 000019 (dev-only) seeds sample add-on rows.
--
-- Re-run safety: CREATE TABLE uses IF NOT EXISTS; policy and trigger creation
-- is preceded by DROP ... IF EXISTS.
--
-- Reference: docs/DECISIONS/0010-per-service-addons.md §4.1
-- =============================================================================

-- ---------------------------------------------------------------------------
-- TABLE: addon
--
-- Tenant-scoped add-on catalog. Each row is an optional paid extra that can
-- be selected alongside any service during a booking. No mapping table is
-- needed because add-ons are globally available within the tenant.
--
-- Soft-delete only: lustia_app has no DELETE grant on this table.
-- ---------------------------------------------------------------------------

CREATE TABLE IF NOT EXISTS addon (
    id          UUID        NOT NULL DEFAULT gen_random_uuid(),
    tenant_id   UUID        NOT NULL,
    name        TEXT        NOT NULL,
    description TEXT        NULL,
    price_idr   BIGINT      NOT NULL,
    is_active   BOOLEAN     NOT NULL DEFAULT true,
    sort_order  INT         NOT NULL DEFAULT 0,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_by  UUID        NULL,
    updated_by  UUID        NULL,
    deleted_at  TIMESTAMPTZ NULL,

    CONSTRAINT pk_addon PRIMARY KEY (id),

    CONSTRAINT fk_addon_tenant
        FOREIGN KEY (tenant_id) REFERENCES tenant(id)
        ON DELETE RESTRICT,

    CONSTRAINT fk_addon_created_by
        FOREIGN KEY (created_by) REFERENCES "user"(id)
        ON DELETE SET NULL,

    CONSTRAINT fk_addon_updated_by
        FOREIGN KEY (updated_by) REFERENCES "user"(id)
        ON DELETE SET NULL,

    CONSTRAINT chk_addon_name_length
        CHECK (char_length(name) BETWEEN 1 AND 120),

    CONSTRAINT chk_addon_description_length
        CHECK (description IS NULL OR char_length(description) <= 500),

    CONSTRAINT chk_addon_price_non_negative
        CHECK (price_idr >= 0),

    CONSTRAINT chk_addon_sort_order_range
        CHECK (sort_order BETWEEN 0 AND 9999)
);

COMMENT ON TABLE addon IS
    'Tenant-wide optional extras (add-ons) that a customer can select with any '
    'service during a booking. Scoped to tenant — no per-service mapping needed. '
    'See ADR 0010 (docs/DECISIONS/0010-per-service-addons.md).';

COMMENT ON COLUMN addon.price_idr IS
    'Price in whole Rupiah (IDR). BIGINT matches service.price (BIGINT after '
    'migration 000016). Named price_idr to make the currency explicit at the '
    'column level.';

COMMENT ON COLUMN addon.sort_order IS
    'Display order in the admin catalog list and Phase 5 customer picker (0 = '
    'first). Mutated atomically by the PUT /tenant/addons/reorder endpoint. '
    'Valid range 0–9999.';

COMMENT ON COLUMN addon.deleted_at IS
    'Soft-delete sentinel. lustia_app has no DELETE grant on this table — set '
    'deleted_at = now() instead of issuing a DELETE statement.';

-- ---------------------------------------------------------------------------
-- PARTIAL UNIQUE: no duplicate active add-on names within the same tenant.
-- Deleted rows are excluded so a name can be re-used after soft-deletion.
-- ---------------------------------------------------------------------------

CREATE UNIQUE INDEX IF NOT EXISTS addon_tenant_name_uidx
    ON addon (tenant_id, name)
    WHERE deleted_at IS NULL;

-- ---------------------------------------------------------------------------
-- INDEXES
-- ---------------------------------------------------------------------------

-- Primary list-scan for admin UI and Phase 5 customer picker:
-- "all active add-ons for this tenant, in display order".
CREATE INDEX IF NOT EXISTS addon_tenant_active_idx
    ON addon (tenant_id, sort_order)
    WHERE is_active = true AND deleted_at IS NULL;

-- RLS predicate scan — every SELECT/INSERT/UPDATE from lustia_app uses this.
CREATE INDEX IF NOT EXISTS addon_tenant_id_idx
    ON addon (tenant_id);

-- ---------------------------------------------------------------------------
-- ROW-LEVEL SECURITY
--
-- Mirrors the service table (migration 000004 §7). Direct tenant_id equality
-- check against app.current_tenant. The 'true' flag in current_setting means
-- "return NULL if unset" — NULL equality evaluates to NULL (false), so an
-- unauthenticated session sees no rows.
-- ---------------------------------------------------------------------------

ALTER TABLE addon ENABLE ROW LEVEL SECURITY;
ALTER TABLE addon FORCE ROW LEVEL SECURITY;

-- SELECT ---------------------------------------------------------------
DROP POLICY IF EXISTS addon_tenant_select ON addon;
CREATE POLICY addon_tenant_select ON addon
    AS PERMISSIVE FOR SELECT
    USING (tenant_id::text = current_setting('app.current_tenant', true));

-- INSERT ---------------------------------------------------------------
DROP POLICY IF EXISTS addon_tenant_insert ON addon;
CREATE POLICY addon_tenant_insert ON addon
    AS PERMISSIVE FOR INSERT
    WITH CHECK (tenant_id::text = current_setting('app.current_tenant', true));

-- UPDATE ---------------------------------------------------------------
DROP POLICY IF EXISTS addon_tenant_update ON addon;
CREATE POLICY addon_tenant_update ON addon
    AS PERMISSIVE FOR UPDATE
    USING  (tenant_id::text = current_setting('app.current_tenant', true))
    WITH CHECK (tenant_id::text = current_setting('app.current_tenant', true));

-- No DELETE policy — hard DELETE is blocked at the DB grant level.

-- ---------------------------------------------------------------------------
-- GRANT
-- lustia_app gets SELECT/INSERT/UPDATE only. No DELETE (soft-delete only).
-- ---------------------------------------------------------------------------

GRANT SELECT, INSERT, UPDATE ON addon TO lustia_app;

-- ---------------------------------------------------------------------------
-- TRIGGER: keep updated_at current
-- set_updated_at() is defined in migration 000001 and shared by all tables.
-- ---------------------------------------------------------------------------

DROP TRIGGER IF EXISTS trg_addon_updated_at ON addon;
CREATE TRIGGER trg_addon_updated_at
    BEFORE UPDATE ON addon
    FOR EACH ROW
    EXECUTE FUNCTION set_updated_at();

-- ---------------------------------------------------------------------------
-- PERMISSION + ROLE_PERMISSION: addon.* codes
--
-- New permission namespace per ADR 0010 §4.2. Distinct from service.* because
-- the add-on catalog is a separate resource and role access may diverge in
-- future phases (e.g. tenant_admin_readonly gets addon.read but not
-- addon.write).
--
-- Fixed UUID prefix: c0000000-0000-0000-0018-* (matches migration number 18).
-- Previous draft of this migration used 0010-*, but that namespace was already
-- allocated by the booking.* permissions (migration 000013). The collision was
-- masked by `ON CONFLICT (id) DO NOTHING`, causing the addon permissions to
-- silently not insert on first-run environments. 0018 avoids every existing
-- permission UUID namespace (0001–0013).
--
-- ON CONFLICT DO NOTHING: safe to re-run; also compatible with any future
-- migration that pre-seeds these rows.
-- ---------------------------------------------------------------------------

INSERT INTO permission (id, code, description, created_at, updated_at)
VALUES
    ('c0000000-0000-0000-0018-000000000001', 'addon.read',   'List and view add-ons',          now(), now()),
    ('c0000000-0000-0000-0018-000000000002', 'addon.create', 'Create an add-on',                now(), now()),
    ('c0000000-0000-0000-0018-000000000003', 'addon.update', 'Update an add-on',                now(), now()),
    ('c0000000-0000-0000-0018-000000000004', 'addon.delete', 'Soft-delete an add-on',           now(), now())
ON CONFLICT (id) DO NOTHING;

-- super_admin: full access
INSERT INTO role_permission (role_id, permission_id)
SELECT r.id, p.id
FROM role r
CROSS JOIN permission p
WHERE r.name = 'super_admin'
  AND p.code IN ('addon.read', 'addon.create', 'addon.update', 'addon.delete')
ON CONFLICT DO NOTHING;

-- tenant_admin: full access (owns the add-on catalog)
INSERT INTO role_permission (role_id, permission_id)
SELECT r.id, p.id
FROM role r
CROSS JOIN permission p
WHERE r.name = 'tenant_admin'
  AND p.code IN ('addon.read', 'addon.create', 'addon.update', 'addon.delete')
ON CONFLICT DO NOTHING;

-- branch_admin: read-only (ADR 0010 §4.2.1 — add-ons are tenant-scoped;
-- branch_admin must not mutate the tenant catalog)
INSERT INTO role_permission (role_id, permission_id)
SELECT r.id, p.id
FROM role r
CROSS JOIN permission p
WHERE r.name = 'branch_admin'
  AND p.code IN ('addon.read')
ON CONFLICT DO NOTHING;

-- therapist, customer, finance: no add-on permissions in Phase 4.
