-- =============================================================================
-- Migration: 000013_phase4_master_data (UP)
-- Purpose  : Phase 4 — Master Operational Data.
--            Implements ADR 0009 §2.1 exactly:
--              - Add branch_id to `therapist` (therapist is branch-scoped).
--              - Add category to `service` (tenant-scoped service catalog).
--              - Add is_active to `therapist_service` (per-therapist activation).
--              - therapist_availability already fully specified in migration
--                000003; no structural change required.
--              - Guard-insert any missing permission / role_permission rows
--                (all were seeded by migration 000005; ON CONFLICT DO NOTHING).
--
-- Re-run safety: every ALTER TABLE step uses ADD COLUMN IF NOT EXISTS.
-- Adding a constraint that already exists is guarded via DO $$ ... END $$.
-- The permission / role_permission inserts use ON CONFLICT DO NOTHING.
--
-- Migration 000014 (dev-only) loads sample data for these structures.
-- Do not apply 000014 in staging or production.
--
-- Reference: docs/DECISIONS/0009-phase-4-master-operational-data.md
-- =============================================================================

-- ---------------------------------------------------------------------------
-- PART 1: therapist — add branch_id
--
-- ADR 0009 §2.1: a therapist works at exactly one branch; tenant_id is
-- already present for RLS; branch_id is now required for all Phase 4+ logic.
--
-- Safe ADD: no rows exist in this table outside the dev seed chain (which
-- lands in migration 000014, after this migration). Adding NOT NULL without
-- a DEFAULT is therefore safe in every environment. The IF NOT EXISTS guard
-- protects against re-runs.
-- ---------------------------------------------------------------------------

ALTER TABLE therapist
    ADD COLUMN IF NOT EXISTS branch_id UUID NULL
        CONSTRAINT fk_therapist_branch
        REFERENCES branch(id)
        ON DELETE RESTRICT;

-- Back-fill guard: on a fresh DB the column is empty; set NOT NULL after add.
-- We use a DO block so this is idempotent (the NOT NULL constraint cannot be
-- added via IF NOT EXISTS, but ALTER COLUMN on a NOT NULL column is a no-op).
DO $$
BEGIN
    -- Only enforce NOT NULL if every existing row already has a branch_id.
    -- On a fresh chain this is always true (no rows). On a partial-migration
    -- retry this is also true because the add-column step above inserted NULL
    -- and would need data back-fill before this point is reached.
    IF NOT EXISTS (
        SELECT 1 FROM therapist WHERE branch_id IS NULL
    ) THEN
        ALTER TABLE therapist ALTER COLUMN branch_id SET NOT NULL;
    END IF;
END
$$;

-- Index: FK lookup and the primary Phase 5 booking-engine query pattern
-- "available therapists at branch X" — composite covers both.
CREATE INDEX IF NOT EXISTS therapist_branch_id_idx
    ON therapist (branch_id);

CREATE INDEX IF NOT EXISTS therapist_tenant_branch_active_idx
    ON therapist (tenant_id, branch_id)
    WHERE is_active = true AND deleted_at IS NULL;

COMMENT ON COLUMN therapist.branch_id IS
    'Branch this therapist is assigned to (Phase 4, ADR 0009). '
    'One therapist row = one branch. A human working two branches has two rows '
    '(optionally linked by the same user_id for portal login).';

-- ---------------------------------------------------------------------------
-- PART 2: service — add category
--
-- ADR 0009 §2.1 names category as a first-class column on service.
-- service.branch_id is already nullable (NULL = tenant-wide); confirmed
-- correct per ADR 0009 — service is tenant-scoped, not branch-scoped.
-- No structural change to branch_id; it stays nullable.
-- ---------------------------------------------------------------------------

ALTER TABLE service
    ADD COLUMN IF NOT EXISTS category TEXT NULL
        CHECK (category IS NULL OR char_length(category) <= 100);

-- Index: "list active services by category" is the primary tenant catalog query.
CREATE INDEX IF NOT EXISTS service_tenant_category_active_idx
    ON service (tenant_id, category)
    WHERE is_active = true AND deleted_at IS NULL;

COMMENT ON COLUMN service.category IS
    'Optional grouping label for the service catalog (e.g. "Pijat", "Refleksi", '
    '"Aromaterapi"). Free-form in Phase 4; promote to enum if operator-defined '
    'taxonomies become a requirement.';

COMMENT ON COLUMN service.branch_id IS
    'NULL = service is tenant-wide (default, Phase 4 norm). '
    'Set to branch ID for a branch-specific override. Phase 4 creates only '
    'tenant-wide services (branch_id IS NULL).';

-- ---------------------------------------------------------------------------
-- PART 3: therapist_service — add is_active
--
-- ADR 0009 §2.1: a service can be "temporarily not offered by this therapist"
-- without deleting the mapping row. is_active = false suspends the offering
-- for booking without losing the assignment history.
-- ---------------------------------------------------------------------------

ALTER TABLE therapist_service
    ADD COLUMN IF NOT EXISTS is_active BOOLEAN NOT NULL DEFAULT true;

-- Index: booking engine reads "active services for therapist X at tenant Y".
-- Composite covers the typical Phase 5 booking query pattern.
CREATE INDEX IF NOT EXISTS therapist_service_active_idx
    ON therapist_service (therapist_id)
    WHERE is_active = true;

