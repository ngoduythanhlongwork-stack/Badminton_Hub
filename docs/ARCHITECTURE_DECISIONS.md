# Architecture Decisions

This file is the initial decision log. Decisions marked **Accepted** govern implementation; unresolved choices are explicit to prevent accidental coupling.

## ADR-001 — Modular monolith first

Status: **Accepted**

Use one Go deployable with strongly bounded modules. This minimizes operational complexity during product discovery while preserving seams for later extraction. A module owns its domain model, application workflows, persistence mapping, and public contracts.

Initial modules:

- Identity
- Players
- Venues
- Matches
- Payments
- Notifications
- Communication
- Recommendations
- Moderation

Do not split into microservices based on anticipated scale. Extraction requires measured load, independent deployment needs, or clear team ownership.

## ADR-002 — Platform baseline

Status: **Accepted**

- Backend: Go 1.26+ with standard-library HTTP routing (see ADR-011).
- Web: Next.js 16 App Router with TypeScript, mobile-first PWA.
- Primary database: PostgreSQL.
- Cache/ephemeral coordination: Redis.
- Local infrastructure: Docker Compose.

Install Go to run backend verification. Node.js must satisfy Next.js 16's minimum requirement.

## ADR-003 — Data ownership and integration

Status: **Accepted**

Start with one PostgreSQL database and separate schemas per module. Modules do not query another module's tables. Cross-module reads use explicit contracts/read models; state changes publish in-process domain/application events. Use an outbox when delivery must survive a process failure.

Avoid a generic shared domain library. Building blocks may contain only technical primitives with broad, stable meaning.

## ADR-004 — PostgreSQL is authoritative; Redis is disposable

Status: **Accepted**

Redis may serve cache, rate limiting, short-lived locks, or transient coordination. Correctness must survive Redis loss. Never make Redis the only record of a join, payment, attendance, rating, or notification obligation.

## ADR-005 — Concurrency and idempotency are domain requirements

Status: **Accepted**

Match joins, capacity changes, payment callbacks, cancellations, and refunds require idempotency keys and database-level concurrency protection. Application-only “check then write” logic is insufficient.

Use optimistic concurrency and/or transactional constraints appropriate to the final persistence design. Add integration tests that race competing joins.

## ADR-006 — Recommendation V1 is deterministic and explainable

Status: **Accepted**

Apply hard constraints, then weighted scoring for skill, distance, schedule, preferences, and reliability. Persist/configure scoring versions and return reason codes. An LLM may parse natural language to a validated DTO but may not access the database directly or decide authorization/eligibility.

## ADR-007 — Permission-based authorization

Status: **Accepted**

Use explicit permission policies in Go application services and HTTP middleware. Player and Host are contextual capabilities; Host derives from match ownership. Admin and Court Manager permissions are scoped and auditable.

## ADR-008 — Time, money, and location

Status: **Accepted**

- Store event instants in UTC and retain venue IANA time zone.
- Store money as integer minor units plus ISO currency code.
- Store geographic coordinates using PostgreSQL/PostGIS when distance queries enter implementation; do not calculate large candidate sets in application memory.

## ADR-009 — PWA integrity boundaries

Status: **Accepted**

Cache only safe, non-authoritative experiences. Join, payment, cancellation, capacity, and check-in always require server confirmation. The UI must clearly distinguish queued/offline input from confirmed state.

## ADR-010 — Observability and auditability from the first slice

Status: **Accepted**

Use structured logs, correlation/trace identifiers, health checks, metrics for primary journeys, and durable audit events for sensitive operations. Do not log credentials, tokens, payment secrets, exact unnecessary location, or private chat content.

## ADR-011 — Go backend foundation

Status: **Accepted — 2026-09-28, user-directed platform change**

Replace the initial ASP.NET Core/.NET skeleton with Go. This supersedes the
original backend language choices in ADR-001, ADR-002 and ADR-007. Product scope,
module ownership, PostgreSQL authority and Redis disposability remain unchanged.

Use one Go module under `src/backend`, one `cmd/api` executable, standard-library
`net/http` routing and `log/slog` structured logging. Wire dependencies explicitly
at the executable/HTTP composition boundary. Public module contracts live at
`internal/modules/<name>`; private implementations belong in each module's own
`internal/` packages so the compiler restricts cross-module imports.

The initial health endpoint checks process liveness only. Persistence adapters,
migrations, authentication, dependency readiness and product workflows are added
with tested vertical slices. Do not mistake this bootstrap for completion of Stage 0.
Use a TLS reverse proxy for production traffic.

## ADR-012 — Connection foundation before database design

Status: **Accepted — 2026-09-28**

Use pgxpool for PostgreSQL and go-redis/v9 for Redis. Versions and checksums are
recorded in go.mod/go.sum. Own clients in the executable and close them after HTTP
shutdown. Connection helpers are technical infrastructure, not shared repositories.
Future modules receive only the dependencies they need and own their SQL.

DATABASE_URL and REDIS_URL are required process environment variables. Use
rediss:// and PostgreSQL sslmode=verify-full for TLS-enabled deployments; the
sample credentials and disabled TLS are strictly for local Compose.

