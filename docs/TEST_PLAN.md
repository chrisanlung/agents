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
