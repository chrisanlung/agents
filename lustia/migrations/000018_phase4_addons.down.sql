-- =============================================================================
-- Migration: 000018_phase4_addons (DOWN)
-- Purpose  : Reverse the UP migration — remove the tenant-wide addon table,
--            its indexes, RLS policies, trigger, grants, and the addon.*
--            permission + role_permission rows.
--
-- Safe to run after a rollback even if the UP migration was applied partially:
-- each DROP uses IF EXISTS.
-- =============================================================================

-- Remove role_permission rows for addon.* codes first (FK references permission)
DELETE FROM role_permission
WHERE permission_id IN (
    SELECT id FROM permission
    WHERE code IN ('addon.read', 'addon.create', 'addon.update', 'addon.delete')
);

-- Remove permission rows
DELETE FROM permission
WHERE code IN ('addon.read', 'addon.create', 'addon.update', 'addon.delete');

-- Drop the table (cascades to indexes, policies, and the trigger automatically)
DROP TABLE IF EXISTS addon;
