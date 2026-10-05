# Go backend

One Go module and one deployable API. Requires Go 1.26 or newer.
PostgreSQL uses pgxpool; Redis uses go-redis/v9. Dependencies are pinned in go.mod/go.sum.

## Local setup

From repository root, start the dedicated local infrastructure:

```powershell
docker compose -f infra/compose.yaml up -d --wait
cd src/backend
$env:DATABASE_URL="postgres://badminton_hub:local_only_change_me@localhost:55432/badminton_hub?sslmode=disable"
$env:REDIS_URL="redis://localhost:6379/0"
$env:IDENTITY_TOKEN_SECRET="local_only_change_me_identity_token_secret_32_bytes"
go run ./cmd/api
```

Apply migrations before starting a new API version:

```powershell
go run ./cmd/migrate
```

The migration command is intentionally separate from API startup so deploys can serialize and
observe schema changes before new code serves traffic. Migrations are forward-only and checksum
protected. Recover a failed production rollout with a reviewed corrective migration or database
restore; never edit an applied migration.

The binary reads process environment, not .env files automatically. The root
.env.example documents settings. Never commit actual deployment credentials.

| Setting | Meaning |
| --- | --- |
| HTTP_ADDR | Listen address; default 127.0.0.1:5080 |
| DATABASE_URL | Required PostgreSQL URL |
| TEST_DATABASE_URL | Integration-test admin URL; role must have CREATEDB |
| REDIS_URL | Required redis:// or rediss:// URL; supports credentials and database index |
| IDENTITY_TOKEN_SECRET | Required 32+ byte secret used to derive one-time email tokens without storing bearer values |
| DEPENDENCY_TIMEOUT | Startup ping/readiness timeout; default 2s, greater than 0 and at most 30s |

Local Compose binds PostgreSQL/Redis to loopback and preserves named data volumes.
Use PostgreSQL sslmode=verify-full and Redis rediss:// for TLS deployments.
The local example uses disabled PostgreSQL TLS and a development-only password.

## Lifecycle and health

- GET /: service metadata.
- GET /health: process liveness; never queries dependencies.
- GET /ready: PostgreSQL and Redis checks within one shared timeout.
  PostgreSQL failure gives HTTP 503/unavailable; Redis-only failure gives
  HTTP 200/degraded; both available gives HTTP 200/ok.
- PostgreSQL must respond before API startup. Redis outage emits a warning but
  allows startup because Redis is disposable acceleration.
- Pools are reused across requests and closed after HTTP shutdown.
- The PostgreSQL outbox worker starts with the API and stops during graceful shutdown. Apply
  migrations first; Redis is never used as the durable job source.
- Driver errors/connection strings are not exposed in health responses or startup logs.
- HTTP timeouts and bounded graceful shutdown are configured.
- Use a TLS reverse proxy for production HTTPS.

Redis recovery is checked on subsequent readiness calls; clients remain reusable.
Future cache consumers must tolerate Redis errors and preserve correctness in PostgreSQL.

## Verification

From this directory:

```powershell
go fmt ./...
go vet ./...
go test ./...
go build ./...
go test -tags=integration ./... -count=1
```

Connection integration tests require explicit `DATABASE_URL` and `REDIS_URL`. Migration and outbox
tests require `TEST_DATABASE_URL`; they create and drop a unique database for each test, and leave
the configured database untouched.
Unit tests require no running services and cover readiness failures/deadlines,
liveness independence and invalid configuration.

## R2 API and operator bootstrap

Business routes live under `/api/v1`: authentication and recovery, owner onboarding,
public player projection, organizer review, venue management/discovery, match draft/publish/search,
free/paid instant or approval join, payment ledger, cancellation/refund and notification inbox.
Commands that may duplicate effects require `Idempotency-Key`.

Paid matches keep participation, hold and money states independent. A Player transfer report does
not confirm receipt or joining. Hosts acknowledge actual receipts; Matches then rechecks the live
hold, capacity and schedule. Payment instructions/evidence are returned only to the payer, payee or
an authorized future case workflow. Refunds remain open until the Player confirms or disputes them.

The API maintenance loop expires holds, marks 48-hour refunds overdue and dispatches due 24h/2h
reminders. PostgreSQL remains authoritative when Redis is unavailable. Local development uses an
in-memory capture adapter for notification email; select and verify a production provider before pilot.

Verification/reset delivery uses the PostgreSQL outbox. The database stores only token hashes and
non-secret delivery metadata; the worker derives the bearer from `IDENTITY_TOKEN_SECRET` only while
calling the email adapter. Development uses an in-memory capture adapter. Select and verify a
production email provider before pilot deployment.

After the first account is verified, an operator can grant the initial audited Admin permissions
exactly once:

```powershell
$env:ADMIN_ACCOUNT_ID="<verified-account-id>"
$env:ADMIN_BOOTSTRAP_RATIONALE="Initial pilot operator"
go run ./cmd/bootstrap-admin
```

## Layout and next step

- cmd/api: lifecycle and dependency composition.
- internal/config: environment configuration validation.
- internal/platform/connections: shared connection plumbing only.
- internal/platform/migrations: immutable deployment manifest and transactional runner.
- internal/platform/outbox: transactional enqueue and lease-based worker.
- internal/platform/testdb: database-per-test integration fixture.
- internal/httpapi: HTTP routing, liveness and readiness.
- internal/modules/<module>: bounded business modules.

R2 is closed. The next vertical slice is attendance, completion, review and reliability. Do not
add gateway/court-booking behavior or merge payment/refund state into participation.

Expose cross-module contracts from each module's root package. Put private code
under the module's own internal/ directory. Future modules own their SQL/migrations;
do not introduce a shared generic repository.

Each future module stores SQL below `internal/modules/<owner>/migrations`, exposes only root-level
contracts, and never queries another module's schema. Add its globally unique version to the
migration catalog; do not create a shared repository. Follow the
[backend implementation plan](../../docs/BACKEND_IMPLEMENTATION_PLAN.md) for vertical slices.
