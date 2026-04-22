-- =============================================================================
-- Migration: 000003_operational_tables
-- Purpose  : customer, service, therapist, therapist_service,
--            therapist_availability, booking, invoice, payment, audit_log.
--            These tables all carry tenant_id and will receive RLS policies
--            in migration 000004.
-- =============================================================================

-- ---------------------------------------------------------------------------
-- 1. customer
--    Tenant-scoped customer records. Separate from "user" — customers are
--    external parties, not internal staff. A customer may also have a "user"
--    account in a later phase (Phase 5 public signup).
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS customer (
    id              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id       UUID        NOT NULL
                        CONSTRAINT fk_customer_tenant
                        REFERENCES tenant(id)
                        ON DELETE RESTRICT,
    full_name       TEXT        NOT NULL CHECK (char_length(full_name) BETWEEN 1 AND 200),
    email           CITEXT      NULL     CHECK (email IS NULL OR char_length(email::text) BETWEEN 3 AND 320),
    phone           TEXT        NULL     CHECK (phone IS NULL OR char_length(phone) <= 30),
    date_of_birth   DATE        NULL,
    gender          TEXT        NULL     CHECK (gender IS NULL OR gender IN ('male', 'female', 'other', 'prefer_not_to_say')),
    -- Address (free-form; structured address deferred to later phase)
    address         TEXT        NULL,
    notes           TEXT        NULL,
    metadata        JSONB       NOT NULL DEFAULT '{}'::jsonb,
    -- Soft delete
    deleted_at      TIMESTAMPTZ NULL,
    -- Audit
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_by      UUID        NULL
                        CONSTRAINT fk_customer_created_by
                        REFERENCES "user"(id)
                        ON DELETE SET NULL,
    updated_by      UUID        NULL
                        CONSTRAINT fk_customer_updated_by
                        REFERENCES "user"(id)
                        ON DELETE SET NULL
);

-- Unique email per tenant (only when email is provided and not deleted)
CREATE UNIQUE INDEX IF NOT EXISTS customer_tenant_email_uidx
    ON customer (tenant_id, email)
    WHERE email IS NOT NULL AND deleted_at IS NULL;

-- Unique phone per tenant (only when phone is provided and not deleted)
CREATE UNIQUE INDEX IF NOT EXISTS customer_tenant_phone_uidx
    ON customer (tenant_id, phone)
    WHERE phone IS NOT NULL AND deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS customer_tenant_id_idx       ON customer (tenant_id);
CREATE INDEX IF NOT EXISTS customer_metadata_gin        ON customer USING GIN (metadata);

CREATE TRIGGER trg_customer_updated_at
    BEFORE UPDATE ON customer
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- ---------------------------------------------------------------------------
-- 2. service
--    An offering available for booking (e.g. "60-min Deep Tissue Massage").
--    branch_id NULL = tenant-wide offering; branch_id set = branch-specific.
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS service (
    id                  UUID            PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id           UUID            NOT NULL
                            CONSTRAINT fk_service_tenant
                            REFERENCES tenant(id)
                            ON DELETE RESTRICT,
    branch_id           UUID            NULL
                            CONSTRAINT fk_service_branch
                            REFERENCES branch(id)
                            ON DELETE RESTRICT,
    -- Human-readable slug, unique within tenant
    code                TEXT            NOT NULL CHECK (char_length(code) BETWEEN 1 AND 100),
    name                TEXT            NOT NULL CHECK (char_length(name) BETWEEN 1 AND 200),
    description         TEXT            NULL,
    duration_minutes    INT             NOT NULL CHECK (duration_minutes > 0 AND duration_minutes <= 1440),
    price               NUMERIC(12,2)   NOT NULL CHECK (price >= 0),
    currency            CHAR(3)         NOT NULL DEFAULT 'IDR',
    is_active           BOOLEAN         NOT NULL DEFAULT true,
    metadata            JSONB           NOT NULL DEFAULT '{}'::jsonb,
    -- Soft delete
    deleted_at          TIMESTAMPTZ     NULL,
    -- Audit
    created_at          TIMESTAMPTZ     NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ     NOT NULL DEFAULT now(),
    created_by          UUID            NULL
                            CONSTRAINT fk_service_created_by
                            REFERENCES "user"(id)
                            ON DELETE SET NULL,
    updated_by          UUID            NULL
                            CONSTRAINT fk_service_updated_by
                            REFERENCES "user"(id)
                            ON DELETE SET NULL
);

