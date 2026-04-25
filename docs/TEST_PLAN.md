# Test Plan — Lustia

_Owned by `qa-expert`. Cross-layer test strategy. Unit tests stay with the specialist agents who wrote the code._

_Last updated: 2026-04-24 — Tenant-wide add-on catalog (ADR 0010 revised) unit test review; 11 gap tests added by qa-expert (total suite now 40 tests)._

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

### T15 — Service Add-ons lifecycle (ADR 0010) — SUPERSEDED ENTRY 2026-04-24

> **SUPERSEDED 2026-04-24.** The original T15 entry below was written for a per-service add-on model (`service_addon` table, add-ons owned by a service). That design was rejected in the same session it was implemented (see ADR 0010 change log). All code from that design was replaced before commit. The test function names listed below never existed in source control. The entry is preserved as a design-trail record only.
>
> See **T15 (tenant-wide)** below for the current, active entry.

~~**Rationale (superseded):** Full add-on lifecycle per ADR 0010 v1 — create, read, update, activate/deactivate, soft-delete, reorder, cross-tenant isolation, and duplicate-name enforcement (within-service rejected; across-services allowed).~~

~~**Tests (superseded):** `TestAddonCreate_Success`, `TestAddonCreate_EmptyName_Rejected`, `TestAddonCreate_NegativePrice_Rejected`, `TestAddonCreate_DescriptionTooLong_Rejected`, `TestAddonCreate_CrossTenant_Rejected`, `TestAddonCreate_DuplicateName_Rejected`, `TestAddonCreate_NameAt120Chars_Accepted`, `TestAddonCreate_NameAt121Chars_Rejected`, `TestAddonCreate_DuplicateNameAcrossServices_Allowed`, `TestAddonList_AllNonDeleted`, `TestAddonList_FilterActive`, `TestAddonUpdate_Success`, `TestAddonUpdate_WrongService_Rejected`, `TestAddonChangeStatus_Deactivate`, `TestAddonChangeStatus_ReactivateAfterDeactivate`, `TestAddonSoftDelete_Success`, `TestAddonSoftDelete_AlreadyDeletedOrNotFound`, `TestAddonReorder_Atomic_AllOrNothing`, `TestAddonReorder_CrossTenant_Rejected`, `TestAddonReorder_AddonBelongsToDifferentService_Rejected`, `TestAddonCrossTenantIsolation`~~

~~**Status: SUPERSEDED** — replaced by T15 (tenant-wide) below.~~

---

### T15 — Tenant-wide Add-on Catalog lifecycle (ADR 0010 revised)

**Rationale:** Full add-on lifecycle per ADR 0010 (tenant-wide flat catalog) — create, read, update, activate/deactivate, soft-delete, reorder, cross-tenant isolation, duplicate-name enforcement, name/price/sort_order/description boundaries. Any add-on is available across all services for the tenant. No per-service mapping table.

**Critical paths covered:**

