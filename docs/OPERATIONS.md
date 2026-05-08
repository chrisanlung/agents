# Operations — Lustia

_Owned by `devops-expert`. Every change that touches Docker images, compose,
CI, deployment, or runtime infra updates this file in the same turn as the
diff._

_Last updated: 2026-04-25 — Phase 4 storage abstraction (ADR 0011, devops-expert)._

---

## 1. Runtime topology (dev-local)

Phase 1+2 runs as **four containers** on one Docker network, composed from
`lustia/deploy/docker-compose.yml`:

| Name       | Image                          | Role                                                             | Depends on                                  |
| ---------- | ------------------------------ | ---------------------------------------------------------------- | ------------------------------------------- |
| `postgres` | `postgres:16-alpine`           | Single cluster, single database (`lustia`). Hosts `lustia_app` and `lustia_migrator` roles; enforces RLS for `lustia_app` traffic. Named volume `lustia_postgres_data`. First-boot init scripts in `deploy/init-db/` create the `lustia_app` role. | —                                           |
| `migrator` | `migrate/migrate:v4.17.1`      | One-shot. Runs `/migrations` forward (`up`) and exits 0. Pinned image tag. Restart policy `no`. For Phase 2 dev, runs as the Postgres superuser (the dedicated `lustia_migrator` role is itself created inside migration 000001). | `postgres` healthy                          |
| `mailpit`  | `axllent/mailpit:v1.20`        | Local SMTP catcher + web UI. Accepts any credentials; no persistence (inbox cleared on restart). Exposes SMTP on `${MAILPIT_SMTP_PORT:-1025}` and UI on `${MAILPIT_UI_PORT:-8025}`. Swap to a real provider (AWS SES, SendGrid, Postmark) for staging/prod by pointing `SMTP_HOST`/credentials elsewhere. | —                                           |
| `auth`    | `lustia-auth:dev` (built local) | Gin + GORM Go service. Listens on `${AUTH_PORT}`. Connects as `lustia_app`. Loads JWT private key from a mounted PEM file. Dispatches password-reset emails via SMTP (Mailpit in dev). Serves uploaded files at `GET /uploads/*` when `STORAGE_DRIVER=local`. | `migrator` completed successfully; `postgres` healthy; `mailpit` healthy |

**Named volumes:**

| Volume                  | Container path   | Purpose                                                        |
| ----------------------- | ---------------- | -------------------------------------------------------------- |
| `lustia_postgres_data`  | `/var/lib/postgresql/data` | Postgres WAL + data files. Persistent across restarts.  |
| `lustia_uploads_data`   | `/app/uploads`   | Local-driver upload storage for the auth-service. Must be included in any DB snapshot that captures `photo_key` values (see §11). Owned by uid 65532 (distroless nonroot). |

Shape: health endpoints (`/healthz`, `/readyz`), structured logs, graceful
shutdown, background refresh-token cleanup goroutine. Resource limits are
intentionally not set in the dev compose file — the host picks defaults.

```
  ┌──────────────┐          ┌─────────────┐
  │   postgres   │          │   mailpit   │  UI :8025, SMTP :1025 (published)
  └──────┬───────┘          └──────┬──────┘
         │                         │
    ┌────┴────────┐                │
    │             │                │
┌───▼──────┐  ┌───▼──────────────────────┐
│ migrator │  │           auth           │◄── SMTP (password-reset mail)
└──────────┘  │  :${AUTH_PORT} published │
  (exits 0)   │  GET /uploads/* (local)  │
              └───────────┬──────────────┘
                          │
                 ┌────────▼──────────┐
                 │  lustia_uploads_  │   named volume (uid 65532)
                 │      data         │   /app/uploads
                 └───────────────────┘
```

Open `http://localhost:8025` during development to view any email the
auth-service dispatched. The forgot-password flow delivers a real message to
Mailpit; clicking the link (local frontend not yet running) verifies the
`?token=` payload round-trips correctly.

---

## 2. Environments

| Env         | URL  | Host    | Deploys from                            | Approval | Data                        |
| ----------- | ---- | ------- | --------------------------------------- | -------- | --------------------------- |
| `dev-local` | —    | laptop  | `docker compose up -d` in `lustia/deploy/` | —        | seeded from migrations only |
| `dev-vm`    | TBD  | 1× VM   | `git pull` + `docker compose up -d` on the VM (systemd for reboot survival) | manual (operator on VM) | seeded + operator-generated |
| `staging`   | TBD  | deferred | deferred                                | deferred | deferred                    |
| `prod`      | TBD  | deferred | deferred                                | deferred | deferred                    |

`dev-local` and `dev-vm` both run the same `docker-compose.yml`. The only
difference is where `.env` lives, how the stack survives reboots, and the
network exposure. See [`../lustia/deploy/vm/README.md`](../lustia/deploy/vm/README.md)
for the VM setup + ongoing-operations runbook.

`staging` and `prod` are deferred to the phase that introduces a real
deployment target (see ADR 0006 for the graduation path).

