# Lustia Integration Tests

Black-box HTTP integration tests for the Lustia auth-service. They hit the live service over HTTP and perform minimal DB assertions via a direct Postgres connection. No internal packages from the auth-service are imported.

## Prerequisites

| Requirement | Version | Notes |
|---|---|---|
| Go | 1.24+ | Match auth-service |
| Auth-service | running at `BASE_URL` (default `http://localhost:8080`) | `GET /healthz` must return 200 |
| PostgreSQL 17 | running at `TEST_DB_URL` | Migrations 1–12 applied, Alice seeded |
| Mailpit | running at `MAILPIT_URL` (default `http://localhost:8025`) | Optional; T4 email check degrades gracefully if absent |

## Environment variables

| Variable | Default | Required | Notes |
|---|---|---|---|
| `BASE_URL` | `http://localhost:8080` | No | Auth-service base URL |
| `TEST_DB_URL` | `postgres://lustia_app:lustia_app_local@localhost:5432/lustia?sslmode=disable` | No | Direct DB connection for assertions |
| `MAILPIT_URL` | `http://localhost:8025` | No | Mailpit JSON API base URL |
| `SUPER_ADMIN_EMAIL` | — | **Yes** (T4–T7) | Super admin login email |
| `SUPER_ADMIN_PASSWORD` | — | **Yes** (T4–T7) | Super admin password |
| `SMTP_ENABLED` | — | No | Set to `true` to enable Mailpit welcome-email assertion in T4 |

Tests that require `SUPER_ADMIN_EMAIL`/`SUPER_ADMIN_PASSWORD` skip themselves with a clear message if those vars are absent.

## Quickstart

```bash
cd tests/integration

# Run all integration tests (verbose)
SUPER_ADMIN_EMAIL=admin@lustia.local \
SUPER_ADMIN_PASSWORD=SuperAdmin2026! \
SMTP_ENABLED=true \
go test -v -timeout 180s ./...

# Run only fast/offline tests (skips all integration tests)
go test -short ./...

# Run a single test
SUPER_ADMIN_EMAIL=admin@lustia.local \
SUPER_ADMIN_PASSWORD=SuperAdmin2026! \
go test -v -run TestApprovalFlow ./...
```

## Seeding super-admin password for first run

The bootstrap super admin (`admin@lustia.local`) is seeded with a placeholder Argon2id hash that cannot be used to log in. Before running T4–T7 you must set a real password.

**Step 1 — generate the hash:**

```bash
cd lustia/services/auth
go run ./cmd/hashpw -password 'SuperAdmin2026!'
```

**Step 2 — apply the UPDATE:**

```sql
-- Copy the SQL emitted by hashpw and run it against the lustia database.
-- Example (use the hash from Step 1, not this literal):
UPDATE "user"
   SET password_hash = '$argon2id$v=19$m=65536,t=3,p=2$...',
       failed_login_count = 0,
       locked_until = NULL,
       must_change_password = false
 WHERE id = 'a0000000-0000-0000-0000-000000000001';
```

**Step 3 — set the env vars and run:**

```bash
export SUPER_ADMIN_EMAIL=admin@lustia.local
export SUPER_ADMIN_PASSWORD=SuperAdmin2026!
```

If the account becomes locked (10 failed attempts), reset with:

```sql
UPDATE "user"
   SET failed_login_count = 0, locked_until = NULL
 WHERE email = 'admin@lustia.local';
```

## Test inventory

| ID | Function | File | Critical path |
|---|---|---|---|
| T1 | `TestRegistrationHappyPath` | `registration_happy_path_test.go` | Yes |
| T2 | `TestRegistrationDuplicateEmail_Returns409Conflict` | `registration_uniqueness_test.go` | Yes (M-1 fix) |
| T2b | `TestRegistrationDuplicateSlug_Returns409Conflict` | `registration_uniqueness_test.go` | Yes (M-1 fix) |
| T3 | `TestRegistrationRateLimit` | `registration_rate_limit_test.go` | Yes |
| T4 | `TestApprovalFlow` | `approval_flow_test.go` | Yes |
| T5 | `TestForcedPasswordChange` | `forced_password_change_test.go` | Yes |
| T6a | `TestBranchLimitAlice` | `branch_crud_limit_test.go` | Yes |
| T6b | `TestBranchCRUDAndLimit` | `branch_crud_limit_test.go` | Yes |
| T7 | `TestTenantDeactivationCascade` | `tenant_deactivation_cascade_test.go` | Yes (**FAILING** — 2 backend bugs) |
| T8 | `TestPIINotInAuditLog` | `pii_in_audit_log_test.go` | Yes (H-2 fix) |

## Known issues

**T7 — TestTenantDeactivationCascade** fails due to two backend bugs documented at `tenant_deactivation_cascade_test.go:106` and `:124`. See the Final Report in `docs/TEST_PLAN.md` for full details.