| Path | Test function(s) |
|---|---|
| Create success | `TestAddonService_Create_Success` |
| Duplicate name within tenant rejected | `TestAddonService_Create_DuplicateName_SameTenant` |
| Duplicate name across tenants allowed | `TestAddonService_Create_DuplicateName_DifferentTenant_Allowed` |
| Name = 0 chars rejected | `TestAddonService_Create_EmptyName_Rejected` |
| Name = 1 char accepted | `TestAddonService_Create_NameAt1Char_Accepted` |
| Name = 120 chars accepted (ASCII) | `TestAddonService_Create_NameExactly120_Accepted` |
| Name = 121 chars rejected | `TestAddonService_Create_NameTooLong_Rejected` |
| Name = 120 multi-byte runes accepted (rune-count, not byte-count) | `TestAddonService_Create_Name120MultibyteRunes_Accepted` |
| Price negative rejected | `TestAddonService_Create_NegativePrice_Rejected` |
| Price zero accepted | `TestAddonService_Create_ZeroPrice_Accepted` |
| Price large positive accepted | `TestAddonService_Create_LargePositivePrice_Accepted` |
| Description 501 chars rejected | `TestAddonService_Create_DescriptionTooLong_Rejected` |
| Description 500 chars accepted | `TestAddonService_Create_DescriptionExactly500_Accepted` |
| sort_order = 0 accepted | `TestAddonService_Create_Success` (SortOrder: 0) |
| sort_order = 9999 accepted | `TestAddonService_Create_SortOrderAt9999_Accepted` |
| sort_order = 10000 rejected (Create) | `TestAddonService_Create_SortOrderAt10000_Rejected` |
| sort_order = −1 rejected (Create) | `TestAddonService_Create_NegativeSortOrder_Rejected` |
| sort_order = 10000 rejected (Reorder) | `TestAddonService_Reorder_SortOrderOutOfBounds_Rejected` |
| List returns only caller tenant | `TestAddonService_List_ReturnsOnlyCallerTenant` |
| List excludes soft-deleted | `TestAddonService_List_SoftDeletedExcluded` |
| List is_active filter | `TestAddonService_List_IsActiveFilter` |
| List default limit = 10 | `TestAddonService_List_DefaultLimit10` |
| List cursor pagination — non-overlapping pages | `TestAddonService_List_CursorPagination` |
| Get success | `TestAddonService_Get_Success` |
| Get not found | `TestAddonService_Get_NotFound` |
| Get cross-tenant IDOR rejected | `TestAddonService_Get_CrossTenantIDOR` |
| Update success | `TestAddonService_Update_Success` |
| Update cross-tenant IDOR rejected | `TestAddonService_Update_CrossTenantIDOR` |
| Update duplicate name rejected | `TestAddonService_Update_DuplicateName_Rejected` |
| ChangeStatus deactivate + reactivate | `TestAddonService_ChangeStatus_Toggle` |
| ChangeStatus cross-tenant IDOR rejected | `TestAddonService_ChangeStatus_CrossTenantIDOR` |
| ChangeStatus on soft-deleted row returns NotFound | `TestAddonService_ChangeStatus_OnSoftDeletedRow_ReturnsNotFound` |
| SoftDelete success + Get returns NotFound after | `TestAddonService_SoftDelete_Success` |
| SoftDelete cross-tenant IDOR rejected | `TestAddonService_SoftDelete_CrossTenantIDOR` |
| SoftDelete excluded from List | `TestAddonService_SoftDelete_ExcludedFromList` |
| Reorder success | `TestAddonService_Reorder_Success` |
| Reorder with mixed foreign-tenant IDs rejected | `TestAddonService_Reorder_MixedForeignIDs_Rejected` |
| Reorder with nonexistent ID rejected | `TestAddonService_Reorder_NonexistentID_Rejected` |
| Reorder empty items rejected | `TestAddonService_Reorder_EmptyItems_Rejected` |
| Reorder atomicity — bulk DB error rolls back | `TestAddonService_Reorder_Atomicity_BulkFailRollsBack` |
| Full lifecycle: create → update → deactivate → delete → not in list | `TestAddonService_Lifecycle_CreateUpdateStatusDelete` |

**Tests (unit, 40 total):** all `TestAddonService_*` functions in `lustia/services/auth/internal/service/addon_service_test.go`.

Of the 40 tests: 29 were written by go-expert; 11 boundary/gap tests were added by qa-expert on 2026-04-24 (`TestAddonService_Create_EmptyName_Rejected`, `TestAddonService_Create_NameAt1Char_Accepted`, `TestAddonService_Create_Name120MultibyteRunes_Accepted`, `TestAddonService_Create_DescriptionExactly500_Accepted`, `TestAddonService_Create_LargePositivePrice_Accepted`, `TestAddonService_Create_SortOrderAt9999_Accepted`, `TestAddonService_Create_SortOrderAt10000_Rejected`, `TestAddonService_Create_NegativeSortOrder_Rejected`, `TestAddonService_Update_DuplicateName_Rejected`, `TestAddonService_ChangeStatus_OnSoftDeletedRow_ReturnsNotFound`, `TestAddonService_Reorder_SortOrderOutOfBounds_Rejected`).

**Status: PASS** — all 40 unit tests green as of 2026-04-24.

**Flakiness check:** `stubAddonRepo.FindByTenant` sorts results by (sort_order, created_at, id) before returning. No map-iteration order-dependency in any assertion path. All list assertions use set membership or sorted-index checks. Verdict: no flakiness risk identified.

