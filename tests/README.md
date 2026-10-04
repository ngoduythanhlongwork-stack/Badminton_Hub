# Test strategy

- Go unit and HTTP tests live beside their packages as `*_test.go` under `src/backend`.
- Future integration tests cover PostgreSQL/Redis/provider adapters, migrations, idempotency and concurrency.
- Future architecture tests cover module references, forbidden dependencies and ownership boundaries.
- Web tests will live near features for components and under a future end-to-end project for critical journeys.

The first mandatory high-risk scenarios are competing joins for the last slot, schedule overlap,
duplicate transfer reports/Host acknowledgements, hold-expiry and cancellation/refund races,
and unauthorized Host/Admin actions. Payment-provider callbacks are outside the MVP.
