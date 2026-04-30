-- =============================================================================
-- Migration: 000029_phase6_payment_lifecycle (UP)
-- Purpose  : Phase 6 — Payment + settlement lifecycle tables.
--            Implements ADR 0015 §2.3 (schema), §2.1 (three-tier money lifecycle),
--            §2.4 (platform fee), §2.6 (manual disbursement).
--
-- Tables created (in FK-safe order):
--   settlement_batch    — platform-level iPaymu daily settlement record.
--   tenant_disbursement — per-tenant weekly payout (Phase 6: manual).
--   payment_transaction — one row per booking; references both tables above.
--
-- Extensions required:
--   pgcrypto — gen_random_uuid() — already created in migration 000001.
--
-- CLEAN invariant: migrations 1 → 29 produce a working schema.
--
-- Re-run safety:
--   CREATE TABLE uses IF NOT EXISTS.
--   CREATE INDEX uses IF NOT EXISTS.
--   Policy and trigger creation is preceded by DROP … IF EXISTS.
--   GRANT is idempotent.
-- =============================================================================

-- =============================================================================
-- TABLE: settlement_batch
--
-- Platform-level record of each iPaymu daily settlement report.
-- No tenant_id — this is a cross-tenant platform concern.
-- RLS: __platform__ sentinel only (mirrors tenant_registration, migration 11).
--
-- One row is created per reconciliation run (manual button "Tarik laporan iPaymu"
-- in platform-admin, ADR 0015 §2.9). Multiple payment_transaction rows are then
-- bulk-updated with settlement_batch_id and status = settled.
-- =============================================================================

CREATE TABLE IF NOT EXISTS settlement_batch (
    id                  UUID        NOT NULL DEFAULT gen_random_uuid(),

    -- Provider that originated this settlement
    provider            TEXT        NOT NULL,

    -- Timestamp from the provider's settlement report (not the reconciliation time)
    settled_at          TIMESTAMPTZ NOT NULL,

    -- Aggregate totals over all transactions in this batch
    total_amount_idr    BIGINT      NOT NULL,
    transaction_count   INT         NOT NULL,

    -- Raw provider settlement payload (for audit and discrepancy investigation)
    raw_payload         JSONB       NULL,

    -- Platform audit
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_by          UUID        NULL,   -- platform admin who triggered; SET NULL on user deletion

    -- ---------- PRIMARY KEY ----------
    CONSTRAINT pk_settlement_batch PRIMARY KEY (id),

    -- ---------- FOREIGN KEYS ----------
    CONSTRAINT fk_sb_created_by
        FOREIGN KEY (created_by) REFERENCES "user"(id)
        ON DELETE SET NULL,

    -- ---------- CHECK CONSTRAINTS ----------
    CONSTRAINT chk_sb_provider
        CHECK (provider IN ('dummy', 'ipaymu', 'midtrans')),

    CONSTRAINT chk_sb_total_amount_non_negative
        CHECK (total_amount_idr >= 0),

    CONSTRAINT chk_sb_transaction_count_non_negative
        CHECK (transaction_count >= 0)
);

COMMENT ON TABLE settlement_batch IS
    'Platform-level record of each provider (iPaymu) daily settlement report. '
    'No tenant_id — cross-tenant platform concern. One row per reconciliation run; '
    'referenced by payment_transaction.settlement_batch_id. '
    'RLS: __platform__ sentinel only, mirroring tenant_registration pattern. '
    'See ADR 0015 §2.9.';

COMMENT ON COLUMN settlement_batch.settled_at IS
    'Timestamp from the provider''s settlement report, not this row''s created_at. '
    'iPaymu H+1 settlement can range H+1 to H+3 depending on '
    'weekend/holiday (ADR 0015 §4 open question 1).';

COMMENT ON COLUMN settlement_batch.raw_payload IS
    'Provider''s full settlement report payload verbatim. Kept for discrepancy '
    'investigation (ADR 0015 §2.9). Sensitive financial data — mask in logs.';

