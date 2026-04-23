# Test Plan — Lustia

_Owned by `qa-expert`. Cross-layer test strategy. Unit tests stay with the specialist agents who wrote the code._

_Last updated: 2026-04-23 — Phase 3 integration suite written and executed._

---

## 1. Test Pyramid

### Current state (Phase 3 delivery)

| Layer | Tool | Owner | Count now | Notes |
|---|---|---|---|---|
| Unit — backend service | `go test` + testify + hand-written fakes | go-expert | ~40 tests | `lustia/services/auth/internal/service/*_test.go` |
| Integration — HTTP + DB | `go test` (this module) | qa-expert | 13 tests | `tests/integration/` — hits live service |
| E2E | — | — | 0 | Not yet implemented |
| Load | — | — | 0 | Not yet implemented |

### Target per layer (Phase 3+ steady state)

| Layer | Tool | Target count | Coverage goal |
|---|---|---|---|
| Unit — domain / use-cases | `go test` + fakes | ~80 | ≥ 80% line on service layer |
| Unit — adapter (repository, HTTP handlers) | `go test` + testcontainers | ~30 | ≥ 60% line |
| Integration — HTTP + DB | `go test` live service | ~30 | All critical paths + error paths |
| Contract | OpenAPI schema validation (e.g. `schemathesis`) | ~10 | All Phase 3 endpoints |
| E2E — web | Playwright | ~15 | Registration → approval → first-login → branch-create flow |
| Load | k6 | ~3 scenarios | p95 < 300 ms at 50 RPS on login |

**Note on coverage numbers:** aspirational line-coverage targets above 80% for service code are realistic given the existing unit test suite. Pushing adapter-level coverage above 60% requires testcontainers (not yet set up). Framework-glue code (`main.go`, route wiring) has no coverage target.

---

## 2. Critical Paths

These 8 paths must all be green before Phase 3 is signed off. Each maps to one or more test functions.

### T1 — Registration happy path

**Rationale:** The public registration endpoint is the entry point for every new tenant. If it is broken, no tenant can onboard.

**Tests:** `TestRegistrationHappyPath`, `TestRegistrationValidation_MissingContactEmail`, `TestRegistrationValidation_InvalidSlugChars`

**Status: PASS**

---

### T2 — Registration uniqueness / security M-1 fix

**Rationale:** Security finding M-1 required merging `DUPLICATE_PENDING_REGISTRATION` and `TENANT_SLUG_TAKEN` into the generic `CONFLICT` code to prevent email/slug existence enumeration by unauthenticated callers.

**Tests:** `TestRegistrationDuplicateEmail_Returns409Conflict`, `TestRegistrationDuplicateSlug_Returns409Conflict`

**Status: PASS** — both duplicate paths return `409 CONFLICT`, not the specific enumeration-leaking codes.

---

### T3 — Registration rate limit

**Rationale:** The public registration endpoint is unauthenticated and performs non-trivial work (DB writes, email). Rate limiting is the first line of defence against abuse.

**Tests:** `TestRegistrationRateLimit`

**Status: PASS** — 3 requests succeed from a unique synthetic IP; 4th returns `429 RATE_LIMITED`.

**Flaky risk:** The in-memory rate limiter resets on service restart. If a prior test run used the same synthetic IP segment and the limiter has not expired, this test may fail spuriously. Mitigation: each test run generates a fresh random IP via `harness.UniqueIP()`. Residual risk: low.

---

### T4 — Tenant approval flow

**Rationale:** The full onboarding workflow — registration submission → super-admin approval → tenant + user + membership created atomically → welcome email sent → correct DB state.

**Tests:** `TestApprovalFlow`

**Status: PASS** — all assertions pass including DB state verification (tenant active, must_change_password=true, membership=active, user_role=tenant_admin).

---

### T5 — Forced password change gate

**Rationale:** Users created by admin approval must rotate their temporary password before accessing any protected resource. This enforces the `must_change_password` flag shipped in migration 000006.

**Tests:** `TestForcedPasswordChange`

**Status: PASS** — JWT has `must_change_password=true` on first login; `GET /tenant/branches` returns `403 PASSWORD_CHANGE_REQUIRED`; `POST /auth/me/password` clears the flag; new JWT has `must_change_password=false`; old password is rejected; new password works.

---

### T6 — Branch CRUD + limit enforcement