PostgreSQL is required at startup and for readiness. Redis is pinged on startup
but loss only produces a degraded state. GET /health is dependency-independent
liveness. GET /ready returns 503 for PostgreSQL failure and 200/degraded for Redis
failure. Checks have bounded deadlines and do not expose driver errors/credentials.

This step creates no business tables, ERD or migrations. Review the database
design next, then implement business slices.

## ADR-013 — Built-in Identity and delivery ports

Status: **Accepted — 2026-10-04**

The pilot uses a built-in Identity module rather than an external identity provider. Identity will
issue opaque access tokens valid for 15 minutes and rotating refresh-token families valid for at
most 30 days. Only cryptographic token hashes are stored. Refresh reuse revokes the token family.
The exact credential and session tables belong to Identity and are deferred to Slice 1.

Password hashing is behind an Identity-owned replaceable interface; the first adapter will use a
current memory-hard password hash with parameters recorded alongside the hash. Email delivery is
also a port. Development and tests use a local capture adapter; a production provider must be
selected and operationally verified before the pilot. Neither adapter may expose credentials or
tokens in logs. This ADR selects the direction and contract only; Slice 0 adds no auth behavior.

## ADR-014 — Versioned HTTP API contract

Status: **Accepted — 2026-10-04**

Business endpoints live under `/api/v1`. `/health` and `/ready` remain unversioned operational
endpoints. JSON failures use a stable envelope with `error.code`, a safe human-readable `message`,
`requestId`, and optional structured `details`. Error codes are API contract; internal/driver error
text is not. Every response carries `X-Request-ID`; a syntactically safe caller value is preserved
and an invalid or absent value is replaced. Request bodies are bounded before decoding, panics are
recovered, and access logs contain allowlisted request metadata only.

List endpoints will use a bounded `limit` plus opaque `cursor`; filters use explicit query fields.
Commands whose retry could duplicate an effect require `Idempotency-Key` and store the result at
the transaction boundary owned by their module. These endpoint-specific stores are added with the
owning slice, not as a generic repository in Slice 0.

## ADR-015 — Module-owned forward SQL migrations

Status: **Accepted — 2026-10-04**

Use embedded, forward-only SQL migrations, one globally ordered manifest, and `cmd/migrate` as a
separate deployment step. Every migration declares an owner and its SQL lives under that module's
`migrations/` directory; platform owns only technical tables in schema `platform`. Business
modules own same-named PostgreSQL schemas. A PostgreSQL advisory lock serializes runners, each
migration is transactional, and applied checksums prevent silent history edits. Running `up`
again is a no-op.

Each module exposes its own migration slice. The executable composition root supplies those slices
to the runner, which merges them with platform migrations, validates metadata and rejects duplicate
global versions before touching PostgreSQL. Platform therefore never imports business modules.

Production recovery uses database backup/restore or a reviewed corrective forward migration;
automatic down migrations are avoided because destructive rollback is unsafe after application
writes. Integration tests create a database per test from `TEST_DATABASE_URL`; the role must have
`CREATEDB`. Tests never share business schemas or truncate another test's data.

## ADR-016 — PostgreSQL transactional outbox and in-process workers

Status: **Accepted — 2026-10-04**

Durable asynchronous work uses the PostgreSQL `platform.outbox_messages` table and workers in the
same deployable. Producers insert a message using the business `pgx.Tx`, so state and delivery
obligation commit atomically. `(topic, idempotency_key)` is unique. Workers claim bounded batches
with `FOR UPDATE SKIP LOCKED`, a lease token and expiry; completion/retry is conditional on that
lease. A crash may cause redelivery, so every handler must be idempotent using message ID or its
domain idempotency key. Redis and an external broker are not correctness dependencies.

The API owns worker startup and cancellation and stops workers during graceful shutdown. Retry is
bounded exponential backoff; domain-specific attempt limits/dead-letter operations are added with
the first real notification/job workflow, when their operational requirements are known.

## ADR-017 — Independent paid-join lifecycles and transactional notification

Status: **Accepted — 2026-10-05**

Matches owns participation, capacity, schedule and time-bounded holds. Payments owns fee snapshots,
transfer reports, Host receipts and refund obligations. A transfer report never changes participation;
after an acknowledged deposit, Matches rechecks the hold, match, capacity and schedule. Late money is
retained by Payments and creates a refund obligation without restoring a slot. Cross-module effects use
idempotent outbox events and explicit root-package contracts; neither module reads the other's schema.

Notifications persists one obligation per source event/version, recipient, channel and purpose. In-app
and email delivery state remain separate from business state. Email delivery is abandoned after three
failed attempts, while the underlying join/payment/cancellation remains committed. The API also owns a
PostgreSQL-backed maintenance loop for hold expiry, 48-hour refund overdue marking and due reminder
dispatch. Redis is not involved in these correctness decisions.

## Open decisions

1. Production email delivery provider (the port and local adapter direction are decided).
2. Direct-to-Host transfers are chosen for the pilot; provider/settlement integration is future scope.
3. Map/geocoding provider and cost/privacy constraints.
4. Production hosting and object storage.
