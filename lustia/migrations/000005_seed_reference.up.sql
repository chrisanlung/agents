-- =============================================================================
-- Migration: 000005_seed_reference
-- Purpose  : Insert roles, permissions, role_permission matrix, and a
--            bootstrap platform super_admin user.
--
-- All primary keys are fixed UUIDs so dev/staging/prod environments share
-- the same role/permission identity. This enables consistent foreign key
-- references in code and fixtures without lookup queries.
--
-- BOOTSTRAP SUPER ADMIN:
--   The platform super_admin user is inserted with a placeholder password hash.
--   BEFORE running this migration in any environment, substitute the real
--   Argon2id hash by setting the BOOTSTRAP_ADMIN_PASSWORD_HASH environment
--   variable and using a pre-processing step, OR run the following SQL after
--   migrations complete:
--
--     UPDATE "user"
--     SET password_hash = '<your_argon2id_hash>'
--     WHERE id = 'a0000000-0000-0000-0000-000000000001';
--
--   The placeholder hash below will NOT authenticate successfully. It is a
--   safety measure: the account is unusable until the password is set.
--   See docs/DECISIONS/0003-bootstrap-super-admin.md for rationale.
-- =============================================================================

-- ---------------------------------------------------------------------------
-- ROLES (6 total)
-- ---------------------------------------------------------------------------
INSERT INTO role (id, name, description, created_at, updated_at)
VALUES
    (
        'b0000000-0000-0000-0000-000000000001',
        'super_admin',
        'Platform-wide administrator. Can manage tenants, approve registrations, and access all data.',
        now(), now()
    ),
    (
        'b0000000-0000-0000-0000-000000000002',
        'tenant_admin',
        'Tenant-level administrator. Can manage branches, staff, and all settings within their tenant.',
        now(), now()
    ),
    (
        'b0000000-0000-0000-0000-000000000003',
        'branch_admin',
        'Branch-level administrator. Can manage therapists, services, schedules, and bookings at their assigned branch.',
        now(), now()
    ),
    (
        'b0000000-0000-0000-0000-000000000004',
        'finance',
        'Financial staff. Can view and manage invoices, payments, and financial reports within their tenant.',
        now(), now()
    ),
    (
        'b0000000-0000-0000-0000-000000000005',
        'therapist',
        'Therapist. Can view and manage their own schedule, availability, and assigned bookings.',
        now(), now()
    ),
    (
        'b0000000-0000-0000-0000-000000000006',
        'customer',
        'Customer. Can view their own profile, browse services, and manage their own bookings.',
        now(), now()
    )
ON CONFLICT (id) DO NOTHING;

