# ADR 0002 — Booking Double-Booking Prevention: DB-Level Exclusion Constraint

**Status:** Accepted
**Date:** 2026-04-18
**Author:** db-designer

---

## Context

A therapist can only perform one session at a time. The system must prevent two bookings for the same therapist from overlapping in their scheduled time window. The question is where this constraint is enforced: at the application layer (SELECT then INSERT) or at the database layer (exclusion constraint).

---

## Options considered

### Option A: Application-layer check (SELECT then INSERT)

```
1. SELECT COUNT(*) FROM booking
   WHERE therapist_id = ? AND status NOT IN ('cancelled','no_show')
     AND tstzrange(scheduled_start, scheduled_end) && tstzrange(?, ?)
2. If count = 0, proceed with INSERT
```

**Problems:**
- This is a classic TOCTOU (time-of-check / time-of-use) race condition. Under concurrent load, two requests can both pass step 1 with count = 0, then both proceed to step 2, resulting in a double booking.
- Requires the application developer to remember to check on every code path that creates or reschedules a booking. A new booking endpoint, an admin rescheduling tool, a data import script — any of them can forget.
- Unit-testable but not structurally enforced.

### Option B: `SELECT FOR UPDATE` pessimistic lock

Lock the therapist row (or a dedicated therapist-slot row) before checking. Prevents the race condition but introduces serialization at the therapist level — all booking attempts for a therapist queue up, reducing throughput.

### Option C: DB-level `EXCLUDE USING gist` constraint (chosen)

```sql
EXCLUDE USING gist (
    therapist_id WITH =,
    tstzrange(scheduled_start, scheduled_end) WITH &&
)
WHERE (
    therapist_id IS NOT NULL
    AND status NOT IN ('cancelled', 'no_show')
)
```

---

## Decision

**Use a PostgreSQL GiST exclusion constraint on the `booking` table.**

This constraint, combined with `btree_gist` extension (which adds GiST support for equality operators on non-range types like UUID), ensures that no two active bookings for the same therapist can overlap in scheduled time — regardless of which application code path created them.

---

## Why DB-level wins

**1. Race-condition proof by construction.**
PostgreSQL evaluates the exclusion constraint atomically as part of the INSERT/UPDATE statement within its MVCC engine. Two concurrent transactions that both try to insert an overlapping booking cannot both succeed — the second will fail with an exclusion violation, which the application maps to a `409 Conflict` response.

**2. Enforcement is universal.**
Any code path — REST API, admin CLI, data migration, GORM model — that touches the `booking` table is covered. There is no way to bypass the constraint without explicitly dropping it.

**3. Partial constraint correctly handles terminal statuses.**
The `WHERE status NOT IN ('cancelled', 'no_show')` partial condition means cancelled bookings do not block the same slot from being rebooked. This matches the business rule: a therapist whose 10am booking was cancelled is available again at 10am.

**4. Index is also a query accelerator.**
The GiST index created by the exclusion constraint also serves availability queries: "Is therapist X available between 2pm–3pm?" can be answered with an index scan rather than a full table scan.

**5. Performance.**
The GiST index evaluation cost is O(log N) per booking. The therapist double-booking check does not add perceptible latency compared to the overall INSERT transaction.

---

## Consequences for the application layer

- The Go booking use case **must** still return a user-friendly error message when the exclusion constraint fires (`pq: conflicting key value violates exclusion constraint`). Map this DB error code to a `409 Conflict` HTTP response with a message like "This therapist is not available at the requested time."
- When rescheduling a booking (UPDATE of `scheduled_start`/`scheduled_end`), the exclusion constraint also fires — no extra application logic needed.
- When cancelling a booking (UPDATE `status = 'cancelled'`), the partial condition `WHERE status NOT IN ('cancelled','no_show')` means the constraint no longer applies to that row, freeing the slot.
- Flag for `go-expert`: ensure the booking repository correctly distinguishes the exclusion constraint violation error code from other DB errors.

---

## Therapist availability overlap prevention

The same pattern is applied to `therapist_availability` to prevent two availability windows for the same therapist + branch + day from overlapping:

```sql
EXCLUDE USING gist (
    therapist_id WITH =,
    branch_id    WITH =,
    day_of_week  WITH =,
    tsrange(
        ('2000-01-01'::date + start_time)::timestamp,
        ('2000-01-01'::date + end_time)::timestamp
    ) WITH &&
)
```

The `TIME` type does not have a native GiST operator class, so we normalize the start/end times onto the arbitrary fixed date `2000-01-01` to form a `TSRANGE`. This is a well-known idiom; the fixed date carries no semantic meaning — only the time-of-day component matters. `TSRANGE` (without timezone) is used because the day_of_week scoping removes the need for timezone awareness at this level (availability is interpreted in the branch's local timezone by the application).

---

## Review trigger

Revisit this decision if:
- The booking table is partitioned by `tenant_id` — exclusion constraints do not work across partitions in PostgreSQL < 17. Evaluate pg_partman + per-partition constraints at that point.
- Therapist double-booking detection needs to span multiple tables (e.g. external calendar sync) — at that point an application-layer coordination service may be needed in addition to the DB constraint.
