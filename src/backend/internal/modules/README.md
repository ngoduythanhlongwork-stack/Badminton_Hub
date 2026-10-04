# Module boundaries

Each module owns its domain, workflows, storage and contracts.
Only explicit public contracts/events may cross boundaries.
Use module-local `internal/` packages for private implementations.
Store forward SQL changes in `internal/modules/<owner>/migrations/`, qualify every table with the
owner's PostgreSQL schema, expose the immutable migration slice from the module root, and pass it
to the runner at the executable composition root. The runner builds and validates global order.
Application SQL may access only its owner's schema; cross-module behavior uses root-level contracts
or durable events. The architecture test enforces private-import and cross-schema restrictions.

| Module | Ownership |
| --- | --- |
| identity | Accounts, sessions, verification and permissions |
| players | Profiles, preferences and skill/reliability views |
| venues | Venues, courts, availability and discovery |
| matches | Match lifecycle, participation, eligibility and attendance |
| payments | Fee obligations, transfer reports, Host acknowledgements, receipts and refunds |
| notifications | Notification obligations and delivery |
| communication | Match-room membership and messages |
| recommendations | Candidate filtering, scoring and explanations |
| moderation | Blocks, reports and admin cases |

The package markers establish intended boundaries only. No product epic is implemented.
