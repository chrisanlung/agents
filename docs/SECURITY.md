# Security — Lustia Auth Service

_Owned by `security-expert`. Every diff that touches auth, authorization, crypto, DB schema, or secrets must be reviewed against this document before merge. Blocking authority: Critical and High findings block "done"._

_Last updated: 2026-04-18 — Phase 2 initial design (security-expert)_

---

## 1. STRIDE Threat Model (Phase 2 scope: auth-service + DB)

Scope boundary: the trust boundary is at the HTTPS ingress. Everything inside is considered semi-trusted; the DB layer is trusted only after middleware has set tenant context. Super-admin flows are treated as a separate, higher-risk path.

| # | Category | Concrete Threat | Where | Impact | Mitigation | Status |
|---|---|---|---|---|---|---|
| S-1 | Spoofing | Attacker replays a stolen access token to impersonate a legitimate user | `POST /auth/*` all protected endpoints | Account takeover | Short-lived JWT (15 min), `kid`-keyed RS256 signature verified from JWKS, no shared secret across services | Design |
| S-2 | Spoofing | Login request specifies a `tenant_slug` belonging to a different tenant, causing auth to run against the wrong user table slice | `POST /auth/login` | Cross-tenant credential stuffing — attacker may find a user whose email+password is reused in another tenant | Tenant resolved by `tenant_slug` in body; unknown tenant returns generic `INVALID_CREDENTIALS` — same response body, same timing as wrong password. Never confirm whether a tenant slug is valid to an unauthenticated caller | Design |
| S-3 | Spoofing | Forged `kid` in JWT header directing verifier to use attacker-controlled key | JWKS endpoint + JWT middleware | Token forgery → full account takeover | Verifier fetches JWKS only from the configured internal URL; it does not follow a `kid` that is a URL or path. `kid` is matched against the cached JWKS key set; unknown `kid` fails immediately | Design |
| T-1 | Tampering | JWT `claims` altered after issue (e.g. `tenant_id`, `permissions` array) | JWT middleware in all services | Privilege escalation, cross-tenant data access | RS256 asymmetric signature — no downstream service holds the private key; signature verification detects any claim mutation | Design |
| T-2 | Tampering | Refresh token row in DB modified to extend `expires_at` or clear `revoked_at` | `refresh_token` table | Persistent session after intended revocation | `lustia_app` DB role has no UPDATE privilege on `revoked_at` outside the application's controlled write path; all revocation goes through the repository layer | Design |
| T-3 | Tampering | `app.current_tenant` session variable overwritten mid-request to read another tenant's data | PostgreSQL session variable | Cross-tenant data read | `SET LOCAL` scopes the variable to the transaction only; it is reset on commit/rollback. The application must set it once at the start of each transaction and never accept it from user input | Design |
| R-1 | Repudiation | Admin claims they did not create a user or assign a role; no evidence trail | `POST /admin/users`, `PUT /admin/users/:id/roles` | Compliance gap — GDPR audit trail | Every state-changing action writes a row to `audit_log` with `actor_user_id`, `action`, `resource_type`, `resource_id`, `meta` (before/after diff), and `created_at`. Audit log is append-only, no RLS, `lustia_app` cannot DELETE rows | Design |
| R-2 | Repudiation | User denies logging in from an unusual location | `POST /auth/login` → `refresh_token` row | Insufficient evidence for incident response | `refresh_token` stores `ip` (INET) and `user_agent` on issue. Login success is written to `audit_log` with IP, user agent, and timestamp. These fields have a 90-day retention minimum (see Section 9) | Design |
| I-1 | Information Disclosure | `password_hash` leaks in API response, log line, or error message | `"user"` GORM model, any log call in usecase/handler | Offline Argon2id cracking of all user passwords | GORM struct tag `json:"-"` on `PasswordHash`; log redaction rule (see Section 7); unit test asserting serialized User struct contains no `password_hash` key; usecase never returns the full domain `User` entity — returns a `UserDTO` that has no hash field | Design |
| I-2 | Information Disclosure | Password reset response reveals whether an email exists in a tenant | `POST /auth/forgot-password` | User enumeration — attacker maps all valid emails per tenant | Response is always HTTP 204 regardless of whether the email exists. Processing time is equalized by always running Argon2id or a constant-time dummy operation before responding | Design |
| I-3 | Information Disclosure | `refresh_token.ip` and `refresh_token.user_agent` (PII) exposed in an API response or log | `refresh_token` table, any log/response path that includes token rows | GDPR violation — PII exfiltration | These fields are never returned in any API response. They are written on token issue and readable only by the super_admin or in a security incident. Retained for 90 days post-revocation (see Section 9), then purged | Design |
| I-4 | Information Disclosure | JWT private key exposed via misconfiguration (e.g. mounted at a web-accessible path, logged at startup) | `lustia/services/auth/config/`, startup logger | Platform-wide token forgery | Private key loaded from a mounted file (not env var); startup log confirms key was loaded but logs only the `kid` (SHA-256 of public key), never the key material | Design |
| D-1 | Denial of Service | Credential stuffing / brute-force login against a single account | `POST /auth/login` | Account lockout or server CPU exhaustion from Argon2id hashing | Per-IP rate limit: 10 req/min. Per-email rate limit: 5 req/min. After 10 consecutive failed attempts within 15 minutes: set `user.locked_until = now() + 30 minutes`. Argon2id hashing is gated behind a pre-check that short-circuits on locked accounts without hashing | Design |
| D-2 | Denial of Service | Mass password reset requests cause mail provider rate-limit or CPU spike | `POST /auth/forgot-password` | Email provider exhaustion; CPU from token hashing | Per-IP: 3 req/hour. Per-email: 3 req/hour. Rate limit checked before any DB read or token generation | Design |
| D-3 | Denial of Service | Oversized JSON body sent to any auth endpoint exhausts memory | All `auth-service` endpoints | Service crash / OOM | Request body size capped at 1 MB in Gin middleware (see Section 6). Log + reject oversized bodies with HTTP 413 | Design |
| E-1 | Elevation of Privilege | `tenant_admin` assigns `super_admin` role to themselves or another user within their tenant | `PUT /admin/users/:id/roles` | Platform-level takeover | `super_admin` role assignment is blocked for any caller whose JWT `tenant_id` is non-null. The RBAC middleware checks `user.assign_role` permission, but the usecase additionally enforces: if target role is `super_admin`, caller must have `tenant_id IS NULL` in claims. This is a hard rule in the usecase, not just middleware | Design |
| E-2 | Elevation of Privilege | `branch_admin` reads or modifies users outside their assigned branch | `GET /admin/users/:id`, `PUT /admin/users/:id` | Cross-branch data access within a tenant | After RBAC middleware passes, the usecase verifies `target_user.branch_ids ∩ caller.branch_ids != ∅`. This is an explicit object-level check, not a generic filter. `code-reviewer` checklist item: every `/admin/users/:id` handler must have this check | Design |
| E-3 | Elevation of Privilege | Expired or revoked refresh token reused after rotation | `POST /auth/refresh` | Persistent access after logout or password change | Single-use refresh tokens. On use: mark `revoked_at = now()`, insert new token. If a previously-revoked token is presented, treat it as a theft indicator: revoke the entire token family (all tokens sharing the same root `replaced_by` chain) and force re-login | Design |

---

## 2. Authentication Design

### 2.1 Password Hashing

Algorithm: **Argon2id** via `github.com/alexedwards/argon2id`.

Parameters:

| Parameter | Value | Rationale |
|---|---|---|
| Time (`t`) | 3 iterations | Minimum recommended for interactive logins per RFC 9106 |
| Memory (`m`) | 65536 KiB (64 MiB) | Resists GPU/ASIC attacks; acceptable latency on 1-core |
| Parallelism (`p`) | 2 threads | Matches minimum recommended; scales to 2 cores |
| Salt length | 16 bytes | From `crypto/rand`; embedded in the encoded string |
| Key length | 32 bytes | 256-bit output |

The `argon2id.DefaultParams` from the library does not match these values. Explicitly construct:

```go
params := &argon2id.Params{
    Memory:      64 * 1024,
    Iterations:  3,
    Parallelism: 2,
    SaltLength:  16,
    KeyLength:   32,
}
```

**Rehash-on-login policy:** On every successful login, compare the stored hash's parameters against the current configured parameters. If they differ (parameters were tightened during a security upgrade), re-hash the provided plaintext password with the new parameters and update `user.password_hash` atomically within the same transaction. This upgrades hashes transparently without forcing password resets.

### 2.2 Account Lockout

- **Threshold:** 10 consecutive failed login attempts within a 15-minute sliding window.
- **Action:** Set `user.locked_until = now() + 30 minutes`. Do not reset `failed_login_count` on lockout — only on a successful login.
- **Behavior during lockout:** Return the generic `INVALID_CREDENTIALS` error (same body and timing as a wrong password — do not reveal the account is locked to an unauthenticated caller). Internally log the lockout event to `audit_log`.
- **Unlock:** `locked_until` expires automatically (the auth service checks `locked_until > now()` at login). Manual early unlock is an admin-only operation: `DELETE`/clear `locked_until` via `PUT /admin/users/:id/unlock`, requiring `user.update` permission and tenant scope verification.
- **Short-circuit behavior:** If `locked_until > now()`, the auth service returns `INVALID_CREDENTIALS` immediately without computing Argon2id. This prevents the lockout path from being used as a CPU DoS oracle.
- **Implementation note for `go-expert`:** The failed_login_count increment and lockout check must be atomic (single UPDATE with conditional logic or a SELECT FOR UPDATE on the user row) to prevent race conditions under concurrent login attempts.

### 2.3 Password Policy

| Rule | Value |
|---|---|
| Minimum length | 10 characters |
| Maximum length | 128 characters (prevent DoS via Argon2id on gigabyte inputs) |
| Complexity | At least 1 uppercase, 1 lowercase, 1 digit, 1 special character |
| Banned list | Top-1000 common passwords checked at registration and password change (static list embedded; Pwned Passwords API integration deferred to Phase 10) |
| Rotation | No mandatory periodic rotation (NIST SP 800-63B guidance: forced rotation increases weak password reuse) |
| History | No repeated-password check in Phase 2 (deferred; requires storing previous hashes) |

Validation is enforced in the HTTP handler DTO binding layer (`go-expert`: use `go-playground/validator/v10` custom validators for complexity and banned list).

### 2.4 JWT Access Tokens

**Algorithm:** RS256 (RSA + SHA-256). The auth-service holds the RSA private key exclusively. All other services verify signatures using the public key fetched from the JWKS endpoint. No downstream service ever needs — or should ever receive — the private key.

**Key size:** Minimum 3072-bit RSA key (recommended: 4096-bit for new deployments). Generate with:

```bash
openssl genrsa -out auth-private.pem 4096
openssl rsa -in auth-private.pem -pubout -out auth-public.pem
```

