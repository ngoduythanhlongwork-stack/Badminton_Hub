# Product Blueprint

Status: **Approved baseline**
Product: **Badminton Hub**
Primary market assumption: Vietnam, starting with a geographically dense pilot

## 1. Vision and positioning

Badminton Hub is the operating layer connecting players, hosts, courts, coaches, clubs, and tournament organizers. The long-term ecosystem is broad, but the entry wedge is deliberately narrow:

> Find the right badminton game, with the right players, at the right time.

Vietnamese promise:

> **Muốn đánh cầu — luôn có kèo phù hợp.**

Badminton Hub V1 is **not primarily a court-booking app**. Court inventory supports matchmaking. Coach, club, tournament, and richer community products are later verticals.

## 2. Problem statement

The current experience is fragmented across courts, informal groups, Facebook/Zalo, spreadsheets, and manual bank transfers. Players without an established group struggle to find a game that fits location, time, skill, cost, and playing style. Hosts struggle to fill slots and manage attendance. Court owners have unused capacity and manual operations.

The product combines four capabilities:

```text
Discovery + Matchmaking + Booking + Trust
```

## 3. North-star experience

A user opens the app and can answer within roughly 30 seconds:

> “Tối nay có kèo nào phù hợp với tôi?”

Core loop:

```text
Discover -> Find Match -> Join -> Play -> Rate -> Build Rating
    ^                                                   |
    +------------- Better Recommendations <------------+
```

## 4. Users and roles

### MVP actors

| Actor | Primary job | Priority |
|---|---|---:|
| Player | Find, evaluate, and join a suitable match | P0 |
| Host | Create a match and manage participants | P0 |
| Court Manager | Publish venue/court information and availability | P0 supporting role |
| Admin | Moderate reports and resolve operational issues | P0 internal role |

A single identity may hold multiple capabilities. In the pilot, a Player needs
verified email, eligible completed onboarding (18+) and Admin-approved organizing
capability to publish a match and act as its Host; do not create separate accounts.

### Later actors

- Coach
- Student or parent
- Club manager
- Tournament organizer
- Company/event organizer

## 5. Value propositions

### Player

Find a game that fits skill, place, time, format, style, and budget without already belonging to a group. See slots, cost, host reliability, and participants before joining.

### Host

Fill matches faster, approve or reject requests, manage slots and attendance, collect deposits, reduce no-shows, and communicate within the match context.

### Court Manager

Improve court utilization through discoverability and published venue information.
Hosts arrange courts outside the app in the pilot; integrated booking is a later capability.

## 6. Product assets and defensibility

Long-term defensibility comes from three compounding assets:

1. Player network and trust history.
2. Badminton rating and rating confidence.
3. Match, attendance, preference, and venue behavior data that improves recommendations.

AI is an enabling layer, not the product by itself.

## 7. MVP hypotheses

The MVP exists to test:

1. Players will join games with people outside their existing group.
2. Hosts will publish and manage games in Badminton Hub instead of relying only on Facebook/Zalo.
3. Relevant, explainable recommendations improve the probability of joining.

If these are not validated, do not accelerate later marketplaces.

## 8. Working success targets

These are pilot targets, not market benchmarks:

| Metric | Working target |
|---|---:|
| Registered players | 300–500 |
| Monthly active players | 100 |
| Completed matches/month | 30–50 |
| Match fill rate | > 60% |
| Search -> match view | > 40% |
| Match view -> join | > 20% |
| Repeat within 30 days | > 30% |
| No-show rate | < 10% |

Instrument the funnel before treating any target as validated.

## 9. Product phases

### Phase 1 — Matchmaking MVP

Email/password authentication, adult onboarding, player profile, skill level, venue
listing/discovery, approved Hosts creating/managing matches, find/join, direct-to-Host
transfer acknowledgement and refund tracking, check-in, reliability, ratings,
in-app/email notifications, match-scoped communication and explainable basic recommendation.
Hosts book courts outside the app. Natural-language search is deferred from the current spec baseline.

The [business specification set](specs/README.md) version 1.0 and decisions D-01–D-25
are PO-approved inputs for ERD/API design and feature implementation by vertical slice.

### Phase 2 — Community

Friends/follows, groups/clubs, invitations, richer chat, deeper history, and personalized recommendations.

### Phase 3 — Coaching

Coach profiles and booking, training products, progress tracking, coach recommendations, and training plans.

### Phase 4 — Tournament

Registration, brackets, court assignment, scoring, ranking, schedule optimization, and team balancing.

Video analysis remains a later high-complexity capability, not an MVP dependency.

## 10. Commercial hypotheses

Potential future revenue streams include court-booking commission, per-player open-play fees, coach commission, tournament SaaS, court-management SaaS, and premium player capabilities. Pricing is not yet validated and is not an MVP implementation commitment.