### 2.2 Storage env vars per environment (ADR 0011)

| Var | dev-local / dev-vm | staging | prod |
| --- | --- | --- | --- |
| `STORAGE_DRIVER` | `local` | `r2` or `supabase` — **TBD; see prod-flip checklist §10.4** | same as staging |
| `STORAGE_LOCAL_PATH` | `/app/uploads` (compose) / `$(pwd)/storage-data/uploads` (run-local.sh) | — (local driver not used) | — |
| `STORAGE_PUBLIC_BASE_URL` | `http://localhost:8080/uploads` | — | — |
| `STORAGE_R2_ACCOUNT_ID` | — | see §8 | see §8 |
| `STORAGE_R2_ACCESS_KEY_ID` | — | see §8 | see §8 |
| `STORAGE_R2_SECRET_ACCESS_KEY` | — | see §8 | see §8 |
| `STORAGE_R2_BUCKET` | — | see §8 | see §8 |
| `STORAGE_SUPABASE_URL` | — | see §8 | see §8 |
| `STORAGE_SUPABASE_SERVICE_KEY` | — | see §8 | see §8 |
| `UPLOAD_MAX_MB` | `5` | `5` (review before flip) | `5` (review before flip) |
| `UPLOAD_TENANT_HOURLY_LIMIT` | `30` | `30` (review before flip) | `30` (review before flip) |

`STORAGE_DRIVER` has **no default** — it is a fail-fast required var. An
unset or unknown value causes the service to exit at startup. Switching
staging or prod from `local` to `r2` / `supabase` requires the prod-flip
checklist (§10.4) to be completed first.

### 2.1 Secret handling per environment

Tracks ADR 0006 two-step path for VM deployments:

| Env | Secret at rest | Secret in memory | Access |
| --- | --- | --- | --- |
| `dev-local` | `.env` on laptop (gitignored); PEM in `deploy/secrets/` | env vars in container | whoever has the laptop |
| `dev-vm` (Step 1) | `.env` at `/srv/lustia/lustia/deploy/.env`, mode `0600`, owner `lustia` | env vars in container | SSH-authorized users on the VM |
| `dev-vm` / `staging` (Step 2) | SOPS-encrypted `.env.{env}.enc` committed to git; age key at `~/.config/sops/age/keys.txt` on the VM | env vars (decrypted at deploy time, written to tmpfs) | SSH + holder of the age recipient key |
| `staging` / `prod` (Step 3) | Cloud secret manager (AWS SM / Vault) | env vars or mounted file, injected by External Secrets Operator / CSI driver | IAM / Vault ACL per environment |

**Migration triggers** (second operator, real user data, multi-VM, k8s move) are
in ADR 0006 §"Migration triggers".

---

## 3. Image build

### 3.1 auth-service image

Multi-stage, defined in `lustia/services/auth/Dockerfile`.

- Build stage: `golang:1.24-alpine`. `CGO_ENABLED=0`, `GOFLAGS=-trimpath`,
  `-ldflags="-s -w"`. Static binary. `go mod verify` runs before compile.
- Runtime stage: `gcr.io/distroless/static-debian12:nonroot`. No shell, no
  package manager, uid 65532. `USER nonroot:nonroot` is explicit. No
  healthcheck in the Dockerfile — the image has no probe binary, so the
  compose-level check and the caller hitting `/readyz` fill that role.
- The service listens on `${HTTP_PORT}` (default 8080). `EXPOSE 8080` is
  documentation only; the compose file publishes the actual port.

### 3.2 Pinned third-party images

| Image         | Version          | Why pinned                                             |
| ------------- | ---------------- | ------------------------------------------------------ |
| `postgres`    | `16-alpine`      | PostgreSQL 16 is the project minimum per PRD § 6.      |
| `migrate/migrate` | `v4.17.1`    | Pin migration tooling exactly to avoid silent CLI drift. |
| `golang`      | `1.24-alpine`    | Matches the `go 1.24` directive in `go.mod`.           |
| `gcr.io/distroless/static-debian12` | `nonroot` | Base for the final image (floating tag — acceptable for dev; see open items). |

Image-digest pinning (`@sha256:…`) for all base images is tracked as an open
item once the CI pipeline lands (see § 11).

### 3.3 Build-context decision (common-configs replace)

The auth-service `go.mod` has

```go
replace github.com/chrisanlung/common-configs => ../../../../../GO/common-configs
```

pointing OUTSIDE the `lustia/` tree to `F:/Projects/GO/common-configs`.
Docker cannot reach outside its build context. Three options were considered:

- **Option A — include both trees in the build context.** Requires setting
  the compose `build.context` above both `lustia/` and `GO/common-configs/`.
  These two directories are siblings under `F:/Projects/` but `lustia/` is
  already nested below `lusthing/`, and `common-configs/` is under
  `GO/`. There is no single ancestor that is also a reasonable source root.
  Rejected.