-- ---------------------------------------------------------------------------
-- INDEXES
-- ---------------------------------------------------------------------------

-- Reconciliation history list (platform-admin "Settlement & Payout" page)
CREATE INDEX IF NOT EXISTS sb_provider_settled_at_idx
    ON settlement_batch (provider, settled_at DESC);

-- FK support for created_by
CREATE INDEX IF NOT EXISTS sb_created_by_idx
    ON settlement_batch (created_by)
    WHERE created_by IS NOT NULL;

-- ---------------------------------------------------------------------------
-- ROW-LEVEL SECURITY — platform-only (mirrors tenant_registration, migration 11)
-- ---------------------------------------------------------------------------

ALTER TABLE settlement_batch ENABLE ROW LEVEL SECURITY;
ALTER TABLE settlement_batch FORCE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS sb_platform_only ON settlement_batch;
CREATE POLICY sb_platform_only ON settlement_batch
    AS PERMISSIVE FOR ALL
    USING (
        current_setting('app.current_tenant', true) = '__platform__'
    )
    WITH CHECK (
        current_setting('app.current_tenant', true) = '__platform__'
    );

-- ---------------------------------------------------------------------------
-- GRANT
-- No UPDATE: settlement batches are immutable once created.
-- No DELETE: financial audit records must not be deleted.
-- ---------------------------------------------------------------------------

GRANT SELECT, INSERT ON settlement_batch TO lustia_app;

-- =============================================================================
-- TABLE: tenant_disbursement
--
-- Per-tenant weekly payout record. Phase 6: manual bank transfer by Lustia
-- platform-admin. Phase 7+: auto-disbursement via banking API (Flip / BRI).
--
-- Status machine (ADR 0015 §2.6):
--   pending     → processing  (admin starts payout)
--   processing  → transferred (admin marks done after bank transfer)
--   processing  → failed      (bank rejects transfer)
--   failed      → processing  (admin retries)
--   pending     → cancelled   (admin cancels before processing)
--
-- Once transferred: all related payment_transaction rows updated to
--   status = disbursed, disbursed_at = now() in the same DB transaction.
-- =============================================================================

CREATE TABLE IF NOT EXISTS tenant_disbursement (
    id                  UUID        NOT NULL DEFAULT gen_random_uuid(),

    tenant_id           UUID        NOT NULL,

    -- Disbursement period (inclusive on both ends; ADR 0015 §2.5)
    period_start        DATE        NOT NULL,
    period_end          DATE        NOT NULL,

    -- Aggregate amounts over all settled transactions in this period
    gross_amount_idr    BIGINT      NOT NULL,
    platform_fee_idr    BIGINT      NOT NULL,
    net_amount_idr      BIGINT      NOT NULL,  -- gross - platform_fee
    transaction_count   INT         NOT NULL,

    -- Lifecycle
    status              TEXT        NOT NULL DEFAULT 'pending',

    -- Transfer metadata (populated when transferred)
    bank_reference      TEXT        NULL,
    notes               TEXT        NULL,
    transferred_at      TIMESTAMPTZ NULL,
    transferred_by      UUID        NULL,   -- platform admin; SET NULL on user deletion

    -- Standard audit
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now(),

    -- ---------- PRIMARY KEY ----------
    CONSTRAINT pk_tenant_disbursement PRIMARY KEY (id),

    -- ---------- FOREIGN KEYS ----------
    CONSTRAINT fk_td_tenant
        FOREIGN KEY (tenant_id) REFERENCES tenant(id)
        ON DELETE RESTRICT,

    CONSTRAINT fk_td_transferred_by
        FOREIGN KEY (transferred_by) REFERENCES "user"(id)
        ON DELETE SET NULL,

    -- ---------- CHECK CONSTRAINTS ----------
    CONSTRAINT chk_td_status
        CHECK (status IN (
            'pending', 'processing', 'transferred', 'failed', 'cancelled'
        )),

    CONSTRAINT chk_td_period_order
        CHECK (period_end >= period_start),

    CONSTRAINT chk_td_gross_non_negative
        CHECK (gross_amount_idr >= 0),

    CONSTRAINT chk_td_fee_non_negative
        CHECK (platform_fee_idr >= 0),

    CONSTRAINT chk_td_net_non_negative
        CHECK (net_amount_idr >= 0),

    CONSTRAINT chk_td_transaction_count_non_negative
        CHECK (transaction_count >= 0),

    CONSTRAINT chk_td_notes_length
        CHECK (notes IS NULL OR char_length(notes) <= 2000)
);

