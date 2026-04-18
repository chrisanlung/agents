# Test Plan

_Owned by `qa-expert`. Cross-layer strategy — unit tests stay with the specialist agents who wrote the code._

## Strategy

Pyramid shape (target):

| Layer | Tool(s) | Owner | Target count |
| --- | --- | --- | --- |
| Unit (backend) | `go test` | go-expert | ~70% of tests |
| Unit (web) | Jest / Vitest | nextjs-expert |  |
| Unit (mobile) | `flutter test` | flutter-expert |  |
| Integration | Testcontainers, real DB | go-expert + qa-expert | ~20% |
| Contract | schema / Pact | qa-expert |  |
| E2E (web) | Playwright | qa-expert | 5–10% |
| E2E (mobile) | Flutter `integration_test` | qa-expert |  |
| Load | k6 | qa-expert + devops-expert | a handful |

## Critical paths
_These are covered end-to-end and block release if red._

- [ ] Signup
- [ ] Login / logout
- [ ] _Primary value flow — fill after PRD_
- [ ] Password reset

## Coverage targets
- Domain / use cases: ≥ 80% line, mutation-tested on critical modules
- Adapters: ≥ 60%
- Framework glue: no target

## Test data
- Strategy: factories, per-test clean DB
- PII rule: synthetic only; never real user data

## Non-functional
- Accessibility: axe in E2E, fail on new violations
- Load target:
- Soak: _duration, env_
- Visual regression: _scope_

## Release readiness checklist
- [ ] All unit + integration + contract tests green on release branch
- [ ] Smoke E2E green against staging built from same commit
- [ ] Load test p95 within SLO
- [ ] No open Critical/High from `security-expert`
- [ ] Migrations tested on prod-sized copy
- [ ] Rollback plan documented + tested
- [ ] Observability verified for new surface area

## Flaky-test log
| Test | First seen | Root cause | Status |
| --- | --- | --- | --- |

## Open questions
-
