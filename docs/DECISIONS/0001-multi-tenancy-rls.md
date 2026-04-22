# ADR 0001 — Multi-Tenancy Strategy: Shared Schema + Row-Level Security

**Status:** Accepted
**Date:** 2026-04-18
**Author:** db-designer

---

## Context

Lustia is a B2B SaaS platform where multiple independent companies (tenants) share the same application and database infrastructure. Each tenant's data must be completely invisible to other tenants — a booking from Tenant A must never appear in Tenant B's query results, even if a bug in the application layer forgets a WHERE clause.

Three standard patterns were evaluated:

| Pattern | Description | Typical tenant count |
|---|---|---|
| Shared schema + `tenant_id` + RLS | All tenants in one schema; DB enforces isolation | Hundreds to thousands |
| Schema-per-tenant | Separate `pg_schema` per tenant; `search_path` switches context | Tens to low hundreds |
| Database-per-tenant | Separate Postgres instance or database per tenant | Low tens; max isolation |

---

## Decision

**Shared schema + `tenant_id` column on every operational table + PostgreSQL Row-Level Security (RLS).**

Every table that contains tenant data carries:

```sql
tenant_id UUID NOT NULL REFERENCES tenant(id)
```

RLS is enabled on every such table with a `USING` policy:

```sql
CREATE POLICY tenant_isolation ON <table>
    USING (tenant_id::text = current_setting('app.current_tenant', true));
```

The application middleware calls `SET LOCAL app.current_tenant = '<uuid>'` at the start of every request transaction. `current_setting(..., true)` returns NULL (not an error) when the setting is absent, which causes every row check to fail — effectively denying all access until the setting is explicitly established.

---

## Reasons for choosing shared schema + RLS

**1. Operational simplicity at scale.**
Lustia targets hundreds to thousands of tenants. Schema-per-tenant would require managing hundreds of Postgres schemas — schema migrations (e.g. adding a column) would need to run once per schema, turning a single `ALTER TABLE` into a batch job. With a shared schema, one migration file applies to all tenants.

**2. Defense in depth.**
Application-layer `WHERE tenant_id = ?` filters are the first line of defense, but they are fragile — a developer who forgets the clause, or a new query that bypasses the repository layer, can leak data. RLS is a second line that the DB enforces regardless of how the query arrives. This is especially important in a microservices architecture where multiple services share the same DB.

**3. PostgreSQL RLS is production-grade.**
RLS has been in Postgres since 9.5 (2016). The `current_setting` approach is idiomatic and well-understood. The performance overhead of a simple equality check on an indexed column is negligible.

**4. Schema-per-tenant rejected due to migration complexity.**
Migrating hundreds of schemas atomically requires tooling (e.g. Flyway multi-schema support) not in the current stack. The risk of partial migrations — some schemas on v12, others on v11 — is a significant operational hazard.

**5. Database-per-tenant rejected due to cost and connection overhead.**
Each Postgres database requires its own connection pool. At 100+ tenants, connection pressure alone (Postgres has a hard connection limit) makes this pattern impractical without a PgBouncer tier. The added isolation benefit is not justified for Lustia's threat model.

---

## Super-admin bypass

The platform `super_admin` role needs to read across all tenants (e.g. approve a new tenant registration, view platform-wide reports). Two approaches:

- **`BYPASSRLS` DB role:** A Postgres role with `BYPASSRLS` attribute skips all RLS checks. The migrator role (`lustia_migrator`) already has this.
- **Application-level sentinel:** When the application authenticates a `super_admin` user, it sets `app.current_tenant = '__platform__'` instead of a real UUID. The RLS policies on `tenant`-nullable tables include an explicit `OR (tenant_id IS NULL AND current_setting(...) = '__platform__')` clause.

**Decision:** Use the sentinel value `__platform__` for `super_admin` requests. This keeps all connections going through `lustia_app` (simpler connection pool) while still correctly scoping reads. The `lustia_migrator` role with `BYPASSRLS` is used only for DDL migrations, never for application traffic.

---

## Tables excluded from tenant RLS

The following tables are **platform-level** and intentionally cross-tenant:

| Table | Reason |
|---|---|
| `tenant` | It IS the root of tenancy. Managed by platform ops only. |
| `role` | Platform-wide role definitions. Super admin reads all. |
| `permission` | Platform-wide permission codes. Read-only for app. |
| `role_permission` | Role→permission junction. Platform-managed seed data. |
| `audit_log` | Platform ops need to read all events. Tenant rows filtered by app layer. |

For these tables, application-layer authorization (RBAC) is the primary control: only `super_admin` can write to `tenant`; `lustia_app` has read-only access to `role`, `permission`, `role_permission`.

---

## Consequences

- Every new table with tenant-scoped data **must** add `tenant_id` and have a matching RLS policy applied in the migrations.
- All application DB connections must connect as `lustia_app` (not `postgres` or any superuser).
- The auth middleware **must** call `SET LOCAL app.current_tenant` within the transaction before executing any query — failing to do so causes all row checks to fail (safe-fail: empty result, not data leak).
- Cross-tenant queries (e.g. platform-level reporting) must use a connection that bypasses RLS (either `lustia_migrator` or a dedicated `lustia_super_admin` role with `BYPASSRLS`).
- `golang-migrate` runs as `lustia_migrator` (BYPASSRLS) so DDL migrations are not blocked by RLS.

---

## Review trigger

Revisit this decision if:
- Tenant count exceeds ~5,000 and RLS policy evaluation overhead becomes measurable.
- A tenant requires data residency in a different geographic region.
- Regulatory requirements demand physical DB isolation per tenant.