- **Option B — relative symlink trickery.** A symlink inside
  `services/auth/` pointing at the external tree would need to resolve
  inside the Docker build context. Docker follows symlinks that stay within
  the context and rejects ones that escape. Rejected on portability grounds
  (symlinks on Windows require admin or developer mode).

- **Option C — do NOT use the local `replace` for Docker builds.** The
  Dockerfile runs `go mod download` with the build context at
  `services/auth/`. The `replace` line is present in `go.mod` but its target
  path does not exist inside the container; `go mod download` then falls back
  to the module proxy and fetches `common-configs` from the public GitHub
  repo. Local `go build` on the host continues to honour the replace for
  fast iteration. **Accepted.**

Consequences:

- Developers editing `common-configs/` who want the change in an image must
  push to GitHub first (any commit; `go mod tidy` pins to a
  `v0.0.0-<date>-<sha>` pseudo-version). Host-side `go build` sees the change
  immediately without a push.
- `go.sum` must contain a real entry for `common-configs` (not the
  zero-value the replace would otherwise allow). Flag for `go-expert`:
  run `go mod tidy` against the remote repo once the service is wired up,
  commit `go.sum`. If a fetch fails in CI with "no matching versions", that's
  the fix.
- Revisit when `common-configs` has tagged releases (`v1.0.0+`). At that
  point the replace can be removed entirely and both host + image resolve
  identically.

---

## 4. CI pipeline (shape)

`.github/workflows/` files are not in scope for Phase 2 — the actual YAML is
tracked as open item #4 below. The pipeline shape every job must implement is:

| Job                   | Purpose                                                                                   |
| --------------------- | ----------------------------------------------------------------------------------------- |
| `lint`                | `golangci-lint run ./...` against each service module. Fails on any warning.              |
| `test`                | `go test -race ./...` per service. Race detector mandatory.                              |
| `govulncheck`         | `govulncheck ./...` — blocks on any known CVE in the module graph (SECURITY § 8).        |
| `mod-verify`          | `go mod verify` + `go mod tidy -diff` (no uncommitted tidy changes allowed).              |
| `build`               | `docker build services/auth/` and tag with the commit SHA. Image is pushed only on main.  |
| `migrations-up-test`  | Spin up `postgres:16-alpine`, run migrations `up`, run `down 1`, run `up` again. All steps must return 0. |

Requirements for each workflow file (SECURITY § 8):

- Pin every `uses: actions/…` line to a full commit SHA, not a tag.
- `permissions: contents: read` as the workflow default; escalate per-job
  only when a job needs to write (publish, release).
- OIDC federation from GitHub Actions into the cloud provider when
  staging/prod lands — no long-lived access keys.

Open item: create the actual `.github/workflows/*.yml` in Phase 10 (or when
this repo is promoted to a CI-watched remote).

---

## 5. Release strategy

Deferred. The auth-service Docker image is deployable to any container
runtime (k8s, ECS, Cloud Run, Fly.io). Choice of runtime and release
cadence is made at the point staging is provisioned.

When the decision lands, update this section with:

- Tagging convention (semver vs commit-SHA vs date-based).
- Canary / blue-green / rolling strategy.
- Approval gates.

---

## 6. DB migration strategy

- **Forward-only in staging/prod.** Every change is a new migration file;
  no edits to applied migrations.
- **Backward (reversible) in dev.** Every `up.sql` has a matching
  `down.sql`. To walk back locally:
  ```bash
  docker compose run --rm migrator \
      -path=/migrations \
      -database="postgres://${POSTGRES_USER}:${POSTGRES_PASSWORD}@postgres:5432/${POSTGRES_DB}?sslmode=disable" \
      down 1
  ```
- **Role separation.** For Phase 2 dev, the migrator runs as the Postgres
  superuser (migration 000001 itself creates `lustia_migrator` and sets it
  `BYPASSRLS`). For staging/prod, run migrations as the dedicated
  `lustia_migrator` role once it exists — this is an open item.
- **Application traffic never uses the migrator role.** `lustia_app` is the
  only role the auth-service (and every later service) uses.

---

## 7. Rollback

### 7.1 auth-service

- **Code path:** redeploy the previous image tag. The Docker image is the
  only build artifact; there is no in-place in-container patching.
- **State:** refresh tokens live in the DB (`refresh_token` table), so user
  sessions survive an auth-service redeploy. Access tokens are stateless
  JWTs — no impact.
- **JWT keys:** the `kid` is stable unless explicitly rotated. Rolling back
  to a previous image with the same key material is safe. Rolling back
  across a key-rotation boundary requires the JWKS to still advertise the
  older key (see SECURITY § 9.2).

### 7.2 Migrations

If a migration has already landed in staging/prod, **do not** edit the
applied `up.sql`. Write a new forward migration that corrects the issue. The
`down.sql` is intended for dev; applying `down` in production is an
exceptional operation that requires a runbook review.

---

## 8. Secrets inventory

