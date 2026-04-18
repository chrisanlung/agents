# Architecture

_Status: **stub** — filled after PRD is agreed._

## Components in scope
- [ ] `/backend` — Go service (Gin + GORM, Clean Architecture)
- [ ] `/web` — Next.js App Router
- [ ] `/mobile` — Flutter

## Component diagram
```mermaid
flowchart LR
  web[Next.js web] -->|HTTPS/JSON| api[Go API]
  mobile[Flutter app] -->|HTTPS/JSON| api
  api -->|SQL| db[(PostgreSQL)]
```

## Deployment target
-

## Major third-party dependencies
-

## Cross-cutting concerns
- Auth:
- Observability:
- Config & secrets:

## Open questions
-
