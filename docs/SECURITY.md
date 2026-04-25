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
| 2026-04-24 | Phase 4 therapist photo upload — design review (pre-implementation) | **[High — BLOCKING x4]** (1) MIME sniff via `http.DetectContentType` alone is insufficient: polyglot files pass the 512-byte sniff and can cause XSS if served from the same origin. Fix: add `image.DecodeConfig` structural validation (header-only, no full decode) after MIME sniff; reject on decode error. (3) Static file serving must use `http.FileServer` via `router.StaticFS(http.Dir(...))` only; custom `os.Open(basePath + userPath)` handler would introduce directory traversal. `X-Content-Type-Options: nosniff` and `Cache-Control: public, max-age=31536000, immutable` headers must be added to all `/uploads/*` responses. (4) Multipart endpoint lacks `MaxBytesReader` before `ParseMultipartForm`; a multi-gigabyte upload body bypasses the 5 MB file check and can exhaust server memory. Fix: `http.MaxBytesReader` on the upload handler before any form parsing. (7) EXIF metadata (GPS, device model) is PII under GDPR/PDPA; JPEG/PNG/WebP uploads will contain it. Fix: strip EXIF on upload before writing to storage (re-encode via `github.com/disintegration/imaging`). **[High — Gated, not blocking code now]** (9) R2/Supabase production adapters must not be enabled until bucket policy (no public ListObjects, least-priv service credential), CORS (explicit origin allowlist on bucket), and credentials-in-secret-manager controls are in place. Flag for `devops-expert`. **[Medium]** (5) Use `image.DecodeConfig` (not `image.Decode`) with `LimitReader` to prevent pixel-flood allocation. (6) Cross-tenant photo visibility accepted: UUID v4 keys are 122-bit entropy; public customer-facing photos have no confidentiality requirement. Accepted risk; add Phase 5 draft-state caveat to threat model. (8) Delete-old-then-write pattern risks data loss on partial failure; reverse to write-new-first, async best-effort delete. (10) Same-origin photo serving is acceptable for Phase 4 (tenant admin only); Phase 5 must use a dedicated CDN subdomain before customer-facing launch — added to production readiness checklist. **[Low]** (2) Add key regex guard in `Storage.Delete`/`URL` to reject keys containing `..` or leading `/`. (11) Content moderation (PhotoDNA/Vision API) deferred to Phase 5 pre-launch gate — acceptable for Phase 4 (admin-only uploads, not yet customer-surfaced). | BLOCKED — 4 High findings must be resolved by `go-expert` before implementation proceeds |

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

---

## Phase 4 Add-ons Review — 2026-04-24 (ADR 0010, migration 000018)