| Secret                                    | Location (dev-local)                             | Consumer                         | Rotation (cadence)                  |
| ----------------------------------------- | ------------------------------------------------ | -------------------------------- | ----------------------------------- |
| Postgres superuser password               | `deploy/.env` → `POSTGRES_PASSWORD`              | `postgres`, `migrator` containers | Semi-annually (placeholder — prod)  |
| `lustia_app` password                     | `deploy/.env` → `LUSTIA_APP_PASSWORD`            | `auth` container, init-db script | Semi-annually; immediately on compromise |
| JWT RS256 private key                     | `deploy/secrets/jwt_private.pem` (dev only)      | `auth` container                 | Annually; immediately on compromise |
| Super-admin bootstrap email               | `deploy/.env` → `SUPER_ADMIN_EMAIL`              | post-migration SQL step          | Replace before staging (one-time)   |
| Super-admin initial password              | `deploy/.env` → `SUPER_ADMIN_INITIAL_PASSWORD`   | post-migration SQL step          | On first login (forced — see flag below) |
| `STORAGE_R2_ACCOUNT_ID`                   | **TBD — not yet provisioned** (see note below)   | `auth` container (r2 adapter)    | On compromise; annually             |
| `STORAGE_R2_ACCESS_KEY_ID`                | **TBD — not yet provisioned** (see note below)   | `auth` container (r2 adapter)    | On compromise; annually             |
| `STORAGE_R2_SECRET_ACCESS_KEY`            | **TBD — not yet provisioned** (see note below)   | `auth` container (r2 adapter)    | On compromise; annually             |
| `STORAGE_R2_BUCKET`                       | **TBD — not yet provisioned** (see note below)   | `auth` container (r2 adapter)    | Static (rename requires migration)  |
| `STORAGE_SUPABASE_URL`                    | **TBD — not yet provisioned** (see note below)   | `auth` container (supabase adapter) | On compromise; annually          |
| `STORAGE_SUPABASE_SERVICE_KEY`            | **TBD — not yet provisioned** (see note below)   | `auth` container (supabase adapter) | On compromise; annually          |

**Storage credential note (ADR 0011):** the six R2 / Supabase vars above are
required only when `STORAGE_DRIVER=r2` or `=supabase`. They are not yet
provisioned because the prod-flip checklist (§10.4) has not been completed.
When provisioned, they follow the same ADR 0006 two-step path as other
secrets: `.env` on dev-vm, then secret manager for staging/prod. Service
credentials must be scoped to `PutObject` + `DeleteObject` only (no list, no
global read).

**Dev rules** (enforced here; staging/prod has its own inventory):

- `.env` is gitignored; only `.env.example` is committed.
- PEM key material is gitignored; `deploy/secrets/.gitkeep` preserves the
  directory.
- Startup log prints only the `kid` (SHA-256 thumbprint of the public key),
  never the key material (SECURITY § I-4).

**Staging/prod** secrets management is deferred to a secret manager — see
open item #3.

---

## 9. Observability

Phase 2 baseline — every item below exists today; depth is deferred.

- **Logs:** structured JSON via `common-configs/log`. Every line carries
  `timestamp`, `trace_id`, `request_id`. Redaction rules in
  SECURITY § 7.2 are enforced by review plus the `TestNoSensitiveFieldsInLogs`
  regression test (qa-expert).
- **Metrics:** not exported in Phase 2. Add a `/metrics` Prometheus endpoint
  as part of open item #6.
- **Traces:** OTel SDK is wired into `common-configs` and accepts a
  configurable endpoint. No collector is running in `docker-compose.yml` yet;
  the service exports traces only when the endpoint env var is set.
- **Health probes:** `/healthz` (liveness — process is up) and `/readyz`
  (readiness — can reach the DB). Both are served at the root. Compose relies
  on process liveness + the `depends_on` chain; host-side consumers should
  call `/readyz` directly (the port is published).

### 9.1 Storage metrics (ADR 0011)

Three Prometheus metrics prepared by the storage layer (emitted now;
scraped once open item #6 lands):

| Metric | Type | Labels | Description |
| --- | --- | --- | --- |
| `storage_upload_total` | Counter | `driver`, `tenant_id`, `status` (`ok`/`fail`) | Incremented once per upload attempt. Alert on `status="fail"` rate. |
| `storage_upload_bytes_bucket` | Histogram | `driver` | Bytes written per upload. Standard Prometheus buckets. |
| `storage_delete_total` | Counter | `driver`, `tenant_id`, `status` (`ok`/`fail`) | Incremented per delete attempt (fire-and-forget; failure is non-fatal but tracked here). |

### 9.2 Required structured-log fields on every upload event

Every upload event emitted by `internal/helper/storage/` and the upload
handler must carry these fields. Missing fields block code review.

| Field | Example | Notes |
| --- | --- | --- |
| `tenant_id` | `"acme-spa"` | From the request context. |
| `photo_key` | `"therapists/abc123/7f3a2b1c.jpg"` | Opaque key written to DB. Never a presigned URL. |
| `size_bytes` | `204800` | After EXIF stripping; the bytes actually written to storage. |
| `driver` | `"local"` | Value of `STORAGE_DRIVER` at startup. |
| `duration_ms` | `47` | Wall-clock time for the `Storage.Upload` call only. |
| `trace_id` | `"4bf92f3577b34da6..."` | Propagated from OTel context. |

**Never log** file contents, presigned URLs with embedded credentials, or
raw multipart bodies.

---

## 10. Runbooks

### 10.1 Service fails `/readyz`

**Symptom:** `curl localhost:${AUTH_PORT}/readyz` returns 503 with body
`{"status":"db_unreachable"}` (or `db_unavailable`).

1. `docker compose ps` — is `postgres` up and healthy? If not, jump to the
   migration runbook below.
2. `docker compose logs postgres --tail=50` — look for "database system is
   ready to accept connections". If Postgres is still initializing (first
   boot can take 10–20s), wait and retry.
