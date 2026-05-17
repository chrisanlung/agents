-- Migration: 000037_payment_va_support
-- Purpose  : Add Virtual Account payment channel alongside QRIS (Jalur 3 — parallel).
--
-- iPaymu supports both QRIS (e-wallet QR) and VA (bank transfer) channels.
-- The customer mobile app now lets the customer pick at checkout. This
-- migration adds the columns needed to persist whichever channel was chosen
-- and the VA number/bank when VA is selected.
--
-- Existing rows (all created before this migration) default to channel='qris'
-- so historical lookups via qr_string still work.

ALTER TABLE payment_transaction
    ADD COLUMN channel    TEXT NOT NULL DEFAULT 'qris'
        CONSTRAINT chk_payment_transaction_channel
        CHECK (channel IN ('qris', 'va_bca', 'va_mandiri', 'va_bni', 'va_bri', 'va_permata', 'va_cimb')),
    ADD COLUMN va_number  TEXT,
    ADD COLUMN va_bank    TEXT
        CONSTRAINT chk_payment_transaction_va_bank
        CHECK (va_bank IS NULL OR va_bank IN ('bca', 'mandiri', 'bni', 'bri', 'permata', 'cimb'));

COMMENT ON COLUMN payment_transaction.channel IS
    'Payment channel selected at checkout. qris = QRIS QR code (e-wallet); '
    'va_<bank> = Virtual Account transfer. The customer mobile app shows a '
    'channel picker on the payment screen and stores the chosen value here. '
    'Existing rows default to qris (the only channel pre-migration 037).';

COMMENT ON COLUMN payment_transaction.va_number IS
    'Virtual Account number returned by the payment provider when '
    'channel = va_*. NULL for QRIS rows. Format depends on the bank '
    '(typically 12-16 digits, sometimes prefixed with a provider code).';

COMMENT ON COLUMN payment_transaction.va_bank IS
    'Bank code for VA payments — bca, mandiri, bni, bri, permata, cimb. '
    'NULL for QRIS rows. Mirrors the suffix of channel for VA rows.';

-- Drop the NOT NULL constraint on qr_string/qr_image_url is unnecessary —
-- they were already nullable per migration 000029.

-- Helpful partial index for status sweep / lookup by channel.
CREATE INDEX IF NOT EXISTS pt_channel_status_idx
    ON payment_transaction (channel, status)
    WHERE status IN ('awaiting', 'paid');
