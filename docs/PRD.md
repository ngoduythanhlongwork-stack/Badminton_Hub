# Product Requirements Document — Matchmaking MVP

Status: **MVP scope and business specifications approved 2026-10-04**

## Confirmed pilot decisions and detailed specifications

Pilot participation is limited to adults aged 18 or older. Accounts use email and
password with email verification. Publishing a match requires a verified email,
completed eligible player profile and Admin-approved organizing capability.
Player and Host remain capabilities of the same account; match ownership defines Host permissions.

Hosts arrange courts outside the app and attest that they have a court. The app
does not reserve court inventory or guarantee that a booking was verified.
Players transfer money directly to the Host, who acknowledges receipt and performs
refunds outside the app. Reporting a transfer is not confirmation of receipt.
Payment-gateway confirmation, in-app court booking and automated settlement are future scope.

Read [the business specification index](specs/README.md), [decision register](specs/DECISIONS.md)
and [traceability matrix](specs/TRACEABILITY.md). D-01–D-25 and spec version 1.0 are approved
implementation requirements. A later decision may override them only when PRD/spec/AC are updated together.

## 1. Objective

Enable a badminton player to find and join a suitable match in the shortest practical time while giving hosts enough control to fill and operate a match reliably.

## 2. Primary journey

```text
Onboard -> Discover/Search -> Evaluate Match -> Eligibility Check
       -> Deposit/Payment -> Joined -> Reminder -> Check-in
       -> Complete -> Rate -> Update Reliability/Recommendation Signals
```

## 3. MVP epics

### E01 Authentication and authorization

P0 capabilities:

- Register, sign in, sign out, refresh session, forgot/reset password.
- Verify email; phone OTP and social sign-in are outside the current pilot baseline.
- Admin approves organizing capability after email verification and eligible onboarding.
- One identity may receive Player, Court Manager, or Admin permissions.
- Authorization uses policies/permissions, not scattered role booleans.

Social sign-in is optional and must not block the MVP.

### E02 Player onboarding

Collect:

- Name, avatar, date of birth, optional gender.
- Playing experience: `<3 months`, `3–12 months`, `1–3 years`, `3+ years`.
- Self-assessed level: Beginner, Beginner+, Intermediate, Intermediate+, Advanced, Competitive.
- Preferred formats: Singles, Doubles, Mixed.
- Style: Casual, Social, Training, Competitive.
- Usual periods: Morning, Afternoon, Evening.
- Regular playing area.

Onboarding completion must be explicit and resumable. Only players aged 18 or older
are eligible for the pilot. Initial skill is an estimate with confidence, not an unquestionable fact.

### E03 Player profile and trust

Show badminton identity: level/rating, match count/history, reliability, preferred formats, style, and area. Rating may use an internal numeric scale while presenting understandable level bands.

### E04 Venue discovery

List and detail views include photos, address/map location, opening hours, amenities,
price guidance, courts, and non-guaranteed availability information when supplied.
Distance, price and court type support discovery. Availability/rating filters require
a defined source; do not fabricate court inventory or a venue rating from match reviews.

Hosts book outside the app. Venue information supports matchmaking; no in-app
court reservations, booking payments or booking guarantees are included in this pilot.

### E05 Create match

An Admin-approved organizer selects venue/court context, attests that the court was
arranged outside the app, and supplies:

- Title and description.
- Date, start time, end time, and venue time zone.
- Game mode/format and play style.
- Minimum/maximum rating or level.
- Maximum participant count.
- Cost per player and deposit requirement.
- Join mode: Instant Join or Approval Required.
- Reliability requirement and match rules.

### E06 Find and evaluate match

Discovery supports Nearby, filters, and recommendations. Filters include time, skill, distance, game type, and cost.

A match card should support a fast decision with time, venue, distance, level, participants/capacity, cost, recommendation score, and primary action.

Detail adds host, participant social proof, court, rules, reliability requirement, and payment/cancellation information.

Natural-language search is deferred from this specification baseline. A future beta
may convert language into a validated DTO before normal search services; it cannot decide eligibility.

### E07 Join match

Eligibility checks:

- Match is open and in the future.
- Capacity is available.
- User is not banned/blocked from the relevant context.
- Skill and reliability satisfy rules or route to Host approval.
- User has no overlapping joined/confirmed match.
- Required deposit is acknowledged by the receiving Host, or the participation is
  explicitly pending under the approved hold workflow. A Player transfer report alone does not confirm joining.

Join must be idempotent. Concurrent attempts must not exceed capacity.

Skill mismatch should prefer a Join Request over a hard block when the Host allows approval.

### E08 Host match management

Host can view participants and pending requests; approve, reject, or remove; distinguish paid/unpaid/cancelled/no-show states; cancel the match; and message participants.

### E09 Check-in and attendance

MVP allows Host-confirmed check-in. Participant flow: `JOINED -> CHECKED_IN`. A missed check-in does not automatically become a no-show without Host confirmation. QR check-in is later.

### E10 Rating and reliability

After a completed match collect:

- Match quality: 1–5.
- Host rating: 1–5.
- Player tags: Friendly, Fair, Competitive, On Time, Good Skill Match.
- Skill feedback: As Expected, Stronger Than Profile, Lower Than Profile.

Do not ask players for a toxic-feeling numeric skill score. Use signals to adjust rating confidence.

Reliability receives positive signals for completion, negative signals for late cancellation, and stronger negative signals for confirmed no-show. The internal formula need not be public; the displayed result must remain understandable.