3. `docker compose exec postgres psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" -c '\du'`
   — does `lustia_app` exist and have `LOGIN`? If missing, `init-db`
   failed; inspect `docker compose logs postgres | grep init-db`. Fix: repair
   the script, `docker compose down -v` (destroys data), `up` again.
4. `docker compose logs auth --tail=100` — look for a DB dial error. The
   error message will quote the connection string (with password elided).
   Mismatch between `LUSTIA_APP_PASSWORD` in `.env` and what init-db set is
   the most common cause.
5. If all else looks right, `docker compose restart auth`. Persistent
   failures after a restart: escalate — likely a code bug in the repository
   layer, not infra.

### 10.2 Migration failed mid-way

**Symptom:** `migrator` exits non-zero; `auth` stays unstarted because
`depends_on: service_completed_successfully` blocks.

1. `docker compose logs migrator` — note the version that failed and the
   SQL error. Migration tooling marks the failed version as `dirty` in the
   `schema_migrations` table.
2. Inspect the DB:
   ```bash
   docker compose exec postgres psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" \
       -c 'SELECT * FROM schema_migrations;'
   ```
3. Fix the SQL in the offending `.up.sql`. If the migration partially
   applied, you may need to clean up manually or force:
   ```bash
   docker compose run --rm migrator \
       -path=/migrations \
       -database="postgres://${POSTGRES_USER}:${POSTGRES_PASSWORD}@postgres:5432/${POSTGRES_DB}?sslmode=disable" \
       force <last_good_version>
   ```
4. Re-run `up`:
   ```bash
   docker compose up migrator
   ```
5. Once `migrator` exits 0, `docker compose up -d auth`.

### 10.3 Bootstrap super-admin locked out / forgot initial password

**Symptom:** nobody knows the super-admin password; `POST /auth/login`
returns `INVALID_CREDENTIALS`; no other account can reset it.

Per ADR 0003, the super-admin hash is replaceable by any operator with DB
access. Phase 2 does not yet enforce `must_change_password`, so the password
remains whatever was set by the last post-migration UPDATE.

1. Generate a new Argon2id hash (parameters: t=3, m=64 MiB, p=2 —
   SECURITY § 2.1). Any Go one-shot works; avoid public online tools for
   any environment that has seen real user data.
2. Apply it:
   ```bash
   docker compose exec -T postgres psql \
       -U "$POSTGRES_USER" -d "$POSTGRES_DB" \
       -c "UPDATE \"user\" \
           SET password_hash = '<new_argon2id_hash>', \
               failed_login_count = 0, \
               locked_until = NULL \
           WHERE id = 'a0000000-0000-0000-0000-000000000001';"
   ```
3. Log in with the new password. Immediately change it via
   `PUT /api/v1/auth/me/password`.
4. Write a `security.super_admin_password_reset` row to `audit_log` with
   the operator's identifier and reason. (Phase 2 does this via the login
   flow's standard audit writes; for an out-of-band reset, insert manually.)

### 10.4 Prod-flip checklist — switching `STORAGE_DRIVER` to `r2` or `supabase`

All ten items must be true before `STORAGE_DRIVER=r2` or `=supabase` is set
in any non-dev environment. The operator completing this checklist signs off
in the commit message referencing this section.

1. **Bucket denies public `ListObjects` / `ListBucket`.** Verify via the
   provider console or CLI before deploying.
2. **Service credential permissions limited to `PutObject` + `DeleteObject`.**
   No list, no global read. Scope the IAM key / API token to the bucket only.
3. **Bucket CORS explicitly allowlists frontend origins.** Not `*`. List each
   expected origin (`https://admin.lustia.id`, etc.).
4. **Credentials stored in Vault or cloud secret manager per ADR 0006.**
   The six R2 / Supabase vars in §8 must come from the secret manager, not a
   plain `.env` file, for staging/prod.
5. **Observability collecting.** `/metrics` endpoint exposed and Prometheus is
   scraping `storage_upload_total`, `storage_upload_bytes_bucket`,
   `storage_delete_total`. Log shipper is forwarding upload events with all
   fields from §9.2.
6. **`/readyz` validates storage health at startup.** The go-expert must wire
   a storage reachability check into the readiness probe before the flip.
   (This is a flag to go-expert — see downstream notes.)
