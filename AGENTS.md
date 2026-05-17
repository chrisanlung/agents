# Project playbook — how the agents build this app together

This file is read automatically by Codex on every session. It is the single source of truth for **how the specialist agents collaborate** when the user asks to build or extend the app.

## The team

| Agent | Role | Owns | Must read before acting |
| --- | --- | --- | --- |
| `db-designer` | Data architect | `docs/DATA_MODEL.md` (ERD, tables, constraints) | `docs/PRD.md`, `docs/API_CONTRACT.md` |
| `go-expert` | Backend engineer (Gin + GORM, layered architecture + SOLID) | `docs/API_CONTRACT.md` (endpoints, DTOs) + `/backend/` code | `docs/PRD.md`, `docs/DATA_MODEL.md`, `docs/SECURITY.md` |
| `nextjs-expert` | Web frontend engineer | `/web/` code | `docs/PRD.md`, `docs/API_CONTRACT.md`, `docs/DESIGN_SYSTEM.md` |
| `flutter-expert` | Mobile engineer | `/mobile/` code | `docs/PRD.md`, `docs/API_CONTRACT.md`, `docs/DESIGN_SYSTEM.md` |
| `ui-ux-expert` | Designer | `docs/DESIGN_SYSTEM.md` (tokens, components, flows) | `docs/PRD.md` |
| `security-expert` | Security reviewer (can block) | `docs/SECURITY.md` (threat model, controls, review log) | everything — reviews all layers |
| `devops-expert` | Platform / infra engineer | `docs/OPERATIONS.md` + `Dockerfile`, `compose.yaml`, `.github/workflows/`, `k8s/`, `helm/`, `terraform/` | `docs/ARCHITECTURE.md`, `docs/SECURITY.md` |
| `qa-expert` | Quality / cross-layer test strategist (can block) | `docs/TEST_PLAN.md` + `/tests/` integration & E2E suites | `docs/PRD.md`, `docs/API_CONTRACT.md`, `docs/DATA_MODEL.md` |
| `code-reviewer` | Independent quality reviewer | nothing — produces review output | the diff + whichever docs the change should have updated |

No agent talks directly to another agent. They collaborate through the **shared documents in `/docs/`**. The orchestrator (the main Codex session that reads this file) hands work between agents in the right order.

**Blocking authority:** `security-expert` (Critical/High findings), `qa-expert` (critical-path E2E failures, S1/S2 bugs), and `code-reviewer` (Must-fix findings) can all block "done." The orchestrator does not mark work complete while any of these is open.

## Shared documents — contract

All planning lives in `/docs/`. Every agent reads the docs relevant to its role before touching code, and updates its owned doc whenever a decision is made. If a doc does not yet exist, the first agent that needs it creates a stub with an `## Open questions` section for the orchestrator to resolve.

- `docs/PRD.md` — **What** we are building. User stories, scope, non-goals, constraints. Owned by the orchestrator, informed by the user's command.
- `docs/ARCHITECTURE.md` — **Which** pieces exist and how they fit. High-level component diagram, repo layout, deployment target. Owned by the orchestrator.
- `docs/DATA_MODEL.md` — ER diagram (Mermaid `erDiagram`), `CREATE TABLE` statements, indexing plan, design-decision log. Owned by `db-designer`.
- `docs/API_CONTRACT.md` — Endpoint list, request/response DTOs, auth expectations, error codes. Written in OpenAPI-lite Markdown. Owned by `go-expert`, consumed by `nextjs-expert` and `flutter-expert`.
- `docs/DESIGN_SYSTEM.md` — Design tokens (spacing, color, typography, radius, motion), component inventory, key screen flows, accessibility baseline. Owned by `ui-ux-expert`.
- `docs/SECURITY.md` — STRIDE threat model, authN/authZ design, crypto choices, secret handling, OWASP Top 10 review log. Owned by `security-expert`.
- `docs/OPERATIONS.md` — Runtime topology, environments, release/rollback, observability, secrets inventory, runbooks. Owned by `devops-expert`.
- `docs/TEST_PLAN.md` — Cross-layer test strategy, critical paths, coverage targets, release readiness checklist, flaky-test log. Owned by `qa-expert`.
- `docs/DECISIONS/` — One Markdown file per non-obvious decision (ADR-style): context, options considered, decision, consequences.

## Build workflow (default)

When the user issues a "build X" command, the orchestrator runs this pipeline. Steps can be parallelized where marked.

