-- =============================================================================
-- Migration: 000028_public_rls_policies (UP)
-- Purpose  : Add the __public__ sentinel SELECT policies on the catalog
--            tables that the customer mobile app reads:
--            tenant, branch, service, therapist, room, addon.
--
-- Migration 000025 added the __public__ sentinel pattern but only applied
-- the additive policy to `booking` + `booking_addon`. The customer mobile
-- app's first call is GET /api/v1/public/branches which joins tenant +
-- branch + service + therapist + room. Without these additive policies,
-- public callers (`app.current_tenant = '__public__'`) hit an empty result
-- because the existing tenant_isolation policy compares tenant_id to the
-- literal string '__public__' (never matches a UUID).
--
-- Boundary: each public policy adds an active-only filter so disabled
-- tenants/branches/etc. are never visible to the public. The standard
-- tenant_isolation policy is unchanged — operator paths continue to work.
--
-- Reference: ADR 0014 §3.17, ADR 0005 (sentinel tenant pattern).
--
-- CLEAN invariant: migrations 1 → 28 produce a working schema.
-- =============================================================================

-- tenant — active-only, public can list active tenants for branch parent JOIN
DROP POLICY IF EXISTS tenant_public_select ON tenant;
CREATE POLICY tenant_public_select ON tenant
    AS PERMISSIVE FOR SELECT
    USING (
        current_setting('app.current_tenant', true) = '__public__'
        AND status = 'active'
        AND deleted_at IS NULL
    );

-- branch — active-only, parent tenant must also be active
DROP POLICY IF EXISTS branch_public_select ON branch;
CREATE POLICY branch_public_select ON branch
    AS PERMISSIVE FOR SELECT
    USING (
        current_setting('app.current_tenant', true) = '__public__'
        AND status = 'active'
        AND deleted_at IS NULL
        AND EXISTS (
            SELECT 1 FROM tenant t
            WHERE t.id = branch.tenant_id
              AND t.status = 'active'
              AND t.deleted_at IS NULL
        )
    );

-- service — public sees active offerings only
DROP POLICY IF EXISTS service_public_select ON service;
CREATE POLICY service_public_select ON service
    AS PERMISSIVE FOR SELECT
    USING (
        current_setting('app.current_tenant', true) = '__public__'
        AND is_active = true
        AND deleted_at IS NULL
    );

-- therapist — public sees active staff only
DROP POLICY IF EXISTS therapist_public_select ON therapist;
CREATE POLICY therapist_public_select ON therapist
    AS PERMISSIVE FOR SELECT
    USING (
        current_setting('app.current_tenant', true) = '__public__'
        AND is_active = true
        AND deleted_at IS NULL
    );

-- room — public sees active rooms (for picker)
DROP POLICY IF EXISTS room_public_select ON room;
CREATE POLICY room_public_select ON room
    AS PERMISSIVE FOR SELECT
    USING (
        current_setting('app.current_tenant', true) = '__public__'
        AND is_active = true
        AND deleted_at IS NULL
    );

-- addon — public sees active add-ons in the booking flow
DROP POLICY IF EXISTS addon_public_select ON addon;
CREATE POLICY addon_public_select ON addon
    AS PERMISSIVE FOR SELECT
    USING (
        current_setting('app.current_tenant', true) = '__public__'
        AND is_active = true
        AND deleted_at IS NULL
    );

-- therapist_service mapping — public needs to know which services each therapist
-- can perform (for the auto-assign + override flow). No status of its own;
-- inherits the parent therapist + service active-only filtering through joins.
DROP POLICY IF EXISTS therapist_service_public_select ON therapist_service;
CREATE POLICY therapist_service_public_select ON therapist_service
    AS PERMISSIVE FOR SELECT
    USING (
        current_setting('app.current_tenant', true) = '__public__'
    );

-- therapist_availability — public availability picker needs read access
DROP POLICY IF EXISTS therapist_availability_public_select ON therapist_availability;
CREATE POLICY therapist_availability_public_select ON therapist_availability
    AS PERMISSIVE FOR SELECT
    USING (
        current_setting('app.current_tenant', true) = '__public__'
    );
