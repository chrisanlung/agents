# ADR 0011 — Storage abstraction + therapist extended customer-facing profile

- **Status:** Accepted
- **Date:** 2026-04-25
- **Deciders:** owner (2026-04-25), orchestrator
- **Extends:** ADR 0009 (Phase 4 master operational data)
- **Reviewers before drafting:** go-expert (architecture), security-expert (upload hardening — blocking), devops-expert (dev/prod strategy)

---

## 1. Context

Tenants need to publish therapist profile data that customers will see during booking (Phase 5): a photo, height, weight, and body build category. Photo uploads require a storage layer. Dev runs on local disk; production must support Cloudflare R2 and/or Supabase Storage without code changes, only env reconfiguration.

Two coupled decisions in one ADR because the schema change (adding `photo_key` to `therapist`) is only meaningful with the storage abstraction that produces those keys. Splitting would strand one without the other.

## 2. Decisions

### 2.1 Storage abstraction

**A `Storage` interface with three methods, declared in `service/interfaces.go` (consumer-owned — matches `TherapistRepository`, `EmailSender`, etc.):**

```go
type Storage interface {
    Upload(ctx context.Context, key string, r io.Reader, mimeType string) error
    Delete(ctx context.Context, key string) error
    URL(ctx context.Context, key string) (string, error)
}
```

Rationale for each method signature:
- `Upload` takes a streaming `io.Reader` (no buffering the whole file in memory); `mimeType` is passed explicitly because the adapter may need it for Content-Type metadata on the remote object.
- `Delete` is best-effort — adapters may return an error, but callers log and continue. Deleting a missing key is not an error.
- **`URL(ctx, key) (string, error)`** — context-aware + error-returning. Critical: R2 and Supabase signed-URL generation can fail (network, credentials, quota). A pure `URL(key) string` signature would be a breaking change when those adapters are implemented. Locking the correct signature now costs nothing today; deferring it costs a cross-cutting refactor later.

**Package placement:** `internal/helper/storage/` — matches the repo's existing convention where external-service adapters live under `helper/` (`helper/hash.go` for Argon2id, `helper/jwt.go` for the RS256 signer, `helper/email.go` for SMTP). A top-level `internal/storage/` was considered but rejected: it would read as a peer-layer to `service/`/`repository/`, which it is not.

**Adapters:**
- `internal/helper/storage/local.go` — dev/on-prem. Writes to an absolute path on disk; auth-service serves `GET /uploads/*` via `gin.Router.StaticFS`.
- `internal/helper/storage/r2.go` — Cloudflare R2 (S3-compatible API) — stub with TODO body in Phase 4. Interface satisfied so the factory can compile.
- `internal/helper/storage/supabase.go` — Supabase Storage — stub with TODO body in Phase 4.
- `internal/helper/storage/factory.go` — reads `STORAGE_DRIVER` and returns the right adapter. Fail-fast on unknown driver.

**URL resolution happens at the controller boundary, not the service layer.** The service receives and stores `photo_key` only. `toTherapistResponse` in the controller calls `storage.URL(ctx, key)` during DTO mapping. This keeps the service layer free of infra dependencies (see OC-1 flag back to go-expert).

### 2.2 Config (env vars)

| Var | Required | Dev default | Notes |
|---|---|---|---|
| `STORAGE_DRIVER` | yes | — (fail-fast) | `local` / `r2` / `supabase`. No default: unset is a startup error. |
| `STORAGE_LOCAL_PATH` | when driver=local | `./storage-data/uploads` (resolved to absolute at startup) | Must be absolute after resolution. |
| `STORAGE_PUBLIC_BASE_URL` | when driver=local | `http://localhost:8080/uploads` | Used by `LocalStorage.URL()` to produce public URLs. |
| `STORAGE_R2_*` | when driver=r2 | — | `ACCOUNT_ID`, `ACCESS_KEY_ID`, `SECRET_ACCESS_KEY`, `BUCKET`. |
| `STORAGE_SUPABASE_*` | when driver=supabase | — | `URL`, `SERVICE_KEY`, `BUCKET`. |
| `UPLOAD_MAX_MB` | no | `5` | Hard limit per file. |
| `UPLOAD_TENANT_HOURLY_LIMIT` | no | `30` | Per-tenant rate cap. |