The `kid` (Key ID) for JWKS is the SHA-256 thumbprint of the public key in JWK format (per RFC 7638). This ensures `kid` is deterministic and rotatable.

**Claim set:**

| Claim | Type | Value |
|---|---|---|
| `iss` | string | `https://auth.lustia.internal` (configurable) |
| `sub` | string | `user.id` (UUID) |
| `aud` | string | `lustia` (platform audience; configurable) |
| `exp` | NumericDate | `iat + 900` (15 minutes) |
| `iat` | NumericDate | Unix timestamp at issue |
| `jti` | string | `crypto/rand` UUID per token (for future revocation blacklist) |
| `kid` | header | Key ID for JWKS lookup |
| `tenant_id` | string | `user.tenant_id` UUID, or `null` for super_admin |
| `roles` | []string | User's assigned role names |
| `permissions` | []string | Resolved permission codes for this user |
| `branches` | []string | Branch UUIDs assigned to this user (empty for tenant-wide roles) |

**Do not embed:** `password_hash`, `email`, `full_name`, or any PII beyond the minimum needed for routing.

**JWKS endpoint:** `GET /auth/.well-known/jwks.json` — public, unauthenticated, cacheable. Returns the current public key in JWK format.

**Downstream verification protocol:**
- Services fetch JWKS on startup and cache with a 5-minute TTL.
- On TTL expiry or unknown `kid`, re-fetch JWKS before rejecting the token (allows key rotation without downtime).
- Clock-skew allowance: 30 seconds (`golang-jwt/jwt/v5` `WithLeeway(30 * time.Second)`).
- Reject tokens with `alg: none` or any algorithm other than `RS256`. Use `jwt.WithValidMethods([]string{"RS256"})`.

### 2.5 Refresh Tokens