### E11 Payment lifecycle

MVP purposes: `MATCH_DEPOSIT`, `MATCH_PAYMENT`, `REFUND`.
`COURT_BOOKING`, provider callbacks and automated settlement are deferred.

Players transfer directly to Hosts. The app records the amount due, Player reports,
Host acknowledgements, refund obligations, reports of refund execution and their
confirmation/disputes. It does not hold money or certify bank transfers.

Requirements:

- Keep amount due, reported transfer, acknowledged receipt and refund lifecycle distinct.
- Host acknowledgement and repeat user actions are idempotent and auditable.
- Acknowledging money does not override match capacity, an expired hold or schedule conflicts.
- Money uses an integer minor unit plus currency; never use only `IsPaid`.
- Preserve evidence and corrections with actor/source/time; restrict access to relevant parties.
- Detailed payment outcomes and invariants follow D-13–D-17; physical state names remain a technical design choice.

Cancellation policy is approved by D-15: Player cancellation at least 6 hours before start receives
a full refund of acknowledged receipts; later Player cancellation receives no refund unless an Admin case decides otherwise.
Host cancellation or removal receives a full refund. MVP has no partial refund tier.

Policy version and the applied result must be stored so later configurable policies do not rewrite history.

### E12 Notifications

Required events: join confirmation/request/approval, player joined, match full/cancelled/reminder,
transfer reported, Host receipt acknowledged/not found, and refund obligation/report/confirmation/dispute.
Keep receipt and participation outcomes distinct. MVP channels are in-app plus email; push is deferred.

### E13 Match room communication

Provide only match-scoped communication. This is not a general social messenger. Membership follows confirmed participation/host status and respects moderation.

### E14 Recommendation V1

Hard filters:

- Open, future, and not full.
- No schedule conflict.
- Visibility/moderation rules permit display.

Weighted signals:

- Skill compatibility.
- Distance.
- Schedule preference.
- Game/style preference.
- Reliability.

Return score plus reasons such as “same skill level,” “1.5 km away,” or “matches your usual time.” Do not show an unexplained percentage.

## 4. Core business rules

### Capacity

Confirmed participants must not exceed `maxParticipants`. Paid-join holds also protect against
over-allocation and schedule conflicts according to D-13. Waitlist is deferred.

### Schedule conflict

Two intervals overlap when:

```text
existingStart < newEnd AND existingEnd > newStart
```

Only participation states that reserve the user's time are considered. Store instants in UTC and evaluate using normalized instants.

### Match state model

Approved high-level business lifecycle (physical enum design remains implementation-specific):

```text
DRAFT -> OPEN -> FULL -> IN_PROGRESS -> COMPLETED
             \-> CANCELLED
```

Transitions require explicit authorization and the invariants defined in `MATCHES.md`.

### Join state model

Initial state set:

```text
REQUESTED -> APPROVED -> PAYMENT_PENDING -> JOINED -> CHECKED_IN -> COMPLETED
        \-> REJECTED       \-> PAYMENT_FAILED
JOINED -> CANCELLED | NO_SHOW
```

Instant Join may skip `REQUESTED/APPROVED`. Payment-free matches may skip payment states.
This combined journey is not the database state model. Spec 1.0 separates participation,
holds and payment records; implementation enums must preserve those independent lifecycles.

## 5. Roles and permissions

Baseline capabilities:

- Player: view/join and manage own profile/activity; request organizing capability.
- Approved organizer: publish own matches after verified email and eligible onboarding.
- Court Manager: manage assigned venues, courts, and schedules.
- Admin: moderation, user/venue administration, and dispute operations.

Host is match ownership, not necessarily a global role.

## 6. Moderation

MVP supports report user, block user, and report match. Reasons: Spam, Fraud, Harassment, Fake Skill, No-show, Other. Admin handling may begin with a minimal internal queue, but reports and audit history must be durable.

## 7. Non-functional requirements

- Typical API response target: under 500 ms at pilot load.
- Recommendation target: under 1 second at pilot load.
- Pilot availability target: 99%.
- HTTPS, secure session/token handling, rate limiting, audit events, least privilege, and protection of personal/location data.
- Accessibility target: WCAG 2.2 AA for key journeys.
- Mobile-first PWA; degraded/offline behavior must never falsely confirm a join or payment.
- Horizontal scaling should remain possible; multi-node deployment is not required for pilot.

## 8. Analytics events

At minimum capture: onboarding completed, search submitted, recommendation viewed, match card viewed, match detail viewed, join started, join succeeded/failed with reason, payment result, cancellation, check-in, match completion, rating submitted, and recommendation reason exposure.

## 9. MVP sitemap

```text
Home
├── Find Match
│   ├── Search
│   ├── Recommendations
│   └── Match Detail
├── Create Match
├── Courts
│   ├── Search Court
│   └── Court Detail
├── My Activity
│   ├── Upcoming
│   ├── Hosted
│   └── History
├── Notifications
└── Profile
```

## 10. Explicitly out of scope

- Full social feed or general-purpose messenger.
- Club management.
- Coach marketplace and training courses.
- Tournament platform.
- Video analysis or livestream.
- Advanced wallet.
- Membership subscriptions.
- Gamification.
- Complex league ranking.
- In-app court booking, payment gateway callbacks, platform custody of money and automated settlement.
- AI search beta, waitlist, QR check-in and push are outside the current specification baseline.