COMMENT ON TABLE tenant_disbursement IS
    'Per-tenant weekly payout record. Phase 6: manual bank transfer by Lustia '
    'platform-admin. Phase 7+: auto-disbursement via banking API. '
    'Status machine: pending → processing → transferred; or → failed → processing '
    '(retry); or pending → cancelled. Once transferred, all related '
    'payment_transaction rows are updated to status=disbursed in the same DB '
    'transaction. See ADR 0015 §2.5, §2.6.';

COMMENT ON COLUMN tenant_disbursement.net_amount_idr IS
    'gross_amount_idr - platform_fee_idr. Computed by the service layer at '
    'disbursement creation time. Immutable after row is created.';

COMMENT ON COLUMN tenant_disbursement.bank_reference IS
    'Bank transfer confirmation number populated by platform-admin when marking '
    'status=transferred. Optional but recommended for reconciliation.';

-- ---------------------------------------------------------------------------
-- INDEXES
-- ---------------------------------------------------------------------------

-- Tenant disbursement history (tenant-admin "Riwayat Pencairan")
CREATE INDEX IF NOT EXISTS td_tenant_period_start_idx
    ON tenant_disbursement (tenant_id, period_start DESC);

-- Admin work queue: open payouts needing action
CREATE INDEX IF NOT EXISTS td_status_partial_idx
    ON tenant_disbursement (status, created_at)
    WHERE status IN ('pending', 'processing');

-- RLS predicate acceleration
CREATE INDEX IF NOT EXISTS td_tenant_id_idx
    ON tenant_disbursement (tenant_id);

-- FK support
CREATE INDEX IF NOT EXISTS td_transferred_by_idx
    ON tenant_disbursement (transferred_by)
    WHERE transferred_by IS NOT NULL;

-- ---------------------------------------------------------------------------
-- ROW-LEVEL SECURITY
-- ---------------------------------------------------------------------------

ALTER TABLE tenant_disbursement ENABLE ROW LEVEL SECURITY;
ALTER TABLE tenant_disbursement FORCE ROW LEVEL SECURITY;

-- Standard tenant SELECT
DROP POLICY IF EXISTS td_tenant_select ON tenant_disbursement;
CREATE POLICY td_tenant_select ON tenant_disbursement
    AS PERMISSIVE FOR SELECT
    USING (tenant_id::text = current_setting('app.current_tenant', true));

-- Standard tenant INSERT
DROP POLICY IF EXISTS td_tenant_insert ON tenant_disbursement;
CREATE POLICY td_tenant_insert ON tenant_disbursement
    AS PERMISSIVE FOR INSERT
    WITH CHECK (tenant_id::text = current_setting('app.current_tenant', true));

-- Standard tenant UPDATE (status transitions by platform-admin acting as tenant)
DROP POLICY IF EXISTS td_tenant_update ON tenant_disbursement;
CREATE POLICY td_tenant_update ON tenant_disbursement
    AS PERMISSIVE FOR UPDATE
    USING  (tenant_id::text = current_setting('app.current_tenant', true))
    WITH CHECK (tenant_id::text = current_setting('app.current_tenant', true));

-- No DELETE policy. lustia_app has no DELETE grant on tenant_disbursement.

