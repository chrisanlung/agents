-- =============================================================================
-- Migration: 000007_update_bootstrap_email
-- Purpose  : Replace the unroutable bootstrap super-admin email
--            (superadmin@lustia.internal) with a local-loopback-friendly
--            placeholder (admin@lustia.local) that works with local SMTP
--            catchers (Mailpit / MailHog / MailCatcher) during development.
--
--            `.internal` is a reserved TLD (RFC 6762, RFC 8375) and will
--            never have public MX records, so mail sent to it disappears
--            silently — that was the Medium finding in SECURITY.md § 10.
--
-- Staging/prod gate (from SECURITY.md § 9.4):
--            Before any staging deploy, replace this with a real, monitored
--            mailbox via:
--              UPDATE "user"
--                 SET email = 'security@your-real-domain.tld'
--               WHERE id = 'a0000000-0000-0000-0000-000000000001';
--            Leave must_change_password = true.
-- =============================================================================

UPDATE "user"
   SET email = 'admin@lustia.local'
 WHERE id = 'a0000000-0000-0000-0000-000000000001'
   AND email = 'superadmin@lustia.internal';
