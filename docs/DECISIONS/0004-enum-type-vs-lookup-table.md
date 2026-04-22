# ADR 0004 — Enum Strategy: Native Postgres Enums for Status Fields

**Status:** Accepted
**Date:** 2026-04-18
**Author:** db-designer

---

## Context

Several tables require columns with a finite set of valid values: booking status, invoice status, payment method, etc. Two common approaches in PostgreSQL:

| Approach | Description |
|---|---|
| Native `CREATE TYPE ... AS ENUM` | Type-checked at DB level; stored efficiently (4 bytes OID + lookup) |
| Lookup table + FK | A `status` table with rows; status column is FK UUID or TEXT |

---

## Decision

**Use native Postgres enums for all status and source fields that are stable and defined by the domain model, not by operators.**

Enums used: `tenant_status`, `branch_status`, `booking_status`, `booking_source`, `invoice_status`, `payment_method`, `payment_status`.

Lookup tables are used only when:
- Values will be created/edited by operators (e.g. a configurable list of service categories).
- Values carry display metadata (label, sort order, color).
- Values are tenant-specific.

None of the status fields above meet these criteria — they are part of the application's state machine, not user-configurable data.

---

## Reasons

**1. DB-level type safety.**
An enum column will reject any value not in the declared set at INSERT/UPDATE time, with no application code required. A lookup-table FK provides similar safety but requires a JOIN to validate and a separate seed migration.

**2. Self-documenting schema.**
`\dT+ booking_status` in psql shows all valid values. This is immediately useful for developers, DBAs, and tooling.

**3. Idempotency workaround.**
`CREATE TYPE` does not have `IF NOT EXISTS` syntax. All enum creation statements are wrapped in `DO $$ BEGIN ... EXCEPTION WHEN duplicate_object THEN NULL; END $$;` blocks. This is documented in the migrations and is a known PostgreSQL limitation.

**4. Adding values is non-destructive.**
`ALTER TYPE booking_status ADD VALUE 'rescheduled';` is a non-blocking, append-only migration. Renaming or removing values requires a more involved process — but status values defined here are designed to be terminal (the state machine is stable by design).

**5. Performance.**
Enum storage is 4 bytes (OID). A TEXT column with a CHECK constraint is also valid but loses the type-system benefit. A UUID FK to a lookup table adds a JOIN to every query that resolves the label.

---

## Consequences

- Adding a new enum value requires a migration: `ALTER TYPE <enum_name> ADD VALUE '<value>';`
- Removing or renaming an enum value is a multi-step migration (add new, migrate data, remove old). Plan enum values carefully — the ones defined here are final for Phase 1–6.
- The Go application must define matching constants/types for each enum. Flag for `go-expert`: generate or maintain Go string constants that mirror the Postgres enum values; mismatches cause runtime errors.

---

## Review trigger

Revisit if any of the following occurs:
- A tenant needs to define their own booking statuses (customizable workflows) — switch that field to a lookup table with `tenant_id`.
- An enum value needs a display label or sort order managed via admin UI — switch to a lookup table.
