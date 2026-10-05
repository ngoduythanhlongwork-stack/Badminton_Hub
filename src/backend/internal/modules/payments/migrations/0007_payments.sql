CREATE SCHEMA IF NOT EXISTS payments;

CREATE TABLE payments.obligations (
    id uuid PRIMARY KEY,
    participation_id uuid NOT NULL,
    match_id uuid NOT NULL,
    payer_id uuid NOT NULL,
    payee_id uuid NOT NULL,
    purpose text NOT NULL CHECK(purpose IN ('MATCH_DEPOSIT','MATCH_PAYMENT')),
    amount_minor bigint NOT NULL CHECK(amount_minor > 0),
    currency char(3) NOT NULL CHECK(currency='VND'),
    status text NOT NULL CHECK(status IN ('OPEN','PARTIALLY_SATISFIED','SATISFIED','CANCELLED')),
    due_at timestamptz NOT NULL,
    instruction_recipient text NOT NULL,
    instruction_text text NOT NULL,
    instruction_version integer NOT NULL CHECK(instruction_version > 0),
    policy_version text NOT NULL,
    source_event_id text NOT NULL,
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL,
    UNIQUE(participation_id,purpose),
    UNIQUE(source_event_id,purpose)
);
CREATE INDEX payments_obligations_party_idx ON payments.obligations(payer_id,payee_id,created_at DESC);

CREATE TABLE payments.transfer_reports (
    id uuid PRIMARY KEY,
    obligation_id uuid NOT NULL REFERENCES payments.obligations(id),
    reporter_id uuid NOT NULL,
    amount_minor bigint NOT NULL CHECK(amount_minor > 0),
    claimed_at timestamptz NOT NULL,
    reference text NOT NULL,
    evidence_ref text NOT NULL DEFAULT '',
    status text NOT NULL CHECK(status IN ('SUBMITTED','ACKNOWLEDGED','NOT_FOUND','DISPUTED')),
    resolution_reason text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL
);

CREATE TABLE payments.receipts (
    id uuid PRIMARY KEY,
    obligation_id uuid NOT NULL REFERENCES payments.obligations(id),
    transfer_report_id uuid NOT NULL UNIQUE REFERENCES payments.transfer_reports(id),
    acknowledged_by uuid NOT NULL,
    amount_minor bigint NOT NULL CHECK(amount_minor >= 0),
    state text NOT NULL CHECK(state IN ('RECORDED','ADJUSTED')),
    revision integer NOT NULL DEFAULT 1 CHECK(revision > 0),
    source text NOT NULL,
    recorded_at timestamptz NOT NULL,
    adjusted_at timestamptz
);
CREATE INDEX payments_receipts_obligation_idx ON payments.receipts(obligation_id,recorded_at);

CREATE TABLE payments.refunds (
    id uuid PRIMARY KEY,
    participation_id uuid NOT NULL,
    match_id uuid NOT NULL,
    payer_id uuid NOT NULL,
    payee_id uuid NOT NULL,
    source_event_id text NOT NULL UNIQUE,
    source_kind text NOT NULL CHECK(source_kind IN ('CANCELLATION','LATE_MONEY','OVERPAYMENT','ADMIN_DECISION')),
    policy_version text NOT NULL,
    amount_minor bigint NOT NULL CHECK(amount_minor > 0),
    currency char(3) NOT NULL CHECK(currency='VND'),
    status text NOT NULL CHECK(status IN ('DUE','OVERDUE','REPORTED_SENT','CONFIRMED','DISPUTED')),
    due_at timestamptz NOT NULL,
    sent_amount_minor bigint,
    sent_at timestamptz,
    sent_reference text NOT NULL DEFAULT '',
    evidence_ref text NOT NULL DEFAULT '',
    reported_by uuid,
    confirmed_by uuid,
    confirmed_at timestamptz,
    dispute_reason text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL
);
CREATE INDEX payments_refunds_party_idx ON payments.refunds(payer_id,payee_id,status,due_at);

CREATE TABLE payments.idempotency (
    actor_id uuid NOT NULL,
    action text NOT NULL,
    idempotency_key text NOT NULL,
    fingerprint text NOT NULL,
    resource_type text NOT NULL,
    resource_id uuid NOT NULL,
    created_at timestamptz NOT NULL,
    PRIMARY KEY(actor_id,action,idempotency_key)
);

CREATE TABLE payments.audit (
    id uuid PRIMARY KEY,
    actor_id uuid,
    action text NOT NULL,
    object_type text NOT NULL,
    object_id uuid NOT NULL,
    reason text NOT NULL DEFAULT '',
    source text NOT NULL,
    occurred_at timestamptz NOT NULL
);