-- ---------------------------------------------------------------------------
-- GRANT
-- ---------------------------------------------------------------------------

GRANT SELECT, INSERT, UPDATE ON tenant_disbursement TO lustia_app;

-- ---------------------------------------------------------------------------
-- TRIGGER: keep updated_at current
-- ---------------------------------------------------------------------------

DROP TRIGGER IF EXISTS trg_td_updated_at ON tenant_disbursement;
CREATE TRIGGER trg_td_updated_at
    BEFORE UPDATE ON tenant_disbursement
    FOR EACH ROW
    EXECUTE FUNCTION set_updated_at();

-- =============================================================================
-- TABLE: payment_transaction
--
-- One row per booking. Tracks the full money lifecycle from customer QR payment
-- through iPaymu settlement to Lustia → tenant disbursement.
--
-- Status machine (ADR 0015 §2.1):
--   awaiting  → paid      (webhook confirms customer payment)
--   paid      → settled   (settlement_batch reconciliation run)
--   settled   → disbursed (tenant_disbursement transferred)
--   awaiting  → expired   (qr_expires_at passed; sweep job)
--   awaiting  → failed    (webhook reports payment failure)
--   any live  → voided    (ops override, pre-settlement only)
--
-- Invariant (Phase 6): UNIQUE (booking_id) — non-partial.
--   One booking = one payment_transaction, no exceptions.
--
-- No DELETE grant: status transitions carry lifecycle.
-- =============================================================================

CREATE TABLE IF NOT EXISTS payment_transaction (
    -- Identity
    id                      UUID        NOT NULL DEFAULT gen_random_uuid(),

    -- Tenant context — denormalised from booking for fast per-tenant queries
    -- without joining through booking on every finance list view.
    tenant_id               UUID        NOT NULL,

    -- Booking linkage
    booking_id              UUID        NOT NULL,

    -- Provider identification
    provider                TEXT        NOT NULL,
    provider_reference      TEXT        NOT NULL,  -- iPaymu trx id; idempotency key for webhooks

    -- QR display data (kept after expiry/payment for audit trail)
    qr_string               TEXT        NULL,
    qr_image_url            TEXT        NULL,
    qr_expires_at           TIMESTAMPTZ NOT NULL,  -- 15-min hard cap from creation (ADR 0015 §2.7)

    -- Amounts in whole Rupiah (IDR has no decimal subunit in practice)
    expected_amount_idr     BIGINT      NOT NULL,
    received_amount_idr     BIGINT      NULL,       -- populated on paid webhook
    platform_fee_idr        BIGINT      NULL,       -- floor(received * 0.05); set at settlement
    tenant_net_idr          BIGINT      NULL,       -- received - platform_fee; immutable after settled

    -- Lifecycle status
    status                  TEXT        NOT NULL DEFAULT 'awaiting',

    -- Transition timestamps
    paid_at                 TIMESTAMPTZ NULL,
    settled_at              TIMESTAMPTZ NULL,
    disbursed_at            TIMESTAMPTZ NULL,

    -- Cross-table linkage (NULLable FK; set on transition)
    settlement_batch_id     UUID        NULL,
    disbursement_id         UUID        NULL,

    -- Audit
    raw_webhook             JSONB       NULL,   -- last inbound webhook payload for ops audit
    created_at              TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at              TIMESTAMPTZ NOT NULL DEFAULT now(),

    -- ---------- PRIMARY KEY ----------
    CONSTRAINT pk_payment_transaction PRIMARY KEY (id),

    -- ---------- FOREIGN KEYS ----------
    CONSTRAINT fk_pt_tenant
        FOREIGN KEY (tenant_id) REFERENCES tenant(id)
        ON DELETE RESTRICT,

    CONSTRAINT fk_pt_booking
        FOREIGN KEY (booking_id) REFERENCES booking(id)
        ON DELETE CASCADE,

    CONSTRAINT fk_pt_settlement_batch
        FOREIGN KEY (settlement_batch_id) REFERENCES settlement_batch(id)
        ON DELETE RESTRICT,

    CONSTRAINT fk_pt_disbursement
        FOREIGN KEY (disbursement_id) REFERENCES tenant_disbursement(id)
        ON DELETE RESTRICT,

    -- ---------- CHECK CONSTRAINTS ----------
    CONSTRAINT chk_pt_status
        CHECK (status IN (
            'awaiting', 'paid', 'settled', 'failed', 'expired', 'disbursed', 'voided'
        )),

    CONSTRAINT chk_pt_provider
        CHECK (provider IN ('dummy', 'ipaymu', 'midtrans')),

    CONSTRAINT chk_pt_expected_amount_positive
        CHECK (expected_amount_idr > 0),

    CONSTRAINT chk_pt_received_amount_non_negative
        CHECK (received_amount_idr IS NULL OR received_amount_idr >= 0),

    CONSTRAINT chk_pt_platform_fee_non_negative
        CHECK (platform_fee_idr IS NULL OR platform_fee_idr >= 0),

    CONSTRAINT chk_pt_tenant_net_non_negative
        CHECK (tenant_net_idr IS NULL OR tenant_net_idr >= 0),

    CONSTRAINT chk_pt_provider_reference_not_empty
        CHECK (char_length(provider_reference) > 0)
);

