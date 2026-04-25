-- =============================================================================
-- Migration: 000021_phase4_rooms (DOWN)
-- Purpose  : Reverse migration 000021 — remove room table, permissions, and
--            all associated DB objects.
--
-- Reversal order:
--   1. Delete role_permission rows by permission code (not UUID — resilient
--      to namespace changes and manual fixups).
--   2. Delete permission rows by code.
--   3. Drop trigger.
--   4. Revoke grants.
--   5. Drop RLS policies.
--   6. Drop indexes (partial unique + regular).
--   7. Drop table (CASCADE drops FK constraints referencing room if any exist).
--
-- WARNING: Any data in the room table will be permanently lost.
-- =============================================================================

-- ---------------------------------------------------------------------------
-- STEP 1: Remove role_permission wiring for room.* codes.
-- Deleting by permission code (via sub-select) avoids hard-coding UUIDs and
-- is resilient to any future namespace correction.
-- ---------------------------------------------------------------------------

DELETE FROM role_permission
WHERE permission_id IN (
    SELECT id FROM permission
    WHERE code IN ('room.read', 'room.create', 'room.update', 'room.delete')
);

-- ---------------------------------------------------------------------------
-- STEP 2: Remove permission rows by code.
-- ---------------------------------------------------------------------------

DELETE FROM permission
WHERE code IN ('room.read', 'room.create', 'room.update', 'room.delete');

-- ---------------------------------------------------------------------------
-- STEP 3: Drop trigger.
-- ---------------------------------------------------------------------------

DROP TRIGGER IF EXISTS trg_room_updated_at ON room;

-- ---------------------------------------------------------------------------
-- STEP 4: Revoke grants.
-- ---------------------------------------------------------------------------

REVOKE SELECT, INSERT, UPDATE ON room FROM lustia_app;

-- ---------------------------------------------------------------------------
-- STEP 5: Drop RLS policies.
-- ---------------------------------------------------------------------------

DROP POLICY IF EXISTS room_tenant_select ON room;
DROP POLICY IF EXISTS room_tenant_insert ON room;
DROP POLICY IF EXISTS room_tenant_update ON room;

-- ---------------------------------------------------------------------------
-- STEP 6: Drop indexes (the partial unique and the regular indexes).
-- The table drop in step 7 would also cascade-drop indexes, but explicit
-- drops here keep the intent clear and allow partial rollback debugging.
-- ---------------------------------------------------------------------------

DROP INDEX IF EXISTS room_branch_name_uidx;
DROP INDEX IF EXISTS room_tenant_branch_idx;
DROP INDEX IF EXISTS room_tenant_id_idx;

-- ---------------------------------------------------------------------------
-- STEP 7: Drop table (and its constraints).
-- IF EXISTS guards against a partially-applied up migration.
-- ---------------------------------------------------------------------------

DROP TABLE IF EXISTS room;