COMMENT ON COLUMN therapist_service.is_active IS
    'When false the therapist temporarily does not offer this service '
    '(e.g. on leave, skill lapse). Set to false rather than deleting the row '
    'to preserve mapping history. Phase 5 booking engine filters WHERE is_active.';

-- ---------------------------------------------------------------------------
-- PART 4: therapist_availability — no structural changes
--
-- Full schema was specified in migration 000003:
--   - tenant_id, therapist_id, branch_id, day_of_week (0=Sunday),
--     start_time, end_time (TIME columns, minute precision), effective_from,
--     effective_until, audit columns.
--   - GiST exclusion constraint prevents overlapping windows.
--   - CHECK (end_time > start_time) enforced.
--
-- Open question resolutions (ADR 0009 Q1, Q2) are documented below.
-- No DDL change required.
-- ---------------------------------------------------------------------------

-- Q1 resolved: TIME columns provide minute precision. See DATA_MODEL.md §Phase 4.
-- Q2 resolved: duration_minutes is a single fixed INT on service. See §Phase 4.

-- ---------------------------------------------------------------------------
-- PART 5: permission + role_permission guard-inserts
--
-- All Phase 4 permission codes are already present from migration 000005:
--   therapist.*   c0000000-0000-0000-0007-*
--   service.*     c0000000-0000-0000-0008-*
--   availability.* c0000000-0000-0000-0009-*
--
-- The inserts below are ON CONFLICT DO NOTHING safety nets. On a standard
-- 1→13 chain they match 0 new rows. They exist so this migration is
-- self-contained if run on a non-standard base.
-- ---------------------------------------------------------------------------

INSERT INTO permission (id, code, description, created_at, updated_at)
VALUES
    ('c0000000-0000-0000-0007-000000000001', 'therapist.read',       'Read therapist profiles',              now(), now()),
    ('c0000000-0000-0000-0007-000000000002', 'therapist.create',     'Create a therapist profile',           now(), now()),
    ('c0000000-0000-0000-0007-000000000003', 'therapist.update',     'Update a therapist profile',           now(), now()),
    ('c0000000-0000-0000-0007-000000000004', 'therapist.delete',     'Soft-delete a therapist profile',      now(), now()),
    ('c0000000-0000-0000-0008-000000000001', 'service.read',         'Read service catalog',                 now(), now()),
    ('c0000000-0000-0000-0008-000000000002', 'service.create',       'Create a service',                     now(), now()),
    ('c0000000-0000-0000-0008-000000000003', 'service.update',       'Update a service',                     now(), now()),
    ('c0000000-0000-0000-0008-000000000004', 'service.delete',       'Soft-delete a service',                now(), now()),
    ('c0000000-0000-0000-0009-000000000001', 'availability.read',    'Read therapist availability',          now(), now()),
    ('c0000000-0000-0000-0009-000000000002', 'availability.create',  'Set therapist availability',           now(), now()),
    ('c0000000-0000-0000-0009-000000000003', 'availability.update',  'Update therapist availability',        now(), now()),
    ('c0000000-0000-0000-0009-000000000004', 'availability.delete',  'Delete an availability window',        now(), now())
ON CONFLICT (id) DO NOTHING;

-- super_admin, tenant_admin, branch_admin, therapist, customer already wired in
-- migration 000005. Guard re-insert in LIFO-safe way.
INSERT INTO role_permission (role_id, permission_id)
SELECT r.id, p.id
FROM role r
CROSS JOIN permission p
WHERE r.name = 'super_admin'
  AND p.code IN (
      'therapist.read', 'therapist.create', 'therapist.update', 'therapist.delete',
      'service.read', 'service.create', 'service.update', 'service.delete',
      'availability.read', 'availability.create', 'availability.update', 'availability.delete'
  )
ON CONFLICT DO NOTHING;

INSERT INTO role_permission (role_id, permission_id)
SELECT r.id, p.id
FROM role r
CROSS JOIN permission p
WHERE r.name = 'tenant_admin'
  AND p.code IN (
      'therapist.read', 'therapist.create', 'therapist.update', 'therapist.delete',
      'service.read', 'service.create', 'service.update', 'service.delete',
      'availability.read', 'availability.create', 'availability.update', 'availability.delete'
  )
ON CONFLICT DO NOTHING;

INSERT INTO role_permission (role_id, permission_id)
SELECT r.id, p.id
FROM role r
CROSS JOIN permission p
WHERE r.name = 'branch_admin'
  AND p.code IN (
      'therapist.read', 'therapist.create', 'therapist.update', 'therapist.delete',
      'service.read', 'service.create', 'service.update', 'service.delete',
      'availability.read', 'availability.create', 'availability.update', 'availability.delete'
  )
ON CONFLICT DO NOTHING;

INSERT INTO role_permission (role_id, permission_id)
SELECT r.id, p.id
FROM role r
CROSS JOIN permission p
WHERE r.name = 'therapist'
  AND p.code IN (
      'service.read',
      'availability.read', 'availability.create', 'availability.update', 'availability.delete'
  )
ON CONFLICT DO NOTHING;

INSERT INTO role_permission (role_id, permission_id)
SELECT r.id, p.id
FROM role r
CROSS JOIN permission p
WHERE r.name = 'customer'
  AND p.code IN ('therapist.read', 'service.read')
ON CONFLICT DO NOTHING;
