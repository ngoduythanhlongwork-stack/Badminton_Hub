# Delivery Roadmap

This is an outcome-based six-month working plan. Calendar dates begin only after the team confirms capacity and pilot geography.

## Current priority — backend vertical slices (2026-10-04)

Business specification 1.0 is PO-approved; Q-01–Q-17 are closed by D-08–D-24.
Use the [spec index and backend handoff order](specs/README.md), [decision register](specs/DECISIONS.md)
and [traceability matrix](specs/TRACEABILITY.md). Start with Identity/onboarding and the free-match
discovery/join slice, then add paid holds before attendance/recommendations. Resolve the technical ADRs
needed by each slice without changing the approved business policy.

## Stage 0 — Foundation (Weeks 1–2)

The BA gate is complete. Design only the ERD/API needed for the active vertical slice,
then implement migrations and workflows with module ownership and concurrency tests.

Outcome: the team can build and verify one deployable modular monolith and one PWA.

- Confirm pilot city/district and map provider; email verification is chosen.
- Direct-to-Host transfers are chosen; payment-provider integration is deferred.
- Establish CI, local PostgreSQL/Redis, migrations, telemetry, secret handling, and environments.
- Create module boundary tests and a thin end-to-end health slice.
- Define analytics event vocabulary and privacy/retention baseline.

Exit gate: clean checkout can build/test; local stack starts predictably; ADRs cover unresolved vendor choices.

## Stage 1 — Identity and player readiness (Weeks 3–5)

Outcome: a player can register, complete onboarding, and own a badminton identity.

- E01 Authentication/authorization.
- E02 Onboarding.
- E03 Profile, initial level/rating confidence, preferences.
- Basic audit trail and moderation primitives.

Exit gate: tested account/session recovery and resumable onboarding; no critical security finding.

## Stage 2 — Match marketplace core (Weeks 6–10)

Outcome: a Host creates a match and a Player finds and joins it safely.

- E04 venue discovery baseline.
- E05 create match.
- E06 search/card/detail.
- E07 join with capacity and schedule-conflict protection.
- E08 host management.

Exit gate: concurrent join tests preserve capacity; primary journey succeeds end to end without payment.

## Stage 3 — Money and cancellation (Weeks 11–14)

Outcome: deposits/payments/refunds are traceable and resilient.

- E11 direct-transfer reports, Host acknowledgements and refund obligations.
- Idempotent manual confirmations, evidence and reconciliation/case handling.
- Versioned cancellation policy and refund decisions.

Exit gate: manual-payment scenarios pass, including repeated reports/acknowledgements,
missing/late transfers, full refunds, disputes and cancellation races. MVP has no partial refund tier.

## Stage 4 — Trust and operations (Weeks 15–18)

Outcome: real matches can be operated with reminders, attendance, and feedback.

- E09 check-in/no-show confirmation.
- E10 rating and reliability.
- E12 in-app/email notifications.
- E13 match room communication.
- Admin report queue and audit visibility.

Exit gate: a completed match produces trustworthy attendance/rating signals; abuse report flow is operable.

## Stage 5 — Recommendation and pilot (Weeks 19–24)

Outcome: the pilot validates the three product hypotheses.

- E14 deterministic recommendation V1 with explanations.
- Natural-language search deferred from the current specification baseline.
- Pilot onboarding and supply seeding.
- Funnel dashboard: fill, conversion, repeat, and no-show metrics.

Exit gate: review pilot evidence against working targets and make an explicit continue/pivot/stop decision before Phase 2.

## Backlog discipline

- P0: required to complete and measure the primary loop.
- P1/Phase 1.5: waitlist, push notifications, QR check-in, richer court availability, quality-of-life improvements.
- Phase 2+: community, coaching, and tournament capabilities in the Product Blueprint.

No later-phase item should enter an active milestone without a written product decision and updated PRD.