7. **Multi-replica guard active.** `STORAGE_DRIVER=local` with
   `KUBERNETES_SERVICE_HOST` set must still cause a startup hard-fail
   (ADR 0011 §2.2). Confirm this check is in place before deploying to any
   orchestrated environment.
8. **`UPLOAD_MAX_MB` and `UPLOAD_TENANT_HOURLY_LIMIT` reviewed** and committed
   in the environment config. Default values (5 MB, 30/hr) may be too tight or
   too loose for production tenants.
9. **Alert wired on `storage_upload_total{status="fail"}` rate > X%.**
   Agree on the threshold (suggested: > 5% over 5 minutes) and confirm the
   alert fires in the staging alerting stack before promoting to prod.
10. **Runbooks §10.5 and §10.6 tested end-to-end** in staging. At least one
    operator has walked through each runbook against a real storage failure
    scenario (use a deliberately mis-configured credential to simulate §10.5).

### 10.5 Photo missing in API response (DB has `photo_key`, storage returns 404)

**Symptom:** therapist DTO has `photo_url` set (non-null) but the URL returns
HTTP 404. Or `Storage.URL()` logs an error during DTO mapping.

1. Check `STORAGE_DRIVER` in the running container:
   ```bash
   docker compose exec auth env | grep STORAGE_DRIVER
   ```
2. If `driver=local`, verify the file exists on the volume:
   ```bash
   docker compose exec auth ls /app/uploads/<photo_key>
   ```
   If missing, the volume was wiped or the container was recreated without
   reattaching the named volume. See the split-brain note below.
3. If `driver=r2` or `supabase`, check object presence via the provider CLI
   or console using the exact `photo_key` value from the DB:
   ```sql
   SELECT photo_key FROM therapist WHERE id = '<id>';
   ```
4. Verify `STORAGE_PUBLIC_BASE_URL` (local) or the R2/Supabase endpoint
   (`STORAGE_R2_*` / `STORAGE_SUPABASE_URL`) matches the environment. A URL
   resolver misconfiguration produces structurally valid but wrong URLs.
5. **Split-brain scenario:** the DB was restored from backup but the storage
   volume / bucket was not restored to the same point in time. `photo_key`
   values in the DB reference objects that no longer exist.
   - For `local`: restore the `lustia_uploads_data` volume from the same
     snapshot as the DB restore (see §11).
   - For R2/Supabase: run the storage consistency check in §11 to enumerate
     dangling keys.
6. **Remediation:** if the file is unrecoverable, null the `photo_key`:
   ```sql
   UPDATE therapist SET photo_key = NULL WHERE id = '<id>';
   ```
   The admin can then re-upload via the UI. Notify the tenant.

### 10.6 Tenant upload volume spike (quota hit or unexpected cost)

**Symptom:** a tenant is hitting 429 responses on `POST /tenant/therapists/:id/photo`,
or storage costs are spiking unexpectedly.

1. Identify the tenant by querying `storage_upload_total{status="ok"}` grouped
   by `tenant_id` over the last hour in your metrics backend (Prometheus /
   Grafana / CloudWatch). Sort descending by count.
2. Inspect recent upload log events for the tenant to understand the pattern:
   ```bash
   docker compose logs auth --tail=500 | grep '"tenant_id":"<slug>"' | grep photo_key
   ```
3. Verify that `UPLOAD_TENANT_HOURLY_LIMIT` is set to the expected value:
   ```bash
   docker compose exec auth env | grep UPLOAD_TENANT_HOURLY_LIMIT
   ```
   The in-memory window resets on service restart. A recent restart may have
   cleared an earlier quota hit.
4. Engage with the tenant to understand the root cause — bulk import, scripted
   upload loop, or a misbehaving client.
5. If the spike is intentional and the product team approves a higher limit,
   update `UPLOAD_TENANT_HOURLY_LIMIT` in the env config and redeploy. A
   per-tenant override is not implemented in Phase 4; treat this as a global
   limit change.
6. If the uploads are abusive or credential-compromised, revoke the tenant's
   API tokens via the admin UI (or directly: `DELETE FROM refresh_token WHERE
   user_id IN (SELECT id FROM "user" WHERE tenant_id = '<slug>')`). Contact
   the tenant.
7. Document the incident in `docs/DECISIONS/` or the incident log so the
   per-tenant quota feature (Phase 5 backlog) is prioritized correctly.

---

## 11. Backups & DR

N/A for `dev-local`. Volume `lustia_postgres_data` is disposable by design;
`docker compose down -v` wipes it. All state is re-createable from
migrations plus the bootstrap runbook.

Staging/prod backup strategy (PITR, retention, tested restores) is
deferred — see open item #3 (the secret manager work and backups are the
same milestone).

### 11.1 Storage backup implications (ADR 0011)

The introduction of `photo_key` in `therapist` creates a cross-store
dependency that backup and DR procedures must respect.

