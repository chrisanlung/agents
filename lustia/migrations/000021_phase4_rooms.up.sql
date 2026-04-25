-- =============================================================================
-- Migration: 000021_phase4_rooms (UP)
-- Purpose  : Phase 4 — Room catalog (Ruangan) for the booking engine.
--            Implements ADR 0012 §2.1 + §2.2 exactly.
--
-- Each room belongs to exactly one branch. The booking engine (Phase 5) will
-- add a (room_id, starts_at, ends_at) exclusion constraint on the booking
-- table to guarantee no double-booking. This migration ships the catalog only.
--
-- CLEAN invariant: migrations 1 → 21 produce a working schema.
-- Migration 000022 (dev-only) seeds sample room rows.
--
-- Re-run safety: CREATE TABLE / CREATE INDEX use IF NOT EXISTS; policy and
-- trigger creation are preceded by DROP ... IF EXISTS; INSERT rows use
-- ON CONFLICT DO NOTHING / ON CONFLICT (id) DO NOTHING.
--
-- UUID namespace for permission rows: c0000000-0000-0000-0021-*
-- Verified ZERO matches in lustia/migrations/ before writing this file.
--
-- Reference: docs/DECISIONS/0012-rooms.md
-- =============================================================================

-- ---------------------------------------------------------------------------
-- TABLE: room
--
-- Branch-scoped room catalog. Each row represents a physical room that can be
-- allocated to a booking. Branch-scoped (not tenant-wide) because rooms are
-- physical assets tied to a specific location.
--
-- Soft-delete only: lustia_app has no DELETE grant on this table.
-- ---------------------------------------------------------------------------

CREATE TABLE IF NOT EXISTS room (
    id          UUID        NOT NULL DEFAULT gen_random_uuid(),
    tenant_id   UUID        NOT NULL,
    branch_id   UUID        NOT NULL,
    name        TEXT        NOT NULL,
    description TEXT        NULL,
    room_type   TEXT        NOT NULL,
    capacity    SMALLINT    NOT NULL DEFAULT 1,
    amenities   TEXT[]      NOT NULL DEFAULT '{}',
    photo_key   TEXT        NULL,
    is_active   BOOLEAN     NOT NULL DEFAULT true,
    sort_order  INT         NOT NULL DEFAULT 0,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_by  UUID        NULL,
    updated_by  UUID        NULL,
    deleted_at  TIMESTAMPTZ NULL,

    CONSTRAINT pk_room PRIMARY KEY (id),

    CONSTRAINT fk_room_tenant
        FOREIGN KEY (tenant_id) REFERENCES tenant(id)
        ON DELETE RESTRICT,

    CONSTRAINT fk_room_branch
        FOREIGN KEY (branch_id) REFERENCES branch(id)
        ON DELETE RESTRICT,

    CONSTRAINT fk_room_created_by
        FOREIGN KEY (created_by) REFERENCES "user"(id)
        ON DELETE SET NULL,

    CONSTRAINT fk_room_updated_by
        FOREIGN KEY (updated_by) REFERENCES "user"(id)
        ON DELETE SET NULL,

    CONSTRAINT chk_room_name_length
        CHECK (char_length(name) BETWEEN 1 AND 120),

    CONSTRAINT chk_room_description_length
        CHECK (description IS NULL OR char_length(description) <= 500),

    CONSTRAINT chk_room_type
        CHECK (room_type IN ('single', 'couple', 'group', 'vip')),

    CONSTRAINT chk_room_capacity_range
        CHECK (capacity BETWEEN 1 AND 20),

    CONSTRAINT chk_room_photo_key_length
        CHECK (photo_key IS NULL OR char_length(photo_key) <= 512),

    CONSTRAINT chk_room_sort_order_range
        CHECK (sort_order BETWEEN 0 AND 9999)
);

COMMENT ON TABLE room IS
    'Branch-scoped physical room catalog. Each room can be allocated to one '
    'booking at a time; the booking-engine double-booking constraint (Phase 5) '
    'will enforce this via a tstzrange exclusion constraint on the booking table. '
    'See ADR 0012 (docs/DECISIONS/0012-rooms.md).';