**Rationale:** Branch creation must respect `tenant.max_branches`. Starter tenants (max 1) cannot create a second branch. The full branch lifecycle (create → activate → deactivate → delete) must work correctly.

**Tests:** `TestBranchLimitAlice`, `TestBranchCRUDAndLimit`

**Status: PASS** — `BRANCH_LIMIT_REACHED` returned on over-limit create; full CRUD + status lifecycle verified on a fresh starter tenant.

---

### T7 — Tenant deactivation cascade

**Rationale:** Deactivating a tenant must immediately suspend all memberships and revoke all active refresh tokens, preventing any further access by that tenant's users.

**Tests:** `TestTenantDeactivationCascade`

**Status: FAIL — 2 backend bugs (see §4 below)**

---

### T8 — PII not in audit log (security H-2 fix)

**Rationale:** Security finding H-2 required replacing raw email in audit log entries with a hashed prefix. This test is a regression gate preventing the fix from being undone.

**Tests:** `TestPIINotInAuditLog`, `TestPIIAuditLog_OldEntriesAreHistorical`

**Status: PASS** — new entries have `email_prefix` (8 hex chars) and no `email` key. 3 historical pre-fix entries remain in the DB; this is expected and documented.

---

### T9 — Therapist CRUD

**Rationale:** Full therapist lifecycle — create, read, update, status toggle, soft-delete, and cross-tenant isolation on the create call.

**Tests:** `TestTherapistCRUD`

**Status: FAIL — BUG-P4-A (see §4)**

---

### T10 — Service CRUD

**Rationale:** Full service lifecycle — create, category-filtered list, deactivate/filter/reactivate, update, soft-delete.

**Tests:** `TestServiceCRUD`

**Status: FAIL — BUG-P4-B (see §4)**

---

### T11 — Therapist ↔ Service mapping

**Rationale:** Critical flow per ADR 0009 §2.7. Verifies soft-reconcile: PUT partial deactivates without deleting rows, re-PUT re-activates without inserting new rows. DB-level row-count assertion confirms no phantom INSERTs.

**Tests:** `TestTherapistServiceMapping`

**Status: FAIL — blocked by BUG-P4-A (therapist create fails; mapping test setup cannot proceed)**

---

### T12 — Availability PUT

**Rationale:** Critical flow per ADR 0009 §2.7. Validates all service-layer rules: overlap detection (409), end ≤ start (400), invalid dow (400), non-5-minute boundary (400). Also confirms full-replace semantics (empty PUT clears, 3-window PUT survives subsequent validation failures unchanged).

**Tests:** `TestAvailabilityPut`

**Status: FAIL — blocked by BUG-P4-A (therapist create fails; availability test setup cannot proceed)**

---

### T13 — Cross-branch isolation

**Rationale:** Critical flow per ADR 0009 §2.7. Two sub-tests: (A) cross-tenant 404 isolation when Alice mutates a different tenant's therapist; (B) cross-branch 403 CROSS_BRANCH_FORBIDDEN when a branch_admin at branch A tries to mutate therapists at branch B.

**Tests:** `TestCrossBranchIsolation`

**Status: FAIL — blocked by BUG-P4-A (therapist create in second/third tenant fails)**

---

### T14 — Seed data integrity

**Rationale:** Sanity check that migration 14 produced exactly the expected rows for acme-spa, and that those rows are correctly visible via the API to Alice (tenant isolation + RLS verification).

**Tests:** `TestSeedDataIntegrity`

**DB assertions Status: PASS** — migration 14 counts correct (3 services, 2 therapists, 5 mappings, 6 availability windows).

**API assertions Status: FAIL — BUG-P4-A** — GET /tenant/therapists/:id returns 500 INTERNAL for seeded therapist IDs.

---

## 3. Release Readiness Checklist — Phase 3

- [x] T1 — Registration happy path: PASS
- [x] T2 — Uniqueness / M-1 fix: PASS
- [x] T3 — Rate limit: PASS
- [x] T4 — Approval flow: PASS
- [x] T5 — Forced password change: PASS
- [x] T6 — Branch CRUD + limit: PASS
- [ ] T7 — Tenant deactivation cascade: **FAIL** — blocks sign-off (see §4)
- [x] T8 — PII in audit log (H-2): PASS
- [x] No open Critical/High from security-expert (H-1, H-2 resolved; T7 bugs are new S2 findings)
- [ ] H-1 (hardcoded localhost URL) — fix verified by running with a real `TENANT_ADMIN_LOGIN_URL` env var; test env uses default fallback. Production readiness checklist updated.
- [ ] Backend bugs BUG-T7-A and BUG-T7-B resolved and T7 turned green
- [ ] `go test -race ./...` passes in auth-service CI
- [ ] Migrations 1–12 tested on a clean Postgres 17 instance (docker compose up from scratch)
- [ ] `docs/OPERATIONS.md` runbook updated with Phase 3 env vars (`TENANT_ADMIN_LOGIN_URL`, `SMTP_ENABLED`, `SMTP_HOST`, `SMTP_PORT`)
- [ ] Integration tests added to CI pipeline (`.github/workflows/`)

