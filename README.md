# Badminton Hub

Badminton Hub is a mobile-first platform that helps badminton players find the right game, with the right players, at the right time.

> Product promise: **Muốn đánh cầu — luôn có kèo phù hợp.**

The MVP is a matchmaking product. Court discovery supports that core loop; Hosts
arrange courts outside the app and receive Player transfers directly. Coach, club,
tournament, full social and video-analysis capabilities remain later-phase verticals.

## Source of truth

Read these documents before changing product behavior:

1. [`docs/PRODUCT_BLUEPRINT.md`](docs/PRODUCT_BLUEPRINT.md) — vision, users, value proposition, product loop, and phase boundaries.
2. [`docs/PRD.md`](docs/PRD.md) — MVP requirements, rules, acceptance criteria, and out-of-scope list.
3. [`docs/ROADMAP.md`](docs/ROADMAP.md) — delivery sequence and validation gates.
4. [`docs/ARCHITECTURE_DECISIONS.md`](docs/ARCHITECTURE_DECISIONS.md) — technical decisions and constraints.
5. [`AGENTS.md`](AGENTS.md) — durable instructions for Codex and contributors.
6. [`docs/specs/README.md`](docs/specs/README.md) — PO-approved business specifications, decisions and acceptance traceability.
7. [`docs/BACKEND_IMPLEMENTATION_PLAN.md`](docs/BACKEND_IMPLEMENTATION_PLAN.md) — backend vertical slices, technical gates and Definition of Done.

If documents conflict, use this order: `PRD` for MVP behavior, `PRODUCT_BLUEPRINT` for product intent, then `ROADMAP`. Record any intentional change as a new architecture/product decision; do not silently widen scope.

## Repository layout

```text
src/
  backend/        Go modular monolith (cmd/api, internal/modules)
  web/            Next.js App Router, mobile-first PWA
tests/
  README.md       test strategy; Go tests live beside their packages
docs/             product and architecture source of truth
infra/            local infrastructure
```

## Local prerequisites

- Go 1.26 or newer ([official downloads](https://go.dev/dl/))
- Node.js 20.9 or newer
- npm 10 or newer
- Docker Desktop (for PostgreSQL and Redis)

## Getting started

```powershell
docker compose -f infra/compose.yaml up -d
cd src/backend
$env:DATABASE_URL="postgres://badminton_hub:local_only_change_me@localhost:55432/badminton_hub?sslmode=disable"
$env:REDIS_URL="redis://localhost:6379/0"
$env:IDENTITY_TOKEN_SECRET="local_only_change_me_identity_token_secret_32_bytes"
go run ./cmd/api
```

In a second terminal:

```powershell
cd src/web
npm install
npm run dev
```

Default local URLs:

- Web: `http://localhost:3000`
- API health: `http://localhost:5080/health`
- PostgreSQL: `localhost:55432` (container port `5432`)
- Redis: `localhost:6379`

The Go API reads process environment variables, not `.env` files automatically.
For example, set `$env:HTTP_ADDR="127.0.0.1:5080"` before starting the API.
Set DATABASE_URL and REDIS_URL using the local values above or your own services.
The API connects to PostgreSQL and Redis; `GET /ready` checks their availability.
See [backend setup and verification](src/backend/README.md). Never commit credentials.

## Current status

Business specification 1.0 is PO-approved. Backend R2 is verified through `/api/v1`: the R1
identity/onboarding/venue/free-join journey plus paid holds, direct-transfer reporting and Host
acknowledgement, cancellation/refund tracking, in-app/email notification obligations and reminders.
The next milestone is R3 attendance, review, reliability and deterministic recommendations.
