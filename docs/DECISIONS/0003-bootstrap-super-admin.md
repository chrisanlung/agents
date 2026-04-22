# ADR 0003 — Bootstrap Super Admin: Placeholder Hash in Migration

**Status:** Accepted
**Date:** 2026-04-18
**Author:** db-designer

---

## Context

The platform needs at least one `super_admin` user to exist after a fresh migration so operators can log in and begin platform management. Three approaches were considered:

| Approach | Description |
|---|---|
| A. Env-interpolated migration | Migration contains `${SUPER_ADMIN_PASSWORD_HASH}` and a wrapper script substitutes it with `envsubst` before running `golang-migrate`. |
| B. Post-migration SQL step | Migration seeds the user with an unusable placeholder hash; operator runs a follow-up UPDATE with the real hash. |
| C. Out-of-band admin CLI | No seeded user in the migration; a separate `admin-cli` tool creates the first user interactively. |

---

## Decision

**Option B: post-migration SQL UPDATE step.**

The migration (`000005_seed_reference.up.sql`) inserts the bootstrap user with:
- A fixed UUID: `a0000000-0000-0000-0000-000000000001`
- A placeholder Argon2id hash that will not authenticate any real password
- `is_active = true`

After migrations run, the operator must execute:

```sql
UPDATE "user"
SET password_hash = '<real_argon2id_hash>'
WHERE id = 'a0000000-0000-0000-0000-000000000001';
```

The migration file itself contains clear instructions in a comment block.

---

## Why Option B over the alternatives

**Option A rejected: env-interpolated migration.**
`golang-migrate` does not natively support environment variable substitution in SQL files. Implementing `envsubst` or a wrapper script adds tooling complexity and creates a fragile coupling between the migration runner and the shell environment. It also means the migration file in version control contains a template variable, which looks broken to anyone reading it cold. The substituted output is not in VCS, making it harder to audit what was actually applied.

**Option C rejected: out-of-band CLI.**
An admin CLI is valuable for day-2 operations (reset password, create tenant) but adds scope to Phase 1. The goal of Phase 1 is to ship a working DB schema with minimal dependencies. Requiring a separate tool before the system is usable increases the operational surface area.

**Option B accepted: minimal, auditable, safe.**
- The placeholder hash (`$argon2id$v=19...AAAAAAA...`) is structurally valid but will never match any real password — the salt and digest are all zeros, which Argon2id will reject even if someone tries the literal string.
- The user account exists but is unusable until the operator explicitly sets a real password. This is a safe-fail default.
- The fixed UUID `a0000000-0000-0000-0000-000000000001` is stable across environments, making the update command copyable from runbooks without a lookup step.
- The approach is fully reversible by the `down.sql`.

---

## Operational runbook

After running `golang-migrate up`:

1. Generate an Argon2id hash of the desired initial password. In Go:
   ```go
   hash, _ := argon2id.CreateHash("YourSecurePassword", argon2id.DefaultParams)
   ```
   Or use an online tool (for local dev only): https://argon2.online/

2. Connect to the database and run:
   ```sql
   UPDATE "user"
   SET password_hash = '<hash_output>'
   WHERE id = 'a0000000-0000-0000-0000-000000000001';
   ```

3. Log in via `POST /auth/login` with `email: superadmin@lustia.internal` and the password you just set.

4. The auth-service should enforce `must_change_password` logic (flag for `go-expert`): consider adding a `must_change_password BOOLEAN` column in a future migration.

---

## Security considerations (flag for security-expert)

- The placeholder hash in version control is not a secret — it is intentionally unusable. However, ensure the VCS history is reviewed so that a real hash is never accidentally committed.
- The bootstrap email `superadmin@lustia.internal` is a non-routable internal address. Change it to a real monitored address before production deployment.
- Consider adding a `must_change_password` column to `"user"` to force password rotation on first login — this is not in Phase 1 scope but should be added before production launch.
- Secrets management for the production password should follow the project's secret manager policy (not `.env` files).