---

## 3b. Release Readiness Checklist — Phase 4

- [ ] BUG-P4-A resolved — `therapist.specialties` NOT NULL violation on INSERT (blocks T9–T13)
- [ ] BUG-P4-B resolved — `service.code` NOT NULL + `price_idr` column name mismatch (blocks T10)
- [ ] T9 `TestTherapistCRUD`: PASS
- [ ] T10 `TestServiceCRUD`: PASS
- [ ] T11 `TestTherapistServiceMapping`: PASS
- [ ] T12 `TestAvailabilityPut`: PASS
- [ ] T13 `TestCrossBranchIsolation`: PASS
- [ ] T14 `TestSeedDataIntegrity` (DB + API halves): PASS
- [ ] All Phase 3 tests still green after Phase 4 migration
- [ ] `go test -race ./...` passes in auth-service CI
- [ ] Migrations 1–14 tested on a clean Postgres 17 instance

---

## 4. Bugs Found During Test Authoring

### BUG-T7-A — Refresh tokens not revoked on tenant deactivation

**Severity:** S2 (security gap — exploitable in production)

**File:line:** `lustia/services/auth/internal/service/tenant_service.go` — `TransitionStatus` function (the section that transitions to `deactivated`).

**Symptom:** After `PATCH /admin/tenants/:id/status` transitions a tenant to `deactivated`, the memberships for that tenant are correctly transitioned to `suspended` — but the `refresh_token` rows for the affected users' tokens are **not** revoked (`revoked_at` remains NULL).

**Expected:** `revoked_at = now()` on all refresh tokens belonging to users whose only active membership was to the deactivated tenant.

**Actual:** `revoked_at IS NULL` after deactivation.

**Impact:** An attacker who obtained a refresh token before deactivation can call `POST /auth/refresh` and receive a new access token indefinitely (until the 14-day expiry window). The membership suspension prevents tenant-scoped API calls, but the user can still obtain tokens with `scope=user`. This partially mitigates the risk (no tenant data accessible) but contradicts the documented revocation contract in `SECURITY.md §2.5` and `API_CONTRACT.md §10`.

**Fix direction:** In `TransitionStatus` (or in a dedicated `deactivateTenant` method), after suspending all memberships, collect the `user_id` values from those memberships and call `RefreshTokenRepository.RevokeAllForUser` for each, within the same transaction.

---

### BUG-T7-B — Login succeeds (HTTP 200) after tenant deactivation

**Severity:** S2 (security gap — unexpected behavior)

**File:line:** `lustia/services/auth/internal/service/auth_service.go` — `Login` function, membership resolution path.

**Symptom:** After a tenant is deactivated (all memberships suspended), `POST /auth/login` for a user whose only membership is to that tenant returns **HTTP 200** with `scope=user` and an empty `memberships` array. The user is authenticated but has no tenant context.

**Expected:** Login should fail with `401 ACCOUNT_INACTIVE` or `401 TENANT_INACTIVE` for users whose all memberships are suspended/deactivated — or at minimum, `scope=user` tokens should be rejected by all protected endpoints.

**Actual:** HTTP 200, `scope=user`. The user can subsequently call `POST /auth/refresh` to maintain their session indefinitely (see BUG-T7-A for the refresh token concern).

**Impact:** Deactivated tenant's admin can still authenticate and hold valid (though tenant-context-free) tokens. Combined with BUG-T7-A, this means the token chain continues indefinitely.

**Fix direction:** In the `Login` use-case, after loading memberships, check if all memberships are in non-active status (`suspended`, `left`, `invited`). If the user has zero active memberships AND is not a super admin, return `401 ACCOUNT_INACTIVE`. Alternatively, `scope=user` tokens could be configured to expire faster or be blocked on any protected endpoint other than `/auth/select-tenant`.

---

