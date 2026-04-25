-- =============================================================================
-- Migration: 000025_phase5_bookings (UP)
-- Purpose  : Phase 5 — Booking engine core tables.
--            Implements ADR 0014 §3.15 (schema) + ADR 0002 (exclusion
--            constraint design) + ADR 0014 §3.17 (RLS).
--
-- Tables created:
--   booking       — one row per appointment reservation.
--   booking_addon — snapshot of add-ons selected at booking time.
--
-- Extensions:
--   btree_gist — already created in migration 000001. Needed for EXCLUDE
--   USING gist with UUID (non-range) equality. NOT re-created here.
--
-- Lifecycle note (ADR 0014 §3.13):
--   No deleted_at column. Booking lifecycle is entirely expressed through
--   status. Cancelled/expired/no_show rows persist forever for audit.
--
-- CLEAN invariant: migrations 1 → 25 produce a working schema.
--
-- Re-run safety:
--   CREATE TABLE uses IF NOT EXISTS.
--   CREATE INDEX uses IF NOT EXISTS.
--   Policy and trigger creation is preceded by DROP … IF EXISTS.
--   Permission INSERTs use ON CONFLICT (id) DO NOTHING.
-- =============================================================================

-- ---------------------------------------------------------------------------
-- Drop the Phase 1 stub booking schema (migration 000003).
--
-- Migration 000003 created an early `booking` + `booking_addon` shape with a
-- `customer_id` FK to a `customer` table, `booking_status` / `booking_source`
-- enums, and different column names (`booking_code` vs `code`). Phase 5 (ADR
-- 0014 §3.2) removes the customer account model — booking carries customer
-- info inline. The schemas are incompatible enough that an ALTER chain would
-- be longer + more error-prone than a fresh CREATE. Pre-Phase-5 has no
-- production booking data anywhere, so the drop is safe.
-- ---------------------------------------------------------------------------

DROP TABLE IF EXISTS booking_addon CASCADE;
DROP TABLE IF EXISTS booking CASCADE;
-- Phase 1 enums are no longer referenced after the table is gone; leave them
-- in place — booking_status / booking_source could be reused in future. They
-- do not collide with the new TEXT-based status CHECK constraint below.

-- ---------------------------------------------------------------------------
-- TABLE: booking
--
-- Core appointment record. Each row represents one booking by one customer
-- for one service at one branch. The customer supplies their name/phone/email
-- directly on the booking row — there is no customer account (ADR 0014 §3.2).
--
-- No deleted_at: status field ('cancelled', 'expired') replaces soft-delete.
-- No DELETE grant: lustia_app may SELECT, INSERT, UPDATE only.
-- ---------------------------------------------------------------------------