-- ---------------------------------------------------------------------------
-- PERMISSIONS
-- Permission codes follow the pattern: <resource>.<action>
-- Fixed UUIDs for stable cross-environment references.
-- ---------------------------------------------------------------------------
INSERT INTO permission (id, code, description, created_at, updated_at)
VALUES
    -- tenant.*
    ('c0000000-0000-0000-0001-000000000001', 'tenant.read',      'Read tenant details',                  now(), now()),
    ('c0000000-0000-0000-0001-000000000002', 'tenant.create',    'Create a new tenant',                  now(), now()),
    ('c0000000-0000-0000-0001-000000000003', 'tenant.update',    'Update tenant details',                now(), now()),
    ('c0000000-0000-0000-0001-000000000004', 'tenant.delete',    'Soft-delete a tenant',                 now(), now()),
    ('c0000000-0000-0000-0001-000000000005', 'tenant.approve',   'Approve or reject a tenant registration', now(), now()),

    -- branch.*
    ('c0000000-0000-0000-0002-000000000001', 'branch.read',      'Read branch details',                  now(), now()),
    ('c0000000-0000-0000-0002-000000000002', 'branch.create',    'Create a new branch',                  now(), now()),
    ('c0000000-0000-0000-0002-000000000003', 'branch.update',    'Update branch details',                now(), now()),
    ('c0000000-0000-0000-0002-000000000004', 'branch.delete',    'Soft-delete a branch',                 now(), now()),

    -- user.*
    ('c0000000-0000-0000-0003-000000000001', 'user.read',        'Read user profiles',                   now(), now()),
    ('c0000000-0000-0000-0003-000000000002', 'user.create',      'Create a new user',                    now(), now()),
    ('c0000000-0000-0000-0003-000000000003', 'user.update',      'Update a user',                        now(), now()),
    ('c0000000-0000-0000-0003-000000000004', 'user.delete',      'Deactivate / soft-delete a user',      now(), now()),
    ('c0000000-0000-0000-0003-000000000005', 'user.assign_role', 'Assign or revoke roles for a user',    now(), now()),

    -- role.*
    ('c0000000-0000-0000-0004-000000000001', 'role.read',        'Read role definitions',                now(), now()),
    ('c0000000-0000-0000-0004-000000000002', 'role.create',      'Create a new role',                    now(), now()),
    ('c0000000-0000-0000-0004-000000000003', 'role.update',      'Update role details',                  now(), now()),
    ('c0000000-0000-0000-0004-000000000004', 'role.delete',      'Delete a role',                        now(), now()),

    -- permission.*
    ('c0000000-0000-0000-0005-000000000001', 'permission.read',  'Read permission definitions',          now(), now()),
    ('c0000000-0000-0000-0005-000000000002', 'permission.manage','Create / update / delete permissions', now(), now()),

    -- customer.*
    ('c0000000-0000-0000-0006-000000000001', 'customer.read',    'Read customer records',                now(), now()),
    ('c0000000-0000-0000-0006-000000000002', 'customer.create',  'Create a new customer record',         now(), now()),
    ('c0000000-0000-0000-0006-000000000003', 'customer.update',  'Update a customer record',             now(), now()),
    ('c0000000-0000-0000-0006-000000000004', 'customer.delete',  'Soft-delete a customer record',        now(), now()),

    -- therapist.*
    ('c0000000-0000-0000-0007-000000000001', 'therapist.read',   'Read therapist profiles',              now(), now()),
    ('c0000000-0000-0000-0007-000000000002', 'therapist.create', 'Create a therapist profile',           now(), now()),
    ('c0000000-0000-0000-0007-000000000003', 'therapist.update', 'Update a therapist profile',           now(), now()),
    ('c0000000-0000-0000-0007-000000000004', 'therapist.delete', 'Soft-delete a therapist profile',      now(), now()),

    -- service.*
    ('c0000000-0000-0000-0008-000000000001', 'service.read',     'Read service catalog',                 now(), now()),
    ('c0000000-0000-0000-0008-000000000002', 'service.create',   'Create a service',                     now(), now()),
    ('c0000000-0000-0000-0008-000000000003', 'service.update',   'Update a service',                     now(), now()),
    ('c0000000-0000-0000-0008-000000000004', 'service.delete',   'Soft-delete a service',                now(), now()),

    -- availability.*
    ('c0000000-0000-0000-0009-000000000001', 'availability.read',   'Read therapist availability',       now(), now()),
    ('c0000000-0000-0000-0009-000000000002', 'availability.create', 'Set therapist availability',        now(), now()),
    ('c0000000-0000-0000-0009-000000000003', 'availability.update', 'Update therapist availability',     now(), now()),
    ('c0000000-0000-0000-0009-000000000004', 'availability.delete', 'Delete an availability window',     now(), now()),

    -- booking.*
    ('c0000000-0000-0000-0010-000000000001', 'booking.read',     'Read bookings',                        now(), now()),
    ('c0000000-0000-0000-0010-000000000002', 'booking.create',   'Create a new booking',                 now(), now()),
    ('c0000000-0000-0000-0010-000000000003', 'booking.update',   'Update booking details',               now(), now()),
    ('c0000000-0000-0000-0010-000000000004', 'booking.cancel',   'Cancel a booking',                     now(), now()),
    ('c0000000-0000-0000-0010-000000000005', 'booking.checkin',  'Check in a customer for a booking',    now(), now()),
    ('c0000000-0000-0000-0010-000000000006', 'booking.complete', 'Mark a booking as completed',          now(), now()),

    -- invoice.*
    ('c0000000-0000-0000-0011-000000000001', 'invoice.read',     'Read invoices',                        now(), now()),
    ('c0000000-0000-0000-0011-000000000002', 'invoice.create',   'Create an invoice',                    now(), now()),
    ('c0000000-0000-0000-0011-000000000003', 'invoice.update',   'Update an invoice',                    now(), now()),
    ('c0000000-0000-0000-0011-000000000004', 'invoice.void',     'Void an invoice',                      now(), now()),

    -- payment.*
    ('c0000000-0000-0000-0012-000000000001', 'payment.read',     'Read payment records',                 now(), now()),
    ('c0000000-0000-0000-0012-000000000002', 'payment.record',   'Record a payment (capture/refund)',    now(), now()),

    -- report.*
    ('c0000000-0000-0000-0013-000000000001', 'report.read',      'Read financial and operational reports', now(), now()),
    ('c0000000-0000-0000-0013-000000000002', 'report.export',    'Export reports',                       now(), now())
ON CONFLICT (id) DO NOTHING;

-- ---------------------------------------------------------------------------
-- ROLE → PERMISSION MATRIX
--
-- super_admin    : all permissions
-- tenant_admin   : all except tenant.approve, role/permission management
-- branch_admin   : branch-scoped ops (no tenant mgmt, no finance reports)
-- finance        : invoice read/create/update/void, payment read/record, report read/export
-- therapist      : own availability (CRUD), booking read/checkin/complete
-- customer       : customer.read (own), booking.read/create/cancel
--
-- Implementation: insert all; ON CONFLICT DO NOTHING for idempotency.
-- ---------------------------------------------------------------------------