### BUG-P4-A — `therapist` INSERT fails: `specialties` NOT NULL violation

**Severity:** S1 (blocks all Phase 4 write paths — no therapist can be created)

**File:line:** `lustia/services/auth/internal/model/therapist.go` — `Therapist` struct, `Specialties` field.

**Symptom:** Every `POST /api/v1/tenant/therapists` returns `500 INTERNAL`. The DB error is:
```
null value in column "specialties" of relation "therapist" violates not-null constraint
```

**Root cause:** The `Specialties` field is declared as `*string` (Go nil pointer). When `nil`, GORM sends an explicit SQL `NULL` for the column, which violates `specialties jsonb NOT NULL DEFAULT '[]'::jsonb`. The DB default only fires when the column is omitted from the INSERT, but GORM includes all struct fields.

**Fix direction:** Two options, either sufficient:
1. Change `Specialties *string` → `string` with `gorm:"column:specialties;default:'[]'"` and initialise to `"[]"` before INSERT in the service layer.
2. Keep `*string` but add `gorm:"column:specialties;default:'[]'"` and ensure the service sets it to `"[]"` (not nil) on new records.

**Affected endpoints:** `POST /tenant/therapists`, `GET /tenant/therapists/:id` (500 on read too — serialisation fails for seeded rows because the jsonb value can't scan into `*string` cleanly), `PUT /tenant/therapists/:id/services`, `PUT /tenant/therapists/:id/availability`, `DELETE /tenant/therapists/:id`.

---

### BUG-P4-B — `service` INSERT fails: `code` NOT NULL + `price_idr` column mismatch

**Severity:** S1 (blocks all Phase 4 service write paths — no service can be created)

**File:line:** `lustia/services/auth/internal/model/service_catalog.go` — `ServiceCatalog` struct.

**Symptom:** Every `POST /api/v1/tenant/services` returns `500 INTERNAL`. Two separate DB-level errors:

1. `gorm:"column:price_idr"` — GORM maps to column `price_idr` but the DB column is `price`. GORM will either error with "column price_idr does not exist" or silently omit the price.
2. No `Code` field in the model — the DB column `code text NOT NULL` has no default and is not nullable. GORM omits the column from the INSERT, producing a NOT NULL violation.

**Root cause:** Schema drift between `internal/model/service_catalog.go` and the actual `service` table DDL. The DB has `price numeric(12,2)` and `code text NOT NULL`; the model has `PriceIDR int64 gorm:"column:price_idr"` and no `Code` field.

**Fix direction:**
1. Rename the GORM column tag: `gorm:"column:price;not null"`. The field name can stay `PriceIDR` for Go-side clarity.
2. Add a `Code string` field: `gorm:"column:code;not null"`. The service layer must generate a slug-like code (e.g. from `uuid.New().String()[:8]` or the service name) before calling `Save`.

---

## 5. Flaky Test Log

| Test | First seen | Root cause | Status |
|---|---|---|---|
| `TestRegistrationRateLimit` | 2026-04-23 | In-memory rate limiter resets on service restart; reuse of synthetic IP segment across runs could exhaust the bucket prematurely. Mitigation: `harness.UniqueIP()` per run. | Mitigated — low residual risk |

---

## 6. Future Gaps

The following test categories are not implemented in Phase 3 and should be addressed before production launch:

| Gap | Priority | Notes |
|---|---|---|
| E2E via Playwright — registration → approval → first-login → branch-create | High | Requires at least the platform-admin Next.js app to be deployed |
| Load test — `POST /auth/login` at 50 RPS | High | Validate Argon2id latency under load (p95 < 300 ms target) |
| Load test — `POST /register/company` at 10 RPS sustained | Medium | Confirm in-memory rate limiter doesn't degrade under burst |
| Contract validation with `schemathesis` against OpenAPI spec | Medium | Catches endpoint drift between contract doc and implementation |
| Security automation — OWASP ZAP scan on Phase 3 endpoints | Medium | Complements manual H/M/L findings in SECURITY.md |
| Testcontainers-based repository tests | Medium | Currently no tests exercise the GORM repository layer against a real DB |
| Refresh token rotation — theft detection path | High | E-3 in threat model; no test coverage yet |
| Password reset flow | Medium | `POST /auth/forgot-password` + `POST /auth/reset-password` — unit tests exist but no integration coverage |
| Admin user create flow (`POST /admin/users`) | Medium | Phase 2 endpoint; not covered by Phase 3 integration suite |
