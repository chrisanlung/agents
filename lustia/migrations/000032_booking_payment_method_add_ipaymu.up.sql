-- =============================================================================
-- Migration: 000032_booking_payment_method_add_ipaymu (UP)
-- Purpose  : Extend booking.payment_method CHECK constraint to include
--            'ipaymu' (Phase 6 provider, ADR 0015) and 'dummy' (dev testing).
--
-- DB-designer flag #1 (docs/DATA_MODEL.md): the existing constraint only
-- allowed 'midtrans' and 'paid_at_venue'. Go-expert responsibility per
-- Phase 6 implementation brief.
--
-- Re-run safety: DROP … IF EXISTS before ALTER to make the migration
-- idempotent when re-applied on a fresh schema.
-- =============================================================================

ALTER TABLE booking
    DROP CONSTRAINT IF EXISTS chk_booking_payment_method;

ALTER TABLE booking
    ADD CONSTRAINT chk_booking_payment_method
        CHECK (
            payment_method IS NULL
            OR payment_method IN ('midtrans', 'paid_at_venue', 'ipaymu', 'dummy')
        );
