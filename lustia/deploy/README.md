# Lustia — local dev stack

Everything needed to run Phase 1 + Phase 2 (Postgres, migrator, auth-service,
Mailpit SMTP catcher) on a laptop, via a single `docker compose up`.

Full operational detail lives in [`../../docs/OPERATIONS.md`](../../docs/OPERATIONS.md).

---

## Prerequisites

- Docker Engine 24+ with Compose v2 (`docker compose`, not `docker-compose`).
- `openssl` in PATH (used once, to generate the dev JWT key pair).
- `curl` (or any HTTP client) to smoke-test `/readyz`.

No local Go toolchain is required to run the stack. A toolchain IS required if
you want to run the auth-service on the host against the containerized DB
(see "Local `go build` vs container build" below).

---

## Quick start

From `lustia/deploy/`:

```bash
# 1. Environment
cp .env.example .env
$EDITOR .env                # replace every CHANGE_ME_* value

# 2. Dev JWT key pair (4096-bit RSA — SECURITY.md § 2.4)
#    Run these from lustia/deploy/ so the paths are right.
openssl genrsa -out secrets/jwt_private.pem 4096
openssl rsa   -in  secrets/jwt_private.pem -pubout -out secrets/jwt_public.pem
chmod 600 secrets/jwt_private.pem
#    ⚠  Dev only. Staging / prod load this key from a secret manager (Vault,
#       AWS / GCP / Azure secret stores). Never commit these PEM files.

# 3. Bring the stack up
docker compose up -d

# 4. Watch the one-shot migrator finish
docker compose logs -f migrator
#    Exit when you see `no change` or the last migration applied. The
#    container's Exited (0) status is the success signal the auth-service
#    depends on.

# 5. Smoke test — auth + Mailpit
curl -sf localhost:8080/readyz && echo
#    Expected: {"status":"ok"}

# Open the Mailpit inbox. Any email the auth-service dispatches
# (password-reset link in Phase 2, more flows later) lands here.
#    http://localhost:8025

# 6. Bootstrap super-admin password (ADR 0003 — post-migration step)
#    The seed migration inserts the super-admin with an unusable placeholder
#    hash. Set a real Argon2id hash, then log in with it.
#
#    Generate the hash — the argon2id CLI, or a one-shot Go snippet, or
#    https://argon2.online/ (dev only). Use parameters: t=3, m=64 MiB, p=2
#    (SECURITY.md § 2.1).
#
#    Apply it:
docker compose exec -T postgres psql \
    -U "$POSTGRES_USER" -d "$POSTGRES_DB" \
    -c "UPDATE \"user\" SET password_hash = '<your_argon2id_hash>' \
        WHERE id = 'a0000000-0000-0000-0000-000000000001';"

# 7. Log in, verify the JWT, rotate the password
curl -sf -X POST localhost:8080/api/v1/auth/login \
     -H 'Content-Type: application/json' \
     -d '{"email":"admin@lustia.local","password":"<your_password>","tenant_slug":"__platform__"}'
#    The access token carries must_change_password=true. Every endpoint
#    except GET /auth/me, POST /auth/me/password, and POST /auth/logout
#    returns 403 PASSWORD_CHANGE_REQUIRED until you rotate via
#    POST /api/v1/auth/me/password.

# 8. (optional) Try the forgot-password flow end-to-end via Mailpit
curl -sf -X POST localhost:8080/api/v1/auth/password/forgot \
     -H 'Content-Type: application/json' \
     -d '{"email":"admin@lustia.local","tenant_slug":"__platform__"}'
#    Expect HTTP 204. Open http://localhost:8025 — the reset email with
#    a ?token=... link will be waiting in the Mailpit inbox.
```

Teardown:

```bash
docker compose down            # stop + remove containers, keep DB volume
docker compose down -v         # also drop the DB volume (re-runs init-db)
```

---

## Local `go build` vs container build

The auth-service `go.mod` has:

```
replace github.com/chrisanlung/common-configs => ../../../../../GO/common-configs
```

This `replace` points OUTSIDE the `lustia/` tree
(`F:/Projects/GO/common-configs`). That is convenient for **host-side**
development — edit `common-configs/`, run `go build ./...` in `services/auth/`,
see changes immediately without publishing.

Docker images cannot reach outside their build context, so the Dockerfile
does not honour the replace. Instead it fetches `common-configs` from the
public GitHub repo during `go mod download`. This is **Option C** — see
[`../../docs/OPERATIONS.md` § Image build](../../docs/OPERATIONS.md) for the
alternatives considered and why this one was chosen.

Practical consequence:

| Where | How common-configs is resolved |
|---|---|
| `go build` / `go test` on your host | local `replace` → fast iteration |
| `docker compose build auth`        | fetched from GitHub at build time |

When you need an image to include in-flight changes to `common-configs`,
push those changes to GitHub first (any commit is fine — `go mod tidy` will
pin a `v0.0.0-<date>-<sha>` pseudo-version), then `docker compose build auth`.

---

## Troubleshooting

- **`docker compose up` fails with "LUSTIA_APP_PASSWORD is required"** — you
  skipped step 1 or the compose file is reading a different `.env` (it must
  live next to `docker-compose.yml`).
- **`/readyz` returns 503 `db_unreachable`** — migrations are still running
  or the `lustia_app` password in `.env` does not match what init-db set.
  `docker compose down -v && up` rebuilds from scratch.
- **Migrator exits non-zero** — inspect `docker compose logs migrator`. Most
  failures are SQL errors in a migration. Fix the SQL, then
  `docker compose up migrator` to retry. See the "Migration failed mid-way"
  runbook in `OPERATIONS.md`.
- **JWT key mount fails on Windows** — ensure Docker Desktop has the
  `lustia/` path shared, and that `secrets/jwt_private.pem` exists with
  non-empty contents.
- **Mailpit inbox is empty after `forgot-password`** — check that
  `SMTP_ENABLED=true` and `SMTP_HOST=mailpit` in `.env`, that `docker compose
  logs mailpit` shows the container running, and that the requested email
  matches a real user in the DB (forgot-password silently succeeds for
  unknown emails — anti-enumeration). Auth-service logs "email sender:
  disabled" at startup when SMTP is off.

---

## File layout

```
deploy/
├── README.md                    ← you are here
├── docker-compose.yml           ← the stack
├── .env.example                 ← copy to .env
├── init-db/
│   └── 01-create-app-role.sh    ← runs once on first Postgres boot
└── secrets/
    ├── .gitkeep
    ├── jwt_private.pem          ← generated by openssl, gitignored
    └── jwt_public.pem           ← generated by openssl, gitignored
```
