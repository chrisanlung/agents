# Integration Test Suite — Lustia Auth Service

Go integration tests that run against the live auth-service + Postgres.
All tests require the service to be running on `BASE_URL` (default `http://localhost:8080`).

## Running

```bash
# Run all integration tests
SUPER_ADMIN_EMAIL=admin@lustia.local \
SUPER_ADMIN_PASSWORD=QaFix2026Strong! \
go test -v ./... -timeout 180s

# Skip slow/multi-tenant tests
go test -short ./...

# Run a specific flow
go test -v -run TestTherapistCRUD
```

## Environment Variables

| Variable | Default | Required |
|---|---|---|
| `BASE_URL` | `http://localhost:8080` | No |
| `TEST_DB_URL` | `postgres://lustia_app:lustia_app_local@localhost:5432/lustia?sslmode=disable` | No |
| `SUPER_ADMIN_EMAIL` | — | Yes for T4, T7, T13 |
| `SUPER_ADMIN_PASSWORD` | — | Yes for T4, T7, T13 |
| `SMTP_ENABLED` | (detected) | No — Mailpit check is best-effort |

## Test Files

### Phase 1–3 (existing)

| File | ID | Critical Path |
|---|---|---|
| `registration_happy_path_test.go` | T1 | Registration happy path |
| `registration_uniqueness_test.go` | T2 | Uniqueness / M-1 enumeration fix |
| `registration_rate_limit_test.go` | T3 | Rate limit enforcement |
| `approval_flow_test.go` | T4 | Full approval flow + DB assertions |
| `forced_password_change_test.go` | T5 | `must_change_password` gate |
| `branch_crud_limit_test.go` | T6 | Branch CRUD + limit enforcement |
| `tenant_deactivation_cascade_test.go` | T7 | Tenant deactivation cascade (**FAIL** — BUG-T7-A, BUG-T7-B) |
| `pii_in_audit_log_test.go` | T8 | PII not in audit log (H-2 regression) |

### Phase 4 (new — added 2026-04-23)

| File | ID | Critical Path |
|---|---|---|
| `therapist_crud_test.go` | T9 | Therapist full lifecycle CRUD |
| `service_crud_test.go` | T10 | Service full lifecycle CRUD |
| `therapist_service_mapping_test.go` | T11 | Soft-reconcile mapping semantics |
| `availability_put_test.go` | T12 | Availability PUT + validation rules |
| `cross_branch_isolation_test.go` | T13 | Cross-tenant + cross-branch isolation |
| `seed_data_integrity_test.go` | T14 | Migration 14 seed data sanity |

## Harness packages

| File | Purpose |
|---|---|
| `harness/client.go` | Typed HTTP helpers for all endpoints |
| `harness/phase4_types.go` | Phase 4 request/response structs |
| `harness/db.go` | Shared `*sql.DB` + `TxWithTenant` helper |
| `harness/jwt.go` | JWT payload decoder (no signature verification) |
| `harness/random.go` | Unique email / slug / IP generators |
| `harness/mailpit.go` | Mailpit API client for email assertions |