**Integration gap:** HTTP-level integration tests not yet written; deferred per Phase 4 precedent (§6). Must be added before Phase 5 booking integration that consumes add-ons.

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
- [ ] BUG-P4-C resolved — `TestMappingReconcile_*` stub fixture stores empty `TenantID` (see §4 BUG-P4-C)
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

## 3c. Release Readiness Checklist — Tenant-wide Add-on Catalog (ADR 0010 revised)

- [ ] BUG-P4-A and BUG-P4-B resolved (parallel Phase 4 bugs; add-ons themselves have no dependency on therapist/service write paths, but the shared migration chain must be clean)
- [x] 40 unit tests in `addon_service_test.go` all PASS (29 go-expert + 11 added by qa-expert 2026-04-24) — verified 2026-04-24
- [ ] Integration test for add-on HTTP endpoints deferred (see §6 gap entry) — acceptable per Phase 4 precedent; must be added before Phase 5 booking integration
- [ ] No frontend E2E coverage (Phase 4 has zero Playwright tests; deferral documented in §6)
- [ ] Migration `000018` (`addon` table — tenant-wide, no service FK) tested on a clean Postgres 17 instance
- [ ] Migration `000019` (dev seed add-ons for `acme-spa`) tested on a clean Postgres 17 instance
- [ ] RLS on `addon` confirmed — cross-tenant SELECT/INSERT/UPDATE blocked (unit tests cover service-layer IDOR; DB-level RLS spot-check recommended before production)
- [ ] Permissions `addon.read` / `addon.create` / `addon.update` / `addon.delete` wired to `tenant_admin` role; `branch_admin` read-only wiring confirmed

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

### BUG-P4-C — `TestMappingReconcile_*` unit tests fail: stub therapist has empty `TenantID`

**Severity:** S3 (test fixture bug — production code correct; no user-visible defect)

**File:line:** `lustia/services/auth/internal/service/mapping_service_test.go` — all 5 `TestMappingReconcile_*` test setup blocks.

**Symptom:** All 5 `TestMappingReconcile_*` tests fail with "therapist not found". Error originates at `mapping_service.go:45`: `if t.TenantID != in.CallerTenantID`.

**Root cause:** Every test seeds the therapist as `&model.Therapist{ID: "th1", BranchID: "b1", IsActive: true}` — `TenantID` is omitted, so it defaults to `""`. The tests pass `CallerTenantID: "t1"`, so the isolation check `"" != "t1"` is true and the service correctly returns `ErrTherapistNotFound`. The isolation logic is correct; the fixture is wrong.

**Verification:** Introduced in commit `41cc0fe` (Phase 4). The mapping test file has exactly one commit in its history — predates the add-on feature entirely. Git stash / baseline check confirms no regression from ADR 0010 work.

**Fix direction:** In each `TestMappingReconcile_*` test, change the stub seed line to include `TenantID: "t1"`:
```go
therapistRepo.rows["th1"] = &model.Therapist{ID: "th1", TenantID: "t1", BranchID: "b1", IsActive: true}
```
This is a 5-line fix across the 5 tests, owned by go-expert.

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
| Integration tests for add-on HTTP endpoints (tenant-wide, ADR 0010 revised) | Medium | Only service-layer unit tests (40) exist. HTTP-level coverage (`POST /tenant/addons`, `PATCH`, `DELETE`, `PUT /reorder`, cross-tenant 404, duplicate-name 409) deferred per Phase 4 precedent. Must be added before Phase 5 booking integration that consumes add-ons. |
| `GET /api/v1/tenant/addons` permission wiring — integration smoke | Medium | Verify `branch_admin` JWT receives 403 on write endpoints and 200 on read endpoints. Unit tests do not exercise the auth middleware layer. One integration test covering the permission boundary is sufficient. |
| E2E via Playwright — add-on CRUD in tenant-admin `/master/addons` | Low | No Playwright suite exists in Phase 4; defer to the same milestone that ships the Phase 4 registration→approval Playwright suite (§6 row 1). |
| Reorder — maximum items (201) rejected | Low | `len > 200` guard exists in code but has no explicit unit test. Low risk — add alongside integration tests. |