CREATE UNIQUE INDEX IF NOT EXISTS service_tenant_code_uidx
    ON service (tenant_id, code)
    WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS service_tenant_id_idx        ON service (tenant_id);
CREATE INDEX IF NOT EXISTS service_branch_id_idx        ON service (branch_id);
CREATE INDEX IF NOT EXISTS service_is_active_idx        ON service (is_active) WHERE deleted_at IS NULL;

CREATE TRIGGER trg_service_updated_at
    BEFORE UPDATE ON service
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- ---------------------------------------------------------------------------
-- 3. therapist
--    Therapist profile. Not every therapist needs a platform login (walk-in
--    clinics may manage therapists without portal accounts), hence user_id
--    is nullable.
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS therapist (
    id              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id       UUID        NOT NULL
                        CONSTRAINT fk_therapist_tenant
                        REFERENCES tenant(id)
                        ON DELETE RESTRICT,
    -- Optional link to a "user" account (e.g. therapist can log in)
    user_id         UUID        NULL
                        CONSTRAINT fk_therapist_user
                        REFERENCES "user"(id)
                        ON DELETE SET NULL,
    full_name       TEXT        NOT NULL CHECK (char_length(full_name) BETWEEN 1 AND 200),
    gender          TEXT        NULL     CHECK (gender IS NULL OR gender IN ('male', 'female', 'other')),
    bio             TEXT        NULL,
    photo_url       TEXT        NULL     CHECK (photo_url IS NULL OR char_length(photo_url) <= 2048),
    -- Array of specialty tags; promotes to dedicated table if queried heavily
    specialties     JSONB       NOT NULL DEFAULT '[]'::jsonb,
    is_active       BOOLEAN     NOT NULL DEFAULT true,
    metadata        JSONB       NOT NULL DEFAULT '{}'::jsonb,
    -- Soft delete
    deleted_at      TIMESTAMPTZ NULL,
    -- Audit
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_by      UUID        NULL
                        CONSTRAINT fk_therapist_created_by
                        REFERENCES "user"(id)
                        ON DELETE SET NULL,
    updated_by      UUID        NULL
                        CONSTRAINT fk_therapist_updated_by
                        REFERENCES "user"(id)
                        ON DELETE SET NULL
);

CREATE INDEX IF NOT EXISTS therapist_tenant_id_idx      ON therapist (tenant_id);
CREATE INDEX IF NOT EXISTS therapist_user_id_idx        ON therapist (user_id);
CREATE INDEX IF NOT EXISTS therapist_is_active_idx      ON therapist (is_active) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS therapist_specialties_gin    ON therapist USING GIN (specialties);

CREATE TRIGGER trg_therapist_updated_at
    BEFORE UPDATE ON therapist
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- ---------------------------------------------------------------------------
-- 4. therapist_service
--    Junction: which services a therapist can deliver.
--    Both therapist and service must belong to the same tenant.
--    Cross-tenant enforcement: fk to therapist.tenant_id = service.tenant_id
--    is structural (both FKs exist) but not a FOREIGN KEY across two tables.
--    Application layer is the primary guard; a DB trigger can be added if
--    needed (see open questions in DATA_MODEL.md).
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS therapist_service (
    therapist_id    UUID        NOT NULL
                        CONSTRAINT fk_therapist_service_therapist
                        REFERENCES therapist(id)
                        ON DELETE CASCADE,
    service_id      UUID        NOT NULL
                        CONSTRAINT fk_therapist_service_service
                        REFERENCES service(id)
                        ON DELETE CASCADE,
    -- Audit
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_by      UUID        NULL
                        CONSTRAINT fk_therapist_service_created_by
                        REFERENCES "user"(id)
                        ON DELETE SET NULL,

    CONSTRAINT pk_therapist_service PRIMARY KEY (therapist_id, service_id)
);