**Local driver (`STORAGE_DRIVER=local`):**
- The `lustia_uploads_data` named volume is the authoritative store for all
  uploaded files. It is logically coupled to the Postgres `lustia_postgres_data`
  volume: a `photo_key` value in the DB references an object in the volume.
- **Any DB snapshot must be co-snapshotted with the uploads volume at the
  same point in time.** Restoring the DB without restoring the uploads volume
  (or vice versa) produces a split-brain state where `photo_key` values
  reference missing files (runbook §10.5).
- In `dev-local` this is disposable by design; `docker compose down -v`
  wipes both. For `dev-vm`, if the VM disk is snapshotted, snapshot both
  volumes in a single consistent operation.

**R2 / Supabase driver:**
- Cloud object storage is independently durable (multi-AZ by default). No
  additional snapshot job is required for the objects themselves.
- **DR exercises must verify key-to-object integrity.** After any DB restore,
  run the storage consistency check below to confirm that every `photo_key`
  in the DB resolves to an existing object in the bucket.
- Bucket versioning (R2: object versioning; Supabase: not natively supported)
  is a future decision. Accepted risk in Phase 4.

### 11.2 Storage consistency check (manual procedure)

Run after any DB restore or storage migration. Automation is a future task.

```bash
# 1. Export all non-null photo_key values from the DB.
docker compose exec -T postgres psql \
    -U "$POSTGRES_USER" -d "$POSTGRES_DB" \
    -t -A -c "SELECT photo_key FROM therapist WHERE photo_key IS NOT NULL;" \
    > /tmp/photo_keys.txt

# 2a. For local driver: verify each file exists on the volume.
while IFS= read -r key; do
    docker compose exec auth test -f "/app/uploads/$key" \
        || echo "MISSING: $key"
done < /tmp/photo_keys.txt

# 2b. For R2: use the AWS CLI (R2 is S3-compatible).
#   aws s3api head-object --bucket "$STORAGE_R2_BUCKET" --key "$key"
#   (Wrap in the same loop; a 404 response means MISSING.)

# 2c. For Supabase: use the Supabase CLI or REST API to HEAD each key.
```

Any `MISSING` output means a `photo_key` in the DB has no backing object.
Remediate per runbook §10.5 step 6 (null the key; re-upload via admin UI).

---

## 12. Cost

N/A for `dev-local`. Cost tagging (`app`, `env`, `owner`, `cost-center`),
budget alerts, and per-env cost views land with staging.

---

## 13. Open items for devops (prioritized)

1. **Move the rate limiter to Redis.** Today the auth-service uses an
   in-memory `x/time/rate`-style bucket (`services/auth/infra/ratelimit_memory.go`).
   This only works correctly with one instance of the service. Before any
   horizontal scaling, add `redis:7-alpine` to the compose stack and
   replace the limiter implementation with a Redis-backed sliding-window
   script. SECURITY § 11 item 7.

2. **JWT key rotation story.** Today a single `kid` ("primary") is signed
   with one private key loaded from disk. JWKS surfaces one key.
   Implement: multi-key JWKS (keep old public key for 20 minutes after
   rotation per SECURITY § 9.2), a rotation runbook, and a configuration
   schema that lists `{kid, private_key_path, public_key_path}` tuples.

3. **Secret manager for staging/prod.** Evaluate Vault vs. cloud-native
   (AWS Secrets Manager / GCP Secret Manager / Azure Key Vault) and pick
   one. JWT private key, DB passwords, and bootstrap super-admin password
   all move to it. SECURITY § 5.2.

4. **CI actions pinned + `govulncheck`.** Create
   `.github/workflows/ci.yml` implementing the job shape described in § 4.
   Every `uses:` pinned to a commit SHA. `govulncheck` blocks on Critical
   and High findings.

5. **Dockerfile build-context revisit.** Once `common-configs` has tagged
   releases (`v1.0.0+`), remove the `replace` from `services/auth/go.mod`
   entirely. Host and image then resolve identically. Until then the
   Option C approach (§ 3.3) stands.

6. **Telemetry collector + dashboards.** Add `otel/opentelemetry-collector`
   to the compose stack with an exporter to a dev backend (Jaeger or
   Grafana Tempo). Wire auth-service OTel traces and metrics through it.
   Expose `/metrics` from the service for Prometheus scrape.

Secondary items (not blocking dev-local, but flagged elsewhere):

- Refresh-token cleanup job ownership decision (in-service goroutine exists
  today; may move to a scheduled job). SECURITY § 11 item 6.
- `audit_log` table partitioning before production launch.
  SECURITY § 9.3 + DATA_MODEL open question 5.
- `must_change_password` column + enforcement so the bootstrap super-admin
  password can't be left weak indefinitely. SECURITY review log, Critical.
- Migration role separation in staging/prod — migrator should stop using
  the Postgres superuser once `lustia_migrator` exists (§ 6).

---

## 12. Payments — iPaymu sandbox

_Phase 6 (ADR 0015). Covers sandbox-only setup using ngrok as a public webhook
relay. Production credentials do not exist yet._