**Startup hard-fail conditions:**
1. `STORAGE_DRIVER` unset or not in the allowed set.
2. `STORAGE_LOCAL_PATH` is relative or does not exist / is not writable.
3. `STORAGE_DRIVER=local` **and** `KUBERNETES_SERVICE_HOST` env var is present (local storage with probable multi-replica deployment — data loss trap per devops review).

### 2.3 Therapist schema extensions (migration 000020)

Renames `therapist.photo_url` to `therapist.photo_key` (semantics change from full URL to opaque storage key). Adds three new columns.

| Column | Type | Constraints | Notes |
|---|---|---|---|
| `photo_key` (renamed from `photo_url`) | `TEXT` | NULL, length ≤ 512 | Opaque storage key produced by `Storage.Upload`. Never contains scheme/host. Existing `photo_url` values: **blank on first deploy** — see §2.3.1. |
| `height_cm` | `SMALLINT` | NOT NULL via DEFAULT after migration window, CHECK 100–250 | Customer-visible. |
| `weight_kg` | `SMALLINT` | NOT NULL via DEFAULT after migration window, CHECK 30–250 | Customer-visible. |
| `build` | `TEXT` | NOT NULL via DEFAULT, CHECK `IN ('langsing','sedang','atletis','tegap')` | Customer-visible category. |

**§2.3.1 Migration of existing `photo_url` data:** existing rows have `photo_url` values that are full URLs, not keys. On rename, they become stranded keys that `LocalStorage.URL()` cannot resolve. Two options:
- (a) Null out `photo_url` values during migration. Existing therapists lose their photo; admin must re-upload. Acceptable since dev seeds are the only production data today.
- (b) Add `photo_url` and `photo_key` as coexisting columns during a transition, deprecate `photo_url` in a later migration.

**Decision: (a).** Dev seeds are the only populated rows (acme-spa). Re-uploading from the admin UI is trivial during testing. Dual-column path introduces dual source of truth that go-expert flagged as a landmine.

**§2.3.2 New fields required for CREATE and UPDATE of therapist.** Validation enforced in the service layer:
- `height_cm` required, 100–250
- `weight_kg` required, 30–250
- `build` required, oneof the 4 values
- `photo_key` NOT set via JSON; only via the upload endpoint (§2.4).

Migration uses `DEFAULT 0` for `height_cm`/`weight_kg` and `DEFAULT 'sedang'` for `build` to fill existing rows, then immediately `ALTER` to `NOT NULL`. A banner on the therapist list UI will flag rows with default/placeholder data so admins can curate (tracked as Phase 4 polish, not blocking).

### 2.4 Upload endpoint

`POST /api/v1/tenant/therapists/:id/photo`
- Auth: `scope=tenant` + permission `therapist.update`
- Body: `multipart/form-data` with one field `photo` (file)
- Response: `200 OK` + the updated therapist DTO (includes resolved `photo_url`)

**Server-side pipeline (enforced in this exact order — each step MUST execute):**

1. **Body-size hard cap.** First statement of the handler:
   `c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxBytes+1024)` — cuts off oversized bodies before parsing. Without this, the 5MB limit can be bypassed per security-expert H-3.