COMMENT ON TABLE payment_transaction IS
    'One row per booking. Full money-lifecycle record from customer QR payment '
    'through iPaymu settlement to Lustia → tenant disbursement. '
    'Status machine: awaiting → paid → settled → disbursed; or → failed/expired/voided. '
    'Phase 6 invariant: UNIQUE (booking_id) — one transaction per booking. '
    'platform_fee_idr = floor(received_amount_idr * 0.05), computed at settlement '
    '(ADR 0015 §2.4). See ADR 0015 §2.1, §2.3.';

COMMENT ON COLUMN payment_transaction.provider_reference IS
    'Provider-assigned transaction identifier (e.g. iPaymu trx id). '
    'Used as idempotency key for webhook processing — same reference arriving '
    'twice is a no-op. UNIQUE constraint prevents double-credit.';

COMMENT ON COLUMN payment_transaction.qr_expires_at IS
    'Hard expiry for the QR code, 15 minutes from creation (ADR 0015 §2.7). '
    'Expiry sweep transitions status awaiting → expired after this timestamp. '
    'Value is not cleared after expiry — kept for audit trail.';

COMMENT ON COLUMN payment_transaction.platform_fee_idr IS
    'Lustia platform fee (5% flat), computed at settlement time (not payment time). '
    'Formula: floor(received_amount_idr * 0.05). Immutable once status=settled. '
    'Phase 7 may introduce per-tenant overrides via tenant.platform_fee_pct.';

COMMENT ON COLUMN payment_transaction.raw_webhook IS
    'Last inbound webhook payload verbatim. Updated on each delivery of the same '
    'provider_reference (idempotent — same txn, refreshed payload). '
    'Sensitive financial data — must be masked in application logs.';

-- ---------------------------------------------------------------------------
-- UNIQUE INDEXES
-- ---------------------------------------------------------------------------

-- Phase 6 invariant: one booking → one payment_transaction. NON-PARTIAL.
CREATE UNIQUE INDEX IF NOT EXISTS pt_booking_id_uidx
    ON payment_transaction (booking_id);

-- Webhook idempotency: same provider_reference must not credit twice.
CREATE UNIQUE INDEX IF NOT EXISTS pt_provider_reference_uidx
    ON payment_transaction (provider_reference);

-- ---------------------------------------------------------------------------
-- INDEXES
-- ---------------------------------------------------------------------------

-- RLS predicate + tenant finance list views (all statuses)
CREATE INDEX IF NOT EXISTS pt_tenant_id_idx
    ON payment_transaction (tenant_id);