CREATE TABLE IF NOT EXISTS booking (
    -- Identity
    id                  UUID        NOT NULL DEFAULT gen_random_uuid(),

    -- Tenant + location context (required)
    tenant_id           UUID        NOT NULL,
    branch_id           UUID        NOT NULL,
    service_id          UUID        NOT NULL,

    -- Assignable resources (auto-assigned by service layer; may be NULL if
    -- no resource is available and assignment is deferred — should not reach
    -- a terminal state without being set)
    room_id             UUID        NULL,
    therapist_id        UUID        NULL,

    -- Customer info — captured at booking time, no customer account required
    customer_name       TEXT        NOT NULL,
    customer_phone      TEXT        NOT NULL,
    customer_email      TEXT        NOT NULL,

    -- Human-readable booking code: format "XXXX-XXXX" (9 chars incl. hyphen)
    -- Crockford Base32 character set [A-Z2-7]. Used for QR + ops check-in.
    code                TEXT        NOT NULL,

    -- Scheduled slot
    scheduled_start     TIMESTAMPTZ NOT NULL,
    scheduled_end       TIMESTAMPTZ NOT NULL,

    -- Payment
    total_price_idr     BIGINT      NOT NULL,
    payment_method      TEXT        NULL,
    payment_reference   TEXT        NULL,   -- snap_token or Midtrans txn_id
    paid_at             TIMESTAMPTZ NULL,

    -- Lifecycle audit columns (set on state transitions)
    status              TEXT        NOT NULL DEFAULT 'pending_payment',
    cancelled_at        TIMESTAMPTZ NULL,
    cancelled_by        UUID        NULL,
    cancel_reason       TEXT        NULL,
    checked_in_at       TIMESTAMPTZ NULL,
    checked_in_by       UUID        NULL,
    completed_at        TIMESTAMPTZ NULL,
    completed_by        UUID        NULL,

    -- Standard audit
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now(),

    -- ---------- PRIMARY KEY ----------
    CONSTRAINT pk_booking PRIMARY KEY (id),

    -- ---------- FOREIGN KEYS ----------
    CONSTRAINT fk_booking_tenant
        FOREIGN KEY (tenant_id) REFERENCES tenant(id)
        ON DELETE RESTRICT,

    CONSTRAINT fk_booking_branch
        FOREIGN KEY (branch_id) REFERENCES branch(id)
        ON DELETE RESTRICT,

    CONSTRAINT fk_booking_service
        FOREIGN KEY (service_id) REFERENCES service(id)
        ON DELETE RESTRICT,

    CONSTRAINT fk_booking_room
        FOREIGN KEY (room_id) REFERENCES room(id)
        ON DELETE RESTRICT,

    CONSTRAINT fk_booking_therapist
        FOREIGN KEY (therapist_id) REFERENCES therapist(id)
        ON DELETE RESTRICT,

    CONSTRAINT fk_booking_cancelled_by
        FOREIGN KEY (cancelled_by) REFERENCES "user"(id)
        ON DELETE SET NULL,

    CONSTRAINT fk_booking_checked_in_by
        FOREIGN KEY (checked_in_by) REFERENCES "user"(id)
        ON DELETE SET NULL,

    CONSTRAINT fk_booking_completed_by
        FOREIGN KEY (completed_by) REFERENCES "user"(id)
        ON DELETE SET NULL,

    -- ---------- CHECK CONSTRAINTS ----------

    -- Slot validity
    CONSTRAINT chk_booking_end_after_start
        CHECK (scheduled_end > scheduled_start),

    -- Customer info lengths
    CONSTRAINT chk_booking_customer_name_length
        CHECK (char_length(customer_name) BETWEEN 1 AND 200),

    CONSTRAINT chk_booking_customer_phone_length
        CHECK (char_length(customer_phone) BETWEEN 5 AND 30),

    CONSTRAINT chk_booking_customer_email_length
        CHECK (char_length(customer_email) <= 320),

    -- Basic structural email check: must contain @ with at least one char on each side
    CONSTRAINT chk_booking_customer_email_shape
        CHECK (customer_email ~ '^[^@\s]+@[^@\s]+\.[^@\s]+$'),

    -- Booking code: exactly 9 characters, format "XXXX-XXXX"
    CONSTRAINT chk_booking_code_format
        CHECK (code ~ '^[A-Z2-7]{4}-[A-Z2-7]{4}$'),

    -- Price non-negative
    CONSTRAINT chk_booking_total_price_non_negative
        CHECK (total_price_idr >= 0),

    -- Status valid values
    CONSTRAINT chk_booking_status
        CHECK (status IN (
            'pending_payment',
            'paid',
            'checked_in',
            'completed',
            'expired',
            'cancelled',
            'no_show'
        )),

    -- Payment method valid values (NULL = not yet set / pending)
    CONSTRAINT chk_booking_payment_method
        CHECK (payment_method IS NULL OR payment_method IN ('midtrans', 'paid_at_venue')),

    -- Cancel reason length
    CONSTRAINT chk_booking_cancel_reason_length
        CHECK (cancel_reason IS NULL OR char_length(cancel_reason) <= 1000),

    -- ---------- EXCLUSION CONSTRAINTS (ADR 0002) ----------
    --
    -- These constraints prevent double-booking at the database level,
    -- providing atomic enforcement regardless of which code path creates
    -- the booking. btree_gist is required (already in migration 000001)
    -- to apply GiST equality operators on UUID columns.
    --
    -- Partial predicate includes 'pending_payment' as a soft-lock during
    -- the 15-minute payment window (ADR 0014 §3.13). A lazy expiry sweep
    -- transitions pending_payment → expired after 15min, releasing the slot.
    --
    -- 'expired', 'cancelled', 'no_show' are excluded from the predicate so
    -- those terminal-state rows do not block future bookings of the same slot.

    -- Therapist double-booking prevention
    CONSTRAINT excl_booking_therapist_no_overlap
        EXCLUDE USING gist (
            therapist_id WITH =,
            tstzrange(scheduled_start, scheduled_end) WITH &&
        )
        WHERE (
            therapist_id IS NOT NULL
            AND status IN ('pending_payment', 'paid', 'checked_in', 'completed')
        ),

    -- Room double-booking prevention
    CONSTRAINT excl_booking_room_no_overlap
        EXCLUDE USING gist (
            room_id WITH =,
            tstzrange(scheduled_start, scheduled_end) WITH &&
        )
        WHERE (
            room_id IS NOT NULL
            AND status IN ('pending_payment', 'paid', 'checked_in', 'completed')
        )
);