2. **Explicit multipart parse** with 1 MB in-memory spill: `c.Request.ParseMultipartForm(1 << 20)`.
3. **MIME sniff** — read first 512 bytes, `http.DetectContentType`, reject anything outside `{image/jpeg, image/png, image/webp}`.
4. **Structural validation** — pass the body (wrapped in `io.MultiReader` to re-include the sniffed prefix) through `image.DecodeConfig` inside `io.LimitReader(1 << 20)`. Reject on error. This kills polyglot files and most decompression bombs at the header level without a full decode.
5. **Re-encode to strip EXIF/metadata** via `github.com/disintegration/imaging`. Output is the canonical-format byte stream written to storage. GPS coords / device info on uploads is PII per security-expert H-4.
6. **Per-tenant quota check.** In-memory sliding window: if this tenant has done ≥ `UPLOAD_TENANT_HOURLY_LIMIT` uploads in the last hour, return 429. Resets on service restart — acceptable tradeoff, no persistence in Phase 4.
7. **Storage write.** `Storage.Upload(ctx, newKey, stripped, mime)`. Key format: `therapists/{therapist_id}/{random-16-hex}.{ext}`.
8. **DB UPDATE in transaction.** Inside `tx.WithTx`, UPDATE `therapist.photo_key = newKey`. The old `photo_key` is captured in memory before the UPDATE.
9. **Fire-and-forget old-key delete** after tx commits. Done via goroutine with `context.Background()` (same pattern as audit appends). Failure logged, not fatal. **Critical invariant: never delete the old file before the new one is written AND the DB update has committed.**

This order is drawn from go-expert (atomic write-then-delete) and security-expert (H-1 through H-4) — see the review entries in `docs/SECURITY.md`.

### 2.5 Static file serving (dev only)

`router.StaticFS("/uploads", gin.Dir(absLocalPath, false))` — `false` = no directory listing. Middleware chain on this route group:
- `X-Content-Type-Options: nosniff`
- `Cache-Control: public, max-age=31536000, immutable`
- The existing CORS middleware (explicitly re-wrapped — Gin does not inherit it automatically across route groups per devops review DO-3)

Prod adapters (R2/Supabase) return CDN URLs directly; auth-service is out of the photo-serving path in prod.

### 2.6 Observability

Three metrics added (prepared now even if `/metrics` endpoint is not yet exposed):
- `storage_upload_total{driver, tenant_id, status}` — counter.
- `storage_upload_bytes_bucket{driver}` — histogram.
- `storage_delete_total{driver, tenant_id, status}` — counter.

Structured log fields on every upload event: `tenant_id`, `photo_key`, `size_bytes`, `driver`, `duration_ms`, `trace_id`. **Never log** file contents or presigned URLs with embedded credentials.

### 2.7 Non-goals (deferred)

- Generic `/uploads` endpoint reusable across resources — addon photos, service images, tenant logos in Phase 5+ get their own resource-specific endpoint (per go-expert Q4).
- Orphan file cleanup job — accepted risk in Phase 4 (rare, filesystem tolerance acceptable at dev scale).
- Content moderation / malware / CSAM scanning — Phase 5+ concern per security-expert Q11.
- Signed URLs with TTL — photos are publicly accessible customer-facing content; signed URLs add latency without benefit today. R2/Supabase adapters will generate signed URLs only when the bucket is configured private (Phase 5 decision).
- Quota persistence across restarts — in-memory sliding window is sufficient for Phase 4.

## 3. Review blockers resolved before this ADR was accepted

- security-expert: 4 Highs (MIME deeper validation, static serving hardening, multipart DoS, EXIF stripping) all resolved in §2.4 pipeline.
- go-expert: `URL` signature, package placement, interface consumer-ownership, atomic write order, landmines (io.Reader, DecodeConfig, ParseMultipartForm, os.Rename Windows) — resolved in §2.1 and §2.4.
- devops-expert: absolute local path, fail-fast on missing driver, multi-replica check, tenant quota, observability, named volume, DR notes — resolved in §2.2, §2.4.6, §2.6.

## 4. Open questions

1. **Windows `os.Rename` non-atomicity.** Dev team runs Windows 11. On Windows, rename-over-existing is not atomic. Accepted risk for dev — documented in §2.4 implementation notes. Prod runs Linux containers where rename is atomic; R2/Supabase adapters don't rename at all (PUT is atomic-by-key).
2. **`photo_key` storage key hash collision.** 16 hex chars = 64 bits of randomness; collision probability is negligible under birthday bound for realistic photo volumes (10^9 uploads = 10^-4 collision odds). Accepted.
3. **Pre-existing `photo_url` dev seed values.** Migration nulls them out (§2.3.1); admin re-uploads during testing. Phase 5 customer-facing: if any stale `photo_url` is live when this migration runs, notify the tenant. Acceptable today — only acme-spa seed rows affected.