-- Tenant balance queries: rows where money is in-flight (paid or settled)
CREATE INDEX IF NOT EXISTS pt_tenant_status_partial_idx
    ON payment_transaction (tenant_id, status)
    WHERE status IN ('paid', 'settled');

-- Expiry sweep: awaiting transactions past their QR expiry
CREATE INDEX IF NOT EXISTS pt_expiry_sweep_idx
    ON payment_transaction (status, qr_expires_at)
    WHERE status = 'awaiting';

-- FK support (Postgres does not auto-index FKs)
CREATE INDEX IF NOT EXISTS pt_settlement_batch_id_idx
    ON payment_transaction (settlement_batch_id)
    WHERE settlement_batch_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS pt_disbursement_id_idx
    ON payment_transaction (disbursement_id)
    WHERE disbursement_id IS NOT NULL;

-- ---------------------------------------------------------------------------
-- ROW-LEVEL SECURITY
--
-- Standard tenant isolation + additive __public__ SELECT policy.
-- Public read is for the customer polling endpoint (ADR 0015 §2.7):
--   GET /api/v1/public/bookings/:code/payment-status
-- Service layer MUST narrow to a single row via WHERE booking_id = <resolved_id>.
-- No public INSERT: service layer sets app.current_tenant = <tenant_uuid> first.
-- ---------------------------------------------------------------------------

ALTER TABLE payment_transaction ENABLE ROW LEVEL SECURITY;
ALTER TABLE payment_transaction FORCE ROW LEVEL SECURITY;

-- Standard tenant SELECT
DROP POLICY IF EXISTS pt_tenant_select ON payment_transaction;
CREATE POLICY pt_tenant_select ON payment_transaction
    AS PERMISSIVE FOR SELECT
    USING (tenant_id::text = current_setting('app.current_tenant', true));

-- Public SELECT: customer polling reads status by booking_id.
-- Service layer MUST add WHERE booking_id = ? — no ListPayments on public path.
DROP POLICY IF EXISTS pt_public_select ON payment_transaction;
CREATE POLICY pt_public_select ON payment_transaction
    AS PERMISSIVE FOR SELECT
    USING (
        current_setting('app.current_tenant', true) = '__public__'
        AND EXISTS (
            SELECT 1 FROM booking b
            JOIN   branch  br ON br.id = b.branch_id
            JOIN   tenant  t  ON t.id  = b.tenant_id
            WHERE  b.id = payment_transaction.booking_id
              AND  br.deleted_at IS NULL
              AND  br.status = 'active'
              AND  t.status  = 'active'
        )
    );

-- Standard tenant INSERT
DROP POLICY IF EXISTS pt_tenant_insert ON payment_transaction;
CREATE POLICY pt_tenant_insert ON payment_transaction
    AS PERMISSIVE FOR INSERT
    WITH CHECK (tenant_id::text = current_setting('app.current_tenant', true));

-- Standard tenant UPDATE (webhook, settlement, disbursement flows)
DROP POLICY IF EXISTS pt_tenant_update ON payment_transaction;
CREATE POLICY pt_tenant_update ON payment_transaction
    AS PERMISSIVE FOR UPDATE
    USING  (tenant_id::text = current_setting('app.current_tenant', true))
    WITH CHECK (tenant_id::text = current_setting('app.current_tenant', true));

-- No DELETE policy. lustia_app has no DELETE grant on payment_transaction.

-- ---------------------------------------------------------------------------
-- GRANT
-- ---------------------------------------------------------------------------

GRANT SELECT, INSERT, UPDATE ON payment_transaction TO lustia_app;

-- ---------------------------------------------------------------------------
-- TRIGGER: keep updated_at current
-- ---------------------------------------------------------------------------

DROP TRIGGER IF EXISTS trg_pt_updated_at ON payment_transaction;
CREATE TRIGGER trg_pt_updated_at
    BEFORE UPDATE ON payment_transaction
    FOR EACH ROW
    EXECUTE FUNCTION set_updated_at();
