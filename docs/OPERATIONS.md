# Operations

_Owned by `devops-expert`. Covers how the app is built, shipped, and run._

## Runtime topology
_One line per service: name, replicas, resources, what it talks to._

## Environments
| Env | URL | Deploys from | Approval | Data |
| --- | --- | --- | --- | --- |
| dev (local) |  | laptop |  | seed |
| staging |  | `main` on merge | auto |  |
| production |  | tagged release | manual | real |

## Build & release

### Image build
- Base image:
- Final image:
- Signed with:

### CI pipeline (GitHub Actions)
_Stage order: lint → test → build → scan → push → deploy._

### Release strategy
- Default:
- DB migration strategy:

### Rollback
_How to roll back in under 5 minutes._

## Secrets inventory
| Secret | Where stored | Consumer | Rotation |
| --- | --- | --- | --- |

## Observability
- Logs:
- Metrics:
- Traces:
- Errors:
- Dashboards:
- Alerts:

## Runbooks
- [ ] Service unhealthy
- [ ] Database unreachable
- [ ] Bad deploy rollback
- [ ] Cert expiring

## Backups & DR
- What is backed up:
- Retention:
- Last tested restore:
- RTO / RPO:

## Cost
- Tags enforced: `app`, `env`, `owner`, `cost-center`
- Budgets + alerts:

## Open questions
-