CREATE INDEX IF NOT EXISTS therapist_service_service_id_idx ON therapist_service (service_id);

-- ---------------------------------------------------------------------------
-- 5. therapist_availability
--    Recurring weekly availability windows for a therapist at a branch.
--
--    Overlap prevention strategy:
--    A GiST exclusion constraint prevents two rows for the same therapist +
--    branch + day_of_week from having overlapping time ranges.
--    We normalize start_time/end_time onto an arbitrary fixed date (2000-01-01)
--    to form a TSRANGE for the gist operator, since TIME does not support
--    gist natively.
--
--    Rationale: a DB constraint is cheaper than an app-side SELECT-then-INSERT
--    race condition, especially when multiple branch_admins could be editing
--    schedules concurrently. See ADR 0002.
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS therapist_availability (
    id                  UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id           UUID        NOT NULL
                            CONSTRAINT fk_therapist_avail_tenant
                            REFERENCES tenant(id)
                            ON DELETE RESTRICT,
    therapist_id        UUID        NOT NULL
                            CONSTRAINT fk_therapist_avail_therapist
                            REFERENCES therapist(id)
                            ON DELETE CASCADE,
    branch_id           UUID        NOT NULL
                            CONSTRAINT fk_therapist_avail_branch
                            REFERENCES branch(id)
                            ON DELETE RESTRICT,
    day_of_week         SMALLINT    NOT NULL CHECK (day_of_week BETWEEN 0 AND 6),  -- 0=Sunday
    start_time          TIME        NOT NULL,
    end_time            TIME        NOT NULL,
    -- Validity window; null effective_until = indefinite
    effective_from      DATE        NOT NULL DEFAULT CURRENT_DATE,
    effective_until     DATE        NULL,
    -- Audit
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_by          UUID        NULL
                            CONSTRAINT fk_therapist_avail_created_by
                            REFERENCES "user"(id)
                            ON DELETE SET NULL,
    updated_by          UUID        NULL
                            CONSTRAINT fk_therapist_avail_updated_by
                            REFERENCES "user"(id)
                            ON DELETE SET NULL,

    CONSTRAINT chk_availability_times CHECK (end_time > start_time),
    CONSTRAINT chk_availability_dates CHECK (effective_until IS NULL OR effective_until >= effective_from),

    -- Prevent overlapping windows for the same therapist + branch + day.
    -- TSRANGE is used because TIME has no native gist operator class.
    -- The fixed date 2000-01-01 is a normalisation trick with no semantic meaning.
    EXCLUDE USING gist (
        therapist_id WITH =,
        branch_id    WITH =,
        day_of_week  WITH =,
        tsrange(
            ('2000-01-01'::date + start_time)::timestamp,
            ('2000-01-01'::date + end_time)::timestamp
        ) WITH &&
    )
);

CREATE INDEX IF NOT EXISTS therapist_avail_tenant_id_idx    ON therapist_availability (tenant_id);
CREATE INDEX IF NOT EXISTS therapist_avail_therapist_id_idx ON therapist_availability (therapist_id);
CREATE INDEX IF NOT EXISTS therapist_avail_branch_id_idx    ON therapist_availability (branch_id);
CREATE INDEX IF NOT EXISTS therapist_avail_day_idx          ON therapist_availability (therapist_id, day_of_week);