COMMENT ON TABLE booking IS
    'Core appointment record for Phase 5 booking engine. One row per customer '
    'booking. Customer info is captured inline (no customer account). Status '
    'lifecycle: pending_payment → paid → checked_in → completed; or → expired '
    '/ cancelled / no_show. No deleted_at — status field carries lifecycle. '
    'See ADR 0014 §3.13 and ADR 0002 (exclusion constraint design).';

COMMENT ON COLUMN booking.code IS
    'Human-readable booking code in format "XXXX-XXXX" (Crockford Base32, '
    '9 chars including hyphen). Generated by service layer using crypto/rand. '
    'Used for QR display and ops check-in lookup. UNIQUE across all bookings — '
    'cancelled/expired rows retain their code for audit trail. See ADR 0014 §3.4.';

COMMENT ON COLUMN booking.payment_reference IS
    'For midtrans: the Snap token returned by CreateTransaction. '
    'For paid_at_venue: the manual reference entered by ops (may be NULL). '
    'For the real Midtrans adapter: stores the Midtrans order_id / transaction_id.';

COMMENT ON COLUMN booking.total_price_idr IS
    'Snapshot of service price + sum of selected addon prices at booking time, '
    'in whole Rupiah. Stored as BIGINT to match service.price and addon.price_idr. '
    'This value does not change even if service/addon prices are updated later.';

COMMENT ON COLUMN booking.room_id IS
    'NULL = no room assigned (or assignment deferred). The booking engine '
    'auto-assigns the lowest sort_order available room. Customer may override.';

COMMENT ON COLUMN booking.therapist_id IS
    'NULL = no therapist assigned (or assignment deferred). The booking engine '
    'auto-assigns the lowest sort_order available therapist. Customer may override.';

COMMENT ON COLUMN booking.cancelled_by IS
    'UUID of the operator (user) who cancelled the booking. NULL if cancelled by '
    'system (expiry sweep). FK SET NULL on user deletion preserves audit trail.';

-- ---------------------------------------------------------------------------
-- UNIQUE INDEX: code
--
-- The booking code must be globally unique (one lookup table for all tenants).
-- No partial WHERE clause: cancelled/expired rows still own their code so the
-- QR printed on old receipts cannot be re-used for a different booking.
-- ---------------------------------------------------------------------------

CREATE UNIQUE INDEX IF NOT EXISTS booking_code_uidx
    ON booking (code);

-- ---------------------------------------------------------------------------
-- INDEXES
-- ---------------------------------------------------------------------------

-- Primary list scan: "bookings at this branch today/this week"
CREATE INDEX IF NOT EXISTS booking_tenant_branch_start_idx
    ON booking (tenant_id, branch_id, scheduled_start);

-- Lazy expiry sweep + cron query: "all pending_payment bookings before cutoff"
CREATE INDEX IF NOT EXISTS booking_status_start_idx
    ON booking (status, scheduled_start);

-- RLS predicate acceleration
CREATE INDEX IF NOT EXISTS booking_tenant_id_idx
    ON booking (tenant_id);

-- FK support (Postgres does not auto-index FKs)
CREATE INDEX IF NOT EXISTS booking_branch_id_idx
    ON booking (branch_id);

CREATE INDEX IF NOT EXISTS booking_service_id_idx
    ON booking (service_id);

CREATE INDEX IF NOT EXISTS booking_therapist_id_idx
    ON booking (therapist_id);

