-- =============================================================================
-- Migration: 000032_booking_payment_method_add_ipaymu (DOWN)
-- Purpose  : Revert booking.payment_method CHECK to Phase 5 values only
--            ('midtrans', 'paid_at_venue'). Removing 'ipaymu' and 'dummy'.
--
-- WARNING: This down migration will fail if any booking row already has
-- payment_method = 'ipaymu' or 'dummy'. Clean those rows first.
-- =============================================================================

ALTER TABLE booking
    DROP CONSTRAINT IF EXISTS chk_booking_payment_method;

ALTER TABLE booking
    ADD CONSTRAINT chk_booking_payment_method
        CHECK (
            payment_method IS NULL
            OR payment_method IN ('midtrans', 'paid_at_venue')
        );