CREATE TRIGGER trg_therapist_avail_updated_at
    BEFORE UPDATE ON therapist_availability
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- ---------------------------------------------------------------------------
-- 6. booking
--    Core booking entity. One row = one appointment.
--
--    Double-booking prevention:
--    A GiST exclusion constraint on (therapist_id, TSTZRANGE) prevents two
--    bookings for the same therapist from overlapping in time, ignoring
--    cancelled/no_show rows. See ADR 0002.
--
--    booking_code is a human-readable reference (e.g. "BKG-20240418-0001")
--    generated by the application; unique per tenant.
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS booking (
    id                  UUID            PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id           UUID            NOT NULL
                            CONSTRAINT fk_booking_tenant
                            REFERENCES tenant(id)
                            ON DELETE RESTRICT,
    branch_id           UUID            NOT NULL
                            CONSTRAINT fk_booking_branch
                            REFERENCES branch(id)
                            ON DELETE RESTRICT,
    -- Human-readable code, generated by app (e.g. BKG-20240418-001)
    booking_code        TEXT            NOT NULL CHECK (char_length(booking_code) BETWEEN 1 AND 50),
    customer_id         UUID            NOT NULL
                            CONSTRAINT fk_booking_customer
                            REFERENCES customer(id)
                            ON DELETE RESTRICT,
    therapist_id        UUID            NULL        -- nullable: unassigned at creation
                            CONSTRAINT fk_booking_therapist
                            REFERENCES therapist(id)
                            ON DELETE RESTRICT,
    service_id          UUID            NOT NULL
                            CONSTRAINT fk_booking_service
                            REFERENCES service(id)
                            ON DELETE RESTRICT,
    scheduled_start     TIMESTAMPTZ     NOT NULL,
    scheduled_end       TIMESTAMPTZ     NOT NULL,
    status              booking_status  NOT NULL DEFAULT 'draft',
    source              booking_source  NOT NULL DEFAULT 'walk_in',
    notes               TEXT            NULL,
    created_by          UUID            NULL
                            CONSTRAINT fk_booking_created_by
                            REFERENCES "user"(id)
                            ON DELETE SET NULL,
    -- Lifecycle timestamps
    assigned_at         TIMESTAMPTZ     NULL,
    started_at          TIMESTAMPTZ     NULL,
    ended_at            TIMESTAMPTZ     NULL,
    cancelled_at        TIMESTAMPTZ     NULL,
    cancellation_reason TEXT            NULL,
    metadata            JSONB           NOT NULL DEFAULT '{}'::jsonb,
    -- Audit
    created_at          TIMESTAMPTZ     NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ     NOT NULL DEFAULT now(),
    updated_by          UUID            NULL
                            CONSTRAINT fk_booking_updated_by
                            REFERENCES "user"(id)
                            ON DELETE SET NULL,

    CONSTRAINT chk_booking_times    CHECK (scheduled_end > scheduled_start),
    CONSTRAINT chk_booking_cancel   CHECK (
        (status = 'cancelled' AND cancelled_at IS NOT NULL) OR
        (status != 'cancelled')
    )
);

-- Unique booking code per tenant
CREATE UNIQUE INDEX IF NOT EXISTS booking_tenant_code_uidx
    ON booking (tenant_id, booking_code);

-- Double-booking prevention: same therapist cannot have overlapping bookings
-- (excludes cancelled and no_show rows where overbooking is irrelevant).
-- EXCLUDE USING gist requires btree_gist for the therapist_id (UUID) equality.
ALTER TABLE booking
    ADD CONSTRAINT booking_no_double_book
    EXCLUDE USING gist (
        therapist_id WITH =,
        tstzrange(scheduled_start, scheduled_end) WITH &&
    )
    WHERE (
        therapist_id IS NOT NULL
        AND status NOT IN ('cancelled', 'no_show')
    );

-- Query pattern indexes
CREATE INDEX IF NOT EXISTS booking_tenant_branch_start_idx
    ON booking (tenant_id, branch_id, scheduled_start);

CREATE INDEX IF NOT EXISTS booking_tenant_customer_idx
    ON booking (tenant_id, customer_id);

