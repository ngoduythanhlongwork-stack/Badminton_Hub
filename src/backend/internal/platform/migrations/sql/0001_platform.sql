CREATE SCHEMA IF NOT EXISTS platform;

CREATE TABLE IF NOT EXISTS platform.outbox_messages (
    id uuid PRIMARY KEY,
    topic text NOT NULL CHECK (topic <> ''),
    payload jsonb NOT NULL,
    idempotency_key text NOT NULL CHECK (idempotency_key <> ''),
    occurred_at timestamptz NOT NULL,
    available_at timestamptz NOT NULL,
    status text NOT NULL DEFAULT 'pending'
        CHECK (status IN ('pending', 'processing', 'processed')),
    attempt_count integer NOT NULL DEFAULT 0 CHECK (attempt_count >= 0),
    lease_token uuid,
    lease_until timestamptz,
    processed_at timestamptz,
    last_error text,
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (topic, idempotency_key),
    CHECK (
        (status = 'processing' AND lease_token IS NOT NULL AND lease_until IS NOT NULL)
        OR (status <> 'processing' AND lease_token IS NULL AND lease_until IS NULL)
    )
);

CREATE INDEX IF NOT EXISTS outbox_messages_claim_idx
    ON platform.outbox_messages (available_at, occurred_at)
    WHERE status IN ('pending', 'processing');
