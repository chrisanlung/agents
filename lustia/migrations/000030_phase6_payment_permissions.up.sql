-- =============================================================================
-- Migration: 000030_phase6_payment_permissions (UP)
-- Purpose  : Phase 6 — Insert finance/disbursement/settlement permission codes
--            and wire them to roles.
--            Implements ADR 0015 §2.11 (permissions matrix).
--
-- UUID namespace: c0000000-0000-0000-0030-*
-- Verified ZERO matches in lustia/migrations/ before writing this file.
-- The 0030 suffix matches the migration number for traceability.
--
-- Permissions added:
--   finance.read         — tenant_admin, branch_admin (own-tenant view)
--   finance.read_all     — super_admin (platform-wide view)
--   disbursement.create  — super_admin only
--   disbursement.transfer — super_admin only
--   settlement.reconcile — super_admin only
--
-- Role wiring:
--   super_admin  : all five permissions
--   tenant_admin : finance.read
--   branch_admin : finance.read
--   therapist    : none
--   customer     : N/A (no internal role)
--
-- CLEAN invariant: migrations 1 → 30 produce a working schema.
--
-- Re-run safety:
--   INSERT uses ON CONFLICT (code) DO NOTHING on permission rows.
--   INSERT uses ON CONFLICT DO NOTHING on role_permission rows
--   (composite unique key: role_id, permission_id).
-- =============================================================================

INSERT INTO permission (id, code, description, created_at, updated_at)
VALUES
    ('c0000000-0000-0000-0030-000000000001', 'finance.read',          'View payment transactions and disbursement history for own tenant',    now(), now()),
    ('c0000000-0000-0000-0030-000000000002', 'finance.read_all',      'View payment transactions and disbursements across all tenants (platform-wide)', now(), now()),
    ('c0000000-0000-0000-0030-000000000003', 'disbursement.create',   'Create a tenant disbursement payout record',                           now(), now()),
    ('c0000000-0000-0000-0030-000000000004', 'disbursement.transfer', 'Mark a disbursement as transferred and record bank reference',          now(), now()),
    ('c0000000-0000-0000-0030-000000000005', 'settlement.reconcile',  'Trigger iPaymu settlement reconciliation and create settlement_batch',  now(), now())
ON CONFLICT (code) DO NOTHING;

-- super_admin: all five permissions
INSERT INTO role_permission (role_id, permission_id)
SELECT r.id, p.id
FROM role r
CROSS JOIN permission p
WHERE r.name = 'super_admin'
  AND p.code IN (
      'finance.read', 'finance.read_all',
      'disbursement.create', 'disbursement.transfer',
      'settlement.reconcile'
  )
ON CONFLICT DO NOTHING;

-- tenant_admin: finance.read (own-tenant finance view)
INSERT INTO role_permission (role_id, permission_id)
SELECT r.id, p.id
FROM role r
CROSS JOIN permission p
WHERE r.name = 'tenant_admin'
  AND p.code IN ('finance.read')
ON CONFLICT DO NOTHING;

-- branch_admin: finance.read (own-tenant finance view; branch-scope is service-layer)
INSERT INTO role_permission (role_id, permission_id)
SELECT r.id, p.id
FROM role r
CROSS JOIN permission p
WHERE r.name = 'branch_admin'
  AND p.code IN ('finance.read')
ON CONFLICT DO NOTHING;

-- therapist: no finance permissions.
-- customer: N/A — no internal role, customer-facing endpoints use public path.