CREATE INDEX IF NOT EXISTS booking_room_id_idx
    ON booking (room_id);

-- ---------------------------------------------------------------------------
-- ROW-LEVEL SECURITY
--
-- Standard tenant isolation + additive public-read policy for customer
-- endpoints using the '__public__' sentinel (ADR 0014 §3.17, mirrors
-- '__platform__' from ADR 0005).
--
-- PUBLIC READ CAVEAT:
--   The public SELECT policy permits reads when current_tenant = '__public__'
--   AND the booking's branch is active AND the tenant is active. However,
--   this would expose ALL booking rows under '__public__' if queried without
--   a WHERE clause. The service layer MUST add a WHERE code = ? predicate
--   before executing any public booking query. The RLS policy cannot enforce
--   this by itself — it is a service-layer contract, documented here and in
--   DATA_MODEL.md §Phase 5.
--
-- PUBLIC INSERT:
--   Not permitted under '__public__'. The service layer resolves the
--   tenant_id from branch_id, then sets app.current_tenant = <tenant_uuid>
--   for the INSERT transaction. The standard tenant INSERT policy applies.
-- ---------------------------------------------------------------------------

ALTER TABLE booking ENABLE ROW LEVEL SECURITY;
ALTER TABLE booking FORCE ROW LEVEL SECURITY;

-- Standard tenant SELECT
DROP POLICY IF EXISTS booking_tenant_select ON booking;
CREATE POLICY booking_tenant_select ON booking
    AS PERMISSIVE FOR SELECT
    USING (tenant_id::text = current_setting('app.current_tenant', true));

-- Public SELECT (customer code-lookup endpoint)
-- Additive: passes when __public__ sentinel is set. Service layer narrows to
-- single row via WHERE code = ?.
DROP POLICY IF EXISTS booking_public_select ON booking;
CREATE POLICY booking_public_select ON booking
    AS PERMISSIVE FOR SELECT
    USING (
        current_setting('app.current_tenant', true) = '__public__'
        AND EXISTS (
            SELECT 1 FROM branch  b
            JOIN   tenant t ON t.id = b.tenant_id
            WHERE  b.id = booking.branch_id
              AND  b.deleted_at IS NULL
              AND  b.status = 'active'
              AND  t.status = 'active'
        )
    );

-- Standard tenant INSERT
DROP POLICY IF EXISTS booking_tenant_insert ON booking;
CREATE POLICY booking_tenant_insert ON booking
    AS PERMISSIVE FOR INSERT
    WITH CHECK (tenant_id::text = current_setting('app.current_tenant', true));

-- Standard tenant UPDATE
DROP POLICY IF EXISTS booking_tenant_update ON booking;
CREATE POLICY booking_tenant_update ON booking
    AS PERMISSIVE FOR UPDATE
    USING  (tenant_id::text = current_setting('app.current_tenant', true))
    WITH CHECK (tenant_id::text = current_setting('app.current_tenant', true));

-- No DELETE policy — lustia_app has no DELETE grant on booking.

-- ---------------------------------------------------------------------------
-- GRANT
-- ---------------------------------------------------------------------------

GRANT SELECT, INSERT, UPDATE ON booking TO lustia_app;

-- ---------------------------------------------------------------------------
-- TRIGGER: keep updated_at current
-- ---------------------------------------------------------------------------

DROP TRIGGER IF EXISTS trg_booking_updated_at ON booking;
CREATE TRIGGER trg_booking_updated_at
    BEFORE UPDATE ON booking
    FOR EACH ROW
    EXECUTE FUNCTION set_updated_at();

-- =============================================================================
-- TABLE: booking_addon
--
-- Snapshot of add-ons selected at booking creation. price_idr is captured
-- at booking time — changes to addon.price_idr do not retroactively affect
-- existing bookings. PK is composite (booking_id, addon_id): one row per
-- selected add-on per booking; duplicates are rejected by the PK.
--
-- No updated_at: this table is insert-only. Add-on selection is locked at
-- booking creation time (no add/remove after booking is created).
-- No DELETE grant: to remove an add-on from a booking, the booking would
-- need to be cancelled and recreated (Phase 5 UX simplification).
-- =============================================================================