-- Helper: map role name → UUID and permission code → UUID inline using CTEs
-- (avoids hard-coding UUIDs twice; readable and maintainable)

-- ---- super_admin: all permissions ----
INSERT INTO role_permission (role_id, permission_id)
SELECT
    'b0000000-0000-0000-0000-000000000001'::uuid AS role_id,
    p.id                                           AS permission_id
FROM permission p
ON CONFLICT DO NOTHING;

-- ---- tenant_admin: everything except tenant.approve, role/permission mgmt ----
INSERT INTO role_permission (role_id, permission_id)
SELECT
    'b0000000-0000-0000-0000-000000000002'::uuid,
    p.id
FROM permission p
WHERE p.code NOT IN (
    'tenant.approve',
    'role.create', 'role.update', 'role.delete',
    'permission.manage'
)
ON CONFLICT DO NOTHING;

-- ---- branch_admin: branch + user (branch scope) + customer + therapist + service + availability + booking ----
INSERT INTO role_permission (role_id, permission_id)
SELECT
    'b0000000-0000-0000-0000-000000000003'::uuid,
    p.id
FROM permission p
WHERE p.code IN (
    'branch.read',
    'user.read', 'user.create', 'user.update',
    'customer.read', 'customer.create', 'customer.update', 'customer.delete',
    'therapist.read', 'therapist.create', 'therapist.update', 'therapist.delete',
    'service.read', 'service.create', 'service.update', 'service.delete',
    'availability.read', 'availability.create', 'availability.update', 'availability.delete',
    'booking.read', 'booking.create', 'booking.update', 'booking.cancel',
    'booking.checkin', 'booking.complete',
    'invoice.read', 'invoice.create',
    'payment.read'
)
ON CONFLICT DO NOTHING;

-- ---- finance: invoices, payments, reports ----
INSERT INTO role_permission (role_id, permission_id)
SELECT
    'b0000000-0000-0000-0000-000000000004'::uuid,
    p.id
FROM permission p
WHERE p.code IN (
    'invoice.read', 'invoice.create', 'invoice.update', 'invoice.void',
    'payment.read', 'payment.record',
    'report.read', 'report.export',
    'customer.read',
    'booking.read'
)
ON CONFLICT DO NOTHING;

-- ---- therapist: own availability + assigned bookings ----
INSERT INTO role_permission (role_id, permission_id)
SELECT
    'b0000000-0000-0000-0000-000000000005'::uuid,
    p.id
FROM permission p
WHERE p.code IN (
    'availability.read', 'availability.create', 'availability.update', 'availability.delete',
    'booking.read', 'booking.checkin', 'booking.complete',
    'service.read'
)
ON CONFLICT DO NOTHING;

-- ---- customer: own profile + own bookings ----
INSERT INTO role_permission (role_id, permission_id)
SELECT
    'b0000000-0000-0000-0000-000000000006'::uuid,
    p.id
FROM permission p
WHERE p.code IN (
    'customer.read',
    'booking.read', 'booking.create', 'booking.cancel',
    'service.read',
    'therapist.read'
)
ON CONFLICT DO NOTHING;

-- ---------------------------------------------------------------------------
-- BOOTSTRAP PLATFORM SUPER ADMIN USER
--
-- tenant_id IS NULL  => platform-level user, not scoped to any tenant.
-- password_hash is a PLACEHOLDER — it will NOT authenticate.
-- After running migrations, update it:
--
--   UPDATE "user"
--   SET password_hash = '<argon2id_hash_of_your_password>'
--   WHERE id = 'a0000000-0000-0000-0000-000000000001';
--
-- Generate the hash with: https://argon2.online/ or the Go CLI:
--   go run ./cmd/argon2hash <password>
--
-- The initial password MUST be rotated on first login (enforced by auth-service).
-- ---------------------------------------------------------------------------
INSERT INTO "user" (
    id,
    tenant_id,
    email,
    password_hash,
    full_name,
    is_active,
    created_at,
    updated_at
)
VALUES (
    'a0000000-0000-0000-0000-000000000001',
    NULL,  -- platform super_admin has no tenant
    'superadmin@lustia.internal',
    -- PLACEHOLDER: this hash authenticates the literal string '__CHANGE_ME__'
    -- Replace before any real use. See comment above.
    '$argon2id$v=19$m=65536,t=3,p=4$AAAAAAAAAAAAAAAAAAAAAA$AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA',
    'Platform Super Admin',
    true,
    now(),
    now()
)
ON CONFLICT (id) DO NOTHING;

-- Assign super_admin role to the bootstrap user
INSERT INTO user_role (user_id, role_id)
VALUES (
    'a0000000-0000-0000-0000-000000000001',
    'b0000000-0000-0000-0000-000000000001'
)
ON CONFLICT DO NOTHING;
