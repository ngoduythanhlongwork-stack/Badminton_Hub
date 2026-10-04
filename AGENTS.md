# Badminton Hub contributor instructions

## Product guardrails

- The MVP's core value is badminton match discovery and matchmaking, not a generic court-booking app.
- Optimize the primary journey: discover -> evaluate -> join -> play -> rate -> receive better recommendations.
- Player and Host are capabilities of the same user account, not separate account types.
- Do not add Club, Coach Marketplace, Tournament, social feed, livestream, video analysis, advanced wallet, subscription, gamification, or complex league ranking to the MVP unless the PRD is deliberately revised.
- AI may parse natural-language search into a validated DTO. It must not query the database directly.
- Recommendation V1 is deterministic and explainable: hard constraints first, weighted scoring second.

## Architecture guardrails

- Keep one deployable Go modular monolith until measured scale or team ownership justifies extraction.
- Modules own their domain model and data access. Do not read another module's tables directly.
- Cross-module calls use explicit contracts/events. Avoid a shared-domain dumping ground.
- PostgreSQL is the source of truth. Redis is disposable acceleration for cache, rate limiting, and short-lived coordination.
- Enforce permissions through policies/permissions; do not scatter `IsAdmin` checks.
- Payment data uses lifecycle states and idempotency; never model payment as a single boolean.
- Store timestamps in UTC and keep the venue's IANA time-zone identifier for display and scheduling.
- Treat join capacity, schedule conflicts, payment confirmation, and cancellation as concurrent workflows. Protect invariants in the database/transaction boundary.

## Working agreement

- Read `docs/PRODUCT_BLUEPRINT.md`, `docs/PRD.md`, and relevant ADRs before implementation.
- Keep changes small and vertical: API, domain rule, persistence, UI, and tests for one user outcome.
- Add or update tests for every business rule.
- Update docs when behavior or an architectural decision changes.
- Prefer Vietnamese product copy and English code identifiers.
- Never commit secrets, personal data, generated build output, or local environment files.

## Verification

Run the narrowest relevant checks, then the full baseline before handoff:

```powershell
cd src/backend
go fmt ./...
go vet ./...
go test ./...
go build ./...
cd ../web
npm run lint
npm run typecheck
npm run build
```

## Agent scaffold compatibility

- The Ennam scaffold's original Claude Code files live under `.claude/`; do not assume Claude-only commands, hooks, or `superpowers:*` skills exist in Codex.
- Codex discovers repository skills from `.agents/skills/` and project custom agents from `.codex/agents/`.
- Project-specific instructions in this file and the product/architecture documents override generic scaffold guidance.
- Use the custom agents only when the user explicitly requests delegation or parallel agent work.
