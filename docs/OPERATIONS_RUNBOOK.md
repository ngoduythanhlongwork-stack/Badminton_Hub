# Backend operations runbook

Status: pilot-ready baseline, 2026-10-07. Production provider credentials are intentionally not stored in this repository.

## Release gate

Every change to `main` must pass `.github/workflows/ci.yml`: format, vet, unit tests, PostgreSQL/Redis
integration tests, migration replay, D-24 load test, build, container build and the web lint/typecheck/build baseline.
Apply `migrate` as a serialized release job before starting the new API image. Never run multiple migration
jobs concurrently. The API does not auto-migrate.

Production must set:

- `APP_ENV=production`;
- an HTTPS `PUBLIC_BASE_URL` without a path;
- PostgreSQL TLS (`sslmode=require`, `verify-ca`, or `verify-full`);
- an independent random `IDENTITY_TOKEN_SECRET` from the deployment secret manager;
- private network endpoints for PostgreSQL, Redis and `/metrics` scraping.

The process rejects obvious production placeholders, plaintext public origins and `sslmode=disable`.

## Health, metrics and alerts

- `GET /health`: process liveness only.
- `GET /ready`: PostgreSQL/Redis dependency readiness.
- `GET /metrics`: Prometheus counters, in-flight gauge and request-duration histogram. It contains no route,
  account, request body or other high-cardinality/PII labels.
- Import [alerts.yml](../infra/alerts.yml) into the chosen Prometheus-compatible alert manager.

Keep `/ready` and `/metrics` private to the cluster or monitoring network. The baseline alerts cover API down,
5xx rate above 5%, and p95 above one second for ten minutes. Add outbox backlog, PostgreSQL saturation and
disk alerts in the selected hosting platform.

## Backup and restore drill

Create a PostgreSQL custom-format backup from the local Compose database:

```powershell
./infra/scripts/backup-postgres.ps1 -OutputDirectory D:\Backups\BadmintonHub
```

Restore is destructive and requires an explicit switch:

```powershell
./infra/scripts/restore-postgres.ps1 `
  -BackupPath D:\Backups\BadmintonHub\badminton_hub-YYYYMMDDTHHMMSSZ.dump `
  -ConfirmDatabaseReset
```

For production, configure provider-managed encrypted backups, point-in-time recovery and retention before
launch. Run a restore drill into a separate database, verify migration manifest/checksums and execute the R3
E2E smoke journey. A backup is not accepted until a restore drill succeeds.

The existing local Docker volume may report migration checksum drift because it predates the finalized source
manifest. Preserve any needed development data with the backup script first. Recreating it with
`docker compose -f infra/compose.yaml down -v` deletes PostgreSQL and Redis volumes; only the operator may
authorize that action.

## Provider handoff

The domain already depends on ports rather than vendor SDKs:

- Identity and Notifications use their `EmailSender` contracts.
- Moderation stores an evidence reference; `platform/blob.Store` is the object-storage port.
- `platform/geocoding.Provider` is reserved for a deliberately approved distance-search increment. The current
  MVP remains area-based.

Blob and geocoding default adapters fail closed. Select providers, create least-privilege service identities,
configure lifecycle/retention and inject credentials through the deployment secret manager. Do not add API keys
to `.env.example`, GitHub variables visible to builds, logs or analytics.

## Staging performance validation

Run against an isolated PostgreSQL role with `CREATEDB`:

```powershell
cd src/backend
$env:TEST_DATABASE_URL="postgres://.../postgres?sslmode=require"
go test -tags=integration ./cmd/api -run TestD24RecommendationPilotLoad -count=1 -v
```

The harness creates 500 accounts, 10,000 historical matches and 1,000 active candidates, warms 50 concurrent
users, then requires recommendation p95 below one second. Record image digest, database size/class, region,
connection-pool settings, candidate p95 and end-to-end p95 with the release evidence.
