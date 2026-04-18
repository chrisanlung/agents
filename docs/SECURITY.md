# Security

_Owned by `security-expert`. Reviewed by every other agent against their work._

## Threat model (STRIDE)

| Threat | Where | Impact | Mitigation | Status |
| --- | --- | --- | --- | --- |
| Spoofing |  |  |  |  |
| Tampering |  |  |  |  |
| Repudiation |  |  |  |  |
| Information disclosure |  |  |  |  |
| Denial of service |  |  |  |  |
| Elevation of privilege |  |  |  |  |

## Authentication & session design
- Password hashing:
- Session / token strategy:
- MFA:
- Account recovery:

## Authorization model
- Model: _RBAC / ABAC / ReBAC_
- Central decision point:
- Resource-level checks:

## Cryptography choices
- TLS:
- Symmetric:
- Asymmetric:
- Randomness sources:
- Secret storage:

## Web platform hardening
- [ ] HSTS
- [ ] CSP (no `unsafe-inline`, nonces where needed)
- [ ] `X-Content-Type-Options: nosniff`
- [ ] `Referrer-Policy: strict-origin-when-cross-origin`
- [ ] `Permissions-Policy` minimized
- [ ] CORS allowlist (no `*` with credentials)
- [ ] CSRF strategy defined
- [ ] Rate limiting on auth + expensive endpoints

## Logging rules
- **Must log:** auth success/failure, authz denials, admin actions, data exports.
- **Must not log:** passwords, tokens, session IDs, full PII, `Authorization` headers.

## Supply chain
- [ ] Lockfiles committed
- [ ] Dependency scanning in CI
- [ ] SBOM generated for releases
- [ ] GitHub Actions pinned to SHAs

## Review log
_Each security review of a diff/PR appends an entry here._

| Date | Change reviewed | Findings (Crit/High/Med/Low/Info) | Status |
| --- | --- | --- | --- |

## Open questions
-