### 12.1 How the adapters work

| `PAYMENT_PROVIDER` | When to use | Binary tag required |
| --- | --- | --- |
| `dummy` (default) | Normal local development — no internet, no credentials | `-tags dev` or `-tags local` |
| `ipaymu` | End-to-end sandbox testing with the real iPaymu API + real QRIS QR | any tag (no build gate) |

The dummy adapter is the default in `run-local.sh` and will not change unless
you explicitly uncomment the iPaymu block.

### 12.2 One-time ngrok setup

iPaymu's sandbox must be able to reach your local service to deliver webhook
notifications. ngrok creates a public HTTPS tunnel to your localhost.

1. Install ngrok (one-time):
   ```
   winget install ngrok
   ```
   Or download from https://ngrok.com/download and add to PATH.

2. Authenticate (one-time — uses a free ngrok account):
   ```
   ngrok config add-authtoken <YOUR_NGROK_TOKEN>
   ```
   Get the token at https://dashboard.ngrok.com/get-started/your-authtoken.

3. Start the tunnel (every test session — keep this terminal open):
   ```
   ngrok http 8080
   ```
   Grab the `https://xxxx.ngrok-free.app` URL shown in the "Forwarding" line.
   The subdomain changes every run on free accounts unless you have a reserved
   domain.

### 12.3 Configuring the auth service for iPaymu sandbox

Open `lustia/services/auth/run-local.sh`. Near the top you will find:

```bash
export PAYMENT_PROVIDER=dummy

# --- iPaymu sandbox (Phase 6) ---
# export PAYMENT_PROVIDER=ipaymu
# export IPAYMU_BASE_URL=https://sandbox.ipaymu.com
# export IPAYMU_VA=0000005714983489
# export IPAYMU_API_KEY=SANDBOX429CAC48-22DE-43FE-9153-26DD0BB5671D
# export IPAYMU_NOTIFY_URL=https://YOUR-NGROK-SUBDOMAIN.ngrok-free.app/api/v1/public/payments/webhook
```

To activate:
1. Comment out `export PAYMENT_PROVIDER=dummy`.
2. Uncomment the five iPaymu lines.
3. Replace `YOUR-NGROK-SUBDOMAIN` with the live subdomain from ngrok output.
4. Source and run:
   ```
   source run-local.sh && go run -tags dev ./cmd/auth
   ```
   The startup log will print: `payment provider: ipaymu (APP_ENV=dev)`.

### 12.4 End-to-end test flow

1. Create a booking via `POST /api/v1/public/bookings` — the response
   contains `qr_string` and `qr_expires_at`.
2. The auth service calls iPaymu's sandbox API; iPaymu returns a real QRIS
   payload string. Render it in the Flutter app or any QR library.
3. To simulate payment without a real phone, use the iPaymu sandbox notifier:
   - Open https://sandbox.ipaymu.com/send-notify in your browser.
   - Fill in the `trx_id` from the CreateQR response log (check stdout — the
     adapter logs the iPaymu `TransactionId` at INFO level).
   - Set `notifyUrl` to your ngrok URL:
     `https://xxxx.ngrok-free.app/api/v1/public/payments/webhook`
   - Submit. iPaymu sends a POST to your ngrok relay → auth service.
4. Poll `GET /api/v1/public/bookings/:code/payment-status` to confirm the
   booking transitioned to `paid`.

### 12.5 Differences between adapters

| Behaviour | `dummy` | `ipaymu` (sandbox) |
| --- | --- | --- |
| QR string | Synthetic QRIS-format string | Real QRIS payload from iPaymu |
| QR image URL | Google Charts API URL | iPaymu-hosted PNG (may be empty) |
| Payment confirmation | Dev-only `POST /dummy-trigger` endpoint | iPaymu sandbox notifier or real scan |
| Signature verification | Skipped (no HMAC check) | HMAC-SHA256 (VA-keyed) enforced |
| `GetStatus` | Returns `paid` always | Returns `errIPaymuNotImplemented` (TODO Phase 7) |
| `ListSettlements` | Returns one synthetic item | Returns `errIPaymuNotImplemented` (TODO Phase 7) |

### 12.6 Troubleshooting

| Symptom | Likely cause | Fix |
| --- | --- | --- |
| Startup error: `IPAYMU_NOTIFY_URL is required` | `PAYMENT_PROVIDER=ipaymu` set but notify URL line still commented out | Uncomment `IPAYMU_NOTIFY_URL` in run-local.sh |
| `ipaymu: API error (http=401 status=401)` | Wrong VA or API key | Double-check sandbox credentials in iPaymu dashboard |
| Webhook arrives but signature mismatch | ngrok relays the body correctly; mismatch means the VA used to verify differs from the VA iPaymu signed with | Confirm `IPAYMU_VA` matches the account VA exactly |
| Booking stays `pending_payment` after notifier fires | Wrong `reference_id` in the notifier — it must match `provider_reference` in the CreateQR response | Check auth service logs for the `referenceId` value sent to iPaymu |