COMMENT ON COLUMN room.tenant_id IS
    'RLS anchor. Copied from branch.tenant_id on write; kept denormalized for '
    'efficient RLS predicate evaluation without a join to branch.';

COMMENT ON COLUMN room.branch_id IS
    'The physical location this room belongs to. FK is RESTRICT: deleting a '
    'branch that still has rooms is rejected — operator must reassign or delete '
    'rooms first. Mirrors the same pattern as therapist.branch_id.';

COMMENT ON COLUMN room.room_type IS
    'Customer-facing category. Controlled vocabulary: single | couple | group | '
    'vip. Stored as TEXT + CHECK (not native ENUM) so new values can be added '
    'via a non-blocking CHECK update, not ALTER TYPE. See ADR 0012 §2.1.';

COMMENT ON COLUMN room.capacity IS
    'Maximum simultaneous occupants. Customer-visible in Phase 5 booking picker. '
    'Booking engine uses this as a hard upper bound for room allocation. '
    'Range 1–20; SMALLINT saves 2 bytes vs INT on a frequently-read catalog row.';

COMMENT ON COLUMN room.amenities IS
    'Operator-defined free-text feature tags (e.g. "shower", "aromaterapi", '
    '"tv"). TEXT[] rather than a lookup FK table because amenity labels are '
    'tenant-specific, vary freely, and are always written/read as a whole set. '
    'A GIN index enables array-contains queries if needed. See ADR 0012 §3.1.';

COMMENT ON COLUMN room.photo_key IS
    'Opaque storage key produced by Storage.Upload() (ADR 0011 §2.1). '
    'Format: "rooms/{room_id}/{16-hex}.{ext}". '
    'Never contains scheme or host — controllers call Storage.URL(ctx, key) '
    'at read time. NULL = no photo uploaded yet.';

COMMENT ON COLUMN room.sort_order IS
    'Display order in the admin catalog list and Phase 5 customer room picker '
    '(0 = first). Mutated atomically by PUT /tenant/rooms/reorder endpoint '
    'scoped to a single branch. Range 0–9999.';

COMMENT ON COLUMN room.deleted_at IS
    'Soft-delete sentinel. lustia_app has no DELETE grant on this table — set '
    'deleted_at = now() instead of issuing a DELETE statement.';

-- ---------------------------------------------------------------------------
-- PARTIAL UNIQUE: no duplicate active room names within the same branch.
-- Deleted rows are excluded so a name can be re-used after soft-deletion.
-- Different branches may reuse the same name ("VIP 1" across two branches).
-- ---------------------------------------------------------------------------

CREATE UNIQUE INDEX IF NOT EXISTS room_branch_name_uidx
    ON room (branch_id, name)
    WHERE deleted_at IS NULL;

-- ---------------------------------------------------------------------------
-- INDEXES
-- ---------------------------------------------------------------------------

-- Primary list-scan: "all active rooms for this tenant+branch, ordered by
-- sort_order". Covers the admin list view and Phase 5 customer room picker.
CREATE INDEX IF NOT EXISTS room_tenant_branch_idx
    ON room (tenant_id, branch_id, sort_order)
    WHERE is_active = true AND deleted_at IS NULL;

-- RLS predicate scan — every SELECT/INSERT/UPDATE from lustia_app evaluates
-- tenant_id::text = current_setting('app.current_tenant', true).
CREATE INDEX IF NOT EXISTS room_tenant_id_idx
    ON room (tenant_id);

-- ---------------------------------------------------------------------------
-- ROW-LEVEL SECURITY
--
-- Mirrors the addon table pattern (migration 000018). Direct tenant_id
-- equality check against app.current_tenant. The 'true' flag in
-- current_setting means "return NULL if unset" — NULL equality evaluates to
-- NULL (false), so an unauthenticated session sees no rows.
--
-- Branch-level isolation (branch_admin may only manage rooms in their own
-- branch(es)) is NOT enforced here — it is the responsibility of RoomService
-- in the application layer, following the same pattern as TherapistService.
-- ---------------------------------------------------------------------------

ALTER TABLE room ENABLE ROW LEVEL SECURITY;
ALTER TABLE room FORCE ROW LEVEL SECURITY;