> **SUPERSEDED** — This review covers the per-service `service_addon` design that was pivoted in the same session before any code was merged or deployed. The design described here no longer exists in the codebase. Retained as an audit trail only.
> See [Phase 4 Add-ons Review (ADR 0010, tenant-wide redesign) — 2026-04-24](#phase-4-add-ons-review-adr-0010-tenant-wide-redesign--2026-04-24) for the current review.

_Reviewer: security-expert. Code read: `000018_phase4_service_addons.up.sql`, `model/service_addon.go`, `repository/service_addon_repository.go`, `service/service_addon_service.go`, `controller/service_addon_controller.go`, `route/route.go`, `controller/dto_request.go` (add-on DTOs), `controller/dto_response.go`, `service/catalog_service.go` (`Get` + `ListForServiceDetail`), `web/tenant-admin/app/master/services/[id]/addon-actions.ts`, `lib/api.ts`, `lib/types.ts`, `docs/API_CONTRACT.md` §12, `docs/DECISIONS/0010-per-service-addons.md`._

---

### Executive Summary

ADR 0010 adds one new table (`service_addon`), six new endpoints nested under `/tenant/services/:id/addons`, one extension to `GET /services/:id`, and a set of Next.js server actions. The overall implementation is solid: cross-tenant isolation is enforced correctly at the service layer via `resolveAddon` and parent-service tenant checks, RLS policies follow the project-established pattern, hard DELETE is blocked at DB level (grant: SELECT/INSERT/UPDATE only), soft-delete cannot be reversed, and the reorder bulk operation is atomic. No SQL injection vector was found — all queries are parameterized. Frontend server actions correctly derive tenant context from the session cookie only.

**One Medium finding and one Low finding require `go-expert` attention before sign-off.** No Critical or High findings were identified, so this review does not block completion.

**Finding count:** 0 Critical, 0 High, 1 Medium, 2 Low, 2 Info.

---

### STRIDE Extension for ADR 0010

| # | Category | Concrete Threat | Where | Impact | Mitigation | Status |
|---|---|---|---|---|---|---|
| T-6 | Tampering | `PUT /reorder` body contains add-on IDs belonging to a different service or tenant | `service_addon_service.go:247–254` | Cross-tenant sort_order pollution | Service layer loops `FindByID` on every item and asserts `a.ServiceID == in.ServiceID && a.TenantID == in.CallerTenantID` before the bulk UPDATE | Implemented |
| T-7 | Tampering | PATCH/DELETE supply an `addonID` from a different service or tenant | `service_addon_service.go:300–308` (`resolveAddon`) | Cross-tenant mutation | `resolveAddon` checks `a.ServiceID != serviceID \|\| a.TenantID != callerTenantID` — returns 404 on mismatch | Implemented |
| I-6 | Information Disclosure | `GET /services/:id/addons` returns add-ons without parent service ownership check in the DB query | `service_addon_repository.go:54` | Cross-tenant data leak | Service-layer `List` pre-checks parent service's `TenantID` before calling `FindByServiceID`; RLS backstop further constrains the query to caller's tenant | Implemented |
| D-5 | Denial of Service | Reorder body with 201+ items; or unbounded list if `max=200` binding is bypassed | `dto_request.go:319` (`binding:"max=200"`) | Service CPU / DB overload | `max=200` binding tag caps array size; list endpoint has no array size (but add-on count per service is expected small; ADR 0010 §5 Q2 notes no hard limit) | See M-1 |
| E-5 | Elevation of Privilege | `branch_admin` mutates add-ons on a service that their branch does not manage | Permission check uses `service.update`; add-ons are tenant-scoped | Write ops on another branch's service catalogue | Reuses `service.update` permission — which is already a tenant-admin-level permission; `branch_admin` role does not hold it in the current seed | Noted below — see ADR open question |

---

### Cross-Cutting Check Results

**1. RLS correctness.** The `service_addon_tenant_select` policy is `USING (tenant_id::text = current_setting('app.current_tenant', true))`. The second argument `true` to `current_setting` means "missing-ok" — returns empty string if the variable is unset. An empty string cannot equal any valid UUID text representation, so the policy fails closed. No `__platform__` branch is needed or present (no NULL-tenant rows in `service_addon`, consistent with comment in migration header). SELECT, INSERT, and UPDATE policies all use the same predicate. No DELETE policy is needed — the grant is absent. Pattern is correct and consistent with ADR 0009 tables. RLS coverage table:

| Table | SELECT | INSERT | UPDATE | DELETE | lustia_app grants | Assessment |
|---|---|---|---|---|---|---|
| `service_addon` | tenant equality | tenant equality | tenant equality (USING + WITH CHECK) | N/A | SELECT, INSERT, UPDATE | Complete. Hard DELETE blocked at DB level as intended. |

**2. Cross-tenant isolation at service layer.** Every mutation path (`Create`, `Update`, `ChangeStatus`, `SoftDelete`) calls `resolveAddon(ctx, addonID, serviceID, callerTenantID)` which asserts both `a.ServiceID == serviceID` and `a.TenantID == callerTenantID` before returning the row. Mismatch returns `ErrServiceAddonNotFound` (404), which reveals nothing about the target row's existence. `Create` and `List` independently fetch the parent service and assert `parent.TenantID == in.CallerTenantID`. `Reorder` separately validates the parent service and then checks every item ID. Isolation is correct and multi-layered.

**3. `ListForServiceDetail` path (embedded in `GET /services/:id`).** `CatalogService.Get` (`catalog_service.go:145`) first verifies `sv.TenantID != callerTenantID` on the parent service, then calls `s.addons.FindByServiceID(ctx, serviceID, ServiceAddonFilter{})` directly (bypassing `ServiceAddonService`). The query runs under the same RLS-protected DB connection (same `app.current_tenant` session variable), so the RLS backstop is in effect. The parent-service ownership check provides application-layer isolation before the DB query. No cross-tenant add-on leak is possible via this path.

**4. Soft-delete / undelete.**  `SoftDelete` sets `deleted_at = now()` and `is_active = false`. There is no "undelete" or "restore" endpoint. `PATCH` (`Update`, `ChangeStatus`) both filter `AND deleted_at IS NULL` in their respective WHERE clauses — a soft-deleted row returns `ErrServiceAddonNotFound`, not a mutable row. A caller cannot accidentally or deliberately resurface a soft-deleted add-on via any existing API surface.

**5. Duplicate-name race safety.** The partial unique index `service_addon_service_name_uidx ON (service_id, name) WHERE deleted_at IS NULL` is the final serialization point. Concurrent inserts with the same `(service_id, name)` will race, but the one that loses receives `ERROR 23505` which is caught by `translateAddonDBError` and mapped to `ErrDuplicateAddonName` (HTTP 409). No silent data corruption. Correct.

**6. Numeric bounds.** `price_idr` is `int64` in Go and `BIGINT` in SQL, with a `CHECK (price_idr >= 0)` constraint backed by a service-layer check (`priceIDR < 0` returns `ErrInvalidInput`). Maximum representable value is 9,223,372,036,854,775,807 — overflow from JSON parsing would require a value outside JSON number precision before GORM receives it. The `go-playground/validator/v10` binding tag `binding:"min=0"` on `CreateServiceAddonRequest.PriceIDR` and `binding:"omitempty,min=0"` on the update DTO enforce the non-negative constraint at the controller layer. No overflow risk in practice for IDR denomination.

**7. Input validation and XSS.** Name and description length constraints exist at three layers: DB `CHECK`, service `validateAddonFields`, and DTO binding tags. All are consistent. The `ServiceAddonResponse` struct exposes `name` and `description` as plain JSON strings — no HTML markup is stored or reflected. The frontend renders these through standard React text interpolation. No XSS vector identified.

**8. Authorization — branch_admin scope.** The ADR 0010 §4.5 open question is: "branch_admin at branch A cannot mutate add-ons on a service owned by tenant X if they aren't a member — should be blocked." Current implementation: add-on write ops require `service.update`. Looking at the existing permission seed in migration 000013, `service.update` is granted to `tenant_admin` role but NOT to `branch_admin`. This means `branch_admin` is implicitly blocked from all add-on mutations today. This is the intended behaviour per ADR 0010 §4.2 ("add-ons are tenant-scoped"). The absence of an explicit RBAC check specific to add-ons is acceptable because the inherited `service.update` permission already achieves the desired outcome. See M-2 for a hardening note.

**9. Logging.** Audit events: `service_addon.created` logs `{"service_id": uuid}` only. `service_addon.updated` logs `{"service_id": uuid}`. `service_addon.activated/deactivated` logs `{"is_active": bool}`. `service_addon.deleted` logs `{"service_id": uuid}`. `service_addon.reordered` logs `{"service_id": uuid, "count": int}`. None of these contain `name`, `description`, or `price_idr`. No PII. Phase 3 H-2 class violation is not repeated.

**10. Server actions tenant isolation.** `addon-actions.ts` functions accept `serviceId` as a parameter (from the URL slug — a UUID already validated by the route) and forward it to `apiFetch`. `apiFetch` in `lib/api.ts` calls `getAccessToken()` from `lib/session` and attaches the `Authorization: Bearer` header — tenant context is derived entirely from the signed JWT, not from any request-body field. No `tenant_id` is passed in any request body. Correct.

**11. Reorder atomicity.** `Reorder` validates all item IDs (N+1 queries — see M-1) then calls `tx.WithTx(ctx, ...)` wrapping `BulkUpdateSortOrder`. The bulk UPDATE uses a CASE expression with parameterized bind variables — no string interpolation of user-supplied IDs in SQL outside the `?` placeholders. On transaction failure all sort_order changes are rolled back. Correct.

---

### Findings — Medium

---

#### M-1: Reorder validation is N+1 `FindByID` calls outside a transaction — TOCTOU window + potential DoS

**Severity:** Medium

**File:line:** `lustia/services/auth/internal/service/service_addon_service.go:247–254`

**Attack scenario:** `Reorder` validates ownership by calling `s.addons.FindByID(ctx, item.ID)` in a loop — one DB round trip per item. With `max=200` items allowed by the binding tag, this is up to 200 sequential `SELECT` statements before the transaction begins. Two issues:

1. **TOCTOU window:** An add-on can be soft-deleted between the validation loop and the `BulkUpdateSortOrder` transaction. The bulk UPDATE filters `AND deleted_at IS NULL`, so a concurrently-deleted add-on is silently skipped — 0 rows affected for that ID. The caller receives HTTP 204 but the deleted add-on's sort_order is not updated. This is a minor correctness issue, not a security issue — the service returns no error and the stale sort_order on a deleted (invisible) row causes no harm.

2. **DoS (moderate):** An authenticated `tenant_admin` with `service.update` can POST a 200-item reorder payload, causing 200 `SELECT` statements before a single write. If called in a rapid loop (no per-endpoint rate limit exists), this creates heavy read load on the `service_addon_tenant_id_idx`. The global 60 req/s rate limiter (`route.go:47`) is the only guard.

**Recommended fix (go-expert):**

Replace the N+1 loop with a single batched query. Add a `FindByIDs(ctx, ids []string) ([]*model.ServiceAddon, error)` method to `ServiceAddonRepository` that runs `SELECT ... WHERE id IN (?) AND deleted_at IS NULL` and returns all matching rows in one query. In `Reorder`, compare the returned set against the input set:

```go
rows, err := s.addons.FindByIDs(ctx, itemIDs)
if err != nil { return err }
if len(rows) != len(in.Items) {
    return constants.ErrServiceAddonNotFound // one or more IDs invalid/deleted
}
for _, row := range rows {
    if row.ServiceID != in.ServiceID || row.TenantID != in.CallerTenantID {
        return constants.ErrServiceAddonNotFound
    }
}
```

This reduces validation from N queries to 1 query and eliminates the TOCTOU gap when run inside the same transaction as `BulkUpdateSortOrder`.

**Blocks sign-off:** No. Exploitable only by an authenticated `tenant_admin`; blast radius limited to their own tenant's add-on sort ordering. Fix before production load testing.

---

### Findings — Low

---

#### L-1: `reorderAddons` server action sends bare array instead of `{"items": [...]}` — request body shape mismatch

**Severity:** Low (functional bug with security annotation)

**File:line:** `lustia/web/tenant-admin/app/master/services/[id]/addon-actions.ts:128`

**Attack scenario:** `reorderAddons` calls:
```typescript
body: JSON.stringify(items)   // items: ReorderAddonItem[]
```
This sends the body as a bare JSON array: `[{id, sort_order}, ...]`.

The backend `ReorderAddonsRequest` struct expects:
```go
type ReorderAddonsRequest struct {
    Items []AddonSortOrderItemRequest `json:"items" binding:"required,min=1,max=200,dive"`
}
```
GORM/Gin's `ShouldBindJSON` will fail to unmarshal a bare array into a struct — `Items` will be an empty slice, and the `binding:"required,min=1"` tag will return HTTP 400 `VALIDATION`. The reorder endpoint is effectively broken from the frontend today.

No security impact — the backend correctly rejects the malformed body. But the failure mode means the `max=200` DoS guard is trivially bypassed by accident (an empty `Items` slice is always rejected before counting).

**Recommended fix (nextjs-expert):**

```typescript
body: JSON.stringify({ items })
```

**Blocks sign-off:** No. Backend rejects the malformed body safely. Fix in the same PR as the feature.

---

#### L-2: List endpoint has no server-side item count cap — implicit reliance on ADR 0010 §5 Q2 "expected small" assumption

**Severity:** Low

**File:line:** `lustia/services/auth/internal/repository/service_addon_repository.go:51–69`, `lustia/services/auth/internal/service/service_addon_service.go:106`

**Assessment:** `FindByServiceID` and `List` return all non-deleted add-ons for a service with no `LIMIT` clause and no pagination. The ADR notes "no hard limit; UX may paginate if a service somehow has >20" (§5 Q2). If a tenant creates a large number of add-ons (e.g. 10,000 by scripted abuse), a single `GET /services/:id/addons` would load all rows into memory. The global body-size cap (1 MB per Section 6.5) does not bound the result set size; it bounds the request. For Phase 4 with no booking engine the risk is low, but the assumption "expected to be small" should be hardened before Phase 5.

**Recommended fix (go-expert):** Add a `LIMIT 200` to `FindByServiceID` (matching the reorder cap as a natural ceiling), or enforce a configurable soft cap in the service layer that returns an error if exceeded. Alternatively, add a partial unique index guard that limits `(service_id) WHERE deleted_at IS NULL` rows to a configurable maximum via a trigger or application check.

**Blocks sign-off:** No. Add-on creation rate is bounded by human UX in Phase 4. Address before Phase 5 booking engine.

---

### Findings — Info

---

#### I-1: `sort_order` has no `max` constraint — INT allows values up to 2,147,483,647

**Severity:** Info

**File:line:** `lustia/services/auth/internal/controller/dto_request.go:284,294,313`

**Assessment:** DTO binding uses `binding:"min=0"` for `sort_order` but no `max`. The DB column is `INT NOT NULL` which allows values up to `2^31-1`. No security risk (sort_order is internal display ordering, not used in access control decisions). A client could store `sort_order = 2147483647` which would push the add-on to the last position and could look surprising in a future UI that displays raw integers. Harmless for Phase 4.

**Recommended fix:** Add `max=9999` (or a configurable reasonable ceiling) to the binding tag. Not blocking.

---

#### I-2: `ServiceAddon.TenantID` is included in `ServiceAddonDetail` DTO but excluded from `ServiceAddonResponse` — minor API surface inconsistency

**Severity:** Info

**File:line:** `lustia/services/auth/internal/service/dto.go` (`ServiceAddonDetail`), `lustia/services/auth/internal/controller/dto_response.go:327` (`ServiceAddonResponse`)

**Assessment:** `ServiceAddonDetail` in the service layer includes `TenantID string` (used for audit and cross-layer calls). `ServiceAddonResponse` (the API response type) correctly omits `TenantID`, per the API contract note at §12.4: "tenant_id is not exposed in the response (RLS ensures the caller can only see their own rows)." The service-to-controller translation in `toServiceAddonResponse` strips `TenantID`. This is the correct behaviour — no information disclosure. Noted only to confirm the reviewer checked this path.

---

### Phase 4 Add-ons Sign-off Recommendation

**Decision: NOT BLOCKED — no Critical or High findings. Completion is permitted with the following tracked items.**

| Severity | Finding | Owner | Target |
|---|---|---|---|
| Medium | M-1: Reorder validation is N+1 queries outside a transaction (TOCTOU + modest DoS surface) | `go-expert` | Before production load testing |
| Low | L-1: `reorderAddons` server action sends bare array instead of `{"items": [...]}` — reorder broken from frontend | `nextjs-expert` | Same PR / immediate fix |
| Low | L-2: List endpoint has no `LIMIT` — relies on "expected small" assumption | `go-expert` | Before Phase 5 booking integration |
| Info | I-1: `sort_order` has no `max` binding constraint | `go-expert` | Hardening, no urgency |
| Info | I-2: `TenantID` stripped correctly in API response — confirmed design | — | No action needed |

The ADR 0010 open question (branch_admin scope for add-ons) is resolved by the existing permission model: `service.update` is not granted to `branch_admin`, so branch-scoped users are already blocked from add-on mutations. No schema or code change required. This should be documented in ADR 0010 §5 to close the question explicitly.

RLS on `service_addon` is complete and correct. Cross-tenant isolation is enforced at service layer (resolveAddon pattern) and backstopped by RLS. Soft-delete is irreversible via the API. Audit events contain no PII. Frontend server actions derive tenant context from session cookie only. The only functional defect found (L-1) is a body-shape mismatch that causes the reorder endpoint to return HTTP 400 from the frontend — the backend safely rejects it, making this a correctness issue rather than a security one.

| Date | Change reviewed | Findings | Status |
|---|---|---|---|
| 2026-04-24 | ADR 0010 — `service_addon` table, migration 000018, 6 new endpoints + `GET /services/:id` extension, Next.js server actions | **[Medium]** M-1: Reorder validation N+1 queries — TOCTOU + DoS surface (`service_addon_service.go:247–254`). **[Low]** L-1: `reorderAddons` server action sends bare array instead of `{"items": [...]}` — reorder endpoint broken from frontend (`addon-actions.ts:128`). **[Low]** L-2: List endpoint has no `LIMIT` cap (`service_addon_repository.go:51–69`). **[Info]** I-1: `sort_order` has no `max` binding constraint. **[Info]** I-2: `TenantID` correctly excluded from API response — confirmed. | **NOT BLOCKED — no Critical/High findings** |
| 2026-04-24 | ADR 0010 (tenant-wide redesign) — `addon` table, migration 000018 (rewritten), 7 new endpoints `/tenant/addons/*`, `000019` dev seed, Next.js server actions `master/addons/actions.ts` | **[Medium]** M-1: Cursor subquery lacks explicit tenant constraint — implicit RLS dependency (`addon_repository.go:91–96`). **[Low]** L-1: `updateAddon` server action sends `is_active` in PATCH body — silently ignored by backend (`actions.ts:120–125`). **[Low]** L-2: Dev seed `000019` has no database-name guard against accidental staging application (`000019_seed_dev_addons.up.sql`). **[Info]** I-1: `addon.created` audit event logs `addon_id` in `meta` redundantly — minor but fine. **[Info]** I-2: Superseded-design review above marked with SUPERSEDED banner. | **NOT BLOCKED — no Critical/High findings** |

---

## Phase 4 Add-ons Review (ADR 0010, tenant-wide redesign) -- 2026-04-24

_Reviewer: security-expert. This is the SECOND review of the add-on feature. The first review (above, now marked SUPERSEDED) covered the per-service `service_addon` design that was pivoted in the same session. This review covers the replacement: tenant-wide `addon` table, migration 000018 (rewritten), 7 endpoints under `/api/v1/tenant/addons/*`, dev seed 000019, and Next.js server actions in `lustia/web/tenant-admin/app/master/addons/actions.ts`._

_Code read: `000018_phase4_addons.up.sql`, `000019_seed_dev_addons.up.sql`, `model/addon.go`, `repository/addon_repository.go`, `service/addon_service.go`, `service/addon_service_test.go`, `controller/addon_controller.go`, `controller/dto_request.go` (addon DTOs), `route/route.go`, `middleware/tenant.go`, `middleware/rbac.go`, `middleware/scope_gate.go`, `constants/permissions.go`, `web/tenant-admin/app/master/addons/actions.ts`, `lib/api.ts`, `docs/DECISIONS/0010-per-service-addons.md`._

---

### Executive Summary

The tenant-wide redesign is a sound implementation. The `addon` table has complete RLS coverage (SELECT/INSERT/UPDATE policies; no DELETE grant). Cross-tenant isolation is enforced at the service layer on every read and mutation path. Reorder batch-validates all IDs inside the transaction (the N+1 M-1 finding from the previous review is resolved by design). Input validation is UTF-8-aware (character length, not bytes) at the service layer and backed by DB CHECK constraints. Permission seeding is idempotent. The `branch_admin` write-block is correct: migration seeds `addon.read` only to `branch_admin`; each write endpoint applies `PermAddonCreate/Update/Delete` RBAC.

**No Critical or High findings.** One Medium, two Low, and two Info findings are detailed below. This review does not block completion.

**Finding count:** 0 Critical, 0 High, 1 Medium, 2 Low, 2 Info.

---

### STRIDE Extension for ADR 0010 Tenant-wide Redesign

| # | Category | Concrete Threat | Where | Impact | Mitigation | Status |
|---|---|---|---|---|---|---|
| T-8 | Tampering | `PUT /tenant/addons/reorder` body contains IDs from another tenant | `addon_service.go:228-251` | Cross-tenant sort_order pollution | `FindByIDs` inside tx + per-item tenant check | Implemented |
| T-9 | Tampering | `PATCH/DELETE /tenant/addons/:id` targets an add-on owned by another tenant | `addon_service.go:97-106, 115-116, 160-161, 192-193` | Cross-tenant mutation | Service: `a.TenantID != callerTenantID` returns 404 on mismatch; RLS backstop | Implemented |
| I-7 | Information Disclosure | Cursor UUID probing -- caller provides a cross-tenant add-on UUID as cursor | `addon_repository.go:87-97` | UUID existence oracle (empty page vs. normal page) | RLS returns NULL from subquery -- empty page, no data exposed; see M-1 | Partial -- see M-1 |
| D-6 | Denial of Service | Reorder payload with 200 items in rapid loop | `addon_service.go:212`, `dto_request.go:317` | DB write load | `binding:"max=200"` caps array; global rate limiter only guard | Acceptable |
| E-6 | Elevation of Privilege | `branch_admin` JWT calls `POST/PATCH/DELETE /tenant/addons/*` | `route.go:112`, `rbac.go:36` | Mutating tenant-wide catalog without privilege | `rbacMW(PermAddonCreate/Update/Delete)` aborts with 403; `branch_admin` holds `addon.read` only | Implemented |

---

### Cross-Cutting Check Results

**1. RLS fail-closed behavior.** Policy: `tenant_id::text = current_setting('app.current_tenant', true)`. The second argument `true` is the "missing-ok" flag -- if `app.current_tenant` is not set, `current_setting` returns empty string `''`. An empty string cannot equal any UUID text representation, so the policy evaluates false and returns no rows. No fail-open edge case.

**2. `app.current_tenant` guaranteed on every request.** `middleware/tenant.go` runs as a Gin group middleware on `tenantGroup` (line 101 in `route.go`) before any handler executes. It calls `tm.BeginForRequest` then `tm.SetTenantContext(txCtx, tenantID, userID)`. If `SetTenantContext` fails the middleware aborts with 500 -- the handler never runs. For `scope=tenant` JWTs the real tenant UUID is used; any other scope gets `__platform__` which matches no `addon` row (correct). There is no code path through `tenantGroup` where a handler runs before `app.current_tenant` is set.

**3. Permission model -- `branch_admin` write-block.** Migration 000018 seeds `addon.read/create/update/delete` and wires them: `super_admin` and `tenant_admin` receive all four; `branch_admin` receives `addon.read` only. Route registration at `route.go:112-114` uses `rbacMW(PermAddonCreate)` on POST, `rbacMW(PermAddonUpdate)` on PATCH and PUT (reorder), `rbacMW(PermAddonDelete)` on DELETE. A `branch_admin` JWT calling `POST /tenant/addons` receives HTTP 403. Confirmed correct.

**4. Cross-tenant isolation.** Every service-layer operation checks `TenantID` after the repository load: `Get` (line 102), `Update` (line 115), `ChangeStatus` (line 160), `SoftDelete` (line 192), `Reorder` (line 245 per-item). All return 404 on mismatch -- no information about the target row's existence in another tenant is disclosed.

**5. Soft-delete irreversibility.** `SoftDelete` sets `deleted_at = now()` and `is_active = false`. Both `Update` and `ChangeStatus` filter `WHERE id = ? AND deleted_at IS NULL` via repository -- a soft-deleted add-on returns `ErrAddonNotFound`. No restore endpoint exists. Soft-deleted add-ons cannot be resurrected via any current API surface.

**6. Reorder atomicity and IDOR guard (M-1 from previous review resolved).** `Reorder` at `addon_service.go:220` wraps everything in `s.tx.WithTx(...)`. Inside the transaction: (a) `FindByIDs` batch-fetches all requested IDs in one query with `AND deleted_at IS NULL`; (b) per-item tenant check; (c) `BulkUpdateSortOrder` issues a single CASE-expression UPDATE. All steps share the same transaction. The N+1 TOCTOU finding from the previous review is resolved by design.

**7. Input validation.** `validateAddonInput` at `addon_service.go:275` uses `utf8.RuneCountInString` -- character-length, not byte-length. DB CHECKs use `char_length()` (also character-aware in PostgreSQL). DTO binding tags enforce `min=1,max=120` (name), `max=500` (description), `min=0` (price), `min=0,max=9999` (sort_order). All three layers are consistent.

**8. SQL injection surface -- reorder CASE expression.** `BulkUpdateSortOrder` at `addon_repository.go:179-215` builds a CASE expression using `"WHEN ? THEN ? "` with `args = append(args, it.ID, it.SortOrder)`. All values are bind parameters. No injection surface.

**9. Server actions tenant isolation.** `actions.ts` functions call `apiFetch` with `{ auth: true }`, which calls `getAccessToken()` and attaches `Authorization: Bearer <token>`. No `tenant_id` field is included in any request body. Tenant context is derived entirely from the signed JWT. Correct.

**10. Permission seed idempotency and FK correctness.** All INSERTs use `ON CONFLICT (id) DO NOTHING` or `ON CONFLICT DO NOTHING`. Role_permission rows are seeded via `SELECT ... FROM role r CROSS JOIN permission p WHERE r.name = 'X'`. If the role does not exist the CROSS JOIN returns zero rows and zero inserts occur -- no FK error. Re-running is safe.

**11. Audit events.** `addon.created` meta: `{"addon_id": uuid}`. `addon.updated` meta: empty. `addon.activated/deactivated` meta: `{"is_active": bool}`. `addon.reordered` meta: `{"count": int}`. No `name`, `description`, or `price_idr` in any audit event. No PII. Phase 3 H-2 class violation not repeated.

**12. `TenantID` exclusion from API response.** `toAddonResponse` at `addon_controller.go:225-235` maps from `service.AddonDetail` (which includes `TenantID`) to `AddonResponse` (which does not). Tenant ID is correctly stripped from the wire response.

---

### Findings -- Medium

---

#### M-1: Cursor subquery lacks explicit tenant constraint -- implicit RLS dependency (CWE-89 latent, same class as Phase 4 M-3)

**Severity:** Medium

**File:line:** `lustia/services/auth/internal/repository/addon_repository.go:91-96`

**Assessment:** The cursor subquery is:

```sql
(sort_order, created_at, id) > (
    SELECT sort_order, created_at, id FROM addon WHERE id = ? AND deleted_at IS NULL
)
```

The `filter.Cursor` UUID is a bind variable (no injection). However the subquery has no `AND tenant_id = ?` predicate. PostgreSQL applies RLS to the `addon` table in the subquery because it runs on the same `lustia_app` connection with `app.current_tenant` set -- so a cursor UUID belonging to another tenant returns NULL, and `> NULL` evaluates to false (empty page, no data leak).

The correctness of cursor isolation depends on implicit RLS behavior in a subquery rather than an explicit WHERE clause. If any future code path constructs this query before the tenant middleware sets `app.current_tenant`, the subquery would return cross-tenant rows. Additionally, a caller providing a foreign-tenant UUID as cursor receives an empty page rather than a validation error -- a minor UUID existence oracle (same as Phase 4 M-3 finding on therapist/service repositories).

**Recommended fix (go-expert):**

```go
q = q.Where(
    `(sort_order, created_at, id) > (
        SELECT sort_order, created_at, id FROM addon
        WHERE id = ? AND tenant_id = ? AND deleted_at IS NULL
    )`,
    filter.Cursor, tenantID,
)
```

This makes the intent explicit, removes the implicit RLS dependency, and eliminates the UUID existence oracle.

**Blocks sign-off:** No. RLS provides actual isolation today. Fix before Phase 5 booking integration.

---

### Findings -- Low

---

#### L-1: `updateAddon` server action sends `is_active` in the PATCH body -- silently ignored by backend (functional gap)

**Severity:** Low

**File:line:** `lustia/web/tenant-admin/app/master/addons/actions.ts:120-125`

**Assessment:** `updateAddon` sends `{ name, description, price_idr, is_active }` to `PATCH /tenant/addons/:id`. The backend `UpdateAddonRequest` accepts only `name`, `description`, `price_idr`, and `sort_order` -- no `is_active` field. Gin's `ShouldBindJSON` silently ignores unknown fields. The `is_active` value in the update body is therefore dropped. Toggling the active checkbox on the edit form has no effect; the correct path is `PATCH /tenant/addons/:id/status` via `toggleAddonStatus`.

No security impact -- the backend correctly ignores the unknown field. The failure mode is silent incorrect UX.

**Recommended fix (nextjs-expert):** Remove `is_active` from the `updateAddon` PATCH body and ensure the edit form calls `toggleAddonStatus` separately for the active/inactive toggle.

**Blocks sign-off:** No. Fix before user testing.

---

#### L-2: Dev seed migration 000019 has no database-name guard against accidental staging application (same class as Phase 4 L-3)

**Severity:** Low

**File:line:** `lustia/migrations/000019_seed_dev_addons.up.sql:1-10`

**Assessment:** The header clearly states "DEV-ONLY" and provides a stopping instruction (`migrate ... up 18`). All INSERTs use `ON CONFLICT (id) DO NOTHING`. No credentials or privilege grants are seeded. Risk is purely operational: an operator who accidentally runs the full migration chain in staging inserts 4 add-on rows into real tenant data.

**Recommended fix:** Add a database-name guard (same pattern recommended for 000014 in Phase 4 L-3):

```sql
DO $$
BEGIN
    IF current_database() NOT LIKE '%dev%' AND current_database() NOT LIKE '%local%' THEN
        RAISE EXCEPTION 'Migration 000019 is dev-only. Database "%" does not match dev/local pattern.', current_database();
    END IF;
END $$;
```

Alternatively, enforce via CI/CD pipeline configuration (staging/prod jobs run `migrate up 18` only).

**Blocks sign-off:** No. Fix before first staging deployment.

---

### Findings -- Info

---

#### I-1: `addon.created` audit event includes `addon_id` in `meta` -- redundant with `ResourceID`

**Severity:** Info

**File:line:** `lustia/services/auth/internal/service/addon_service.go:66-68`

**Assessment:** `addon.created` sets `ResourceID: a.ID` and `Meta: map[string]interface{}{"addon_id": a.ID}` -- the UUID appears twice in the audit row. Not a security issue. `addon.updated`, `addon.deleted`, and `addon.reordered` do not repeat the resource ID in meta. Minor inconsistency worth cleaning up for audit log hygiene.

---

#### I-2: No way to create an add-on in inactive state -- design gap, not a security issue

**Severity:** Info

**File:line:** `lustia/services/auth/internal/service/addon_service.go:50`

**Assessment:** `AddonService.Create` hardcodes `IsActive: true`. `CreateAddonRequest` has no `is_active` field. The frontend `createAddon` action sends `is_active` in the POST body but it is silently ignored. There is currently no API path to create a pre-inactive add-on. This is likely intentional (new add-ons go live immediately; operators deactivate if needed). Should be documented as an explicit design decision in ADR 0010 to prevent future confusion.

---

### RLS Coverage Assessment -- ADR 0010 Tenant-wide `addon` Table

| Table | SELECT | INSERT | UPDATE | DELETE | lustia_app grants | Assessment |
|---|---|---|---|---|---|---|
| `addon` | `addon_tenant_select` (tenant equality) | `addon_tenant_insert` (tenant equality WITH CHECK) | `addon_tenant_update` (tenant equality USING + WITH CHECK) | N/A | SELECT, INSERT, UPDATE (no DELETE) | Complete. Hard DELETE blocked at DB level as intended. |

---

### Phase 4 Add-ons (Tenant-wide Redesign) Sign-off Recommendation

**Decision: NOT BLOCKED -- no Critical or High findings.**

| Severity | Finding | Owner | Target |
|---|---|---|---|
| Medium | M-1: Cursor subquery lacks explicit tenant constraint -- implicit RLS dependency (`addon_repository.go:91-96`) | `go-expert` | Before Phase 5 booking integration |
| Low | L-1: `updateAddon` sends `is_active` in PATCH body -- silently ignored; status change does not persist (`actions.ts:120-125`) | `nextjs-expert` | Before user testing |
| Low | L-2: Dev seed 000019 has no database-name guard (`000019_seed_dev_addons.up.sql`) | `go-expert` or `devops-expert` | Before first staging deployment |
| Info | I-1: Redundant `addon_id` in `addon.created` meta | `go-expert` | Hardening, no urgency |
| Info | I-2: No way to create an inactive add-on -- likely intentional; document in ADR 0010 | orchestrator | Clarify in ADR |

The previous review M-1 finding (N+1 reorder validation outside a transaction) has been fully resolved in this redesign: `Reorder` now calls `FindByIDs` inside `tx.WithTx`. The `branch_admin` write-block is correctly implemented via migration seeding and RBAC middleware. Soft-delete is irreversible. No mass-assignment surface. No PII in audit events. Frontend server actions derive tenant context from session cookie only.

Phase 1-4 controls (Argon2id, RS256 JWT, RLS tenant isolation, refresh token rotation, `must_change_password`, branch-level isolation) remain sound and were not regressed by this change.

---

## Phase 5 — Booking Engine + Customer Mobile App

_Reviewer: security-expert. Authority: may BLOCK Phase 5 implementation on Critical or High findings — these must be resolved in ADR or design before `go-expert` writes booking code. Reviewed: ADR 0014, migration 000025, ADR 0005, existing SECURITY.md Sections 1–11 incorporated by reference._

_Date: 2026-04-25_

---

### Executive Summary

Phase 5 is the highest-risk phase to date. It introduces the first unauthenticated public API surface, a payment webhook endpoint, customer PII (name/phone/email) on a new table, and a mobile app client. The threat model is dominated by three concerns: (1) **public endpoint abuse** without auth to lean on, (2) **payment integrity** (webhook forgery, total-price tampering, replay), and (3) **cross-tenant contamination** via the new `__public__` sentinel path. The schema and RLS design from migration 000025 are generally sound. Several design constraints MUST be locked in the ADR before coding begins — these are marked BLOCK. All others are Code Review territory.

**Finding count by severity: 2 Critical | 6 High | 6 Medium | 4 Low | 2 Info**

**Blocking verdict: BLOCKED — 2 Critical and 6 High findings must be resolved (ADR-documented or mitigated in design) before `go-expert` writes booking service code.**

---

### STRIDE Extension for Phase 5

| # | Category | Concrete Threat | Where | Impact | Mitigation | Status |
|---|---|---|---|---|---|---|
| S-5 | Spoofing | Midtrans webhook forged by attacker — dummy adapter accepts any payload | `POST /public/payments/webhook` | Booking marked paid without real payment | Dummy adapter reachable only in non-production builds; real adapter verifies Midtrans SHA-512 signature | Must enforce — see H-3 |
| S-6 | Spoofing | Customer presents an expired/cancelled booking code at check-in | `POST /tenant/bookings/:id/checkin` | Denied customer claims; social-engineering ops staff | Service layer checks `status == paid`; code is looked up server-side; fake code returns 404 | Design |
| T-10 | Tampering | `total_price_idr` injected via request body overrides server-computed total | `POST /public/bookings` | Revenue loss — attacker pays IDR 1 for any service | Total computed server-side only; body field must be ignored | Must enforce — see C-1 |
| T-11 | Tampering | Webhook payload claims `gross_amount` lower than `booking.total_price_idr` | `POST /public/payments/webhook` | Booking marked paid for underpayment | Webhook handler must compare amounts before transitioning status | Must enforce — see H-4 |
| T-12 | Tampering | Concurrent booking requests for the same slot — slot double-booked | `booking` GiST exclusion constraint | Duplicate booking, double revenue | DB-level exclusion constraint (migration 000025) covers `pending_payment`+`paid`+`checked_in`+`completed` | Design (see H-5 for edge) |
| T-13 | Tampering | Cross-tenant FK smuggling in `POST /public/bookings` — attacker sends `service_id` from tenant B for a branch of tenant A | `booking` INSERT path | Cross-tenant data pollution | Service layer validates every FK against the resolved `tenant_id`; see H-1 | Must enforce |
| R-3 | Repudiation | Booking cancelled by ops; no evidence of amount collected | `POST /tenant/bookings/:id/cancel` | Dispute with no audit trail | `cancelled_by` + `cancel_reason` + `cancelled_at` stored on booking row; `booking.cancelled` event to `audit_log` required | Design |
| I-8 | Information Disclosure | `GET /public/bookings/:code` returns full customer PII (name/phone/email) to anyone with the code | Customer-facing code lookup | PII exposure if code shared | By design — code is the auth artifact; partial masking recommended for phone/email | See M-3 |
| I-9 | Information Disclosure | Public branch listing leaks branches of inactive tenants | `GET /public/branches` | Business intelligence leak | RLS + SQL filter: `tenant.status = 'active' AND branch.status = 'active'` | Design |
| D-7 | Denial of Service | Unauthenticated flood of `POST /public/bookings` — each call runs availability check, exclusion constraint, email send | Public endpoint | Service CPU/DB/email exhaustion | Per-IP rate limit (see H-2); body-size cap; lazy expiry sweep on every call | Must enforce |
| D-8 | Denial of Service | Webhook endpoint flooded — each call reads + writes booking row | `POST /public/payments/webhook` | DB connection exhaustion | Midtrans source IP allowlist in production; rate limit on webhook endpoint | See M-4 |
| E-7 | Elevation of Privilege | Branch admin creates concierge booking with `service_id`/`room_id`/`therapist_id` from a different branch | `POST /tenant/bookings` (concierge) | Cross-branch resource booking | Service layer checks all FK resources belong to caller's branch scope | Must enforce — see H-6 |
| E-8 | Elevation of Privilege | `__public__` sentinel passed as a real tenant UUID to resolve a booking INSERT under another tenant's RLS | INSERT path in booking service | Cross-tenant booking injection | `__public__` is blocked from INSERT policy; only resolved `tenant_uuid` may be used for INSERT | Design (see H-1) |

---

### Section A — Public Endpoints (No Auth)

---

#### C-1 (Critical): `total_price_idr` must NEVER be accepted from the request body — server must compute it

**Severity:** Critical — BLOCKS implementation

**Location:** `POST /api/v1/public/bookings` body parsing (to be implemented by `go-expert`)

**Attack scenario:** The ADR 0014 §3.16 request shape includes `{branch_id, service_id, addon_ids[], ...}` — it does NOT include `total_price_idr`. However if the `go-expert` models the incoming DTO with a `total_price_idr` field (e.g. to mirror the response DTO), a client can supply `"total_price_idr": 1` and if that value flows into the booking INSERT it overrides the server calculation. The customer then pays IDR 1 for a IDR 500,000 service. Blast radius: every booking on the platform at a user-chosen price.

**Required design constraint (must be in ADR before code):**

The `CreateBookingRequest` DTO (public) MUST NOT contain `total_price_idr`, `payment_reference`, `status`, `paid_at`, `tenant_id`, or any lifecycle field. These are computed/set server-side exclusively. The service layer computes total as:

```
total = service.price + SUM(addon.price_idr for each addon_id in validated list)
```

where each `addon_id` is validated to belong to the same `tenant_id` as the resolved branch. The `booking_addon.price_idr` column stores a snapshot of `addon.price_idr` at booking time — the INSERT must use the DB-fetched price, not any client-supplied value.

**Blocks Phase 5 coding:** Yes. `go-expert` must confirm this constraint in `dto_request.go` before the booking controller is written.

---

#### C-2 (Critical): Webhook dummy adapter MUST be hard-blocked in production builds — not merely "unreachable"

**Severity:** Critical — BLOCKS implementation

**Location:** `POST /api/v1/public/payments/webhook` (stub implementation, future real adapter)

**Attack scenario:** ADR 0014 §3.3 documents a dummy `MidtransClient` that returns `paid` for any payload. In Phase 5 this is the only adapter. If the service is deployed to staging or production with the dummy adapter active, any attacker can `POST` to the webhook endpoint with an arbitrary payload and mark any booking as paid without payment. Since the webhook is a public endpoint (no auth), the exploit requires only knowledge of the endpoint URL and a `booking_id` or `payment_reference` (which may be guessable if `payment_reference` stores the `code` value).

**Required design constraint:**

The adapter selection MUST be controlled by an environment variable (e.g. `PAYMENT_ADAPTER=dummy|midtrans`) AND the application MUST fail-fast at startup if `PAYMENT_ADAPTER=dummy` AND `APP_ENV != local|dev`. Concretely in `main.go`:

```go
if cfg.Payment.Adapter == "dummy" && cfg.AppEnv != "local" && cfg.AppEnv != "dev" {
    log.Fatal(ctx, "dummy payment adapter is not permitted outside local/dev environments")
}
```

This means an accidental `PAYMENT_ADAPTER=dummy` in a staging `.env` causes the service to refuse to start — a visible, loud failure rather than a silent security hole. The dummy adapter must be in a separate Go package (`internal/adapter/payment/dummy`) with a build tag (`//go:build dev`) to prevent it from being included in production binaries entirely (belt-and-suspenders with the runtime guard).

**Blocks Phase 5 coding:** Yes. `go-expert` must implement the adapter selection guard before any payment code is written.

---

### Section B — Payment Integration

---

#### H-1: Cross-tenant FK validation on `POST /public/bookings` — service MUST validate all resource IDs against resolved tenant

**Severity:** High — BLOCKS implementation

**Location:** `POST /api/v1/public/bookings` service layer (to be written)

**Attack scenario:** The public booking endpoint receives `{branch_id, service_id, addon_ids[], room_id?, therapist_id?, scheduled_start}`. The `branch_id` is used to resolve `tenant_id`. An attacker could supply a valid `branch_id` from tenant A alongside a `service_id` from tenant B. If the service layer does not validate cross-FK ownership, the booking row is inserted with `tenant_id = tenant_A.id` but references a service that belongs to tenant B. This corrupts booking analytics, allows cross-tenant service disclosure (confirms UUID existence via `404 vs 409`), and may allow price manipulation if tenant B has cheaper or zero-price services.

**Required validation pattern for `go-expert`:**

```go
// Step 1: Resolve tenant from branch
branch, err := branchRepo.FindActiveByID(ctx, req.BranchID)  // checks active + not deleted
if err != nil { return 404 }
tenantID := branch.TenantID

// Step 2: Set app.current_tenant = tenantID for this transaction
tx.SetTenantContext(ctx, tenantID)

// Step 3: Validate all resource FKs belong to same tenant
service, err := serviceRepo.FindByIDInTenant(ctx, tenantID, req.ServiceID)
if err != nil { return 404 }  // not 422 — no oracle

for _, addonID := range req.AddonIDs {
    addon, err := addonRepo.FindByIDInTenant(ctx, tenantID, addonID)
    // ... also confirm addon is_active
}

if req.RoomID != nil {
    room, err := roomRepo.FindByIDInBranchTenant(ctx, tenantID, branch.ID, *req.RoomID)
    // room must belong to both tenant AND the specific branch
}

if req.TherapistID != nil {
    // same pattern
}
```

Every mismatch returns 404 (not 422 / 409) to avoid confirming whether the UUID exists in another tenant.

**Blocks Phase 5 coding:** Yes. Encoding this pattern in the ADR before coding prevents accidental omission.

---

#### H-2: Rate limiting on `POST /public/bookings` — concrete thresholds required before coding

**Severity:** High — BLOCKS implementation

**Location:** `POST /api/v1/public/bookings` + `GET /api/v1/public/branches`

**Requirements (must be documented in ADR before coding):**

| Endpoint | Per-IP limit | Global limit | Notes |
|---|---|---|---|
| `POST /public/bookings` | 5 req/min, 20 req/hour | 500 req/hour platform-wide | Each call: DB read + GiST constraint eval + email send |
| `GET /public/bookings/:code` | 20 req/min per IP | — | Hard rate limit to limit code brute-force |
| `GET /public/branches` | 60 req/min per IP | — | Read-only, cacheable; less restrictive |
| `GET /public/branches/:id` | 60 req/min per IP | — | Same |
| `GET /public/branches/:id/availability` | 30 req/min per IP | — | Computes slot availability; moderate cost |
| `POST /public/payments/webhook` | 60 req/min per IP | — | Allowlisted to Midtrans IPs in prod |

Implementation note for `go-expert`: use the existing in-memory limiter for Phase 5 (single instance). The global platform-wide cap on `POST /public/bookings` (500/hour) must be a separate atomic counter (e.g. a Redis counter in production, or a goroutine-safe in-memory counter for Phase 5 dev) — it cannot be expressed as a per-IP rule.

CAPTCHA (hCaptcha or Cloudflare Turnstile) is deferred but must be added before public launch. Add to Section 9.4 production readiness checklist.

**Blocks Phase 5 coding:** Yes — the rate limit middleware must exist before the booking handler is registered. Without it the public endpoint is a DoS vector on day one.

---

#### H-3: Webhook signature verification — dummy vs real adapter boundary enforcement

**Severity:** High (design constraint, reinforces C-2)

**Location:** `POST /api/v1/public/payments/webhook` real adapter

**Pattern for `go-expert` (in the real `MidtransAdapter`):**

Midtrans sends `signature_key = SHA-512(order_id + status_code + gross_amount + server_key)` in the notification body. Verification:

```go
expected := sha512.Sum512([]byte(
    notification.OrderID +
    notification.StatusCode +
    notification.GrossAmount +
    cfg.Midtrans.ServerKey,
))
if !subtle.ConstantTimeCompare(
    []byte(hex.EncodeToString(expected[:])),
    []byte(notification.SignatureKey),
) {
    return http.StatusUnauthorized, ErrWebhookSignatureInvalid
}
```

`subtle.ConstantTimeCompare` is mandatory — timing-safe comparison. Reject the payload before any DB read or status transition if the signature fails.

The dummy adapter MUST NOT implement this check (it accepts any payload) — this is deliberate for dev, but requires the hard build/runtime guard from C-2 to prevent it reaching production.

**Blocks Phase 5 coding:** Yes (as C-2 dependency). Recorded here as a specific implementation requirement for `go-expert`.

---

#### H-4: Webhook amount validation — underpayment must not mark booking paid

**Severity:** High — BLOCKS implementation

**Location:** `POST /api/v1/public/payments/webhook` handler (both dummy and real)

**Required check in the payment status transition:**

```go
// After signature verification (real) or payload parse (dummy):
if notification.TransactionStatus == "settlement" || notification.TransactionStatus == "capture" {
    booking, err := bookingRepo.FindByPaymentReference(ctx, notification.OrderID)
    if err != nil { return 404 }

    // Parse gross_amount from Midtrans (string in their API) to int64
    paid, err := parseIDRAmount(notification.GrossAmount)
    if err != nil || paid < booking.TotalPriceIDR {
        log.Warn(ctx, "webhook_amount_mismatch",
            "order_id", notification.OrderID,
            "expected", booking.TotalPriceIDR,
            "received", paid)
        // Do NOT mark paid. Treat as fraud signal. Notify ops.
        return http.StatusOK  // Return 200 to Midtrans so they don't retry; log + alert internally.
    }
    // Proceed to mark paid.
}
```

Returning HTTP 200 to Midtrans while internally suppressing the state transition is the correct pattern — Midtrans retries non-200 responses, which would flood logs and alert queues. Log the mismatch as a `payment.amount_mismatch` event to `audit_log` with full details for ops review.

**Blocks Phase 5 coding:** Yes.

---

#### H-5: Race condition on expiry sweep vs payment confirmation — sweep must use `RETURNING` and reject stale IDs

**Severity:** High — BLOCKS implementation

**Location:** Lazy expiry sweep + webhook handler (concurrent execution)

**Attack scenario:** Timeline:
1. `T=0`: Customer creates booking. Status = `pending_payment`. `created_at = T=0`.
2. `T=14m59s`: Customer completes payment on Midtrans. Webhook fires.
3. `T=15m01s`: Availability query triggers lazy sweep: `UPDATE booking SET status='expired' WHERE status='pending_payment' AND created_at < now()-interval '15 min' RETURNING id`.
4. Race: if the sweep UPDATE commits before the webhook handler reads the booking row, the booking is already `expired` when the webhook tries to transition it to `paid`. The webhook must not silently ignore this.

**Required implementation pattern:**

```go
// In webhook handler (after amount validation):
rowsAffected, err := bookingRepo.TransitionStatus(ctx,
    bookingID,
    StatusPendingPayment,  // expected current status
    StatusPaid,
    paidAt,
    paymentReference,
)
if rowsAffected == 0 {
    // Either already expired/paid/cancelled — look up current status
    current, _ := bookingRepo.FindByID(ctx, bookingID)
    if current.Status == StatusExpired {
        log.Warn(ctx, "payment_for_expired_booking", "booking_id", bookingID)
        // Alert ops — customer may have paid; needs manual refund review
        auditLog.Append(ctx, AuditEntry{
            Action: "payment.expired_booking_payment",
            ResourceID: bookingID,
            Meta: map[string]interface{}{"payment_reference": paymentReference},
        })
        return http.StatusOK  // Don't cause Midtrans retry storm
    }
    if current.Status == StatusPaid {
        // Idempotent — already processed. Return 200.
        return http.StatusOK
    }
}
```

The `TransitionStatus` repo method must use a conditional UPDATE:

```sql
UPDATE booking
SET status = 'paid', paid_at = $3, payment_reference = $4, updated_at = now()
WHERE id = $1 AND status = $2
```

The `WHERE status = $2` clause ensures the transition is atomic and conditional — if the sweep already flipped to `expired`, zero rows are affected and the handler can detect and alert.

**Blocks Phase 5 coding:** Yes. Without this pattern, a customer who pays at T=14m59s loses their booking silently.

---

#### H-6: Concierge booking — all resource FKs must be validated against caller's branch scope

**Severity:** High — BLOCKS implementation

**Location:** `POST /api/v1/tenant/bookings` (concierge, ops JWT)

**Scenario:** An ops staff member at Branch A creates a concierge booking. If the service layer does not validate that `service_id`, `room_id`, `therapist_id` all belong to the caller's tenant AND the specific `branch_id` in the request, the ops staff can book a therapist from Branch B into Branch A's slot (cross-branch resource theft). In a multi-branch tenant this is an IDOR at the branch level.

**Required check (mirrors `RoomService`/`TherapistService` pattern from ADR 0014 §3.16):**

```go
// After RBAC check (booking.create permission):
// Caller JWT claims: tenantID, callerBranches

// Branch must be in caller's branch scope (for branch_admin)
if !callerClaims.IsAdmin && !containsBranch(callerClaims.Branches, req.BranchID) {
    return 403 ErrCrossBranchForbidden
}

// All resource FKs: same tenant + same branch
service must have service.tenant_id == callerTenantID (service is tenant-scoped, not branch-scoped — OK)
therapist must have therapist.branch_id == req.BranchID AND therapist.tenant_id == callerTenantID
room must have room.branch_id == req.BranchID AND room.tenant_id == callerTenantID
```

**Blocks Phase 5 coding:** Yes.

---

### Section C — `__public__` Sentinel and Cross-Tenant Isolation

---

#### H-7: Public SELECT RLS policy on `booking` is overly broad — service layer MUST add `WHERE code = ?` predicate

**Severity:** High — design constraint

**Location:** `migration 000025_phase5_bookings.up.sql` — `booking_public_select` policy (lines 315–330)

**Assessment:** The migration comment explicitly documents this (the `PUBLIC READ CAVEAT` block at line 296–303). The `booking_public_select` policy passes when `current_tenant = '__public__'` AND the booking's branch is active AND tenant is active — but it does NOT restrict to a specific code. Without an application-level `WHERE code = ?` predicate, a query running under `__public__` sentinel could return ALL booking rows for active tenants.

**Required service-layer contract (must be enforced by `go-expert` and verified by `qa-expert`):**

1. Every public booking query MUST be `WHERE code = $1` — never `WHERE tenant_id = $1` or unbounded.
2. The repository method for public code lookup must be named `FindByCodePublic` (distinct from `FindByID`) and its signature must require a `code string` parameter with no way to call it without one.
3. `qa-expert` must add a test: call the public booking repository method without a `WHERE code` and assert it returns `ErrMethodNotAllowed` (or equivalent) at compile time via the type system, or at test time.

**Blocks Phase 5 coding:** Yes (noted as design constraint; `go-expert` must acknowledge this in the implementation). Not a schema fix — the schema is correct; the service-layer contract must be explicit.

---

### Section D — Booking-Engine Integrity

---

#### M-1: `GET /public/bookings/:code` — booking code brute-force feasibility

**Severity:** Medium

**Location:** `GET /api/v1/public/bookings/:code`

**Analysis:** Code format `[A-Z2-7]{4}-[A-Z2-7]{4}` uses 32 symbols × 8 positions = 32^8 = ~1.1×10^12 combinations. The rate limit from H-2 (20 req/min per IP) limits one IP to 28,800 attempts/day. To enumerate 0.1% of the space (1.1×10^9 attempts) from a single IP takes ~104 years — computationally infeasible for a single IP. A distributed botnet of 1,000 IPs at 20 req/min each attempts ~28.8M/day, which exhausts 0.0026% of the space per day — still infeasible for targeted brute-force.

**Assessment:** The 20 req/min hard rate limit from H-2 is sufficient. No additional mitigation needed beyond the rate limit and the UNIQUE DB index. Brute-force is not a practical threat at this keyspace + rate limit combination. Document the assumption: if the booking volume ever exceeds 10^8 active bookings simultaneously, collision probability needs re-evaluation (currently negligible per ADR 0014 §3.4).

**Code Review territory:** Confirm `booking_code_uidx` is present (it is, in migration 000025). Confirm the rate limit is applied. No ADR change needed.

---

#### M-2: `GET /public/bookings/:code` PII exposure — partial masking recommended

**Severity:** Medium

**Location:** `GET /api/v1/public/bookings/:code` response DTO

**Issue:** ADR 0014 §3.16 says the response "returns booking detail (no customer email/phone — only what the holder needs to verify)." However the API contract must make this explicit. The response DTO must be defined as:

```
booking_code, branch_name, service_name, scheduled_start, scheduled_end,
status, total_price_idr, customer_name, customer_phone (masked: last 4 only),
customer_email (masked: first 2 chars + domain), addons[]
```

Rationale: if a customer forwards their booking code to a friend (e.g., "scan this for me at check-in"), the friend should not see the full phone number and email of the person who booked.

**Required action for `go-expert`:** The public booking response DTO must mask `customer_phone` to `****XXXX` (last 4 digits) and `customer_email` to `fi**@domain.com` (first 2 + masked). The full values are available on the tenant-side endpoint (`GET /tenant/bookings/:id`) which requires authentication.

**Blocks Phase 5 coding:** No — Code Review territory. But must be agreed in the API contract before `nextjs-expert` / `flutter-expert` consume the response shape.

---

#### M-3: Lazy expiry sweep — must be triggered at booking-create, not just availability-list

**Severity:** Medium

**Location:** Lazy sweep invocation points

**Issue:** ADR 0014 §4.1 decides "lazy sweep on every list-availability + booking-create call." The concern is: if a slot has a stale `pending_payment` row and the customer never queries availability again (they navigated directly from a deep link or cached result), the expired row stays in the exclusion predicate and blocks the slot even after 15min. The sweep MUST also run on `POST /public/bookings` immediately before the exclusion constraint check. Without this, a stale `pending_payment` row for an expired booking blocks a new booking attempt with a confusing 409.

**Pattern:**

```go
// BookingService.Create — BEFORE the booking INSERT:
bookingRepo.SweepExpired(ctx)  // UPDATE ... WHERE status='pending_payment' AND created_at < now()-interval '15 min'
// Then attempt the INSERT (exclusion constraint now applies only to live pending_payment rows)
```

`SweepExpired` must use `RETURNING id` so the result can be logged and (in the `H-5` race case) the webhook handler can detect IDs that were just expired.

**Blocks Phase 5 coding:** No — Code Review territory. But must be implemented correctly per H-5.

---

#### M-4: Webhook endpoint — Midtrans source IP allowlist in production

**Severity:** Medium

**Location:** `POST /api/v1/public/payments/webhook`

**Issue:** Without an IP allowlist, any internet host can send arbitrary POST bodies to the webhook endpoint. In production the signature check (H-3) is the primary control, but defense-in-depth requires restricting the endpoint to Midtrans's published IP ranges at the ingress/WAF layer. This is an operational control for `devops-expert`, not a code control.

**Required action:** Add to Section 9.4 production readiness checklist: "Configure WAF/ingress to allowlist `POST /public/payments/webhook` to Midtrans IP ranges only (published at `https://docs.midtrans.com/reference/ip-address-and-api-whitelist`)."

**Blocks Phase 5 coding:** No — operational control. Code Review territory for the webhook handler itself. `devops-expert` owns this checklist item.

---

#### M-5: Webhook idempotency — same notification arriving twice

**Severity:** Medium

**Location:** `POST /api/v1/public/payments/webhook` handler

**Issue:** Midtrans may send the same notification more than once (retries on non-200, network duplicates). The handler must be idempotent: transitioning `paid → paid` again must be a no-op, not an error, and must not duplicate any side-effect (email send, audit log entry).

**Required pattern:** The `TransitionStatus` conditional UPDATE from H-5 (`WHERE status = 'pending_payment'`) naturally handles this: a second webhook for an already-paid booking finds zero rows and the handler detects `StatusPaid` → returns HTTP 200. No duplicate email is sent because the email send is inside the `if rowsAffected > 0` block.

**Blocks Phase 5 coding:** No — Code Review territory. But must be implemented as part of H-5 pattern.

---

#### M-6: No-cancel dispute path — ops contact information must be surfaced in app

**Severity:** Medium

**Location:** `flutter-expert` booking confirmation screen + T&C

**Issue:** ADR 0014 §3.5 correctly states "no refund path in Phase 5." However, a customer who has a legitimate dispute (payment charged but service not rendered, booking cancelled by ops after payment, etc.) has no documented escalation path. Without a contact channel in the app, customers will dispute directly with their bank/card (chargeback) rather than with the spa — chargebacks damage the platform's Midtrans merchant account.

**Required action (flutter-expert + ui-ux-expert):** The booking confirmation screen and the T&C screen must display: "Untuk keluhan atau pertanyaan, hubungi [branch phone/email from branch detail]." The API contract must include `branch.contact_phone` or `branch.contact_email` in the `GET /public/branches/:id` response so the Flutter app can surface it on the confirmation screen.

**Blocks Phase 5 coding:** No — design coordination item. Notify `flutter-expert` and `ui-ux-expert`.

---

### Section E — Geo Data

---

#### L-1: Branch lat/lng input validation — DB CHECK constraints are correct; document the trust chain

**Severity:** Low

**Location:** `branch.latitude` CHECK (-90 ≤ lat ≤ 90), `branch.longitude` CHECK (-180 ≤ lng ≤ 180) (ADR 0014 §3.9)

**Assessment:** DB CHECK constraints are the correct final gate. The service layer should additionally validate before the INSERT:

```go
if lat < -90 || lat > 90 { return ErrInvalidInput }
if lng < -180 || lng > 180 { return ErrInvalidInput }
```

This gives a clean 422 validation error rather than a DB-level constraint failure. The DTO binding tag should use `binding:"omitempty,min=-90,max=90"` for latitude and `binding:"omitempty,min=-180,max=180"` for longitude. No security issue — this is defense-in-depth input validation.

**Blocks Phase 5 coding:** No — Code Review territory.

---

#### L-2: Haversine distance calculation — earthdistance extension does not require PostGIS; confirm extension availability

**Severity:** Low

**Location:** `GET /public/branches?lat=&lng=` sorting logic

**Assessment:** ADR 0014 §3.9 specifies `cube` + `earthdistance` PostgreSQL extensions. The migration must confirm these are created before any distance query runs. The extensions are NOT installed by default on all PostgreSQL distributions (e.g. RDS may need explicit enabling). Flag for `devops-expert`: verify `cube` and `earthdistance` are available and enabled in the production DB before deploying Phase 5.

**Blocks Phase 5 coding:** No — operational item. Add to Section 9.4 checklist.

---

### Section F — Mobile App Surface

---

#### L-3: Local storage for booking codes — acceptable risk with documented assumptions

**Severity:** Low

**Location:** Flutter app `shared_preferences` (favorites + recent codes)

**Assessment:** `shared_preferences` on Android stores data in unencrypted XML in the app's private data directory. On iOS it stores data in the app's sandbox (unencrypted). On non-rooted/non-jailbroken devices this is accessible only to the app — acceptable for the threat model.

**Documented threat model assumption:** The booking code is NOT a secret authentication token in the traditional sense. Anyone physically possessing the code (printed receipt, screenshot) can present it for check-in. This is equivalent to a movie ticket. Loss of the device means loss of the code display, but the code was also sent via email — the customer can retrieve it there. Storing codes in `shared_preferences` (rather than `flutter_secure_storage`) is acceptable because the codes have no standalone value beyond confirming a booking to ops staff who are co-located with the customer.

**Caveat:** If Phase 6 adds customer login and codes become linked to a permanent customer identity, revisit this assessment and migrate to `flutter_secure_storage`.

**Blocks Phase 5 coding:** No — Info/acceptance record.

---

#### L-4: HTTPS enforcement — Android `cleartext` and iOS ATS

**Severity:** Low

**Location:** Flutter app Android manifest + iOS ATS config

**Required actions for `flutter-expert`:**

Android `AndroidManifest.xml` production build:
```xml
<application android:usesCleartextTraffic="false" ...>
```
The `dev` flavor targeting `10.0.2.2` (emulator) may use a separate `network_security_config.xml` that allows cleartext to localhost only. Do NOT set `usesCleartextTraffic="false"` globally in the dev flavor or emulator testing breaks.

iOS: Ensure `Info.plist` does NOT contain `NSAllowsArbitraryLoads=true` in the production build. App Transport Security (ATS) defaults to HTTPS-only on iOS 9+; the risk is that a developer adds this key during debugging and it ships in production.

**Build flavor strategy:** The `prod` flavor (ADR 0014 §3.18) must explicitly set `usesCleartextTraffic="false"` for Android. Add this to the `flutter-expert` implementation checklist.

**Blocks Phase 5 coding:** No — implementation checklist item.

---

### Section G — Remaining Phase 4 Open Findings (Carry-forward)

The following Phase 4 findings remain open and MUST be resolved before Phase 5 ops-portal ships (they affect the same code paths Phase 5 extends):

| Finding | Phase | Severity | Status | Required action |
|---|---|---|---|---|
| M-2: `therapist`-role user can overwrite colleague availability | Phase 4 | Medium | Open | `go-expert`: restrict availability write to own therapist record or remove role grant |
| M-3 / Addon M-1: Cursor subquery implicit RLS dependency | Phase 4 | Medium | Open | `go-expert`: add explicit `AND tenant_id = ?` to all cursor subqueries |
| L-1: `therapist.user_id` same-tenant membership check missing | Phase 4 | Low (escalates to High in Phase 5) | Open | `go-expert`: add `userRepo.FindByIDInTenant` check when `user_id != nil` |

The `therapist.user_id` finding from Phase 4 Section 5 is escalated: Phase 5 uses `user_id` for the ops portal therapist-schedule view. A dangling cross-tenant `user_id` would allow tenant A's therapist to appear in tenant B's schedule view. **Resolve before Phase 5 ops portal ships.**

---

### RLS Coverage Assessment — Phase 5 New Tables

| Table | SELECT | INSERT | UPDATE | DELETE | lustia_app grants | Assessment |
|---|---|---|---|---|---|---|
| `booking` | `booking_tenant_select` + `booking_public_select` | `booking_tenant_insert` | `booking_tenant_update` | No policy, no grant | SELECT, INSERT, UPDATE | Complete. Public SELECT is correct but requires service-layer `WHERE code=?` predicate (H-7). INSERT under `__public__` is correctly blocked — only resolved `tenant_uuid` may INSERT. |
| `booking_addon` | `booking_addon_tenant_select` + `booking_addon_public_select` | `booking_addon_tenant_insert` | No policy, no grant | No policy, no grant | SELECT, INSERT | Complete. Add-ons are immutable after booking creation (INSERT-only). |

---

### Production Readiness Checklist Additions (Section 9.4 updates)

Add these items to Section 9.4:

- [ ] `PAYMENT_ADAPTER=dummy` must fail-fast at startup when `APP_ENV != local|dev` (C-2).
- [ ] Midtrans server key loaded from secret manager (not `.env`) before real integration.
- [ ] WAF/ingress allowlist for `POST /public/payments/webhook` to Midtrans IP ranges (M-4).
- [ ] `cube` and `earthdistance` PostgreSQL extensions enabled in staging and production DB (L-2).
- [ ] Flutter production build: `android:usesCleartextTraffic="false"` in production manifest (L-4).
- [ ] CAPTCHA (hCaptcha or Cloudflare Turnstile) added to `POST /public/bookings` before public launch (H-2).
- [ ] Content moderation for customer-entered text fields (name, cancel reason) — Phase 6 scope; acceptable for Phase 5 closed beta.
- [ ] `booking.customer_phone` and `booking.customer_email` redacted from application logs (extend Section 7.2 rules).
- [ ] Booking confirmation email: confirm that the booking `code` is NOT logged to any structured log system during email send.

---

### Phase 5 Threat Model Sign-off

**Decision: BLOCKED — 2 Critical and 6 High findings must be resolved (as ADR updates or explicit design constraints acknowledged by `go-expert`) before booking service code is written.**

**Must-do checklist for `go-expert` before writing booking code:**

| Priority | Item | Why blocking |
|---|---|---|
| 1 | C-1: Confirm `CreateBookingRequest` DTO has NO `total_price_idr` field. Server computes total from DB-fetched prices only. | Revenue integrity — attacker can set price to IDR 1 |
| 2 | C-2: Implement `PAYMENT_ADAPTER` env gate with `log.Fatal` when dummy in non-local env. Add build tag `//go:build dev` to dummy package. | Any prod/staging deploy with dummy adapter = free bookings for attackers |
| 3 | H-1: Validate every FK in `POST /public/bookings` against resolved `tenant_id`. Cross-tenant UUID → 404 (not 409/422). | Cross-tenant data pollution and service disclosure |
| 4 | H-2: Register rate limit middleware on all `/public/*` routes before any handler. Thresholds as per H-2 table. | Public endpoint is unauthenticated DoS vector |
| 5 | H-3: Real webhook adapter uses `subtle.ConstantTimeCompare` for Midtrans SHA-512 signature verification before any DB read. | Webhook forgery → free paid bookings |
| 6 | H-4: Webhook handler compares `gross_amount` to `booking.total_price_idr`; underpayment → log + alert, return 200, do NOT mark paid. | Revenue integrity |
| 7 | H-5: `TransitionStatus` uses conditional UPDATE `WHERE status = 'pending_payment'`; webhook handler detects `rowsAffected == 0` and distinguishes `expired` vs `paid` cases. | Customer pays at T=14m59s, booking expires, money taken but booking lost |
| 8 | H-6: Concierge booking validates `branch_id` against caller's `claims.Branches`; all resource FKs (`room_id`, `therapist_id`) validated against `branch_id`. | Cross-branch resource theft by ops staff |
| 9 | H-7: `FindByCodePublic` repository method requires `code string` parameter; no unbounded public booking query is possible. | Public policy without predicate exposes all booking rows |

**Medium findings (M-1 through M-6) are NOT blocking code start.** They are Code Review territory — `go-expert` must address them in the booking service implementation or note them explicitly in the PR for the review gate.

| Date | Change reviewed | Findings | Status |
|---|---|---|---|
| 2026-04-25 | Phase 5 — Booking Engine + Customer Mobile App threat model (pre-implementation, ADR 0014 + migration 000025) | **[Critical — BLOCKING]** C-1: `total_price_idr` must never be trusted from request body. **[Critical — BLOCKING]** C-2: Dummy payment adapter must be hard-blocked in non-local environments via startup fail-fast + build tag. **[High — BLOCKING]** H-1: Cross-tenant FK validation on POST /public/bookings. **[High — BLOCKING]** H-2: Rate limits on all `/public/*` endpoints — concrete thresholds required before handler registration. **[High — BLOCKING]** H-3: Webhook signature verification — `subtle.ConstantTimeCompare` on Midtrans SHA-512. **[High — BLOCKING]** H-4: Webhook underpayment check — `gross_amount < total_price_idr` must not mark paid. **[High — BLOCKING]** H-5: Expiry-sweep race — conditional UPDATE `WHERE status='pending_payment'` with `rowsAffected` detection. **[High — BLOCKING]** H-6: Concierge booking branch-scope FK validation. **[High — BLOCKING]** H-7: `booking_public_select` RLS overly broad — service-layer `WHERE code=?` contract mandatory. **[Medium]** M-1: Booking code brute-force feasibility — rate limit sufficient, no ADR change needed. **[Medium]** M-2: PII masking in `GET /public/bookings/:code` response. **[Medium]** M-3: Lazy expiry sweep must trigger on `POST /public/bookings` before INSERT. **[Medium]** M-4: Webhook source IP allowlist — production operational control for `devops-expert`. **[Medium]** M-5: Webhook idempotency — `paid→paid` must be no-op. **[Medium]** M-6: No-cancel dispute path — contact channel must be surfaced in app. **[Low]** L-1: Lat/lng service-layer validation before DB constraint. **[Low]** L-2: `earthdistance` extension availability — production checklist item. **[Low]** L-3: `shared_preferences` for booking codes — documented acceptable risk. **[Low]** L-4: Android `usesCleartextTraffic=false` in prod build. | **BLOCKED — 2 Critical + 6 High must be resolved before booking code is written** |