- **Format:** 32 bytes from `crypto/rand`, base64url-encoded (43 characters). Opaque to the client.
- **Storage:** SHA-256 hash of the raw token stored in `refresh_token.token_hash` (hex, exactly 64 chars). The raw token is returned to the client once on issue and never stored in plaintext.
- **Lifetime:** 14-day sliding window (each successful use resets the 14-day window). Absolute cap: 30 days from first issue (tracked via the root token's `issued_at` in the rotation chain). After 30 days, the user must re-authenticate regardless of activity.
- **Single-use rotation:** On `POST /auth/refresh`:
  1. Look up `token_hash` using `subtle.ConstantTimeCompare` (see Section 4).
  2. Verify `expires_at > now()` and `revoked_at IS NULL`.
  3. Atomically: set `revoked_at = now()` on the old row, insert a new row with `replaced_by` pointing from old → new.
  4. Return new raw token to client; client must discard the old one.
- **Theft detection:** If a token that has already been marked `revoked_at IS NOT NULL` is presented, this indicates token replay (possible theft). Action: revoke all tokens in the same chain (walk the `replaced_by` graph) and write a `security.refresh_token_replay` event to `audit_log`. The user must re-authenticate.
- **Revocation triggers:**
  - User-initiated logout (`POST /auth/logout`): revokes the presented token only.
  - Password change (`PUT /auth/me/password`): revokes all refresh tokens for the user.
  - Admin deactivation (`PUT /admin/users/:id` `is_active=false`): revokes all refresh tokens for the user.
  - Account lockout: does not auto-revoke existing tokens (tokens remain valid until their next use, which will fail the `is_active` check). This is acceptable for Phase 2; force-revocation on lockout can be added later.
- **Cleanup job:** A periodic job (flag for `go-expert` and `devops-expert`) should `DELETE FROM refresh_token WHERE expires_at < now() - interval '30 days'` to prevent unbounded table growth.

### 2.6 Password Reset

- **Token format:** 32 bytes from `crypto/rand`, base64url-encoded. SHA-256 hash stored in `password_reset.token_hash`.
- **Expiry:** 60 minutes from issue (`expires_at = now() + interval '1 hour'`).
- **Single-use:** On successful consumption, set `used_at = now()`. Any subsequent use of the same token is rejected (check `used_at IS NULL AND expires_at > now()`).
- **Constant-time lookup:** Do not query by `token_hash` as a literal WHERE clause if the DB can return the hash value in an error path. Use `WHERE token_hash = $1` with a parameterized query; compare the returned hash against the request value with `subtle.ConstantTimeCompare` in application code.
- **Rate limits:** Per-IP: 3 requests/hour. Per-email address: 3 requests/hour. Rate limit is checked before any DB read or token generation.
- **Response:** Always HTTP 204. Never confirm or deny that the email exists in the tenant. Processing time is equalized: if the email does not exist, perform a constant-time dummy comparison before responding (to prevent timing oracle on email existence).
- **No RLS on `password_reset` table — confirmed acceptable:** The lookup is by opaque token hash before tenant context is established. The hash is cryptographically random (32 bytes of entropy); enumeration is computationally infeasible. The additional controls (single-use, 60-minute expiry, per-IP and per-email rate limiting, constant-time comparison) are sufficient. No schema change needed. This is the accepted design.

### 2.7 MFA

MFA is **deferred to Phase 10** (before production customer-facing launch). The schema currently has no MFA columns on `"user"`. When Phase 10 is scoped, add:

- `mfa_enabled BOOLEAN NOT NULL DEFAULT false`
- `mfa_secret TEXT NULL` (encrypted TOTP seed or WebAuthn credential reference)
- `mfa_backup_codes TEXT[] NULL` (hashed backup codes)

Flag for `db-designer` when Phase 10 begins. For Phase 2, the absence of MFA is a documented and accepted risk. Super admin accounts should be given a strong password and the bootstrap email must be a monitored address (see Section 11, production-readiness checklist).

---

## 3. Authorization Design

### 3.1 Model

**RBAC with tenant + branch scoping.** Permissions are fine-grained string codes (`booking.create`, `user.assign_role`, etc.) seeded in the `permission` table. Roles aggregate permissions via `role_permission`. Users hold roles via `user_role`. Branch assignments via `user_branch` further scope branch-admin, finance, and therapist roles.

This is not full ABAC. For Phase 2, the combination of permission string + tenant scoping + branch scoping covers all required access control decisions. Migrate to ABAC or ReBAC only if dynamic attribute-based rules emerge that cannot be expressed as permission codes.

### 3.2 JWT Claims for Authorization

The JWT `permissions` claim is a pre-resolved, flat array of permission code strings. On login, the auth-service:

1. Loads the user's roles from `user_role`.
2. Resolves all permissions for those roles from `role_permission`.
3. Embeds the deduplicated permission list in the JWT `permissions` claim.
4. Embeds `tenant_id` (null for super_admin), `roles`, and `branches` (array of branch UUIDs).

This avoids a DB lookup on every protected request. The trade-off: permissions are stale until the token expires (15 minutes). For Phase 2 this is acceptable. If a permission must be revoked immediately (e.g. role removed from a compromised account), the remediation is to revoke all refresh tokens for that user (forcing re-login and claim re-issue) and wait up to 15 minutes for existing access tokens to expire.

### 3.3 Enforcement Layers (Three Layers — All Must Pass)

**Layer 1 — JWT Middleware** (`lustia/services/auth/internal/adapter/http/middleware/jwt.go`):
- Validates token signature, `exp`, `iss`, `aud`, algorithm.
- Extracts and stores claims in Gin context.
- On failure: HTTP 401, log event (never log the token itself).

**Layer 2 — RBAC Permission Middleware** (`middleware/rbac.go`):
- `RequirePermission("booking.create")` applied per route group.
- Checks `claims.Permissions` contains the required code.
- On failure: HTTP 403, write `authz.denied` event to `audit_log` with endpoint, user ID, required permission.

**Layer 3 — Object-Level Check in Usecase** (every usecase that touches a resource by ID):
- `GET /admin/users/:id` — usecase must verify `target_user.tenant_id == caller.tenant_id` (and, for branch-scoped roles, `target_user.branch_ids ∩ caller.branch_ids != ∅`).
- This check happens in the usecase layer, not the handler and not the middleware. The middleware cannot verify object ownership because it does not load the target resource.
- `code-reviewer` checklist item (mandatory at every PR): every endpoint with a resource ID parameter must have a corresponding object-level tenant/branch check in the usecase. "Passes RBAC middleware" is not sufficient.
- RLS at the DB layer is the final backstop: even if both Layer 2 and Layer 3 have bugs, an incorrectly scoped DB query returns an empty result set rather than leaking data.

### 3.4 Super Admin Authorization

**Decision: `lustia_super_admin` DB role with `BYPASSRLS` is NOT recommended for application traffic. The `__platform__` sentinel value is the accepted approach.**

Reasoning (for `go-expert` — this gates implementation):

The `lustia_super_admin` DB role with `BYPASSRLS` requires the connection pool to either (a) maintain a separate pool of connections authenticated as `lustia_super_admin`, or (b) use `SET ROLE lustia_super_admin` per-request. Option (a) doubles connection pool complexity. Option (b) requires careful cleanup on connection return to the pool — a missed `RESET ROLE` would allow a subsequent regular request to run as the super_admin DB role, which is a critical privilege escalation bug. The blast radius of getting that wrong is the entire DB with no RLS protection.

The `__platform__` sentinel approach uses a single `lustia_app` connection pool. The sentinel is validated in application code (a super_admin JWT has `tenant_id: null`; the middleware sets `app.current_tenant = '__platform__'`). The RLS policies for the relevant tables include the explicit sentinel branch. All connections are the same DB role. If the app forgets to set the variable, the safe-fail behavior (empty result) applies regardless of whether the caller is a super_admin or a regular user.

**The `lustia_migrator` role with `BYPASSRLS` is retained for DDL migrations only** and must never be used for application request handling.

See also: `docs/DECISIONS/0005-super-admin-rls-bypass.md`.

### 3.5 Multi-Tenant Login

**Tenant resolution on login:**

1. Client sends `POST /auth/login` with body: `{ "email": "...", "password": "...", "tenant_slug": "..." }`.
2. Auth-service looks up tenant by `slug` WHERE `status = 'active' AND deleted_at IS NULL`.
3. If tenant not found: return `INVALID_CREDENTIALS` (HTTP 401) — do not reveal the tenant slug was invalid.
4. Looks up user by `(tenant_id, email)` using the tenant found in step 2.
5. If user not found: return `INVALID_CREDENTIALS` — same body, same timing as wrong password (run a dummy Argon2id verify or `subtle.ConstantTimeCompare` against a fixed dummy hash to equalize timing).
6. Verifies password, checks `is_active`, checks `locked_until`.
7. On success: issue tokens, reset `failed_login_count = 0`, update `last_login_at`.

**Super admin login:** Uses a fixed `tenant_slug` value of `__platform__` (or omits `tenant_slug`; the auth-service checks `tenant_id IS NULL` when the slug is absent or `__platform__`). The super admin user row has `tenant_id IS NULL`.

**Timing equalization is mandatory:** steps 3, 4, and 5 must each take the same wall-clock time regardless of which branch was taken. Use `subtle.ConstantTimeCompare` with a dummy value when no real comparison is needed.

### 3.6 IDOR Prevention Checklist (for `code-reviewer`)

Every endpoint with a path parameter that is a resource ID must answer "yes" to all three questions:

1. Does the usecase load the resource and verify `resource.tenant_id == claims.TenantID`?
2. If the endpoint is branch-scoped: does it verify the resource's branch is in `claims.Branches`?
3. If the super_admin is the caller (`claims.TenantID == nil`): is cross-tenant access explicitly intended and logged?

Failure on any question is a High severity finding that blocks merge.

---

## 4. Cryptography Choices

| Concern | Algorithm / Library | Notes |
|---|---|---|
| Password hashing | Argon2id (`github.com/alexedwards/argon2id`) | Parameters: t=3, m=64MiB, p=2, salt=16B, key=32B |
| JWT signing | RS256 — RSA-PKCS1v15 + SHA-256 (`github.com/golang-jwt/jwt/v5`) | 4096-bit key recommended; 3072-bit minimum |
| JWT key ID | SHA-256 JWK thumbprint (RFC 7638) | Deterministic, rotation-safe `kid` |
| Refresh token raw value | 32 bytes from `crypto/rand`, base64url-encoded | Never stored; only the SHA-256 hash is persisted |
| Password reset token | 32 bytes from `crypto/rand`, base64url-encoded | Same as above |
| Token hash storage | `crypto/sha256` → hex encoding | Standard library only |
| Token comparison | `crypto/subtle.ConstantTimeCompare` | Mandatory for all hash/token equality checks |
| TLS | TLS 1.2 minimum, TLS 1.3 preferred | Enforced at ingress (nginx/caddy/load balancer); flag for `devops-expert` |
| HSTS | `max-age=31536000; includeSubDomains` | Add `preload` once stable in production |
| Randomness | `crypto/rand` exclusively | `math/rand` must not be used for any security-relevant value |
| Symmetric encryption (future) | AES-256-GCM or ChaCha20-Poly1305 | Not in Phase 2 scope; state this for future reference |

**`jti` claim:** Each access token gets a unique `jti` generated via `crypto/rand` UUID. Not actively checked against a blacklist in Phase 2 (stateless verification is the goal). When a revocation blacklist is needed (e.g. immediate token invalidation without waiting 15 minutes), the `jti` is the key to blacklist. Flag for `go-expert`: store the blacklist in Redis when that need arises.

**RSA private key generation for local dev:**

```bash
openssl genrsa -out lustia/services/auth/config/jwt-private.pem 4096
# Never commit this file. It is listed in .gitignore.
```

Key is loaded from a file path specified in config (`jwt.private_key_path`). It is never pasted into an environment variable (to avoid shell history and process list exposure).

---

## 5. Secret Storage

### 5.1 Local Development

- `.env` file at `lustia/.env` (gitignored; template at `lustia/.env.example` with placeholder values only).
- JWT private key: mounted as a PEM file at a path specified in `.env` (`JWT_PRIVATE_KEY_PATH`). Not pasted into the env var itself.
- DB credentials: `lustia_app` user password in `.env`. `lustia_migrator` password in `.env` (used only by the migrator container).
- `.env.example` must never contain real secrets — only commented-out variable names and format hints.

### 5.2 Staging and Production

Secret management is deferred. Flag for `devops-expert` with the following recommendation:

- Use a managed secret store: HashiCorp Vault (self-hosted) or AWS Secrets Manager / GCP Secret Manager / Azure Key Vault (cloud-native). Doppler is a viable middle ground for early-stage.
- JWT private key should be stored as a secret and mounted into the container as a file (not an env var) at deploy time.
- DB passwords should be rotated semi-annually or on any suspected compromise.
- Kubernetes deployments: use `ExternalSecrets` operator or `sealed-secrets`. Do not use `kubectl create secret` with passwords in shell history.
- CI/CD: use OIDC federation from GitHub Actions to the cloud provider. No long-lived access keys stored as GitHub secrets.

### 5.3 Never Log the Following

(Hard rules — enforced by code review and the redaction test described in Section 7)

- `password` (plaintext)
- `password_hash`
- JWT access token (full value)
- Refresh token (full value)
- Password reset token (full value)
- `Authorization` header contents
- DB password
- JWT private key (any portion)
- `client_secret` (future OAuth flows)

---

## 6. Web Platform Hardening

### 6.1 Security Headers

Applied to every HTTP response from the auth-service via Gin middleware (flag for `go-expert` to implement as a global middleware):

| Header | Value |
|---|---|
| `Strict-Transport-Security` | `max-age=31536000; includeSubDomains` (add `preload` once stable) |
| `X-Content-Type-Options` | `nosniff` |
| `Referrer-Policy` | `strict-origin-when-cross-origin` |
| `Permissions-Policy` | `camera=(), microphone=(), geolocation=(), interest-cohort=()` |
| `X-Frame-Options` | `DENY` |
| `Cache-Control` | `no-store` on all auth endpoints (`/auth/*`) |

CSP is deferred — this is a pure API service returning JSON; there is no HTML surface in Phase 2. Add a strict CSP if any HTML responses are ever introduced.

### 6.2 CORS

- Explicit allowlist from configuration (`cors.allowed_origins: ["https://app.lustia.com", "https://admin.lustia.com"]`).
- Never `Access-Control-Allow-Origin: *` with credentials.
- For Phase 2 (API-only, no browser client yet): CORS may be `*` without credentials on public endpoints only (`/auth/.well-known/jwks.json`). All other endpoints require explicit origin allowlist.
- Flag for `go-expert`: implement CORS as a Gin middleware that reads the `allowed_origins` list from config and reflects only trusted origins. Use `github.com/gin-contrib/cors`.

### 6.3 CSRF

Bearer-token authentication (`Authorization: Bearer <token>`) is not vulnerable to classical CSRF because browsers do not automatically attach Bearer tokens to cross-origin requests. No CSRF token is needed for Phase 2.

**Exception:** If a cookie-based refresh token flow is introduced in a later phase (e.g. `HttpOnly; Secure; SameSite=Strict` cookie for refresh token), CSRF protection (`SameSite=Strict` + double-submit cookie or signed CSRF token) must be added at that time. Flag this decision point for the phase that introduces cookie-based flows.

### 6.4 Rate Limiting

All limits are per the identifying dimension shown. Limits are enforced in application middleware using a sliding-window counter (Redis-backed for multi-instance deployments; in-memory with `golang.org/x/time/rate` for single-instance Phase 2 local dev). Flag for `go-expert`.

| Endpoint | Per-IP limit | Per-account/email limit | Notes |
|---|---|---|---|
| `POST /auth/login` | 10 req/min | 5 req/min by email | Account lockout at 10 consecutive failures (separate from rate limit) |
| `POST /auth/refresh` | 30 req/min | 20 req/min by user ID | Token theft detection handles abuse beyond rate limiting |
| `POST /auth/logout` | 20 req/min | — | |
| `POST /auth/forgot-password` | 3 req/hour | 3 req/hour by email | Checked before any DB read |
| `POST /auth/reset-password` | 3 req/hour | 3 req/hour by email | |
| `POST /admin/users` | 30 req/min | — | Admin endpoints: baseline app-wide limit |
| All other endpoints | 60 req/min per IP | — | Default baseline |

### 6.5 Request Body Size

Maximum JSON body size: **1 MB** for all auth endpoints. Enforced as the first Gin middleware before any handler is called:

```go
router.Use(func(c *gin.Context) {
    c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 1<<20)
    c.Next()
})
```

Return HTTP 413 on oversized bodies.

---

## 7. Logging Rules

### 7.1 Must Log

Write structured log entries (JSON, via `common-configs` logger) for all of the following. Each entry must include: `timestamp`, `trace_id`, `request_id`, `event_type`, and the fields noted.

| Event | Fields to include |
|---|---|
| Login success | `user_id`, `tenant_id`, IP hash (SHA-256 first 8 chars, not raw IP), `user_agent` hash |
| Login failure | `tenant_id`, email hash (SHA-256 first 8 chars, not raw email), failure reason code, IP hash |
| Account lockout | `user_id`, `tenant_id`, IP hash |
| Logout | `user_id`, `tenant_id` |
| Password change | `user_id`, `tenant_id` |
| Token refresh | `user_id` |
| Token replay detected | `user_id`, `tenant_id`, full event detail to `audit_log` |
| Password reset requested | email hash, IP hash |
| Password reset consumed | `user_id`, `tenant_id` |
| Authz denied | `user_id`, `tenant_id`, endpoint, required permission |
| Admin user create/update/deactivate | `actor_user_id`, `target_user_id`, `tenant_id`, action |
| Admin role assign/revoke | `actor_user_id`, `target_user_id`, role name |
| JWT key rotation (new `kid` active) | `kid`, timestamp |

**Note on IP and email in logs:** Log a short prefix of the SHA-256 hash (not the raw value) to allow correlation within an incident investigation without storing raw PII in structured logs. The raw IP and user agent are stored in `refresh_token` table (see Section 1, I-3 for retention rules).

### 7.2 Must Never Log

The following values must never appear in any log line, error message, HTTP response body, stack trace, or metric label:

- `password` (any plaintext password)
- `password_hash` (Argon2id encoded string)
- JWT access token (full value or payload)
- Refresh token (raw value)
- Password reset token (raw value)
- `Authorization` header value
- DB credentials (`lustia_app` or `lustia_migrator` passwords)
- JWT private key (PEM content or any derivative)
- Full email address in logs (use hashed prefix)
- Full IP address in application logs (raw IP is in `refresh_token` table only)

### 7.3 Redaction Test (for `qa-expert`)

The following test must exist in `lustia/services/auth/internal/` and pass in CI:

**Test name:** `TestNoSensitiveFieldsInLogs`

**Method:** In a login flow integration test, capture all log output (redirect logger to a buffer). Attempt login with a known password. Assert that the log buffer does not contain any of the following substrings: the plaintext password, the word `password_hash`, any substring matching the pattern of an Argon2id encoded string (`$argon2id$`), any substring matching a JWT structure (three base64 segments separated by `.` with 100+ characters).

This test is a regression gate. If a future refactor accidentally starts logging the hash (e.g. via GORM debug mode or a struct marshaled to log), the test fails the CI build.

**Additional test:** `TestUserDTODoesNotIncludePasswordHash` — marshal a `User` domain entity (with a non-empty `PasswordHash` field) to JSON and assert the result contains no `password_hash` key. This validates the `json:"-"` struct tag on the GORM model.

---

## 8. Supply Chain

| Control | Tool | Where | Status |
|---|---|---|---|
| Go version pinned | `go 1.24` directive in `go.mod` | `lustia/services/auth/go.mod` | Required |
| Dependency integrity | `go mod verify` | CI pipeline | Required |
| Vulnerability scan | `govulncheck ./...` | CI — runs on every push to main and on PRs | Flag for `devops-expert` |
| Container image scan | `trivy image` | CI — on Docker build | Flag for `devops-expert` |
| Dependency updates | Dependabot or Renovate | `.github/dependabot.yml` | Flag for `devops-expert` |
| GitHub Actions pins | All `uses:` lines must pin to full commit SHA | `.github/workflows/` | Flag for `devops-expert` |
| Workflow permissions | `permissions: contents: read` as default; escalate per-job only | `.github/workflows/` | Flag for `devops-expert` |
| SBOM | Syft / CycloneDX on release builds | CI release workflow | Deferred to Phase 3 |

`go mod verify` must pass in CI before any build artifact is produced. A failed `go mod verify` (checksum mismatch) is a build-blocking error, not a warning.

---

## 9. Operational and Runtime Security

### 9.1 Secret Rotation Cadence

| Secret | Scheduled rotation | On-compromise rotation |
|---|---|---|
| JWT RS256 private key | Annually | Immediately — see Section 9.2 |
| DB password (`lustia_app`) | Semi-annually | Immediately |
| DB password (`lustia_migrator`) | Semi-annually | Immediately |
| Bootstrap super admin password | On first login (enforced by `must_change_password` — see flag below) | Immediately |

✅ **Resolved 2026-04-21** — migration `000006_user_must_change_password` adds the column (NOT NULL, DEFAULT false) and flips the bootstrap super admin to `true`. The auth-service `middleware.PasswordChangeRequired` blocks every protected endpoint except `GET /auth/me`, `POST /auth/me/password`, and `POST /auth/logout`, returning HTTP 403 `PASSWORD_CHANGE_REQUIRED`. `UserRepository.UpdatePassword` clears the flag atomically with the new hash. Admin-created users (`service.UserService.CreateUser`) are seeded with `must_change_password = true`. Regression tests live in `internal/service/security_test.go`.

### 9.2 JWT Key Rotation Incident Response

If the JWT private key is suspected compromised:

1. Generate a new RSA key pair. Give it a new `kid`.
2. Deploy the new auth-service with both keys in the JWKS — the old public key and the new public key. The service begins signing with the new private key immediately.
3. Old access tokens (signed with the old `kid`) remain verifiable from JWKS for up to 15 minutes (their maximum remaining lifetime). After 15 minutes all old tokens have expired.
4. Remove the old public key from JWKS after 20 minutes (15-min token lifetime + 30-sec clock skew allowance + operational margin).
5. Refresh tokens are opaque and not JWT-signed. They are stored as hashes in the DB. A key compromise does not directly affect refresh tokens, but if the attacker used a forged JWT to issue refresh tokens, those must be identified and revoked. This requires reviewing `audit_log` for `auth.login_success` events issued during the compromise window and revoking all refresh tokens for those users.
6. Write a `security.jwt_key_rotated` event to `audit_log` with the old and new `kid`.

### 9.3 Audit Log Retention

| Phase | Retention | Storage |
|---|---|---|
| Hot (queryable) | 18 months | Primary PostgreSQL DB |
| Archive | 18 months–5 years | Cold storage (S3/GCS/Blob with lifecycle policy) |
| Deletion | After 5 years | Per applicable data retention law (adjust for GDPR/PDPA if Indonesian law requires shorter) |

Flag for `devops-expert`: implement `audit_log` table partitioning by `created_at` (monthly range partitions) before production launch. Without partitioning, purging old rows requires a full table scan. See `DATA_MODEL.md` open question 5.

### 9.4 Production Readiness Checklist (pre-staging-deploy)

The following items must be completed before any staging or production deployment. They are not Phase 2 code tasks — they are operational gates:

- [ ] Bootstrap super admin email (currently `admin@lustia.local` per migration 000007 — local-dev placeholder, works with Mailpit) replaced with a real, monitored mailbox address via `UPDATE "user" SET email = '...' WHERE id = 'a0000000-0000-0000-0000-000000000001'` before any staging deploy. *(Migration 000007 replaced the earlier unroutable `superadmin@lustia.internal` placeholder on 2026-04-21.)*
- [x] `must_change_password` column added to `"user"` (migration 000006); bootstrap user seeded with `must_change_password = true`; auth-service enforces password change on first login via `middleware.PasswordChangeRequired`. ✅ 2026-04-21.
- [ ] JWT private key generated with at least 3072-bit RSA, stored in a secret manager (not `.env`).
- [ ] TLS 1.2+ enforced at ingress; HSTS header enabled.
- [ ] Rate limiting backed by Redis (not in-memory) for multi-instance deployments.
- [ ] `govulncheck` and `trivy` passing with no Critical/High findings.
- [ ] `audit_log` table partitioned by month (or archival policy defined).
- [ ] Super admin bootstrap email changed and password set via the post-migration runbook (see ADR 0003).
- [ ] CORS `allowed_origins` list populated with real production origins (not `*`).
- [ ] Log pipeline configured to redact (or never receive) the fields listed in Section 7.2.
- [ ] Refresh token cleanup job scheduled.

---

## 10. Review Log

_Each security review of a diff or PR appends an entry here. Blocking findings (Critical, High) must be resolved before the orchestrator marks the work complete._

| Date | Change reviewed | Findings | Status |
| --- | --- | --- | --- |
| 2026-04-21 | `must_change_password` enforcement shipped | **[Critical — Resolved]** Migration 000006, `middleware.PasswordChangeRequired`, service wiring (Login / Refresh / CreateUser), atomic clear in `UpdatePassword`, regression tests in `security_test.go`. | resolved |
| 2026-04-18 | Phase 1 schema + Phase 2 auth-service design (initial security design review) | **[Critical — Resolved in follow-up 2026-04-21]** `must_change_password` enforcement was missing; see row above for the fix. **[High]** No object-level tenant/branch check specification existed prior to this document: `/admin/users/:id` and similar resource-by-ID endpoints had no explicit requirement to verify the target resource belongs to the caller's tenant. Without this check, any `tenant_admin` with `user.read` permission could read any user on the platform by guessing UUIDs (IDOR / CWE-639). Fix: Section 3.3 Layer 3 and Section 3.6 are now the explicit requirement. `code-reviewer` must treat absence of this check as a blocking finding. **[High]** `refresh_token.ip` and `user_agent` PII retention was undefined: no retention or access rule existed. These fields are accessible to anyone with DB read access. Fix: Section 1 (I-3) and Section 9.3 now mandate 90-day post-revocation retention, no API exposure, and access restricted to security incident review. **[Medium]** Bootstrap super admin email is non-routable (`superadmin@lustia.internal`): in production this address receives no mail, so password reset emails and security alerts go nowhere. Fix: production readiness checklist item added (Section 9.4). **[Medium]** JWT lifetime and revocation gap: a token issued to a user whose account is subsequently deactivated or whose role is removed remains valid for up to 15 minutes. This is an accepted design trade-off (stateless JWT) but must be documented. Fix: documented in Section 3.2; immediate remediation path (revoke all refresh tokens) documented in Section 2.5. **[Low]** No explicit request body size cap was specified in prior docs: without a cap, an attacker can send a multi-gigabyte body to the Argon2id password verification endpoint and exhaust memory. Fix: 1 MB cap specified in Section 6.5. **[Info]** MFA absence is a documented and accepted risk for Phase 2; super admin accounts are the highest-value target. Noted in Section 2.7. | in-review |

---

## 11. Open Questions for the Team

The following questions require a decision from the user or the relevant agent before the listed work can be completed. They are not blockers for Phase 2 code unless noted.

1. **MFA timing** — Phase 2 (optional enrollment now, while the auth-service is being built) or Phase 10 (before production launch)? Adding MFA to the auth-service is significantly cheaper now than retrofitting it in Phase 10. The DB schema has no MFA columns yet; adding them now is a one-migration change. Recommendation: add the columns as nullable in a Phase 2 migration, implement TOTP enrollment as optional, gate on a feature flag. Blocks: `db-designer` (schema), `go-expert` (TOTP flow). Needs decision from user.

2. **Password reset token distribution channel** — email only, or SMS/WhatsApp as well? If multi-channel, what is the sending service (SendGrid, Mailgun, Twilio, WhatsApp Business API)? This gates the `forgot-password` endpoint implementation. Flag for `go-expert`. Needs decision from user.

3. **JWKS rotation schedule and ownership** — who is responsible for generating a new RSA key pair and deploying it? Platform team (a single shared key for all tenants and all services) or per-tenant keys? Recommendation: single platform key, owned by the platform team, rotated annually with the incident procedure defined in Section 9.2. Per-tenant keys add significant operational complexity without a commensurate security benefit for Phase 2. Needs confirmation from user.

4. **Customer-facing login (Phase 5) — shared JWT format or separate issuer?** If customers (Phase 5) use the same auth-service and JWT format as staff, the `roles` and `permissions` claims need to express customer-scope permissions. If a separate issuer is used, downstream services must verify two distinct JWKS endpoints and `iss` values. Recommendation: same auth-service, same JWT format, same JWKS endpoint. Add a `user_type: staff | customer` claim to distinguish at the application layer. Needs decision from user before Phase 5 design begins.

5. **`audit_log` partitioning schedule** — monthly range partitions are recommended (see Section 9.3 and `DATA_MODEL.md` open question 5). Should this be included in Phase 2 migrations or deferred to a pre-production migration? Recommendation: add partitioning in a Phase 2 migration now while the table is empty. The cost of retrofitting range partitioning on a populated table is high. Needs decision from `db-designer` and user.

6. **Refresh token cleanup job ownership** — scheduled DELETE of expired/revoked tokens. Should this be a cron job in the auth-service itself (using a goroutine on a ticker), a separate Kubernetes CronJob, or a pg_cron job in the DB? Flag for `go-expert` and `devops-expert`. Needs decision before production.

7. **Rate limiting backend for multi-instance deployments** — in-memory rate limiting (acceptable for Phase 2 single-instance Docker Compose) does not work correctly when multiple auth-service instances are running. A Redis-backed rate limiter (e.g. `go-redis` + sliding-window Lua script) is required for production. Flag for `devops-expert`. Needs confirmation before any horizontal scaling.

---

_End of SECURITY.md — Phase 2 baseline._

---

## Phase 3 Security Review — 2026-04-23

_Reviewer: security-expert. Authority: may block Phase 3 sign-off on Critical or High findings. Existing Phase 1–2 threat model and controls are incorporated by reference (Sections 1–11 above). This section covers only the Phase 3 delta._

### Executive Summary

Phase 3 adds a public self-registration endpoint, a platform-admin approval workflow, tenant-scoped branch CRUD, and a forced password-change UI. The overall architecture is well-structured: RLS correctly backstops all tenant-scoped tables, the approval transaction is atomic, and the change-password flow properly revokes all sessions. The most consequential finding is a **hardcoded localhost URL inside the service layer** — in production the welcome email would send tenants to `http://localhost:3002/login`, which is a usability-breaking defect with a minor phishing facilitation angle. Several **information-disclosure** issues allow an attacker to enumerate whether an email or slug is already registered. The temporary password has **marginally acceptable entropy** but relies on `uuid.New()` being a CSPRNG-backed UUIDv4, which is confirmed by the `github.com/google/uuid` library — this is acceptable with the conditions noted below. No critical authentication bypass or tenant data-leak was identified. Two High findings must be resolved before sign-off.

**Finding count by severity:** High: 2 | Medium: 4 | Low: 3 | Info: 2

---

### Findings — High

---

#### H-1: Hardcoded `localhost` URL in welcome email — production tokens delivered to wrong host

**Severity:** High

**File:line:** `lustia/services/auth/internal/service/registration_service.go:86`

**Attack scenario:** The constant `tenantAdminLoginURL = "http://localhost:3002/login"` is the fallback value used when `NewRegistrationService` receives an empty `loginURL` string. In `main.go:130` the value is read from `TENANT_ADMIN_LOGIN_URL` env var, which is correct. However, the constant itself is also used as the package-level default inside the service constructor: `if loginURL == "" { loginURL = tenantAdminLoginURL }`. If the env var is accidentally left unset in staging or production (a realistic mistake — it is absent from the `deploy/.env.example` file's required-section), the welcome email body (`registration_service.go:439`) sends the new tenant admin to `http://localhost:3002/login`. The tenant cannot onboard. A secondary concern: if an attacker in a shared-hosting environment controlled port 3002 on that host, they could capture the temporary password when the user follows the link.

**Recommended fix:**

1. Add `TENANT_ADMIN_LOGIN_URL` to `deploy/.env.example` in the required (not optional) section with a clear production placeholder value (e.g. `TENANT_ADMIN_LOGIN_URL=https://app.YOUR_DOMAIN/login`).
2. In `main.go`, fail fast at startup if the value is missing in non-local environments:
   ```go
   tenantAdminLoginURL := envStr("TENANT_ADMIN_LOGIN_URL", "")
   if tenantAdminLoginURL == "" && appEnv != "local" {
       log.Fatal(ctx, "TENANT_ADMIN_LOGIN_URL must be set in non-local environments")
   }
   ```
3. Remove the in-service fallback constant entirely (or keep it only for unit tests). The service should not have a production-meaningful default.
4. Add `TENANT_ADMIN_LOGIN_URL` to the Section 9.4 production readiness checklist.

**Blocks Phase 3 sign-off:** Yes — without the env-var gate, a misconfigured production deploy silently breaks tenant onboarding and exposes temporary passwords to localhost.

---

#### H-2: Full contact email written to audit log — PII logging violation (CWE-532)

**Severity:** High

**File:line:** `lustia/services/auth/internal/service/registration_service.go:245`

**Attack scenario:** The audit entry written on registration submission includes `"email": in.ContactEmail` — the full, raw email address. Section 7.2 of this document explicitly prohibits full email addresses from structured logs (rule: "Full email address in logs — use hashed prefix"). The audit log table is append-only and accessible to anyone with DB read access. If the audit log is also streamed to an external log aggregator (Datadog, Loki, CloudWatch), the raw email propagates to that system's retention and search indexes. Under Indonesian PDPA and GDPR this constitutes unauthorized storage of personal data in a secondary system without the data subject's specific consent for that purpose. For a multi-tenant SaaS with potentially hundreds of business registrations per day, this creates a meaningful PII aggregation risk.

**Recommended fix:**

Replace the raw email with a hashed prefix (consistent with the login and password-reset audit pattern already established in Section 7.1):

```go
_ = s.audit.Append(ctx, AuditEntry{
    Action:       "registration.submitted",
    ResourceType: "tenant_registration",
    ResourceID:   reg.ID,
    Meta: map[string]interface{}{
        "slug":         resolvedSlug,
        "email_prefix": helper.SHA256Prefix(in.ContactEmail, 8), // first 8 hex chars
    },
})
```

Use whatever `helper.SHA256Prefix` (or equivalent) is already used for login audit entries.

**Blocks Phase 3 sign-off:** Yes — this is a direct violation of the project's own logging rules (Section 7.2) and applicable data-protection law.

---

### Findings — Medium

---

#### M-1: Email and slug existence enumeration via distinct error codes (CWE-204)

**Severity:** Medium

**File:line:** `lustia/services/auth/internal/service/registration_service.go:186-205` and frontend `web/tenant-admin/app/register/actions.ts:78-95`

**Attack scenario:** `SubmitRegistration` returns `ErrDuplicatePendingRegistration` when a pending registration with the same email exists, and `ErrTenantSlugTaken` when the requested slug is already live. The frontend maps these to distinct field-level error messages: `contact_email` shows "Email ini sudah memiliki permintaan pendaftaran" and `company_name` shows the slug-collision message. An attacker can enumerate: (a) whether a specific company email already has a pending application, and (b) whether a specific slug belongs to an active tenant — without being authenticated. For a B2B SaaS where company identities are semi-sensitive this leaks competitive intelligence (which businesses have registered) and enables targeted spear-phishing by confirming a prospective customer's status.

This is distinct from the password-reset enumeration protection already in place (Section 2.6) — the registration flow has no equivalent constant-response requirement.

**Recommended fix:**

Return a single generic error code (`CONFLICT`) for all registration uniqueness failures. The frontend already handles `CONFLICT` generically. Replace the two specific codes with one:

```go
// Both duplicate-email and slug-taken become the same opaque CONFLICT response.
return RegistrationOutput{}, constants.ErrRegistrationConflict
```

Map to HTTP 409 with code `CONFLICT` and message "A registration conflict occurred. Please review your details or contact support." Internally log the specific reason (email conflict vs slug conflict) to the audit log using a hashed email prefix (see H-2 fix).

**Blocks Phase 3 sign-off:** No — but should be fixed before public beta. Accepted risk window: internal/closed-beta only.

---

#### M-2: `ChangeTenantStatusRequest.Status` has no allowlist validation (CWE-20)

**Severity:** Medium

**File:line:** `lustia/services/auth/internal/controller/dto_request.go:129`

**Attack scenario:** The DTO field is `Status string \`json:"status" binding:"required"\`` — the `binding` tag has no `oneof` constraint. Any string value passes validation and reaches `TenantService.TransitionStatus`. The service delegates to `tenant.CanTransitionTo(newStatus)` which uses a whitelist of valid transitions, so an unexpected value returns `ErrInvalidStatusTransition`. The DB update is parameterized, so there is no injection risk. However, the missing validation means: (a) the DTO contract is ambiguous to API consumers, (b) if the model's state machine is ever refactored incorrectly, the service-level gate is the only check, and (c) unexpected values are processed further into the service before being rejected, creating a wider path for future logic bugs. Compare: `ChangeBranchStatusRequest.Status` at line 169 correctly uses `binding:"required,oneof=active inactive"`.

**Recommended fix:**

```go
type ChangeTenantStatusRequest struct {
    Status string `json:"status" binding:"required,oneof=active suspended deactivated"`
    Reason string `json:"reason" binding:"omitempty,max=1000"`
}
```

Align the allowed values with the `validTenantTransitions` map in `model/tenant.go`. This is defense-in-depth at the controller layer, consistent with all other status-change DTOs.

**Blocks Phase 3 sign-off:** No — the service-layer state machine provides adequate protection.

---

#### M-3: `RequestedSlug` accepts arbitrary characters — slug injection into tenant slug column (CWE-20)

**Severity:** Medium

**File:line:** `lustia/services/auth/internal/controller/dto_request.go:90`

**Attack scenario:** The DTO field `RequestedSlug string \`json:"requested_slug" binding:"omitempty,min=2,max=100"\`` imposes only length constraints. When the caller supplies an explicit slug (non-empty), the service skips the `slugify()` sanitizer and stores the raw value directly in `tenant_registration.requested_slug`, then copies it verbatim into `tenant.slug` at approval time. A caller could submit a slug containing characters outside `[a-z0-9-]` — for example Unicode characters, path traversal sequences (`../admin`), or leading/trailing hyphens. The slug is used as a URL segment and in the welcome email body, so an unexpected character set could cause routing ambiguity in the frontend, produce unexpected URL encoding behaviour in HTTP clients, or cause display issues in email templates.

**Recommended fix:**

Add a custom validator or a regexp constraint to `RequestedSlug`:
```go
RequestedSlug string `json:"requested_slug" binding:"omitempty,min=2,max=100,alphanum_dash"`
```
Where `alphanum_dash` is a custom `go-playground/validator/v10` validator that accepts `[a-z0-9-]+`, no leading/trailing hyphens, and no consecutive hyphens. Alternatively, always run `slugify()` on the caller-supplied value and compare it to the original — reject if they differ:
```go
if slugify(explicit) != explicit {
    return RegistrationOutput{}, constants.ErrInvalidInput
}
```

**Blocks Phase 3 sign-off:** No — the downstream tenant slug column is parameterized (no injection risk) and the value is always under admin review before approval. Fix before public launch.

---

#### M-4: In-memory rate limiter for registration is ineffective against distributed abuse and does not survive restarts (OWASP A04)

**Severity:** Medium

**File:line:** `lustia/services/auth/cmd/auth/main.go:129`, `registration_service.go:171-176`

**Attack scenario:** The registration rate limiter is instantiated as `helper.NewMemoryRateLimiter(3, float64(3)/3600)` — a token-bucket counter living in process memory. Three weaknesses follow: (1) On service restart (deploy, crash, OOM kill) all rate-limit state is reset; an attacker can trigger a restart (e.g. by sending a DoS spike elsewhere) and then fire 3 requests per IP again. (2) In a multi-instance deployment (horizontal scaling, canary deploy) each replica has independent state — effective limit per attacker becomes `3 × N` where N is instance count. (3) IP-based rate limiting is bypassed by rotating through residential proxies or a botnet with multiple IPs; each IP gets a fresh 3-request allowance. The registration flow creates DB rows, sends emails, and performs multiple slug-uniqueness queries per submission — the cost-per-request is non-trivial. This is acknowledged as an open question in Section 11.7 for the broader rate limiting concern, but the registration endpoint is higher-risk than most because it is fully public and unauthenticated.

**Recommended fix:**

This is a pre-existing accepted design choice (Section 11.7) for Phase 2 single-instance deployments. For Phase 3 (public endpoint exposure), two additional controls should be added regardless of Redis migration timing:
1. Add a global per-hour cap on total registrations (e.g. 50/hour platform-wide) enforced in the service, to bound DB/email cost even without per-IP accuracy.
2. Add `CAPTCHA` (hCaptcha or Cloudflare Turnstile) to the registration form before public launch — this is the correct defense against distributed IP rotation, and it keeps the Go service rate-limit as a backstop rather than the primary control.
3. When Redis is introduced (Section 11.7), use a sliding-window Lua script for the registration limiter.

**Blocks Phase 3 sign-off:** No for closed beta; must be addressed before public launch.

---

### Findings — Low

---

#### L-1: Temporary password entropy is sufficient but derives from UUIDv4 hex — document the dependency (CWE-331 risk awareness)

**Severity:** Low

**File:line:** `lustia/services/auth/internal/service/registration_service.go:93-107`

**Assessment:** `generateTemporaryPassword()` takes the first 16 hex characters of a UUIDv4 produced by `github.com/google/uuid`. The `google/uuid` library generates v4 UUIDs from `crypto/rand`, providing 128 bits of cryptographically random material before the UUID format fields are applied. A UUIDv4 has 6 bits of fixed format (version and variant nibbles), leaving 122 bits of random content across the full 32-hex-char string. The first 16 hex characters (64 bits) include the version nibble (4 bits fixed as `4`) and no variant bits, so the effective entropy of the 16-character prefix is approximately 60 bits. For a one-time-use temporary password delivered via email, 60 bits of entropy is acceptable — it is computationally infeasible to brute-force online, and the password is single-use by design (`must_change_password=true`). However, the implementation is fragile: if `uuid.New()` is ever swapped for a non-CSPRNG source (e.g. in a testing mock that leaks into production via a config flag), the entropy collapses without warning.

**Recommended fix:**

Replace with an explicit `crypto/rand`-backed generator that is obviously correct and does not depend on UUID version semantics:

```go
import "crypto/rand"
import "encoding/hex"

func generateTemporaryPassword() string {
    b := make([]byte, 10) // 80 bits of entropy → 20 hex chars
    if _, err := rand.Read(b); err != nil {
        panic("crypto/rand unavailable: " + err.Error())
    }
    return hex.EncodeToString(b)
}
```

80 bits → 20 hex characters exceeds the Section 2.3 password length minimum (10 chars) and provides comfortable margin above NIST SP 800-63B's 112-bit recommendation for out-of-band tokens. The `panic` on `rand.Read` failure is intentional — if the CSPRNG is unavailable the service should not continue issuing credentials.

**Blocks Phase 3 sign-off:** No — current entropy is adequate. Hardening recommended before production.

---

#### L-2: `ListTenantsQuery.Status` has no allowlist — arbitrary status values forwarded to repository (CWE-20)

**Severity:** Low

**File:line:** `lustia/services/auth/internal/controller/dto_request.go:122`

**Assessment:** `ListTenantsQuery.Status string \`form:"status" binding:"omitempty"\`` accepts any string. It is forwarded via `TenantFilter.Status` to the repository layer, which likely uses it in a `WHERE status = ?` parameterized query — so there is no injection risk. However, passing arbitrary strings to the DB filter returns an empty result set silently rather than a validation error, which is a minor correctness issue and could mask misconfigured clients. Compare: `ListRegistrationsQuery.Status` at line 111 correctly uses `oneof=pending approved rejected all`.

**Recommended fix:**
```go
Status string `form:"status" binding:"omitempty,oneof=active suspended deactivated all"`
```

---

#### L-3: Timezone and country-code fields accept unvalidated strings (CWE-20)

**Severity:** Low

**File:line:** `lustia/services/auth/internal/controller/dto_request.go:146,161`

**Assessment:** `Country` is validated `len=2` which is correct for ISO 3166-1 alpha-2 but does not restrict to known country codes — a caller can send `"XX"` or `"00"`. `Timezone` is validated `max=100` only — arbitrary strings pass through and are stored in the `branch.timezone` JSONB column (actually a text column per the branch model). If the timezone string is later used to call `time.LoadLocation()` in Go or `AT TIME ZONE` in PostgreSQL, an invalid value will cause a runtime error. This is not exploitable for data exfiltration but creates a reliability risk.

**Recommended fix:**

For `Country`: add a custom validator that checks against the ISO 3166-1 alpha-2 list (a static ~250-entry slice).

For `Timezone`: validate against the IANA tz database. Go's `time.LoadLocation(tz)` returns an error for unknown names — call it at validation time:
```go
if _, err := time.LoadLocation(req.Timezone); err != nil {
    return ErrInvalidInput
}
```
This check should live in the service layer (on `CreateBranchInput.Timezone`) rather than the DTO validator, since IANA tz names are runtime data, not a static enum.

---

### Findings — Info

---

#### I-1: JWT `must_change_password` claim read unverified in Edge middleware — accepted design with adequate backstop

**Severity:** Info (not a finding — design assessment)

**File:line:** `lustia/web/tenant-admin/middleware.ts:45-51`

**Assessment:** The Next.js Edge middleware decodes the JWT payload without verifying the RS256 signature (this is explicitly documented in the middleware comment). It reads `claims.must_change_password` to decide whether to redirect to the forced password-change screen. An attacker who controls an `access_token` cookie could craft a JWT with `must_change_password: false` and bypass the frontend redirect gate. However: (1) The backend's `PasswordChangeRequired` middleware enforces the same gate at every protected API endpoint server-side — a crafted token will still be rejected by the server for any state-changing operation. (2) The frontend gate is a UX mechanism, not a security gate. (3) `GET /auth/me` (the first call made by every dashboard Server Component) validates the JWT server-side and returns the current `must_change_password` flag from the DB; the frontend would immediately re-trigger the redirect. The design matches the documented model in Phase 2 (Section 9.1). No fix required.

---

#### I-2: CSRF on server actions — Next.js 15 built-in protection is sufficient

**Severity:** Info (not a finding — design assessment)

**File:line:** `lustia/web/tenant-admin/app/register/actions.ts`, `web/tenant-admin/app/pengaturan/ubah-kata-sandi/actions.ts`

**Assessment:** Next.js 15 Server Actions use the `Same-Origin` enforcement in the `Origin` header check built into the framework since Next.js 14.1 (CVE-2024-34351 addressed the prior bypass). Additionally, server actions are dispatched via a `POST` with a `Next-Action` header that browsers cannot set cross-origin without CORS preflight approval. The tenant-admin app's cookie should be `SameSite=Lax` (or `Strict`) — verify this is set on the `access_token` and `refresh_token` cookies issued by the backend. If `SameSite=None` is used without explicit CSRF tokens, revisit. No fix required under current configuration; flag for `devops-expert` to confirm cookie attributes in the production cookie-setting path.

---

### RLS Bypass Scope Assessment (super_admin deactivation cascade)

The `TransitionStatus` deactivation cascade (`tenant_service.go:103-125`) calls `memberships.SuspendAllForTenant` and `tokens.RevokeAllForTenantUsers`. Both repository methods are parameterized `WHERE tenant_id = ?` queries executed through the standard `lustia_app` connection pool. The `app.current_tenant` session variable is set to `__platform__` for super_admin requests by the JWT middleware (per Section 3.4), which allows the `user` RLS policy's sentinel branch to pass. The `membership` table does not have RLS (it is a platform-level table per the migration comment), so the suspension UPDATE runs without needing the sentinel. The `refresh_token` RLS policy uses a join through `user` — the `RevokeAllForTenantUsers` update targets `WHERE tenant_id = ?` directly on the `refresh_token` table, which requires a `tenant_id` column on that table; the query at `refresh_token_repository.go:75` uses this column. No cross-tenant blast radius was identified: the cascade is correctly scoped to the single target `tenantID` in all three write paths.

---

### Phase 3 Sign-off Recommendation

**Decision: BLOCK — two conditions must be resolved before Phase 3 is considered done.**

| # | Finding | Required action |
|---|---|---|
| H-1 | Hardcoded localhost URL in welcome email | Add `TENANT_ADMIN_LOGIN_URL` to `.env.example` required section; add startup fail-fast guard in `main.go`; add to Section 9.4 checklist |
| H-2 | Full contact email in audit log | Replace `"email": in.ContactEmail` with `"email_prefix": helper.SHA256Prefix(...)` in `registration_service.go:245` |

All Medium findings (M-1 through M-4) are accepted for the current closed-beta window but must be tracked and resolved before public launch. Low and Info findings are hardening recommendations with no sign-off impact.

The Phase 1–2 controls (Argon2id, RS256 JWT, RLS tenant isolation, refresh token rotation, `must_change_password` enforcement) remain sound and were not regressed by Phase 3. The new branch and tenant management endpoints correctly enforce tenant scoping at both the service layer and the RLS layer.

| Date | Change reviewed | Findings | Status |
|---|---|---|---|
| 2026-04-23 | Phase 3 delivery — public registration, approval workflow, branch CRUD, change-password UI | **[High — OPEN]** H-1: Hardcoded localhost URL in welcome email (`registration_service.go:86`). **[High — OPEN]** H-2: Full contact email written to audit log (`registration_service.go:245`, Section 7.2 violation). **[Medium]** M-1: Email/slug enumeration via distinct error codes. **[Medium]** M-2: `ChangeTenantStatusRequest.Status` missing `oneof` validation. **[Medium]** M-3: `RequestedSlug` accepts non-slugified characters. **[Medium]** M-4: In-memory rate limiter ineffective against distributed abuse. **[Low]** L-1: Temporary password derives entropy from UUID hex — recommend explicit `crypto/rand`. **[Low]** L-2: `ListTenantsQuery.Status` missing allowlist. **[Low]** L-3: Timezone/country fields accept unvalidated strings. | **BLOCKED — H-1 and H-2 must be resolved** |
| 2026-04-23 | Phase 4 delivery — therapist CRUD, service catalog, therapist↔service mapping, per-therapist availability | See ## Phase 4 Security Review — 2026-04-23 below. | **BLOCKED — H-1 must be resolved** |

---

## Phase 4 Security Review — 2026-04-23

_Reviewer: security-expert. Code read: therapist_svc.go, availability_service.go, mapping_service.go, catalog_service.go, all four controllers, all four repositories, dto_request.go, migration 000013, migration 000014, migration 000004 (RLS), middleware.ts, and master/*.tsx frontend pages. Review covers ADR 0009, API contract §11, and the 14 new tenant-scoped endpoints._

---

### Executive Summary

Phase 4 introduces 14 new endpoints and 4 DB tables. The authorization architecture is generally sound: cross-branch isolation is correctly enforced via the `IsAdmin / containsBranch` pattern on every write path, IDOR protection follows the established `tenant_id` equality check pattern from Phase 1–3, and RLS policies are in place for three of the four new tables. Mass assignment is not possible — DTOs do not expose `tenant_id`, `branch_id`, `id`, or soft-delete fields. The frontend renders all free-form fields through standard React JSX (no `dangerouslySetInnerHTML`), so the `service.category` XSS vector is not present.

**One High finding blocks sign-off.** The `therapist_service` table is missing both an UPDATE RLS policy and the UPDATE privilege grant. The reconcile operation in `PUT /therapists/:id/services` issues UPDATE statements that therefore run without row-level tenant isolation — RLS defence-in-depth is absent for this mutation path. The service-layer tenant check is the only guard.

**Finding count:** 1 High, 3 Medium, 3 Low, 1 Info.

---

### STRIDE Extension for Phase 4

| # | Category | Concrete Threat | Where | Impact | Mitigation | Status |
|---|---|---|---|---|---|---|
| S-4 | Spoofing | `branch_admin` guesses a therapist UUID belonging to another branch and calls PATCH/DELETE | All therapist write endpoints | Cross-branch profile mutation | `containsBranch(callerBranches, t.BranchID)` enforced after tenant equality check | Implemented |
| T-4 | Tampering | Caller submits `service_ids` from another tenant in `PUT /therapists/:id/services` | `mapping_service.go:56` | Cross-tenant mapping injection | `FindByIDs(ctx, callerTenantID, ids)` validates all IDs against caller's tenant | Implemented |
| T-5 | Tampering | `PUT /therapists/:id/availability` replaces availability for an unowned therapist | `availability_service.go:63` | Unauthorized schedule override | Tenant + `containsBranch` check before any write | Implemented — note H-1 mitigates via service layer; DB layer gap in therapist_service, not availability |
| I-5 | Information Disclosure | IDOR: attacker reads availability of another tenant's therapist | `GET /therapists/:id/availability` | Confidential schedule leak | `t.TenantID != callerTenantID` check before returning rows | Implemented |
| D-4 | Denial of Service | Rapid `PUT /therapists/:id/availability` calls create DELETE+INSERT storm | `therapist_availability_repository.go:43` | Table churn, booking engine degradation in Phase 5 | No per-endpoint rate limit exists | Open — see M-1 |
| E-4 | Elevation of Privilege | `therapist`-role user overwrites a colleague's availability at same branch | `availability_service.go:71` | Unauthorized schedule modification by peer | No `user_id == callerUserID` check; only branch membership checked | Open — see M-2 |

---

### Findings — High

---

#### H-1: `therapist_service` RLS missing UPDATE policy and UPDATE privilege grant — reconcile writes bypass row-level security (CWE-284)

**Severity:** High

**File:line:** `lustia/migrations/000004_rls_policies.up.sql:317` (grant: `SELECT, INSERT, DELETE` only), `lustia/services/auth/internal/repository/therapist_service_repository.go:88–93` and `118–124`

**Attack scenario:** Migration 000004 section 9 creates `tenant_isolation` (FOR SELECT via subquery join) and `tenant_isolation_write` (FOR INSERT via subquery join) policies on `therapist_service`, then grants `SELECT, INSERT, DELETE` to `lustia_app`. No `FOR UPDATE` policy and no UPDATE grant exists.

`ReconcileForTherapist` at lines 84–127 issues `db.Model(&model.TherapistService{}).Where(...).Updates(...)` calls to re-activate deactivated mappings (lines 88–93) and to deactivate removed mappings (lines 119–124). GORM translates both into `UPDATE therapist_service SET ... WHERE therapist_id = ? AND service_id = ?` statements.

Consequence 1 (current, likely breaking): `lustia_app` has no UPDATE privilege on `therapist_service`. These UPDATE statements will fail at runtime with PostgreSQL error `ERROR: permission denied for table therapist_service`, meaning `PUT /therapists/:id/services` is currently broken whenever it needs to change `is_active` on an existing row. New mappings (INSERT path) work fine; mutations to existing mappings do not.

Consequence 2 (forward): Once the UPDATE privilege is added, without a corresponding RLS UPDATE policy, the UPDATE statements run without row-level tenant isolation. PostgreSQL's default for a table with RLS enabled and no matching PERMISSIVE UPDATE policy is to deny all rows — so the updates would return 0 rows affected. This would silently succeed from the application's perspective (no error thrown) but make no database change, creating a silent correctness bug where re-activation and deactivation of existing mappings appear to succeed but do not persist.

The service-layer tenant check (`t.TenantID != in.CallerTenantID`) is the correct first-line control. However, removing the DB-layer defence-in-depth violates the project's established security pattern (every other mutable table has a full SELECT/INSERT/UPDATE RLS policy set) and creates a blast radius if the service layer is ever bypassed (maintenance scripts, future repositories, direct DB access by `lustia_app`).

**Recommended fix:**

Add to a new migration (000015) or to the down/up pair of a 000013 amendment:

```sql
-- 1. Add UPDATE RLS policy on therapist_service (mirrors therapist_availability pattern)
CREATE POLICY tenant_isolation_update ON therapist_service
    AS PERMISSIVE FOR UPDATE
    USING (
        EXISTS (
            SELECT 1 FROM therapist t
            WHERE t.id = therapist_service.therapist_id
              AND t.tenant_id::text = current_setting('app.current_tenant', true)
        )
    );

-- 2. Add UPDATE privilege
GRANT SELECT, INSERT, UPDATE, DELETE ON therapist_service TO lustia_app;

-- 3. While here, add DELETE RLS policy (no current code path exercises DELETE,
--    but the grant exists and should be guarded):
CREATE POLICY tenant_isolation_delete ON therapist_service
    AS PERMISSIVE FOR DELETE
    USING (
        EXISTS (
            SELECT 1 FROM therapist t
            WHERE t.id = therapist_service.therapist_id
              AND t.tenant_id::text = current_setting('app.current_tenant', true)
        )
    );
```

**Blocks Phase 4 sign-off:** Yes. `PUT /therapists/:id/services` is broken for any call that mutates an existing mapping row, and the missing RLS policy removes defence-in-depth for this table's mutation path.

---

### Findings — Medium

---

#### M-1: No per-therapist rate limit on `PUT /therapists/:id/availability` — unbounded DELETE+INSERT storm (OWASP A04)

**Severity:** Medium

**File:line:** `lustia/services/auth/internal/repository/therapist_availability_repository.go:43–63`, `lustia/services/auth/internal/service/availability_service.go:104`

**Attack scenario:** `ReplaceAllForTherapist` executes `DELETE WHERE therapist_id = ?` then bulk-inserts up to 21 rows (3 windows × 7 days) inside a transaction on every PUT call. An authenticated `branch_admin` or `tenant_admin` can call this endpoint in a tight loop. Each call issues a full table-replace on the therapist's partition of `therapist_availability`. The global middleware imposes no per-endpoint rate limit. The service-layer validation (overlap, 5-minute boundary, max-3-per-day) runs before the DB write, but it does not prevent rapid repeated calls with valid payloads. The table has a GiST exclusion constraint (migration 000003 comment) which adds constraint-evaluation overhead on every INSERT batch. The Phase 5 booking engine will query this table on hot paths; rapid-churn inflates dead tuple count.

**Recommended fix:** Apply a per-therapist-per-caller rate limit of 10 PUT calls per minute at the controller layer using the existing in-memory limiter pattern:

```go
// In AvailabilityController.handleReplace after claims extraction:
key := fmt.Sprintf("avail_replace:%s:%s", claims.TenantID, id)
if !rateLimiter.Allow(key, 10, time.Minute) {
    helper.RespondError(c, http.StatusTooManyRequests, constants.CodeRateLimited, "too many availability updates")
    return
}
```

**Blocks Phase 4 sign-off:** No. Requires authentication; blast radius limited to one tenant's availability data. Must be fixed before Phase 5 booking integration.

---

#### M-2: `therapist`-role user can overwrite a colleague's availability at the same branch — missing `user_id == callerUserID` check (OWASP A01)

**Severity:** Medium

**File:line:** `lustia/services/auth/internal/service/availability_service.go:71–75`, `lustia/migrations/000013_phase4_master_data.up.sql:212–215`

**Attack scenario:** Migration 000013 grants `availability.create` and `availability.update` to the `therapist` role. `HasTenantAdminRole` returns false for a `therapist`-role user, triggering the `containsBranch` guard. The guard checks `callerBranches` (from `claims.Branches`, populated from `user_branch` table assignments) against `t.BranchID`. If a `therapist`-role user is assigned to Branch X via a `user_branch` row, they can call `PUT /therapists/:id/availability` for any therapist at Branch X — including colleagues — because there is no check that `t.UserID == in.CallerUserID`. The `therapist.user_id` link is the only mechanism to associate a therapist record with a portal user, but the availability service does not consult it.

**Recommended fix:**

Option A (recommended for Phase 4): Remove `availability.create` and `availability.update` from the `therapist` role in the migration, deferring self-service availability to Phase 5 when the portal for therapists is in scope.

Option B (if self-service is needed now): In `AvailabilitySvc.Replace`, after the `containsBranch` check, add:

```go
// If caller is not admin and not branch_admin, enforce own-record-only rule.
if !in.IsAdmin && !hasBranchAdminRole(in.CallerRoles) {
    if t.UserID == nil || *t.UserID != in.CallerUserID {
        return AvailabilityOutput{}, constants.ErrCrossBranchForbidden
    }
}
```

This requires passing `CallerRoles` through `ReplaceAvailabilityInput`.

**Blocks Phase 4 sign-off:** No. Exploiting this requires an admin to deliberately assign a `therapist`-role user to a branch and that user to have portal access. Must be resolved before Phase 5 ops-portal delivery.

---

#### M-3: Cursor subquery in `FindByTenant` lacks explicit tenant constraint — implicit RLS dependency in subquery (CWE-89 latent)

**Severity:** Medium

**File:line:** `lustia/services/auth/internal/repository/therapist_repository.go:77`, `lustia/services/auth/internal/repository/service_repository.go:72`

**Attack scenario:** Both list endpoints use a cursor subquery of the form:

```go
q = q.Where(
    "(full_name, created_at) > (SELECT full_name, created_at FROM therapist WHERE id = ?)",
    filter.Cursor,
)
```

The `filter.Cursor` value is an opaque UUID from a previous response, passed as a bind variable (no injection risk). However, the subquery `FROM therapist WHERE id = ?` has no `AND tenant_id = ?` constraint. In PostgreSQL, a subquery within the same connection respects RLS on the referenced table — so the subquery returns NULL for a UUID belonging to another tenant (the `app.current_tenant` session variable filters the row away). GORM then generates `> NULL` which evaluates to false, returning an empty page rather than leaking data.

The finding is that the correctness of cursor isolation depends on implicit RLS behaviour in a subquery rather than an explicit WHERE clause. If the subquery ever runs under a connection where `app.current_tenant` is not set or is set incorrectly (e.g. a future code path that initializes the query before setting tenant context), it would return rows from any tenant. Additionally, a caller who provides a cross-tenant UUID as a cursor receives an empty page rather than a validation error, which constitutes a minor UUID existence oracle: an empty page when a syntactically-valid UUID is provided as cursor vs. a normal query result is a timing/behaviour side-channel.

**Recommended fix:**

Add `AND tenant_id = ?` to both subqueries, passing `tenantID` as the second bind variable. This makes the intent explicit and removes the implicit RLS dependency:

```go
// therapist_repository.go:77
q = q.Where(
    "(full_name, created_at) > (SELECT full_name, created_at FROM therapist WHERE id = ? AND tenant_id = ?)",
    filter.Cursor, tenantID,
)
// service_repository.go:72
q = q.Where(
    "(name, created_at) > (SELECT name, created_at FROM service WHERE id = ? AND tenant_id = ?)",
    filter.Cursor, tenantID,
)
```

**Blocks Phase 4 sign-off:** No. RLS provides the actual isolation. Fix before Phase 5.

---

### Findings — Low

---

#### L-1: `service.created` audit event logs `service.Name` — free-form business data in audit metadata (CWE-532 risk precedent)

**Severity:** Low

**File:line:** `lustia/services/auth/internal/service/catalog_service.go:64`

**Assessment:** The `service.created` event writes `Meta: map[string]interface{}{"name": sv.Name}`. `sv.Name` is a free-form string up to 200 chars — not PII in the narrow sense (service catalog name, not personal data). The therapist audit events are clean: `therapist.created` logs `branch_id` only; `therapist.updated` logs nothing; `therapist.deleted` logs `cascade_mappings_deactivated: true`. No PII appears in Phase 4 audit events.

The risk is the pattern itself: writing free-form business fields into `meta` establishes a template that future developers may follow, accidentally logging `therapist.full_name`, `therapist.email`, or `therapist.phone`. Phase 3 H-2 was exactly this class of issue.

**Recommended fix:** Replace `"name": sv.Name` with `"service_id": sv.ID` (already present as `ResourceID`). Update the SECURITY.md §7 logging policy to explicitly state that audit `meta` must not contain personal names, email addresses, phone numbers, or free-form description fields.

---

#### L-2: `AvailabilityWindowRequest.DOW` binding tag missing `required` — zero-value ambiguity silently creates Sunday windows (CWE-20)

**Severity:** Low

**File:line:** `lustia/services/auth/internal/controller/dto_request.go:264`

**Assessment:** The struct tag is `binding:"min=0,max=6"` without `required`. In Go, an absent `int` JSON field defaults to `0`. `DOW=0` is valid (Sunday). A client that omits `dow` silently gets a Sunday window instead of a validation error. Not exploitable, but creates surprising behaviour for misconfigured clients.

**Recommended fix:** Change the field to `*int` and use `binding:"required,min=0,max=6"`. Update the service layer to dereference the pointer. This makes an absent `dow` a binding error rather than a silent zero.

---

#### L-3: Dev seed migration 000014 lacks a runtime database-name guard against accidental non-dev application (OWASP A05)

**Severity:** Low

**File:line:** `lustia/migrations/000014_seed_dev_master_data.up.sql:1–10`

**Assessment:** The migration header clearly states "DEV-ONLY — DO NOT APPLY IN STAGING OR PRODUCTION" and provides the `migrate ... up 13` stopping instruction. No credentials or privilege grants are seeded. Both therapist rows have `user_id = NULL`. All rows carry `"source": "dev_seed"` in metadata for detectability. The risk is purely operational: an operator who accidentally runs the full migration chain in staging inserts two therapists and six availability rows into real tenant data. The `ON CONFLICT (id) DO NOTHING` clauses make re-runs safe.

**Recommended fix:** Add a runtime guard at the top of the migration:

```sql
DO $$
BEGIN
    IF current_database() NOT LIKE '%dev%' AND current_database() NOT LIKE '%local%' THEN
        RAISE EXCEPTION 'Migration 000014 is dev-only. Database "%" does not match dev/local pattern.', current_database();
    END IF;
END $$;
```

As an alternative, enforce via the migration runner in the CI/CD pipeline configuration (staging/prod jobs run `migrate up 13` only).

---

### Findings — Info

---

#### I-1: `therapist_service` also missing DELETE RLS policy — no current code path exercises it, but the DELETE grant exists (design gap)

**Severity:** Info

**File:line:** `lustia/migrations/000004_rls_policies.up.sql:317`

**Assessment:** The DELETE grant on `therapist_service` exists but no application code issues a hard DELETE on this table (ADR 0009 Q4 preserves rows for audit; `DeactivateAllForTherapist` uses UPDATE). The DELETE grant is present for potential future use (cleanup jobs). Without a DELETE RLS policy, any future code path that hard-deletes rows would run without tenant isolation. The recommended fix in H-1 includes a DELETE RLS policy alongside the UPDATE fix — handle both in the same migration.

---

### RLS Coverage Assessment — Phase 4 New Tables

| Table | SELECT | INSERT | UPDATE | DELETE | lustia_app grants | Assessment |
|---|---|---|---|---|---|---|
| `service` | tenant_isolation | tenant_isolation_write | tenant_isolation_update | (grant present) | SELECT, INSERT, UPDATE, DELETE | Complete |
| `therapist` | tenant_isolation | tenant_isolation_write | tenant_isolation_update | Withheld (ADR 0009 Q4) | SELECT, INSERT, UPDATE, DELETE | Complete. Hard DELETE blocked at DB level as intended. |
| `therapist_availability` | tenant_isolation | tenant_isolation_write | tenant_isolation_update | (grant present) | SELECT, INSERT, UPDATE, DELETE | Complete |
| `therapist_service` | tenant_isolation (join subquery) | tenant_isolation_write (join subquery) | **MISSING** | **MISSING** | SELECT, INSERT, DELETE (UPDATE missing) | **Gap — H-1** |

---

### Cross-Cutting Check Results

**1. Cross-branch authorization:** Confirmed on all five write paths. `Create`, `Update`, `ChangeStatus`, `SoftDelete` in `therapist_svc.go` all call `containsBranch` after tenant equality check. `Reconcile` in `mapping_service.go:49` does the same. `Replace` in `availability_service.go:71` does the same. LIST correctly restricts `branch_admin` to `filter.BranchIDs = in.CallerBranches` (`therapist_repository.go:68`). GET (read-only) requires only tenant equality — correct per §11.4.3.

**2. Mass assignment:** Clean. `UpdateTherapistRequest` and `UpdateServiceRequest` (dto_request.go) exclude `tenant_id`, `branch_id`, `id`, `deleted_at`, `created_at`, `updated_at`. The `therapist_repository.go:Update` method uses an explicit column map (lines 103–112). No GORM `Save(struct)` that could promote zero-value fields.

**3. IDOR via path param:** Clean. `AvailabilitySvc.Get` (line 41), `MappingService.Reconcile` (line 45), `TherapistSvc.Get` (line 141), and both `GetMappings` (line 307) all load the resource first and check `TenantID == callerTenantID` before returning data. Wrong-tenant returns 404, not the data.

**4. Soft-delete bypass:** Clean. Both `FindByID` implementations (`WHERE id = ? AND deleted_at IS NULL`) and `FindByTenant` queries filter deleted rows. `UpdateStatus` filters `deleted_at IS NULL` — PATCH on a soft-deleted row returns `ErrTherapistNotFound`. Mapping endpoint does not apply `deleted_at` to `therapist_service` (correct — that table uses `is_active` as its only state flag, no `deleted_at` column).

**5. `therapist.user_id` link integrity:** The service layer in `therapist_svc.go` (Create at line 79, Update at line 196) stores `in.UserID` directly without performing the same-tenant membership check specified in ADR 0009 §2.3 and API contract §11.4.1. RLS on the `user` table would prevent reading a cross-tenant user row but does not prevent storing a cross-tenant UUID as `user_id`. The field is informational only in Phase 4 (not used for authentication). Risk is a dangling reference. Must be fixed in Phase 5 before `user_id` is used for portal login: add `userRepo.FindByIDInTenant(ctx, tenantID, userID)` check in the service layer when `UserID != nil`.

**6. Availability window abuse:** Clean. All four service-layer rules are enforced in `validateAvailabilityWindows`: end > start, 5-minute boundaries, max 3 per day, no overlaps. DB exclusion constraint provides backup. Zero-length and reverse windows rejected.

**7. `service.category` XSS (frontend):** Clean. All Phase 4 master pages render data through React's standard JSX text interpolation and `<Badge>` components — no `dangerouslySetInnerHTML`. `services/page.tsx:213` renders `{s.category}` inside `<Badge>`. `therapists/page.tsx` renders therapist names and branch names as text nodes. `service-form.tsx` uses `react-hook-form` controlled inputs. No XSS vector found.

**8. Mapping soft-update race:** Acceptable for Phase 4. Two concurrent PUT calls for the same therapist could both see an existing mapping as inactive and both issue re-activate UPDATEs — the second is a no-op. No corruption. The pre-validation `FindByIDs` call is outside the transaction, creating a narrow race where a service could be soft-deleted between validation and reconcile. Blast radius: one inactive mapping visible to no booking query. Note for Phase 5: hold a SELECT FOR UPDATE on the therapist row at the start of the reconcile transaction.

**9. Availability index adequacy for Phase 5:** Migration 000013 adds `therapist_tenant_branch_active_idx` on `therapist(tenant_id, branch_id)`. Phase 4 availability queries use `therapist_id` (covered by existing indexes from migration 000003). The Phase 5 booking-engine pattern (available therapists at branch X for service Y on day D) will need a composite index on `therapist_availability(branch_id, day_of_week)` — flag for the Phase 5 migration author.

**10. Seed migration 000014:** No credentials, no privilege grants, both therapists have `user_id = NULL`. Fixed UUIDs with `ON CONFLICT DO NOTHING`. See L-3.

**11. RLS coverage:** Detailed table above. Three of four new tables complete; `therapist_service` missing UPDATE policy and grant — H-1.

**12. Audit PII:** No PII in any Phase 4 audit event. `therapist.created` → `branch_id` only. `therapist.updated` → no meta. `therapist.deleted` → `cascade_mappings_deactivated: true`. `therapist_service.updated` → `service_ids` (UUIDs). `therapist_availability.replaced` → `window_count` (int). Phase 3 H-2 class violation not repeated.

---

### Phase 4 Sign-off Recommendation

**Decision: BLOCK — H-1 must be resolved before Phase 4 is done.**

| # | Finding | Required action |
|---|---|---|
| H-1 | `therapist_service` missing UPDATE RLS policy + UPDATE grant | Write a new migration (000015 or 000013 amendment) that adds `CREATE POLICY tenant_isolation_update ON therapist_service FOR UPDATE USING (EXISTS (SELECT 1 FROM therapist t WHERE t.id = therapist_service.therapist_id AND t.tenant_id::text = current_setting(...)))` and `GRANT SELECT, INSERT, UPDATE, DELETE ON therapist_service TO lustia_app`. Add DELETE RLS policy in the same migration. |

Medium findings M-1 through M-3 (rate limit on availability PUT, therapist self-service cross-colleague risk, cursor subquery implicit RLS) are accepted for the current closed-beta window and must be tracked to Phase 5. M-2 is the most important of the three and should be resolved before the ops-portal (Phase 5) ships. Low findings are hardening recommendations.

Phase 1–3 controls (Argon2id, RS256 JWT, RLS tenant isolation, refresh token rotation, `must_change_password`) remain sound and were not regressed by Phase 4. The 13 of 14 new endpoints have correct tenant isolation, cross-branch enforcement, and IDOR protection. The `therapist_service` UPDATE gap in H-1 is the only control failure in this delivery.