CREATE INDEX IF NOT EXISTS booking_tenant_therapist_start_idx
    ON booking (tenant_id, therapist_id, scheduled_start)
    WHERE therapist_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS booking_status_idx
    ON booking (tenant_id, status);

CREATE INDEX IF NOT EXISTS booking_metadata_gin
    ON booking USING GIN (metadata);

CREATE TRIGGER trg_booking_updated_at
    BEFORE UPDATE ON booking
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- ---------------------------------------------------------------------------
-- 7. invoice
--    Financial document attached to a booking (or standalone for ad-hoc).
--    invoice_number is a human-readable reference unique per tenant.
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS invoice (
    id              UUID            PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id       UUID            NOT NULL
                        CONSTRAINT fk_invoice_tenant
                        REFERENCES tenant(id)
                        ON DELETE RESTRICT,
    branch_id       UUID            NOT NULL
                        CONSTRAINT fk_invoice_branch
                        REFERENCES branch(id)
                        ON DELETE RESTRICT,
    invoice_number  TEXT            NOT NULL CHECK (char_length(invoice_number) BETWEEN 1 AND 50),
    -- Nullable: ad-hoc invoices may not be tied to a specific booking
    booking_id      UUID            NULL
                        CONSTRAINT fk_invoice_booking
                        REFERENCES booking(id)
                        ON DELETE RESTRICT,
    customer_id     UUID            NOT NULL
                        CONSTRAINT fk_invoice_customer
                        REFERENCES customer(id)
                        ON DELETE RESTRICT,
    -- Financials
    subtotal        NUMERIC(12,2)   NOT NULL CHECK (subtotal >= 0),
    discount        NUMERIC(12,2)   NOT NULL DEFAULT 0 CHECK (discount >= 0),
    tax             NUMERIC(12,2)   NOT NULL DEFAULT 0 CHECK (tax >= 0),
    total           NUMERIC(12,2)   NOT NULL CHECK (total >= 0),
    currency        CHAR(3)         NOT NULL DEFAULT 'IDR',
    status          invoice_status  NOT NULL DEFAULT 'draft',
    -- Lifecycle timestamps
    issued_at       TIMESTAMPTZ     NULL,
    due_at          TIMESTAMPTZ     NULL,
    paid_at         TIMESTAMPTZ     NULL,
    metadata        JSONB           NOT NULL DEFAULT '{}'::jsonb,
    -- Audit
    created_at      TIMESTAMPTZ     NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ     NOT NULL DEFAULT now(),
    created_by      UUID            NULL
                        CONSTRAINT fk_invoice_created_by
                        REFERENCES "user"(id)
                        ON DELETE SET NULL,
    updated_by      UUID            NULL
                        CONSTRAINT fk_invoice_updated_by
                        REFERENCES "user"(id)
                        ON DELETE SET NULL
);

CREATE UNIQUE INDEX IF NOT EXISTS invoice_tenant_number_uidx
    ON invoice (tenant_id, invoice_number);

CREATE INDEX IF NOT EXISTS invoice_tenant_id_idx        ON invoice (tenant_id);
CREATE INDEX IF NOT EXISTS invoice_branch_id_idx        ON invoice (branch_id);
CREATE INDEX IF NOT EXISTS invoice_booking_id_idx       ON invoice (booking_id);
CREATE INDEX IF NOT EXISTS invoice_customer_id_idx      ON invoice (customer_id);
CREATE INDEX IF NOT EXISTS invoice_status_idx           ON invoice (tenant_id, status);
CREATE INDEX IF NOT EXISTS invoice_issued_at_idx        ON invoice (tenant_id, issued_at);

