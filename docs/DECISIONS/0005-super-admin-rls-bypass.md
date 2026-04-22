# ADR 0005 — Super-Admin RLS Bypass: `__platform__` Sentinel Over `lustia_super_admin` DB Role

- **Status:** Accepted
- **Date:** 2026-04-18
- **Deciders:** security-expert, db-designer, go-expert (implementation owner)

---

## Context

The `super_admin` role needs to read and write across all tenants — approving tenant registrations, viewing platform-wide data, and managing the user lifecycle across all tenant boundaries. The PostgreSQL RLS policies that protect every operational table block cross-tenant reads by design. The question is how to grant `super_admin` requests the access they legitimately require without creating a mechanism that can be misused or that introduces connection-pool complexity.

Two concrete options were evaluated (a third — no RLS at all for super_admin — was rejected immediately as it removes a critical defense layer):

---

## Options Considered

### Option A: `lustia_super_admin` DB Role with `BYPASSRLS`

Create a separate PostgreSQL role with the `BYPASSRLS` attribute. When the auth middleware identifies a super_admin request (JWT has `tenant_id: null`), it either:

- (A1) Switches the session role via `SET ROLE lustia_super_admin` at the start of the request transaction and `RESET ROLE` at the end, or
- (A2) Maintains a second connection pool authenticated as `lustia_super_admin`.

**Pros:**
- RLS bypass is enforced entirely at the DB layer with no application-level sentinel value to forge.
- Clean separation: `lustia_app` always has RLS enforced; `lustia_super_admin` always bypasses it.

**Cons (A1 — SET ROLE per request):**
- A missed `RESET ROLE` on connection return to the pool causes all subsequent requests on that connection to run as `lustia_super_admin` with `BYPASSRLS`. This is a critical privilege escalation bug whose trigger condition (a panic or early-return before RESET) is easy to hit and hard to detect in review. The blast radius: every tenant's data is readable/writable until the connection is cycled.
- Requires careful deferred cleanup (`defer db.Exec("RESET ROLE")`) on every request that touches this path. A single missing defer = silent data exposure.
- Not compatible with GORM's connection pool management without custom hooks.

**Cons (A2 — separate pool):**
- Doubles connection pool management complexity in `common-configs` and in the auth-service wiring.
- `go-expert` must implement pool selection logic that keys on a JWT claim at the repository layer — a cross-cutting concern that bleeds into the domain layer.
- Two pools mean two sets of credentials, two sets of rotation events, and two log streams to correlate.

### Option B: `__platform__` Sentinel Value in `app.current_tenant` (chosen)

When the auth middleware identifies a super_admin request (JWT has `tenant_id: null`), it sets `SET LOCAL app.current_tenant = '__platform__'` (the same `SET LOCAL` call used for all other requests). The RLS policies on tables with nullable `tenant_id` (the `"user"` table) include an additional `OR` branch:

```sql
(tenant_id IS NULL AND current_setting('app.current_tenant', true) = '__platform__')
```

All connections use the single `lustia_app` DB role. There is no `BYPASSRLS` in application traffic paths.

**Pros:**
- Single connection pool. No pool selection logic in repositories.
- The safe-fail behavior is preserved: if `app.current_tenant` is not set, all row checks fail (empty result set, not data leak). This is true for both regular and super_admin paths.
- The sentinel value is validated upstream (in the JWT middleware) before `SET LOCAL` is called. A regular-tenant JWT cannot set `__platform__` because the middleware reads `tenant_id` from the verified JWT claim, not from the request body.
- Operationally simpler: one DB role, one credential to rotate, one connection pool to monitor.

**Cons:**
- The sentinel is an application-level contract. If a super_admin JWT is forged (compromised private key), the attacker can cross tenant boundaries. Mitigation: this is the same threat model as all JWT-based auth — a compromised private key enables account takeover regardless of the bypass mechanism. The key rotation procedure in `SECURITY.md` Section 9.2 is the correct response.
- The `__platform__` string must never appear as a real tenant slug. Enforce: the `tenant.slug` validation rule must reject `__platform__` as a reserved value. Flag for `go-expert`: add a `checkReservedSlugs` validator that blocks `__platform__`, `null`, `undefined`, `system`, `admin` as tenant slugs.
- The RLS policy `OR` branch slightly increases policy complexity. This is a one-time cost at schema definition time.

---

## Decision

**Use Option B: the `__platform__` sentinel value for super_admin application traffic.**

The `lustia_migrator` DB role with `BYPASSRLS` is retained for DDL migrations only (no application traffic ever uses this role).

A dedicated `lustia_super_admin` DB role with `BYPASSRLS` is not created for application traffic. Option A is rejected because the `SET ROLE` / `RESET ROLE` pattern creates a latent privilege escalation vulnerability whose trigger condition is a normal exception-handling edge case in Go (panic, early return). The sentinel approach has no equivalent silent-failure mode.

---

## Consequences

**Positive:**
- Single `lustia_app` connection pool simplifies `go-expert`'s repository implementation.
- No new DB credentials to manage or rotate.
- Safe-fail behavior (empty result on missing `app.current_tenant`) applies uniformly to all connections.

**Negative / trade-off:**
- The sentinel `__platform__` is an application-level gate, not a DB-level gate. A JWT private key compromise enables cross-tenant access. This risk exists under either option; Option B does not make it worse.
- All RLS policies on `"user"` (and any future table with nullable `tenant_id`) must include the sentinel branch. Future `db-designer` work must remember this when adding new cross-tenant tables.

**Follow-up work:**
- `go-expert`: add `__platform__` (and similar reserved strings) to the `tenant_slug` validator blocklist.
- `db-designer`: confirm that any new table whose `tenant_id` is nullable follows the same RLS sentinel pattern.
- `go-expert`: the middleware must set `app.current_tenant = '__platform__'` only when the JWT `tenant_id` claim is `null` AND the user's role is `super_admin`. If either condition is false, fall back to the regular tenant UUID path or reject the request.