1. **Discovery** — orchestrator turns the user's command into `docs/PRD.md` (user stories, scope, non-goals, open questions). Ask the user to clarify anything that would force a guess on scope, auth model, or target platforms.
2. **Architecture outline** — orchestrator drafts `docs/ARCHITECTURE.md`: which of `/backend`, `/web`, `/mobile` are in scope, deployment target, major third-party dependencies.
3. **Parallel design phase** *(spawn in one message):*
   - `db-designer` → `docs/DATA_MODEL.md`
   - `ui-ux-expert` → `docs/DESIGN_SYSTEM.md`
   - `security-expert` → initial `docs/SECURITY.md` threat model
   - `devops-expert` → initial `docs/OPERATIONS.md` (topology, environments, CI outline)
   - `qa-expert` → initial `docs/TEST_PLAN.md` (critical paths, pyramid shape)
4. **API contract** — `go-expert` drafts `docs/API_CONTRACT.md` from PRD + data model + security constraints. Frontend agents review and request changes via the orchestrator before any code is written.
5. **Parallel implementation phase** *(spawn in one message once contract is signed off):*
   - `go-expert` → `/backend/` scaffolding + first endpoints
   - `nextjs-expert` → `/web/` scaffolding + first screens
   - `flutter-expert` → `/mobile/` scaffolding + first screens
   - `devops-expert` → Dockerfiles, `compose.yaml`, first CI pipeline alongside the app scaffolding
6. **Review gate** — all three reviewers in parallel:
   - `code-reviewer` — quality/readability/correctness review of each layer's diff.
   - `security-expert` — OWASP Top 10 + threat-model review; append findings to `docs/SECURITY.md`.
   - `qa-expert` — integration + critical-path E2E tests; update `docs/TEST_PLAN.md`.
7. **Integration check** — orchestrator verifies the clients actually talk to the backend against the API contract, the app runs via `docs/OPERATIONS.md` instructions, and `qa-expert`'s smoke suite is green. No unresolved blocking findings from reviewers.

Small changes (single-feature, single-layer) skip Steps 1–2 and jump to the relevant agent, but **still update the owned doc** before or alongside the code change.

## Hand-off protocol

When the orchestrator delegates to an agent, the prompt must include:

1. **Goal** — one sentence of what this agent is expected to produce.
2. **Relevant docs** — explicit paths to read. The agent does not guess which docs apply.
3. **Authority** — what this agent may change on its own vs. what requires approval (e.g., "you may add tables; you may not rename existing columns without an ADR").
4. **Deliverable** — either a diff, or an updated doc, or a short review. Not both open-ended "do something useful".

Example: *"Read `docs/PRD.md` sections 2.1–2.3 and `docs/API_CONTRACT.md`. Produce a first-pass `docs/DATA_MODEL.md` covering the user, session, and order entities. You may introduce new tables; flag any PRD ambiguity in `## Open questions` instead of guessing."*

## Collaboration rules for every agent

- **Read before you write.** Open the docs listed in "Must read before acting" for your role. If something you need is missing, add it to your own doc's `## Open questions` — don't guess silently.
- **Update your owned doc in the same turn as the code change.** A code change without a doc update is incomplete. A doc change without flagging downstream impact is incomplete.
- **Flag cross-agent impact.** If your change affects another agent's domain (e.g., a new DB column that needs an API field and a UI control), call it out explicitly in your response so the orchestrator can dispatch the follow-up.
- **Write ADRs for non-obvious choices.** If you picked option B over option A and a future reader would reasonably question it, drop a file in `docs/DECISIONS/`. Keep it short: 10–20 lines.
- **Stay in your lane, but don't break the neighbor's fence.** A backend agent doesn't redesign UI; a UI agent doesn't invent API fields. Cross-lane needs go back through the orchestrator.

## Repo layout (target)

```
/backend/        # Go service — Gin + GORM, Clean Architecture (go-expert)
/web/            # Next.js App Router (nextjs-expert)
/mobile/         # Flutter (flutter-expert)
/tests/          # Cross-layer integration & E2E suites (qa-expert)
/.github/        # CI workflows (devops-expert)
/deploy/         # Dockerfiles, compose, k8s/helm, terraform (devops-expert)
/docs/           # Shared design docs — see contract above
/.Codex/agents/ # The agents themselves
AGENTS.md        # This file
```

Layers that are not yet in scope simply do not exist yet. Don't scaffold a layer until the PRD puts it in scope.

## What the orchestrator (main Codex) does *not* do

- Does not write domain code directly when a specialist agent exists for it. Delegates.
- Does not let two agents edit the same file in the same turn. Serialize or split the file.
- Does not accept an agent's claim that something works without verification. Trust but verify — read the diff, run the tests, open the dev server.