CREATE TRIGGER trg_invoice_updated_at
    BEFORE UPDATE ON invoice
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- ---------------------------------------------------------------------------
-- 8. payment
--    One payment row = one payment attempt or capture.
--    An invoice can have multiple payment rows (partial payments, retries).
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS payment (
    id                  UUID            PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id           UUID            NOT NULL
                            CONSTRAINT fk_payment_tenant
                            REFERENCES tenant(id)
                            ON DELETE RESTRICT,
    branch_id           UUID            NOT NULL
                            CONSTRAINT fk_payment_branch
                            REFERENCES branch(id)
                            ON DELETE RESTRICT,
    invoice_id          UUID            NOT NULL
                            CONSTRAINT fk_payment_invoice
                            REFERENCES invoice(id)
                            ON DELETE RESTRICT,
    amount              NUMERIC(12,2)   NOT NULL CHECK (amount > 0),
    currency            CHAR(3)         NOT NULL DEFAULT 'IDR',
    method              payment_method  NOT NULL,
    gateway_reference   TEXT            NULL     CHECK (gateway_reference IS NULL OR char_length(gateway_reference) <= 500),
    status              payment_status  NOT NULL DEFAULT 'pending',
    paid_at             TIMESTAMPTZ     NULL,
    metadata            JSONB           NOT NULL DEFAULT '{}'::jsonb,
    -- Audit
    created_at          TIMESTAMPTZ     NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ     NOT NULL DEFAULT now(),
    created_by          UUID            NULL
                            CONSTRAINT fk_payment_created_by
                            REFERENCES "user"(id)
                            ON DELETE SET NULL,
    updated_by          UUID            NULL
                            CONSTRAINT fk_payment_updated_by
                            REFERENCES "user"(id)
                            ON DELETE SET NULL
);

CREATE INDEX IF NOT EXISTS payment_tenant_id_idx        ON payment (tenant_id);
CREATE INDEX IF NOT EXISTS payment_branch_id_idx        ON payment (branch_id);
CREATE INDEX IF NOT EXISTS payment_invoice_id_idx       ON payment (invoice_id);
CREATE INDEX IF NOT EXISTS payment_status_idx           ON payment (tenant_id, status);
CREATE INDEX IF NOT EXISTS payment_paid_at_idx          ON payment (tenant_id, paid_at);

CREATE TRIGGER trg_payment_updated_at
    BEFORE UPDATE ON payment
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- ---------------------------------------------------------------------------
-- 9. audit_log
--    Append-only event log for compliance and debugging.
--    Covers both platform-level events (tenant_id NULL) and
--    tenant-scoped events.
--    No updated_at, no soft delete — immutable by design.
--    Not subject to tenant RLS (platform-managed; super_admin reads all).
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS audit_log (
    id              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id       UUID        NULL        -- NULL for platform-level events
                        CONSTRAINT fk_audit_log_tenant
                        REFERENCES tenant(id)
                        ON DELETE RESTRICT,
    actor_user_id   UUID        NULL        -- NULL for system actions
                        CONSTRAINT fk_audit_log_user
                        REFERENCES "user"(id)
                        ON DELETE SET NULL,
    action          TEXT        NOT NULL CHECK (char_length(action) BETWEEN 1 AND 100),
    resource_type   TEXT        NOT NULL CHECK (char_length(resource_type) BETWEEN 1 AND 100),
    resource_id     TEXT        NULL     CHECK (resource_id IS NULL OR char_length(resource_id) <= 100),
    -- Arbitrary metadata: before/after state, request context, etc.
    meta            JSONB       NOT NULL DEFAULT '{}'::jsonb,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS audit_log_tenant_id_idx      ON audit_log (tenant_id);
CREATE INDEX IF NOT EXISTS audit_log_actor_user_id_idx  ON audit_log (actor_user_id);
CREATE INDEX IF NOT EXISTS audit_log_resource_idx       ON audit_log (resource_type, resource_id);
CREATE INDEX IF NOT EXISTS audit_log_created_at_idx     ON audit_log (created_at);
CREATE INDEX IF NOT EXISTS audit_log_meta_gin           ON audit_log USING GIN (meta);