-- SELECT ---------------------------------------------------------------
DROP POLICY IF EXISTS room_tenant_select ON room;
CREATE POLICY room_tenant_select ON room
    AS PERMISSIVE FOR SELECT
    USING (tenant_id::text = current_setting('app.current_tenant', true));

-- INSERT ---------------------------------------------------------------
DROP POLICY IF EXISTS room_tenant_insert ON room;
CREATE POLICY room_tenant_insert ON room
    AS PERMISSIVE FOR INSERT
    WITH CHECK (tenant_id::text = current_setting('app.current_tenant', true));

-- UPDATE ---------------------------------------------------------------
DROP POLICY IF EXISTS room_tenant_update ON room;
CREATE POLICY room_tenant_update ON room
    AS PERMISSIVE FOR UPDATE
    USING  (tenant_id::text = current_setting('app.current_tenant', true))
    WITH CHECK (tenant_id::text = current_setting('app.current_tenant', true));

-- No DELETE policy — hard DELETE is blocked at the DB grant level.

-- ---------------------------------------------------------------------------
-- GRANT
-- lustia_app gets SELECT/INSERT/UPDATE only. No DELETE (soft-delete only).
-- ---------------------------------------------------------------------------

GRANT SELECT, INSERT, UPDATE ON room TO lustia_app;

-- ---------------------------------------------------------------------------
-- TRIGGER: keep updated_at current.
-- set_updated_at() is defined in migration 000001 and shared by all tables.
-- ---------------------------------------------------------------------------

DROP TRIGGER IF EXISTS trg_room_updated_at ON room;
CREATE TRIGGER trg_room_updated_at
    BEFORE UPDATE ON room
    FOR EACH ROW
    EXECUTE FUNCTION set_updated_at();

-- ---------------------------------------------------------------------------
-- PERMISSION + ROLE_PERMISSION: room.* codes
--
-- UUID namespace: c0000000-0000-0000-0021-*
-- The 0021 suffix matches the migration number for traceability.
-- Verified: grepping lustia/migrations/ for this prefix before writing
-- this file returned ZERO matches. No collision risk.
--
-- ON CONFLICT (id) DO NOTHING on permission rows: safe to re-run.
-- ON CONFLICT DO NOTHING on role_permission rows: uses the composite unique
-- key (role_id, permission_id) — more robust than UUID-based conflict.
-- ---------------------------------------------------------------------------

INSERT INTO permission (id, code, description, created_at, updated_at)
VALUES
    ('c0000000-0000-0000-0021-000000000001', 'room.read',   'List and view rooms',                        now(), now()),
    ('c0000000-0000-0000-0021-000000000002', 'room.create', 'Create a room',                              now(), now()),
    ('c0000000-0000-0000-0021-000000000003', 'room.update', 'Update a room (incl. photo + reorder)',      now(), now()),
    ('c0000000-0000-0000-0021-000000000004', 'room.delete', 'Soft-delete a room',                        now(), now())
ON CONFLICT (id) DO NOTHING;

-- super_admin: full access
INSERT INTO role_permission (role_id, permission_id)
SELECT r.id, p.id
FROM role r
CROSS JOIN permission p
WHERE r.name = 'super_admin'
  AND p.code IN ('room.read', 'room.create', 'room.update', 'room.delete')
ON CONFLICT DO NOTHING;

-- tenant_admin: full access (manages all branches)
INSERT INTO role_permission (role_id, permission_id)
SELECT r.id, p.id
FROM role r
CROSS JOIN permission p
WHERE r.name = 'tenant_admin'
  AND p.code IN ('room.read', 'room.create', 'room.update', 'room.delete')
ON CONFLICT DO NOTHING;

-- branch_admin: all 4 permissions; branch-scope restriction (own branch only)
-- is enforced by RoomService in the application layer — not by the DB.
-- Same pattern as TherapistService (Phase 4, ADR 0012 §2.2).
INSERT INTO role_permission (role_id, permission_id)
SELECT r.id, p.id
FROM role r
CROSS JOIN permission p
WHERE r.name = 'branch_admin'
  AND p.code IN ('room.read', 'room.create', 'room.update', 'room.delete')
ON CONFLICT DO NOTHING;

-- finance, therapist, customer: no room permissions in Phase 4.
