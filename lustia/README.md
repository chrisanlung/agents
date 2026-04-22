# Lustia

Lustia is a multi-tenant B2B platform for special-therapist businesses — spa,
wellness, and therapy clinics. Each tenant (company) runs one or more
branches, each with therapists, services, schedules, customers, bookings,
invoices, and payments. This repo contains the shared PostgreSQL schema and
the Go microservices that back it. For the product scope, see
[`../docs/PRD.md`](../docs/PRD.md); for the architecture,
[`../docs/ARCHITECTURE.md`](../docs/ARCHITECTURE.md).

## What is in this repo

- **Phase 1** — foundation database schema (`migrations/`), `golang-migrate`
  compatible, covers every operational table the platform will ever need.
- **Phase 2** — `auth-service` (`services/auth/`): login, refresh, logout,
  password management, admin user CRUD, JWKS. Gin + GORM, Clean Architecture,
  RS256 JWT, Argon2id passwords, row-level security enforced at the DB.
- **Local dev stack** — `deploy/` brings up Postgres + migrator + auth with
  one command.

Later phases (tenant / master / booking / billing services, web + mobile
clients) are not in this repo yet. See `../docs/ARCHITECTURE.md` for the
roadmap.

## Tree

```
lustia/
├── README.md                    ← you are here
├── .gitignore
├── migrations/                  ← 000001 … 000005, shared by every service
├── services/
│   └── auth/                    ← Phase 2 auth-service
│       ├── cmd/auth/main.go
│       ├── internal/{domain,usecase,port,adapter}/…
│       ├── infra/               ← clock, hasher, JWT, rate limiter
│       ├── config/app.yaml
│       ├── Dockerfile
│       └── go.mod
└── deploy/
    ├── README.md                ← how to run it locally
    ├── docker-compose.yml
    ├── .env.example
    ├── init-db/                 ← first-boot Postgres init scripts
    └── secrets/                 ← dev JWT keys (gitignored)
```

## Running it

See [`deploy/README.md`](deploy/README.md) for the one-command quick start,
the JWT-key generation recipe, and the super-admin bootstrap steps.

Operational detail (CI shape, rollback, secrets inventory, runbooks) lives in
[`../docs/OPERATIONS.md`](../docs/OPERATIONS.md).