CREATE TABLE IF NOT EXISTS booking_addon (
    booking_id  UUID    NOT NULL,
    addon_id    UUID    NOT NULL,
    price_idr   BIGINT  NOT NULL,

    CONSTRAINT pk_booking_addon PRIMARY KEY (booking_id, addon_id),

    CONSTRAINT fk_booking_addon_booking
        FOREIGN KEY (booking_id) REFERENCES booking(id)
        ON DELETE CASCADE,

    CONSTRAINT fk_booking_addon_addon
        FOREIGN KEY (addon_id) REFERENCES addon(id)
        ON DELETE RESTRICT,

    CONSTRAINT chk_booking_addon_price_non_negative
        CHECK (price_idr >= 0)
);

COMMENT ON TABLE booking_addon IS
    'Add-ons selected at booking creation. price_idr is a point-in-time snapshot '
    '— addon prices may change after a booking is created; the snapshotted value '
    'is what the customer paid for. ON DELETE CASCADE from booking: if a booking '
    'row is physically deleted (migrations/admin only), its add-ons go with it. '
    'addon FK is RESTRICT: an add-on that has booking_addon rows cannot be hard-deleted. '
    'See ADR 0014 §3.15.';

COMMENT ON COLUMN booking_addon.price_idr IS
    'Snapshot of addon.price_idr at booking creation time, in whole Rupiah. '
    'Immutable after insert. Summed with service price to form booking.total_price_idr.';

-- FK support index (Postgres does not auto-index FKs)
CREATE INDEX IF NOT EXISTS booking_addon_addon_id_idx
    ON booking_addon (addon_id);

-- ---------------------------------------------------------------------------
-- RLS on booking_addon
--
-- Duplicate tenant_id policy rather than joining through booking, because
-- a join-through policy would require booking rows to pass their own RLS
-- first — creating a dependency that is harder to reason about. Instead,
-- we resolve the tenant from booking directly in the USING clause.
--
-- The same __public__ sentinel logic applies: public callers can read
-- booking_addon rows for bookings that their public SELECT policy allows.
-- The service layer is responsible for scoping the query to a specific code.
-- ---------------------------------------------------------------------------

ALTER TABLE booking_addon ENABLE ROW LEVEL SECURITY;
ALTER TABLE booking_addon FORCE ROW LEVEL SECURITY;

-- Standard tenant SELECT via booking
DROP POLICY IF EXISTS booking_addon_tenant_select ON booking_addon;
CREATE POLICY booking_addon_tenant_select ON booking_addon
    AS PERMISSIVE FOR SELECT
    USING (
        EXISTS (
            SELECT 1 FROM booking b
            WHERE b.id = booking_addon.booking_id
              AND b.tenant_id::text = current_setting('app.current_tenant', true)
        )
    );

-- Public SELECT (same caveat as booking — service layer must scope to a code)
DROP POLICY IF EXISTS booking_addon_public_select ON booking_addon;
CREATE POLICY booking_addon_public_select ON booking_addon
    AS PERMISSIVE FOR SELECT
    USING (
        current_setting('app.current_tenant', true) = '__public__'
        AND EXISTS (
            SELECT 1 FROM booking b
            JOIN   branch  br ON br.id = b.branch_id
            JOIN   tenant  t  ON t.id  = b.tenant_id
            WHERE  b.id = booking_addon.booking_id
              AND  br.deleted_at IS NULL
              AND  br.status = 'active'
              AND  t.status  = 'active'
        )
    );

-- Standard tenant INSERT (RLS routes through booking.tenant_id)
DROP POLICY IF EXISTS booking_addon_tenant_insert ON booking_addon;
CREATE POLICY booking_addon_tenant_insert ON booking_addon
    AS PERMISSIVE FOR INSERT
    WITH CHECK (
        EXISTS (
            SELECT 1 FROM booking b
            WHERE b.id = booking_addon.booking_id
              AND b.tenant_id::text = current_setting('app.current_tenant', true)
        )
    );

-- No UPDATE, no DELETE policy — add-ons are immutable after booking creation.
-- lustia_app has only SELECT + INSERT granted below.

-- ---------------------------------------------------------------------------
-- GRANT
-- ---------------------------------------------------------------------------

GRANT SELECT, INSERT ON booking_addon TO lustia_app;
